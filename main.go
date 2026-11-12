// Gocast is a launcher for macOS in the manner of Raycast, in MyGo's native
// UI: Option+Shift+Space shows a search panel over every app to find and
// open installed apps (by their English and Chinese names, and by pinyin)
// and terminal presets; Control+` shows a floating terminal whose tabs run
// the presets of presets.json. It lives in the menu bar, without a Dock
// icon, and runs once.
//
//	go run .
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"

	"gocast/internal/presets"
)

const (
	appName    = "Gocast"
	identifier = "dev.gocast.app"
)

// The global shortcuts, each tried in turn: the first one free is taken.
// Not Alt+Space, which Raycast takes (macOS would give the key to both).
var (
	launcherShortcuts = []string{"Alt+Shift+Space", "CmdOrCtrl+Alt+Shift+Space"}
	terminalShortcuts = []string{"Ctrl+`", "Ctrl+Alt+`"}
)

// gocast holds the app's state. Everything here is read and changed on the
// main thread.
type gocast struct {
	dir      string // ~/Library/Application Support/dev.gocast.app
	launcher *launcher
	term     *termPanel
	tray     *mygo.Tray

	launcherKey, terminalKey string // the shortcuts taken, or ""

	// quitting is set while the app quits: the panels, which hide rather
	// than close, close then, or their OnClose would cancel the quit.
	quitting bool

	presets      []presets.Preset
	presetsStamp time.Time
	usage        map[string]int // times opened, by item ID
}

func (g *gocast) presetsPath() string { return filepath.Join(g.dir, "presets.json") }
func (g *gocast) usagePath() string   { return filepath.Join(g.dir, "usage.json") }

// loadPresets reads presets.json again when it changed (or the first
// time), writing the defaults when there is none. It reports whether the
// presets changed.
func (g *gocast) loadPresets() bool {
	path := g.presetsPath()
	if fi, err := os.Stat(path); err == nil && g.presets != nil && fi.ModTime().Equal(g.presetsStamp) {
		return false
	}
	ps, err := presets.Load(path)
	if err != nil {
		log.Printf("presets: %v", err)
	}
	g.presets = ps
	if fi, err := os.Stat(path); err == nil {
		g.presetsStamp = fi.ModTime()
	}
	return true
}

func (g *gocast) loadUsage() {
	g.usage = map[string]int{}
	if data, err := os.ReadFile(g.usagePath()); err == nil {
		_ = json.Unmarshal(data, &g.usage)
	}
}

// used counts an opening of the item, which ranks it higher next time.
func (g *gocast) used(id string) {
	g.usage[id]++
	data, _ := json.Marshal(g.usage)
	if err := os.WriteFile(g.usagePath(), data, 0o644); err != nil {
		log.Printf("usage: %v", err)
	}
}

// hideOnClose makes closing win (Escape, Cmd-W) hide it, so that it shows
// again at once, as it was; quitting closes it.
func (g *gocast) hideOnClose(win *mygo.Window) {
	win.OnClose(func(e *mygo.CloseEvent) {
		if !g.quitting {
			e.PreventDefault()
			win.Hide()
		}
	})
}

func register(shortcuts []string, fn func()) string {
	for _, s := range shortcuts {
		err := mygo.GlobalShortcut.Register(s, fn)
		if err == nil {
			return s
		}
		log.Printf("shortcut %s: %v", s, err)
	}
	return ""
}

func (g *gocast) ready() {
	// Load libghostty-vt now, off the main thread: a program not packaged
	// by the CLI (go run) downloads it the first time, which the first
	// terminal would otherwise wait for with the UI stuck.
	go func() {
		if err := terminal.Load(); err != nil {
			log.Printf("terminal: %v", err)
		}
	}()
	g.loadUsage()
	g.loadPresets()
	g.launcher = newLauncher(g)
	g.term = newTermPanel(g)
	g.launcherKey = register(launcherShortcuts, g.launcher.toggle)
	g.terminalKey = register(terminalShortcuts, g.term.toggle)
	g.setUpTray()
	if !mygo.App.WasOpenedAtLogin() {
		g.launcher.show() // started by the user: show what it does
	}
}

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running Gocast shows its panel (OnSecondInstance)
	}
	base, err := os.UserConfigDir() // ~/Library/Application Support
	if err != nil {
		log.Fatal(err)
	}
	g := &gocast{dir: filepath.Join(base, identifier)}
	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		log.Fatal(err)
	}
	mygo.App.SetName(appName)
	// No Dock icon or app switcher entry; a bundle says so from the start
	// with LSUIElement (mygo.json).
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {}) // keep running without windows
	mygo.App.OnSecondInstance(func([]string, string) { g.launcher.show() })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { g.quitting = true })
	mygo.App.OnQuitCanceled(func() { g.quitting = false })
	mygo.App.WhenReady(g.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
