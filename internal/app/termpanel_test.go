package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo"

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
		// Unset: 50% by 60% (design.md 3.2), with no cap on the width.
		{"default", 1512, 949, settings.Settings{}, 756, 569},
		{"wide display", 2560, 1400, settings.Settings{}, 1280, 840},
		{"30% wide: used as 43%, never under 640", 1512, 949, settings.Settings{TerminalWidth: 30}, 650, 569},
		{"30% of a wide display", 2560, 1400, settings.Settings{TerminalWidth: 30}, 768, 840},
		{"60×40", 1512, 949, settings.Settings{TerminalWidth: 60, TerminalHeight: 40}, 907, 380},
		{"63×37", 1512, 949, settings.Settings{TerminalWidth: 63, TerminalHeight: 37}, 953, 351},
		{"largest", 2560, 1400, settings.Settings{TerminalWidth: 100, TerminalHeight: 90}, 2560, 1260},
		// Never under 640×300, never over the work area.
		{"minimum", 1000, 800, settings.Settings{TerminalWidth: 50, TerminalHeight: 30}, 640, 300},
		{"tiny work area", 600, 280, settings.Settings{TerminalWidth: 50, TerminalHeight: 30}, 600, 280},
		// Out of range values from a hand-edited file are clamped.
		{"clamped", 1000, 1000, settings.Settings{TerminalWidth: 20, TerminalHeight: 99}, 640, 900},
		{"clamped to 30% wide", 2560, 1400, settings.Settings{TerminalWidth: 10}, 768, 840},
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
		{1000, 1512, 50, 100, 66}, // 66.1%
		{1050, 1512, 50, 100, 69}, // 69.4%
		{1060, 1512, 50, 100, 70}, // 70.1%
		{640, 1512, 30, 100, 42},  // 42.3%: the minimum width
		{400, 1512, 30, 100, 30},  // under the range
		{1512, 1512, 50, 100, 100},
		{300, 949, 30, 90, 32}, // 31.6%
		{200, 949, 30, 90, 30}, // under the range
		{949, 949, 30, 90, 90}, // dragged to the bottom of the screen
		{522, 949, 30, 90, 55}, // 55.0%
	} {
		if got := termPercent(c.px, c.wa, c.lo, c.hi); got != c.want {
			t.Errorf("termPercent(%d, %d) = %d, want %d", c.px, c.wa, got, c.want)
		}
	}
	// A size set from a percentage reads back as that percentage.
	for pct := settings.MinTermWidth(1512); pct <= settings.MaxTerminalWidth; pct++ {
		w, _ := termSize(1512, 949, settings.Settings{TerminalWidth: pct})
		if w > termMinWidth {
			if got := termPercent(w, 1512, settings.MinTerminalWidth, settings.MaxTerminalWidth); got != pct {
				t.Errorf("width %d%% → %d px → %d%%", pct, w, got)
			}
		}
	}
	for pct := settings.MinTerminalHeight; pct <= settings.MaxTerminalHeight; pct++ {
		_, h := termSize(1512, 949, settings.Settings{TerminalHeight: pct})
		if h > termMinHeight {
			if got := termPercent(h, 949, settings.MinTerminalHeight, settings.MaxTerminalHeight); got != pct {
				t.Errorf("height %d%% → %d px → %d%%", pct, h, got)
			}
		}
	}
}

func TestDragSize(t *testing.T) {
	wa := mygo.Rectangle{X: 0, Y: 33, Width: 1512, Height: 949}
	prev := mygo.Rectangle{X: 302, Y: 41, Width: 908, Height: 380} // centered at 756
	r := func(x, y, w, h int) mygo.Rectangle { return mygo.Rectangle{X: x, Y: y, Width: w, Height: h} }
	for _, c := range []struct {
		name         string
		b            mygo.Rectangle
		wantW, wantH int
	}{
		{"bottom edge down", r(302, 41, 908, 480), 908, 480},
		{"bottom edge up", r(302, 41, 908, 330), 908, 330},
		{"left edge out by 50: both sides", r(252, 41, 958, 380), 1008, 380},
		{"left edge in by 40", r(342, 41, 868, 380), 828, 380},
		{"right edge out by 30: both sides", r(302, 41, 938, 380), 968, 380},
		{"bottom-right corner", r(302, 41, 938, 420), 968, 420},
		{"top edge: nothing", r(302, 21, 908, 400), 908, 380},
		{"under the minimum", r(302, 41, 908, 120), 908, 300},
		{"over 90% high", r(302, 41, 908, 940), 908, 854},
		{"wider than the screen", r(0, 41, 1300, 380), 1512, 380},
		{"under 43% wide (650)", r(500, 41, 512, 380), 650, 380},
	} {
		w, h := dragSize(prev, c.b, wa)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: dragSize = %d×%d, want %d×%d", c.name, w, h, c.wantW, c.wantH)
		}
	}
}
