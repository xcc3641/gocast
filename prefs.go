package main

import (
	"context"
	"errors"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// Settings pages, by their sidebar IDs (design.md 4.2).
const (
	pageShortcuts = "shortcuts"
	pagePresets   = "presets"
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

	// The shortcut recorder: the row recording (0 for none, 1 the search
	// panel's, 2 the terminal's), and each row's problem.
	recording int
	problem   [3]string
	recorder  [3]ui.Handle

	// The preset editor.
	editing       bool
	editID        string // "" for a new preset
	draft         presets.Preset
	envText       string
	envOpen       bool
	makeDefault   bool
	nameProblem   string
	dirProblem    string
	nameInput     ui.Handle
	focusName     bool
	confirmDelete bool // the editor asks before deleting

	// General.
	login    bool
	fontSize float64
	hideBlur bool

	// About.
	checking     bool
	status       string
	statusDanger bool
	statusGen    int
	update       *mygo.Update
	updateOpen   bool
}

func newPrefsWindow(g *gocast) *prefsWindow {
	p := &prefsWindow{g: g, page: pageShortcuts}
	p.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         locales.SettingsTitle(),
		Hidden:        true,
		Width:         720,
		Height:        500,
		DisableResize: true,
		TitleBarStyle: mygo.TitleBarHidden, // the view draws under it, the traffic lights kept
		Transparent:   true,
		Vibrancy:      mygo.VibrancySidebar,
		Content:       ui.View(p.view),
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
	p.login = mygo.App.OpenAtLogin()
	p.fontSize = float64(p.g.settings.Font())
	p.hideBlur = !p.g.settings.KeepTerminalOnBlur
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
	p.stopRecording()
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
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		p.hide()
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		p.sidebar(c, k)
		ui.Scroll(c).Grow(1).Background(k.surface).Children(func() {
			ui.Column(c).Padding(52, 28, 24, 28).Children(func() {
				switch p.page {
				case pagePresets:
					p.presetsPage(c, k)
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
	if p.update != nil {
		switch ui.AlertDialog(c, &p.updateOpen, locales.AboutUpdateTitle(p.update.Version), "",
			locales.AboutUpdateLater(), locales.AboutUpdateRestart()) {
		case 1:
			p.install(p.update)
		case 0, ui.AlertDismissed:
			p.update = nil
		}
	}
}

// sidebar lists the pages, under room for the traffic lights, on the
// window's sidebar material.
func (p *prefsWindow) sidebar(c *ui.Context, k tokens) {
	pages := []struct{ id, icon, label string }{
		{pageShortcuts, "keyboard", locales.PageShortcuts()},
		{pagePresets, "apple.terminal", locales.PagePresets()},
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
		for _, pg := range pages {
			on := p.page == pg.id
			fg := k.text
			if on {
				fg = c.Theme().AccentText
			}
			id := pg.id
			c.Key(id)
			ui.Row(c).Height(28).Padding(0, 8).Radius(6).Gap(8).AlignItems(ui.Center).Selected(on).
				Role(ui.RoleListItem).Label(pg.label).OnClick(func() { p.goTo(id) }).Children(func() {
				sfsymbols.Symbol(c, pg.icon).Size(16).Color(fg)
				ui.Text(c, pg.label).FontSize(13).TextColor(fg).SingleLine()
			})
		}
	})
}

func (p *prefsWindow) goTo(page string) {
	p.stopRecording()
	p.page = page
}

// --- Layout helpers (design.md 4.3) ---

func pageTitle(c *ui.Context, k tokens, title string) {
	ui.Text(c, title).FontSize(20).FontWeight(600).TextColor(k.text).Margin(0, 0, 16, 0).Role(ui.RoleHeading)
}

// group is a rounded block of rows, rows separated by hairlines.
type group struct{ rows int }

func newGroup(c *ui.Context, k tokens, rows func(g *group)) {
	g := &group{}
	ui.Column(c).Background(k.groupFill).Radius(10).Padding(0, 0, 0, 14).Margin(0, 0, 20, 0).Children(func() { rows(g) })
}

// row is a row of a group: label and a description under it at the left,
// the control at the right.
func (g *group) row(c *ui.Context, k tokens, label, desc string, descColor ui.Color, control func()) {
	if g.rows > 0 {
		ui.Box(c).Height(1).Background(k.hairline)
	}
	g.rows++
	ui.Row(c).MinHeight(44).Padding(8, 14, 8, 0).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(13).TextColor(k.text)
			if desc != "" {
				ui.Text(c, desc).FontSize(12).TextColor(descColor)
			}
		})
		if control != nil {
			control()
		}
	})
}

// --- Shortcuts ---

func (p *prefsWindow) shortcutsPage(c *ui.Context, k tokens) {
	pageTitle(c, k, locales.PageShortcuts())
	newGroup(c, k, func(g *group) {
		p.shortcutRow(c, k, g, 1)
		p.shortcutRow(c, k, g, 2)
	})
}

// shortcutRow is a shortcut's row: its recorder, and Reset to Default when
// it is not the default.
func (p *prefsWindow) shortcutRow(c *ui.Context, k tokens, g *group, row int) {
	label, current, def := locales.ShortcutLauncher(), p.g.launcherKey, settings.DefaultLauncherShortcut
	if row == 2 {
		label, current, def = locales.ShortcutTerminal(), p.g.terminalKey, settings.DefaultTerminalShortcut
	}
	if current == "" { // the system refused it: what the settings ask for
		current = p.g.settings.Launcher()
		if row == 2 {
			current = p.g.settings.Terminal()
		}
	}
	desc, descColor := "", k.textSecondary
	switch {
	case p.recording == row:
		desc = locales.ShortcutRecordingHint()
	case p.problem[row] != "":
		desc, descColor = p.problem[row], k.danger
	}
	g.row(c, k, label, desc, descColor, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if current != def && p.recording != row {
				ui.Text(c, locales.ShortcutReset()).FontSize(12).TextColor(c.Theme().Accent).
					Role(ui.RoleButton).Label(locales.ShortcutReset()).OnClick(func() { p.apply(row, def) })
			}
			if p.problem[row] != "" && p.recording != row {
				ui.Box(c).Size(6, 6).Radius(3).Background(k.danger)
			}
			c.Key("recorder") // the same element as Reset comes and goes, for the focus
			box := ui.Row(c).Height(28).MinWidth(140).Padding(0, 6).Radius(6).Background(k.surface).
				Justify(ui.Center).AlignItems(ui.Center).Focusable().Bind(&p.recorder[row]).
				Role(ui.RoleButton).Label(label + " " + shortcut.Glyphs(current))
			switch {
			case p.recording == row:
				box.Border(2, c.Theme().Accent)
			case p.problem[row] != "":
				box.Border(1, k.danger)
			default:
				box.Border(1, k.hairline)
			}
			box.OnClick(func() { p.startRecording(row) })
			box.HandleInput(func(ev ui.InputEvent) bool { return p.recordKey(row, ev) })
			box.Children(func() {
				if p.recording == row {
					ui.Text(c, locales.ShortcutRecording()).FontSize(12).TextColor(k.textSecondary).SingleLine()
					return
				}
				keycaps(c, k, shortcut.Caps(current), 13)
			})
		})
	})
}

// startRecording records the row's new shortcut, the global shortcuts off
// meanwhile, so that pressing the current ones reaches the recorder.
func (p *prefsWindow) startRecording(row int) {
	if p.recording == row {
		return
	}
	if p.recording == 0 {
		p.g.suspendShortcuts()
	}
	p.recording = row
	p.problem[row] = ""
	p.recorder[row].Focus()
}

func (p *prefsWindow) stopRecording() {
	if p.recording != 0 {
		p.recording = 0
		p.g.resumeShortcuts()
	}
}

// recordKey takes the keys pressed in a recorder while it records: Escape
// cancels, a valid shortcut is saved at once.
func (p *prefsWindow) recordKey(row int, ev ui.InputEvent) bool {
	if p.recording != row || ev.Kind != ui.InputKeyDown {
		return false
	}
	if ev.Key == ui.KeyEscape && ev.Mods == 0 {
		p.stopRecording()
		return true
	}
	accel, ok := shortcut.FromKey(ev.Mods, ev.Key)
	if !ok {
		return true // a modifier alone, or a key that needs one: keep recording
	}
	p.recording = 0
	p.apply(row, accel)
	return true
}

// apply makes accel the row's shortcut, unless the other row has it or
// another app does; the shortcut stays as it was then, and the row says
// why. It resumes the global shortcuts.
func (p *prefsWindow) apply(row int, accel string) {
	other, otherLabel := p.g.terminalKey, locales.ShortcutTerminal()
	if row == 2 {
		other, otherLabel = p.g.launcherKey, locales.ShortcutLauncher()
	}
	p.g.suspendShortcuts()
	switch {
	case strings.EqualFold(accel, other):
		p.g.resumeShortcuts()
		p.problem[row] = locales.ShortcutUsedFor(otherLabel)
	case p.g.setShortcut(row == 2, accel) != nil:
		p.problem[row] = locales.ShortcutTaken()
	default:
		p.problem[row] = ""
	}
	p.win.Invalidate()
}

// --- Presets ---

func (p *prefsWindow) presetsPage(c *ui.Context, k tokens) {
	pageTitle(c, k, locales.PagePresets())
	ui.Text(c, locales.PresetsIntro()).FontSize(12).TextColor(k.textSecondary).Margin(0, 0, 8, 0)
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
				ui.Box(c).Height(1).Background(k.hairline)
			}
			id := pr.ID
			c.Key(id)
			ui.Row(c).Height(52).Padding(0, 14, 0, 0).Gap(8).AlignItems(ui.Center).Role(ui.RoleButton).
				Label(presetTitle(pr) + "，" + presetSubtitle(pr)).OnClick(func() { p.edit(id) }).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, presetTitle(pr)).FontSize(13).FontWeight(500).TextColor(k.text).SingleLine().Shrink(1)
						if pr.Default {
							ui.Box(c).Height(18).Padding(0, 6).Radius(9).Background(k.keycap).Center().Children(func() {
								ui.Text(c, locales.PresetDefaultBadge()).FontSize(11).TextColor(k.textSecondary)
							})
						}
					})
					ui.Text(c, presetSubtitle(pr)).FontSize(12).TextColor(k.textSecondary).SingleLine().TruncateMiddle()
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
	p.nameProblem, p.dirProblem = "", ""
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
						dir := ""
						if strings.TrimSpace(p.draft.Dir) != "" {
							dir = presets.ShortPath(p.draft.Dir)
						}
						ui.Box(c).Height(28).Grow(1).Shrink(1).Padding(0, 8).Radius(6).Background(k.groupFill).
							Justify(ui.Center).Children(func() {
							ui.Text(c, dir).FontSize(12).Font("monospace").TextColor(k.text).SingleLine().TruncateMiddle()
						})
						ui.Button(c, locales.PresetChoose()).OnClick(p.chooseFolder)
					})
				}).Error(p.dirProblem)
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
	start := presets.ExpandHome(p.draft.Dir)
	if strings.TrimSpace(p.draft.Dir) == "" {
		start = presets.ExpandHome("~")
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: p.win, Directory: true, DefaultPath: start, CreateDirectories: true})
	if err != nil {
		log.Print(err)
		return
	}
	if len(paths) > 0 {
		p.draft.Dir = presets.ShortPath(paths[0])
		p.dirProblem = ""
		p.win.Invalidate()
	}
}

// save checks the draft and saves it, or says what to fix under its
// field, the focus in the first.
func (p *prefsWindow) save() {
	d := p.draft
	d.Name = strings.TrimSpace(d.Name)
	d.Command = strings.TrimSpace(d.Command)
	d.ID = p.editID
	name, dir := presets.Validate(d, p.g.presets)
	p.nameProblem, p.dirProblem = "", ""
	switch name {
	case presets.NameEmpty:
		p.nameProblem = locales.PresetNameEmpty()
	case presets.NameTaken:
		p.nameProblem = locales.PresetNameTaken(d.Name)
	}
	if dir == presets.DirEmpty {
		p.dirProblem = locales.PresetFolderEmpty()
	}
	if p.nameProblem != "" {
		p.nameInput.Focus()
	}
	if p.nameProblem != "" || p.dirProblem != "" {
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
	})
	newGroup(c, k, func(g *group) {
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
		g.row(c, k, locales.GeneralHideOnBlur(), locales.GeneralHideOnBlurHint(), k.textSecondary, func() {
			if ui.Switch(c, &p.hideBlur).Label(locales.GeneralHideOnBlur()).Changed() {
				p.g.settings.KeepTerminalOnBlur = !p.hideBlur
				p.g.saveSettings()
			}
		})
	})
	newGroup(c, k, func(g *group) {
		g.row(c, k, locales.GeneralAppearance(), "", k.textSecondary, func() {
			ui.Text(c, locales.GeneralAppearanceHint()).FontSize(12).TextColor(k.textSecondary)
		})
	})
}

// --- About ---

func (p *prefsWindow) aboutPage(c *ui.Context, k tokens) {
	ui.Column(c).MinHeight(500 - 52 - 24).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		// The app's icon is to come (design.md 1.6): the tray's mark.
		ui.Box(c).Size(64, 64).Center().AccessHidden(true).Children(func() {
			sfsymbols.Symbol(c, "command.square").Size(48).Color(k.text)
		})
		ui.Text(c, appName).FontSize(20).FontWeight(600).TextColor(k.text).Margin(12, 0, 0, 0)
		ui.Text(c, locales.AboutTagline()).FontSize(13).TextColor(k.textSecondary)
		ui.Text(c, locales.AboutVersion(appVersion())).FontSize(12).TextColor(k.textSecondary).Margin(4, 0, 16, 0)
		if p.checking {
			ui.Row(c).Gap(8).Height(28).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(14, 14)
				ui.Text(c, locales.AboutChecking()).FontSize(13).TextColor(k.textSecondary)
			})
		} else {
			ui.Button(c, locales.AboutCheck()).OnClick(p.check)
		}
		color := k.textSecondary
		if p.statusDanger {
			color = k.danger
		}
		ui.Text(c, p.status).FontSize(12).TextColor(color).Height(16).Margin(8, 0, 0, 0)
	})
}

// appVersion is the packaged app's version, "dev" for a build not
// packaged (go run, go build).
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "dev"
}

// check asks for an update: an alert offers it, or a line under the
// button says there is none (for 3 seconds) or that the check failed.
func (p *prefsWindow) check() {
	if p.checking {
		return
	}
	p.checking, p.status = true, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		up, err := mygo.Updater.Check(ctx)
		p.win.Update(func() {
			p.checking = false
			switch {
			case err != nil:
				if !errors.Is(err, mygo.ErrUpdatesDisabled) {
					log.Printf("update: %v", err)
				}
				p.say(locales.AboutCheckFailed(), true, false)
			case up == nil:
				p.say(locales.AboutUpToDate(), false, true)
			default:
				p.update, p.updateOpen = up, true
			}
		})
	}()
}

// install downloads and installs the update, then relaunches.
func (p *prefsWindow) install(up *mygo.Update) {
	p.checking = true
	go func() {
		err := up.Install(context.Background(), nil)
		p.win.Update(func() {
			p.checking, p.update = false, nil
			if err != nil {
				log.Printf("update: %v", err)
				p.say(locales.AboutCheckFailed(), true, false)
				return
			}
			mygo.App.Relaunch()
		})
	}()
}

// say shows a line under the Check for Updates button, for 3 seconds when
// it fades.
func (p *prefsWindow) say(text string, danger, fades bool) {
	p.status, p.statusDanger = text, danger
	p.statusGen++
	gen := p.statusGen
	if fades {
		time.AfterFunc(3*time.Second, func() {
			p.win.Update(func() {
				if p.statusGen == gen {
					p.status = ""
				}
			})
		})
	}
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

// checkForUpdates shows the About page and checks.
func (g *gocast) checkForUpdates() {
	g.prefs.show(pageAbout)
	g.prefs.check()
}

// defaultPreset returns the default preset, or nil for a plain terminal.
func (g *gocast) defaultPreset() *presets.Preset {
	if i := presets.DefaultPreset(g.presets); i >= 0 {
		p := g.presets[i]
		return &p
	}
	return nil
}
