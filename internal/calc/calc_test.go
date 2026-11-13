package calc

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

// testRates are fixed rates per USD.
func testRates(t testing.TB) *Rates {
	r, err := NewRates("USD", []Quote{
		{"JPY", "159.52", "2026-11-13"},
		{"CNY", "6.6635", "2026-11-13"},
		{"EUR", "0.9", "2026-11-12"},
		{"GBP", "0.75", "2026-11-12"},
		{"HKD", "7.85", "2026-11-12"},
		{"TWD", "32.937", "2026-11-12"},
		{"KRW", "1334.71", "2026-11-12"},
		{"AUD", "1.5", "2026-11-12"},
		{"BTC", "0.00001", "2026-11-11"}, // not fiat: left out
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type evalCase struct {
	in     string
	answer string // Plain.Answer; "" with ok false or another status
	ok     bool
	status Status
}

func runCases(t *testing.T, env Env, cases []evalCase) {
	t.Helper()
	for _, c := range cases {
		r, ok := Eval(c.in, env)
		if ok != c.ok {
			t.Errorf("Eval(%q) ok = %v, want %v (%+v)", c.in, ok, c.ok, r)
			continue
		}
		if !ok {
			continue
		}
		if r.Status != c.status {
			t.Errorf("Eval(%q) status = %d, want %d (err %v)", c.in, r.Status, c.status, r.Err)
			continue
		}
		if got := Plain.Answer(r); got != c.answer {
			t.Errorf("Eval(%q) = %q, want %q", c.in, got, c.answer)
		}
	}
}

func ans(in, answer string) evalCase { return evalCase{in: in, answer: answer, ok: true} }
func not(in string) evalCase         { return evalCase{in: in} }
func incomplete(in string) evalCase  { return evalCase{in: in, ok: true, status: Incomplete} }
func cant(in string) evalCase        { return evalCase{in: in, ok: true, status: Error} }

// The brief's acceptance cases (验收 1–5) that are the engine's.
func TestAcceptance(t *testing.T) {
	env := Env{Region: "CN", Rates: testRates(t)}
	runCases(t, env, []evalCase{
		// 1
		ans("2+3*4", "14"),
		ans("0.1+0.2", "0.3"),
		ans("（12+8）×3", "60"),
		ans("2^10", "1,024"),
		// 2
		ans("52% of 900", "468"),
		ans("20% off 80", "64"),
		// 3
		ans("10 ft in m", "3.048 m"),
		ans("100 斤 in kg", "50 kg"),
		ans("30 c in f", "86 °F"),
		ans("2rem in px", "32 px"),
		// 4
		ans("100 usd in jpy", "15,952 JPY"),
		ans("100 美元换人民币", "666.35 CNY"),
		// 5
		ans("pi*2", "6.283185307"),
		not("pi"),
		not("safari"),
		not("2024"),
	})

	r, _ := Eval("100 usd in jpy", env)
	if r.RatesDate != "2026-11-13" || r.Kind != KindCurrency || r.From != "USD" || r.Unit != "JPY" {
		t.Errorf("100 usd in jpy: %+v", r)
	}
	if got := Plain.Equation(r); got != "100 USD = 15,952 JPY" {
		t.Errorf("Equation = %q", got)
	}
	if got := r.Raw(); got != "15952" {
		t.Errorf("Raw = %q", got)
	}
}

func TestNotCalculations(t *testing.T) {
	env := Env{Region: "CN", Rates: testRates(t)}
	var cases []evalCase
	for _, in := range []string{
		"", "   ", "safari", "Safari", "monday", "github", "pi", "π", "e", "tau", "2024", "-5", "+5", "(5)", "1,234",
		"10k", "1e5", "10%", "1万", "$", "€", "¥", "HK$", "sqrt", "sin", "log", "in", "to", "=", "%", "of",
		"hello world", "2 apples", "1 2", "1.2.3", "192.168.1.1", "2024-11-13", "2024/11/13",
		"Dark Mode", "dev server", "npm run dev", "git status", "System Settings", "visual studio code",
		"10 ft", "12.5 s", "100 斤", "30 °C", "20°C", "100 km/h", "5 GB", "2 days", "1 h", "1h 30min", "10 in",
		"10 usd in 1/0", "x", "foo + 1", "1 + foo", "2 +* 3", "*3", ")", "((", "1)",
		"微信", "计算器", "日元", "人民币", "多少", "√",
	} {
		cases = append(cases, not(in))
	}
	runCases(t, env, cases)
}

func TestArithmetic(t *testing.T) {
	cases := []evalCase{}
	for _, c := range []evalCase{
		ans("1+1", "2"), ans("2 + 2", "4"), ans("2 +2", "4"), ans("10-3-2", "5"), ans("2*3+4", "10"),
		ans("2*(3+4)", "14"), ans("8/2/2", "2"), ans("7/2", "3.5"), ans("1/3", "0.3333333333"),
		ans("2/3", "0.6666666667"), ans("1/3*3", "1"), ans("0.1*3", "0.3"), ans("1.1*1.1", "1.21"),
		ans("-2^2", "-4"), ans("(-2)^2", "4"), ans("2^-2", "0.25"), ans("2^3^2", "512"), ans("2**8", "256"),
		ans("2^0.5", "1.414213562"), ans("(-8)^(1/3)", "-2"), ans("4(2+3)", "20"), ans("(1+2)(3+4)", "21"),
		ans("2pi", "6.283185307"), ans("2 x 3", "6"), ans("6 ÷ 4", "1.5"), ans("3 × 4", "12"),
		ans("（1＋2）×3－4÷2", "7"), ans("１２＋３", "15"), ans("1,234.5*2", "2,469"), ans("1，234 + 1", "1,235"),
		ans("1,000,000 / 4", "250,000"), ans("10k + 1", "10,001"), ans("1万 + 1", "10,001"),
		ans("3千万 / 1", "30,000,000"), ans("1e3 + 1", "1,001"), ans(".5 + .25", "0.75"), ans("2+3=", "5"),
		ans("0.1 + 0.2 = ", "0.3"), ans("2+3?", "5"), ans("5!", "120"), ans("0!", "1"), ans("sqrt(625)", "25"),
		ans("sqrt 16 + 9", "13"), ans("√16", "4"), ans("sqrt(2)", "1.414213562"), ans("sqrt(1/4)", "0.5"),
		ans("cbrt(27)", "3"), ans("3 sqrt 4", "6"), ans("e^2", "7.389056099"), ans("ln(e)", "1"),
		ans("log(1000)", "3"), ans("log(0.01)", "-2"), ans("log(8, 2)", "3"), ans("log2(1024)", "10"),
		ans("exp(0)", "1"), ans("sin(0)", "0"), ans("sin(pi)", "0"), ans("sin(pi/2)", "1"), ans("cos(pi)", "-1"),
		ans("cos(pi/2)", "0"), ans("tan(pi/4)", "1"), ans("sin(30°)", "0.5"), ans("atan(1)*4", "3.141592654"),
		ans("abs(-5)", "5"), ans("abs(3-10)", "7"), ans("round(2.5)", "3"), ans("round(-2.5)", "-3"),
		ans("round(2.345, 2)", "2.35"), ans("round(1234.5, -2)", "1,200"), ans("floor(2.7)", "2"),
		ans("floor(-2.1)", "-3"), ans("ceil(2.1)", "3"), ans("-1/3", "-0.3333333333"), ans("10 - 20", "-10"),
		ans("1/7", "0.1428571429"), ans("π*2", "6.283185307"), ans("sqrt(625", ""),
	} {
		if c.answer == "" {
			c = incomplete(c.in)
		}
		cases = append(cases, c)
	}
	runCases(t, Env{}, cases)
}

func TestPercent(t *testing.T) {
	runCases(t, Env{Region: "US", Rates: testRates(t)}, []evalCase{
		ans("52% of 900", "468"),
		ans("20% off 80", "64"),
		ans("80 + 10%", "88"),
		ans("80 - 10%", "72"),
		ans("100 - 20%", "80"),
		ans("900 的 52%", "468"),
		ans("900的52%", "468"),
		ans("50% * 10", "5"),
		ans("10% of 50 usd", "5.00 USD"),
		ans("15% off $40", "34.00 USD"),
		ans("12.5% of 80", "10"),
		ans("１０％ of 50", "5"),
	})
	runCases(t, Env{}, []evalCase{incomplete("10% of"), incomplete("20% off"), incomplete("900 的"), not("half of 10")})
}

func TestUnits(t *testing.T) {
	runCases(t, Env{}, []evalCase{
		// Length
		ans("10 ft in m", "3.048 m"),
		ans("1 m in ft", "3.280839895 ft"),
		ans("1 mi in km", "1.609344 km"),
		ans("5 km to mi", "3.106855961 mi"),
		ans("12 in to cm", "30.48 cm"),
		ans("10 in cm", "25.4 cm"),
		ans("5 ft 3 in in cm", "160.02 cm"),
		ans("1 yd in ft", "3 ft"),
		ans("1000 mm in m", "1 m"),
		ans("100 cm = m", "1 m"),
		ans("1 inch in mm", "25.4 mm"),
		ans("3 feet in inches", "36 in"),
		ans("5 公里换成米", "5,000 m"),
		ans("1 英里 in 公里", "1.609344 km"),
		ans("1 英尺 in 厘米", "30.48 cm"),
		ans("10 英寸 in 厘米", "25.4 cm"),
		ans("1千米 to 米", "1,000 m"),
		ans("3 毫米 in cm", "0.3 cm"),
		// Mass
		ans("100 斤 in kg", "50 kg"),
		ans("100 kg in 斤", "200 斤"),
		ans("1斤2两 in kg", "0.6 kg"),
		ans("1 lb in g", "453.59237 g"),
		ans("1 kg in lb", "2.204622622 lb"),
		ans("16 oz in lb", "1 lb"),
		ans("1 t in kg", "1,000 kg"),
		ans("500 mg in g", "0.5 g"),
		ans("1 公斤 in 克", "1,000 g"),
		ans("2 千克 in 磅", "4.409245244 lb"),
		ans("1 吨 in 斤", "2,000 斤"),
		ans("1 盎司 in 克", "28.34952313 g"),
		ans("100 pounds in kg", "45.359237 kg"),
		// Temperature
		ans("30 c in f", "86 °F"),
		ans("30 °C in °F", "86 °F"),
		ans("30℃ in ℉", "86 °F"),
		ans("98.6 f to c", "37 °C"),
		ans("-40 c in f", "-40 °F"),
		ans("0 K in C", "-273.15 °C"),
		ans("300 K in °C", "26.85 °C"),
		ans("100 celsius in fahrenheit", "212 °F"),
		ans("37 摄氏度 in 华氏度", "98.6 °F"),
		ans("212 华氏度换成摄氏度", "100 °C"),
		// Area
		ans("1 亩 in 平方米", "666.6666667 m²"),
		ans("1 km2 in 亩", "1,500 亩"),
		ans("100 m² in ft²", "1,076.391042 ft²"),
		ans("1 acre in m2", "4,046.856422 m²"),
		ans("1 ha in 亩", "15 亩"),
		ans("1000 sq ft in sqm", "92.90304 m²"),
		ans("1 公顷 in 平方米", "10,000 m²"),
		ans("100 平方米 in 亩", "0.15 亩"),
		// Volume
		ans("1 gal in l", "3.785411784 L"),
		ans("500 ml in l", "0.5 L"),
		ans("2 升 in 毫升", "2,000 mL"),
		ans("1 cup in ml", "236.5882365 mL"),
		ans("1 m³ in l", "1,000 L"),
		ans("1 l in gal", "0.2641720524 gal"),
		ans("12 fl oz in ml", "354.8823548 mL"),
		// Speed
		ans("60 mph in km/h", "96.56064 km/h"),
		ans("100 km/h in mph", "62.13711922 mph"),
		ans("10 m/s in km/h", "36 km/h"),
		ans("36 km/h in m/s", "10 m/s"),
		ans("100 公里/小时 in 米每秒", "27.77777778 m/s"),
		ans("1 knot in km/h", "1.852 km/h"),
		// Data
		ans("1 GB in MB", "1,000 MB"),
		ans("1 GiB in MiB", "1,024 MiB"),
		ans("1 GiB in MB", "1,073.741824 MB"),
		ans("1 TB in GiB", "931.3225746 GiB"),
		ans("1024 KiB in MiB", "1 MiB"),
		ans("1 kb in b", "1,000 B"),
		ans("8 bit in B", "1 B"),
		ans("500 MB in GB", "0.5 GB"),
		// Time
		ans("145 min", "2 h 25 min"),
		ans("145 分钟", "2 h 25 min"),
		ans("90 s", "1 min 30 s"),
		ans("0.5 h", "30 min"),
		ans("1.5 min", "1 min 30 s"),
		ans("48h", "2 d"),
		ans("3 weeks", "21 d"),
		ans("100000 s", "1 d 4 h"),
		ans("59.9 min", "59 min 54 s"),
		ans("59.999 min", "1 h"),
		ans("0.9985 min", "59.91 s"),
		ans("0.5 min", "30 s"),
		ans("1.5 day", "1 d 12 h"),
		ans("1h + 45min", "1 h 45 min"),
		ans("145 min in h", "2.416666667 h"),
		ans("1 day in h", "24 h"),
		ans("2 weeks in days", "14 d"),
		ans("3 小时 in 分钟", "180 min"),
		ans("1 天 in 小时", "24 h"),
		ans("1500 ms in s", "1.5 s"),
		// Design
		ans("2rem in px", "32 px"),
		ans("24px in rem", "1.5 rem"),
		ans("1.5 em in px", "24 px"),
		// Arithmetic with units
		ans("10 m + 5 ft", "11.524 m"),
		ans("10 m + 5", "15 m"),
		ans("2 * 3 kg", "6 kg"),
		ans("10 kg / 4", "2.5 kg"),
		ans("10 km / 500 m", "20"),
		ans("(10 m + 5 ft) in ft", "37.80839895 ft"),
		ans("1 km + 500 m in m", "1,500 m"),
	})
}

func TestCurrency(t *testing.T) {
	rates := testRates(t)
	cn := Env{Region: "CN", Rates: rates}
	runCases(t, cn, []evalCase{
		ans("100 usd in jpy", "15,952 JPY"),
		ans("100 USD to JPY", "15,952 JPY"),
		ans("100 美元换人民币", "666.35 CNY"),
		ans("100美元换人民币", "666.35 CNY"),
		ans("100 美金 换成 人民币", "666.35 CNY"),
		ans("100美元等于多少日元", "15,952 JPY"),
		ans("100美元换多少日元", "15,952 JPY"),
		ans("100美元转日元", "15,952 JPY"),
		ans("¥5000 in usd", "750.36 USD"),
		ans("￥5000 in usd", "750.36 USD"),
		ans("$12", "79.96 CNY"),
		ans("$12.50", "83.29 CNY"),
		ans("12$", "79.96 CNY"),
		ans("10k jpy to cny", "417.72 CNY"),
		ans("1万日元换人民币", "417.72 CNY"),
		ans("5000日元", "208.86 CNY"),
		ans("5000 日币", "208.86 CNY"),
		ans("100 欧元 in 美元", "111.11 USD"),
		ans("100 英镑换人民币", "888.47 CNY"),
		ans("100 港币 in 人民币", "84.89 CNY"),
		ans("100 新台币 in 人民币", "20.23 CNY"),
		ans("100 台币 in 人民币", "20.23 CNY"),
		ans("10000 韩元 in 人民币", "49.92 CNY"),
		ans("100 澳元 in 人民币", "444.23 CNY"),
		ans("100 元 in usd", "15.01 USD"),
		ans("100 块 in usd", "15.01 USD"),
		ans("100 块钱 in usd", "15.01 USD"),
		ans("100 人民币", "15.01 USD"), // already CNY: into USD
		ans("€100", "740.39 CNY"),
		ans("£100 in usd", "133.33 USD"),
		ans("HK$100 in usd", "12.74 USD"),
		ans("NT$1000 in usd", "30.36 USD"),
		ans("₩10000 in usd", "7.49 USD"),
		ans("100 euros in dollars", "111.11 USD"),
		ans("100 pounds in usd", "133.33 USD"),
		ans("100 usd in pounds", "75.00 GBP"),
		ans("100 yen in yuan", "4.18 CNY"),
		ans("$10 + €5", "103.65 CNY"),
		ans("$10 + $5 in jpy", "2,393 JPY"),
		ans("$100 * 3", "1,999.05 CNY"),
		ans("10% of 50 usd in jpy", "798 JPY"),
		ans("0.004 usd in cny", "0.03 CNY"),
		ans("1 usd in usd", "1.00 USD"),
		ans("1000000000 jpy in usd", "6,268,806.42 USD"),
	})

	r, _ := Eval("¥5000", cn)
	if !r.YenByRegion || r.From != "CNY" || r.Unit != "USD" {
		t.Errorf("¥5000 in CN: %+v", r)
	}
	if got := Plain.Equation(r); got != "5,000 CNY = 750.36 USD" {
		t.Errorf("Equation = %q", got)
	}
	r, _ = Eval("100 usd in jpy", cn)
	if r.YenByRegion {
		t.Error("YenByRegion without ¥")
	}
	r, _ = Eval("100 usd in eur", cn)
	if r.RatesDate != "2026-11-12" {
		t.Errorf("RatesDate = %q, want the older rate's", r.RatesDate)
	}

	// Japan: ¥ is JPY; no target converts into JPY, JPY itself into USD.
	jp := Env{Region: "JP", Rates: rates}
	runCases(t, jp, []evalCase{
		ans("¥5000", "31.34 USD"),
		ans("¥5000 in cny", "208.86 CNY"),
		ans("$12", "1,914 JPY"),
		ans("100 人民币", "2,394 JPY"),
		ans("100 jpy in ¥", "100 JPY"),
	})
	r, _ = Eval("¥5000", jp)
	if !r.YenByRegion || r.From != "JPY" {
		t.Errorf("¥5000 in JP: %+v", r)
	}

	// US: USD stays USD; a bare amount isn't a calculation.
	us := Env{Region: "US", Rates: rates}
	runCases(t, us, []evalCase{
		not("$12"),
		ans("$12 + $3", "15.00 USD"),
		ans("100 eur", "111.11 USD"),
		ans("¥100", "15.01 USD"),
	})
	// Unknown region: as the US.
	runCases(t, Env{Rates: rates}, []evalCase{ans("100 eur", "111.11 USD")})
	// A region whose currency has no rate: can't calculate.
	runCases(t, Env{Region: "VN", Rates: rates}, []evalCase{cant("$12")})

	runCases(t, cn, []evalCase{
		cant("100 usd in kg"),
		cant("5 kg in usd"),
		cant("$10 + 5 kg"),
		not("100 btc in usd"),
	})
}

func TestNeedsRates(t *testing.T) {
	env := Env{Region: "CN"}
	for _, in := range []string{"100 usd in jpy", "100 美元换人民币", "$12", "¥5000", "¥5000 in usd", "$10 + €5", "10k jpy to cny"} {
		r, ok := Eval(in, env)
		if !ok || r.Status != NeedsRates || r.Kind != KindCurrency || r.Value != nil || r.Unit == "" {
			t.Errorf("Eval(%q) = %+v, %v", in, r, ok)
		}
		if Plain.Answer(r) != "" || r.Raw() != "" {
			t.Errorf("Eval(%q) answers without rates", in)
		}
	}
	r, _ := Eval("100 usd in jpy", env)
	if r.From != "USD" || r.Unit != "JPY" || r.Amount == nil || r.Amount.Cmp(big.NewRat(100, 1)) != 0 {
		t.Errorf("100 usd in jpy: %+v", r)
	}
	r, _ = Eval("¥5000", Env{Region: "JP"})
	if r.From != "JPY" || r.Unit != "USD" || !r.YenByRegion {
		t.Errorf("¥5000 JP: %+v", r)
	}
	// Same currency needs no rates.
	r, ok := Eval("$1 + $2", Env{Region: "US"})
	if !ok || r.Status != OK || Plain.Answer(r) != "3.00 USD" {
		t.Errorf("$1 + $2 = %+v", r)
	}
	// Units never need rates.
	if r, _ := Eval("10 ft in m", Env{}); r.Status != OK {
		t.Error("units need rates")
	}
	// Errors win over missing rates.
	if r, _ := Eval("100 usd / 0 in jpy", env); r.Status != Error {
		t.Errorf("1/0 with no rates: %+v", r)
	}
}

func TestIncomplete(t *testing.T) {
	env := Env{Region: "CN", Rates: testRates(t)}
	runCases(t, env, []evalCase{
		incomplete("2+"),
		incomplete("2 +"),
		incomplete("2*"),
		incomplete("2^"),
		incomplete("(3*"),
		incomplete("(3*4"),
		incomplete("12 × (3 +"),
		incomplete("sqrt("),
		incomplete("sqrt(2"),
		incomplete("100 usd in"),
		incomplete("100 usd to"),
		incomplete("100 usd in j"),
		incomplete("100 美元换"),
		incomplete("100 美元换成"),
		incomplete("10 ft to"),
		incomplete("10 ft in"),
		incomplete("10 ft to me"),
		incomplete("2+3 to"),
		incomplete("2+$"),
		incomplete("52% of"),
		incomplete("2 x"),
		incomplete("2024-"),
		incomplete("3 in in"),
		incomplete("-3 *"),
		not("("),
		not("-"),
		not("$"),
		not("safari+"),
		not("2 in"),
	})
}

func TestErrors(t *testing.T) {
	env := Env{Region: "CN", Rates: testRates(t)}
	for in, want := range map[string]error{
		"1/0":              ErrDivideByZero,
		"0/0":              ErrDivideByZero,
		"5 / (2-2)":        ErrDivideByZero,
		"sqrt(-1)":         ErrDomain,
		"√-4":              ErrDomain,
		"ln(0)":            ErrDomain,
		"log(-1)":          ErrDomain,
		"(-8)^0.5":         ErrDomain,
		"asin(2)":          ErrDomain,
		"0^-1":             ErrDivideByZero,
		"(-1)!":            ErrDomain,
		"2.5!":             ErrDomain,
		"5000!":            ErrOverflow,
		"9^9^9":            ErrOverflow,
		"10^5000":          ErrOverflow,
		"1e1000^5":         ErrOverflow,
		"5 kg in m":        ErrUnits,
		"10 m * 5 m":       ErrUnits,
		"10 m + 5 kg":      ErrUnits,
		"100 in usd":       ErrUnits,
		"2 / 5 m":          ErrUnits,
		"100 usd in kg":    ErrUnits,
		"30 c in km":       ErrUnits,
		"sqrt(4 m)":        ErrUnits,
		"1/0 + 10 ft in m": ErrDivideByZero,
	} {
		r, ok := Eval(in, env)
		if !ok || r.Status != Error || !errors.Is(r.Err, want) {
			t.Errorf("Eval(%q) = %v %d %v, want error %v", in, ok, r.Status, r.Err, want)
		}
		if Plain.Answer(r) != "" || Plain.Equation(r) != "" || r.Raw() != "" {
			t.Errorf("Eval(%q) has an answer", in)
		}
	}
}

func TestRounding(t *testing.T) {
	runCases(t, Env{}, []evalCase{
		ans("2^40", "1,099,511,627,776"), // exact integers up to 15 digits
		ans("10^14 + 1", "100,000,000,000,001"),
		ans("10^15 * 1", "1e+15"),
		ans("2^1000", "1.071508607e+301"),
		ans("1e400*1e400", "1e+800"),
		ans("1000000000.25 * 10", "1e+10"), // past 10 digits, not an integer
		ans("123456789.123 * 1", "123,456,789.1"),
		ans("9999999999.5 * 1", "1e+10"),
		ans("0.000001 * 1", "0.000001"),
		ans("0.0000001 * 1", "1e-7"),
		ans("1e-9*2", "2e-9"),
		ans("1 / 3e7", "3.333333333e-8"),
		ans("2/3 * -1", "-0.6666666667"),
		ans("0.12345678905 * 1", "0.1234567891"),
		ans("0.12345678904 * 1", "0.123456789"),
		ans("1.5 - 1.5", "0"),
		ans("-0.000000000001 * 1", "-1e-12"),
	})
	cn := Env{Region: "CN", Rates: testRates(t)}
	runCases(t, cn, []evalCase{
		ans("1 usd in jpy", "160 JPY"),      // JPY: 0 decimals (159.52)
		ans("1 usd in krw", "1,335 KRW"),    // KRW: 0 decimals (1334.71)
		ans("1 usd in eur", "0.90 EUR"),     // others: 2 decimals, kept
		ans("0.005 usd in usd", "0.01 USD"), // half away from zero
		ans("1e20 jpy in usd", "6.268806419e+17 USD"),
	})
}

func TestNumberAndFormatter(t *testing.T) {
	cn := Env{Region: "CN", Rates: testRates(t)}
	r, _ := Eval("1234.5678 * 1", cn)
	if got := r.Raw(); got != "1234.5678" {
		t.Errorf("Raw = %q", got)
	}
	n := r.Number()
	if n.Neg || n.Int != "1234" || n.Frac != "5678" || n.Sci {
		t.Errorf("Number = %+v", n)
	}
	de := Formatter{Group: ".", Decimal: ","}
	if got := de.Answer(r); got != "1.234,5678" {
		t.Errorf("de = %q", got)
	}
	fr := Formatter{Group: "\u202f", Decimal: ","}
	if got := fr.Number(Round(big.NewRat(-12345678, 100), -1)); got != "-123\u202f456,78" {
		t.Errorf("fr = %q", got)
	}
	if got := Plain.Number(Round(big.NewRat(1, 1), 2)); got != "1.00" {
		t.Errorf("fixed = %q", got)
	}
	if got := Plain.Number(Round(nil, -1)); got != "0" {
		t.Errorf("nil = %q", got)
	}

	r, _ = Eval("100 usd in jpy", cn)
	if got := de.Equation(r); got != "100 USD = 15.952 JPY" {
		t.Errorf("de Equation = %q", got)
	}
	r, _ = Eval("1000 usd in cny", cn)
	if got := de.Answer(r); got != "6.663,50 CNY" {
		t.Errorf("de currency = %q", got)
	}
	if r.Raw() != "6663.50" {
		t.Errorf("Raw = %q", r.Raw())
	}

	// Durations in Chinese, through the Symbol hook.
	zh := Formatter{Group: ",", Decimal: ".", Symbol: func(code string) string {
		return map[string]string{"day": "天", "h": "小时", "min": "分", "s": "秒"}[code]
	}}
	r, _ = Eval("145 min", cn)
	if got := zh.Answer(r); got != "2 小时 25 分" {
		t.Errorf("zh = %q", got)
	}
	if r.Kind != KindDuration || r.Unit != "h" || r.Raw() != "2.416666667" || len(r.Parts) != 2 {
		t.Errorf("145 min: %+v", r)
	}
	if got := Plain.Equation(r); got != "145 min = 2 h 25 min" {
		t.Errorf("Equation = %q", got)
	}
	r, _ = Eval("1500 min", cn)
	if got := zh.Answer(r); got != "1 天 1 小时" {
		t.Errorf("zh days = %q", got)
	}
	r, _ = Eval("30 c in f", cn)
	if got := zh.Answer(r); got != "86 °F" { // unknown to the hook: default symbol
		t.Errorf("zh °F = %q", got)
	}

	// Expression and Input.
	r, _ = Eval("  （12+8）×3  ", cn)
	if r.Input != "(12+8)*3" || r.Expr != "(12+8)*3" || Plain.Equation(r) != "(12+8)*3 = 60" {
		t.Errorf("Input %q Expr %q", r.Input, r.Expr)
	}
	r, _ = Eval("$10 + €5 in jpy", cn)
	if r.Expr != "$10 + €5" || r.Amount != nil || Plain.Source(r) != "$10 + €5" {
		t.Errorf("Expr = %q", r.Expr)
	}
	r, _ = Eval("5 ft 3 in in cm", cn)
	if r.Amount != nil || Plain.Source(r) != "5 ft 3 in" {
		t.Errorf("compound source = %q", Plain.Source(r))
	}
	r, _ = Eval("100 usd in", cn)
	if r.Input != "100 usd in" {
		t.Errorf("incomplete Input = %q", r.Input)
	}
}

func TestDefaultSymbol(t *testing.T) {
	for code, want := range map[string]string{"degF": "°F", "degC": "°C", "m2": "m²", "JPY": "JPY", "km/h": "km/h", "l": "L", "mu": "亩", "day": "d"} {
		if got := DefaultSymbol(code); got != want {
			t.Errorf("DefaultSymbol(%q) = %q", code, got)
		}
	}
}

// Every unit code is reachable by some name, and the names don't
// shadow each other unexpectedly.
func TestUnitTable(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range []map[string]entry{names, exact} {
		for _, e := range m {
			if e.u != nil {
				seen[e.u.code] = true
			}
		}
	}
	for _, d := range unitDefs {
		if !seen[d.code] {
			t.Errorf("unit %s has no name", d.code)
		}
	}
	for c := range currencyNames {
		if !fiat[c] {
			t.Errorf("currency name for unknown %s", c)
		}
	}
	for _, kw := range []string{"in", "to", "换成", "换", "转"} {
		if !convWords[kw] {
			t.Errorf("keyword %s", kw)
		}
	}
	if e, _ := lookup("pound"); e.u.code != "lb" || e.alt == nil || e.alt.code != "GBP" {
		t.Errorf("pound = %+v", e)
	}
	if e, _ := lookup("K"); e.u.code != "K" {
		t.Error("K is kelvin")
	}
	if _, ok := lookup("k"); ok {
		t.Error("k is a multiplier, not a unit")
	}
	if !strings.Contains(fiatCodes, "TWD") || IsCurrency("BTC") || IsCurrency("XAU") {
		t.Error("fiat list")
	}
}

func TestCurrencyForRegion(t *testing.T) {
	for region, want := range map[string]string{"JP": "JPY", "CN": "CNY", "cn": "CNY", "US": "USD", "DE": "EUR", "TW": "TWD", "HK": "HKD", "": "USD", "ZZ": "USD"} {
		if got := CurrencyForRegion(region); got != want {
			t.Errorf("CurrencyForRegion(%q) = %q", region, got)
		}
	}
	if CurrencyDecimals("JPY") != 0 || CurrencyDecimals("KRW") != 0 || CurrencyDecimals("USD") != 2 {
		t.Error("decimals")
	}
}

func TestNewRates(t *testing.T) {
	if _, err := NewRates("XXX", nil); err == nil {
		t.Error("unknown base")
	}
	if _, err := NewRates("USD", []Quote{{"JPY", "-1", ""}, {"CNY", "abc", ""}}); err == nil {
		t.Error("no valid rates")
	}
	r := testRates(t)
	if r.Base() != "USD" || r.Date() != "2026-11-13" || !r.Has("USD") || r.Has("BTC") {
		t.Errorf("rates = %+v", r)
	}
}

func TestDisplay(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"（12+8）*3", "(12+8)×3"},
		{"10/4", "10÷4"},
		{"2 ** 3", "2 ** 3"},
		{"100 km/h in mph", "100 km/h in mph"},
		{"10 km / 2 h", "10 km ÷ 2 h"},
		{"6 ft/2", "6 ft÷2"},
		{"2×3÷4", "2×3÷4"},
		{"100 美元换日元", "100 美元换日元"},
	} {
		res, _ := Eval(c.in, Env{Region: "US"})
		in := res.Input
		if in == "" {
			in = c.in
		}
		if got := Display(in); got != c.want {
			t.Errorf("Display(%q) = %q, want %q", in, got, c.want)
		}
	}
}
