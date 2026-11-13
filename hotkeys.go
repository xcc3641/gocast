package main

// The hotkeys of apps and terminal presets: global shortcuts that open an
// app (or hide it, frontmost) or a preset's tab in the floating terminal
// (or hide it, showing that tab). settings.json keeps them; Settings ›
// Shortcuts and the search panel's Cmd-K menu set them.

import (
	"errors"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"

	"gocast/internal/presets"
	"gocast/internal/running"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// shortcutRegistry registers global shortcuts: MyGo's, which needs the
// app running, and a fake in tests.
type shortcutRegistry interface {
	Register(accel string, fn func()) error
	Unregister(accel string)
}

var globalShortcuts shortcutRegistry = mygo.GlobalShortcut

// hotkeyState is how the hotkey of an app or preset stands, for Settings
// and the search panel to say (gocast.hotkeyStates).
type hotkeyState int

const (
	hotkeyOn       hotkeyState = iota // registered: it works
	hotkeyTaken                       // the system refused it: another app has it
	hotkeyMissing                     // the app is not installed: kept, not registered
	hotkeyConflict                    // Gocast's own or another's (settings.json edited by hand)
	hotkeyInvalid                     // not one the recorder takes (edited by hand)
)

// hotkeyOwner is who has a shortcut: one of Gocast's own, by its row in
// Settings › Shortcuts (1 to 3), else the app or preset of item.
type hotkeyOwner struct {
	row  int
	item settings.Hotkey
}

var (
	// errHotkeyInvalid: the shortcut needs ⌘, ⌥ or ⌃, or to be a
	// function key.
	errHotkeyInvalid = errors.New("hotkey needs Command, Option or Control, or a function key")
	// errNoPreset: the preset is gone.
	errNoPreset = errors.New("no such preset")
)

// hotkeyConflictError is the error of a shortcut another has, which the
// recorder says.
type hotkeyConflictError struct{ owner hotkeyOwner }

func (e *hotkeyConflictError) Error() string { return "shortcut used by another" }

// --- What the UI calls ---

// setAppHotkey makes accel the hotkey of the app with bundleID, at once,
// and saves it. It keeps the old one, and says why, when accel is not a
// shortcut the recorder takes (errHotkeyInvalid), another has it
// (*hotkeyConflictError) or the system refused it (errShortcutTaken). An
// app not installed keeps it, unregistered (hotkeyMissing).
func (g *gocast) setAppHotkey(bundleID, accel string) error {
	return g.setHotkey(settings.Hotkey{App: bundleID}, accel)
}

// removeAppHotkey removes the hotkey of the app with bundleID.
func (g *gocast) removeAppHotkey(bundleID string) {
	g.removeHotkey(settings.Hotkey{App: bundleID})
}

// setPresetHotkey is setAppHotkey for the preset with the ID, which must
// exist (errNoPreset).
func (g *gocast) setPresetHotkey(id, accel string) error {
	if presets.Find(g.presets, id) < 0 {
		return errNoPreset
	}
	return g.setHotkey(settings.Hotkey{Preset: id}, accel)
}

// removePresetHotkey removes the hotkey of the preset with the ID.
func (g *gocast) removePresetHotkey(id string) {
	g.removeHotkey(settings.Hotkey{Preset: id})
}

// hotkeyStatus is how the hotkey of h stands; ok is false when it has
// none.
func (g *gocast) hotkeyStatus(h settings.Hotkey) (state hotkeyState, ok bool) {
	state, ok = g.hotkeyStates[h]
	return state, ok
}

// hotkeyItem is an app or preset that may have a hotkey, as a list of
// them shows it.
type hotkeyItem struct {
	item  settings.Hotkey
	name  string
	app   *mygo.InstalledApp // nil for a preset, or an app not installed
	accel string             // its hotkey, "" for none
	state hotkeyState        // how accel stands
}

// hotkeyItems are all the items that may have a hotkey: the presets, the
// apps the search panel knows, and the apps not installed that have one.
// Cheap enough to call as a list shows (the settings' maps, no lookups
// but for the names of apps not installed).
func (g *gocast) hotkeyItems() []hotkeyItem {
	items := make([]hotkeyItem, 0, len(g.presets)+len(g.launcher.apps))
	add := func(h settings.Hotkey, name string, app *mygo.InstalledApp) {
		it := hotkeyItem{item: h, name: name, app: app, accel: g.settings.Hotkey(h)}
		it.state = g.hotkeyStates[h]
		items = append(items, it)
	}
	for _, p := range g.presets {
		add(settings.Hotkey{Preset: p.ID}, presetTitle(p), nil)
	}
	known := make(map[string]bool, len(g.launcher.apps))
	for i := range g.launcher.apps {
		a := &g.launcher.apps[i]
		known[a.ID] = true
		add(settings.Hotkey{App: a.ID}, a.Name, a)
	}
	for _, h := range g.settings.Hotkeys() {
		if h.App != "" && !known[h.App] {
			add(h, g.hotkeyLabel(h), nil)
		}
	}
	return items
}

// hotkeyProblem is what the recorder says of a hotkey refused (copy.md
// 11.2): a conflict names the other, by its row's label or its name.
func (g *gocast) hotkeyProblem(err error) string {
	var ce *hotkeyConflictError
	switch {
	case errors.As(err, &ce) && ce.owner.row > 0:
		return locales.ShortcutUsedFor(shortcutLabel(ce.owner.row))
	case errors.As(err, &ce):
		return locales.ShortcutUsedFor(g.hotkeyLabel(ce.owner.item))
	case errors.Is(err, errShortcutTaken):
		return locales.HotkeyInUse()
	}
	return locales.HotkeyNeedsModifier()
}

// shortcutLabel is the label of the row of one of Gocast's own
// shortcuts, 1 to 3.
func shortcutLabel(row int) string {
	switch row {
	case 1:
		return locales.ShortcutLauncher()
	case 2:
		return locales.ShortcutTerminal()
	}
	return locales.ShortcutClipboard()
}

// hotkeyLabel names the app or preset of h, as a conflict says it: the
// app's name, else its bundle's, else (not installed) the name stored
// when its hotkey was set, else its bundle ID.
func (g *gocast) hotkeyLabel(h settings.Hotkey) string {
	if h.App == "" {
		if i := presets.Find(g.presets, h.Preset); i >= 0 {
			return presetTitle(g.presets[i])
		}
		return h.Preset
	}
	if name := g.appName(h.App); name != "" {
		return name
	}
	if name := g.settings.AppHotkeyName(h.App); name != "" {
		return name
	}
	return h.App
}

// appName is the name of the app with the bundle ID id as installed: the
// search panel's, else its bundle's; "" when not installed.
func (g *gocast) appName(id string) string {
	if i := slices.IndexFunc(g.launcher.apps, func(a mygo.InstalledApp) bool { return a.ID == id }); i >= 0 {
		return g.launcher.apps[i].Name
	}
	if path := running.AppPath(id); path != "" {
		return strings.TrimSuffix(filepath.Base(path), ".app")
	}
	return ""
}

// storeHotkeyName stores with the hotkey of h, an app's, the name it has
// now, for when it is gone; not installed, the name stored stays.
func (g *gocast) storeHotkeyName(h settings.Hotkey) {
	if h.App == "" {
		return
	}
	if name := g.appName(h.App); name != "" {
		g.settings.SetAppHotkeyName(h.App, name)
	}
}

// --- Checking ---

// ownShortcuts are Gocast's own shortcuts by row (1 to 3), each as
// registered and as set, which differ for ⌥⇧Space standing in for ⌘Space
// or a shortcut the system refused: neither may be another's.
func (g *gocast) ownShortcuts() [4][]string {
	set := [4]string{"", g.settings.Launcher(), g.settings.Terminal(), g.settings.Clipboard()}
	var own [4][]string
	for r, s := range g.shortcutRows() {
		if r == 0 {
			continue
		}
		for _, k := range []string{*s.key, set[r]} {
			if k != "" && !slices.Contains(own[r], k) {
				own[r] = append(own[r], k)
			}
		}
	}
	return own
}

// shortcutOwner is who but self has accel: one of Gocast's own shortcuts
// (own, by row), else an app or preset of st, however each is spelled.
func shortcutOwner(accel string, own [4][]string, st settings.Settings, self settings.Hotkey) (hotkeyOwner, bool) {
	for r, keys := range own {
		if slices.ContainsFunc(keys, func(k string) bool { return shortcut.Same(accel, k) }) {
			return hotkeyOwner{row: r}, true
		}
	}
	if h, ok := st.HotkeyOwner(accel, self); ok {
		return hotkeyOwner{item: h}, true
	}
	return hotkeyOwner{}, false
}

// checkHotkey is accel normalized ("Cmd+Alt+T" as "Alt+Cmd+T"), when it
// may be h's hotkey: one the recorder takes, which no other has.
func checkHotkey(h settings.Hotkey, accel string, own [4][]string, st settings.Settings) (string, error) {
	n, ok := shortcut.Normalize(accel)
	if !ok || !shortcut.Valid(n) {
		return "", errHotkeyInvalid
	}
	if o, ok := shortcutOwner(n, own, st, h); ok {
		return "", &hotkeyConflictError{o}
	}
	return n, nil
}

// --- Registering ---

// registerHotkeys registers the hotkeys of the settings, after Gocast's
// own, and notes how each stands.
func (g *gocast) registerHotkeys() {
	g.prunePresetHotkeys()
	g.hotkeys = map[settings.Hotkey]string{}
	g.hotkeyStates = map[settings.Hotkey]hotkeyState{}
	for _, h := range g.settings.Hotkeys() {
		g.hotkeyStates[h] = g.registerHotkey(h)
	}
}

// registerHotkey registers the hotkey of h as the settings have it,
// unless the app is not installed or one registered before has it, and
// says how it stands.
func (g *gocast) registerHotkey(h settings.Hotkey) hotkeyState {
	n, ok := shortcut.Normalize(g.settings.Hotkey(h))
	if !ok || !shortcut.Valid(n) {
		log.Printf("hotkey %v: %q is not a shortcut", h, g.settings.Hotkey(h))
		return hotkeyInvalid
	}
	if h.App != "" && running.AppPath(h.App) == "" {
		return hotkeyMissing
	}
	if _, ok := shortcutOwner(n, g.ownShortcuts(), settings.Settings{}, h); ok || g.registeredOwner(n, h) {
		log.Printf("hotkey %v: %s is another's", h, n)
		return hotkeyConflict
	}
	if err := globalShortcuts.Register(n, g.hotkeyFunc(h)); err != nil {
		log.Printf("hotkey %v: %s: %v", h, n, err)
		return hotkeyTaken
	}
	g.hotkeys[h] = n
	return hotkeyOn
}

// registeredOwner reports whether a hotkey registered but self's is
// accel.
func (g *gocast) registeredOwner(accel string, self settings.Hotkey) bool {
	for h, k := range g.hotkeys {
		if h != self && shortcut.Same(accel, k) {
			return true
		}
	}
	return false
}

// hotkeyFunc is what the hotkey of h does.
func (g *gocast) hotkeyFunc(h settings.Hotkey) func() {
	if h.App != "" {
		return func() { g.appHotkeyPressed(h.App) }
	}
	return func() { g.presetHotkeyPressed(h.Preset) }
}

// unregisterHotkey lets go of the hotkey of h, if registered.
func (g *gocast) unregisterHotkey(h settings.Hotkey) {
	if k, ok := g.hotkeys[h]; ok {
		if !g.suspended { // suspended, it is not registered now
			globalShortcuts.Unregister(k)
		}
		delete(g.hotkeys, h)
	}
}

// suspendHotkeys and resumeHotkeys are suspendShortcuts and
// resumeShortcuts for the hotkeys of apps and presets.
func (g *gocast) suspendHotkeys() {
	if g.suspended {
		return
	}
	for _, k := range g.hotkeys {
		globalShortcuts.Unregister(k)
	}
	g.suspended = true
}

func (g *gocast) resumeHotkeys() {
	if !g.suspended {
		return
	}
	g.suspended = false
	for h, k := range g.hotkeys {
		if err := globalShortcuts.Register(k, g.hotkeyFunc(h)); err != nil {
			// Another app took it meanwhile, or ⌥⇧Space came back for
			// ⌘Space (keepFallback).
			log.Printf("hotkey %v: %s: %v", h, k, err)
			delete(g.hotkeys, h)
			g.hotkeyStates[h] = hotkeyTaken
		}
	}
}

// setHotkey makes accel h's hotkey: see setAppHotkey. It resumes the
// shortcuts if a recorder suspended them, unless accel is refused before
// it is tried (errHotkeyInvalid, a conflict): the recorder goes on then.
func (g *gocast) setHotkey(h settings.Hotkey, accel string) error {
	n, err := checkHotkey(h, accel, g.ownShortcuts(), g.settings)
	if err != nil {
		return err
	}
	if g.suspended {
		g.resumeShortcuts()
	}
	old, had := g.hotkeys[h]
	if had && old == n {
		if h.App != "" && g.settings.AppHotkeyName(h.App) == "" { // set by an older build
			g.storeHotkeyName(h)
			g.saveSettings()
		}
		return nil
	}
	installed := h.App == "" || running.AppPath(h.App) != ""
	g.unregisterHotkey(h)
	state := hotkeyMissing
	if installed {
		if err := globalShortcuts.Register(n, g.hotkeyFunc(h)); err != nil {
			log.Printf("hotkey %v: %s: %v", h, n, err)
			g.reregister(h, old, had) // the old one back
			return errShortcutTaken
		}
		g.hotkeys[h] = n
		state = hotkeyOn
	}
	g.hotkeyStates[h] = state
	g.settings.SetHotkey(h, n)
	g.storeHotkeyName(h)
	g.saveSettings()
	return nil
}

// moveHotkey gives to the hotkey of from, another app's or preset's (not
// Gocast's own, which can't be taken), in one step: from loses it, and
// to's own, if it had one, is let go. When the system refuses it for to
// (errShortcutTaken), both keep what they had.
func (g *gocast) moveHotkey(from, to settings.Hotkey) error {
	accel, ok := shortcut.Normalize(g.settings.Hotkey(from))
	switch {
	case !ok || from == to:
		return errHotkeyInvalid
	case to.Preset != "" && presets.Find(g.presets, to.Preset) < 0:
		return errNoPreset
	}
	if g.suspended {
		g.resumeShortcuts()
	}
	fromKey, fromOn := g.hotkeys[from]
	toKey, toOn := g.hotkeys[to]
	g.unregisterHotkey(from)
	g.unregisterHotkey(to)
	state := hotkeyMissing
	if to.App == "" || running.AppPath(to.App) != "" {
		if err := globalShortcuts.Register(accel, g.hotkeyFunc(to)); err != nil {
			log.Printf("hotkey %v: %s: %v", to, accel, err)
			g.reregister(from, fromKey, fromOn)
			g.reregister(to, toKey, toOn)
			return errShortcutTaken
		}
		g.hotkeys[to] = accel
		state = hotkeyOn
	}
	g.hotkeyStates[to] = state
	delete(g.hotkeyStates, from)
	g.settings.SetHotkey(to, accel)
	g.storeHotkeyName(to)
	g.settings.RemoveHotkey(from)
	g.saveSettings()
	return nil
}

// takeHotkey gives one of Gocast's own shortcuts, by its row (1 to 3),
// the hotkey of from, an app's or preset's, which loses it (design.md
// 4.4: Use Here). When the system refuses it, both keep what they had
// (errShortcutTaken).
func (g *gocast) takeHotkey(row int, from settings.Hotkey) error {
	accel, ok := shortcut.Normalize(g.settings.Hotkey(from))
	if !ok || row < 1 || row > 3 {
		return errHotkeyInvalid
	}
	key, on := g.hotkeys[from]
	g.suspendShortcuts()
	delete(g.hotkeys, from) // suspended: not registered now
	if err := g.setShortcut(row, accel); err != nil {
		g.reregister(from, key, on)
		return err
	}
	delete(g.hotkeyStates, from)
	g.settings.RemoveHotkey(from)
	g.saveSettings()
	return nil
}

// reregister registers again the hotkey of h that was registered as key
// (on), as a change that failed leaves it; taken if the system now
// refuses it.
func (g *gocast) reregister(h settings.Hotkey, key string, on bool) {
	if !on {
		return
	}
	if err := globalShortcuts.Register(key, g.hotkeyFunc(h)); err != nil {
		g.hotkeyStates[h] = hotkeyTaken
		return
	}
	g.hotkeys[h] = key
}

// removeHotkey removes the hotkey of h, and saves it.
func (g *gocast) removeHotkey(h settings.Hotkey) {
	g.unregisterHotkey(h)
	delete(g.hotkeyStates, h)
	g.settings.RemoveHotkey(h)
	g.saveSettings()
}

// prunePresetHotkeys removes the hotkeys of the presets deleted, unless
// presets.json could not be read, which leaves no presets for now.
func (g *gocast) prunePresetHotkeys() {
	if g.presetsErr != nil {
		return
	}
	exists := func(id string) bool { return presets.Find(g.presets, id) >= 0 }
	if !g.settings.PrunePresetHotkeys(exists) {
		return
	}
	for h := range g.hotkeyStates {
		if h.Preset != "" && !exists(h.Preset) {
			g.unregisterHotkey(h)
			delete(g.hotkeyStates, h)
		}
	}
	g.saveSettings()
}

// refreshAppHotkeys registers the hotkeys of apps installed again, and,
// with all, lets go of those of apps gone and tries again the ones the
// system refused (the user may have quit Raycast). A Launch Services
// lookup each: cheap enough for the search panel to call as it shows
// (only the apps missing), and Settings (all).
func (g *gocast) refreshAppHotkeys(all bool) {
	if g.suspended {
		return
	}
	for h, st := range g.hotkeyStates {
		switch {
		case st == hotkeyMissing && running.AppPath(h.App) != "",
			all && st == hotkeyTaken:
			g.hotkeyStates[h] = g.registerHotkey(h)
		case all && st == hotkeyOn && h.App != "" && running.AppPath(h.App) == "":
			g.unregisterHotkey(h)
			g.hotkeyStates[h] = hotkeyMissing
		}
	}
}

// --- Pressing ---

// appAction is what an app's hotkey does.
type appAction int

const (
	appOpen appAction = iota // launch it, or bring it forward
	appHide                  // hide it, as Command-H
)

// appHotkeyAction decides what the hotkey of the app with bundleID does:
// hide it when it is the frontmost app and not hidden, with no panel of
// Gocast's over it; else open it (design brief: a toggle, as ⌃` is).
func appHotkeyAction(bundleID, frontmost string, hidden, panelOpen bool) appAction {
	if !panelOpen && !hidden && frontmost != "" && strings.EqualFold(bundleID, frontmost) {
		return appHide
	}
	return appOpen
}

// appHotkeyPressed opens the app with bundleID, or hides it when it is
// frontmost; the search panel or clipboard history closes first. It
// needs no Accessibility permission: NSWorkspace and NSRunningApplication
// only.
func (g *gocast) appHotkeyPressed(bundleID string) {
	panel := g.launcher.win.IsVisible()
	if panel {
		g.launcher.hide()
	}
	front, hidden := running.Frontmost()
	if appHotkeyAction(bundleID, front, hidden, panel) == appHide {
		running.Hide(bundleID)
		return
	}
	if err := running.Open(bundleID); err != nil {
		log.Printf("hotkey: opening %s: %v", bundleID, err)
		if errors.Is(err, running.ErrNotInstalled) { // uninstalled meanwhile
			h := settings.Hotkey{App: bundleID}
			g.unregisterHotkey(h)
			g.hotkeyStates[h] = hotkeyMissing
		}
		return
	}
	g.record("app:"+bundleID, "") // as opening it from the search panel does
}

// presetHotkeyAction decides what the hotkey of the preset with the ID
// does, for the floating terminal shown or not, on its tab active, its
// tabs running the presets of tabs ("" for a plain one): hide it when it
// shows a tab of the preset (as ⌃` would); else show the tab to show,
// the active one if it runs the preset, else the first that does, or -1
// for a new tab.
func presetHotkeyAction(shown bool, active int, tabs []string, id string) (hide bool, tab int) {
	if active >= 0 && active < len(tabs) && tabs[active] == id {
		return shown, active
	}
	return false, slices.Index(tabs, id)
}

// presetHotkeyPressed shows the floating terminal on the preset's tab,
// opening one with it the first time, or hides the terminal showing it.
func (g *gocast) presetHotkeyPressed(id string) {
	if g.launcher.win.IsVisible() {
		g.launcher.hide()
	}
	if g.loadPresets() { // edited by hand meanwhile
		g.presetsChanged()
	}
	t := g.term
	tabs := make([]string, len(t.tabs))
	for i, s := range t.tabs {
		tabs[i] = s.presetID
	}
	hide, tab := presetHotkeyAction(t.shown, t.active, tabs, id)
	switch {
	case hide:
		t.hide()
		return
	case tab >= 0:
		t.activate(tab)
		t.show()
	case presets.Find(g.presets, id) < 0:
		return // deleted: its hotkey goes with it
	default:
		t.openPreset(id, true)
	}
	g.record("preset:"+id, "")
}
