// Gocast is a launcher for macOS in the manner of Raycast, in MyGo's native
// UI: Option+Shift+Space shows a search panel over every app to find and
// open installed apps (by their English and Chinese names, and by pinyin)
// and terminal presets; Control+` drops a floating terminal from the top
// of the screen, whose tabs run the presets. It lives in the menu bar,
// without a Dock icon, and runs once.
//
//	go run .
package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/internal/usage"
	"gocast/locales"
)

const (
	appName    = "Gocast"
	identifier = "dev.gocast.app"
)

// Fallbacks of the default shortcuts, taken when another app has the
// default (Raycast often has ⌘Space). Not Alt+Space, which Raycast takes
// as well (macOS would give the key to both).
var fallbackShortcuts = map[string]string{
	settings.DefaultLauncherShortcut:  settings.FallbackLauncherShortcut,
	settings.FallbackLauncherShortcut: "Cmd+Alt+Shift+Space",
	settings.DefaultTerminalShortcut:  "Ctrl+Alt+`",
}

// gocast holds the app's state. Everything here is read and changed on the
// main thread.
type gocast struct {
	dir      string // ~/Library/Application Support/dev.gocast.app
	launcher *launcher
	term     *termPanel
	prefs    *prefsWindow
	tray     *mygo.Tray
	clip     *clipService

	settings settings.Settings
	// The shortcuts registered, or "": the search panel's, the floating
	// terminal's and the clipboard history's.
	launcherKey, terminalKey, clipKey string
	// spotlight: Spotlight has ⌘Space, the search panel's shortcut, which
	// would open both; ⌥⇧Space stands in until it doesn't.
	spotlight bool

	// quitting is set while the app quits: the panels, which hide rather
	// than close, close then, or their OnClose would cancel the quit.
	quitting bool

	presets      []presets.Preset
	presetsStamp time.Time
	usage        *usage.Store
}

func (g *gocast) presetsPath() string  { return filepath.Join(g.dir, "presets.json") }
func (g *gocast) usagePath() string    { return filepath.Join(g.dir, "usage.json") }
func (g *gocast) settingsPath() string { return filepath.Join(g.dir, "settings.json") }

// loadPresets reads presets.json again when it changed (or the first
// time). It reports whether the presets changed.
func (g *gocast) loadPresets() bool {
	path := g.presetsPath()
	fi, err := os.Stat(path)
	if err == nil && fi.ModTime().Equal(g.presetsStamp) {
		return false
	}
	if err != nil && g.presetsStamp.IsZero() && g.presets != nil {
		return false
	}
	ps, err := presets.Load(path)
	if err != nil {
		log.Printf("presets: %v", err)
	}
	g.presets = ps
	if g.presets == nil {
		g.presets = []presets.Preset{}
	}
	if fi, err := os.Stat(path); err == nil {
		g.presetsStamp = fi.ModTime()
	}
	return true
}

// savePresets writes the presets and tells the panels.
func (g *gocast) savePresets(ps []presets.Preset) error {
	if err := presets.Save(g.presetsPath(), ps); err != nil {
		return err
	}
	g.presets = ps
	if fi, err := os.Stat(g.presetsPath()); err == nil {
		g.presetsStamp = fi.ModTime()
	}
	g.presetsChanged()
	return nil
}

// presetsChanged tells the panels the presets changed.
func (g *gocast) presetsChanged() {
	g.launcher.rebuild()
	g.term.presetsChanged()
	g.launcher.win.Invalidate()
}

// record notes that query opened the item id, which ranks it higher next
// time, and first for that query.
func (g *gocast) record(id, query string) {
	if id == "" {
		return
	}
	now := time.Now()
	g.usage.Record(id, query, now)
	if err := g.usage.Save(g.usagePath(), now); err != nil {
		log.Printf("usage: %v", err)
	}
}

func (g *gocast) saveSettings() {
	if err := g.settings.Save(g.settingsPath()); err != nil {
		log.Printf("settings: %v", err)
	}
}

// errShortcutTaken is the error of a shortcut the system refused, as
// another app has it.
var errShortcutTaken = errors.New("shortcut taken")

// registerShortcuts takes the shortcuts of the settings, but ⌥⇧Space for
// ⌘Space while Spotlight has it, and the fallback of a default that
// another app has, which Settings › Shortcuts then says.
func (g *gocast) registerShortcuts() {
	try := func(row int, accel string, fn func()) string {
		if err := mygo.GlobalShortcut.Register(accel, fn); err != nil {
			// Carbon refuses a shortcut another app registered (Raycast's
			// ⌘Space), not Spotlight's, which the check below is for.
			log.Printf("shortcut %s: %v", accel, err)
			fb, ok := fallbackShortcuts[accel]
			if !ok {
				return ""
			}
			if err := mygo.GlobalShortcut.Register(fb, fn); err != nil {
				log.Printf("shortcut %s: %v", fb, err)
				return ""
			}
			g.prefs.problem[row] = locales.ShortcutTaken()
			return fb
		}
		return accel
	}
	launcher := g.settings.Launcher()
	g.spotlight = launcher == settings.DefaultLauncherShortcut && shortcut.SpotlightHasCmdSpace()
	if g.spotlight {
		launcher = settings.FallbackLauncherShortcut
	}
	g.launcherKey = try(1, launcher, g.launcher.toggle)
	g.terminalKey = try(2, g.settings.Terminal(), g.term.toggle)
	g.clipKey = try(3, g.settings.Clipboard(), g.launcher.toggleClip)
}

// shortcutRows are the shortcuts by their row in Settings › Shortcuts
// (1 to 3): what each is registered as, and what it does.
func (g *gocast) shortcutRows() [4]struct {
	key *string
	fn  func()
} {
	var rows [4]struct {
		key *string
		fn  func()
	}
	rows[1].key, rows[1].fn = &g.launcherKey, g.launcher.toggle
	rows[2].key, rows[2].fn = &g.terminalKey, g.term.toggle
	rows[3].key, rows[3].fn = &g.clipKey, g.launcher.toggleClip
	return rows
}

// spotlightChanged checks again whether Spotlight has ⌘Space, as the user
// may have turned it off in System Settings, and takes ⌘Space for the
// search panel when it doesn't (design.md 4.4). It reports whether
// ⌘Space is the search panel's now.
func (g *gocast) spotlightChanged() bool {
	if !g.spotlight || g.prefs.recording != 0 || shortcut.SpotlightHasCmdSpace() {
		return false
	}
	g.spotlight = false
	g.suspendShortcuts()
	if err := g.setShortcut(1, settings.DefaultLauncherShortcut); err != nil {
		g.prefs.problem[1] = locales.ShortcutTaken() // another app has it: ⌥⇧Space stays
		return false
	}
	return true
}

// keepFallback makes ⌥⇧Space the search panel's default for good, as the
// user chose it over ⌘Space, which Spotlight has (design.md 4.4).
func (g *gocast) keepFallback() {
	g.spotlight = false
	g.settings.LauncherFallback = true
	if g.settings.LauncherShortcut == settings.FallbackLauncherShortcut {
		g.settings.LauncherShortcut = ""
	}
	if g.launcherKey == settings.FallbackLauncherShortcut {
		g.saveSettings()
		return
	}
	g.suspendShortcuts()
	if err := g.setShortcut(1, settings.FallbackLauncherShortcut); err != nil {
		g.prefs.problem[1] = locales.ShortcutTaken()
		g.saveSettings()
	}
}

// suspendShortcuts lets the keys of the shortcuts through to the app,
// while the user records a new one; resumeShortcuts takes them again.
func (g *gocast) suspendShortcuts() {
	rows := g.shortcutRows()
	for _, r := range rows[1:] {
		if *r.key != "" {
			mygo.GlobalShortcut.Unregister(*r.key)
		}
	}
}

func (g *gocast) resumeShortcuts() {
	rows := g.shortcutRows()
	for _, r := range rows[1:] {
		if *r.key != "" {
			_ = mygo.GlobalShortcut.Register(*r.key, r.fn)
		}
	}
}

// setShortcut makes accel the shortcut of the row (1 the search panel, 2
// the floating terminal, 3 the clipboard history), at once, keeping the
// old one when the system refuses it (errShortcutTaken). Call it with the
// shortcuts suspended; it resumes them.
func (g *gocast) setShortcut(row int, accel string) error {
	rows := g.shortcutRows()
	if err := mygo.GlobalShortcut.Register(accel, rows[row].fn); err != nil {
		log.Printf("shortcut %s: %v", accel, err)
		g.resumeShortcuts()
		return errShortcutTaken
	}
	*rows[row].key = accel
	switch row {
	case 1:
		g.settings.LauncherShortcut = accel
		if accel == g.settings.DefaultLauncher() {
			g.settings.LauncherShortcut = ""
		}
	case 2:
		g.settings.TerminalShortcut = accel
		if accel == settings.DefaultTerminalShortcut {
			g.settings.TerminalShortcut = ""
		}
	case 3:
		g.settings.ClipboardShortcut = accel
		if accel == settings.DefaultClipboardShortcut {
			g.settings.ClipboardShortcut = ""
		}
	}
	for i, r := range rows[1:] {
		if i+1 != row && *r.key != "" {
			_ = mygo.GlobalShortcut.Register(*r.key, r.fn)
		}
	}
	g.saveSettings()
	g.updateTray()
	return nil
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
	var err error
	if g.settings, err = settings.Load(g.settingsPath()); err != nil {
		log.Printf("settings: %v", err)
	}
	if g.usage, err = usage.Load(g.usagePath(), time.Now()); err != nil {
		log.Printf("usage: %v", err)
	}
	g.loadPresets()
	// The clipboard history, read on a goroutine from now: the view shows
	// it at once later (design.md 2.8.8).
	g.clip = newClipService(g)
	g.launcher = newLauncher(g)
	g.term = newTermPanel(g)
	g.prefs = newPrefsWindow(g)
	g.registerShortcuts()
	g.setUpTray()
	g.clip.start()
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { g.clip.stop() })
	// Back from System Settings, where the user may have freed ⌘Space.
	mygo.App.OnDidBecomeActive(func() {
		if g.spotlightChanged() {
			g.prefs.spotlightSolved()
		}
		g.prefs.win.Invalidate() // Settings › Clipboard asks for the permission again
	})
	if !g.settings.SpotlightChecked {
		// The first launch says that Spotlight has ⌘Space, in Settings,
		// the only window it opens then; the next ones don't.
		g.settings.SpotlightChecked = true
		g.saveSettings()
		if g.spotlight {
			g.prefs.notice = noticeSpotlight
			g.prefs.show(pageShortcuts)
			return
		}
	}
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
	// GOCAST_APPEARANCE=dark or light forces Gocast's appearance, whatever
	// the system's: for screenshots and checking both, the Mac left as is.
	switch os.Getenv("GOCAST_APPEARANCE") {
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	}
	// No Dock icon or app switcher entry; a bundle says so from the start
	// with LSUIElement (mygo.json).
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {}) // keep running without windows
	mygo.App.OnSecondInstance(func([]string, string) { g.launcher.show() })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { g.quitting = true })
	mygo.App.OnQuitCanceled(func() { g.quitting = false })
	useUpdater()
	mygo.App.WhenReady(g.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
