package main

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/apps"
	"gocast/internal/presets"
	"gocast/internal/running"
	"gocast/internal/search"
	"gocast/locales"
)

// The search panel's measures (design.md 2.1–2.4), in DIPs. The window is
// as large as the expanded panel all the time, plus room for its shadow:
// the panel grows and shrinks inside it, drawn in the same frame as its
// content, so that no frame shows it at a size its content was not laid
// out for (no blank frame), and the mouse goes through the rest of the
// window (SetInputRegion).
const (
	panelWidth     = 750
	barHeight      = 66
	listHeight     = 409 // to the panel's bottom, under the actions capsule
	emptyHeight    = 194
	capsuleHeight  = 36
	capsuleInset   = 8
	expandedHeight = barHeight + listHeight  // 475
	noResultHeight = barHeight + emptyHeight // 260
	panelRadius    = 26
	rowHeight      = 40
	iconSize       = 22

	marginX, marginTop, marginBottom = 32, 12, 44
	windowWidth                      = panelWidth + 2*marginX
	windowHeight                     = marginTop + expandedHeight + marginBottom

	// restoreFor is how long the panel keeps its query and choice once
	// hidden (Raycast's "pop to root" delay).
	// suggestionsEnabled shows the most used items (frecency) under an
	// empty query, under a "Suggestions" header, as Raycast does, rather
	// than the search bar alone. Off until product decides (design.md).
	suggestionsEnabled = false
	suggestionCount    = 9

	restoreFor = 90 * time.Second
	noticeFor  = 4 * time.Second
)

// entry is what a search result opens: an app, or a terminal preset.
type entry struct {
	app    *mygo.InstalledApp
	preset string // its ID, when app is nil
}

// action is an item of the actions menu (Cmd-K).
type action struct {
	icon, label string
	caps        []string
	run         func()
}

// launcher is the search panel. Its state is read and changed on the main
// thread; the apps come from a goroutine through Window.Update.
type launcher struct {
	g   *gocast
	win *mygo.Window

	apps     []mygo.InstalledApp
	alts     [][]string // the apps' other names (English, Chinese), by index
	items    []search.Item
	entries  []entry // by item
	results  []int   // items matching the query, best first; kept as the query empties, for the panel to shrink with them
	query    string
	filtered string // the query of the results
	chosen   int
	moved    bool // the user moved the choice since the results changed
	list     ui.ListState
	input    ui.Handle
	scroll   bool                  // show the choice in the next frame
	icons    map[string]*ui.Bitmap // by app path; nil where the app has none
	running  map[string]bool       // by bundle ID, as isRunning found it

	loading, loaded bool // listing the apps; listed them once

	hiddenAt  time.Time
	selectAll bool // select the query in the next frame (restored on show)
	gen       int  // the panel's key: each show starts its animation afresh
	composing bool // the input method composes in the field
	region    float32

	notice       string // in the footer for a while: a failure, a path copied
	noticeDanger bool
	noticeGen    int
	announce     bool

	menuOpen      bool
	menuComposing bool
	menuQuery     string
	menuChosen    int
	menuInput     ui.Handle
	menuItems     []action // the menu's actions, as it opened
	menuShown     []int    // those matching menuQuery
}

func newLauncher(g *gocast) *launcher {
	l := &launcher{g: g, icons: map[string]*ui.Bitmap{}, running: map[string]bool{}, filtered: noQuery}
	l.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         appName,
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true, // the panel draws its own, around its own shape
		Width:         windowWidth,
		Height:        windowHeight,
		DisableResize: true,
		Content:       ui.View(l.view),
	})
	// Hide rather than close, on Escape and Cmd-W, and when the user goes
	// elsewhere: the panel shows again at once, as it was.
	l.win.OnClose(func(e *mygo.CloseEvent) {
		if !g.quitting {
			e.PreventDefault()
			l.hide()
		}
	})
	l.win.OnBlur(l.hide)
	l.rebuild()
	return l
}

// compact reports whether the panel shows its search bar alone: while the
// query is empty.
// query is empty, and no suggestions show (suggestionsEnabled).
func (l *launcher) compact() bool {
	return l.empty() && (!suggestionsEnabled || len(l.results) == 0)
}

// empty reports whether the query is empty.
func (l *launcher) empty() bool { return strings.TrimSpace(l.query) == "" }

// targetHeight is the height of the panel for what it shows now.
func (l *launcher) targetHeight() float32 {
	switch {
	case l.compact():
		return barHeight
	case len(l.results) > 0:
		return expandedHeight
	case l.loading && !l.loaded:
		return barHeight // the apps come: no "no results" yet
	default:
		return noResultHeight
	}
}

func (l *launcher) view(c *ui.Context) {
	c.SetTheme(themed(c))
	k := tokensFor(c)
	c.Root().Background(ui.Transparent)
	reduce := c.Preferences().ReduceMotion

	l.shortcuts(c)

	target := l.targetHeight()
	if target != l.region {
		l.region = target
		l.win.SetInputRegion([]mygo.Rectangle{{X: marginX, Y: marginTop, Width: panelWidth, Height: int(target)}})
	}
	// What the panel lays out below its bar: the results, the "no
	// results" message, or, while it shrinks to its bar, what it showed.
	body := target
	if l.compact() {
		body = expandedHeight
		if len(l.results) == 0 {
			body = noResultHeight
		}
	}

	c.Key(l.gen)
	panel := ui.Column(c).Absolute().Left(marginX).Top(marginTop).Width(panelWidth).
		Radius(panelRadius).Material(panelGlass{fallback: k.surface}).Clip().
		Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.22))
	h := target
	if !reduce {
		h = panel.AnimateTo("h", target, ui.Animation{Spring: ui.SpringOptions{Duration: 120 * time.Millisecond}})
	}
	panel.Height(h).Children(func() {
		l.searchBar(c, k)
		if body > barHeight {
			ui.Column(c).Height(body - barHeight).Shrink(0).Children(func() {
				if len(l.results) > 0 {
					l.resultList(c, k)
				} else {
					l.noResults(c, k)
				}
			})
			l.footer(c, k)
		}
		if l.menuOpen {
			l.actionsMenu(c, k, reduce)
		}
	})
	if l.announce {
		l.announce = false
		c.Announce(l.notice)
	}
}

// shortcuts are the panel's Command keys, whatever has the focus.
func (l *launcher) shortcuts(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		l.toggleMenu()
	}
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		l.hide()
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		l.hide()
		l.g.openSettings("")
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyC) {
		if e, ok := l.current(); ok && e.app != nil {
			l.copyPath(*e.app)
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyE) {
		if e, ok := l.current(); ok && e.app == nil {
			l.editPreset(e.preset)
		}
	}
	c.OnShortcut(ui.Cmd, ui.KeyQ, mygo.App.Quit)
}

func (l *launcher) searchBar(c *ui.Context, k tokens) {
	ui.Row(c).Height(barHeight).Shrink(0).Padding(0, 20).Gap(12).AlignItems(ui.Center).Children(func() {
		sfsymbols.Symbol(c, "command.square").Size(20).Color(k.textOnGlass).AccessHidden(true) // Gocast's mark, as the tray's
		field := ui.TextInputBase(c, &l.query).Bind(&l.input).Grow(1).FontSize(20).
			Placeholder(locales.SearchPlaceholder()).Label(locales.SearchPlaceholder()).
			OnTextKey(l.textKey)
		l.composing = field.Composing()
		if l.selectAll {
			l.selectAll = false
			field.SetTextSelection(0, len([]rune(l.query)))
		}
		if field.Changed() {
			l.filter()
		}
		if l.loading && !l.loaded && !l.compact() {
			ui.Spinner(c).Size(14, 14)
		}
	})
}

// textKey takes the keys of the search field that move the choice and act
// on it, as they come, before the field: none while the input method
// composes, whose keys (arrows, Enter, Escape) are its own.
func (l *launcher) textKey(key ui.TextKey) bool {
	if l.composing {
		return false
	}
	// Keys typed before this one may not have reached the view yet: act
	// on the query as it is now (type-ahead, then Enter at once), and
	// put the bound value back as it was, which the field writes as the
	// next frame is built (writing it here would make the field take it
	// as the app's, over what it holds).
	bound := l.query
	if q := key.Text(); q != l.query {
		l.query = q
		l.filter()
		defer func() { l.query = bound }()
	}
	mods := key.Mods
	switch {
	case key.Key == ui.KeyEscape && mods == 0:
		if l.query != "" {
			key.SetText("") // and the bound value
			bound, l.query = "", ""
			l.filter()
		} else {
			l.hide()
		}
		return true
	case key.Key == ui.KeyEnter && mods == 0:
		l.primary()
		return true
	case key.Key == ui.KeyEnter && mods == ui.Cmd:
		l.secondary()
		return true
	case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl, key.Key == ui.KeyJ && mods == ui.Ctrl:
		l.move(1)
		return true
	case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl, key.Key == ui.KeyK && mods == ui.Ctrl:
		l.move(-1)
		return true
	}
	return false
}

func (l *launcher) move(step int) {
	if l.compact() || len(l.results) == 0 {
		return
	}
	l.moved = true
	l.choose(max(0, min(l.chosen+step, len(l.results)-1))) // no wrapping
}

// choose chooses the result i, telling the list the app chose it (else
// it keeps the choice on the item it had, or on none when the item is
// gone), and shows it in the next frame.
func (l *launcher) choose(i int) {
	l.list.Selected = &l.chosen
	l.list.Choose(i)
	l.scroll = true
}

func (l *launcher) resultList(c *ui.Context, k tokens) {
	l.list.Selected = &l.chosen
	l.list.Key = func(i int) any { return l.items[l.results[i]].ID }
	l.list.Label = func(i int) string { return l.rowLabel(l.results[i]) }
	var list ui.Element
	if l.empty() { // suggestions
		ui.Text(c, locales.SearchSuggestions()).FontSize(12).TextColor(k.textOnGlass).Padding(0, 20, 4, 20).Shrink(0)
	}
	ui.Themed(c, listTheme(c, k)).Background(ui.Transparent).Grow(1).Children(func() { // the glass shows through
		// Room at the end for the last row to scroll above the actions
		// capsule, which floats over the list.
		list = ui.List(c, &l.list, len(l.results), func(i int) { l.row(c, k, i) }).Grow(1).Padding(0, 8, capsuleInset+capsuleHeight+capsuleInset, 8)
	})
	if l.scroll && l.chosen >= 0 && l.chosen < len(l.results) {
		l.scroll = false
		l.list.ScrollIntoView(l.chosen)
	}
	if list.Changed() {
		// A click chose a row: the keyboard stays in the field.
		l.moved = true
		l.input.Focus()
	}
	if list.Submitted() && !l.compact() {
		l.primary() // a double click
	}
}

// row shows a result: its icon, its name, a preset's folder and command,
// and what it is.
func (l *launcher) row(c *ui.Context, k tokens, i int) {
	it := l.items[l.results[i]]
	e := l.entries[l.results[i]]
	var subtitle string
	tag := locales.SearchTagApp()
	if e.app == nil {
		tag = locales.SearchTagPreset()
		if j := presets.Find(l.g.presets, e.preset); j >= 0 {
			subtitle = presetSubtitle(l.g.presets[j])
		}
	}
	ui.Row(c).Height(rowHeight).Padding(0, 12).Gap(12).AlignItems(ui.Center).Children(func() {
		switch {
		case e.app == nil:
			ui.Box(c).Size(iconSize, iconSize).Radius(6).Background(k.keycap).Center().Shrink(0).Children(func() {
				sfsymbols.Symbol(c, "apple.terminal.fill").Size(13).Color(k.text)
			})
		default:
			app := *e.app
			ui.Box(c).Size(iconSize, iconSize).Shrink(0).Children(func() {
				if b := l.icon(app); b != nil {
					ui.Image(c, b).Size(iconSize, iconSize)
				} else {
					ui.Avatar(c, it.Title, nil).Size(iconSize, iconSize) // its initial
				}
				if l.isRunning(app.ID) {
					// A dot under the icon, as the Dock's and Raycast's.
					// 2 under the icon, in the row's height.
					ui.Box(c).Absolute().Left(iconSize/2-2).Bottom(-6).Size(4, 4).Radius(2).Background(k.textSecondary)
				}
			})
		}
		// The title at its width, the subtitle at most 45% of the row and
		// shrinking first, the title only once it is all but gone.
		title := ui.Text(c, it.Title).FontSize(14).TextColor(k.text).SingleLine().Shrink(1)
		if subtitle != "" {
			title.Margin(0, -4, 0, 0) // 8 to the subtitle, not the row's 12
			ui.Text(c, subtitle).FontSize(13).TextColor(k.textOnGlass).SingleLine().TruncateMiddle().
				MaxWidth((panelWidth - 16 - 24) * 0.45).Shrink(1000)
		}
		ui.Spacer(c)
		ui.Text(c, tag).FontSize(12).TextColor(k.textOnGlass).SingleLine().Shrink(0)
	})
}

// rowLabel is what assistive technology reads of a result: "Safari, App",
// a preset's folder after.
func (l *launcher) rowLabel(item int) string {
	it, e := l.items[item], l.entries[item]
	sep := ", "
	if strings.HasPrefix(i18n.Locale(), "zh") {
		sep = "，"
	}
	if e.app != nil {
		s := it.Title + sep + locales.SearchTagApp()
		if l.isRunning(e.app.ID) {
			s += sep + locales.SearchRunning()
		}
		return s
	}
	s := it.Title + sep + locales.SearchTagPreset()
	if j := presets.Find(l.g.presets, e.preset); j >= 0 {
		s += sep + l.g.presets[j].ShortDir()
	}
	return s
}

func (l *launcher) noResults(c *ui.Context, k tokens) {
	q := strings.TrimSpace(l.query)
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Padding(0, 24).Children(func() {
		ui.Text(c, locales.SearchNoResults(q)).FontSize(14).FontWeight(500).TextColor(k.text).SingleLine()
		ui.Text(c, locales.SearchNoResultsHint()).FontSize(13).TextColor(k.textOnGlass).Margin(8, 0, 0, 0)
		if len(l.g.presets) == 0 {
			ui.Text(c, locales.SearchNoPresetsHint()).FontSize(13).TextColor(k.textOnGlass).Margin(4, 0, 0, 0)
		}
	})
}

// footer floats over the end of the list, as Raycast's (design.md 2.4):
// a glass capsule at the bottom right with the primary action and the
// actions menu, each clickable, and one at the bottom left with a notice,
// for a few seconds.
func (l *launcher) footer(c *ui.Context, k tokens) {
	// The capsules are the system's glass, as the panel: MyGo's glass
	// over the panel's would show what the window painted under it,
	// nothing, as gray.
	capsule := func() ui.Element {
		return ui.Row(c).Absolute().Bottom(capsuleInset).Height(capsuleHeight).Radius(capsuleHeight/2).
			AlignItems(ui.Center).Material(panelGlass{fallback: k.surface}).Border(1, capsuleBorder(c)).
			Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.12))
	}
	// segment is a clickable part of the actions capsule, in the selection
	// color while pressed.
	segment := func(label string, mods ui.Modifiers, key ui.Key) ui.Element {
		seg := ui.Row(c).Height(capsuleHeight-6).Radius(14).Padding(0, 6, 0, 8).Gap(8).AlignItems(ui.Center).
			Label(label).KeyShortcuts(mods, key)
		if seg.Pressed() {
			seg.Background(k.selection)
		}
		return seg
	}
	reduce := c.Preferences().ReduceMotion
	if l.notice != "" {
		icon, color := "checkmark.circle", k.textOnGlass
		if l.noticeDanger {
			icon, color = "exclamationmark.triangle.fill", k.danger
		}
		fade := &ui.Motion{Opacity: 0}
		c.Key("notice")
		n := capsule().Left(capsuleInset).MaxWidth(panelWidth-capsuleWidth-24).Padding(0, 14).Gap(6)
		if !reduce {
			n.Transition(ui.ElementTransition{Enter: fade, Exit: fade, Duration: 150 * time.Millisecond})
		}
		n.Children(func() {
			sfsymbols.Symbol(c, icon).Size(14).Color(color)
			ui.Text(c, l.notice).FontSize(12).TextColor(color).SingleLine().Shrink(1)
		})
	}
	e, ok := l.current()
	c.Key("actions")
	capsule().Right(capsuleInset).Padding(0, 3, 0, 6).Gap(4).Children(func() {
		if ok {
			name := locales.FooterOpen()
			if e.app == nil {
				name = locales.FooterOpenInTerminal()
			}
			segment(name, 0, ui.KeyEnter).OnClick(l.primary).Children(func() {
				ui.Text(c, name).FontSize(13).TextColor(k.text).SingleLine()
				outlineKeycaps(c, k, k.textOnGlass, "↩")
			})
		}
		actions := segment(locales.FooterActions(), ui.Cmd, ui.KeyK)
		color := k.textOnGlass
		if ok {
			actions.OnClick(l.toggleMenu)
		} else {
			// No results: nothing to act on. Dimmed by its colors, as
			// opacity under glass draws the glass dark.
			color = color.Alpha(0.4)
			actions.AccessDisabled(true)
		}
		actions.Children(func() {
			ui.Text(c, locales.FooterActions()).FontSize(13).TextColor(color).SingleLine()
			outlineKeycaps(c, k, color, "⌘", "K")
		})
	})
}

// capsuleWidth is about the actions capsule's width, which bounds the
// notice capsule's.
const capsuleWidth = 280

// isRunning reports whether the app with the bundle ID runs, as it did
// when the panel showed.
func (l *launcher) isRunning(id string) bool {
	on, ok := l.running[id]
	if !ok {
		on = running.IsRunning(id)
		l.running[id] = on
	}
	return on
}

// current returns the result chosen, while the panel shows results.
func (l *launcher) current() (entry, bool) {
	if l.compact() || l.chosen < 0 || l.chosen >= len(l.results) {
		return entry{}, false
	}
	return l.entries[l.results[l.chosen]], true
}

// currentID returns the item ID of the result chosen.
func (l *launcher) currentID() string {
	if l.compact() || l.chosen < 0 || l.chosen >= len(l.results) {
		return ""
	}
	return l.items[l.results[l.chosen]].ID
}

// --- The actions menu (Cmd-K) ---

func (l *launcher) toggleMenu() {
	if l.menuOpen {
		l.closeMenu()
		return
	}
	if _, ok := l.current(); !ok {
		return
	}
	l.menuOpen = true
	l.menuQuery = ""
	l.menuChosen = 0
	l.menuItems = l.actions()
	l.filterMenu()
	l.menuInput.Focus()
	l.win.Invalidate()
}

func (l *launcher) closeMenu() {
	l.menuOpen = false
	l.input.Focus()
	l.win.Invalidate()
}

// actions are what the menu offers for the result chosen, its primary
// action first (copy.md 2.2).
func (l *launcher) actions() []action {
	e, ok := l.current()
	if !ok {
		return nil
	}
	if e.app != nil {
		app := *e.app
		as := []action{
			{"arrow.up.forward.app", locales.ActionOpen(), []string{"↩"}, l.primary},
			{"folder", locales.ActionShowInFinder(), []string{"⌘", "↩"}, l.secondary},
			{"doc.on.doc", locales.ActionCopyPath(), []string{"⌘", "⇧", "C"}, func() { l.copyPath(app) }},
		}
		if running.IsRunning(app.ID) {
			as = append(as, action{"xmark.circle", locales.ActionQuitApp(app.Name), nil, func() {
				running.Quit(app.ID)
				l.hide()
			}})
		}
		return as
	}
	id := e.preset
	return []action{
		{"apple.terminal", locales.ActionOpenInTerminal(), []string{"↩"}, l.primary},
		{"plus.square.on.square", locales.ActionNewTab(), []string{"⌘", "↩"}, l.secondary},
		{"pencil", locales.ActionEditPreset(), []string{"⌘", "E"}, func() { l.editPreset(id) }},
		{"folder", locales.ActionShowFolder(), nil, func() {
			if j := presets.Find(l.g.presets, id); j >= 0 {
				l.hide()
				mygo.Shell.ShowItemInFolder(l.g.presets[j].WorkDir())
			}
		}},
	}
}

func (l *launcher) filterMenu() {
	l.menuShown = l.menuShown[:0]
	for i, a := range l.menuItems {
		if search.NewKey(a.label).Rank(l.menuQuery) >= 0 {
			l.menuShown = append(l.menuShown, i)
		}
	}
	l.menuChosen = max(0, min(l.menuChosen, len(l.menuShown)-1))
}

func (l *launcher) runMenu(i int) {
	if i < 0 || i >= len(l.menuShown) {
		return
	}
	a := l.menuItems[l.menuShown[i]]
	l.menuOpen = false
	l.input.Focus()
	a.run()
}

func (l *launcher) menuKey(key ui.TextKey) bool {
	if l.menuComposing {
		return false
	}
	if q := key.Text(); q != l.menuQuery { // as in textKey
		bound := l.menuQuery
		l.menuQuery = q
		l.filterMenu()
		defer func() { l.menuQuery = bound }()
	}
	mods := key.Mods
	switch {
	case key.Key == ui.KeyEscape && mods == 0:
		l.closeMenu()
		return true
	case key.Key == ui.KeyEnter && mods == 0:
		l.runMenu(l.menuChosen)
		return true
	case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl, key.Key == ui.KeyJ && mods == ui.Ctrl:
		l.menuChosen = min(l.menuChosen+1, len(l.menuShown)-1)
		return true
	case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl, key.Key == ui.KeyK && mods == ui.Ctrl:
		l.menuChosen = max(l.menuChosen-1, 0)
		return true
	}
	return false
}

func (l *launcher) actionsMenu(c *ui.Context, k tokens, reduce bool) {
	enter := &ui.Motion{Opacity: 0, Scale: 0.97}
	tr := ui.ElementTransition{Enter: enter, Exit: enter, Duration: 120 * time.Millisecond, ExitDuration: 80 * time.Millisecond}
	if reduce {
		fade := &ui.Motion{Opacity: 0}
		tr = ui.ElementTransition{Enter: fade, Exit: fade, Duration: 80 * time.Millisecond}
	}
	c.Key("actions")
	ui.Column(c).Absolute().Right(capsuleInset).Bottom(capsuleInset+capsuleHeight+8).Width(320).MaxHeight(300).
		Radius(12).Material(panelGlass{fallback: k.surface}).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.20)).Clip().
		TransformOrigin(1, 1).Transition(tr).Children(func() {
		ui.Row(c).Height(36).Shrink(0).Padding(0, 12).AlignItems(ui.Center).Children(func() {
			f := ui.TextInputBase(c, &l.menuQuery).Bind(&l.menuInput).Grow(1).FontSize(13).
				Placeholder(locales.ActionsPlaceholder()).Label(locales.ActionsPlaceholder()).
				OnTextKey(l.menuKey)
			l.menuComposing = f.Composing()
			if f.Changed() {
				l.filterMenu()
			}
		})
		ui.Box(c).Height(1).Background(k.hairline).Shrink(0)
		ui.Scroll(c).Shrink(1).Children(func() {
			ui.Column(c).Padding(6).Children(func() {
				if len(l.menuShown) == 0 {
					ui.Text(c, locales.ActionsNoMatch()).FontSize(13).TextColor(k.textOnGlass).Padding(8)
				}
				for i, ai := range l.menuShown {
					a := l.menuItems[ai]
					bg := ui.Transparent
					if i == l.menuChosen {
						bg = k.selection
					}
					c.Key(a.label)
					row := ui.Row(c).Height(32).Padding(0, 8).Radius(8).Gap(8).AlignItems(ui.Center).Background(bg).Label(a.label)
					idx := i
					row.OnClick(func() { l.runMenu(idx) }).Children(func() {
						sfsymbols.Symbol(c, a.icon).Size(16).Color(k.text)
						ui.Text(c, a.label).FontSize(13).TextColor(k.text).SingleLine().Shrink(1)
						ui.Spacer(c)
						if len(a.caps) > 0 {
							keycaps(c, k, a.caps, 12)
						}
					})
				}
			})
		})
	})
}

// --- Acting ---

// primary opens the result chosen: an app, or a preset in the floating
// terminal (its tab, if it has one).
func (l *launcher) primary() { l.act(false) }

// secondary shows an app in Finder, or opens a preset in a new tab.
func (l *launcher) secondary() { l.act(true) }

func (l *launcher) act(second bool) {
	e, ok := l.current()
	if !ok {
		return
	}
	l.g.record(l.currentID(), l.query)
	l.hide() // the keyboard goes back to the app the user was in first
	if e.app == nil {
		l.g.term.openPreset(e.preset, second)
		return
	}
	if second {
		mygo.Shell.ShowItemInFolder(e.app.Path)
		return
	}
	if err := mygo.Shell.LaunchApp(*e.app); err != nil {
		log.Printf("launching %s: %v", e.app.Name, err)
		l.show()
		l.say(locales.SearchOpenFailed(e.app.Name), true)
	}
}

func (l *launcher) copyPath(app mygo.InstalledApp) {
	mygo.Clipboard.WriteText(app.Path)
	l.say(locales.FooterPathCopied(), false)
}

func (l *launcher) editPreset(id string) {
	l.hide()
	l.g.editPreset(id)
}

// say shows a notice in the footer for a few seconds.
func (l *launcher) say(text string, danger bool) {
	l.notice, l.noticeDanger, l.announce = text, danger, true
	l.noticeGen++
	gen := l.noticeGen
	time.AfterFunc(noticeFor, func() {
		l.win.Update(func() {
			if l.noticeGen == gen {
				l.notice = ""
			}
		})
	})
	l.win.Invalidate()
}

// --- Items ---

// rebuild makes the searchable items, the terminal presets then the apps,
// and filters them again, keeping the choice on its item.
func (l *launcher) rebuild() {
	l.items, l.entries = l.items[:0], l.entries[:0]
	now := time.Now()
	for _, p := range l.g.presets {
		id := "preset:" + p.ID
		it := search.NewItem(id, presetTitle(p), p.Command, "Terminal", "终端")
		it.Score = l.g.usage.Score(id, now)
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{preset: p.ID})
	}
	for i := range l.apps {
		app := &l.apps[i]
		id := "app:" + app.ID
		it := search.NewItem(id, app.Name, l.alts[i]...)
		it.Score = l.g.usage.Score(id, now)
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{app: app})
	}
	if l.empty() && !suggestionsEnabled {
		return
	}
	// Keep the choice on its item as the apps come in, unless it is the
	// first row the user left as it was: the new first row takes it.
	was := l.currentID()
	first := l.chosen == 0 && !l.moved
	l.filtered = l.query
	l.results = l.find()
	chosen := 0
	if !first {
		if i := slices.IndexFunc(l.results, func(r int) bool { return l.items[r].ID == was }); i >= 0 {
			chosen = i
		}
	}
	l.choose(chosen)
}

// filter finds the results of a new query, the first chosen. An empty
// query keeps the results it had, which the panel shrinks with.
func (l *launcher) filter() {
	if l.empty() && !suggestionsEnabled {
		l.filtered = noQuery // the same query again starts afresh
		return
	}
	if l.query == l.filtered {
		return
	}
	l.filtered = l.query
	l.moved = false
	l.menuOpen = false
	l.results = l.find()
	l.choose(0)
}

// noQuery is l.filtered when no results were found for the query.
const noQuery = "\x00"

// find returns the results of the query, or the suggestions for an empty
// one.
func (l *launcher) find() []int {
	if l.empty() {
		return search.Suggestions(l.items, suggestionCount)
	}
	return search.Filter(l.items, l.query, l.g.usage.Choice(l.query, time.Now()))
}

// load lists the installed apps and their other names on a goroutine, as
// at each show, where apps may have come or gone since.
func (l *launcher) load() {
	if l.loading {
		return
	}
	l.loading = true
	go func() {
		found, err := mygo.Shell.InstalledApps()
		if err != nil {
			log.Print(err)
		}
		type named struct {
			app  mygo.InstalledApp
			alts []string
			key  string
		}
		list := make([]named, len(found))
		locale := mygo.App.Locale()
		for i, app := range found {
			info := apps.Names(app.Path)
			alts := append([]string{app.Name}, info.Names...)
			// The name in the language the app shows, which the system's
			// name may not be (see apps.Info.Localized).
			if n := info.Localized(locale); n != "" {
				app.Name = n
			}
			list[i] = named{app, alts, search.SortKey(app.Name)}
		}
		// By name, Chinese names among the others by their pinyin.
		slices.SortStableFunc(list, func(a, b named) int { return strings.Compare(a.key, b.key) })
		found = found[:0]
		alts := make([][]string, len(list))
		for i, n := range list {
			found = append(found, n.app)
			alts[i] = n.alts
		}
		l.win.Update(func() {
			l.apps, l.alts, l.loading, l.loaded = found, alts, false, true
			l.rebuild()
		})
	}()
}

// --- Showing and hiding ---

// show shows the panel on the display with the pointer, its top a fifth of
// the way down: as it was if it hid less than 90 seconds ago, its query
// selected so that typing replaces it; else empty, its bar alone.
func (l *launcher) show() {
	if l.win.IsVisible() {
		l.input.Focus()
		return
	}
	restore := !l.hiddenAt.IsZero() && time.Since(l.hiddenAt) < restoreFor
	if !restore {
		l.query, l.filtered, l.results, l.moved = "", noQuery, nil, false
		l.notice = ""
		l.list = ui.ListState{}
		l.choose(0)
		l.filter() // the suggestions, when they show
	}
	l.selectAll = restore && l.query != ""
	clear(l.running) // apps started or quit meanwhile
	l.menuOpen = false
	l.gen++ // appears at its size, without animating to it
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	wa := d.WorkArea
	l.win.SetPosition(wa.X+(wa.Width-windowWidth)/2, wa.Y+wa.Height/5-marginTop)
	l.input.Focus()
	l.win.Show()
	l.win.Invalidate() // draw the state changed outside a frame
	if l.g.loadPresets() {
		l.g.presetsChanged()
	}
	l.load()
}

func (l *launcher) hide() {
	if !l.win.IsVisible() {
		return
	}
	l.hiddenAt = time.Now()
	l.menuOpen = false
	l.win.Hide()
}

func (l *launcher) toggle() {
	if l.win.IsVisible() {
		l.hide()
	} else {
		l.show()
	}
}

// icon returns an app's icon, drawn for the sharpest display the first time
// it shows (a few milliseconds), and kept.
func (l *launcher) icon(app mygo.InstalledApp) *ui.Bitmap {
	if b, ok := l.icons[app.Path]; ok {
		return b
	}
	scale := 1.0
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, d.ScaleFactor)
	}
	var bitmap *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(app.Path, int(iconSize*scale)); err == nil {
		bitmap, _ = ui.DecodeBitmap(data)
	}
	l.icons[app.Path] = bitmap
	return bitmap
}

// presetTitle is a preset's name, or "Terminal" for one without.
func presetTitle(p presets.Preset) string {
	if strings.TrimSpace(p.Name) == "" {
		return locales.TermTabDefault()
	}
	return p.Name
}

// presetSubtitle says where and what a preset runs: "~/Work/x · pi".
func presetSubtitle(p presets.Preset) string {
	if strings.TrimSpace(p.Command) == "" {
		return locales.PresetSubtitleShell(p.ShortDir())
	}
	return locales.PresetSubtitle(p.ShortDir(), p.Command)
}
