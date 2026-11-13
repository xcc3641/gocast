package shortcut

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// recordable is the recorder's rule (FromKey's, and Valid's for an
// accelerator from settings.json): a shortcut needs Control, Option or
// Command, as Shift alone would take a letter from typing, unless its key
// is a function key.
func recordable(mods ui.Modifiers, fnKey bool) bool {
	return fnKey || mods&(ui.Ctrl|ui.Alt|ui.Super) != 0
}

// namedKeys are the keys other than letters, digits and punctuation, by
// the names MyGo takes (lower case), as FromKey writes them.
var namedKeys = map[string]string{
	"space": "Space", "enter": "Enter", "return": "Enter", "tab": "Tab",
	"backspace": "Backspace", "delete": "Delete", "del": "Delete",
	"home": "Home", "end": "End", "pageup": "PageUp", "pagedown": "PageDown",
	"left": "Left", "right": "Right", "up": "Up", "down": "Down",
	"escape": "Escape", "esc": "Escape", "plus": "+",
}

// parse reads an accelerator into its modifiers and key, the key as
// FromKey writes it; ok is false for one MyGo would not take, or one
// with a key Gocast doesn't record.
func parse(accel string) (mods ui.Modifiers, key string, ok bool) {
	accel = strings.TrimSpace(accel)
	if accel == "" {
		return 0, "", false
	}
	parts := strings.Split(accel, "+")
	if strings.HasSuffix(accel, "++") {
		parts = append(parts[:len(parts)-2], "+")
	}
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "control", "ctrl":
			mods |= ui.Ctrl
		case "alt", "option", "altgr":
			mods |= ui.Alt
		case "shift":
			mods |= ui.Shift
		case "command", "cmd", "super", "meta", "cmdorctrl", "commandorcontrol":
			mods |= ui.Super
		default:
			return 0, "", false
		}
	}
	key = keyOf(strings.TrimSpace(parts[len(parts)-1]))
	return mods, key, key != ""
}

// keyOf is a key's name as FromKey writes it, or "".
func keyOf(k string) string {
	if len([]rune(k)) == 1 {
		return strings.ToUpper(k)
	}
	if n := fnKey(k); n > 0 {
		return "F" + strconv.Itoa(n)
	}
	return namedKeys[strings.ToLower(k)]
}

// fnKey is the number of a function key's name (F1–F19, as Mac keyboards
// have), or 0.
func fnKey(k string) int {
	if len(k) < 2 || (k[0] != 'F' && k[0] != 'f') {
		return 0
	}
	n, err := strconv.Atoi(k[1:])
	if err != nil || n < 1 || n > 19 || strconv.Itoa(n) != k[1:] {
		return 0
	}
	return n
}

// Normalize writes an accelerator as FromKey does, ⌃⌥⇧⌘ then the key
// ("Cmd+Alt+t" is "Alt+Cmd+T"), for two spellings of a shortcut to
// compare equal; ok is false for one parse can't read.
func Normalize(accel string) (string, bool) {
	mods, key, ok := parse(accel)
	if !ok {
		return "", false
	}
	return spell(mods, key), true
}

// spell writes the modifiers and key in macOS's order.
func spell(mods ui.Modifiers, key string) string {
	var b strings.Builder
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}, {ui.Super, "Cmd"}} {
		if mods&m.mod != 0 {
			b.WriteString(m.name)
			b.WriteByte('+')
		}
	}
	b.WriteString(key)
	return b.String()
}

// Valid reports whether an accelerator is one the recorder would take:
// with Control, Option or Command, or a function key (F1–F19).
func Valid(accel string) bool {
	mods, key, ok := parse(accel)
	return ok && recordable(mods, fnKey(key) > 0)
}

// Same reports whether two accelerators are the same shortcut, however
// they are spelled; ones parse can't read are the same when their text
// is, but for case.
func Same(a, b string) bool {
	na, okA := Normalize(a)
	nb, okB := Normalize(b)
	if okA && okB {
		return na == nb
	}
	return a != "" && strings.EqualFold(a, b)
}

// spokenKeys are the names VoiceOver says for the keys that are not
// letters, digits or function keys.
var spokenKeys = map[string]string{
	"Space": "Space", "Enter": "Return", "Tab": "Tab", "Backspace": "Delete",
	"Delete": "Forward Delete", "Escape": "Escape", "Home": "Home", "End": "End",
	"PageUp": "Page Up", "PageDown": "Page Down",
	"Left": "Left Arrow", "Right": "Right Arrow", "Up": "Up Arrow", "Down": "Down Arrow",
	"`": "Grave Accent", "-": "Minus", "=": "Equals", ",": "Comma", ".": "Period",
	"/": "Slash", ";": "Semicolon", "'": "Quote", "[": "Left Bracket", "]": "Right Bracket",
	"\\": "Backslash", "+": "Plus",
}

// Spoken says an accelerator as VoiceOver names keys, in English whatever
// the language, the modifiers in copy.md 11.4's order, Control Option
// Command Shift: "Alt+Cmd+T" is "Option Command T".
func Spoken(accel string) string {
	mods, key, ok := parse(accel)
	if !ok {
		return accel
	}
	var words []string
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Control"}, {ui.Alt, "Option"}, {ui.Super, "Command"}, {ui.Shift, "Shift"}} {
		if mods&m.mod != 0 {
			words = append(words, m.name)
		}
	}
	if name, ok := spokenKeys[key]; ok {
		key = name
	}
	return strings.Join(append(words, key), " ")
}
