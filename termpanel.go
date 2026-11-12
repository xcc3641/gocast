package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/shortcut"
	"gocast/locales"
)

const (
	termWidth, termHeight = 900, 520
	tabBarHeight          = 34
)

var hideIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 15-6-6-6 6"/></svg>`))

// session is a preset's tab: its terminal, which keeps running while the
// panel hides, until it ends or the user closes the tab.
type session struct {
	preset presets.Preset
	term   *terminal.Terminal
	gen    int // a new view for each run, keyed by it
	focus  ui.Handle

	pending    bool // to start in the next frame, which sizes it
	exited     bool
	exitCode   int
	notice     string // over the terminal, as a missing folder
	ranCommand string // the command of this run, for "can't find the command"
}

// termPanel is the floating terminal: a borderless panel over every app,
// with a tab for each preset the user opened.
type termPanel struct {
	g        *gocast
	win      *mygo.Window
	sessions map[string]*session // by preset name
	active   string              // the preset name of the tab showing
}

func newTermPanel(g *gocast) *termPanel {
	p := &termPanel{g: g, sessions: map[string]*session{}}
	p.win = mygo.NewWindow(mygo.WindowOptions{
		Title:        locales.TrayTerminal(),
		Panel:        true, // over every app and full-screen ones, on every Space
		Hidden:       true,
		Frameless:    true,
		Transparent:  true,
		CornerRadius: 12,
		Width:        termWidth,
		Height:       termHeight,
		MinWidth:     480,
		MinHeight:    240,
		Content:      ui.View(p.view),
	})
	// Hiding never ends the programs: Cmd-W, an Escape the terminal
	// leaves, and going elsewhere hide the panel, as Ghostty's quick
	// terminal does.
	p.win.OnClose(func(e *mygo.CloseEvent) {
		if !g.quitting {
			e.PreventDefault()
			p.win.Hide()
		}
	})
	p.win.OnBlur(p.win.Hide)
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		for _, s := range p.sessions {
			if s.term != nil {
				s.term.Close()
			}
		}
	})
	return p
}

func (p *termPanel) view(c *ui.Context) {
	t := c.Theme()
	c.Root().Background(t.Background)
	p.tabShortcuts(c)
	s := p.sessions[p.active]
	ui.Column(c).Fill().Children(func() {
		p.tabBar(c)
		ui.Divider(c)
		if s == nil {
			ui.Box(c).Grow(1)
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
		if s.notice != "" {
			ui.Row(c).Padding(6, 12).Gap(8).AlignItems(ui.Center).Background(t.Warning.Alpha(0.18)).Children(func() {
				ui.Text(c, s.notice).FontSize(12).TextColor(t.Text).Grow(1)
				if ui.Button(c, "×").Label("Dismiss").Clicked() {
					s.notice = ""
				}
			})
		}
		if s.term == nil {
			ui.Box(c).Grow(1) // it could not start: the notice says why
			return
		}
		c.Key(s.preset.Name + "#" + strconv.Itoa(s.gen))
		terminal.View(c, s.term).Grow(1).Bind(&s.focus).AutoFocus()
		if s.exited {
			p.finishedBar(c, s)
		}
	})
}

// finishedBar says the command ended, under the terminal, with Run Again
// (which Enter presses) and Close Tab.
func (p *termPanel) finishedBar(c *ui.Context, s *session) {
	t := c.Theme()
	msg := locales.TermFinished()
	if s.exitCode != 0 {
		msg = locales.TermFinishedCode(s.exitCode)
	}
	if s.exitCode == 127 && s.ranCommand != "" {
		msg = locales.TermCommandMissing(s.ranCommand)
	}
	// While the command runs Enter is the program's; once it ended, Enter
	// runs it again (the window's shortcut comes before the terminal).
	again := c.Shortcut(0, ui.KeyEnter)
	ui.Row(c).Padding(8, 12).Gap(10).AlignItems(ui.Center).Background(t.Surface).Children(func() {
		ui.Text(c, msg).FontSize(12).TextColor(t.Text).Grow(1)
		if ui.Button(c, locales.TermCloseTab()).Clicked() {
			p.closeTab(s)
			again = false
		}
		if ui.PrimaryButton(c, locales.TermRunAgain()+"  ↩").Clicked() {
			again = true
		}
	})
	if again && p.sessions[p.active] == s {
		p.restart(s)
	}
}

// tabBar shows a tab for each preset (the presets of presets.json), the
// open ones marked, and a button that hides the panel. Its empty part
// drags the panel.
func (p *termPanel) tabBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(tabBarHeight).Padding(4, 8).Gap(4).AlignItems(ui.Center).Children(func() {
		for i, pr := range p.g.presets {
			name := pr.Name
			on := name == p.active
			s := p.sessions[name]
			bg, fg := ui.Transparent, t.TextMuted
			if on {
				bg, fg = t.Surface, t.Text
			}
			label := presetTitle(pr)
			if s != nil && s.exited {
				label += " ·"
			}
			tip := presetSubtitle(pr)
			if i < 9 {
				tip += "  ⌘" + strconv.Itoa(i+1)
			}
			c.Key("tab:" + name)
			ui.Row(c).Padding(4, 12).Radius(6).Background(bg).Gap(6).AlignItems(ui.Center).Children(func() {
				if s != nil {
					dot := t.Success
					if s.exited {
						dot = t.TextMuted
					}
					ui.Box(c).Size(6, 6).Radius(3).Background(dot)
				}
				ui.Text(c, label).FontSize(12).TextColor(fg).SingleLine()
			}).Tooltip(tip).Label(label).OnClick(func() { p.open(name) })
		}
		ui.Box(c).Grow(1).FillHeight().DragWindow()
		hide := ui.Box(c).Padding(4, 6).Radius(6).Children(func() {
			ui.Icon(c, hideIcon).FontSize(14).TextColor(t.TextMuted)
		})
		hide.Tooltip(locales.TermHide(shortcut.Glyphs(p.g.terminalKey))).Label(locales.TermHide(shortcut.Glyphs(p.g.terminalKey))).OnClick(p.win.Hide)
	})
}

// tabShortcuts: Cmd-1…9 choose a tab, Cmd-Shift-[ and ] the one before
// and after, as terminal apps do. Command keys never reach the program.
func (p *termPanel) tabShortcuts(c *ui.Context) {
	digits := []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, k := range digits {
		if i < len(p.g.presets) && c.Shortcut(ui.Cmd, k) {
			p.open(p.g.presets[i].Name)
		}
	}
	step := 0
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketRight) {
		step = 1
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketLeft) {
		step = -1
	}
	if step != 0 && len(p.g.presets) > 0 {
		cur := 0
		for i, pr := range p.g.presets {
			if pr.Name == p.active {
				cur = i
			}
		}
		n := len(p.g.presets)
		p.open(p.g.presets[(cur+step+n)%n].Name)
	}
}

// start runs the session's preset in a new terminal: its command through
// the user's shell as a login and interactive shell ($SHELL -lic), in its
// folder, or the home folder when that is gone.
func (p *termPanel) start(s *session) {
	pr := s.preset
	dir := pr.WorkDir()
	s.notice = ""
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		s.notice = locales.TermFolderMissing(pr.ShortDir())
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
		Padding: terminal.Padding{X: 10, Y: 6},
		// Programs that read the clipboard themselves, as coding agents
		// taking pasted images, are offered every format of a paste (the
		// Kitty clipboard protocol, mode 5522); an image alone pastes an
		// empty bracketed paste, which Claude Code takes as its cue. Both
		// are the plugin's defaults: reads outside a paste stay refused.
		OnExit: func(code int) {
			p.win.Update(func() {
				if s.term == term {
					s.exited, s.exitCode = true, code
				}
			})
		},
	}
	t, err := terminal.New(opts)
	if err != nil {
		// No libghostty-vt (the first run downloads it), or no shell.
		log.Printf("terminal: %v", err)
		s.notice = err.Error()
		return
	}
	term = t
	s.term = t
}

// restart runs the session's preset again, from presets.json as it is now.
func (p *termPanel) restart(s *session) {
	if s.term != nil {
		s.term.Close()
	}
	for _, pr := range p.g.presets {
		if pr.Name == s.preset.Name {
			s.preset = pr
		}
	}
	p.start(s)
	s.focus.Focus()
}

// closeTab ends a tab's program and goes to another open tab, or hides
// the panel when none is left.
func (p *termPanel) closeTab(s *session) {
	name := s.preset.Name
	if s.term != nil {
		s.term.Close()
	}
	delete(p.sessions, name)
	if p.active != name {
		return
	}
	p.active = ""
	for _, pr := range p.g.presets {
		if p.sessions[pr.Name] != nil {
			p.active = pr.Name
			p.sessions[pr.Name].focus.Focus()
			return
		}
	}
	p.win.Hide()
}

// openPreset shows the panel on the tab of the preset with the ID,
// starting it the first time.
func (p *termPanel) openPreset(id string, _ bool) {
	if i := presets.Find(p.g.presets, id); i >= 0 {
		p.open(p.g.presets[i].Name)
	}
}

// presetsChanged closes the tabs of presets that are gone.
func (p *termPanel) presetsChanged() {
	for name, s := range p.sessions {
		found := false
		for _, pr := range p.g.presets {
			found = found || pr.Name == name
		}
		if !found {
			p.closeTab(s)
		}
	}
	p.win.Invalidate()
}

// open shows the panel on the preset's tab, starting it the first time.
func (p *termPanel) open(name string) {
	p.syncPresets()
	var pr *presets.Preset
	for i := range p.g.presets {
		if p.g.presets[i].Name == name {
			pr = &p.g.presets[i]
		}
	}
	if pr == nil {
		return
	}
	s := p.sessions[name]
	if s == nil {
		s = &session{preset: *pr, pending: true}
		p.sessions[name] = s
	}
	p.active = name
	if !p.win.IsVisible() {
		p.place()
		p.win.Show()
	}
	s.focus.Focus()
	// The panel draws what changed outside its frames on the next one.
	p.win.Invalidate()
}

// syncPresets reads presets.json again when it changed, and closes the
// tabs of presets that are gone.
func (p *termPanel) syncPresets() {
	if p.g.loadPresets() {
		p.g.presetsChanged()
	}
}

// place centers the panel near the top of the display with the pointer.
func (p *termPanel) place() {
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	wa := d.WorkArea
	w, h := min(termWidth, wa.Width-40), min(termHeight, wa.Height-40)
	p.win.SetSize(w, h)
	p.win.SetPosition(wa.X+(wa.Width-w)/2, wa.Y+wa.Height/12)
}

// toggle shows the panel on the tab it last showed (the first preset the
// first time), or hides it.
func (p *termPanel) toggle() {
	if p.win.IsVisible() {
		p.win.Hide()
		return
	}
	p.syncPresets()
	name := p.active
	if name == "" && len(p.g.presets) > 0 {
		name = p.g.presets[0].Name
	}
	p.open(name)
}
