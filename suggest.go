package main

import (
	"slices"
	"strings"
	"time"

	"gocast/internal/presets"
	"gocast/internal/shortcut"
	"gocast/locales"
)

// The search panel's list under an empty query (design.md 2.5a): the
// suggestions, then Gocast's own items.

// The IDs of Gocast's own items, which usage.json scores as it does apps
// and presets. Of these, the search finds the clipboard history alone.
const (
	clipItemID      = "gocast:clipboard"
	termItemID      = "gocast:terminal"
	addPresetItemID = "gocast:add-preset"
	settingsItemID  = "gocast:settings"
	quitItemID      = "gocast:quit"
)

// ownItemPrefix begins the IDs of Gocast's own items.
const ownItemPrefix = "gocast:"

// ownItem is one of Gocast's own items: a row of the Gocast group, which
// the suggestions may show too.
type ownItem struct {
	id, symbol, label string
	caps              []string // its shortcut, at the right of its row in the Gocast group
	run               func()
}

// gocastDo is what Gocast's own items do.
type gocastDo struct {
	clipboard, terminal, addPreset, settings, quit func()
}

// gocastItems are the rows of the Gocast group, in copy.md 2.2a's order:
// the clipboard history and the floating terminal with their shortcuts,
// Add Preset, Settings… with ⌘, and Quit Gocast, without ⌘Q.
func gocastItems(terminalKey, clipKey string, do gocastDo) []ownItem {
	return []ownItem{
		{clipItemID, "doc.on.clipboard", locales.ClipboardHistory(), shortcut.Caps(clipKey), do.clipboard},
		{termItemID, "apple.terminal", locales.MenuTerminal(), shortcut.Caps(terminalKey), do.terminal},
		{addPresetItemID, "plus", locales.PresetsAdd(), nil, do.addPreset},
		{settingsItemID, "gearshape", locales.TraySettings(), []string{"⌘", ","}, do.settings},
		{quitItemID, "power", locales.TrayQuit(), nil, do.quit},
	}
}

// ownIndex is the index of the item with the ID among Gocast's own, or -1.
func ownIndex(own []ownItem, id string) int {
	return slices.IndexFunc(own, func(o ownItem) bool { return o.id == id })
}

// scored reports whether opening the item from the panel counts toward
// the suggestions: all but Quit Gocast, which would otherwise come first
// among them the next day.
func scored(id string) bool { return id != quitItemID }

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
// item (an app, a preset, the clipboard history), or one of Gocast's own
// items.
type panelRow struct {
	title string // the group's title, for a title's row
	item  int    // the index of the search item, or -1
	own   int    // the index of Gocast's own item, or -1
	group bool   // in the Gocast group: its shortcut at the right, rather than what it is
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
// under their title, if any, then Gocast's own items under theirs. item
// returns the index of the search item with an ID, or -1; Gocast's own
// items show as theirs, the clipboard history too.
func suggestionRows(suggestions []string, item func(id string) int, own []ownItem) []panelRow {
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
	if len(own) > 0 {
		rows = append(rows, panelRow{title: locales.SearchGocastGroup(), item: -1, own: -1})
	}
	for i := range own {
		rows = append(rows, panelRow{item: -1, own: i, group: true})
	}
	return rows
}

// firstRow is the first row that is not a title, or -1.
func firstRow(rows []panelRow) int {
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
