package app

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
)

// LocalTime is "YYYY-MM-DDTHH:MM" in the accompanying IANA zone.
const LocalTime = "2006-01-02T15:04"

// StopDTO is a recorded arrival.
type StopDTO struct {
	Arrival  string `json:"arrival" doc:"local time YYYY-MM-DDTHH:MM"`
	TZ       string `json:"tz"`
	Country  string `json:"country" minLength:"2" maxLength:"2"`
	City     string `json:"city,omitempty"`
	Business bool   `json:"business"`
}

// DayDTO holds per-day meal inputs.
type DayDTO struct {
	Date             string `json:"date" format:"date"`
	Breakfast        bool   `json:"breakfast"`
	Lunch            bool   `json:"lunch"`
	Dinner           bool   `json:"dinner"`
	ContribBreakfast int64  `json:"contribBreakfast" minimum:"0"`
	ContribLunch     int64  `json:"contribLunch" minimum:"0"`
	ContribDinner    int64  `json:"contribDinner" minimum:"0"`
	BreakfastOrigin  string `json:"breakfastOrigin,omitempty" doc:"set when the flag was suggested from a hotel invoice"`
	OverrideCountry  string `json:"overrideCountry,omitempty"`
	OverrideCity     string `json:"overrideCity,omitempty"`
	OverrideReason   string `json:"overrideReason,omitempty"`
	Excluded         bool   `json:"excluded"`
	ExcludeReason    string `json:"excludeReason,omitempty"`
}

// NightDTO is the lodging arrangement for one night.
type NightDTO struct {
	Date string         `json:"date" format:"date"`
	Kind calc.NightKind `json:"kind" enum:"private_actual,private_allowance,private_none,employer,in_transit,none"`
}

// JourneyDTO is a private-vehicle journey.
type JourneyDTO struct {
	Date           string       `json:"date" format:"date"`
	Vehicle        calc.Vehicle `json:"vehicle" enum:"car,other_motor_vehicle"`
	DistanceMeters int64        `json:"distanceMeters" minimum:"1"`
	Description    string       `json:"description,omitempty"`
}

// TripInput creates or replaces a trip and its day, night, stop and
// journey records.
type TripInput struct {
	Purpose     string       `json:"purpose" minLength:"1" maxLength:"300"`
	Destination string       `json:"destination,omitempty" maxLength:"300"`
	Start       string       `json:"start" doc:"local time YYYY-MM-DDTHH:MM"`
	StartTZ     string       `json:"startTz"`
	End         string       `json:"end" doc:"local time YYYY-MM-DDTHH:MM"`
	EndTZ       string       `json:"endTz"`
	Project     string       `json:"project,omitempty" maxLength:"200"`
	Notes       string       `json:"notes,omitempty" maxLength:"2000"`
	Stops       []StopDTO    `json:"stops,omitempty"`
	Days        []DayDTO     `json:"days,omitempty"`
	Nights      []NightDTO   `json:"nights,omitempty"`
	Journeys    []JourneyDTO `json:"journeys,omitempty"`
	Version     int          `json:"version,omitempty" doc:"required for updates (optimistic concurrency)"`
}

// Trip is a stored trip.
type Trip struct {
	TripInput
	ID         int64  `json:"id"`
	EmployerID int64  `json:"employerId"`
	ClaimID    *int64 `json:"claimId"`
	ClaimLabel string `json:"claimLabel,omitempty"`
	Locked     bool   `json:"locked"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

func parseLocal(s, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		return time.Time{}, Invalid("time.zone_invalid", "unknown time zone", "zone", tz)
	}
	t, err := time.ParseInLocation(LocalTime, s, loc)
	if err != nil {
		return time.Time{}, Invalid("time.invalid", "invalid local time", "value", s)
	}
	return t, nil
}

func validDate(s string) error {
	if _, err := calc.ParseDate(s); err != nil {
		return Invalid("date.invalid", "invalid date", "value", s)
	}
	return nil
}

// normalize validates a trip input and returns its instants.
func (in *TripInput) normalize() (time.Time, time.Time, error) {
	in.Purpose = strings.TrimSpace(in.Purpose)
	if in.Purpose == "" {
		return time.Time{}, time.Time{}, Invalid("trip.purpose_required", "purpose is required")
	}
	start, err := parseLocal(in.Start, in.StartTZ)
	if err != nil {
		return start, start, err
	}
	end, err := parseLocal(in.End, in.EndTZ)
	if err != nil {
		return start, end, err
	}
	if !end.After(start) {
		return start, end, Invalid("trip.end_before_start", "end must be after start")
	}
	first, last := calc.DateOf(start), calc.DateOf(end)
	inRange := func(s string) bool {
		d, err := calc.ParseDate(s)
		return err == nil && !d.Before(first) && !last.Before(d)
	}
	for _, d := range in.Days {
		if !inRange(d.Date) {
			return start, end, Invalid("trip.day_outside", "day outside the trip", "date", d.Date)
		}
		if d.OverrideCountry != "" && strings.TrimSpace(d.OverrideReason) == "" {
			return start, end, Invalid("trip.override_reason", "a location override needs a reason", "date", d.Date)
		}
		if d.Excluded && strings.TrimSpace(d.ExcludeReason) == "" {
			return start, end, Invalid("trip.exclude_reason", "an exclusion needs a reason", "date", d.Date)
		}
	}
	for _, n := range in.Nights {
		if !inRange(n.Date) || n.Date == last.String() {
			return start, end, Invalid("trip.night_outside", "night outside the trip", "date", n.Date)
		}
	}
	for _, s := range in.Stops {
		at, err := parseLocal(s.Arrival, s.TZ)
		if err != nil {
			return start, end, err
		}
		if at.Before(start) || at.After(end) {
			return start, end, Invalid("trip.stop_outside", "stop outside the trip", "arrival", s.Arrival)
		}
	}
	for _, j := range in.Journeys {
		if err := validDate(j.Date); err != nil {
			return start, end, err
		}
		if j.DistanceMeters <= 0 {
			return start, end, Invalid("mileage.distance_invalid", "distance must be positive")
		}
	}
	return start, end, nil
}

// tripLocked reports whether a trip belongs to a submitted or paid claim.
func tripLocked(ctx context.Context, q querier, tripID int64) (bool, error) {
	var status sql.NullString
	err := q.QueryRowContext(ctx, `SELECT c.status FROM trips t LEFT JOIN claims c ON c.id = t.claim_id WHERE t.id = ?`, tripID).Scan(&status)
	if err != nil {
		return false, notFound(err)
	}
	return status.Valid && status.String != "draft", nil
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// CreateTrip records a new trip for the user's employer.
func (a *App) CreateTrip(ctx context.Context, u *User, in TripInput) (*Trip, error) {
	start, end, err := in.normalize()
	if err != nil {
		return nil, err
	}
	emp, err := requireEmployer(ctx, a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	var id int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if err := checkOverlap(ctx, tx, u.ID, 0, start, end); err != nil {
			return err
		}
		now := a.stamp()
		res, err := tx.ExecContext(ctx, `INSERT INTO trips(user_id, employer_id, purpose, destination, start_at, start_tz, end_at, end_tz, project, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, emp.ID, in.Purpose, in.Destination, start.UTC().Format(time.RFC3339), in.StartTZ, end.UTC().Format(time.RFC3339), in.EndTZ, in.Project, in.Notes, now, now)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		if err := writeTripChildren(ctx, tx, id, in); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "trip", id, "create", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetTrip(ctx, u, id)
}

// UpdateTrip replaces a trip's data; locked trips cannot change (P15).
func (a *App) UpdateTrip(ctx context.Context, u *User, id int64, in TripInput) (*Trip, error) {
	start, end, err := in.normalize()
	if err != nil {
		return nil, err
	}
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if err := a.editableTrip(ctx, tx, u.ID, id, in.Version); err != nil {
			return err
		}
		if err := checkOverlap(ctx, tx, u.ID, id, start, end); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE trips SET purpose = ?, destination = ?, start_at = ?, start_tz = ?, end_at = ?, end_tz = ?, project = ?, notes = ?, version = version + 1, updated_at = ? WHERE id = ?`,
			in.Purpose, in.Destination, start.UTC().Format(time.RFC3339), in.StartTZ, end.UTC().Format(time.RFC3339), in.EndTZ, in.Project, in.Notes, a.stamp(), id); err != nil {
			return err
		}
		for _, t := range []string{"trip_stops", "trip_days", "trip_nights", "journeys"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE trip_id = ?`, id); err != nil {
				return err
			}
		}
		if err := writeTripChildren(ctx, tx, id, in); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "trip", id, "update", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetTrip(ctx, u, id)
}

// DeleteTrip removes an unclaimed trip and its expenses.
func (a *App) DeleteTrip(ctx context.Context, u *User, id int64) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		if err := a.editableTrip(ctx, tx, u.ID, id, 0); err != nil {
			return err
		}
		var claim sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT claim_id FROM trips WHERE id = ?`, id).Scan(&claim); err != nil {
			return err
		}
		if claim.Valid {
			return Invalid("trip.in_claim", "remove the trip from its claim first")
		}
		var frozen int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM frozen_days WHERE trip_id = ?`, id).Scan(&frozen); err != nil {
			return err
		}
		if frozen > 0 {
			return ErrLocked
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM receipt_links WHERE expense_id IN (SELECT id FROM expenses WHERE trip_id = ?)`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM expenses WHERE trip_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM trips WHERE id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "trip", id, "delete", "", nil)
	})
}

// editableTrip checks ownership, lock state and optionally the version.
func (a *App) editableTrip(ctx context.Context, q querier, userID, id int64, version int) error {
	var owner int64
	var current int
	if err := q.QueryRowContext(ctx, `SELECT user_id, version FROM trips WHERE id = ?`, id).Scan(&owner, &current); err != nil || owner != userID {
		return ErrNotFound
	}
	locked, err := tripLocked(ctx, q, id)
	if err != nil {
		return err
	}
	if locked {
		return ErrLocked
	}
	if version != 0 && version != current {
		return ErrConflict
	}
	return nil
}

// checkOverlap rejects overlapping trips; touching intervals are allowed (C02).
func checkOverlap(ctx context.Context, tx *sql.Tx, userID, self int64, start, end time.Time) error {
	var other int64
	var purpose string
	err := tx.QueryRowContext(ctx, `SELECT id, purpose FROM trips WHERE user_id = ? AND id <> ? AND start_at < ? AND end_at > ? LIMIT 1`,
		userID, self, end.UTC().Format(time.RFC3339), start.UTC().Format(time.RFC3339)).Scan(&other, &purpose)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return Invalid("trip.overlap", "the trip overlaps another trip", "trip", purpose)
}

func writeTripChildren(ctx context.Context, tx *sql.Tx, id int64, in TripInput) error {
	for _, s := range in.Stops {
		at, _ := parseLocal(s.Arrival, s.TZ)
		if _, err := tx.ExecContext(ctx, `INSERT INTO trip_stops(trip_id, arrival_at, tz, country, city, business) VALUES (?, ?, ?, ?, ?, ?)`,
			id, at.UTC().Format(time.RFC3339), s.TZ, strings.ToUpper(s.Country), s.City, boolInt(s.Business)); err != nil {
			return err
		}
	}
	for _, d := range in.Days {
		var oc, ocity any
		if d.OverrideCountry != "" {
			oc, ocity = strings.ToUpper(d.OverrideCountry), d.OverrideCity
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO trip_days(trip_id, date, breakfast, lunch, dinner, contrib_breakfast, contrib_lunch, contrib_dinner, breakfast_origin, override_country, override_city, override_reason, excluded, exclude_reason) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, d.Date, boolInt(d.Breakfast), boolInt(d.Lunch), boolInt(d.Dinner), d.ContribBreakfast, d.ContribLunch, d.ContribDinner, d.BreakfastOrigin, oc, ocity, d.OverrideReason, boolInt(d.Excluded), d.ExcludeReason); err != nil {
			return err
		}
	}
	for _, n := range in.Nights {
		if _, err := tx.ExecContext(ctx, `INSERT INTO trip_nights(trip_id, date, kind) VALUES (?, ?, ?)`, id, n.Date, n.Kind); err != nil {
			return err
		}
	}
	for _, j := range in.Journeys {
		if _, err := tx.ExecContext(ctx, `INSERT INTO journeys(trip_id, date, vehicle, distance_m, description) VALUES (?, ?, ?, ?, ?)`, id, j.Date, j.Vehicle, j.DistanceMeters, j.Description); err != nil {
			return err
		}
	}
	return nil
}

// GetTrip loads one owned trip.
func (a *App) GetTrip(ctx context.Context, u *User, id int64) (*Trip, error) {
	trips, err := a.loadTrips(ctx, a.DB, u.ID, "t.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(trips) == 0 {
		return nil, ErrNotFound
	}
	return trips[0], nil
}

// ListTrips lists the user's trips, newest first.
func (a *App) ListTrips(ctx context.Context, u *User) ([]*Trip, error) {
	return a.loadTrips(ctx, a.DB, u.ID, "1 = 1")
}

func (a *App) loadTrips(ctx context.Context, q querier, userID int64, where string, args ...any) ([]*Trip, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.id, t.employer_id, t.claim_id, coalesce(c.number, c.title, ''), coalesce(c.status, ''), t.purpose, t.destination, t.start_at, t.start_tz, t.end_at, t.end_tz, t.project, t.notes, t.version
		FROM trips t LEFT JOIN claims c ON c.id = t.claim_id WHERE t.user_id = ? AND `+where+` ORDER BY t.start_at DESC`, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	var out []*Trip
	for rows.Next() {
		t := &Trip{}
		var startAt, endAt, status string
		if err := rows.Scan(&t.ID, &t.EmployerID, &t.ClaimID, &t.ClaimLabel, &status, &t.Purpose, &t.Destination, &startAt, &t.StartTZ, &endAt, &t.EndTZ, &t.Project, &t.Notes, &t.Version); err != nil {
			rows.Close()
			return nil, err
		}
		t.Locked = status == "submitted" || status == "paid"
		t.Start, t.StartDate = localString(startAt, t.StartTZ)
		t.End, t.EndDate = localString(endAt, t.EndTZ)
		out = append(out, t)
	}
	rows.Close()
	for _, t := range out {
		if err := loadTripChildren(ctx, q, t); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func localString(utc, tz string) (string, string) {
	t, _ := time.Parse(time.RFC3339, utc)
	loc, err := time.LoadLocation(tz)
	if err == nil {
		t = t.In(loc)
	}
	return t.Format(LocalTime), t.Format(time.DateOnly)
}

func loadTripChildren(ctx context.Context, q querier, t *Trip) error {
	t.Stops, t.Days, t.Nights, t.Journeys = []StopDTO{}, []DayDTO{}, []NightDTO{}, []JourneyDTO{}
	rows, err := q.QueryContext(ctx, `SELECT arrival_at, tz, country, city, business FROM trip_stops WHERE trip_id = ? ORDER BY arrival_at`, t.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var s StopDTO
		var at string
		if err := rows.Scan(&at, &s.TZ, &s.Country, &s.City, &s.Business); err != nil {
			rows.Close()
			return err
		}
		s.Arrival, _ = localString(at, s.TZ)
		t.Stops = append(t.Stops, s)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT date, breakfast, lunch, dinner, contrib_breakfast, contrib_lunch, contrib_dinner, breakfast_origin, coalesce(override_country, ''), coalesce(override_city, ''), override_reason, excluded, exclude_reason FROM trip_days WHERE trip_id = ? ORDER BY date`, t.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var d DayDTO
		if err := rows.Scan(&d.Date, &d.Breakfast, &d.Lunch, &d.Dinner, &d.ContribBreakfast, &d.ContribLunch, &d.ContribDinner, &d.BreakfastOrigin, &d.OverrideCountry, &d.OverrideCity, &d.OverrideReason, &d.Excluded, &d.ExcludeReason); err != nil {
			rows.Close()
			return err
		}
		t.Days = append(t.Days, d)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT date, kind FROM trip_nights WHERE trip_id = ? ORDER BY date`, t.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n NightDTO
		if err := rows.Scan(&n.Date, &n.Kind); err != nil {
			rows.Close()
			return err
		}
		t.Nights = append(t.Nights, n)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT date, vehicle, distance_m, description FROM journeys WHERE trip_id = ? ORDER BY date, id`, t.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var j JourneyDTO
		if err := rows.Scan(&j.Date, &j.Vehicle, &j.DistanceMeters, &j.Description); err != nil {
			return err
		}
		t.Journeys = append(t.Journeys, j)
	}
	return rows.Err()
}
