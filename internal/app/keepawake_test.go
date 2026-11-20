package app

import (
	"errors"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gocast/internal/keepawake"
	"gocast/internal/settings"
)

// fakeAssertions stands for the system's power assertions: it holds
// them by number, refusing them all while refuse is set.
type fakeAssertions struct {
	held   map[keepawake.Assertion]keepawake.AssertionType
	next   keepawake.Assertion
	refuse bool
}

func (f *fakeAssertions) Create(t keepawake.AssertionType, _ string) (keepawake.Assertion, error) {
	if f.refuse {
		return 0, errors.New("refused")
	}
	f.next++
	f.held[f.next] = t
	return f.next, nil
}

func (f *fakeAssertions) Release(a keepawake.Assertion) error {
	delete(f.held, a)
	return nil
}

// types are the types of the assertions held.
func (f *fakeAssertions) types() []keepawake.AssertionType {
	var out []keepawake.AssertionType
	for _, t := range f.held {
		out = append(out, t)
	}
	return out
}

// awakeNow is when the tests' Keep Awake starts: 15:00.
var awakeNow = time.Date(2026, 11, 20, 15, 0, 0, 0, time.Local)

// awakeFakes are a Keep Awake panel's clock and assertions.
type awakeFakes struct {
	now *time.Time
	as  *fakeAssertions
}

// advance moves the clock by d and has the Keeper check it, as it does
// every few seconds.
func (f awakeFakes) advance(g *gocast, d time.Duration) {
	*f.now = f.now.Add(d)
	g.awake.Check()
}

// awakePanel is the search panel with Keep Awake, over fake assertions,
// at awakeNow on a 24-hour clock; its changes shown at once, its own
// goroutine never checking.
func awakePanel(t *testing.T, locale string) (*launcher, *ui.Tester, awakeFakes) {
	t.Helper()
	l, tt, _ := actionPanel(t, locale)
	oldPost, oldClock := sysAwakePost, sysClock
	t.Cleanup(func() { sysAwakePost, sysClock = oldPost, oldClock })
	sysAwakePost = func(_ *gocast, fn func()) { fn() }
	sysClock = func(t time.Time, _ string) string { return t.Format("15:04") }
	now := awakeNow
	f := awakeFakes{now: &now, as: &fakeAssertions{held: map[keepawake.Assertion]keepawake.AssertionType{}}}
	l.g.awakeNow = func() time.Time { return *f.now }
	l.g.newAwake(keepawake.Options{Backend: f.as, After: func(time.Duration) <-chan time.Time { return nil }})
	t.Cleanup(l.g.awake.Close)
	l.rebuild()
	return l, tt, f
}

// awakeTitles are the titles of Keep Awake's rows, in order, the chosen
// one marked.
func awakeTitles(l *launcher) []string {
	var out []string
	for i, r := range l.rows {
		if !l.isAwake(r) {
			continue
		}
		s := l.items[r.item].Title
		if i == l.chosen {
			s = "> " + s
		}
		out = append(out, s)
	}
	return out
}

// chooseTitle chooses the row titled title.
func chooseTitle(t *testing.T, l *launcher, title string) {
	t.Helper()
	for i, r := range l.rows {
		if r.item >= 0 && r.title == "" && !r.calc && r.own < 0 && l.items[r.item].Title == title {
			l.choose(i)
			return
		}
	}
	t.Fatalf("no %q in %q", title, awakeTitles(l))
}

// Off, Keep Awake and its presets show in copy.md 18.1's order, however
// often each was used; a preset starts one assertion, the panel staying
// with Stop Keeping Awake in Keep Awake's place, saying what is left, the
// choice on the preset; a whole minute later the row says so; Stop
// releases the assertion and Keep Awake is back, chosen still.
func TestAwakeRowsStartAndStop(t *testing.T) {
	l, tt, f := awakePanel(t, "en")
	for range 3 {
		l.g.usage.Record(awakePrefix+"240", "keep awake", awakeNow)
	}
	l.rebuild()
	typeQuery(l, "keep awake")
	want := []string{"> Keep Awake", "Keep Awake for 30 Minutes", "Keep Awake for 1 Hour", "Keep Awake for 2 Hours", "Keep Awake for 4 Hours"}
	if got := awakeTitles(l); !slices.Equal(got, want) {
		t.Fatalf("off: %q", got)
	}
	chooseTitle(t, l, "Keep Awake for 1 Hour")
	l.primary()
	if !l.shown || l.notice != "Keeping awake, 1 hr left" || l.noticeAct.icon != "cup.and.saucer.fill" {
		t.Errorf("notice %q, shown %v", l.notice, l.shown)
	}
	if got := f.as.types(); !slices.Equal(got, []keepawake.AssertionType{keepawake.PreventDisplaySleep}) {
		t.Errorf("held %q", got)
	}
	want = []string{"Stop Keeping Awake", "Keep Awake for 30 Minutes", "> Keep Awake for 1 Hour", "Keep Awake for 2 Hours", "Keep Awake for 4 Hours"}
	if got := awakeTitles(l); !slices.Equal(got, want) {
		t.Fatalf("on: %q", got)
	}
	chooseTitle(t, l, "Stop Keeping Awake")
	tt.Frame()
	if !has(tt, "1 hr left") || !has(tt, "Stop") || l.rowLabel(l.chosen) != "Stop Keeping Awake, System, 1 hr left" {
		t.Errorf("Stop: %q, %q", tt.Texts(), l.rowLabel(l.chosen))
	}
	f.advance(l.g, time.Minute+time.Second)
	tt.Frame()
	if !has(tt, "59 min left") {
		t.Errorf("a minute on: %q", tt.Texts())
	}

	l.primary()
	if len(f.as.held) != 0 || l.notice != "Stopped keeping awake" || l.noticeAct.icon != "cup.and.saucer" {
		t.Errorf("stopped: held %v, notice %q", f.as.held, l.notice)
	}
	if got := awakeTitles(l); got[0] != "> Keep Awake" {
		t.Errorf("after stopping: %q", got)
	}
	l.primary() // Keep Awake: Stop Keeping Awake in its place, chosen
	if got := awakeTitles(l); got[0] != "> Stop Keeping Awake" {
		t.Errorf("after starting: %q", got)
	}
}

// A length or time typed is its own row, first, the names' rows gone;
// starting one while on replaces the session, one assertion held; a
// time past today's is tomorrow's; and an invalid length starts nothing,
// leaving the session as it was.
func TestAwakeTyped(t *testing.T) {
	l, _, f := awakePanel(t, "en")
	typeQuery(l, "keep awake 1h30m")
	if got := awakeTitles(l); !slices.Equal(got, []string{"> Keep Awake for 1 Hour 30 Minutes"}) {
		t.Fatalf("typed: %q", got)
	}
	l.primary()
	if l.notice != "Keeping awake, 1 hr 30 min left" {
		t.Errorf("notice %q", l.notice)
	}
	typeQuery(l, "coffee until 9am")
	if got := awakeTitles(l); !slices.Equal(got, []string{"> Keep Awake Until Tomorrow 09:00", "Stop Keeping Awake"}) {
		t.Fatalf("until, on: %q", got)
	}
	l.primary()
	st := l.g.awake.State()
	if len(f.as.held) != 1 || st.Kind != keepawake.Until || l.notice != "Keeping awake until tomorrow 09:00" {
		t.Errorf("replaced: held %d, %+v, %q", len(f.as.held), st, l.notice)
	}

	chooseTitle(t, l, "Stop Keeping Awake") // the last row goes: the one over it chosen
	l.primary()
	if got := awakeTitles(l); !slices.Equal(got, []string{"> Keep Awake Until Tomorrow 09:00"}) {
		t.Fatalf("stopped from the typed rows: %q", got)
	}
	l.primary()

	typeQuery(l, "keep awake 100h") // over 99 hours: Keep Awake itself
	if got := awakeTitles(l); !slices.Equal(got, []string{"> Stop Keeping Awake"}) {
		t.Errorf("invalid: %q", got)
	}
	if err := l.g.awake.Start(keepawake.Request{Kind: keepawake.For, Duration: 100 * time.Hour}); err == nil || l.g.awake.State() != st {
		t.Errorf("invalid started: %v", err)
	}
}

// Only "keep awake until": a dimmed row with the hint, nothing to act on,
// Return doing nothing.
func TestAwakeNeedsTime(t *testing.T) {
	l, tt, f := awakePanel(t, "en")
	typeQuery(l, "keep awake until")
	if got := awakeTitles(l); !slices.Equal(got, []string{"> Keep Awake Until"}) {
		t.Fatalf("rows %q", got)
	}
	tt.Frame()
	if !has(tt, "Type a time, like 17:30 or 5pm") || has(tt, "Start") {
		t.Errorf("drawn %q", tt.Texts())
	}
	l.primary()
	if len(f.as.held) != 0 || l.notice != "" {
		t.Errorf("acted: %v %q", f.as.held, l.notice)
	}
}

// Keep Awake's name and what it can't read shows Keep Awake alone; its
// aliases and Chinese names find it, the presets with it.
func TestAwakeFound(t *testing.T) {
	l, _, _ := awakePanel(t, "zh-Hans")
	typeQuery(l, "keep awake soon")
	if got := awakeTitles(l); !slices.Equal(got, []string{"> 保持唤醒"}) {
		t.Errorf("unread: %q", got)
	}
	for _, q := range []string{"咖啡", "caffeinate", "bxm"} {
		typeQuery(l, q)
		if got := awakeTitles(l); len(got) != 5 || !strings.HasSuffix(got[0], "保持唤醒") {
			t.Errorf("%s: %q", q, got)
		}
	}
}

// Ending on its own, the session says nothing and the panel's rows turn
// back; the tray's lines follow, Keep Awake › under the clipboard
// history while off, the status, Stop and Change To › over all while on.
func TestAwakeEndsAndTray(t *testing.T) {
	l, _, f := awakePanel(t, "en")
	g := l.g
	labels := trayLabels(g)
	if i := slices.Index(labels, "Keep Awake"); i < 0 || labels[i-1] != "Clipboard History" {
		t.Fatalf("off: %q", labels)
	}
	var sub []string
	for _, it := range g.trayMenu().Items() {
		if it.Label == "Keep Awake" {
			for _, s := range it.Submenu {
				sub = append(sub, s.Label)
			}
			it.Submenu[1].Click(it.Submenu[1], nil) // 30 Minutes
		}
	}
	if !slices.Equal(sub, []string{"Until I Stop", "30 Minutes", "1 Hour", "2 Hours", "4 Hours"}) {
		t.Errorf("submenu %q", sub)
	}
	labels = trayLabels(g)
	if !slices.Equal(labels[:4], []string{"Keeping Awake · 30 min left", "Stop Keeping Awake", "Change To", "-"}) || slices.Contains(labels, "Keep Awake") {
		t.Errorf("on: %q", labels)
	}
	if it := g.trayMenu().Items()[0]; !it.Disabled {
		t.Error("status line clickable")
	}

	typeQuery(l, "awake")
	f.advance(g, 30*time.Minute)
	if len(f.as.held) != 0 || l.notice != "" || awakeTitles(l)[0] != "> Keep Awake" || trayLabels(g)[0] != "Search…" {
		t.Errorf("ended: held %v, notice %q, rows %q, tray %q", f.as.held, l.notice, awakeTitles(l), trayLabels(g))
	}
}

// The tray's tooltip says "keeping awake" from the start, through Change
// To, and is Gocast's again at once as the session stops, ends on time
// (checked as the Mac wakes too) or Gocast quits.
func TestAwakeTrayTooltip(t *testing.T) {
	l, _, f := awakePanel(t, "en")
	g := l.g
	var tips []string
	old := sysTrayLook
	t.Cleanup(func() { sysTrayLook = old })
	sysTrayLook = func(_ *gocast, _ []byte, tip string) { tips = append(tips, tip) }
	last := func() string {
		if len(tips) == 0 {
			return ""
		}
		return tips[len(tips)-1]
	}
	change := func(label string) {
		for _, it := range g.trayMenu().Items() {
			if it.Label == "Change To" || it.Label == "Keep Awake" {
				for _, s := range it.Submenu {
					if s.Label == label {
						s.Click(s, nil)
						return
					}
				}
			}
		}
		t.Fatalf("no %q in the tray", label)
	}

	change("Until I Stop")
	change("30 Minutes") // Change To: still on
	if last() != "Gocast, keeping awake" {
		t.Fatalf("on: %q", tips)
	}
	g.stopAwake()
	if last() != appName {
		t.Errorf("stopped: %q", tips)
	}
	change("30 Minutes")
	*f.now = f.now.Add(31 * time.Minute) // asleep past its end
	g.awake.Check()                      // as the Mac wakes
	if last() != appName || g.awakeActive() {
		t.Errorf("ended: %q", tips)
	}
	change("1 Hour")
	g.closeAwake()
	if last() != appName || len(f.as.held) != 0 {
		t.Errorf("quit: %q, held %v", tips, f.as.held)
	}
	if want := []string{"Gocast, keeping awake", appName, "Gocast, keeping awake", appName, "Gocast, keeping awake", appName}; !slices.Equal(tips, want) {
		t.Errorf("tips %q", tips)
	}
}

// The switch in Settings › General applies at once to the session
// running, and is kept; when the new assertion can't be made, nothing
// changes.
func TestAwakeScreenOffSwitch(t *testing.T) {
	l, _, f := awakePanel(t, "en")
	g := l.g
	g.keepAwake(keepawake.Request{Kind: keepawake.Forever})
	if !g.setAwakeScreenOff(true) || !g.settings.AwakeScreenOff {
		t.Fatal("not switched")
	}
	if got := f.as.types(); !slices.Equal(got, []keepawake.AssertionType{keepawake.PreventSystemSleep}) {
		t.Errorf("held %q", got)
	}
	f.as.refuse = true
	if g.setAwakeScreenOff(false) || !g.settings.AwakeScreenOff {
		t.Error("switched without an assertion")
	}
}

// --- Screenshots ---

func TestScreensAwake(t *testing.T) {
	dir := screensDir(t)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		for _, loc := range []string{"zh-Hans", "en"} {
			shot := func(name string, setup func(l *launcher, f awakeFakes)) {
				l, tt, f := awakePanel(t, loc)
				tt.SetScale(2)
				tt.SetDark(dark)
				setup(l, f)
				saveScreen(t, tt, dir, "awake-"+name+"-"+loc+"-"+mode, panelRect)
			}
			q := "keep awake"
			if loc == "zh-Hans" {
				q = "保持唤醒"
			}
			shot("off", func(l *launcher, f awakeFakes) { typeQuery(l, q) })
			shot("started", func(l *launcher, f awakeFakes) {
				typeQuery(l, q)
				chooseTitle(t, l, l.items[l.itemAt[awakePrefix+"60"]].Title)
				l.primary()
			})
			shot("on", func(l *launcher, f awakeFakes) {
				l.g.keepAwake(keepawake.Request{Kind: keepawake.For, Duration: 83 * time.Minute})
				typeQuery(l, q)
			})
			shot("typed", func(l *launcher, f awakeFakes) { typeQuery(l, q+" 1h30m") })
			shot("until", func(l *launcher, f awakeFakes) { typeQuery(l, q+" until 9am") })
			shot("need-time", func(l *launcher, f awakeFakes) { typeQuery(l, q+" until") })
			shot("stopped", func(l *launcher, f awakeFakes) {
				l.g.keepAwake(keepawake.Request{Kind: keepawake.Forever})
				typeQuery(l, q)
				l.primary()
			})
		}
		for _, loc := range []string{"zh-Hans", "en"} {
			p, tt := positionPrefs(t, loc, settings.PositionTop)
			tt.SetSize(prefsWidth, prefsHeight)
			p.goTo(pageGeneral)
			tt.SetDark(dark)
			saveScreen(t, tt, dir, "awake-settings-"+loc+"-"+mode, image.Rect(0, 0, prefsWidth, prefsHeight))
		}
	}
}
