package app

import (
	"github.com/egoist/mygo"

	"gocast/internal/settings"
)

// Where the floating terminal shows, how it moves as it shows and hides,
// and which of its edges stay as the user drags the others (design.md
// 3.7), as pure maths of the work area, the panel's size and the pointer.

// The panel's distances, in DIPs (design.md 3.7).
const (
	termGap = 8 // from the work area's edges, in every position
	// Near the pointer, the panel's top edge is pointerBelow under the
	// pointer, out of the way of the arrow's ~17; flipped above it, its
	// bottom edge pointerAbove over it, above the I-beam's upper half.
	pointerBelow, pointerAbove = 24, 16
)

// The panel's travel as it shows and hides (design.md 1.5, 3.7): at the
// top it drops dropIn and rises riseOut, as in 0.1.8, where its start is
// under the menu bar; anywhere else it travels travelIn and travelOut,
// which start against the work area's edge, never over the next display
// or behind the Dock.
const (
	dropIn, riseOut     = 12, 8
	travelIn, travelOut = 8, 8
	centerSide          = sideAbove // where the panel at the center comes from
	pointerUnplaceable  = sideAbove // ... and the one by the pointer, with room on neither side of it
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
	// Near the pointer: flipped above it, as there was no room below;
	// or against the work area's edge on the side with more room, as
	// there was room on neither side (it may cover the pointer then).
	flipped, unplaceable bool
}

// termPlacement is where a panel w×h shows at pos, one of
// settings.TerminalPositions, in the work area wa of the display with the
// pointer pt (design.md 3.7): 8 from the work area's edges, along an
// axis where it is no larger than the work area less 16, else against
// them; never outside it. pt counts for settings.PositionPointer only.
func termPlacement(pos string, wa mygo.Rectangle, w, h int, pt mygo.Point) termPlace {
	gx, gy := edgeGap(wa.Width, w), edgeGap(wa.Height, h)
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
		x = pt.X - w/2
		below, above := pt.Y+pointerBelow, pt.Y-pointerAbove-h
		switch {
		case below+h <= bottom-gy:
			y = below
			pl.fixY, pl.from = fixStart, sideAbove // unfolding away from the pointer
		case above >= wa.Y+gy:
			y, pl.flipped = above, true
			pl.fixY, pl.from = fixEnd, sideBelow
		default: // against the edge of the side with more room
			pl.unplaceable, pl.from = true, pointerUnplaceable
			if bottom-pt.Y >= pt.Y-wa.Y {
				y, pl.fixY = bottom-gy-h, fixEnd
			} else {
				y, pl.fixY = wa.Y+gy, fixStart
			}
		}
	}
	pl.rect = mygo.Rectangle{X: clampInside(x, wa.X, wa.Width, w, gx), Y: clampInside(y, wa.Y, wa.Height, h, gy), Width: w, Height: h}
	return pl
}

// edgeGap is the panel's distance from the work area's edges along an
// axis span long where it is size: termGap, none where that leaves no
// room.
func edgeGap(span, size int) int {
	if size > span-2*termGap {
		return 0
	}
	return termGap
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
