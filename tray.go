package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"

	"github.com/egoist/mygo"

	"gocast/locales"
)

// trayPNG is SF Symbols' command.square, 18 DIP (36 px), black on
// transparent for macOS to tint (design.md 1.6). ui's symbols are drawn
// into bitmaps the tray cannot take, so it is drawn ahead, by AppKit:
// see docs/dev-notes.md.
//
//go:embed assets/tray.png
var trayPNG []byte

// setUpTray puts Gocast's icon in the menu bar, with its menu (copy.md 1):
// search, the floating terminal, Settings, updates, and quitting, each
// with its shortcut.
func (g *gocast) setUpTray() {
	var err error
	g.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(), IconIsTemplate: true, ToolTip: appName})
	if err != nil {
		log.Print("no tray icon: ", err)
		return
	}
	g.updateTray()
	mygo.App.OnLocaleChanged(func(string) { g.updateTray() })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { g.tray.Destroy() })
}

// updateTray builds the tray's menu again, as the shortcuts it shows
// change.
func (g *gocast) updateTray() {
	if g.tray != nil {
		g.tray.SetMenu(g.trayMenu())
	}
}

func (g *gocast) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.TraySearch(), Accelerator: g.launcherKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.launcher.show() }},
		{Label: locales.TrayTerminal(), Accelerator: g.terminalKey, Click: func(*mygo.MenuItem, *mygo.Window) { g.term.toggle() }},
		mygo.Separator(),
		{Label: locales.TraySettings(), Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) { g.openSettings("") }},
		{Label: locales.TrayCheckUpdates(), Click: func(*mygo.MenuItem, *mygo.Window) { g.checkForUpdates() }},
		mygo.Separator(),
		{Label: locales.TrayQuit(), Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}

// trayIcon is the tray's icon: command.square, or a magnifying glass
// were it missing.
func trayIcon() []byte {
	if len(trayPNG) > 0 {
		return trayPNG
	}
	return magnifierIcon()
}

// magnifierIcon draws a magnifying glass, black on transparent for macOS
// to tint.
func magnifierIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{A: 255}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx, dy := float64(x)-13, float64(y)-13
			if r := dx*dx + dy*dy; r >= 7*7 && r <= 10*10 {
				img.Set(x, y, ink)
			}
		}
	}
	for i := 0; i < 8; i++ { // the handle
		draw.Draw(img, image.Rect(20+i, 20+i, 23+i, 23+i), image.NewUniform(ink), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
