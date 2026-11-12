package main

import (
	"math"
	"testing"

	"gocast/internal/settings"
)

func TestCubicBezier(t *testing.T) {
	for _, ease := range []func(float64) float64{easeIn, easeOut} {
		if ease(0) != 0 || ease(1) != 1 {
			t.Fatalf("ends: %v, %v", ease(0), ease(1))
		}
		prev := 0.0
		for i := 1; i <= 100; i++ {
			v := ease(float64(i) / 100)
			if v < prev-1e-9 {
				t.Fatalf("not monotonic at %d: %v < %v", i, v, prev)
			}
			prev = v
		}
	}
	// Ease-out is ahead of linear time early, ease-in behind it.
	if easeOut(0.25) <= 0.25 || easeIn(0.25) >= 0.25 {
		t.Fatalf("easeOut(.25) = %v, easeIn(.25) = %v", easeOut(0.25), easeIn(0.25))
	}
	// The linear curve is the identity.
	if v := cubicBezier(1.0/3, 1.0/3, 2.0/3, 2.0/3, 0.4); math.Abs(v-0.4) > 1e-6 {
		t.Fatalf("linear(0.4) = %v", v)
	}
}

func TestTermSize(t *testing.T) {
	for _, c := range []struct {
		name         string
		waW, waH     int
		st           settings.Settings
		wantW, wantH int
	}{
		// Unset: 80% up to 1200, half the height, at least 360.
		{"default", 1512, 945, settings.Settings{}, 1200, 473},
		{"default small", 1000, 600, settings.Settings{}, 800, 360},
		// Set: no 1200 cap.
		{"80 set", 1512, 945, settings.Settings{TerminalWidth: 80, TerminalHeight: 50}, 1210, 473},
		{"60×40", 1512, 945, settings.Settings{TerminalWidth: 60, TerminalHeight: 40}, 907, 378},
		{"wide display", 2560, 1400, settings.Settings{TerminalWidth: 100, TerminalHeight: 90}, 2560, 1260},
		// Never under 480×240, never over the work area.
		{"minimum", 800, 600, settings.Settings{TerminalWidth: 50, TerminalHeight: 30}, 480, 240},
		{"tiny work area", 400, 200, settings.Settings{TerminalWidth: 50, TerminalHeight: 30}, 400, 200},
		// Out of range values from a hand-edited file are clamped.
		{"clamped", 1000, 1000, settings.Settings{TerminalWidth: 20, TerminalHeight: 99}, 500, 900},
	} {
		w, h := termSize(c.waW, c.waH, c.st)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: termSize = %d×%d, want %d×%d", c.name, w, h, c.wantW, c.wantH)
		}
	}
}

func TestTermPercent(t *testing.T) {
	for _, c := range []struct {
		px, wa, lo, hi, want int
	}{
		{907, 1512, 50, 100, 60},  // 59.99%
		{1000, 1512, 50, 100, 65}, // 66.1%
		{1050, 1512, 50, 100, 70}, // 69.4%
		{480, 1512, 50, 100, 50},  // the minimum width is under 50%
		{1512, 1512, 50, 100, 100},
		{240, 945, 30, 90, 30}, // 25.4%
		{945, 945, 30, 90, 90}, // dragged to the bottom of the screen
		{520, 945, 30, 90, 55}, // 55.0%
		{544, 945, 30, 90, 60}, // 57.6%: rounds up past the half step
		{540, 945, 30, 90, 55}, // 57.1%
	} {
		if got := termPercent(c.px, c.wa, c.lo, c.hi); got != c.want {
			t.Errorf("termPercent(%d, %d) = %d, want %d", c.px, c.wa, got, c.want)
		}
	}
	// A size set from a percentage reads back as that percentage.
	for pct := settings.MinTerminalWidth; pct <= settings.MaxTerminalWidth; pct += settings.SizeStep {
		w, _ := termSize(1512, 945, settings.Settings{TerminalWidth: pct, TerminalHeight: 50})
		if w > termMinWidth {
			if got := termPercent(w, 1512, settings.MinTerminalWidth, settings.MaxTerminalWidth); got != pct {
				t.Errorf("width %d%% → %d px → %d%%", pct, w, got)
			}
		}
	}
	for pct := settings.MinTerminalHeight; pct <= settings.MaxTerminalHeight; pct += settings.SizeStep {
		_, h := termSize(1512, 945, settings.Settings{TerminalWidth: 80, TerminalHeight: pct})
		if h > termMinHeight {
			if got := termPercent(h, 945, settings.MinTerminalHeight, settings.MaxTerminalHeight); got != pct {
				t.Errorf("height %d%% → %d px → %d%%", pct, h, got)
			}
		}
	}
}
