package sysact

import (
	"errors"
	"sync"
)

// Windows is what the window actions ask of macOS, through the
// Accessibility API: the app holds the real one (NewWindows), its tests
// a fake with windows and screens of their own, so that no test moves
// the user's windows.
type Windows interface {
	// Trusted reports whether Gocast may control other apps' windows
	// (Privacy & Security › Accessibility).
	Trusted() bool
	// Focused is the focused window of the app of the process pid: its
	// main window, else the first standard one. ErrNoWindow when it has
	// none to arrange (none, or minimized), ErrFullScreen when it is
	// full screen.
	Focused(pid int) (Window, error)
	// Screens are the displays, their frames and visible frames.
	Screens() []Screen
}

// Window is a window of another app, as the Accessibility API has it.
type Window interface {
	// Key tells the window from others while it is open, for Restore to
	// find its frame: the same for the same window, found again.
	Key() string
	Frame() (Rect, error)
	// SetFrame resizes and moves it: its size, then where, then its
	// size again, as an app may refuse a size that would not fit where
	// the window was. The app may keep it larger (a least size) or snap
	// its size, which Frame then tells.
	SetFrame(r Rect) error
	// Release lets go of it.
	Release()
}

// The window actions' failures, which the search panel says (copy.md
// 15.4).
var (
	ErrNotTrusted = errors.New("sysact: Gocast is not allowed to control windows")
	ErrNoWindow   = errors.New("sysact: no window to arrange")
	ErrFullScreen = errors.New("sysact: the window is full screen")
	ErrOneScreen  = errors.New("sysact: only one display")
	ErrNotMoved   = errors.New("sysact: the window has not been moved")
)

// Arranger runs the window actions, remembering for Restore where each
// window was before Gocast first moved it.
type Arranger struct {
	W Windows

	mu    sync.Mutex
	moved map[string]moved // by Window.Key
}

// moved is a window Gocast moved: where it was first, and where Gocast
// left it last, to tell a window the user moved since.
type moved struct{ from, last Rect }

// most is how many windows Restore remembers: the oldest go first.
const most = 64

// Arrange runs the window action id on the focused window of the process
// pid.
func (a *Arranger) Arrange(id string, pid int) error {
	if !a.W.Trusted() {
		return ErrNotTrusted
	}
	if pid <= 0 {
		return ErrNoWindow
	}
	win, err := a.W.Focused(pid)
	if err != nil {
		return err
	}
	defer win.Release()
	frame, err := win.Frame()
	if err != nil {
		return ErrNoWindow
	}
	key := win.Key()

	a.mu.Lock()
	rec, had := a.moved[key]
	a.mu.Unlock()
	// The user moved it since Gocast did: Restore goes back to where the
	// user left it, not to where it was before.
	if had && !near(rec.last, frame) {
		had = false
	}

	var to Rect
	switch id {
	case Restore:
		if !had {
			return ErrNotMoved
		}
		to = rec.from
	case NextDisplay, PreviousDisplay:
		screens := a.W.Screens()
		if len(screens) < 2 {
			return ErrOneScreen
		}
		step := 1
		if id == PreviousDisplay {
			step = -1
		}
		to = OtherScreen(frame, screens, ScreenOf(frame, screens), step)
	default:
		screens := a.W.Screens()
		i := ScreenOf(frame, screens)
		if i < 0 {
			return ErrNoWindow
		}
		var ok bool
		if to, ok = Target(id, frame, screens[i].Visible); !ok {
			return errors.New("sysact: not a window action: " + id)
		}
	}

	got, err := place(win, to)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if id == Restore {
		delete(a.moved, key)
		return nil
	}
	if a.moved == nil {
		a.moved = map[string]moved{}
	}
	from := frame
	if had {
		from = rec.from
	} else if len(a.moved) >= most {
		for k := range a.moved { // any one: windows long closed, mostly
			delete(a.moved, k)
			break
		}
	}
	a.moved[key] = moved{from: from, last: got}
	return nil
}

// place puts win at r. A window that can't be made that small is left
// at r's top left (copy.md 15.4: no notice); one the app moved elsewhere
// as it resized is moved back there. It returns where the window is
// then.
func place(win Window, r Rect) (Rect, error) {
	if err := win.SetFrame(r); err != nil {
		return Rect{}, err
	}
	got, err := win.Frame()
	if err != nil {
		return r, nil
	}
	if !near(Rect{X: got.X, Y: got.Y}, Rect{X: r.X, Y: r.Y}) {
		if err := win.SetFrame(Rect{r.X, r.Y, got.W, got.H}); err != nil {
			return Rect{}, err
		}
		if g, err := win.Frame(); err == nil {
			got = g
		}
	}
	return got, nil
}

// near reports whether a and b are the same frame but for a point of
// rounding.
func near(a, b Rect) bool {
	d := func(x, y float64) bool { return x-y <= 1 && y-x <= 1 }
	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.W, b.W) && d(a.H, b.H)
}
