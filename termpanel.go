package main

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
	termMinWidth  = 480 // as small as the panel gets, by any setting or drag
	termMinHeight = 240
	// The size until the user sets one: 80% of the work area's width up to
	// termDefaultMaxWidth, half its height, at least termDefaultMinHeight.
	termDefaultMaxWidth  = 1200
	termDefaultMinHeight = 360
	termTopGap           = 8
	termRadius           = 16
	tabBarH              = 40
	tabH                 = 26
	bannerH              = 36
	endBarH              = 44
	minFontSize          = 11
	maxFontSize          = 20
	slideIn              = 180 * time.Millisecond
	slideOut             = 120 * time.Millisecond
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

	pending    bool // to start in the next frame, which sizes it
	exited     bool
	exitCode   int
	ranCommand string // the command of this run, for "can't find the command"
	missingDir string // the folder of the preset, gone: opened in the home folder
	failure    string // the terminal could not start
	fontSize   float32
	announced  bool // the banner or end bar was read out
}

func (s *session) plain() bool { return s.presetID == "" }

func (s *session) title() string {
	if s.plain() {
		return locales.TermTabDefault()
	}
	return presetTitle(s.preset)
}

// commandMissing reports that the preset's command was not found: its
// shell ended with 127.
func (s *session) commandMissing() bool {
	return s.exited && s.exitCode == 127 && s.ranCommand != ""
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
	top    int          // the top of the work area the panel shows in

	wa      mygo.Rectangle // the work area the panel shows in
	size    [2]int         // the panel's width and height as last placed or resized
	placing bool           // the panel's bounds are being set, not dragged
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
		CornerRadius: termRadius,
		Width:        termDefaultMaxWidth,
		Height:       termDefaultMinHeight * 2,
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
		if !g.settings.KeepTerminalOnBlur && !p.menu {
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
	p.shortcuts(c)
	s := p.current()
	ui.Column(c).Fill().Children(func() {
		p.tabBar(c, k)
		ui.Column(c).Grow(1).Background(k.surface).Children(func() {
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
			p.banner(c, k, s)
			if s.term == nil {
				ui.Box(c).Grow(1) // it could not start: the banner says why
				return
			}
			c.Key(strconv.Itoa(s.key) + "#" + strconv.Itoa(s.gen))
			terminal.View(c, s.term).Grow(1).Bind(&s.focus).AutoFocus()
			if s.exited {
				p.endBar(c, k, s)
			}
		})
	})
}

// shortcuts are the panel's Command keys, which never reach the program:
// Cmd-T, Cmd-W, Cmd-Shift-[ and ], Cmd-1…9, and the font size.
func (p *termPanel) shortcuts(c *ui.Context) {
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
	if s := p.current(); s != nil {
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
				s.term.SetFont(terminal.Font{Size: size})
			}
		}
	}
}

// tabBar shows the tabs, the new tab button and its menu, and the hide
// button, on the glass.
func (p *termPanel) tabBar(c *ui.Context, k tokens) {
	ui.Row(c).Height(tabBarH).Shrink(0).Padding(7, 8, 7, 12).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.ScrollHorizontal(c).Grow(1).Shrink(1).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for i, s := range p.tabs {
					p.tab(c, k, i, s)
				}
			})
		})
		newTab := iconButton(c, k, "plus", locales.TermNewTab())
		newTab.PopoverMenuButton(p.newTabMenu, ui.PopoverMenuButtonOptions{
			At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd, Open: &p.menu,
			Panel: func(panel ui.Element) { panel.Width(240) },
		})
		hide := locales.TermHide(shortcut.Glyphs(p.g.terminalKey))
		iconButton(c, k, "chevron.up", hide).OnClick(p.hide)
	})
}

// tab is a tab of the tab bar: its title, a dot once its program ended,
// and its close button, on the tab showing and the one under the pointer.
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
	tab := ui.Row(c).Height(tabH).MinWidth(96).MaxWidth(220).Padding(0, 6, 0, 10).Radius(7).Background(bg).
		Gap(6).AlignItems(ui.Center).Tooltip(tip).Label(s.title())
	hovered := tab.Hovered()
	idx := i
	tab.OnClick(func() { p.activate(idx) }).Children(func() {
		if s.exited {
			ui.Box(c).Size(6, 6).Radius(3).Background(k.textOnGlass).Shrink(0)
		}
		ui.Text(c, s.title()).FontSize(12).FontWeight(500).TextColor(fg).SingleLine().Grow(1).Shrink(1)
		closeBox := ui.Box(c).Size(16, 16).Radius(4).Center().Shrink(0)
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

// newTabMenu lists the presets, then a plain terminal.
func (p *termPanel) newTabMenu(m *ui.Menu) {
	m.Item(locales.TermNewTabMenu()).Disabled(true)
	for _, pr := range p.g.presets {
		item := m.Item(presetTitle(pr)).Trailing(pr.LastDir())
		if item.Chosen() {
			pr := pr
			p.newTab(&pr)
		}
	}
	if len(p.g.presets) > 0 {
		m.Separator()
	}
	if m.Item(locales.TermPlain()).Chosen() {
		p.newTab(nil)
	}
}

// banner is the line over the terminal: the plain terminal's tip, the
// preset's folder that is gone, or why the terminal could not start.
func (p *termPanel) banner(c *ui.Context, k tokens, s *session) {
	var icon, text string
	color := k.textSecondary
	var dismiss func()
	addPreset := false
	switch {
	case s.failure != "":
		icon, text, color = "exclamationmark.triangle", s.failure, k.danger
	case s.missingDir != "":
		icon, text, color = "exclamationmark.triangle", locales.TermFolderMissing(s.missingDir), k.warning
		dismiss = func() { s.missingDir = "" }
	case s.plain() && len(p.g.presets) == 0 && !p.g.settings.PlainTipDismissed:
		icon, text, addPreset = "info.circle", locales.TermPlainTip(), true
		dismiss = func() {
			p.g.settings.PlainTipDismissed = true // never again
			p.g.saveSettings()
		}
	default:
		return
	}
	if !s.announced {
		s.announced = true
		c.Announce(text)
	}
	ui.Row(c).Height(bannerH).Shrink(0).Padding(0, 12).Gap(8).AlignItems(ui.Center).Background(k.groupFill).Children(func() {
		sfsymbols.Symbol(c, icon).Size(14).Color(color)
		ui.Text(c, text).FontSize(12).TextColor(k.text).SingleLine().Grow(1).Shrink(1)
		if addPreset {
			ui.Button(c, locales.PresetsAdd()).Height(24).OnClick(func() {
				p.hide()
				p.g.addPreset()
			})
		}
		if dismiss != nil {
			ui.Box(c).Size(20, 20).Radius(4).Center().Label(locales.Dismiss()).Tooltip(locales.Dismiss()).
				OnClick(dismiss).Children(func() {
				sfsymbols.Symbol(c, "xmark").Size(10).Color(k.textSecondary)
			})
		}
	})
}

// endBar says the preset's command ended, under the terminal (which keeps
// what it showed), with Close Tab and Run Again, which Enter presses, or
// Edit Preset when the command was not found.
func (p *termPanel) endBar(c *ui.Context, k tokens, s *session) {
	icon, color := "checkmark.circle", k.textSecondary
	msg := locales.TermFinished()
	if s.exitCode != 0 {
		icon, color = "exclamationmark.triangle", k.warning
		msg = locales.TermFinishedCode(s.exitCode)
	}
	missing := s.commandMissing()
	if missing {
		icon, color = "exclamationmark.triangle", k.danger
		msg = locales.TermCommandMissing(s.ranCommand)
	}
	if !s.announced {
		s.announced = true
		c.Announce(msg)
	}
	// While the command runs Enter is the program's; once it ended, Enter
	// presses the primary button (the window's shortcut comes before the
	// terminal).
	primary := c.Shortcut(0, ui.KeyEnter)
	closing := false
	ui.Column(c).Height(endBarH).Shrink(0).Background(k.groupFill).Children(func() {
		ui.Box(c).Height(1).Background(k.hairline)
		ui.Row(c).Grow(1).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			sfsymbols.Symbol(c, icon).Size(14).Color(color)
			ui.Text(c, msg).FontSize(13).TextColor(k.text).SingleLine().Grow(1).Shrink(1)
			if ui.Button(c, locales.TermCloseTab()).KeyShortcuts(ui.Cmd, ui.KeyW).Clicked() {
				closing = true
			}
			name := locales.TermRunAgain()
			if missing {
				name = locales.ActionEditPreset()
			}
			if ui.PrimaryButton(c, name).KeyShortcuts(0, ui.KeyEnter).Clicked() {
				primary = true
			}
		})
	})
	switch {
	case closing:
		p.closeTab(s)
	case primary && missing:
		p.hide()
		p.g.editPreset(s.presetID)
	case primary:
		p.restart(s)
	}
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
	s.exited, s.exitCode = false, 0
	s.ranCommand = strings.TrimSpace(pr.Command)
	s.gen++
	var term *terminal.Terminal
	opts := terminal.Options{
		Command: pr.Argv(presets.UserShell()),
		Dir:     dir,
		Env:     pr.Environ(),
		Font:    terminal.Font{Size: s.fontSize},
		Padding: terminal.Padding{X: 12, Y: 7},
		// Programs that read the clipboard themselves, as coding agents
		// taking pasted images, are offered every format of a paste (the
		// Kitty clipboard protocol, mode 5522); an image alone pastes an
		// empty bracketed paste, which Claude Code takes as its cue. Both
		// are the plugin's defaults: reads outside a paste stay refused.
		OnExit: func(code int) {
			p.win.Update(func() {
				if s.term != term {
					return
				}
				if s.plain() || s.ranCommand == "" {
					p.closeTab(s) // the shell exited, as in every terminal app
					return
				}
				s.exited, s.exitCode, s.announced = true, code, false
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

// restart runs the tab's preset again, as it is now.
func (p *termPanel) restart(s *session) {
	if s.term != nil {
		s.term.Close()
	}
	if i := presets.Find(p.g.presets, s.presetID); i >= 0 {
		s.preset = p.g.presets[i]
	}
	p.start(s)
	s.focus.Focus()
}

// openPreset shows the panel on the tab of the preset with the ID, opening
// one the first time, or always with newTab.
func (p *termPanel) openPreset(id string, newTab bool) {
	i := presets.Find(p.g.presets, id)
	if i < 0 {
		return
	}
	j := slices.IndexFunc(p.tabs, func(s *session) bool { return s.presetID == id })
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
			s.term.SetFont(terminal.Font{Size: size})
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
	b := p.win.Bounds() // as AppKit constrained it
	p.size = [2]int{b.Width, b.Height}
}

// resized saves the size the user dragged the panel to, in percent of
// the work area, rounded to the settings' steps: the next show uses it,
// and Settings shows it. Only the side that changed is saved, so that a
// side held at the panel's minimum does not drift.
func (p *termPanel) resized() {
	if p.placing || !p.shown || p.wa.Width <= 0 || p.wa.Height <= 0 {
		return
	}
	b := p.win.Bounds()
	st := &p.g.settings
	was := *st
	if b.Width != p.size[0] {
		st.TerminalWidth = termPercent(b.Width, p.wa.Width, settings.MinTerminalWidth, settings.MaxTerminalWidth)
	}
	if b.Height != p.size[1] {
		st.TerminalHeight = termPercent(b.Height, p.wa.Height, settings.MinTerminalHeight, settings.MaxTerminalHeight)
	}
	p.size = [2]int{b.Width, b.Height}
	if st.TerminalWidth != was.TerminalWidth || st.TerminalHeight != was.TerminalHeight {
		p.g.saveSettings()
		p.g.prefs.terminalSizeChanged()
	}
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

// termSize is the panel's size in a work area of waW×waH: the
// percentages of the settings, or the default size while they are unset,
// no smaller than termMinWidth×termMinHeight, no larger than the work
// area.
func termSize(waW, waH int, st settings.Settings) (w, h int) {
	if st.TerminalWidth == 0 {
		w = min(termDefaultMaxWidth, percentOf(waW, settings.DefaultTerminalWidth))
	} else {
		w = percentOf(waW, st.TermWidth())
	}
	if st.TerminalHeight == 0 {
		h = max(termDefaultMinHeight, percentOf(waH, settings.DefaultTerminalHeight))
	} else {
		h = percentOf(waH, st.TermHeight())
	}
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

// iconButton is a plain 26×26 button with a symbol, for the tab bar:
// no background, the selection color while pressed.
func iconButton(c *ui.Context, k tokens, symbol, tip string) ui.Element {
	b := ui.Box(c).Size(tabH, tabH).Radius(7).Center().Shrink(0).Label(tip).Tooltip(tip)
	if b.Pressed() {
		b.Background(k.selection)
	}
	b.Children(func() { sfsymbols.Symbol(c, symbol).Size(13).Color(k.textOnGlass) })
	return b
}
