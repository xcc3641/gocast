package calc

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Number is a rounded answer as digits, before locale formatting.
type Number struct {
	Neg  bool
	Int  string // integer digits, ungrouped ("1234"); one digit when Sci
	Frac string // fraction digits ("5678"); "" for none
	Sci  bool   // scientific: Int.Frac × 10^Exp
	Exp  int
}

// String is the number unformatted: no grouping, "." for the decimal
// point ("1234.5678", "1.5e+20") — what 拷贝纯数字 copies.
func (n Number) String() string {
	return Plain.number(n, "", ".")
}

const (
	sigDigits  = 10
	maxExact   = 15 // integers up to this many digits show in full
	minPlainE  = -6 // 0.000001 is plain, 0.0000001 is 1e-7
	maxPlainE  = sigDigits - 1
	maxFixedIn = 15 // a currency amount past this many integer digits goes scientific
)

// Round rounds r as an answer is shown: decimals ≥ 0 fixes the fraction
// digits (currencies), -1 keeps up to 10 significant digits.
func Round(r *big.Rat, decimals int) Number {
	if r == nil {
		return Number{Int: "0"}
	}
	if decimals >= 0 {
		return fixedNumber(r, decimals)
	}
	return sigNumber(r)
}

// Number is the answer rounded for display.
func (r Result) Number() Number { return Round(r.Value, r.Decimals) }

// Raw is the answer as an unformatted number: "1234.5678".
func (r Result) Raw() string {
	if r.Status != OK || r.Value == nil {
		return ""
	}
	return r.Number().String()
}

func sigNumber(x *big.Rat) Number {
	if x.Sign() == 0 {
		return Number{Int: "0"}
	}
	n := Number{Neg: x.Sign() < 0}
	a := new(big.Rat).Abs(x)
	if a.IsInt() && a.Num().BitLen() <= 50 {
		if s := a.Num().String(); len(s) <= maxExact {
			n.Int = s
			return n
		}
	}
	e := exp10(a)
	m := roundInt(new(big.Rat).Mul(a, pow10(sigDigits-1-e)))
	digits := m.String()
	if len(digits) > sigDigits { // 9.9999999996 → 10.00000000
		e++
		digits = digits[:sigDigits]
	}
	digits = strings.TrimRight(digits, "0")
	if digits == "" {
		digits = "0"
	}
	switch {
	case e > maxPlainE || e < minPlainE:
		n.Sci, n.Exp = true, e
		n.Int, n.Frac = digits[:1], digits[1:]
	case e >= 0:
		if len(digits) <= e+1 {
			n.Int = digits + strings.Repeat("0", e+1-len(digits))
		} else {
			n.Int, n.Frac = digits[:e+1], digits[e+1:]
		}
	default:
		n.Int, n.Frac = "0", strings.Repeat("0", -e-1)+digits
	}
	return n
}

// exp10 is floor(log10(a)) for a > 0.
func exp10(a *big.Rat) int {
	f := new(big.Float).SetPrec(64).SetRat(a)
	mant := new(big.Float)
	be := f.MantExp(mant)
	m, _ := mant.Float64()
	e := int(math.Floor(math.Log10(m) + float64(be)*math.Log10(2)))
	for a.Cmp(pow10(e)) < 0 {
		e--
	}
	for a.Cmp(pow10(e+1)) >= 0 {
		e++
	}
	return e
}

func fixedNumber(x *big.Rat, d int) Number {
	m := roundInt(new(big.Rat).Mul(x, pow10(d)))
	n := Number{Neg: m.Sign() < 0}
	s := new(big.Int).Abs(m).String()
	if len(s) <= d {
		s = strings.Repeat("0", d+1-len(s)) + s
	}
	n.Int, n.Frac = s[:len(s)-d], s[len(s)-d:]
	if len(n.Int) > maxFixedIn {
		return sigNumber(x)
	}
	if m.Sign() == 0 {
		n.Neg = false
	}
	return n
}

// Formatter writes answers with a locale's number format. Plain is the
// default; the UI makes one from the system's grouping separator and
// decimal point.
type Formatter struct {
	Group   string // grouping separator: "," (or "." "\u00a0" "'" …)
	Decimal string // decimal point: "." (or ",")
	// Symbol writes a unit or currency code ("degF" → "°F", "h" → "小时");
	// nil is DefaultSymbol.
	Symbol func(code string) string
}

// Plain groups with "," and uses "." for the decimal point.
var Plain = Formatter{Group: ",", Decimal: "."}

func (f Formatter) symbol(code string) string {
	if f.Symbol != nil {
		if s := f.Symbol(code); s != "" {
			return s
		}
	}
	return DefaultSymbol(code)
}

// Number writes n: "1,234.57", "1.5e+20".
func (f Formatter) Number(n Number) string { return f.number(n, f.Group, f.Decimal) }

func (f Formatter) number(n Number, group, dec string) string {
	var b strings.Builder
	if n.Neg {
		b.WriteByte('-')
	}
	in := n.Int
	if in == "" {
		in = "0"
	}
	if group != "" && !n.Sci && len(in) > 3 {
		lead := len(in) % 3
		if lead > 0 {
			b.WriteString(in[:lead])
		}
		for k := lead; k < len(in); k += 3 {
			if k > 0 {
				b.WriteString(group)
			}
			b.WriteString(in[k : k+3])
		}
	} else {
		b.WriteString(in)
	}
	if n.Frac != "" {
		b.WriteString(dec)
		b.WriteString(n.Frac)
	}
	if n.Sci {
		b.WriteByte('e')
		if n.Exp >= 0 {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(n.Exp))
	}
	return b.String()
}

// Answer is the answer as the card shows it and ↵ copies it: "3.048 m",
// "15,432 JPY", "2 h 25 min", "1,024". Empty unless Status is OK.
func (f Formatter) Answer(r Result) string {
	if r.Status != OK {
		return ""
	}
	switch r.Kind {
	case KindDuration:
		parts := make([]string, len(r.Parts))
		for k, p := range r.Parts {
			parts[k] = f.Number(sigNumber(p.Value)) + " " + f.symbol(p.Unit)
		}
		return strings.Join(parts, " ")
	case KindNumber:
		return f.Number(r.Number())
	}
	return f.Number(r.Number()) + " " + f.symbol(r.Unit)
}

// Source is what was calculated: the input quantity ("100 USD",
// "5,000 CNY") or the expression as typed ("2+3*4").
func (f Formatter) Source(r Result) string {
	if r.Amount != nil && r.From != "" {
		return f.Number(sigNumber(r.Amount)) + " " + f.symbol(r.From)
	}
	return r.Expr
}

// Equation is "expression = answer": "100 USD = 15,432 JPY" (拷贝算式和答案).
func (f Formatter) Equation(r Result) string {
	a := f.Answer(r)
	if a == "" {
		return ""
	}
	return f.Source(r) + " = " + a
}

var symbols = map[string]string{
	"degC": "°C", "degF": "°F",
	"mm2": "mm²", "cm2": "cm²", "m2": "m²", "km2": "km²", "in2": "in²", "ft2": "ft²",
	"yd2": "yd²", "mi2": "mi²", "acre": "ac", "mu": "亩",
	"m3": "m³", "cm3": "cm³", "in3": "in³", "ft3": "ft³", "floz": "fl oz", "l": "L", "ml": "mL", "cl": "cL", "dl": "dL",
	"jin": "斤", "liang": "两", "chi": "尺", "cun": "寸", "um": "µm",
	"day": "d", "week": "wk",
}

// DefaultSymbol is a unit code's symbol ("degF" → "°F", "m2" → "m²");
// currency codes and most units are their own symbol.
func DefaultSymbol(code string) string {
	if s, ok := symbols[code]; ok {
		return s
	}
	return code
}
