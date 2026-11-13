package calc

import (
	"errors"
	"math"
	"math/big"
	"strings"
)

var (
	errSyntax = errors.New("calc: not an expression")
	errEOF    = errors.New("calc: expression ends early")
)

const maxDepth = 48

// value is a number, maybe in a unit or currency; a percent is kept as
// its fraction (52% = 0.52) with pct set.
type value struct {
	r   *big.Rat
	u   *unit
	alt *unit // another reading of u's name ("pound": lb or GBP)
	pct bool
}

// parser reads and evaluates tokens together. Errors in the arithmetic
// (division by zero…) don't stop it: the first is kept in math and the
// parse goes on, so input that isn't an expression at all still says so.
type parser struct {
	toks      []token
	i         int
	env       *Env
	depth     int
	ops       int  // operations done: none means the input is a bare number
	nums      int  // numbers and constants read
	compound  bool // a quantity in parts: 5 ft 3 in
	math      error
	needRates bool
	yen       bool
	ratesDate string
}

func newParser(toks []token, env *Env) *parser { return &parser{toks: toks, env: env} }

// all parses the whole token list as one expression.
func (p *parser) all() (value, error) {
	v, err := p.expr()
	if err != nil {
		return v, err
	}
	if p.i < len(p.toks) {
		return v, errSyntax
	}
	return v, nil
}

func (p *parser) fail(err error, keep *unit) value {
	if p.math == nil {
		p.math = err
	}
	return value{r: new(big.Rat), u: keep}
}

func (p *parser) peek() *token {
	if p.i < len(p.toks) {
		return &p.toks[p.i]
	}
	return nil
}

func isOp(t *token, ops string) bool {
	return t != nil && t.kind == tOp && strings.Contains(ops, t.text) && t.text != ""
}

func isWord(t *token, w string) bool {
	return t != nil && t.kind == tWord && strings.EqualFold(t.text, w)
}

func (p *parser) expr() (value, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return value{}, errSyntax
	}
	l, err := p.term()
	if err != nil {
		return l, err
	}
	for {
		t := p.peek()
		if !isOp(t, "+-") {
			return l, nil
		}
		p.i++
		r, err := p.term()
		if err != nil {
			return r, err
		}
		p.ops++
		l = p.addSub(l, r, t.text == "-")
	}
}

func (p *parser) term() (value, error) {
	l, err := p.unary()
	if err != nil {
		return l, err
	}
	for {
		t := p.peek()
		var op string
		switch {
		case isOp(t, "*/"):
			op = t.text
			p.i++
		case isWord(t, "x") && p.i+1 == len(p.toks):
			return l, errEOF
		case isWord(t, "x") && p.i+1 < len(p.toks) && p.startsOperand(&p.toks[p.i+1]):
			op = "*"
			p.i++
		case isWord(t, "of"), isWord(t, "off"), isWord(t, "的"):
			op = strings.ToLower(t.text)
			p.i++
		case t != nil && t.kind != tNum && p.startsOperand(t) && !isOp(t, "+-"):
			op = "implicit"
		default:
			return l, nil
		}
		r, err := p.unary()
		if err != nil {
			return r, err
		}
		p.ops++
		switch op {
		case "*", "implicit":
			l = p.mul(l, r)
		case "/":
			l = p.div(l, r)
		case "of": // 52% of 900
			if !l.pct || r.pct {
				return l, errSyntax
			}
			l = p.mul(r, value{r: l.r})
		case "off": // 20% off 80
			if !l.pct || r.pct {
				return l, errSyntax
			}
			f := new(big.Rat).Sub(big.NewRat(1, 1), l.r)
			l = p.mul(r, value{r: f})
		case "的": // 900 的 52%
			if !r.pct || l.pct {
				return l, errSyntax
			}
			l = p.mul(l, value{r: r.r})
		}
	}
}

// startsOperand reports whether t can begin a factor that multiplies
// what's before it without a sign: 4(2+3), 2pi, 3 sqrt 4.
func (p *parser) startsOperand(t *token) bool {
	switch t.kind {
	case tNum:
		return true
	case tOp:
		return t.text == "(" || t.text == "√"
	case tWord:
		_, c := constWords[strings.ToLower(t.text)]
		_, f := funcWords[strings.ToLower(t.text)]
		return c || f
	}
	return false
}

func (p *parser) unary() (value, error) {
	t := p.peek()
	switch {
	case isOp(t, "-"):
		p.i++
		v, err := p.unary()
		if err == nil {
			v.r = new(big.Rat).Neg(v.r)
		}
		return v, err
	case isOp(t, "+"):
		p.i++
		return p.unary()
	case isOp(t, "√"):
		p.i++
		v, err := p.unary()
		if err != nil {
			return v, err
		}
		p.ops++
		return p.apply("sqrt", v, nil), nil
	}
	return p.power()
}

func (p *parser) power() (value, error) {
	b, err := p.postfix()
	if err != nil {
		return b, err
	}
	if !isOp(p.peek(), "^") {
		return b, nil
	}
	p.i++
	e, err := p.unary()
	if err != nil {
		return e, err
	}
	p.ops++
	if e.u != nil || b.u != nil && e.r.Cmp(big.NewRat(1, 1)) != 0 {
		return p.fail(ErrUnits, nil), nil
	}
	r, _, perr := power(b.r, e.r)
	if perr != nil {
		return p.fail(perr, b.u), nil
	}
	return value{r: r, u: b.u, alt: b.alt}, nil
}

func (p *parser) postfix() (value, error) {
	v, err := p.primary()
	if err != nil {
		return v, err
	}
	for {
		t := p.peek()
		switch {
		case isOp(t, "%"):
			if v.u != nil || v.pct {
				return v, errSyntax
			}
			p.i++
			v.r = new(big.Rat).Quo(v.r, big.NewRat(100, 1))
			v.pct = true
		case isOp(t, "!"):
			p.i++
			p.ops++
			if v.u != nil {
				return p.fail(ErrUnits, nil), nil
			}
			r, ferr := factorial(v.r)
			if ferr != nil {
				v = p.fail(ferr, nil)
			} else {
				v.r = r
			}
		case isOp(t, "°"): // degrees, for trigonometry
			if v.u != nil {
				return v, errSyntax
			}
			p.i++
			v.r = new(big.Rat).Mul(v.r, ratPi)
			v.r.Quo(v.r, big.NewRat(180, 1))
		default:
			return v, nil
		}
	}
}

func (p *parser) primary() (value, error) {
	t := p.peek()
	if t == nil {
		return value{}, errEOF
	}
	switch t.kind {
	case tNum:
		p.i++
		p.nums++
		v := value{r: p.multiplied(new(big.Rat).Set(t.num))}
		p.attachUnit(&v)
		return v, nil
	case tSym: // $12, ¥5000, HK$100
		e, _ := lookup(t.text)
		p.i++
		n := p.peek()
		if n == nil {
			return value{}, errEOF
		}
		if n.kind != tNum {
			return value{}, errSyntax
		}
		p.i++
		p.nums++
		return value{r: p.multiplied(new(big.Rat).Set(n.num)), u: p.resolve(e)}, nil
	case tOp:
		if t.text != "(" {
			if isOp(t, "+-√") {
				return p.unary()
			}
			return value{}, errSyntax
		}
		p.i++
		v, err := p.expr()
		if err != nil {
			return v, err
		}
		c := p.peek()
		if c == nil {
			return v, errEOF
		}
		if !isOp(c, ")") {
			return v, errSyntax
		}
		p.i++
		if v.u == nil && !v.pct {
			p.attachUnit(&v)
		}
		return v, nil
	case tWord:
		low := strings.ToLower(t.text)
		if c, ok := constWords[low]; ok {
			p.i++
			p.nums++
			switch c {
			case "pi":
				return value{r: ratPi}, nil
			case "tau":
				return value{r: ratTau}, nil
			}
			return value{r: ratE}, nil
		}
		if f, ok := funcWords[low]; ok {
			p.i++
			return p.call(f)
		}
	}
	return value{}, errSyntax
}

// multiplied applies the multipliers after a number: 10k, 1万, 3千万.
func (p *parser) multiplied(r *big.Rat) *big.Rat {
	for {
		t := p.peek()
		if t == nil || t.kind != tWord {
			return r
		}
		switch {
		case t.text == "k" && !t.space:
		case t.text == "K" && !t.space && p.i+1 < len(p.toks) && p.unitAt(p.i+1):
		case t.text != "k" && t.text != "K" && multipliers[t.text] != nil:
		default:
			return r
		}
		r.Mul(r, multipliers[strings.ToLower(t.text)])
		p.i++
	}
}

func (p *parser) unitAt(i int) bool {
	_, n := p.matchUnit(i)
	return n > 0
}

// matchUnit reads the unit or currency name at toks[i:] — one to three
// words ("sq ft", "miles per hour") or a "/" pair (km/h) — longest first.
func (p *parser) matchUnit(i int) (entry, int) {
	toks := p.toks
	if i >= len(toks) {
		return entry{}, 0
	}
	if toks[i].kind == tSym {
		e, ok := lookup(toks[i].text)
		if ok {
			return e, 1
		}
		return entry{}, 0
	}
	word := func(k int) bool { return k < len(toks) && toks[k].kind == tWord }
	if !word(i) {
		return entry{}, 0
	}
	if word(i+1) && word(i+2) {
		if e, ok := lookup(toks[i].text + " " + toks[i+1].text + " " + toks[i+2].text); ok {
			return e, 3
		}
	}
	if word(i+2) && isOp(&toks[i+1], "/") {
		if e, ok := lookup(toks[i].text + "/" + toks[i+2].text); ok {
			return e, 3
		}
	}
	if word(i + 1) {
		if e, ok := lookup(toks[i].text + " " + toks[i+1].text); ok {
			return e, 2
		}
	}
	if e, ok := lookup(toks[i].text); ok {
		return e, 1
	}
	return entry{}, 0
}

func (p *parser) resolve(e entry) *unit {
	if e.yen {
		p.yen = true
		return units[yenFor(p.env.Region)]
	}
	return e.u
}

// attachUnit reads the unit after a number, and any further parts of a
// compound quantity: 5 ft 3 in, 1 h 30 min, 1斤2两.
func (p *parser) attachUnit(v *value) {
	e, n := p.matchUnit(p.i)
	if n == 0 {
		return
	}
	p.i += n
	v.u, v.alt = p.resolve(e), e.alt
	if v.u.cat == catCurrency || v.u.cat == catTemp {
		return
	}
	for p.i+1 < len(p.toks) && p.toks[p.i].kind == tNum {
		e2, n2 := p.matchUnit(p.i + 1)
		if n2 == 0 || e2.u == nil || e2.u.cat != v.u.cat {
			return
		}
		add := v.u.fromBase(e2.u.toBase(p.toks[p.i].num))
		v.r = new(big.Rat).Add(v.r, add)
		v.alt = nil
		p.compound = true
		p.i += 1 + n2
		p.nums++
	}
}

func (p *parser) call(f string) (value, error) {
	var a, b value
	var two bool
	if isOp(p.peek(), "(") {
		p.i++
		p.nums++ // "sqrt(" is a calculation being typed
		var err error
		if a, err = p.expr(); err != nil {
			return a, err
		}
		if isOp(p.peek(), ",") && (f == "round" || f == "log") {
			p.i++
			if b, err = p.expr(); err != nil {
				return b, err
			}
			two = true
		}
		c := p.peek()
		if c == nil {
			return a, errEOF
		}
		if !isOp(c, ")") {
			return a, errSyntax
		}
		p.i++
	} else {
		var err error
		if a, err = p.power(); err != nil {
			return a, err
		}
	}
	p.ops++
	if two {
		return p.apply(f, a, &b), nil
	}
	return p.apply(f, a, nil), nil
}

// apply runs function f on a (and b: round's digits, log's base).
func (p *parser) apply(f string, a value, b *value) value {
	if a.pct {
		a.pct = false
	}
	switch f {
	case "abs":
		a.r = new(big.Rat).Abs(a.r)
		return a
	case "floor":
		a.r = floorRat(a.r)
		return a
	case "ceil":
		a.r = new(big.Rat).Neg(floorRat(new(big.Rat).Neg(a.r)))
		return a
	case "round":
		digits := 0
		if b != nil {
			if b.u != nil || !b.r.IsInt() || b.r.Num().BitLen() > 8 || abs(int(b.r.Num().Int64())) > 30 {
				return p.fail(ErrDomain, a.u)
			}
			digits = int(b.r.Num().Int64())
		}
		s := pow10(digits)
		r := new(big.Rat).SetInt(roundInt(new(big.Rat).Mul(a.r, s)))
		a.r = r.Quo(r, s)
		return a
	}
	if a.u != nil || b != nil && b.u != nil {
		return p.fail(ErrUnits, nil)
	}
	var (
		r   *big.Rat
		err error
	)
	switch f {
	case "sqrt":
		r, _, err = sqrtRat(a.r)
	case "cbrt":
		r, _, err = power(a.r, big.NewRat(1, 3))
		if err == nil {
			// Exact cubes come back exact: cbrt(27) = 3.
			if c := new(big.Rat).SetInt(roundInt(r)); new(big.Rat).Mul(c, new(big.Rat).Mul(c, c)).Cmp(a.r) == 0 {
				r = c
			}
		}
	case "log":
		r, _, err = log10Rat(a.r)
		if err == nil && b != nil {
			var lb *big.Rat
			if lb, _, err = log10Rat(b.r); err == nil {
				if lb.Sign() == 0 {
					err = ErrDivideByZero
				} else {
					r, err = fromFloat(toFloat(r) / toFloat(lb))
				}
			}
		}
	case "ln", "log2":
		var l float64
		if l, err = logRat(a.r); err == nil {
			if f == "log2" {
				l /= math.Ln2
			}
			r, err = fromFloat(l)
		}
	case "exp":
		r, _, err = power(ratE, a.r)
	default: // trigonometry, in radians
		x := toFloat(a.r)
		var y float64
		switch f {
		case "sin":
			y = math.Sin(x)
		case "cos":
			y = math.Cos(x)
		case "tan":
			y = math.Tan(x)
		case "asin":
			y = math.Asin(x)
		case "acos":
			y = math.Acos(x)
		case "atan":
			y = math.Atan(x)
		}
		if math.Abs(y) < 1e-14 {
			y = 0 // sin(pi) is 0, not 1.2e-16
		}
		r, err = fromFloat(y)
	}
	if err != nil {
		return p.fail(err, nil)
	}
	return value{r: r}
}

func (p *parser) addSub(l, r value, minus bool) value {
	if r.pct && !l.pct { // 80 + 10%
		f := big.NewRat(1, 1)
		if minus {
			f.Sub(f, r.r)
		} else {
			f.Add(f, r.r)
		}
		return value{r: p.fit(new(big.Rat).Mul(l.r, f)), u: l.u, alt: l.alt}
	}
	out := value{u: l.u, alt: l.alt, pct: l.pct && r.pct}
	rr := r.r
	switch {
	case l.u == nil && r.u != nil:
		out.u, out.alt = r.u, r.alt
	case l.u != nil && r.u != nil:
		lu, cv := p.unify(l, r)
		if cv.u == nil {
			return value{r: new(big.Rat), u: l.u}
		}
		out.u, out.alt, rr = lu, nil, cv.r
	}
	if minus {
		out.r = new(big.Rat).Sub(l.r, rr)
	} else {
		out.r = new(big.Rat).Add(l.r, rr)
	}
	out.r = p.fit(out.r)
	return out
}

// unify converts r into l's unit (or the readings that match).
func (p *parser) unify(l, r value) (*unit, value) {
	for _, lu := range []*unit{l.u, l.alt} {
		if lu == nil {
			continue
		}
		for _, ru := range []*unit{r.u, r.alt} {
			if ru != nil && ru.cat == lu.cat {
				return lu, p.convert(value{r: r.r, u: ru}, lu)
			}
		}
	}
	p.fail(ErrUnits, nil)
	return l.u, value{}
}

func (p *parser) mul(l, r value) value {
	if l.u != nil && r.u != nil {
		return p.fail(ErrUnits, nil)
	}
	out := value{r: p.fit(new(big.Rat).Mul(l.r, r.r)), u: l.u, alt: l.alt}
	if r.u != nil {
		out.u, out.alt = r.u, r.alt
	}
	return out
}

func (p *parser) div(l, r value) value {
	if r.r.Sign() == 0 {
		return p.fail(ErrDivideByZero, l.u)
	}
	switch {
	case l.u != nil && r.u != nil: // 10 km / 500 m = 20
		_, cv := p.unify(l, r)
		if cv.u == nil {
			return value{r: new(big.Rat)}
		}
		if cv.r.Sign() == 0 {
			return p.fail(ErrDivideByZero, nil)
		}
		return value{r: p.fit(new(big.Rat).Quo(l.r, cv.r))}
	case r.u != nil:
		return p.fail(ErrUnits, nil)
	}
	return value{r: p.fit(new(big.Rat).Quo(l.r, r.r)), u: l.u, alt: l.alt}
}

func (p *parser) fit(r *big.Rat) *big.Rat {
	out, err := fit(r)
	if err != nil {
		p.fail(err, nil)
		return new(big.Rat)
	}
	return out
}

// convert changes v into unit to (same category; currencies by the
// rates).
func (p *parser) convert(v value, to *unit) value {
	u := v.u
	if u.cat != to.cat {
		if v.alt == nil || v.alt.cat != to.cat {
			return p.fail(ErrUnits, to)
		}
		u = v.alt
	}
	if u == to {
		return value{r: v.r, u: to}
	}
	if u.cat != catCurrency {
		return value{r: p.fit(to.fromBase(u.toBase(v.r))), u: to}
	}
	if p.env.Rates == nil {
		p.needRates = true
		return value{r: new(big.Rat), u: to}
	}
	r, date, ok := p.env.Rates.convert(v.r, u.code, to.code)
	if !ok {
		return p.fail(ErrNoRate, to)
	}
	if p.ratesDate == "" || date != "" && date < p.ratesDate {
		p.ratesDate = date
	}
	return value{r: p.fit(r), u: to}
}
