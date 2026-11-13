package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHotkeysRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	var s Settings
	s.SetHotkey(Hotkey{App: "com.apple.Terminal"}, "Alt+Cmd+T")
	s.SetHotkey(Hotkey{App: "com.apple.Safari"}, "Alt+Cmd+S")
	s.SetHotkey(Hotkey{Preset: "a1b2c3"}, "Ctrl+Alt+1")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{`"appHotkeys": {`, `"com.apple.Terminal": "Alt+Cmd+T"`, `"presetHotkeys": {`, `"a1b2c3": "Ctrl+Alt+1"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("settings.json has no %s: %s", want, data)
		}
	}
	out, err := Load(path)
	if err != nil || !reflect.DeepEqual(out, s) {
		t.Fatalf("Load = %+v, %v; want %+v", out, err, s)
	}
	if got := out.Hotkey(Hotkey{App: "com.apple.Safari"}); got != "Alt+Cmd+S" {
		t.Errorf("Safari's hotkey = %q", got)
	}

	// Removed, all of them: the maps are left out of the file.
	for _, h := range out.Hotkeys() {
		out.RemoveHotkey(h)
	}
	if out.AppHotkeys != nil || out.PresetHotkeys != nil {
		t.Errorf("emptied maps kept: %v, %v", out.AppHotkeys, out.PresetHotkeys)
	}
	if err := out.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "Hotkeys") {
		t.Errorf("empty hotkeys written: %s", data)
	}
}

// A settings.json of a build before hotkeys reads as before, with none.
func TestHotkeysOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	old := `{"launcherShortcut": "Ctrl+Alt+K", "fontSize": 14, "someFutureKey": true}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil || s.Launcher() != "Ctrl+Alt+K" || s.Font() != 14 || len(s.Hotkeys()) != 0 {
		t.Fatalf("Load = %+v, %v", s, err)
	}
	if got := s.Hotkey(Hotkey{App: "com.apple.Safari"}); got != "" {
		t.Errorf("a hotkey from nowhere: %q", got)
	}
}

func TestHotkeyOwner(t *testing.T) {
	var s Settings
	term, safari, preset := Hotkey{App: "com.apple.Terminal"}, Hotkey{App: "com.apple.Safari"}, Hotkey{Preset: "p1"}
	s.SetHotkey(term, "Alt+Cmd+T")
	s.SetHotkey(safari, "Alt+Cmd+S")
	s.SetHotkey(preset, "Ctrl+Alt+1")
	for _, c := range []struct {
		accel  string
		except Hotkey
		want   Hotkey
		ok     bool
	}{
		{"Cmd+Alt+T", Hotkey{}, term, true},  // spelled otherwise
		{"Alt+Cmd+T", term, Hotkey{}, false}, // its own
		{"Alt+Ctrl+1", safari, preset, true},
		{"Alt+Cmd+X", Hotkey{}, Hotkey{}, false},
	} {
		got, ok := s.HotkeyOwner(c.accel, c.except)
		if got != c.want || ok != c.ok {
			t.Errorf("HotkeyOwner(%q, %v) = %v, %v; want %v, %v", c.accel, c.except, got, ok, c.want, c.ok)
		}
	}
	// Two with the same (edited by hand): the first by ID has it.
	s.SetHotkey(Hotkey{App: "com.apple.AAA"}, "Cmd+Alt+T")
	if got, _ := s.HotkeyOwner("Alt+Cmd+T", Hotkey{}); got != (Hotkey{App: "com.apple.AAA"}) {
		t.Errorf("owner of a shared hotkey = %v", got)
	}
}

func TestPrunePresetHotkeys(t *testing.T) {
	var s Settings
	s.SetHotkey(Hotkey{Preset: "kept"}, "Ctrl+Alt+1")
	s.SetHotkey(Hotkey{Preset: "deleted"}, "Ctrl+Alt+2")
	s.SetHotkey(Hotkey{App: "com.apple.Safari"}, "Alt+Cmd+S")
	exists := func(id string) bool { return id == "kept" }
	if !s.PrunePresetHotkeys(exists) {
		t.Fatal("nothing pruned")
	}
	if s.Hotkey(Hotkey{Preset: "deleted"}) != "" || s.Hotkey(Hotkey{Preset: "kept"}) == "" || s.Hotkey(Hotkey{App: "com.apple.Safari"}) == "" {
		t.Errorf("after pruning: %v, %v", s.PresetHotkeys, s.AppHotkeys)
	}
	if s.PrunePresetHotkeys(exists) {
		t.Error("pruned twice")
	}
	s.PrunePresetHotkeys(func(string) bool { return false })
	if s.PresetHotkeys != nil {
		t.Errorf("all pruned, kept %v", s.PresetHotkeys)
	}
}
