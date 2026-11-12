// Package settings keeps Gocast's preferences in settings.json: the two
// global shortcuts, the terminal's font size, whether the floating
// terminal hides as the user clicks away, and the tips dismissed.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	DefaultLauncherShortcut  = "Alt+Shift+Space"
	DefaultTerminalShortcut  = "Ctrl+`"
	DefaultFontSize          = 13
	MinFontSize, MaxFontSize = 11, 20
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
