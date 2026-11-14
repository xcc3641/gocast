package sysact

import (
	"math"
	"slices"
)

// Rect is a rectangle in the screen's coordinates as the Accessibility API
// and MyGo's displays have them: points, the origin at the top left of the
// main display, y growing down.
type Rect struct{ X, Y, W, H float64 }

// Screen is a display: its whole frame, and the part of it windows go in,
// without the menu bar and the Dock (NSScreen's visibleFrame).
type Screen struct{ Frame, Visible Rect }

// The sizes of the window actions that don't fill a part of the screen
// (design.md 2.9 draws them so): Almost Maximize leaves 5% of the screen
// at each side, Reasonable Size is two thirds wide and three quarters
// high, both centred; Make Larger and Make Smaller move each side by 5% of
// the screen, Make Smaller down to a quarter of it.
const (
	almostMargin   = 0.05
	reasonableW    = 2.0 / 3
	reasonableH    = 3.0 / 4
	resizeStep     = 0.05
	smallestOfSide = 0.25
)

// fractions are the parts of the screen each placing action fills: from
// x0 to x1 across and y0 to y1 down, as fractions of the visible frame.
var fractions = map[string][4]float64{
	LeftHalf:       {0, 0.5, 0, 1},
	RightHalf:      {0.5, 1, 0, 1},
	TopHalf:        {0, 1, 0, 0.5},
	BottomHalf:     {0, 1, 0.5, 1},
	TopLeft:        {0, 0.5, 0, 0.5},
	TopRight:       {0.5, 1, 0, 0.5},
	BottomLeft:     {0, 0.5, 0.5, 1},
	BottomRight:    {0.5, 1, 0.5, 1},
	LeftThird:      {0, 1.0 / 3, 0, 1},
	CenterThird:    {1.0 / 3, 2.0 / 3, 0, 1},
	RightThird:     {2.0 / 3, 1, 0, 1},
	LeftTwoThirds:  {0, 2.0 / 3, 0, 1},
	RightTwoThirds: {1.0 / 3, 1, 0, 1},
	Maximize:       {0, 1, 0, 1},
	AlmostMaximize: {almostMargin, 1 - almostMargin, almostMargin, 1 - almostMargin},
	ReasonableSize: {(1 - reasonableW) / 2, (1 + reasonableW) / 2, (1 - reasonableH) / 2, (1 + reasonableH) / 2},
}

// at is the point a fraction f along a side starting at start, length
// long: the side's own ends at 0 and 1, else a whole point, so that the
// parts of one screen meet with no gap and no overlap (the left half ends
// where the right half starts).
func at(start, length, f float64) float64 {
	switch f {
	case 0:
		return start
	case 1:
		return start + length
	}
	return start + math.Round(length*f)
}

// part is the part of vis from fractions f.
func part(vis Rect, f [4]float64) Rect {
	x0, x1 := at(vis.X, vis.W, f[0]), at(vis.X, vis.W, f[1])
	y0, y1 := at(vis.Y, vis.H, f[2]), at(vis.Y, vis.H, f[3])
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Target is where the window action id puts a window at win on a screen
// whose visible frame is vis; ok is false for the actions that need more
// than that (Restore, the displays').
func Target(id string, win, vis Rect) (r Rect, ok bool) {
	if f, ok := fractions[id]; ok {
		return part(vis, f), true
	}
	switch id {
	case Center:
		w, h := min(win.W, vis.W), min(win.H, vis.H)
		return Rect{vis.X + math.Round((vis.W-w)/2), vis.Y + math.Round((vis.H-h)/2), w, h}, true
	case MakeLarger, MakeSmaller:
		dx, dy := math.Round(vis.W*resizeStep), math.Round(vis.H*resizeStep)
		if id == MakeSmaller {
			dx, dy = -dx, -dy
		}
		least := func(side float64) float64 { return math.Round(side * smallestOfSide) }
		w := min(max(win.W+2*dx, min(win.W, least(vis.W))), vis.W)
		h := min(max(win.H+2*dy, min(win.H, least(vis.H))), vis.H)
		// Around the same centre, kept on the screen.
		x := math.Round(win.X + (win.W-w)/2)
		y := math.Round(win.Y + (win.H-h)/2)
		return inside(Rect{x, y, w, h}, vis), true
	}
	return Rect{}, false
}

// inside moves r, no larger than vis, into vis.
func inside(r, vis Rect) Rect {
	r.X = min(max(r.X, vis.X), vis.X+vis.W-r.W)
	r.Y = min(max(r.Y, vis.Y), vis.Y+vis.H-r.H)
	return r
}

// ScreenOf is the index of the screen a window at win is on: the one with
// most of it, else the one nearest its centre; -1 for no screens.
func ScreenOf(win Rect, screens []Screen) int {
	best, most := -1, 0.0
	for i, s := range screens {
		if a := overlap(win, s.Frame); a > most {
			best, most = i, a
		}
	}
	if best >= 0 {
		return best
	}
	cx, cy := win.X+win.W/2, win.Y+win.H/2
	far := math.Inf(1)
	for i, s := range screens {
		dx := max(s.Frame.X-cx, 0, cx-(s.Frame.X+s.Frame.W))
		dy := max(s.Frame.Y-cy, 0, cy-(s.Frame.Y+s.Frame.H))
		if d := dx*dx + dy*dy; d < far {
			best, far = i, d
		}
	}
	return best
}

// overlap is the area a and b share.
func overlap(a, b Rect) float64 {
	w := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
	h := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// OtherScreen is where Move to Next Display (step 1) or Move to Previous
// Display (step -1) puts a window at win, on the screen at index from:
// on the next screen to the right (else below), round to the first, in
// the same place and size relative to its visible frame, so that a window
// in a half is in that half there.
func OtherScreen(win Rect, screens []Screen, from, step int) Rect {
	order := make([]int, len(screens))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		fa, fb := screens[a].Frame, screens[b].Frame
		switch {
		case fa.X != fb.X:
			return cmpFloat(fa.X, fb.X)
		default:
			return cmpFloat(fa.Y, fb.Y)
		}
	})
	pos := slices.Index(order, from)
	n := len(order)
	to := order[((pos+step)%n+n)%n]
	src, dst := screens[from].Visible, screens[to].Visible
	sx, sy := dst.W/src.W, dst.H/src.H
	r := Rect{
		X: dst.X + math.Round((win.X-src.X)*sx),
		Y: dst.Y + math.Round((win.Y-src.Y)*sy),
		W: min(math.Round(win.W*sx), dst.W),
		H: min(math.Round(win.H*sy), dst.H),
	}
	// A window that filled its screen to an edge does so there too.
	if win.X+win.W >= src.X+src.W {
		r.W = dst.X + dst.W - r.X
	}
	if win.Y+win.H >= src.Y+src.H {
		r.H = dst.Y + dst.H - r.Y
	}
	return inside(r, dst)
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
