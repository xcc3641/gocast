package main

// kit.go is Gocast's one set of components, drawn with the tokens of
// theme.go, which the search panel, the floating terminal and Settings
// share (design.md 8): the glass and its tint, the windows' outline and
// shadow, the search panel's list rows, sections and dividers, the
// capsules, the keycaps, the popup menus, the round buttons and Gocast's
// mark. Neither window draws its own kind of any of them.

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// The glass windows' and their parts' measures (design.md 1.4, 2.4, 2.5,
// 3.2, 3.4), in DIPs.
const (
	panelRadius      = 26 // both glass windows
	panelBorderWidth = 0.5
	capsuleHeight    = 36
	menuWidth        = 320
	menuMaxHeight    = 300
	menuRowHeight    = 32
	menuPad          = 6  // around a popup menu's rows
	menuNameGap      = 12 // the least between a row's name and keys
	menuRowPad       = 8  // in a row, before its icon
	menuRowPadR      = 6  // in a row, after its keys
	// menuTextInset is where a popup menu's rows start their icon, from
	// its left edge, as its filter field and title do; menuTextInsetR
	// where they end their keys, from its right edge, as its filter field
	// does: 12, 20 from the search panel's, on its right line (design.md
	// 2.1a).
	menuTextInset   = menuPad + menuRowPad
	menuTextInsetR  = menuPad + menuRowPadR
	roundButtonSize = 32

	// rightLine is the line, from the search panel's right edge, the ends
	// of what is at its right are on (design.md 2.1a): a row's tag (list
	// 8, row 12), the actions capsule's last key (inset 8, border 1,
	// capsulePadR 3, segmentPadR 8), a popup menu's keys (inset 8,
	// menuPad 6, menuRowPadR 6), the search bar's spinner (its padding).
	rightLine = 20
)

// --- Glass ---

// panelGlass is the material of the search panel, the capsules and the
// popup menus: the system's Liquid Glass under the tint where it has it,
// else the tint opaque (older macOS, Reduce Transparency), design.md 2.1.
type panelGlass struct{ fallback, tint ui.Color }

// glassOf is the glass under the tint at alpha (searchTint,
// terminalTint).
func glassOf(k tokens, alpha float32) panelGlass {
	return panelGlass{fallback: k.glassTint, tint: k.glassTint.Alpha(alpha)}
}

func (g panelGlass) NativeGlass() (ui.NativeGlassSpec, bool) {
	return glass.Glass{Native: true}.NativeGlass()
}

func (g panelGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if p.NativeGlassSupported() && !p.Preferences().ReduceTransparency {
		p.Hole(box, radii)
		if g.tint.A > 0 {
			p.FillRadii(box, radii, g.tint)
		}
		return
	}
	p.FillRadii(box, radii, g.fallback)
}

// glassFill is the tint at alpha over a window whose own material is the
// glass (the floating terminal), or the tint opaque where the window has
// no material (c.Vibrancy is false: older macOS, Reduce Transparency), as
// panelGlass does.
func glassFill(c *ui.Context, k tokens, alpha float32) ui.Color {
	if c.Vibrancy() {
		return k.glassTint.Alpha(alpha)
	}
	return k.glassTint
}

// panelOutline outlines a glass window, as both have it (design.md 3.2).
func panelOutline(e ui.Element, k tokens) ui.Element {
	return e.Radius(panelRadius).Border(panelBorderWidth, k.panelBorder)
}

// panelShadow is the glass windows' shadow, for a panel drawn in a larger
// window (the search panel): 0 24 70.
func panelShadow(e ui.Element, k tokens) ui.Element {
	return e.Shadow(0, 24, 70, 0, k.panelShadow)
}

// The room the shadow takes around a panel: about its blur past its
// offset on each side.
const shadowLeft, shadowTop, shadowBottom = 72, 48, 96

// --- Gocast's mark ---

// logoPNG is Gocast's mark, assets/icon/make-icon.py's template image:
// 20 DIP (40 px), black on transparent (design.md 1.6).
//
//go:embed assets/icon/logo-template.png
var logoPNG []byte

// logoBitmap is logoPNG in white, keeping its alpha, so that a tint
// (which multiplies) draws it in the tint's color; made once.
var logoBitmap *ui.Bitmap

func gocastLogo() *ui.Bitmap {
	if logoBitmap == nil {
		logoBitmap = templateBitmap(logoPNG)
	}
	return logoBitmap
}

// templateBitmap makes a bitmap of a template image's alpha, in white;
// nil if the PNG can't be read.
func templateBitmap(data []byte) *ui.Bitmap {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b := src.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			_, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a >> 8)})
		}
	}
	return ui.NewBitmap(img)
}

// gocastMark is Gocast's mark at the start of the search bar and the
// terminal's tab bar, the shape of the tray's (design.md 1.6): the
// template image drawn at 20×20 in textSecondaryOnGlass, so it follows
// light and dark.
func gocastMark(c *ui.Context, k tokens) ui.Element {
	logo := gocastLogo()
	return ui.Box(c).Size(20, 20).Shrink(0).AccessHidden(true).Draw(func(p *ui.Painter, r ui.Rect) {
		p.ImageRegion(logo, ui.Rect{}, r, ui.ImageOptions{Tint: k.textOnGlass})
	})
}

// --- Keycaps ---

// keycap draws a key of a shortcut, as ⌘ or K, in menus and Settings;
// assistive technology skips it (the control says its shortcut).
func keycap(c *ui.Context, k tokens, label string, size float32) {
	ui.Box(c).Height(20).MinWidth(20).Padding(0, 5).Radius(5).Background(k.keycap).Center().AccessHidden(true).Children(func() {
		capLabel(c, label, size, k.textOnGlass)
	})
}

// capLabel is the label of a keycap, moved by capNudge to look centered.
func capLabel(c *ui.Context, label string, size float32, color ui.Color) {
	t := ui.Text(c, label).FontSize(size).TextColor(color).SingleLine()
	// A margin on one side moves the centered text by half of it.
	if n := capNudge(label); n > 0 {
		t.Margin(2*n, 0, 0, 0)
	} else if n < 0 {
		t.Margin(0, 0, -2*n, 0)
	}
}

// capNudge is how far a keycap's label moves down (up if negative) for
// its ink to sit in the middle of the key. The text's line box is
// centered, which leaves glyphs standing on the baseline (letters, digits,
// ⌘⇧⌥) 0.5–0.75 DIP low, and arrows, on the math axis, 0.75 DIP high, as
// measured on 2× captures (12 and 13 pt). ⌃ and ` are drawn high by
// design, as in menus, and stay.
func capNudge(label string) float32 {
	switch label {
	case "↵", "↩", "↑", "↓", "←", "→", "⇥":
		return 0.75
	case "⌃", "`":
		return 0
	}
	return -0.5
}

// keycaps draws the keys of a shortcut, 4 apart.
func keycaps(c *ui.Context, k tokens, caps []string, size float32) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).AccessHidden(true).Children(func() {
		for _, cp := range caps {
			keycap(c, k, cp, size)
		}
	})
}

// outlineKeycaps draws the keys of a shortcut in an actions capsule
// (design.md 2.4): 22 high, outlined in the hairline, not filled.
func outlineKeycaps(c *ui.Context, k tokens, color ui.Color, caps ...string) {
	ui.Row(c).Gap(capGap).AlignItems(ui.Center).Shrink(0).AccessHidden(true).Children(func() {
		for _, cp := range caps {
			ui.Box(c).Height(outlineCap).MinWidth(outlineCap).Padding(0, outlineCapPad).Radius(6).Border(1, k.hairline).Center().Children(func() {
				capLabel(c, cp, 12, color)
			})
		}
	})
}

// --- Capsules ---

// capsule is a glass capsule floating at the bottom of a glass window,
// bottom from its bottom (8 in the search panel, 12 in the terminal; in
// the search bar alone, centred in it: capsuleBottom): place it with Left
// or Right, and fill it.
func capsule(c *ui.Context, k tokens, bottom float32) ui.Element {
	// The capsules are the system's glass, as the panel: MyGo's glass
	// over the window's would show what the window painted under it,
	// nothing, as gray.
	return ui.Row(c).Absolute().Bottom(bottom).Height(capsuleHeight).Radius(capsuleHeight/2).
		AlignItems(ui.Center).Material(glassOf(k, searchTint)).Border(1, k.capsuleBorder).
		Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.12))
}

// actionsCapsule is the capsule of actions at the bottom right, inset
// from the right and bottom from the bottom, as wide as its segments are,
// never shortened (design.md 2.4): the notice capsule beside it gives way.
func actionsCapsule(c *ui.Context, k tokens, inset, bottom float32, segs ...segment) {
	c.Key("actions")
	e := capsule(c, k, bottom)
	e.Right(inset).Shrink(0).Padding(0, capsulePadR, 0, capsulePadL).Gap(segmentGap).Children(func() {
		for _, sg := range segs {
			sg.draw(c, k)
		}
	})
}

// The measures of an actions capsule: its padding, left and right; the
// gap between its segments; a segment's padding, left and right, and the
// gap between its name and keys; a key's least width, padding and the gap
// between keys (outlineKeycaps).
const (
	capsulePadL, capsulePadR, segmentGap  = 6, 3, 4
	segmentPadL, segmentPadR, segmentKeys = 8, 8, 8
	outlineCap, outlineCapPad, capGap     = 22, 5, 4
)

// actionsCapsuleWidth is the width of the actions capsule of segs, its
// border included, as laid out.
func actionsCapsuleWidth(c *ui.Context, segs ...segment) float32 {
	return capsuleWidthOf(func(text string, size float32) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: text, Size: size})
		return w
	}, segs...)
}

// capsuleWidthOf is actionsCapsuleWidth, the texts' widths given by
// measure.
func capsuleWidthOf(measure func(text string, size float32) float32, segs ...segment) float32 {
	w := float32(capsulePadL + capsulePadR + 2) // and the 1 DIP border
	for i, sg := range segs {
		if i > 0 {
			w += segmentGap
		}
		w += segmentPadL + measure(sg.label, 13) + segmentKeys + segmentPadR
		for j, cp := range sg.caps {
			if j > 0 {
				w += capGap
			}
			w += max(outlineCap, measure(cp, 12)+2*outlineCapPad+2) // and its border
		}
	}
	return w
}

// noticeRoom is the most a notice capsule may take beside an actions
// capsule actions wide, in a window width wide (design.md 2.4): the
// window less both, the insets at both sides and 8 between.
func noticeRoom(width, actions float32) float32 {
	return width - actions - 24
}

// segment is a clickable part of an actions capsule, its action's name
// and keys, in the selection color while pressed. dim draws it for an
// action that can't be taken.
type segment struct {
	label string
	color ui.Color
	mods  ui.Modifiers
	key   ui.Key
	caps  []string
	dim   bool
	act   func()
}

func (sg segment) draw(c *ui.Context, k tokens) {
	label, color, caps, dim, act := sg.label, sg.color, sg.caps, sg.dim, sg.act
	seg := ui.Row(c).Height(capsuleHeight-6).Radius(14).Padding(0, segmentPadR, 0, segmentPadL).Gap(segmentKeys).
		AlignItems(ui.Center).Shrink(0).Label(label).KeyShortcuts(sg.mods, sg.key)
	if dim {
		// Dimmed by its colors, as opacity under glass draws the glass
		// dark.
		color = color.Alpha(0.4)
		seg.AccessDisabled(true)
	} else {
		if seg.Pressed() {
			seg.Background(k.selection)
		}
		seg.OnClick(act)
	}
	capColor := k.textOnGlass
	if dim {
		capColor = capColor.Alpha(0.4)
	}
	seg.Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(color).SingleLine().Shrink(0)
		outlineKeycaps(c, k, capColor, caps...)
	})
}

// notice is what a notice capsule says, at the bottom left.
type notice struct {
	icon             string
	iconColor, color ui.Color
	text             string
	size             float32 // of the text: 12 unless set
	// extra comes after the text: a button, the × that dismisses.
	extra func()
}

// noticeCapsule shows a notice at the bottom left, inset from the left
// and bottom from the bottom, at most maxWidth wide,
// its text cut at the end if it must, which its label and tooltip say
// whole; fading in and out (design.md 2.4, 3.6).
func noticeCapsule(c *ui.Context, k tokens, inset, bottom, maxWidth float32, n notice) {
	size := n.size
	if size == 0 {
		size = 12
	}
	c.Key("notice")
	e := capsule(c, k, bottom).Left(inset).MaxWidth(maxWidth).Padding(0, 14).Gap(6).Label(n.text).Tooltip(n.text)
	if n.extra != nil {
		e.Padding(0, 8, 0, 14)
	}
	if !c.Preferences().ReduceMotion {
		fade := &ui.Motion{Opacity: 0}
		e.Transition(ui.ElementTransition{Enter: fade, Exit: fade, Duration: 150 * time.Millisecond})
	}
	e.Children(func() {
		sfsymbols.Symbol(c, n.icon).Size(14).Color(n.iconColor)
		ui.Text(c, n.text).FontSize(size).TextColor(n.color).SingleLine().Shrink(1).AccessHidden(true)
		if n.extra != nil {
			n.extra()
		}
	})
}

// capsuleButton is a small button inside a capsule (design.md 3.6): 24
// high, round, in the selection gray.
func capsuleButton(c *ui.Context, k tokens, label string, act func()) {
	b := ui.Row(c).Height(24).Radius(12).Padding(0, 10).AlignItems(ui.Center).Shrink(0).
		Background(k.selection).Role(ui.RoleButton).Label(label).OnClick(act)
	if b.Pressed() {
		b.Background(k.selection.Alpha(2))
	}
	b.Margin(0, 0, 0, 6).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(k.text).SingleLine()
	})
}

// dismissButton is the × of a notice capsule.
func dismissButton(c *ui.Context, k tokens, label string, act func()) {
	ui.Box(c).Size(20, 20).Radius(10).Center().Shrink(0).Role(ui.RoleButton).Label(label).Tooltip(label).
		OnClick(act).Children(func() {
		sfsymbols.Symbol(c, "xmark").Size(10).Color(k.textOnGlass)
	})
}

// --- Popup menus ---

// popupMenu is a menu drawn in its window, on glass (design.md 2.5): the
// actions menu (Cmd-K) and the terminal's new tab menu. Place it with
// Absolute's edges; origin is the corner it grows from, as TransformOrigin.
func popupMenu(c *ui.Context, k tokens, originX, originY float32) ui.Element {
	enter := &ui.Motion{Opacity: 0, Scale: 0.97}
	tr := ui.ElementTransition{Enter: enter, Exit: enter, Duration: 120 * time.Millisecond, ExitDuration: 80 * time.Millisecond}
	if c.Preferences().ReduceMotion {
		fade := &ui.Motion{Opacity: 0}
		tr = ui.ElementTransition{Enter: fade, Exit: fade, Duration: 80 * time.Millisecond}
	}
	return ui.Column(c).Absolute().Width(menuWidth).MaxHeight(menuMaxHeight).
		Radius(12).Material(glassOf(k, searchTint)).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.20)).Clip().
		TransformOrigin(originX, originY).Transition(tr)
}

// menuField is the top of a popup menu, 36 high, over a hairline, for its
// filter field, which starts where the rows' icons do and ends where their
// keys do (design.md 2.5).
func menuField(c *ui.Context, k tokens, field func()) {
	ui.Row(c).Height(36).Shrink(0).Padding(0, menuTextInsetR, 0, menuTextInset).AlignItems(ui.Center).Children(field)
	ui.Box(c).Height(1).Background(k.hairline).Shrink(0)
}

// menuItems is the scrolling list of a popup menu's rows.
func menuItems(c *ui.Context, rows func()) {
	ui.Scroll(c).Shrink(1).Children(func() {
		ui.Column(c).Padding(menuPad).Children(rows)
	})
}

// menuRow is a row of a popup menu: an icon, its name, then at the right
// a note (a folder) or keys, in the selection gray when chosen.
func menuRow(c *ui.Context, k tokens, icon, label, note string, caps []string, chosen bool, act func()) {
	bg := ui.Transparent
	if chosen {
		bg = k.selection
	}
	c.Key(label)
	ui.Row(c).Height(menuRowHeight).Padding(0, menuRowPadR, 0, menuRowPad).Radius(8).Gap(8).AlignItems(ui.Center).Background(bg).
		Role(ui.RoleMenuItem).Label(label).OnClick(act).Children(func() {
		if icon != "" {
			sfsymbols.Symbol(c, icon).Size(16).Color(k.text)
		}
		// The name cut at its end, the keys whole, 12 apart at least
		// (design.md 2.5): the row's 8 and 4.
		ui.Text(c, label).FontSize(13).TextColor(k.text).SingleLine().Shrink(1).Grow(1).Margin(0, menuNameGap-8, 0, 0)
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(k.textOnGlass).SingleLine().Shrink(1)
		}
		if len(caps) > 0 {
			keycaps(c, k, caps, 12)
		}
	})
}

// menuTitle is the small title over a popup menu's rows.
func menuTitle(c *ui.Context, k tokens, title string) {
	ui.Text(c, title).FontSize(12).TextColor(k.textOnGlass).Padding(6, menuRowPad, 4, menuRowPad).SingleLine()
}

// menuSeparator separates groups of a popup menu's rows.
func menuSeparator(c *ui.Context, k tokens) {
	ui.Box(c).Height(1).Background(k.hairline).Margin(4, 8).Shrink(0)
}

// --- The search panel's list (design.md 2.3, 2.5a) ---

// The measures of the search panel's list: a row, its icon's cell (an app
// icon's frame: macOS app icons have about 10% clear around them), a
// preset's square and a letter avatar in it, a section's title, a divider
// with the room over and under it.
const (
	rowHeight             = 40
	iconCell              = 26
	iconSize              = 22
	sectionHeight         = 28
	dividerHeight         = 9
	listInset             = 8                     // the list's padding, left and right
	listRowPadR           = rightLine - listInset // a row's padding at its right
	listTextLeft  float32 = 54
)

// listRow is a row of the search panel's list: left 10, an icon in its
// cell, 10, the title starting 54 from the panel's edge (the list's 8
// included), then the rest, 12 from the right; in the selection gray when
// chosen (a ui.List draws its own).
func listRow(c *ui.Context, k tokens, chosen bool, children func()) ui.Element {
	r := ui.Row(c).Height(rowHeight).Shrink(0).Padding(0, listRowPadR, 0, 10).Gap(10).AlignItems(ui.Center).Radius(8)
	if chosen {
		r.Background(k.selection)
	}
	return r.Children(children)
}

// listIcon is a row's icon cell, its icon centered in it.
func listIcon(c *ui.Context, icon func()) ui.Element {
	return ui.Box(c).Size(iconCell, iconCell).Center().Shrink(0).Children(icon)
}

// listSymbol is a row's icon cell with a symbol, 16, secondary.
func listSymbol(c *ui.Context, k tokens, symbol string) {
	listIcon(c, func() { sfsymbols.Symbol(c, symbol).Size(16).Color(k.textOnGlass) })
}

// presetSquare is a preset's icon: a terminal on a keycap-gray square.
func presetSquare(c *ui.Context, k tokens) {
	listIcon(c, func() {
		ui.Box(c).Size(iconSize, iconSize).Radius(6).Background(k.keycap).Center().Children(func() {
			sfsymbols.Symbol(c, "apple.terminal.fill").Size(13).Color(k.text)
		})
	})
}

// listTitle is a row's name, 14.
func listTitle(c *ui.Context, k tokens, title string) ui.Element {
	return ui.Text(c, title).FontSize(14).TextColor(k.text).SingleLine().Shrink(1)
}

// listNoteMax is the most a row's subtitle, or the folder at its right,
// takes: 40% of the row (design.md 2.3, 2.5a).
const listNoteMax = (panelWidth - 2*listInset) * 0.4

// listTitleLeast is the least a row's title keeps of its width as its
// subtitle takes room (design.md 2.3).
const listTitleLeast = 160

// listNote is the gray text at a row's right, 12. It reaches noteBearing
// past the row's padding: its glyphs end inside their advance (应用 1.5,
// a digit 1.5, App and ~ 0.5 in), and then end within 1 of the right line
// the keys at the right end on (design.md 2.1a).
func listNote(c *ui.Context, k tokens, note string) ui.Element {
	return ui.Text(c, note).FontSize(12).TextColor(k.textOnGlass).SingleLine().Margin(0, -noteBearing, 0, 0)
}

// noteBearing is how much of a 12 text's advance, at its right, is
// commonly blank.
const noteBearing = 1

// listSection is the title of a section of rows: 28 high, 12 Medium,
// secondary, aligned with the rows' titles, 6 over its bottom.
func listSection(c *ui.Context, k tokens, title, label string) {
	ui.Column(c).Height(sectionHeight).Shrink(0).Justify(ui.End).Padding(0, 0, 6, listTextLeft-listInset).
		Label(label).Children(func() {
		ui.Text(c, title).FontSize(12).FontWeight(500).TextColor(k.textOnGlass).SingleLine().AccessHidden(true)
	})
}

// listDivider separates sections of rows: a hairline, 10 in from the
// rows' sides, 4 over and under.
func listDivider(c *ui.Context, k tokens) {
	ui.Box(c).Height(1).Margin(4, 10).Background(k.hairline).Shrink(0).AccessHidden(true)
}

// --- Buttons ---

// roundButton is a round button with a symbol on glass (design.md 3.4): no
// background, the selection gray while pressed or while on (its menu
// open).
func roundButton(c *ui.Context, k tokens, symbol, tip string, on bool) ui.Element {
	b := ui.Box(c).Size(roundButtonSize, roundButtonSize).Radius(roundButtonSize / 2).Center().Shrink(0).
		Role(ui.RoleButton).Label(tip).Tooltip(tip)
	if on || b.Pressed() {
		b.Background(k.selection)
	}
	b.Children(func() { sfsymbols.Symbol(c, symbol).Size(14).Color(k.textOnGlass) })
	return b
}
