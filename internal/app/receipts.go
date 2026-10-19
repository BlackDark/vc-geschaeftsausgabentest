package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // decode configs for pixel limits
	_ "image/png"
	"path/filepath"
	"strings"
)

// Receipt is evidence: an uploaded file or a self-issued receipt.
type Receipt struct {
	ID          int64        `json:"id"`
	Kind        string       `json:"kind" enum:"file,self_issued"`
	Status      string       `json:"status" enum:"inbox,confirmed,void"`
	Number      string       `json:"number"`
	Filename    string       `json:"filename"`
	Mime        string       `json:"mime"`
	Size        int64        `json:"size"`
	Pages       int          `json:"pages"`
	SHA256      string       `json:"sha256"`
	Duplicate   bool         `json:"duplicate" doc:"another receipt of this user has the same SHA-256"`
	HasPreview  bool         `json:"hasPreview"`
	SelfIssued  *SelfIssued  `json:"selfIssued,omitempty"`
	Note        string       `json:"note"`
	VoidReason  string       `json:"voidReason,omitempty"`
	CreatedAt   string       `json:"createdAt"`
	ConfirmedAt string       `json:"confirmedAt,omitempty"`
	Expenses    []ExpenseRef `json:"expenses"`
}

// ExpenseRef is an expense a receipt supports.
type ExpenseRef struct {
	ID          int64  `json:"id"`
	TripID      *int64 `json:"tripId"`
	Date        string `json:"date"`
	Supplier    string `json:"supplier"`
	Description string `json:"description"`
	Locked      bool   `json:"locked"`
}

// SelfIssued is an Eigenbeleg for missing third-party evidence (P11).
type SelfIssued struct {
	Date        string `json:"date" format:"date"`
	AmountMinor int64  `json:"amountMinor" minimum:"1"`
	Currency    string `json:"currency" minLength:"3" maxLength:"3"`
	Payee       string `json:"payee" minLength:"1"`
	Purpose     string `json:"purpose" minLength:"1"`
	Issuer      string `json:"issuer" minLength:"1"`
	Reason      string `json:"reason" minLength:"1" doc:"why no third-party receipt exists"`
}

// detect validates a file by content and returns its MIME type and extension.
func detect(b []byte) (string, string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", ".jpg", true
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", ".png", true
	case bytes.HasPrefix(b, []byte("%PDF-")):
		return "application/pdf", ".pdf", true
	}
	return "", "", false
}

// Upload stores an original receipt file in the user's inbox (P10).
func (a *App) Upload(ctx context.Context, u *User, filename string, data []byte, note string) (*Receipt, error) {
	if int64(len(data)) > a.Cfg.MaxUploadBytes {
		return nil, Invalid("upload.too_large", "file exceeds the size limit", "limitMb", fmt.Sprint(a.Cfg.MaxUploadBytes>>20))
	}
	mime, ext, ok := detect(data)
	if !ok {
		return nil, Invalid("upload.unsupported", "only JPEG, PNG and PDF files are supported")
	}
	pages := 1
	if mime == "application/pdf" {
		if bytes.Contains(data, []byte("/Encrypt")) {
			return nil, Invalid("upload.pdf_encrypted", "encrypted PDFs are not supported; please upload an unprotected copy")
		}
		n, err := a.Typst.PDFInfo(ctx, data)
		if err != nil {
			return nil, Invalid("upload.pdf_invalid", "the PDF could not be read")
		}
		if n > a.Cfg.MaxPDFPages {
			return nil, Invalid("upload.too_many_pages", "the PDF has too many pages", "limit", fmt.Sprint(a.Cfg.MaxPDFPages))
		}
		pages = n
	} else {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, Invalid("upload.image_invalid", "the image could not be read")
		}
		if int64(cfg.Width)*int64(cfg.Height) > a.Cfg.MaxPixels {
			return nil, Invalid("upload.too_many_pixels", "the image resolution is too large")
		}
	}
	key, sum, size, err := a.Files.Put("files", ext, bytes.NewReader(data), a.Cfg.MaxUploadBytes)
	if err != nil {
		return nil, err
	}
	// Rendering the preview also validates the whole file: Typst decodes it
	// exactly as the claim PDF will.
	png, err := a.Typst.Preview(ctx, data, ext, 600)
	if err != nil {
		a.Files.Remove(key)
		a.Log.Info("receipt rejected", "reason", "preview failed")
		return nil, Invalid("upload.unreadable", "the file could not be read; please upload a valid JPEG, PNG or PDF")
	}
	previewKey, _, _, err := a.Files.Put("files", ".png", bytes.NewReader(png), 1<<26)
	if err != nil {
		a.Files.Remove(key)
		return nil, err
	}
	name := sanitizeName(filename, ext)
	var id int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO receipts(user_id, kind, status, sha256, filename, mime, size, pages, storage_key, preview_key, note, created_at) VALUES (?, 'file', 'inbox', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, sum, name, mime, size, pages, key, previewKey, note, a.stamp())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		return a.audit(ctx, tx, u.ID, u.ID, "receipt", id, "upload", "", map[string]any{"sha256": sum, "size": size, "filename": name})
	})
	if err != nil {
		a.Files.Remove(key)
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

func sanitizeName(name, ext string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "beleg" + ext
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

// allocateReceiptNumber assigns the next number for the user and year.
// It runs inside the confirming transaction, so failures consume nothing.
func (a *App) allocateReceiptNumber(ctx context.Context, tx *sql.Tx, userID int64) (string, int, int, error) {
	year := a.now().Year()
	var seq int
	if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(number_seq), 0) + 1 FROM receipts WHERE user_id = ? AND number_year = ?`, userID, year).Scan(&seq); err != nil {
		return "", 0, 0, err
	}
	return fmt.Sprintf("B-%d-%04d", year, seq), year, seq, nil
}

// ConfirmReceipt records the user's visual confirmation and assigns a
// permanent receipt number. Confirmed bytes are immutable.
func (a *App) ConfirmReceipt(ctx context.Context, u *User, id int64) (*Receipt, error) {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM receipts WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&status); err != nil {
			return notFound(err)
		}
		if status != "inbox" {
			return Invalid("receipt.not_in_inbox", "only inbox receipts can be confirmed")
		}
		number, year, seq, err := a.allocateReceiptNumber(ctx, tx, u.ID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE receipts SET status = 'confirmed', number = ?, number_year = ?, number_seq = ?, confirmed_at = ? WHERE id = ?`, number, year, seq, a.stamp(), id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "receipt", id, "confirm", "", map[string]any{"number": number})
	})
	if err != nil {
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

// CreateSelfIssued records an Eigenbeleg; it is confirmed on creation.
func (a *App) CreateSelfIssued(ctx context.Context, u *User, in SelfIssued) (*Receipt, error) {
	if err := validDate(in.Date); err != nil {
		return nil, err
	}
	for _, f := range []string{in.Payee, in.Purpose, in.Issuer, in.Reason} {
		if strings.TrimSpace(f) == "" {
			return nil, Invalid("self_issued.incomplete", "all fields are required")
		}
	}
	b, _ := json.Marshal(in)
	var id int64
	err := a.tx(ctx, func(tx *sql.Tx) error {
		number, year, seq, err := a.allocateReceiptNumber(ctx, tx, u.ID)
		if err != nil {
			return err
		}
		now := a.stamp()
		res, err := tx.ExecContext(ctx, `INSERT INTO receipts(user_id, kind, status, self_json, filename, number, number_year, number_seq, confirmed_at, created_at) VALUES (?, 'self_issued', 'confirmed', ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, string(b), "Eigenbeleg "+number, number, year, seq, now, now)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		return a.audit(ctx, tx, u.ID, u.ID, "receipt", id, "self_issued", in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

// VoidReceipt retires evidence that no locked trip/outlay uses. The record
// and its number are retained.
func (a *App) VoidReceipt(ctx context.Context, u *User, id int64, reason string) (*Receipt, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, Invalid("receipt.void_reason", "a reason is required")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM receipts WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&status); err != nil {
			return notFound(err)
		}
		if status == "void" {
			return nil
		}
		rows, err := tx.QueryContext(ctx, `SELECT expense_id FROM receipt_links WHERE receipt_id = ?`, id)
		if err != nil {
			return err
		}
		var exps []int64
		for rows.Next() {
			var e int64
			if err := rows.Scan(&e); err != nil {
				rows.Close()
				return err
			}
			exps = append(exps, e)
		}
		rows.Close()
		for _, e := range exps {
			if locked, err := expenseLocked(ctx, tx, e); err != nil || locked {
				if err == nil {
					err = ErrLocked
				}
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM receipt_links WHERE receipt_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE receipts SET status = 'void', void_reason = ? WHERE id = ?`, reason, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "receipt", id, "void", reason, map[string]any{"unlinked": exps})
	})
	if err != nil {
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

// DeleteInboxReceipt removes an unconfirmed upload that is not linked.
func (a *App) DeleteInboxReceipt(ctx context.Context, u *User, id int64) error {
	var key, preview string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status, storage_key, preview_key FROM receipts WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&status, &key, &preview); err != nil {
			return notFound(err)
		}
		if status != "inbox" {
			return Invalid("receipt.confirmed_immutable", "confirmed receipts can only be voided")
		}
		var links int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM receipt_links WHERE receipt_id = ?`, id).Scan(&links); err != nil {
			return err
		}
		if links > 0 {
			return Invalid("receipt.linked", "unlink the receipt first")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM receipts WHERE id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "receipt", id, "delete_unconfirmed", "", nil)
	})
	if err == nil {
		a.Files.Remove(key)
		a.Files.Remove(preview)
	}
	return err
}

// GetReceipt loads one owned receipt.
func (a *App) GetReceipt(ctx context.Context, u *User, id int64) (*Receipt, error) {
	list, err := a.loadReceipts(ctx, u.ID, "r.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

// ListReceipts lists receipts by status ("" for all).
func (a *App) ListReceipts(ctx context.Context, u *User, status string) ([]*Receipt, error) {
	if status == "" {
		return a.loadReceipts(ctx, u.ID, "1 = 1")
	}
	return a.loadReceipts(ctx, u.ID, "r.status = ?", status)
}

func (a *App) loadReceipts(ctx context.Context, userID int64, where string, args ...any) ([]*Receipt, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT r.id, r.kind, r.status, coalesce(r.number, ''), r.filename, r.mime, r.size, r.pages, r.sha256, r.preview_key <> '', r.self_json, r.note, r.void_reason, r.created_at, coalesce(r.confirmed_at, ''),
		(SELECT count(*) FROM receipts d WHERE d.user_id = r.user_id AND d.sha256 = r.sha256 AND d.sha256 <> '' AND d.id <> r.id AND d.status <> 'void') > 0
		FROM receipts r WHERE r.user_id = ? AND `+where+` ORDER BY r.id DESC`, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	var out []*Receipt
	for rows.Next() {
		r := &Receipt{}
		var self sql.NullString
		if err := rows.Scan(&r.ID, &r.Kind, &r.Status, &r.Number, &r.Filename, &r.Mime, &r.Size, &r.Pages, &r.SHA256, &r.HasPreview, &self, &r.Note, &r.VoidReason, &r.CreatedAt, &r.ConfirmedAt, &r.Duplicate); err != nil {
			rows.Close()
			return nil, err
		}
		if self.Valid {
			r.SelfIssued = &SelfIssued{}
			_ = json.Unmarshal([]byte(self.String), r.SelfIssued)
		}
		out = append(out, r)
	}
	rows.Close()
	for _, r := range out {
		r.Expenses = []ExpenseRef{}
		rows, err := a.DB.QueryContext(ctx, `SELECT e.id, e.trip_id, e.date, e.supplier, e.description FROM receipt_links l JOIN expenses e ON e.id = l.expense_id WHERE l.receipt_id = ? ORDER BY e.date`, r.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e ExpenseRef
			if err := rows.Scan(&e.ID, &e.TripID, &e.Date, &e.Supplier, &e.Description); err != nil {
				rows.Close()
				return nil, err
			}
			r.Expenses = append(r.Expenses, e)
		}
		rows.Close()
		for i := range r.Expenses {
			r.Expenses[i].Locked, _ = expenseLocked(ctx, a.DB, r.Expenses[i].ID)
		}
	}
	return out, nil
}

// ReceiptFile returns an owned receipt's original or preview bytes.
func (a *App) ReceiptFile(ctx context.Context, u *User, id int64, preview bool) ([]byte, string, string, error) {
	var key, previewKey, mime, name string
	err := a.DB.QueryRowContext(ctx, `SELECT storage_key, preview_key, mime, filename FROM receipts WHERE id = ? AND user_id = ? AND kind = 'file'`, id, u.ID).Scan(&key, &previewKey, &mime, &name)
	if err != nil {
		return nil, "", "", notFound(err)
	}
	if preview {
		if previewKey == "" {
			return nil, "", "", ErrNotFound
		}
		key, mime = previewKey, "image/png"
	}
	b, err := a.Files.Read(key)
	return b, mime, name, err
}
