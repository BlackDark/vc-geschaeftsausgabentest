package app

import (
	"context"
	"database/sql"
	"strings"
)

// Employer is the organization reimbursing the user. The UI manages one
// default employer per user (DECISIONS 2026-10-19).
type Employer struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Version int    `json:"version"`
}

// EmployerInput creates or updates the employer.
type EmployerInput struct {
	Name    string `json:"name" minLength:"1" maxLength:"200"`
	Address string `json:"address" maxLength:"500"`
	Version int    `json:"version,omitempty"`
}

// DefaultEmployer returns the user's employer or ErrNotFound.
func (a *App) DefaultEmployer(ctx context.Context, userID int64) (*Employer, error) {
	return defaultEmployer(ctx, a.DB, userID)
}

func defaultEmployer(ctx context.Context, q querier, userID int64) (*Employer, error) {
	var e Employer
	err := q.QueryRowContext(ctx, `SELECT id, name, address, version FROM employers WHERE user_id = ? AND is_default = 1 ORDER BY id LIMIT 1`, userID).Scan(&e.ID, &e.Name, &e.Address, &e.Version)
	if err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

// requireEmployer returns the default employer or a validation error.
func requireEmployer(ctx context.Context, q querier, userID int64) (*Employer, error) {
	e, err := defaultEmployer(ctx, q, userID)
	if err == ErrNotFound {
		return nil, Invalid("employer.missing", "set up your employer first")
	}
	return e, err
}

// SaveEmployer creates or updates the default employer. Historical claims
// keep their own copy of these details in the snapshot.
func (a *App) SaveEmployer(ctx context.Context, u *User, in EmployerInput) (*Employer, error) {
	name, addr := strings.TrimSpace(in.Name), strings.TrimSpace(in.Address)
	if name == "" {
		return nil, Invalid("employer.name_required", "employer name is required")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var id int64
		var version int
		err := tx.QueryRowContext(ctx, `SELECT id, version FROM employers WHERE user_id = ? AND is_default = 1 ORDER BY id LIMIT 1`, u.ID).Scan(&id, &version)
		if err == sql.ErrNoRows {
			res, err := tx.ExecContext(ctx, `INSERT INTO employers(user_id, name, address, created_at) VALUES (?, ?, ?, ?)`, u.ID, name, addr, a.stamp())
			if err != nil {
				return err
			}
			id, _ = res.LastInsertId()
			return a.audit(ctx, tx, u.ID, u.ID, "employer", id, "create", "", in)
		}
		if err != nil {
			return err
		}
		if in.Version != 0 && in.Version != version {
			return ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE employers SET name = ?, address = ?, version = version + 1 WHERE id = ?`, name, addr, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "employer", id, "update", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.DefaultEmployer(ctx, u.ID)
}
