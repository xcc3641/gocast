package app

// The search panel's part of the hotkeys of apps and presets (design.md
// 2.3, 2.5, 2.5b): the Cmd-K items that set and remove one, the recorder
// card that takes the Cmd-K menu's place, and the keycaps of the row
// chosen.

import (
	"strings"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// The recorder card's measures (design.md 2.5b).
const (
	recBoxWidth  = menuWidth - 2*14 // 292
	recBoxHeight = 56
	bigCap       = 32 // a big keycap's height and least width
)

// hotkeyRecorder is the state of the search panel's recorder card.
type hotkeyRecorder struct {
	on    bool
	item  settings.Hotkey
	name  string
	entry entry // what the row chosen opens, for the card's icon
	// mods are the modifiers held, shown before the key comes; pressed
	// the shortcut pressed last, refused with problem.
	mods    ui.Modifiers
	pressed string
	problem string
	// say is read out in the next frame: a problem.
	say string
}

// entryHotkey is the item of the hotkey of what a row opens: an app, a
// preset or an action; ok is false for the others.
func entryHotkey(e entry) (h settings.Hotkey, ok bool) {
	switch {
	case e.calc || e.own != "":
		return h, false
	case e.app != nil:
		return settings.Hotkey{App: e.app.ID}, true
	case e.preset != "":
		return settings.Hotkey{Preset: e.preset}, true
	case e.action != "":
		return actionHotkey(e.action), true
	}
	return h, false
}

// hotkeyActions are the Cmd-K items of the hotkey of h, the item named
// name (copy.md 11.1): set or change it, and remove it when it has one.
func (l *launcher) hotkeyActions(h settings.Hotkey, name string, e entry) []action {
	has := l.g.settings.Hotkey(h) != ""
	label := locales.ActionSetShortcut()
	if has {
		label = locales.ActionChangeShortcut()
	}
	as := []action{{"keyboard", label, nil, func() { l.startRecorder(h, name, e) }}}
	if has {
		as = append(as, action{"minus.circle", locales.ActionRemoveShortcut(), nil, func() {
			l.g.removeHotkey(h)
			l.say(locales.HotkeyRemoved(), false)
		}})
	}
	return as
}

// entryAccel is the hotkey of what a row opens, "" for none.
func (l *launcher) entryAccel(e entry) string {
	if h, ok := entryHotkey(e); ok {
		return l.g.settings.Hotkey(h)
	}
	return ""
}

// startRecorder shows the recorder card in the Cmd-K menu's place, every
// global shortcut off meanwhile (design.md 2.5b).
func (l *launcher) startRecorder(h settings.Hotkey, name string, e entry) {
	l.menuOpen = false
	l.rec = hotkeyRecorder{on: true, item: h, name: name, entry: e}
	l.g.suspendShortcuts()
	l.recInput.Focus()
	l.redraw()
}

// stopRecorder closes the card, back to the list, the shortcuts on again.
func (l *launcher) stopRecorder() {
	if !l.rec.on {
		return
	}
	l.rec.on = false
	l.g.resumeShortcuts()
	l.input.Focus()
	l.redraw()
}

// recordKey takes every key while the card records: Escape alone cancels;
// a modifier shows as it is held; a shortcut is saved at once, or refused
// with why, the card recording on (design.md 2.5b).
func (l *launcher) recordKey(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown, ui.InputKeyUp:
	case ui.InputText, ui.InputCompose, ui.InputCommand:
		return true // nothing typed
	default:
		return false
	}
	if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEscape && ev.Mods == 0 {
		l.stopRecorder()
		return true
	}
	r := &l.rec
	pressed := shortcut.Pressed(ev)
	if pressed == "" || ev.Kind == ui.InputKeyUp { // a modifier, or a key let go
		r.mods = ev.Mods
		if ev.Kind == ui.InputKeyDown && pressed == "" && ev.Mods != 0 {
			r.problem, r.pressed = "", "" // pressing again: recording afresh
		}
		return true
	}
	if ev.Repeat {
		return true
	}
	accel, ok := shortcut.FromEvent(ev)
	var err error
	if !ok {
		err = errHotkeyInvalid
	} else {
		err = l.g.setHotkey(r.item, accel)
	}
	if err != nil {
		l.g.suspendShortcuts() // setHotkey resumed them, refused by the system
		r.pressed, r.problem, r.mods = pressed, l.g.hotkeyProblem(err), ev.Mods
		r.say = r.problem
		return true
	}
	l.rec.on = false // setHotkey resumed the shortcuts
	l.input.Focus()
	n, _ := shortcut.Normalize(accel)
	l.say(locales.HotkeySetTo(shortcut.Glyphs(n)), false)
	return true
}

// recorderCard is the card recording a hotkey: the Cmd-K menu's place
// and glass, 320 wide, its bottom where the menu's is, growing up; over
// a clear layer that cancels on a click outside it (design.md 2.5b).
func (l *launcher) recorderCard(c *ui.Context, k tokens) {
	r := &l.rec
	c.Key("recorderOutside")
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).AccessHidden(true).OnClick(l.stopRecorder)
	title := locales.HotkeyRecordTitle(r.name)
	hint, hintColor := locales.ShortcutRecordingHint(), k.textOnGlass
	if r.problem != "" {
		hint, hintColor = r.problem, k.danger
	}
	c.Key("recorder")
	card := popupMenu(c, k, 1, 1).Right(capsuleInset).Bottom(capsuleInset + capsuleHeight + 8).MaxHeight(400).
		Role(ui.RoleGroup).Label(title + listSep() + locales.ShortcutRecording()).
		Focusable().FocusRing(false).Bind(&l.recInput).AutoFocus().HandleInput(l.recordKey)
	card.Children(func() {
		ui.Row(c).Height(36).Shrink(0).Padding(0, 12, 0, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			l.recorderIcon(c, k)
			quotedTitle(c, k, title, r.name)
		})
		ui.Box(c).Height(1).Background(k.hairline).Shrink(0)
		ui.Column(c).Padding(12, 14, 14, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			box := ui.Row(c).Size(recBoxWidth, recBoxHeight).Radius(8).Background(k.cardFill).
				Justify(ui.Center).AlignItems(ui.Center).Gap(6).AccessHidden(true)
			if r.problem != "" {
				box.Border(1, k.danger)
			} else {
				box.Border(2, c.Theme().Accent)
			}
			box.Children(func() {
				old := l.g.settings.Hotkey(r.item)
				switch {
				case r.problem != "":
					bigKeycaps(c, k, shortcut.Caps(r.pressed), k.text)
				case r.mods != 0:
					bigKeycaps(c, k, shortcut.ModifierCaps(r.mods), k.text)
				case old != "":
					bigKeycaps(c, k, shortcut.Caps(old), k.textOnGlass)
				default:
					ui.Text(c, locales.ShortcutRecording()).FontSize(13).TextColor(k.textOnGlass).SingleLine()
				}
			})
			ui.Text(c, hint).FontSize(12).TextColor(hintColor).TextAlign(ui.Center).MaxLines(2).
				Width(recBoxWidth).AccessHidden(true)
		})
	})
	if r.say != "" {
		c.Announce(r.say)
		r.say = ""
	}
}

// recorderIcon is the icon of the item recording, 16: the app's, a
// preset's square, or an action's.
func (l *launcher) recorderIcon(c *ui.Context, k tokens) {
	e := l.rec.entry
	if e.action != "" {
		actionSquare(c, k, e.action, 16, 4, 10, k.textOnGlass)
		return
	}
	if e.app != nil {
		if b := l.icon(*e.app); b != nil {
			ui.Image(c, b).Size(16, 16).Shrink(0)
			return
		}
	}
	ui.Box(c).Size(16, 16).Radius(4).Background(k.keycap).Center().Shrink(0).Children(func() {
		sfsymbols.Symbol(c, "apple.terminal.fill").Size(10).Color(k.text)
	})
}

// quotedTitle is a title of 13 Medium with a name in it, the name alone
// cut at its end (design.md 2.5b: the quotes stay).
func quotedTitle(c *ui.Context, k tokens, title, name string) {
	before, after := around(title, name)
	if name == "" || !strings.Contains(title, name) {
		before, after, name = title, "", ""
	}
	ui.Row(c).Grow(1).Shrink(1).AccessHidden(true).Children(func() {
		for i, part := range []string{before, name, after} {
			if part == "" {
				continue
			}
			t := ui.Text(c, part).FontSize(13).FontWeight(500).TextColor(k.text).SingleLine().Shrink(0)
			if i == 1 {
				t.Shrink(1)
			}
		}
	})
}

// bigKeycaps are the recorder's keycaps (design.md 2.5b): 32 high, 15
// Medium, 6 apart.
func bigKeycaps(c *ui.Context, k tokens, caps []string, color ui.Color) {
	for _, cp := range caps {
		ui.Box(c).Height(bigCap).MinWidth(bigCap).Padding(0, 8).Radius(7).Background(k.keycap).Center().Shrink(0).Children(func() {
			ui.Text(c, cp).FontSize(15).FontWeight(500).TextColor(color).SingleLine()
		})
	}
}

// rowKeycaps are the keycaps of the hotkey of the row chosen, before its
// tag, 8 from it (design.md 2.3).
func rowKeycaps(c *ui.Context, k tokens, accel string) {
	if accel == "" {
		return
	}
	ui.Row(c).Shrink(0).Margin(0, -2, 0, 0).Children(func() { keycaps(c, k, shortcut.Caps(accel), 12) })
}

// hotkeySpoken is what assistive technology reads after a row's label
// when it has a hotkey (copy.md 11.4): ", shortcut Option Command T".
func hotkeySpoken(accel string) string {
	if accel == "" {
		return ""
	}
	return listSep() + locales.HotkeySpoken(shortcut.Spoken(accel))
}

// presetName is the name of the preset with the ID, as its row shows it.
func (l *launcher) presetName(id string) string {
	if j := presets.Find(l.g.presets, id); j >= 0 {
		return presetTitle(l.g.presets[j])
	}
	return id
}
