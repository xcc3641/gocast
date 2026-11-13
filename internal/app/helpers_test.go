package app

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"gocast/internal/running"
	"gocast/internal/termhost"
)

// fakeAppPaths are the apps the tests have installed, by bundle ID, as
// Launch Services would find them (spelled any way): those of the
// tests' search panels. Any other is not installed.
var fakeAppPaths = map[string]string{
	"com.apple.terminal": "/System/Applications/Utilities/Terminal.app",
	"com.apple.safari":   "/Applications/Safari.app",
	"com.apple.finder":   "/System/Library/CoreServices/Finder.app",
	"com.apple.notes":    "/System/Applications/Notes.app",
}

// TestMain stands in for the running apps: no test asks the system of
// them or acts on them (a CI runner with no login session crashes, and
// a test must not open, hide or quit the user's apps). None runs, none
// is in front.
func TestMain(m *testing.M) {
	sysIsRunning = func(string) bool { return false }
	sysAppPath = func(id string) string { return fakeAppPaths[strings.ToLower(id)] }
	sysFrontmost = func() (string, bool) { return "", false }
	sysHide = func(string) bool { return false }
	sysOpen = func(id string) error {
		if sysAppPath(id) == "" {
			return running.ErrNotInstalled
		}
		return nil
	}
	sysQuit = func(string) bool { return false }
	// Nor of MyGo's updater: no test checks, downloads or relaunches
	// (useFakeUpdater stands in for it).
	sysUpdatesEnabled = func() bool { return false }
	sysDevBuild = func() bool { return true }
	sysCheckUpdate = func(context.Context) (*foundUpdate, error) { return nil, errors.New("no checks in tests") }
	sysRelaunch = func() { panic("relaunched in a test") }
	// Nor of the system's displays and icons, which need the main loop.
	sysLocale = func() string { return "en" }
	sysMaxScale = func() float64 { return 2 }
	sysFileIcon = func(string, int) ([]byte, error) { return nil, errors.New("no icons in tests") }
	sysTypeIcon = sysFileIcon
	// Nor of the terminal host: no test starts one, or connects to the
	// user's; the terminals run in Gocast. Nor asks before closing them.
	sysTermHost = func(string, bool) (*termhost.Client, error) { return nil, errors.New("no terminal host in tests") }
	sysConfirmCloseAll = func() bool { return false }
	os.Exit(m.Run())
}

// fakeShortcuts stands for MyGo's global shortcuts: it refuses the
// shortcuts of refuse, as the system does those another app has.
type fakeShortcuts struct {
	on     map[string]bool
	refuse map[string]bool
}

func (f *fakeShortcuts) Register(accel string, fn func()) error {
	if f.refuse[accel] || f.on[accel] {
		return errShortcutTaken
	}
	f.on[accel] = true
	return nil
}

func (f *fakeShortcuts) Unregister(accel string) { delete(f.on, accel) }

// useFakeShortcuts puts a fakeShortcuts in place of the global shortcuts
// for the test.
func useFakeShortcuts(t *testing.T) *fakeShortcuts {
	f := &fakeShortcuts{on: map[string]bool{}, refuse: map[string]bool{}}
	old := globalShortcuts
	globalShortcuts = f
	t.Cleanup(func() { globalShortcuts = old })
	return f
}

// fakeUpdater stands for MyGo's updater, and for the update service's
// clock, timers and main thread: a check finds found ("" for none) or
// fails with checkErr, an install fails with installErr; what the
// checks and installs post waits in posted until next runs it, and the
// timers fire as advance moves the clock.
type fakeUpdater struct {
	t      *testing.T
	u      *updateService
	now    time.Time
	timers []*fakeTimer
	posted chan func()

	found                string
	checkErr, installErr error
	dev                  bool // a development build, once Check says updates are off

	checks     int      // the checks begun
	installs   []string // the versions installed, or begun
	relaunches int
}

// fakeTimer is a timer of the fake clock: f runs at at, unless stopped.
type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

// updateNow is when the tests' Gocast launched.
var updateNow = time.Date(2026, 11, 13, 9, 0, 0, 0, time.Local)

// useFakeUpdater gives g an update service over a fakeUpdater, launched
// at updateNow, its settings in a temporary directory; it does not start
// it (start asks the system of sleep and quits): schedule does.
func useFakeUpdater(t *testing.T, g *gocast) *fakeUpdater {
	t.Helper()
	f := &fakeUpdater{t: t, now: updateNow, posted: make(chan func(), 16)}
	old := []any{sysUpdatesEnabled, sysDevBuild, sysCheckUpdate, sysRelaunch}
	t.Cleanup(func() {
		sysUpdatesEnabled = old[0].(func() bool)
		sysDevBuild = old[1].(func() bool)
		sysCheckUpdate = old[2].(func(context.Context) (*foundUpdate, error))
		sysRelaunch = old[3].(func())
	})
	sysUpdatesEnabled = func() bool { return true }
	sysDevBuild = func() bool { return f.dev }
	sysCheckUpdate = func(context.Context) (*foundUpdate, error) {
		f.checks++
		if f.checkErr != nil || f.found == "" {
			return nil, f.checkErr
		}
		v, err := f.found, f.installErr
		return &foundUpdate{version: v, install: func(context.Context) error {
			f.installs = append(f.installs, v)
			return err
		}}, nil
	}
	sysRelaunch = func() { f.relaunches++ }
	if g.dir == "" {
		g.dir = t.TempDir()
	}
	f.u = newUpdateService(g, updateNow)
	f.u.post = func(fn func()) { f.posted <- fn }
	f.u.after = func(d time.Duration, fn func()) func() {
		tm := &fakeTimer{at: f.now.Add(d), f: fn}
		f.timers = append(f.timers, tm)
		return func() { tm.stopped = true }
	}
	f.u.now = func() time.Time { return f.now }
	g.updates = f.u
	return f
}

// next runs what a check or an install posted next, failing if none
// comes.
func (f *fakeUpdater) next() {
	f.t.Helper()
	select {
	case fn := <-f.posted:
		fn()
	case <-time.After(5 * time.Second):
		f.t.Fatal("nothing posted")
	}
}

// settle runs what the check under way posts until it ended, downloaded
// or not.
func (f *fakeUpdater) settle() {
	f.t.Helper()
	for f.u.phase != updateIdle {
		f.next()
	}
}

// advance moves the clock by d, firing the timers due as it does.
func (f *fakeUpdater) advance(d time.Duration) {
	end := f.now.Add(d)
	for {
		var due *fakeTimer
		for _, tm := range f.timers {
			if !tm.stopped && !tm.at.After(end) && (due == nil || tm.at.Before(due.at)) {
				due = tm
			}
		}
		if due == nil {
			break
		}
		due.stopped = true
		f.now = due.at
		due.f()
	}
	f.now = end
	f.timers = slices.DeleteFunc(f.timers, func(tm *fakeTimer) bool { return tm.stopped })
}

// nextTimer is when the next timer fires, or the zero time.
func (f *fakeUpdater) nextTimer() time.Time {
	var at time.Time
	for _, tm := range f.timers {
		if !tm.stopped && (at.IsZero() || tm.at.Before(at)) {
			at = tm.at
		}
	}
	return at
}

// ready makes version ready, as a check that found and installed it.
func (f *fakeUpdater) ready(version string) {
	f.t.Helper()
	f.found = version
	f.u.check(false)
	f.settle()
	if f.u.ready != version {
		f.t.Fatalf("not ready: %q", f.u.ready)
	}
}
