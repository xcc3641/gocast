package app

// updates.go keeps Gocast up to date without a window (design.md 4.5),
// with MyGo's updater itself rather than its plugin: it checks in the
// background while 「自动检查更新」 is on, 10 seconds after launch and
// then once a day, and downloads and installs a newer version at once,
// which runs once Gocast relaunches or at its next launch. Once it is
// ready, four places say so: the pill at the bottom left of the search
// panel, the first row of the Gocast group, the tray menu's line in place
// of 检查更新…, and the status line of Settings › About, where 检查更新
// checks on the page.

import (
	"context"
	"errors"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/locales"
)

const (
	// firstCheckAfter is how long after launch the first automatic check
	// comes, and checkEvery how long after a check the next one does.
	firstCheckAfter = 10 * time.Second
	checkEvery      = 24 * time.Hour
	// upToDateFor is how long About says 已是最新版本.
	upToDateFor = 4 * time.Second

	// relaunchedArg tells a Gocast that relaunched to update that it did:
	// it shows nothing as it opens, as at login (design.md 4.5).
	relaunchedArg = "--relaunched"
)

// foundUpdate is a newer version than the one running, which Check found,
// and how to install it.
type foundUpdate struct {
	version string
	install func(ctx context.Context) error
}

// MyGo's updater, which tests replace (helpers_test.go): whether this
// build can update itself, whether it is a development build, checking
// for a newer version, and relaunching into it.
var (
	sysUpdatesEnabled = func() bool { return mygo.Updater.Enabled() }
	sysDevBuild       = func() bool { return mygo.IsDev() || mygo.App.Version() == "" }
	sysCheckUpdate    = func(ctx context.Context) (*foundUpdate, error) {
		up, err := mygo.Updater.Check(ctx)
		if err != nil || up == nil {
			return nil, err
		}
		// Install downloads the delta or the archive, checks its signature
		// and stages the new bundle, which replaces Gocast's once it has
		// exited; installing another later stages that one instead.
		return &foundUpdate{version: up.Version, install: func(ctx context.Context) error {
			return up.Install(ctx, nil)
		}}, nil
	}
	sysRelaunch = func() { mygo.App.Relaunch(relaunchedArg) }
)

// relaunched reports whether this Gocast relaunched to update.
func relaunched() bool { return slices.Contains(os.Args[1:], relaunchedArg) }

// updatePhase is what the update service does.
type updatePhase int

const (
	updateIdle updatePhase = iota
	updateChecking
	updateDownloading
)

// updateResult is what a check the user asked for ended with, which About
// says.
type updateResult int

const (
	resultNone updateResult = iota
	resultUpToDate
	resultCheckFailed
	resultDownloadFailed
)

// updateService checks for updates and installs them. Its fields are read
// and changed on the main thread; checks and installs run on goroutines,
// which post their results back.
type updateService struct {
	g *gocast
	// post runs a function on the main thread; after runs one there in a
	// while, and returns what stops it; now is the time. Tests replace
	// them.
	post  func(func())
	after func(time.Duration, func()) (stop func())
	now   func() time.Time

	// enabled: this build can update itself (MyGo's Updater.Enabled);
	// dev: it is a development build, which About says when it can't.
	enabled, dev bool
	launched     time.Time
	stopTimer    func() // the timer of the next automatic check, or nil

	phase   updatePhase
	version string // the version downloading
	// asked: the user asked for the check under way (About's button, the
	// tray), or joined it; only then does About say what it found.
	asked     bool
	result    updateResult
	resultGen int
	// ready is the version installed, which runs once Gocast relaunches,
	// or "".
	ready       string
	relaunching bool
	// The pill: closed with its × for this run; announced once.
	pillClosed, pillSaid bool
}

func newUpdateService(g *gocast, now time.Time) *updateService {
	u := &updateService{g: g, launched: now, now: time.Now}
	u.post = func(f func()) { g.launcher.win.Update(f) }
	u.after = func(d time.Duration, f func()) func() {
		t := time.AfterFunc(d, func() { u.post(f) })
		return func() { t.Stop() }
	}
	u.enabled = sysUpdatesEnabled()
	u.dev = !u.enabled && sysDevBuild()
	return u
}

// start arms the first automatic check, and checks again as the Mac wakes
// when the last check is a day old (design.md 4.5): the timer counts no
// time asleep.
func (u *updateService) start() {
	u.schedule()
	mygo.Power.OnResume(func() { u.post(u.schedule) })
	mygo.App.OnQuitCanceled(u.quitCanceled)
}

// quitCanceled takes the quit of a relaunch canceled (an app asked, the
// user kept it): the pill offers to relaunch again.
func (u *updateService) quitCanceled() {
	if u.relaunching {
		u.relaunching = false
		u.changed(false)
	}
}

// nextCheck is when the automatic check after the one at last is due, in
// a Gocast launched at launched, at now: a day after it, and no sooner
// than 10 seconds after launch, so that relaunching does not check again.
// A last check after now (the clock was set back) counts for none.
func nextCheck(last, launched, now time.Time) time.Time {
	first := launched.Add(firstCheckAfter)
	if last.IsZero() || last.After(now) {
		return first
	}
	if next := last.Add(checkEvery); next.After(first) {
		return next
	}
	return first
}

// schedule arms the timer of the next automatic check, or checks now if
// it is due; none while 「自动检查更新」 is off, a check runs, or this
// build can't update itself.
func (u *updateService) schedule() {
	if u.stopTimer != nil {
		u.stopTimer()
		u.stopTimer = nil
	}
	if !u.enabled || !u.g.settings.AutoUpdates() || u.phase != updateIdle {
		return
	}
	now := u.now()
	due := nextCheck(u.g.settings.UpdatesChecked, u.launched, now)
	if !due.After(now) {
		u.check(false)
		return
	}
	u.stopTimer = u.after(due.Sub(now), u.schedule)
}

// check checks for a newer version, and installs it; asked: the user
// asked (About's button, the tray), and About says what it found. A check
// asked for while one runs joins it.
func (u *updateService) check(asked bool) {
	if !u.enabled {
		return // About says why; its button is dimmed
	}
	if u.phase != updateIdle {
		u.asked = u.asked || asked
		u.changed(false)
		return
	}
	if u.stopTimer != nil {
		u.stopTimer()
		u.stopTimer = nil
	}
	u.phase, u.asked, u.result = updateChecking, asked, resultNone
	u.g.settings.UpdatesChecked = u.now()
	u.g.saveSettings()
	u.changed(false)
	go func() {
		up, err := sysCheckUpdate(context.Background())
		u.post(func() { u.checked(up, err) })
	}()
}

// checked takes what the check found: a newer version, which it
// downloads, unless it is the one ready.
func (u *updateService) checked(up *foundUpdate, err error) {
	switch {
	case errors.Is(err, mygo.ErrUpdatesDisabled):
		u.enabled, u.dev = false, sysDevBuild()
		u.finish(resultNone)
	case err != nil:
		log.Printf("updates: %v", err)
		u.finish(resultCheckFailed)
	case up == nil:
		u.finish(resultUpToDate)
	case up.version == u.ready:
		u.finish(resultNone) // About says it is ready
	default:
		u.phase, u.version = updateDownloading, up.version
		u.changed(false)
		go func() {
			err := up.install(context.Background())
			u.post(func() { u.installed(up.version, err) })
		}()
	}
}

// installed takes the end of an install: the version is ready, or it
// failed and the next check tries again.
func (u *updateService) installed(version string, err error) {
	if err != nil {
		log.Printf("updates: installing %s: %v", version, err)
		u.finish(resultDownloadFailed)
		return
	}
	u.ready = version
	u.finish(resultNone)
	u.changed(true)
}

// finish ends the check, About saying how if the user asked; 已是最新版本
// for a while.
func (u *updateService) finish(r updateResult) {
	u.phase, u.version = updateIdle, ""
	if u.asked {
		u.result = r
		u.resultGen++
		if r == resultUpToDate {
			gen := u.resultGen
			u.after(upToDateFor, func() {
				if u.resultGen == gen && u.result == resultUpToDate {
					u.result = resultNone
					u.changed(false)
				}
			})
		}
	}
	u.asked = false
	u.changed(false)
	u.schedule()
}

// autoChanged takes 「自动检查更新」 switched in Settings: on, the next
// check is armed; off, none is. A version ready stays ready.
func (u *updateService) autoChanged() { u.schedule() }

// relaunch quits Gocast and opens it again, which runs the version ready:
// the pill, the Gocast group's row, the tray and About do it alike.
func (u *updateService) relaunch() {
	if u.ready == "" || u.relaunching {
		return
	}
	u.relaunching = true
	u.changed(false)
	sysRelaunch()
}

// readyVersion is the version ready, which runs once Gocast relaunches, or
// "": none, or no update service (a test's panel).
func (u *updateService) readyVersion() string {
	if u == nil {
		return ""
	}
	return u.ready
}

// pill is what the search panel's pill shows: the version ready, unless
// closed; relaunching, whatever the pill showed (design.md 4.5).
func (u *updateService) pill() (version string, relaunching bool) {
	if u == nil || u.ready == "" {
		return "", false
	}
	if u.relaunching {
		return u.ready, true
	}
	if u.pillClosed {
		return "", false
	}
	return u.ready, false
}

// changed shows the state where it shows: the search panel, its Gocast
// group again when a version became ready, the tray and Settings.
func (u *updateService) changed(ready bool) {
	g := u.g
	if g.launcher != nil {
		if ready {
			g.launcher.rebuild()
		}
		g.launcher.redraw()
	}
	if ready {
		g.updateTray()
	}
	if g.prefs != nil && g.prefs.win != nil {
		g.prefs.win.Invalidate()
	}
}

// checkForUpdates is the tray's 检查更新…: Settings › About, checking
// there (design.md 4.5, 5).
func (g *gocast) checkForUpdates() {
	g.openSettings(pageAbout)
	g.updates.check(true)
}

// setAutoUpdates switches 「自动检查更新」 and saves it.
func (g *gocast) setAutoUpdates(on bool) {
	g.settings.SetAutoUpdates(on)
	g.saveSettings()
	if g.updates != nil {
		g.updates.autoChanged()
	}
}

// --- About's status line (design.md 4.5, 关于页) ---

// The button of About, over the status line.
const (
	aboutCheck    = iota // 检查更新
	aboutCheckOff        // 检查更新, dimmed
	aboutRelaunch        // 重新打开, the primary button
)

// aboutStatus is what About shows of the updates: its button, and the
// status line under it, with an icon (an SF Symbol, or spinnerIcon), and
// the link to the changes of version, if any. say: read out as it comes.
type aboutStatus struct {
	button     int
	icon, text string
	version    string
	say        bool
}

// spinnerIcon is the status line's spinner, in place of a symbol.
const spinnerIcon = "spinner"

// about is what About shows, as the service stands. A background check
// shows as it runs, but neither 已是最新版本 nor a failure: the user did
// not ask.
func (u *updateService) about() aboutStatus {
	switch {
	case u == nil:
		return aboutStatus{button: aboutCheckOff}
	case !u.enabled && u.dev:
		return aboutStatus{button: aboutCheckOff, icon: "info.circle", text: locales.UpdateDevBuild()}
	case !u.enabled:
		return aboutStatus{button: aboutCheckOff, icon: "info.circle", text: locales.UpdateUnavailable()}
	case u.phase == updateDownloading:
		return aboutStatus{button: aboutCheckOff, icon: spinnerIcon, text: locales.UpdateDownloading(u.version), version: u.version}
	case u.ready != "":
		return aboutStatus{button: aboutRelaunch, text: locales.UpdateReady(u.ready), version: u.ready, say: true}
	case u.phase == updateChecking:
		return aboutStatus{button: aboutCheckOff, icon: spinnerIcon, text: locales.UpdateChecking(), say: true}
	case u.result == resultUpToDate:
		return aboutStatus{button: aboutCheck, icon: "checkmark.circle", text: locales.UpdateUpToDate(), say: true}
	case u.result == resultCheckFailed:
		return aboutStatus{button: aboutCheck, icon: "exclamationmark.triangle", text: locales.UpdateCheckFailed(), say: true}
	case u.result == resultDownloadFailed:
		return aboutStatus{button: aboutCheck, icon: "exclamationmark.triangle", text: locales.UpdateDownloadFailed(), say: true}
	}
	return aboutStatus{button: aboutCheck}
}

// changelogURL is the website's page of the changes of version, in the
// language Gocast shows (design.md 4.5).
func changelogURL(version string) string {
	page := "https://gocast.yichenlab.com/changelog/"
	if strings.HasPrefix(i18n.Locale(), "zh") {
		page = "https://gocast.yichenlab.com/zh/changelog/"
	}
	return page + "#v" + version
}

// The system's link color (design.md 4.5), which keeps 4.5:1 on Settings'
// white and dark surfaces.
var (
	linkLight = ui.Hex("#0068DA")
	linkDark  = ui.Hex("#419CFF")
)

// The status line's measures: as high as two lines of it, which it
// keeps empty, so that nothing above it moves as it changes; at most as
// wide as aboutStatusWidth.
const (
	aboutStatusHeight = 32
	aboutStatusWidth  = 360
	aboutLine         = 16 // the line height of its 12 pt text
)

// aboutButton is About's button: 检查更新, dimmed while it can't check,
// or 重新打开 once a version is ready, in a slot 24 high.
func (p *prefsWindow) aboutButton(c *ui.Context, st aboutStatus) {
	u := p.g.updates
	ui.Row(c).Height(24).Shrink(0).AlignItems(ui.Center).Children(func() {
		if st.button == aboutRelaunch {
			ui.PrimaryButton(c, locales.UpdateRelaunch()).OnClick(u.relaunch)
			return
		}
		ui.Button(c, locales.AboutCheck()).Disabled(st.button == aboutCheckOff).OnClick(func() { u.check(true) })
	})
}

// aboutStatusLine is the line under About's button, 32 high whatever it
// says (design.md 4.5); 已是最新版本 fades as it goes.
func (p *prefsWindow) aboutStatusLine(c *ui.Context, k tokens, st aboutStatus) {
	if st.say && st.text != p.updateSaid {
		c.Announce(st.text)
	}
	p.updateSaid = ""
	if st.say {
		p.updateSaid = st.text
	}
	link := linkLight
	if c.Theme().Dark {
		link = linkDark
	}
	ui.Box(c).Height(aboutStatusHeight).Width(aboutStatusWidth).Shrink(0).Margin(8, 0, 0, 0).Children(func() {
		if st.text == "" {
			return
		}
		c.Key(st.text)
		line := ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Start).Gap(6).Role(ui.RoleStatus)
		if !c.Preferences().ReduceMotion {
			line.Transition(ui.ElementTransition{Exit: &ui.Motion{Opacity: 0}, ExitDuration: 150 * time.Millisecond})
		}
		line.Children(func() {
			// The icon on the middle of the first line.
			switch st.icon {
			case "":
			case spinnerIcon:
				ui.Spinner(c).Size(12, 12).Shrink(0).Margin((aboutLine-12)/2, 0, 0, 0)
			default:
				sfsymbols.Symbol(c, st.icon).Size(12).Color(k.textSecondary).Margin((aboutLine-12)/2, 0, 0, 0)
			}
			ui.Text(c, st.text).FontSize(12).LineHeight(aboutLine / 12.0).TextColor(k.textSecondary).MaxLines(2).Shrink(1)
			if st.version != "" {
				// 「 · 」: the dot 4 from either side (the text trims the
				// spaces of " · ").
				ui.Row(c).Shrink(0).Gap(4).Margin(0, 0, 0, -2).Children(func() {
					ui.Text(c, "·").FontSize(12).LineHeight(aboutLine / 12.0).TextColor(k.textSecondary).AccessHidden(true)
					ui.Link(c, locales.UpdateWhatsNew(), changelogURL(st.version)).FontSize(12).LineHeight(aboutLine / 12.0).TextColor(link)
				})
			}
		})
	})
}

// --- The pill (design.md 4.5, 新版本胶囊) ---

// The pill's measures, besides the notice capsule's (2.4): its padding,
// its button's and its ×'s.
const (
	pillPadR          = 6
	pillButtonPad     = 10
	pillButtonHeight  = 24
	pillButtonGap     = 10 // from the text to the button
	pillDismissGap    = 2  // from the button to the ×
	pillDismissSize   = 24
	pillTextIconGap   = 6
	noticeCapsulePadL = 14
)

// updatePill is the new version pill at the bottom left of the search
// panel, inset from the left and bottom from the bottom, at most maxWidth
// wide, its text cut at the end if it must: the version ready, Relaunch
// and ×; or, relaunching, a spinner and 正在重新打开….
func (l *launcher) updatePill(c *ui.Context, k tokens, inset, bottom, maxWidth float32, version string, relaunching bool) {
	u := l.g.updates
	c.Key("update")
	e := capsule(c, k, bottom).Left(inset).MaxWidth(maxWidth).Padding(0, pillPadR, 0, noticeCapsulePadL)
	if !c.Preferences().ReduceMotion {
		fade := &ui.Motion{Opacity: 0}
		e.Transition(ui.ElementTransition{Enter: fade, Exit: fade, Duration: 150 * time.Millisecond})
	}
	if relaunching {
		text := locales.UpdateRelaunching()
		e.Padding(0, noticeCapsulePadL).Role(ui.RoleStatus).Label(text).Children(func() {
			ui.Spinner(c).Size(14, 14).TextColor(k.textOnGlass).Shrink(0).AccessHidden(true)
			ui.Text(c, text).FontSize(12).TextColor(k.textOnGlass).SingleLine().Shrink(1).
				Margin(0, 0, 0, pillTextIconGap).AccessHidden(true)
		})
		return
	}
	text := locales.UpdateReadyPill(version)
	label := text + listSep() + locales.UpdateRelaunch() // read whole (copy.md 12.1)
	if !u.pillSaid {
		u.pillSaid = true // read the first time this run, not at every show (design.md 6)
		c.Announce(label)
	}
	e.Tooltip(text).Children(func() {
		sfsymbols.Symbol(c, "arrow.down.circle.fill").Size(14).Color(c.Theme().Accent)
		ui.Text(c, text).FontSize(12).TextColor(k.text).SingleLine().Shrink(1).
			Margin(0, 0, 0, pillTextIconGap).AccessHidden(true)
		b := ui.Row(c).Height(pillButtonHeight).Radius(pillButtonHeight/2).Padding(0, pillButtonPad).
			AlignItems(ui.Center).Shrink(0).Margin(0, 0, 0, pillButtonGap).Background(k.keycap).
			Role(ui.RoleButton).Label(label).OnClick(func() {
			l.input.Focus()
			u.relaunch()
		})
		if b.Pressed() {
			b.Background(pressedFill(k))
		}
		b.Children(func() {
			ui.Text(c, locales.UpdateRelaunch()).FontSize(12).FontWeight(500).TextColor(k.text).SingleLine()
		})
		x := ui.Box(c).Size(pillDismissSize, pillDismissSize).Radius(pillDismissSize/2).Center().Shrink(0).
			Margin(0, 0, 0, pillDismissGap).Role(ui.RoleButton).Label(locales.Dismiss()).Tooltip(locales.Dismiss()).
			OnClick(func() {
				u.pillClosed = true
				l.input.Focus()
				l.redraw()
			})
		if x.Pressed() {
			x.Background(k.selection)
		}
		x.Children(func() {
			sfsymbols.Symbol(c, "xmark").Size(10).Weight(sfsymbols.Semibold).Color(k.textOnGlass)
		})
	})
}

// pressedFill is the pill's Relaunch button pressed: the selection gray
// with 0.04 more black (white in dark mode) over it (design.md 4.5): the
// selection's own color, black or white, so the two make one fill.
func pressedFill(k tokens) ui.Color {
	s := k.selection
	a := float32(s.A) / 255
	return s.Alpha((a + 0.04*(1-a)) / a)
}
