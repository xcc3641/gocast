package app

// The notes window's cards (design.md 11.6, 11.7): Browse Notes (⌘P) and
// the actions menu (⌘K), drawn in the window over the note; and the keys
// of the window, which act on them while they show.

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/notes"
	"gocast/internal/search"
	"gocast/internal/sysfmt"
	"gocast/locales"
)

// Browse Notes' measures (design.md 11.6), in DIPs.
const (
	browseWidth    = 440
	browseRowH     = 44
	browseTitleH   = 26
	browseNoneH    = 64
	browsePad      = 6
	notesBarHeight = 44
	notesMenuWidth = 280
)

// noteBrowse is Browse Notes' state.
type noteBrowse struct {
	open      bool
	query     string
	input     ui.Handle
	composing bool
	focus     bool // focus the field in the next frame
	rows      []browseRow
	list      ui.ListState
	chosen    int
	scroll    bool // show the choice in the next frame
}

// browseRow is a row of Browse Notes: a section's title, or a note found.
type browseRow struct {
	title string
	m     notes.Match
	nth   int // its place among the notes pinned, 1 to 9, for its keys; else 0
}

// openBrowse opens Browse Notes, the note showing chosen (design.md
// 11.6); not in the empty state.
func (n *notesWindow) openBrowse() {
	if n.cur == nil {
		return // the empty state: ⌘P does nothing (design.md 11.8)
	}
	n.scan()
	b := &n.browse
	n.menu.open = false
	b.open, b.query, b.focus = true, "", true
	b.refilter(n)
	b.chooseNote(n.curID())
	n.redraw()
}

// curID is the ID of the note showing, "" for a new one or none.
func (n *notesWindow) curID() string {
	if n.cur == nil {
		return ""
	}
	return n.cur.id
}

// closeBrowse closes Browse Notes, back to the note (or the first, the
// one showing deleted), the caret where it was.
func (n *notesWindow) closeBrowse() {
	if !n.browse.open {
		return
	}
	n.browse.open = false
	if n.cur == nil {
		n.showFirst()
	}
	n.focusText = true
	n.redraw()
}

// closeCards closes both cards.
func (n *notesWindow) closeCards() {
	n.closeBrowse()
	n.menu.open = false
}

// refilter finds the rows again for the query, the choice on the note it
// was on, else the first (design.md 11.6).
func (b *noteBrowse) refilter(n *notesWindow) {
	if !b.open {
		return
	}
	was := b.chosenID()
	b.rows = browseRows(n.store, b.query)
	b.chooseNote(was)
}

// browseRows lays out the notes the query finds: the pinned under their
// title, then the others under theirs, the titles only while some are
// pinned.
func browseRows(s *notes.Store, query string) []browseRow {
	found := notes.Search(s.List(), query)
	pinned := s.PinnedIDs()
	var rows []browseRow
	titled := len(found) > 0 && s.Pinned(found[0].Note.ID) // some pinned found
	recent := false
	for i, m := range found {
		on := s.Pinned(m.Note.ID)
		if titled && i == 0 && on {
			rows = append(rows, browseRow{title: locales.NotesSectionPinned()})
		}
		if titled && !on && !recent {
			recent = true
			rows = append(rows, browseRow{title: locales.NotesSectionRecent()})
		}
		r := browseRow{m: m}
		if k := slices.Index(pinned, m.Note.ID); on && k < 9 {
			r.nth = k + 1
		}
		rows = append(rows, r)
	}
	return rows
}

// chosenID is the note of the row chosen, "".
func (b *noteBrowse) chosenID() string {
	if b.chosen >= 0 && b.chosen < len(b.rows) && b.rows[b.chosen].title == "" {
		return b.rows[b.chosen].m.Note.ID
	}
	return ""
}

// chooseNote chooses the row of the note id, else the first note.
func (b *noteBrowse) chooseNote(id string) {
	at := -1
	for i, r := range b.rows {
		if r.title != "" {
			continue
		}
		if at < 0 {
			at = i
		}
		if id != "" && r.m.Note.ID == id {
			at = i
			break
		}
	}
	b.choose(at)
}

func (b *noteBrowse) choose(i int) {
	b.chosen = i
	b.scroll = true
}

// move moves the choice by step rows, over the titles, not wrapping.
func (b *noteBrowse) move(step int) {
	at := b.chosen
	for i := b.chosen + sign(step); i >= 0 && i < len(b.rows) && step != 0; i += sign(step) {
		if b.rows[i].title == "" {
			at = i
			step -= sign(step)
		}
	}
	if at >= 0 {
		b.choose(at)
	}
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

// removed follows a note deleted from the card: the choice on the row
// after it, or before it when it was the last (design.md 11.6).
func (b *noteBrowse) removed(n *notesWindow, id string) {
	if !b.open {
		return
	}
	at := b.chosen
	b.rows = browseRows(n.store, b.query)
	for at < len(b.rows) && at >= 0 && b.rows[at].title != "" {
		at++
	}
	if at >= len(b.rows) {
		at = len(b.rows) - 1
		for at >= 0 && b.rows[at].title != "" {
			at--
		}
	}
	b.choose(at)
	if len(b.rows) == 0 && b.query == "" {
		n.closeBrowse() // none left: the empty state
	}
}

// openChosen opens the note chosen, the card closed.
func (n *notesWindow) openChosen() {
	id := n.browse.chosenID()
	if id == "" {
		return
	}
	n.browse.open = false
	n.showNote(id)
	n.focusText = true
	n.redraw()
}

// browseKey takes the keys of Browse Notes' field before it: none while
// the input method composes (design.md 11.9).
func (n *notesWindow) browseKey(key ui.TextKey) bool {
	b := &n.browse
	if b.composing {
		return false
	}
	if q := key.Text(); q != b.query {
		bound := b.query
		b.query = q
		b.refilter(n)
		b.chooseNote("")
		defer func() { b.query = bound }()
	}
	mods := key.Mods
	switch {
	case key.Key == ui.KeyEscape && mods == 0:
		n.closeBrowse()
	case key.Key == ui.KeyEnter && mods == 0:
		n.openChosen()
	case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl, key.Key == ui.KeyJ && mods == ui.Ctrl:
		b.move(1)
	case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl, key.Key == ui.KeyK && mods == ui.Ctrl:
		b.move(-1)
	case key.Key == ui.KeyBackspace && mods == ui.Cmd && key.Text() == "":
		n.removeChosen() // ⌘⌫ deletes the note, the field empty (design.md 11.6)
	default:
		return false
	}
	return true
}

// removeChosen deletes the note chosen in Browse Notes.
func (n *notesWindow) removeChosen() {
	if id := n.browse.chosenID(); id != "" {
		n.remove(id)
	}
}

// --- The actions menu ---

// noteMenu is the actions menu's state.
type noteMenu struct {
	open      bool
	query     string
	input     ui.Handle
	composing bool
	chosen    int
	shown     []int // the actions the query leaves, by index
}

// noteAction is a row of the actions menu.
type noteAction struct {
	icon, label string
	caps        []string
	danger, dim bool
	run         func()
}

// menuActions are the menu's rows (copy.md 16.5): in the empty state,
// New Note alone can be taken; a note with no text can't be pinned,
// copied or deleted.
func (n *notesWindow) menuActions() []noteAction {
	none := n.cur == nil
	empty := none || n.cur.doc.Text().Len() == 0
	pin, pinIcon := locales.NotesPin(), "pin"
	if !none && n.store.Pinned(n.cur.id) {
		pin, pinIcon = locales.NotesUnpin(), "pin.slash"
	}
	return []noteAction{
		{icon: "plus", label: locales.NotesNewItem(), caps: []string{"⌘", "N"}, run: n.newNote},
		{icon: "list.bullet.rectangle", label: locales.NotesBrowse(), caps: []string{"⌘", "P"}, dim: none, run: n.openBrowse},
		{icon: pinIcon, label: pin, caps: []string{"⇧", "⌘", "P"}, dim: empty, run: n.pinCurrent},
		{icon: "doc.on.doc", label: locales.NotesCopyMarkdown(), caps: []string{"⇧", "⌘", "C"}, dim: empty, run: n.copyMarkdown},
		{icon: "trash", label: locales.NotesDelete(), caps: []string{"⌃", "X"}, danger: true, dim: empty, run: n.removeCurrent},
	}
}

// toggleMenu opens the actions menu, or closes it.
func (n *notesWindow) toggleMenu() {
	m := &n.menu
	if m.open {
		n.closeMenu()
		return
	}
	n.browse.open = false
	m.open, m.query, m.chosen = true, "", 0
	n.filterMenu()
	m.input.Focus()
	n.redraw()
}

func (n *notesWindow) closeMenu() {
	n.menu.open = false
	n.focusText = true
	n.redraw()
}

// filterMenu leaves the actions the query finds; the first that can be
// taken is chosen.
func (n *notesWindow) filterMenu() {
	m := &n.menu
	as := n.menuActions()
	m.shown = m.shown[:0]
	for i, a := range as {
		if search.NewKey(a.label).Rank(m.query) >= 0 {
			m.shown = append(m.shown, i)
		}
	}
	m.chosen = max(0, min(m.chosen, len(m.shown)-1))
}

// runMenu takes the action of the row i, unless it can't be taken.
func (n *notesWindow) runMenu(i int) {
	m := &n.menu
	if i < 0 || i >= len(m.shown) {
		return
	}
	a := n.menuActions()[m.shown[i]]
	if a.dim {
		return
	}
	m.open = false
	n.focusText = true
	a.run()
	n.redraw()
}

// menuKey takes the keys of the menu's field before it.
func (n *notesWindow) menuKey(key ui.TextKey) bool {
	m := &n.menu
	if m.composing {
		return false
	}
	if q := key.Text(); q != m.query {
		bound := m.query
		m.query = q
		n.filterMenu()
		defer func() { m.query = bound }()
	}
	mods := key.Mods
	switch {
	case key.Key == ui.KeyEscape && mods == 0:
		n.closeMenu()
	case key.Key == ui.KeyEnter && mods == 0:
		n.runMenu(m.chosen)
	case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl:
		m.chosen = min(m.chosen+1, len(m.shown)-1)
	case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl:
		m.chosen = max(m.chosen-1, 0)
	default:
		return false
	}
	return true
}

// --- The window's keys ---

// digitKeys are ⌘1 to ⌘9's keys.
var digitKeys = []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}

// windowKeys are the window's keys (design.md 11.9), whatever has the
// focus: the editor's onKey takes them first where it has it.
var windowKeys = []struct {
	mods ui.Modifiers
	key  ui.Key
}{
	{ui.Cmd, ui.KeyN}, {ui.Cmd, ui.KeyP}, {ui.Cmd, ui.KeyK}, {ui.Cmd | ui.Shift, ui.KeyP},
	{ui.Cmd | ui.Shift, ui.KeyC}, {ui.Ctrl, ui.KeyX}, {ui.Cmd, ui.KeyF}, {ui.Cmd, ui.KeyG},
	{ui.Cmd | ui.Shift, ui.KeyG}, {ui.Cmd, ui.KeyEqual}, {ui.Cmd | ui.Shift, ui.KeyEqual},
	{ui.Cmd, ui.KeyMinus}, {ui.Cmd, ui.Key0}, {ui.Cmd, ui.KeyW}, {ui.Cmd, ui.KeyQ},
	{ui.Cmd, ui.Key1}, {ui.Cmd, ui.Key2}, {ui.Cmd, ui.Key3}, {ui.Cmd, ui.Key4}, {ui.Cmd, ui.Key5},
	{ui.Cmd, ui.Key6}, {ui.Cmd, ui.Key7}, {ui.Cmd, ui.Key8}, {ui.Cmd, ui.Key9},
}

// command runs the window's key, on the card showing where it has one
// (design.md 11.6, 11.8, 11.9), and reports whether it took it.
func (n *notesWindow) command(key ui.Key, mods ui.Modifiers) bool {
	b := &n.browse
	empty := n.cur == nil // the empty state
	switch {
	case mods == ui.Cmd && key == ui.KeyN:
		n.closeCards()
		n.newNote()
	case mods == ui.Cmd && key == ui.KeyP:
		switch {
		case b.open:
			n.closeBrowse()
		case !empty:
			n.openBrowse()
		}
	case mods == ui.Cmd && key == ui.KeyK:
		n.toggleMenu()
	case mods == ui.Cmd|ui.Shift && key == ui.KeyP:
		if b.open {
			n.togglePin(b.chosenID())
		} else {
			n.pinCurrent()
		}
	case mods == ui.Cmd|ui.Shift && key == ui.KeyC:
		n.copyMarkdown()
	case mods == ui.Ctrl && key == ui.KeyX:
		if b.open {
			n.removeChosen()
		} else {
			n.removeCurrent()
		}
	case mods == ui.Cmd && key == ui.KeyF:
		if !b.open && !n.menu.open {
			n.openFind()
		}
	case mods == ui.Cmd && key == ui.KeyG, mods == ui.Cmd|ui.Shift && key == ui.KeyG:
		if !n.find.open {
			return false
		}
		n.stepFind(map[bool]int{true: -1, false: 1}[mods&ui.Shift != 0])
	case mods == ui.Cmd && key == ui.KeyEqual, mods == ui.Cmd|ui.Shift && key == ui.KeyEqual:
		n.zoom(1)
	case mods == ui.Cmd && key == ui.KeyMinus:
		n.zoom(-1)
	case mods == ui.Cmd && key == ui.Key0:
		n.zoom(0)
	case mods == ui.Cmd && (key == ui.KeyW || key == ui.KeyQ):
		n.hide() // ⌘Q too: Gocast stays (design.md 11.2)
	case mods == ui.Cmd && slices.Contains(digitKeys, key):
		n.openPinned(slices.Index(digitKeys, key) + 1)
	default:
		return false
	}
	n.redraw()
	return true
}

// escape is Esc's (design.md 11.2): the input method's while it
// composes, else it closes the card, else the find bar, else the window.
func (n *notesWindow) escape() {
	switch {
	case n.composing:
	case n.browse.open:
		n.closeBrowse()
	case n.menu.open:
		n.closeMenu()
	case n.find.open:
		n.closeFind()
	default:
		n.hide()
	}
}

// --- Drawing the cards ---

// browseCard draws Browse Notes (design.md 11.6) in a window w by h.
func (n *notesWindow) browseCard(c *ui.Context, k tokens, w, h float32) {
	b := &n.browse
	c.Key("browse")
	width := min(w-16, browseWidth)
	card := popupMenu(c, k, 0.5, 0).Top(notesBarHeight + 4).Left((w - width) / 2).Width(width).MaxHeight(h - 56)
	card.Children(func() {
		menuField(c, k, func() {
			f := ui.TextInputBase(c, &b.query).Bind(&b.input).Grow(1).FontSize(13).
				Placeholder(locales.NotesSearchPlaceholder()).Label(locales.NotesSearchPlaceholder()).
				OnTextKey(n.browseKey)
			b.composing = f.Composing()
			if f.Changed() {
				b.refilter(n)
				b.chooseNote("")
			}
			if b.focus {
				b.focus = false
				b.input.Focus()
			}
		})
		if len(b.rows) == 0 {
			ui.Box(c).Height(browseNoneH).Center().Shrink(0).Children(func() {
				ui.Text(c, locales.NotesNoMatch()).FontSize(13).TextColor(k.textOnGlass)
			})
			return
		}
		b.list.Selected = &b.chosen
		b.list.Key = func(i int) any {
			if r := b.rows[i]; r.title != "" {
				return "title:" + r.title
			}
			return b.rows[i].m.Note.ID
		}
		b.list.Label = func(i int) string { return n.browseLabel(b.rows[i]) }
		var list ui.Element
		ui.Themed(c, searchListTheme(c, k)).Background(ui.Transparent).Shrink(1).Children(func() {
			list = ui.List(c, &b.list, len(b.rows)).Padding(browsePad).
				Rows(func(r ui.ListRow) { n.browseRow(c, k, r.Index, r.Selected()) })
		})
		if b.scroll && b.chosen >= 0 {
			b.scroll = false
			b.list.ScrollIntoView(b.chosen)
		}
		if list.Changed() { // a click chose a row: the keys stay in the field
			if b.chosen >= 0 && b.chosen < len(b.rows) && b.rows[b.chosen].title != "" {
				b.move(1)
			}
			b.input.Focus()
		}
		if list.Submitted() {
			n.openChosen() // a double click
		}
	})
}

// browseRow draws a row of Browse Notes: a section's title, or a note's
// title over when it changed (or the line found), its keys at the right
// (design.md 11.6).
func (n *notesWindow) browseRow(c *ui.Context, k tokens, i int, chosen bool) {
	r := n.browse.rows[i]
	if r.title != "" {
		ui.Column(c).Height(browseTitleH).Shrink(0).Justify(ui.End).Padding(0, 0, 4, 8).Label(locales.SearchHeading(r.title)).Children(func() {
			ui.Text(c, r.title).FontSize(12).FontWeight(500).TextColor(k.textOnGlass).SingleLine().AccessHidden(true)
		})
		return
	}
	note := r.m.Note
	title, titleColor := note.Title(), k.text
	if title == "" {
		title, titleColor = locales.NotesUntitled(), k.textOnGlass
	}
	when := sysfmt.Date(note.Modified, n.now(), sysLocale())
	sub := when
	if r.m.Snippet != "" {
		sub = r.m.Snippet
	}
	row := ui.Row(c).Height(browseRowH).Label(n.browseLabel(r)).Shrink(0).Padding(0, 6, 0, 8).Gap(8).AlignItems(ui.Center).Radius(8)
	if chosen {
		row.Background(k.selection)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(13).TextColor(titleColor).SingleLine()
			ui.Text(c, sub).FontSize(12).TextColor(k.textOnGlass).SingleLine()
		})
		switch {
		case r.m.Snippet != "":
			ui.Text(c, when).FontSize(12).TextColor(k.textOnGlass).SingleLine().Shrink(0)
		case r.nth > 0 && n.browse.query == "":
			ui.Row(c).Shrink(0).AccessHidden(true).Children(func() { // design.md 11.10
				keycaps(c, k, []string{"⌘", strconv.Itoa(r.nth)}, 12)
			})
		}
	})
}

// browseLabel is what assistive technology reads of a row (copy.md 16.9):
// "Groceries, Pinned, Today 14:05".
func (n *notesWindow) browseLabel(r browseRow) string {
	if r.title != "" {
		return locales.SearchHeading(r.title)
	}
	title := r.m.Note.Title()
	if title == "" {
		title = locales.NotesUntitled()
	}
	when := sysfmt.Date(r.m.Note.Modified, n.now(), sysLocale())
	if n.store.Pinned(r.m.Note.ID) {
		return locales.NotesRowLabelPinned(title, when)
	}
	return locales.NotesRowLabel(title, when)
}

// menuCard draws the actions menu (design.md 11.7): 280 wide, at the top
// right, under the top bar.
func (n *notesWindow) menuCard(c *ui.Context, k tokens) {
	m := &n.menu
	c.Key("notesMenu")
	as := n.menuActions()
	popupMenu(c, k, 1, 0).Top(notesBarHeight + 4).Right(8).Width(notesMenuWidth).Children(func() {
		menuField(c, k, func() {
			f := ui.TextInputBase(c, &m.query).Bind(&m.input).Grow(1).FontSize(13).
				Placeholder(locales.ActionsPlaceholder()).Label(locales.ActionsPlaceholder()).
				OnTextKey(n.menuKey)
			m.composing = f.Composing()
			if f.Changed() {
				n.filterMenu()
			}
		})
		menuItems(c, func() {
			if len(m.shown) == 0 {
				ui.Text(c, locales.ActionsNoMatch()).FontSize(13).TextColor(k.textOnGlass).Padding(8)
			}
			for i, ai := range m.shown {
				a := as[ai]
				if a.danger && i > 0 {
					menuSeparator(c, k) // the one row that takes the note away, apart
				}
				idx := i
				noteMenuRow(c, k, a, i == m.chosen, func() { n.runMenu(idx) })
			}
		})
	})
}

// noteMenuRow is menuRowDim, its icon and name in danger for Delete
// Note (design.md 11.7).
func noteMenuRow(c *ui.Context, k tokens, a noteAction, chosen bool, act func()) {
	if !a.danger {
		menuRowDim(c, k, a.icon, a.label, "", a.caps, chosen, a.dim, act)
		return
	}
	bg := ui.Transparent
	if chosen {
		bg = k.selection
	}
	text, secondary := k.danger, k.textOnGlass
	if a.dim {
		text, secondary = text.Alpha(0.4), secondary.Alpha(0.4)
	}
	c.Key(a.label)
	r := ui.Row(c).Height(menuRowHeight).Padding(0, menuRowPadR, 0, menuRowPad).Radius(8).Gap(8).AlignItems(ui.Center).Background(bg).
		Role(ui.RoleMenuItem).Label(a.label)
	if a.dim {
		r.AccessDisabled(true)
	} else {
		r.OnClick(act)
	}
	r.Children(func() {
		sfsymbols.Symbol(c, a.icon).Size(16).Color(text)
		ui.Text(c, a.label).FontSize(13).TextColor(text).SingleLine().Shrink(1).Grow(1).Margin(0, menuNameGap-8, 0, 0)
		keycapsColored(c, k, a.caps, 12, secondary)
	})
}
