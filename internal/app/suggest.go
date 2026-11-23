package app

import (
	"slices"
	"strings"
	"time"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// The search panel's list under an empty query (design.md 2.5a): the
// suggestions, then Gocast's own items.

// The IDs of Gocast's own items, which usage.json scores as it does apps
// and presets, and the search finds as it does them.
const (
	clipItemID = "gocast:clipboard"
	termItemID = "gocast:terminal"
	// The notes' items (design.md 11.1): the notes window, a new note,
	// Browse Notes. The Gocast group has the first alone.
	notesItemID     = "gocast:notes"
	newNoteItemID   = "gocast:new-note"
	searchNotesID   = "gocast:search-notes"
	addPresetItemID = "gocast:add-preset"
	settingsItemID  = "gocast:settings"
	quitItemID      = "gocast:quit"
	// updateItemID is 重新打开以更新到 {版本}, which the Gocast group has
	// first while a new version is ready (design.md 4.5); never scored.
	updateItemID = "gocast:update"
)

// ownItemPrefix begins the IDs of Gocast's own items.
const ownItemPrefix = "gocast:"

// ownItem is one of Gocast's own items: a row of the Gocast group, which
// the suggestions may show too.
type ownItem struct {
	id, symbol, label string
	caps              []string // its shortcut, at the right of its row in the Gocast group
	run               func()
	// searched: found by the search alone, not a row of the Gocast group
	// (New Note, Search Notes: design.md 11.1).
	searched bool
}

// gocastDo is what Gocast's own items do; update is the version ready,
// which relaunch relaunches into, or "".
type gocastDo struct {
	clipboard, terminal, addPreset, settings, quit func()
	update                                         string
	relaunch                                       func()
	// The notes' items and their hotkeys (design.md 11.1).
	notes, newNote, searchNotes func()
	notesKey, newNoteKey        string
	searchNotesKey              string
	// The hotkeys of Add Preset and Settings… (design.md 4.4), "" for
	// none: Settings… then shows ⌘,.
	addPresetKey, settingsKey string
}

// gocastItems are the rows of the Gocast group, in copy.md 2.2a's order:
// the clipboard history, Notes and the floating terminal with their
// shortcuts, Add Preset, Settings… with ⌘, and Quit Gocast, without ⌘Q;
// first, while a new version is ready, Relaunch to Update to it, without
// keys (design.md 4.5); then New Note and Search Notes, which the search
// finds but the group has not (design.md 11.1).
func gocastItems(terminalKey, clipKey string, do gocastDo) []ownItem {
	var own []ownItem
	for _, fn := range []*func(){&do.clipboard, &do.terminal, &do.addPreset, &do.settings, &do.quit, &do.relaunch, &do.notes, &do.newNote, &do.searchNotes} {
		if *fn == nil {
			*fn = func() {} // a caller with no notes window (the tests')
		}
	}
	if do.update != "" {
		own = append(own, ownItem{id: updateItemID, symbol: "arrow.down.circle", label: locales.UpdateRelaunchTo(do.update), run: do.relaunch})
	}
	settingsCaps := []string{"⌘", ","}
	if do.settingsKey != "" {
		settingsCaps = shortcut.Caps(do.settingsKey)
	}
	return append(own, []ownItem{
		{id: clipItemID, symbol: "doc.on.clipboard", label: locales.ClipboardHistory(), caps: shortcut.Caps(clipKey), run: do.clipboard},
		{id: notesItemID, symbol: ownSymbolOf(notesItemID), label: locales.NotesItem(), caps: shortcut.Caps(do.notesKey), run: do.notes},
		{id: termItemID, symbol: "apple.terminal", label: locales.MenuTerminal(), caps: shortcut.Caps(terminalKey), run: do.terminal},
		{id: addPresetItemID, symbol: "plus", label: locales.PresetsAdd(), caps: shortcut.Caps(do.addPresetKey), run: do.addPreset},
		{id: settingsItemID, symbol: "gearshape", label: locales.TraySettings(), caps: settingsCaps, run: do.settings},
		{id: quitItemID, symbol: "power", label: locales.TrayQuit(), run: do.quit},
		{id: newNoteItemID, symbol: ownSymbolOf(newNoteItemID), label: locales.NotesNewItem(), caps: shortcut.Caps(do.newNoteKey), run: do.newNote, searched: true},
		{id: searchNotesID, symbol: ownSymbolOf(searchNotesID), label: locales.NotesSearchItem(), caps: shortcut.Caps(do.searchNotesKey), run: do.searchNotes, searched: true},
	}...)
}

// noteOwnID is the own item of the notes' hotkey item: noteItem's
// reverse.
func noteOwnID(item string) string {
	switch item {
	case settings.NoteNew:
		return newNoteItemID
	case settings.NoteSearch:
		return searchNotesID
	}
	return notesItemID
}

// ownSymbolOf is the symbol of the notes' own item id (design.md 11.1).
func ownSymbolOf(id string) string {
	switch id {
	case newNoteItemID:
		return "square.and.pencil"
	case searchNotesID:
		return "doc.text.magnifyingglass"
	}
	return "note.text"
}

// noteItemName is the name of the notes' hotkey item (settings.NoteOpen,
// ...), as the search panel's row has it (copy.md 16.8).
func noteItemName(item string) string {
	switch item {
	case settings.NoteNew:
		return locales.NotesNewItem()
	case settings.NoteSearch:
		return locales.NotesSearchItem()
	}
	return locales.NotesItem()
}

// noteItem is the hotkey's item of a notes' own item (design.md 11.1),
// "" for another.
func noteItem(id string) string {
	switch id {
	case notesItemID:
		return settings.NoteOpen
	case newNoteItemID:
		return settings.NoteNew
	case searchNotesID:
		return settings.NoteSearch
	}
	return ""
}

// ownTitle is the title of Gocast's own item in the language of m.
func ownTitle(m locales.Messages, id string) string {
	switch id {
	case clipItemID:
		return m.ClipboardHistory()
	case termItemID:
		return m.MenuTerminal()
	case addPresetItemID:
		return m.PresetsAdd()
	case settingsItemID:
		return m.TraySettings()
	case quitItemID:
		return m.TrayQuit()
	case notesItemID:
		return m.NotesItem()
	case newNoteItemID:
		return m.NotesNewItem()
	case searchNotesID:
		return m.NotesSearchItem()
	}
	return ""
}

// ownSearchNames are the names the search finds Gocast's own item by,
// besides its title: its titles in English and Chinese, as people type
// either whatever the language Gocast shows (pinyin and initials come
// with the Chinese); for the clipboard history, also the words users say
// for it (copy.md 6.1); for Relaunch to Update to the version ready
// (update), the words of it people type (design.md 4.5).
func ownSearchNames(id, update string) []string {
	if id == updateItemID {
		return append([]string{locales.In("en").UpdateRelaunchTo(update), locales.In("zh-Hans").UpdateRelaunchTo(update)}, updateSearchWords...)
	}
	names := []string{ownTitle(locales.In("en"), id), ownTitle(locales.In("zh-Hans"), id)}
	switch id {
	case clipItemID:
		names = append(names, clipSearchWords...)
	case notesItemID, newNoteItemID, searchNotesID:
		names = append(names, noteSearchWords[id]...)
	}
	return names
}

// noteSearchWords are the other words the notes' items are found by
// (copy.md 16.1); not 备忘录, Apple Notes' name, which those looking for
// it would find ours first by.
var noteSearchWords = map[string][]string{
	notesItemID:   {"便签", "备忘", "记事本", "note", "notes", "memo", "scratchpad", "jot"},
	newNoteItemID: {"写笔记", "记一下", "新便签", "create note", "quick note", "new memo"},
	searchNotesID: {"找笔记", "浏览笔记", "所有笔记", "find note", "browse notes"},
}

// updateSearchWords find Relaunch to Update by its first letters, in
// either language (design.md 4.5).
var updateSearchWords = []string{"Update", "Relaunch", "更新", "重新打开"}

// ownSymbol is the symbol of Gocast's own item.
func ownSymbol(own []ownItem, id string) string {
	if i := ownIndex(own, id); i >= 0 {
		return own[i].symbol
	}
	return "doc.on.clipboard"
}

// ownIndex is the index of the item with the ID among Gocast's own, or -1.
func ownIndex(own []ownItem, id string) int {
	return slices.IndexFunc(own, func(o ownItem) bool { return o.id == id })
}

// scored reports whether opening the item from the panel counts toward
// the suggestions: all but Quit Gocast, which would otherwise come first
// among them the next day, and Relaunch to Update, which is gone once
// it ran (design.md 4.5).
func scored(id string) bool { return id != quitItemID && id != updateItemID }

// The apps that pad the suggestions, after the default preset, by bundle
// ID (design.md 2.5a): Finder, Safari, System Settings, Notes.
var padApps = []string{"com.apple.finder", "com.apple.Safari", "com.apple.systempreferences", "com.apple.Notes"}

// padIDs are the items that pad the suggestions while they are fewer than
// suggestLeast, in order: the default preset (else the first), then the
// common apps.
func padIDs(ps []presets.Preset) []string {
	var ids []string
	if i := presets.DefaultPreset(ps); i >= 0 {
		ids = append(ids, "preset:"+ps[i].ID)
	} else if len(ps) > 0 {
		ids = append(ids, "preset:"+ps[0].ID)
	}
	for _, id := range padApps {
		ids = append(ids, "app:"+id)
	}
	return ids
}

// suggested picks the suggestions: of the items ranked, those there are,
// at most suggestMost; while fewer than suggestLeast, the padding there is
// and not among them, up to suggestLeast.
func suggested(ranked, pad []string, exists func(id string) bool) []string {
	var out []string
	for _, id := range ranked {
		if len(out) == suggestMost {
			break
		}
		if exists(id) {
			out = append(out, id)
		}
	}
	for _, id := range pad {
		if len(out) >= suggestLeast {
			break
		}
		if exists(id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// panelRow is a row of the search panel's list: a group's title, a search
// item (an app, a preset, the clipboard history), one of Gocast's own
// items, or the calculator's card (design.md 2.3a).
type panelRow struct {
	calc  bool   // the calculator's card, the first row
	title string // the group's title, for a title's row
	item  int    // the index of the search item, or -1
	own   int    // the index of Gocast's own item, or -1
	group bool   // in the Gocast group: its shortcut at the right, rather than what it is
	// later: not chosen as the list shows, though first of its group
	// (Relaunch to Update, design.md 4.5).
	later bool
}

// itemRows are the rows of the search results.
func itemRows(results []int) []panelRow {
	rows := make([]panelRow, len(results))
	for i, r := range results {
		rows[i] = panelRow{item: r, own: -1}
	}
	return rows
}

// suggestionRows lays out the list under an empty query: the suggestions
// under their title, if any, then Gocast's own items under theirs, those
// shown says (not hidden in the search, design.md 4.4). item returns the
// index of the search item with an ID, or -1; Gocast's own items show as
// theirs, the clipboard history too.
func suggestionRows(suggestions []string, item func(id string) int, own []ownItem, shown func(id string) bool) []panelRow {
	var rows []panelRow
	for _, id := range suggestions {
		r := panelRow{item: -1, own: -1}
		if strings.HasPrefix(id, ownItemPrefix) {
			r.own = ownIndex(own, id)
		} else {
			r.item = item(id)
		}
		if r.own < 0 && r.item < 0 {
			continue
		}
		if len(rows) == 0 {
			rows = append(rows, panelRow{title: locales.SearchSuggestions(), item: -1, own: -1})
		}
		rows = append(rows, r)
	}
	titled := false
	for i, o := range own {
		if o.searched || !shown(o.id) {
			continue
		}
		if !titled {
			rows = append(rows, panelRow{title: locales.SearchGocastGroup(), item: -1, own: -1})
			titled = true
		}
		rows = append(rows, panelRow{item: -1, own: i, group: true, later: o.id == updateItemID})
	}
	return rows
}

// firstRow is the row chosen as the list shows: the first that is not a
// title, nor Relaunch to Update unless it is the only row; or -1.
func firstRow(rows []panelRow) int {
	if i := slices.IndexFunc(rows, func(r panelRow) bool { return r.title == "" && !r.later }); i >= 0 {
		return i
	}
	return slices.IndexFunc(rows, func(r panelRow) bool { return r.title == "" })
}

// stepRow moves the choice from the row by step rows, over the titles,
// stopping at either end without wrapping.
func stepRow(rows []panelRow, from, step int) int {
	if from < 0 || from >= len(rows) {
		return firstRow(rows)
	}
	dir := 1
	if step < 0 {
		dir, step = -1, -step
	}
	at := from
	for r := from + dir; r >= 0 && r < len(rows) && step > 0; r += dir {
		if rows[r].title == "" {
			at = r
			step--
		}
	}
	return at
}

// revealedRow is the row to scroll into view to show the row chosen: its
// group's title with it when it is the first of its group and the title is
// not in view, first being the first row in view (design.md 2.5a).
func revealedRow(rows []panelRow, chosen, first int) int {
	if chosen > 0 && chosen < len(rows) && rows[chosen-1].title != "" && chosen-1 <= first {
		return chosen - 1
	}
	return chosen
}

// restoring reports whether the panel, hidden at hiddenAt, shows again as
// it was at now: its query, choice and suggestions (design.md 2.5a, 2.6).
func restoring(hiddenAt, now time.Time) bool {
	return !hiddenAt.IsZero() && now.Sub(hiddenAt) < restoreFor
}
