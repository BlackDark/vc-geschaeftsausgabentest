package calc

import (
	"math/big"
	"sort"
	"strconv"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/money"
)

type computer struct {
	in    Input
	rates Rates
	res   *Result
}

// Compute calculates all trips and standalone outlays of one user. Trips
// outside a claim must be included so the daily cap can be applied (C07).
func Compute(in Input, rates Rates) *Result {
	c := &computer{in: in, rates: rates, res: &Result{Trips: map[int64]*TripResult{}}}
	for _, t := range in.Trips {
		c.trip(t.ID)
	}
	c.meals()
	for i := range c.in.Trips {
		t := &c.in.Trips[i]
		c.lodging(t)
		c.mileage(t)
		c.tripExpenses(t)
		c.tripTotals(t)
	}
	for _, e := range in.Outlays {
		if !e.CostType.IsOutlay() {
			c.issue(Issue{Severity: Blocker, RuleID: RuleOutlay, Code: "outlay.invalid_category", ExpenseID: e.ID})
		}
		r := c.expense(e, 0)
		r.RuleIDs = append(r.RuleIDs, RuleOutlay)
		c.res.Outlays = append(c.res.Outlays, r)
	}
	sort.SliceStable(c.res.Issues, func(i, j int) bool {
		return severityRank(c.res.Issues[i].Severity) < severityRank(c.res.Issues[j].Severity)
	})
	return c.res
}

func severityRank(s Severity) int {
	switch s {
	case Blocker:
		return 0
	case Warning:
		return 1
	}
	return 2
}

func (c *computer) trip(id int64) *TripResult {
	r, ok := c.res.Trips[id]
	if !ok {
		r = &TripResult{TripID: id, Days: []Day{}, Nights: []Night{}, Journeys: []JourneyResult{}, Expenses: []ExpenseResult{}}
		c.res.Trips[id] = r
	}
	return r
}

func (c *computer) issue(is Issue) { c.res.Issues = append(c.res.Issues, is) }

// lodging evaluates each night after a travel day (C12).
func (c *computer) lodging(t *Trip) {
	if !t.End.After(t.Start) {
		return
	}
	res := c.trip(t.ID)
	dates := datesBetween(DateOf(t.Start), DateOf(t.End))
	for i, d := range dates[:len(dates)-1] {
		kind := t.Nights[d]
		n := Night{Date: d, Kind: kind, Location: c.location(t, d, i == len(dates)-1), RuleID: RuleLodging}
		switch kind {
		case NightUnset:
			c.issue(Issue{Severity: Blocker, RuleID: RuleLodging, Code: "lodging.unset", TripID: t.ID, Date: d.String()})
		case NightPrivateAllowance:
			rate, ok := c.mealRate(d, n.Location, t.ID)
			if ok {
				ref := rate.Ref
				n.Rate = &ref
				n.Allowance = rate.Lodging
			}
		case NightPrivateActual:
			if !hasLodgingExpense(t, d, PayerUser) {
				c.issue(Issue{Severity: Warning, RuleID: RuleLodging, Code: "lodging.actual_without_expense", TripID: t.ID, Date: d.String()})
			}
		}
		res.Nights = append(res.Nights, n)
	}
}

func hasLodgingExpense(t *Trip, d Date, payer Payer) bool {
	for _, e := range t.Expenses {
		if e.CostType != CostLodging || e.Payer != payer {
			continue
		}
		for _, n := range e.Nights {
			if n == d {
				return true
			}
		}
	}
	return false
}

// mileage computes private-vehicle reimbursement (C14).
func (c *computer) mileage(t *Trip) {
	res := c.trip(t.ID)
	fuel := false
	for _, e := range t.Expenses {
		fuel = fuel || e.Fuel
	}
	for _, j := range t.Journeys {
		jr := JourneyResult{JourneyID: j.ID, Date: j.Date, Vehicle: j.Vehicle, DistanceMeters: j.DistanceMeters, RuleID: RuleMileage}
		rate, err := c.rates.Mileage(j.Date, j.Vehicle)
		if err != nil {
			c.issue(Issue{Severity: Blocker, RuleID: RuleRates, Code: "rate.mileage_missing", TripID: t.ID, Date: j.Date.String(), Params: map[string]string{"vehicle": string(j.Vehicle)}})
		} else {
			ref := rate.Ref
			jr.Rate = &ref
			jr.CentsPerKm = rate.CentsPerKm
			jr.Amount = Cents(money.RoundRat(big.NewRat(j.DistanceMeters*rate.CentsPerKm, 1000)))
		}
		if j.DistanceMeters <= 0 {
			c.issue(Issue{Severity: Blocker, RuleID: RuleMileage, Code: "mileage.distance_invalid", TripID: t.ID, Date: j.Date.String()})
		}
		res.Journeys = append(res.Journeys, jr)
	}
	if fuel && len(t.Journeys) > 0 {
		c.issue(Issue{Severity: Warning, RuleID: RuleMileage, Code: "mileage.fuel_and_mileage", TripID: t.ID})
	}
}

// tripExpenses evaluates trip-linked expenses, applying the lodging method.
func (c *computer) tripExpenses(t *Trip) {
	res := c.trip(t.ID)
	first, last := DateOf(t.Start), DateOf(t.End)
	for _, e := range t.Expenses {
		r := c.expense(e, t.ID)
		if !e.CostType.IsTravel() {
			c.issue(Issue{Severity: Blocker, RuleID: RuleExpense, Code: "expense.invalid_travel_category", TripID: t.ID, ExpenseID: e.ID})
		}
		if e.Date.Before(first.AddDays(-1)) || last.AddDays(1).Before(e.Date) {
			c.issue(Issue{Severity: Warning, RuleID: RuleExpense, Code: "expense.outside_trip", TripID: t.ID, ExpenseID: e.ID, Date: e.Date.String()})
		}
		if e.CostType == CostLodging {
			c.applyLodgingMethod(t, e, &r)
		}
		res.Expenses = append(res.Expenses, r)
	}
}

func (c *computer) applyLodgingMethod(t *Trip, e Expense, r *ExpenseResult) {
	r.RuleIDs = append(r.RuleIDs, RuleLodging)
	if len(e.Nights) == 0 {
		c.issue(Issue{Severity: Blocker, RuleID: RuleLodging, Code: "lodging.expense_without_nights", TripID: t.ID, ExpenseID: e.ID})
		return
	}
	kinds := map[NightKind]bool{}
	for _, n := range e.Nights {
		kinds[t.Nights[n]] = true
	}
	if e.Payer == PayerEmployer {
		if kinds[NightPrivateAllowance] || kinds[NightPrivateActual] || kinds[NightPrivateNone] {
			c.issue(Issue{Severity: Blocker, RuleID: RuleLodging, Code: "lodging.employer_paid_private_method", TripID: t.ID, ExpenseID: e.ID})
		}
		return
	}
	if len(kinds) > 1 {
		c.issue(Issue{Severity: Blocker, RuleID: RuleLodging, Code: "lodging.mixed_methods", TripID: t.ID, ExpenseID: e.ID})
		r.Reimbursable = 0
		return
	}
	switch {
	case kinds[NightPrivateActual]:
		// payable as recorded
	case kinds[NightPrivateAllowance]:
		r.Reimbursable, r.Reason = 0, "replaced_by_allowance"
	case kinds[NightEmployer]:
		c.issue(Issue{Severity: Blocker, RuleID: RuleLodging, Code: "lodging.user_paid_employer_night", TripID: t.ID, ExpenseID: e.ID})
		r.Reimbursable = 0
	default:
		r.Reimbursable, r.Reason = 0, "not_reimbursed"
	}
}

// expense converts an expense to EUR and determines its payable amount
// (C15, C16, C18, C19, C20).
func (c *computer) expense(e Expense, tripID int64) ExpenseResult {
	r := ExpenseResult{ExpenseID: e.ID, CostType: e.CostType, Payer: e.Payer, RuleIDs: []string{RuleExpense}}
	issue := func(sev Severity, rule, code string) {
		c.issue(Issue{Severity: sev, RuleID: rule, Code: code, TripID: tripID, ExpenseID: e.ID, Date: e.Date.String()})
	}
	exp, ok := money.Exponent(e.Currency)
	if !ok {
		issue(Blocker, RuleMoney, "expense.currency_invalid")
		return r
	}
	if e.GrossMinor <= 0 {
		issue(Blocker, RuleMoney, "expense.amount_invalid")
	}

	// Currency conversion: convert the total once.
	var eurRate *big.Rat
	switch {
	case e.Currency == "EUR":
		r.GrossEUR = Cents(e.GrossMinor)
	case e.EURDebit != nil:
		r.GrossEUR = *e.EURDebit
		r.FXRule = "actual_debit"
		r.RuleIDs = append(r.RuleIDs, RuleFX)
	case e.FXRate != "":
		rate, err := money.ParseRate(e.FXRate)
		if err != nil {
			issue(Blocker, RuleFX, "fx.rate_invalid")
			break
		}
		eurRate = rate
		r.GrossEUR = money.ToEUR(e.GrossMinor, exp, rate)
		r.FXRule = "manual_rate"
		r.RuleIDs = append(r.RuleIDs, RuleFX)
	default:
		issue(Blocker, RuleFX, "fx.missing")
		r.Reason = "fx_missing"
	}

	// VAT portions (C16, C19).
	if len(e.VAT) > 0 {
		r.VAT = c.vat(e, exp, eurRate, r.GrossEUR, issue)
		r.RuleIDs = append(r.RuleIDs, RuleVAT)
	} else if !e.VATUnknown {
		issue(Notice, RuleVAT, "vat.not_recorded")
	}

	// Evidence (P10/P11).
	if e.Unconfirmed > 0 {
		issue(Blocker, RuleExpense, "receipt.unconfirmed")
	}
	if e.Payer == PayerUser && e.Receipts == 0 && !e.SelfIssued && e.Unconfirmed == 0 {
		issue(Warning, RuleExpense, "receipt.missing")
	}
	if e.DuplicateSHA {
		issue(Warning, RuleExpense, "receipt.duplicate")
	}

	// Hospitality (P12, C20).
	if e.CostType == CostHospitality {
		r.RuleIDs = append(r.RuleIDs, RuleHospitality)
		h := e.Hospitality
		if h == nil || h.Occasion == "" || h.Place == "" || len(h.Participants) == 0 || h.ExternalCount < 1 || !h.Confirmed {
			issue(Blocker, RuleHospitality, "hospitality.incomplete")
		}
	}

	if e.Payer == PayerEmployer {
		r.Reimbursable, r.Reason = 0, "employer_paid"
	} else if r.Reason == "" {
		r.Reimbursable = r.GrossEUR
	}
	return r
}

// vat validates portions and converts them to EUR, distributing rounding
// residuals so the EUR portions sum to the converted total (C19).
func (c *computer) vat(e Expense, exp int, eurRate *big.Rat, totalEUR Cents, issue func(Severity, string, string)) []VATResult {
	var sum int64
	for _, p := range e.VAT {
		sum += p.GrossMinor
		if p.StatedNet != nil && p.StatedVAT != nil && *p.StatedNet+*p.StatedVAT != p.GrossMinor {
			issue(Blocker, RuleVAT, "vat.portion_inconsistent")
		}
	}
	if sum != e.GrossMinor {
		issue(Blocker, RuleVAT, "vat.portions_sum")
		return nil
	}
	out := make([]VATResult, len(e.VAT))
	var converted Cents
	largest := 0
	for i, p := range e.VAT {
		var gross Cents
		switch {
		case e.Currency == "EUR":
			gross = Cents(p.GrossMinor)
		case eurRate != nil:
			gross = money.ToEUR(p.GrossMinor, exp, eurRate)
		default:
			// Actual debit: allocate proportionally to the foreign gross.
			gross = Cents(money.RoundRat(big.NewRat(int64(totalEUR)*p.GrossMinor, e.GrossMinor)))
		}
		out[i] = VATResult{RateBP: p.RateBP, Country: p.Country, Gross: gross}
		converted += gross
		if p.GrossMinor > e.VAT[largest].GrossMinor {
			largest = i
		}
	}
	out[largest].Gross += totalEUR - converted
	for i, p := range e.VAT {
		g := out[i].Gross
		var vat Cents
		if p.StatedVAT != nil {
			vat = Cents(money.RoundRat(big.NewRat(int64(g)**p.StatedVAT, p.GrossMinor)))
			computed := money.RoundRat(big.NewRat(p.GrossMinor*p.RateBP, 10000+p.RateBP))
			if d := computed - *p.StatedVAT; d > 1 || d < -1 {
				issue(Warning, RuleVAT, "vat.stated_deviates")
			}
		} else {
			vat = Cents(money.RoundRat(big.NewRat(int64(g)*p.RateBP, 10000+p.RateBP)))
		}
		out[i].VAT = vat
		out[i].Net = g - vat
	}
	return out
}

// tripTotals sums each payable component once (C21).
func (c *computer) tripTotals(t *Trip) {
	res := c.trip(t.ID)
	tot := TripTotals{ByCostType: map[CostType]Cents{}}
	for _, d := range res.Days {
		tot.Meals += d.Payable
	}
	for _, n := range res.Nights {
		tot.LodgingAllowance += n.Allowance
	}
	for _, j := range res.Journeys {
		tot.Mileage += j.Amount
	}
	for _, e := range res.Expenses {
		tot.Expenses += e.Reimbursable
		tot.ByCostType[e.CostType] += e.Reimbursable
		if e.Payer == PayerEmployer {
			tot.CompanyPaid += e.GrossEUR
		}
	}
	tot.Total = tot.Meals + tot.LodgingAllowance + tot.Mileage + tot.Expenses
	res.Totals = tot
}

func itoa(n int) string { return strconv.Itoa(n) }
