package calc

import (
	"fmt"
	"time"
)

// Date is a local calendar date without a time zone.
type Date struct {
	Y int
	M time.Month
	D int
}

// DateOf returns the calendar date of t in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{y, m, d}
}

// ParseDate parses an ISO date (YYYY-MM-DD).
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q", s)
	}
	return DateOf(t), nil
}

// MustDate parses an ISO date and panics on error; for tests and fixtures.
func MustDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Y, d.M, d.D) }

// AddDays returns the date n days later (or earlier for negative n).
func (d Date) AddDays(n int) Date {
	return DateOf(time.Date(d.Y, d.M, d.D+n, 12, 0, 0, 0, time.UTC))
}

// Before reports whether d is strictly before o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// Compare returns -1, 0 or 1.
func (d Date) Compare(o Date) int {
	switch {
	case d.Y != o.Y:
		return sign(d.Y - o.Y)
	case d.M != o.M:
		return sign(int(d.M) - int(o.M))
	default:
		return sign(d.D - o.D)
	}
}

// Midnight returns the instant the date starts in loc.
func (d Date) Midnight(loc *time.Location) time.Time {
	return time.Date(d.Y, d.M, d.D, 0, 0, 0, 0, loc)
}

// MarshalText encodes the date as YYYY-MM-DD.
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText decodes YYYY-MM-DD.
func (d *Date) UnmarshalText(b []byte) error {
	p, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = p
	return nil
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// datesBetween lists every date from a to b inclusive.
func datesBetween(a, b Date) []Date {
	var out []Date
	for d := a; !b.Before(d); d = d.AddDays(1) {
		out = append(out, d)
	}
	return out
}
