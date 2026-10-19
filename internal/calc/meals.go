package calc

import (
	"sort"
	"time"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/money"
)

const oneDayThreshold = 8 * time.Hour

// candidate is one trip's claim to a date's meal allowance.
type candidate struct {
	trip    *Trip
	typ     DayType
	loc     Location
	absence time.Duration // one-day absences only
}

// tripDays describes the dates a trip touches and its candidates.
type tripDays struct {
	dates      []Date
	candidates map[Date]candidate
	locations  map[Date]Location
	absence    map[Date]time.Duration
	notes      map[Date][]string
}

// dateEntitlement is the per-user result for one calendar date (C06).
type dateEntitlement struct {
	typ        DayType
	loc        Location
	rate       *MealRate
	base       Cents
	deductions []Deduction
	after      Cents
	excluded   bool
	frozen     Cents
	target     *Trip
	remainder  Cents
	rules      []string
}

// mealsFor derives every trip's meal-allowance days.
func (c *computer) meals() {
	days := make(map[*Trip]*tripDays)
	byDate := make(map[Date][]candidate)
	touching := make(map[Date][]*Trip)
	for i := range c.in.Trips {
		t := &c.in.Trips[i]
		td := c.deriveDays(t)
		if td == nil {
			continue
		}
		days[t] = td
		for _, d := range td.dates {
			touching[d] = append(touching[d], t)
		}
		for d, cand := range td.candidates {
			byDate[d] = append(byDate[d], cand)
		}
	}

	entitlements := make(map[Date]*dateEntitlement)
	for d, cands := range byDate {
		entitlements[d] = c.entitle(d, cands, touching[d])
	}

	for i := range c.in.Trips {
		t := &c.in.Trips[i]
		td := days[t]
		if td == nil {
			continue
		}
		res := c.trip(t.ID)
		for _, d := range td.dates {
			res.Days = append(res.Days, c.dayResult(t, td, d, entitlements[d]))
		}
	}
}

// deriveDays computes dates, day types and locations for one trip.
func (c *computer) deriveDays(t *Trip) *tripDays {
	if !t.End.After(t.Start) {
		c.issue(Issue{Severity: Blocker, RuleID: RuleChronology, Code: "trip.end_before_start", TripID: t.ID})
		return nil
	}
	first, last := DateOf(t.Start), DateOf(t.End)
	td := &tripDays{
		dates:      datesBetween(first, last),
		candidates: make(map[Date]candidate),
		locations:  make(map[Date]Location),
		absence:    make(map[Date]time.Duration),
		notes:      make(map[Date][]string),
	}
	for i, d := range td.dates {
		td.locations[d] = c.location(t, d, i == len(td.dates)-1)
	}

	if overnight(t, td.dates) {
		for i, d := range td.dates {
			typ := DayFull
			if i == 0 || i == len(td.dates)-1 {
				typ = DayPartial
			}
			td.candidates[d] = candidate{trip: t, typ: typ, loc: td.locations[d]}
		}
		return td
	}

	// Absence without an overnight stay (C05): one partial allowance at most,
	// assigned to the date with the greater absence; ties go to the later date.
	zone := t.Start.Location()
	var best Date
	var bestDur time.Duration
	for _, d := range td.dates {
		from := maxTime(t.Start, d.Midnight(zone))
		to := minTime(t.End, d.AddDays(1).Midnight(zone))
		dur := to.Sub(from)
		td.absence[d] = dur
		if dur >= bestDur {
			best, bestDur = d, dur
		}
	}
	total := t.End.Sub(t.Start)
	td.candidates[best] = candidate{trip: t, typ: DayOneDay, loc: td.locations[best], absence: total}
	for _, d := range td.dates {
		if d != best {
			td.notes[d] = append(td.notes[d], "midnight_assigned_to:"+best.String())
		}
	}
	return td
}

// overnight reports whether the trip includes an overnight stay. Unset nights
// count as overnight; the lodging check reports them as blockers.
func overnight(t *Trip, dates []Date) bool {
	if len(dates) < 2 {
		return false
	}
	if len(dates) > 2 {
		return true
	}
	return t.Nights[dates[0]] != NightNone
}

// location selects the rate location for a date (C08, C09).
func (c *computer) location(t *Trip, d Date, last bool) Location {
	if in, ok := t.Days[d]; ok && in.LocationOverride != nil {
		return *in.LocationOverride
	}
	stops := sortedStops(t.Stops)
	zone := t.Start.Location()
	for _, s := range stops {
		if s.Arrival.Before(d.Midnight(zone)) {
			zone = s.Arrival.Location()
		}
	}
	instant := minTime(t.End, d.AddDays(1).Midnight(zone))
	loc := t.Origin
	if loc.Country == "" {
		loc = Domestic
	}
	origin := loc
	for _, s := range stops {
		if !s.Arrival.After(instant) {
			loc = s.Location
		}
	}
	if last && loc.Country == origin.Country {
		// Returning home from abroad: the last foreign business location applies.
		for i := len(stops) - 1; i >= 0; i-- {
			s := stops[i]
			if !s.Arrival.After(t.End) && s.Business && s.Location.Country != origin.Country {
				return s.Location
			}
		}
	}
	return loc
}

// entitle computes the single meal allowance for a date across all trips.
func (c *computer) entitle(d Date, cands []candidate, touching []*Trip) *dateEntitlement {
	e := &dateEntitlement{typ: DayNone, rules: []string{RuleMealBase}}

	// Aggregate one-day absences (C06).
	var oneDay time.Duration
	var oneDayCands []candidate
	for _, cd := range cands {
		if cd.typ == DayOneDay {
			oneDay += cd.absence
			oneDayCands = append(oneDayCands, cd)
		}
	}

	type option struct {
		typ    DayType
		loc    Location
		rate   MealRate
		amount Cents
		end    time.Time
	}
	var best *option
	consider := func(o option) {
		if best == nil || o.amount > best.amount || (o.amount == best.amount && o.end.After(best.end)) {
			best = &o
		}
	}
	for _, cd := range cands {
		if cd.typ == DayOneDay {
			continue
		}
		rate, ok := c.mealRate(d, cd.loc, cd.trip.ID)
		if !ok {
			continue
		}
		amount := rate.Full
		if cd.typ == DayPartial {
			amount = rate.Partial
		}
		consider(option{cd.typ, cd.loc, rate, amount, cd.trip.End})
	}
	if oneDay > oneDayThreshold {
		for _, cd := range oneDayCands {
			rate, ok := c.mealRate(d, cd.loc, cd.trip.ID)
			if !ok {
				continue
			}
			consider(option{DayOneDay, cd.loc, rate, rate.Partial, cd.trip.End})
		}
	} else if len(oneDayCands) > 0 && best == nil {
		// Below the threshold: keep the location for display.
		e.loc = oneDayCands[0].loc
	}
	if len(cands) > 1 {
		e.rules = append(e.rules, RuleDailyCap)
	}

	if best != nil {
		rate := best.rate
		e.typ, e.loc, e.rate, e.base = best.typ, best.loc, &rate, best.amount
	}

	// Meal deductions (C10): provided flags combine once per meal; each
	// contribution counts once.
	var provided Meals
	var contrib Contributions
	for _, t := range touching {
		in, ok := t.Days[d]
		if !ok {
			continue
		}
		provided.Breakfast = provided.Breakfast || in.Provided.Breakfast
		provided.Lunch = provided.Lunch || in.Provided.Lunch
		provided.Dinner = provided.Dinner || in.Provided.Dinner
		contrib.Breakfast = max(contrib.Breakfast, in.Contributions.Breakfast)
		contrib.Lunch = max(contrib.Lunch, in.Contributions.Lunch)
		contrib.Dinner = max(contrib.Dinner, in.Contributions.Dinner)
		if in.Excluded {
			e.excluded = true
		}
	}
	e.after = e.base
	if e.rate != nil && e.base > 0 {
		add := func(meal string, on bool, pct int, paid Cents) {
			if !on {
				return
			}
			gross := money.Percent(e.rate.Full, int64(pct)*100)
			amount := max(gross-paid, 0)
			e.deductions = append(e.deductions, Deduction{Meal: meal, Percent: pct, Gross: gross, Contribution: paid, Amount: amount})
			e.after -= amount
		}
		add("breakfast", provided.Breakfast, 20, contrib.Breakfast)
		add("lunch", provided.Lunch, 40, contrib.Lunch)
		add("dinner", provided.Dinner, 40, contrib.Dinner)
		e.after = max(e.after, 0)
		if len(e.deductions) > 0 {
			e.rules = append(e.rules, RuleMealDeduction)
		}
	} else if provided.Breakfast || provided.Lunch || provided.Dinner {
		c.issue(Issue{Severity: Notice, RuleID: RuleMealDeduction, Code: "meal.provided_without_allowance", TripID: touching[0].ID, Date: d.String()})
	}
	if e.excluded {
		e.after = 0
		e.rules = append(e.rules, RuleExclusion)
	}

	// Frozen amounts from submitted claims (C07).
	var editable []*Trip
	for _, cd := range cands {
		if cd.trip.Locked {
			continue
		}
		editable = append(editable, cd.trip)
	}
	frozenClaims := map[string]bool{}
	for _, t := range touching {
		if t.Locked {
			e.frozen += t.FrozenMeals[d]
			if t.FrozenMeals[d] > 0 {
				frozenClaims[t.FrozenClaim] = true
			}
		}
	}
	e.remainder = e.after - e.frozen
	if e.frozen > 0 {
		e.rules = append(e.rules, RuleFrozen)
	}
	if e.remainder < 0 {
		for claim := range frozenClaims {
			for _, t := range editable {
				c.issue(Issue{Severity: Blocker, RuleID: RuleFrozen, Code: "meal.overallocated", TripID: t.ID, Date: d.String(), Params: map[string]string{"claim": claim, "frozen": e.frozen.String(), "entitlement": e.after.String()}})
			}
		}
		e.remainder = 0
	}
	// Attribute the remainder to the editable trip ending latest (C06).
	sort.SliceStable(editable, func(i, j int) bool {
		if !editable[i].End.Equal(editable[j].End) {
			return editable[i].End.After(editable[j].End)
		}
		return editable[i].ID > editable[j].ID
	})
	if len(editable) > 0 {
		e.target = editable[0]
	}
	return e
}

func (c *computer) mealRate(d Date, loc Location, tripID int64) (MealRate, bool) {
	rate, err := c.rates.Meal(d.Y, loc)
	if err != nil {
		c.issue(Issue{Severity: Blocker, RuleID: RuleRates, Code: "rate.missing", TripID: tripID, Date: d.String(), Params: map[string]string{"country": loc.Country, "city": loc.City, "year": itoa(d.Y)}})
		return MealRate{}, false
	}
	return rate, true
}

// dayResult renders one trip's view of a date.
func (c *computer) dayResult(t *Trip, td *tripDays, d Date, e *dateEntitlement) Day {
	day := Day{Date: d, Type: DayNone, Location: td.locations[d], AbsenceMinutes: int64(td.absence[d] / time.Minute), Notes: td.notes[d], RuleIDs: []string{RuleMealBase}}
	cand, has := td.candidates[d]
	if !has || e == nil {
		if len(td.notes[d]) > 0 {
			day.RuleIDs = append(day.RuleIDs, RuleMidnight)
		}
		if t.Locked {
			day.Payable = t.FrozenMeals[d]
		}
		return day
	}
	if cand.typ == DayOneDay {
		day.AbsenceMinutes = int64(cand.absence / time.Minute)
	}
	day.RuleIDs = append([]string(nil), e.rules...)
	day.Location = e.loc
	if e.rate != nil {
		ref := e.rate.Ref
		day.Rate = &ref
		day.FullRate = e.rate.Full
	}
	day.Type = e.typ
	day.Base = e.base
	day.Deductions = e.deductions
	day.AfterDeduction = e.after
	day.Excluded = e.excluded
	if e.typ == DayNone {
		day.Base, day.AfterDeduction = 0, 0
	}
	if t.Locked {
		day.Payable = t.FrozenMeals[d]
		return day
	}
	day.FrozenElsewhere = e.frozen
	if e.target == t {
		day.Payable = e.remainder
	} else if e.target != nil && e.after > 0 {
		day.AllocatedTo = e.target.ID
		day.Type = DayAssigned
	}
	return day
}

func sortedStops(stops []Stop) []Stop {
	out := append([]Stop(nil), stops...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Arrival.Before(out[j].Arrival) })
	return out
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
