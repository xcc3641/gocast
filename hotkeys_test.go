package main

import (
	"errors"
	"testing"

	"gocast/internal/presets"
	"gocast/internal/settings"
)

func TestCheckHotkey(t *testing.T) {
	// ⌥⇧Space registered for ⌘Space, which Spotlight has: both are the
	// search panel's.
	own := [4][]string{nil, {"Alt+Shift+Space", "Cmd+Space"}, {"Ctrl+`"}, {"Ctrl+Cmd+V"}}
	term, safari, preset := settings.Hotkey{App: "com.apple.Terminal"}, settings.Hotkey{App: "com.apple.Safari"}, settings.Hotkey{Preset: "p1"}
	var st settings.Settings
	st.SetHotkey(term, "Alt+Cmd+T")
	st.SetHotkey(preset, "Ctrl+Alt+1")
	st.SetHotkey(settings.Hotkey{App: "gone.app"}, "Alt+Cmd+G") // not installed: still its

	for _, c := range []struct {
		name  string
		h     settings.Hotkey
		accel string
		want  string
		owner *hotkeyOwner
		err   error
	}{
		{"free", safari, "Cmd+Alt+s", "Alt+Cmd+S", nil, nil},
		{"function key alone", safari, "F5", "F5", nil, nil},
		{"its own again", term, "Cmd+Alt+T", "Alt+Cmd+T", nil, nil},
		{"shift alone", safari, "Shift+S", "", nil, errHotkeyInvalid},
		{"no modifier", safari, "S", "", nil, errHotkeyInvalid},
		{"garbage", safari, "Hyper+S", "", nil, errHotkeyInvalid},
		{"search panel, as set", safari, "Cmd+Space", "", &hotkeyOwner{row: 1}, nil},
		{"search panel, the fallback", safari, "Shift+Alt+Space", "", &hotkeyOwner{row: 1}, nil},
		{"floating terminal", preset, "Ctrl+`", "", &hotkeyOwner{row: 2}, nil},
		{"clipboard", safari, "Cmd+Ctrl+V", "", &hotkeyOwner{row: 3}, nil},
		{"another app", safari, "Alt+Cmd+T", "", &hotkeyOwner{item: term}, nil},
		{"an app not installed", safari, "Alt+Cmd+G", "", &hotkeyOwner{item: settings.Hotkey{App: "gone.app"}}, nil},
		{"a preset", safari, "Alt+Ctrl+1", "", &hotkeyOwner{item: preset}, nil},
	} {
		got, err := checkHotkey(c.h, c.accel, own, st)
		var ce *hotkeyConflictError
		switch {
		case c.owner != nil:
			if !errors.As(err, &ce) || ce.owner != *c.owner {
				t.Errorf("%s: err = %v, want owner %+v", c.name, err, *c.owner)
			}
		case c.err != nil:
			if !errors.Is(err, c.err) {
				t.Errorf("%s: err = %v, want %v", c.name, err, c.err)
			}
		case err != nil || got != c.want:
			t.Errorf("%s: checkHotkey = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestAppHotkeyAction(t *testing.T) {
	for _, c := range []struct {
		name      string
		frontmost string
		hidden    bool
		panel     bool
		want      appAction
	}{
		{"frontmost: hide", "com.apple.Safari", false, false, appHide},
		{"frontmost, spelled otherwise", "com.apple.safari", false, false, appHide},
		{"behind another", "com.apple.Terminal", false, false, appOpen},
		{"frontmost but hidden", "com.apple.Safari", true, false, appOpen},
		{"under the search panel", "com.apple.Safari", false, true, appOpen},
		{"none frontmost", "", false, false, appOpen},
	} {
		if got := appHotkeyAction("com.apple.Safari", c.frontmost, c.hidden, c.panel); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPresetHotkeyAction(t *testing.T) {
	for _, c := range []struct {
		name     string
		shown    bool
		active   int
		tabs     []string
		wantHide bool
		wantTab  int
	}{
		{"no tabs", false, -1, nil, false, -1},
		{"no tab of it", true, 0, []string{"", "other"}, false, -1},
		{"showing its tab: hide", true, 1, []string{"other", "p"}, true, 1},
		{"its tab, hidden: show it", false, 1, []string{"other", "p"}, false, 1},
		{"showing another tab: switch", true, 0, []string{"other", "p"}, false, 1},
		{"two of it: the active one", false, 2, []string{"p", "x", "p"}, false, 2},
		{"two of it, another active: the first", true, 1, []string{"p", "x", "p"}, false, 0},
	} {
		hide, tab := presetHotkeyAction(c.shown, c.active, c.tabs, "p")
		if hide != c.wantHide || tab != c.wantTab {
			t.Errorf("%s: hide %v, tab %d; want %v, %d", c.name, hide, tab, c.wantHide, c.wantTab)
		}
	}
}

// Deleting a preset deletes its hotkey, but presets.json unreadable
// (no presets for now) deletes none.
func TestPrunePresetHotkeys(t *testing.T) {
	kept, deleted := settings.Hotkey{Preset: "kept"}, settings.Hotkey{Preset: "deleted"}
	g := &gocast{dir: t.TempDir(), presets: []presets.Preset{{ID: "kept", Name: "Kept"}}}
	g.settings.SetHotkey(kept, "Ctrl+Alt+1")
	g.settings.SetHotkey(deleted, "Ctrl+Alt+2")
	g.hotkeyStates = map[settings.Hotkey]hotkeyState{kept: hotkeyTaken, deleted: hotkeyTaken}

	g.presetsErr = errors.New("unreadable")
	g.prunePresetHotkeys()
	if g.settings.Hotkey(deleted) == "" {
		t.Fatal("pruned with presets.json unreadable")
	}

	g.presetsErr = nil
	g.prunePresetHotkeys()
	if g.settings.Hotkey(deleted) != "" || g.settings.Hotkey(kept) == "" {
		t.Errorf("after pruning: %v", g.settings.PresetHotkeys)
	}
	if _, ok := g.hotkeyStatus(deleted); ok {
		t.Error("the deleted preset's hotkey has a state")
	}
	saved, err := settings.Load(g.settingsPath())
	if err != nil || saved.Hotkey(deleted) != "" || saved.Hotkey(kept) != "Ctrl+Alt+1" {
		t.Errorf("saved %v, %v", saved.PresetHotkeys, err)
	}
}
