package app

import (
	"log"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// The floating terminal's measures (design.md 3.2–3.6), in DIPs.
const (
	termMinWidth  = settings.MinTerminalPixels // as small as the panel gets, by any setting or drag
	termMinHeight = 300
	termTopGap    = 8
	tabBarH       = 52
	tabH          = 32
	// The terminal's room in the panel: across, under it, and under it
	// while capsules float over its bottom (design.md 3.3).
	termPadX, termPadBottom, termPadCapsules = 20, 12, 60
	termInset                                = 12 // the capsules' inset
	termLineHeight                           = 1.25
	minFontSize                              = 11
	maxFontSize                              = 20
	// sizeSaveDelay is how long after the last step of a drag of the
	// panel's edges its size is saved: once the drag ended.
	sizeSaveDelay = 400 * time.Millisecond
	slideIn       = 180 * time.Millisecond
	slideOut      = 120 * time.Millisecond
	// The panel drops dropIn to its place as it shows, and rises
	// riseOut as it hides (design.md 1.5).
	dropIn, riseOut = 12, 8
	fadeIn, fadeOut = 120 * time.Millisecond, 100 * time.Millisecond
	animationFrame  = time.Second / 120
)

// session is a tab: a terminal running a preset, or a plain one (no
// preset), which keeps running while the panel hides, until its program
// ends or the user closes the tab.
type session struct {
	presetID string         // "" for a plain terminal
	preset   presets.Preset // as when it last started
	term     *terminal.Terminal
	gen      int // a new view for each run, keyed by it
	key      int // the tab's key, for its whole life
	focus    ui.Handle

	pending    bool   // to start in the next frame, which sizes it
	missingDir string // the folder of the preset, gone: opened in the home folder
	failure    string // the terminal could not start
	fontSize   float32
	announced  bool // the banner was read out
	done       bool // its command ended: a plain terminal now (design.md 3.6)
}

func (s *session) plain() bool { return s.presetID == "" }

func (s *session) title() string {
	if s.plain() || s.done {
		return locales.TermTabDefault()
	}
	return presetTitle(s.preset)
}

// termPanel is the floating terminal: a borderless panel that drops from
// the top of the screen, over every app, with tabs.
type termPanel struct {
	g      *gocast
	win    *mygo.Window
	tabs   []*session
	active int // the tab showing, -1 for none
	keys   int

	shown  bool         // shown, or showing: hide is what the shortcut does next
	anim   atomic.Int64 // the generation of the slide in progress
	reveal bool         // scroll the tab bar to the active tab
	menu   bool         // the new tab menu shows
	chosen int          // the row of the new tab menu chosen
	top    int          // the top of the work area the panel shows in

	wa      mygo.Rectangle // the work area the panel shows in
	bounds  mygo.Rectangle // the panel's bounds as last placed or resized
	placing bool           // the panel's bounds are being set, not dragged
	saveGen int            // the size dragged to last, to save
	accents [2]ui.Color    // the light and dark accents of the themes
	themes  [2]*terminal.Theme
}

func newTermPanel(g *gocast) *termPanel {
	p := &termPanel{g: g, active: -1}
	p.win = mygo.NewWindow(mygo.WindowOptions{
		Title:        locales.TrayTerminal(),
		Panel:        true, // over every app and full-screen ones, on every Space
		Hidden:       true,
		Frameless:    true,
		Transparent:  true,
		Vibrancy:     mygo.VibrancyGlass,
		CornerRadius: panelRadius,
		Width:        1200,
		Height:       600,
		MinWidth:     termMinWidth,
		MinHeight:    termMinHeight,
		Content:      ui.View(p.view),
	})
	// Hiding never ends the programs: Cmd-W closes a tab, and going
	// elsewhere hides the panel, as Ghostty's quick terminal does, unless
	// the user keeps it on screen (Settings).
	p.win.OnClose(func(e *mygo.CloseEvent) {
		if !g.quitting {
			e.PreventDefault()
			p.hide()
		}
	})
	p.win.OnResize(p.resized)
	p.win.OnBlur(func() {
		if !g.settings.KeepTerminalOnBlur {
			p.hide()
		}
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		for _, s := range p.tabs {
			if s.term != nil {
				s.term.Close()
			}
		}
	})
	return p
}

func (p *termPanel) current() *session {
	if p.active < 0 || p.active >= len(p.tabs) {
		return nil
	}
	return p.tabs[p.active]
}

func (p *termPanel) view(c *ui.Context) {
	c.SetTheme(themed(c))
	k := tokensFor(c)
	c.Root().Background(ui.Transparent)
	p.followAccent(c)
	p.shortcuts(c)
	s := p.current()
	// One surface, the tinted glass, under the tab bar and the terminal,
	// with no line between them, outlined as the search panel (design.md
	// 3.2, 3.3).
	root := ui.Column(c).Fill().Background(glassFill(c, k, terminalTint))
	panelOutline(root, k).Children(func() {
		p.tabBar(c, k)
		bottom := float32(termPadBottom)
		if s != nil && p.hasCapsules(s) {
			bottom = termPadCapsules // the last line and the cursor above the capsules
		}
		ui.Column(c).Grow(1).Padding(0, termPadX, bottom, termPadX).Children(func() {
			if s == nil {
				return
			}
			if s.pending {
				// Started in the frame that shows it, so that the program
				// starts at the size of the view, not at 80×24 (which
				// agents' interfaces, as pi's, keep drawing at until they
				// redraw).
				s.pending = false
				p.start(s)
			}
			if s.term == nil {
				ui.Box(c).Grow(1) // it could not start: the capsule says why
				return
			}
			c.Key(strconv.Itoa(s.key) + "#" + strconv.Itoa(s.gen))
			terminal.View(c, s.term).Grow(1).Bind(&s.focus).AutoFocus()
		})
		if s != nil {
			p.capsules(c, k, s)
		}
		if p.menu {
			p.newTabMenu(c, k)
		}
	})
}

// followAccent gives the terminals the themes of the user's accent color,
// which colors their cursor and selection, as it changes.
func (p *termPanel) followAccent(c *ui.Context) {
	prefs := c.Preferences()
	accents := [2]ui.Color{ui.LightTheme().WithPreferences(prefs).Accent, ui.DarkTheme().WithPreferences(prefs).Accent}
	if p.themes[0] != nil && accents == p.accents {
		return
	}
	p.accents = accents
	p.themes = [2]*terminal.Theme{terminalTheme(false, accents[0]), terminalTheme(true, accents[1])}
	for _, s := range p.tabs {
		if s.term != nil {
			s.term.SetTheme(p.themes[0], p.themes[1])
		}
	}
}

// termFont is the terminal's font at size: the system's monospaced font
// (SF Mono), its rows 1.25 its line height (design.md 3.3a).
func termFont(size float32) terminal.Font {
	return terminal.Font{Size: size, LineHeight: termLineHeight}
}

// shortcuts are the panel's Command keys, which never reach the program:
// Cmd-T, Cmd-W, Cmd-Shift-[ and ], Cmd-1…9, and the font size; the keys of
// the new tab menu while it shows; Enter once a preset's command ended.
func (p *termPanel) shortcuts(c *ui.Context) {
	if p.menu {
		p.menuKeys(c)
	}
	if c.Shortcut(ui.Cmd, ui.KeyT) {
		p.newTab(p.g.defaultPreset())
	}
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		if s := p.current(); s != nil {
			p.closeTab(s)
		} else {
			p.hide()
		}
	}
	digits := []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, key := range digits {
		if c.Shortcut(ui.Cmd, key) && i < len(p.tabs) {
			p.activate(i)
		}
	}
	if n := len(p.tabs); n > 0 {
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketRight) {
			p.activate((p.active + 1) % n)
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketLeft) {
			p.activate((p.active - 1 + n) % n)
		}
	}
	s := p.current()
	if s == nil {
		return
	}
	size := s.fontSize
	if c.Shortcut(ui.Cmd, ui.KeyEqual) || c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEqual) {
		size = min(size+1, maxFontSize)
	}
	if c.Shortcut(ui.Cmd, ui.KeyMinus) {
		size = max(size-1, minFontSize)
	}
	if c.Shortcut(ui.Cmd, ui.Key0) {
		size = float32(p.g.settings.Font())
	}
	if size != s.fontSize {
		s.fontSize = size // this tab's, not written to the settings
		if s.term != nil {
			s.term.SetFont(termFont(size))
		}
	}
}

// tabBar shows Gocast's mark, the tabs, and the new tab and hide buttons,
// on the glass, as the search bar (design.md 3.4).
func (p *termPanel) tabBar(c *ui.Context, k tokens) {
	ui.Row(c).Height(tabBarH).Shrink(0).Padding(0, 12, 0, 20).Gap(12).AlignItems(ui.Center).Children(func() {
		gocastMark(c, k)
		ui.ScrollHorizontal(c).Grow(1).Shrink(1).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for i, s := range p.tabs {
					p.tab(c, k, i, s)
				}
			})
		})
		ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Children(func() {
			roundButton(c, k, "plus", locales.TermNewTab(), p.menu).HasPopup(ui.PopupMenu).Expanded(p.menu).
				OnClick(p.toggleMenu)
			hide := locales.TermHide(shortcut.Glyphs(p.g.terminalKey))
			roundButton(c, k, "chevron.up", hide, false).OnClick(p.hide)
		})
	})
}

// tab is a tab of the tab bar, a capsule as a search result: its title, a
// dot once its program ended, and its close button, on the tab showing
// and the one under the pointer.
func (p *termPanel) tab(c *ui.Context, k tokens, i int, s *session) {
	on := i == p.active
	bg, fg := ui.Transparent, k.textOnGlass
	if on {
		bg, fg = k.selection, k.text
	}
	tip := locales.TermTabDefault()
	if !s.plain() {
		tip = presetSubtitle(s.preset)
	}
	if i < 9 {
		tip += "  ⌘" + strconv.Itoa(i+1)
	}
	c.Key(s.key)
	tab := ui.Row(c).Height(tabH).MinWidth(96).MaxWidth(220).Padding(0, 8, 0, 14).Radius(tabH / 2).Background(bg).
		Gap(6).AlignItems(ui.Center).Tooltip(tip).Label(s.title())
	hovered := tab.Hovered()
	idx := i
	tab.OnClick(func() { p.activate(idx) }).Children(func() {
		ui.Text(c, s.title()).FontSize(13).TextColor(fg).SingleLine().Grow(1).Shrink(1)
		closeBox := ui.Box(c).Size(16, 16).Radius(8).Center().Shrink(0)
		if on || hovered {
			closeBox.Label(locales.TermCloseTabTip()).Tooltip(locales.TermCloseTabTip()).
				OnClick(func() { p.closeTab(s) }).Children(func() {
				sfsymbols.Symbol(c, "xmark").Size(10).Color(fg)
			})
		}
	})
	if on && p.reveal {
		p.reveal = false
		tab.ScrollIntoView()
	}
}

// --- The new tab menu ---

// menuEntries are the rows of the new tab menu: the presets, then a plain
// terminal (nil).
func (p *termPanel) menuEntries() []*presets.Preset {
	var rows []*presets.Preset
	for i := range p.g.presets {
		rows = append(rows, &p.g.presets[i])
	}
	return append(rows, nil)
}

func (p *termPanel) toggleMenu() {
	p.menu = !p.menu
	p.chosen = 0
	if !p.menu {
		p.refocus()
	}
	p.win.Invalidate()
}

func (p *termPanel) closeMenu() {
	if p.menu {
		p.menu = false
		p.refocus()
		p.win.Invalidate()
	}
}

// refocus gives the keyboard back to the terminal showing.
func (p *termPanel) refocus() {
	if s := p.current(); s != nil {
		s.focus.Focus()
	}
}

// runMenu opens the menu's row i in a new tab.
func (p *termPanel) runMenu(i int) {
	rows := p.menuEntries()
	if i < 0 || i >= len(rows) {
		return
	}
	p.menu = false
	if pr := rows[i]; pr != nil {
		pr := *pr
		p.newTab(&pr)
	} else {
		p.newTab(nil)
	}
}

// menuKeys are the new tab menu's keys, before the terminal's: Up and
// Down (and Ctrl-P, Ctrl-N) choose, Enter opens, Escape closes it.
func (p *termPanel) menuKeys(c *ui.Context) {
	n := len(p.menuEntries())
	switch {
	case c.Shortcut(0, ui.KeyEscape):
		p.closeMenu()
	case c.Shortcut(0, ui.KeyDown), c.Shortcut(ui.Ctrl, ui.KeyN):
		p.chosen = min(p.chosen+1, n-1)
	case c.Shortcut(0, ui.KeyUp), c.Shortcut(ui.Ctrl, ui.KeyP):
		p.chosen = max(p.chosen-1, 0)
	case c.Shortcut(0, ui.KeyEnter):
		p.runMenu(p.chosen)
	}
}

// newTabMenu is the menu of the new tab button, as the search panel's
// actions menu (design.md 3.4), under the button, at its right: Open
// with Preset over the presets, then a plain terminal. A click elsewhere
// closes it.
func (p *termPanel) newTabMenu(c *ui.Context, k tokens) {
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).AccessHidden(true).OnClick(p.closeMenu)
	right := float32(12 + roundButtonSize + 4)
	top := float32((tabBarH-roundButtonSize)/2 + roundButtonSize + 8)
	c.Key("new-tab-menu")
	popupMenu(c, k, 1, 0).Right(right).Top(top).Role(ui.RoleMenu).Label(locales.TermNewTabMenu()).Children(func() {
		menuItems(c, func() {
			menuTitle(c, k, locales.TermNewTabMenu())
			rows := p.menuEntries()
			for i, pr := range rows {
				idx := i
				if pr == nil {
					if i > 0 {
						menuSeparator(c, k)
					}
					menuRow(c, k, "", locales.TermPlain(), "", nil, i == p.chosen, func() { p.runMenu(idx) })
					continue
				}
				menuRow(c, k, "", presetTitle(*pr), lastDir(pr.ShortDir()), nil, i == p.chosen, func() { p.runMenu(idx) })
			}
		})
	})
}

// lastDir is the last folder of a path, as ~/Projects/x → x.
func lastDir(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 && i < len(path)-1 {
		return path[i+1:]
	}
	return path
}

// --- Capsules ---

// hasCapsules reports whether capsules float over the bottom of the
// session's terminal.
func (p *termPanel) hasCapsules(s *session) bool {
	return s.failure != "" || s.missingDir != "" || p.plainTip(s)
}

// plainTip reports whether the tip over a plain terminal shows: while
// there are no presets, until the user closes it.
func (p *termPanel) plainTip(s *session) bool {
	return s.plain() && len(p.g.presets) == 0 && !p.g.settings.PlainTipDismissed
}

// capsules float over the bottom of the terminal, as the search panel's
// (design.md 3.6): why the terminal could not start, that the preset's
// folder is gone, or the plain terminal's tip. A preset's command that
// ended says so in the terminal itself (presets.After).
func (p *termPanel) capsules(c *ui.Context, k tokens, s *session) {
	var n notice
	switch {
	case s.failure != "":
		n = notice{icon: "exclamationmark.triangle", iconColor: k.danger, color: k.danger, text: s.failure}
	case s.missingDir != "":
		n = notice{icon: "exclamationmark.triangle", iconColor: k.warning, text: locales.TermFolderMissing(s.missingDir)}
		n.extra = func() { dismissButton(c, k, locales.Dismiss(), func() { s.missingDir = "" }) }
	case p.plainTip(s):
		n = notice{icon: "info.circle", iconColor: k.textOnGlass, text: locales.TermPlainTip()}
		n.extra = func() {
			capsuleButton(c, k, locales.PresetsAdd(), func() {
				p.hide()
				p.g.addPreset()
			})
			dismissButton(c, k, locales.Dismiss(), func() {
				p.g.settings.PlainTipDismissed = true // never again
				p.g.saveSettings()
			})
		}
	default:
		return
	}
	if n.color.A == 0 {
		n.color = k.text
	}
	if !s.announced {
		s.announced = true
		c.Announce(n.text)
	}
	room := float32(p.win.Bounds().Width) - 2*termInset
	noticeCapsule(c, k, termInset, termInset, max(room, 160), n)
}

// --- Tabs ---

// newTab opens a tab running the preset, or a plain terminal for nil, and
// shows it.
func (p *termPanel) newTab(pr *presets.Preset) {
	p.keys++
	s := &session{pending: true, key: p.keys, fontSize: float32(p.g.settings.Font())}
	if pr != nil {
		s.presetID, s.preset = pr.ID, *pr
	}
	p.tabs = append(p.tabs, s)
	p.activate(len(p.tabs) - 1)
}

func (p *termPanel) activate(i int) {
	p.active = i
	p.reveal = true
	if s := p.current(); s != nil {
		s.focus.Focus()
	}
	p.win.Invalidate()
}

// closeTab ends a tab's program and shows the tab after it, or the one
// before; the panel hides with the last.
func (p *termPanel) closeTab(s *session) {
	i := slices.Index(p.tabs, s)
	if i < 0 {
		return
	}
	if s.term != nil {
		s.term.Close()
	}
	p.tabs = slices.Delete(p.tabs, i, i+1)
	switch {
	case len(p.tabs) == 0:
		p.active = -1
		p.hide() // the shortcut opens the default preset next
	case p.active > i:
		p.activate(p.active - 1)
	case p.active == i:
		p.activate(min(i, len(p.tabs)-1)) // the tab after it, else the one before
	}
	p.win.Invalidate()
}

// start runs the session's preset (or the user's shell) in a new
// terminal: a preset's command through the user's shell as a login and
// interactive shell ($SHELL -lic), in its folder, or the home folder when
// that is gone.
func (p *termPanel) start(s *session) {
	pr := s.preset
	if s.plain() {
		pr = presets.Preset{Dir: "~"}
	}
	dir := pr.WorkDir()
	s.missingDir, s.failure, s.announced = "", "", false
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		s.missingDir = pr.ShortDir()
		dir = presets.ExpandHome("~")
	}
	s.done = false
	s.gen++
	var term *terminal.Terminal
	opts := terminal.Options{
		Command: pr.Argv(presets.UserShell(), afterCommand(pr)),
		Dir:     dir,
		Env:     pr.Environ(),
		Font:    termFont(s.fontSize),
		// The glass shows through the terminal, in the colors of
		// design.md 3.3a; its room is the view's padding (termPadX…),
		// which changes as capsules come and go.
		Transparent:     true,
		Theme:           p.themes[0],
		DarkTheme:       p.themes[1],
		MinimumContrast: terminalContrast,
		Padding:         terminal.Padding{X: -1, Y: -1},
		// A tab whose shell exited closes, as in every terminal app, so
		// the terminal's dim [Process exited] line would never be seen.
		NoExitMessage: true,
		// Programs that read the clipboard themselves, as coding agents
		// taking pasted images, are offered every format of a paste (the
		// Kitty clipboard protocol, mode 5522); an image alone pastes an
		// empty bracketed paste, which Claude Code takes as its cue. Both
		// are the plugin's defaults: reads outside a paste stay refused.

		// The preset's command ended and its shell goes on as a plain
		// terminal (presets.After, design.md 3.6 「命令结束」).
		OnTitle: func(title string) {
			if title != presets.AfterTitle {
				return
			}
			p.win.Update(func() {
				if s.term == term {
					s.done = true
				}
			})
		},
		OnExit: func(int) {
			p.win.Update(func() {
				if s.term != term {
					return
				}
				p.closeTab(s) // the shell exited, as in every terminal app
			})
		},
	}
	t, err := terminal.New(opts)
	if err != nil {
		// No libghostty-vt (the first run downloads it), or no shell.
		log.Printf("terminal: %v", err)
		s.failure = err.Error()
		s.term = nil
		return
	}
	term = t
	s.term = t
}

// openPreset shows the panel on the tab of the preset with the ID, opening
// one the first time, or always with newTab.
func (p *termPanel) openPreset(id string, newTab bool) {
	i := presets.Find(p.g.presets, id)
	if i < 0 {
		return
	}
	j := slices.IndexFunc(p.tabs, func(s *session) bool { return s.presetID == id && !s.done })
	if newTab || j < 0 {
		pr := p.g.presets[i]
		p.newTab(&pr)
	} else {
		p.activate(j)
	}
	p.show()
}

// presetsChanged follows renamed presets in the tab titles. Tabs of
// presets deleted keep running (copy.md 4.2).
func (p *termPanel) presetsChanged() {
	for _, s := range p.tabs {
		if i := presets.Find(p.g.presets, s.presetID); i >= 0 {
			s.preset.Name = p.g.presets[i].Name
		}
	}
	p.win.Invalidate()
}

// fontChanged sets the font size of Settings in every tab.
func (p *termPanel) fontChanged() {
	size := float32(p.g.settings.Font())
	for _, s := range p.tabs {
		s.fontSize = size
		if s.term != nil {
			s.term.SetFont(termFont(size))
		}
	}
	p.win.Invalidate()
}

// --- Showing and hiding ---

// toggle shows the panel on the tab it last showed, or hides it. With no
// tab it opens one with the default preset, or a plain terminal.
func (p *termPanel) toggle() {
	if p.shown {
		p.hide()
		return
	}
	if p.g.loadPresets() {
		p.g.presetsChanged()
	}
	if len(p.tabs) == 0 {
		p.newTab(p.g.defaultPreset())
	}
	p.show()
}

// focused reports whether the panel shows and is the key window: where
// the clipboard history pastes.
func (p *termPanel) focused() bool { return p.shown && p.win.IsFocused() }

// paste focuses the panel and pastes text into its tab, as the paste keys
// paste the clipboard: false when it has no tab running.
func (p *termPanel) paste(text string) bool {
	s := p.current()
	if s == nil || s.term == nil {
		return false
	}
	p.show()
	s.term.Paste(text)
	return true
}

// open shows the panel as toggle does, or focuses it where it shows.
func (p *termPanel) open() {
	if p.shown {
		p.win.Focus()
		return
	}
	p.toggle()
}

// show drops the panel from the top of the work area of the display with
// the pointer, at its default size (design.md 3.2, 1.5).
func (p *termPanel) show() {
	if p.shown {
		p.win.Focus()
		if s := p.current(); s != nil {
			s.focus.Focus()
		}
		return
	}
	p.shown = true
	wa := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint()).WorkArea
	w, h := termSize(wa.Width, wa.Height, p.g.settings)
	x, y := wa.X+(wa.Width-w)/2, wa.Y+termTopGap
	p.top, p.wa = wa.Y, wa
	gen := p.anim.Add(1)
	reduce := mygo.Theme.Preferences().ReduceMotion
	// AppKit keeps a window's top under the menu bar (a frameless window
	// is a titled one underneath), so the panel cannot slide from above
	// the screen: it fades in as it drops the last dropIn (design.md
	// 1.5), held at the menu bar for the first frames where termTopGap is
	// less.
	from := y - dropIn
	if reduce {
		from = y
	}
	p.place(mygo.Rectangle{X: x, Y: from, Width: w, Height: h})
	p.win.SetOpacity(0)
	if s := p.current(); s != nil {
		s.focus.Focus()
	}
	p.win.Show()
	p.win.Invalidate()
	if reduce {
		p.animate(gen, fadeIn, easeOut, func(t float64) { p.win.SetOpacity(t) }, nil)
		return
	}
	p.animate(gen, slideIn, easeOut, func(t float64) {
		p.win.SetOpacity(t)
		p.win.SetPosition(x, from+int(math.Round(float64(y-from)*t)))
	}, nil)
}

// place sets the panel's bounds, which resized then knows the user did not
// drag.
func (p *termPanel) place(r mygo.Rectangle) {
	p.placing = true
	p.win.SetBounds(r)
	p.placing = false
	p.bounds = p.win.Bounds() // as AppKit constrained it
}

// resized follows the user's drag of the panel's edges (design.md 3.2):
// the bottom edge sets its height, the left and right ones its width, on
// both sides, so that it stays centered; its top stays. The size, in
// percent of the work area, shows in Settings at once, and is saved once
// the drag ends, for the next show.
func (p *termPanel) resized() {
	if p.placing || !p.shown || p.wa.Width <= 0 || p.wa.Height <= 0 {
		return
	}
	b, prev := p.win.Bounds(), p.bounds
	if b == prev {
		return
	}
	w, h := dragSize(prev, b, p.wa)
	r := mygo.Rectangle{X: prev.X + (prev.Width-w)/2, Y: prev.Y, Width: w, Height: h}
	if r != b {
		p.place(r)
	} else {
		p.bounds = b
	}
	st := &p.g.settings
	was := *st
	if w != prev.Width {
		st.TerminalWidth = termPercent(w, p.wa.Width, settings.MinTerminalWidth, settings.MaxTerminalWidth)
	}
	if h != prev.Height {
		st.TerminalHeight = termPercent(h, p.wa.Height, settings.MinTerminalHeight, settings.MaxTerminalHeight)
	}
	if st.TerminalWidth == was.TerminalWidth && st.TerminalHeight == was.TerminalHeight {
		return
	}
	p.g.prefs.terminalSizeChanged()
	p.saveGen++
	gen := p.saveGen
	time.AfterFunc(sizeSaveDelay, func() {
		p.win.Update(func() {
			if gen == p.saveGen {
				p.g.saveSettings()
			}
		})
	})
}

// dragSize is the panel's size as the user drags its edges from prev to
// b, in the work area wa: a left edge dragged grows or shrinks it on both
// sides, as does a right one; the bottom edge sets the height; a top edge
// dragged changes nothing. Within the settings' ranges and the minimum.
func dragSize(prev, b, wa mygo.Rectangle) (w, h int) {
	w, h = prev.Width, prev.Height
	center2 := 2*prev.X + prev.Width // twice the center, in whole DIPs
	switch {
	case b.X != prev.X: // the left edge
		w = center2 - 2*b.X
	case b.Width != prev.Width: // the right edge
		w = 2*(b.X+b.Width) - center2
	}
	if b.Y == prev.Y && b.Height != prev.Height { // the bottom edge
		h = b.Height
	}
	w = clampSize(w, wa.Width, settings.MinTermWidth(wa.Width), settings.MaxTerminalWidth, termMinWidth)
	h = clampSize(h, wa.Height, settings.MinTerminalHeight, settings.MaxTerminalHeight, termMinHeight)
	return w, h
}

// clampSize keeps px within lo% and hi% of wa, at least least, and never
// over wa.
func clampSize(px, wa, lo, hi, least int) int {
	return min(max(px, percentOf(wa, lo), least), percentOf(wa, hi), wa)
}

// sizeChanged gives the panel showing the size of Settings, at the top
// of its work area, centered.
func (p *termPanel) sizeChanged() {
	if !p.shown || p.wa.Width <= 0 {
		return
	}
	p.anim.Add(1) // ends a drop in progress where it would end
	p.win.SetOpacity(1)
	w, h := termSize(p.wa.Width, p.wa.Height, p.g.settings)
	p.place(mygo.Rectangle{X: p.wa.X + (p.wa.Width-w)/2, Y: p.wa.Y + termTopGap, Width: w, Height: h})
}

// workAreaWidth is the width of the work area of the display the panel
// is on, else of the one under the mouse, where it would show.
func (p *termPanel) workAreaWidth() int {
	if p.shown && p.wa.Width > 0 {
		return p.wa.Width
	}
	return mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint()).WorkArea.Width
}

// termSize is the panel's size in a work area of waW×waH for the
// percentages of the settings (design.md 3.2): no smaller than
// termMinWidth×termMinHeight, no larger than the work area.
func termSize(waW, waH int, st settings.Settings) (w, h int) {
	w, h = percentOf(waW, st.TermWidthOn(waW)), percentOf(waH, st.TermHeight())
	return min(max(w, termMinWidth), waW), min(max(h, termMinHeight), waH)
}

// percentOf is pct percent of n, rounded.
func percentOf(n, pct int) int { return int(math.Round(float64(n) * float64(pct) / 100)) }

// termPercent is px in percent of the work area's wa, rounded to the
// settings' step, within lo and hi.
func termPercent(px, wa, lo, hi int) int {
	return settings.SnapPercent(float64(px)*100/float64(wa), lo, hi)
}

// hide lifts the panel riseOut as it fades out (fades it only,
// with Reduce Motion), its tabs and programs kept.
func (p *termPanel) hide() {
	if !p.shown {
		return
	}
	p.shown = false
	p.menu = false
	gen := p.anim.Add(1)
	done := func() {
		if p.anim.Load() == gen {
			p.win.Hide()
			p.win.SetOpacity(1)
		}
	}
	if mygo.Theme.Preferences().ReduceMotion {
		p.animate(gen, fadeOut, easeIn, func(t float64) { p.win.SetOpacity(1 - t) }, done)
		return
	}
	b := p.win.Bounds()
	to := max(b.Y-riseOut, p.top) // no higher than AppKit lets it (see show)
	p.animate(gen, slideOut, easeIn, func(t float64) {
		p.win.SetOpacity(1 - t)
		p.win.SetPosition(b.X, b.Y+int(math.Round(float64(to-b.Y)*t)))
	}, done)
}

// animate calls step with the eased progress, from 0 to 1, at each frame
// of a 120 Hz display for d, then done on the main thread, unless another
// animation started meanwhile.
func (p *termPanel) animate(gen int64, d time.Duration, ease func(float64) float64, step func(float64), done func()) {
	start := time.Now()
	go func() {
		tick := time.NewTicker(animationFrame)
		defer tick.Stop()
		for range tick.C {
			if p.anim.Load() != gen {
				return
			}
			t := min(1, float64(time.Since(start))/float64(d))
			step(ease(t)) // the window's methods run on the main thread
			if t >= 1 {
				break
			}
		}
		if done != nil {
			p.win.Update(done)
		}
	}()
}

// easeOut is design.md's cubic-bezier(0.2, 0.8, 0.2, 1); easeIn the
// standard ease-in, cubic-bezier(0.42, 0, 1, 1).
func easeOut(t float64) float64 { return cubicBezier(0.2, 0.8, 0.2, 1, t) }
func easeIn(t float64) float64  { return cubicBezier(0.42, 0, 1, 1, t) }

// cubicBezier is the CSS timing function through (x1, y1) and (x2, y2) at
// the time x.
func cubicBezier(x1, y1, x2, y2, x float64) float64 {
	if x <= 0 || x >= 1 {
		return min(max(x, 0), 1)
	}
	b := func(a, b, t float64) float64 { return 3*a*t*(1-t)*(1-t) + 3*b*t*t*(1-t) + t*t*t }
	lo, hi := 0.0, 1.0
	for range 40 { // bisect the parameter of x
		mid := (lo + hi) / 2
		if b(x1, x2, mid) < x {
			lo = mid
		} else {
			hi = mid
		}
	}
	return b(y1, y2, (lo+hi)/2)
}

// afterCommand is what the shell of the preset pr says and does once its
// command ends (design.md 3.6, copy.md 3).
func afterCommand(pr presets.Preset) presets.After {
	before, after, _ := strings.Cut(locales.TermEndedCode(presetTitle(pr), "\x00"), "\x00")
	return presets.After{
		Failed:  [2]string{before, after},
		Missing: locales.TermCommandMissing(commandName(pr.Command)),
		Ended:   presets.AfterTitle,
	}
}

// commandName is the first word of command, the one the shell did not
// find, as it says it: "pi" of "pi --continue".
func commandName(command string) string {
	if f := strings.Fields(command); len(f) > 0 {
		return f[0]
	}
	return ""
}
