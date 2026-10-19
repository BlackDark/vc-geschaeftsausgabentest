package export

import (
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
)

// SchemaVersion of the snapshot JSON in exports.
const SchemaVersion = 1

// Snapshot freezes everything a submitted claim version needs: inputs,
// rules, rates, results, evidence references and user/employer details (P14).
type Snapshot struct {
	SchemaVersion int              `json:"schemaVersion"`
	App           string           `json:"app"`
	Locale        string           `json:"locale"`
	Claim         ClaimInfo        `json:"claim"`
	User          Person           `json:"user"`
	Employer      Party            `json:"employer"`
	Trips         []Trip           `json:"trips"`
	Outlays       []Expense        `json:"outlays"`
	Advances      []Advance        `json:"advances"`
	Totals        calc.ClaimTotals `json:"totals"`
	Issues        []calc.Issue     `json:"issues"`
	Acknowledged  bool             `json:"warningsAcknowledged"`
	Receipts      []Receipt        `json:"receipts"`
	Rates         []calc.RateRef   `json:"rates"`
	GeneratedAt   string           `json:"generatedAt"`
}

// ClaimInfo identifies the claim version.
type ClaimInfo struct {
	ID          int64  `json:"id"`
	Number      string `json:"number"`
	Version     int    `json:"version"`
	Title       string `json:"title"`
	PeriodStart string `json:"periodStart"`
	PeriodEnd   string `json:"periodEnd"`
	SubmittedAt string `json:"submittedAt"`
	Previous    string `json:"previousUnlockReason,omitempty"`
}

// Person is the claimant.
type Person struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	EmployeeNumber string `json:"employeeNumber"`
	Address        string `json:"address"`
	IBAN           string `json:"iban"`
}

// Party is the employer.
type Party struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Stop is a frozen itinerary stop.
type Stop struct {
	Arrival  string        `json:"arrival"`
	TZ       string        `json:"tz"`
	Location calc.Location `json:"location"`
	Business bool          `json:"business"`
}

// Trip is a frozen trip with its calculation.
type Trip struct {
	ID          int64           `json:"id"`
	Purpose     string          `json:"purpose"`
	Destination string          `json:"destination"`
	Start       string          `json:"start"`
	StartTZ     string          `json:"startTz"`
	End         string          `json:"end"`
	EndTZ       string          `json:"endTz"`
	Project     string          `json:"project"`
	Notes       string          `json:"notes"`
	Stops       []Stop          `json:"stops"`
	Days        []DayInput      `json:"dayInputs"`
	Expenses    []Expense       `json:"expenses"`
	Result      calc.TripResult `json:"result"`
}

// DayInput is a frozen per-day input.
type DayInput struct {
	Date   string        `json:"date"`
	Input  calc.DayInput `json:"input"`
	Origin string        `json:"breakfastOrigin,omitempty"`
}

// Expense is a frozen expense with its result and evidence.
type Expense struct {
	ID          int64              `json:"id"`
	TripID      int64              `json:"tripId,omitempty"`
	Date        string             `json:"date"`
	CostType    calc.CostType      `json:"costType"`
	Payer       calc.Payer         `json:"payer"`
	Supplier    string             `json:"supplier"`
	Description string             `json:"description"`
	Currency    string             `json:"currency"`
	GrossMinor  int64              `json:"grossMinor"`
	Exponent    int                `json:"exponent"`
	FXRate      string             `json:"fxRate,omitempty"`
	FXDate      string             `json:"fxDate,omitempty"`
	EURDebit    *int64             `json:"eurDebit,omitempty"`
	VATUnknown  bool               `json:"vatUnknown"`
	VAT         []calc.VATPortion  `json:"vat"`
	Nights      []string           `json:"nights"`
	Hospitality *calc.Hospitality  `json:"hospitality,omitempty"`
	Receipts    []string           `json:"receipts"`
	Result      calc.ExpenseResult `json:"result"`
}

// Advance is a frozen advance.
type Advance struct {
	ID     int64      `json:"id"`
	Date   string     `json:"date"`
	Amount calc.Cents `json:"amount"`
	Note   string     `json:"note"`
}

// Receipt is evidence referenced by the claim.
type Receipt struct {
	ID         int64  `json:"id"`
	Number     string `json:"number"`
	Kind       string `json:"kind"`
	Filename   string `json:"filename"`
	Mime       string `json:"mime"`
	Pages      int    `json:"pages"`
	SHA256     string `json:"sha256"`
	ZipPath    string `json:"zipPath"`
	StorageKey string `json:"-"`
	SelfIssued any    `json:"selfIssued,omitempty"`
}
