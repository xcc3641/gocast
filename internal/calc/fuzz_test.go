package calc

import (
	"testing"
	"time"
)

var seeds = []string{
	"2+3*4", "0.1+0.2", "（12+8）×3", "2^10", "52% of 900", "20% off 80", "80 + 10%", "900 的 52%",
	"10 ft in m", "100 斤 in kg", "30 c in f", "2rem in px", "100 usd in jpy", "100 美元换人民币",
	"¥5000 in usd", "$12", "10k jpy to cny", "pi*2", "pi", "safari", "2024", "145 min", "sqrt(625)",
	"4(2+3)", "1,234.5", "2+", "(3*", "100 usd in", "1/0", "sqrt(-1)", "5 kg in m", "9^9^9", "1000!",
	"5 ft 3 in in cm", "1斤2两 in kg", "100 pounds in usd", "HK$100", "NT$1000 in usd", "sin(30°)",
	"log(8, 2)", "round(2.345, 2)", "1e400*1e400", "((((((((((1))))))))))", "1万日元换人民币",
	"100美元等于多少日元", "$10 + €5 in jpy", "2 x 3", "10 in cm", "60 mph in km/h", "1 GiB in MB",
	"2024-11-13", "0^-1", "e^1000", "10^-1000", "(-8)^(1/3)", "1h + 45min", "1.5 day",
}

func FuzzEval(f *testing.F) {
	for _, s := range seeds {
		f.Add(s, "CN", true)
	}
	f.Add("¥5000", "JP", false)
	rates := testRates(f)
	f.Fuzz(func(t *testing.T, in, region string, withRates bool) {
		env := Env{Region: region}
		if withRates {
			env.Rates = rates
		}
		start := time.Now()
		r, ok := Eval(in, env)
		if d := time.Since(start); d > 200*time.Millisecond {
			t.Errorf("Eval(%q) took %v", in, d)
		}
		if !ok {
			return
		}
		switch r.Status {
		case OK:
			if r.Value == nil || Plain.Answer(r) == "" || Plain.Equation(r) == "" || r.Raw() == "" {
				t.Errorf("Eval(%q): OK without an answer: %+v", in, r)
			}
			if r.Kind == KindDuration && len(r.Parts) == 0 {
				t.Errorf("Eval(%q): duration without parts", in)
			}
		case Error:
			if r.Err == nil {
				t.Errorf("Eval(%q): Error without Err", in)
			}
		case NeedsRates:
			if withRates || r.Unit == "" {
				t.Errorf("Eval(%q): NeedsRates %+v", in, r)
			}
		}
	})
}

func TestSeedsQuick(t *testing.T) {
	rates := testRates(t)
	for _, s := range seeds {
		start := time.Now()
		Eval(s, Env{Region: "CN", Rates: rates})
		if d := time.Since(start); d > 50*time.Millisecond {
			t.Errorf("Eval(%q) took %v", s, d)
		}
	}
}

var benchInputs = []string{
	"2+3*4", "0.1+0.2", "（12+8）×3", "52% of 900", "10 ft in m", "100 usd in jpy",
	"100 美元换人民币", "145 min", "sqrt(625) + sin(pi/4)", "safari", "github", "2024",
}

func BenchmarkEval(b *testing.B) {
	env := Env{Region: "CN", Rates: testRates(b)}
	for _, in := range benchInputs {
		b.Run(in, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Eval(in, env)
			}
		})
	}
}

func BenchmarkEvalAndFormat(b *testing.B) {
	env := Env{Region: "CN", Rates: testRates(b)}
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range benchInputs {
			r, _ := Eval(in, env)
			_ = Plain.Answer(r)
		}
	}
}
