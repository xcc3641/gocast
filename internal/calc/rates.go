package calc

import (
	"errors"
	"math/big"
)

// Rates is a snapshot of exchange rates: how many of each currency one
// unit of the base buys, and the date each rate is from.
type Rates struct {
	base   string
	date   string
	quotes map[string]rateQuote
}

type rateQuote struct {
	rate *big.Rat
	date string
}

// Quote is one rate as served: Rate units of Code per one of the base,
// as decimal text ("159.52"), from Date ("2026-11-13").
type Quote struct {
	Code string
	Rate string
	Date string
}

// NewRates builds a snapshot from quotes against base. Quotes that aren't
// a positive number or a known currency are left out.
func NewRates(base string, quotes []Quote) (*Rates, error) {
	if !fiat[base] {
		return nil, errors.New("calc: unknown base currency " + base)
	}
	r := &Rates{base: base, quotes: make(map[string]rateQuote, len(quotes))}
	for _, q := range quotes {
		if !fiat[q.Code] || q.Code == base || len(q.Rate) > 64 {
			continue
		}
		v, ok := new(big.Rat).SetString(q.Rate)
		if !ok || v.Sign() <= 0 {
			continue
		}
		r.quotes[q.Code] = rateQuote{rate: v, date: q.Date}
		if q.Date > r.date {
			r.date = q.Date
		}
	}
	if len(r.quotes) == 0 {
		return nil, errors.New("calc: no rates")
	}
	return r, nil
}

// Base is the currency the rates are against.
func (r *Rates) Base() string { return r.base }

// Date is the newest rate's date, "2026-11-13".
func (r *Rates) Date() string { return r.date }

// Has reports whether code converts.
func (r *Rates) Has(code string) bool {
	_, ok := r.quotes[code]
	return ok || code == r.base
}

func (r *Rates) quote(code string) (rateQuote, bool) {
	if code == r.base {
		return rateQuote{rate: big.NewRat(1, 1)}, true
	}
	q, ok := r.quotes[code]
	return q, ok
}

// convert changes amount from one currency to another, with the date of
// the older rate used.
func (r *Rates) convert(amount *big.Rat, from, to string) (*big.Rat, string, bool) {
	qf, ok1 := r.quote(from)
	qt, ok2 := r.quote(to)
	if !ok1 || !ok2 {
		return nil, "", false
	}
	v := new(big.Rat).Quo(amount, qf.rate)
	v.Mul(v, qt.rate)
	date := qf.date
	if date == "" || qt.date != "" && qt.date < date {
		date = qt.date
	}
	return v, date, true
}
