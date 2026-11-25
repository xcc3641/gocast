package app

import (
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/icon"
	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// Settings pages, by their sidebar IDs (design.md 4.2).
const (
	pageShortcuts = "shortcuts"
	pagePresets   = "presets"
	pageClipboard = "clipboard"
	pageGeneral   = "general"
	pageAbout     = "about"
)

// prefsBarHeight is the band across the top of Settings, under the
// traffic lights, where the sidebar's tabs start and the page's header
// is: the window's title bar, hidden, so the view's to drag by (design.md
// 4.1).
const prefsBarHeight = 52

// The measures of a page (design.md 4.3): its padding, its content from
// pageTop, under the header; its cards, their border, their rows'
// padding; the header's frost fading in and out.
const (
	pageTop         = prefsBarHeight + 6
	pagePadX        = 28
	pageBottom      = 32
	prefsCardRadius = 12
	cardBorder      = 1
	cardPadX        = 16
	cardPadY        = 12
	frostFor        = 180 * time.Millisecond
)

// prefsWindow is the Settings window: an ordinary window, the only one that
// makes Gocast the active app, which hides rather than closes, back on the
// page it showed.
type prefsWindow struct {
	g    *gocast
	win  *mygo.Window
	page string

	// Settings › Shortcuts: the list, its recorder, and what stands wrong
	// with Gocast's own shortcuts by row (1 the search panel's, 2 the
	// terminal's, 3 the clipboard history's), as the system refused one.
	hk      hotkeyPage
	problem [4]string
	// The notice over the shortcuts that Spotlight has ⌘Space, or that
	// ⌘Space is the search panel's now (design.md 4.4).
	notice    int
	noticeGen int

	// The preset editor.
	editing       bool
	editID        string // "" for a new preset
	draft         presets.Preset
	envText       string
	envOpen       bool
	makeDefault   bool
	nameProblem   string
	nameInput     ui.Handle
	focusName     bool
	confirmDelete bool // the editor asks before deleting

	// General.
	login      bool
	autoUpdate bool // 自动检查更新 (design.md 4.5)
	searchOnly bool // the search panel opens with its search field alone
	calculator bool // the search panel calculates (design.md 2.3a)
	screenOff  bool // Keep Awake lets the screen turn off (design.md 2.10)
	termLook   int  // the floating terminal's look chosen: its index in settings.TerminalLooks
	fontSize   float64
	folderGone bool    // the default folder is not there, as of the last look
	termPos    string  // the floating terminal's position chosen (settings.TerminalPositions)
	termWidth  float64 // percent of the work area
	termHeight float64
	hideBlur   bool

	// Clipboard.
	clipKeep, clipPaste bool
	clipRetention       string // the label chosen
	clipIgnoredSel      string // the bundle ID of the ignored app chosen
	clipConfirm         bool   // asking before clearing the history

	// About: the status line read out last (design.md 6).
	updateSaid string

	// What moves the window: the sidebar, but for its tabs; the page's
	// header; and over a dialog, the band across the window's top.
	sideBar, header, bar ui.Handle
	// Where each page that scrolls is scrolled to, by its ID, and how
	// much the header's frost shows: what scrolls under it, or the
	// pointer over it, brings it in (design.md 4.3).
	scrolls map[string]*ui.ScrollState
	frostOn bool
	frost   float32

	// The pictures of 出现位置: the one playing, and the one the pointer
	// is on, since when, played yet (design.md 3.7).
	play       thumbPlay
	restPos    string
	restSince  time.Time
	restPlayed bool
}

func newPrefsWindow(g *gocast) *prefsWindow {
	p := &prefsWindow{g: g, page: pageShortcuts}
	p.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         locales.SettingsTitle(),
		Hidden:        true,
		Width:         prefsWidth,
		Height:        prefsHeight,
		DisableResize: true,
		TitleBarStyle: mygo.TitleBarHidden, // the view draws under it, the traffic lights kept
		Transparent:   true,
		Vibrancy:      mygo.VibrancySidebar,
		Content:       ui.View(p.view),
	})
	p.win.OnFocus(func() { // the default folder, gone or back meanwhile
		p.checkDefaultFolder()
		p.win.Invalidate()
	})
	p.win.OnClose(func(e *mygo.CloseEvent) {
		if g.quitting {
			return
		}
		e.PreventDefault()
		p.hide()
	})
	return p
}

// show shows Settings on page ("" for the one it showed last), centered,
// with Gocast the active app.
func (p *prefsWindow) show(page string) {
	if page != "" {
		p.page = page
	}
	if p.g.loadPresets() { // edited by hand meanwhile
		p.g.presetsChanged()
	}
	p.g.refreshAppHotkeys(true)
	if p.page == pageShortcuts {
		p.enterShortcuts()
	}
	p.login = mygo.App.OpenAtLogin()
	p.autoUpdate = p.g.settings.AutoUpdates()
	p.searchOnly = p.g.settings.SearchOnly
	p.calculator = p.g.settings.Calculator()
	p.screenOff = p.g.settings.AwakeScreenOff
	p.termLook = slices.Index(settings.TerminalLooks, p.g.settings.TermLook())
	p.fontSize = float64(p.g.settings.Font())
	p.termPos = p.g.settings.TermPosition()
	p.termWidth = float64(p.g.settings.TermWidth())
	p.termHeight = float64(p.g.settings.TermHeight())
	p.hideBlur = !p.g.settings.KeepTerminalOnBlur
	p.checkDefaultFolder()
	p.loadClipboard()
	if !p.win.IsVisible() {
		p.win.Center()
	}
	mygo.App.Focus()
	p.win.Show()
	p.win.Focus()
	p.win.Invalidate()
}

// hide hides Settings, and Gocast with it, for the keyboard to go back to
// the app the user was in.
func (p *prefsWindow) hide() {
	p.stopRecording(false)
	p.win.Hide()
	mygo.App.Hide()
}

func (p *prefsWindow) view(c *ui.Context) {
	c.SetTheme(themed(c))
	k := tokensFor(c)
	root := c.Root()
	if c.Vibrancy() {
		root.Background(ui.Transparent)
	} else {
		root.Background(k.groupFill)
	}
	if !p.isRecording() && c.Shortcut(ui.Cmd, ui.KeyW) { // recording, the recorder takes it
		p.hide()
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		p.sidebar(c, k)
		ui.Column(c).Grow(1).Shrink(1).Background(k.surface).Children(func() {
			scrolls := p.page != pageShortcuts
			if scrolls {
				c.Key("page-" + p.page)
				ui.Scroll(c).Grow(1).TrackScroll(p.scrollOf(p.page)).ScrollbarInsets(prefsBarHeight, 0, 0, 0).Children(func() {
					ui.Column(c).Padding(pageTop, pagePadX, pageBottom, pagePadX).Children(func() {
						switch p.page {
						case pagePresets:
							p.presetsPage(c, k)
						case pageClipboard:
							p.clipboardPage(c, k)
						case pageGeneral:
							p.generalPage(c, k)
						default:
							p.aboutPage(c, k)
						}
					})
				})
			} else {
				// The page itself does not scroll, its list does (design.md
				// 4.4).
				ui.Column(c).Grow(1).Shrink(1).Padding(pageTop, pagePadX, pageBottom, pagePadX).Children(func() {
					p.shortcutsPage(c, k)
				})
			}
			p.pageHeader(c, k, scrolls)
			// The line between the sidebar and the page, the page's
			// (design.md 4.2).
			ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(1).Background(k.hairline).PassThrough().ZIndex(2)
		})
	})
	p.editor(c, k)
	p.clearConfirmation(c, k)
	if p.editing || p.clipConfirm {
		p.titleBar(c)
	}
}

// scrollOf is where the page with the ID is scrolled to.
func (p *prefsWindow) scrollOf(page string) *ui.ScrollState {
	if p.scrolls == nil {
		p.scrolls = map[string]*ui.ScrollState{}
	}
	if p.scrolls[page] == nil {
		p.scrolls[page] = &ui.ScrollState{}
	}
	return p.scrolls[page]
}

// pageName is the name of the page with the ID.
func pageName(page string) string {
	switch page {
	case pagePresets:
		return locales.PagePresets()
	case pageClipboard:
		return locales.PageClipboard()
	case pageGeneral:
		return locales.PageGeneral()
	case pageAbout:
		return locales.PageAbout()
	}
	return locales.PageShortcuts()
}

// pageHeader is the bar across the top of the page, under the traffic
// lights, which stays as the page scrolls: the page's name, and on a page
// that scrolls, the frosted edge of what scrolls under it, with a hairline
// along its bottom, fading in as the page is scrolled from its top or the
// pointer is over the bar, out as neither is (design.md 4.3). It moves the
// window, as a title bar.
func (p *prefsWindow) pageHeader(c *ui.Context, k tokens, scrolls bool) {
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(prefsBarHeight).Padding(0, pagePadX).AlignItems(ui.Center).
		DragWindow().Bind(&p.header).ZIndex(2)
	p.frostOn = scrolls && (p.scrollOf(p.page).Y > 0 || bar.Hovered())
	target := float32(0)
	if p.frostOn {
		target = 1
	}
	// At once with less motion, which it is not told is no motion.
	p.frost = bar.AnimateTo("frost", target, ui.Animation{Duration: frostFor})
	bar.Children(func() {
		ui.Text(c, pageName(p.page)).FontSize(15).FontWeight(700).TextColor(k.text).SingleLine().Shrink(1).Role(ui.RoleHeading)
	})
	// The frost, under the bar's name, not inside the bar: there, it
	// would frost the name too.
	if p.frost > 0 {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(prefsBarHeight - 1).Opacity(p.frost).PassThrough().ZIndex(1).
			Material(glass.ScrollEdge{Hard: true, Background: k.surface})
		ui.Box(c).Absolute().Left(0).Right(0).Top(prefsBarHeight - 1).Height(1).Opacity(p.frost).PassThrough().ZIndex(1).
			Background(k.hairline)
	}
}

// titleBar is the band across the top of the window over a dialog's
// dimmed window, which moves the window as a title bar does: Settings
// hides its own (design.md 4.1). Without a dialog, the sidebar and the
// page's header do.
func (p *prefsWindow) titleBar(c *ui.Context) {
	ui.Overlay(c, func() {
		ui.Box(c).Absolute().Left(0).Top(0).Right(0).Height(prefsBarHeight).DragWindow().AccessHidden(true).Bind(&p.bar)
	})
}

// sidebar lists the pages, under room for the traffic lights, on the
// window's sidebar material; anywhere but on its tabs, it moves the
// window (design.md 4.2).
func (p *prefsWindow) sidebar(c *ui.Context, k tokens) {
	pages := []struct{ id, icon, label string }{
		{pageShortcuts, "keyboard", locales.PageShortcuts()},
		{pagePresets, "apple.terminal", locales.PagePresets()},
		{pageClipboard, "doc.on.clipboard", locales.PageClipboard()},
		{pageGeneral, "gearshape", locales.PageGeneral()},
		{pageAbout, "info.circle", locales.PageAbout()},
	}
	// Not ui.Sidebar, whose items take SVG icons rather than SF Symbols;
	// as it, one stop of Tab, where Up and Down choose the page.
	move := func(d int) {
		i := slices.IndexFunc(pages, func(pg struct{ id, icon, label string }) bool { return pg.id == p.page })
		if j := i + d; j >= 0 && j < len(pages) {
			p.goTo(pages[j].id)
		}
	}
	accent := c.Theme().Accent
	ui.Column(c).Width(200).Shrink(0).Padding(prefsBarHeight, 10, 10, 10).DragWindow().Bind(&p.sideBar).Children(func() {
		ui.Column(c).Role(ui.RoleList).Label(locales.SettingsTitle()).Focusable().HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyDown || ev.Mods != 0 {
				return false
			}
			switch ev.Key {
			case ui.KeyUp:
				move(-1)
			case ui.KeyDown:
				move(1)
			default:
				return false
			}
			p.win.Invalidate()
			return true
		}).Children(func() {
			// Chosen in the selection gray, as the search results and
			// the terminal's tabs, its icon in the accent (design.md 4.2).
			ui.Themed(c, sidebarTheme(c, k)).Background(ui.Transparent).Gap(2).Children(func() {
				for _, pg := range pages {
					on := p.page == pg.id
					icon := k.textSecondary
					if on {
						icon = accent
					}
					id := pg.id
					c.Key(id)
					ui.Row(c).Height(32).Padding(0, 9).Radius(8).Gap(9).AlignItems(ui.Center).Selected(on).
						Role(ui.RoleListItem).Label(pg.label).OnClick(func() { p.goTo(id) }).Children(func() {
						sfsymbols.Symbol(c, pg.icon).Size(16).Color(icon)
						ui.Text(c, pg.label).FontSize(13).FontWeight(500).TextColor(k.text).SingleLine().Shrink(1)
					})
				}
			})
		})
	})
}

func (p *prefsWindow) goTo(page string) {
	p.stopRecording(false)
	p.page = page
	if page == pageShortcuts {
		p.enterShortcuts()
	}
	if page == pageGeneral {
		p.autoUpdate = p.g.settings.AutoUpdates()
	}
	if page == pageClipboard {
		p.loadClipboard()
	}
}

// --- Layout helpers (design.md 4.3) ---

// groupTitle is the title over a card (13 Semibold), and its description
// under it (12, textSecondary), in by 4, 8 over the card.
func groupTitle(c *ui.Context, k tokens, title, desc string) {
	ui.Column(c).Gap(2).Margin(0, 4, 8, 4).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
		if desc != "" {
			ui.Text(c, desc).FontSize(12).LineHeight(1.4).TextColor(k.textSecondary)
		}
	})
}

// card is a block of Settings on the page's own white, set apart by a
// hairline around it: a card of rows, or what else stands in a block.
func card(c *ui.Context, k tokens) ui.Element {
	return ui.Column(c).Background(k.surface).Border(cardBorder, k.hairline).Radius(prefsCardRadius)
}

// group is a card of rows, rows separated by hairlines.
type group struct{ rows int }

func newGroup(c *ui.Context, k tokens, rows func(g *group)) {
	g := &group{}
	card(c, k).Margin(0, 0, 20, 0).Children(func() { rows(g) })
}

// newGroupNoted is a group with a note under it (12, textSecondary), in by
// 4, 20 over what follows.
func newGroupNoted(c *ui.Context, k tokens, note string, rows func(g *group)) {
	g := &group{}
	card(c, k).Children(func() { rows(g) })
	ui.Text(c, note).FontSize(12).LineHeight(1.4).TextColor(k.textSecondary).Margin(6, 4, 20, 4)
}

// groupDivider is the hairline between a group's rows, across the card
// (design.md 4.3).
func groupDivider(c *ui.Context, k tokens) {
	ui.Box(c).Height(1).Background(k.hairline).Shrink(0)
}

// row is a row of a group: label and a description under it at the left,
// the control at the right. A description in the warning color says a
// problem, after a warning sign.
func (g *group) row(c *ui.Context, k tokens, label, desc string, descColor ui.Color, control func()) {
	g.rowWith(c, k, label, desc, descColor, nil, control)
}

// rowWith is a row whose label has under, as a button, under its
// description.
func (g *group) rowWith(c *ui.Context, k tokens, label, desc string, descColor ui.Color, under, control func()) {
	if g.rows > 0 {
		groupDivider(c, k)
	}
	g.rows++
	ui.Row(c).MinHeight(48).Padding(cardPadY, cardPadX).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(k.text)
			if desc != "" && descColor == k.warning {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					sfsymbols.Symbol(c, "exclamationmark.triangle").Size(12).Color(k.warning)
					ui.Text(c, desc).FontSize(12).LineHeight(1.4).TextColor(descColor).Shrink(1)
				})
			} else if desc != "" {
				ui.Text(c, desc).FontSize(12).LineHeight(1.4).TextColor(descColor)
			}
			if under != nil {
				under()
			}
		})
		if control != nil {
			control()
		}
	})
}

// --- Shortcuts ---

// The notices of the Shortcuts page.
const (
	noticeNone      = iota
	noticeSpotlight // Spotlight has ⌘Space: ⌥⇧Space stands in
	noticeSolved    // ⌘Space is the search panel's now
)

// noticeSolvedFor is how long "Now using ⌘Space" shows.
const noticeSolvedFor = 3 * time.Second

// spotlightNotice is the notice over the shortcuts, on the groups' fill:
// that Spotlight has ⌘Space, with a button to System Settings and one to
// keep ⌥⇧Space; then, once the user freed ⌘Space, that the search panel
// has it, for a while (design.md 4.4).
func (p *prefsWindow) spotlightNotice(c *ui.Context, k tokens) {
	if p.notice == noticeNone {
		return
	}
	fade := &ui.Motion{Opacity: 0}
	tr := ui.ElementTransition{Exit: fade, ExitDuration: 150 * time.Millisecond}
	if c.Preferences().ReduceMotion {
		tr = ui.ElementTransition{}
	}
	c.Key("spotlightNotice")
	block := card(c, k).Row().Padding(14).Gap(10).Margin(0, 0, 14, 0).Shrink(0).
		AlignItems(ui.Start).Transition(tr)
	if p.notice == noticeSolved {
		block.AlignItems(ui.Center).Role(ui.RoleStatus).Children(func() {
			sfsymbols.Symbol(c, "checkmark.circle.fill").Size(16).Color(k.success)
			ui.Text(c, locales.ShortcutSpotlightSolved()).FontSize(13).FontWeight(600).TextColor(k.text)
		})
		return
	}
	block.Role(ui.RoleGroup).Label(locales.ShortcutSpotlightTitle()).Children(func() {
		sfsymbols.Symbol(c, "exclamationmark.triangle.fill").Size(16).Color(k.warning)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, locales.ShortcutSpotlightTitle()).FontSize(13).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
			ui.Text(c, locales.ShortcutSpotlightBody()).FontSize(12).TextColor(k.textSecondary)
			ui.Row(c).Gap(8).Margin(8, 0, 0, 0).Children(func() { // 10 under the text with the column's gap
				ui.PrimaryButton(c, locales.ShortcutSpotlightOpen()).OnClick(func() {
					if err := mygo.Shell.OpenExternal(shortcut.SpotlightURL); err != nil {
						log.Printf("system settings: %v", err)
					}
				})
				ui.Button(c, locales.ShortcutSpotlightKeep()).OnClick(func() {
					p.notice = noticeNone
					p.g.keepFallback()
					p.win.Invalidate()
				})
			})
		})
	})
}

// spotlightSolved says, in place of the notice, that the search panel
// has ⌘Space now, for noticeSolvedFor, if the notice showed.
func (p *prefsWindow) spotlightSolved() {
	if p.notice == noticeNone {
		p.win.Invalidate()
		return
	}
	p.notice = noticeSolved
	p.noticeGen++
	gen := p.noticeGen
	time.AfterFunc(noticeSolvedFor, func() {
		p.win.Update(func() {
			if p.noticeGen == gen && p.notice == noticeSolved {
				p.notice = noticeNone
			}
		})
	})
	p.win.Invalidate()
}

// --- Floating Terminal: the terminal, then its presets ---

// terminalGroup is the floating terminal's own settings, over the
// presets on their page, with the note on resizing under it (design.md
// 4.4).
func (p *prefsWindow) terminalGroup(c *ui.Context, k tokens) {
	// From the least the terminal is wide on its display; the width set
	// shown no less, though kept until changed. Before the position's
	// pictures, which draw the terminal at that width in the same frame.
	lo := settings.MinTermWidth(p.g.term.workAreaWidth())
	p.termWidth = max(p.termWidth, float64(lo))
	newGroupNoted(c, k, locales.GeneralTerminalGroupHint(), func(g *group) {
		p.defaultFolderRow(c, k, g)
		p.lookRow(c, k, g)
		g.row(c, k, locales.GeneralFontSize(), "", k.textSecondary, func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, strconv.Itoa(int(p.fontSize))).FontSize(13).Font("monospace").TextColor(k.text)
				if ui.Stepper(c, &p.fontSize, settings.MinFontSize, settings.MaxFontSize, 1).Label(locales.GeneralFontSize()).Changed() {
					p.g.settings.FontSize = int(p.fontSize)
					if p.g.settings.FontSize == settings.DefaultFontSize {
						p.g.settings.FontSize = 0
					}
					p.g.saveSettings()
					p.g.term.fontChanged()
				}
			})
		})
		p.positionRow(c, k, g)
		sizeHint := locales.GeneralSizeHint()
		g.row(c, k, locales.GeneralWidth(), sizeHint, k.textSecondary, func() {
			if p.percentSlider(c, k, locales.GeneralWidth(), &p.termWidth, lo, settings.MaxTerminalWidth) {
				p.g.settings.TerminalWidth = int(p.termWidth)
				p.g.saveSettings()
				p.g.term.sizeChanged()
			}
		})
		g.row(c, k, locales.GeneralHeight(), sizeHint, k.textSecondary, func() {
			if p.percentSlider(c, k, locales.GeneralHeight(), &p.termHeight, settings.MinTerminalHeight, settings.MaxTerminalHeight) {
				p.g.settings.TerminalHeight = int(p.termHeight)
				p.g.saveSettings()
				p.g.term.sizeChanged()
			}
		})
		g.row(c, k, locales.GeneralHideOnBlur(), locales.GeneralHideOnBlurHint(), k.textSecondary, func() {
			if ui.Switch(c, &p.hideBlur).Label(locales.GeneralHideOnBlur()).Changed() {
				p.g.settings.KeepTerminalOnBlur = !p.hideBlur
				p.g.saveSettings()
			}
		})
	})
}

// lookRow chooses what the floating terminal looks like (design.md 3.3b,
// 4.4): Ghostty or Gocast, the note under it saying what the one chosen
// is (copy.md 20). Saved, and every tab of the terminal changed, at once.
func (p *prefsWindow) lookRow(c *ui.Context, k tokens, g *group) {
	hint := locales.GeneralTermLookGhosttyHint()
	if settings.TerminalLooks[p.termLook] == settings.LookGocast {
		hint = locales.GeneralTermLookGocastHint()
	}
	g.row(c, k, locales.GeneralTermLook(), hint, k.textSecondary, func() {
		ui.Themed(c, segmentTheme(c)).Background(ui.Transparent).Shrink(0).Children(func() {
			seg := ui.Segmented(c, &p.termLook, locales.GeneralTermLookGhostty(), locales.GeneralTermLookGocast()).
				Height(24).FontSize(12).Shrink(0).Label(locales.GeneralTermLook())
			if seg.Changed() {
				p.g.settings.SetTermLook(settings.TerminalLooks[p.termLook])
				p.g.saveSettings()
				p.g.term.lookChanged()
			}
		})
	})
}

// positionLabel names the floating terminal's position pos, one of
// settings.TerminalPositions (copy.md 17).
func positionLabel(pos string) string {
	switch pos {
	case settings.PositionCenter:
		return locales.GeneralPositionCenter()
	case settings.PositionBottom:
		return locales.GeneralPositionBottom()
	case settings.PositionLeft:
		return locales.GeneralPositionLeft()
	case settings.PositionRight:
		return locales.GeneralPositionRight()
	case settings.PositionPointer:
		return locales.GeneralPositionPointer()
	}
	return locales.GeneralPositionTop()
}

// positionRow chooses where the floating terminal shows (design.md 3.7):
// its name and note over a row of the six positions' pictures, a radio
// group, each named under its picture. A picture clicked, or its name, or
// the arrows, choose it: saved at once, and the terminal showing moved
// there at once. The note says how At Pointer places it, while chosen.
func (p *prefsWindow) positionRow(c *ui.Context, k tokens, g *group) {
	if g.rows > 0 {
		groupDivider(c, k)
	}
	g.rows++
	// The note of the position chosen, as the pictures show it, in the
	// same frame; as high as the higher of the two, wrapped, for the
	// pictures under it not to move as it changes (design.md 3.7).
	hint := positionHint(p.termPos)
	lines := max(textLines(c, locales.GeneralPositionHint(), 12, positionRowWidth), textLines(c, locales.GeneralPositionPointerHint(), 12, positionRowWidth))
	hintHeight := float32(lines) * 12 * 1.4
	ui.Column(c).Padding(cardPadY, cardPadX).Gap(12).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.GeneralPosition()).FontSize(13).FontWeight(500).TextColor(k.text)
			ui.Text(c, hint).FontSize(12).LineHeight(1.4).TextColor(k.textSecondary).MinHeight(hintHeight)
		})
		p.positionPictures(c, k, hint)
	})
}

// textLines is how many lines s takes at size, wrapped at width.
func textLines(c *ui.Context, s string, size, width float32) int {
	_, line := c.MeasureText(0, ui.Span{Text: "x", Size: size})
	_, h := c.MeasureText(width, ui.Span{Text: s, Size: size})
	return max(int(math.Round(float64(h/line))), 1)
}

// positionHint is the note under 出现位置 while pos is chosen: how At
// Pointer places the terminal, else on which display (copy.md 17).
func positionHint(pos string) string {
	if pos == settings.PositionPointer {
		return locales.GeneralPositionPointerHint()
	}
	return locales.GeneralPositionHint()
}

// positionRowWidth is the width of the row of pictures, in a card on a
// page of the Settings window, which does not resize: the window less the
// sidebar and the page's padding, less the card's border and the row's
// padding; about 88 for each of six (design.md 4.3).
const positionRowWidth = prefsWidth - hkContentLeft - pagePadX - 2*cardBorder - 2*cardPadX

// positionsPerRow is how many pictures a row of them holds: all of them,
// or half where a cell would be under pictureCell wide, from the row's
// width, which is fixed; not as laid out in the last frame, which the
// first frame of the page doesn't have, or has of another page's element.
func positionsPerRow(n int) int {
	if positionRowWidth/float32(n) < pictureCell {
		return (n + 1) / 2
	}
	return n
}

// positionPictures is the radio group of the positions' pictures, across
// the row in cells of equal width, in two rows of three where a cell
// would be under pictureCell wide. The arrows choose the next or the one
// before, stopping at the ends. The picture chosen plays, and one the
// pointer rests on (design.md 3.7).
func (p *prefsWindow) positionPictures(c *ui.Context, k tokens, hint string) {
	positions := settings.TerminalPositions
	perRow := positionsPerRow(len(positions))
	focusedAt := -1
	now, on := c.Now(), ""
	cell := func(i int, pos string) {
		name := positionLabel(pos)
		c.Key("position-" + pos)
		item := ui.RadioBase(c, &p.termPos, pos).Column().Grow(1).Basis(0).AlignItems(ui.Center).Gap(nameGap).
			FocusRing(false).Cursor(ui.CursorDefault).Label(name)
		if item.Changed() {
			p.g.settings.SetTermPosition(p.termPos)
			p.g.saveSettings()
			p.g.term.positionChanged()
			p.choosePicture(c, pos)
		}
		if item.Focused() {
			focusedAt = i
		}
		chosen := p.termPos == pos
		item.Children(func() {
			pose := func() thumbPose { return p.play.poseOf(pos, now) }
			if p.positionPicture(c, pos, chosen, item.FocusVisible(), item.Pressed(), pose) {
				on = pos
			}
			t := ui.Text(c, name).FontSize(12).FixedLineHeight(nameLine).SingleLine().AccessHidden(true)
			if chosen {
				t.FontWeight(500).TextColor(k.text)
			} else {
				t.TextColor(k.textSecondary)
			}
		})
	}
	group := ui.RadioGroup(c, func() {
		for start := 0; start < len(positions); start += perRow {
			ui.Row(c).Children(func() {
				for i := start; i < min(start+perRow, len(positions)); i++ {
					cell(i, positions[i])
				}
			})
		}
	}).Gap(pictureRowGap).Label(locales.GeneralPosition()).Description(hint)
	p.restOn(c, on)
	p.playOn(c)
	// The group goes round from the last to the first: not here.
	switch focusedAt {
	case 0:
		group.Shortcut(0, ui.KeyLeft)
		group.Shortcut(0, ui.KeyUp)
	case len(positions) - 1:
		group.Shortcut(0, ui.KeyRight)
		group.Shortcut(0, ui.KeyDown)
	}
}

func (p *prefsWindow) presetsPage(c *ui.Context, k tokens) {
	p.terminalGroup(c, k)
	groupTitle(c, k, locales.PresetsGroup(), locales.PresetsIntro())
	if len(p.g.presets) == 0 {
		card(c, k).Height(160).Justify(ui.Center).AlignItems(ui.Center).Gap(6).Children(func() {
			sfsymbols.Symbol(c, "apple.terminal").Size(28).Color(k.textSecondary)
			ui.Text(c, locales.PresetsEmptyTitle()).FontSize(15).FontWeight(600).TextColor(k.text)
			ui.Text(c, locales.PresetsEmptyExample()).FontSize(13).TextColor(k.textSecondary)
			ui.PrimaryButton(c, locales.PresetsAdd()).Margin(6, 0, 0, 0).OnClick(func() { p.edit("") })
		})
		return
	}
	card(c, k).Children(func() {
		for i, pr := range p.g.presets {
			if i > 0 {
				groupDivider(c, k)
			}
			id := pr.ID
			c.Key(id)
			ui.Row(c).Height(52).Padding(0, cardPadX).Gap(8).AlignItems(ui.Center).Role(ui.RoleButton).
				Label(presetTitle(pr) + "，" + p.g.presetSubtitle(pr)).OnClick(func() { p.edit(id) }).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, presetTitle(pr)).FontSize(13).FontWeight(500).TextColor(k.text).SingleLine().Shrink(1)
						if pr.Default {
							ui.Box(c).Height(18).Padding(0, 6).Radius(9).Background(k.keycap).Center().Children(func() {
								ui.Text(c, locales.PresetDefaultBadge()).FontSize(11).TextColor(k.textSecondary)
							})
						}
					})
					ui.Text(c, p.g.presetSubtitle(pr)).FontSize(12).TextColor(k.textSecondary).SingleLine().TruncateMiddle()
				})
				sfsymbols.Symbol(c, "chevron.right").Size(12).Color(k.textSecondary)
			})
		}
	})
	ui.Row(c).Margin(12, 0, 0, 0).Children(func() {
		ui.Button(c, "+ "+locales.PresetsAdd()).OnClick(func() { p.edit("") })
	})
}

// edit opens the editor of the preset with the ID, or of a new one.
func (p *prefsWindow) edit(id string) {
	p.editID, p.draft = "", presets.Preset{}
	if i := presets.Find(p.g.presets, id); i >= 0 {
		p.editID, p.draft = id, p.g.presets[i]
	}
	p.envText = presets.FormatEnv(p.draft.Env)
	p.envOpen = p.envText != ""
	p.makeDefault = p.draft.Default
	p.nameProblem = ""
	p.editing, p.focusName, p.confirmDelete = true, true, false
	p.win.Invalidate()
}

// editor is the dialog editing a preset, over Settings.
func (p *prefsWindow) editor(c *ui.Context, k tokens) {
	if !p.editing {
		return
	}
	submit := false // Enter in a single-line field saves
	ui.Modal(c, &p.editing, func() {
		if p.confirmDelete {
			p.deleteConfirmation(c, k)
			return
		}
		ui.Column(c).Width(460).Padding(20).Gap(16).Children(func() {
			ui.Form(c, func() {
				ui.Field(c, locales.PresetName(), func() {
					in := ui.TextInput(c, &p.draft.Name).Placeholder(locales.PresetNamePlaceholder()).Bind(&p.nameInput)
					if p.focusName {
						p.focusName = false
						in.Focus()
					}
					submit = submit || in.Submitted()
				}).Error(p.nameProblem)
				ui.Field(c, locales.PresetFolder(), func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Height(28).Grow(1).Shrink(1).Padding(0, 8).Radius(6).Background(k.cardFill).Border(1, k.hairline).
							Justify(ui.Center).Children(func() {
							if strings.TrimSpace(p.draft.Dir) != "" {
								ui.Text(c, presets.ShortPath(p.draft.Dir)).FontSize(12).Font("monospace").TextColor(k.text).SingleLine().TruncateMiddle()
							} else { // the default folder, which it starts in
								ui.Text(c, locales.PresetFolderDefault(presets.ShortPath(p.g.settings.TerminalDir))).FontSize(12).TextColor(k.textSecondary).SingleLine().TruncateMiddle()
							}
						})
						ui.Button(c, locales.PresetChoose()).OnClick(p.chooseFolder)
					})
				})
				ui.Field(c, locales.PresetCommand(), func() {
					cmd := ui.TextInput(c, &p.draft.Command).Font("monospace").FontSize(13).Placeholder(locales.PresetCommandPlaceholder())
					submit = submit || cmd.Submitted()
				})
				ui.Field(c, "", func() {
					ui.Collapsible(c, locales.PresetEnv(), &p.envOpen, func() {
						ui.TextArea(c, &p.envText).Label(locales.PresetEnv()).Font("monospace").FontSize(12).Lines(4, 4).Placeholder(locales.PresetEnvHint())
					})
				})
				ui.Field(c, "", func() {
					ui.Checkbox(c, &p.makeDefault, locales.PresetMakeDefault())
				}).Description(locales.PresetDefaultHint())
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if p.editID != "" {
					ui.Text(c, locales.PresetDelete()).FontSize(13).TextColor(k.danger).Role(ui.RoleButton).
						Label(locales.PresetDelete()).OnClick(func() { p.confirmDelete = true })
				}
				ui.Spacer(c)
				ui.Button(c, locales.Cancel()).OnClick(func() { p.editing = false })
				ui.PrimaryButton(c, locales.Save()).KeyShortcuts(0, ui.KeyEnter).OnClick(p.save)
			})
		})
	})
	if !p.editing && p.confirmDelete { // Escape: back to the editor
		p.editing, p.confirmDelete = true, false
	}
	if submit {
		p.save()
	}
}

// deleteConfirmation asks before deleting the preset, in place of the
// editor: an alert, but with Delete in danger red, and Cancel the default,
// as for any destructive choice on the Mac.
func (p *prefsWindow) deleteConfirmation(c *ui.Context, k tokens) {
	ui.Column(c).Width(300).Padding(20).Gap(8).Role(ui.RoleAlertDialog).
		Label(locales.PresetDeleteTitle(presetTitle(p.draft))).Children(func() {
		ui.Text(c, locales.PresetDeleteTitle(presetTitle(p.draft))).FontSize(13).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
		ui.Text(c, locales.PresetDeleteMessage()).FontSize(12).TextColor(k.textSecondary)
		ui.Row(c).Margin(12, 0, 0, 0).Gap(8).Children(func() {
			ui.Spacer(c)
			ui.Button(c, locales.Cancel()).AutoFocus().OnClick(func() { p.confirmDelete = false })
			ui.Button(c, locales.Delete()).TextColor(k.danger).OnClick(p.delete)
		})
	})
}

func (p *prefsWindow) chooseFolder() {
	if dir, ok := p.pickFolder(p.g.workDir(p.draft)); ok {
		p.draft.Dir = dir
		p.win.Invalidate()
	}
}

// pickFolder asks for a folder with the system's dialog, from start: the
// one chosen, as the user reads it (~/Work), and whether there was one.
func (p *prefsWindow) pickFolder(start string) (string, bool) {
	if !isDir(start) {
		start = presets.ExpandHome("~")
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: p.win, Directory: true, DefaultPath: start, CreateDirectories: true, ButtonLabel: locales.DialogChoose()})
	if err != nil {
		log.Print(err)
		return "", false
	}
	if len(paths) == 0 {
		return "", false
	}
	return presets.ShortPath(paths[0]), true
}

// save checks the draft and saves it, or says what to fix under its
// field, the focus in the first.
func (p *prefsWindow) save() {
	d := p.draft
	d.Name = strings.TrimSpace(d.Name)
	d.Command = strings.TrimSpace(d.Command)
	d.ID = p.editID
	name := presets.Validate(d, p.g.presets)
	p.nameProblem = ""
	switch name {
	case presets.NameEmpty:
		p.nameProblem = locales.PresetNameEmpty()
	case presets.NameTaken:
		p.nameProblem = locales.PresetNameTaken(d.Name)
	}
	if p.nameProblem != "" {
		p.nameInput.Focus()
	}
	if p.nameProblem != "" {
		return
	}
	if d.ID == "" {
		d.ID = presets.NewID()
	}
	d.Env = presets.ParseEnv(p.envText)
	d.Default = p.makeDefault
	ps := slices.Clone(p.g.presets)
	if d.Default {
		for i := range ps {
			ps[i].Default = false
		}
	}
	if i := presets.Find(ps, d.ID); i >= 0 {
		ps[i] = d
	} else {
		ps = append(ps, d)
	}
	if err := p.g.savePresets(ps); err != nil {
		log.Printf("presets: %v", err)
		return
	}
	p.editing = false
}

func (p *prefsWindow) delete() {
	p.confirmDelete = false
	ps := slices.Clone(p.g.presets)
	if i := presets.Find(ps, p.editID); i >= 0 {
		ps = slices.Delete(ps, i, i+1)
	}
	if err := p.g.savePresets(ps); err != nil {
		log.Printf("presets: %v", err)
		return
	}
	p.editing = false
}

// --- General ---

// searchOnlyHint is the note under "Show only the search field", with the
// search panel's shortcut as registered, else as set (copy.md 4.3).
func searchOnlyHint(registered, set string) string {
	if registered == "" {
		registered = set
	}
	return locales.GeneralSearchOnlyHint(shortcut.Glyphs(registered))
}

func (p *prefsWindow) generalPage(c *ui.Context, k tokens) {
	newGroup(c, k, func(g *group) {
		g.row(c, k, locales.GeneralLogin(), locales.GeneralLoginHint(), k.textSecondary, func() {
			if ui.Switch(c, &p.login).Label(locales.GeneralLogin()).Changed() {
				if err := mygo.App.SetOpenAtLogin(p.login); err != nil {
					log.Printf("open at login: %v", err)
					if p.g.loadPresets() { // edited by hand meanwhile
						p.g.presetsChanged()
					}
					p.login = mygo.App.OpenAtLogin()
				}
			}
		})
		// Off: checks only as asked; a version downloaded stays ready
		// (design.md 4.4, 4.5).
		g.row(c, k, locales.GeneralAutoUpdate(), locales.GeneralAutoUpdateHint(), k.textSecondary, func() {
			if ui.Switch(c, &p.autoUpdate).Label(locales.GeneralAutoUpdate()).Changed() {
				p.g.setAutoUpdates(p.autoUpdate)
			}
		})
		// Taken as the panel next shows (design.md 4.4).
		g.row(c, k, locales.GeneralSearchOnly(), searchOnlyHint(p.g.launcherKey, p.g.settings.Launcher()), k.textSecondary, func() {
			if ui.Switch(c, &p.searchOnly).Label(locales.GeneralSearchOnly()).Changed() {
				p.g.settings.SearchOnly = p.searchOnly
				p.g.saveSettings()
			}
		})
		// Off: no card, no fetching; the rates kept stay (design.md 4.4).
		g.row(c, k, locales.GeneralCalculator(), locales.GeneralCalculatorHint(), k.textSecondary, func() {
			if ui.Switch(c, &p.calculator).Label(locales.GeneralCalculator()).Changed() {
				p.g.calc.setOn(p.calculator)
			}
		})
	})
	// Keep Awake's one switch, a group of its own with no title: at once,
	// to a session running too, saying nothing (design.md 2.10).
	newGroup(c, k, func(g *group) {
		g.row(c, k, locales.AwakeScreenOff(), locales.AwakeScreenOffNote(), k.textSecondary, func() {
			if ui.Switch(c, &p.screenOff).Label(locales.AwakeScreenOff()).Changed() && !p.g.setAwakeScreenOff(p.screenOff) {
				p.screenOff = p.g.settings.AwakeScreenOff
			}
		})
	})
	newGroup(c, k, func(g *group) {
		g.row(c, k, locales.GeneralAppearance(), "", k.textSecondary, func() {
			ui.Text(c, locales.GeneralAppearanceHint()).FontSize(12).TextColor(k.textSecondary)
		})
	})
}

// percentSlider is a slider of the floating terminal's size, in steps of
// settings.SizeStep, with its value at its left, as the font size's; it
// reports a new value.
func (p *prefsWindow) percentSlider(c *ui.Context, k tokens, label string, value *float64, lo, hi int) bool {
	changed := false
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.GeneralPercent(int(*value))).FontSize(13).Font("monospace").TextColor(k.text).
			Width(40).TextAlign(ui.End)
		changed = ui.Slider(c, value, float64(lo), float64(hi)).Step(settings.SizeStep).Width(200).Label(label).Changed()
	})
	if changed {
		*value = float64(settings.SnapPercent(*value, lo, hi))
	}
	return changed
}

// terminalSizeChanged shows the size the floating terminal was dragged to.
func (p *prefsWindow) terminalSizeChanged() {
	p.termWidth = float64(p.g.settings.TermWidth())
	p.termHeight = float64(p.g.settings.TermHeight())
	p.win.Invalidate()
}

// --- About ---

// aboutPage is About: the app icon, name, tagline and version, then the
// button of updates and the status line under it, which says what the
// check found on the page (design.md 4.4, 4.5).
func (p *prefsWindow) aboutPage(c *ui.Context, k tokens) {
	st := p.g.updates.about()
	// In the room under the header (design.md 4.3).
	ui.Column(c).MinHeight(prefsHeight - pageTop - pageBottom).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		// The app icon, icon.png at 64 (design.md 1.6, 4.2).
		ui.Image(c, appIcon()).Size(64, 64).Shrink(0)
		ui.Text(c, appName).FontSize(20).FontWeight(600).TextColor(k.text).Margin(12, 0, 0, 0)
		ui.Text(c, locales.AboutTagline()).FontSize(13).TextColor(k.textSecondary)
		ui.Text(c, locales.AboutVersion(appVersion())).FontSize(12).TextColor(k.textSecondary).Margin(4, 0, 16, 0)
		p.aboutButton(c, st)
		p.aboutStatusLine(c, k, st)
	})
}

// iconPNG is the app icon, 1024 px; icon/make-icon.py draws it.
var iconPNG = icon.App

var appIconBitmap *ui.Bitmap

// appIcon is iconPNG as a bitmap, made once; MyGo draws it at 64 from a
// halved copy, smoothly.
func appIcon() *ui.Bitmap {
	if appIconBitmap == nil {
		appIconBitmap, _ = ui.DecodeBitmap(iconPNG)
	}
	return appIconBitmap
}

// appVersion is the packaged app's version, "dev" for a build not
// packaged (go run, go build).
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "dev"
}

// --- Opening Settings ---

// openSettings shows Settings on a page ("" for the last one shown).
func (g *gocast) openSettings(page string) { g.prefs.show(page) }

// editPreset shows Settings with the editor of the preset with the ID.
func (g *gocast) editPreset(id string) {
	g.prefs.show(pagePresets)
	g.prefs.edit(id)
}

// addPreset shows Settings with the editor of a new preset.
func (g *gocast) addPreset() {
	g.prefs.show(pagePresets)
	g.prefs.edit("")
}

// defaultPreset returns the default preset, or nil for a plain terminal.
func (g *gocast) defaultPreset() *presets.Preset {
	if i := presets.DefaultPreset(g.presets); i >= 0 {
		p := g.presets[i]
		return &p
	}
	return nil
}

// defaultFolderRow is the first row of the terminal's group (design.md
// 4.4): the folder new terminals start in, and Choose….
func (p *prefsWindow) defaultFolderRow(c *ui.Context, k tokens, g *group) {
	dir := p.g.settings.TerminalDir
	home := strings.TrimSpace(dir) == "" || presets.ExpandHome(dir) == presets.ExpandHome("~")
	hint, hintColor := locales.GeneralDefaultFolderHint(), k.textSecondary
	if p.folderGone {
		hint, hintColor = locales.GeneralDefaultFolderMissing(), k.warning
	}
	g.row(c, k, locales.GeneralDefaultFolder(), hint, hintColor, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			// A small box on the card, not a card (design.md 4.3).
			box := ui.Box(c).Width(240).Height(28).Padding(0, 8).Radius(6).Background(k.cardFill).Border(1, k.hairline).Justify(ui.Center).
				OnClick(p.chooseDefaultFolder)
			if home {
				box.Children(func() {
					ui.Text(c, locales.GeneralDefaultFolderHome()).FontSize(12).TextColor(k.text).SingleLine()
				})
				return
			}
			box.Tooltip(presets.ExpandHome(dir)).Children(func() {
				t := ui.Text(c, presets.ShortPath(dir)).FontSize(12).Font("monospace").SingleLine().TruncateMiddle()
				if p.folderGone {
					t.TextColor(k.textSecondary).Strikethrough()
				} else {
					t.TextColor(k.text)
				}
			})
		})
		ui.Button(c, locales.PresetChoose()).Label(locales.GeneralDefaultFolder()).OnClick(p.chooseDefaultFolder)
	})
}

// chooseDefaultFolder asks for the default folder, from the one it is;
// choosing the home folder sets it back.
func (p *prefsWindow) chooseDefaultFolder() {
	dir, ok := p.pickFolder(p.g.workDir(presets.Preset{}))
	if !ok {
		return
	}
	if dir == "~" {
		dir = ""
	}
	p.g.settings.TerminalDir = dir
	p.g.saveSettings()
	p.checkDefaultFolder()
	p.win.Invalidate()
}

// checkDefaultFolder looks whether the default folder is there: as
// Settings shows, and each time it comes to the front.
func (p *prefsWindow) checkDefaultFolder() {
	dir := p.g.settings.TerminalDir
	p.folderGone = strings.TrimSpace(dir) != "" && !isDir(presets.ExpandHome(dir))
}
