package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gocast/internal/calc"
	"gocast/internal/calc/rates"
	"gocast/internal/presets"
	"gocast/internal/usage"
	"gocast/locales"
)

var calcNow = time.Date(2026, 11, 13, 15, 0, 0, 0, time.Local)

// testRates are rates as Frankfurter served them, read from a cache in a
// temporary folder: no network.
func testRates(t *testing.T) *rates.Store {
	t.Helper()
	dir := t.TempDir()
	snap := rates.Snapshot{Source: "test", Base: "USD", Date: "2026-11-13", Fetched: calcNow, Rates: map[string]rates.Quote{
		"JPY": {Rate: "154.32", Date: "2026-11-13"},
		"CNY": {Rate: "7.1", Date: "2026-11-13"},
		"EUR": {Rate: "0.86", Date: "2026-11-12"},
	}}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rates.FileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
	s := rates.New(dir, nil)
	if err := s.Load(); err != nil || s.Rates() == nil {
		t.Fatalf("rates: %v", err)
	}
	return s
}

func eval(t *testing.T, in string, r *calc.Rates) calc.Result {
	t.Helper()
	res, ok := calc.Eval(in, calc.Env{Region: "US", Rates: r})
	if !ok {
		t.Fatalf("%q is not a calculation", in)
	}
	return res
}

func TestCalcCard(t *testing.T) {
	inLocale(t, "en")
	r := testRates(t).Rates()
	f := calc.Formatter{Group: ",", Decimal: ".", Symbol: calcSymbol}

	// Plain arithmetic: no notes (copy.md 7.1).
	c := makeCalcCard(eval(t, "(12+8)*3", r), nil, f, false, calcNow)
	if c.state != cardAnswer || c.expr != "(12+8)×3" || c.answer != "60" || c.from != "" || c.to != "" {
		t.Errorf("arithmetic: %+v", c)
	}
	if c.label != "(12+8) times 3 equals 60, Calculator" || c.number != "60" || c.equation != "(12+8)*3 = 60" {
		t.Errorf("arithmetic: %q %q %q", c.label, c.number, c.equation)
	}

	// Units: their full names under both sides.
	c = makeCalcCard(eval(t, "10 ft in m", r), nil, f, false, calcNow)
	if c.answer != "3.048 m" || c.from != "feet" || c.to != "meters" || c.label != "10 feet equals 3.048 meters, Calculator" {
		t.Errorf("units: %+v", c)
	}

	// Currencies: the rates' day after the target's name.
	c = makeCalcCard(eval(t, "100 usd in jpy", r), nil, f, false, calcNow)
	if c.answer != "15,432 JPY" || c.from != "US dollars" || c.equation != "100 USD = 15,432 JPY" || c.number != "15432" {
		t.Errorf("currency: %+v", c)
	}
	if want := "Japanese yen · Rates from Today"; !strings.EqualFold(c.to, want) {
		t.Errorf("currency's note: %q, want %q", c.to, want)
	}
	if !strings.HasPrefix(c.label, "100 US dollars equals 15,432 Japanese yen, Calculator, rates from ") {
		t.Errorf("currency's label: %q", c.label)
	}

	// Being typed: the last answer, greyed, no actions.
	last := eval(t, "2+3", r)
	c = makeCalcCard(eval(t, "2+3*", r), &last, f, false, calcNow)
	if c.state != cardStale || c.answer != "5" || c.expr != "2+3×" || c.ok() {
		t.Errorf("being typed: %+v", c)
	}
	if c.label != "2+3 times, unfinished, last answer 5, Calculator" {
		t.Errorf("being typed's label: %q", c.label)
	}

	// Can't: no answer, no notes.
	c = makeCalcCard(eval(t, "5 kg in m", r), nil, f, false, calcNow)
	if c.state != cardCant || c.answer != "" || c.from != "" || c.to != "" || c.ok() {
		t.Errorf("can't: %+v", c)
	}
	if c.label != "5 kilograms, can’t calculate this, Calculator" {
		t.Errorf("can't's label: %q", c.label)
	}

	// No rates: offline, or getting them; the target's name under.
	res := eval(t, "100 usd in jpy", nil)
	c = makeCalcCard(res, nil, f, false, calcNow)
	if c.state != cardNeedsRates || c.from != "US dollars" || c.to != "Japanese yen" || c.ok() {
		t.Errorf("no rates: %+v", c)
	}
	if c.label != "100 US dollars, go online once to get exchange rates, Calculator" {
		t.Errorf("no rates' label: %q", c.label)
	}
	if c = makeCalcCard(res, nil, f, true, calcNow); c.state != cardGettingRates || c.label != "100 US dollars, getting exchange rates, Calculator" {
		t.Errorf("getting rates: %+v", c)
	}

	// Chinese: units of time in words, names and operators too.
	inLocale(t, "zh-Hans")
	c = makeCalcCard(eval(t, "100 美元换日元", r), nil, f, false, calcNow)
	if c.from != "美元" || !strings.HasPrefix(c.to, "日元 · 汇率更新于 ") || !strings.HasPrefix(c.label, "100 美元 等于 15,432 日元，计算器，汇率更新于 ") {
		t.Errorf("Chinese: %+v", c)
	}
	if got := speakOps(calc.Display("2^10*3")); got != "2的10次方 乘 3" {
		t.Errorf("spoken in Chinese: %q", got)
	}
}

func TestSpeakOps(t *testing.T) {
	inLocale(t, "en")
	for in, want := range map[string]string{
		"12 × (3 +":  "12 times (3 +",
		"10÷4":       "10 divided by 4",
		"2^10":       "2 to the power of 10",
		"(1+2)**3":   "(1+2) to the power of 3",
		"15% of 80":  "15 percent of 80",
		"100 km/h":   "100 km/h",
		"2 ^ -1 × 4": "2 to the power of -1 times 4",
	} {
		if got := speakOps(in); got != want {
			t.Errorf("speakOps(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCalcNames(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hans"} {
		inLocale(t, locale)
		for code, name := range calcUnitNames {
			if n := name(); n == "" || strings.HasPrefix(n, "calc") {
				t.Errorf("%s: unit %s named %q", locale, code, n)
			}
		}
		for code, name := range calcCurrencyNames {
			if !calc.IsCurrency(code) {
				t.Errorf("%s is not a currency", code)
			}
			if n := name(); n == "" || strings.HasPrefix(n, "calc") {
				t.Errorf("%s: currency %s named %q", locale, code, n)
			}
		}
	}
	// Every currency calc converts has a name: those of the regions'.
	for _, region := range []string{"US", "JP", "CN", "GB", "DE", "KR", "IN", "BR", "CH", "ZA"} {
		if code := calc.CurrencyForRegion(region); calcCurrencyNames[code] == nil {
			t.Errorf("no name for %s", code)
		}
	}
	if calcName("XYZ") != "XYZ" {
		t.Error("an unknown code is not itself")
	}
	inLocale(t, "zh-Hans")
	if got := calcSymbol("h") + calcSymbol("min") + calcSymbol("degF"); got != "小时分°F" {
		t.Errorf("symbols: %q", got)
	}
}

func TestFitHead(t *testing.T) {
	measure := func(s string) float32 { return float32(len([]rune(s))) }
	if got := fitHead("12+8", 10, measure); got != "12+8" {
		t.Errorf("fits: %q", got)
	}
	if got := fitHead("1000 + 2000 + 125 × 3", 10, measure); got != "…+ 125 × 3" { // design.md 2.3a
		t.Errorf("cut: %q", got)
	}
}

// calcLauncher is a search panel with the calculator, and rates.
func calcLauncher(t *testing.T) *launcher {
	t.Helper()
	inLocale(t, "en")
	l := testLauncher(t, usage.New(), []presets.Preset{{ID: "p", Name: "pi", Dir: "~"}})
	l.g.calc = &calcService{g: l.g, store: testRates(t), region: "US", group: ",", decimal: "."}
	l.g.launcher = l
	l.clip = newClipView(l)
	l.shown = true
	return l
}

func typeQuery(l *launcher, q string) {
	l.query = q
	l.filter()
}

// The card is the first row, chosen, while the query is a calculation;
// the panel is 475 high with it, results or none (design.md 2.3a).
func TestCalcCardRows(t *testing.T) {
	l := calcLauncher(t)

	typeQuery(l, "2+") // nothing calculated yet: no card
	if l.calcOn || l.targetHeight() != noResultHeight {
		t.Fatalf("2+: %q, %v", shown(l), l.targetHeight())
	}
	typeQuery(l, "2+3")
	if got := shown(l); !slices.Equal(got, []string{"> calc"}) || l.targetHeight() != expandedHeight {
		t.Fatalf("2+3: %q, %v", got, l.targetHeight())
	}
	if e, ok := l.current(); !ok || !e.calc || l.currentID() != "" {
		t.Errorf("current: %+v", e)
	}
	typeQuery(l, "2+3*") // being typed: the card stays, greyed
	if !l.calcOn || l.card(calcNow).state != cardStale || l.card(calcNow).answer != "5" {
		t.Errorf("2+3*: %v %+v", l.calcOn, l.card(calcNow))
	}
	typeQuery(l, "2+3*4")
	if l.card(calcNow).answer != "14" {
		t.Errorf("2+3*4: %+v", l.card(calcNow))
	}
	typeQuery(l, "safari") // words: no card, the results
	if l.calcOn || slices.Contains(shown(l), "calc") || slices.Contains(shown(l), "> calc") {
		t.Errorf("safari: %q", shown(l))
	}
	typeQuery(l, "2*") // the last answer forgotten with the words between
	if l.calcOn {
		t.Error("2* after words shows a card")
	}

	typeQuery(l, "1/0") // can't: a card, nothing to act on
	if !l.calcOn || l.card(calcNow).state != cardCant || !l.menuDim() {
		t.Errorf("1/0: %+v", l.card(calcNow))
	}

	// Off in Settings: no card; on again, the card.
	l.g.settings.CalculatorOff = true
	l.calcChanged()
	if l.calcOn || len(l.rows) > 0 && l.rows[0].calc {
		t.Errorf("off: %q", shown(l))
	}
	l.g.settings.CalculatorOff = false
	l.calcChanged()
	if got := shown(l); len(got) == 0 || got[0] != "> calc" {
		t.Errorf("on again: %q", got)
	}

	// The actions: the answer's four, dimmed while it can't.
	typeQuery(l, "100 usd in jpy")
	if l.menuDim() {
		t.Error("an answer's actions dimmed")
	}
	var labels []string
	for _, a := range l.actions() {
		labels = append(labels, a.label)
	}
	if want := []string{"Copy Answer", "Paste", "Copy Expression and Answer", "Copy Number Only"}; !slices.Equal(labels, want) {
		t.Errorf("actions: %q", labels)
	}
	l.target.app.Name = "Notes"
	if got := l.actions()[1].label; got != locales.ClipPasteTo("Notes") {
		t.Errorf("paste: %q", got)
	}
}

// The panel draws the card: the expression and the answer, the notes,
// no "no results" under it.
func TestCalcCardDrawn(t *testing.T) {
	l := calcLauncher(t)
	for _, a := range l.apps {
		l.icons[a.Path] = nil
	}
	l.g.clip = &clipService{g: l.g, noAX: true}
	tt := ui.NewTester(l.view, windowWidth, windowHeight)
	l.enterSearch(calcNow)
	typeQuery(l, "10 ft in m")
	tt.Frame()
	texts := tt.Texts()
	for _, want := range []string{"10 ft in m", "3.048 m", "feet", "meters", "Copy Answer"} {
		if !slices.Contains(texts, want) {
			t.Errorf("no %q in %q", want, texts)
		}
	}
	if slices.ContainsFunc(texts, func(s string) bool { return strings.Contains(s, "No results") }) {
		t.Errorf("no results under the card: %q", texts)
	}

	typeQuery(l, "100 usd in eur")
	l.calcRes.Status = calc.NeedsRates // as with no rates, offline
	tt.Frame()
	if texts := tt.Texts(); !slices.Contains(texts, locales.CalcNeedsRates()) || !slices.Contains(texts, "euros") || slices.Contains(texts, "Copy Answer") {
		t.Errorf("no rates: %q", texts)
	}
}
