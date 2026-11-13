package shortcut

import (
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
