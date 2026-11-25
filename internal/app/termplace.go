package app

import (
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
)

// Where the floating terminal shows, how it moves as it shows and hides,
// and which of its edges stay as the user drags the others (design.md
// 3.7), as pure maths of the work area, the panel's size, the pointer and,
// at the pointer, the line typed on.

// termGap is the panel's distance from the work area's edges, in every
// position, in DIPs (design.md 3.7).
const termGap = 8

// The panel's travel as it shows and hides (design.md 1.5, 3.7): at the
// top it drops dropIn and rises riseOut, as in 0.1.8, where its start is
// under the menu bar; anywhere else it travels travelIn and travelOut,
// which start against the work area's edge, never over the next display
// or behind the Dock.
const (
	dropIn, riseOut     = 12, 8
	travelIn, travelOut = 8, 8
	centerSide          = sideAbove // where the panel at the center comes from
)

// termSide is the side the panel comes from as it shows, and goes back
// to as it hides.
type termSide int

const (
	sideAbove termSide = iota // it drops
	sideBelow                 // it rises
	sideLeft
	sideRight
)

// axisFix is what stays along an axis as the user drags the panel's
// edges: its start (left, top), its end (right, bottom), or its center
// line, the edges then moving symmetrically.
type axisFix int

const (
	fixCenter axisFix = iota
	fixStart
	fixEnd
)

// termPlace is where the panel shows, and how it behaves there.
type termPlace struct {
	rect mygo.Rectangle
	// What stays, across and down, as the user drags its edges.
	fixX, fixY axisFix
	// The side it comes from, and how far it travels in and out.
	from    termSide
	in, out int
}

// termPlacement is where a panel w×h shows at pos, one of
// settings.TerminalPositions, in the work area wa of the display with the
// pointer pt (design.md 3.7): 8 from the work area's edges, along an
// axis where it is no larger than the work area less 16, else against
// them; never outside it. pt counts for settings.PositionPointer only,
// where the panel's line typed on, anchor DIPs under its top
// (cursorAnchor), is at the pointer, the panel centered across it.
func termPlacement(pos string, wa mygo.Rectangle, w, h int, pt mygo.Point, anchor int) termPlace {
	return placeIn(pos, wa, w, h, pt, anchor, termGap)
}

// placeIn is termPlacement with gap for termGap: the same rules on a
// screen of another scale, as Settings' pictures draw them.
func placeIn(pos string, wa mygo.Rectangle, w, h int, pt mygo.Point, anchor, gap int) termPlace {
	gx, gy := edgeGap(wa.Width, w, gap), edgeGap(wa.Height, h, gap)
	cx, cy := wa.X+(wa.Width-w)/2, wa.Y+(wa.Height-h)/2
	right, bottom := wa.X+wa.Width, wa.Y+wa.Height
	pl := termPlace{in: travelIn, out: travelOut}
	x, y := cx, cy
	switch pos {
	default: // settings.PositionTop, as in 0.1.8
		y = wa.Y + gy
		pl.fixY, pl.from, pl.in, pl.out = fixStart, sideAbove, dropIn, riseOut
	case settings.PositionCenter:
		pl.from = centerSide
	case settings.PositionBottom:
		y = bottom - gy - h
		pl.fixY, pl.from = fixEnd, sideBelow
	case settings.PositionLeft:
		x = wa.X + gx
		pl.fixX, pl.from = fixStart, sideLeft
	case settings.PositionRight:
		x = right - gx - w
		pl.fixX, pl.from = fixEnd, sideRight
	case settings.PositionPointer:
		// Over the pointer, mostly: the line typed on, near the bottom
		// as an agent's input box is, rises to the pointer and stops
		// there, and stays as the panel grows up. Kept inside the work
		// area below, where the line then misses the pointer.
		x, y = pt.X-w/2, pt.Y-anchor
		pl.fixY, pl.from = fixEnd, sideBelow
	}
	pl.rect = mygo.Rectangle{X: clampInside(x, wa.X, wa.Width, w, gx), Y: clampInside(y, wa.Y, wa.Height, h, gy), Width: w, Height: h}
	return pl
}

// The line typed on, at the pointer (design.md 3.7). A terminal that
// tells no cursor, as one not started yet, has its line estimated where
// an agent's input box is: its middle typedLinesUp lines, each its font's
// line height times its size, over the bottom of the terminal's room,
// with a line and two of status under it.
const typedLinesUp = 3.5

// termGridTop is how far under the panel's top the terminal's rows start:
// under its outline and the tab bar, the terminal having no padding of
// its own (termPanel.view, run).
const termGridTop = panelBorderWidth + tabBarH

// cursorAnchor is how far under the top of a panel h high the middle of
// its line typed on is, which shows at the pointer (design.md 3.7), for a
// tab of font f, its bottom padding that of capsules or not: of the
// cursor's row, on a display of scale, as the terminal lays its rows out,
// kept to the rows the panel holds; estimated with a row < 0, not known.
func cursorAnchor(h int, f terminal.Font, scale float64, capsules bool, row int) int {
	bottom := float64(termPadBottom)
	if capsules {
		bottom = termPadCapsules
	}
	if row < 0 {
		return int(math.Round(float64(h) - bottom - typedLinesUp*float64(f.Size)*float64(termRows(f))))
	}
	// As the terminal's view: its cells a whole number of pixels high, its
	// rows from the pixel its top is on.
	cell := termCellPixels(f, scale)
	room := float64(h) - 2*panelBorderWidth - tabBarH - bottom
	rows := max(int(math.Round(room*scale))/cell, 1)
	row = min(row, rows-1)
	top := math.Round(termGridTop*scale) / scale
	return int(math.Round(top + (float64(row)+0.5)*float64(cell)/scale))
}

// termCellPixels is how many pixels high the terminal's rows are in font
// f on a display of scale, as MyGo's terminal view makes them: the
// font's line, f.LineHeight high, up to a whole pixel.
func termCellPixels(f terminal.Font, scale float64) int {
	family := f.Family
	if family == "" {
		family = "monospace"
	}
	m := ui.Font{Family: family, Size: f.Size, Weight: 400}.Metrics()
	return max(int(math.Ceil(float64((m.Ascent+m.Descent+m.LineGap)*termRows(f)*float32(scale)-0.05))), 1)
}

// termRows is the height of the terminal's rows in font f, as a multiple
// of its line: 1 when unset, as the terminal takes it.
func termRows(f terminal.Font) float32 {
	if f.LineHeight <= 0 {
		return 1
	}
	return f.LineHeight
}

// edgeGap is the panel's distance from the work area's edges along an
// axis span long where it is size: gap, none where that leaves no room.
func edgeGap(span, size, gap int) int {
	if size > span-2*gap {
		return 0
	}
	return gap
}

// clampInside is v, the start of size along an axis from lo span long,
// kept gap inside its ends; at lo where size is larger than span.
func clampInside(v, lo, span, size, gap int) int {
	if size > span {
		return lo
	}
	return min(max(v, lo+gap), lo+span-gap-size)
}

// entryFrom is where the panel placed at pl starts as it shows: in from
// its side.
func entryFrom(pl termPlace) mygo.Point {
	x, y := pl.rect.X, pl.rect.Y
	switch pl.from {
	case sideAbove:
		y -= pl.in
	case sideBelow:
		y += pl.in
	case sideLeft:
		x -= pl.in
	case sideRight:
		x += pl.in
	}
	return mygo.Point{X: x, Y: y}
}

// exitTo is where the panel placed at pl, now at b, goes as it hides, in
// the work area wa: out to its side, no higher than the top of wa, as
// AppKit keeps it under the menu bar.
func exitTo(pl termPlace, b, wa mygo.Rectangle) mygo.Point {
	x, y := b.X, b.Y
	switch pl.from {
	case sideAbove:
		y = max(y-pl.out, wa.Y)
	case sideBelow:
		y += pl.out
	case sideLeft:
		x -= pl.out
	case sideRight:
		x += pl.out
	}
	return mygo.Point{X: x, Y: y}
}

// dragRect is the panel's bounds as the user drags its edges from prev to
// b, in the work area wa, placed as pl (design.md 3.7): an edge that
// stays does not move however it is dragged; along an axis whose center
// line stays, an edge dragged moves the other one as much the other way;
// a corner does both its edges. Within the settings' ranges and the
// minimum, and never outside wa: a side that reaches its edge stops
// there, the other one going on.
func dragRect(prev, b, wa mygo.Rectangle, pl termPlace) mygo.Rectangle {
	w := dragAxis(prev.X, prev.Width, b.X, b.Width, pl.fixX)
	h := dragAxis(prev.Y, prev.Height, b.Y, b.Height, pl.fixY)
	w = clampSize(w, wa.Width, settings.MinTermWidth(wa.Width), settings.MaxTerminalWidth, termMinWidth)
	h = clampSize(h, wa.Height, settings.MinTerminalHeight, settings.MaxTerminalHeight, termMinHeight)
	x := fixAxis(prev.X, prev.Width, w, pl.fixX)
	y := fixAxis(prev.Y, prev.Height, h, pl.fixY)
	return mygo.Rectangle{X: clampInside(x, wa.X, wa.Width, w, 0), Y: clampInside(y, wa.Y, wa.Height, h, 0), Width: w, Height: h}
}

// dragAxis is the panel's size along an axis as its edges there go from
// start p, size ps to start b, size bs, with fix staying.
func dragAxis(p, ps, b, bs int, fix axisFix) int {
	startMoved := b != p
	endMoved := !startMoved && bs != ps
	switch {
	case fix == fixCenter && startMoved:
		return 2*p + ps - 2*b // twice the center, in whole DIPs, less twice the start
	case fix == fixCenter && endMoved:
		return 2*(b+bs) - (2*p + ps)
	case fix == fixStart && endMoved:
		return bs
	case fix == fixEnd && startMoved:
		return p + ps - b
	}
	return ps
}

// fixAxis is the start along an axis of the panel, size there now, which
// was at p, ps long, with fix staying.
func fixAxis(p, ps, size int, fix axisFix) int {
	switch fix {
	case fixStart:
		return p
	case fixEnd:
		return p + ps - size
	}
	return p + (ps-size)/2
}
