package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/export"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/money"
)

// ClaimInput creates a claim for an inclusive period.
type ClaimInput struct {
	Title       string `json:"title,omitempty" maxLength:"200"`
	PeriodStart string `json:"periodStart" format:"date"`
	PeriodEnd   string `json:"periodEnd" format:"date"`
}

// ClaimVersion is a submitted, immutable version.
type ClaimVersion struct {
	Version      int    `json:"version"`
	SubmittedAt  string `json:"submittedAt"`
	Payout       int64  `json:"payout"`
	PDFSHA256    string `json:"pdfSha256"`
	ZipSHA256    string `json:"zipSha256"`
	SupersededAt string `json:"supersededAt,omitempty"`
	UnlockReason string `json:"unlockReason,omitempty"`
}

// Claim is an Abrechnung.
type Claim struct {
	ID             int64          `json:"id"`
	EmployerID     int64          `json:"employerId"`
	Title          string         `json:"title"`
	PeriodStart    string         `json:"periodStart"`
	PeriodEnd      string         `json:"periodEnd"`
	Status         string         `json:"status" enum:"draft,submitted,paid"`
	Number         string         `json:"number"`
	CurrentVersion int            `json:"currentVersion"`
	PaidAt         string         `json:"paidAt,omitempty"`
	PaidNote       string         `json:"paidNote,omitempty"`
	Version        int            `json:"version"`
	TripIDs        []int64        `json:"tripIds"`
	OutlayIDs      []int64        `json:"outlayIds"`
	AdvanceIDs     []int64        `json:"advanceIds"`
	Versions       []ClaimVersion `json:"versions"`
	Payout         int64          `json:"payout" doc:"payout of the current submitted version, or 0 for drafts"`
}

// PreviewTrip is a trip in a claim preview.
type PreviewTrip struct {
	Trip     *Trip            `json:"trip"`
	Result   *calc.TripResult `json:"result"`
	Expenses []*Expense       `json:"expenses"`
}

// PreviewOutlay is an outlay in a claim preview.
type PreviewOutlay struct {
	Expense *Expense           `json:"expense"`
	Result  calc.ExpenseResult `json:"result"`
}

// Suggestions are unassigned items that fit the claim period (P13).
type Suggestions struct {
	Trips    []*Trip    `json:"trips"`
	Outlays  []*Expense `json:"outlays"`
	Advances []*Advance `json:"advances"`
}

// Preview shows blockers, warnings, totals and the draft content (P14).
type Preview struct {
	Claim       *Claim           `json:"claim"`
	Trips       []PreviewTrip    `json:"trips"`
	Outlays     []PreviewOutlay  `json:"outlays"`
	Advances    []*Advance       `json:"advances"`
	Totals      calc.ClaimTotals `json:"totals"`
	Blockers    []calc.Issue     `json:"blockers"`
	Warnings    []calc.Issue     `json:"warnings"`
	Notices     []calc.Issue     `json:"notices"`
	Revision    string           `json:"revision" doc:"identifies the current inputs; submit must name it"`
	Suggestions Suggestions      `json:"suggestions"`
}

// CreateClaim creates a draft and selects the suggested items.
func (a *App) CreateClaim(ctx context.Context, u *User, in ClaimInput) (*Claim, error) {
	if err := validDate(in.PeriodStart); err != nil {
		return nil, err
	}
	if err := validDate(in.PeriodEnd); err != nil {
		return nil, err
	}
	if in.PeriodEnd < in.PeriodStart {
		return nil, Invalid("claim.period_invalid", "period end is before start")
	}
	emp, err := requireEmployer(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	var id int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		now := a.stamp()
		res, err := tx.ExecContext(ctx, `INSERT INTO claims(user_id, employer_id, title, period_start, period_end, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			u.ID, emp.ID, strings.TrimSpace(in.Title), in.PeriodStart, in.PeriodEnd, now, now)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		sug, err := a.suggestions(ctx, tx, u.ID, emp.ID, in.PeriodStart, in.PeriodEnd)
		if err != nil {
			return err
		}
		for _, t := range sug.Trips {
			if _, err := tx.ExecContext(ctx, `UPDATE trips SET claim_id = ? WHERE id = ? AND claim_id IS NULL`, id, t.ID); err != nil {
				return err
			}
		}
		for _, e := range sug.Outlays {
			if _, err := tx.ExecContext(ctx, `UPDATE expenses SET claim_id = ? WHERE id = ? AND claim_id IS NULL AND trip_id IS NULL`, id, e.ID); err != nil {
				return err
			}
		}
		for _, v := range sug.Advances {
			if _, err := tx.ExecContext(ctx, `UPDATE advances SET claim_id = ? WHERE id = ? AND claim_id IS NULL`, id, v.ID); err != nil {
				return err
			}
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "create", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetClaim(ctx, u, id)
}

// suggestions lists unclaimed trips by local end date, unclaimed outlays by
// expense date and advances dated no later than the period end.
func (a *App) suggestions(ctx context.Context, q querier, userID, employerID int64, start, end string) (Suggestions, error) {
	var s Suggestions
	trips, err := a.loadTrips(ctx, q, userID, "t.claim_id IS NULL AND t.employer_id = ?", employerID)
	if err != nil {
		return s, err
	}
	for _, t := range trips {
		if t.EndDate >= start && t.EndDate <= end {
			s.Trips = append(s.Trips, t)
		}
	}
	if s.Outlays, err = a.loadExpenses(ctx, q, userID, "e.trip_id IS NULL AND e.claim_id IS NULL AND e.employer_id = ? AND e.date BETWEEN ? AND ?", employerID, start, end); err != nil {
		return s, err
	}
	if s.Advances, err = a.loadAdvances(ctx, q, userID, "a.claim_id IS NULL AND a.employer_id = ? AND a.date <= ?", employerID, end); err != nil {
		return s, err
	}
	s.Trips, s.Outlays = nonNil(s.Trips), nonNil(s.Outlays)
	return s, nil
}

// ListClaims lists the user's claims.
func (a *App) ListClaims(ctx context.Context, u *User) ([]*Claim, error) {
	return a.loadClaims(ctx, a.DB, u.ID, "1 = 1")
}

// GetClaim loads one owned claim.
func (a *App) GetClaim(ctx context.Context, u *User, id int64) (*Claim, error) {
	list, err := a.loadClaims(ctx, a.DB, u.ID, "id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (a *App) loadClaims(ctx context.Context, q querier, userID int64, where string, args ...any) ([]*Claim, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, employer_id, title, period_start, period_end, status, coalesce(number, ''), current_version, coalesce(paid_at, ''), paid_note, version FROM claims WHERE user_id = ? AND `+where+` ORDER BY period_end DESC, id DESC`, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	var out []*Claim
	for rows.Next() {
		c := &Claim{}
		if err := rows.Scan(&c.ID, &c.EmployerID, &c.Title, &c.PeriodStart, &c.PeriodEnd, &c.Status, &c.Number, &c.CurrentVersion, &c.PaidAt, &c.PaidNote, &c.Version); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	rows.Close()
	for _, c := range out {
		if c.TripIDs, err = ids(ctx, q, `SELECT id FROM trips WHERE claim_id = ? ORDER BY start_at`, c.ID); err != nil {
			return nil, err
		}
		if c.OutlayIDs, err = ids(ctx, q, `SELECT id FROM expenses WHERE claim_id = ? ORDER BY date`, c.ID); err != nil {
			return nil, err
		}
		if c.AdvanceIDs, err = ids(ctx, q, `SELECT id FROM advances WHERE claim_id = ? ORDER BY date`, c.ID); err != nil {
			return nil, err
		}
		vrows, err := q.QueryContext(ctx, `SELECT version, submitted_at, payout, pdf_sha256, zip_sha256, coalesce(superseded_at, ''), unlock_reason FROM claim_versions WHERE claim_id = ? ORDER BY version DESC`, c.ID)
		if err != nil {
			return nil, err
		}
		c.Versions = []ClaimVersion{}
		for vrows.Next() {
			var v ClaimVersion
			if err := vrows.Scan(&v.Version, &v.SubmittedAt, &v.Payout, &v.PDFSHA256, &v.ZipSHA256, &v.SupersededAt, &v.UnlockReason); err != nil {
				vrows.Close()
				return nil, err
			}
			c.Versions = append(c.Versions, v)
		}
		vrows.Close()
		if c.Status != "draft" && len(c.Versions) > 0 {
			c.Payout = c.Versions[0].Payout
		}
	}
	return out, nil
}

func ids(ctx context.Context, q querier, query string, args ...any) ([]int64, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// draftClaim loads a claim and requires it to be an editable draft.
func (a *App) draftClaim(ctx context.Context, q querier, userID, id int64) (*Claim, error) {
	list, err := a.loadClaims(ctx, q, userID, "id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	if list[0].Status != "draft" {
		return nil, ErrLocked
	}
	return list[0], nil
}

// ClaimItem identifies an item to add to or remove from a draft.
type ClaimItem struct {
	Kind string `json:"kind" enum:"trip,outlay,advance"`
	ID   int64  `json:"id"`
}

// SetClaimItem adds (include=true) or removes an item. Items belong to at
// most one claim; ownership and employer are verified (P13, integrity).
func (a *App) SetClaimItem(ctx context.Context, u *User, claimID int64, item ClaimItem, include bool) (*Claim, error) {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		c, err := a.draftClaim(ctx, tx, u.ID, claimID)
		if err != nil {
			return err
		}
		var table, extra string
		switch item.Kind {
		case "trip":
			table = "trips"
		case "outlay":
			table, extra = "expenses", " AND trip_id IS NULL"
		case "advance":
			table = "advances"
			var date string
			if err := tx.QueryRowContext(ctx, `SELECT date FROM advances WHERE id = ? AND user_id = ?`, item.ID, u.ID).Scan(&date); err == nil && include && date > c.PeriodEnd {
				return Invalid("advance.after_period", "the advance is dated after the claim period")
			}
		default:
			return Invalid("claim.item_kind", "unknown item kind")
		}
		var owner, employer int64
		var current sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT user_id, employer_id, claim_id FROM `+table+` WHERE id = ?`+extra, item.ID).Scan(&owner, &employer, &current); err != nil || owner != u.ID {
			return ErrNotFound
		}
		if employer != c.EmployerID {
			return Invalid("claim.employer_mismatch", "the item belongs to another employer")
		}
		if include {
			if current.Valid && current.Int64 != claimID {
				return Invalid("claim.item_taken", "the item already belongs to another claim")
			}
			_, err = tx.ExecContext(ctx, `UPDATE `+table+` SET claim_id = ? WHERE id = ?`, claimID, item.ID)
		} else {
			if !current.Valid || current.Int64 != claimID {
				return ErrNotFound
			}
			_, err = tx.ExecContext(ctx, `UPDATE `+table+` SET claim_id = NULL WHERE id = ?`, item.ID)
		}
		if err != nil {
			return err
		}
		action := "remove_item"
		if include {
			action = "add_item"
		}
		_, err = tx.ExecContext(ctx, `UPDATE claims SET version = version + 1, updated_at = ? WHERE id = ?`, a.stamp(), claimID)
		if err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", claimID, action, "", item)
	})
	if err != nil {
		return nil, err
	}
	return a.GetClaim(ctx, u, claimID)
}

// UpdateClaim changes a draft's title or period.
func (a *App) UpdateClaim(ctx context.Context, u *User, id int64, in ClaimInput) (*Claim, error) {
	if err := validDate(in.PeriodStart); err != nil {
		return nil, err
	}
	if err := validDate(in.PeriodEnd); err != nil {
		return nil, err
	}
	if in.PeriodEnd < in.PeriodStart {
		return nil, Invalid("claim.period_invalid", "period end is before start")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := a.draftClaim(ctx, tx, u.ID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE claims SET title = ?, period_start = ?, period_end = ?, version = version + 1, updated_at = ? WHERE id = ?`, strings.TrimSpace(in.Title), in.PeriodStart, in.PeriodEnd, a.stamp(), id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "update", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetClaim(ctx, u, id)
}

// DeleteClaim deletes a draft that was never submitted.
func (a *App) DeleteClaim(ctx context.Context, u *User, id int64) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		c, err := a.draftClaim(ctx, tx, u.ID, id)
		if err != nil {
			return err
		}
		if c.CurrentVersion > 0 {
			return Invalid("claim.submitted_once", "a claim that was submitted cannot be deleted")
		}
		for _, t := range []string{"trips", "expenses", "advances"} {
			if _, err := tx.ExecContext(ctx, `UPDATE `+t+` SET claim_id = NULL WHERE claim_id = ?`, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM claims WHERE id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "delete", "", nil)
	})
}

// PreviewClaim computes the claim with all of the user's trips so the daily
// cap and frozen amounts apply (C07).
func (a *App) PreviewClaim(ctx context.Context, u *User, id int64) (*Preview, error) {
	p, _, err := a.preview(ctx, a.DB, u.ID, id)
	return p, err
}

func (a *App) preview(ctx context.Context, q querier, userID, id int64) (*Preview, *userData, error) {
	list, err := a.loadClaims(ctx, q, userID, "id = ?", id)
	if err != nil {
		return nil, nil, err
	}
	if len(list) == 0 {
		return nil, nil, ErrNotFound
	}
	c := list[0]
	ud, err := a.loadUserData(ctx, q, userID)
	if err != nil {
		return nil, nil, err
	}
	res := calc.Compute(ud.input, a.Rates())
	p := &Preview{Claim: c, Trips: []PreviewTrip{}, Outlays: []PreviewOutlay{}, Blockers: []calc.Issue{}, Warnings: []calc.Issue{}, Notices: []calc.Issue{}}
	tripSet, expSet := map[int64]bool{}, map[int64]bool{}
	for _, t := range ud.trips {
		if slices.Contains(c.TripIDs, t.ID) {
			tripSet[t.ID] = true
			p.Trips = append(p.Trips, PreviewTrip{Trip: t, Result: res.Trips[t.ID], Expenses: nonNil(ud.expenses[t.ID])})
		}
	}
	outlayResults := map[int64]calc.ExpenseResult{}
	for _, r := range res.Outlays {
		outlayResults[r.ExpenseID] = r
	}
	for _, e := range ud.outlays {
		if slices.Contains(c.OutlayIDs, e.ID) {
			expSet[e.ID] = true
			p.Outlays = append(p.Outlays, PreviewOutlay{Expense: e, Result: outlayResults[e.ID]})
		}
	}
	if p.Advances, err = a.loadAdvances(ctx, q, userID, "a.claim_id = ?", id); err != nil {
		return nil, nil, err
	}
	var adv []calc.Cents
	for _, v := range p.Advances {
		adv = append(adv, calc.Cents(v.Amount))
	}
	p.Totals = res.Totals(c.TripIDs, c.OutlayIDs, adv)
	issues := res.IssuesFor(tripSet, expSet)
	if len(c.TripIDs) == 0 && len(c.OutlayIDs) == 0 {
		issues = append(issues, calc.Issue{Severity: calc.Warning, RuleID: calc.RuleTotals, Code: "claim.empty"})
	}
	for _, is := range issues {
		switch is.Severity {
		case calc.Blocker:
			p.Blockers = append(p.Blockers, is)
		case calc.Warning:
			p.Warnings = append(p.Warnings, is)
		default:
			p.Notices = append(p.Notices, is)
		}
	}
	if c.Status == "draft" {
		if p.Suggestions, err = a.suggestions(ctx, q, userID, c.EmployerID, c.PeriodStart, c.PeriodEnd); err != nil {
			return nil, nil, err
		}
	}
	p.Suggestions.Trips, p.Suggestions.Outlays, p.Suggestions.Advances = nonNil(p.Suggestions.Trips), nonNil(p.Suggestions.Outlays), nonNil(p.Suggestions.Advances)
	rev, err := json.Marshal(struct {
		C       *Claim
		T       []PreviewTrip
		O       []PreviewOutlay
		A       []*Advance
		Tot     calc.ClaimTotals
		B, W, N []calc.Issue
	}{c, p.Trips, p.Outlays, p.Advances, p.Totals, p.Blockers, p.Warnings, p.Notices})
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(rev)
	p.Revision = hex.EncodeToString(sum[:12])
	return p, ud, nil
}

// SubmitInput confirms the reviewed revision and acknowledges warnings.
type SubmitInput struct {
	Revision            string `json:"revision"`
	AcknowledgeWarnings bool   `json:"acknowledgeWarnings"`
}

// SubmitClaim freezes a version and publishes its PDF and ZIP (P14–P16).
//
// The whole submission runs in one write transaction: the daily-allocation
// check, number allocation, artifact publication and status change commit
// together or not at all. Files are staged, verified and moved into storage
// before the commit; on failure they are removed, so no partial export is
// ever published and the claim stays an editable draft (A23).
func (a *App) SubmitClaim(ctx context.Context, u *User, id int64, in SubmitInput) (*Claim, error) {
	var stored []string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		p, ud, err := a.preview(ctx, tx, u.ID, id)
		if err != nil {
			return err
		}
		c := p.Claim
		if c.Status != "draft" {
			return &Error{Status: 409, Code: "claim.not_draft", Message: "the claim is already submitted"}
		}
		if in.Revision != p.Revision {
			return &Error{Status: 409, Code: "claim.revision_changed", Message: "the claim changed since the preview; review it again"}
		}
		if len(p.Blockers) > 0 {
			return &Error{Status: 422, Code: "claim.blocked", Message: "resolve the blockers before submitting"}
		}
		if len(p.Warnings) > 0 && !in.AcknowledgeWarnings {
			return &Error{Status: 422, Code: "claim.warnings_unacknowledged", Message: "acknowledge the warnings before submitting"}
		}
		user, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, u.ID))
		if err != nil {
			return err
		}
		var emp export.Party
		if err := tx.QueryRowContext(ctx, `SELECT name, address FROM employers WHERE id = ?`, c.EmployerID).Scan(&emp.Name, &emp.Address); err != nil {
			return err
		}

		number := c.Number
		if number == "" {
			year := a.now().Year()
			var seq int
			if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(number_seq), 0) + 1 FROM claims WHERE user_id = ? AND employer_id = ? AND number_year = ?`, u.ID, c.EmployerID, year).Scan(&seq); err != nil {
				return err
			}
			number = fmt.Sprintf("A-%d-%03d", year, seq)
			if _, err := tx.ExecContext(ctx, `UPDATE claims SET number = ?, number_year = ?, number_seq = ? WHERE id = ?`, number, year, seq, id); err != nil {
				return err
			}
		}
		version := c.CurrentVersion + 1
		previousReason := ""
		if len(c.Versions) > 0 {
			previousReason = c.Versions[0].UnlockReason
		}
		snap, err := a.buildSnapshot(ctx, tx, p, ud, user, emp, number, version, previousReason, in.AcknowledgeWarnings)
		if err != nil {
			return err
		}
		staging := fmt.Sprintf("claim-%d-v%d", id, version)
		dir, err := a.Files.Staging(staging)
		if err != nil {
			return err
		}
		defer a.Files.ClearStaging(staging)
		countries, err := countryNames(ctx, tx)
		if err != nil {
			return err
		}
		art, err := export.Build(ctx, a.Typst, snap, countries, a.Files.Read, dir)
		if err != nil {
			a.Log.Error("export failed", "claim", id, "err", err)
			return &Error{Status: 500, Code: "export.failed", Message: "the export could not be generated; the claim is still a draft"}
		}
		pdfKey, err := a.Files.PutFile("exports", ".pdf", art.PDFPath, art.PDFSHA)
		if err != nil {
			return err
		}
		stored = append(stored, pdfKey)
		zipKey, err := a.Files.PutFile("exports", ".zip", art.ZipPath, art.ZipSHA)
		if err != nil {
			return err
		}
		stored = append(stored, zipKey)
		snapJSON, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		now := a.stamp()
		if _, err := tx.ExecContext(ctx, `UPDATE claim_versions SET superseded_at = ? WHERE claim_id = ? AND superseded_at IS NULL`, now, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_versions(claim_id, version, snapshot_json, payout, pdf_key, pdf_sha256, zip_key, zip_sha256, submitted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, version, string(snapJSON), p.Totals.Payout, pdfKey, art.PDFSHA, zipKey, art.ZipSHA, now); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE claims SET status = 'submitted', current_version = ?, version = version + 1, updated_at = ? WHERE id = ? AND status = 'draft' AND current_version = ?`, version, now, id, c.CurrentVersion)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrConflict
		}
		for _, t := range p.Trips {
			if _, err := tx.ExecContext(ctx, `DELETE FROM frozen_days WHERE trip_id = ?`, t.Trip.ID); err != nil {
				return err
			}
			for _, d := range t.Result.Days {
				if _, err := tx.ExecContext(ctx, `INSERT INTO frozen_days(trip_id, date, claim_id, cents) VALUES (?, ?, ?, ?)`, t.Trip.ID, d.Date.String(), id, d.Payable); err != nil {
					return err
				}
			}
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "submit", "", map[string]any{"number": number, "version": version, "payout": p.Totals.Payout, "pdfSha256": art.PDFSHA, "zipSha256": art.ZipSHA, "warningsAcknowledged": len(p.Warnings)})
	})
	if err != nil {
		for _, k := range stored {
			a.Files.Remove(k)
		}
		return nil, err
	}
	return a.GetClaim(ctx, u, id)
}

func countryNames(ctx context.Context, q querier) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT iso, bmf_name FROM countries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var iso, name string
		if err := rows.Scan(&iso, &name); err != nil {
			return nil, err
		}
		out[iso] = name
	}
	return out, rows.Err()
}

func (a *App) buildSnapshot(ctx context.Context, q querier, p *Preview, ud *userData, u *User, emp export.Party, number string, version int, previousReason string, ack bool) (*export.Snapshot, error) {
	now := a.stamp()
	s := &export.Snapshot{
		SchemaVersion: export.SchemaVersion, App: "spesen", Locale: u.Locale,
		Claim:    export.ClaimInfo{ID: p.Claim.ID, Number: number, Version: version, Title: p.Claim.Title, PeriodStart: p.Claim.PeriodStart, PeriodEnd: p.Claim.PeriodEnd, SubmittedAt: now, Previous: previousReason},
		User:     export.Person{Name: u.Name, Email: u.Email, EmployeeNumber: u.EmployeeNumber, Address: u.Address, IBAN: u.IBAN},
		Employer: emp, Totals: p.Totals, Acknowledged: ack, GeneratedAt: now,
		Trips: []export.Trip{}, Outlays: []export.Expense{}, Advances: []export.Advance{}, Receipts: []export.Receipt{}, Issues: []calc.Issue{},
	}
	s.Issues = append(append(s.Issues, p.Warnings...), p.Notices...)
	receipts := map[int64]bool{}
	var receiptOrder []int64
	rates := map[string]calc.RateRef{}
	addRate := func(r *calc.RateRef) {
		if r != nil {
			rates[fmt.Sprintf("%s/%d/%s/%s/%d", r.Table, r.Year, r.Location.Country, r.Location.City, r.Revision)] = *r
		}
	}
	expense := func(e *Expense, r calc.ExpenseResult) export.Expense {
		exp, _ := money.Exponent(e.Currency)
		x := export.Expense{ID: e.ID, Date: e.Date, CostType: e.CostType, Payer: e.Payer, Supplier: e.Supplier, Description: e.Description, Currency: e.Currency,
			GrossMinor: e.GrossMinor, Exponent: exp, FXRate: e.FXRate, FXDate: e.FXDate, EURDebit: e.EURDebit, VATUnknown: e.VATUnknown, VAT: nonNil(e.VAT),
			Nights: nonNil(e.Nights), Hospitality: e.Hospitality, Receipts: []string{}, Result: r}
		if e.TripID != nil {
			x.TripID = *e.TripID
		}
		for _, rc := range e.Receipts {
			x.Receipts = append(x.Receipts, rc.Number)
			if !receipts[rc.ID] {
				receipts[rc.ID] = true
				receiptOrder = append(receiptOrder, rc.ID)
			}
		}
		return x
	}
	for _, pt := range p.Trips {
		t := pt.Trip
		et := export.Trip{ID: t.ID, Purpose: t.Purpose, Destination: t.Destination, Start: t.Start, StartTZ: t.StartTZ, End: t.End, EndTZ: t.EndTZ, Project: t.Project, Notes: t.Notes,
			Stops: []export.Stop{}, Days: []export.DayInput{}, Expenses: []export.Expense{}, Result: *pt.Result}
		for _, st := range t.Stops {
			et.Stops = append(et.Stops, export.Stop{Arrival: st.Arrival, TZ: st.TZ, Location: calc.Location{Country: st.Country, City: st.City}, Business: st.Business})
		}
		ct, err := toCalcTrip(t)
		if err != nil {
			return nil, err
		}
		for _, d := range t.Days {
			et.Days = append(et.Days, export.DayInput{Date: d.Date, Input: ct.Days[calc.MustDate(d.Date)], Origin: d.BreakfastOrigin})
		}
		results := map[int64]calc.ExpenseResult{}
		for _, r := range pt.Result.Expenses {
			results[r.ExpenseID] = r
		}
		for _, e := range pt.Expenses {
			et.Expenses = append(et.Expenses, expense(e, results[e.ID]))
		}
		for _, d := range pt.Result.Days {
			addRate(d.Rate)
		}
		for _, n := range pt.Result.Nights {
			addRate(n.Rate)
		}
		for _, j := range pt.Result.Journeys {
			addRate(j.Rate)
		}
		s.Trips = append(s.Trips, et)
	}
	for _, po := range p.Outlays {
		s.Outlays = append(s.Outlays, expense(po.Expense, po.Result))
	}
	for _, v := range p.Advances {
		s.Advances = append(s.Advances, export.Advance{ID: v.ID, Date: v.Date, Amount: calc.Cents(v.Amount), Note: v.Note})
	}
	for _, r := range rates {
		s.Rates = append(s.Rates, r)
	}
	slices.SortFunc(s.Rates, func(x, y calc.RateRef) int {
		return strings.Compare(x.Source+x.Location.Country, y.Source+y.Location.Country)
	})
	slices.Sort(receiptOrder)
	for _, rid := range receiptOrder {
		var r export.Receipt
		var self sql.NullString
		if err := q.QueryRowContext(ctx, `SELECT id, coalesce(number, ''), kind, filename, mime, pages, sha256, storage_key, self_json FROM receipts WHERE id = ?`, rid).
			Scan(&r.ID, &r.Number, &r.Kind, &r.Filename, &r.Mime, &r.Pages, &r.SHA256, &r.StorageKey, &self); err != nil {
			return nil, err
		}
		if self.Valid {
			var si SelfIssued
			_ = json.Unmarshal([]byte(self.String), &si)
			r.SelfIssued = si
		}
		if r.Kind == "file" {
			_, ext, _ := detectByMime(r.Mime)
			r.ZipPath = "evidence/" + r.Number + ext
		}
		s.Receipts = append(s.Receipts, r)
	}
	return s, nil
}

func detectByMime(m string) (string, string, bool) {
	switch m {
	case "image/jpeg":
		return m, ".jpg", true
	case "image/png":
		return m, ".png", true
	case "application/pdf":
		return m, ".pdf", true
	}
	return m, "", false
}

// UnlockClaim returns a submitted claim to draft for correction (P15). The
// submitted versions and their exports stay unchanged.
func (a *App) UnlockClaim(ctx context.Context, u *User, id int64, reason string) (*Claim, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, Invalid("claim.reason_required", "a reason is required")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var status string
		var current int
		if err := tx.QueryRowContext(ctx, `SELECT status, current_version FROM claims WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&status, &current); err != nil {
			return notFound(err)
		}
		if status == "paid" {
			return Invalid("claim.paid", "undo the payment before unlocking")
		}
		if status != "submitted" {
			return Invalid("claim.not_submitted", "only submitted claims can be unlocked")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE claims SET status = 'draft', version = version + 1, updated_at = ? WHERE id = ?`, a.stamp(), id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE claim_versions SET unlock_reason = ? WHERE claim_id = ? AND version = ?`, reason, id, current); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM frozen_days WHERE claim_id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "unlock", reason, map[string]any{"version": current})
	})
	if err != nil {
		return nil, err
	}
	return a.GetClaim(ctx, u, id)
}

// PaymentInput records a payout made outside the app.
type PaymentInput struct {
	Date string `json:"date" format:"date"`
	Note string `json:"note,omitempty" maxLength:"500"`
}

// MarkPaid records payment metadata; existing exports are not rewritten.
func (a *App) MarkPaid(ctx context.Context, u *User, id int64, in PaymentInput) (*Claim, error) {
	if err := validDate(in.Date); err != nil {
		return nil, err
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE claims SET status = 'paid', paid_at = ?, paid_note = ?, version = version + 1, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'submitted'`, in.Date, in.Note, a.stamp(), id, u.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return Invalid("claim.not_submitted", "only submitted claims can be marked as paid")
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "paid", in.Note, in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetClaim(ctx, u, id)
}

// UndoPaid reverts Paid to Submitted with a reason.
func (a *App) UndoPaid(ctx context.Context, u *User, id int64, reason string) (*Claim, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, Invalid("claim.reason_required", "a reason is required")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE claims SET status = 'submitted', paid_at = NULL, paid_note = '', version = version + 1, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'paid'`, a.stamp(), id, u.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return Invalid("claim.not_paid", "the claim is not marked as paid")
		}
		return a.audit(ctx, tx, u.ID, u.ID, "claim", id, "unpaid", reason, nil)
	})
	if err != nil {
		return nil, err
	}
	return a.GetClaim(ctx, u, id)
}

// VersionFile returns the stored PDF or ZIP bytes of a submitted version.
func (a *App) VersionFile(ctx context.Context, u *User, id int64, version int, kind string) ([]byte, string, error) {
	var pdfKey, zipKey, number string
	err := a.DB.QueryRowContext(ctx, `SELECT v.pdf_key, v.zip_key, coalesce(c.number, '') FROM claim_versions v JOIN claims c ON c.id = v.claim_id WHERE c.id = ? AND c.user_id = ? AND v.version = ?`, id, u.ID, version).Scan(&pdfKey, &zipKey, &number)
	if err != nil {
		return nil, "", notFound(err)
	}
	key, name := pdfKey, fmt.Sprintf("%s-v%d.pdf", number, version)
	if kind == "zip" {
		key, name = zipKey, fmt.Sprintf("%s-v%d.zip", number, version)
	}
	b, err := a.Files.Read(key)
	return b, name, err
}

// recoverSubmissions clears staging left by an interrupted submission and
// removes export files that no committed version references (A23).
func (a *App) recoverSubmissions(ctx context.Context) error {
	if err := a.Files.ClearStaging(""); err != nil {
		return err
	}
	if _, err := a.Files.Staging(""); err != nil {
		return err
	}
	referenced := map[string]bool{}
	rows, err := a.DB.QueryContext(ctx, `SELECT pdf_key, zip_key FROM claim_versions`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p, z string
		if err := rows.Scan(&p, &z); err != nil {
			rows.Close()
			return err
		}
		referenced[p], referenced[z] = true, true
	}
	rows.Close()
	return a.Files.Sweep("exports", referenced, a.Log)
}
