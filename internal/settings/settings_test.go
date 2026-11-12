package settings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSnapPercent(t *testing.T) {
	for _, c := range []struct {
		pct    float64
		lo, hi int
		want   int
	}{
		{80, 50, 100, 80},
		{62.4, 50, 100, 62},
		{62.5, 50, 100, 63}, // halves round up
		{59.99, 50, 100, 60},
		{12, 50, 100, 50},   // below the range
		{140, 50, 100, 100}, // above it
		{90.4, 30, 90, 90},
		{91, 30, 90, 90},
		{29.6, 30, 90, 30},
		{33.3, 30, 90, 33},
	} {
		if got := SnapPercent(c.pct, c.lo, c.hi); got != c.want {
			t.Errorf("SnapPercent(%v, %d, %d) = %d, want %d", c.pct, c.lo, c.hi, got, c.want)
		}
	}
}

func TestTerminalSize(t *testing.T) {
	var s Settings
	if s.TermWidth() != 50 || s.TermHeight() != 60 { // design.md 3.2
		t.Fatalf("defaults: %d×%d", s.TermWidth(), s.TermHeight())
	}
	if MinTerminalWidth != 30 || MaxTerminalWidth != 100 || MinTerminalHeight != 30 || MaxTerminalHeight != 90 {
		t.Fatalf("ranges: width %d–%d, height %d–%d", MinTerminalWidth, MaxTerminalWidth, MinTerminalHeight, MaxTerminalHeight)
	}
	s.TerminalWidth, s.TerminalHeight = 20, 200 // edited by hand
	if s.TermWidth() != MinTerminalWidth || s.TermHeight() != MaxTerminalHeight {
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

func TestLauncherShortcut(t *testing.T) {
	for _, c := range []struct {
		name          string
		s             Settings
		want, wantDef string
	}{
		{"default", Settings{}, "Cmd+Space", "Cmd+Space"},
		{"chose ⌥⇧Space", Settings{LauncherFallback: true}, "Alt+Shift+Space", "Alt+Shift+Space"},
		{"recorded", Settings{LauncherShortcut: "Ctrl+Alt+K"}, "Ctrl+Alt+K", "Cmd+Space"},
		{"recorded after choosing", Settings{LauncherShortcut: "Ctrl+Alt+K", LauncherFallback: true}, "Ctrl+Alt+K", "Alt+Shift+Space"},
	} {
		if got := c.s.Launcher(); got != c.want {
			t.Errorf("%s: Launcher = %q, want %q", c.name, got, c.want)
		}
		if got := c.s.DefaultLauncher(); got != c.wantDef {
			t.Errorf("%s: DefaultLauncher = %q, want %q", c.name, got, c.wantDef)
		}
	}
}

func TestShortcutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := Settings{LauncherFallback: true, SpotlightChecked: true}
	if err := in.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"launcherDefaultAltShiftSpace": true`) || !strings.Contains(string(data), `"spotlightChecked": true`) {
		t.Errorf("settings.json = %s", data)
	}
	out, err := Load(path)
	if err != nil || out != in {
		t.Errorf("Load = %+v, %v; want %+v", out, err, in)
	}
	// An empty file, as a first launch's, checks Spotlight.
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _ := Load(path); out.SpotlightChecked || out.Launcher() != DefaultLauncherShortcut {
		t.Errorf("{} loads as %+v", out)
	}
}

func TestMinTermWidth(t *testing.T) {
	for _, c := range []struct{ workArea, want int }{
		{1512, 43}, // 42.3: up
		{2560, 30}, // 25: at least 30
		{2133, 31}, // 30.004
		{2134, 30}, // 29.99
		{1280, 50}, // exactly
		{1600, 40},
		{640, 100},
		{500, 100}, // never over 100
		{0, 30},    // none known
	} {
		if got := MinTermWidth(c.workArea); got != c.want {
			t.Errorf("MinTermWidth(%d) = %d, want %d", c.workArea, got, c.want)
		}
	}
}

func TestTermWidthOn(t *testing.T) {
	for _, c := range []struct {
		set, workArea, want int
	}{
		{30, 1512, 43}, // used no less than the least
		{30, 2560, 30},
		{0, 1512, 50}, // the default
		{60, 1512, 60},
	} {
		s := Settings{TerminalWidth: c.set}
		if got := s.TermWidthOn(c.workArea); got != c.want {
			t.Errorf("TermWidthOn(%d) with %d set = %d, want %d", c.workArea, c.set, got, c.want)
		}
		if s.TerminalWidth != c.set {
			t.Errorf("TerminalWidth became %d", s.TerminalWidth)
		}
	}
}

func TestClipboardSettings(t *testing.T) {
	var s Settings
	if s.Clipboard() != "Ctrl+Cmd+V" || !s.PasteOnReturn() {
		t.Errorf("defaults: %q, paste %v", s.Clipboard(), s.PasteOnReturn())
	}
	defaults := []string{"a", "b"}
	if got := s.IgnoredApps(defaults); !slices.Equal(got, defaults) {
		t.Errorf("unset ignored apps = %v", got)
	}
	s.SetIgnoredApps(nil, defaults) // all removed: empty, not the defaults
	if got := s.IgnoredApps(defaults); got == nil || len(got) != 0 {
		t.Errorf("emptied ignored apps = %#v", got)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"clipboardIgnoredApps": []`) {
		t.Errorf("saved %s", data)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.IgnoredApps(defaults); got == nil || len(got) != 0 {
		t.Errorf("emptied ignored apps load as %#v", got)
	}
	out.SetIgnoredApps([]string{"a", "b"}, defaults) // the defaults again: unset
	if out.ClipboardIgnoredApps != nil {
		t.Errorf("defaults kept as %v", *out.ClipboardIgnoredApps)
	}
	out.SetIgnoredApps([]string{"c"}, defaults)
	if got := out.IgnoredApps(defaults); !slices.Equal(got, []string{"c"}) {
		t.Errorf("ignored apps = %v", got)
	}
}
