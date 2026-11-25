// Package settings keeps Gocast's preferences in settings.json: the two
// global shortcuts, the terminal's look and font size, whether the
// floating terminal hides as the user clicks away, its size and position,
// the tips dismissed, the clipboard history's and the calculator's
// switches, the hotkeys of apps and terminal presets, what the search
// leaves out, and 自动检查更新 with the last check.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"
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
	// TerminalPosition is where the floating terminal shows on the
	// display with the pointer, one of TerminalPositions; "" for
	// PositionTop (copy.md 17).
	TerminalPosition string `json:"terminalPosition,omitempty"`
	// TerminalLook is what the floating terminal looks like, one of
	// TerminalLooks; "" for LookGhostty, everyone's from the version
	// that brought it (copy.md 20).
	TerminalLook string `json:"terminalLook,omitempty"`
	// TerminalDir is the folder new plain terminals start in, and presets
	// without a folder of their own, as the user reads it (~/Work); empty
	// for the home folder.
	TerminalDir string `json:"terminalDir,omitempty"`
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
	// AwakeScreenOff: Keep Awake lets the screen turn off, keeping only
	// the Mac from sleeping (the switch "Let the screen turn off while
	// keeping awake", off by default; design.md 2.10, copy.md 18.4).
	AwakeScreenOff bool `json:"keepAwakeScreenOff,omitempty"`
	// AutoUpdate is the switch 「自动检查更新」 (design.md 4.4, 4.5), on
	// by default: nil until MigrateUpdater took it from the updater
	// plugin's updater.json, once.
	AutoUpdate *bool `json:"autoUpdate,omitempty"`
	// UpdatesChecked is when Gocast last checked for updates, by itself
	// or as asked: the next automatic check is a day after it.
	UpdatesChecked time.Time `json:"updatesCheckedAt,omitzero"`

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

	// AppHotkeys open apps, by their bundle IDs, which outlive updates
	// and moves of the app; PresetHotkeys open terminal presets, by their
	// IDs. The values are MyGo's accelerators. An app not installed keeps
	// its hotkey, for when it comes back.
	AppHotkeys    map[string]string `json:"appHotkeys,omitempty"`
	PresetHotkeys map[string]string `json:"presetHotkeys,omitempty"`
	// ActionHotkeys run Gocast's actions (the system's and the window's,
	// package sysact), by their IDs; none by default (copy.md 15).
	ActionHotkeys map[string]string `json:"actionHotkeys,omitempty"`

	// AppHotkeyNames are the names of the apps of AppHotkeys, by bundle
	// ID, as they were when the hotkey was set: what Settings shows for
	// an app no longer installed, rather than its bundle ID. Gone with
	// the hotkey; files of before have none.
	AppHotkeyNames map[string]string `json:"appHotkeyNames,omitempty"`

	// NoteHotkeys open the notes window, by what they open: NoteOpen,
	// NoteNew, NoteSearch; none by default (copy.md 16.8).
	NoteHotkeys map[string]string `json:"noteHotkeys,omitempty"`

	// GocastHotkeys run Gocast's own items that have no shortcut of
	// their own above, by what they are: OwnAddPreset, OwnSettings;
	// AwakeHotkeys start and stop Keep Awake, by its rows: AwakeToggle,
	// or the minutes of one of its lengths ("30"); none by default
	// (design.md 4.4).
	GocastHotkeys map[string]string `json:"gocastHotkeys,omitempty"`
	AwakeHotkeys  map[string]string `json:"keepAwakeHotkeys,omitempty"`

	// What the search panel leaves out (design.md 4.4, Settings › Search
	// & Shortcuts), by the IDs of its items ("app:" + bundle ID,
	// "preset:" + ID, ...), see Hidden: the items hidden one by one, the
	// groups hidden whole (Group...), and, by group, the items shown one
	// by one while their group is hidden. IDs of apps not installed are
	// kept, for when they come back.
	HiddenItems  []string            `json:"hiddenInSearch,omitempty"`
	HiddenGroups []string            `json:"hiddenGroupsInSearch,omitempty"`
	ShownItems   map[string][]string `json:"shownInHiddenGroups,omitempty"`
	// CollapsedGroups are the groups Settings › Search & Shortcuts shows
	// collapsed; none at first.
	CollapsedGroups []string `json:"collapsedShortcutGroups,omitempty"`

	// NotesZoom is the notes' text size in percent, 0 for 100 (design.md
	// 11.4: one of NotesZooms, for every note).
	NotesZoom int `json:"notesZoomPercent,omitempty"`
	// NotesFrame is where the user left the notes window, and its size;
	// nil until it first shows (design.md 11.2).
	NotesFrame *Frame `json:"notesWindow,omitempty"`
}

// Frame is a window's place and size, in DIPs of the desktop.
type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NotesZooms are the steps of the notes' text size, in percent (design.md
// 11.4).
var NotesZooms = []int{75, 85, 100, 115, 130, 150, 175, 200}

// Zoom is the notes' text size in percent: one of NotesZooms, 100 for
// any other (settings.json edited by hand).
func (s Settings) Zoom() int {
	if slices.Contains(NotesZooms, s.NotesZoom) {
		return s.NotesZoom
	}
	return 100
}

// SetZoom makes pct the notes' text size, 100 the default.
func (s *Settings) SetZoom(pct int) {
	s.NotesZoom = pct
	if pct == 100 {
		s.NotesZoom = 0
	}
}

// Calculator reports whether the search panel calculates.
func (s Settings) Calculator() bool { return !s.CalculatorOff }

// AutoUpdates reports whether Gocast checks for updates, and downloads
// them, by itself.
func (s Settings) AutoUpdates() bool { return s.AutoUpdate == nil || *s.AutoUpdate }

// SetAutoUpdates turns the automatic checks on or off.
func (s *Settings) SetAutoUpdates(on bool) { s.AutoUpdate = &on }

// MigrateUpdater takes 「自动检查更新」 and the time of the last check
// from updater.json at path, which MyGo's updater plugin kept before
// Gocast checked for updates itself (design.md 4.5): its automaticChecks,
// on when absent, or when the file is missing or damaged. It does so
// once, the first time AutoUpdate is nil, and only reads the file. It
// reports whether it changed the settings, which are then to be saved.
func (s *Settings) MigrateUpdater(path string) bool {
	if s.AutoUpdate != nil {
		return false
	}
	var old struct {
		AutomaticChecks *bool     `json:"automaticChecks"`
		LastCheck       time.Time `json:"lastCheck"`
	}
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &old) != nil {
			old.AutomaticChecks, old.LastCheck = nil, time.Time{}
		}
	}
	s.SetAutoUpdates(old.AutomaticChecks == nil || *old.AutomaticChecks)
	if s.UpdatesChecked.IsZero() {
		s.UpdatesChecked = old.LastCheck
	}
	return true
}

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

// Where the floating terminal shows, in the work area of the display with
// the pointer (copy.md 17): its top edge centered, the default; its
// center; its bottom edge centered; its left or right edge, centered
// vertically; or just below the pointer, above it where there is no room.
const (
	PositionTop     = "top"
	PositionCenter  = "center"
	PositionBottom  = "bottom"
	PositionLeft    = "left"
	PositionRight   = "right"
	PositionPointer = "pointer"
)

// TerminalPositions are the positions of the floating terminal, in the
// order Settings lists them.
var TerminalPositions = []string{PositionTop, PositionCenter, PositionBottom, PositionLeft, PositionRight, PositionPointer}

// TermPosition is where the floating terminal shows: one of
// TerminalPositions, PositionTop while unset or for any other value
// (settings.json edited by hand, or by a later Gocast).
func (s Settings) TermPosition() string {
	if slices.Contains(TerminalPositions, s.TerminalPosition) {
		return s.TerminalPosition
	}
	return PositionTop
}

// SetTermPosition makes pos where the floating terminal shows, "" again
// for the default.
func (s *Settings) SetTermPosition(pos string) {
	s.TerminalPosition = pos
	if pos == PositionTop {
		s.TerminalPosition = ""
	}
}

// What the floating terminal looks like (design.md 3.3b, copy.md 20): as
// Ghostty out of the box, the default, or Gocast's own frosted glass.
const (
	LookGhostty = "ghostty"
	LookGocast  = "gocast"
)

// TerminalLooks are the looks of the floating terminal, in the order
// Settings lists them.
var TerminalLooks = []string{LookGhostty, LookGocast}

// TermLook is what the floating terminal looks like: one of
// TerminalLooks, LookGhostty while unset or for any other value
// (settings.json edited by hand, or by a later Gocast).
func (s Settings) TermLook() string {
	if slices.Contains(TerminalLooks, s.TerminalLook) {
		return s.TerminalLook
	}
	return LookGhostty
}

// SetTermLook makes look what the floating terminal looks like, "" again
// for the default.
func (s *Settings) SetTermLook(look string) {
	s.TerminalLook = look
	if look == LookGhostty {
		s.TerminalLook = ""
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
