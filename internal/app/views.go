package app

import (
	"context"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
)

// TripDetail is a trip with its server-side calculation (C03).
type TripDetail struct {
	Trip     *Trip            `json:"trip"`
	Result   *calc.TripResult `json:"result"`
	Expenses []*Expense       `json:"expenses"`
	Issues   []calc.Issue     `json:"issues"`
}

// TripListItem summarizes a trip for lists.
type TripListItem struct {
	*Trip
	Total    calc.Cents `json:"total"`
	Blockers int        `json:"blockers"`
	Warnings int        `json:"warnings"`
}

// TripDetail loads one trip with its calculation.
func (a *App) TripDetail(ctx context.Context, u *User, id int64) (*TripDetail, error) {
	ud, err := a.loadUserData(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	res := calc.Compute(ud.input, a.Rates())
	for _, t := range ud.trips {
		if t.ID == id {
			issues := res.IssuesFor(map[int64]bool{id: true}, nil)
			return &TripDetail{Trip: t, Result: res.Trips[id], Expenses: nonNil(ud.expenses[id]), Issues: nonNil(issues)}, nil
		}
	}
	return nil, ErrNotFound
}

// TripList lists trips with totals and issue counts.
func (a *App) TripList(ctx context.Context, u *User) ([]TripListItem, error) {
	ud, err := a.loadUserData(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	res := calc.Compute(ud.input, a.Rates())
	out := []TripListItem{}
	for _, t := range ud.trips {
		item := TripListItem{Trip: t}
		if r := res.Trips[t.ID]; r != nil {
			item.Total = r.Totals.Total
		}
		for _, is := range res.IssuesFor(map[int64]bool{t.ID: true}, nil) {
			switch is.Severity {
			case calc.Blocker:
				item.Blockers++
			case calc.Warning:
				item.Warnings++
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// OutlayListItem is a standalone outlay with its result.
type OutlayListItem struct {
	*Expense
	Result calc.ExpenseResult `json:"result"`
	Issues []calc.Issue       `json:"issues"`
}

// OutlayList lists standalone outlays with their payable amounts.
func (a *App) OutlayList(ctx context.Context, u *User) ([]OutlayListItem, error) {
	ud, err := a.loadUserData(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	res := calc.Compute(calc.Input{Outlays: ud.input.Outlays}, a.Rates())
	out := []OutlayListItem{}
	for i, e := range ud.outlays {
		issues := res.IssuesFor(nil, map[int64]bool{e.ID: true})
		out = append(out, OutlayListItem{Expense: e, Result: res.Outlays[i], Issues: nonNil(issues)})
	}
	return out, nil
}

// Dashboard summarizes open work for the start page.
type Dashboard struct {
	HasEmployer      bool       `json:"hasEmployer"`
	InboxReceipts    int        `json:"inboxReceipts"`
	UnclaimedTrips   int        `json:"unclaimedTrips"`
	UnclaimedOutlays int        `json:"unclaimedOutlays"`
	UnclaimedTotal   calc.Cents `json:"unclaimedTotal"`
	DraftClaims      int        `json:"draftClaims"`
	OpenPayout       int64      `json:"openPayout" doc:"submitted, not yet paid"`
	Claims           []*Claim   `json:"claims"`
}

// Dashboard computes the start page summary.
func (a *App) Dashboard(ctx context.Context, u *User) (*Dashboard, error) {
	d := &Dashboard{}
	_, err := a.DefaultEmployer(ctx, u.ID)
	d.HasEmployer = err == nil
	if err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM receipts WHERE user_id = ? AND status = 'inbox'`, u.ID).Scan(&d.InboxReceipts); err != nil {
		return nil, err
	}
	ud, err := a.loadUserData(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	res := calc.Compute(ud.input, a.Rates())
	for _, t := range ud.trips {
		if t.ClaimID == nil {
			d.UnclaimedTrips++
			if r := res.Trips[t.ID]; r != nil {
				d.UnclaimedTotal += r.Totals.Total
			}
		}
	}
	for i, e := range ud.outlays {
		if e.ClaimID == nil {
			d.UnclaimedOutlays++
			d.UnclaimedTotal += res.Outlays[i].Reimbursable
		}
	}
	claims, err := a.ListClaims(ctx, u)
	if err != nil {
		return nil, err
	}
	for _, c := range claims {
		switch c.Status {
		case "draft":
			d.DraftClaims++
		case "submitted":
			d.OpenPayout += c.Payout
		}
	}
	if len(claims) > 5 {
		claims = claims[:5]
	}
	d.Claims = nonNil(claims)
	return d, nil
}
