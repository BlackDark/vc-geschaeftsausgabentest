package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
)

// claimFor creates a draft over the period and keeps only the given trips.
func claimFor(t *testing.T, a *App, u *User, period string, tripIDs ...int64) *Claim {
	t.Helper()
	ctx := context.Background()
	c := must(a.CreateClaim(ctx, u, ClaimInput{PeriodStart: period, PeriodEnd: period}))
	keep := map[int64]bool{}
	for _, id := range tripIDs {
		keep[id] = true
	}
	for _, id := range c.TripIDs {
		if !keep[id] {
			c = must(a.SetClaimItem(ctx, u, c.ID, ClaimItem{Kind: "trip", ID: id}, false))
		}
	}
	return c
}

func submit(t *testing.T, a *App, u *User, claimID int64) (*Claim, *Preview) {
	t.Helper()
	ctx := context.Background()
	p := must(a.PreviewClaim(ctx, u, claimID))
	if len(p.Blockers) > 0 {
		t.Fatalf("blockers: %+v", p.Blockers)
	}
	return must(a.SubmitClaim(ctx, u, claimID, SubmitInput{Revision: p.Revision, AcknowledgeWarnings: true})), p
}

func newUser(t *testing.T, a *App) *User {
	t.Helper()
	ctx := context.Background()
	must(a.Setup(ctx, "tok", "tok", "user-a", "User A", "correct horse battery"))
	u := must(a.PasswordLogin(ctx, "user-a", "correct horse battery", "test"))
	must(a.SaveEmployer(ctx, u, EmployerInput{Name: "Example Company"}))
	return u
}

// TestA25DailyCapAcrossClaims: two one-day trips on the same day, claimed
// separately, together earn one allowance (C06/C07). The 4 h and 5 h
// absences add up to more than 8 h, so the day is worth 14 EUR once.
func TestA25DailyCapAcrossClaims(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	u := newUser(t, a)
	day := func(start, end string) *Trip {
		return must(a.CreateTrip(ctx, u, TripInput{Purpose: "Termin " + start, Start: "2026-05-04T" + start, StartTZ: "Europe/Berlin", End: "2026-05-04T" + end, EndTZ: "Europe/Berlin"}))
	}
	morning, afternoon := day("07:00", "11:00"), day("13:00", "18:00")

	first, p1 := submit(t, a, u, claimFor(t, a, u, "2026-05-04", afternoon.ID).ID)
	if p1.Totals.Payout != 1400 {
		t.Fatalf("first claim payout = %v, want 14.00", p1.Totals.Payout)
	}

	second := claimFor(t, a, u, "2026-05-04", morning.ID)
	p2 := must(a.PreviewClaim(ctx, u, second.ID))
	if p2.Totals.Payout != 0 {
		t.Fatalf("second claim payout = %v, want 0: the day is already paid", p2.Totals.Payout)
	}
	d := p2.Trips[0].Result.Days[0]
	if d.Payable != 0 || d.FrozenElsewhere != 1400 {
		t.Fatalf("second claim day: payable %v, frozen elsewhere %v; want 0 and 14.00", d.Payable, d.FrozenElsewhere)
	}
	submit(t, a, u, second.ID)

	// Unlocking the first claim must not pay the day twice either: the
	// second claim's frozen 0 leaves the full day to the first.
	must(a.UnlockClaim(ctx, u, first.ID, "Korrektur"))
	again, _ := submit(t, a, u, first.ID)
	if again.Versions[0].Payout != 1400 {
		t.Fatalf("resubmitted first claim payout = %v, want 14.00", again.Versions[0].Payout)
	}
}

// TestA22ExportMatchesPreview checks that the submitted snapshot (which the
// PDF is rendered from) and the CSV extracts add up to the preview the user
// confirmed, for a claim with meals, lodging, mileage and an outlay.
func TestA22ExportMatchesPreview(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	u := newUser(t, a)
	trip := must(a.CreateTrip(ctx, u, TripInput{
		Purpose: "Messe", Start: "2026-06-08T07:00", StartTZ: "Europe/Berlin", End: "2026-06-09T19:00", EndTZ: "Europe/Berlin",
		Nights:   []NightDTO{{Date: "2026-06-08", Kind: "private_actual"}},
		Journeys: []JourneyDTO{{Date: "2026-06-08", Vehicle: calc.VehicleCar, DistanceMeters: 123_400}, {Date: "2026-06-09", Vehicle: calc.VehicleCar, DistanceMeters: 123_400}},
	}))
	must(a.CreateExpense(ctx, u, ExpenseInput{TripID: &trip.ID, Date: "2026-06-09", CostType: "lodging", Payer: "user", Supplier: "Hotel", Currency: "EUR", GrossMinor: 9950, VATUnknown: true, Nights: []string{"2026-06-08"}, ReceiptIDs: []int64{receipt(t, a, u, "hotel.png", 1)}}))
	must(a.CreateExpense(ctx, u, ExpenseInput{TripID: &trip.ID, Date: "2026-06-08", CostType: "transport", Payer: "user", Supplier: "Parkhaus", Currency: "EUR", GrossMinor: 1800, VATUnknown: true, ReceiptIDs: []int64{receipt(t, a, u, "parking.png", 2)}}))
	must(a.CreateExpense(ctx, u, ExpenseInput{Date: "2026-06-10", CostType: "tools_equipment", Payer: "user", Supplier: "Werkzeug", Currency: "EUR", GrossMinor: 2390, VATUnknown: true, ReceiptIDs: []int64{receipt(t, a, u, "tools.png", 3)}}))
	must(a.CreateAdvance(ctx, u, AdvanceInput{Date: "2026-06-01", Amount: 5000}))

	c := must(a.CreateClaim(ctx, u, ClaimInput{PeriodStart: "2026-06-01", PeriodEnd: "2026-06-30"}))
	claim, p := submit(t, a, u, c.ID)
	zipBytes, _ := must2(a.VersionFile(ctx, u, claim.ID, 1, "zip"))
	files := unzip(t, zipBytes)

	var snap struct {
		Totals calc.ClaimTotals `json:"totals"`
	}
	if err := json.Unmarshal(files["snapshot-v1.json"], &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Totals != p.Totals {
		t.Fatalf("snapshot totals %+v, preview %+v", snap.Totals, p.Totals)
	}

	expenses := csvRows(t, files["expenses-v1.csv"])
	tripExpenses := sumColumn(t, expenses, "reimbursable_eur", func(r map[string]string) bool { return r["standalone"] == "false" })
	outlays := sumColumn(t, expenses, "reimbursable_eur", func(r map[string]string) bool { return r["standalone"] == "true" })
	meals := sumColumn(t, csvRows(t, files["travel-days-v1.csv"]), "payable_eur", nil)
	mileage := sumColumn(t, csvRows(t, files["mileage-v1.csv"]), "amount_eur", nil)
	tr := p.Trips[0].Result.Totals
	for _, c := range []struct {
		name      string
		csv, want calc.Cents
	}{
		{"meals", meals, tr.Meals},
		{"mileage", mileage, tr.Mileage},
		{"trip expenses", tripExpenses, tr.Expenses},
		{"outlays", outlays, p.Totals.Outlays},
		{"travel", meals + mileage + tripExpenses + tr.LodgingAllowance, p.Totals.Travel},
		{"payout", meals + mileage + tripExpenses + tr.LodgingAllowance + outlays - p.Totals.Advances, p.Totals.Payout},
	} {
		if c.csv != c.want {
			t.Errorf("%s: CSV %v, preview %v", c.name, c.csv, c.want)
		}
	}
	if mileage == 0 || meals == 0 || outlays == 0 || tripExpenses == 0 {
		t.Fatalf("scenario does not exercise every part: meals %v, mileage %v, expenses %v, outlays %v", meals, mileage, tripExpenses, outlays)
	}
}

func unzip(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		r, _ := f.Open()
		files[f.Name], _ = io.ReadAll(r)
		r.Close()
	}
	return files
}

// csvRows parses a CSV extract into header-keyed rows.
func csvRows(t *testing.T, b []byte) []map[string]string {
	t.Helper()
	records, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]string
	for _, rec := range records[1:] {
		row := map[string]string{}
		for i, h := range records[0] {
			row[h] = rec[i]
		}
		rows = append(rows, row)
	}
	return rows
}

// sumColumn adds a decimal EUR column ("12.34") as cents.
func sumColumn(t *testing.T, rows []map[string]string, col string, keep func(map[string]string) bool) calc.Cents {
	t.Helper()
	var sum calc.Cents
	for _, r := range rows {
		if keep != nil && !keep(r) {
			continue
		}
		whole, frac, _ := strings.Cut(r[col], ".")
		n, err := strconv.ParseInt(whole+frac, 10, 64)
		if err != nil || len(frac) != 2 {
			t.Fatalf("%s: %q is not a cent amount", col, r[col])
		}
		sum += calc.Cents(n)
	}
	return sum
}
