package calc

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestSharedFixtures runs F01–F03 from fixtures/core-cases.json.
func TestSharedFixtures(t *testing.T) {
	raw, err := os.ReadFile("../../fixtures/core-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Cases []json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	type outlay struct {
		Date     string   `json:"date"`
		Category CostType `json:"category"`
		Gross    int64    `json:"gross_cents"`
	}
	type caseT struct {
		ID   string `json:"id"`
		Trip *struct {
			Start        time.Time       `json:"start"`
			End          time.Time       `json:"end"`
			RateLocation string          `json:"rate_location"`
			Lodging      json.RawMessage `json:"lodging"`
			Breakfasts   []string        `json:"provided_breakfast_dates"`
			Transport    *struct {
				Gross int64 `json:"gross_cents"`
			} `json:"transport"`
		} `json:"trip"`
		Outlays  []outlay `json:"standalone_outlays"`
		Advance  Cents    `json:"advance_cents"`
		Expected struct {
			Daily   []Cents `json:"daily_meal_cents"`
			Travel  *Cents  `json:"travel_reimbursement_cents"`
			Outlays *Cents  `json:"standalone_outlays_cents"`
			Payout  Cents   `json:"payout_cents"`
		} `json:"expected"`
		Correction *struct {
			Payout Cents `json:"expected_payout_cents"`
		} `json:"correction"`
	}
	for _, rc := range fx.Cases {
		var c caseT
		if err := json.Unmarshal(rc, &c); err != nil {
			t.Fatal(err)
		}
		t.Run(c.ID, func(t *testing.T) {
			rates := newFixtureRates(2026)
			in := Input{}
			var tripIDs []int64
			if c.Trip != nil {
				tr := Trip{ID: 1, Start: c.Trip.Start.In(berlin), End: c.Trip.End.In(berlin), Origin: Domestic, Days: map[Date]DayInput{}, Nights: map[Date]NightKind{}}
				if c.Trip.RateLocation == "TEST_FOREIGN" {
					// Synthetic foreign location confirmed for every date.
					tr.Stops = []Stop{{Arrival: tr.Start, Location: Location{Country: "XF"}, Business: true}}
				}
				for _, b := range c.Trip.Breakfasts {
					withMeal(&tr, b, Meals{Breakfast: true})
				}
				var single struct {
					Nights []string `json:"nights"`
					Payer  Payer    `json:"payer"`
					Method string   `json:"method"`
					Gross  int64    `json:"gross_cents"`
				}
				var many []struct {
					Night  string `json:"night"`
					Payer  Payer  `json:"payer"`
					Gross  int64  `json:"gross_cents"`
					Method string `json:"method"`
				}
				id := int64(100)
				if json.Unmarshal(c.Trip.Lodging, &single) == nil && len(single.Nights) > 0 {
					e := userExpense(id, single.Nights[len(single.Nights)-1], CostLodging, single.Gross)
					for _, n := range single.Nights {
						e.Nights = append(e.Nights, MustDate(n))
						tr.Nights[MustDate(n)] = nightKind(single.Payer, single.Method)
					}
					tr.Expenses = append(tr.Expenses, e)
				} else if err := json.Unmarshal(c.Trip.Lodging, &many); err == nil {
					for _, n := range many {
						id++
						e := userExpense(id, n.Night, CostLodging, n.Gross)
						e.Payer = n.Payer
						e.Nights = []Date{MustDate(n.Night)}
						tr.Nights[MustDate(n.Night)] = nightKind(n.Payer, n.Method)
						tr.Expenses = append(tr.Expenses, e)
					}
				}
				if c.Trip.Transport != nil {
					tr.Expenses = append(tr.Expenses, userExpense(200, DateOf(tr.Start).String(), CostTransport, c.Trip.Transport.Gross))
				}
				in.Trips = append(in.Trips, tr)
				tripIDs = append(tripIDs, 1)
			}
			var outlayIDs []int64
			for i, o := range c.Outlays {
				in.Outlays = append(in.Outlays, userExpense(int64(300+i), o.Date, o.Category, o.Gross))
				outlayIDs = append(outlayIDs, int64(300+i))
			}
			r := Compute(in, rates)
			if b := blockers(r); len(b) > 0 {
				t.Fatalf("unexpected blockers: %v", b)
			}
			if c.Trip != nil {
				eqSlice(t, "daily", meals(r, 1), c.Expected.Daily)
			}
			var adv []Cents
			if c.Advance > 0 {
				adv = []Cents{c.Advance}
			}
			tot := r.Totals(tripIDs, outlayIDs, adv)
			if c.Expected.Travel != nil {
				eq(t, "travel", tot.Travel, *c.Expected.Travel)
			}
			if c.Expected.Outlays != nil {
				eq(t, "outlays", tot.Outlays, *c.Expected.Outlays)
			}
			eq(t, "payout", tot.Payout, c.Expected.Payout)

			if c.Correction != nil {
				// Change the software outlay from 2000 to 3000.
				for i := range in.Outlays {
					if in.Outlays[i].CostType == CostSoftwareServices {
						in.Outlays[i].GrossMinor = 3000
					}
				}
				tot = Compute(in, rates).Totals(tripIDs, outlayIDs, adv)
				eq(t, "corrected payout", tot.Payout, c.Correction.Payout)
			}
		})
	}
}

func nightKind(p Payer, method string) NightKind {
	switch {
	case p == PayerEmployer:
		return NightEmployer
	case method == "actual":
		return NightPrivateActual
	case method == "allowance":
		return NightPrivateAllowance
	}
	return NightPrivateNone
}

func TestF02CompanyPaidNightCannotUseAllowance(t *testing.T) {
	tr := trip(1, "2026-05-04 08:00", "2026-05-05 18:00")
	e := userExpense(1, "2026-05-04", CostLodging, 30000)
	e.Payer = PayerEmployer
	e.Nights = []Date{MustDate("2026-05-04")}
	tr.Expenses = []Expense{e}
	tr.Nights[MustDate("2026-05-04")] = NightPrivateAllowance
	r := run(t, Input{Trips: []Trip{tr}})
	eq(t, "blocked", hasIssue(r, "lodging.employer_paid_private_method"), true)
}
