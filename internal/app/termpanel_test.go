package app

import (
	"math"
	"strings"
	"testing"

	"gocast/internal/presets"
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

// Plain terminals and presets without a folder start in the default
// folder of Settings; presets with one, in theirs (copy.md 4.3).
func TestDefaultFolder(t *testing.T) {
	home := presets.ExpandHome("~")
	g := &gocast{}
	if got := g.workDir(presets.Preset{}); got != home {
		t.Errorf("unset: %q", got)
	}
	g.settings.TerminalDir = "~/Work"
	if got, short := g.workDir(presets.Preset{}), g.shortDir(presets.Preset{}); got != home+"/Work" || short != "~/Work" {
		t.Errorf("set: %q %q", got, short)
	}
	pr := presets.Preset{Dir: "/tmp"}
	if got, short := g.workDir(pr), g.shortDir(pr); got != "/tmp" || short != "/tmp" {
		t.Errorf("own folder: %q %q", got, short)
	}
	if got := g.presetSubtitle(presets.Preset{Command: "pi"}); !strings.Contains(got, "~/Work") {
		t.Errorf("subtitle: %q", got)
	}
}
