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

// The names of the apps with hotkeys are kept apart from the hotkeys,
// gone with them; a file with hotkeys and no names loads as it did.
func TestAppHotkeyNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	var s Settings
	chat := Hotkey{App: "com.tencent.xinWeChat"}
	s.SetHotkey(chat, "Alt+Cmd+W")
	s.SetAppHotkeyName(chat.App, "微信")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{`"appHotkeys": {`, `"com.tencent.xinWeChat": "Alt+Cmd+W"`, `"appHotkeyNames": {`, `"com.tencent.xinWeChat": "微信"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("settings.json has no %s: %s", want, data)
		}
	}
	out, err := Load(path)
	if err != nil || out.AppHotkeyName(chat.App) != "微信" || out.Hotkey(chat) != "Alt+Cmd+W" {
		t.Fatalf("Load = %+v, %v", out, err)
	}
	out.RemoveHotkey(chat)
	if out.AppHotkeyNames != nil || out.AppHotkeyName(chat.App) != "" {
		t.Errorf("name kept with the hotkey removed: %v", out.AppHotkeyNames)
	}
	// Setting "" removes it too; a preset's hotkey has none.
	out.SetHotkey(chat, "Alt+Cmd+W")
	out.SetAppHotkeyName(chat.App, "微信")
	out.SetHotkey(Hotkey{Preset: "p"}, "Ctrl+Alt+1")
	out.SetHotkey(Hotkey{Preset: "p"}, "")
	if out.AppHotkeyName(chat.App) != "微信" {
		t.Error("a preset's hotkey removed took an app's name")
	}
	out.SetHotkey(chat, "")
	if out.AppHotkeyNames != nil {
		t.Errorf("name kept with the hotkey set to none: %v", out.AppHotkeyNames)
	}

	// A file of before: hotkeys, no names.
	old := `{"appHotkeys": {"com.tencent.xinWeChat": "Alt+Cmd+W"}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = Load(path)
	if err != nil || s.Hotkey(chat) != "Alt+Cmd+W" || s.AppHotkeyName(chat.App) != "" {
		t.Fatalf("Load of before = %+v, %v", s, err)
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

func TestActionHotkeys(t *testing.T) {
	var s Settings
	s.SetHotkey(Hotkey{Action: "left-half"}, "Ctrl+Alt+Left")
	s.SetHotkey(Hotkey{App: "com.apple.Safari"}, "Alt+Cmd+S")
	if got := s.Hotkey(Hotkey{Action: "left-half"}); got != "Ctrl+Alt+Left" {
		t.Fatalf("action hotkey = %q", got)
	}
	if s.AppHotkeys["left-half"] != "" || s.PresetHotkeys["left-half"] != "" {
		t.Fatal("the action's hotkey went to another map")
	}
	hs := s.Hotkeys()
	if len(hs) != 2 || hs[0] != (Hotkey{App: "com.apple.Safari"}) || hs[1] != (Hotkey{Action: "left-half"}) {
		t.Fatalf("Hotkeys() = %v", hs)
	}
	if h, ok := s.HotkeyOwner("Alt+Ctrl+Left", Hotkey{App: "x"}); !ok || h.Action != "left-half" {
		t.Fatalf("HotkeyOwner = %v, %v", h, ok)
	}
	s.RemoveHotkey(Hotkey{Action: "left-half"})
	if s.ActionHotkeys != nil {
		t.Fatalf("emptied map kept: %v", s.ActionHotkeys)
	}
}
