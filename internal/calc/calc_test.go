package calc

import (
	"fmt"
	"testing"
	"time"
)

// fixtureRates are synthetic test rates (fixtures/core-cases.json), not
// production data.
type fixtureRates struct {
	meal    map[string]MealRate // key: year/country/city
	mileage map[Vehicle]int64
}

func newFixtureRates(years ...int) *fixtureRates {
	r := &fixtureRates{meal: map[string]MealRate{}, mileage: map[Vehicle]int64{VehicleCar: 30, VehicleOther: 20}}
	for _, y := range years {
		r.set(y, Domestic, 2800, 1400, 2000)
		r.set(y, Location{Country: "XF"}, 7000, 4700, 10000)
	}
	return r
}

func (r *fixtureRates) set(year int, loc Location, full, partial, lodging Cents) {
	r.meal[fmt.Sprintf("%d/%s/%s", year, loc.Country, loc.City)] = MealRate{
		Ref: RateRef{Table: "fixture", Year: year, Location: loc, Source: "fixture", Revision: 1}, Full: full, Partial: partial, Lodging: lodging,
	}
}

func (r *fixtureRates) Meal(year int, loc Location) (MealRate, error) {
	if m, ok := r.meal[fmt.Sprintf("%d/%s/%s", year, loc.Country, loc.City)]; ok {
		return m, nil
	}
	if m, ok := r.meal[fmt.Sprintf("%d/%s/", year, loc.Country)]; ok {
		return m, nil
	}
	return MealRate{}, fmt.Errorf("no rate")
}

func (r *fixtureRates) Mileage(d Date, v Vehicle) (MileageRate, error) {
	c, ok := r.mileage[v]
	if !ok {
		return MileageRate{}, fmt.Errorf("no rate")
	}
	return MileageRate{Ref: RateRef{Table: "fixture", Year: d.Y}, CentsPerKm: c}, nil
}

var berlin, _ = time.LoadLocation("Europe/Berlin")

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, berlin)
	if err != nil {
		panic(err)
	}
	return t
}

func trip(id int64, start, end string) Trip {
	return Trip{ID: id, Start: at(start), End: at(end), Origin: Domestic, Days: map[Date]DayInput{}, Nights: map[Date]NightKind{}}
}

func run(t *testing.T, in Input) *Result {
	t.Helper()
	return Compute(in, newFixtureRates(2025, 2026))
}

func meals(r *Result, id int64) []Cents {
	var out []Cents
	for _, d := range r.Trips[id].Days {
		out = append(out, d.Payable)
	}
	return out
}

func eq[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func eqSlice(t *testing.T, name string, got, want []Cents) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func blockers(r *Result) []string {
	var out []string
	for _, i := range r.Issues {
		if i.Severity == Blocker {
			out = append(out, i.Code)
		}
	}
	return out
}

func hasIssue(r *Result, code string) bool {
	for _, i := range r.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func withMeal(t *Trip, date string, m Meals) {
	in := t.Days[MustDate(date)]
	in.Provided = m
	t.Days[MustDate(date)] = in
}

func userExpense(id int64, date string, ct CostType, cents int64) Expense {
	return Expense{ID: id, Date: MustDate(date), CostType: ct, Payer: PayerUser, Currency: "EUR", GrossMinor: cents, VATUnknown: true, Receipts: 1}
}

func TestA01OneDayLunch(t *testing.T) {
	tr := trip(1, "2026-03-02 07:00", "2026-03-02 17:30")
	withMeal(&tr, "2026-03-02", Meals{Lunch: true})
	r := run(t, Input{Trips: []Trip{tr}})
	eqSlice(t, "meals", meals(r, 1), []Cents{280})
}

func TestA02EightHourThreshold(t *testing.T) {
	r := run(t, Input{Trips: []Trip{trip(1, "2026-03-02 08:00", "2026-03-02 16:00")}})
	eqSlice(t, "exactly 8h", meals(r, 1), []Cents{0})
	r = run(t, Input{Trips: []Trip{trip(1, "2026-03-02 08:00", "2026-03-02 16:01")}})
	eqSlice(t, "8h01", meals(r, 1), []Cents{1400})
}

func TestA03HotelWithBreakfast(t *testing.T) {
	tr := trip(1, "2026-03-02 08:00", "2026-03-04 18:00")
	tr.Nights[MustDate("2026-03-02")] = NightPrivateActual
	tr.Nights[MustDate("2026-03-03")] = NightPrivateActual
	withMeal(&tr, "2026-03-03", Meals{Breakfast: true})
	withMeal(&tr, "2026-03-04", Meals{Breakfast: true})
	hotel := userExpense(10, "2026-03-04", CostLodging, 17800)
	hotel.Nights = []Date{MustDate("2026-03-02"), MustDate("2026-03-03")}
	tr.Expenses = []Expense{hotel}
	r := run(t, Input{Trips: []Trip{tr}})
	eqSlice(t, "meals", meals(r, 1), []Cents{1400, 2240, 840})
	eq(t, "lodging", r.Trips[1].Totals.Expenses, Cents(17800))
	eq(t, "total", r.Trips[1].Totals.Total, Cents(22280))
	eq(t, "blockers", len(blockers(r)), 0)
}

func TestA04MidnightWithoutLodging(t *testing.T) {
	tr := trip(1, "2026-03-03 19:30", "2026-03-04 06:00")
	tr.Nights[MustDate("2026-03-03")] = NightNone
	r := run(t, Input{Trips: []Trip{tr}})
	eqSlice(t, "meals", meals(r, 1), []Cents{0, 1400})
}

func TestA05AggregatedOneDayTrips(t *testing.T) {
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		trips := []Trip{trip(1, "2026-03-02 06:30", "2026-03-02 10:00"), trip(2, "2026-03-02 13:00", "2026-03-02 18:00")}
		r := run(t, Input{Trips: []Trip{trips[order[0]], trips[order[1]]}})
		eqSlice(t, "first", meals(r, 1), []Cents{0})
		eqSlice(t, "later", meals(r, 2), []Cents{1400})
		eq(t, "allocated", r.Trips[1].Days[0].AllocatedTo, int64(2))
	}
}

func TestA06ForeignDayUsesOwnFullRate(t *testing.T) {
	tr := trip(1, "2026-03-02 06:00", "2026-03-02 20:00")
	tr.Stops = []Stop{{Arrival: at("2026-03-02 09:00"), Location: Location{Country: "XF"}, Business: true}}
	withMeal(&tr, "2026-03-02", Meals{Lunch: true})
	r := run(t, Input{Trips: []Trip{tr}})
	eqSlice(t, "meals", meals(r, 1), []Cents{1900})
}

func TestA07NoNegativeDay(t *testing.T) {
	tr := trip(1, "2026-03-02 08:00", "2026-03-03 18:00")
	tr.Nights[MustDate("2026-03-02")] = NightEmployer
	withMeal(&tr, "2026-03-02", Meals{Lunch: true, Dinner: true})
	withMeal(&tr, "2026-03-03", Meals{Breakfast: true})
	r := run(t, Input{Trips: []Trip{tr}})
	eqSlice(t, "meals", meals(r, 1), []Cents{0, 840})
}

func TestA08Contribution(t *testing.T) {
	for _, tc := range []struct {
		contrib Cents
		want    Cents
	}{{500, 780}, {1500, 1400}} {
		tr := trip(1, "2026-03-02 07:00", "2026-03-02 17:30")
		tr.Days[MustDate("2026-03-02")] = DayInput{Provided: Meals{Lunch: true}, Contributions: Contributions{Lunch: tc.contrib}}
		r := run(t, Input{Trips: []Trip{tr}})
		eqSlice(t, fmt.Sprint("contribution ", tc.contrib), meals(r, 1), []Cents{tc.want})
	}
}

func TestA09LodgingAllowanceAndProvided(t *testing.T) {
	tr := trip(1, "2026-03-02 08:00", "2026-03-04 18:00")
	tr.Nights[MustDate("2026-03-02")] = NightPrivateAllowance
	tr.Nights[MustDate("2026-03-03")] = NightEmployer
	r := run(t, Input{Trips: []Trip{tr}})
	eq(t, "lodging allowance", r.Trips[1].Totals.LodgingAllowance, Cents(2000))

	// An actual invoice for an allowance night is kept as context, not paid.
	inv := userExpense(10, "2026-03-03", CostLodging, 9000)
	inv.Nights = []Date{MustDate("2026-03-02")}
	tr.Expenses = []Expense{inv}
	r = run(t, Input{Trips: []Trip{tr}})
	eq(t, "invoice payable", r.Trips[1].Expenses[0].Reimbursable, Cents(0))
	eq(t, "reason", r.Trips[1].Expenses[0].Reason, "replaced_by_allowance")
	eq(t, "total lodging", r.Trips[1].Totals.LodgingAllowance+r.Trips[1].Totals.Expenses, Cents(2000))
}

func TestA10Mileage(t *testing.T) {
	tr := trip(1, "2026-03-02 07:00", "2026-03-02 12:00")
	tr.Journeys = []Journey{
		{ID: 1, Date: MustDate("2026-03-02"), Vehicle: VehicleCar, DistanceMeters: 284000},
		{ID: 2, Date: MustDate("2026-03-02"), Vehicle: VehicleOther, DistanceMeters: 38000},
	}
	r := run(t, Input{Trips: []Trip{tr}})
	eq(t, "mileage", r.Trips[1].Totals.Mileage, Cents(9280))
}

func TestA11VATPortions(t *testing.T) {
	e := userExpense(1, "2026-03-02", CostToolsEquipment, 13317)
	e.VATUnknown = false
	e.VAT = []VATPortion{{GrossMinor: 11770, RateBP: 700, Country: "DE"}, {GrossMinor: 1547, RateBP: 1900, Country: "DE"}}
	r := run(t, Input{Outlays: []Expense{e}})
	v := r.Outlays[0].VAT
	eq(t, "net", v[0].Net+v[1].Net, Cents(12300))
	eq(t, "vat", v[0].VAT+v[1].VAT, Cents(1017))
	eq(t, "vat7", v[0].VAT, Cents(770))
	eq(t, "vat19", v[1].VAT, Cents(247))

	e.VAT[1].GrossMinor = 1500
	r = run(t, Input{Outlays: []Expense{e}})
	eq(t, "sum mismatch blocks", hasIssue(r, "vat.portions_sum"), true)
}

func TestA12ForeignCurrency(t *testing.T) {
	e := userExpense(1, "2026-03-07", CostToolsEquipment, 4850)
	e.Currency = "USD"
	e.FXRate = "1.1650"
	fri := MustDate("2026-03-06")
	e.FXDate = &fri
	r := run(t, Input{Outlays: []Expense{e}})
	eq(t, "eur", r.Outlays[0].Reimbursable, Cents(4163))

	debit := Cents(4237)
	e.EURDebit = &debit
	r = run(t, Input{Outlays: []Expense{e}})
	eq(t, "actual debit", r.Outlays[0].Reimbursable, Cents(4237))

	// Non-two-decimal exponent: JPY 1000 at 160.00 per EUR = 6.25.
	j := userExpense(2, "2026-03-07", CostToolsEquipment, 1000)
	j.Currency, j.FXRate = "JPY", "160"
	r = run(t, Input{Outlays: []Expense{j}})
	eq(t, "jpy", r.Outlays[0].Reimbursable, Cents(625))

	j.FXRate = ""
	r = run(t, Input{Outlays: []Expense{j}})
	eq(t, "fx missing blocks", hasIssue(r, "fx.missing"), true)
}

func TestA13AdvancesAndRepayment(t *testing.T) {
	r := run(t, Input{Outlays: []Expense{userExpense(1, "2026-03-02", CostOtherOutlay, 41240)}})
	tot := r.Totals(nil, []int64{1}, []Cents{30000})
	eq(t, "payout", tot.Payout, Cents(11240))
	tot = r.Totals(nil, []int64{1}, []Cents{50000})
	eq(t, "repayment", tot.Payout, Cents(-8760))
	eq(t, "repayment flag", tot.Repayment, true)
}

func TestA14YearBoundary(t *testing.T) {
	rates := &fixtureRates{meal: map[string]MealRate{}}
	rates.set(2025, Domestic, 4700, 3100, 2000)
	rates.set(2026, Domestic, 5900, 3900, 2000)
	tr := trip(1, "2025-12-30 08:00", "2026-01-01 18:00")
	tr.Nights[MustDate("2025-12-30")] = NightEmployer
	tr.Nights[MustDate("2025-12-31")] = NightEmployer
	r := Compute(Input{Trips: []Trip{tr}}, rates)
	eqSlice(t, "days", meals(r, 1), []Cents{3100, 4700, 3900})

	missing := &fixtureRates{meal: map[string]MealRate{}}
	missing.set(2025, Domestic, 4700, 3100, 2000)
	r = Compute(Input{Trips: []Trip{tr}}, missing)
	eq(t, "missing year blocks", hasIssue(r, "rate.missing"), true)
}

func TestFrozenAllocation(t *testing.T) {
	locked := trip(1, "2026-03-02 06:30", "2026-03-02 10:00")
	locked.Locked = true
	locked.FrozenMeals = map[Date]Cents{}
	later := trip(2, "2026-03-02 13:00", "2026-03-02 18:00")
	r := run(t, Input{Trips: []Trip{locked, later}})
	eqSlice(t, "remainder", meals(r, 2), []Cents{1400})

	// The locked trip already received the full day: nothing remains.
	locked.FrozenMeals[MustDate("2026-03-02")] = 1400
	r = run(t, Input{Trips: []Trip{locked, later}})
	eqSlice(t, "nothing left", meals(r, 2), []Cents{0})
	eqSlice(t, "locked keeps", meals(r, 1), []Cents{1400})

	// Frozen exceeds the recalculated entitlement: blocker.
	locked.FrozenMeals[MustDate("2026-03-02")] = 2800
	r = run(t, Input{Trips: []Trip{locked, later}})
	eq(t, "overallocated", hasIssue(r, "meal.overallocated"), true)
}

func TestReturnFromAbroadUsesLastForeignBusinessLocation(t *testing.T) {
	tr := trip(1, "2026-03-02 08:00", "2026-03-04 20:00")
	tr.Nights[MustDate("2026-03-02")] = NightEmployer
	tr.Nights[MustDate("2026-03-03")] = NightEmployer
	tr.Stops = []Stop{
		{Arrival: at("2026-03-02 12:00"), Location: Location{Country: "XF"}, Business: true},
		{Arrival: at("2026-03-04 19:00"), Location: Domestic},
	}
	r := run(t, Input{Trips: []Trip{tr}})
	eqSlice(t, "meals", meals(r, 1), []Cents{4700, 7000, 4700})
}

func TestUnsetNightBlocks(t *testing.T) {
	r := run(t, Input{Trips: []Trip{trip(1, "2026-03-02 08:00", "2026-03-03 18:00")}})
	eq(t, "unset", hasIssue(r, "lodging.unset"), true)
}

func TestHospitalityRequiresParticipants(t *testing.T) {
	tr := trip(1, "2026-03-02 07:00", "2026-03-02 20:00")
	h := userExpense(1, "2026-03-02", CostHospitality, 12000)
	h.Hospitality = &Hospitality{Occasion: "Projekt", Place: "Restaurant", Participants: []string{"A"}, ExternalCount: 0, Confirmed: true}
	tr.Expenses = []Expense{h}
	r := run(t, Input{Trips: []Trip{tr}})
	eq(t, "no external", hasIssue(r, "hospitality.incomplete"), true)
	tr.Expenses[0].Hospitality.ExternalCount = 1
	r = run(t, Input{Trips: []Trip{tr}})
	eq(t, "complete", hasIssue(r, "hospitality.incomplete"), false)
	eq(t, "gross once", r.Trips[1].Totals.Expenses, Cents(12000))
}

func TestReceiptStates(t *testing.T) {
	e := userExpense(1, "2026-03-02", CostOtherOutlay, 1000)
	e.Receipts = 0
	r := run(t, Input{Outlays: []Expense{e}})
	eq(t, "missing is warning", hasIssue(r, "receipt.missing") && len(blockers(r)) == 0, true)
	e.Unconfirmed = 1
	r = run(t, Input{Outlays: []Expense{e}})
	eq(t, "unconfirmed blocks", hasIssue(r, "receipt.unconfirmed"), true)
}

func TestDSTOneDayTrip(t *testing.T) {
	// 2026-03-29: clocks jump from 02:00 to 03:00. 01:00–09:30 local is 7.5h.
	r := run(t, Input{Trips: []Trip{trip(1, "2026-03-29 01:00", "2026-03-29 09:30")}})
	eqSlice(t, "dst", meals(r, 1), []Cents{0})
	eq(t, "minutes", r.Trips[1].Days[0].AbsenceMinutes, int64(450))
}
