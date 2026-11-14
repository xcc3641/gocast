package app

// Gocast's actions (design.md 2.9, copy.md 15): the system's (lock the
// screen, sleep, the volume, the Trash, ...) and the window's (arrange
// the window that was in front into a half, a third, ...), found in the
// search panel and run by their hotkeys. What they ask of macOS goes
// through package sysact, which the tests stand in for.

import (
	"errors"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
	"gocast/internal/sysact"
	"gocast/locales"
)

// actionPrefix starts the item ID of an action, which usage.json scores.
const actionPrefix = "action:"

// The system the actions act on: macOS, and in tests fakes (TestMain),
// so that no test locks, sleeps or restarts the Mac, changes its volume
// or appearance, or moves the user's windows.
var (
	sysActions sysact.System  = sysact.NewSystem(identifier, identifier+".dev")
	sysWindows sysact.Windows = sysact.NewWindows(displayScreens)
	// sysConfirmAction asks before the action id (copy.md 15.3): the
	// system's alert, Gocast in front, Cancel the default; terminals: the
	// floating terminal runs some, which a restart would end.
	sysConfirmAction = confirmAction
	// sysOpenURL opens System Settings at a pane.
	sysOpenURL = func(url string) error { return exec.Command("/usr/bin/open", url).Run() }
	// sysBackground runs work off the main thread, then what it returns
	// on it: the actions that wait for Finder or System Events (or for
	// the user to allow Gocast to control them). Tests run both at once.
	sysBackground = func(g *gocast, work func() func()) {
		go func() {
			done := work()
			g.launcher.win.Update(done)
		}()
	}
)

// displayScreens are the displays, as the window actions measure them.
func displayScreens() []sysact.Screen {
	var out []sysact.Screen
	for _, d := range mygo.Screen.Displays() {
		b, w := d.Bounds, d.WorkArea
		out = append(out, sysact.Screen{
			Frame:   sysact.Rect{X: float64(b.X), Y: float64(b.Y), W: float64(b.Width), H: float64(b.Height)},
			Visible: sysact.Rect{X: float64(w.X), Y: float64(w.Y), W: float64(w.Width), H: float64(w.Height)},
		})
	}
	return out
}

// --- Names ---

// actionTitle is the name of the action id in the language of m (copy.md
// 15.1, 15.2).
func actionTitle(m locales.Messages, id string) string {
	if n := sysact.VolumeLevel(id); n >= 0 {
		return m.ActVolumeSet(n)
	}
	switch id {
	case sysact.LockScreen:
		return m.ActLockScreen()
	case sysact.Sleep:
		return m.ActSleep()
	case sysact.SleepDisplays:
		return m.ActSleepDisplays()
	case sysact.ScreenSaver:
		return m.ActScreenSaver()
	case sysact.Restart:
		return m.ActRestart()
	case sysact.ShutDown:
		return m.ActShutDown()
	case sysact.LogOut:
		return m.ActLogOut()
	case sysact.ToggleMute:
		return m.ActToggleMute()
	case sysact.VolumeUp:
		return m.ActVolumeUp()
	case sysact.VolumeDown:
		return m.ActVolumeDown()
	case sysact.ToggleAppearance:
		return m.ActToggleAppearance()
	case sysact.ShowDesktop:
		return m.ActShowDesktop()
	case sysact.HideOthers:
		return m.ActHideOthers()
	case sysact.UnhideAll:
		return m.ActUnhideAll()
	case sysact.QuitAll:
		return m.ActQuitAll()
	case sysact.OpenTrash:
		return m.ActOpenTrash()
	case sysact.EmptyTrash:
		return m.ActEmptyTrash()
	case sysact.EjectAll:
		return m.ActEjectAll()
	case sysact.ToggleHiddenFiles:
		return m.ActToggleHiddenFiles()
	case sysact.LeftHalf:
		return m.WinLeftHalf()
	case sysact.RightHalf:
		return m.WinRightHalf()
	case sysact.TopHalf:
		return m.WinTopHalf()
	case sysact.BottomHalf:
		return m.WinBottomHalf()
	case sysact.TopLeft:
		return m.WinTopLeft()
	case sysact.TopRight:
		return m.WinTopRight()
	case sysact.BottomLeft:
		return m.WinBottomLeft()
	case sysact.BottomRight:
		return m.WinBottomRight()
	case sysact.LeftThird:
		return m.WinLeftThird()
	case sysact.CenterThird:
		return m.WinCenterThird()
	case sysact.RightThird:
		return m.WinRightThird()
	case sysact.LeftTwoThirds:
		return m.WinLeftTwoThirds()
	case sysact.RightTwoThirds:
		return m.WinRightTwoThirds()
	case sysact.Maximize:
		return m.WinMaximize()
	case sysact.AlmostMaximize:
		return m.WinAlmostMaximize()
	case sysact.ReasonableSize:
		return m.WinReasonableSize()
	case sysact.Center:
		return m.WinCenter()
	case sysact.MakeLarger:
		return m.WinMakeLarger()
	case sysact.MakeSmaller:
		return m.WinMakeSmaller()
	case sysact.Restore:
		return m.WinRestore()
	case sysact.NextDisplay:
		return m.WinNextDisplay()
	case sysact.PreviousDisplay:
		return m.WinPreviousDisplay()
	}
	return id
}

// actionName is the name of the action id in Gocast's language.
func actionName(id string) string { return actionTitle(locales.In(i18n.Locale()), id) }

// actionSearchNames are the names the search finds the action id by,
// besides its name: its names in English and Chinese, and the other
// words of copy.md (15.1, 15.2).
func actionSearchNames(id string) []string {
	names := []string{actionTitle(locales.In("en"), id), actionTitle(locales.In("zh-Hans"), id)}
	return append(names, sysact.Aliases[id]...)
}

// actionTag is the tag of an action's row, and its type in Settings ›
// Shortcuts: System or Window (copy.md 15.0).
func actionTag(id string) string {
	if a, ok := sysact.Find(id); ok && a.Kind == sysact.KindWindow {
		return locales.SearchTagWindow()
	}
	return locales.SearchTagSystem()
}

// actionTier orders the actions after the apps, presets and Gocast's own
// items of the same match (design.md 2.9): the system's, then the
// window's.
func actionTier(a sysact.Action) int {
	if a.Kind == sysact.KindWindow {
		return 2
	}
	return 1
}

// --- Icons ---

// actionSquare is an action's icon (design.md 2.9): on a keycap-gray
// square of size (22 in the search panel, 20 in Settings), a symbol of
// symbolSize, or a window action's diagram, scaled from the 22 one:
// a frame 16×12 outlined in frame's color, the place the window goes
// filled in the text color inside it.
func actionSquare(c *ui.Context, k tokens, id string, size, radius, symbolSize float32, frame ui.Color) ui.Element {
	a, _ := sysact.Find(id)
	return ui.Box(c).Size(size, size).Radius(radius).Background(k.keycap).Center().Shrink(0).AccessHidden(true).Children(func() {
		x, y, w, h, ok := sysact.Diagram(id)
		if !ok {
			sfsymbols.Symbol(c, a.Symbol).Size(symbolSize).Color(k.text)
			return
		}
		s := size / iconSize
		ui.Box(c).Absolute().Left(3*s).Top(5*s).Size(16*s, 12*s).Radius(2*s).Border(1, frame)
		ui.Box(c).Absolute().Left((5+x)*s).Top((7+y)*s).Size(w*s, h*s).Radius(1 * s).Background(k.text)
	})
}

// --- Running ---

// actionSource is where an action runs from: the search panel, which
// may stay and say how it went, or its hotkey, which says nothing
// (copy.md 15.4).
type actionSource int

const (
	fromPanel actionSource = iota
	fromHotkey
)

// windowsArranger is the window actions' arranger, which remembers where
// the windows were for Restore.
func (g *gocast) windowsArranger() *sysact.Arranger {
	if g.arranger == nil {
		g.arranger = &sysact.Arranger{W: sysWindows}
	}
	return g.arranger
}

// actionHotkeyPressed runs the action id of a hotkey, on the window in
// front now; through the search panel's state while it shows, on the
// window it showed over.
func (g *gocast) actionHotkeyPressed(id string) {
	l := g.launcher
	pid := frontmostOther().PID
	if l.visible() && !l.clip.open {
		pid = l.target.app.PID
	}
	g.runAction(id, fromHotkey, pid)
}

// runAction runs the action id from src, on the frontmost window of the
// process pid for a window action (design.md 2.9's table).
func (g *gocast) runAction(id string, src actionSource, pid int) {
	a, ok := sysact.Find(id)
	if !ok {
		return
	}
	if a.Kind == sysact.KindWindow {
		g.arrange(id, src, pid)
		return
	}
	l := g.launcher
	sys := sysActions
	switch id {
	case sysact.ToggleMute, sysact.VolumeUp, sysact.VolumeDown,
		sysact.Volume0, sysact.Volume25, sysact.Volume50, sysact.Volume75, sysact.Volume100:
		g.volume(id, src) // the panel stays, to go on
	case sysact.ToggleHiddenFiles:
		sysBackground(g, func() func() {
			shown, err := sys.ToggleHiddenFiles()
			return func() {
				if err != nil {
					g.actionFailed(id, src, err)
					return
				}
				text := locales.NoticeHiddenHidden()
				if shown {
					text = locales.NoticeHiddenShown()
				}
				g.actionNotice(src, actionNotice{icon: "eye.circle", text: text})
			}
		})
	case sysact.EjectAll:
		sysBackground(g, func() func() {
			n, busy, err := sys.EjectAll()
			return func() {
				switch {
				case err != nil:
					g.actionFailed(id, src, err)
				case len(busy) > 0:
					g.actionNotice(src, actionNotice{icon: "exclamationmark.triangle", warning: true, text: locales.NoticeDiskBusy(busy[0])})
				case n == 0:
					g.actionNotice(src, actionNotice{icon: "exclamationmark.triangle", warning: true, text: locales.NoticeNoDisks()})
				default:
					g.actionNotice(src, actionNotice{icon: "eject", text: locales.NoticeEjected(n)})
				}
			}
		})
	case sysact.EmptyTrash:
		// Empty already: said, the panel staying, and no alert.
		sysBackground(g, func() func() {
			empty, err := sys.TrashEmpty()
			return func() {
				switch {
				case err != nil:
					g.actionFailed(id, src, err)
				case empty:
					g.actionNotice(src, actionNotice{icon: "trash", text: locales.NoticeTrashEmpty()})
				default:
					g.confirmThen(id, src, sys.EmptyTrash)
				}
			}
		})
	case sysact.Restart:
		g.confirmThen(id, src, sys.Restart)
	case sysact.ShutDown:
		g.confirmThen(id, src, sys.ShutDown)
	case sysact.LogOut:
		g.confirmThen(id, src, sys.LogOut)
	case sysact.QuitAll:
		g.confirmThen(id, src, sys.QuitAll)
	default:
		// Seen as done: the panel goes first, saying nothing.
		l.hideForAction()
		do := map[string]func() error{
			sysact.LockScreen:    sys.LockScreen,
			sysact.Sleep:         sys.Sleep,
			sysact.SleepDisplays: sys.SleepDisplays,
			sysact.ScreenSaver:   sys.StartScreenSaver,
			sysact.ShowDesktop:   sys.ShowDesktop,
			sysact.HideOthers:    func() error { return sys.HideOthers(pid) },
			sysact.UnhideAll:     sys.UnhideAll,
			sysact.OpenTrash:     sys.OpenTrash,
			sysact.ToggleAppearance: func() error {
				_, err := sys.ToggleAppearance()
				return err
			},
		}[id]
		if do == nil {
			return
		}
		sysBackground(g, func() func() {
			err := do()
			return func() {
				if err != nil {
					g.actionFailed(id, src, err)
				}
			}
		})
	}
}

// confirmThen hides the panel and asks before the action id (copy.md
// 15.3), then runs do, saying nothing unless Gocast may not.
func (g *gocast) confirmThen(id string, src actionSource, do func() error) {
	g.launcher.hideForAction()
	if !sysConfirmAction(id, g.term != nil && len(g.term.tabs) > 0) {
		return
	}
	sysBackground(g, func() func() {
		err := do()
		return func() {
			if err != nil {
				g.actionFailed(id, src, err)
			}
		}
	})
}

// volume runs a volume action, saying the volume after (copy.md 15.4).
func (g *gocast) volume(id string, src actionSource) {
	sys := sysActions
	level, muted, err := sys.Volume()
	if err != nil {
		g.actionFailed(id, src, err)
		return
	}
	if id == sysact.ToggleMute {
		if err := sys.SetMuted(!muted); err != nil {
			g.actionFailed(id, src, err)
			return
		}
		text, icon := locales.NoticeMuted(), "speaker.slash"
		if muted {
			text, icon = locales.NoticeUnmuted(), volumeSymbol(level)
		}
		g.actionNotice(src, actionNotice{icon: icon, text: text})
		return
	}
	switch id {
	case sysact.VolumeUp:
		level = sysact.StepVolume(level, true)
	case sysact.VolumeDown:
		level = sysact.StepVolume(level, false)
	default:
		level = float64(sysact.VolumeLevel(id)) / 100
	}
	if err := sys.SetVolume(level); err != nil {
		g.actionFailed(id, src, err)
		return
	}
	// As the volume keys, a change unmutes; but to 0.
	if muted && level > 0 {
		if err := sys.SetMuted(false); err != nil {
			log.Printf("action %s: unmuting: %v", id, err)
		}
	}
	g.actionNotice(src, actionNotice{icon: volumeSymbol(level), text: locales.NoticeVolume(sysact.Percent(level))})
}

// volumeSymbol is the speaker of a volume, 0 to 1, as the notice shows
// it.
func volumeSymbol(level float64) string {
	switch {
	case level <= 0:
		return "speaker"
	case level < 1.0/3:
		return "speaker.wave.1"
	case level < 2.0/3:
		return "speaker.wave.2"
	}
	return "speaker.wave.3"
}

// arrange runs the window action id on the frontmost window of the
// process pid: the panel goes first, the keyboard back to that window;
// it stays to say why it can't (design.md 2.9).
func (g *gocast) arrange(id string, src actionSource, pid int) {
	l := g.launcher
	arr := g.windowsArranger()
	if !sysWindows.Trusted() {
		// The banner asks, over the panel: shown by a hotkey, empty
		// (copy.md 15.5).
		l.askWindows = true
		if src == fromHotkey {
			l.showForAction()
			l.query = ""
			l.filter()
		}
		l.redraw()
		return
	}
	err := arr.Arrange(id, pid)
	if err == nil {
		l.hideForAction()
		return
	}
	var text string
	switch {
	case errors.Is(err, sysact.ErrNoWindow):
		text = locales.NoticeNoWindow()
	case errors.Is(err, sysact.ErrOneScreen):
		text = locales.NoticeOneDisplay()
	case errors.Is(err, sysact.ErrNotMoved):
		text = locales.NoticeNotMoved()
	case errors.Is(err, sysact.ErrFullScreen):
		text = locales.NoticeFullScreen()
	default:
		log.Printf("action %s: %v", id, err)
		text = locales.NoticeNoWindow()
	}
	g.actionNotice(src, actionNotice{icon: "exclamationmark.triangle", warning: true, text: text})
}

// actionFailed says why the action id failed, from the panel: the
// system did not let Gocast control an app, with a button to System
// Settings (copy.md 15.4); else nothing but the log.
func (g *gocast) actionFailed(id string, src actionSource, err error) {
	log.Printf("action %s: %v", id, err)
	var ae *sysact.AutomationError
	if !errors.As(err, &ae) {
		return
	}
	app := ae.App
	if app == "Finder" {
		app = locales.AppFinder()
	}
	g.actionNotice(src, actionNotice{icon: "exclamationmark.triangle", warning: true, text: locales.NoticeNoAutomation(app),
		settings: sysact.AutomationSettingsURL})
}

// actionNotice is what an action says in the search panel's notice
// capsule (design.md 2.9): its symbol, in the warning's color when it
// warns; a button to a pane of System Settings, which keeps it until
// dismissed.
type actionNotice struct {
	icon     string
	warning  bool
	text     string
	settings string // the URL of the pane, "" for no button
}

// actionNotice says n in the panel, run from it, which shows it again
// if it hid meanwhile; a hotkey says nothing (copy.md 15.4).
func (g *gocast) actionNotice(src actionSource, n actionNotice) {
	if src != fromPanel {
		return
	}
	l := g.launcher
	l.showForAction()
	l.sayAction(n)
}

// confirmAction is sysConfirmAction: the system's alert (design.md 2.9,
// as Close All Terminals'), the action's button red, Cancel the default
// for Return and Escape.
func confirmAction(id string, terminals bool) bool {
	title, detail, button := confirmText(id, terminals)
	mygo.App.Focus()
	r, err := mygo.Dialog.Message(mygo.MessageOptions{
		Message:            title,
		Detail:             detail,
		Buttons:            []string{button, locales.Cancel()},
		DefaultButton:      1,
		CancelButton:       1,
		DestructiveButtons: []int{0},
	})
	return err == nil && r.Button == 0
}

// confirmText is the alert of the action id (copy.md 15.3): its title,
// message and button; terminals: the floating terminal runs some, which
// restarting, shutting down and logging out end.
func confirmText(id string, terminals bool) (title, detail, button string) {
	stop := ""
	if terminals {
		stop = locales.ConfirmTerminalsStop()
	}
	switch id {
	case sysact.Restart:
		return locales.ConfirmRestartTitle(), stop, locales.ConfirmRestart()
	case sysact.ShutDown:
		return locales.ConfirmShutDownTitle(), stop, locales.ConfirmShutDown()
	case sysact.LogOut:
		detail = locales.ConfirmLogOutDetail()
		if terminals {
			detail += sentenceSep() + locales.ConfirmLogOutTerminals()
		}
		return locales.ConfirmLogOutTitle(), detail, locales.ConfirmLogOut()
	case sysact.QuitAll:
		return locales.ConfirmQuitAllTitle(), locales.ConfirmQuitAllDetail(), locales.ConfirmQuitAll()
	case sysact.EmptyTrash:
		return locales.ConfirmEmptyTrashTitle(), locales.ConfirmEmptyTrashDetail(), locales.ConfirmEmptyTrash()
	}
	return actionName(id), "", locales.FooterRun()
}

// sentenceSep joins two sentences: nothing between Chinese ones, a space
// between English ones.
func sentenceSep() string {
	if strings.HasPrefix(i18n.Locale(), "zh") {
		return ""
	}
	return " "
}

// actionHotkey is the hotkey of the action id.
func actionHotkey(id string) settings.Hotkey { return settings.Hotkey{Action: id} }

// --- In the search panel ---

// visible reports whether the search panel shows; without its window (a
// test), whether it would.
func (l *launcher) visible() bool {
	if l.win == nil {
		return l.shown
	}
	return l.win.IsVisible()
}

// hideForAction hides the search panel, if it shows, as an action runs.
func (l *launcher) hideForAction() {
	switch {
	case l.win != nil:
		l.hide()
	case l.shown:
		l.leave(time.Now())
	}
}

// showForAction shows the search panel, if hidden, for what an action
// says.
func (l *launcher) showForAction() {
	switch {
	case l.visible():
		if l.clip.open {
			l.leaveClip()
		}
	case l.win != nil:
		l.show()
	default:
		l.checkWindows()
		l.enterSearch(time.Now())
	}
}

// actionNoticeOf is the notice capsule of what an action said (design.md
// 2.9): its symbol, secondary or the warning's, the text in the text
// color; a button to System Settings, 12 from the text (the gap and its
// margin), and a × when it has one.
func (l *launcher) actionNoticeOf(c *ui.Context, k tokens, a actionNotice) notice {
	n := notice{icon: a.icon, iconColor: k.textOnGlass, color: k.text, text: a.text}
	if a.warning {
		n.iconColor = k.warning
	}
	if a.settings != "" {
		url := a.settings
		n.extra = func() {
			capsuleButton(c, k, locales.ClipOpenSettings(), func() {
				if err := sysOpenURL(url); err != nil {
					log.Printf("system settings: %v", err)
				}
			})
			dismissButton(c, k, locales.Dismiss(), func() { l.notice, l.noticeAct = "", nil })
		}
	}
	return n
}

// checkWindows asks again, as the panel shows, whether Gocast may arrange
// windows, the banner asking for it: allowed now, the banner says so for
// a while (copy.md 15.5).
func (l *launcher) checkWindows() {
	if !l.askWindows || !sysWindows.Trusted() {
		return
	}
	l.askWindows, l.windowsReady = false, true
	l.windowsReadyN++
	n := l.windowsReadyN
	if l.win == nil { // drawn in a test
		return
	}
	time.AfterFunc(readyFor, func() {
		l.win.Update(func() {
			if l.windowsReadyN == n {
				l.windowsReady = false
			}
		})
	})
}

// windowsBanner asks for Accessibility over the list, as the clipboard
// view's does (design.md 2.8.7, 2.9), with its one button; or says it is
// allowed now.
func (l *launcher) windowsBanner(c *ui.Context, k tokens) {
	if !l.askWindows && !l.windowsReady {
		return
	}
	fade := &ui.Motion{Opacity: 0}
	tr := ui.ElementTransition{Exit: fade, ExitDuration: 150 * time.Millisecond}
	if c.Preferences().ReduceMotion {
		tr = ui.ElementTransition{}
	}
	c.Key("windowsBanner")
	block := ui.Row(c).Margin(8, 8, 8, 8).Radius(12).Background(k.keycap).Padding(12, 14).Gap(10).Shrink(0).Transition(tr)
	if l.windowsReady {
		block.Height(44).Padding(0, 14).AlignItems(ui.Center).Role(ui.RoleStatus).Children(func() {
			sfsymbols.Symbol(c, "checkmark.circle.fill").Size(16).Color(k.success)
			ui.Text(c, locales.WinAXReady()).FontSize(13).TextColor(k.text)
		})
		return
	}
	title := locales.WinAXTitle()
	block.AlignItems(ui.Center).Role(ui.RoleGroup).Label(title).Children(func() {
		ui.Box(c).AlignSelf(ui.Start).Children(func() {
			sfsymbols.Symbol(c, "hand.raised.fill").Size(16).Color(k.textOnGlass)
		})
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Margin(0, 6, 0, 0).Children(func() {
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
			ui.Text(c, locales.WinAXBody()).FontSize(12).TextColor(k.textOnGlass)
		})
		clipButton(c, k, locales.ClipOpenSettings(), true, func() {
			if err := sysOpenURL(sysact.AccessibilitySettingsURL); err != nil {
				log.Printf("accessibility settings: %v", err)
			}
		})
	})
}
