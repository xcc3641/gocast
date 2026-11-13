package calc

import (
	"math/big"
	"strings"
	"unicode/utf8"
)

// category is what a unit measures; only units of one category convert
// into each other.
type category uint8

const (
	catLength category = iota + 1
	catMass
	catTemp
	catArea
	catVolume
	catSpeed
	catData
	catTime
	catType
	catCurrency
)

// unit is a unit or a currency. A quantity of v in u is v*factor+offset
// in the category's base unit (m, g, K, m², l, m/s, B, s, px); a
// currency has no factor, it converts by the rates.
type unit struct {
	code   string
	cat    category
	factor *big.Rat
	offset *big.Rat
}

func (u *unit) toBase(v *big.Rat) *big.Rat {
	b := new(big.Rat).Mul(v, u.factor)
	if u.offset != nil {
		b.Add(b, u.offset)
	}
	return b
}

func (u *unit) fromBase(b *big.Rat) *big.Rat {
	v := new(big.Rat).Set(b)
	if u.offset != nil {
		v.Sub(v, u.offset)
	}
	return v.Quo(v, u.factor)
}

// unitDefs lists every unit: its stable code (Result.Unit, From; the UI
// maps these to symbols and full names), category, size in the base unit
// and the names it is typed as, "|"-separated. Names are matched without
// case, except those starting with "=", matched as written.
var unitDefs = []struct {
	code, factor string
	cat          category
	names        string
}{
	// Length, in metres.
	{"nm", "1e-9", catLength, "nm|nanometer|nanometers|nanometre|nanometres|纳米"},
	{"um", "1e-6", catLength, "um|µm|μm|micrometer|micrometers|micrometre|micrometres|micron|microns|微米"},
	{"mm", "0.001", catLength, "mm|millimeter|millimeters|millimetre|millimetres|毫米"},
	{"cm", "0.01", catLength, "cm|centimeter|centimeters|centimetre|centimetres|厘米|公分"},
	{"dm", "0.1", catLength, "dm|decimeter|decimeters|decimetre|decimetres|分米"},
	{"m", "1", catLength, "m|meter|meters|metre|metres|米|公尺"},
	{"km", "1000", catLength, "km|kilometer|kilometers|kilometre|kilometres|公里|千米"},
	{"in", "0.0254", catLength, "in|inch|inches|英寸|吋"},
	{"ft", "0.3048", catLength, "ft|foot|feet|英尺|呎"},
	{"yd", "0.9144", catLength, "yd|yard|yards|码"},
	{"mi", "1609.344", catLength, "mi|mile|miles|英里"},
	{"nmi", "1852", catLength, "nmi|nautical mile|nautical miles|海里"},
	{"chi", "1/3", catLength, "尺|市尺"},
	{"cun", "1/30", catLength, "寸|市寸"},

	// Mass, in grams.
	{"mg", "0.001", catMass, "mg|milligram|milligrams|毫克"},
	{"g", "1", catMass, "g|gram|grams|gramme|grammes|克"},
	{"kg", "1000", catMass, "kg|kilogram|kilograms|kilo|kilos|千克|公斤"},
	{"t", "1000000", catMass, "t|tonne|tonnes|ton|tons|metric ton|metric tons|吨|公吨"},
	{"oz", "28.349523125", catMass, "oz|ounce|ounces|盎司"},
	{"lb", "453.59237", catMass, "lb|lbs|pound|pounds|磅"},
	{"st", "6350.29318", catMass, "st|stone|stones|英石"},
	{"ct", "0.2", catMass, "ct|carat|carats|克拉"},
	{"jin", "500", catMass, "jin|斤|市斤"},
	{"liang", "50", catMass, "liang|两|市两"},

	// Temperature, in kelvins (factor and offset set in init).
	{"degC", "1", catTemp, "c|°c|℃|celsius|degc|摄氏度|摄氏"},
	{"degF", "5/9", catTemp, "f|°f|℉|fahrenheit|degf|华氏度|华氏"},
	{"K", "1", catTemp, "=K|kelvin|kelvins|开尔文"},

	// Area, in square metres.
	{"mm2", "0.000001", catArea, "mm2|mm²|sq mm|square millimeter|square millimeters|平方毫米"},
	{"cm2", "0.0001", catArea, "cm2|cm²|sq cm|square centimeter|square centimeters|square centimetre|square centimetres|平方厘米"},
	{"m2", "1", catArea, "m2|m²|sq m|sqm|square meter|square meters|square metre|square metres|平方米|平米|平方"},
	{"km2", "1000000", catArea, "km2|km²|sq km|square kilometer|square kilometers|square kilometre|square kilometres|平方公里|平方千米"},
	{"in2", "0.00064516", catArea, "in2|in²|sq in|square inch|square inches|平方英寸"},
	{"ft2", "0.09290304", catArea, "ft2|ft²|sq ft|sqft|square foot|square feet|平方英尺"},
	{"yd2", "0.83612736", catArea, "yd2|yd²|sq yd|square yard|square yards|平方码"},
	{"mi2", "2589988.110336", catArea, "mi2|mi²|sq mi|square mile|square miles|平方英里"},
	{"ha", "10000", catArea, "ha|hectare|hectares|公顷"},
	{"acre", "4046.8564224", catArea, "ac|acre|acres|英亩"},
	{"mu", "10000/15", catArea, "mu|亩|市亩"},

	// Volume, in litres.
	{"ml", "0.001", catVolume, "ml|milliliter|milliliters|millilitre|millilitres|毫升"},
	{"cm3", "0.001", catVolume, "cm3|cm³|cc|cubic centimeter|cubic centimeters|cubic centimetre|cubic centimetres|立方厘米"},
	{"cl", "0.01", catVolume, "cl|centiliter|centiliters|centilitre|centilitres|厘升"},
	{"dl", "0.1", catVolume, "dl|deciliter|deciliters|decilitre|decilitres|分升"},
	{"l", "1", catVolume, "l|liter|liters|litre|litres|升|公升"},
	{"m3", "1000", catVolume, "m3|m³|cubic meter|cubic meters|cubic metre|cubic metres|立方米|立方"},
	{"gal", "3.785411784", catVolume, "gal|gallon|gallons|加仑"},
	{"qt", "0.946352946", catVolume, "qt|quart|quarts|夸脱"},
	{"pt", "0.473176473", catVolume, "pt|pint|pints|品脱"},
	{"cup", "0.2365882365", catVolume, "cup|cups"},
	{"floz", "0.0295735295625", catVolume, "floz|fl oz|fluid ounce|fluid ounces|液量盎司"},
	{"tbsp", "0.01478676478125", catVolume, "tbsp|tablespoon|tablespoons"},
	{"tsp", "0.00492892159375", catVolume, "tsp|teaspoon|teaspoons"},
	{"in3", "0.016387064", catVolume, "in3|in³|cu in|cubic inch|cubic inches|立方英寸"},
	{"ft3", "28.316846592", catVolume, "ft3|ft³|cu ft|cubic foot|cubic feet|立方英尺"},

	// Speed, in metres per second.
	{"km/h", "5/18", catSpeed, "km/h|kmh|kph|kmph|km per hour|kilometers per hour|kilometres per hour|公里/小时|千米/小时|公里每小时|千米每小时"},
	{"mph", "0.44704", catSpeed, "mph|mi/h|miles per hour|英里/小时|英里每小时"},
	{"m/s", "1", catSpeed, "m/s|mps|meters per second|metres per second|米/秒|米每秒"},
	{"ft/s", "0.3048", catSpeed, "ft/s|fps|feet per second|英尺/秒|英尺每秒"},
	{"kn", "1852/3600", catSpeed, "kn|knot|knots|节"},

	// Data, in bytes: KB, MB… are powers of 1000, KiB, MiB… of 1024.
	{"bit", "1/8", catData, "bit|bits|比特"},
	{"B", "1", catData, "b|byte|bytes|字节"},
	{"kB", "1000", catData, "kb|kilobyte|kilobytes"},
	{"MB", "1000000", catData, "mb|megabyte|megabytes"},
	{"GB", "1000000000", catData, "gb|gigabyte|gigabytes"},
	{"TB", "1000000000000", catData, "tb|terabyte|terabytes"},
	{"PB", "1000000000000000", catData, "pb|petabyte|petabytes"},
	{"KiB", "1024", catData, "kib|kibibyte|kibibytes"},
	{"MiB", "1048576", catData, "mib|mebibyte|mebibytes"},
	{"GiB", "1073741824", catData, "gib|gibibyte|gibibytes"},
	{"TiB", "1099511627776", catData, "tib|tebibyte|tebibytes"},
	{"PiB", "1125899906842624", catData, "pib|pebibyte|pebibytes"},

	// Time, in seconds.
	{"ms", "0.001", catTime, "ms|millisecond|milliseconds|毫秒"},
	{"s", "1", catTime, "s|sec|secs|second|seconds|秒|秒钟"},
	{"min", "60", catTime, "min|mins|minute|minutes|分钟|分"},
	{"h", "3600", catTime, "h|hr|hrs|hour|hours|小时|钟头"},
	{"day", "86400", catTime, "d|day|days|天|日"},
	{"week", "604800", catTime, "wk|wks|week|weeks|周|星期|礼拜"},

	// Design units, in CSS pixels (1rem = 1em = 16px).
	{"px", "1", catType, "px|pixel|pixels|像素"},
	{"rem", "16", catType, "rem"},
	{"em", "16", catType, "em"},
}

// entry is what a name means: a unit, maybe another reading of it (a
// currency; "pound" is lb or GBP, whichever converts), or ¥, whose
// currency depends on the region.
type entry struct {
	u, alt *unit
	yen    bool
}

var (
	units     = map[string]*unit{} // by code
	names     = map[string]entry{} // lowercase names
	exact     = map[string]entry{} // case-sensitive names
	cjkWords  = map[string]bool{}  // words the lexer cuts Chinese runs into
	cjkMaxLen = 1                  // longest such word, in runes
)

// Words that aren't units.
var (
	// Conversion keywords: "100 usd in jpy", "100 美元换人民币".
	convWords = map[string]bool{
		"in": true, "to": true, "into": true, "as": true,
		"换成": true, "换算成": true, "转换成": true, "转成": true, "换为": true,
		"转为": true, "转换为": true, "换算为": true, "换": true, "转": true,
		"兑换": true, "兑换成": true, "兑": true, "等于": true, "是": true, "折合": true,
	}
	// Fillers after a conversion keyword: "换多少人民币".
	fillerWords = map[string]bool{"多少": true}
	// Multipliers after a number: 10k, 1万.
	multipliers = map[string]*big.Rat{
		"k": big.NewRat(1000, 1), "百": big.NewRat(100, 1), "千": big.NewRat(1000, 1),
		"万": big.NewRat(10000, 1), "亿": big.NewRat(100000000, 1),
	}
	constWords = map[string]string{"pi": "pi", "π": "pi", "e": "e", "tau": "tau", "τ": "tau"}
	funcWords  = map[string]string{
		"sqrt": "sqrt", "cbrt": "cbrt", "sin": "sin", "cos": "cos", "tan": "tan",
		"asin": "asin", "acos": "acos", "atan": "atan", "arcsin": "asin", "arccos": "acos", "arctan": "atan",
		"log": "log", "lg": "log", "log10": "log", "ln": "ln", "log2": "log2", "exp": "exp",
		"abs": "abs", "round": "round", "floor": "floor", "ceil": "ceil",
	}
	// Other Chinese words the lexer knows.
	cjkOther = []string{"的", "多少", "百", "千", "万", "亿"}
)

func addName(name string, e entry) {
	m := names
	if strings.HasPrefix(name, "=") {
		m, name = exact, name[1:]
	} else {
		name = strings.ToLower(name)
	}
	if old, ok := m[name]; ok {
		// A unit and a currency share the name: keep both readings.
		if old.alt == nil && old.u != nil && e.u != nil && old.u.cat != e.u.cat {
			old.alt = e.u
			m[name] = old
		}
	} else {
		m[name] = e
	}
	addCJK(name)
}

func addCJK(word string) {
	for _, r := range word {
		if !isHan(r) {
			return
		}
	}
	cjkWords[word] = true
	if n := utf8.RuneCountInString(word); n > cjkMaxLen {
		cjkMaxLen = n
	}
}

func init() {
	for _, d := range unitDefs {
		u := &unit{code: d.code, cat: d.cat, factor: mustRat(d.factor)}
		switch d.code {
		case "degC":
			u.offset = mustRat("273.15")
		case "degF":
			u.offset = mustRat("45967/180") // 459.67 × 5/9
		}
		units[d.code] = u
		for _, n := range strings.Split(d.names, "|") {
			addName(n, entry{u: u})
		}
	}
	initCurrencies()
	for w := range convWords {
		addCJK(w)
	}
	for w := range fillerWords {
		addCJK(w)
	}
	for _, w := range cjkOther {
		addCJK(w)
	}
}

func mustRat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("calc: bad number " + s)
	}
	return r
}

// lookup finds what a name means.
func lookup(name string) (entry, bool) {
	if e, ok := exact[name]; ok {
		return e, true
	}
	e, ok := names[strings.ToLower(name)]
	return e, ok
}
