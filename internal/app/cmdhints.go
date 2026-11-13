package app

// cmdhints.go is the floating terminal's shortcut hints while Command is
// held alone (design.md 3.5a, copy.md 14): a number on each of the first
// nine tabs, in place of their close buttons, and a card of the
// terminal's shortcuts at the bottom right.

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"gocast/locales"
)

// The hints' timing and measures (design.md 3.5a), in DIPs.
const (
	// cmdHintDelay is how long Command is held alone before the hints
	// show: longer than a Cmd-C or Cmd-V takes, short enough not to feel
	// slow.
	cmdHintDelay = 500 * time.Millisecond
	cmdHintFade  = 120 * time.Millisecond
	// The card's rows and padding, and the smaller ones of a terminal too
	// short for them (its height less than the card's and 24).
	hintRowH, hintPad           = 28, 8
	hintRowHShort, hintPadShort = 24, 4
	hintRowPad                  = 8  // a row's, left and right
	hintNameGap                 = 16 // the least between a row's name and keys
	hintComboGap                = 8  // between the two shortcuts of a row
	// The card's least and most width: it is as wide as its widest row,
	// whose name is cut at its end past the most.
	hintCardMinW, hintCardMaxW = 220, 300
)

// cmdHints follows Command in the floating terminal's frames: pressed
// alone (no other modifier, no key held), with the panel showing and
// focused, it starts the timer; held cmdHintDelay, the hints show. A key,
// a click, Control or Option ends them at once, and Command has to be
// let go of and pressed again for them to come back (Shift keeps them).
type cmdHints struct {
	held  bool      // Command was down in the frame before, or is taken to be
	since time.Time // when Command went down alone; zero while it doesn't count
	shown bool
}

// update follows the frame at now, with the modifiers mods and keys keys
// held, open while the panel shows and has the keyboard: it reports
// whether the hints show, and else how long until they do (0 for never).
func (h *cmdHints) update(now time.Time, mods ui.Modifiers, keys int, open bool) (bool, time.Duration) {
	held := mods&ui.Cmd != 0
	pressed := held && !h.held
	h.held = held
	switch {
	case pressed && mods == ui.Cmd && keys == 0 && open:
		h.since, h.shown = now, false
	case !held || !open || keys > 0 || mods&(ui.Ctrl|ui.Alt) != 0:
		h.cancel()
	}
	if h.since.IsZero() {
		return false, 0
	}
	if !h.shown {
		if wait := h.since.Add(cmdHintDelay).Sub(now); wait > 0 {
			return false, wait
		}
		h.shown = true
	}
	return true, 0
}

// cancel hides the hints, or stops their timer: they wait for Command
// pressed again.
func (h *cmdHints) cancel() { h.since, h.shown = time.Time{}, false }

// reset is the hints of a panel that shows: Command held as it appears
// (as part of the shortcut that showed it) does not count.
func (h *cmdHints) reset() { *h = cmdHints{held: true} }

// input follows what is pressed in the panel: any key but a modifier, a
// command of the menus (Cmd-C, Cmd-V), text and a click end the hints;
// Command held with it then has to be let go of first, even if the frame
// never saw it down without the key.
func (h *cmdHints) input(ev ui.InputEvent) {
	switch ev.Kind {
	case ui.InputKeyDown:
		switch ev.Code {
		case ui.CodeSuperLeft, ui.CodeSuperRight, ui.CodeShiftLeft, ui.CodeShiftRight,
			ui.CodeControlLeft, ui.CodeControlRight, ui.CodeAltLeft, ui.CodeAltRight:
			if ev.Key == ui.KeyUnknown {
				return // a modifier on its own: the frame follows it
			}
		}
	case ui.InputText, ui.InputCompose, ui.InputCommand, ui.InputPointerDown:
	default:
		return
	}
	h.cancel()
	if ev.Mods&ui.Cmd != 0 {
		h.held = true
	}
}

// hintsShown follows Command in this frame (cmdHints): whether the hints
// show, asking for the frame they show in.
func (p *termPanel) hintsShown(c *ui.Context) bool {
	open := p.shown && c.WindowActive() && !p.g.suspended
	show, wait := p.hints.update(c.Now(), c.Modifiers(), len(c.KeysDown()), open)
	if wait > 0 {
		c.After(wait)
	}
	return show
}

// hintFade is how the hints come in: fading in over cmdHintFade, at once
// with Reduce Motion; they go at once.
func hintFade(e ui.Element, c *ui.Context) ui.Element {
	if c.Preferences().ReduceMotion {
		return e
	}
	return e.Transition(ui.ElementTransition{Enter: &ui.Motion{Opacity: 0}, Duration: cmdHintFade, ExitDuration: -1, Colors: true})
}

// tabNumber is the keycap of a tab's number, 1 to 9, in place of its
// close button (design.md 3.5a): read by no assistive technology.
func tabNumber(c *ui.Context, k tokens, n int) {
	c.Key("number")
	e := ui.Box(c).Size(20, 20).Radius(5).Background(k.keycap).Center().AccessHidden(true).IgnoreInput(true)
	hintFade(e, c).Children(func() {
		capLabel(c, string(rune('0'+n)), 12, k.textOnGlass).FontFeatures("tnum")
	})
}

// hintRow is a row of the hint card: what a shortcut does, and its keys,
// one or two shortcuts of keycaps.
type hintRow struct {
	name   string
	combos [][]string
}

// hintRows are the card's rows (copy.md 14).
func hintRows() []hintRow {
	one := func(name string, caps ...string) hintRow { return hintRow{name, [][]string{caps}} }
	return []hintRow{
		one(locales.TermHintNewTab(), "⌘", "T"),
		one(locales.TermHintCloseTab(), "⌘", "W"),
		one(locales.TermHintPrevTab(), "⌘", "⇧", "["),
		one(locales.TermHintNextTab(), "⌘", "⇧", "]"),
		one(locales.TermHintGoToTab(), "⌘", "1–9"),
		one(locales.TermHintCopy(), "⌘", "C"),
		one(locales.TermHintPaste(), "⌘", "V"),
		{locales.TermHintTextSize(), [][]string{{"⌘", "+"}, {"⌘", "-"}}},
		one(locales.TermHintActualSize(), "⌘", "0"),
	}
}

// hintSize is the hint card's width, row height and padding.
type hintSize struct{ w, rowH, pad float32 }

// hintCardSize is the size of the hint card in this frame.
func (p *termPanel) hintCardSize(c *ui.Context) hintSize {
	_, h := c.Size()
	return hintCardFor(hintRows(), h-tabBarH, func(text string, size float32) float32 {
		tw, _ := c.MeasureText(0, ui.Span{Text: text, Size: size})
		return tw
	})
}

// hintCardFor is the size of the card of rows in a terminal area termH
// high: as wide as its widest row, within hintCardMinW and hintCardMaxW;
// rows of hintRowH, or of hintRowHShort where the area is too short for
// them; measure gives a text's width at a size.
func hintCardFor(rows []hintRow, termH float32, measure func(text string, size float32) float32) hintSize {
	sz := hintSize{rowH: hintRowH, pad: hintPad}
	if termH < float32(len(rows))*hintRowH+2*hintPad+2*termInset {
		sz.rowH, sz.pad = hintRowHShort, hintPadShort
	}
	var widest float32
	for _, r := range rows {
		widest = max(widest, measure(r.name, 13)+hintNameGap+combosWidth(r.combos, measure))
	}
	w := float32(math.Ceil(float64(widest))) + 2*hintRowPad + 2*sz.pad
	sz.w = min(max(w, hintCardMinW), hintCardMaxW)
	return sz
}

// combosWidth is the width of a row's keycaps (keycap: 20 at least, the
// label and 5 at each side).
func combosWidth(combos [][]string, measure func(text string, size float32) float32) float32 {
	var w float32
	for i, caps := range combos {
		if i > 0 {
			w += hintComboGap
		}
		for j, cp := range caps {
			if j > 0 {
				w += 4
			}
			w += max(20, measure(cp, 12)+10)
		}
	}
	return w
}

// hintCard is the card of the terminal's shortcuts, at the bottom right
// of the terminal area, 12 from its edges, as the capsules (design.md
// 3.5a): on the ⌘K menu's glass, its padding counted from its outer edge
// (the border inside it), taking no pointer (a click goes through to the
// terminal) and read by no assistive technology.
func (p *termPanel) hintCard(c *ui.Context, k tokens, sz hintSize) {
	rows := hintRows()
	c.Key("hint-card")
	card := ui.Column(c).Absolute().Right(termInset).Bottom(termInset).Width(sz.w).Padding(sz.pad-1).
		Radius(12).Material(panelGlass{fallback: k.surface, tint: k.glassTint.Alpha(searchTint)}).Border(1, k.capsuleBorder).
		Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.20)).AccessHidden(true).IgnoreInput(true)
	hintFade(card, c).Children(func() {
		for _, r := range rows {
			ui.Row(c).Height(sz.rowH).Shrink(0).Padding(0, hintRowPad).Gap(hintNameGap).AlignItems(ui.Center).Children(func() {
				ui.Text(c, r.name).FontSize(13).TextColor(k.text).SingleLine().Grow(1).Shrink(1)
				ui.Row(c).Gap(hintComboGap).Shrink(0).AlignItems(ui.Center).Children(func() {
					for _, caps := range r.combos {
						keycaps(c, k, caps, 12)
					}
				})
			})
		}
	})
}
