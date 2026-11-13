package main

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/apps"
	"gocast/internal/calc"
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
	panelWidth     = 640
	barHeight      = 66
	listHeight     = 409 // to the panel's bottom, under the actions capsule
	emptyHeight    = 194
	capsuleInset   = 8
	expandedHeight = barHeight + listHeight  // 475
	noResultHeight = barHeight + emptyHeight // 260

	// The window's room around the panel, for its shadow. The window is
	// as wide as the clipboard view (design.md 2.8.1), the search panel
	// centred in it: the view grows 60 at each side, its middle where the
	// search panel's is.
	marginX, marginTop, marginBottom = shadowLeft, shadowTop, shadowBottom
	windowWidth                      = clipWidth + 2*marginX
	searchLeft                       = marginX + (clipWidth-panelWidth)/2
	windowHeight                     = marginTop + expandedHeight + marginBottom

	// The suggestions under an empty query (design.md 2.5a): the items
	// opened most, at most suggestMost, padded to suggestLeast with
	// common ones while there are fewer.
	suggestMost  = 8
	suggestLeast = 5

	// restoreFor is how long the panel keeps its query, choice and
	// suggestions once hidden (Raycast's "pop to root" delay).
	restoreFor = 90 * time.Second
	noticeFor  = 4 * time.Second
)

// entry is what a row opens: an app, a terminal preset, one of Gocast's
// own items (the clipboard history, ...), or the calculator's card.
type entry struct {
	calc   bool
	app    *mygo.InstalledApp
	place  string // what tells the app from another of its name: its file name or folder (apps.Places)
	preset string // its ID, for a preset
	own    string // the ID of Gocast's own item (clipItemID, ...)
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

	apps    []mygo.InstalledApp
	alts    [][]string // the apps' other names (English, Chinese), by index
	places  []string   // by index, what tells an app from another of its name (apps.Places)
	items   []search.Item
	entries []entry        // by item
	itemAt  map[string]int // the index of each item, by ID
	// rows are the list's: the items matching the query, best first, or
	// the suggestions and Gocast's own items under an empty one; kept as
	// the panel shrinks to its bar alone, for it to shrink with them.
	rows     []panelRow
	query    string
	filtered string // the query of the rows
	chosen   int    // the row chosen
	moved    bool   // the user moved the choice since the rows changed
	list     ui.ListState
	input    ui.Handle
	scroll   bool                  // show the choice in the next frame
	icons    map[string]*ui.Bitmap // by app path; nil where the app has none
	running  map[string]bool       // by bundle ID, as isRunning found it

	loading, loaded bool // listing the apps; listed them once

	hiddenAt time.Time
	// shown: the panel shows, or is about to, as show and openClip set
	// it before they order the window in; hide clears it. Hidden (and
	// at first), the panel draws nothing (see view).
	shown     bool
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

	// own are Gocast's own items, the Gocast group's rows (design.md
	// 2.5a).
	own []ownItem
	// ranked are the items opened most, as they ranked as the panel
	// showed: the suggestions do not move while it shows.
	ranked []string
	// searchOnly: the panel shows its search bar alone under an empty
	// query (Settings › General, as it showed), until expanded (Cmd-K or
	// Down) to the suggestions.
	searchOnly, expanded bool

	// clip is the clipboard history's view, which the panel turns into
	// (design.md 2.8).
	clip *clipView
	// regionW is the width of the panel the input region is for.
	regionW float32

	// The calculator's card (design.md 2.3a), the first row while calcOn:
	// the evaluation of the query, and the last one not being typed,
	// whose answer shows greyed while the next is typed.
	calcRes  calc.Result
	calcLast *calc.Result
	calcOn   bool
	// target is where Cmd-Return pastes the answer: the app the panel
	// showed over, or the floating terminal.
	target pasteTarget
}

func newLauncher(g *gocast) *launcher {
	l := &launcher{g: g, icons: map[string]*ui.Bitmap{}, running: map[string]bool{}, filtered: noQuery}
	l.clip = newClipView(l)
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

// compact reports whether the panel shows its search bar alone: under an
// empty query, as Settings › General asks, until expanded (design.md 2.5a).
func (l *launcher) compact() bool {
	return l.empty() && l.searchOnly && !l.expanded
}

// empty reports whether the query is empty.
func (l *launcher) empty() bool { return strings.TrimSpace(l.query) == "" }

// targetHeight is the height of the panel for what it shows now.
func (l *launcher) targetHeight() float32 {
	switch {
	case l.compact():
		return barHeight
	case len(l.rows) > 0:
		return expandedHeight
	case l.loading && !l.loaded && l.searchOnly:
		return barHeight // the apps come: no "no results" yet
	case l.loading && !l.loaded:
		return expandedHeight // as it was: it does not move as the user types
	default:
		return noResultHeight
	}
}

func (l *launcher) view(c *ui.Context) {
	c.SetTheme(themed(c))
	c.Root().Background(ui.Transparent)
	// Hidden, the panel draws nothing, so that it shows nothing stale as
	// it shows again: the window keeps the last frame it drew, which
	// AppKit shows as it orders the window in, until the window server
	// tells it shows, a pass or more later, and the frame of what it shows
	// now draws (frames wait while it is out of sight). Its last frame
	// would be the search panel as the clipboard shortcut opens the
	// clipboard view, or the reverse (design.md 2.8.1). Asking
	// WindowHidden builds the view once more as the window hides: that
	// frame, transparent, is the one the window keeps. Not shown yet, at
	// first, it draws nothing either.
	if hidden := c.WindowHidden(); hidden || !l.shown {
		return
	}
	k := tokensFor(c)
	reduce := c.Preferences().ReduceMotion

	if l.clip.open {
		l.setRegion(clipWidth, expandedHeight)
		l.clip.view(c, k)
		return
	}
	l.shortcuts(c)

	target := l.targetHeight()
	l.setRegion(panelWidth, target)
	// What the panel lays out below its bar: the rows, the "no results"
	// message, or, while it shrinks to its bar, what it showed.
	body := target
	if l.compact() {
		body = expandedHeight
		if len(l.rows) == 0 {
			body = noResultHeight
		}
	}

	c.Key(l.gen)
	panel := ui.Column(c).Absolute().Left(searchLeft).Top(marginTop).Width(panelWidth).
		Material(glassOf(k, searchTint)).Clip()
	panelShadow(panelOutline(panel, k), k)
	h := target
	if !reduce {
		h = panel.AnimateTo("h", target, ui.Animation{Spring: ui.SpringOptions{Duration: 120 * time.Millisecond}})
	}
	panel.Height(h).Children(func() {
		l.searchBar(c, k)
		if body > barHeight {
			ui.Column(c).Height(body - barHeight).Shrink(0).Children(func() {
				switch {
				case len(l.rows) > 0:
					l.resultList(c, k)
				case l.loading && !l.loaded: // the apps come: no "no results" yet
				default:
					l.noResults(c, k)
				}
			})
			l.footer(c, k, h)
		}
		if l.menuOpen {
			l.actionsMenu(c, k)
		}
	})
	if l.announce {
		l.announce = false
		c.Announce(l.notice)
	}
}

// setRegion takes the mouse over the panel, w wide and h high, centred in
// the window; it goes through the rest of it.
func (l *launcher) setRegion(w, h float32) {
	if w == l.regionW && h == l.region || l.win == nil { // nil: drawn in a test
		return
	}
	l.regionW, l.region = w, h
	left := marginX + (clipWidth-w)/2
	l.win.SetInputRegion([]mygo.Rectangle{{X: int(left), Y: marginTop, Width: int(w), Height: int(h)}})
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
		if e, ok := l.current(); ok && e.preset != "" {
			l.editPreset(e.preset)
		}
	}
}

func (l *launcher) searchBar(c *ui.Context, k tokens) {
	ui.Row(c).Height(barHeight).Shrink(0).Padding(0, rightLine, 0, 20).Gap(12).AlignItems(ui.Center).Children(func() {
		// Half a DIP low (a margin of 1 in a centred row), so that its
		// middle meets the 20 pt text's: the caps' and the Chinese
		// characters' ink are 0.75 and 1 below the row's middle.
		gocastMark(c, k).Margin(1, 0, 0, 0)
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
		if l.loading && !l.loaded && !l.empty() {
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
		switch {
		case l.query != "":
			key.SetText("") // and the bound value
			bound, l.query = "", ""
			l.filter()
		case l.searchOnly && l.expanded: // back to the bar alone
			l.expanded = false
			l.filter()
		default:
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
	if l.compact() {
		if step > 0 {
			l.expand()
		}
		return
	}
	if len(l.rows) == 0 {
		return
	}
	l.moved = true
	l.choose(stepRow(l.rows, l.chosen, step)) // no wrapping
}

// expand shows the suggestions under the search bar alone (Cmd-K or Down,
// design.md 2.5a), the first chosen.
func (l *launcher) expand() {
	l.expanded = true
	l.filtered = noQuery
	l.filter()
	if l.win != nil {
		l.win.Invalidate()
	}
}

// choose chooses the row i, off a title, telling the list the app chose
// it (else it keeps the choice on the item it had, or on none when the
// item is gone), and shows it in the next frame.
func (l *launcher) choose(i int) {
	if i >= 0 && i < len(l.rows) && l.rows[i].title != "" {
		i = stepRow(l.rows, i, 1)
	}
	l.list.Selected = &l.chosen
	l.list.Choose(i)
	l.scroll = true
}

func (l *launcher) resultList(c *ui.Context, k tokens) {
	l.list.Selected = &l.chosen
	l.list.Key = l.rowKey
	l.list.Label = l.rowLabel
	var list ui.Element
	ui.Themed(c, searchListTheme(c, k)).Background(ui.Transparent).Grow(1).Children(func() { // the glass shows through
		// Room at the end for the last row to scroll above the actions
		// capsule, which floats over the list. The rows draw their
		// selection themselves, the calculator's card its own way.
		list = ui.List(c, &l.list, len(l.rows)).Grow(1).Padding(0, listInset, capsuleInset+capsuleHeight+capsuleInset, listInset).
			Rows(func(r ui.ListRow) { l.row(c, k, r.Index, r.Selected()) })
	})
	if l.scroll && l.chosen >= 0 && l.chosen < len(l.rows) {
		l.scroll = false
		first, _ := l.list.Visible()
		l.list.ScrollIntoView(revealedRow(l.rows, l.chosen, first))
	}
	if list.Changed() {
		// A click chose a row (off a title): the keyboard stays in the
		// field.
		l.choose(l.chosen)
		l.scroll = false
		l.moved = true
		l.input.Focus()
	}
	if list.Submitted() && !l.compact() {
		l.primary() // a double click
	}
}

// row shows a row, in the selection gray when chosen: the calculator's
// card, a group's title, Gocast's own item, or a search item.
func (l *launcher) row(c *ui.Context, k tokens, i int, chosen bool) {
	r := l.rows[i]
	switch {
	case r.calc:
		l.calcRow(c, k, chosen)
	case r.title != "":
		listSection(c, k, r.title, locales.SearchHeading(r.title))
	case r.own >= 0:
		l.ownRow(c, k, l.own[r.own], r.group, chosen)
	default:
		l.itemRow(c, k, r.item, chosen)
	}
}

// ownRow shows Gocast's own item as a search result shows (design.md
// 2.5a): its symbol on a square, its name, and "Gocast" at the right; in
// the Gocast group, its shortcut instead.
func (l *launcher) ownRow(c *ui.Context, k tokens, o ownItem, group, chosen bool) {
	listRow(c, k, chosen, func() {
		symbolSquare(c, k, o.symbol)
		listTitle(c, k, o.label)
		ui.Spacer(c)
		if !group {
			listNote(c, k, locales.SearchTagGocast()).Shrink(0)
		} else if len(o.caps) > 0 {
			keycaps(c, k, o.caps, 12)
		}
	})
}

// itemRow shows a search item: its icon, its name, a preset's folder and
// command, and what it is.
func (l *launcher) itemRow(c *ui.Context, k tokens, item int, chosen bool) {
	it := l.items[item]
	e := l.entries[item]
	subtitle := e.place // what tells an app from another of its name (design.md 2.3)
	tag := locales.SearchTagApp()
	switch {
	case e.own != "":
		tag = locales.SearchTagGocast()
	case e.app == nil:
		tag = locales.SearchTagPreset()
		if j := presets.Find(l.g.presets, e.preset); j >= 0 {
			subtitle = presetSubtitle(l.g.presets[j])
		}
	}
	listRow(c, k, chosen, func() {
		switch {
		case e.own != "":
			symbolSquare(c, k, ownSymbol(l.own, e.own))
		case e.app == nil:
			presetSquare(c, k)
		default:
			app := *e.app
			listIcon(c, func() {
				if b := l.icon(app); b != nil {
					ui.Image(c, b).Size(iconCell, iconCell)
				} else {
					ui.Avatar(c, it.Title, nil).Size(iconSize, iconSize) // its initial
				}
				if l.isRunning(app.ID) {
					// A dot under the icon, as the Dock's and Raycast's:
					// centered at the row's y 35, the cell spanning 7–33.
					ui.Box(c).Absolute().Left(iconCell/2-2).Bottom(-4).Size(4, 4).Radius(2).Background(k.textSecondary)
				}
			})
		}
		// The subtitle at most 40% of the row, cut in its middle; the
		// title shrinking first, to 160 (or its width), then the subtitle
		// (design.md 2.3).
		title := listTitle(c, k, it.Title)
		if subtitle != "" {
			w, _ := c.MeasureText(0, ui.Span{Text: it.Title, Size: 14})
			title.Margin(0, -2, 0, 0).Shrink(1000).MinWidth(min(w, listTitleLeast)) // 8 to the subtitle, not the row's 10
			ui.Text(c, subtitle).FontSize(13).TextColor(k.textOnGlass).SingleLine().TruncateMiddle().
				MaxWidth(listNoteMax).Shrink(1)
		}
		ui.Spacer(c)
		listNote(c, k, tag).Shrink(0)
	})
}

// rowKey keeps the choice on its row as the rows change: its item's ID,
// in the Gocast group apart from the suggestions.
func (l *launcher) rowKey(i int) any {
	r := l.rows[i]
	switch {
	case r.calc:
		return "calc"
	case r.title != "":
		return "title:" + r.title
	case r.own >= 0 && r.group:
		return "group:" + l.own[r.own].id
	case r.own >= 0:
		return l.own[r.own].id
	}
	return l.items[r.item].ID
}

// rowLabel is what assistive technology reads of a row (copy.md 2.3): a
// group's title, "Suggestions, heading"; in the Gocast group, the name;
// Gocast's own item elsewhere, as the clipboard history's result.
func (l *launcher) rowLabel(i int) string {
	r := l.rows[i]
	switch {
	case r.calc:
		return l.card(time.Now()).label
	case r.title != "":
		return locales.SearchHeading(r.title)
	case r.own >= 0 && r.group:
		return l.own[r.own].label
	case r.own >= 0:
		return l.own[r.own].label + listSep() + locales.SearchTagGocast()
	}
	return l.itemLabel(r.item)
}

// itemLabel is what assistive technology reads of a search item: "Safari,
// App"; "Siri, Siri AI, App" for an app told from another of its name, as
// shown; a preset's folder after.
func (l *launcher) itemLabel(item int) string {
	it, e := l.items[item], l.entries[item]
	sep := listSep()
	if e.own != "" {
		return it.Title + sep + locales.SearchTagGocast()
	}
	if e.app != nil {
		s := it.Title
		if e.place != "" {
			s += sep + e.place
		}
		s += sep + locales.SearchTagApp()
		if l.isRunning(e.app.ID) {
			s += sep + locales.SearchRunning()
		}
		return s
	}
	if j := presets.Find(l.g.presets, e.preset); j >= 0 {
		return presetLabel(l.g.presets[j])
	}
	return it.Title + sep + locales.SearchTagPreset()
}

// presetLabel is what assistive technology reads of a preset's row:
// "my-app · pi, Preset, ~/Projects/my-app".
func presetLabel(p presets.Preset) string {
	sep := listSep()
	return presetTitle(p) + sep + locales.SearchTagPreset() + sep + p.ShortDir()
}

func (l *launcher) noResults(c *ui.Context, k tokens) {
	q := strings.TrimSpace(l.query)
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Padding(0, 24).Children(func() {
		// Only the query is cut, at its end, the line at most 544 wide
		// (design.md 2.6).
		before, after := around(locales.SearchNoResults(queryMark), queryMark)
		ui.Row(c).MaxWidth(panelWidth - 96).Label(locales.SearchNoResults(q)).Children(func() {
			for i, part := range []string{before, q, after} {
				if part == "" {
					continue
				}
				t := ui.Text(c, part).FontSize(14).FontWeight(500).TextColor(k.text).SingleLine().Shrink(0).AccessHidden(true)
				if i == 1 {
					t.Shrink(1)
				}
			}
		})
		ui.Text(c, locales.SearchNoResultsHint()).FontSize(13).TextColor(k.textOnGlass).Margin(8, 0, 0, 0)
		if len(l.g.presets) == 0 {
			ui.Text(c, locales.SearchNoPresetsHint()).FontSize(13).TextColor(k.textOnGlass).Margin(4, 0, 0, 0)
		}
	})
}

// queryMark stands for the query in the no-results line, to find what is
// around it.
const queryMark = "\x00"

// around returns what comes before and after mark in s, all of s before
// it if it is not there.
func around(s, mark string) (before, after string) {
	before, after, _ = strings.Cut(s, mark)
	return before, after
}

// footer floats over the end of the list, as Raycast's (design.md 2.4):
// a glass capsule at the bottom right with the primary action and the
// actions menu, each clickable, and one at the bottom left with a notice,
// for a few seconds. h is the panel's height as drawn: with the search bar
// alone the capsule is centred in it (capsuleBottom).
func (l *launcher) footer(c *ui.Context, k tokens, h float32) {
	e, ok := l.current()
	// The calculator's card with no answer: Actions alone, dimmed, though
	// Cmd-K opens them (design.md 2.3a).
	can := ok && (!e.calc || l.card(time.Now()).ok())
	var segs []segment
	if can {
		segs = append(segs, segment{label: primaryName(e), color: k.text, key: ui.KeyEnter, caps: []string{"↩"}, act: l.primary})
	}
	// No results: nothing to act on. The search bar alone: the
	// suggestions (design.md 2.5a).
	segs = append(segs, segment{label: locales.FooterActions(), color: k.textOnGlass, mods: ui.Cmd, key: ui.KeyK,
		caps: []string{"⌘", "K"}, dim: !can && !l.compact(), act: l.toggleMenu})
	bottom := capsuleBottom(h)
	if l.notice != "" {
		n := notice{icon: "checkmark.circle", iconColor: k.textOnGlass, color: k.textOnGlass, text: l.notice}
		if l.noticeDanger {
			n.icon, n.iconColor, n.color = "exclamationmark.triangle.fill", k.danger, k.danger
		}
		noticeCapsule(c, k, capsuleInset, bottom, noticeRoom(panelWidth, actionsCapsuleWidth(c, segs...)), n)
	}
	actionsCapsule(c, k, capsuleInset, bottom, segs...)
}

// capsuleBottom is how far the actions capsule is from the bottom of the
// panel's inside, within its 0.5 border, at the height h (design.md 2.4):
// 8 at the bottom right of the list, and centred in the search bar alone,
// 15 above and below from the panel's edges (14.5 within the border),
// where it would otherwise sit low (22 above, 8 below). Between the two,
// as the panel grows or shrinks, it comes down to 8 over the panel's first
// 6.5 and then follows its bottom, so that it never jumps.
func capsuleBottom(h float32) float32 {
	const centred = (barHeight-capsuleHeight)/2 - panelBorderWidth // 14.5
	return capsuleInset + max(0, centred-capsuleInset-(h-barHeight))
}

// primaryName is the name of the primary action of what the row opens
// (copy.md 2.2a): Open; Open in Floating Terminal for a preset; Quit for
// Quit Gocast.
func primaryName(e entry) string {
	switch {
	case e.calc:
		return locales.CalcCopyAnswer()
	case e.own == quitItemID:
		return locales.FooterQuit()
	case e.own == "" && e.app == nil:
		return locales.FooterOpenInTerminal()
	}
	return locales.FooterOpen()
}

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

// chosenRow returns the row chosen, while the panel shows rows.
func (l *launcher) chosenRow() (panelRow, bool) {
	if l.compact() || l.chosen < 0 || l.chosen >= len(l.rows) || l.rows[l.chosen].title != "" {
		return panelRow{}, false
	}
	return l.rows[l.chosen], true
}

// current returns what the row chosen opens.
func (l *launcher) current() (entry, bool) {
	r, ok := l.chosenRow()
	switch {
	case !ok:
		return entry{}, false
	case r.calc:
		return entry{calc: true}, true
	case r.own >= 0:
		return entry{own: l.own[r.own].id}, true
	}
	return l.entries[r.item], true
}

// currentID returns the item ID of the row chosen, which usage.json
// scores.
func (l *launcher) currentID() string {
	r, ok := l.chosenRow()
	switch {
	case !ok, r.calc:
		return ""
	case r.own >= 0:
		return l.own[r.own].id
	}
	return l.items[r.item].ID
}

// --- The actions menu (Cmd-K) ---

func (l *launcher) toggleMenu() {
	if l.menuOpen {
		l.closeMenu()
		return
	}
	if l.compact() { // the search bar alone: the suggestions
		l.expand()
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
	if e.calc {
		return l.calcActions()
	}
	if e.own != "" { // its primary action alone (design.md 2.5a)
		return []action{{ownSymbol(l.own, e.own), primaryName(e), []string{"↩"}, l.primary}}
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

// menuDim reports whether the menu's actions can't be taken: the
// calculator's with no answer, all dimmed, as the card is now (the rates
// may come while the menu shows; design.md 2.3a).
func (l *launcher) menuDim() bool {
	e, ok := l.current()
	return ok && e.calc && !l.card(time.Now()).ok()
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
	if l.menuDim() {
		return
	}
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

func (l *launcher) actionsMenu(c *ui.Context, k tokens) {
	c.Key("actionsMenu")
	dim := l.menuDim()
	popupMenu(c, k, 1, 1).Right(capsuleInset).Bottom(capsuleInset + capsuleHeight + 8).Children(func() {
		menuField(c, k, func() {
			f := ui.TextInputBase(c, &l.menuQuery).Bind(&l.menuInput).Grow(1).FontSize(13).
				Placeholder(locales.ActionsPlaceholder()).Label(locales.ActionsPlaceholder()).
				OnTextKey(l.menuKey)
			l.menuComposing = f.Composing()
			if f.Changed() {
				l.filterMenu()
			}
		})
		menuItems(c, func() {
			if len(l.menuShown) == 0 {
				ui.Text(c, locales.ActionsNoMatch()).FontSize(13).TextColor(k.textOnGlass).Padding(8)
			}
			for i, ai := range l.menuShown {
				a := l.menuItems[ai]
				idx := i
				menuRowDim(c, k, a.icon, a.label, "", a.caps, i == l.menuChosen, dim, func() { l.runMenu(idx) })
			}
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
	if e.calc {
		l.calcAct(second)
		return
	}
	if e.own != "" { // its primary action alone
		if second {
			return
		}
		if scored(e.own) {
			l.g.record(e.own, l.query)
		}
		if i := ownIndex(l.own, e.own); i >= 0 && l.own[i].run != nil {
			l.own[i].run()
		}
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

// ownItems are Gocast's own items, with the shortcuts as they are.
func (l *launcher) ownItems() []ownItem {
	return gocastItems(l.g.terminalKey, l.g.clipKey, gocastDo{
		clipboard: func() { l.openClip(true) },
		terminal:  func() { l.hide(); l.g.term.open() },
		addPreset: func() { l.hide(); l.g.addPreset() },
		settings:  func() { l.hide(); l.g.openSettings("") },
		quit:      mygo.App.Quit,
	})
}

// rebuild makes the searchable items, the terminal presets then the apps,
// and finds the rows again, keeping the choice on its row.
func (l *launcher) rebuild() {
	// Keep the choice on its row as the apps come in, unless it is the
	// first row the user left as it was: the new first row takes it.
	var was any
	if _, ok := l.chosenRow(); ok {
		was = l.rowKey(l.chosen)
	}
	first := l.chosen == firstRow(l.rows) && !l.moved
	l.own = l.ownItems()
	l.items, l.entries = l.items[:0], l.entries[:0]
	now := time.Now()
	for _, p := range l.g.presets {
		id := "preset:" + p.ID
		it := search.NewItem(id, presetTitle(p), p.Command, "Terminal", "终端")
		it.Score = l.g.usage.Score(id, now)
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{preset: p.ID})
	}
	// Gocast's own items, by their titles in both languages, the
	// clipboard history also by the words users say for it (copy.md 2.2a,
	// 6.1).
	for _, o := range l.own {
		it := search.NewItem(o.id, o.label, ownSearchNames(o.id)...)
		it.Score = l.g.usage.Score(o.id, now)
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{own: o.id})
	}
	for i := range l.apps {
		app := &l.apps[i]
		id := "app:" + app.ID
		it := search.NewItem(id, app.Name, l.alts[i]...)
		it.Score = l.g.usage.Score(id, now)
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{app: app, place: l.places[i]})
	}
	l.itemAt = make(map[string]int, len(l.items))
	for i, it := range l.items {
		l.itemAt[it.ID] = i
	}
	if l.compact() {
		l.rows = l.find() // of the items as they are, for the panel to shrink with
		return
	}
	l.filtered = l.queryKey()
	l.rows = l.find()
	chosen := firstRow(l.rows)
	if !first && was != nil {
		for i := range l.rows {
			if l.rowKey(i) == was {
				chosen = i
				break
			}
		}
	}
	l.choose(chosen)
}

// filter finds the rows of a new query, the first chosen: the results, or
// the suggestions under an empty one, first at the top (design.md 2.5a).
// The search bar alone keeps the rows it had, which the panel shrinks
// with.
func (l *launcher) filter() {
	if !l.empty() {
		l.expanded = false // cleared again, the search bar alone
	}
	if l.compact() {
		l.filtered = noQuery // expanded, it starts afresh
		return
	}
	if l.queryKey() == l.filtered {
		return
	}
	l.filtered = l.queryKey()
	l.moved = false
	l.menuOpen = false
	l.rows = l.find()
	l.choose(firstRow(l.rows))
}

// queryKey is the query the rows are for: "" for an empty one.
func (l *launcher) queryKey() string {
	if l.empty() {
		return ""
	}
	return l.query
}

// noQuery is l.filtered when no rows were found for the query.
const noQuery = "\x00"

// find returns the rows of the query: its results, or the suggestions and
// Gocast's own items for an empty one.
func (l *launcher) find() []panelRow {
	l.evalCalc()
	if l.empty() {
		return suggestionRows(suggested(l.ranked, padIDs(l.g.presets), l.exists), l.itemIndex, l.own)
	}
	rows := itemRows(search.Filter(l.items, l.query, l.g.usage.Choice(l.query, time.Now())))
	if l.calcOn { // the calculator's card first (design.md 2.3a)
		rows = append([]panelRow{{calc: true, item: -1, own: -1}}, rows...)
	}
	return rows
}

// exists reports whether the item with the ID is there to open: an app
// still installed, a preset not deleted.
func (l *launcher) exists(id string) bool {
	if strings.HasPrefix(id, ownItemPrefix) {
		return ownIndex(l.own, id) >= 0
	}
	_, ok := l.itemAt[id]
	return ok
}

// itemIndex is the index of the search item with the ID, or -1.
func (l *launcher) itemIndex(id string) int {
	if i, ok := l.itemAt[id]; ok {
		return i
	}
	return -1
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
		// One of the copies of an app; apps of one name told apart by
		// their folders.
		all := make([]apps.App, len(list))
		for i, n := range list {
			all[i] = apps.App{ID: n.app.ID, Path: n.app.Path, Name: n.app.Name}
		}
		keep := apps.Unique(all)
		found = found[:0]
		alts := make([][]string, len(keep))
		kept := make([]apps.App, len(keep))
		for i, j := range keep {
			found = append(found, list[j].app)
			alts[i] = list[j].alts
			kept[i] = all[j]
		}
		places := apps.Places(kept)
		l.win.Update(func() {
			l.apps, l.alts, l.places, l.loading, l.loaded = found, alts, places, false, true
			l.rebuild()
			if l.g.prefs != nil {
				l.g.prefs.win.Invalidate() // Settings › Clipboard shows the apps it ignores
			}
		})
	}()
}

// --- Showing and hiding ---

// show shows the panel on the display with the pointer, its top a fifth of
// the way down: as it was if it hid less than 90 seconds ago, its query
// selected so that typing replaces it; else empty, with the suggestions
// (or its bar alone, as Settings › General asks).
func (l *launcher) show() {
	if l.win.IsVisible() {
		if l.clip.open {
			l.leaveClip()
		}
		l.input.Focus()
		return
	}
	// Where Cmd-Return pastes the calculator's answer, as the clipboard
	// view's (design.md 2.3a, 2.8.6): known before Gocast comes forward.
	l.target = pasteTarget{app: frontmostOther(), floating: l.g.term.focused()}
	l.g.calc.shown() // the region and number format, and the rates if due
	l.enterSearch(time.Now())
	l.calcChanged() // a query kept, in the region and with the rates as they are now
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	wa := d.WorkArea
	l.win.SetPosition(wa.X+(wa.Width-windowWidth)/2, wa.Y+wa.Height/5-marginTop)
	l.input.Focus()
	l.win.Show()
	l.win.Invalidate() // draw the state changed outside a frame
	if l.g.loadPresets() {
		l.g.presetsChanged()
	}
	l.g.refreshAppHotkeys(false) // an app installed again
	l.load()
}

// enterSearch is show's state: all of it changes before the window
// shows, which then draws the search panel as its first frame.
func (l *launcher) enterSearch(now time.Time) {
	l.clip.open = false
	restore := restoring(l.hiddenAt, now)
	// The setting as the panel shows: switched while it shows, it does not
	// change (design.md 4.4).
	l.searchOnly = l.g.settings.SearchOnly
	l.shown = true
	l.own = l.ownItems()
	// Ranked as it shows, the suggestions do not move until it shows again
	// with an empty query after 90 seconds (design.md 2.5a).
	if !restore || !l.empty() {
		l.ranked = l.g.usage.Ranked(now)
	}
	if !restore {
		l.query, l.filtered, l.rows, l.moved, l.expanded = "", noQuery, nil, false, false
		l.notice = ""
		l.list = ui.ListState{}
	}
	l.filter() // afresh; as they were, if restored
	l.selectAll = restore && l.query != ""
	clear(l.running) // apps started or quit meanwhile
	l.menuOpen = false
	l.gen++ // appears at its size, without animating to it
}

func (l *launcher) hide() {
	if !l.win.IsVisible() {
		return
	}
	l.leave(time.Now())
	l.win.Hide()
	l.win.Invalidate() // nothing, the frame the window keeps (see view)
}

// leave is hide's state: the search panel kept for 90 seconds, the
// clipboard view not (design.md 2.8.1); nothing drawn (see view).
func (l *launcher) leave(now time.Time) {
	if l.clip.open {
		l.clip.closed()
	} else {
		l.hiddenAt = now
	}
	l.menuOpen = false
	l.shown = false
}

// toggle shows or hides the search panel: from the clipboard view, it goes
// back to the search panel, empty (design.md 2.8.1).
func (l *launcher) toggle() {
	if l.win.IsVisible() && l.clip.open {
		l.leaveClip()
		return
	}
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
	if data, err := mygo.Shell.FileIcon(app.Path, int(iconCell*scale)); err == nil {
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

// presetSubtitle says where and what a preset runs: "~/Projects/x · pi".
func presetSubtitle(p presets.Preset) string {
	if strings.TrimSpace(p.Command) == "" {
		return locales.PresetSubtitleShell(p.ShortDir())
	}
	return locales.PresetSubtitle(p.ShortDir(), p.Command)
}
