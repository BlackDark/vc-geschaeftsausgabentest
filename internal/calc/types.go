// Package calc is the single authoritative calculation module. It is pure:
// no database, network or clock access. Every result line carries a stable
// rule ID from docs/CALCULATION_RULES.md.
package calc

import (
	"time"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/money"
)

// Cents re-exports money.Cents for brevity.
type Cents = money.Cents

// Location is a rate location: an ISO country code and an optional city
// as named in the annual rate table.
type Location struct {
	Country string `json:"country"`
	City    string `json:"city,omitempty"`
}

// Domestic is the home rate location.
var Domestic = Location{Country: "DE"}

// RateRef identifies the rate data a line used.
type RateRef struct {
	Table    string   `json:"table"`
	Year     int      `json:"year"`
	Location Location `json:"location"`
	Source   string   `json:"source"`
	Revision int      `json:"revision"`
}

// MealRate holds the annual meal and lodging allowances for a location.
type MealRate struct {
	Ref     RateRef
	Full    Cents
	Partial Cents
	Lodging Cents
}

// Vehicle classifies a private vehicle for mileage.
type Vehicle string

const (
	VehicleCar   Vehicle = "car"
	VehicleOther Vehicle = "other_motor_vehicle"
)

// MileageRate is cents per kilometre for a vehicle class.
type MileageRate struct {
	Ref        RateRef
	CentsPerKm int64
}

// Rates looks up annual rate data. Missing data returns an error, which the
// calculation turns into a blocker (C04).
type Rates interface {
	Meal(year int, loc Location) (MealRate, error)
	Mileage(d Date, v Vehicle) (MileageRate, error)
}

// Stop is a recorded arrival during a trip (Ortswechsel).
type Stop struct {
	Arrival  time.Time `json:"arrival"`
	Location Location  `json:"location"`
	Business bool      `json:"business"`
}

// Meals flags provided breakfast, lunch and dinner.
type Meals struct {
	Breakfast bool `json:"breakfast"`
	Lunch     bool `json:"lunch"`
	Dinner    bool `json:"dinner"`
}

// Contributions are the user's own payments towards provided meals.
type Contributions struct {
	Breakfast Cents `json:"breakfast"`
	Lunch     Cents `json:"lunch"`
	Dinner    Cents `json:"dinner"`
}

// DayInput holds user decisions for one travel day.
type DayInput struct {
	Provided         Meals         `json:"provided"`
	Contributions    Contributions `json:"contributions"`
	LocationOverride *Location     `json:"locationOverride,omitempty"`
	OverrideReason   string        `json:"overrideReason,omitempty"`
	Excluded         bool          `json:"excluded"`
	ExcludeReason    string        `json:"excludeReason,omitempty"`
}

// NightKind is the lodging arrangement for the night after a travel day.
type NightKind string

const (
	NightUnset            NightKind = ""
	NightPrivateActual    NightKind = "private_actual"    // personally paid, reimburse actual cost
	NightPrivateAllowance NightKind = "private_allowance" // personally paid or free, claim lodging allowance
	NightPrivateNone      NightKind = "private_none"      // personally paid, no reimbursement
	NightEmployer         NightKind = "employer"          // company-paid or provided lodging
	NightInTransit        NightKind = "in_transit"        // night train, flight, ferry
	NightNone             NightKind = "none"              // no overnight stay away from home
)

// Payer records who paid a charge.
type Payer string

const (
	PayerUser     Payer = "user"
	PayerEmployer Payer = "employer"
)

// CostType classifies an expense.
type CostType string

const (
	CostTransport        CostType = "transport"
	CostMeals            CostType = "meals"
	CostLodging          CostType = "lodging"
	CostIncidental       CostType = "incidental"
	CostHospitality      CostType = "hospitality"
	CostToolsEquipment   CostType = "tools_equipment"
	CostSoftwareServices CostType = "software_services"
	CostOtherOutlay      CostType = "other_outlay"
)

// IsTravel reports whether the cost type belongs to a trip.
func (c CostType) IsTravel() bool {
	switch c {
	case CostTransport, CostMeals, CostLodging, CostIncidental, CostHospitality:
		return true
	}
	return false
}

// IsOutlay reports whether the cost type is a standalone outlay category.
func (c CostType) IsOutlay() bool {
	switch c {
	case CostToolsEquipment, CostSoftwareServices, CostOtherOutlay:
		return true
	}
	return false
}

// VATPortion is a part of an expense with one VAT rate. Amounts are in the
// expense currency's minor units. Net and VAT are optional invoice-stated
// values; when absent they are derived from the rate (C16).
type VATPortion struct {
	GrossMinor int64  `json:"grossMinor"`
	RateBP     int64  `json:"rateBp"`
	Country    string `json:"country" required:"false"`
	StatedNet  *int64 `json:"statedNet,omitempty"`
	StatedVAT  *int64 `json:"statedVat,omitempty"`
}

// Hospitality documents business entertainment (P12).
type Hospitality struct {
	Occasion      string   `json:"occasion"`
	Place         string   `json:"place"`
	Participants  []string `json:"participants"`
	ExternalCount int      `json:"externalCount" required:"false"`
	Confirmed     bool     `json:"confirmed" required:"false"`
	TipMinor      int64    `json:"tipMinor" required:"false"`
}

// Expense is a recorded charge, trip-linked or standalone.
type Expense struct {
	ID           int64        `json:"id"`
	Date         Date         `json:"date"`
	CostType     CostType     `json:"costType"`
	Payer        Payer        `json:"payer"`
	Supplier     string       `json:"supplier"`
	Description  string       `json:"description"`
	Currency     string       `json:"currency"`
	GrossMinor   int64        `json:"grossMinor"`
	FXRate       string       `json:"fxRate,omitempty"`
	FXDate       *Date        `json:"fxDate,omitempty"`
	EURDebit     *Cents       `json:"eurDebit,omitempty"`
	VATUnknown   bool         `json:"vatUnknown"`
	VAT          []VATPortion `json:"vat,omitempty"`
	Nights       []Date       `json:"nights,omitempty"`
	Fuel         bool         `json:"fuel"`
	Hospitality  *Hospitality `json:"hospitality,omitempty"`
	Receipts     int          `json:"receipts"`
	Unconfirmed  int          `json:"unconfirmed"`
	SelfIssued   bool         `json:"selfIssued"`
	DuplicateSHA bool         `json:"duplicateSha"`
}

// Journey is a private-vehicle mileage journey with explicit total distance.
type Journey struct {
	ID             int64   `json:"id"`
	Date           Date    `json:"date"`
	Vehicle        Vehicle `json:"vehicle"`
	DistanceMeters int64   `json:"distanceMeters"`
	Description    string  `json:"description"`
}

// Trip is the calculation input for one business trip.
type Trip struct {
	ID       int64
	Start    time.Time // carries its IANA location
	End      time.Time
	Origin   Location // usually Domestic
	Stops    []Stop
	Days     map[Date]DayInput
	Nights   map[Date]NightKind
	Expenses []Expense
	Journeys []Journey
	// Locked trips belong to a submitted claim; FrozenMeals holds the meal
	// allowance per date frozen in that claim's snapshot (C07).
	Locked      bool
	FrozenMeals map[Date]Cents
	FrozenClaim string
}

// Input is everything a calculation needs for one user.
type Input struct {
	Trips   []Trip
	Outlays []Expense
}
