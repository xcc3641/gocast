package app

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

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
// whatever display, with the edge that stays and the side it comes from;
// at the pointer, its line typed on, anchor under its top, there.
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
		anchor     int
		want       mygo.Rectangle
		fixX, fixY axisFix
		from       termSide
	}{
		// 756×569 is the default 50%×60% of the MacBook's work area.
		{"top", top, waMain, 756, 569, pt(10, 10), 0, rect(378, 41, 756, 569), fixCenter, fixStart, sideAbove},
		{"center", center, waMain, 756, 569, pt(10, 10), 0, rect(378, 223, 756, 569), fixCenter, fixCenter, sideAbove},
		{"bottom: 8 over the Dock", bottom, waMain, 756, 569, pt(10, 10), 0, rect(378, 405, 756, 569), fixCenter, fixEnd, sideBelow},
		{"left", left, waMain, 756, 569, pt(10, 10), 0, rect(8, 223, 756, 569), fixStart, fixCenter, sideLeft},
		{"right", right, waMain, 756, 569, pt(10, 10), 0, rect(748, 223, 756, 569), fixEnd, fixCenter, sideRight},
		{"unknown: the top", "middle", waMain, 756, 569, pt(10, 10), 0, rect(378, 41, 756, 569), fixCenter, fixStart, sideAbove},
		{"the line typed on counts at the pointer only", center, waMain, 756, 569, pt(700, 700), 500, rect(378, 223, 756, 569), fixCenter, fixCenter, sideAbove},

		// At the pointer: the line typed on there, here 500 under the
		// top as estimated at 13 points, the panel centered across it.
		{"pointer: the line at it", near, waMain, 756, 569, pt(700, 700), 500, rect(322, 200, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer: the first row at it", near, waMain, 756, 569, pt(700, 300), 62, rect(322, 238, 756, 569), fixCenter, fixEnd, sideBelow},
		// No room over it: against the top, the line under the pointer.
		{"pointer high up: against the top", near, waMain, 756, 569, pt(700, 300), 500, rect(322, 41, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer in the menu bar", near, waMain, 756, 569, pt(756, 5), 500, rect(378, 41, 756, 569), fixCenter, fixEnd, sideBelow},
		// No room under it: against the bottom, over the Dock.
		{"pointer low, the line near the top: against the bottom", near, waMain, 756, 569, pt(700, 900), 100, rect(322, 405, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer: the line at it, 8 over the Dock", near, waMain, 756, 569, pt(756, 905), 500, rect(378, 405, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer: one lower, kept 8 over the Dock", near, waMain, 756, 569, pt(756, 906), 500, rect(378, 405, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer in the Dock", near, waMain, 756, 569, pt(756, 1000), 500, rect(378, 405, 756, 569), fixCenter, fixEnd, sideBelow},
		// Across: inside, 8 from the edges.
		{"pointer at the left edge", near, waMain, 756, 569, pt(0, 700), 500, rect(8, 200, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer at the right edge", near, waMain, 756, 569, pt(1511, 700), 500, rect(748, 200, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer at the bottom-left corner", near, waMain, 756, 569, pt(0, 981), 500, rect(8, 405, 756, 569), fixCenter, fixEnd, sideBelow},
		{"pointer at the top-right corner", near, waMain, 756, 569, pt(1511, 33), 500, rect(748, 41, 756, 569), fixCenter, fixEnd, sideBelow},

		// Wider or taller than the work area less 16: no gap that way.
		{"too tall: top", top, waMain, 756, 940, pt(10, 10), 0, rect(378, 33, 756, 940), fixCenter, fixStart, sideAbove},
		{"too tall: bottom", bottom, waMain, 756, 940, pt(10, 10), 0, rect(378, 42, 756, 940), fixCenter, fixEnd, sideBelow},
		{"too tall: pointer", near, waMain, 756, 949, pt(700, 500), 880, rect(322, 33, 756, 949), fixCenter, fixEnd, sideBelow},
		{"too wide: left", left, waMain, 1512, 569, pt(10, 10), 0, rect(0, 223, 1512, 569), fixStart, fixCenter, sideLeft},
		{"too wide: right", right, waMain, 1500, 569, pt(10, 10), 0, rect(12, 223, 1500, 569), fixEnd, fixCenter, sideRight},
		{"too wide: pointer", near, waMain, 1512, 569, pt(1500, 700), 500, rect(0, 200, 1512, 569), fixCenter, fixEnd, sideBelow},

		// The display left of the main one, at negative X.
		{"left display: top", top, waLeft, 960, 633, pt(-500, 500), 0, rect(-1440, 33, 960, 633), fixCenter, fixStart, sideAbove},
		{"left display: right", right, waLeft, 960, 633, pt(-500, 500), 0, rect(-968, 236, 960, 633), fixEnd, fixCenter, sideRight},
		{"left display: left", left, waLeft, 960, 633, pt(-500, 500), 0, rect(-1912, 236, 960, 633), fixStart, fixCenter, sideLeft},
		{"left display: pointer", near, waLeft, 960, 633, pt(-500, 800), 560, rect(-980, 240, 960, 633), fixCenter, fixEnd, sideBelow},
		{"left display: pointer at its right edge", near, waLeft, 960, 633, pt(-1, 800), 560, rect(-968, 240, 960, 633), fixCenter, fixEnd, sideBelow},
		{"left display: pointer at its bottom-left corner", near, waLeft, 960, 633, pt(-1920, 1079), 560, rect(-1912, 439, 960, 633), fixCenter, fixEnd, sideBelow},
		// The larger display above the main one, at negative Y.
		{"display above: center", center, waAbove, 1280, 849, pt(1000, -700), 0, rect(840, -1157, 1280, 849), fixCenter, fixCenter, sideAbove},
		{"display above: bottom", bottom, waAbove, 1280, 849, pt(1000, -700), 0, rect(840, -882, 1280, 849), fixCenter, fixEnd, sideBelow},
		{"display above: pointer", near, waAbove, 1280, 849, pt(1000, -300), 750, rect(360, -1050, 1280, 849), fixCenter, fixEnd, sideBelow},
		{"display above: pointer at its bottom-right corner", near, waAbove, 1280, 849, pt(2759, -26), 750, rect(1472, -882, 1280, 849), fixCenter, fixEnd, sideBelow},
		{"display above: pointer at its top-left corner", near, waAbove, 1280, 849, pt(200, -1440), 750, rect(208, -1432, 1280, 849), fixCenter, fixEnd, sideBelow},
	} {
		pl := termPlacement(c.pos, c.wa, c.w, c.h, c.pt, c.anchor)
		if pl.rect != c.want {
			t.Errorf("%s: at %+v, want %+v", c.name, pl.rect, c.want)
		}
		if pl.fixX != c.fixX || pl.fixY != c.fixY || pl.from != c.from {
			t.Errorf("%s: fixed %v,%v from %v; want %v,%v from %v", c.name, pl.fixX, pl.fixY, pl.from, c.fixX, c.fixY, c.from)
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
				pl := termPlacement(settings.PositionTop, wa, w, h, mygo.Point{X: wa.X + 3, Y: wa.Y + 700}, 0)
				if want := rect(wa.X+(wa.Width-w)/2, wa.Y+8, w, h); pl.rect != want || pl.in != 12 || pl.out != 8 {
					t.Fatalf("%+v, %d×%d: at %+v travelling %d/%d, want %+v travelling 12/8", wa, w, h, pl.rect, pl.in, pl.out, want)
				}
			}
		}
	}
}

// In no position, on no display, is the panel ever partly out of the
// work area, or of another size; at the pointer, its line typed on is at
// the pointer, unless that would take the panel out: it is against the
// work area's edge then, 8 from it where there is room.
func TestTermPlacementInside(t *testing.T) {
	inside := func(r, wa mygo.Rectangle) bool {
		return r.X >= wa.X && r.Y >= wa.Y && r.X+r.Width <= wa.X+wa.Width && r.Y+r.Height <= wa.Y+wa.Height
	}
	for _, wa := range []mygo.Rectangle{waMain, waLeft, waAbove, rect(0, 0, 640, 300)} {
		for _, size := range [][2]int{{640, 300}, {wa.Width / 2, wa.Height * 6 / 10}, {wa.Width, wa.Height * 9 / 10}, {wa.Width, wa.Height}} {
			w, h := min(size[0], wa.Width), min(size[1], wa.Height)
			gy := edgeGap(wa.Height, h, termGap)
			for _, anchor := range []int{62, h / 2, h - 60} {
				for px := wa.X - 40; px <= wa.X+wa.Width+40; px += 37 {
					for py := wa.Y - 30; py <= wa.Y+wa.Height+30; py += 23 {
						for _, pos := range settings.TerminalPositions {
							name := fmt.Sprintf("%s in %+v, %d×%d, pointer %d,%d, line %d down", pos, wa, w, h, px, py, anchor)
							r := termPlacement(pos, wa, w, h, mygo.Point{X: px, Y: py}, anchor).rect
							if !inside(r, wa) || r.Width != w || r.Height != h {
								t.Fatalf("%s: at %+v, out of the work area", name, r)
							}
							if pos == settings.PositionPointer && r.Y+anchor != py && r.Y != wa.Y+gy && r.Y+h != wa.Y+wa.Height-gy {
								t.Fatalf("%s: at %+v, the line at %d, with room for it at the pointer", name, r, r.Y+anchor)
							}
						}
					}
				}
			}
		}
	}
}

// The panel comes in from its side and goes back out to it: at the top
// 12 down and 8 up, as in 0.1.8, no higher than the work area; anywhere
// else 8 each way; at the pointer up, its line typed on rising to the
// pointer, as at the bottom (design.md 3.7).
func TestTermMotion(t *testing.T) {
	p := func(x, y int) mygo.Point { return mygo.Point{X: x, Y: y} }
	for _, c := range []struct {
		pos     string
		in, out mygo.Point // relative to where it rests
	}{
		{settings.PositionTop, p(0, -12), p(0, -8)},
		{settings.PositionCenter, p(0, -8), p(0, -8)},
		{settings.PositionBottom, p(0, 8), p(0, 8)},
		{settings.PositionLeft, p(-8, 0), p(-8, 0)},
		{settings.PositionRight, p(8, 0), p(8, 0)},
		{settings.PositionPointer, p(0, 8), p(0, 8)},
	} {
		pl := termPlacement(c.pos, waMain, 756, 569, p(700, 700), 500)
		r := pl.rect
		if got := entryFrom(pl); got != p(r.X+c.in.X, r.Y+c.in.Y) {
			t.Errorf("%s: in from %+v, want %+v off %d,%d", c.pos, got, c.in, r.X, r.Y)
		}
		if got := exitTo(pl, r, waMain); got != p(r.X+c.out.X, r.Y+c.out.Y) {
			t.Errorf("%s: out to %+v, want %+v off %d,%d", c.pos, got, c.out, r.X, r.Y)
		}
	}
	// At the top, 8 under the work area's top, it hides up to the top
	// only, under the menu bar, as in 0.1.8.
	pl := termPlacement(settings.PositionTop, waMain, 756, 569, mygo.Point{}, 0)
	if got := exitTo(pl, pl.rect, waMain); got != p(378, 33) {
		t.Errorf("top: out to %+v, want 378,33", got)
	}
}

// Dragging the panel's edges keeps the edge it shows against where it is,
// and the center line across it, both at the center (design.md 3.7); a
// side that reaches the work area's edge stops there.
func TestDragRect(t *testing.T) {
	r := rect
	at := func(pos string, h int, pt mygo.Point) termPlace { return termPlacement(pos, waMain, 908, h, pt, 300) }
	top := at(settings.PositionTop, 380, mygo.Point{})                       // 302, 41
	bottom := at(settings.PositionBottom, 380, mygo.Point{})                 // 302, 594
	left := at(settings.PositionLeft, 380, mygo.Point{})                     // 8, 317
	right := at(settings.PositionRight, 380, mygo.Point{})                   // 596, 317
	center := at(settings.PositionCenter, 380, mygo.Point{})                 // 302, 317
	pointer := at(settings.PositionPointer, 380, mygo.Point{X: 500, Y: 700}) // 46, 400: its line, 300 down, at the pointer
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

		// At the pointer, as at the bottom: the line typed on stays
		// where it is, near the bottom.
		{"pointer: top edge up", pointer, r(46, 350, 908, 430), r(46, 350, 908, 430)},
		{"pointer: bottom edge, nothing", pointer, r(46, 400, 908, 420), r(46, 400, 908, 380)},
		{"pointer: top-left corner", pointer, r(26, 350, 928, 430), r(26, 350, 948, 430)},
		// Its left side reaches the work area's edge: stops, the right
		// one goes on.
		{"pointer: right edge out by 100", pointer, r(46, 400, 1008, 380), r(0, 400, 1108, 380)},
	} {
		prev := c.pl.rect
		if got := dragRect(prev, c.b, waMain, c.pl); got != c.want {
			t.Errorf("%s: from %+v dragged to %+v: %+v, want %+v", c.name, prev, c.b, got, c.want)
		}
	}
}

// At the pointer, the line typed on is the cursor's row, as the terminal
// lays its rows out under the tab bar, kept to the rows the panel holds;
// with no cursor known, 3.5 lines of 1.25 the font size over the bottom
// of the terminal's room, which capsules raise (design.md 3.7).
func TestCursorAnchor(t *testing.T) {
	// Estimated: 569 − 12 − 3.5 × 16.25 = 500.1, at 13 points; with
	// capsules, 569 − 60 − 56.9.
	for _, c := range []struct {
		h        int
		size     float32
		capsules bool
		want     int
	}{
		{569, 13, false, 500},
		{569, 13, true, 452},
		{300, 13, false, 231},
		{569, 20, false, 470}, // 569 − 12 − 87.5
		{854, 11, true, 746},  // 854 − 60 − 48.1
	} {
		if got := cursorAnchor(c.h, c.size, 2, c.capsules, -1); got != c.want {
			t.Errorf("estimated, %d high at %v points, capsules %v: %d, want %d", c.h, c.size, c.capsules, got, c.want)
		}
	}
	// The cursor's row: its middle, the rows from under the outline and
	// the tab bar, a whole number of pixels high each.
	for _, scale := range []float64{1, 2} {
		for _, size := range []float32{11, 13, 20} {
			cell := float64(termCellPixels(size, scale)) / scale
			if want := float64(size) * termLineHeight; cell < want || cell > want*1.4 {
				t.Fatalf("rows at %v points, %v×: %v high", size, scale, cell)
			}
			for _, capsules := range []bool{false, true} {
				bottom := 569 - termPadBottom
				if capsules {
					bottom = 569 - termPadCapsules
				}
				name := fmt.Sprintf("%v points at %v×, capsules %v", size, scale, capsules)
				first := cursorAnchor(569, size, scale, capsules, 0)
				if math.Abs(float64(first)-(termGridTop+cell/2)) > 1 {
					t.Errorf("%s: the first row's middle at %d, want %.1f", name, first, termGridTop+cell/2)
				}
				if got, want := cursorAnchor(569, size, scale, capsules, 10), termGridTop+10.5*cell; math.Abs(float64(got)-want) > 1 {
					t.Errorf("%s: the 11th row's middle at %d, want %.1f", name, got, want)
				}
				// Rows the panel doesn't hold, as a taller terminal's at
				// the host: the last it holds, which shows whole over the
				// bottom padding.
				last := cursorAnchor(569, size, scale, capsules, 1000)
				if float64(last)+cell/2 > float64(bottom)+1 || float64(last)+cell*1.5 < float64(bottom)-1 {
					t.Errorf("%s: the last row's middle at %d, %.1f high, the room to %d", name, last, cell, bottom)
				}
				if below := cursorAnchor(569, size, scale, capsules, 999); below != last {
					t.Errorf("%s: rows 999 and 1000 at %d and %d", name, below, last)
				}
			}
		}
	}
}

// The panel at the pointer has its line typed on where the tab showing
// says, at its own font size, else estimated: for a new tab, as one that
// has not started yet, and with the plain terminal's tip over its bottom.
func TestTypedAnchor(t *testing.T) {
	row := -1
	was := sessionCursor
	sessionCursor = func(*session) int { return row }
	t.Cleanup(func() { sessionCursor = was })
	g := &gocast{}
	p := &termPanel{g: g, active: -1, pos: settings.PositionPointer, scale: 2}
	if got := p.typedAnchor(569); got != 500 {
		t.Errorf("no tab: %d, want 500", got)
	}
	p.tabs, p.active = []*session{{presetID: "claude", pending: true, fontSize: 13}}, 0
	if got := p.typedAnchor(569); got != 500 {
		t.Errorf("a new tab: %d, want 500", got)
	}
	p.tabs[0].presetID = "" // a plain terminal with no presets: the tip shows
	if got := p.typedAnchor(569); got != 452 {
		t.Errorf("under the tip: %d, want 452", got)
	}
	row = 4
	p.tabs[0].fontSize = 16
	if got, want := p.typedAnchor(569), cursorAnchor(569, 16, 2, true, 4); got != want {
		t.Errorf("on row 4 at 16 points: %d, want %d", got, want)
	}
	p.pos = settings.PositionBottom
	if got := p.typedAnchor(569); got != 0 {
		t.Errorf("at the bottom: %d", got)
	}
}

// Choosing a position in Settings, by its picture or its name or with the
// arrows, saves it at once, for the next show, the note over the pictures
// saying how At Pointer places the terminal while it is chosen (copy.md
// 17). The pictures are one radio group, which the arrows go through and
// stop at the ends of.
func TestPositionSetting(t *testing.T) {
	p, tt := positionPrefs(t, "zh-Hans", settings.PositionTop)
	tt.SetSize(prefsWidth, 900)
	tt.Frame()
	saved := func() string {
		t.Helper()
		st, err := settings.Load(p.g.settingsPath())
		if err != nil {
			t.Fatal(err)
		}
		return st.TermPosition()
	}
	const hint, pointerHint = "在鼠标所在的那块屏幕上。", "终端里正在输入的那一行对准鼠标。"
	if !tt.HasText(hint) {
		t.Fatalf("no note: %q", tt.Texts())
	}

	// The picture of 鼠标处.
	at, ok := tt.AccessByRole(ui.RoleRadio, "鼠标处")
	if !ok {
		t.Fatalf("no 鼠标处: %+v", tt.AccessAll())
	}
	tt.ClickAt(at.Bounds.X+at.Bounds.W/2, at.Bounds.Y+ringBoxH/2)
	tt.Frame()
	if got := saved(); got != settings.PositionPointer {
		t.Fatalf("saved %q", got)
	}
	if !tt.HasText(pointerHint) || tt.HasText(hint) {
		t.Errorf("At Pointer's note: %q", tt.Texts())
	}
	group, ok := tt.AccessByRole(ui.RoleRadioGroup, "出现位置")
	if !ok || group.Description != pointerHint {
		t.Errorf("the radio group: %+v, %v", group, ok)
	}
	for _, pos := range settings.TerminalPositions {
		a, _ := tt.AccessByRole(ui.RoleRadio, positionLabel(pos))
		if a.Checked != (pos == settings.PositionPointer) {
			t.Errorf("%s: checked %v", a.Name, a.Checked)
		}
	}

	// The name of 顶部, under its picture.
	if err := tt.Click("顶部"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := saved(); got != settings.PositionTop || !tt.HasText(hint) {
		t.Errorf("back to the top: saved %q, %q", got, tt.Texts())
	}

	// The keyboard: one stop of Tab, at the one chosen.
	for range 40 {
		if tt.Focused("顶部") {
			break
		}
		tt.Key(0, ui.KeyTab)
	}
	if !tt.Focused("顶部") {
		t.Fatal("Tab never reaches the pictures")
	}
	key := func(k ui.Key, want string) {
		t.Helper()
		tt.Key(0, k)
		tt.Frame()
		if got := saved(); got != want || !tt.Focused(positionLabel(want)) {
			t.Errorf("after %v: saved %q, want %q, focused there %v", k, got, want, tt.Focused(positionLabel(want)))
		}
	}
	key(ui.KeyLeft, settings.PositionTop) // the first: not round to the last
	key(ui.KeyUp, settings.PositionTop)
	key(ui.KeyRight, settings.PositionCenter)
	key(ui.KeyDown, settings.PositionBottom)
	key(ui.KeyUp, settings.PositionCenter)
	key(ui.KeyRight, settings.PositionBottom)
	key(ui.KeyRight, settings.PositionLeft)
	key(ui.KeyRight, settings.PositionRight)
	key(ui.KeyRight, settings.PositionPointer)
	key(ui.KeyRight, settings.PositionPointer) // the last: not round to the first
	key(ui.KeyDown, settings.PositionPointer)
	if !tt.HasText(pointerHint) {
		t.Errorf("At Pointer's note, from the keyboard: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyTab)
	if tt.Focused("顶部") || tt.Focused("中央") {
		t.Error("Tab went from picture to picture")
	}
}
