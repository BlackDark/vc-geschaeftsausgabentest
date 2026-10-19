package app

import (
	"context"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
)

// userData is everything loaded for one user's calculation.
type userData struct {
	trips    []*Trip
	expenses map[int64][]*Expense // by trip
	outlays  []*Expense
	input    calc.Input
}

// loadUserData loads all trips and outlays and builds the calculation
// input. Locked trips carry their frozen daily amounts (C07).
func (a *App) loadUserData(ctx context.Context, q querier, userID int64) (*userData, error) {
	trips, err := a.loadTrips(ctx, q, userID, "1 = 1")
	if err != nil {
		return nil, err
	}
	all, err := a.loadExpenses(ctx, q, userID, "1 = 1")
	if err != nil {
		return nil, err
	}
	ud := &userData{trips: trips, expenses: map[int64][]*Expense{}}
	for _, e := range all {
		if e.TripID != nil {
			ud.expenses[*e.TripID] = append(ud.expenses[*e.TripID], e)
		} else {
			ud.outlays = append(ud.outlays, e)
		}
	}
	dupes, err := duplicateReceipts(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	frozen, err := frozenDays(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	for _, t := range trips {
		ct, err := toCalcTrip(t)
		if err != nil {
			return nil, err
		}
		for _, e := range ud.expenses[t.ID] {
			ct.Expenses = append(ct.Expenses, toCalcExpense(e, dupes))
		}
		if t.Locked {
			ct.Locked = true
			ct.FrozenMeals = map[calc.Date]calc.Cents{}
			for d, f := range frozen[t.ID] {
				ct.FrozenMeals[d] = f.cents
				ct.FrozenClaim = f.claim
			}
		}
		ud.input.Trips = append(ud.input.Trips, ct)
	}
	for _, e := range ud.outlays {
		ud.input.Outlays = append(ud.input.Outlays, toCalcExpense(e, dupes))
	}
	return ud, nil
}

func toCalcTrip(t *Trip) (calc.Trip, error) {
	start, err := parseLocal(t.Start, t.StartTZ)
	if err != nil {
		return calc.Trip{}, err
	}
	end, err := parseLocal(t.End, t.EndTZ)
	if err != nil {
		return calc.Trip{}, err
	}
	ct := calc.Trip{ID: t.ID, Start: start, End: end, Origin: calc.Domestic, Days: map[calc.Date]calc.DayInput{}, Nights: map[calc.Date]calc.NightKind{}}
	for _, s := range t.Stops {
		at, err := parseLocal(s.Arrival, s.TZ)
		if err != nil {
			return ct, err
		}
		ct.Stops = append(ct.Stops, calc.Stop{Arrival: at, Location: calc.Location{Country: s.Country, City: s.City}, Business: s.Business})
	}
	for _, d := range t.Days {
		in := calc.DayInput{
			Provided:      calc.Meals{Breakfast: d.Breakfast, Lunch: d.Lunch, Dinner: d.Dinner},
			Contributions: calc.Contributions{Breakfast: calc.Cents(d.ContribBreakfast), Lunch: calc.Cents(d.ContribLunch), Dinner: calc.Cents(d.ContribDinner)},
			Excluded:      d.Excluded, ExcludeReason: d.ExcludeReason, OverrideReason: d.OverrideReason,
		}
		if d.OverrideCountry != "" {
			in.LocationOverride = &calc.Location{Country: d.OverrideCountry, City: d.OverrideCity}
		}
		ct.Days[calc.MustDate(d.Date)] = in
	}
	for _, n := range t.Nights {
		ct.Nights[calc.MustDate(n.Date)] = n.Kind
	}
	for i, j := range t.Journeys {
		ct.Journeys = append(ct.Journeys, calc.Journey{ID: int64(i + 1), Date: calc.MustDate(j.Date), Vehicle: j.Vehicle, DistanceMeters: j.DistanceMeters, Description: j.Description})
	}
	return ct, nil
}

func toCalcExpense(e *Expense, dupes map[int64]bool) calc.Expense {
	ce := calc.Expense{
		ID: e.ID, Date: calc.MustDate(e.Date), CostType: e.CostType, Payer: e.Payer, Supplier: e.Supplier, Description: e.Description,
		Currency: e.Currency, GrossMinor: e.GrossMinor, FXRate: e.FXRate, VATUnknown: e.VATUnknown, VAT: e.VAT, Fuel: e.Fuel, Hospitality: e.Hospitality,
	}
	if e.EURDebit != nil {
		v := calc.Cents(*e.EURDebit)
		ce.EURDebit = &v
	}
	if e.FXDate != "" {
		d := calc.MustDate(e.FXDate)
		ce.FXDate = &d
	}
	for _, n := range e.Nights {
		ce.Nights = append(ce.Nights, calc.MustDate(n))
	}
	for _, r := range e.Receipts {
		switch {
		case r.Status == "inbox":
			ce.Unconfirmed++
		case r.Status == "confirmed" && r.Kind == "self_issued":
			ce.SelfIssued = true
			ce.Receipts++
		case r.Status == "confirmed":
			ce.Receipts++
		}
		if dupes[r.ID] {
			ce.DuplicateSHA = true
		}
	}
	return ce
}

// duplicateReceipts returns receipts sharing a SHA-256 with another active
// receipt of the same user (P10: exact duplicates within the owner boundary).
func duplicateReceipts(ctx context.Context, q querier, userID int64) (map[int64]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT r.id FROM receipts r WHERE r.user_id = ? AND r.sha256 <> '' AND r.status <> 'void'
		AND EXISTS (SELECT 1 FROM receipts d WHERE d.user_id = r.user_id AND d.sha256 = r.sha256 AND d.id <> r.id AND d.status <> 'void')`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

type frozenDay struct {
	cents calc.Cents
	claim string
}

func frozenDays(ctx context.Context, q querier, userID int64) (map[int64]map[calc.Date]frozenDay, error) {
	rows, err := q.QueryContext(ctx, `SELECT f.trip_id, f.date, f.cents, coalesce(c.number, '') FROM frozen_days f JOIN trips t ON t.id = f.trip_id JOIN claims c ON c.id = f.claim_id WHERE t.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[calc.Date]frozenDay{}
	for rows.Next() {
		var trip int64
		var date string
		var f frozenDay
		if err := rows.Scan(&trip, &date, &f.cents, &f.claim); err != nil {
			return nil, err
		}
		if out[trip] == nil {
			out[trip] = map[calc.Date]frozenDay{}
		}
		out[trip][calc.MustDate(date)] = f
	}
	return out, rows.Err()
}

// Overview is the calculation view of all the user's trips and outlays.
type Overview struct {
	Result *calc.Result `json:"result"`
}

// Calculate returns the current server-side calculation (C03).
func (a *App) Calculate(ctx context.Context, u *User) (*calc.Result, error) {
	ud, err := a.loadUserData(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	return calc.Compute(ud.input, a.Rates()), nil
}
