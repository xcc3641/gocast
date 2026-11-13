package shortcut

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Alt+Cmd+T":                 "Alt+Cmd+T",
		"Cmd+Alt+t":                 "Alt+Cmd+T",
		"command+option+T":          "Alt+Cmd+T",
		"Super+Space":               "Cmd+Space",
		"CmdOrCtrl+Shift+Alt+space": "Alt+Shift+Cmd+Space",
		"Cmd+Ctrl+v":                "Ctrl+Cmd+V",
		"Ctrl+`":                    "Ctrl+`",
		"f5":                        "F5",
		"Ctrl+Return":               "Ctrl+Enter",
		"Cmd++":                     "Cmd++",
	} {
		got, ok := Normalize(in)
		if !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "Cmd+", "Hyper+T", "Cmd+F20", "Cmd+F05", "Cmd+VolumeUp"} {
		if got, ok := Normalize(in); ok {
			t.Errorf("Normalize(%q) = %q, want not ok", in, got)
		}
	}
}

// The recorder's accelerators normalize to themselves.
func TestNormalizeFromKey(t *testing.T) {
	for _, c := range []struct {
		mods ui.Modifiers
		key  ui.Key
	}{
		{ui.Alt | ui.Super, ui.KeyT},
		{ui.Ctrl | ui.Alt | ui.Shift | ui.Super, ui.KeySpace},
		{ui.Ctrl, ui.KeyBackquote},
		{ui.Super, ui.KeyPageDown},
		{0, ui.KeyF12},
	} {
		accel, ok := FromKey(c.mods, c.key)
		if n, nok := Normalize(accel); !ok || !nok || n != accel {
			t.Errorf("FromKey = %q, normalized %q", accel, n)
		}
	}
}

func TestValid(t *testing.T) {
	for accel, want := range map[string]bool{
		"Alt+Cmd+T":     true,
		"Ctrl+`":        true,
		"Cmd+Space":     true,
		"Alt+Shift+K":   true,
		"F5":            true,
		"F19":           true,
		"Shift+F13":     true,
		"Shift+T":       false, // Shift alone takes a capital
		"T":             false,
		"Space":         false,
		"F20":           false,
		"":              false,
		"Cmd+Nonsense":  false,
		"Hyper+Shift+T": false,
	} {
		if got := Valid(accel); got != want {
			t.Errorf("Valid(%q) = %v, want %v", accel, got, want)
		}
	}
}

func TestSame(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"Cmd+Alt+T", "Alt+Cmd+T", true},
		{"Alt+Shift+Space", "shift+alt+space", true},
		{"Super+Space", "Cmd+Space", true},
		{"Ctrl+Cmd+V", "Cmd+V", false},
		{"Ctrl+`", "Ctrl+Alt+`", false},
		{"", "", false},
		{"Weird+X", "weird+x", true}, // unreadable: by their text
	} {
		if got := Same(c.a, c.b); got != c.want {
			t.Errorf("Same(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestFromEventFunctionKeys(t *testing.T) {
	for _, c := range []struct {
		ev   ui.InputEvent
		want string
		ok   bool
	}{
		{ui.InputEvent{Code: ui.CodeF13}, "F13", true},
		{ui.InputEvent{Code: ui.CodeF19, Mods: ui.Shift}, "Shift+F19", true},
		{ui.InputEvent{Code: ui.CodeF20}, "", false}, // no Mac has it
		{ui.InputEvent{Key: ui.KeyF5}, "F5", true},
		{ui.InputEvent{Key: ui.KeyT, Mods: ui.Alt | ui.Super}, "Alt+Cmd+T", true},
		{ui.InputEvent{Key: ui.KeyT, Mods: ui.Shift}, "", false},
		{ui.InputEvent{Code: ui.CodeShiftLeft, Mods: ui.Shift}, "", false},
	} {
		got, ok := FromEvent(c.ev)
		if got != c.want || ok != c.ok {
			t.Errorf("FromEvent(%+v) = %q, %v; want %q, %v", c.ev, got, ok, c.want, c.ok)
		}
	}
}

func TestPressed(t *testing.T) {
	for _, c := range []struct {
		ev   ui.InputEvent
		want string
	}{
		{ui.InputEvent{Key: ui.KeyS, Mods: ui.Shift}, "Shift+S"}, // shown, though refused
		{ui.InputEvent{Key: ui.KeyS}, "S"},
		{ui.InputEvent{Key: ui.KeySpace, Mods: ui.Alt}, "Alt+Space"},
		{ui.InputEvent{Code: ui.CodeAltLeft, Mods: ui.Alt}, ""}, // a modifier alone
		{ui.InputEvent{Code: ui.CodeCapsLock}, ""},
	} {
		if got := Pressed(c.ev); got != c.want {
			t.Errorf("Pressed(%+v) = %q, want %q", c.ev, got, c.want)
		}
	}
}

func TestModifierCaps(t *testing.T) {
	got := ModifierCaps(ui.Super | ui.Ctrl | ui.Shift)
	if want := []string{"⌃", "⇧", "⌘"}; !slices.Equal(got, want) {
		t.Errorf("ModifierCaps = %q, want %q", got, want)
	}
	if ModifierCaps(0) != nil {
		t.Error("caps of no modifier")
	}
}

func TestSpoken(t *testing.T) {
	for accel, want := range map[string]string{
		"Alt+Cmd+T":       "Option Command T", // copy.md 11.4
		"Ctrl+`":          "Control Grave Accent",
		"Alt+Shift+Space": "Option Shift Space",
		"Ctrl+Cmd+V":      "Control Command V",
		"F5":              "F5",
		"Cmd+Up":          "Command Up Arrow",
		"Shift+Cmd+K":     "Command Shift K",
	} {
		if got := Spoken(accel); got != want {
			t.Errorf("Spoken(%q) = %q, want %q", accel, got, want)
		}
	}
}
