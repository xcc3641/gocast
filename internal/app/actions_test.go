package app

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gocast/internal/clipboard"
	"gocast/internal/settings"
	"gocast/internal/sysact"
	"gocast/internal/usage"
	"gocast/locales"
)

// fakeSystem stands for macOS as the system actions ask of it: it notes
// what was asked, and fails with errs[the call].
type fakeSystem struct {
	calls  []string
	volume float64
	muted  bool
	empty  bool // the Trash
	hidden bool // Finder shows hidden files
	disks  int
	busy   []string
	errs   map[string]error
}

func (f *fakeSystem) do(name string) error {
	f.calls = append(f.calls, name)
	return f.errs[name]
}

func (f *fakeSystem) LockScreen() error       { return f.do("LockScreen") }
func (f *fakeSystem) Sleep() error            { return f.do("Sleep") }
func (f *fakeSystem) SleepDisplays() error    { return f.do("SleepDisplays") }
func (f *fakeSystem) StartScreenSaver() error { return f.do("StartScreenSaver") }
func (f *fakeSystem) Restart() error          { return f.do("Restart") }
func (f *fakeSystem) ShutDown() error         { return f.do("ShutDown") }
func (f *fakeSystem) LogOut() error           { return f.do("LogOut") }
func (f *fakeSystem) Volume() (float64, bool, error) {
	return f.volume, f.muted, f.errs["Volume"]
}

func (f *fakeSystem) SetVolume(v float64) error {
	if err := f.do(fmt.Sprintf("SetVolume %.4g", v)); err != nil {
		return err
	}
	f.volume = v
	return nil
}

func (f *fakeSystem) SetMuted(m bool) error {
	if err := f.do(fmt.Sprintf("SetMuted %v", m)); err != nil {
		return err
	}
	f.muted = m
	return nil
}

func (f *fakeSystem) ToggleAppearance() (bool, error) { return true, f.do("ToggleAppearance") }
func (f *fakeSystem) ShowDesktop() error              { return f.do("ShowDesktop") }
func (f *fakeSystem) HideOthers(pid int) error        { return f.do(fmt.Sprintf("HideOthers %d", pid)) }
func (f *fakeSystem) UnhideAll() error                { return f.do("UnhideAll") }
func (f *fakeSystem) QuitAll() error                  { return f.do("QuitAll") }
func (f *fakeSystem) OpenTrash() error                { return f.do("OpenTrash") }
func (f *fakeSystem) TrashEmpty() (bool, error)       { return f.empty, f.errs["TrashEmpty"] }
func (f *fakeSystem) EmptyTrash() error               { return f.do("EmptyTrash") }
func (f *fakeSystem) EjectAll() (int, []string, error) {
	return f.disks, f.busy, f.do("EjectAll")
}

func (f *fakeSystem) ToggleHiddenFiles() (bool, error) {
	if err := f.do("ToggleHiddenFiles"); err != nil {
		return f.hidden, err
	}
	f.hidden = !f.hidden
	return f.hidden, nil
}

// fakeWindows stands for the windows of other apps: one each by pid.
type fakeWindows struct {
	trusted bool
	screens []sysact.Screen
	wins    map[int]*fakeWindow
}

func (f *fakeWindows) Trusted() bool            { return f.trusted }
func (f *fakeWindows) Screens() []sysact.Screen { return f.screens }
func (f *fakeWindows) Focused(pid int) (sysact.Window, error) {
	if w, ok := f.wins[pid]; ok {
		return w, nil
	}
	return nil, sysact.ErrNoWindow
}

type fakeWindow struct {
	key   string
	frame sysact.Rect
}

func (w *fakeWindow) Key() string                 { return w.key }
func (w *fakeWindow) Frame() (sysact.Rect, error) { return w.frame, nil }
func (w *fakeWindow) SetFrame(r sysact.Rect) error {
	w.frame = r
	return nil
}
func (w *fakeWindow) Release() {}

// The display of the fake windows: 1440×900, the menu bar 25 high.
var testScreen = sysact.Screen{
	Frame:   sysact.Rect{X: 0, Y: 0, W: 1440, H: 900},
	Visible: sysact.Rect{X: 0, Y: 25, W: 1440, H: 875},
}

// testPID is the app that was in front as the search panel showed, with
// a window, testWindow's frame at first.
const testPID = 42

var testWindow = sysact.Rect{X: 200, Y: 200, W: 700, H: 500}

// actionFakes are the fakes the actions run on in a test.
type actionFakes struct {
	sys       *fakeSystem
	wins      *fakeWindows
	win       *fakeWindow
	asked     []string // the actions confirmAction asked for
	terminals bool     // what it was told last
	yes       bool     // what the user answers
	opened    []string // the URLs opened
}

// useFakeActions puts fakes in place of the system and the windows for
// the test: the volume 50%, Gocast allowed to arrange windows, a window
// in testPID, the Trash full; confirmations answered no.
func useFakeActions(t *testing.T) *actionFakes {
	t.Helper()
	f := &actionFakes{
		sys:  &fakeSystem{volume: 0.5, disks: 2, errs: map[string]error{}},
		win:  &fakeWindow{key: "42:1", frame: testWindow},
		wins: &fakeWindows{trusted: true, screens: []sysact.Screen{testScreen}},
	}
	f.wins.wins = map[int]*fakeWindow{testPID: f.win}
	oldSys, oldWins, oldConfirm, oldOpen := sysActions, sysWindows, sysConfirmAction, sysOpenURL
	sysActions, sysWindows = f.sys, f.wins
	sysConfirmAction = func(id string, terminals bool) bool {
		f.asked, f.terminals = append(f.asked, id), terminals
		return f.yes
	}
	sysOpenURL = func(url string) error {
		f.opened = append(f.opened, url)
		return nil
	}
	t.Cleanup(func() { sysActions, sysWindows, sysConfirmAction, sysOpenURL = oldSys, oldWins, oldConfirm, oldOpen })
	return f
}

// actionPanel is hotkeyPanel with the actions on fakes, the panel shown
// over testPID's window.
func actionPanel(t *testing.T, locale string) (*launcher, *ui.Tester, *actionFakes) {
	t.Helper()
	f := useFakeActions(t)
	l, tt, _ := hotkeyPanel(t, locale)
	l.target = pasteTarget{app: clipboard.App{BundleID: "com.apple.Notes", PID: testPID}}
	return l, tt, f
}

// rowIDs are the IDs of the items of the panel's rows, in order.
func rowIDs(l *launcher) []string {
	var out []string
	for _, r := range l.rows {
		if r.title == "" && !r.calc && r.own < 0 {
			out = append(out, l.items[r.item].ID)
		}
	}
	return out
}

// withoutActions are the rows of shown but the actions', which the
// subsequences of their names in both languages (pinyin too) often
// match, after the apps' (design.md 2.9).
func withoutActions(rows []string) []string {
	return slices.DeleteFunc(rows, func(r string) bool { return strings.Contains(r, actionPrefix) })
}

// runRow chooses the row of the action id, by a query, and presses Return.
func runRow(t *testing.T, l *launcher, query, id string) {
	t.Helper()
	typeQuery(l, query)
	chooseRow(t, l, actionPrefix+id)
	l.primary()
}

func TestActionNames(t *testing.T) {
	// Every action has a name in both languages, other than its ID, and
	// none two the same; as many as copy.md lists: 24 and 22.
	kinds := map[sysact.Kind]int{}
	for _, loc := range []string{"en", "zh-Hans"} {
		seen := map[string]string{}
		for _, a := range sysact.Actions {
			kinds[a.Kind]++
			n := actionTitle(locales.In(loc), a.ID)
			if n == "" || n == a.ID {
				t.Errorf("%s: %s has no name", loc, a.ID)
			}
			if other, ok := seen[n]; ok {
				t.Errorf("%s: %s and %s are both %q", loc, a.ID, other, n)
			}
			seen[n] = a.ID
		}
	}
	if kinds[sysact.KindSystem] != 2*24 || kinds[sysact.KindWindow] != 2*22 {
		t.Errorf("kinds: %v", kinds)
	}
}

// The actions are found by their names in both languages, the pinyin of
// the Chinese and their other words, in either language Gocast shows.
func TestActionSearch(t *testing.T) {
	for _, loc := range []string{"en", "zh-Hans"} {
		inLocale(t, loc)
		l := testLauncher(t, usage.New(), nil)
		for q, id := range map[string]string{
			"lock screen": sysact.LockScreen,
			"锁定屏幕":        sysact.LockScreen,
			"suoding":     sysact.LockScreen,
			"锁屏":          sysact.LockScreen,
			"dark mode":   sysact.ToggleAppearance,
			"左半屏":         sysact.LeftHalf,
			"zbp":         sysact.LeftHalf,
			"left half":   sysact.LeftHalf,
			"回收站":         sysact.OpenTrash,
			"volume 50":   sysact.Volume50,
			"音量 50":       sysact.Volume50,
		} {
			typeQuery(l, q)
			if got := rowIDs(l); len(got) == 0 || got[0] != actionPrefix+id {
				t.Errorf("%s: %q finds %q, want %s first", loc, q, shown(l), id)
			}
		}
	}
}

// Of the same match, the apps, presets and Gocast's own items come before
// the system's actions, those before the window's (design.md 2.9); an
// action opened more comes first as any item does.
func TestActionOrder(t *testing.T) {
	inLocale(t, "en")
	u := usage.New()
	l := testLauncher(t, u, nil)
	typeQuery(l, "s")
	ids := func() []string { return rowIDs(l) }
	got := ids()
	safari, sleep := slices.Index(got, "app:com.apple.Safari"), slices.Index(got, actionPrefix+sysact.Sleep)
	if safari < 0 || sleep < 0 || safari > sleep {
		t.Fatalf("s: %q", got)
	}
	typeQuery(l, "top")
	if got := ids(); len(got) < 2 || got[0] != actionPrefix+sysact.TopHalf {
		t.Errorf("top: %q", got)
	}
	for range 3 {
		u.Record(actionPrefix+sysact.Sleep, "", clipNow)
	}
	l.rebuild()
	typeQuery(l, "s")
	if got := ids(); got[0] != actionPrefix+sysact.Sleep {
		t.Errorf("Sleep opened most: %q", got)
	}
}

// With System & Window, the longest of the filter in English, the search
// field beside it keeps 200 at least (design.md 4.4): from its left edge
// (8 + 12 + 6 before the text) to 10 short of the filter.
func TestHotkeyPrefsSearchWidth(t *testing.T) {
	for _, loc := range []string{"en", "zh-Hans"} {
		useFakeActions(t)
		_, tt, _ := hotkeyPrefs(t, loc)
		field, ok := tt.AccessByRole(ui.RoleTextField, locales.HotkeysSearch())
		filter, ok2 := tt.AccessByRole(ui.RoleRadioGroup, locales.HotkeysFilter())
		if !ok || !ok2 {
			t.Fatalf("%s: no field (%v) or filter (%v)", loc, ok, ok2)
		}
		if w := filter.Bounds.X - 10 - (field.Bounds.X - 26); w < 200 {
			t.Errorf("%s: the search field is %.1f wide", loc, w)
		}
		if !has(tt, locales.HotkeysFilterActions()) {
			t.Errorf("%s: no %q", loc, locales.HotkeysFilterActions())
		}
		// The placeholder whole in the field's text box: MyGo cuts it
		// without an ellipsis.
		if w := textWidth(locales.HotkeysSearch(), 13); w > field.Bounds.W {
			t.Errorf("%s: the placeholder is %.1f wide, the field's text %.1f", loc, w, field.Bounds.W)
		}
	}
}

// textWidth is the width of s on one line at size.
func textWidth(s string, size float32) float32 {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).AlignItems(ui.Start).Children(func() { ui.Text(c, s).FontSize(size) })
	}, 800, 100)
	tt.Frame()
	r, _ := tt.Find(s)
	return r.W
}

// One kind of match is one score (design.md 2.9): the actions it finds
// keep copy.md 15's order, in the search panel and in Settings ›
// Shortcuts, whatever the match's place or the names' lengths.
func TestActionTieOrder(t *testing.T) {
	thirds := []string{sysact.LeftThird, sysact.CenterThird, sysact.RightThird, sysact.LeftTwoThirds, sysact.RightTwoThirds}
	halves := []string{sysact.LeftHalf, sysact.RightHalf, sysact.TopHalf, sysact.BottomHalf}
	ids := func(acts []string) []string {
		var out []string
		for _, a := range acts {
			out = append(out, actionPrefix+a)
		}
		return out
	}
	only := func(rows []string) []string {
		return slices.DeleteFunc(rows, func(r string) bool { return !strings.Contains(r, actionPrefix) })
	}
	for _, c := range []struct {
		locale, query string
		want          []string
	}{
		{"zh-Hans", "三分之", thirds},
		{"zh-Hans", "sanfenzhi", thirds},
		{"en", "third", thirds},
		{"zh-Hans", "半屏", halves},
		{"en", "half", halves},
	} {
		inLocale(t, c.locale)
		l := testLauncher(t, usage.New(), nil)
		typeQuery(l, c.query)
		if got := only(rowIDs(l)); !slices.Equal(got, ids(c.want)) {
			t.Errorf("%s %q: %q", c.locale, c.query, got)
		}
	}
	// "saf" is Safari's: no syllable cut before the query's end ("sa"
	// of san, then "f" of fen) finds the thirds. (Sleep Displays'
	// "display off" has its letters in order, as any English name.)
	for _, loc := range []string{"zh-Hans", "en"} {
		inLocale(t, loc)
		l := testLauncher(t, usage.New(), nil)
		typeQuery(l, "saf")
		for _, id := range rowIDs(l) {
			if slices.Contains(ids(thirds), id) {
				t.Errorf("%s saf: %q", loc, rowIDs(l))
				break
			}
		}
	}
	useFakeActions(t)
	p, tt, _ := hotkeyPrefs(t, "zh-Hans")
	tt.Type("三分之")
	tt.Frame()
	if got := only(shownIDs(p)); !slices.Equal(got, ids(thirds)) {
		t.Errorf("Settings 三分之: %q", got)
	}
}

// A row: the icon, the name, its tag; Run in the capsule; Cmd-K: Run and
// its shortcut alone; read as "Lock Screen, System" (design.md 2.9).
func TestActionRow(t *testing.T) {
	l, tt, _ := actionPanel(t, "en")
	typeQuery(l, "lock screen")
	chooseRow(t, l, actionPrefix+sysact.LockScreen)
	tt.Frame()
	for _, s := range []string{"Lock Screen", "System", "Run"} {
		if !has(tt, s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	if got := menuLabels(l); !slices.Equal(got, []string{"Run", "Set Shortcut…"}) {
		t.Errorf("Cmd-K: %q", got)
	}
	if got := l.rowLabel(l.chosen); got != "Lock Screen, System" {
		t.Errorf("label %q", got)
	}
	typeQuery(l, "left half")
	chooseRow(t, l, actionPrefix+sysact.LeftHalf)
	tt.Frame()
	if !has(tt, "Window") || !has(tt, "Left Half") {
		t.Errorf("Left Half: %q", tt.Texts())
	}

	// A shortcut set from Cmd-K, as an app's: it runs the action.
	runAction(t, l, "Set Shortcut…")
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Alt, ui.KeyLeft)
	tt.Frame()
	h := actionHotkey(sysact.LeftHalf)
	if got := l.g.settings.Hotkey(h); got == "" || l.rec.on {
		t.Fatalf("set: %q, recording %v", got, l.rec.on)
	}
	if got := menuLabels(l); !slices.Equal(got, []string{"Run", "Change Shortcut…", "Remove Shortcut"}) {
		t.Errorf("Cmd-K with a shortcut: %q", got)
	}
	if got := l.rowLabel(l.chosen); !strings.HasPrefix(got, "Left Half, Window, ") {
		t.Errorf("label %q", got)
	}
}

// What each system action does from the panel: the panel hides before
// those seen done, asking nothing, saying nothing.
func TestActionRunsHidden(t *testing.T) {
	for id, call := range map[string]string{
		sysact.LockScreen:       "LockScreen",
		sysact.Sleep:            "Sleep",
		sysact.SleepDisplays:    "SleepDisplays",
		sysact.ScreenSaver:      "StartScreenSaver",
		sysact.ToggleAppearance: "ToggleAppearance",
		sysact.ShowDesktop:      "ShowDesktop",
		sysact.HideOthers:       fmt.Sprintf("HideOthers %d", testPID),
		sysact.UnhideAll:        "UnhideAll",
		sysact.OpenTrash:        "OpenTrash",
	} {
		l, _, f := actionPanel(t, "en")
		l.g.runAction(id, fromPanel, testPID)
		if !slices.Equal(f.sys.calls, []string{call}) || l.shown || l.notice != "" || len(f.asked) > 0 {
			t.Errorf("%s: %q, shown %v, notice %q, asked %q", id, f.sys.calls, l.shown, l.notice, f.asked)
		}
	}
}

// Restart, Shut Down, Log Out, Quit All Apps and Empty Trash ask first,
// the panel hidden, from a hotkey too (copy.md 15.3).
func TestActionConfirms(t *testing.T) {
	for id, call := range map[string]string{
		sysact.Restart:    "Restart",
		sysact.ShutDown:   "ShutDown",
		sysact.LogOut:     "LogOut",
		sysact.QuitAll:    "QuitAll",
		sysact.EmptyTrash: "EmptyTrash",
	} {
		for _, src := range []actionSource{fromPanel, fromHotkey} {
			l, _, f := actionPanel(t, "en")
			l.g.runAction(id, src, testPID) // Cancel
			if !slices.Equal(f.asked, []string{id}) || len(f.sys.calls) > 0 || l.shown {
				t.Errorf("%s, cancelled: asked %q, %q, shown %v", id, f.asked, f.sys.calls, l.shown)
			}
			f.yes = true
			l.g.runAction(id, src, testPID)
			if !slices.Equal(f.sys.calls, []string{call}) || l.notice != "" {
				t.Errorf("%s: %q, notice %q", id, f.sys.calls, l.notice)
			}
		}
	}
}

// The alerts' words (copy.md 15.3): the floating terminal's commands
// named when some run.
func TestActionConfirmText(t *testing.T) {
	inLocale(t, "en")
	title, detail, button := confirmText(sysact.Restart, false)
	if title != "Restart this Mac?" || detail != "" || button != "Restart" {
		t.Errorf("Restart: %q %q %q", title, detail, button)
	}
	if _, detail, _ := confirmText(sysact.ShutDown, true); detail != locales.ConfirmTerminalsStop() {
		t.Errorf("Shut Down, terminals: %q", detail)
	}
	if _, detail, _ := confirmText(sysact.LogOut, true); detail != "All apps will quit. Commands running in the floating terminal will stop too." {
		t.Errorf("Log Out, terminals: %q", detail)
	}
	inLocale(t, "zh-Hans")
	if _, detail, _ := confirmText(sysact.LogOut, true); strings.Contains(detail, " ") {
		t.Errorf("Log Out in Chinese: %q", detail)
	}
	if _, _, button := confirmText(sysact.EmptyTrash, false); button != locales.ConfirmEmptyTrash() {
		t.Errorf("Empty Trash: %q", button)
	}
	// The five titles end with the full-width question mark (copy.md
	// 15.3); the English ones with "?".
	confirms := []string{sysact.Restart, sysact.ShutDown, sysact.LogOut, sysact.QuitAll, sysact.EmptyTrash}
	for _, id := range confirms {
		if title, _, _ := confirmText(id, false); !strings.HasSuffix(title, "？") {
			t.Errorf("%s in Chinese: %q", id, title)
		}
	}
	inLocale(t, "en")
	for _, id := range confirms {
		if title, _, _ := confirmText(id, false); !strings.HasSuffix(title, "?") {
			t.Errorf("%s in English: %q", id, title)
		}
	}
}

// The Trash already empty: said, the panel staying, no alert.
func TestActionTrashEmpty(t *testing.T) {
	l, tt, f := actionPanel(t, "en")
	f.sys.empty = true
	runRow(t, l, "empty trash", sysact.EmptyTrash)
	tt.Frame()
	if len(f.asked) > 0 || len(f.sys.calls) > 0 || !l.shown || !has(tt, "The Trash is already empty") {
		t.Errorf("asked %q, %q, shown %v: %q", f.asked, f.sys.calls, l.shown, tt.Texts())
	}
	// From a hotkey, nothing said.
	l.hideForAction()
	l.notice = ""
	l.g.runAction(sysact.EmptyTrash, fromHotkey, 0)
	if l.shown || l.notice != "" {
		t.Errorf("from a hotkey: shown %v, %q", l.shown, l.notice)
	}
}

// The volume: set, the panel staying to go on, a notice each time
// saying it (copy.md 15.4); a change unmutes.
func TestActionVolume(t *testing.T) {
	l, tt, f := actionPanel(t, "en")
	runRow(t, l, "volume up", sysact.VolumeUp)
	tt.Frame()
	if f.sys.volume != 0.5625 || !l.shown || !has(tt, "Volume 56%") || l.noticeAct.icon != "speaker.wave.2" {
		t.Fatalf("Volume Up: %v, shown %v, %q", f.sys.volume, l.shown, tt.Texts())
	}
	gen := l.noticeGen
	l.primary() // again: a new notice
	if f.sys.volume != 0.625 || l.notice != "Volume 63%" || l.noticeGen == gen {
		t.Errorf("again: %v %q", f.sys.volume, l.notice)
	}
	f.sys.muted = true
	runRow(t, l, "volume 25", sysact.Volume25)
	if f.sys.volume != 0.25 || f.sys.muted || l.notice != "Volume 25%" || l.noticeAct.icon != "speaker.wave.1" {
		t.Errorf("25%%: %v, muted %v, %q", f.sys.volume, f.sys.muted, l.notice)
	}
	f.sys.muted = true
	runRow(t, l, "volume 0", sysact.Volume0)
	if f.sys.volume != 0 || !f.sys.muted || l.noticeAct.icon != "speaker" {
		t.Errorf("0%%: %v, muted %v, %q", f.sys.volume, f.sys.muted, l.noticeAct.icon)
	}
	runRow(t, l, "mute", sysact.ToggleMute)
	if f.sys.muted || l.notice != "Unmuted" {
		t.Errorf("unmuting: %v %q", f.sys.muted, l.notice)
	}
	l.primary()
	if !f.sys.muted || l.notice != "Muted" || l.noticeAct.icon != "speaker.slash" {
		t.Errorf("muting: %v %q", f.sys.muted, l.notice)
	}
	// From a hotkey: set, nothing said, the panel as it was.
	l.hideForAction()
	l.notice = ""
	l.g.runAction(sysact.VolumeDown, fromHotkey, 0)
	if f.sys.volume != 0 || l.shown || l.notice != "" {
		t.Errorf("from a hotkey: %v, shown %v, %q", f.sys.volume, l.shown, l.notice)
	}
}

// Hidden files, the disks: the panel stays, saying how it went.
func TestActionNotices(t *testing.T) {
	l, _, f := actionPanel(t, "en")
	runRow(t, l, "hidden files", sysact.ToggleHiddenFiles)
	if l.notice != "Showing hidden files" || l.noticeAct.icon != "eye.circle" || !l.shown {
		t.Errorf("shown: %q", l.notice)
	}
	l.primary()
	if l.notice != "Hidden files are hidden again" {
		t.Errorf("hidden: %q", l.notice)
	}
	runRow(t, l, "eject", sysact.EjectAll)
	if l.notice != "Ejected 2 disks" || l.noticeAct.icon != "eject" || l.noticeAct.warning {
		t.Errorf("ejected: %q", l.notice)
	}
	f.sys.disks = 0
	l.primary()
	if l.notice != "No disks to eject" || !l.noticeAct.warning {
		t.Errorf("none: %q", l.notice)
	}
	f.sys.disks, f.sys.busy = 1, []string{"Backup"}
	l.primary()
	if l.notice != "“Backup” is in use and wasn’t ejected" || !l.noticeAct.warning {
		t.Errorf("busy: %q", l.notice)
	}
}

// Not allowed to control Finder: said, with a button to System Settings
// and a ×, until dismissed (copy.md 15.4); other failures in the log.
func TestActionAutomationDenied(t *testing.T) {
	l, tt, f := actionPanel(t, "en")
	f.sys.errs["ToggleHiddenFiles"] = &sysact.AutomationError{App: "Finder"}
	runRow(t, l, "hidden files", sysact.ToggleHiddenFiles)
	tt.Frame()
	if l.notice != "Gocast isn’t allowed to control “Finder”" || l.noticeAct.settings == "" || !has(tt, locales.ClipOpenSettings()) {
		t.Fatalf("denied: %q, %q", l.notice, tt.Texts())
	}
	tt.Click(locales.ClipOpenSettings())
	if !slices.Equal(f.opened, []string{sysact.AutomationSettingsURL}) {
		t.Errorf("opened %q", f.opened)
	}
	tt.Click(locales.Dismiss())
	tt.Frame()
	if l.notice != "" || has(tt, locales.ClipOpenSettings()) {
		t.Errorf("dismissed: %q", tt.Texts())
	}
	// From the alert's action: the panel shows again to say it.
	f.yes, f.sys.errs["QuitAll"] = true, &sysact.AutomationError{App: "System Events"}
	l.g.runAction(sysact.QuitAll, fromPanel, testPID)
	if !l.shown || l.notice != "Gocast isn’t allowed to control “System Events”" {
		t.Errorf("Quit All: shown %v, %q", l.shown, l.notice)
	}
	// Else nothing.
	l.notice = ""
	f.sys.errs["LockScreen"] = errors.New("pmset failed")
	l.g.runAction(sysact.LockScreen, fromPanel, testPID)
	if l.notice != "" || l.shown {
		t.Errorf("failed: %q", l.notice)
	}
}

// A window action arranges the window that was in front as the panel
// showed, the panel hiding; Restore puts it back; what can't be done
// is said, the panel staying.
func TestActionArranges(t *testing.T) {
	l, tt, f := actionPanel(t, "en")
	runRow(t, l, "left half", sysact.LeftHalf)
	if want := (sysact.Rect{X: 0, Y: 25, W: 720, H: 875}); f.win.frame != want || l.shown {
		t.Fatalf("Left Half: %v, shown %v", f.win.frame, l.shown)
	}
	l.enterSearch(clipNow)
	runRow(t, l, "restore", sysact.Restore)
	if f.win.frame != testWindow || l.shown {
		t.Errorf("Restore: %v", f.win.frame)
	}
	for _, c := range []struct {
		query, id, text string
	}{
		{"restore", sysact.Restore, "This window hasn’t been moved yet"},
		{"next display", sysact.NextDisplay, "Only one display is connected"},
	} {
		l.enterSearch(clipNow)
		l.g.arranger = nil // forgotten
		runRow(t, l, c.query, c.id)
		tt.Frame()
		if !l.shown || l.notice != c.text || !l.noticeAct.warning || l.noticeAct.icon != "exclamationmark.triangle" {
			t.Errorf("%s: shown %v, %q", c.id, l.shown, l.notice)
		}
	}
	l.target.app = clipboard.App{} // the desktop, or Gocast alone
	runRow(t, l, "maximize", sysact.Maximize)
	if !l.shown || l.notice != "No window to arrange" {
		t.Errorf("no window: %q", l.notice)
	}
}

// Not allowed to arrange windows: the banner asks over the list, from
// the panel or a hotkey (the panel shown, empty); allowed since, as the
// panel shows next, it says so (copy.md 15.5).
func TestActionAccessibility(t *testing.T) {
	l, tt, f := actionPanel(t, "en")
	f.wins.trusted = false
	runRow(t, l, "left half", sysact.LeftHalf)
	tt.Frame()
	if f.win.frame != testWindow || !l.shown || !has(tt, "Allow Gocast to arrange windows") {
		t.Fatalf("not allowed: %v, shown %v, %q", f.win.frame, l.shown, tt.Texts())
	}
	tt.Click(locales.ClipOpenSettings())
	if !slices.Equal(f.opened, []string{sysact.AccessibilitySettingsURL}) {
		t.Errorf("opened %q", f.opened)
	}

	// From a hotkey, the panel hidden: shown, empty, the banner over the
	// suggestions.
	l.hideForAction()
	l.askWindows = false
	clipFrontmost = func() clipboard.App { return clipboard.App{PID: testPID} }
	t.Cleanup(func() { clipFrontmost = clipboard.FrontmostApp })
	l.g.actionHotkeyPressed(sysact.RightHalf)
	tt.Frame()
	if !l.shown || l.query != "" || !has(tt, "Allow Gocast to arrange windows") || !has(tt, locales.SearchSuggestions()) {
		t.Fatalf("from a hotkey: shown %v, %q, %q", l.shown, l.query, tt.Texts())
	}

	// Allowed: the panel shown again says so, then it works.
	l.hideForAction()
	f.wins.trusted = true
	l.showForAction()
	tt.Frame()
	if l.askWindows || !has(tt, "Ready to arrange windows") {
		t.Errorf("allowed: %q", tt.Texts())
	}
	l.g.actionHotkeyPressed(sysact.RightHalf)
	if want := (sysact.Rect{X: 720, Y: 25, W: 720, H: 875}); f.win.frame != want || l.shown {
		t.Errorf("Right Half: %v, shown %v", f.win.frame, l.shown)
	}
}

// An action's hotkey runs it, the panel hidden or not: the window of the
// app in front, or the one the panel showed over.
func TestActionHotkeys(t *testing.T) {
	l, _, f := actionPanel(t, "en")
	g := l.g
	h := actionHotkey(sysact.Maximize)
	if err := g.setHotkey(h, "Ctrl+Alt+Return"); err != nil {
		t.Fatal(err)
	}
	if st, _ := g.hotkeyStatus(h); st != hotkeyOn {
		t.Fatalf("state %v", st)
	}
	if got := g.hotkeyLabel(h); got != "Maximize" {
		t.Errorf("label %q", got)
	}
	if !slices.ContainsFunc(g.hotkeyItems(), func(it hotkeyItem) bool { return it.item == h && it.accel != "" }) {
		t.Error("not among the hotkey items")
	}
	// Over the panel: the window it showed over.
	g.hotkeyFunc(h)()
	if want := testScreen.Visible; f.win.frame != want || l.shown {
		t.Errorf("Maximize: %v, shown %v", f.win.frame, l.shown)
	}
	// The panel hidden: the app in front.
	other := &fakeWindow{key: "7:1", frame: testWindow}
	f.wins.wins[7] = other
	clipFrontmost = func() clipboard.App { return clipboard.App{PID: 7} }
	t.Cleanup(func() { clipFrontmost = clipboard.FrontmostApp })
	g.hotkeyFunc(h)()
	if other.frame != testScreen.Visible || l.shown {
		t.Errorf("hidden: %v, shown %v", other.frame, l.shown)
	}
	// Another's shortcut: refused, naming the action.
	err := g.setHotkey(settings.Hotkey{App: "com.apple.Safari"}, "Ctrl+Alt+Return")
	if got := g.hotkeyProblem(err); got != locales.ShortcutUsedFor("Maximize") {
		t.Errorf("conflict: %q", got)
	}
}

// Settings › Shortcuts: the actions last, the system's then the
// window's, in copy.md's order; the Actions filter leaves them alone.
func TestHotkeyPrefsActions(t *testing.T) {
	useFakeActions(t)
	p, tt, _ := hotkeyPrefs(t, "en")
	var want []string
	for _, a := range sysact.Actions {
		want = append(want, actionPrefix+a.ID)
	}
	ids := shownIDs(p)
	if len(ids) < len(want) || !slices.Equal(ids[len(ids)-len(want):], want) {
		t.Fatalf("rows: %q", ids)
	}
	p.hk.filter = hkFilterActions
	p.filterHotkeys(true)
	tt.Frame()
	if got := shownIDs(p); !slices.Equal(got, want) {
		t.Errorf("Actions: %q", got)
	}
	for _, s := range []string{"System & Window", "Lock Screen", "System"} {
		if !has(tt, s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	// Found by their other words.
	tt.Type("dark mode")
	tt.Frame()
	if got := shownIDs(p); !slices.Equal(got, []string{actionPrefix + sysact.ToggleAppearance}) {
		t.Errorf("dark mode: %q", got)
	}
	if got := p.hotkeyRowLabel(0); got != "Toggle Appearance, System, "+locales.HotkeysNoShortcut() {
		t.Errorf("label %q", got)
	}
	// Set, a window action's.
	h := actionHotkey(sysact.LeftHalf)
	if err := p.g.setHotkey(h, "Ctrl+Alt+Left"); err != nil {
		t.Fatal(err)
	}
	p.hk.filter = hkFilterSet
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyDelete)
	p.filterHotkeys(true)
	if got := shownIDs(p); !slices.Contains(got, actionPrefix+sysact.LeftHalf) {
		t.Errorf("With Shortcut: %q", got)
	}
}

func TestScreensActions(t *testing.T) {
	dir := screensDir(t)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		shot := func(name string, setup func(l *launcher, tt *ui.Tester, f *actionFakes)) {
			l, tt, f := actionPanel(t, "zh-Hans")
			tt.SetScale(2)
			tt.SetDark(dark)
			setup(l, tt, f)
			saveScreen(t, tt, dir, name+"-"+mode, panelRect)
		}
		shot("actions-system", func(l *launcher, tt *ui.Tester, f *actionFakes) { typeQuery(l, "屏幕") })
		shot("actions-window", func(l *launcher, tt *ui.Tester, f *actionFakes) { typeQuery(l, "半") })
		shot("actions-thirds", func(l *launcher, tt *ui.Tester, f *actionFakes) { typeQuery(l, "三分之") })
		shot("actions-menu", func(l *launcher, tt *ui.Tester, f *actionFakes) {
			typeQuery(l, "锁屏")
			l.toggleMenu()
		})
		shot("actions-volume", func(l *launcher, tt *ui.Tester, f *actionFakes) { runRow(t, l, "音量加", sysact.VolumeUp) })
		shot("actions-no-window", func(l *launcher, tt *ui.Tester, f *actionFakes) {
			l.target.app = clipboard.App{}
			runRow(t, l, "左半屏", sysact.LeftHalf)
		})
		shot("actions-automation", func(l *launcher, tt *ui.Tester, f *actionFakes) {
			f.sys.errs["ToggleHiddenFiles"] = &sysact.AutomationError{App: "Finder"}
			runRow(t, l, "隐藏文件", sysact.ToggleHiddenFiles)
		})
		shot("actions-accessibility", func(l *launcher, tt *ui.Tester, f *actionFakes) {
			f.wins.trusted = false
			runRow(t, l, "左半屏", sysact.LeftHalf)
		})
		shot("actions-accessibility-ready", func(l *launcher, tt *ui.Tester, f *actionFakes) {
			l.askWindows = true
			l.hideForAction()
			l.showForAction()
		})

		// The placeholder, in both languages.
		for _, loc := range []string{"zh-Hans", "en"} {
			l, tt, _ := actionPanel(t, loc)
			tt.SetScale(2)
			tt.SetDark(dark)
			typeQuery(l, "")
			saveScreen(t, tt, dir, "actions-placeholder-"+loc+"-"+mode, panelRect)
		}

		for _, loc := range []string{"zh-Hans", "en"} {
			p, tt, _ := hotkeyPrefs(t, loc)
			tt.SetScale(2)
			tt.SetDark(dark)
			p.hk.filter = hkFilterActions
			p.filterHotkeys(true)
			_ = p.g.setHotkey(actionHotkey(sysact.LeftHalf), "Ctrl+Alt+Left")
			name := "settings-actions-"
			if loc == "en" {
				name += "en-"
			}
			saveScreen(t, tt, dir, name+mode, image.Rect(0, 0, prefsWidth, prefsHeight))
			if loc == "en" {
				continue
			}
			p.hk.list.ScrollTo(len(p.hk.shown)-1, ui.End)
			saveScreen(t, tt, dir, "settings-actions-window-"+mode, image.Rect(0, 0, prefsWidth, prefsHeight))
		}
	}
}
