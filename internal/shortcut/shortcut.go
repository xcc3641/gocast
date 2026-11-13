// Package shortcut spells global shortcuts (MyGo accelerators, as
// "Alt+Shift+Space") as macOS shows them (⌥⇧Space), and makes them from the
// keys the user presses in the shortcut recorder.
package shortcut

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Caps returns the keycaps of an accelerator in macOS's order, ⌃⌥⇧⌘ then
// the key: "Alt+Shift+Space" is ⌥, ⇧, Space.
func Caps(accel string) []string {
	if accel == "" {
		return nil
	}
	parts := strings.Split(accel, "+")
	key := parts[len(parts)-1]
	if strings.HasSuffix(accel, "++") {
		parts, key = parts[:len(parts)-2], "+"
	} else {
		parts = parts[:len(parts)-1]
	}
	var ctrl, alt, shift, cmd bool
	for _, p := range parts {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "control", "ctrl":
			ctrl = true
		case "alt", "option", "altgr":
			alt = true
		case "shift":
			shift = true
		case "command", "cmd", "super", "meta", "cmdorctrl", "commandorcontrol":
			cmd = true
		}
	}
	var caps []string
	if ctrl {
		caps = append(caps, "⌃")
	}
	if alt {
		caps = append(caps, "⌥")
	}
	if shift {
		caps = append(caps, "⇧")
	}
	if cmd {
		caps = append(caps, "⌘")
	}
	return append(caps, keyCap(key))
}

func keyCap(k string) string {
	switch strings.ToLower(k) {
	case "space":
		return "Space"
	case "enter", "return":
		return "↩"
	case "escape", "esc":
		return "esc"
	case "backspace":
		return "⌫"
	case "delete":
		return "⌦"
	case "tab":
		return "⇥"
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	}
	if len([]rune(k)) == 1 {
		return strings.ToUpper(k)
	}
	return k
}

// Glyphs spells an accelerator in one piece: ⌥⇧Space.
func Glyphs(accel string) string {
	return strings.Join(Caps(accel), "")
}

// FromKey makes the accelerator of a key pressed with modifiers, as the
// recorder takes it: it needs Control, Option or Command, or is a function
// key (F1–F12), alone or with modifiers. ok is false for any other key, or
// a modifier pressed alone.
func FromKey(mods ui.Modifiers, key ui.Key) (accel string, ok bool) {
	return FromEvent(ui.InputEvent{Kind: ui.InputKeyDown, Mods: mods, Key: key})
}

// FromEvent is FromKey for a key event, which also names F13–F19 (by
// their codes: they have no Key).
func FromEvent(ev ui.InputEvent) (accel string, ok bool) {
	pressed := Pressed(ev)
	if pressed == "" {
		return "", false
	}
	mods, key, _ := parse(pressed)
	if !recordable(mods, fnKey(key) > 0) {
		return "", false
	}
	return pressed, true
}

// Pressed spells the key of a key event with its modifiers, whatever they
// are ("Shift+S", which the recorder refuses), for the recorder to show
// it; "" for a modifier alone or a key it has no name for.
func Pressed(ev ui.InputEvent) string {
	name := keyName(ev.Key)
	if name == "" && ev.Code >= ui.CodeF13 && ev.Code <= ui.CodeF19 {
		name = "F" + strconv.Itoa(int(ev.Code-ui.CodeF13)+13)
	}
	if name == "" {
		return ""
	}
	return spell(ev.Mods, name)
}

// ModifierCaps are the keycaps of modifiers held, in macOS's order, as the
// recorder shows them before the key comes.
func ModifierCaps(mods ui.Modifiers) []string {
	var caps []string
	for _, m := range []struct {
		mod ui.Modifiers
		cap string
	}{{ui.Ctrl, "⌃"}, {ui.Alt, "⌥"}, {ui.Shift, "⇧"}, {ui.Super, "⌘"}} {
		if mods&m.mod != 0 {
			caps = append(caps, m.cap)
		}
	}
	return caps
}

func keyName(k ui.Key) string {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return string(rune('A' + k - ui.KeyA))
	case k >= ui.Key0 && k <= ui.Key9:
		return string(rune('0' + k - ui.Key0))
	case k >= ui.KeyF1 && k <= ui.KeyF12:
		return "F" + strconv.Itoa(int(k-ui.KeyF1)+1)
	}
	switch k {
	case ui.KeySpace:
		return "Space"
	case ui.KeyEnter:
		return "Enter"
	case ui.KeyTab:
		return "Tab"
	case ui.KeyBackspace:
		return "Backspace"
	case ui.KeyDelete:
		return "Delete"
	case ui.KeyHome:
		return "Home"
	case ui.KeyEnd:
		return "End"
	case ui.KeyPageUp:
		return "PageUp"
	case ui.KeyPageDown:
		return "PageDown"
	case ui.KeyLeft:
		return "Left"
	case ui.KeyRight:
		return "Right"
	case ui.KeyUp:
		return "Up"
	case ui.KeyDown:
		return "Down"
	case ui.KeyMinus:
		return "-"
	case ui.KeyEqual:
		return "="
	case ui.KeyComma:
		return ","
	case ui.KeyPeriod:
		return "."
	case ui.KeySlash:
		return "/"
	case ui.KeySemicolon:
		return ";"
	case ui.KeyQuote:
		return "'"
	case ui.KeyBracketLeft:
		return "["
	case ui.KeyBracketRight:
		return "]"
	case ui.KeyBackslash:
		return "\\"
	case ui.KeyBackquote:
		return "`"
	}
	return ""
}
