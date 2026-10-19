package app

import (
	"context"
	"database/sql"
	"strings"
)

// AdvanceInput records an amount already paid by the employer.
type AdvanceInput struct {
	Date   string `json:"date" format:"date"`
	Amount int64  `json:"amount" minimum:"1" doc:"EUR cents"`
	Note   string `json:"note,omitempty" maxLength:"500"`
}

// Advance is a stored advance.
type Advance struct {
	AdvanceInput
	ID      int64  `json:"id"`
	ClaimID *int64 `json:"claimId"`
	Locked  bool   `json:"locked"`
}

// CreateAdvance records an advance for the default employer.
func (a *App) CreateAdvance(ctx context.Context, u *User, in AdvanceInput) (*Advance, error) {
	if err := validDate(in.Date); err != nil {
		return nil, err
	}
	if in.Amount <= 0 {
		return nil, Invalid("advance.amount_invalid", "amount must be positive")
	}
	emp, err := requireEmployer(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	var id int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO advances(user_id, employer_id, date, amount, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`, u.ID, emp.ID, in.Date, in.Amount, strings.TrimSpace(in.Note), a.stamp())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		return a.audit(ctx, tx, u.ID, u.ID, "advance", id, "create", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.getAdvance(ctx, a.DB, u.ID, id)
}

// DeleteAdvance removes an unassigned advance.
func (a *App) DeleteAdvance(ctx context.Context, u *User, id int64) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		adv, err := a.getAdvance(ctx, tx, u.ID, id)
		if err != nil {
			return err
		}
		if adv.ClaimID != nil {
			return Invalid("advance.in_claim", "remove the advance from its claim first")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM advances WHERE id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "advance", id, "delete", "", adv.AdvanceInput)
	})
}

func (a *App) getAdvance(ctx context.Context, q querier, userID, id int64) (*Advance, error) {
	list, err := a.loadAdvances(ctx, q, userID, "a.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

// ListAdvances lists the user's advances.
func (a *App) ListAdvances(ctx context.Context, u *User) ([]*Advance, error) {
	return a.loadAdvances(ctx, a.DB, u.ID, "1 = 1")
}

func (a *App) loadAdvances(ctx context.Context, q querier, userID int64, where string, args ...any) ([]*Advance, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.id, a.claim_id, coalesce(c.status, ''), a.date, a.amount, a.note FROM advances a LEFT JOIN claims c ON c.id = a.claim_id WHERE a.user_id = ? AND `+where+` ORDER BY a.date DESC`, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Advance{}
	for rows.Next() {
		v := &Advance{}
		var status string
		if err := rows.Scan(&v.ID, &v.ClaimID, &status, &v.Date, &v.Amount, &v.Note); err != nil {
			return nil, err
		}
		v.Locked = status == "submitted" || status == "paid"
		out = append(out, v)
	}
	return out, rows.Err()
}
