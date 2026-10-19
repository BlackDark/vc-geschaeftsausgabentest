// Package app holds the domain services: persistence, ownership checks,
// audit events and claim submission. HTTP handlers live in internal/api.
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/export"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/rates"
)

// Error is a domain error with a stable code the UI translates.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Params  map[string]string `json:"params,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Common errors. Missing and foreign objects look the same (P01).
var (
	ErrNotFound     = &Error{Status: 404, Code: "not_found", Message: "not found"}
	ErrConflict     = &Error{Status: 409, Code: "conflict", Message: "the record was changed by someone else; reload and try again"}
	ErrLocked       = &Error{Status: 409, Code: "locked", Message: "the record belongs to a submitted claim"}
	ErrUnauthorized = &Error{Status: 401, Code: "unauthorized", Message: "login required"}
	ErrForbidden    = &Error{Status: 403, Code: "forbidden", Message: "not allowed"}
)

// Invalid returns a validation error.
func Invalid(code, msg string, params ...string) *Error {
	e := &Error{Status: 422, Code: code, Message: msg}
	if len(params) > 0 {
		e.Params = map[string]string{}
		for i := 0; i+1 < len(params); i += 2 {
			e.Params[params[i]] = params[i+1]
		}
	}
	return e
}

// Config configures the services.
type Config struct {
	DataDir        string
	Typst          string
	MaxUploadBytes int64
	MaxPixels      int64
	MaxPDFPages    int
	FontDir        string
}

// App bundles the database, rate tables and file storage.
type App struct {
	DB    *sql.DB
	Cfg   Config
	Log   *slog.Logger
	Files *Files
	Typst *export.Typst

	ratesMu sync.RWMutex
	rates   *rates.Table
	now     func() time.Time
}

// New creates the services and loads the rate tables.
func New(ctx context.Context, db *sql.DB, cfg Config, log *slog.Logger) (*App, error) {
	if err := rates.Seed(ctx, db); err != nil {
		return nil, fmt.Errorf("seed rates: %w", err)
	}
	files, err := NewFiles(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	a := &App{DB: db, Cfg: cfg, Log: log, Files: files, now: time.Now, Typst: &export.Typst{Bin: cfg.Typst, FontDir: cfg.FontDir}}
	if err := a.ReloadRates(ctx); err != nil {
		return nil, err
	}
	if err := a.recoverSubmissions(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// ReloadRates refreshes the in-memory rate table after an import or override.
func (a *App) ReloadRates(ctx context.Context) error {
	t, err := rates.Load(ctx, a.DB)
	if err != nil {
		return err
	}
	a.ratesMu.Lock()
	a.rates = t
	a.ratesMu.Unlock()
	return nil
}

// Rates returns the current rate table.
func (a *App) Rates() *rates.Table {
	a.ratesMu.RLock()
	defer a.ratesMu.RUnlock()
	return a.rates
}

func (a *App) stamp() string { return a.now().UTC().Format(time.RFC3339) }

// tx runs fn in a transaction.
func (a *App) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// audit appends an audit event inside the transaction committing the change (P18).
func (a *App) audit(ctx context.Context, tx *sql.Tx, owner, actor int64, entity string, id int64, action, reason string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if data == nil {
		b = []byte("{}")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_log(owner_id, actor_id, entity, entity_id, action, reason, data_json, at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		owner, actor, entity, id, action, reason, string(b), a.stamp())
	return err
}

// notFound maps sql.ErrNoRows to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// AuditEntry is one audit record visible to its owner.
type AuditEntry struct {
	ID       int64  `json:"id"`
	Entity   string `json:"entity"`
	EntityID int64  `json:"entityId"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Data     string `json:"data"`
	At       string `json:"at"`
}

// Audit lists the user's audit trail, newest first.
func (a *App) Audit(ctx context.Context, userID int64, limit int) ([]AuditEntry, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, entity, coalesce(entity_id, 0), action, reason, data_json, at FROM audit_log WHERE owner_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Entity, &e.EntityID, &e.Action, &e.Reason, &e.Data, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func formatInt(n int64) string { return strconv.FormatInt(n, 10) }
