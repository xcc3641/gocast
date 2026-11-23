package app

// Settings › Search & Shortcuts (design.md 4.4, copy.md 19, 11.3): one
// list of all the search panel finds, and the rest that can have a
// shortcut, in fixed groups (Gocast, System & Window, Presets, System
// Apps, Other Apps), searched and filtered; each row records its own
// shortcut inline, and its switch says whether the search shows it.

import (
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/keepawake"
	"gocast/internal/search"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/internal/sysact"
	"gocast/locales"
)

// The Settings window's size (design.md 4.1): room for the shortcuts'
// name, recorder and switch columns, about 11 rows high.
const (
	prefsWidth  = 820
	prefsHeight = 580
)

// The measures of the list (design.md 4.4).
const (
	hkRowHeight   = 36
	hkTitleHeight = 40 // a group's title
	hkRowPad      = 10 // a row's padding, left and right
	hkListPad     = 4  // the list's block, around the list
	hkIcon        = 20
	hkRecGap      = 12 // between the name and the recorder
	hkRecWidth    = 150
	hkRecHeight   = 24
	hkSwitchGap   = 16 // between the recorder and the switch
	hkSwitchW     = 32 // the small switch
	hkSwitchH     = 18
	hkDescLeft    = hkIcon + 10 // the line under a row, from the row's padding: under the name
	hkNoteHeight  = 52          // the line under Gocast's own in With Shortcut, none set
	hkExpandFor   = 120 * time.Millisecond
	hkContentLeft = 200 + pagePadX // the sidebar and the content's padding
)

// hkListWidth is a row's width: the content area's, less its padding and
// the list card's border and padding.
const hkListWidth = prefsWidth - hkContentLeft - pagePadX - 2*cardBorder - 2*hkListPad

// hkNoteWidth is the widest a line of the list empty, or of the note
// under Gocast's own, is: 20 clear of either side (design.md 4.4).
const hkNoteWidth = hkListWidth - 2*20

// The filters of the list, in the segmented control's order.
const (
	hkFilterAll = iota
	hkFilterSet
	hkFilterHidden
)

// The kinds of row, which assistive technology reads.
const (
	hkGocast = iota
	hkApp
	hkPreset
	hkSystem
	hkWindow
)

// errSpotlightBusy: ⌘Space for the search panel while Spotlight has it,
// which the system's hot keys don't refuse (both would open).
var errSpotlightBusy = errors.New("spotlight has Cmd-Space")

// hkRow is a row of the list.
type hkRow struct {
	// id: "own:1" to "own:3", "note:" + item, "gocast:" + item, "app:"
	// + bundle ID, "preset:" + ID, "action:" + ID, "awake:" + item.
	id   string
	own  int             // 1 to 3 for Gocast's own shortcuts, by its row; else 0
	item settings.Hotkey // the others', but Quit Gocast's
	// noKeys: no recorder, Quit Gocast (design.md 4.4).
	noKeys bool
	kind   int
	group  string // one of settings.Groups
	// searchID is the search item the row's switch hides, "" for no
	// switch (the search panel's own shortcut).
	searchID string
	symbol   string // a Gocast row's or Keep Awake's, on a square
	name     string
	path     string // an app's bundle, "" for a preset or an app missing
	missing  bool   // an app not installed, kept for its hotkey
	match    search.Item
}

// hkLine is a line of the list: a group's title (row -1) or a row.
type hkLine struct {
	group string
	row   int
}

// hotkeyPage is the state of Settings › Search & Shortcuts.
type hotkeyPage struct {
	rows []hkRow // all, by group, in their order
	made string  // what rows were made of (hotkeyRowsOf)
	// matched are the rows the query and filter left as they last
	// changed, in order: a row changed since (its shortcut cleared,
	// shown again) stays until they change (design.md 4.4).
	matched []int
	lines   []hkLine // what the list shows: the groups' titles, their rows
	// folded are the groups collapsed while searching, which shows them
	// all: until the query changes, the settings' kept as they are.
	folded []string
	// none: With Shortcut, and no app or preset has one: a line says so
	// under Gocast's own, as the list's last row.
	none   bool
	query  string
	filter int
	list   ui.ListState
	chosen int
	search ui.Handle
	scroll bool // show the choice in the next frame
	rec    hkRecording
	recBox ui.Handle
	lost   bool // the recorder lost the focus: cancel
	// composing: the input method composes in the search field.
	composing bool
	accent    ui.Color // the window's, for the recorder recording
	// theme is the window's, for the switches: the list's has no
	// accent.
	theme ui.Theme
}

// hkRecording is the recorder of a row, as it records.
type hkRecording struct {
	id string // the row recording, "" for none
	// mods are the modifiers held, shown before the key comes; pressed
	// the shortcut pressed last, refused with problem; movable when
	// another app's or preset's, from, has it, which Use Here moves.
	mods    ui.Modifiers
	pressed string
	problem string
	movable bool
	from    settings.Hotkey
	say     string // read out in the next frame
}

func (p *prefsWindow) redraw() {
	if p.win != nil {
		p.win.Invalidate()
	}
}

func (p *prefsWindow) isRecording() bool { return p.hk.rec.id != "" }

// --- The rows ---

// hotkeyRowsOf is what the rows are made of: the apps the search panel
// found and the presets, which change them.
func (p *prefsWindow) hotkeyRowsOf() string {
	l := p.g.launcher
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(l.apps)))
	if len(l.apps) > 0 {
		b.WriteString(l.apps[0].Path) // the apps found again
		b.WriteString(l.apps[len(l.apps)-1].Path)
	}
	for _, ps := range p.g.presets {
		b.WriteString("\x00" + ps.ID + "\x01" + presetTitle(ps))
	}
	return b.String()
}

// makeHotkeyRows makes the rows, by group (design.md 4.4): Gocast's, in
// their order; the system's actions, Keep Awake's rows, the window's
// actions, in copy.md's order (Left Half, Right Half... together); the
// presets, the system's apps and the others, those with a hotkey the
// search panel does not know too, by name (Chinese names by their
// pinyin), as the search panel sorts them.
func (p *prefsWindow) makeHotkeyRows() []hkRow {
	g, l := p.g, p.g.launcher
	rows := make([]hkRow, 0, 9+len(g.presets)+len(l.apps)+len(sysact.Actions)+5)
	gocast := func(r hkRow) {
		r.kind, r.group = hkGocast, settings.GroupGocast
		names := ownSearchNames(r.searchID, "")
		if r.own > 0 {
			names = nil
		}
		r.match = search.NewItem(r.id, r.name, names...)
		rows = append(rows, r)
	}
	ownIDs := [4]string{"", "", termItemID, clipItemID} // the search panel's own has no item
	for r := 1; r <= 3; r++ {
		gocast(hkRow{id: "own:" + strconv.Itoa(r), own: r, name: shortcutLabel(r), searchID: ownIDs[r]})
	}
	// The notes' three after them, with no default (design.md 11.1,
	// copy.md 16.8).
	for _, item := range settings.NoteItems {
		gocast(hkRow{id: "note:" + item, item: settings.Hotkey{Note: item}, name: noteItemName(item),
			searchID: noteOwnID(item), symbol: ownSymbolOf(noteOwnID(item))})
	}
	for _, item := range settings.OwnItems {
		id, symbol := ownHotkeyItemID(item), "plus"
		if item == settings.OwnSettings {
			symbol = "gearshape"
		}
		gocast(hkRow{id: id, item: settings.Hotkey{Own: item}, name: ownHotkeyName(item), searchID: id, symbol: symbol})
	}
	gocast(hkRow{id: quitItemID, noKeys: true, name: locales.TrayQuit(), searchID: quitItemID, symbol: "power"})

	// The system's actions, Keep Awake's rows after them, then the
	// window's.
	action := func(a sysact.Action) {
		id, name, kind := actionPrefix+a.ID, actionName(a.ID), hkSystem
		if a.Kind == sysact.KindWindow {
			kind = hkWindow
		}
		rows = append(rows, hkRow{id: id, item: actionHotkey(a.ID), kind: kind, group: settings.GroupSystem, name: name,
			searchID: id, match: search.NewItem(id, name, actionSearchNames(a.ID)...)})
	}
	for _, a := range sysact.Actions {
		if a.Kind != sysact.KindWindow {
			action(a)
		}
	}
	en, zh := locales.In("en"), locales.In("zh-Hans")
	awakeNames := []string{en.AwakeTitle(), zh.AwakeTitle()}
	for _, item := range awakeHotkeyItems() {
		id, name := awakeItemID(item), awakeItemName(item)
		names := awakeNames
		if r, ok := awakeRequest(item); ok { // by their names in both languages too
			names = append([]string{keepawake.Words{Msgs: en}.Title(r, time.Time{}), keepawake.Words{Msgs: zh}.Title(r, time.Time{})}, names...)
		}
		rows = append(rows, hkRow{id: id, item: settings.Hotkey{Awake: item}, kind: hkSystem, group: settings.GroupSystem,
			name: name, searchID: id, symbol: "cup.and.saucer", match: search.NewItem(id, name, names...)})
	}
	for _, a := range sysact.Actions {
		if a.Kind == sysact.KindWindow {
			action(a)
		}
	}

	type keyed struct {
		row hkRow
		key string
	}
	var items []keyed
	add := func(r hkRow) { items = append(items, keyed{r, search.SortKey(r.name)}) }
	for _, ps := range g.presets {
		id, name := "preset:"+ps.ID, presetTitle(ps)
		add(hkRow{id: id, item: settings.Hotkey{Preset: ps.ID}, kind: hkPreset, group: settings.GroupPresets, name: name, searchID: id, match: search.NewItem(id, name)})
	}
	known := make(map[string]bool, len(l.apps))
	for i, a := range l.apps {
		known[a.ID] = true
		var alts []string
		if i < len(l.alts) {
			alts = l.alts[i]
		}
		id := "app:" + a.ID
		add(hkRow{id: id, item: settings.Hotkey{App: a.ID}, kind: hkApp, group: itemGroup(id, a.Path), name: a.Name, path: a.Path, searchID: id, match: search.NewItem(id, a.Name, alts...)})
	}
	for _, h := range g.settings.Hotkeys() {
		if h.App == "" || known[h.App] {
			continue
		}
		// Not among the apps the search panel found: installed elsewhere,
		// or gone (design.md 4.4: greyed, kept for its hotkey).
		r := hkRow{id: "app:" + h.App, item: h, kind: hkApp, path: sysAppPath(h.App)}
		r.missing = r.path == ""
		if r.missing {
			r.name = g.hotkeyLabel(h)
		} else {
			r.name = strings.TrimSuffix(filepath.Base(r.path), ".app")
		}
		r.searchID, r.group = r.id, itemGroup(r.id, r.path)
		r.match = search.NewItem(r.id, r.name)
		add(r)
	}
	slices.SortStableFunc(items, func(a, b keyed) int { return strings.Compare(a.key, b.key) })
	for _, group := range []string{settings.GroupPresets, settings.GroupSystemApps, settings.GroupOtherApps} {
		for _, it := range items {
			if it.row.group == group {
				rows = append(rows, it.row)
			}
		}
	}
	return rows
}

// syncHotkeyRows makes the rows again when the apps or presets changed
// (the apps found as the page shows, a preset renamed), keeping the query,
// the filter and the choice.
func (p *prefsWindow) syncHotkeyRows() {
	made := p.hotkeyRowsOf()
	if made == p.hk.made && p.hk.rows != nil {
		return
	}
	p.hk.made = made
	p.hk.rows = p.makeHotkeyRows()
	p.filterHotkeys(false)
}

// enterShortcuts starts the page afresh (design.md 4.4): All, the search
// field empty and focused, the first row chosen, at the top.
func (p *prefsWindow) enterShortcuts() {
	p.stopRecording(false)
	p.hk.query, p.hk.filter = "", hkFilterAll
	p.hk.made = ""
	p.syncHotkeyRows()
	p.filterHotkeys(true)
	p.hk.search.Focus()
}

// rowHidden reports whether the search leaves out the row's item.
func (p *prefsWindow) rowHidden(r *hkRow) bool {
	return r.searchID != "" && p.g.settings.Hidden(r.searchID, r.group)
}

// filterHotkeys finds the rows the query and the filter leave, in their
// order, and lays them out; with top, the first line chosen, at the top.
// A row whose hotkey was cleared under With Shortcut, or shown again
// under Hidden, stays until this runs again (design.md 4.4: it does not
// go from under the pointer).
func (p *prefsWindow) filterHotkeys(top bool) {
	h := &p.hk
	q := strings.TrimSpace(h.query)
	h.matched = h.matched[:0]
	h.folded = nil
	anySet := false
	for i := range h.rows {
		r := &h.rows[i]
		accel := p.rowAccel(r)
		if r.own == 0 && accel != "" {
			anySet = true
		}
		switch h.filter {
		case hkFilterSet:
			if r.own == 0 && accel == "" {
				continue
			}
		case hkFilterHidden:
			if !p.rowHidden(r) {
				continue
			}
		}
		if q != "" {
			if rank, _ := r.match.Rank(q); rank < 0 {
				continue
			}
		}
		h.matched = append(h.matched, i)
	}
	h.none = h.filter == hkFilterSet && !anySet && len(h.matched) > 0
	p.layoutHotkeys()
	if top {
		h.list.Choose(0)
		h.list.ScrollTo(0, ui.Start)
	}
}

// collapsed reports whether the list shows the group collapsed: as the
// settings keep it, but all expanded while searching, unless collapsed
// since (design.md 4.4).
func (p *prefsWindow) collapsed(group string) bool {
	if strings.TrimSpace(p.hk.query) != "" {
		return slices.Contains(p.hk.folded, group)
	}
	return p.g.settings.Collapsed(group)
}

// layoutHotkeys lays out the rows matched by group, under their titles,
// a group collapsed by its title alone, a group with none not at all;
// the choice stays on its line.
func (p *prefsWindow) layoutHotkeys() {
	h := &p.hk
	var was any
	if h.chosen >= 0 && h.chosen < len(h.lines) {
		was = p.lineKey(h.chosen)
	}
	h.lines = h.lines[:0]
	for _, group := range settings.Groups {
		at := len(h.lines)
		for _, i := range h.matched {
			if h.rows[i].group != group {
				continue
			}
			if len(h.lines) == at {
				h.lines = append(h.lines, hkLine{group: group, row: -1})
				if p.collapsed(group) {
					break
				}
			}
			h.lines = append(h.lines, hkLine{group: group, row: i})
		}
	}
	if was != nil {
		for i := range h.lines {
			if p.lineKey(i) == was {
				h.list.Choose(i)
				break
			}
		}
	}
}

// groupCounts are how many rows of the group the list has, and how many
// of them the search leaves out (design.md 4.4).
func (p *prefsWindow) groupCounts(group string) (n, hidden int) {
	for _, i := range p.hk.matched {
		r := &p.hk.rows[i]
		if r.group != group {
			continue
		}
		n++
		if p.rowHidden(r) {
			hidden++
		}
	}
	return n, hidden
}

// toggleGroup collapses the group, or expands it, keeping it so in the
// settings; while searching, only until the query changes.
func (p *prefsWindow) toggleGroup(group string) {
	p.stopRecording(false)
	if strings.TrimSpace(p.hk.query) != "" {
		if i := slices.Index(p.hk.folded, group); i >= 0 {
			p.hk.folded = slices.Delete(p.hk.folded, i, i+1)
		} else {
			p.hk.folded = append(p.hk.folded, group)
		}
	} else {
		p.g.settings.SetCollapsed(group, !p.g.settings.Collapsed(group))
		p.g.saveSettings()
	}
	p.layoutHotkeys()
	p.hk.scroll = true
	p.redraw()
}

// setGroupFolded collapses or expands the group, if it is not so.
func (p *prefsWindow) setGroupFolded(group string, fold bool) {
	if p.collapsed(group) != fold {
		p.toggleGroup(group)
	}
}

// groupShown reports whether the search shows the group: not hidden
// whole.
func (p *prefsWindow) groupShown(group string) bool { return !p.g.settings.GroupHidden(group) }

// setRowShown shows the row's item in the search, or hides it, at once.
func (p *prefsWindow) setRowShown(r *hkRow, on bool) {
	if r.searchID == "" {
		return
	}
	p.g.setSearchHidden(r.searchID, r.group, !on)
	p.redraw()
}

// setGroupShown shows the group in the search, or hides it whole, at
// once.
func (p *prefsWindow) setGroupShown(group string, on bool) {
	p.g.setGroupHidden(group, !on)
	p.redraw()
}

// toggleShown turns the switch of the line chosen: its row's, or its
// group's (design.md 4.4: Space, the list focused).
func (p *prefsWindow) toggleShown() {
	h := &p.hk
	if h.chosen < 0 || h.chosen >= len(h.lines) {
		return
	}
	if r := p.hkRowAt(h.chosen); r != nil {
		p.setRowShown(r, p.rowHidden(r))
		return
	}
	group := h.lines[h.chosen].group
	p.setGroupShown(group, !p.groupShown(group))
}

// lineKey is what keeps the choice on its line: the row's ID, the
// group's for its title.
func (p *prefsWindow) lineKey(i int) any {
	if r := p.hkRowAt(i); r != nil {
		return r.id
	}
	if g, ok := p.hkTitleAt(i); ok {
		return "group:" + g
	}
	return "none"
}

// rowAccel is a row's shortcut, "" for none: Gocast's own as registered,
// else as the settings ask (the system refused it).
func (p *prefsWindow) rowAccel(r *hkRow) string {
	if r.own == 0 {
		return p.g.settings.Hotkey(r.item)
	}
	if k := *p.g.shortcutRows()[r.own].key; k != "" {
		return k
	}
	switch r.own {
	case 1:
		return p.g.settings.Launcher()
	case 2:
		return p.g.settings.Terminal()
	}
	return p.g.settings.Clipboard()
}

// ownDefault is the default of one of Gocast's own shortcuts: ⌥⇧Space for
// the search panel while Spotlight has ⌘Space.
func (p *prefsWindow) ownDefault(row int) string {
	switch row {
	case 1:
		if p.g.spotlight {
			return settings.FallbackLauncherShortcut
		}
		return p.g.settings.DefaultLauncher()
	case 2:
		return settings.DefaultTerminalShortcut
	}
	return settings.DefaultClipboardShortcut
}

// typeLabel is the label of a row's kind: App, Preset, System, Window or
// Gocast.
func typeLabel(kind int) string {
	switch kind {
	case hkApp:
		return locales.SearchTagApp()
	case hkPreset:
		return locales.SearchTagPreset()
	case hkSystem:
		return locales.SearchTagSystem()
	case hkWindow:
		return locales.SearchTagWindow()
	}
	return locales.SearchTagGocast()
}

// groupName is the name of a group of settings.Groups (copy.md 19.2).
func groupName(group string) string {
	switch group {
	case settings.GroupSystem:
		return locales.GroupSystem()
	case settings.GroupPresets:
		return locales.GroupPresets()
	case settings.GroupSystemApps:
		return locales.GroupSystemApps()
	case settings.GroupOtherApps:
		return locales.GroupOtherApps()
	}
	return locales.GroupGocast()
}

// hkRowAt is the row shown at i, nil for a group's title or the line
// under the rows.
func (p *prefsWindow) hkRowAt(i int) *hkRow {
	if i < 0 || i >= len(p.hk.lines) || p.hk.lines[i].row < 0 {
		return nil
	}
	return &p.hk.rows[p.hk.lines[i].row]
}

// hkTitleAt is the group whose title shows at i, ok false for a row.
func (p *prefsWindow) hkTitleAt(i int) (group string, ok bool) {
	if i < 0 || i >= len(p.hk.lines) || p.hk.lines[i].row >= 0 {
		return "", false
	}
	return p.hk.lines[i].group, true
}

// hkRowByID is the row with the ID, nil if none.
func (p *prefsWindow) hkRowByID(id string) *hkRow {
	for i := range p.hk.rows {
		if p.hk.rows[i].id == id {
			return &p.hk.rows[i]
		}
	}
	return nil
}

// --- Recording ---

// startRecording records the shortcut of the row shown at i, choosing it,
// every global shortcut off meanwhile, so that pressing one reaches the
// recorder (design.md 4.4). One row records at a time.
func (p *prefsWindow) startRecording(i int) {
	r := p.hkRowAt(i)
	if r == nil || r.noKeys || p.hk.rec.id == r.id {
		return
	}
	p.stopRecording(false)
	p.g.suspendShortcuts()
	p.hk.rec = hkRecording{id: r.id}
	if r.own > 0 {
		p.problem[r.own] = ""
	}
	p.hk.list.Choose(i)
	p.hk.scroll = true
	p.hk.recBox.Focus()
	p.redraw()
}

// stopRecording ends the recording, the shortcuts on again; with back,
// the keyboard goes back to the search field.
func (p *prefsWindow) stopRecording(back bool) {
	if p.hk.rec.id == "" {
		return
	}
	p.hk.rec = hkRecording{}
	p.hk.lost = false
	if p.g.suspended { // not resumed by the change that ended it
		p.g.resumeShortcuts()
	}
	if back {
		p.hk.search.Focus()
	}
	p.redraw()
}

// recordKey takes every key while a row records: Escape alone cancels;
// Return moves the shortcut another has, when it may; a modifier shows as
// it is held; a shortcut is saved at once, or refused with why, the row
// recording on (design.md 4.4).
func (p *prefsWindow) recordKey(ev ui.InputEvent) bool {
	rec := &p.hk.rec
	r := p.hkRowByID(rec.id)
	if r == nil {
		return false
	}
	switch ev.Kind {
	case ui.InputKeyDown, ui.InputKeyUp:
	case ui.InputText, ui.InputCompose, ui.InputCommand:
		return true // nothing typed
	default:
		return false
	}
	if ev.Kind == ui.InputKeyDown && ev.Mods == 0 {
		switch {
		case ev.Key == ui.KeyEscape:
			p.stopRecording(true)
			return true
		case ev.Key == ui.KeyEnter && rec.movable:
			p.useHere()
			return true
		}
	}
	pressed := shortcut.Pressed(ev)
	if pressed == "" || ev.Kind == ui.InputKeyUp { // a modifier, or a key let go
		rec.mods = ev.Mods
		if ev.Kind == ui.InputKeyDown && pressed == "" && ev.Mods != 0 {
			rec.problem, rec.pressed, rec.movable = "", "", false // pressing again: recording afresh
		}
		p.redraw()
		return true
	}
	if ev.Repeat {
		return true
	}
	accel, ok := shortcut.FromEvent(ev)
	var err error
	switch {
	case !ok:
		err = errHotkeyInvalid
	case r.own > 0:
		err = p.setOwnShortcut(r.own, accel)
	default:
		err = p.g.setHotkey(r.item, accel)
	}
	if err == nil {
		p.stopRecording(true) // the change resumed the shortcuts
		return true
	}
	p.g.suspendShortcuts() // resumed, refused by the system: recording on
	rec.pressed, rec.mods = pressed, ev.Mods
	rec.problem, rec.movable, rec.from = p.recorderProblem(err)
	rec.say = r.name + listSep() + rec.problem
	p.redraw()
	return true
}

// setOwnShortcut makes accel the shortcut of one of Gocast's own rows, as
// setHotkey does an app's: refused when it is not one the recorder takes,
// another has it, Spotlight has it (⌘Space) or the system refuses it.
// Call it with the shortcuts suspended: they resume, unless it is refused
// before it is tried.
func (p *prefsWindow) setOwnShortcut(row int, accel string) error {
	n, ok := shortcut.Normalize(accel)
	if !ok || !shortcut.Valid(n) {
		return errHotkeyInvalid
	}
	own := p.g.ownShortcuts()
	own[row] = nil // its own: no conflict
	if o, ok := shortcutOwner(n, own, p.g.settings, settings.Hotkey{}); ok {
		return &hotkeyConflictError{o}
	}
	if row == 1 && shortcut.Same(n, settings.DefaultLauncherShortcut) && shortcut.SpotlightHasCmdSpace() {
		return errSpotlightBusy
	}
	return p.g.setShortcut(row, accel)
}

// recorderProblem is what a row says of a shortcut refused (copy.md 11.2,
// 11.3), and whether Use Here may move it from the other, from.
func (p *prefsWindow) recorderProblem(err error) (problem string, movable bool, from settings.Hotkey) {
	var ce *hotkeyConflictError
	switch {
	case errors.As(err, &ce) && ce.owner.row > 0:
		return locales.HotkeysUsedByGocast(shortcutLabel(ce.owner.row)), false, from
	case errors.As(err, &ce):
		return locales.ShortcutUsedFor(p.g.hotkeyLabel(ce.owner.item)), true, ce.owner.item
	case errors.Is(err, errShortcutTaken):
		return locales.HotkeyInUse(), false, from
	case errors.Is(err, errSpotlightBusy):
		return locales.ShortcutSpotlightBusy(), false, from
	}
	return locales.HotkeyNeedsModifier(), false, from
}

// useHere moves the shortcut another app or preset has to the row
// recording, which ends (copy.md 11.3: no confirmation, no notice); if
// the system refuses it, the row says so and records on.
func (p *prefsWindow) useHere() {
	rec := &p.hk.rec
	r := p.hkRowByID(rec.id)
	if r == nil || !rec.movable {
		return
	}
	var err error
	if r.own > 0 {
		err = p.g.takeHotkey(r.own, rec.from)
	} else {
		err = p.g.moveHotkey(rec.from, r.item)
	}
	if err != nil {
		p.g.suspendShortcuts()
		rec.problem, rec.movable = locales.HotkeyInUse(), false
		rec.say = r.name + listSep() + rec.problem
		p.redraw()
		return
	}
	p.stopRecording(true)
}

// clearRow clears the shortcut of a row: an app's or preset's goes, one
// of Gocast's own goes back to its default (design.md 4.4).
func (p *prefsWindow) clearRow(r *hkRow) {
	p.stopRecording(false)
	if r.noKeys {
		return
	}
	if r.own > 0 {
		p.resetShortcut(r.own)
		return
	}
	if p.g.settings.Hotkey(r.item) != "" {
		p.g.removeHotkey(r.item)
	}
	p.redraw()
}

// resetShortcut gives one of Gocast's own shortcuts its default again,
// unless another has it or the system refuses it: the row says why then.
func (p *prefsWindow) resetShortcut(row int) {
	def := p.ownDefault(row)
	if shortcut.Same(def, p.rowAccel(&hkRow{own: row})) {
		return
	}
	p.g.suspendShortcuts()
	err := p.setOwnShortcut(row, def)
	if p.g.suspended {
		p.g.resumeShortcuts()
	}
	p.problem[row] = ""
	if err != nil {
		p.problem[row], _, _ = p.recorderProblem(err)
	}
	p.redraw()
}

// --- Drawing ---

func (p *prefsWindow) shortcutsPage(c *ui.Context, k tokens) {
	p.syncHotkeyRows()
	h := &p.hk
	if !p.isRecording() {
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			h.search.Focus()
		}
		if c.Shortcut(ui.Cmd, ui.KeyBackspace) {
			if r := p.hkRowAt(h.chosen); r != nil {
				p.clearRow(r)
			}
		}
	}
	p.spotlightNotice(c, k)
	ui.Text(c, locales.HotkeysIntro()).FontSize(12).TextColor(k.textSecondary).MaxLines(2).Margin(0, 0, 14, 0).Shrink(0)
	ui.Row(c).Height(28).Shrink(0).Gap(10).AlignItems(ui.Center).Margin(0, 0, 10, 0).Children(func() {
		p.hotkeySearch(c, k)
		ui.Themed(c, segmentTheme(c)).Background(ui.Transparent).Shrink(0).Children(func() {
			seg := ui.Segmented(c, &h.filter, locales.HotkeysFilterAll(), locales.HotkeysFilterSet(),
				locales.HotkeysFilterHidden()).Height(28).FontSize(12).Shrink(0).Label(locales.HotkeysFilter())
			if seg.Changed() {
				p.stopRecording(false)
				p.filterHotkeys(true)
			}
		})
	})
	card(c, k).Grow(1).Shrink(1).Padding(hkListPad).Children(func() {
		if len(h.lines) == 0 {
			p.hotkeysEmpty(c, k)
			return
		}
		p.hotkeyList(c, k)
	})
	if h.rec.say != "" {
		c.Announce(h.rec.say)
		h.rec.say = ""
	}
}

// hotkeySearch is the search field over the list (design.md 4.4): 28
// high, 7 round, a magnifying glass at its left, a button clearing it
// while it holds any.
func (p *prefsWindow) hotkeySearch(c *ui.Context, k tokens) {
	h := &p.hk
	ui.Row(c).Grow(1).Shrink(1).Height(28).Radius(7).Background(k.cardFill).Border(1, k.hairline).Padding(0, 6, 0, 8).Gap(6).AlignItems(ui.Center).Children(func() {
		sfsymbols.Symbol(c, "magnifyingglass").Size(12).Color(k.textSecondary)
		field := ui.TextInputBase(c, &h.query).Bind(&h.search).Grow(1).Shrink(1).FontSize(13).
			Placeholder(locales.HotkeysSearch()).Label(locales.HotkeysSearch()).OnTextKey(p.searchKey)
		h.composing = field.Composing()
		if field.Changed() {
			p.stopRecording(false)
			p.filterHotkeys(true)
		}
		if h.query != "" {
			// Out of assistive technology's way, as Escape empties the
			// field too (no copy for it).
			b := ui.Box(c).Size(16, 16).Shrink(0).Center().AccessHidden(true).OnClick(func() {
				h.query = ""
				p.filterHotkeys(true)
				h.search.Focus()
			})
			b.Children(func() { sfsymbols.Symbol(c, "xmark.circle.fill").Size(12).Color(k.textSecondary) })
		}
	})
}

// searchKey takes the keys of the search field that move the choice and
// act on it (design.md 4.4): Up and Down choose, Return records the row
// chosen, Cmd-Backspace clears its shortcut, Escape empties the field;
// none while the input method composes.
func (p *prefsWindow) searchKey(key ui.TextKey) bool {
	h := &p.hk
	if h.composing {
		return false
	}
	switch {
	case key.Key == ui.KeyUp && key.Mods == 0:
		p.moveChoice(-1)
	case key.Key == ui.KeyDown && key.Mods == 0:
		p.moveChoice(1)
	case key.Key == ui.KeyEnter && key.Mods == 0:
		p.submitLine(h.chosen)
	case (key.Key == ui.KeyLeft || key.Key == ui.KeyRight) && key.Mods == 0 && key.Text() == "":
		// With nothing typed, no caret to move: the list's.
		p.foldKey(key.Key == ui.KeyLeft)
	case key.Key == ui.KeyBackspace && key.Mods == ui.Cmd:
		if r := p.hkRowAt(h.chosen); r != nil {
			p.clearRow(r)
		}
	case key.Key == ui.KeyEscape && key.Mods == 0:
		if key.Text() == "" {
			return true // already empty: nothing, the window stays
		}
		key.SetText("")
		h.query = ""
		p.filterHotkeys(true)
	default:
		return false
	}
	p.redraw()
	return true
}

// submitLine is Return, or a double click, on the line at i: a group's
// title collapses or expands; a row records, if it may (design.md 4.4).
func (p *prefsWindow) submitLine(i int) {
	if g, ok := p.hkTitleAt(i); ok {
		p.toggleGroup(g)
		return
	}
	p.startRecording(i)
}

// foldKey is Left (fold) or Right on the line chosen (design.md 4.4): a
// group's title collapses or expands; Left on a row goes to its group's
// title.
func (p *prefsWindow) foldKey(fold bool) {
	h := &p.hk
	if g, ok := p.hkTitleAt(h.chosen); ok {
		p.setGroupFolded(g, fold)
		return
	}
	if !fold || h.chosen < 0 || h.chosen >= len(h.lines) {
		return
	}
	p.stopRecording(false)
	for i := h.chosen; i >= 0; i-- {
		if h.lines[i].row < 0 {
			h.list.Choose(i)
			h.scroll = true
			break
		}
	}
	p.redraw()
}

func (p *prefsWindow) moveChoice(d int) {
	h := &p.hk
	if len(h.lines) == 0 {
		return
	}
	i := min(max(h.chosen+d, 0), len(h.lines)-1)
	h.list.Choose(i)
	h.scroll = true
	p.redraw()
}

// hotkeysEmpty says why the list is empty, in its middle.
func (p *prefsWindow) hotkeysEmpty(c *ui.Context, k tokens) {
	text := locales.HotkeysNoMatch()
	if p.hk.filter == hkFilterHidden && strings.TrimSpace(p.hk.query) == "" {
		text = locales.HiddenEmpty() // copy.md 19.4
	}
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Padding(0, 20).Children(func() {
		ui.Text(c, text).FontSize(13).TextColor(k.textSecondary).TextAlign(ui.Center).MaxWidth(hkNoteWidth).Role(ui.RoleStatus)
	})
}

func (p *prefsWindow) hotkeyList(c *ui.Context, k tokens) {
	h := &p.hk
	h.list.Selected = &h.chosen
	h.list.Key = p.lineKey
	h.list.Label = p.hotkeyRowLabel
	n := len(h.lines)
	if h.none {
		n++
	}
	var list ui.Element
	h.accent = c.Theme().Accent // the list's theme has none
	h.theme = *c.Theme()
	// The rows draw their own choice, 6 round (design.md 4.4).
	ui.Themed(c, searchListTheme(c, k)).Background(ui.Transparent).Grow(1).Children(func() {
		list = ui.List(c, &h.list, n).Grow(1).Rows(func(r ui.ListRow) { p.hotkeyRow(c, k, r) })
	})
	if h.chosen >= len(h.lines) && len(h.lines) > 0 { // the line under the rows is not chosen
		h.list.Choose(len(h.lines) - 1)
	}
	// The list's keys (design.md 4.4): Space turns the switch of the line
	// chosen; Left and Right collapse and expand.
	if !p.isRecording() {
		switch {
		case h.list.Shortcut(c, 0, ui.KeySpace):
			p.toggleShown()
		case h.list.Shortcut(c, 0, ui.KeyLeft):
			p.foldKey(true)
		case h.list.Shortcut(c, 0, ui.KeyRight):
			p.foldKey(false)
		}
	}
	if list.Changed() {
		if r := p.hkRowAt(h.chosen); r == nil || r.id != h.rec.id {
			p.stopRecording(false)
		}
	}
	if list.Submitted() {
		p.submitLine(h.chosen)
	}
	if h.scroll && h.chosen >= 0 {
		h.scroll = false
		h.list.ScrollIntoView(h.chosen)
	}
	if h.lost {
		h.lost = false
		p.stopRecording(false)
	}
}

// groupTitle draws a group's title (design.md 4.4): a line over it but
// over the first; the arrow, the name, how many rows, how many of them
// hidden; the group's switch. A click but on the switch collapses or
// expands it.
func (p *prefsWindow) groupTitle(c *ui.Context, k tokens, lr ui.ListRow, group string) {
	n, hidden := p.groupCounts(group)
	ui.Column(c).Shrink(0).Children(func() {
		if lr.Index > 0 {
			ui.Box(c).Height(1).Shrink(0).Margin(0, 2).Background(k.hairline).AccessHidden(true)
		}
		row := ui.Row(c).Height(hkTitleHeight).Shrink(0).Padding(0, hkRowPad).Radius(6).AlignItems(ui.Center).
			OnClick(func() { p.toggleGroup(group) })
		if lr.Selected() {
			row.Background(k.selection)
		}
		row.Children(func() {
			arrow := "chevron.down"
			if p.collapsed(group) {
				arrow = "chevron.right"
			}
			ui.Box(c).Size(16, 16).Shrink(0).Center().AccessHidden(true).Children(func() {
				sfsymbols.Symbol(c, arrow).Size(10).Weight(sfsymbols.Semibold).Color(k.textSecondary)
			})
			ui.Text(c, groupName(group)).FontSize(13).FontWeight(600).TextColor(k.text).SingleLine().Shrink(1).Margin(0, 6, 0, 4).AccessHidden(true)
			count := strconv.Itoa(n)
			if hidden > 0 && p.hk.filter != hkFilterHidden {
				count += " · " + locales.GroupHiddenCount(hidden)
			}
			ui.Text(c, count).FontSize(12).TextColor(k.textSecondary).FontFeatures("tnum").SingleLine().Shrink(0).AccessHidden(true)
			ui.Box(c).Grow(1)
			on := p.groupShown(group)
			if p.smallSwitch(c, &on, locales.GroupShowTip(), locales.GroupSwitchSpoken(groupName(group))) {
				p.setGroupShown(group, on)
			}
		})
	})
}

// smallSwitch is the small switch of the list, 32 by 18 (design.md
// 4.4), rather than the regular one of the other pages, in the window's
// accent; it reports whether it was turned.
func (p *prefsWindow) smallSwitch(c *ui.Context, on *bool, tip, label string) bool {
	changed := false
	ui.Themed(c, &p.hk.theme).Background(ui.Transparent).Shrink(0).Children(func() {
		sw := ui.Switch(c, on).Size(hkSwitchW, hkSwitchH).Radius(hkSwitchH / 2).Shrink(0).Tooltip(tip).Label(label)
		changed = sw.Changed()
	})
	return changed
}

// hotkeyRow draws a line: a group's title, or a row: icon, name,
// recorder, switch; under it, while it records or something is wrong, a
// line saying so (design.md 4.4).
func (p *prefsWindow) hotkeyRow(c *ui.Context, k tokens, lr ui.ListRow) {
	if g, ok := p.hkTitleAt(lr.Index); ok {
		p.groupTitle(c, k, lr, g)
		return
	}
	r := p.hkRowAt(lr.Index)
	if r == nil {
		ui.Box(c).Height(hkNoteHeight).Shrink(0).Center().Padding(0, 20).AccessHidden(true).Children(func() {
			ui.Text(c, locales.HotkeysNoneSet()).FontSize(13).TextColor(k.textSecondary).TextAlign(ui.Center).MaxWidth(hkNoteWidth)
		})
		return
	}
	accel := p.rowAccel(r)
	recording := p.hk.rec.id == r.id
	chosen := lr.Selected()
	hot := lr.Hovered() || chosen
	desc, descColor, movable := p.rowDesc(r, accel, recording, k)
	target := float32(hkRowHeight)
	if desc != "" {
		target = p.descHeight(c, desc, movable)
	}
	box := ui.Column(c).Shrink(0).Padding(0, hkRowPad).Radius(6).Clip()
	box.Height(box.AnimateWith("height", target, hkExpandFor, ui.EaseOut))
	if chosen {
		box.Background(k.selection)
	}
	if r.missing {
		box.Tooltip(locales.HotkeysAppMissingTip())
	}
	box.Children(func() {
		ui.Row(c).Height(hkRowHeight).Shrink(0).AlignItems(ui.Center).Children(func() {
			p.rowIcon(c, k, r)
			ui.Row(c).Grow(1).Shrink(1).Margin(0, hkRecGap, 0, 10).Gap(8).AlignItems(ui.Center).Children(func() {
				name := ui.Text(c, r.name).FontSize(13).TextColor(k.text).SingleLine().Shrink(1).AccessHidden(true)
				switch {
				case r.missing:
					name.TextColor(k.textSecondary)
					ui.Text(c, locales.HotkeysAppMissing()).FontSize(12).TextColor(k.textSecondary).SingleLine().Shrink(0).AccessHidden(true)
				case r.own == 1 && p.g.spotlight:
					ui.Text(c, locales.ShortcutSpotlightBusy()).FontSize(12).TextColor(k.textSecondary).SingleLine().Shrink(0).AccessHidden(true)
				}
			})
			p.recorder(c, k, lr.Index, r, accel, recording, hot, desc != "" && descColor == k.danger)
			// The switch, or its room (design.md 4.4: the search panel's
			// own shortcut has none).
			cell := ui.Box(c).Size(hkSwitchW, hkSwitchH).Shrink(0).Margin(0, 0, 0, hkSwitchGap)
			if r.searchID != "" {
				cell.Children(func() {
					on := !p.rowHidden(r)
					if p.smallSwitch(c, &on, locales.RowShowTip(), locales.RowSwitchSpoken(r.name)) {
						p.setRowShown(r, on)
					}
				})
			}
		})
		if desc == "" {
			return
		}
		ui.Row(c).Shrink(0).Padding(0, 0, 0, hkDescLeft).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, desc).FontSize(12).TextColor(descColor).MaxLines(2).Shrink(1).AccessHidden(true)
			if movable {
				b := ui.Box(c).Height(20).Padding(0, 8).Radius(5).Background(k.keycap).Center().Shrink(0).
					Role(ui.RoleButton).Label(locales.HotkeysUseHere()).OnClick(p.useHere)
				b.Children(func() {
					ui.Text(c, locales.HotkeysUseHere()).FontSize(12).TextColor(k.text).SingleLine()
				})
			}
		})
	})
}

// rowDesc is the line under a row: while it records, the hint or what is
// wrong with the shortcut pressed; else what stands wrong with its own,
// as one the system refused; "" for none.
func (p *prefsWindow) rowDesc(r *hkRow, accel string, recording bool, k tokens) (string, ui.Color, bool) {
	if recording {
		if rec := p.hk.rec; rec.problem != "" {
			return rec.problem, k.danger, rec.movable
		}
		return locales.ShortcutRecordingHint(), k.textSecondary, false
	}
	if r.own > 0 {
		if s := p.problem[r.own]; s != "" {
			return s, k.danger, false
		}
		return "", k.textSecondary, false
	}
	if st, ok := p.g.hotkeyStatus(r.item); ok && st == hotkeyTaken && accel != "" {
		return locales.HotkeyInUse(), k.danger, false
	}
	return "", k.textSecondary, false
}

// descHeight is a row's height with the line under it: 56 for a line of
// text, 72 for two (design.md 4.4).
func (p *prefsWindow) descHeight(c *ui.Context, desc string, movable bool) float32 {
	width := float32(hkListWidth - hkRowPad - hkDescLeft - hkRowPad)
	if movable {
		w, _ := c.MeasureText(0, ui.Span{Text: locales.HotkeysUseHere(), Size: 12})
		width -= w + 16 + 8
	}
	_, one := c.MeasureText(0, ui.Span{Text: "Ag", Size: 12})
	if _, h := c.MeasureText(width, ui.Span{Text: desc, Size: 12}); h > one*1.5 {
		return 72
	}
	return 56
}

// rowIcon is a row's icon, 20: the app's, else the system's generic
// application icon, faded for an app missing; a symbol on a square for
// a preset and Gocast's own; an action's, as the search panel's, scaled
// to 20 (design.md 4.4).
func (p *prefsWindow) rowIcon(c *ui.Context, k tokens, r *hkRow) {
	square := func(symbol string, size float32) {
		ui.Box(c).Size(hkIcon, hkIcon).Radius(5).Background(k.keycap).Center().Shrink(0).AccessHidden(true).
			Children(func() { sfsymbols.Symbol(c, symbol).Size(size).Color(k.text) })
	}
	switch {
	case r.own > 0:
		square([4]string{"", "magnifyingglass", "apple.terminal", "doc.on.clipboard"}[r.own], 11)
		return
	case r.symbol != "":
		square(r.symbol, 11)
		return
	case r.kind == hkPreset:
		square("apple.terminal.fill", 11)
		return
	case r.kind == hkSystem || r.kind == hkWindow:
		actionSquare(c, k, r.item.Action, hkIcon, 5, 11, k.textSecondary)
		return
	}
	b := p.g.launcher.clip.genericAppIcon(hkIcon)
	if r.path != "" {
		if icon := p.g.launcher.clip.fileIcon(r.path, hkIcon); icon != nil {
			b = icon
		}
	}
	if b == nil {
		ui.Avatar(c, r.name, nil).Size(hkIcon, hkIcon).Shrink(0).AccessHidden(true).Opacity(missingOpacity(r))
		return
	}
	ui.Image(c, b).Size(hkIcon, hkIcon).Shrink(0).AccessHidden(true).Opacity(missingOpacity(r))
}

// missingOpacity is the opacity of a row's icon: 40% for an app missing.
func missingOpacity(r *hkRow) float32 {
	if r.missing {
		return 0.4
	}
	return 1
}

// recorder is a row's recorder, 150 by 24 (design.md 4.4): the keys of
// its shortcut, "Set Shortcut" without one (on a box only as the row is
// pointed at or chosen), the keys held while it records; at its right,
// clear (an app's or preset's) or Reset to Default (Gocast's own).
func (p *prefsWindow) recorder(c *ui.Context, k tokens, i int, r *hkRow, accel string, recording, hot, wrong bool) {
	if r.noKeys { // its room alone (design.md 4.4: Quit Gocast)
		ui.Box(c).Size(hkRecWidth, hkRecHeight).Shrink(0)
		return
	}
	rec := p.hk.rec
	b := ui.Row(c).Size(hkRecWidth, hkRecHeight).Shrink(0).Radius(6).Padding(0, 4, 0, 6).Gap(4).AlignItems(ui.Center)
	switch {
	case recording && rec.problem != "", wrong:
		b.Background(k.surface).Border(1, k.danger)
	case recording:
		b.Background(k.surface).Border(2, p.hk.accent)
	case accel != "" || hot:
		b.Background(k.surface).Border(1, k.hairline)
	}
	if recording {
		b.Focusable().FocusRing(false).Bind(&p.hk.recBox).HandleInput(p.recordKey)
		// The keys the list takes as its shortcuts, before the recorder's
		// input: the recorder's while it records (design.md 4.4: only
		// Escape cancels; Return is Use Here, or a key alone).
		for _, key := range []ui.Key{ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd, ui.KeyEnter} {
			if b.Shortcut(0, key) {
				p.recordKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: key})
			}
		}
		if b.FocusLeft() {
			p.hk.lost = true
		}
	}
	b.OnClick(func() { p.startRecording(i) })
	b.Children(func() {
		ui.Row(c).Grow(1).Shrink(1).Gap(3).AlignItems(ui.Center).Clip().Children(func() {
			capsColor := k.text
			if r.missing {
				capsColor = k.textSecondary
			}
			switch {
			case recording && rec.problem != "":
				recorderCaps(c, k, shortcut.Caps(rec.pressed), k.text)
			case recording && rec.mods != 0:
				recorderCaps(c, k, shortcut.ModifierCaps(rec.mods), k.text)
			case recording:
				ui.Text(c, locales.ShortcutRecording()).FontSize(12).TextColor(k.textSecondary).SingleLine().AccessHidden(true)
			case accel != "":
				recorderCaps(c, k, shortcut.Caps(accel), capsColor)
			default:
				ui.Text(c, locales.HotkeysSetShortcut()).FontSize(12).TextColor(k.textSecondary).SingleLine().AccessHidden(true)
			}
		})
		switch {
		case recording:
		case r.own > 0 && !shortcut.Same(accel, p.ownDefault(r.own)):
			p.recorderButton(c, k, "arrow.counterclockwise", 11, locales.ShortcutReset(), true, func() { p.clearRow(r) })
		case r.own == 0 && accel != "" && hot:
			p.recorderButton(c, k, "xmark.circle.fill", 12, locales.HotkeysRemoveFor(r.name), false, func() { p.clearRow(r) })
		}
	})
}

// recorderButton is the 16 by 16 button at a recorder's right.
func (p *prefsWindow) recorderButton(c *ui.Context, k tokens, symbol string, size float32, label string, tip bool, act func()) {
	b := ui.Box(c).Size(16, 16).Shrink(0).Center().Role(ui.RoleButton).Label(label).OnClick(act)
	if tip {
		b.Tooltip(label)
	}
	color := k.textSecondary
	if b.Pressed() {
		color = k.text
	}
	b.Children(func() { sfsymbols.Symbol(c, symbol).Size(size).Color(color) })
}

// recorderCaps are the keys of a shortcut in a recorder (design.md 2.4,
// 4.4): 18 high, 12, 3 apart.
func recorderCaps(c *ui.Context, k tokens, caps []string, color ui.Color) {
	ui.Row(c).Gap(3).AlignItems(ui.Center).Shrink(0).AccessHidden(true).Children(func() {
		for _, cp := range caps {
			ui.Box(c).Height(18).MinWidth(18).Padding(0, 4).Radius(4).Background(k.keycap).Center().Shrink(0).Children(func() {
				capLabel(c, cp, 12, color)
			})
		}
	})
}

// hotkeyRowLabel is what assistive technology reads of a row (copy.md
// 11.4): "Terminal, App, shortcut Option Command T"; "…, no shortcut";
// a missing app's; the row recording, or what is wrong with the shortcut
// pressed.
func (p *prefsWindow) hotkeyRowLabel(i int) string {
	sep := listSep()
	if g, ok := p.hkTitleAt(i); ok { // "System Apps, 38 items, 37 hidden, expanded" (copy.md 19.5)
		n, hidden := p.groupCounts(g)
		s := locales.GroupSpoken(groupName(g), n)
		if hidden > 0 && p.hk.filter != hkFilterHidden {
			s += sep + locales.GroupHiddenCount(hidden)
		}
		if p.collapsed(g) {
			return s + sep + locales.GroupCollapsed()
		}
		return s + sep + locales.GroupExpanded()
	}
	r := p.hkRowAt(i)
	if r == nil {
		return locales.HotkeysNoneSet()
	}
	if rec := p.hk.rec; rec.id == r.id {
		switch {
		case rec.movable:
			return r.name + sep + locales.HotkeysUsedForSpoken(p.g.hotkeyLabel(rec.from))
		case rec.problem != "":
			return r.name + sep + rec.problem
		}
		return r.name + sep + locales.HotkeysRecordingSpoken()
	}
	accel := p.rowAccel(r)
	s := r.name + sep + typeLabel(r.kind)
	if r.missing {
		s += sep + locales.HotkeysAppMissingSpoken()
	}
	switch {
	case r.noKeys:
	case accel == "":
		s += sep + locales.HotkeysNoShortcut()
	default:
		s += hotkeySpoken(accel)
	}
	if p.rowHidden(r) { // copy.md 19.5
		s += sep + locales.RowHiddenSpoken()
	}
	return s
}
