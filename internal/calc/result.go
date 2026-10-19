package calc

// Rule IDs. They are stable identifiers referenced by results, exports and
// tests; see docs/CALCULATION_RULES.md.
const (
	RuleMoney          = "C01"
	RuleChronology     = "C02"
	RuleRates          = "C04"
	RuleMidnight       = "C05"
	RuleDailyCap       = "C06"
	RuleFrozen         = "C07"
	RuleLocation       = "C08"
	RuleOverride       = "C09"
	RuleMealDeduction  = "C10"
	RuleExclusion      = "C11"
	RuleLodging        = "C12"
	RuleHotelBreakfast = "C13"
	RuleMileage        = "C14"
	RuleExpense        = "C15"
	RuleVAT            = "C16"
	RuleFX             = "C18"
	RuleFXPortions     = "C19"
	RuleHospitality    = "C20"
	RuleTotals         = "C21"
	RuleOutlay         = "C22"
	RuleMealBase       = "C-MEAL"
)

// Severity of an issue.
type Severity string

const (
	Blocker Severity = "blocker"
	Warning Severity = "warning"
	Notice  Severity = "notice"
)

// Issue is a blocker, warning or notice. Code is a stable message key the
// UI translates; Params fill its placeholders.
type Issue struct {
	Severity  Severity          `json:"severity"`
	RuleID    string            `json:"ruleId"`
	Code      string            `json:"code"`
	TripID    int64             `json:"tripId,omitempty"`
	ExpenseID int64             `json:"expenseId,omitempty"`
	Date      string            `json:"date,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
}

// DayType classifies a travel day for the meal allowance.
type DayType string

const (
	DayFull     DayType = "full"
	DayPartial  DayType = "partial" // arrival/departure day of an overnight trip
	DayOneDay   DayType = "one_day" // absence without overnight stay
	DayNone     DayType = "none"    // no entitlement (e.g. ≤ 8 hours)
	DayAssigned DayType = "assigned_elsewhere"
)

// Deduction is a meal deduction line (C10).
type Deduction struct {
	Meal         string `json:"meal"`
	Percent      int    `json:"percent"`
	Gross        Cents  `json:"gross"`
	Contribution Cents  `json:"contribution"`
	Amount       Cents  `json:"amount"`
}

// Day is the meal-allowance result for one trip and date.
type Day struct {
	Date            Date        `json:"date"`
	Type            DayType     `json:"type"`
	Location        Location    `json:"location"`
	AbsenceMinutes  int64       `json:"absenceMinutes"`
	Rate            *RateRef    `json:"rate,omitempty"`
	FullRate        Cents       `json:"fullRate"`
	Base            Cents       `json:"base"`
	Deductions      []Deduction `json:"deductions"`
	AfterDeduction  Cents       `json:"afterDeduction"`
	Excluded        bool        `json:"excluded"`
	FrozenElsewhere Cents       `json:"frozenElsewhere"`
	Payable         Cents       `json:"payable"`
	// AllocatedTo names the trip receiving the day's allowance when it is
	// not this trip (C06).
	AllocatedTo int64    `json:"allocatedTo,omitempty"`
	RuleIDs     []string `json:"ruleIds"`
	Notes       []string `json:"notes,omitempty"`
}

// Night is the lodging result for one night.
type Night struct {
	Date      Date      `json:"date"`
	Kind      NightKind `json:"kind"`
	Location  Location  `json:"location"`
	Rate      *RateRef  `json:"rate,omitempty"`
	Allowance Cents     `json:"allowance"`
	RuleID    string    `json:"ruleId"`
}

// JourneyResult is a mileage line (C14).
type JourneyResult struct {
	JourneyID      int64    `json:"journeyId"`
	Date           Date     `json:"date"`
	Vehicle        Vehicle  `json:"vehicle"`
	DistanceMeters int64    `json:"distanceMeters"`
	CentsPerKm     int64    `json:"centsPerKm"`
	Rate           *RateRef `json:"rate,omitempty"`
	Amount         Cents    `json:"amount"`
	RuleID         string   `json:"ruleId"`
}

// VATResult is a VAT portion converted to EUR.
type VATResult struct {
	RateBP  int64  `json:"rateBp"`
	Country string `json:"country"`
	Gross   Cents  `json:"gross"`
	Net     Cents  `json:"net"`
	VAT     Cents  `json:"vat"`
}

// ExpenseResult is the payable view of one expense.
type ExpenseResult struct {
	ExpenseID    int64    `json:"expenseId"`
	CostType     CostType `json:"costType"`
	Payer        Payer    `json:"payer"`
	GrossEUR     Cents    `json:"grossEur"`
	Reimbursable Cents    `json:"reimbursable"`
	// Reason explains a zero payout: "employer_paid", "replaced_by_allowance",
	// "not_reimbursed", "fx_missing".
	Reason  string      `json:"reason,omitempty"`
	FXRule  string      `json:"fxRule,omitempty"`
	VAT     []VATResult `json:"vat,omitempty"`
	RuleIDs []string    `json:"ruleIds"`
}

// TripTotals sums a trip's payable components once each (C21).
type TripTotals struct {
	Meals            Cents              `json:"meals"`
	LodgingAllowance Cents              `json:"lodgingAllowance"`
	Mileage          Cents              `json:"mileage"`
	Expenses         Cents              `json:"expenses"`
	ByCostType       map[CostType]Cents `json:"byCostType"`
	CompanyPaid      Cents              `json:"companyPaid"`
	Total            Cents              `json:"total"`
}

// TripResult is the full breakdown for one trip.
type TripResult struct {
	TripID   int64           `json:"tripId"`
	Days     []Day           `json:"days"`
	Nights   []Night         `json:"nights"`
	Journeys []JourneyResult `json:"journeys"`
	Expenses []ExpenseResult `json:"expenses"`
	Totals   TripTotals      `json:"totals"`
}

// Result is the output of Compute.
type Result struct {
	Trips   map[int64]*TripResult `json:"trips"`
	Outlays []ExpenseResult       `json:"outlays"`
	Issues  []Issue               `json:"issues"`
}

// IssuesFor returns the issues concerning the given trips and expenses.
func (r *Result) IssuesFor(tripIDs, expenseIDs map[int64]bool) []Issue {
	var out []Issue
	for _, is := range r.Issues {
		if (is.TripID != 0 && tripIDs[is.TripID]) || (is.TripID == 0 && is.ExpenseID != 0 && expenseIDs[is.ExpenseID]) {
			out = append(out, is)
		}
	}
	return out
}
