package sysact

import (
	"errors"
	"fmt"
	"testing"
)

func TestCatalog(t *testing.T) {
	var sys, win int
	seen := map[string]bool{}
	for _, a := range Actions {
		if seen[a.ID] {
			t.Errorf("%s twice", a.ID)
		}
		seen[a.ID] = true
		if len(Aliases[a.ID]) == 0 {
			t.Errorf("%s has no aliases", a.ID)
		}
		_, _, _, _, drawn := Diagram(a.ID)
		switch a.Kind {
		case KindSystem:
			sys++
			if a.Symbol == "" || drawn {
				t.Errorf("%s: a system action has a symbol, no diagram", a.ID)
			}
		case KindWindow:
			win++
			if (a.Symbol == "") != drawn {
				t.Errorf("%s: a window action has a symbol or a diagram, one of them", a.ID)
			}
			if a.Confirm {
				t.Errorf("%s: no window action asks first", a.ID)
			}
		}
	}
	if sys != 24 || win != 22 {
		t.Fatalf("%d system and %d window actions, want 24 and 22 (copy.md 15)", sys, win)
	}
	var confirm []string
	for _, a := range Actions {
		if a.Confirm {
			confirm = append(confirm, a.ID)
		}
	}
	if fmt.Sprint(confirm) != fmt.Sprint([]string{Restart, ShutDown, LogOut, QuitAll, EmptyTrash}) {
		t.Errorf("confirm first: %v (copy.md 15.3)", confirm)
	}
	for id := range Aliases {
		if !seen[id] {
			t.Errorf("aliases of %q, no such action", id)
		}
	}
	if _, ok := Find("nope"); ok {
		t.Error("Find found what is not")
	}
	if a, ok := Find(Center); !ok || a.Kind != KindWindow {
		t.Errorf("Find(center) = %v, %v", a, ok)
	}
}

func TestDiagramInside(t *testing.T) {
	for _, a := range Actions {
		x, y, w, h, ok := Diagram(a.ID)
		if !ok {
			continue
		}
		if x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > 12 || y+h > 8 {
			t.Errorf("%s: %v %v %v %v outside the 12×8 inner area", a.ID, x, y, w, h)
		}
	}
}

func TestVolumeLevel(t *testing.T) {
	for id, want := range map[string]int{Volume0: 0, Volume25: 25, Volume50: 50, Volume75: 75, Volume100: 100, VolumeUp: -1} {
		if got := VolumeLevel(id); got != want {
			t.Errorf("VolumeLevel(%s) = %d, want %d", id, got, want)
		}
	}
}

func TestStepVolume(t *testing.T) {
	for _, c := range []struct {
		from float64
		up   bool
		want float64
	}{
		{0, true, 1.0 / 16},
		{0.5, true, 9.0 / 16},
		{9.0 / 16, true, 10.0 / 16},
		{0.5, false, 7.0 / 16},
		{0.3, true, 5.0 / 16}, // between two: the next above
		{0.3, false, 4.0 / 16},
		{1, true, 1},
		{0, false, 0},
		{0.0001, false, 0},
	} {
		if got := StepVolume(c.from, c.up); got != c.want {
			t.Errorf("StepVolume(%v, %v) = %v, want %v", c.from, c.up, got, c.want)
		}
	}
	if Percent(9.0/16) != 56 || Percent(1) != 100 {
		t.Errorf("Percent: %d, %d", Percent(9.0/16), Percent(1))
	}
}

// A display of a MacBook: the menu bar 33 high, the Dock 70 at the bottom.
var laptop = Screen{Frame: Rect{0, 0, 1512, 982}, Visible: Rect{0, 33, 1512, 879}}

func TestTargetsFillWithoutGaps(t *testing.T) {
	vis := laptop.Visible
	win := Rect{100, 100, 800, 600}
	get := func(id string) Rect {
		r, ok := Target(id, win, vis)
		if !ok {
			t.Fatalf("no target for %s", id)
		}
		return r
	}
	for _, pair := range [][2]string{{LeftHalf, RightHalf}, {LeftThird, CenterThird}, {CenterThird, RightThird}, {LeftTwoThirds, RightThird}, {LeftThird, RightTwoThirds}, {TopLeft, TopRight}, {BottomLeft, BottomRight}} {
		a, b := get(pair[0]), get(pair[1])
		if a.X+a.W != b.X {
			t.Errorf("%s ends at %v, %s starts at %v", pair[0], a.X+a.W, pair[1], b.X)
		}
	}
	for _, pair := range [][2]string{{TopHalf, BottomHalf}, {TopLeft, BottomLeft}, {TopRight, BottomRight}} {
		a, b := get(pair[0]), get(pair[1])
		if a.Y+a.H != b.Y {
			t.Errorf("%s ends at %v, %s starts at %v", pair[0], a.Y+a.H, pair[1], b.Y)
		}
	}
	if r := get(Maximize); r != vis {
		t.Errorf("maximize = %v, want the visible frame %v", r, vis)
	}
	if r := get(LeftHalf); r != (Rect{0, 33, 756, 879}) {
		t.Errorf("left half = %v", r)
	}
	if r := get(RightThird); r.X+r.W != vis.X+vis.W {
		t.Errorf("right third ends at %v, not the screen's edge", r.X+r.W)
	}
	if r := get(BottomHalf); r.Y+r.H != vis.Y+vis.H {
		t.Errorf("bottom half ends at %v, not the Dock", r.Y+r.H)
	}
	if r := get(Center); r != (Rect{356, 173, 800, 600}) {
		t.Errorf("center = %v", r)
	}
	if r := get(AlmostMaximize); r.X <= vis.X || r.X+r.W >= vis.X+vis.W {
		t.Errorf("almost maximize = %v fills the screen", r)
	}
	// A window larger than the screen, centred: as large as the screen.
	if r, _ := Target(Center, Rect{0, 0, 3000, 2000}, vis); r != vis {
		t.Errorf("center of a huge window = %v", r)
	}
}

func TestLargerAndSmaller(t *testing.T) {
	vis := laptop.Visible
	win := Rect{400, 300, 600, 400}
	r, _ := Target(MakeLarger, win, vis)
	if r.W <= win.W || r.H <= win.H || r.X+r.W/2 != win.X+win.W/2 {
		t.Errorf("larger = %v from %v: not larger about its centre", r, win)
	}
	r, _ = Target(MakeSmaller, win, vis)
	if r.W >= win.W || r.H >= win.H {
		t.Errorf("smaller = %v from %v", r, win)
	}
	// Larger at most the screen, kept on it.
	r, _ = Target(MakeLarger, vis, vis)
	if r != vis {
		t.Errorf("larger of a maximized window = %v", r)
	}
	r, _ = Target(MakeLarger, Rect{1200, 600, 300, 300}, vis)
	if r.X+r.W > vis.X+vis.W || r.Y+r.H > vis.Y+vis.H {
		t.Errorf("larger in a corner = %v, off the screen", r)
	}
	// Smaller stops at a quarter of the screen.
	small := Rect{0, 33, 300, 200}
	r, _ = Target(MakeSmaller, small, vis)
	if r.W < 300 && r.W < 378 || r.H < 200 && r.H < 220 {
		t.Errorf("smaller = %v, smaller than a quarter of the screen", r)
	}
}

// Two displays: an external one to the right of the laptop's, larger,
// its menu bar 25 high and no Dock.
var external = Screen{Frame: Rect{1512, -300, 2560, 1440}, Visible: Rect{1512, -275, 2560, 1415}}

func TestScreenOf(t *testing.T) {
	screens := []Screen{laptop, external}
	if i := ScreenOf(Rect{100, 100, 500, 500}, screens); i != 0 {
		t.Errorf("on the laptop: %d", i)
	}
	if i := ScreenOf(Rect{1400, 100, 1000, 500}, screens); i != 1 {
		t.Errorf("mostly on the external: %d", i)
	}
	if i := ScreenOf(Rect{5000, 100, 100, 100}, screens); i != 1 {
		t.Errorf("off to the right, nearest the external: %d", i)
	}
	if i := ScreenOf(Rect{}, nil); i != -1 {
		t.Errorf("no screens: %d", i)
	}
}

func TestOtherScreen(t *testing.T) {
	screens := []Screen{external, laptop} // the order the system gives is not left to right
	left, _ := Target(LeftHalf, Rect{}, laptop.Visible)
	r := OtherScreen(left, screens, 1, 1)
	if want, _ := Target(LeftHalf, Rect{}, external.Visible); r != want {
		t.Errorf("left half to the next display = %v, want %v", r, want)
	}
	// Round: the next of the rightmost is the leftmost; the previous of
	// the leftmost the rightmost.
	if got := ScreenOf(OtherScreen(Rect{2000, 0, 400, 300}, screens, 0, 1), screens); got != 1 {
		t.Errorf("next of the rightmost is on %d", got)
	}
	if got := ScreenOf(OtherScreen(Rect{100, 100, 400, 300}, screens, 1, -1), screens); got != 0 {
		t.Errorf("previous of the leftmost is on %d", got)
	}
	// A maximized window is maximized there too.
	if r := OtherScreen(external.Visible, screens, 0, 1); r != laptop.Visible {
		t.Errorf("maximized onto the laptop = %v", r)
	}
}

// --- Arranger, with fake windows and screens ---

type fakeWindows struct {
	trusted bool
	screens []Screen
	wins    map[int]*fakeWindow // by pid
	err     error
}

func (f *fakeWindows) Trusted() bool     { return f.trusted }
func (f *fakeWindows) Screens() []Screen { return f.screens }
func (f *fakeWindows) Focused(pid int) (Window, error) {
	if f.err != nil {
		return nil, f.err
	}
	w, ok := f.wins[pid]
	if !ok {
		return nil, ErrNoWindow
	}
	return w, nil
}

type fakeWindow struct {
	key     string
	frame   Rect
	least   Rect // its least size, W and H
	sets    int
	release int
}

func (w *fakeWindow) Key() string          { return w.key }
func (w *fakeWindow) Frame() (Rect, error) { return w.frame, nil }
func (w *fakeWindow) SetFrame(r Rect) error {
	w.sets++
	r.W, r.H = max(r.W, w.least.W), max(r.H, w.least.H)
	w.frame = r
	return nil
}
func (w *fakeWindow) Release() { w.release++ }

func newArranger(screens ...Screen) (*Arranger, *fakeWindow) {
	w := &fakeWindow{key: "42:1", frame: Rect{200, 200, 700, 500}}
	return &Arranger{W: &fakeWindows{trusted: true, screens: screens, wins: map[int]*fakeWindow{42: w}}}, w
}

func TestArrangeAndRestore(t *testing.T) {
	a, w := newArranger(laptop)
	orig := w.frame
	if err := a.Arrange(Restore, 42); !errors.Is(err, ErrNotMoved) {
		t.Fatalf("restore before a move: %v", err)
	}
	if err := a.Arrange(LeftHalf, 42); err != nil {
		t.Fatal(err)
	}
	if want, _ := Target(LeftHalf, orig, laptop.Visible); w.frame != want {
		t.Fatalf("left half: %v, want %v", w.frame, want)
	}
	if err := a.Arrange(Maximize, 42); err != nil {
		t.Fatal(err)
	}
	// Restore goes back to before the first move, not the last.
	if err := a.Arrange(Restore, 42); err != nil {
		t.Fatal(err)
	}
	if w.frame != orig {
		t.Fatalf("restored to %v, want %v", w.frame, orig)
	}
	if err := a.Arrange(Restore, 42); !errors.Is(err, ErrNotMoved) {
		t.Fatalf("restore twice: %v", err)
	}
	if w.release == 0 {
		t.Error("the window was never released")
	}
}

func TestRestoreAfterTheUserMovedIt(t *testing.T) {
	a, w := newArranger(laptop)
	if err := a.Arrange(RightHalf, 42); err != nil {
		t.Fatal(err)
	}
	w.frame = Rect{50, 60, 640, 480} // the user dragged it
	if err := a.Arrange(Restore, 42); !errors.Is(err, ErrNotMoved) {
		t.Fatalf("restore after the user moved it: %v", err)
	}
	user := w.frame
	if err := a.Arrange(TopHalf, 42); err != nil {
		t.Fatal(err)
	}
	if err := a.Arrange(Restore, 42); err != nil || w.frame != user {
		t.Fatalf("restored to %v (%v), want where the user left it, %v", w.frame, err, user)
	}
}

func TestCannotShrinkPinsTopLeft(t *testing.T) {
	a, w := newArranger(laptop)
	w.least = Rect{W: 600, H: 500}
	if err := a.Arrange(LeftThird, 42); err != nil {
		t.Fatal(err)
	}
	want, _ := Target(LeftThird, Rect{}, laptop.Visible)
	if w.frame.X != want.X || w.frame.Y != want.Y || w.frame.W != 600 {
		t.Fatalf("a window that can't shrink: %v, want at %v,%v", w.frame, want.X, want.Y)
	}
	if err := a.Arrange(BottomRight, 42); err != nil {
		t.Fatal(err)
	}
	want, _ = Target(BottomRight, Rect{}, laptop.Visible)
	if w.frame.X != want.X || w.frame.Y != want.Y {
		t.Fatalf("bottom right, can't shrink: %v, want its top left at %v,%v", w.frame, want.X, want.Y)
	}
}

func TestArrangeOnTwoScreens(t *testing.T) {
	a, w := newArranger(laptop, external)
	w.frame = Rect{1600, 0, 800, 600} // on the external display
	if err := a.Arrange(RightHalf, 42); err != nil {
		t.Fatal(err)
	}
	if want, _ := Target(RightHalf, Rect{}, external.Visible); w.frame != want {
		t.Fatalf("right half of the external display: %v, want %v", w.frame, want)
	}
	if err := a.Arrange(NextDisplay, 42); err != nil {
		t.Fatal(err)
	}
	if want, _ := Target(RightHalf, Rect{}, laptop.Visible); w.frame != want {
		t.Fatalf("to the next display, the laptop's right half: %v, want %v", w.frame, want)
	}
	if err := a.Arrange(PreviousDisplay, 42); err != nil {
		t.Fatal(err)
	}
	if ScreenOf(w.frame, a.W.Screens()) != 1 {
		t.Fatalf("back on the external display: %v", w.frame)
	}
	if err := a.Arrange(Restore, 42); err != nil || w.frame != (Rect{1600, 0, 800, 600}) {
		t.Fatalf("restored to %v (%v)", w.frame, err)
	}
}

func TestArrangeFailures(t *testing.T) {
	a, _ := newArranger(laptop)
	if err := a.Arrange(NextDisplay, 42); !errors.Is(err, ErrOneScreen) {
		t.Errorf("next display with one: %v", err)
	}
	if err := a.Arrange(LeftHalf, 7); !errors.Is(err, ErrNoWindow) {
		t.Errorf("an app with no window: %v", err)
	}
	if err := a.Arrange(LeftHalf, 0); !errors.Is(err, ErrNoWindow) {
		t.Errorf("no app: %v", err)
	}
	a.W.(*fakeWindows).err = ErrFullScreen
	if err := a.Arrange(LeftHalf, 42); !errors.Is(err, ErrFullScreen) {
		t.Errorf("full screen: %v", err)
	}
	a.W.(*fakeWindows).trusted = false
	if err := a.Arrange(LeftHalf, 42); !errors.Is(err, ErrNotTrusted) {
		t.Errorf("not trusted: %v", err)
	}
}

func TestRestoreRemembersAtMost(t *testing.T) {
	f := &fakeWindows{trusted: true, screens: []Screen{laptop}, wins: map[int]*fakeWindow{}}
	a := &Arranger{W: f}
	for pid := 1; pid <= most+10; pid++ {
		f.wins[pid] = &fakeWindow{key: fmt.Sprint(pid), frame: Rect{10, 40, 300, 300}}
		if err := a.Arrange(Center, pid); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.moved) > most {
		t.Fatalf("remembers %d windows, at most %d", len(a.moved), most)
	}
}

func TestDryRun(t *testing.T) {
	var said []string
	d := DryRun(func(f string, a ...any) { said = append(said, fmt.Sprintf(f, a...)) })
	if err := d.Restart(); err != nil {
		t.Fatal(err)
	}
	if empty, err := d.TrashEmpty(); empty || err != nil {
		t.Errorf("TrashEmpty: %v, %v", empty, err)
	}
	if len(said) != 1 || said[0] != "sysact (dry run): restart" {
		t.Errorf("said %q", said)
	}
}
