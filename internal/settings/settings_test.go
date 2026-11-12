package settings

import (
	"path/filepath"
	"testing"
)

func TestSnapPercent(t *testing.T) {
	for _, c := range []struct {
		pct    float64
		lo, hi int
		want   int
	}{
		{80, 50, 100, 80},
		{82.4, 50, 100, 80},
		{82.5, 50, 100, 85}, // halves round up
		{87.6, 50, 100, 90},
		{12, 50, 100, 50},   // below the range
		{140, 50, 100, 100}, // above it
		{91, 30, 90, 90},
		{27.6, 30, 90, 30},
		{33, 30, 90, 35},
	} {
		if got := SnapPercent(c.pct, c.lo, c.hi); got != c.want {
			t.Errorf("SnapPercent(%v, %d, %d) = %d, want %d", c.pct, c.lo, c.hi, got, c.want)
		}
	}
}

func TestTerminalSize(t *testing.T) {
	var s Settings
	if s.TermWidth() != DefaultTerminalWidth || s.TermHeight() != DefaultTerminalHeight {
		t.Fatalf("defaults: %d×%d", s.TermWidth(), s.TermHeight())
	}
	s.TerminalWidth, s.TerminalHeight = 63, 200 // edited by hand
	if s.TermWidth() != 65 || s.TermHeight() != 90 {
		t.Fatalf("snapped: %d×%d", s.TermWidth(), s.TermHeight())
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	s = Settings{TerminalWidth: 60, TerminalHeight: 40}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.TermWidth() != 60 || got.TermHeight() != 40 {
		t.Fatalf("Load = %+v, %v", got, err)
	}
}
