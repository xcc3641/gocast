package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/usage"
	"gocast/locales"
)

// hotkeyPanel is the search panel with Terminal, Safari, Finder and
// Notes, and a preset, drawn by a tester, the global shortcuts a fake:
// Gocast's own registered, Terminal's hotkey ⌥⌘T.
func hotkeyPanel(t *testing.T, locale string) (*launcher, *ui.Tester, *fakeShortcuts) {
	t.Helper()
	inLocale(t, locale)
	f := useFakeShortcuts(t)
	u := usage.New()
	g := &gocast{dir: t.TempDir(), usage: u, presets: []presets.Preset{{ID: "p", Name: "gocast", Dir: "~/Projects/gocast", Command: "go test"}}}
	l := &launcher{g: g, icons: map[string]*ui.Bitmap{}, filtered: noQuery}
	g.launcher = l
	zh := strings.HasPrefix(locale, "zh")
	name := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	for _, a := range []mygo.InstalledApp{
		{Name: name("Terminal", "终端"), ID: "com.apple.Terminal", Path: "/System/Applications/Utilities/Terminal.app"},
		{Name: name("Safari", "Safari 浏览器"), ID: "com.apple.Safari", Path: "/Applications/Safari.app"},
		{Name: name("Finder", "访达"), ID: "com.apple.finder", Path: "/System/Library/CoreServices/Finder.app"},
		{Name: name("Notes", "备忘录"), ID: "com.apple.Notes", Path: "/System/Applications/Notes.app"},
	} {
		l.apps = append(l.apps, a)
		l.alts = append(l.alts, nil)
		l.places = append(l.places, "")
		l.icons[a.Path] = nil // drawn without: icons need the app running
	}
	g.launcherKey, g.terminalKey, g.clipKey = "Cmd+Space", "Ctrl+`", "Ctrl+Cmd+V"
	for _, k := range []string{g.launcherKey, g.terminalKey, g.clipKey} {
		f.on[k] = true
	}
	g.settings.SetHotkey(settings.Hotkey{App: "com.apple.Terminal"}, "Alt+Cmd+T")
	g.registerHotkeys()
	l.clip = newClipView(l)
	g.clip = &clipService{g: g, noAX: true}
	// Opened most: Terminal, Safari, Finder, Notes.
	for i, id := range []string{"app:com.apple.Terminal", "app:com.apple.Safari", "app:com.apple.finder", "app:com.apple.Notes"} {
		for range 8 - 2*i {
			u.Record(id, "", clipNow)
		}
	}
	l.rebuild()
	tt := ui.NewTester(l.view, windowWidth, windowHeight)
	l.enterSearch(clipNow)
	tt.Frame()
	return l, tt, f
}

// has reports whether the frame has the text s, whole.
func has(tt *ui.Tester, s string) bool { return slices.Contains(tt.Texts(), s) }

// chooseRow chooses the row of the item with the ID.
func chooseRow(t *testing.T, l *launcher, id string) {
	t.Helper()
	for i := range l.rows {
		if r := l.rows[i]; r.title == "" && !r.calc && r.own < 0 && l.items[r.item].ID == id {
			l.choose(i)
			return
		}
	}
	t.Fatalf("no row of %s in %q", id, shown(l))
}

// menuLabels are the labels of the Cmd-K menu's actions.
func menuLabels(l *launcher) []string {
	var out []string
	for _, a := range l.actions() {
		out = append(out, a.label)
	}
	return out
}

func TestHotkeyMenuItems(t *testing.T) {
	l, _, _ := hotkeyPanel(t, "en")
	chooseRow(t, l, "app:com.apple.Terminal") // running, with a hotkey
	// Then Quit, if Terminal runs as the test does.
	want := []string{"Open", "Show in Finder", "Copy Path", "Change Shortcut…", "Remove Shortcut"}
	if got := menuLabels(l); !slices.Equal(got[:min(5, len(got))], want) {
		t.Errorf("Terminal: %q, want %q", got, want)
	}
	chooseRow(t, l, "app:com.apple.Safari")
	if got := menuLabels(l); !slices.Equal(got, []string{"Open", "Show in Finder", "Copy Path", "Set Shortcut…"}) {
		t.Errorf("Safari: %q", got)
	}
	typeQuery(l, "gocast")
	chooseRow(t, l, "preset:p")
	if got := menuLabels(l); !slices.Equal(got, []string{locales.ActionOpenInTerminal(), locales.ActionNewTab(), locales.ActionEditPreset(), "Set Shortcut…", locales.ActionShowFolder()}) {
		t.Errorf("preset: %q", got)
	}
	// Gocast's own rows: none.
	typeQuery(l, "clipboard")
	if got := menuLabels(l); slices.Contains(got, "Set Shortcut…") {
		t.Errorf("clipboard history: %q", got)
	}
}

// runAction runs the Cmd-K menu's action of the label.
func runAction(t *testing.T, l *launcher, label string) {
	t.Helper()
	l.toggleMenu()
	for i, ai := range l.menuShown {
		if l.menuItems[ai].label == label {
			l.runMenu(i)
			return
		}
	}
	t.Fatalf("no %q in %q", label, menuLabels(l))
}

func TestHotkeyRecorder(t *testing.T) {
	l, tt, f := hotkeyPanel(t, "en")
	safari := settings.Hotkey{App: "com.apple.Safari"}
	chooseRow(t, l, "app:com.apple.Safari")
	runAction(t, l, "Set Shortcut…")
	tt.Frame()
	if !l.rec.on || !has(tt, "Press the new shortcut") || !l.g.suspended || f.on["Cmd+Space"] {
		t.Fatalf("recording: %v, %q, suspended %v, %v", l.rec.on, tt.Texts(), l.g.suspended, f.on)
	}
	if !has(tt, "Safari") || !has(tt, "Press esc to cancel") {
		t.Errorf("card: %q", tt.Texts())
	}

	// Shift alone: refused, recording on.
	tt.Key(ui.Shift, ui.KeyS)
	tt.Frame()
	if !l.rec.on || !has(tt, locales.HotkeyNeedsModifier()) {
		t.Fatalf("Shift+S: %q", tt.Texts())
	}
	// Gocast's own: refused, says which.
	tt.Key(ui.Ctrl|ui.Cmd, ui.KeyV)
	tt.Frame()
	if want := locales.ShortcutUsedFor(locales.ShortcutClipboard()); !l.rec.on || !has(tt, want) {
		t.Fatalf("⌃⌘V: %q", tt.Texts())
	}
	// Another app's.
	tt.Key(ui.Alt|ui.Cmd, ui.KeyT)
	tt.Frame()
	if want := locales.ShortcutUsedFor("Terminal"); !has(tt, want) {
		t.Fatalf("⌥⌘T: %q", tt.Texts())
	}
	// Refused by the system: the shortcuts stay off.
	f.refuse["Alt+Cmd+X"] = true
	tt.Key(ui.Alt|ui.Cmd, ui.KeyX)
	tt.Frame()
	if !has(tt, locales.HotkeyInUse()) || !l.g.suspended || f.on["Cmd+Space"] {
		t.Fatalf("taken: %q, suspended %v", tt.Texts(), l.g.suspended)
	}
	// Cmd-K is a shortcut like another: saved, the card closed, the panel
	// on, the shortcuts back.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if l.rec.on || l.menuOpen || l.g.settings.Hotkey(safari) != "Cmd+K" || !f.on["Cmd+K"] || !f.on["Cmd+Space"] || l.g.suspended {
		t.Fatalf("⌘K: recording %v, menu %v, %q, %v", l.rec.on, l.menuOpen, l.g.settings.Hotkey(safari), f.on)
	}
	if !has(tt, locales.HotkeySetTo("⌘K")) {
		t.Errorf("no notice: %q", tt.Texts())
	}
	// The row chosen shows its keys; its label says them.
	if !has(tt, "⌘") || !has(tt, "K") {
		t.Errorf("no keycaps: %q", tt.Texts())
	}
	if got := l.rowLabel(l.chosen); got != "Safari, App, shortcut Command K" {
		t.Errorf("label %q", got)
	}

	// Escape cancels, back to the list, not the menu.
	runAction(t, l, "Change Shortcut…")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if l.rec.on || l.menuOpen || l.g.suspended || l.g.settings.Hotkey(safari) != "Cmd+K" {
		t.Errorf("escape: recording %v, menu %v", l.rec.on, l.menuOpen)
	}

	// Remove: at once, with a notice.
	runAction(t, l, "Remove Shortcut")
	tt.Frame()
	if l.g.settings.Hotkey(safari) != "" || f.on["Cmd+K"] || !has(tt, locales.HotkeyRemoved()) {
		t.Errorf("removed: %q, %v, %q", l.g.settings.Hotkey(safari), f.on, tt.Texts())
	}
}

// Modifiers held show as they are, before the key comes.
func TestHotkeyRecorderModifiers(t *testing.T) {
	l, tt, _ := hotkeyPanel(t, "en")
	chooseRow(t, l, "app:com.apple.Safari")
	runAction(t, l, "Set Shortcut…")
	tt.Frame()
	tt.CodeDown(ui.Alt, ui.CodeAltLeft)
	tt.CodeDown(ui.Alt|ui.Cmd, ui.CodeSuperLeft)
	tt.Frame()
	if !has(tt, "⌥") || !has(tt, "⌘") || has(tt, "Press the new shortcut") {
		t.Errorf("held: %q", tt.Texts())
	}
	tt.CodeUp(ui.Alt, ui.CodeSuperLeft)
	tt.CodeUp(0, ui.CodeAltLeft)
	tt.Frame()
	if !has(tt, "Press the new shortcut") {
		t.Errorf("let go: %q", tt.Texts())
	}
	// Hiding the panel voids the recording.
	l.leave(clipNow)
	if l.rec.on || l.g.suspended {
		t.Error("still recording, hidden")
	}
}

// The labels of rows with a hotkey say it, chosen or not (copy.md 11.4).
func TestHotkeyRowLabels(t *testing.T) {
	l, _, _ := hotkeyPanel(t, "en")
	l.g.settings.SetHotkey(settings.Hotkey{Preset: "p"}, "Ctrl+Alt+1")
	typeQuery(l, "gocast")
	for i := range l.rows {
		if r := l.rows[i]; r.own < 0 && !r.calc && r.title == "" && l.items[r.item].ID == "preset:p" {
			if got := l.rowLabel(i); got != "gocast, Preset, ~/Projects/gocast, shortcut Control Option 1" {
				t.Errorf("preset: %q", got)
			}
		}
	}
	typeQuery(l, "")
	for i := range l.rows {
		if r := l.rows[i]; r.own < 0 && !r.calc && r.title == "" && l.items[r.item].ID == "app:com.apple.Terminal" {
			if got := l.rowLabel(i); got != "Terminal, App, shortcut Option Command T" {
				t.Errorf("Terminal: %q", got)
			}
		}
	}
}

// --- Screenshots ---

// screensDir is where TestScreens writes, GOCAST_SCREENS; unset, it
// does not run.
func screensDir(t *testing.T) string {
	dir := os.Getenv("GOCAST_SCREENS")
	if dir == "" {
		t.Skip("GOCAST_SCREENS not set")
	}
	return dir
}

// saveScreen writes the part r of the frame, at 2×, as name.png.
func saveScreen(t *testing.T, tt *ui.Tester, dir, name string, r image.Rectangle) {
	t.Helper()
	tt.Frame()
	tt.Advance(time.Second) // past the transitions
	img := tt.Image()
	r = image.Rect(r.Min.X*2, r.Min.Y*2, r.Max.X*2, r.Max.Y*2).Intersect(img.Bounds())
	file, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img.SubImage(r)); err != nil {
		t.Fatal(err)
	}
}

// panelRect is the search panel in its window, expanded.
var panelRect = image.Rect(searchLeft, marginTop, searchLeft+panelWidth, marginTop+expandedHeight)

func TestScreensSearchPanel(t *testing.T) {
	dir := screensDir(t)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		shot := func(name string, setup func(l *launcher, tt *ui.Tester, f *fakeShortcuts)) {
			l, tt, f := hotkeyPanel(t, "zh-Hans")
			tt.SetScale(2)
			tt.SetDark(dark)
			chooseRow(t, l, "app:com.apple.Terminal")
			tt.Frame()
			setup(l, tt, f)
			saveScreen(t, tt, dir, name+"-"+mode, panelRect)
		}
		shot("hk-row", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {})
		shot("hk-menu", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) { l.toggleMenu() })
		shot("hk-menu-unset", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			chooseRow(t, l, "app:com.apple.Safari")
			l.toggleMenu()
		})
		shot("hk-record", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			runAction(t, l, locales.ActionChangeShortcut())
		})
		shot("hk-record-new", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			chooseRow(t, l, "app:com.apple.Safari")
			runAction(t, l, locales.ActionSetShortcut())
		})
		shot("hk-record-mods", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			chooseRow(t, l, "app:com.apple.Safari")
			runAction(t, l, locales.ActionSetShortcut())
			tt.Frame()
			tt.CodeDown(ui.Alt, ui.CodeAltLeft)
			tt.CodeDown(ui.Alt|ui.Cmd, ui.CodeSuperLeft)
		})
		shot("hk-conflict", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			runAction(t, l, locales.ActionChangeShortcut())
			tt.Frame()
			tt.Key(ui.Ctrl|ui.Cmd, ui.KeyV)
		})
		shot("hk-modifier", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			runAction(t, l, locales.ActionChangeShortcut())
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyS)
		})
		shot("hk-inuse", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			runAction(t, l, locales.ActionChangeShortcut())
			tt.Frame()
			f.refuse["Alt+Cmd+S"] = true
			tt.Key(ui.Alt|ui.Cmd, ui.KeyS)
		})
		shot("hk-saved", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			chooseRow(t, l, "app:com.apple.Safari")
			runAction(t, l, locales.ActionSetShortcut())
			tt.Frame()
			tt.Key(ui.Alt|ui.Cmd, ui.KeyS)
		})
		shot("hk-removed", func(l *launcher, tt *ui.Tester, f *fakeShortcuts) {
			runAction(t, l, locales.ActionRemoveShortcut())
		})
	}
}
