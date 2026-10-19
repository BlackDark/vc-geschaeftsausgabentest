// Package rates validates, seeds and looks up annual rate tables (C04, P17).
package rates

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark/vc-geschaeftsausgabentest/data"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/calc"
	"github.com/blackdark/vc-geschaeftsausgabentest/internal/money"
)

// Row is one validated meal/lodging rate.
type Row struct {
	Year    int
	Country string // ISO code
	City    string // empty for the country's general entry
	Label   string // BMF country name
	Full    money.Cents
	Partial money.Cents
	Lodging money.Cents
}

var foreignHeader = []string{"jahr", "land", "ort", "vma_24h_eur", "vma_an_abreise_8h_eur", "uebernachtung_ag_pauschal_eur"}

// generalCity is the BMF label for a country's remaining territory.
const generalCity = "im Übrigen"

// ParseCountries reads country-codes.csv (bmf_name,land_iso).
func ParseCountries(r io.Reader) (map[string]string, error) {
	recs, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 || strings.Join(recs[0], ",") != "bmf_name,land_iso" {
		return nil, errors.New("country codes: unexpected header")
	}
	out := map[string]string{}
	for i, rec := range recs[1:] {
		if len(rec) != 2 || len(rec[1]) != 2 || rec[0] == "" {
			return nil, fmt.Errorf("country codes row %d: invalid", i+2)
		}
		if _, dup := out[rec[0]]; dup {
			return nil, fmt.Errorf("country codes row %d: duplicate %q", i+2, rec[0])
		}
		out[rec[0]] = strings.ToUpper(rec[1])
	}
	return out, nil
}

// ParseForeign validates an annual foreign-rate CSV: header, year, country
// mapping, unique keys and nonnegative whole-euro amounts. Errors name the row.
func ParseForeign(r io.Reader, year int, countries map[string]string) ([]Row, error) {
	recs, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 || strings.Join(recs[0], ",") != strings.Join(foreignHeader, ",") {
		return nil, errors.New("unexpected header")
	}
	seen := map[string]bool{}
	var rows []Row
	for i, rec := range recs[1:] {
		line := i + 2
		if len(rec) != len(foreignHeader) {
			return nil, fmt.Errorf("row %d: expected %d columns", line, len(foreignHeader))
		}
		y, err := strconv.Atoi(rec[0])
		if err != nil || y != year {
			return nil, fmt.Errorf("row %d: year %q does not match %d", line, rec[0], year)
		}
		iso, ok := countries[rec[1]]
		if !ok {
			return nil, fmt.Errorf("row %d: unknown country %q", line, rec[1])
		}
		city := rec[2]
		if city == generalCity {
			city = ""
		}
		key := iso + "/" + city
		if seen[key] {
			return nil, fmt.Errorf("row %d: duplicate %s %q", line, rec[1], rec[2])
		}
		seen[key] = true
		var amounts [3]money.Cents
		for j := range amounts {
			v, err := strconv.Atoi(rec[3+j])
			if err != nil || v < 0 {
				return nil, fmt.Errorf("row %d: invalid amount %q in %s", line, rec[3+j], foreignHeader[3+j])
			}
			amounts[j] = money.Cents(v * 100)
		}
		rows = append(rows, Row{Year: y, Country: iso, City: city, Label: rec[1], Full: amounts[0], Partial: amounts[1], Lodging: amounts[2]})
	}
	return rows, nil
}

// Domestic rates (CALCULATION_RULES: meal allowance, C12, C14). Seeded per
// year; values are data, not constants in the calculation.
var domestic = map[int]struct{ full, partial, lodging money.Cents }{
	2024: {2800, 1400, 2000},
	2025: {2800, 1400, 2000},
	2026: {2800, 1400, 2000},
}

const (
	// Provenance and open verification items: docs/DECISIONS.md.
	sourceDomestic = "Inland: § 9 Abs. 4a EStG, R 9.7 LStR"
	sourceMileage  = "Kilometersätze: § 9 Abs. 1 Satz 3 Nr. 4a EStG"
)

func foreignSource(year int) string {
	return fmt.Sprintf("Ausland: BMF-Schreiben Reisekosten %d", year)
}

// Seed loads the inherited datasets when the rate tables are empty.
func Seed(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM meal_rates`).Scan(&n); err != nil || n > 0 {
		return err
	}
	f, err := data.Files.Open("country-codes.csv")
	if err != nil {
		return err
	}
	defer f.Close()
	countries, err := ParseCountries(f)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for name, iso := range countries {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO countries(iso, bmf_name) VALUES (?, ?)`, iso, name); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO countries(iso, bmf_name) VALUES ('DE', 'Deutschland')`); err != nil {
		return err
	}
	for year, d := range domestic {
		if err := insert(ctx, tx, Row{Year: year, Country: "DE", Label: "Deutschland", Full: d.full, Partial: d.partial, Lodging: d.lodging}, sourceDomestic, now); err != nil {
			return err
		}
	}
	for _, year := range []int{2024, 2025, 2026} {
		rows, err := readForeign(year, countries)
		if err != nil {
			return fmt.Errorf("foreign-rates-%d.csv: %w", year, err)
		}
		for _, r := range rows {
			if err := insert(ctx, tx, r, foreignSource(year), now); err != nil {
				return err
			}
		}
	}
	for _, m := range []struct {
		v     calc.Vehicle
		cents int64
	}{{calc.VehicleCar, 30}, {calc.VehicleOther, 20}} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mileage_rates(vehicle, valid_from, cents_per_km, source, created_at) VALUES (?, '2014-01-01', ?, ?, ?)`, m.v, m.cents, sourceMileage, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func readForeign(year int, countries map[string]string) ([]Row, error) {
	f, err := fs.ReadFile(data.Files, fmt.Sprintf("foreign-rates-%d.csv", year))
	if err != nil {
		return nil, err
	}
	return ParseForeign(strings.NewReader(string(f)), year, countries)
}

func insert(ctx context.Context, tx *sql.Tx, r Row, source, now string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO meal_rates(year, country, city, label, full_day, partial, lodging, source, revision, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		r.Year, r.Country, r.City, r.Label, r.Full, r.Partial, r.Lodging, source, now)
	return err
}

// Import validates a further year's CSV and stores it as a new revision.
func Import(ctx context.Context, db *sql.DB, year int, r io.Reader, source string) (int, error) {
	var countries = map[string]string{}
	rows, err := db.QueryContext(ctx, `SELECT bmf_name, iso FROM countries`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var name, iso string
		if err := rows.Scan(&name, &iso); err != nil {
			rows.Close()
			return 0, err
		}
		countries[name] = iso
	}
	rows.Close()
	parsed, err := ParseForeign(r, year, countries)
	if err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var rev int
	if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(revision), 0) + 1 FROM meal_rates WHERE year = ? AND country <> 'DE'`, year).Scan(&rev); err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `UPDATE meal_rates SET active = 0 WHERE year = ? AND country <> 'DE'`, year); err != nil {
		return 0, err
	}
	for _, p := range parsed {
		if _, err := tx.ExecContext(ctx, `INSERT INTO meal_rates(year, country, city, label, full_day, partial, lodging, source, revision, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Year, p.Country, p.City, p.Label, p.Full, p.Partial, p.Lodging, source, rev, now); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_log(entity, action, reason, data_json, at) VALUES ('meal_rates', 'import', ?, json_object('year', ?, 'revision', ?, 'rows', ?), ?)`, source, year, rev, len(parsed), now); err != nil {
		return 0, err
	}
	return len(parsed), tx.Commit()
}

// Table is an in-memory view of the active rates; it implements calc.Rates.
type Table struct {
	meal    map[string]calc.MealRate
	mileage map[calc.Vehicle][]mileageRow
}

type mileageRow struct {
	from calc.Date
	rate calc.MileageRate
}

// Load reads the active rate revisions.
func Load(ctx context.Context, db *sql.DB) (*Table, error) {
	t := &Table{meal: map[string]calc.MealRate{}, mileage: map[calc.Vehicle][]mileageRow{}}
	rows, err := db.QueryContext(ctx, `SELECT year, country, city, full_day, partial, lodging, source, revision FROM meal_rates WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m calc.MealRate
		var loc calc.Location
		if err := rows.Scan(&m.Ref.Year, &loc.Country, &loc.City, &m.Full, &m.Partial, &m.Lodging, &m.Ref.Source, &m.Ref.Revision); err != nil {
			rows.Close()
			return nil, err
		}
		m.Ref.Location, m.Ref.Table = loc, "meal_rates"
		t.meal[key(m.Ref.Year, loc)] = m
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `SELECT vehicle, valid_from, cents_per_km, source, revision FROM mileage_rates WHERE active = 1 ORDER BY valid_from`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v calc.Vehicle
		var from string
		var m calc.MileageRate
		if err := rows.Scan(&v, &from, &m.CentsPerKm, &m.Ref.Source, &m.Ref.Revision); err != nil {
			return nil, err
		}
		d, err := calc.ParseDate(from)
		if err != nil {
			return nil, err
		}
		m.Ref.Table = "mileage_rates"
		t.mileage[v] = append(t.mileage[v], mileageRow{d, m})
	}
	return t, rows.Err()
}

func key(year int, loc calc.Location) string {
	return fmt.Sprintf("%d/%s/%s", year, loc.Country, loc.City)
}

// Meal returns the city rate, falling back to the country's general entry.
func (t *Table) Meal(year int, loc calc.Location) (calc.MealRate, error) {
	if m, ok := t.meal[key(year, loc)]; ok {
		return m, nil
	}
	if m, ok := t.meal[key(year, calc.Location{Country: loc.Country})]; ok {
		return m, nil
	}
	return calc.MealRate{}, fmt.Errorf("no meal rate for %s %s in %d", loc.Country, loc.City, year)
}

// Mileage returns the rate valid on d.
func (t *Table) Mileage(d calc.Date, v calc.Vehicle) (calc.MileageRate, error) {
	var found *calc.MileageRate
	for _, r := range t.mileage[v] {
		if !d.Before(r.from) {
			rate := r.rate
			rate.Ref.Year = d.Y
			found = &rate
		}
	}
	if found == nil {
		return calc.MileageRate{}, fmt.Errorf("no mileage rate for %s on %s", v, d)
	}
	return *found, nil
}
