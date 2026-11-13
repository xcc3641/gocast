package app

import (
	"bytes"
	"encoding/binary"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	plistlib "howett.net/plist"

	"gocast/internal/settings"
	"gocast/locales"
)

// hotkeyPrefs is Settings on its Shortcuts page, over the search panel of
// hotkeyPanel (Terminal's hotkey ⌥⌘T), drawn by a tester.
func hotkeyPrefs(t *testing.T, locale string) (*prefsWindow, *ui.Tester, *fakeShortcuts) {
	t.Helper()
	l, _, f := hotkeyPanel(t, locale)
	g := l.g
	p := &prefsWindow{g: g, page: pageShortcuts}
	g.prefs = p
	for _, a := range l.apps {
		l.clip.icons[a.Path+"\x00"+string(rune(hkIcon))] = nil // drawn without: icons need the app running
	}
	l.clip.icons[genericAppIconKey(hkIcon)] = nil
	p.enterShortcuts()
	tt := ui.NewTester(p.view, prefsWidth, prefsHeight)
	tt.Frame()
	return p, tt, f
}

// addGone gives an app not installed a hotkey, ⌃⌥G, with the name it
// had when it was set, 微信 (a bundle ID no Mac has, for the tests not to
// depend on the apps installed).
func addGone(p *prefsWindow) {
	h := settings.Hotkey{App: "com.example.Gone"}
	p.g.settings.SetHotkey(h, "Ctrl+Alt+G")
	p.g.settings.SetAppHotkeyName(h.App, "微信")
	p.g.hotkeyStates[h] = p.g.registerHotkey(h)
}

// shownIDs are the IDs of the rows the list shows.
func shownIDs(p *prefsWindow) []string {
	var ids []string
	for _, i := range p.hk.shown {
		ids = append(ids, p.hk.rows[i].id)
	}
	return ids
}

// rowIndex is where the row with the ID shows, failing if it does not.
func rowIndex(t *testing.T, p *prefsWindow, id string) int {
	t.Helper()
	i := slices.Index(shownIDs(p), id)
	if i < 0 {
		t.Fatalf("no row %s in %q", id, shownIDs(p))
	}
	return i
}

func TestHotkeyPrefsRows(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "en")
	want := []string{"own:1", "own:2", "own:3", "app:com.apple.finder", "preset:p", "app:com.apple.Notes", "app:com.apple.Safari", "app:com.apple.Terminal"}
	if got := shownIDs(p); !slices.Equal(got, want) {
		t.Fatalf("rows:\n%q\nwant\n%q", got, want)
	}
	for _, s := range []string{locales.ShortcutLauncher(), "Terminal", locales.HotkeysSetShortcut(), locales.HotkeysIntro()} {
		if !has(tt, s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	if p.hk.chosen != 0 || !tt.Focused(locales.HotkeysSearch()) {
		t.Errorf("chosen %d, search focused %v", p.hk.chosen, tt.Focused(locales.HotkeysSearch()))
	}

	// Searching: by name, Gocast's own by their labels; the first chosen.
	tt.Type("saf")
	tt.Frame()
	if got := shownIDs(p); !slices.Equal(got, []string{"app:com.apple.Safari"}) {
		t.Errorf("searching saf: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("zzz")
	tt.Frame()
	if len(p.hk.shown) != 0 || !has(tt, locales.HotkeysNoMatch()) {
		t.Errorf("no match: %q %q", shownIDs(p), tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if p.hk.query != "" || len(p.hk.shown) != len(want) {
		t.Errorf("Escape: %q, %d rows", p.hk.query, len(p.hk.shown))
	}

	// The filters.
	for f, ids := range map[int][]string{
		hkFilterSet:     {"own:1", "own:2", "own:3", "app:com.apple.Terminal"},
		hkFilterApps:    {"app:com.apple.finder", "app:com.apple.Notes", "app:com.apple.Safari", "app:com.apple.Terminal"},
		hkFilterPresets: {"preset:p"},
	} {
		p.hk.filter = f
		p.filterHotkeys(true)
		if got := shownIDs(p); !slices.Equal(got, ids) {
			t.Errorf("filter %d: %q, want %q", f, got, ids)
		}
	}
}

// With Shortcut: a row cleared stays until the filter or the query
// changes; with none set, a line says so under Gocast's own.
func TestHotkeyPrefsWithShortcut(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "en")
	p.hk.filter = hkFilterSet
	p.filterHotkeys(true)
	tt.Frame()
	if p.hk.none || has(tt, locales.HotkeysNoneSet()) {
		t.Fatal("none set, with Terminal's")
	}
	i := rowIndex(t, p, "app:com.apple.Terminal")
	p.hk.list.Choose(i)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyBackspace)
	tt.Frame()
	if p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Terminal"}) != "" {
		t.Fatal("Cmd-Backspace did not clear it")
	}
	if !slices.Contains(shownIDs(p), "app:com.apple.Terminal") {
		t.Error("the row cleared went at once")
	}
	p.filterHotkeys(true)
	tt.Frame()
	if slices.Contains(shownIDs(p), "app:com.apple.Terminal") || !p.hk.none || !has(tt, locales.HotkeysNoneSet()) {
		t.Errorf("after filtering again: %q, none %v, %q", shownIDs(p), p.hk.none, tt.Texts())
	}
}

func TestHotkeyPrefsRecording(t *testing.T) {
	p, tt, f := hotkeyPrefs(t, "en")
	safari := settings.Hotkey{App: "com.apple.Safari"}
	terminal := settings.Hotkey{App: "com.apple.Terminal"}
	record := func(id string) {
		t.Helper()
		p.startRecording(rowIndex(t, p, id))
		tt.Frame()
		tt.Frame()
		if !p.isRecording() || !p.g.suspended {
			t.Fatalf("not recording %s", id)
		}
	}

	// Return on the row chosen, from the search field, records it.
	p.hk.list.Choose(rowIndex(t, p, "app:com.apple.Safari"))
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.hk.rec.id != "app:com.apple.Safari" || !has(tt, locales.ShortcutRecordingHint()) || !has(tt, locales.ShortcutRecording()) {
		t.Fatalf("Return: recording %q, %q", p.hk.rec.id, tt.Texts())
	}
	if f.on["Cmd+Space"] || f.on["Alt+Cmd+T"] {
		t.Error("the global shortcuts are on while recording")
	}

	// Shift and a key alone: why, recording on.
	tt.Key(ui.Shift, ui.KeyS)
	tt.Frame()
	if p.hk.rec.problem != locales.HotkeyNeedsModifier() || !has(tt, locales.HotkeyNeedsModifier()) {
		t.Errorf("Shift-S: %q", p.hk.rec.problem)
	}

	// Gocast's own: refused, no Use Here.
	tt.Key(ui.Ctrl|ui.Cmd, ui.KeyV)
	tt.Frame()
	if want := locales.HotkeysUsedByGocast(locales.ShortcutClipboard()); p.hk.rec.problem != want || p.hk.rec.movable || has(tt, locales.HotkeysUseHere()) {
		t.Errorf("⌃⌘V: %q, movable %v", p.hk.rec.problem, p.hk.rec.movable)
	}

	// Another app's: Use Here, which Return presses.
	tt.Key(ui.Alt|ui.Cmd, ui.KeyT)
	tt.Frame()
	if want := locales.ShortcutUsedFor("Terminal"); p.hk.rec.problem != want || !p.hk.rec.movable || !has(tt, locales.HotkeysUseHere()) {
		t.Fatalf("⌥⌘T: %q, movable %v, %q", p.hk.rec.problem, p.hk.rec.movable, tt.Texts())
	}
	if label := p.hk.list.Label(rowIndex(t, p, "app:com.apple.Safari")); !strings.Contains(label, "Terminal") {
		t.Errorf("label in conflict: %q", label)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.isRecording() || p.g.suspended {
		t.Fatal("still recording after Use Here")
	}
	if p.g.settings.Hotkey(safari) != "Alt+Cmd+T" || p.g.settings.Hotkey(terminal) != "" || !f.on["Alt+Cmd+T"] || !f.on["Cmd+Space"] {
		t.Errorf("after Use Here: Safari %q, Terminal %q, %v", p.g.settings.Hotkey(safari), p.g.settings.Hotkey(terminal), f.on)
	}

	// Refused by the system: the in-use line, recording on.
	record("app:com.apple.Notes")
	f.refuse["Alt+Cmd+N"] = true
	tt.Key(ui.Alt|ui.Cmd, ui.KeyN)
	tt.Frame()
	if p.hk.rec.problem != locales.HotkeyInUse() || !p.g.suspended {
		t.Errorf("in use: %q, suspended %v", p.hk.rec.problem, p.g.suspended)
	}

	// A valid one: saved at once, the shortcuts on again.
	tt.Key(ui.Ctrl|ui.Alt, ui.KeyN)
	tt.Frame()
	if p.isRecording() || p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Notes"}) != "Ctrl+Alt+N" || !f.on["Ctrl+Alt+N"] {
		t.Errorf("saved: recording %v, %q", p.isRecording(), p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Notes"}))
	}

	// Escape cancels, the old one kept; F13 alone is one.
	record("preset:p")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if p.isRecording() || p.g.suspended || p.g.settings.Hotkey(settings.Hotkey{Preset: "p"}) != "" {
		t.Error("Escape did not cancel")
	}
	record("preset:p")
	tt.CodeDown(0, ui.CodeF13)
	tt.Frame()
	if got := p.g.settings.Hotkey(settings.Hotkey{Preset: "p"}); got != "F13" {
		t.Errorf("F13: %q", got)
	}

	// Down is a key that needs a modifier; a click on another row
	// cancels.
	record("app:com.apple.finder")
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if p.hk.rec.id != "app:com.apple.finder" || p.hk.rec.problem != locales.HotkeyNeedsModifier() {
		t.Errorf("Down while recording: %q %q", p.hk.rec.id, p.hk.rec.problem)
	}
	if err := tt.Click("Notes"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if p.isRecording() || p.g.suspended {
		t.Errorf("a click on another row: recording %q", p.hk.rec.id)
	}
	if r := p.hkRowAt(p.hk.chosen); r == nil || r.id != "app:com.apple.Notes" {
		t.Errorf("chosen after the click: %v", r)
	}

	// A click on a recorder records its row; on the clear button, clears.
	if err := tt.Click(locales.HotkeysSetShortcut()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !p.isRecording() {
		t.Error("a click on Set Shortcut does not record")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	p.hk.list.Choose(rowIndex(t, p, "app:com.apple.Notes"))
	tt.Frame()
	if err := tt.Click(locales.HotkeysRemoveFor("Notes")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if p.isRecording() || p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Notes"}) != "" {
		t.Errorf("clear: recording %v, %q", p.isRecording(), p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Notes"}))
	}
}

// Gocast's own rows: Reset to Default, and Use Here from an app.
func TestHotkeyPrefsOwnRows(t *testing.T) {
	p, tt, f := hotkeyPrefs(t, "en")
	p.startRecording(rowIndex(t, p, "own:2"))
	tt.Frame()
	tt.Key(ui.Alt|ui.Cmd, ui.KeyT) // Terminal's
	tt.Frame()
	if !p.hk.rec.movable {
		t.Fatalf("own row in conflict with an app: %q", p.hk.rec.problem)
	}
	p.useHere()
	tt.Frame()
	if p.g.terminalKey != "Alt+Cmd+T" || p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Terminal"}) != "" || p.isRecording() {
		t.Fatalf("after Use Here: %q, %q", p.g.terminalKey, p.g.settings.Hotkey(settings.Hotkey{App: "com.apple.Terminal"}))
	}
	if _, ok := tt.Access(locales.ShortcutReset()); !ok {
		t.Error("no Reset to Default")
	}
	p.clearRow(p.hkRowAt(rowIndex(t, p, "own:2")))
	if p.g.terminalKey != settings.DefaultTerminalShortcut || !f.on[settings.DefaultTerminalShortcut] {
		t.Errorf("reset: %q", p.g.terminalKey)
	}
}

// What assistive technology reads of the rows (copy.md 11.4).
func TestHotkeyPrefsLabels(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "en")
	addGone(p)
	p.enterShortcuts()
	tt.Frame()
	label := func(id string) string { return p.hotkeyRowLabel(rowIndex(t, p, id)) }
	for id, want := range map[string]string{
		"app:com.apple.Terminal": "Terminal, App, shortcut Option Command T",
		"app:com.apple.Safari":   "Safari, App, no shortcut",
		"preset:p":               "gocast, Preset, no shortcut",
		"own:3":                  "Open clipboard history, Gocast, shortcut Control Command V",
		"app:com.example.Gone":   "微信, App, can’t find this app, shortcut Control Option G",
	} {
		if got := label(id); got != want {
			t.Errorf("%s: %q, want %q", id, got, want)
		}
	}
	p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
	if got := label("app:com.apple.Safari"); got != "Safari, press the new shortcut" {
		t.Errorf("recording: %q", got)
	}
	tt.Frame()
	tt.Key(ui.Alt|ui.Cmd, ui.KeyT)
	tt.Frame()
	if got := label("app:com.apple.Safari"); got != "Safari, this shortcut is already used for “Terminal”" {
		t.Errorf("conflict: %q", got)
	}
	if a := tt.Announcements(); len(a) == 0 || !strings.Contains(a[len(a)-1], "Terminal") {
		t.Errorf("announced %q", a)
	}
	if !has(tt, locales.HotkeysAppMissing()) {
		t.Errorf("the missing app's line: %q", tt.Texts())
	}
}

// An app's name is stored with its hotkey, set or moved there, for when
// the app is gone: what its row, its label and a conflict call it then.
// Only a hotkey of an older build, with none stored, shows the bundle ID.
func TestHotkeyPrefsStoredNames(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "en")
	g, l := p.g, p.g.launcher
	chat := settings.Hotkey{App: "com.example.Chat"} // no Mac has it: never installed for real
	safari := settings.Hotkey{App: "com.apple.Safari"}
	l.apps = append(l.apps, mygo.InstalledApp{Name: "微信", ID: chat.App, Path: "/Applications/Chat.app"})
	l.alts = append(l.alts, nil)
	l.places = append(l.places, "")

	if err := g.setHotkey(chat, "Alt+Cmd+W"); err != nil {
		t.Fatal(err)
	}
	if got := g.settings.AppHotkeyName(chat.App); got != "微信" {
		t.Fatalf("name stored on set: %q", got)
	}
	// Use Here: the name goes with the hotkey.
	if err := g.moveHotkey(chat, safari); err != nil {
		t.Fatal(err)
	}
	if a, b := g.settings.AppHotkeyName(chat.App), g.settings.AppHotkeyName(safari.App); a != "" || b != "Safari" {
		t.Fatalf("names after the move: %q, %q", a, b)
	}
	if err := g.moveHotkey(safari, chat); err != nil {
		t.Fatal(err)
	}
	if a, b := g.settings.AppHotkeyName(chat.App), g.settings.AppHotkeyName(safari.App); a != "微信" || b != "" {
		t.Fatalf("names after moving back: %q, %q", a, b)
	}

	// The app uninstalled: its row, label and conflicts use the name.
	l.apps, l.alts, l.places = l.apps[:len(l.apps)-1], l.alts[:len(l.alts)-1], l.places[:len(l.places)-1]
	p.enterShortcuts()
	tt.Frame()
	i := rowIndex(t, p, "app:com.example.Chat")
	if r := p.hkRowAt(i); r == nil || !r.missing || r.name != "微信" {
		t.Fatalf("the row of the app gone: %+v", r)
	}
	if got, want := p.hotkeyRowLabel(i), "微信, App, can’t find this app, shortcut Option Command W"; got != want {
		t.Errorf("label %q, want %q", got, want)
	}
	tt.Type("微信")
	tt.Frame()
	if got := shownIDs(p); !slices.Equal(got, []string{"app:com.example.Chat"}) {
		t.Errorf("searching its name: %q", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	p.startRecording(rowIndex(t, p, "app:com.apple.Notes"))
	tt.Frame()
	tt.Key(ui.Alt|ui.Cmd, ui.KeyW)
	tt.Frame()
	if want := locales.ShortcutUsedFor("微信"); p.hk.rec.problem != want {
		t.Errorf("conflict: %q, want %q", p.hk.rec.problem, want)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// Removed: the name too, the map left out of settings.json.
	g.removeHotkey(chat)
	if g.settings.AppHotkeyName(chat.App) != "" || g.settings.AppHotkeyNames != nil {
		t.Errorf("name kept after removing: %v", g.settings.AppHotkeyNames)
	}

	// Set by an older build: no name, the bundle ID.
	g.settings.SetHotkey(chat, "Alt+Cmd+W")
	if got := g.hotkeyLabel(chat); got != chat.App {
		t.Errorf("label with no name stored: %q", got)
	}
	// Set again as it was, the app installed: now stored.
	g.settings.SetHotkey(safari, "Alt+Cmd+S")
	g.hotkeyStates[safari] = g.registerHotkey(safari)
	if err := g.setHotkey(safari, "Alt+Cmd+S"); err != nil || g.settings.AppHotkeyName(safari.App) != "Safari" {
		t.Errorf("set again: %v, %q", err, g.settings.AppHotkeyName(safari.App))
	}
}

// --- Screenshots ---

func TestScreensSettingsShortcuts(t *testing.T) {
	dir := screensDir(t)
	full := image.Rect(0, 0, prefsWidth, prefsHeight)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		shot := func(name string, setup func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts)) {
			p, tt, f := hotkeyPrefs(t, "zh-Hans")
			screenIcons(t, p)
			addGone(p)
			p.enterShortcuts()
			tt.SetScale(2)
			tt.SetDark(dark)
			tt.Frame()
			setup(p, tt, f)
			saveScreen(t, tt, dir, name+"-"+mode, full)
		}
		chooseID := func(p *prefsWindow, tt *ui.Tester, id string) {
			p.hk.list.Choose(rowIndex(t, p, id))
			tt.Frame()
		}
		shot("st-list", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			chooseID(p, tt, "app:com.apple.Terminal")
		})
		shot("st-hover-unset", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			chooseID(p, tt, "app:com.apple.Safari")
		})
		shot("st-recording", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
		})
		shot("st-recording-mods", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
			tt.Frame()
			tt.CodeDown(ui.Alt, ui.CodeAltLeft)
			tt.CodeDown(ui.Alt|ui.Cmd, ui.CodeSuperLeft)
		})
		shot("st-conflict", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
			tt.Frame()
			tt.Key(ui.Alt|ui.Cmd, ui.KeyT)
		})
		shot("st-conflict-gocast", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
			tt.Frame()
			tt.Key(ui.Ctrl|ui.Cmd, ui.KeyV)
		})
		shot("st-modifier", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyS)
		})
		shot("st-inuse", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "app:com.apple.Safari"))
			tt.Frame()
			f.refuse["Alt+Cmd+S"] = true
			tt.Key(ui.Alt|ui.Cmd, ui.KeyS)
		})
		shot("st-with-shortcut", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.hk.filter = hkFilterSet
			p.filterHotkeys(true)
		})
		shot("st-none-set", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.g.removeHotkey(settings.Hotkey{App: "com.apple.Terminal"})
			p.g.removeHotkey(settings.Hotkey{App: "com.example.Gone"})
			p.hk.filter = hkFilterSet
			p.filterHotkeys(true)
		})
		shot("st-search", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			tt.Type("bwl")
		})
		shot("st-no-match", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			tt.Type("zzz")
		})
		shot("st-own-changed", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.startRecording(rowIndex(t, p, "own:2"))
			tt.Frame()
			tt.Key(ui.Alt|ui.Cmd, ui.KeyY)
		})
		shot("st-missing-app", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.hk.filter = hkFilterSet
			p.filterHotkeys(true)
			chooseID(p, tt, "app:com.example.Gone")
		})
		shot("st-spotlight", func(p *prefsWindow, tt *ui.Tester, f *fakeShortcuts) {
			p.g.spotlight = true
			p.g.launcherKey = settings.FallbackLauncherShortcut
			p.notice = noticeSpotlight
		})
	}
}

// screenIcons gives the screenshots the icons the tests draw without
// (they need the app running): the apps', and the generic one, from
// their .icns files.
func screenIcons(t *testing.T, p *prefsWindow) {
	t.Helper()
	icons := p.g.launcher.clip.icons
	for _, a := range p.g.launcher.apps {
		info, err := os.ReadFile(filepath.Join(a.Path, "Contents", "Info.plist"))
		if err != nil {
			continue
		}
		var plist map[string]any
		if _, err := plistlib.Unmarshal(info, &plist); err != nil {
			continue
		}
		name, _ := plist["CFBundleIconFile"].(string)
		if name == "" {
			continue
		}
		if filepath.Ext(name) == "" {
			name += ".icns"
		}
		icons[a.Path+"\x00"+string(rune(hkIcon))] = icnsBitmap(filepath.Join(a.Path, "Contents", "Resources", name))
	}
	icons[genericAppIconKey(hkIcon)] = icnsBitmap("/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/GenericApplicationIcon.icns")
}

// icnsBitmap is the smallest PNG of 64 pixels or more in the .icns file
// at path, or nil.
func icnsBitmap(path string) *ui.Bitmap {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 8 || string(data[:4]) != "icns" {
		return nil
	}
	chunks := map[string][]byte{}
	for i := 8; i+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[i+4 : i+8]))
		if n < 8 || i+n > len(data) {
			break
		}
		chunks[string(data[i:i+4])] = data[i+8 : i+n]
		i += n
	}
	for _, typ := range []string{"ic12", "ic07", "ic13", "ic08", "ic14", "ic09", "ic10"} {
		if png := chunks[typ]; bytes.HasPrefix(png, []byte("\x89PNG")) {
			if b, err := ui.DecodeBitmap(png); err == nil {
				return b
			}
		}
	}
	return nil
}
