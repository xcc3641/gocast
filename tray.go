package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"gocast/locales"
)

// trayPNG is Gocast's mark, the search bar's shape, 18 DIP (36 px),
// black on transparent for macOS to tint (design.md 1.6);
// assets/icon/make-icon.py draws it.
//
//go:embed assets/icon/tray.png
var trayPNG []byte

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
// change, and the language (updater.MenuItem says the one shown).
func (g *gocast) updateTray() {
	if g.tray != nil {
		g.tray.SetMenu(g.trayMenu())
	}
}

func (g *gocast) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.TraySearch(), Accelerator: g.launcherKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.launcher.show() }},
		{Label: locales.TrayTerminal(), Accelerator: g.terminalKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.term.toggle() }},
		{Label: locales.ClipboardHistory(), Accelerator: g.clipKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.launcher.openClip(false) }},
		mygo.Separator(),
		{Label: locales.TraySettings(), Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) { g.openSettings("") }},
		updater.MenuItem(), // 检查更新…: the update window
		mygo.Separator(),
		{Label: locales.TrayQuit(), Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}
