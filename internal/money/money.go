// Package money holds exact monetary arithmetic (C01): integer minor units,
// rational intermediate values and half-away-from-zero rounding to cents.
package money

import (
	"fmt"
	"math/big"
	"strings"
)

// Cents is an amount in EUR cents.
type Cents int64

// String formats cents as a plain decimal ("12.34", "-0.50").
func (c Cents) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// RoundRat rounds a rational to the nearest integer, half away from zero.
func RoundRat(r *big.Rat) int64 {
	num := new(big.Int).Set(r.Num())
	den := r.Denom()
	neg := num.Sign() < 0
	if neg {
		num.Neg(num)
	}
	// floor((2*num + den) / (2*den))
	twice := new(big.Int).Mul(num, big.NewInt(2))
	twice.Add(twice, den)
	q := new(big.Int).Quo(twice, new(big.Int).Mul(den, big.NewInt(2)))
	if neg {
		q.Neg(q)
	}
	return q.Int64()
}

// Percent returns round(amount * basisPoints / 10000).
func Percent(amount Cents, basisPoints int64) Cents {
	return Cents(RoundRat(big.NewRat(int64(amount)*basisPoints, 10000)))
}

// ParseRate parses a positive decimal string such as "1.1650" exactly.
func ParseRate(s string) (*big.Rat, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() <= 0 || strings.ContainsAny(s, "/eE") {
		return nil, fmt.Errorf("invalid rate %q", s)
	}
	return r, nil
}

// ToEUR converts an amount in foreign minor units (with the currency's
// exponent) to EUR cents using a rate in foreign units per EUR 1 (C18).
func ToEUR(minor int64, exponent int, ratePerEUR *big.Rat) Cents {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
	// cents = minor / 10^exp / rate * 100
	r := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(minor), big.NewInt(100)), scale)
	r.Quo(r, ratePerEUR)
	return Cents(RoundRat(r))
}

// Exponent returns the ISO 4217 minor-unit exponent for common currencies.
func Exponent(currency string) (int, bool) {
	switch strings.ToUpper(currency) {
	case "JPY", "KRW", "ISK", "CLP", "VND":
		return 0, true
	case "BHD", "KWD", "OMR", "JOD", "TND", "LYD", "IQD":
		return 3, true
	}
	if len(currency) == 3 {
		return 2, true
	}
	return 0, false
}
