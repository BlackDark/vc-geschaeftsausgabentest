package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/money"
)

// ExpenseInput creates or updates an expense. TripID is fixed at creation:
// trip-linked expenses enter a claim through their trip only.
type ExpenseInput struct {
	TripID            *int64            `json:"tripId,omitempty"`
	Date              string            `json:"date" format:"date"`
	CostType          calc.CostType     `json:"costType" enum:"transport,meals,lodging,incidental,hospitality,tools_equipment,software_services,other_outlay"`
	Payer             calc.Payer        `json:"payer" enum:"user,employer"`
	Supplier          string            `json:"supplier,omitempty" maxLength:"200"`
	Description       string            `json:"description,omitempty" maxLength:"1000"`
	Currency          string            `json:"currency" minLength:"3" maxLength:"3"`
	GrossMinor        int64             `json:"grossMinor" minimum:"1" doc:"gross amount in minor units of the currency"`
	FXRate            string            `json:"fxRate,omitempty" doc:"foreign units per EUR 1"`
	FXDate            string            `json:"fxDate,omitempty"`
	EURDebit          *int64            `json:"eurDebit,omitempty" doc:"actual EUR debit in cents"`
	VATUnknown        bool              `json:"vatUnknown"`
	VAT               []calc.VATPortion `json:"vat,omitempty"`
	Nights            []string          `json:"nights,omitempty" doc:"lodging only: nights covered by this invoice"`
	IncludesBreakfast bool              `json:"includesBreakfast"`
	Fuel              bool              `json:"fuel"`
	Hospitality       *calc.Hospitality `json:"hospitality,omitempty"`
	ReceiptIDs        []int64           `json:"receiptIds,omitempty"`
	Version           int               `json:"version,omitempty"`
}

// ReceiptRef is a linked receipt.
type ReceiptRef struct {
	ID     int64  `json:"id"`
	Number string `json:"number"`
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
}

// Expense is a stored expense.
type Expense struct {
	ExpenseInput
	ID         int64        `json:"id"`
	EmployerID int64        `json:"employerId"`
	ClaimID    *int64       `json:"claimId"`
	Locked     bool         `json:"locked"`
	Receipts   []ReceiptRef `json:"receipts"`
}

func (in *ExpenseInput) normalize() error {
	if err := validDate(in.Date); err != nil {
		return err
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if _, ok := money.Exponent(in.Currency); !ok {
		return Invalid("expense.currency_invalid", "unknown currency")
	}
	if in.GrossMinor <= 0 {
		return Invalid("expense.amount_invalid", "amount must be positive")
	}
	if in.Currency == "EUR" {
		in.FXRate, in.FXDate, in.EURDebit = "", "", nil
	} else if in.FXRate != "" {
		if _, err := money.ParseRate(in.FXRate); err != nil {
			return Invalid("fx.rate_invalid", "invalid exchange rate")
		}
	}
	if in.FXDate != "" {
		if err := validDate(in.FXDate); err != nil {
			return err
		}
	}
	if in.TripID == nil && !in.CostType.IsOutlay() {
		return Invalid("expense.outlay_category", "standalone outlays use tools, software or other")
	}
	if in.TripID != nil && !in.CostType.IsTravel() {
		return Invalid("expense.travel_category", "trip expenses use a travel cost type")
	}
	if in.TripID == nil && in.Payer != calc.PayerUser {
		return Invalid("expense.outlay_payer", "standalone outlays are personally paid")
	}
	if in.CostType != calc.CostLodging {
		in.Nights, in.IncludesBreakfast = nil, false
	}
	for _, n := range in.Nights {
		if err := validDate(n); err != nil {
			return err
		}
	}
	if in.CostType != calc.CostHospitality {
		in.Hospitality = nil
	}
	if len(in.VAT) > 0 {
		in.VATUnknown = false
	}
	return nil
}

// expenseLocked reports whether an expense is locked by a submitted claim.
func expenseLocked(ctx context.Context, q querier, id int64) (bool, error) {
	var status sql.NullString
	err := q.QueryRowContext(ctx, `SELECT coalesce(ct.status, cs.status) FROM expenses e
		LEFT JOIN trips t ON t.id = e.trip_id LEFT JOIN claims ct ON ct.id = t.claim_id
		LEFT JOIN claims cs ON cs.id = e.claim_id WHERE e.id = ?`, id).Scan(&status)
	if err != nil {
		return false, notFound(err)
	}
	return status.Valid && status.String != "draft", nil
}

// CreateExpense records a trip expense or standalone outlay.
func (a *App) CreateExpense(ctx context.Context, u *User, in ExpenseInput) (*Expense, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id int64
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var employerID int64
		if in.TripID != nil {
			if err := a.editableTrip(ctx, tx, u.ID, *in.TripID, 0); err != nil {
				return err
			}
			if err := tx.QueryRowContext(ctx, `SELECT employer_id FROM trips WHERE id = ?`, *in.TripID).Scan(&employerID); err != nil {
				return err
			}
		} else {
			emp, err := requireEmployer(ctx, tx, u.ID)
			if err != nil {
				return err
			}
			employerID = emp.ID
		}
		vat, nights, hosp := expenseJSON(in)
		now := a.stamp()
		res, err := tx.ExecContext(ctx, `INSERT INTO expenses(user_id, employer_id, trip_id, date, cost_type, payer, supplier, description, currency, gross_minor, fx_rate, fx_date, eur_debit, vat_unknown, vat_json, nights_json, includes_breakfast, fuel, hospitality_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, employerID, in.TripID, in.Date, in.CostType, in.Payer, in.Supplier, in.Description, in.Currency, in.GrossMinor, in.FXRate, nullStr(in.FXDate), in.EURDebit, boolInt(in.VATUnknown), vat, nights, boolInt(in.IncludesBreakfast), boolInt(in.Fuel), hosp, now, now)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		if err := a.linkReceipts(ctx, tx, u.ID, id, in.ReceiptIDs); err != nil {
			return err
		}
		if err := a.suggestBreakfast(ctx, tx, u.ID, id, in); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "expense", id, "create", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetExpense(ctx, u, id)
}

// UpdateExpense changes an editable expense. Its trip cannot change.
func (a *App) UpdateExpense(ctx context.Context, u *User, id int64, in ExpenseInput) (*Expense, error) {
	cur, err := a.GetExpense(ctx, u, id)
	if err != nil {
		return nil, err
	}
	in.TripID = cur.TripID
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if locked, err := expenseLocked(ctx, tx, id); err != nil || locked {
			if err == nil {
				err = ErrLocked
			}
			return err
		}
		vat, nights, hosp := expenseJSON(in)
		res, err := tx.ExecContext(ctx, `UPDATE expenses SET date = ?, cost_type = ?, payer = ?, supplier = ?, description = ?, currency = ?, gross_minor = ?, fx_rate = ?, fx_date = ?, eur_debit = ?, vat_unknown = ?, vat_json = ?, nights_json = ?, includes_breakfast = ?, fuel = ?, hospitality_json = ?, version = version + 1, updated_at = ?
			WHERE id = ? AND user_id = ? AND (? = 0 OR version = ?)`,
			in.Date, in.CostType, in.Payer, in.Supplier, in.Description, in.Currency, in.GrossMinor, in.FXRate, nullStr(in.FXDate), in.EURDebit, boolInt(in.VATUnknown), vat, nights, boolInt(in.IncludesBreakfast), boolInt(in.Fuel), hosp, a.stamp(), id, u.ID, in.Version, in.Version)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM receipt_links WHERE expense_id = ?`, id); err != nil {
			return err
		}
		if err := a.linkReceipts(ctx, tx, u.ID, id, in.ReceiptIDs); err != nil {
			return err
		}
		if err := a.suggestBreakfast(ctx, tx, u.ID, id, in); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "expense", id, "update", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetExpense(ctx, u, id)
}

// DeleteExpense removes an editable expense that is not in a claim.
func (a *App) DeleteExpense(ctx context.Context, u *User, id int64) error {
	cur, err := a.GetExpense(ctx, u, id)
	if err != nil {
		return err
	}
	if cur.Locked {
		return ErrLocked
	}
	if cur.ClaimID != nil {
		return Invalid("expense.in_claim", "remove the outlay from its claim first")
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM receipt_links WHERE expense_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM expenses WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "expense", id, "delete", "", cur.ExpenseInput)
	})
}

// suggestBreakfast proposes a provided breakfast on the day after each hotel
// night whose invoice includes breakfast (P08, C13). The flag stays visible
// and editable; its origin is kept.
func (a *App) suggestBreakfast(ctx context.Context, tx *sql.Tx, userID, expenseID int64, in ExpenseInput) error {
	if in.TripID == nil || in.CostType != calc.CostLodging || !in.IncludesBreakfast {
		return nil
	}
	var startAt, startTZ, endAt, endTZ string
	if err := tx.QueryRowContext(ctx, `SELECT start_at, start_tz, end_at, end_tz FROM trips WHERE id = ?`, *in.TripID).Scan(&startAt, &startTZ, &endAt, &endTZ); err != nil {
		return err
	}
	_, last := localString(endAt, endTZ)
	origin := "hotel:" + itoa64(expenseID)
	var suggested []string
	for _, n := range in.Nights {
		d := calc.MustDate(n).AddDays(1).String()
		if d > last {
			continue
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO trip_days(trip_id, date, breakfast, breakfast_origin) VALUES (?, ?, 1, ?)
			ON CONFLICT(trip_id, date) DO UPDATE SET breakfast = 1, breakfast_origin = excluded.breakfast_origin WHERE trip_days.breakfast = 0 AND trip_days.breakfast_origin = ''`, *in.TripID, d, origin)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			suggested = append(suggested, d)
		}
	}
	if len(suggested) == 0 {
		return nil
	}
	return a.audit(ctx, tx, userID, userID, "trip", *in.TripID, "suggest_breakfast", "", map[string]any{"expense": expenseID, "dates": suggested})
}

func (a *App) linkReceipts(ctx context.Context, tx *sql.Tx, userID, expenseID int64, ids []int64) error {
	for _, rid := range ids {
		var owner int64
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT user_id, status FROM receipts WHERE id = ?`, rid).Scan(&owner, &status); err != nil || owner != userID {
			return Invalid("receipt.not_found", "receipt not found")
		}
		if status == "void" {
			return Invalid("receipt.void", "a voided receipt cannot be linked")
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO receipt_links(receipt_id, expense_id) VALUES (?, ?)`, rid, expenseID); err != nil {
			return err
		}
	}
	return nil
}

func expenseJSON(in ExpenseInput) (vat, nights string, hosp any) {
	b, _ := json.Marshal(nonNil(in.VAT))
	n, _ := json.Marshal(nonNil(in.Nights))
	if in.Hospitality != nil {
		h, _ := json.Marshal(in.Hospitality)
		hosp = string(h)
	}
	return string(b), string(n), hosp
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetExpense loads one owned expense.
func (a *App) GetExpense(ctx context.Context, u *User, id int64) (*Expense, error) {
	list, err := a.loadExpenses(ctx, a.DB, u.ID, "e.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

// ListOutlays lists standalone outlays.
func (a *App) ListOutlays(ctx context.Context, u *User) ([]*Expense, error) {
	return a.loadExpenses(ctx, a.DB, u.ID, "e.trip_id IS NULL")
}

// TripExpenses lists a trip's expenses.
func (a *App) TripExpenses(ctx context.Context, u *User, tripID int64) ([]*Expense, error) {
	return a.loadExpenses(ctx, a.DB, u.ID, "e.trip_id = ?", tripID)
}

func (a *App) loadExpenses(ctx context.Context, q querier, userID int64, where string, args ...any) ([]*Expense, error) {
	rows, err := q.QueryContext(ctx, `SELECT e.id, e.employer_id, e.trip_id, e.claim_id, coalesce(ct.status, cs.status, ''), e.date, e.cost_type, e.payer, e.supplier, e.description, e.currency, e.gross_minor, e.fx_rate, coalesce(e.fx_date, ''), e.eur_debit, e.vat_unknown, e.vat_json, e.nights_json, e.includes_breakfast, e.fuel, e.hospitality_json, e.version
		FROM expenses e LEFT JOIN trips t ON t.id = e.trip_id LEFT JOIN claims ct ON ct.id = t.claim_id LEFT JOIN claims cs ON cs.id = e.claim_id
		WHERE e.user_id = ? AND `+where+` ORDER BY e.date DESC, e.id DESC`, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	var out []*Expense
	for rows.Next() {
		e := &Expense{}
		var status, vat, nights string
		var hosp sql.NullString
		if err := rows.Scan(&e.ID, &e.EmployerID, &e.TripID, &e.ClaimID, &status, &e.Date, &e.CostType, &e.Payer, &e.Supplier, &e.Description, &e.Currency, &e.GrossMinor, &e.FXRate, &e.FXDate, &e.EURDebit, &e.VATUnknown, &vat, &nights, &e.IncludesBreakfast, &e.Fuel, &hosp, &e.Version); err != nil {
			rows.Close()
			return nil, err
		}
		e.Locked = status == "submitted" || status == "paid"
		_ = json.Unmarshal([]byte(vat), &e.VAT)
		_ = json.Unmarshal([]byte(nights), &e.Nights)
		if hosp.Valid {
			e.Hospitality = &calc.Hospitality{}
			_ = json.Unmarshal([]byte(hosp.String), e.Hospitality)
		}
		out = append(out, e)
	}
	rows.Close()
	for _, e := range out {
		refs, err := receiptRefs(ctx, q, e.ID)
		if err != nil {
			return nil, err
		}
		e.Receipts = refs
		e.ReceiptIDs = make([]int64, len(refs))
		for i, r := range refs {
			e.ReceiptIDs[i] = r.ID
		}
	}
	return out, nil
}

func receiptRefs(ctx context.Context, q querier, expenseID int64) ([]ReceiptRef, error) {
	rows, err := q.QueryContext(ctx, `SELECT r.id, coalesce(r.number, ''), r.status, r.kind, r.filename FROM receipt_links l JOIN receipts r ON r.id = l.receipt_id WHERE l.expense_id = ? ORDER BY r.id`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReceiptRef{}
	for rows.Next() {
		var r ReceiptRef
		if err := rows.Scan(&r.ID, &r.Number, &r.Status, &r.Kind, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func itoa64(n int64) string { return formatInt(n) }
