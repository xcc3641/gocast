// Package calc reads and evaluates what's typed in the search field when
// it is a calculation: arithmetic, percentages, unit conversions and
// currencies (calculator brief v1). It knows nothing about the UI: Eval
// gives a structured Result, and a Formatter writes it with the system's
// number format.
package calc

import (
	"math/big"
	"strings"
	"unicode/utf8"
)

// Env is what an evaluation depends on besides the input.
type Env struct {
	// Region is the system's ISO 3166 region code ("JP", "CN"): it reads
	// ¥ and picks the currency to convert into when none is named.
	Region string
	// Rates are the latest exchange rates; nil until fetched once.
	Rates *Rates
}

// Status says what the card shows.
type Status uint8

const (
	// OK: the answer is in Value (and Parts).
	OK Status = iota
	// Incomplete: a calculation being typed ("2+", "(3*", "100 usd in");
	// the card keeps its last answer, greyed.
	Incomplete
	// Error: a calculation that can't be done (1/0, sqrt(-1), 5 kg in m);
	// Err says why. The card says it can't calculate this.
	Error
	// NeedsRates: a currency conversion with no rates yet. From, Amount
	// and Unit (the target) are set; Value is nil.
	NeedsRates
)

// Kind is what the answer is.
type Kind uint8

const (
	// KindNumber: a plain number (arithmetic, percentages).
	KindNumber Kind = iota
	// Unit: a quantity in Unit (a unit code: "m", "degF", "km/h").
	KindUnit
	// KindCurrency: an amount in Unit (an ISO 4217 code: "JPY").
	KindCurrency
	// KindDuration: a time, written as Parts ("2 h 25 min"); Value is the
	// same time in Unit, the first part's unit.
	KindDuration
)

// Part is one part of a duration: 2 h, 25 min.
type Part struct {
	Value *big.Rat
	Unit  string // "day", "h", "min", "s"
}

// Result is an evaluated calculation.
type Result struct {
	Status Status
	Err    error // why, when Status is Error
	Kind   Kind

	// Value is the answer in Unit, exact as far as the arithmetic is
	// (functions like sin are good to 15 digits). Number rounds it for
	// display.
	Value *big.Rat
	// Unit is the answer's unit or currency code; "" for a plain number.
	Unit string
	// Parts is a Duration's answer.
	Parts []Part
	// Decimals is how many fraction digits the answer has: 2 or 0 for a
	// currency, -1 for up to 10 significant digits.
	Decimals int

	// From is the unit or currency code the input is in ("" for plain
	// arithmetic); Amount is the input quantity when it is a single one
	// ("100 usd", "¥5000"), nil otherwise.
	From   string
	Amount *big.Rat
	// Input is the whole input, trimmed, full-width forms made half-width:
	// the card's left side. Expr is its part before "in"/"to"/换成.
	Input string
	Expr  string

	// YenByRegion: ¥ was read as the region's yen or yuan (From or Unit
	// says which).
	YenByRegion bool
	// RatesDate is the date of the rates used ("2026-11-12"), "" when none.
	RatesDate string
}

// Eval evaluates input. ok is false when input isn't a calculation —
// words (safari, pi, monday), a bare number (2024) or quantity (10 ft),
// empty — so no card shows.
func Eval(input string, env Env) (res Result, ok bool) {
	if len(input) > 4*maxInput {
		return Result{}, false
	}
	s := strings.TrimSpace(normalize(input))
	display := s
	s = strings.TrimRight(s, "=? ")
	if s == "" || utf8.RuneCountInString(s) > maxInput {
		return Result{}, false
	}
	toks, err := lex(s)
	if err != nil || len(toks) > 2*maxInput || isDate(toks) {
		return Result{}, false
	}
	res, ok = evaluate(s, toks, &env)
	res.Input = display
	return res, ok
}

// isDate reports a date like 2024-11-13 or 2024/11/13, which isn't
// arithmetic.
func isDate(toks []token) bool {
	if len(toks) != 5 || toks[0].kind != tNum || len(toks[0].text) != 4 {
		return false
	}
	sep := toks[1].text
	for k, t := range toks {
		if k%2 == 1 && (t.kind != tOp || t.text != sep || sep != "-" && sep != "/") {
			return false
		}
		if k%2 == 0 && (t.kind != tNum || strings.ContainsAny(t.text, ".e")) {
			return false
		}
		if k > 0 && t.space {
			return false
		}
	}
	return true
}

func isConvKeyword(t *token) bool {
	return t.kind == tWord && convWords[strings.ToLower(t.text)] || t.kind == tOp && t.text == "="
}

func evaluate(s string, toks []token, env *Env) (Result, bool) {
	// A conversion: the last keyword at the top level whose right side is
	// a unit.
	depth := make([]int, len(toks))
	d := 0
	for k := range toks {
		depth[k] = d
		if isOp(&toks[k], "(") {
			d++
		} else if isOp(&toks[k], ")") {
			d--
		}
	}
	for k := len(toks) - 1; k >= 1; k-- {
		if depth[k] != 0 || !isConvKeyword(&toks[k]) {
			continue
		}
		if res, ok, done := convertAt(s, toks, k, env); done {
			return res, ok
		}
	}
	p := newParser(toks, env)
	v, err := p.all()
	if err != nil {
		if err == errEOF && p.nums > 0 {
			return Result{Status: Incomplete}, true
		}
		return Result{}, false
	}
	res := Result{Expr: s, Decimals: -1}
	if v.u != nil {
		res.From = v.u.code
		if p.ops == 0 && !p.compound {
			res.Amount = v.r
		}
	}
	if p.math != nil {
		return p.errorResult(res), true
	}
	if v.u == nil {
		if p.ops == 0 {
			return Result{}, false
		}
		res.Value = v.r
		return p.finish(res), true
	}
	switch v.u.cat {
	case catCurrency:
		to := CurrencyForRegion(env.Region)
		if to == v.u.code {
			to = "USD"
		}
		if to == v.u.code {
			if p.ops == 0 {
				return Result{}, false
			}
		} else {
			v = p.convert(v, units[to])
		}
		if p.math != nil {
			return p.errorResult(res), true
		}
		return p.quantity(res, v), true
	case catTime:
		if v.r.Sign() > 0 && v.u.code != "ms" {
			parts := durationParts(v.u.toBase(v.r))
			if p.ops == 0 && (p.compound || len(parts) == 1 && parts[0].Unit == v.u.code) {
				return Result{}, false
			}
			lead := units[parts[0].Unit]
			res.Kind, res.Parts, res.Unit = KindDuration, parts, lead.code
			res.Value = lead.fromBase(v.u.toBase(v.r))
			return p.finish(res), true
		}
	}
	if p.ops == 0 {
		return Result{}, false
	}
	return p.quantity(res, v), true
}

// convertAt tries toks as a conversion at keyword k. done is false when
// it isn't one, so the caller tries elsewhere.
func convertAt(s string, toks []token, k int, env *Env) (res Result, ok, done bool) {
	kw := strings.ToLower(toks[k].text)
	rhs := toks[k+1:]
	for len(rhs) > 0 && rhs[0].kind == tWord && fillerWords[rhs[0].text] {
		rhs = rhs[1:]
	}
	lhs := toks[:k]
	p := newParser(lhs, env)
	v, err := p.all()
	if kw == "in" && (err != nil || v.u == nil) {
		// "10 in cm", "5 ft 3 in cm": the "in" is inches.
		p2 := newParser(toks[:k+1], env)
		if v2, err2 := p2.all(); err2 == nil && v2.u != nil && len(rhs) > 0 {
			p, v, err, lhs = p2, v2, nil, toks[:k+1]
		}
	}
	if err != nil {
		return Result{}, false, false
	}
	var target entry
	n := 0
	if len(rhs) > 0 {
		tp := newParser(rhs, env)
		target, n = tp.matchUnit(0)
		if n != len(rhs) {
			n = 0
		}
		if n > 0 && target.yen {
			p.yen = true
			target.u = units[yenFor(env.Region)]
		}
	}
	if n == 0 {
		// Nothing after the keyword yet, or the target is being typed:
		// "100 usd in", "10 ft to me".
		if v.u != nil && (len(rhs) == 0 || allWords(rhs)) {
			return Result{Status: Incomplete}, true, true
		}
		if len(rhs) == 0 && kw != "in" && p.nums > 0 {
			return Result{Status: Incomplete}, true, true
		}
		return Result{}, false, false
	}
	res = Result{Expr: s[lhs[0].pos:lhs[len(lhs)-1].end], Decimals: -1}
	if v.u != nil {
		res.From = v.u.code
		if p.ops == 0 && !p.compound {
			res.Amount = v.r
		}
	}
	to := target.u
	if v.u != nil && to.cat != v.u.cat && (v.alt == nil || v.alt.cat != to.cat) && target.alt != nil {
		to = target.alt // "100 usd in pounds"
	}
	if v.u != nil && v.u.cat != to.cat && v.alt != nil && v.alt.cat == to.cat {
		res.From = v.alt.code // "100 pounds in usd"
	}
	if v.u == nil {
		p.fail(ErrUnits, nil)
	} else if p.math == nil {
		v = p.convert(v, to)
	}
	if p.math != nil {
		return p.errorResult(res), true, true
	}
	return p.quantity(res, v), true, true
}

func allWords(toks []token) bool {
	for _, t := range toks {
		if t.kind != tWord {
			return false
		}
	}
	return true
}

func (p *parser) errorResult(res Result) Result {
	return Result{Status: Error, Err: p.math, Expr: res.Expr, From: res.From, Amount: res.Amount, YenByRegion: p.yen}
}

// quantity finishes a Result for v, a quantity in a unit or currency.
func (p *parser) quantity(res Result, v value) Result {
	res.Unit, res.Kind = v.u.code, KindUnit
	if v.u.cat == catCurrency {
		res.Kind, res.Decimals = KindCurrency, CurrencyDecimals(v.u.code)
	}
	if p.needRates {
		res.Status, res.YenByRegion = NeedsRates, p.yen
		if res.Kind != KindCurrency {
			res.Kind = KindCurrency
			res.Decimals = 2
		}
		return res
	}
	res.Value = v.r
	return p.finish(res)
}

func (p *parser) finish(res Result) Result {
	res.YenByRegion = p.yen
	res.RatesDate = p.ratesDate
	if p.needRates {
		res.Status, res.Value, res.Parts = NeedsRates, nil, nil
		res.Kind = KindCurrency
	}
	return res
}

var durationUnits = []string{"day", "h", "min", "s"}

// durationParts writes secs (> 0) as its two largest units, the second
// rounded: 145 min → 2 h 25 min, 100000 s → 1 day 4 h. Under a minute
// it's seconds alone.
func durationParts(secs *big.Rat) []Part {
	for k, code := range durationUnits[:len(durationUnits)-1] {
		u, next := units[code], units[durationUnits[k+1]]
		if secs.Cmp(u.factor) < 0 {
			continue
		}
		n := roundInt(new(big.Rat).Quo(secs, next.factor)) // in the next unit
		ratio := new(big.Rat).Quo(u.factor, next.factor).Num()
		whole, rest := new(big.Int).QuoRem(n, ratio, new(big.Int))
		if k > 0 && rest.Sign() == 0 {
			// Rounded up to one of the unit above: 59.999 min → 1 h.
			up := units[durationUnits[k-1]]
			if new(big.Rat).Mul(new(big.Rat).SetInt(whole), u.factor).Cmp(up.factor) == 0 {
				return []Part{{Value: big.NewRat(1, 1), Unit: up.code}}
			}
		}
		parts := []Part{{Value: new(big.Rat).SetInt(whole), Unit: code}}
		if rest.Sign() != 0 {
			parts = append(parts, Part{Value: new(big.Rat).SetInt(rest), Unit: next.code})
		}
		return parts
	}
	n := sigNumber(secs)
	if !n.Sci && n.Int == "60" && n.Frac == "" { // 59.99999999999 s
		return []Part{{Value: big.NewRat(1, 1), Unit: "min"}}
	}
	return []Part{{Value: secs, Unit: "s"}}
}
