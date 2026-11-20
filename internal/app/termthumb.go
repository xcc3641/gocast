package app

import (
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
)

// The pictures of the positions in Settings: each a small screen with a
// small terminal where the position puts it, placed by the floating
// terminal's own maths (design.md 3.7). In points.
const (
	thumbW, thumbH = 72, 45
	thumbGap       = 2 // termGap, scaled down
	// The least the small terminal is.
	thumbMinW, thumbMinH = 12, 8
	// At Pointer draws the line typed on under the pointer, whatever the
	// real rule would do there: the pointer's tip at thumbTipX,
	// thumbTipY, on the line thumbLineY to thumbLineY+thumbLineH, inset
	// thumbLineInset in the terminal, whose bottom is at thumbTermBottom
	// and top no higher than thumbTermTop.
	thumbTipX, thumbTipY = 36, 26
	thumbTermBottom      = 31
	thumbTermTop         = 5
	thumbLineY           = 25
	thumbLineH           = 2
	thumbLineInset       = 3
)

// The picture's ring as it is chosen, and the room around it: ringOut
// outside the picture, ringW wide, the ring's frame ringBoxW×ringBoxH
// with corners of ringRadius; the name nameGap under it. A row of six
// needs pictureCell for each, else they go in two rows of three, rowGap
// apart (design.md 3.7).
const (
	ringOut, ringW     = 3, 2
	ringBoxW, ringBoxH = thumbW + 2*ringOut, thumbH + 2*ringOut
	ringRadius         = 7
	nameGap            = 6
	nameLine           = 16
	pictureCell        = 80
	pictureRowGap      = 12
	pressedOpacity     = 0.85
)

// thumbWorkArea is the picture's work area: under its menu bar, over its
// Dock, with 2 to spare.
var thumbWorkArea = mygo.Rectangle{X: 0, Y: 3, Width: thumbW, Height: 34}

// thumbRect is where the picture of pos draws the small terminal, the
// floating terminal wPct% of the work area wide and hPct% high: placed by
// placeIn as the terminal is on the screen; at the pointer, its line
// typed on under the pointer's tip, as the terminal is with an agent's
// input box.
func thumbRect(pos string, wPct, hPct float64) mygo.Rectangle {
	wa := thumbWorkArea
	w := max(int(math.Round(float64(wa.Width)*wPct/100)), thumbMinW)
	h := max(int(math.Round(float64(wa.Height)*hPct/100)), thumbMinH)
	if pos != settings.PositionPointer {
		return placeIn(pos, wa, w, h, mygo.Point{}, 0, thumbGap).rect
	}
	x := clampInside(thumbTipX-w/2, wa.X, wa.Width, w, edgeGap(wa.Width, w, thumbGap))
	top := max(thumbTermBottom-h, thumbTermTop)
	return mygo.Rectangle{X: x, Y: top, Width: w, Height: thumbTermBottom - top}
}

func uiRect(r mygo.Rectangle) ui.Rect {
	return ui.Rect{X: float32(r.X), Y: float32(r.Y), W: float32(r.Width), H: float32(r.Height)}
}

// thumbColors are the pictures' colors, light or dark (design.md 3.7).
type thumbColors struct {
	screen, screenLine, bars, term, termLine, tabs, line ui.Color
}

func thumbColorsOf(dark bool) thumbColors {
	if dark {
		return thumbColors{
			screen: ui.Hex("#3A3B42"), screenLine: ui.RGBA(255, 255, 255, 0.10), bars: ui.RGBA(255, 255, 255, 0.14),
			term: ui.Hex("#1C1C1E"), termLine: ui.RGBA(255, 255, 255, 0.22), tabs: ui.Hex("#0A84FF"),
			line: ui.RGBA(255, 255, 255, 0.30),
		}
	}
	return thumbColors{
		screen: ui.Hex("#DCDDE3"), screenLine: ui.RGBA(0, 0, 0, 0.10), bars: ui.RGBA(0, 0, 0, 0.12),
		term: ui.Hex("#FFFFFF"), termLine: ui.RGBA(0, 0, 0, 0.18), tabs: ui.Hex("#007AFF"),
		line: ui.RGBA(0, 0, 0, 0.25),
	}
}

// positionPicture is the picture of pos, for the sizes of Settings,
// ringed while chosen, with the focus ring outside that while it has the
// keyboard's focus, at pressedOpacity while pressed, its small terminal
// in the pose that pose says as the window paints, once all of the view
// has built and chosen what plays: a drawing only, hidden from assistive technology, which reads
// the radio button around it (copy.md 17). It reports whether the pointer
// is over it.
func (p *prefsWindow) positionPicture(c *ui.Context, pos string, chosen, focused, pressed bool, pose func() thumbPose) bool {
	col := thumbColorsOf(c.Theme().Dark)
	accent := c.Theme().Accent
	term := uiRect(thumbRect(pos, p.termWidth, p.termHeight))
	pointer := pos == settings.PositionPointer
	ring := ui.Box(c).Size(ringBoxW, ringBoxH).Shrink(0).AccessHidden(true).Draw(func(pt *ui.Painter, r ui.Rect) {
		r = snapRect(pt, r)
		if chosen {
			pt.Stroke(r, accent, ringRadius, ringW)
		}
		if focused {
			pt.FocusRing(r, [4]float32{ringRadius, ringRadius, ringRadius, ringRadius})
		}
	})
	ring.Children(func() {
		pic := ui.Box(c).Size(thumbW, thumbH).Margin(ringOut).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
			drawThumb(pt, r, col, term, pointer, pose())
		})
		if pressed {
			pic.Opacity(pressedOpacity)
		}
	})
	return ring.Hovered()
}

// snapRect is r with its edges on the display's pixels.
func snapRect(pt *ui.Painter, r ui.Rect) ui.Rect {
	s := pt.Scale()
	snap := func(v float32) float32 { return float32(math.Round(float64(v*s))) / s }
	x, y := snap(r.X), snap(r.Y)
	return ui.Rect{X: x, Y: y, W: snap(r.X+r.W) - x, H: snap(r.Y+r.H) - y}
}

// drawThumb draws the picture at r: the screen, its menu bar and Dock, the
// small terminal at term, and with pointer the line typed on and the
// pointer over it; the terminal, with its line, moved and faded as pose
// says, within the screen, and the pointer faded.
func drawThumb(pt *ui.Painter, r ui.Rect, col thumbColors, term ui.Rect, pointer bool, pose thumbPose) {
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
	if pose.term > 0 {
		pt.Clip(screen, 4, func() {
			box := at(term.X+pose.dx, term.Y+pose.dy, term.W, term.H)
			pt.Fill(box, col.term.Alpha(pose.term), 2)
			pt.Stroke(box, col.termLine.Alpha(pose.term), 2, 1)
			pt.FillRadii(ui.Rect{X: box.X, Y: box.Y, W: box.W, H: 3}, [4]float32{2, 2, 0, 0}, col.tabs.Alpha(pose.term)) // the tabs
			if pointer {
				line := at(term.X+pose.dx+thumbLineInset, thumbLineY+pose.dy, term.W-2*thumbLineInset, thumbLineH)
				pt.Fill(line, col.line.Alpha(pose.term), 1)
			}
		})
	}
	if !pointer || pose.pointer <= 0 {
		return
	}
	// An arrow 6 wide and 9 high, its tip at thumbTip: black, with a
	// white line of 1 around it, half of a 2 wide stroke under it.
	x, y := snap(r.X+thumbTipX), snap(r.Y+thumbTipY)
	arrow := new(ui.Path).MoveTo(x, y).LineTo(x, y+8).LineTo(x+1.9, y+6.2).LineTo(x+3.1, y+9).
		LineTo(x+4.4, y+8.4).LineTo(x+3.2, y+5.7).LineTo(x+6, y+5.7).Close()
	arrow.Join(ui.MiterJoin)
	pt.StrokePath(arrow, 2, ui.RGB(255, 255, 255).Alpha(pose.pointer))
	pt.FillPath(arrow, ui.RGB(0, 0, 0).Alpha(pose.pointer))
}
