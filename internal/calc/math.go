package calc

import (
	"errors"
	"math"
	"math/big"
	"strconv"
)

// Errors behind a Result with Status Error.
var (
	ErrDivideByZero = errors.New("calc: division by zero")
	ErrDomain       = errors.New("calc: outside the function's domain")
	ErrOverflow     = errors.New("calc: number too large")
	ErrUnits        = errors.New("calc: units don't convert")
	ErrNoRate       = errors.New("calc: no rate for the currency")
)

const (
	approxBits = 2048  // numbers past this many bits are kept to 128-bit precision
	maxBits    = 14000 // ~ 10^4200: larger is an overflow
)

var (
	ratPi  = mustRat("3.14159265358979323846264338327950288419716939937510582097494459")
	ratE   = mustRat("2.71828182845904523536028747135266249775724709369995957496696763")
	ratTau = new(big.Rat).Mul(ratPi, big.NewRat(2, 1))
)

// fit keeps r a size arithmetic stays fast on: exact while it's small,
// else rounded to 128 bits; past ~10^4200 it's an overflow, below
// ~10^-4200 zero.
func fit(r *big.Rat) (*big.Rat, error) {
	nb, db := r.Num().BitLen(), r.Denom().BitLen()
	if nb <= approxBits && db <= approxBits {
		return r, nil
	}
	if nb-db > maxBits {
		return nil, ErrOverflow
	}
	if db-nb > maxBits {
		return new(big.Rat), nil
	}
	f := new(big.Float).SetPrec(128).SetRat(r)
	out, _ := f.Rat(nil)
	return out, nil
}

func toFloat(r *big.Rat) float64 {
	f, _ := r.Float64()
	return f
}

// fromFloat is f as a rational, cut to 15 significant digits so float
// noise (2.9999999999999996) doesn't show or carry on.
func fromFloat(f float64) (*big.Rat, error) {
	if math.IsNaN(f) {
		return nil, ErrDomain
	}
	if math.IsInf(f, 0) {
		return nil, ErrOverflow
	}
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', 15, 64))
	if !ok {
		return nil, ErrDomain
	}
	return r, nil
}

// pow10 is 10^e.
func pow10(e int) *big.Rat {
	n := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(e))), nil)
	if e < 0 {
		return new(big.Rat).SetFrac(big.NewInt(1), n)
	}
	return new(big.Rat).SetInt(n)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// roundInt rounds r to an integer, halves away from zero.
func roundInt(r *big.Rat) *big.Int {
	num := new(big.Int).Abs(r.Num())
	den := r.Denom()
	// floor((2·num + den) / (2·den))
	n := new(big.Int).Lsh(num, 1)
	n.Add(n, den)
	n.Quo(n, new(big.Int).Lsh(den, 1))
	if r.Sign() < 0 {
		n.Neg(n)
	}
	return n
}

func floorRat(r *big.Rat) *big.Rat {
	// Euclidean division floors for a positive divisor.
	return new(big.Rat).SetInt(new(big.Int).Div(r.Num(), r.Denom()))
}

// power is x^y: exact for integer exponents of moderate size, else in
// floating point.
func power(x, y *big.Rat) (*big.Rat, bool, error) {
	if x.Sign() == 0 {
		switch y.Sign() {
		case -1:
			return nil, false, ErrDivideByZero
		case 0:
			return big.NewRat(1, 1), false, nil
		}
		return new(big.Rat), false, nil
	}
	if y.IsInt() && y.Num().IsInt64() {
		n := y.Num().Int64()
		bits := int64(x.Num().BitLen())
		if b := int64(x.Denom().BitLen()); b > bits {
			bits = b
		}
		if n >= -100000 && n <= 100000 && bits*absInt64(n) <= maxBits {
			e := big.NewInt(absInt64(n))
			num := new(big.Int).Exp(x.Num(), e, nil)
			den := new(big.Int).Exp(x.Denom(), e, nil)
			if n < 0 {
				num, den = den, num
			}
			if den.Sign() < 0 {
				num.Neg(num)
				den.Neg(den)
			}
			r, err := fit(new(big.Rat).SetFrac(num, den))
			return r, false, err
		}
	}
	if x.Sign() < 0 && !y.IsInt() {
		// A negative base takes odd roots only: (-8)^(1/3) = -2.
		if y.Denom().Bit(0) == 1 {
			r, approx, err := power(new(big.Rat).Neg(x), y)
			if err != nil {
				return nil, false, err
			}
			if y.Num().Bit(0) == 1 {
				r.Neg(r)
			}
			return r, approx, nil
		}
		return nil, false, ErrDomain
	}
	if x.Sign() < 0 { // an integer power too large to do exactly
		r, approx, err := power(new(big.Rat).Neg(x), y)
		if err == nil && y.Num().Bit(0) == 1 {
			r.Neg(r)
		}
		return r, approx, err
	}
	xf, yf := toFloat(x), toFloat(y)
	if !math.IsInf(xf, 0) && xf != 0 {
		if v := math.Pow(xf, yf); !math.IsInf(v, 0) && v != 0 {
			r, err := fromFloat(v)
			return r, true, err
		}
	}
	// Outside float range: through logs.
	l, err := logRat(x)
	if err != nil {
		return nil, false, err
	}
	r, err := expRat(l * yf)
	return r, true, err
}

// expRat is e^l, also past float range (good to about 12 digits there).
func expRat(l float64) (*big.Rat, error) {
	if math.IsNaN(l) {
		return nil, ErrDomain
	}
	if math.Abs(l) < 700 {
		return fromFloat(math.Exp(l))
	}
	k := math.Floor(l / math.Ln10)
	if k > maxBits*0.30103 {
		return nil, ErrOverflow
	}
	if k < -maxBits*0.30103 {
		return new(big.Rat), nil
	}
	m, err := fromFloat(math.Exp(l - k*math.Ln10))
	if err != nil {
		return nil, err
	}
	return m.Mul(m, pow10(int(k))), nil
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// logRat is ln(x) for any positive x, even past float range.
func logRat(x *big.Rat) (float64, error) {
	if x.Sign() <= 0 {
		return 0, ErrDomain
	}
	f := new(big.Float).SetPrec(64).SetRat(x)
	mant := new(big.Float)
	exp := f.MantExp(mant)
	m, _ := mant.Float64()
	return math.Log(m) + float64(exp)*math.Ln2, nil
}

// sqrtRat is √x: exact when x is a square of rationals.
func sqrtRat(x *big.Rat) (*big.Rat, bool, error) {
	if x.Sign() < 0 {
		return nil, false, ErrDomain
	}
	n, d := new(big.Int).Sqrt(x.Num()), new(big.Int).Sqrt(x.Denom())
	if new(big.Int).Mul(n, n).Cmp(x.Num()) == 0 && new(big.Int).Mul(d, d).Cmp(x.Denom()) == 0 {
		return new(big.Rat).SetFrac(n, d), false, nil
	}
	f := new(big.Float).SetPrec(128).SetRat(x)
	f.Sqrt(f)
	r, _ := f.Rat(nil)
	r, err := fit(r)
	return r, true, err
}

// log10Rat is log10(x): exact for powers of ten.
func log10Rat(x *big.Rat) (*big.Rat, bool, error) {
	if x.Sign() <= 0 {
		return nil, false, ErrDomain
	}
	if k, ok := tenPower(x); ok {
		return big.NewRat(int64(k), 1), false, nil
	}
	l, err := logRat(x)
	if err != nil {
		return nil, false, err
	}
	r, err := fromFloat(l / math.Ln10)
	return r, true, err
}

// tenPower reports k when x = 10^k.
func tenPower(x *big.Rat) (int, bool) {
	var n *big.Int
	sign := 1
	switch {
	case x.IsInt():
		n = x.Num()
	case x.Num().Cmp(big.NewInt(1)) == 0:
		n, sign = x.Denom(), -1
	default:
		return 0, false
	}
	s := n.String()
	if s[0] != '1' {
		return 0, false
	}
	for _, c := range s[1:] {
		if c != '0' {
			return 0, false
		}
	}
	return sign * (len(s) - 1), true
}

// factorial is n! for integers 0…1000.
func factorial(x *big.Rat) (*big.Rat, error) {
	if !x.IsInt() || x.Sign() < 0 {
		return nil, ErrDomain
	}
	if !x.Num().IsInt64() || x.Num().Int64() > 1000 {
		return nil, ErrOverflow
	}
	n := x.Num().Int64()
	f := new(big.Int).MulRange(1, n)
	if n == 0 {
		f.SetInt64(1)
	}
	return fit(new(big.Rat).SetInt(f))
}
