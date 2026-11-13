package app

import (
	"fmt"
	"image"
	"io"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/locales"
)

// idleConn is the other end of a terminal in a test: nothing comes from
// it until it closes, and what is typed goes nowhere.
type idleConn struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newIdleConn() *idleConn {
	r, w := io.Pipe()
	return &idleConn{r, w}
}

func (c *idleConn) Read(b []byte) (int, error)  { return c.r.Read(b) }
func (c *idleConn) Write(b []byte) (int, error) { return len(b), nil }
func (c *idleConn) Close() error                { return c.w.Close() }

// hintsPanel is a floating terminal showing, w×h, with a tab for each
// title, the first showing; each runs no program, and has a terminal
// (showing out, focused) where libghostty-vt is there.
func hintsPanel(t *testing.T, locale string, w, h int, out string, titles ...string) (*termPanel, *ui.Tester) {
	t.Helper()
	inLocale(t, locale)
	g := &gocast{presets: []presets.Preset{{ID: "p", Name: "pi"}}}
	p := &termPanel{g: g, shown: true}
	g.term = p
	for _, title := range titles {
		p.keys++
		p.tabs = append(p.tabs, &session{key: p.keys, presetID: "p", preset: presets.Preset{ID: "p", Name: title}, fontSize: 13})
	}
	tt := ui.NewTester(p.view, w, h)
	tt.Frame()
	if terminal.Load() == nil {
		for _, s := range p.tabs {
			term, err := terminal.New(terminal.Options{
				Conn: newIdleConn(), Font: termFont(s.fontSize), Transparent: true, Theme: p.themes[0], DarkTheme: p.themes[1],
				MinimumContrast: terminalContrast, Padding: terminal.Padding{X: -1, Y: -1}, NoExitMessage: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { term.Close() })
			term.Feed([]byte(out))
			s.term = term
		}
	}
	p.activate(0)
	tt.Frame()
	return p, tt
}

// hintsUp reports whether the frame shows the hint card.
func hintsUp(tt *ui.Tester) bool { return has(tt, locales.TermHintGoToTab()) }

// cmdDown and cmdUp press and let go of the left Command key, the
// modifiers mods held with it (its own among them for cmdDown), as macOS
// tells a modifier on its own.
func cmdDown(tt *ui.Tester, mods ui.Modifiers) { tt.CodeDown(mods, ui.CodeSuperLeft) }
func cmdUp(tt *ui.Tester, mods ui.Modifiers)   { tt.CodeUp(mods, ui.CodeSuperLeft) }

// Command held alone shows the hints once it has been held 500 ms, keeps
// them as Shift is added, and hides them as it is let go of; the right
// Command key does too.
func TestCmdHintsTiming(t *testing.T) {
	_, tt := hintsPanel(t, "en", 1000, 600, "", "a", "b")
	for _, code := range []ui.KeyCode{ui.CodeSuperLeft, ui.CodeSuperRight} {
		tt.CodeDown(ui.Cmd, code)
		tt.Advance(cmdHintDelay - time.Millisecond)
		if hintsUp(tt) {
			t.Fatalf("%v: shown after %v", code, cmdHintDelay-time.Millisecond)
		}
		tt.Advance(time.Millisecond)
		if !hintsUp(tt) {
			t.Fatalf("%v: not shown after %v", code, cmdHintDelay)
		}
		tt.CodeDown(ui.Cmd|ui.Shift, ui.CodeShiftLeft)
		if !hintsUp(tt) {
			t.Fatalf("%v: Shift hid them", code)
		}
		tt.CodeUp(ui.Cmd, ui.CodeShiftLeft)
		tt.CodeUp(0, code)
		if hintsUp(tt) {
			t.Fatalf("%v: shown with Command let go of", code)
		}
	}
	// Let go of before the time, Command starts it again as it goes down.
	cmdDown(tt, ui.Cmd)
	tt.Advance(300 * time.Millisecond)
	cmdUp(tt, 0)
	cmdDown(tt, ui.Cmd)
	tt.Advance(300 * time.Millisecond)
	if hintsUp(tt) {
		t.Fatal("the time of the press before counted")
	}
	tt.Advance(200 * time.Millisecond)
	if !hintsUp(tt) {
		t.Fatal("not shown 500 ms after the second press")
	}
}

// Quick shortcuts never show the hints, however long Command stays held
// after them, until it is pressed again; the shortcut works as without
// them (design.md 3.5a, acceptance 3).
func TestCmdHintsQuickShortcuts(t *testing.T) {
	p, tt := hintsPanel(t, "en", 1000, 600, "", "a", "b", "c")
	for _, key := range []ui.Key{ui.KeyC, ui.KeyV, ui.Key2} {
		cmdDown(tt, ui.Cmd)
		tt.Advance(120 * time.Millisecond)
		tt.Key(ui.Cmd, key)
		tt.Advance(2 * time.Second)
		if hintsUp(tt) {
			t.Fatalf("Cmd-%v: shown", key)
		}
		cmdUp(tt, 0)
	}
	if p.active != 1 {
		t.Fatalf("Cmd-2: tab %d", p.active)
	}
	// Shown, Cmd-3 hides them and goes to the third tab.
	cmdDown(tt, ui.Cmd)
	tt.Advance(cmdHintDelay)
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(2 * time.Second)
	if hintsUp(tt) || p.active != 2 {
		t.Fatalf("Cmd-3 with the hints: shown %v, tab %d", hintsUp(tt), p.active)
	}
	cmdUp(tt, 0)
	cmdDown(tt, ui.Cmd)
	tt.Advance(cmdHintDelay)
	if !hintsUp(tt) {
		t.Fatal("not shown as Command is pressed again")
	}
}

// What ends the hints at once, and keeps them away while Command stays
// held: Control, Option, Shift-] (Shift alone keeps them), a click, an
// edit command, the panel losing the keyboard or hiding.
func TestCmdHintsDismiss(t *testing.T) {
	cases := []struct {
		name string
		do   func(p *termPanel, tt *ui.Tester)
	}{
		{"control", func(p *termPanel, tt *ui.Tester) { tt.CodeDown(ui.Cmd|ui.Ctrl, ui.CodeControlLeft) }},
		{"option", func(p *termPanel, tt *ui.Tester) { tt.CodeDown(ui.Cmd|ui.Alt, ui.CodeAltRight) }},
		{"shift-]", func(p *termPanel, tt *ui.Tester) {
			tt.CodeDown(ui.Cmd|ui.Shift, ui.CodeShiftLeft)
			tt.Key(ui.Cmd|ui.Shift, ui.KeyBracketRight)
			tt.CodeUp(ui.Cmd, ui.CodeShiftLeft)
		}},
		{"click", func(p *termPanel, tt *ui.Tester) { tt.ClickAtWith(ui.Cmd, 300, 300) }},
		{"inactive", func(p *termPanel, tt *ui.Tester) {
			tt.SetWindowActive(false)
			tt.SetWindowActive(true)
		}},
		{"hidden", func(p *termPanel, tt *ui.Tester) {
			p.shown = false // as hide sets it
			tt.Frame()
			p.shown = true
		}},
	}
	if terminal.Load() == nil { // the focus in a terminal, which the command goes to
		cases = append(cases, struct {
			name string
			do   func(p *termPanel, tt *ui.Tester)
		}{"copy", func(p *termPanel, tt *ui.Tester) { tt.Command("copy") }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, tt := hintsPanel(t, "en", 1000, 600, "", "a", "b")
			cmdDown(tt, ui.Cmd)
			tt.Advance(cmdHintDelay)
			if !hintsUp(tt) {
				t.Fatal("not shown")
			}
			tc.do(p, tt)
			if hintsUp(tt) {
				t.Fatal("still shown")
			}
			tt.HoldModifiers(ui.Cmd)
			tt.Advance(2 * time.Second)
			if hintsUp(tt) {
				t.Fatal("back without Command pressed again")
			}
		})
	}
	// The panel losing the keyboard lets go of every key.
	_, tt := hintsPanel(t, "en", 1000, 600, "", "a")
	cmdDown(tt, ui.Cmd)
	tt.Advance(cmdHintDelay)
	tt.SetFocused(false)
	if hintsUp(tt) {
		t.Fatal("shown in a panel without the keyboard")
	}
}

// Command does not count when pressed with Shift or a key already held,
// when it was down as the panel appeared, while the panel is not the key
// window, or while a shortcut is being recorded.
func TestCmdHintsNotCounting(t *testing.T) {
	cases := []struct {
		name string
		do   func(p *termPanel, tt *ui.Tester)
	}{
		{"shift", func(p *termPanel, tt *ui.Tester) {
			tt.CodeDown(ui.Shift, ui.CodeShiftLeft)
			cmdDown(tt, ui.Cmd|ui.Shift)
		}},
		{"key held", func(p *termPanel, tt *ui.Tester) {
			tt.KeyDown(ui.KeyA)
			cmdDown(tt, ui.Cmd)
		}},
		{"held as it appears", func(p *termPanel, tt *ui.Tester) {
			p.shown = false
			cmdDown(tt, ui.Cmd)
			p.shown = true
			p.hints.reset() // as show does
			tt.Frame()
		}},
		{"inactive", func(p *termPanel, tt *ui.Tester) {
			tt.SetWindowActive(false)
			cmdDown(tt, ui.Cmd)
		}},
		{"recording", func(p *termPanel, tt *ui.Tester) {
			p.g.suspended = true
			cmdDown(tt, ui.Cmd)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, tt := hintsPanel(t, "en", 1000, 600, "", "a")
			tc.do(p, tt)
			tt.Advance(2 * time.Second)
			if hintsUp(tt) {
				t.Fatal("shown")
			}
		})
	}
}

// The hints put a number in place of the close button of each of the
// first nine tabs, and the tabs keep their widths and places
// (design.md 3.5a); the card's rows are 28 high, 24 in a terminal too
// short for them, and its keys end on one line.
func TestCmdHintsLayout(t *testing.T) {
	titles := []string{"pi", "服务器", "a rather long preset name that is cut", "日志"}
	for i := len(titles); i < 11; i++ {
		titles = append(titles, fmt.Sprintf("t%d", i+1))
	}
	_, tt := hintsPanel(t, "zh-Hans", 1600, 600, "", titles...)
	boxes := func() []ui.Rect {
		var r []ui.Rect
		for _, title := range titles {
			b, ok := tt.Find(title)
			if !ok {
				t.Fatalf("no tab %q", title)
			}
			r = append(r, b)
		}
		return r
	}
	before := boxes()
	if !has(tt, locales.TermCloseTabTip()) {
		t.Fatal("no close button on the tab showing")
	}
	cmdDown(tt, ui.Cmd)
	tt.Advance(cmdHintDelay)
	if after := boxes(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("tabs moved:\n%v\n%v", before, after)
	}
	if has(tt, locales.TermCloseTabTip()) {
		t.Fatal("a close button with the hints")
	}
	// Its key, 20 wide, ends 8 from the tab's right: the number's middle
	// is 18 from it.
	for n := 1; n <= 9; n++ {
		num, ok := tt.Find(fmt.Sprint(n))
		tab := before[n-1]
		if mid := tab.X + tab.W - 18 - (num.X + num.W/2); !ok || mid < -0.5 || mid > 0.5 {
			t.Fatalf("number %d at %v, tab at %v", n, num, tab)
		}
	}
	if _, ok := tt.Find("10"); ok {
		t.Fatal("a number on the tenth tab")
	}

	rows := []string{locales.TermHintNewTab(), locales.TermHintCloseTab(), locales.TermHintActualSize()}
	for _, size := range []struct{ h, rowH int }{{600, 28}, {termMinHeight, 24}} {
		tt.SetSize(1000, size.h)
		var ys []float32
		for _, name := range rows {
			r, ok := tt.Find(name)
			if !ok {
				t.Fatalf("%d high: no row %q", size.h, name)
			}
			ys = append(ys, r.Y+r.H/2)
		}
		if ys[1]-ys[0] != float32(size.rowH) || ys[2]-ys[0] != float32(8*size.rowH) {
			t.Fatalf("%d high: rows at %v, want %d apart", size.h, ys, size.rowH)
		}
	}
}

// What the hints look like (design.md 3.5a): the tab bar and the panel
// without and with them, the card in the shortest terminal, in Chinese,
// in English, and dark.
func TestScreensCmdHints(t *testing.T) {
	dir := screensDir(t)
	out := "~/Work/gocast % git status\r\nOn branch cmd-hints\r\nnothing to commit, working tree clean\r\n~/Work/gocast % "
	titles := []string{"pi", "服务器", "gocast 的发布脚本和更新日志", "日志", "终端"}
	shot := func(name, locale string, w, h int, dark, hints bool) {
		p, tt := hintsPanel(t, locale, w, h, out, titles...)
		p.activate(1)
		if name == "cmdhints-capsule-zh" { // a long notice at the bottom left
			p.tabs[1].missingDir = "~/Work/a-folder-that-was-moved-somewhere-else/with/a/rather/long/path"
		}
		tt.SetScale(2)
		tt.SetDark(dark)
		if hints {
			cmdDown(tt, ui.Cmd)
		}
		tt.Frame()
		saveScreen(t, tt, dir, name, image.Rect(0, 0, w, h))
	}
	shot("cmdhints-off-zh", "zh-Hans", 1000, 600, false, false)
	shot("cmdhints-on-zh", "zh-Hans", 1000, 600, false, true)
	shot("cmdhints-short-zh", "zh-Hans", 760, termMinHeight, false, true)
	shot("cmdhints-on-en", "en", 1000, 600, false, true)
	shot("cmdhints-on-zh-dark", "zh-Hans", 1000, 600, true, true)
	shot("cmdhints-capsule-zh", "zh-Hans", 760, 400, false, true)
}
