package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo"

	"gocast/internal/settings"
)

// The work areas of the tests: a MacBook's under its menu bar, with the
// Dock at the bottom; a display left of it, whose origin is negative; and
// a larger one above it, its menu bar elsewhere.
var (
	waMain  = mygo.Rectangle{X: 0, Y: 33, Width: 1512, Height: 949} // bottom 982
	waLeft  = mygo.Rectangle{X: -1920, Y: 25, Width: 1920, Height: 1055}
	waAbove = mygo.Rectangle{X: 200, Y: -1440, Width: 2560, Height: 1415}
)

func rect(x, y, w, h int) mygo.Rectangle { return mygo.Rectangle{X: x, Y: y, Width: w, Height: h} }

// Each position places the panel as design.md 3.7's table has it, on
// whatever display, with the edge that stays and the side it comes from.
func TestTermPlacement(t *testing.T) {
	const (
		top, center, bottom = settings.PositionTop, settings.PositionCenter, settings.PositionBottom
		left, right, near   = settings.PositionLeft, settings.PositionRight, settings.PositionPointer
	)
	pt := func(x, y int) mygo.Point { return mygo.Point{X: x, Y: y} }
	for _, c := range []struct {
		name       string
		pos        string
		wa         mygo.Rectangle
		w, h       int
		pt         mygo.Point
		want       mygo.Rectangle
		fixX, fixY axisFix
		from       termSide
		flipped    bool
		neither    bool
	}{
		// 756×569 is the default 50%×60% of the MacBook's work area.
		{"top", top, waMain, 756, 569, pt(10, 10), rect(378, 41, 756, 569), fixCenter, fixStart, sideAbove, false, false},
		{"center", center, waMain, 756, 569, pt(10, 10), rect(378, 223, 756, 569), fixCenter, fixCenter, sideAbove, false, false},
		{"bottom: 8 over the Dock", bottom, waMain, 756, 569, pt(10, 10), rect(378, 405, 756, 569), fixCenter, fixEnd, sideBelow, false, false},
		{"left", left, waMain, 756, 569, pt(10, 10), rect(8, 223, 756, 569), fixStart, fixCenter, sideLeft, false, false},
		{"right", right, waMain, 756, 569, pt(10, 10), rect(748, 223, 756, 569), fixEnd, fixCenter, sideRight, false, false},
		{"unknown: the top", "middle", waMain, 756, 569, pt(10, 10), rect(378, 41, 756, 569), fixCenter, fixStart, sideAbove, false, false},

		// By the pointer: 24 under it, centered on it.
		{"pointer", near, waMain, 756, 400, pt(700, 100), rect(322, 124, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer at the left edge", near, waMain, 756, 400, pt(0, 500), rect(8, 524, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer at the right edge", near, waMain, 756, 400, pt(1511, 500), rect(748, 524, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer at the top edge", near, waMain, 756, 400, pt(756, 33), rect(378, 57, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer at the bottom edge: above", near, waMain, 756, 400, pt(756, 981), rect(378, 565, 756, 400), fixCenter, fixEnd, sideBelow, true, false},
		{"pointer at the top-left corner", near, waMain, 756, 400, pt(0, 33), rect(8, 57, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer at the top-right corner", near, waMain, 756, 400, pt(1511, 33), rect(748, 57, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer at the bottom-left corner", near, waMain, 756, 400, pt(0, 981), rect(8, 565, 756, 400), fixCenter, fixEnd, sideBelow, true, false},
		{"pointer at the bottom-right corner", near, waMain, 756, 400, pt(1511, 981), rect(748, 565, 756, 400), fixCenter, fixEnd, sideBelow, true, false},
		// It flips once its bottom would be under the work area's less 8:
		// 550 + 24 + 400 = 974 still fits.
		{"pointer: last below", near, waMain, 756, 400, pt(756, 550), rect(378, 574, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer: first above, 16 over it", near, waMain, 756, 400, pt(756, 551), rect(378, 135, 756, 400), fixCenter, fixEnd, sideBelow, true, false},
		// 569 high fits below up to 381, above from 626: between, against
		// the edge of the side with more room, coming in as at the center.
		{"pointer: room on neither side, more below", near, waMain, 756, 569, pt(756, 500), rect(378, 405, 756, 569), fixCenter, fixEnd, sideAbove, false, true},
		{"pointer: room on neither side, more above", near, waMain, 756, 569, pt(756, 520), rect(378, 41, 756, 569), fixCenter, fixStart, sideAbove, false, true},
		// Off the work area, in the menu bar or the Dock: still inside it.
		{"pointer in the menu bar", near, waMain, 756, 400, pt(756, 5), rect(378, 41, 756, 400), fixCenter, fixStart, sideAbove, false, false},
		{"pointer in the Dock", near, waMain, 756, 400, pt(756, 1000), rect(378, 574, 756, 400), fixCenter, fixEnd, sideBelow, true, false},

		// Wider or taller than the work area less 16: no gap that way.
		{"too tall: top", top, waMain, 756, 940, pt(10, 10), rect(378, 33, 756, 940), fixCenter, fixStart, sideAbove, false, false},
		{"too tall: bottom", bottom, waMain, 756, 940, pt(10, 10), rect(378, 42, 756, 940), fixCenter, fixEnd, sideBelow, false, false},
		{"too tall: pointer", near, waMain, 756, 949, pt(700, 500), rect(322, 33, 756, 949), fixCenter, fixEnd, sideAbove, false, true},
		{"too wide: left", left, waMain, 1512, 569, pt(10, 10), rect(0, 223, 1512, 569), fixStart, fixCenter, sideLeft, false, false},
		{"too wide: right", right, waMain, 1500, 569, pt(10, 10), rect(12, 223, 1500, 569), fixEnd, fixCenter, sideRight, false, false},
		{"too wide: pointer", near, waMain, 1512, 400, pt(1500, 100), rect(0, 124, 1512, 400), fixCenter, fixStart, sideAbove, false, false},

		// The display left of the main one, at negative X.
		{"left display: top", top, waLeft, 960, 633, pt(-500, 500), rect(-1440, 33, 960, 633), fixCenter, fixStart, sideAbove, false, false},
		{"left display: right", right, waLeft, 960, 633, pt(-500, 500), rect(-968, 236, 960, 633), fixEnd, fixCenter, sideRight, false, false},
		{"left display: left", left, waLeft, 960, 633, pt(-500, 500), rect(-1912, 236, 960, 633), fixStart, fixCenter, sideLeft, false, false},
		{"left display: pointer at its right edge", near, waLeft, 960, 633, pt(-1, 300), rect(-968, 324, 960, 633), fixCenter, fixStart, sideAbove, false, false},
		{"left display: pointer at its bottom-left corner", near, waLeft, 960, 633, pt(-1920, 1079), rect(-1912, 430, 960, 633), fixCenter, fixEnd, sideBelow, true, false},
		// The larger display above the main one, at negative Y.
		{"display above: center", center, waAbove, 1280, 849, pt(1000, -700), rect(840, -1157, 1280, 849), fixCenter, fixCenter, sideAbove, false, false},
		{"display above: bottom", bottom, waAbove, 1280, 849, pt(1000, -700), rect(840, -882, 1280, 849), fixCenter, fixEnd, sideBelow, false, false},
		{"display above: pointer at its bottom-right corner", near, waAbove, 1280, 849, pt(2759, -26), rect(1472, -891, 1280, 849), fixCenter, fixEnd, sideBelow, true, false},
		{"display above: pointer at its top-left corner", near, waAbove, 1280, 849, pt(200, -1440), rect(208, -1416, 1280, 849), fixCenter, fixStart, sideAbove, false, false},
	} {
		pl := termPlacement(c.pos, c.wa, c.w, c.h, c.pt)
		if pl.rect != c.want {
			t.Errorf("%s: at %+v, want %+v", c.name, pl.rect, c.want)
		}
		if pl.fixX != c.fixX || pl.fixY != c.fixY || pl.from != c.from || pl.flipped != c.flipped || pl.unplaceable != c.neither {
			t.Errorf("%s: fixed %v,%v from %v flipped %v neither %v; want %v,%v from %v flipped %v neither %v", c.name,
				pl.fixX, pl.fixY, pl.from, pl.flipped, pl.unplaceable, c.fixX, c.fixY, c.from, c.flipped, c.neither)
		}
	}
}

// At the top, the default, the panel shows where 0.1.8 showed it, at any
// size of any display: centered, 8 under the work area's top.
func TestTermPlacementTopAsBefore(t *testing.T) {
	for _, wa := range []mygo.Rectangle{waMain, waLeft, waAbove, rect(0, 25, 1440, 875), rect(1512, 0, 1366, 743), rect(0, 38, 1511, 944)} {
		for wpct := settings.MinTerminalWidth; wpct <= settings.MaxTerminalWidth; wpct += 7 {
			for hpct := settings.MinTerminalHeight; hpct <= settings.MaxTerminalHeight; hpct += 3 {
				w, h := termSize(wa.Width, wa.Height, settings.Settings{TerminalWidth: wpct, TerminalHeight: hpct})
				pl := termPlacement(settings.PositionTop, wa, w, h, mygo.Point{X: wa.X + 3, Y: wa.Y + 700})
				if want := rect(wa.X+(wa.Width-w)/2, wa.Y+8, w, h); pl.rect != want || pl.in != 12 || pl.out != 8 {
					t.Fatalf("%+v, %d×%d: at %+v travelling %d/%d, want %+v travelling 12/8", wa, w, h, pl.rect, pl.in, pl.out, want)
				}
			}
		}
	}
}

// In no position, on no display, is the panel ever partly out of the
// work area; by the pointer, it never covers it unless there was room
// on neither side, and is then on the side with more room.
func TestTermPlacementInside(t *testing.T) {
	inside := func(r, wa mygo.Rectangle) bool {
		return r.X >= wa.X && r.Y >= wa.Y && r.X+r.Width <= wa.X+wa.Width && r.Y+r.Height <= wa.Y+wa.Height
	}
	for _, wa := range []mygo.Rectangle{waMain, waLeft, waAbove, rect(0, 0, 640, 300)} {
		for _, size := range [][2]int{{640, 300}, {wa.Width / 2, wa.Height * 6 / 10}, {wa.Width, wa.Height * 9 / 10}, {wa.Width, wa.Height}} {
			w, h := min(size[0], wa.Width), min(size[1], wa.Height)
			for px := wa.X - 40; px <= wa.X+wa.Width+40; px += 37 {
				for py := wa.Y - 30; py <= wa.Y+wa.Height+30; py += 23 {
					for _, pos := range settings.TerminalPositions {
						name := fmt.Sprintf("%s in %+v, %d×%d, pointer %d,%d", pos, wa, w, h, px, py)
						pl := termPlacement(pos, wa, w, h, mygo.Point{X: px, Y: py})
						r := pl.rect
						if !inside(r, wa) || r.Width != w || r.Height != h {
							t.Fatalf("%s: at %+v, out of the work area", name, r)
						}
						if pos != settings.PositionPointer || pl.unplaceable {
							continue
						}
						if py >= r.Y && py < r.Y+r.Height {
							t.Fatalf("%s: at %+v, over the pointer", name, r)
						}
					}
				}
			}
		}
	}
}

// The panel comes in from its side and goes back out to it: at the top
// 12 down and 8 up, as in 0.1.8, no higher than the work area; anywhere
// else 8 each way, by the pointer away from it (design.md 3.7).
func TestTermMotion(t *testing.T) {
	p := func(x, y int) mygo.Point { return mygo.Point{X: x, Y: y} }
	for _, c := range []struct {
		pos      string
		pt       mygo.Point
		in, out  mygo.Point // relative to where it rests
		nearSide string
	}{
		{settings.PositionTop, p(0, 0), p(0, -12), p(0, -8), ""},
		{settings.PositionCenter, p(0, 0), p(0, -8), p(0, -8), ""},
		{settings.PositionBottom, p(0, 0), p(0, 8), p(0, 8), ""},
		{settings.PositionLeft, p(0, 0), p(-8, 0), p(-8, 0), ""},
		{settings.PositionRight, p(0, 0), p(8, 0), p(8, 0), ""},
		{settings.PositionPointer, p(700, 100), p(0, -8), p(0, -8), "below"},
		{settings.PositionPointer, p(700, 900), p(0, 8), p(0, 8), "above"},
		{settings.PositionPointer, p(700, 500), p(0, -8), p(0, -8), "neither: as at the center"},
	} {
		h := 400
		if c.nearSide == "neither: as at the center" {
			h = 569
		}
		pl := termPlacement(c.pos, waMain, 756, h, c.pt)
		r := pl.rect
		if got := entryFrom(pl); got != p(r.X+c.in.X, r.Y+c.in.Y) {
			t.Errorf("%s %s: in from %+v, want %+v off %d,%d", c.pos, c.nearSide, got, c.in, r.X, r.Y)
		}
		if got := exitTo(pl, r, waMain); got != p(r.X+c.out.X, r.Y+c.out.Y) {
			t.Errorf("%s %s: out to %+v, want %+v off %d,%d", c.pos, c.nearSide, got, c.out, r.X, r.Y)
		}
	}
	// At the top, 8 under the work area's top, it hides up to the top
	// only, under the menu bar, as in 0.1.8.
	pl := termPlacement(settings.PositionTop, waMain, 756, 569, mygo.Point{})
	if got := exitTo(pl, pl.rect, waMain); got != p(378, 33) {
		t.Errorf("top: out to %+v, want 378,33", got)
	}
}

// Dragging the panel's edges keeps the edge it shows against where it is,
// and the center line across it, both at the center (design.md 3.7); a
// side that reaches the work area's edge stops there.
func TestDragRect(t *testing.T) {
	r := rect
	at := func(pos string, h int, pt mygo.Point) termPlace { return termPlacement(pos, waMain, 908, h, pt) }
	top := at(settings.PositionTop, 380, mygo.Point{})                       // 302, 41
	bottom := at(settings.PositionBottom, 380, mygo.Point{})                 // 302, 594
	left := at(settings.PositionLeft, 380, mygo.Point{})                     // 8, 317
	right := at(settings.PositionRight, 380, mygo.Point{})                   // 596, 317
	center := at(settings.PositionCenter, 380, mygo.Point{})                 // 302, 317
	below := at(settings.PositionPointer, 380, mygo.Point{X: 500, Y: 100})   // 46, 124
	above := at(settings.PositionPointer, 380, mygo.Point{X: 500, Y: 900})   // 46, 504
	neither := at(settings.PositionPointer, 569, mygo.Point{X: 700, Y: 500}) // 246, 405: against the bottom
	for _, c := range []struct {
		name string
		pl   termPlace
		b    mygo.Rectangle
		want mygo.Rectangle
	}{
		// At the top, as in 0.1.8.
		{"top: bottom edge down", top, r(302, 41, 908, 480), r(302, 41, 908, 480)},
		{"top: bottom edge up", top, r(302, 41, 908, 330), r(302, 41, 908, 330)},
		{"top: left edge out by 50, both sides", top, r(252, 41, 958, 380), r(252, 41, 1008, 380)},
		{"top: left edge in by 40", top, r(342, 41, 868, 380), r(342, 41, 828, 380)},
		{"top: right edge out by 30, both sides", top, r(302, 41, 938, 380), r(272, 41, 968, 380)},
		{"top: bottom-right corner", top, r(302, 41, 938, 420), r(272, 41, 968, 420)},
		{"top: top edge, nothing", top, r(302, 21, 908, 400), r(302, 41, 908, 380)},
		{"top: under the minimum", top, r(302, 41, 908, 120), r(302, 41, 908, 300)},
		{"top: over 90% high", top, r(302, 41, 908, 940), r(302, 41, 908, 854)},
		{"top: wider than the screen", top, r(0, 41, 1300, 380), r(0, 41, 1512, 380)},
		{"top: under 43% wide (650)", top, r(500, 41, 512, 380), r(431, 41, 650, 380)},

		{"bottom: top edge up", bottom, r(302, 494, 908, 480), r(302, 494, 908, 480)},
		{"bottom: bottom edge, nothing", bottom, r(302, 594, 908, 400), r(302, 594, 908, 380)},
		{"bottom: left edge out by 50, both sides", bottom, r(252, 594, 958, 380), r(252, 594, 1008, 380)},
		{"bottom: top-left corner", bottom, r(252, 494, 958, 480), r(252, 494, 1008, 480)},
		{"bottom: over 90% high", bottom, r(302, 0, 908, 974), r(302, 120, 908, 854)},

		{"left: right edge out", left, r(8, 317, 1008, 380), r(8, 317, 1008, 380)},
		{"left: left edge, nothing", left, r(58, 317, 858, 380), r(8, 317, 908, 380)},
		{"left: bottom edge down by 100, both sides", left, r(8, 317, 908, 480), r(8, 217, 908, 580)},
		{"left: top edge up by 50, both sides", left, r(8, 267, 908, 430), r(8, 267, 908, 480)},
		{"left: bottom-right corner", left, r(8, 317, 1008, 480), r(8, 217, 1008, 580)},

		{"right: left edge out", right, r(496, 317, 1008, 380), r(496, 317, 1008, 380)},
		{"right: right edge, nothing", right, r(596, 317, 958, 380), r(596, 317, 908, 380)},
		{"right: top-left corner", right, r(496, 267, 1008, 430), r(496, 267, 1008, 480)},

		{"center: right edge out by 50", center, r(302, 317, 958, 380), r(252, 317, 1008, 380)},
		{"center: bottom edge down by 50", center, r(302, 317, 908, 430), r(302, 267, 908, 480)},
		{"center: top edge up by 50", center, r(302, 267, 908, 430), r(302, 267, 908, 480)},
		{"center: left edge in by 50", center, r(352, 317, 858, 380), r(352, 317, 808, 380)},

		{"pointer below: bottom edge down", below, r(46, 124, 908, 480), r(46, 124, 908, 480)},
		{"pointer below: top edge, nothing", below, r(46, 74, 908, 430), r(46, 124, 908, 380)},
		// Its left side reaches the work area's edge: stops, the right
		// one goes on.
		{"pointer below: right edge out by 100", below, r(46, 124, 1008, 380), r(0, 124, 1108, 380)},
		{"pointer above: top edge up", above, r(46, 454, 908, 430), r(46, 454, 908, 430)},
		{"pointer above: bottom edge, nothing", above, r(46, 504, 908, 400), r(46, 504, 908, 380)},
		{"pointer, neither side: top edge up", neither, r(246, 355, 908, 619), r(246, 355, 908, 619)},
		{"pointer, neither side: bottom edge, nothing", neither, r(246, 405, 908, 600), r(246, 405, 908, 569)},
	} {
		prev := c.pl.rect
		if got := dragRect(prev, c.b, waMain, c.pl); got != c.want {
			t.Errorf("%s: from %+v dragged to %+v: %+v, want %+v", c.name, prev, c.b, got, c.want)
		}
	}
}

// Choosing a position in Settings saves it at once, for the next show,
// with the note under it saying how Near Pointer places the terminal
// while it is chosen (copy.md 17).
func TestPositionSetting(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "zh-Hans")
	p.g.term = &termPanel{g: p.g} // hidden: nothing to move
	p.page = pagePresets
	p.termPos = positionLabel(p.g.settings.TermPosition())
	tt.SetSize(prefsWidth, 900)
	tt.Frame()
	if !tt.HasText("在鼠标所在的那块屏幕上。") {
		t.Fatalf("no note: %q", tt.Texts())
	}
	choose := func(from, label string) {
		t.Helper()
		if err := tt.Click(from); err != nil { // the select, showing its choice
			t.Fatal(err)
		}
		tt.Frame()
		if err := tt.Click(label); err != nil {
			t.Fatalf("%v, of %q", err, tt.Texts())
		}
		tt.Frame()
	}
	choose("顶部居中", "鼠标旁边")
	saved, err := settings.Load(p.g.settingsPath())
	if err != nil || saved.TermPosition() != settings.PositionPointer {
		t.Fatalf("saved %q, %v", saved.TerminalPosition, err)
	}
	if !tt.HasText("在鼠标下方出现，下面放不下就到上方。") || tt.HasText("在鼠标所在的那块屏幕上。") {
		t.Errorf("Near Pointer's note: %q", tt.Texts())
	}
	choose("鼠标旁边", "顶部居中")
	saved, _ = settings.Load(p.g.settingsPath())
	if saved.TerminalPosition != "" || !tt.HasText("在鼠标所在的那块屏幕上。") {
		t.Errorf("back to the top: saved %q, %q", saved.TerminalPosition, tt.Texts())
	}
}
