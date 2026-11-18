package app

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
)

// The position thumbnail of Settings: a small screen, left of the
// position's select, with a small terminal where the position puts it,
// placed by the floating terminal's own maths (design.md 3.7). In points.
const (
	thumbW, thumbH = 72, 45
	thumbGap       = 2 // termGap, scaled down
	// The least the small terminal is.
	thumbMinW, thumbMinH = 12, 8
	// Near Pointer draws the terminal below the pointer, its tip at
	// thumbTipX, thumbTipY, whatever the real rule would do there: its
	// top at thumbBelowTip, no lower down than thumbBelowMax.
	thumbTipX, thumbTipY = 36, 5
	thumbBelowTip        = 16
	thumbBelowMax        = 35 - thumbBelowTip
	// The terminal moves to a new position in thumbMove; the pointer,
	// and with Reduce Motion the terminals, fade in thumbFade.
	thumbMove = 200 * time.Millisecond
	thumbFade = 120 * time.Millisecond
)

// thumbWorkArea is the thumbnail's work area: under its menu bar, over
// its Dock, with 2 to spare.
var thumbWorkArea = mygo.Rectangle{X: 0, Y: 3, Width: thumbW, Height: 34}

// thumbEase is the move's ease-in-out, cubic(0.4, 0, 0.2, 1).
var thumbEase = ui.CubicBezier(0.4, 0, 0.2, 1)

// thumbRect is where the thumbnail draws the small terminal at pos, the
// floating terminal wPct% of the work area wide and hPct% high: placed
// by placeIn as the terminal is on the screen, Near Pointer aside.
func thumbRect(pos string, wPct, hPct float64) mygo.Rectangle {
	wa := thumbWorkArea
	w := max(int(math.Round(float64(wa.Width)*wPct/100)), thumbMinW)
	h := max(int(math.Round(float64(wa.Height)*hPct/100)), thumbMinH)
	if pos != settings.PositionPointer {
		return placeIn(pos, wa, w, h, mygo.Point{}, thumbGap).rect
	}
	// Below the pointer, always: scaled down, the real rule would cover
	// it at the default size, and its note says where it goes otherwise.
	h = min(h, thumbBelowMax)
	x := clampInside(thumbTipX-w/2, wa.X, wa.Width, w, edgeGap(wa.Width, w, thumbGap))
	return mygo.Rectangle{X: x, Y: thumbBelowTip, Width: w, Height: h}
}

// posThumb is the thumbnail's motion: from where it drew the terminal as
// the position last changed to where it is now.
type posThumb struct {
	pos    string    // the position drawn
	prev   string    // the one before, fading out with Reduce Motion
	from   ui.Rect   // the terminal as drawn when pos changed
	tipA   float32   // the pointer's opacity then
	at     time.Time // when pos changed; zero, drawn in place
	reduce bool      // the change crossfades, with Reduce Motion
}

// thumbLayer is a small terminal drawn, with its opacity.
type thumbLayer struct {
	r     ui.Rect
	alpha float32
}

// thumbFrame is what the thumbnail draws at now for the sizes wPct and
// hPct: the terminals, one but while they crossfade, the pointer's
// opacity, and whether it is still moving.
func (t *posThumb) frame(now time.Time, wPct, hPct float64) (terms []thumbLayer, tipA float32, moving bool) {
	to := uiRect(thumbRect(t.pos, wPct, hPct))
	tipTo := float32(0)
	if t.pos == settings.PositionPointer {
		tipTo = 1
	}
	if t.at.IsZero() {
		return []thumbLayer{{to, 1}}, tipTo, false
	}
	el := now.Sub(t.at)
	fade := float32(min(1, max(0, float64(el)/float64(thumbFade))))
	tipA = t.tipA + (tipTo-t.tipA)*fade
	if t.reduce {
		if fade >= 1 {
			return []thumbLayer{{to, 1}}, tipA, false
		}
		was := uiRect(thumbRect(t.prev, wPct, hPct))
		return []thumbLayer{{was, 1 - fade}, {to, fade}}, tipA, true
	}
	move := float32(min(1, max(0, float64(el)/float64(thumbMove))))
	k := thumbEase(move)
	r := ui.Rect{
		X: t.from.X + (to.X-t.from.X)*k, Y: t.from.Y + (to.Y-t.from.Y)*k,
		W: t.from.W + (to.W-t.from.W)*k, H: t.from.H + (to.H-t.from.H)*k,
	}
	return []thumbLayer{{r, 1}}, tipA, move < 1 || fade < 1
}

// set makes pos the position drawn, from now: the terminal moves there
// from where it is drawn, or with reduce fades out there and in here.
func (t *posThumb) set(pos string, now time.Time, wPct, hPct float64, reduce bool) {
	if pos == t.pos {
		return
	}
	terms, tipA, _ := t.frame(now, wPct, hPct)
	t.from = terms[len(terms)-1].r
	t.prev, t.pos, t.tipA, t.at, t.reduce = t.pos, pos, tipA, now, reduce
}

func uiRect(r mygo.Rectangle) ui.Rect {
	return ui.Rect{X: float32(r.X), Y: float32(r.Y), W: float32(r.Width), H: float32(r.Height)}
}

// thumbColors are the thumbnail's colors, light or dark (design.md 3.7).
type thumbColors struct {
	screen, screenLine, bars, term, termLine, tabs ui.Color
}

func thumbColorsOf(dark bool) thumbColors {
	if dark {
		return thumbColors{
			screen: ui.Hex("#3A3B42"), screenLine: ui.RGBA(255, 255, 255, 0.10), bars: ui.RGBA(255, 255, 255, 0.14),
			term: ui.Hex("#1C1C1E"), termLine: ui.RGBA(255, 255, 255, 0.22), tabs: ui.Hex("#0A84FF"),
		}
	}
	return thumbColors{
		screen: ui.Hex("#DCDDE3"), screenLine: ui.RGBA(0, 0, 0, 0.10), bars: ui.RGBA(0, 0, 0, 0.12),
		term: ui.Hex("#FFFFFF"), termLine: ui.RGBA(0, 0, 0, 0.18), tabs: ui.Hex("#007AFF"),
	}
}

// positionThumb is the thumbnail of the position t draws, for the sizes
// of Settings: a drawing only, hidden from assistive technology, which
// the select beside it says (copy.md 17).
func (p *prefsWindow) positionThumb(c *ui.Context) {
	t := &p.thumb
	col := thumbColorsOf(c.Theme().Dark)
	ui.Box(c).Size(thumbW, thumbH).Shrink(0).AccessHidden(true).Draw(func(pt *ui.Painter, r ui.Rect) {
		terms, tipA, moving := t.frame(pt.Now(), p.termWidth, p.termHeight)
		if moving {
			pt.AnimationFrame()
		}
		drawThumb(pt, r, col, terms, tipA)
	})
}

// drawThumb draws the thumbnail at r: the screen, its menu bar and Dock,
// the terminals and the pointer, at tipA.
func drawThumb(pt *ui.Painter, r ui.Rect, col thumbColors, terms []thumbLayer, tipA float32) {
	s := pt.Scale()
	snap := func(v float32) float32 { return float32(math.Round(float64(v*s))) / s } // to the display's pixels
	at := func(x, y, w, h float32) ui.Rect {
		x0, y0 := snap(r.X+x), snap(r.Y+y)
		return ui.Rect{X: x0, Y: y0, W: snap(r.X+x+w) - x0, H: snap(r.Y+y+h) - y0}
	}
	screen := at(0, 0, thumbW, thumbH)
	pt.Fill(screen, col.screen, 4)
	pt.Stroke(screen, col.screenLine, 4, 1)
	pt.FillRadii(at(0, 0, thumbW, 3), [4]float32{4, 4, 0, 0}, col.bars) // the menu bar
	pt.Fill(at((thumbW-28)/2, thumbH-2-4, 28, 4), col.bars, 1.5)        // the Dock
	for _, l := range terms {
		if l.alpha <= 0 {
			continue
		}
		box := at(l.r.X, l.r.Y, l.r.W, l.r.H)
		pt.Fill(box, col.term.Alpha(l.alpha), 2)
		pt.Stroke(box, col.termLine.Alpha(l.alpha), 2, 1)
		pt.FillRadii(ui.Rect{X: box.X, Y: box.Y, W: box.W, H: 3}, [4]float32{2, 2, 0, 0}, col.tabs.Alpha(l.alpha)) // the tabs
	}
	if tipA > 0 {
		// An arrow 6 wide and 9 high, its tip at thumbTip: black, with a
		// white line of 1 around it, half of a 2 wide stroke under it.
		x, y := snap(r.X+thumbTipX), snap(r.Y+thumbTipY)
		arrow := new(ui.Path).MoveTo(x, y).LineTo(x, y+8).LineTo(x+1.9, y+6.2).LineTo(x+3.1, y+9).
			LineTo(x+4.4, y+8.4).LineTo(x+3.2, y+5.7).LineTo(x+6, y+5.7).Close()
		arrow.Join(ui.MiterJoin)
		pt.StrokePath(arrow, 2, ui.RGB(255, 255, 255).Alpha(tipA))
		pt.FillPath(arrow, ui.RGB(0, 0, 0).Alpha(tipA))
	}
}
