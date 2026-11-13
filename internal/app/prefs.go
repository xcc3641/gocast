package app

import (
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
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
	fontSize   float64
	folderGone bool    // the default folder is not there, as of the last look
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
	p.fontSize = float64(p.g.settings.Font())
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
		if p.page == pageShortcuts {
			// The page itself does not scroll, its list does (design.md
			// 4.4).
			ui.Column(c).Grow(1).Shrink(1).Background(k.surface).Padding(52, 28, 24, 28).Children(func() {
				p.shortcutsPage(c, k)
			})
			return
		}
		ui.Scroll(c).Grow(1).Background(k.surface).Children(func() {
			ui.Column(c).Padding(52, 28, 24, 28).Children(func() {
				switch p.page {
				case pagePresets:
					p.presetsPage(c, k)
				case pageClipboard:
					p.clipboardPage(c, k)
				case pageGeneral:
					p.generalPage(c, k)
				case pageAbout:
					p.aboutPage(c, k)
				default:
					p.shortcutsPage(c, k)
				}
			})
		})
	})
	p.editor(c, k)
	p.clearConfirmation(c, k)
}

// sidebar lists the pages, under room for the traffic lights, on the
// window's sidebar material.
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
	ui.Column(c).Width(200).Shrink(0).Padding(52, 10, 10, 10).Gap(2).Role(ui.RoleList).Label(locales.SettingsTitle()).
		Focusable().HandleInput(func(ev ui.InputEvent) bool {
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
		// Chosen in the selection gray, as the search results and the
		// terminal's tabs (design.md 4.2), not the accent.
		ui.Themed(c, listTheme(c, k)).Background(ui.Transparent).Gap(2).Children(func() {
			for _, pg := range pages {
				on := p.page == pg.id
				icon := k.textSecondary
				if on {
					icon = k.text
				}
				id := pg.id
				c.Key(id)
				ui.Row(c).Height(28).Padding(0, 8).Radius(8).Gap(8).AlignItems(ui.Center).Selected(on).
					Role(ui.RoleListItem).Label(pg.label).OnClick(func() { p.goTo(id) }).Children(func() {
					sfsymbols.Symbol(c, pg.icon).Size(16).Color(icon)
					ui.Text(c, pg.label).FontSize(13).TextColor(k.text).SingleLine()
				})
			}
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

func pageTitle(c *ui.Context, k tokens, title string) {
	ui.Text(c, title).FontSize(20).FontWeight(600).TextColor(k.text).Margin(0, 0, 16, 0).Role(ui.RoleHeading)
}

// groupTitle is the title over a group (13 Semibold), and its
// description under it (12, textSecondary), 8 over the group.
func groupTitle(c *ui.Context, k tokens, title, desc string) {
	ui.Column(c).Gap(2).Margin(0, 14, 8, 14).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
		if desc != "" {
			ui.Text(c, desc).FontSize(12).TextColor(k.textSecondary)
		}
	})
}

// group is a rounded block of rows, rows separated by hairlines.
type group struct{ rows int }

func newGroup(c *ui.Context, k tokens, rows func(g *group)) {
	g := &group{}
	ui.Column(c).Background(k.groupFill).Radius(10).Padding(0, 0, 0, 14).Margin(0, 0, 20, 0).Children(func() { rows(g) })
}

// newGroupNoted is a group with a note under it (12, textSecondary), 20
// over what follows.
func newGroupNoted(c *ui.Context, k tokens, note string, rows func(g *group)) {
	g := &group{}
	ui.Column(c).Background(k.groupFill).Radius(10).Padding(0, 0, 0, 14).Children(func() { rows(g) })
	ui.Text(c, note).FontSize(12).TextColor(k.textSecondary).Margin(6, 14, 20, 14)
}

// groupDivider is the hairline between a group's rows, inset 14 from
// both of the group's edges, as System Settings draws it (design.md 4.3):
// the group pads 14 on the left, the divider keeps 14 on the right.
func groupDivider(c *ui.Context, k tokens) {
	ui.Box(c).Height(1).Background(k.hairline).Margin(0, groupInset, 0, 0).Shrink(0)
}

// groupInset is where a group's rows and dividers start and end, from
// its edges.
const groupInset = 14

// row is a row of a group: label and a description under it at the left,
// the control at the right. A description in the warning color says a
// problem, after a warning sign.
func (g *group) row(c *ui.Context, k tokens, label, desc string, descColor ui.Color, control func()) {
	if g.rows > 0 {
		groupDivider(c, k)
	}
	g.rows++
	ui.Row(c).MinHeight(44).Padding(8, 14, 8, 0).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(13).TextColor(k.text)
			if desc != "" && descColor == k.warning {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					sfsymbols.Symbol(c, "exclamationmark.triangle").Size(12).Color(k.warning)
					ui.Text(c, desc).FontSize(12).TextColor(descColor).Shrink(1)
				})
			} else if desc != "" {
				ui.Text(c, desc).FontSize(12).TextColor(descColor)
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
	block := ui.Row(c).Background(k.groupFill).Radius(10).Padding(14).Gap(10).Margin(0, 0, 14, 0).Shrink(0).
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
	newGroupNoted(c, k, locales.GeneralTerminalGroupHint(), func(g *group) {
		p.defaultFolderRow(c, k, g)
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
		sizeHint := locales.GeneralSizeHint()
		g.row(c, k, locales.GeneralWidth(), sizeHint, k.textSecondary, func() {
			// From the least the terminal is wide on its display; the
			// width set shown no less, though kept until changed.
			lo := settings.MinTermWidth(p.g.term.workAreaWidth())
			p.termWidth = max(p.termWidth, float64(lo))
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

func (p *prefsWindow) presetsPage(c *ui.Context, k tokens) {
	pageTitle(c, k, locales.PagePresets())
	p.terminalGroup(c, k)
	groupTitle(c, k, locales.PresetsGroup(), locales.PresetsIntro())
	if len(p.g.presets) == 0 {
		ui.Column(c).Height(160).Background(k.groupFill).Radius(10).Justify(ui.Center).AlignItems(ui.Center).Gap(6).Children(func() {
			sfsymbols.Symbol(c, "apple.terminal").Size(28).Color(k.textSecondary)
			ui.Text(c, locales.PresetsEmptyTitle()).FontSize(15).FontWeight(600).TextColor(k.text)
			ui.Text(c, locales.PresetsEmptyExample()).FontSize(13).TextColor(k.textSecondary)
			ui.PrimaryButton(c, locales.PresetsAdd()).Margin(6, 0, 0, 0).OnClick(func() { p.edit("") })
		})
		return
	}
	ui.Column(c).Background(k.groupFill).Radius(10).Padding(0, 0, 0, 14).Children(func() {
		for i, pr := range p.g.presets {
			if i > 0 {
				groupDivider(c, k)
			}
			id := pr.ID
			c.Key(id)
			ui.Row(c).Height(52).Padding(0, 14, 0, 0).Gap(8).AlignItems(ui.Center).Role(ui.RoleButton).
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
						ui.Box(c).Height(28).Grow(1).Shrink(1).Padding(0, 8).Radius(6).Background(k.groupFill).
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
	pageTitle(c, k, locales.PageGeneral())
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
	ui.Column(c).MinHeight(500 - 52 - 24).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
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
			// On the group's fill: the window's (design.md 4.4).
			box := ui.Box(c).Width(240).Height(28).Padding(0, 8).Radius(6).Background(k.surface).Justify(ui.Center).
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
