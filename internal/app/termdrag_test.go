package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"

	"gocast/internal/settings"
)

// Once the user moved the panel, its edges drag as a window's: the edge
// dragged moves, the other one stays, nothing symmetric; within the
// settings' ranges and the minimum, and never out of the work area, an
// edge dragged past it stopping there (design.md 3.4).
func TestFreeDragRect(t *testing.T) {
	r := rect
	prev := r(200, 300, 908, 380) // moved, off where it shows
	for _, c := range []struct {
		name    string
		b, want mygo.Rectangle
	}{
		{"nothing", prev, prev},
		{"bottom edge down", r(200, 300, 908, 480), r(200, 300, 908, 480)},
		{"top edge up: the bottom stays", r(200, 250, 908, 430), r(200, 250, 908, 430)},
		{"left edge out by 50: the right stays", r(150, 300, 958, 380), r(150, 300, 958, 380)},
		{"right edge out by 30: the left stays", r(200, 300, 938, 380), r(200, 300, 938, 380)},
		{"left edge in by 40", r(240, 300, 868, 380), r(240, 300, 868, 380)},
		{"top-left corner", r(150, 250, 958, 430), r(150, 250, 958, 430)},
		{"bottom-right corner", r(200, 300, 938, 420), r(200, 300, 938, 420)},
		// The least: the edge dragged stops there, the other stays.
		{"bottom edge under the minimum", r(200, 300, 908, 120), r(200, 300, 908, 300)},
		{"top edge under the minimum", r(200, 560, 908, 120), r(200, 380, 908, 300)},
		{"left edge under 43% wide (650)", r(600, 300, 508, 380), r(458, 300, 650, 380)},
		// The work area: the edge dragged stops at it.
		{"left edge past the work area", r(-100, 300, 1208, 380), r(0, 300, 1108, 380)},
		{"right edge past the work area", r(200, 300, 1400, 380), r(200, 300, 1312, 380)},
		{"top edge into the menu bar", r(200, 10, 908, 670), r(200, 33, 908, 647)},
		{"bottom edge past the Dock", r(200, 300, 908, 800), r(200, 300, 908, 682)},
	} {
		if got := freeDragRect(prev, c.b, waMain); got != c.want {
			t.Errorf("%s: from %+v dragged to %+v: %+v, want %+v", c.name, prev, c.b, got, c.want)
		}
	}
	// The most, 90% high, from the edge that stays.
	high := r(200, 100, 908, 380)
	if got, want := freeDragRect(high, r(200, 100, 908, 900), waMain), r(200, 100, 908, 854); got != want {
		t.Errorf("bottom edge over 90%% high: %+v, want %+v", got, want)
	}
}

// The panel the user moved stays in the work area: against its edges at
// most, its size kept, unless the work area is smaller.
func TestKeepIn(t *testing.T) {
	r := rect
	for _, c := range []struct {
		name    string
		b, want mygo.Rectangle
	}{
		{"inside", r(100, 200, 908, 380), r(100, 200, 908, 380)},
		{"against the edges", r(0, 33, 908, 380), r(0, 33, 908, 380)},
		{"out left", r(-300, 200, 908, 380), r(0, 200, 908, 380)},
		{"out right", r(900, 200, 908, 380), r(604, 200, 908, 380)},
		{"under the menu bar", r(100, 20, 908, 380), r(100, 33, 908, 380)},
		{"behind the Dock", r(100, 800, 908, 380), r(100, 602, 908, 380)},
		{"out at a corner", r(1400, 900, 908, 380), r(604, 602, 908, 380)},
		{"larger than the work area", r(-50, 0, 1600, 1000), r(0, 33, 1512, 949)},
	} {
		if got := keepIn(c.b, waMain); got != c.want {
			t.Errorf("%s: %+v kept in as %+v, want %+v", c.name, c.b, got, c.want)
		}
	}
	// On a display left of the main one.
	if got, want := keepIn(r(-100, 300, 908, 380), waLeft), r(-908, 300, 908, 380); got != want {
		t.Errorf("on the left display: %+v, want %+v", got, want)
	}
}

// A move of the panel is the user's only while it is still where it
// showed, not placed by Gocast nor resized; from then on its edges drag as
// a window's, until it shows again, where Settings has it (design.md 3.4).
func TestUserMove(t *testing.T) {
	g := &gocast{}
	g.settings.TerminalPosition = settings.PositionTop
	p := &termPanel{g: g, active: -1, shown: true}
	p.placeFor(mygo.Point{X: 700, Y: 500}, waMain, 2)
	shown := p.pl.rect
	if want := termPlacement(settings.PositionTop, waMain, shown.Width, shown.Height, mygo.Point{}, 0).rect; shown != want {
		t.Fatalf("placed at %+v, want %+v", shown, want)
	}
	p.bounds = shown
	moved := shown
	moved.X, moved.Y = 100, 300

	// No press on the tab bar's blank: the system's, as a display
	// changing.
	if p.movedTo(moved) || p.moved {
		t.Error("a move with no press on the tab bar is the user's")
	}
	p.armMove()

	// Moving in: the slide's steps.
	gen := p.anim.Add(1)
	if p.movedTo(moved) || p.moved {
		t.Error("a step of the slide in is the user's move")
	}
	p.still = gen
	// Gocast's own placing.
	p.placing = true
	if p.movedTo(moved) || p.moved {
		t.Error("Gocast's placing is the user's move")
	}
	p.placing = false
	// Resized: resized's.
	if p.movedTo(rect(shown.X-50, shown.Y, shown.Width+100, shown.Height)) || p.moved {
		t.Error("a resize is the user's move")
	}
	if p.movedTo(shown) || p.moved {
		t.Error("no move is the user's move")
	}
	// Hidden, or hiding.
	p.shown = false
	if p.movedTo(moved) || p.moved {
		t.Error("a move of the panel hidden is the user's")
	}
	p.shown = true

	// Before the move, the edges drag as 3.7 has it: symmetric across.
	left := func(b mygo.Rectangle) mygo.Rectangle { return rect(b.X-50, b.Y, b.Width+50, b.Height) }
	if got, want := p.dragTo(shown, left(shown)), rect(shown.X-50, shown.Y, shown.Width+100, shown.Height); got != want {
		t.Errorf("left edge out before the move: %+v, want %+v", got, want)
	}
	if !p.movedTo(moved) || !p.moved || p.bounds != moved {
		t.Fatalf("the user's move: moved %v, bounds %+v", p.moved, p.bounds)
	}
	// After it, as a window's: the right edge stays.
	if got, want := p.dragTo(moved, left(moved)), left(moved); got != want {
		t.Errorf("left edge out after the move: %+v, want %+v", got, want)
	}

	// Shown again: where Settings has it, its edges as 3.7 has them, and
	// no move the user's till the tab bar is pressed again.
	p.placeFor(mygo.Point{X: 700, Y: 500}, waMain, 2)
	if p.moved || p.armed || p.pl.rect != shown {
		t.Errorf("shown again: moved %v, armed %v, at %+v, want %+v", p.moved, p.armed, p.pl.rect, shown)
	}
	if got, want := p.dragTo(shown, left(shown)), rect(shown.X-50, shown.Y, shown.Width+100, shown.Height); got != want {
		t.Errorf("left edge out, shown again: %+v, want %+v", got, want)
	}
}

// The tab bar moves the panel by its blank and Gocast's mark, its tabs
// still taking their clicks; a press on its blank picks no tab.
func TestTabBarDragsAroundTabs(t *testing.T) {
	p, tt := hintsPanel(t, "en", 1000, 600, "", "first", "second")
	tt.ClickAt(600, tabBarH/2) // the blank between the tabs and the buttons
	tt.Frame()
	if p.active != 0 {
		t.Errorf("a press on the blank picked tab %d", p.active)
	}
	if err := tt.Click("second"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if p.active != 1 {
		t.Errorf("the second tab, clicked: tab %d showing", p.active)
	}
}

// The press that arms a move is the user's while it is down, and for
// armGrace once it lets go, as AppKit tells the window server's move late;
// not after.
func TestArmedMove(t *testing.T) {
	p := &termPanel{g: &gocast{}, active: -1, shown: true, bounds: rect(100, 100, 908, 380)}
	p.armMove()
	if !p.armedNow() {
		t.Error("the press, down: not armed")
	}
	p.letGo = time.Now().Add(-armGrace / 5)
	if !p.armedNow() || !p.movedTo(rect(150, 120, 908, 380)) {
		t.Error("a move told 0.1 s after the release: not the user's")
	}
	p.letGo = time.Now().Add(-2 * armGrace)
	if p.armedNow() || p.movedTo(rect(200, 140, 908, 380)) {
		t.Error("a move long after the release: the user's")
	}
}

// A press on the tab bar's blank, or Gocast's mark, arms a move and keeps
// the keyboard with the terminal; one on a tab or a button does not.
func TestTabBarArmsMove(t *testing.T) {
	p, tt := hintsPanel(t, "en", 1000, 600, "", "first", "second")
	tt.Frame() // the frame that knows where the tabs are
	focused := func() string {
		for _, a := range tt.AccessAll() {
			if a.Focused {
				return a.Name
			}
		}
		return ""
	}
	if p.tabs[0].term != nil {
		was := focused()
		tt.ClickAt(600, tabBarH/2)
		tt.Frame()
		if was == "" || focused() != was {
			t.Errorf("a click on the blank: %q focused, was %q", focused(), was)
		}
	}
	for _, c := range []struct {
		name string
		x    float32
		arms bool
	}{
		{"the blank", 600, true},
		{"Gocast's mark", 30, true},
		{"the padding left", 4, true},
		{"the first tab", 0, false},
		{"the new tab button", 1000 - 12 - 32 - 4 - 16, false},
		{"the hide button", 1000 - 12 - 16, false},
	} {
		x := c.x
		if c.name == "the first tab" {
			r, ok := tt.Find("first")
			if !ok {
				t.Fatal("no first tab")
			}
			x = r.X + r.W/2
		}
		p.armed = false
		tt.Press(x, tabBarH/2)
		if p.armed != c.arms {
			t.Errorf("%s at %v: armed %v, want %v", c.name, x, p.armed, c.arms)
		}
		tt.Release(x, 500) // away, not to click: the panel has no window
		tt.Frame()
	}
}

// Once the user moved the panel out of the work area, it comes back in
// as the drag ends, not while the button is down; on another display, in
// that display's work area, which it then sizes in.
func TestKeepRect(t *testing.T) {
	held := true
	wasHeld, wasDisplay := mouseHeld, displayOf
	mouseHeld = func() bool { return held }
	displayOf = func(b mygo.Rectangle) mygo.Display {
		if b.X+b.Width/2 < 0 {
			return mygo.Display{WorkArea: waLeft, ScaleFactor: 1}
		}
		return mygo.Display{WorkArea: waMain, ScaleFactor: 2}
	}
	t.Cleanup(func() { mouseHeld, displayOf = wasHeld, wasDisplay })
	p := &termPanel{g: &gocast{}, active: -1, shown: true, moved: true, wa: waMain, scale: 2}

	out := rect(1000, 300, 908, 380)
	if r, wait := p.keepRect(out); !wait || r != out {
		t.Errorf("out, held: %+v, wait %v", r, wait)
	}
	in := rect(100, 300, 908, 380)
	if r, wait := p.keepRect(in); wait || r != in {
		t.Errorf("in, held: %+v, wait %v", r, wait)
	}
	held = false
	if r, wait := p.keepRect(out); wait || r != rect(604, 300, 908, 380) {
		t.Errorf("out, let go: %+v, wait %v", r, wait)
	}
	left := rect(-700, 1000, 908, 380)
	if r, wait := p.keepRect(left); wait || r != rect(-908, 700, 908, 380) || p.wa != waLeft || p.scale != 1 {
		t.Errorf("out on the left display: %+v, wait %v, in %+v at %v", r, wait, p.wa, p.scale)
	}
}

// Resized with both edges of an axis moved while the user drags the
// panel moved, the system placed it (as macOS tiles it): kept in the work
// area at that size, not taken for a drag of its edges.
func TestSystemRect(t *testing.T) {
	held := true
	wasHeld, wasDisplay := mouseHeld, displayOf
	mouseHeld = func() bool { return held }
	displayOf = func(mygo.Rectangle) mygo.Display { return mygo.Display{WorkArea: waMain, ScaleFactor: 2} }
	t.Cleanup(func() { mouseHeld, displayOf = wasHeld, wasDisplay })
	p := &termPanel{g: &gocast{}, active: -1, shown: true, moved: true, wa: waMain}
	prev := rect(200, 300, 908, 380)
	tiled := rect(0, 33, 756, 949) // the left half
	if r, ok := p.systemRect(prev, tiled); !ok || r != tiled {
		t.Errorf("tiled: %+v, %v", r, ok)
	}
	if r, ok := p.systemRect(prev, rect(-10, 20, 1600, 1000)); !ok || r != rect(0, 33, 1512, 949) {
		t.Errorf("tiled past the work area: %+v, %v", r, ok)
	}
	for _, b := range []mygo.Rectangle{
		rect(150, 300, 958, 380), // the left edge
		rect(200, 300, 958, 380), // the right edge
		rect(150, 250, 958, 430), // the top-left corner
	} {
		if _, ok := p.systemRect(prev, b); ok {
			t.Errorf("a drag of the edges to %+v: the system's", b)
		}
	}
	held = false
	if _, ok := p.systemRect(prev, tiled); ok {
		t.Error("the button up: the system's")
	}
	held, p.moved = true, false
	if _, ok := p.systemRect(prev, tiled); ok {
		t.Error("not moved by the user: the system's")
	}
}

// A step of a slide that comes once another slide, or a placing
// (replace), started does nothing: it would move the panel from where
// that put it, as a user would.
func TestStepIf(t *testing.T) {
	was := runOnMain
	runOnMain = func(fn func()) { fn() }
	t.Cleanup(func() { runOnMain = was })
	p := &termPanel{g: &gocast{}, active: -1, shown: true}
	gen := p.anim.Add(1)
	steps := 0
	p.stepIf(gen, func() { steps++ })
	p.still = p.anim.Add(1) // replace, as Settings' position changes
	p.stepIf(gen, func() { steps++ })
	if steps != 1 {
		t.Errorf("%d steps, want 1", steps)
	}
}

// Neither edge of an axis dragged, the axis stays as it is, though the
// work area's ranges for it changed, as on another display.
func TestFreeDragRectUntouchedAxis(t *testing.T) {
	small := rect(0, 25, 1000, 600) // 90% high is 540
	prev := rect(100, 30, 700, 590)
	if got, want := freeDragRect(prev, rect(100, 30, 750, 590), small), rect(100, 30, 750, 590); got != want {
		t.Errorf("the right edge on a smaller display: %+v, want %+v", got, want)
	}
}
