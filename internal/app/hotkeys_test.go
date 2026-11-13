package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"

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

// newHotkeyTest is a Gocast with presets a, b and c, a's hotkey ⌃⌥1 and
// b's ⌃⌥2, registered.
func newHotkeyTest(t *testing.T) (*gocast, *fakeShortcuts) {
	f := useFakeShortcuts(t)
	g := &gocast{dir: t.TempDir(), launcher: &launcher{}, presets: []presets.Preset{
		{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"},
	}}
	g.settings.SetHotkey(settings.Hotkey{Preset: "a"}, "Ctrl+Alt+1")
	g.settings.SetHotkey(settings.Hotkey{Preset: "b"}, "Ctrl+Alt+2")
	g.registerHotkeys()
	return g, f
}

func TestMoveHotkey(t *testing.T) {
	a, b, c := settings.Hotkey{Preset: "a"}, settings.Hotkey{Preset: "b"}, settings.Hotkey{Preset: "c"}

	t.Run("to one without", func(t *testing.T) {
		g, f := newHotkeyTest(t)
		if err := g.moveHotkey(a, c); err != nil {
			t.Fatal(err)
		}
		if g.settings.Hotkey(a) != "" || g.settings.Hotkey(c) != "Ctrl+Alt+1" {
			t.Errorf("settings: a %q, c %q", g.settings.Hotkey(a), g.settings.Hotkey(c))
		}
		if _, ok := g.hotkeys[a]; ok || g.hotkeys[c] != "Ctrl+Alt+1" || !f.on["Ctrl+Alt+1"] {
			t.Errorf("registered %v, system %v", g.hotkeys, f.on)
		}
		if _, ok := g.hotkeyStatus(a); ok {
			t.Error("a still has a state")
		}
		if st, _ := g.hotkeyStatus(c); st != hotkeyOn {
			t.Errorf("c is %v", st)
		}
		saved, err := settings.Load(g.settingsPath())
		if err != nil || saved.Hotkey(a) != "" || saved.Hotkey(c) != "Ctrl+Alt+1" {
			t.Errorf("saved %v, %v", saved.PresetHotkeys, err)
		}
	})

	t.Run("to one with its own", func(t *testing.T) {
		g, f := newHotkeyTest(t)
		if err := g.moveHotkey(a, b); err != nil {
			t.Fatal(err)
		}
		if g.settings.Hotkey(a) != "" || g.settings.Hotkey(b) != "Ctrl+Alt+1" {
			t.Errorf("settings: a %q, b %q", g.settings.Hotkey(a), g.settings.Hotkey(b))
		}
		if f.on["Ctrl+Alt+2"] || !f.on["Ctrl+Alt+1"] || len(g.hotkeys) != 1 {
			t.Errorf("registered %v, system %v", g.hotkeys, f.on)
		}
	})

	t.Run("the system refuses it", func(t *testing.T) {
		g, f := newHotkeyTest(t)
		f.refuse["Ctrl+Alt+1"] = true // as if another app took it as a was let go
		if err := g.moveHotkey(a, b); !errors.Is(err, errShortcutTaken) {
			t.Fatalf("err = %v", err)
		}
		if g.settings.Hotkey(a) != "Ctrl+Alt+1" || g.settings.Hotkey(b) != "Ctrl+Alt+2" {
			t.Errorf("settings: a %q, b %q", g.settings.Hotkey(a), g.settings.Hotkey(b))
		}
		if g.hotkeys[b] != "Ctrl+Alt+2" || !f.on["Ctrl+Alt+2"] {
			t.Errorf("b not registered again: %v, system %v", g.hotkeys, f.on)
		}
		// a's could not come back: the other app has it now.
		if st, _ := g.hotkeyStatus(a); st != hotkeyTaken {
			t.Errorf("a is %v", st)
		}
	})

	t.Run("the system refuses it, the old ones come back", func(t *testing.T) {
		g, f := newHotkeyTest(t)
		calls := 0
		globalShortcuts = &refuseOnce{f, &calls}
		if err := g.moveHotkey(a, b); !errors.Is(err, errShortcutTaken) {
			t.Fatalf("err = %v", err)
		}
		if !f.on["Ctrl+Alt+1"] || !f.on["Ctrl+Alt+2"] || g.hotkeys[a] != "Ctrl+Alt+1" || g.hotkeys[b] != "Ctrl+Alt+2" {
			t.Errorf("registered %v, system %v", g.hotkeys, f.on)
		}
		for _, h := range []settings.Hotkey{a, b} {
			if st, _ := g.hotkeyStatus(h); st != hotkeyOn {
				t.Errorf("%v is %v", h, st)
			}
		}
	})

	t.Run("refused", func(t *testing.T) {
		g, _ := newHotkeyTest(t)
		for _, c := range []struct {
			from, to settings.Hotkey
			err      error
		}{
			{c, a, errHotkeyInvalid}, // c has none
			{a, a, errHotkeyInvalid},
			{a, settings.Hotkey{Preset: "gone"}, errNoPreset},
		} {
			if err := g.moveHotkey(c.from, c.to); !errors.Is(err, c.err) {
				t.Errorf("%v → %v: err = %v, want %v", c.from, c.to, err, c.err)
			}
		}
		if g.settings.Hotkey(a) != "Ctrl+Alt+1" {
			t.Error("a lost its hotkey")
		}
	})
}

// refuseOnce refuses the first shortcut registered, then is fakeShortcuts.
type refuseOnce struct {
	*fakeShortcuts
	calls *int
}

func (r *refuseOnce) Register(accel string, fn func()) error {
	*r.calls++
	if *r.calls == 1 {
		return errShortcutTaken
	}
	return r.fakeShortcuts.Register(accel, fn)
}

func TestHotkeyItems(t *testing.T) {
	g, _ := newHotkeyTest(t)
	g.settings.SetHotkey(settings.Hotkey{App: "com.example.gone"}, "Alt+Cmd+G")
	g.hotkeyStates[settings.Hotkey{App: "com.example.gone"}] = hotkeyMissing
	g.launcher.apps = []mygo.InstalledApp{{ID: "com.apple.Safari", Name: "Safari"}}

	items := g.hotkeyItems()
	var got []string
	for _, it := range items {
		got = append(got, fmt.Sprintf("%v %s %q %v %v", it.item, it.name, it.accel, it.state, it.app != nil))
	}
	want := []string{
		`{ a} A "Ctrl+Alt+1" 0 false`,
		`{ b} B "Ctrl+Alt+2" 0 false`,
		`{ c} C "" 0 false`,
		`{com.apple.Safari } Safari "" 0 true`,
		`{com.example.gone } com.example.gone "Alt+Cmd+G" 2 false`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("hotkeyItems:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// One of Gocast's own shortcuts takes a preset's hotkey (Use Here): the
// preset loses it; refused by the system, both keep theirs.
func TestTakeHotkey(t *testing.T) {
	a := settings.Hotkey{Preset: "a"}
	g, f := newHotkeyTest(t)
	g.terminalKey = "Ctrl+`"
	f.on["Ctrl+`"] = true
	if err := g.takeHotkey(2, a); err != nil {
		t.Fatal(err)
	}
	if g.terminalKey != "Ctrl+Alt+1" || g.settings.TerminalShortcut != "Ctrl+Alt+1" || g.settings.Hotkey(a) != "" {
		t.Errorf("terminal %q (%q), a %q", g.terminalKey, g.settings.TerminalShortcut, g.settings.Hotkey(a))
	}
	if f.on["Ctrl+`"] || !f.on["Ctrl+Alt+1"] || !f.on["Ctrl+Alt+2"] || g.suspended {
		t.Errorf("system %v, suspended %v", f.on, g.suspended)
	}
	if _, ok := g.hotkeyStatus(a); ok {
		t.Error("a still has a state")
	}

	b := settings.Hotkey{Preset: "b"}
	f.refuse["Ctrl+Alt+2"] = true
	if err := g.takeHotkey(3, b); !errors.Is(err, errShortcutTaken) {
		t.Fatalf("err = %v", err)
	}
	if g.settings.Hotkey(b) != "Ctrl+Alt+2" || g.clipKey != "" || g.suspended {
		t.Errorf("b %q, clipboard %q, suspended %v", g.settings.Hotkey(b), g.clipKey, g.suspended)
	}
}
