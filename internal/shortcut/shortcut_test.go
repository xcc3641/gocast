package shortcut

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCaps(t *testing.T) {
	for accel, want := range map[string][]string{
		"Alt+Shift+Space":           {"⌥", "⇧", "Space"},
		"Ctrl+`":                    {"⌃", "`"},
		"CmdOrCtrl+Alt+Shift+Space": {"⌥", "⇧", "⌘", "Space"},
		"Shift+Cmd+k":               {"⇧", "⌘", "K"},
		"F5":                        {"F5"},
		"":                          nil,
	} {
		if got := Caps(accel); !slices.Equal(got, want) {
			t.Errorf("Caps(%q) = %q, want %q", accel, got, want)
		}
	}
	if got := Glyphs("Ctrl+Alt+`"); got != "⌃⌥`" {
		t.Errorf("Glyphs = %q", got)
	}
}

func TestFromKey(t *testing.T) {
	cases := []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
		ok   bool
	}{
		{ui.Alt | ui.Shift, ui.KeySpace, "Alt+Shift+Space", true},
		{ui.Super | ui.Ctrl, ui.KeyK, "Ctrl+Cmd+K", true},
		{ui.Ctrl, ui.KeyBackquote, "Ctrl+`", true},
		{0, ui.KeyF12, "F12", true},
		{ui.Shift, ui.KeyA, "", false},
		{0, ui.KeyA, "", false},
		{ui.Ctrl, ui.KeyUnknown, "", false},
	}
	for _, c := range cases {
		got, ok := FromKey(c.mods, c.key)
		if got != c.want || ok != c.ok {
			t.Errorf("FromKey(%v, %v) = %q, %v; want %q, %v", c.mods, c.key, got, ok, c.want, c.ok)
		}
	}
}
