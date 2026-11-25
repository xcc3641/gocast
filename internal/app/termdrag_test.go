package app

import (
	"testing"

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

	// Shown again: where Settings has it, its edges as 3.7 has them.
	p.placeFor(mygo.Point{X: 700, Y: 500}, waMain, 2)
	if p.moved || p.pl.rect != shown {
		t.Errorf("shown again: moved %v, at %+v, want %+v", p.moved, p.pl.rect, shown)
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
