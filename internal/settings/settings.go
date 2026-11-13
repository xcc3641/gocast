// Package settings keeps Gocast's preferences in settings.json: the two
// global shortcuts, the terminal's font size, whether the floating
// terminal hides as the user clicks away, its size, the tips dismissed,
// the clipboard history's and the calculator's switches.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
)

const (
	// The search panel's shortcut is ⌘Space, or ⌥⇧Space while Spotlight
	// has ⌘Space, or once the user chose it over ⌘Space.
	DefaultLauncherShortcut  = "Cmd+Space"
	FallbackLauncherShortcut = "Alt+Shift+Space"
	DefaultTerminalShortcut  = "Ctrl+`"
	DefaultClipboardShortcut = "Ctrl+Cmd+V"
	DefaultFontSize          = 13
	MinFontSize, MaxFontSize = 11, 20

	// The floating terminal's size, in percent of the work area of the
	// display it shows on, in steps of SizeStep (design.md 4.4).
	DefaultTerminalWidth                 = 50
	MinTerminalWidth, MaxTerminalWidth   = 30, 100
	MinTerminalPixels                    = 640 // the least it is wide, in DIPs, which may be over MinTerminalWidth
	DefaultTerminalHeight                = 60
	MinTerminalHeight, MaxTerminalHeight = 30, 90
	SizeStep                             = 1
)

// Settings is what settings.json keeps. The zero value is the defaults.
type Settings struct {
	// The global shortcuts, as MyGo's accelerators; "" is the default.
	LauncherShortcut string `json:"launcherShortcut,omitempty"`
	TerminalShortcut string `json:"terminalShortcut,omitempty"`
	// LauncherFallback: the user chose ⌥⇧Space over ⌘Space, which
	// Spotlight had ("Use ⌥⇧Space"); it is the default since.
	LauncherFallback bool `json:"launcherDefaultAltShiftSpace,omitempty"`
	// SpotlightChecked: a launch checked whether Spotlight had ⌘Space,
	// and said so; the next ones don't.
	SpotlightChecked bool `json:"spotlightChecked,omitempty"`
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
	// SearchOnly: the search panel shows its search field alone as it
	// opens, without the suggestions (the switch "Show only the search
	// field", off by default; design.md 2.5a).
	SearchOnly bool `json:"searchFieldOnly,omitempty"`
	// CalculatorOff: the search panel calculates nothing and Gocast
	// fetches no exchange rates (the switch "Calculate in search", on by
	// default; design.md 2.3a, 4.4).
	CalculatorOff bool `json:"calculatorOff,omitempty"`

	// The clipboard history (design.md 4.4 剪贴板). ClipboardShortcut
	// opens it, "" for the default ⌃⌘V.
	ClipboardShortcut string `json:"clipboardShortcut,omitempty"`
	// ClipboardOff: the history records nothing (the switch "Keep
	// clipboard history", on by default).
	ClipboardOff bool `json:"clipboardHistoryOff,omitempty"`
	// ClipboardRetention is how long unpinned items are kept, as
	// clipboard.Retention spells it ("1d", "1w", "1m", "3m", "forever");
	// "" for a month.
	ClipboardRetention string `json:"clipboardRetention,omitempty"`
	// ClipboardCopyOnly: Return only copies, not pastes (the switch
	// "Press Return to paste", on by default; "Just Copy" turns it off).
	ClipboardCopyOnly bool `json:"clipboardReturnCopiesOnly,omitempty"`
	// ClipboardIgnoredApps are the bundle IDs of the apps whose copies
	// are not kept; nil until the user changes them, for the defaults,
	// and an empty list once the user removed them all.
	ClipboardIgnoredApps *[]string `json:"clipboardIgnoredApps,omitempty"`
}

// Calculator reports whether the search panel calculates.
func (s Settings) Calculator() bool { return !s.CalculatorOff }

// Clipboard is the clipboard history's shortcut.
func (s Settings) Clipboard() string {
	if s.ClipboardShortcut == "" {
		return DefaultClipboardShortcut
	}
	return s.ClipboardShortcut
}

// PasteOnReturn reports whether Return pastes the item chosen in the
// clipboard history, rather than only copying it.
func (s Settings) PasteOnReturn() bool { return !s.ClipboardCopyOnly }

// IgnoredApps are the bundle IDs of the apps whose copies the history does
// not keep: defaults until the user changed the list.
func (s Settings) IgnoredApps(defaults []string) []string {
	if s.ClipboardIgnoredApps == nil {
		return append([]string(nil), defaults...)
	}
	return append([]string{}, (*s.ClipboardIgnoredApps)...)
}

// SetIgnoredApps keeps the list of ignored apps, nil again when it is the
// defaults.
func (s *Settings) SetIgnoredApps(ids, defaults []string) {
	if slices.Equal(ids, defaults) {
		s.ClipboardIgnoredApps = nil
		return
	}
	list := append([]string{}, ids...)
	s.ClipboardIgnoredApps = &list
}

// Launcher is the search panel's shortcut.
func (s Settings) Launcher() string {
	if s.LauncherShortcut == "" {
		return s.DefaultLauncher()
	}
	return s.LauncherShortcut
}

// DefaultLauncher is the search panel's default shortcut: ⌘Space, or
// ⌥⇧Space once the user chose it over ⌘Space.
func (s Settings) DefaultLauncher() string {
	if s.LauncherFallback {
		return FallbackLauncherShortcut
	}
	return DefaultLauncherShortcut
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

// MinTermWidth is the least width the floating terminal has, in percent
// of a work area workArea wide: MinTerminalPixels of it, rounded up, at
// least MinTerminalWidth (design.md 4.4): 43 on a 1512 one, 30 on a 2560
// one.
func MinTermWidth(workArea int) int {
	if workArea <= 0 {
		return MinTerminalWidth
	}
	pct := (MinTerminalPixels*100 + workArea - 1) / workArea
	return min(max(pct, MinTerminalWidth), MaxTerminalWidth)
}

// TermWidthOn is the floating terminal's width in percent of a work area
// workArea wide: TermWidth, at least MinTermWidth. TerminalWidth stays
// what was set, for a wider display.
func (s Settings) TermWidthOn(workArea int) int {
	return max(s.TermWidth(), MinTermWidth(workArea))
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
