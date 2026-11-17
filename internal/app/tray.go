package app

import (
	"log"

	"github.com/egoist/mygo"

	"gocast/icon"
	"gocast/locales"
)

// sysAppQuit quits Gocast; the tests stand in for it.
var sysAppQuit = func() { mygo.App.Quit() }

// trayPNG is Gocast's mark, the search bar's shape, 18 DIP (36 px),
// black on transparent for macOS to tint (design.md 1.6);
// icon/make-icon.py draws it.
var trayPNG = icon.Tray

// setUpTray puts Gocast's icon in the menu bar, with its menu (copy.md 1):
// search, the floating terminal, the clipboard history (copy.md 6.1),
// Settings, updates, and quitting, each
// with its shortcut.
func (g *gocast) setUpTray() {
	var err error
	g.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: trayPNG, IconIsTemplate: true, ToolTip: appName})
	if err != nil {
		log.Print("no tray icon: ", err)
		return
	}
	g.updateTray()
	mygo.App.OnLocaleChanged(func(string) { g.updateTray() })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { g.tray.Destroy() })
}

// updateTray builds the tray's menu again, as the shortcuts it shows
// change, the language, and once a new version is ready.
func (g *gocast) updateTray() {
	if g.tray != nil {
		g.tray.SetMenu(g.trayMenu())
	}
}

func (g *gocast) trayMenu() *mygo.Menu {
	items := []*mygo.MenuItem{
		{Label: locales.TraySearch(), Accelerator: g.launcherKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.launcher.show() }},
		{Label: locales.TrayTerminal(), Accelerator: g.terminalKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.term.toggle() }},
		{Label: locales.ClipboardHistory(), Accelerator: g.clipKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.launcher.openClip(false) }},
		mygo.Separator(),
		{Label: locales.TraySettings(), Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) { g.openSettings("") }},
		g.updateMenuItem(),
		mygo.Separator(),
	}
	// Close All Terminals, over Quit, while there is a tab (design.md 5):
	// the terminals go on after Gocast quits, and only it ends them all.
	if g.term != nil && len(g.term.tabs) > 0 {
		items = append(items, &mygo.MenuItem{Label: locales.TrayCloseAllTerminals(), Click: func(*mygo.MenuItem, *mygo.Window) { g.term.closeAll() }})
	}
	return mygo.NewMenu(append(items,
		&mygo.MenuItem{Label: locales.TrayQuit(), Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	))
}

// setUpAppMenu gives Gocast its application menu in place of MyGo's
// default. Gocast shows no menu bar (LSUIElement), but the menu's keys
// work all the same, in every window of Gocast's that has the keyboard,
// and before it: AppKit offers a key with Command to the menu first, and
// a window sees it only when no item has it. The default menu's Quit
// took ⌘Q, quitting Gocast where the notes window hides (design.md
// 11.2), and its View menu's zoom items, which a window of native UI does
// nothing with, took ⌘=, ⌘- and ⌘0 from the notes and the terminal.
func (g *gocast) setUpAppMenu() {
	mygo.App.SetMenu(g.appMenu())
	mygo.App.OnLocaleChanged(func(string) { mygo.App.SetMenu(g.appMenu()) })
}

// appMenu is MyGo's default menu but for Quit, which is quitKey, and the
// View menu, left with what a window of native UI does: the inspector
// of a development build, and full screen. Its Edit menu gives the
// windows copy, paste and undo; its File menu ⌘W, which closes (hides)
// the window.
func (g *gocast) appMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{ID: "quit", Label: locales.TrayQuit(), Accelerator: "Cmd+Q", Click: func(*mygo.MenuItem, *mygo.Window) { g.quitKey() }},
		}},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleToggleDevTools},
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

// quitKey is ⌘Q, the application menu's Quit: with the notes window
// showing and having the keyboard, it hides it, Gocast staying
// (design.md 11.2); anywhere else Gocast quits, as with MyGo's menu. The
// tray's Quit always quits.
func (g *gocast) quitKey() {
	if n := g.notes; n != nil && n.shown && sysNotesKey(n) {
		n.hide()
		return
	}
	sysAppQuit()
}

// updateMenuItem is the tray's line of updates (design.md 4.5, 5):
// 检查更新…, which checks in Settings › About; once a new version is
// ready, in its place, 重新打开以更新到 {版本}, which relaunches into it.
func (g *gocast) updateMenuItem() *mygo.MenuItem {
	if v := g.updates.readyVersion(); v != "" {
		return &mygo.MenuItem{Label: locales.UpdateRelaunchTo(v), Click: func(*mygo.MenuItem, *mygo.Window) { g.updates.relaunch() }}
	}
	return &mygo.MenuItem{Label: locales.TrayCheckUpdates(), Click: func(*mygo.MenuItem, *mygo.Window) { g.checkForUpdates() }}
}
