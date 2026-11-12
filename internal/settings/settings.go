// Package settings keeps Gocast's preferences in settings.json: the two
// global shortcuts, the terminal's font size, whether the floating
// terminal hides as the user clicks away, its size, and the tips
// dismissed.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
)

const (
	DefaultLauncherShortcut  = "Alt+Shift+Space"
	DefaultTerminalShortcut  = "Ctrl+`"
	DefaultFontSize          = 13
	MinFontSize, MaxFontSize = 11, 20

	// The floating terminal's size, in percent of the work area of the
	// display it shows on, in steps of SizeStep.
	DefaultTerminalWidth                 = 80
	MinTerminalWidth, MaxTerminalWidth   = 50, 100
	DefaultTerminalHeight                = 50
	MinTerminalHeight, MaxTerminalHeight = 30, 90
	SizeStep                             = 5
)

// Settings is what settings.json keeps. The zero value is the defaults.
type Settings struct {
	// The global shortcuts, as MyGo's accelerators; "" is the default.
	LauncherShortcut string `json:"launcherShortcut,omitempty"`
	TerminalShortcut string `json:"terminalShortcut,omitempty"`
	// FontSize of the terminal, 0 for DefaultFontSize.
	FontSize int `json:"fontSize,omitempty"`
	// KeepTerminalOnBlur leaves the floating terminal on screen as the
	// user goes to another window (the switch is "hide when you click
	// away", on by default).
	KeepTerminalOnBlur bool `json:"keepTerminalOnBlur,omitempty"`
	// TerminalWidth and TerminalHeight are the floating terminal's size,
	// in percent of the work area; 0 until the user sets them (Settings,
	// or dragging the panel's edges).
	TerminalWidth  int `json:"terminalWidthPercent,omitempty"`
	TerminalHeight int `json:"terminalHeightPercent,omitempty"`
	// PlainTipDismissed: the tip over a plain terminal, closed for good.
	PlainTipDismissed bool `json:"plainTerminalTipDismissed,omitempty"`
}

// Launcher is the search panel's shortcut.
func (s Settings) Launcher() string {
	if s.LauncherShortcut == "" {
		return DefaultLauncherShortcut
	}
	return s.LauncherShortcut
}

// Terminal is the floating terminal's shortcut.
func (s Settings) Terminal() string {
	if s.TerminalShortcut == "" {
		return DefaultTerminalShortcut
	}
	return s.TerminalShortcut
}

// Font is the terminal's font size, within its bounds.
func (s Settings) Font() int {
	if s.FontSize == 0 {
		return DefaultFontSize
	}
	return min(max(s.FontSize, MinFontSize), MaxFontSize)
}

// TermWidth is the floating terminal's width in percent of the work area,
// the default while unset.
func (s Settings) TermWidth() int {
	if s.TerminalWidth == 0 {
		return DefaultTerminalWidth
	}
	return SnapPercent(float64(s.TerminalWidth), MinTerminalWidth, MaxTerminalWidth)
}

// TermHeight is the floating terminal's height in percent of the work
// area, the default while unset.
func (s Settings) TermHeight() int {
	if s.TerminalHeight == 0 {
		return DefaultTerminalHeight
	}
	return SnapPercent(float64(s.TerminalHeight), MinTerminalHeight, MaxTerminalHeight)
}

// SnapPercent rounds pct to the nearest multiple of SizeStep (halves up),
// within lo and hi.
func SnapPercent(pct float64, lo, hi int) int {
	n := int(math.Floor(pct/SizeStep+0.5)) * SizeStep
	return min(max(n, lo), hi)
}

// Load reads the settings at path: the defaults where there is none or it
// can't be read.
func Load(path string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// Save writes the settings to path.
func (s Settings) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
