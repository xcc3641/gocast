package app

import (
	"errors"
	"image"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
	"gocast/locales"
)

// The automatic check is due 10 seconds after launch, then a day after
// the last one, which a relaunch remembers (design.md 4.5).
func TestNextCheck(t *testing.T) {
	launched := updateNow
	for _, c := range []struct {
		name      string
		last, now time.Time
		want      time.Time
	}{
		{"never checked", time.Time{}, launched, launched.Add(10 * time.Second)},
		{"a day ago", launched.Add(-25 * time.Hour), launched, launched.Add(10 * time.Second)},
		{"an hour ago", launched.Add(-time.Hour), launched, launched.Add(23 * time.Hour)},
		{"as the app ran", launched.Add(time.Minute), launched.Add(time.Hour), launched.Add(24*time.Hour + time.Minute)},
		{"the clock set back", launched.Add(time.Hour), launched, launched.Add(10 * time.Second)},
	} {
		if got := nextCheck(c.last, launched, c.now); !got.Equal(c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// updateGocast is a Gocast with no window, its settings in a temporary
// directory, its update service a fake's.
func updateGocast(t *testing.T) (*gocast, *fakeUpdater) {
	t.Helper()
	inLocale(t, "en")
	g := &gocast{dir: t.TempDir()}
	return g, useFakeUpdater(t, g)
}

// savedSettings reads back what Gocast saved.
func savedSettings(t *testing.T, g *gocast) settings.Settings {
	t.Helper()
	s, err := settings.Load(g.settingsPath())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUpdateSchedule(t *testing.T) {
	g, f := updateGocast(t)
	u := f.u
	u.schedule()
	if want := updateNow.Add(firstCheckAfter); !f.nextTimer().Equal(want) {
		t.Fatalf("first check at %v, want %v", f.nextTimer(), want)
	}
	f.advance(9 * time.Second)
	if f.checks != 0 {
		t.Fatal("checked before 10 seconds")
	}
	f.advance(time.Second)
	f.settle()
	if f.checks != 1 || !savedSettings(t, g).UpdatesChecked.Equal(f.now) {
		t.Fatalf("10 seconds: %d checks, last %v", f.checks, savedSettings(t, g).UpdatesChecked)
	}
	// Nothing new: nothing said, the next check a day later.
	if u.about() != (aboutStatus{button: aboutCheck}) || u.ready != "" {
		t.Errorf("up to date, unasked: %+v", u.about())
	}
	if want := f.now.Add(checkEvery); !f.nextTimer().Equal(want) {
		t.Errorf("next check at %v, want %v", f.nextTimer(), want)
	}

	// A day later, a new version: downloaded and installed at once.
	f.found = "0.2.0"
	f.advance(checkEvery)
	if u.phase != updateChecking {
		t.Fatalf("a day later: phase %v", u.phase)
	}
	f.next()
	if f.checks != 2 || u.phase != updateDownloading || u.about().text != "Downloading 0.2.0…" {
		t.Errorf("downloading: %+v", u.about())
	}
	f.next()
	if u.ready != "0.2.0" || !slices.Equal(f.installs, []string{"0.2.0"}) || u.about().button != aboutRelaunch {
		t.Errorf("installed: %q, %q, %+v", u.ready, f.installs, u.about())
	}
	// It goes on checking: the same version again is not downloaded again,
	// a newer one is, and takes its place.
	f.advance(checkEvery)
	f.settle()
	if f.checks != 3 || len(f.installs) != 1 || u.ready != "0.2.0" {
		t.Errorf("same again: %d checks, %q", f.checks, f.installs)
	}
	f.found = "0.2.1"
	f.advance(checkEvery)
	f.settle()
	if u.ready != "0.2.1" || !slices.Equal(f.installs, []string{"0.2.0", "0.2.1"}) {
		t.Errorf("newer: %q, %q", u.ready, f.installs)
	}

	// Asleep, the timer counted nothing: waking a day after the last
	// check, it checks.
	checks := f.checks
	f.now = f.now.Add(25 * time.Hour)
	u.schedule()
	f.settle()
	if f.checks != checks+1 {
		t.Errorf("woken a day later: %d checks", f.checks-checks)
	}
	// Woken sooner, it waits for the day.
	f.now = f.now.Add(time.Hour)
	u.schedule()
	if f.checks != checks+1 || !f.nextTimer().Equal(f.now.Add(23*time.Hour)) {
		t.Errorf("woken an hour later: %d checks, next %v", f.checks-checks, f.nextTimer())
	}
}

// Relaunched (or launched again) within a day of the last check, Gocast
// waits for the day; failures are not said and wait for the next check.
func TestUpdateScheduleRemembers(t *testing.T) {
	g, f := updateGocast(t)
	g.settings.UpdatesChecked = updateNow.Add(-2 * time.Hour)
	f.checkErr = errors.New("offline")
	f.u.schedule()
	if want := updateNow.Add(22 * time.Hour); !f.nextTimer().Equal(want) {
		t.Fatalf("next check at %v, want %v", f.nextTimer(), want)
	}
	f.advance(22 * time.Hour)
	f.settle()
	if f.checks != 1 || f.u.about() != (aboutStatus{button: aboutCheck}) {
		t.Errorf("failed unasked: %d checks, %+v", f.checks, f.u.about())
	}
	f.checkErr, f.found, f.installErr = nil, "0.2.0", errors.New("offline")
	f.advance(checkEvery)
	f.settle()
	if f.checks != 2 || f.u.ready != "" || f.u.about() != (aboutStatus{button: aboutCheck}) {
		t.Errorf("download failed unasked: %q, %+v", f.u.ready, f.u.about())
	}
}

// Off, 自动检查更新 checks only as asked, and downloads what it finds;
// the version ready stays ready as it is switched (design.md 4.5).
func TestUpdateAutoOff(t *testing.T) {
	g, f := updateGocast(t)
	g.setAutoUpdates(false)
	if savedSettings(t, g).AutoUpdates() {
		t.Fatal("off: not saved")
	}
	f.u.schedule()
	f.advance(48 * time.Hour)
	if f.checks != 0 || len(f.timers) != 0 {
		t.Fatalf("off: %d checks, %d timers", f.checks, len(f.timers))
	}
	f.found = "0.2.0"
	f.u.check(true)
	f.settle()
	if f.checks != 1 || f.u.ready != "0.2.0" {
		t.Fatalf("asked: %d checks, ready %q", f.checks, f.u.ready)
	}
	if len(f.timers) != 0 {
		t.Error("asked, off: the next check armed")
	}

	// On: armed a day after the check asked for.
	g.setAutoUpdates(true)
	if !savedSettings(t, g).AutoUpdates() || !f.nextTimer().Equal(f.now.Add(checkEvery)) {
		t.Errorf("on: saved %v, next %v", savedSettings(t, g).AutoUpdates(), f.nextTimer())
	}
	g.setAutoUpdates(false)
	if f.nextTimer() != (time.Time{}) || f.u.readyVersion() != "0.2.0" {
		t.Errorf("off again: next %v, ready %q", f.nextTimer(), f.u.readyVersion())
	}
}

// A check asked for while one runs joins it: About says what it found.
// 已是最新版本 goes after 4 seconds; a failure stays until the next
// check begins.
func TestUpdateAsked(t *testing.T) {
	_, f := updateGocast(t)
	u := f.u
	u.schedule()
	f.advance(firstCheckAfter) // the background check, under way
	if u.about().text != "Checking…" || u.about().button != aboutCheckOff {
		t.Fatalf("checking: %+v", u.about())
	}
	u.check(true)
	f.settle()
	if f.checks != 1 {
		t.Fatalf("joined: %d checks", f.checks)
	}
	if st := u.about(); st.text != "You’re up to date" || st.icon != "checkmark.circle" || st.button != aboutCheck {
		t.Errorf("up to date: %+v", st)
	}
	f.advance(upToDateFor - time.Millisecond)
	if u.about().text == "" {
		t.Error("up to date: gone before 4 seconds")
	}
	f.advance(time.Millisecond)
	if u.about().text != "" {
		t.Errorf("up to date after 4 seconds: %+v", u.about())
	}

	f.checkErr = errors.New("offline")
	u.check(true)
	f.settle()
	if st := u.about(); st.text != "Couldn’t check for updates. Gocast will try again later." || st.icon != "exclamationmark.triangle" || st.button != aboutCheck {
		t.Errorf("failed: %+v", st)
	}
	f.advance(time.Minute)
	if u.about().text == "" {
		t.Error("failed: gone")
	}
	f.checkErr, f.found, f.installErr = nil, "0.2.0", errors.New("offline")
	u.check(true)
	if u.about().text != "Checking…" {
		t.Errorf("checking again: %+v", u.about())
	}
	f.settle()
	if st := u.about(); st.text != "Couldn’t finish downloading. Gocast will try again later." || u.ready != "" {
		t.Errorf("download failed: %+v", st)
	}
	f.installErr = nil
	u.check(true)
	f.settle()
	if st := u.about(); st != (aboutStatus{button: aboutRelaunch, text: "0.2.0 is ready", version: "0.2.0", say: true}) {
		t.Errorf("ready: %+v", st)
	}
}

// A build that can't update itself checks nothing, and About says why;
// so does one whose check says updates are off.
func TestUpdateUnavailable(t *testing.T) {
	_, f := updateGocast(t)
	f.u.enabled, f.u.dev = false, true
	f.u.schedule()
	f.u.check(true)
	f.advance(48 * time.Hour)
	if f.checks != 0 {
		t.Errorf("dev build: %d checks", f.checks)
	}
	if st := f.u.about(); st.text != "Development builds don’t update themselves." || st.button != aboutCheckOff {
		t.Errorf("dev build: %+v", st)
	}
	f.u.dev = false
	if st := f.u.about(); st.text != "Gocast can’t update itself here. If you installed it another way, update it there." || st.icon != "info.circle" {
		t.Errorf("can't: %+v", st)
	}

	_, f = updateGocast(t)
	f.checkErr = mygo.ErrUpdatesDisabled
	f.u.check(true)
	f.settle()
	if f.u.enabled || f.u.about().button != aboutCheckOff || len(f.timers) != 0 {
		t.Errorf("disabled as checked: %+v, %d timers", f.u.about(), len(f.timers))
	}
}

// Relaunch relaunches once; the quit canceled, it can again.
func TestUpdateRelaunch(t *testing.T) {
	_, f := updateGocast(t)
	f.u.relaunch()
	if f.relaunches != 0 {
		t.Fatal("relaunched with nothing ready")
	}
	f.ready("0.2.0")
	if v, relaunching := f.u.pill(); v != "0.2.0" || relaunching {
		t.Errorf("pill: %q, %v", v, relaunching)
	}
	f.u.relaunch()
	f.u.relaunch()
	if v, relaunching := f.u.pill(); f.relaunches != 1 || v != "0.2.0" || !relaunching {
		t.Errorf("relaunching: %d, %q, %v", f.relaunches, v, relaunching)
	}
	f.u.quitCanceled()
	f.u.relaunch()
	if f.relaunches != 2 {
		t.Errorf("after a quit canceled: %d", f.relaunches)
	}
}

// The tray's line: 检查更新…, then once a version is ready, 重新打开以
// 更新到 {版本}, which relaunches (design.md 4.5, 5).
func TestUpdateTrayItem(t *testing.T) {
	g, f := updateGocast(t)
	if got := g.updateMenuItem().Label; got != "Check for Updates…" {
		t.Errorf("idle: %q", got)
	}
	f.ready("0.2.0")
	it := g.updateMenuItem()
	if it.Label != "Relaunch to Update to 0.2.0" || it.Accelerator != "" {
		t.Fatalf("ready: %+v", it)
	}
	it.Click(it, nil)
	if f.relaunches != 1 {
		t.Errorf("clicked: %d relaunches", f.relaunches)
	}
	g.updates = nil // no update service: as before it
	if got := g.updateMenuItem().Label; got != "Check for Updates…" {
		t.Errorf("none: %q", got)
	}
}

// updatePanel is hotkeyPanel's search panel with a version ready, shown
// again for its Gocast group to have it.
func updatePanel(t *testing.T, locale string) (*launcher, *ui.Tester, *fakeUpdater) {
	t.Helper()
	l, tt, _ := hotkeyPanel(t, locale)
	f := useFakeUpdater(t, l.g)
	f.ready("0.2.0")
	l.enterSearch(clipNow.Add(time.Hour))
	tt.Frame()
	return l, tt, f
}

// The Gocast group has Relaunch to Update first, found by its words in
// both languages; the suggestions keep the choice, and it counts for
// none (design.md 4.5).
func TestUpdateRow(t *testing.T) {
	l, _, _ := hotkeyPanel(t, "en")
	useFakeUpdater(t, l.g)
	if slices.Contains(shown(l), "group:"+updateItemID) {
		t.Fatal("a row with nothing ready")
	}
	l, tt, f := updatePanel(t, "en")
	rows := shown(l)
	i := slices.Index(rows, "group:"+updateItemID)
	if i < 0 || rows[i-1] != "title:Gocast" || rows[0] != "title:Suggestions" || !slices.Contains(rows, "> app:com.apple.Terminal") {
		t.Fatalf("rows: %q", rows)
	}
	if !has(tt, "Relaunch to Update to 0.2.0") {
		t.Errorf("texts: %q", tt.Texts())
	}
	for _, q := range []string{"update", "relaunch", "更新", "重新打开", "0.2.0"} {
		typeQuery(l, q)
		if !slices.Contains(shown(l), updateItemID) && !slices.Contains(shown(l), "> "+updateItemID) {
			t.Errorf("%s: %q", q, shown(l))
		}
	}
	typeQuery(l, "relaunch")
	chooseRow(t, l, updateItemID)
	if got := menuLabels(l); !slices.Equal(got, []string{"Relaunch"}) {
		t.Errorf("menu: %q", got)
	}
	l.primary()
	if f.relaunches != 1 || !l.shown || l.g.usage.Score(updateItemID, clipNow) != 0 {
		t.Errorf("relaunched: %d, shown %v", f.relaunches, l.shown)
	}
	tt.Frame()
	if !has(tt, "Relaunching…") {
		t.Errorf("pill: %q", tt.Texts())
	}
}

// The pill: the version ready, Relaunch and ×, read once; Relaunch
// relaunches, × closes it for this run; a notice covers it while it
// shows (design.md 4.5).
func TestUpdatePill(t *testing.T) {
	l, tt, f := updatePanel(t, "en")
	if !has(tt, "Version 0.2.0 is ready") || !has(tt, "Relaunch") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	if b, ok := tt.AccessByRole(ui.RoleButton, "Version 0.2.0 is ready, Relaunch"); !ok || b.Bounds.X > float32(searchLeft+panelWidth/2) {
		t.Errorf("button: %+v, %v", b, ok)
	}

	l.notice = "Copied"
	tt.Frame()
	tt.Advance(time.Second)
	if has(tt, "Version 0.2.0 is ready") || !has(tt, "Copied") {
		t.Errorf("notice: %q", tt.Texts())
	}
	l.notice = ""
	tt.Frame()
	tt.Advance(time.Second)
	if !has(tt, "Version 0.2.0 is ready") {
		t.Errorf("notice gone: %q", tt.Texts())
	}

	if err := tt.Click("Relaunch"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	if f.relaunches != 1 || !has(tt, "Relaunching…") || has(tt, "Version 0.2.0 is ready") || has(tt, "Dismiss") {
		t.Errorf("relaunching: %d, %q", f.relaunches, tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleStatus, "Relaunching…"); !ok {
		t.Error("relaunching: no status")
	}

	l, tt, f = updatePanel(t, "en")
	b, ok := tt.AccessByRole(ui.RoleButton, "Dismiss")
	if !ok {
		t.Fatal("no ×")
	}
	tt.ClickAt(b.Bounds.X+b.Bounds.W/2, b.Bounds.Y+b.Bounds.H/2)
	tt.Frame()
	tt.Advance(time.Second)
	if has(tt, "Version 0.2.0 is ready") || f.relaunches != 0 {
		t.Errorf("closed: %q", tt.Texts())
	}
	l.enterSearch(clipNow.Add(2 * time.Hour)) // shown again
	tt.Frame()
	if has(tt, "Version 0.2.0 is ready") || !slices.Contains(shown(l), "group:"+updateItemID) {
		t.Errorf("closed, shown again: %q", tt.Texts())
	}
}

// The pill is read out the first time it shows in a run, not each time.
func TestUpdatePillAnnounced(t *testing.T) {
	l, tt, _ := hotkeyPanel(t, "zh-Hans")
	tt.UseScreenReader()
	f := useFakeUpdater(t, l.g)
	f.ready("0.2.0")
	l.enterSearch(clipNow.Add(time.Hour))
	tt.Frame()
	l.enterSearch(clipNow.Add(2 * time.Hour))
	tt.Frame()
	n := 0
	for _, a := range tt.Announcements() {
		if a == "新版本 0.2.0 已就绪，重新打开" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("read %d times: %q", n, tt.Announcements())
	}
}

// updatePrefs is Settings on page, over updatePanel's Gocast without a
// version ready.
func updatePrefs(t *testing.T, locale, page string) (*prefsWindow, *ui.Tester, *fakeUpdater) {
	t.Helper()
	p, tt, _ := hotkeyPrefs(t, locale)
	f := useFakeUpdater(t, p.g)
	// General's terminal sizes: of a work area as a shown panel's, not the
	// screen's.
	p.g.term = &termPanel{g: p.g, shown: true, wa: mygo.Rectangle{Width: 1512, Height: 944}}
	p.page = page
	p.autoUpdate = p.g.settings.AutoUpdates()
	tt.Frame()
	return p, tt, f
}

// About's button and status line: nothing above them moves as the line
// changes; 看看改了什么 opens the changes of the version (design.md 4.5).
func TestAboutStatus(t *testing.T) {
	_, tt, f := updatePrefs(t, "en", pageAbout)
	frame := func() {
		tt.Frame()
		tt.Advance(time.Second)
	}
	at, ok := tt.Find("Check for Updates")
	if !ok {
		t.Fatalf("texts: %q", tt.Texts())
	}
	version, _ := tt.Find(locales.AboutVersion(appVersion()))
	if b, _ := tt.AccessByRole(ui.RoleButton, "Check for Updates"); b.Disabled {
		t.Error("idle: disabled")
	}

	f.found = "0.2.0"
	if err := tt.Click("Check for Updates"); err != nil {
		t.Fatal(err)
	}
	frame()
	if !has(tt, "Checking…") {
		t.Errorf("checking: %q", tt.Texts())
	}
	if b, _ := tt.AccessByRole(ui.RoleButton, "Check for Updates"); !b.Disabled {
		t.Error("checking: enabled")
	}
	f.next()
	frame()
	if !has(tt, "Downloading 0.2.0…") || !has(tt, "What’s New") {
		t.Errorf("downloading: %q", tt.Texts())
	}
	if err := tt.Click("What’s New"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); !slices.Equal(got, []string{"https://gocast.yichenlab.com/changelog/#v0.2.0"}) {
		t.Errorf("opened %q", got)
	}
	f.next()
	frame()
	if !has(tt, "0.2.0 is ready") || has(tt, "Check for Updates") {
		t.Errorf("ready: %q", tt.Texts())
	}
	r, ok := tt.Find("Relaunch")
	if v, _ := tt.Find(locales.AboutVersion(appVersion())); !ok || v != version || r.Y+r.H/2 != at.Y+at.H/2 {
		t.Errorf("moved: version %+v, was %+v; button %+v, was %+v", v, version, r, at)
	}
	if err := tt.Click("Relaunch"); err != nil {
		t.Fatal(err)
	}
	if f.relaunches != 1 {
		t.Errorf("relaunched %d", f.relaunches)
	}
}

// The status line says what it says once, as it comes.
func TestAboutStatusAnnounced(t *testing.T) {
	_, tt, f := updatePrefs(t, "zh-Hans", pageAbout)
	tt.UseScreenReader()
	f.checkErr = errors.New("offline")
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	f.settle()
	tt.Frame()
	tt.Frame()
	want := []string{"正在检查…", "现在检查不了更新，稍后会自动再试。"}
	var got []string
	for _, a := range tt.Announcements() {
		if slices.Contains(want, a) {
			got = append(got, a)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("read %q", tt.Announcements())
	}
}

func TestChangelogURL(t *testing.T) {
	inLocale(t, "zh-Hans")
	if got := changelogURL("0.2.0"); got != "https://gocast.yichenlab.com/zh/changelog/#v0.2.0" {
		t.Errorf("zh: %q", got)
	}
	inLocale(t, "en-GB")
	if got := changelogURL("0.2.0"); got != "https://gocast.yichenlab.com/changelog/#v0.2.0" {
		t.Errorf("en: %q", got)
	}
}

// The switch 「自动检查更新」 in General is settings.json's, and arms or
// stops the checks.
func TestAutoUpdateSwitch(t *testing.T) {
	p, tt, f := updatePrefs(t, "en", pageGeneral)
	f.u.schedule()
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Check for updates automatically")
	if !ok || !p.autoUpdate {
		t.Fatalf("switch: %+v, %v", sw, ok)
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	tt.Frame()
	if p.g.settings.AutoUpdates() || savedSettings(t, p.g).AutoUpdates() || f.nextTimer() != (time.Time{}) {
		t.Errorf("off: %v, next %v", p.g.settings.AutoUpdates(), f.nextTimer())
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	tt.Frame()
	if !savedSettings(t, p.g).AutoUpdates() || f.nextTimer() == (time.Time{}) {
		t.Errorf("on again: next %v", f.nextTimer())
	}
}

// --- Screenshots ---

func TestScreensUpdates(t *testing.T) {
	dir := screensDir(t)
	full := image.Rect(0, 0, prefsWidth, prefsHeight)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		panel := func(name string, setup func(l *launcher, f *fakeUpdater)) {
			l, tt, f := updatePanel(t, "zh-Hans")
			tt.SetScale(2)
			tt.SetDark(dark)
			tt.Frame()
			setup(l, f)
			saveScreen(t, tt, dir, name+"-"+mode, panelRect)
		}
		panel("upd-panel", func(l *launcher, f *fakeUpdater) {})
		panel("upd-relaunching", func(l *launcher, f *fakeUpdater) { f.u.relaunch() })
		panel("upd-search", func(l *launcher, f *fakeUpdater) { typeQuery(l, "更新") })
		panel("upd-row", func(l *launcher, f *fakeUpdater) {
			l.choose(slices.Index(shown(l), "group:"+updateItemID))
		})
		panel("upd-notice", func(l *launcher, f *fakeUpdater) { l.notice = locales.FooterPathCopied() })

		about := func(name string, setup func(p *prefsWindow, f *fakeUpdater)) {
			p, tt, f := updatePrefs(t, "zh-Hans", pageAbout)
			tt.SetScale(2)
			tt.SetDark(dark)
			tt.Frame()
			setup(p, f)
			saveScreen(t, tt, dir, name+"-"+mode, full)
		}
		about("about-idle", func(p *prefsWindow, f *fakeUpdater) {})
		about("about-checking", func(p *prefsWindow, f *fakeUpdater) { f.u.check(true) })
		about("about-uptodate", func(p *prefsWindow, f *fakeUpdater) {
			f.u.check(true)
			f.settle()
		})
		about("about-downloading", func(p *prefsWindow, f *fakeUpdater) {
			f.found = "0.2.0"
			f.u.check(true)
			f.next()
		})
		about("about-ready", func(p *prefsWindow, f *fakeUpdater) { f.ready("0.2.0") })
		about("about-error", func(p *prefsWindow, f *fakeUpdater) {
			f.checkErr = errors.New("offline")
			f.u.check(true)
			f.settle()
		})
		about("about-download-error", func(p *prefsWindow, f *fakeUpdater) {
			f.found, f.installErr = "0.2.0", errors.New("offline")
			f.u.check(true)
			f.settle()
		})
		about("about-unavailable", func(p *prefsWindow, f *fakeUpdater) { f.u.enabled = false })
		about("about-dev", func(p *prefsWindow, f *fakeUpdater) { f.u.enabled, f.u.dev = false, true })
		about("general-autoupdate", func(p *prefsWindow, f *fakeUpdater) { p.page = pageGeneral })
	}
}
