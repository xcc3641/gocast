package app

// What the search leaves out (design.md 4.4, 2.5): the items hidden in
// Settings › Search & Shortcuts, or with Hide from Search in the search
// panel's Cmd-K menu, which typing does not find and the list under an
// empty query does not show. Their hotkeys work as before, and the
// tray menu's items stay.

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"gocast/internal/keepawake"
	"gocast/internal/settings"
	"gocast/locales"
)

// hiddenNoticeFor is how long the notice of an item hidden shows: longer
// than the others, to read where it comes back from (design.md 2.5).
const hiddenNoticeFor = 6 * time.Second

// systemAppsDir holds the system's apps, its Utilities too: the group
// System Apps (design.md 4.4).
const systemAppsDir = "/System/Applications/"

// itemGroup is the group of Settings › Search & Shortcuts of the search
// item with the ID, an app's at path: by the tag of its row in the search
// panel (design.md 4.4), the apps under /System/Applications apart.
func itemGroup(id, path string) string {
	switch {
	case strings.HasPrefix(id, ownItemPrefix):
		return settings.GroupGocast
	case strings.HasPrefix(id, actionPrefix), strings.HasPrefix(id, awakePrefix):
		return settings.GroupSystem
	case strings.HasPrefix(id, "preset:"):
		return settings.GroupPresets
	case strings.HasPrefix(path, systemAppsDir):
		return settings.GroupSystemApps
	}
	return settings.GroupOtherApps
}

// hideable reports whether the search item with the ID may be hidden:
// all but Relaunch to Update, which is for a while, and the row of what
// is typed after Keep Awake (design.md 2.5).
func hideable(id string) bool {
	return id != "" && id != updateItemID && id != awakeTypedID
}

// awakeItemID is the search item of Keep Awake's row of the hotkey item
// of AwakeHotkeys: "awake:toggle", "awake:30", ...
func awakeItemID(item string) string { return awakePrefix + item }

// awakeHotkeyItems are the items of AwakeHotkeys, in the order of the
// search panel's rows: Keep Awake, then its lengths.
func awakeHotkeyItems() []string {
	items := []string{settings.AwakeToggle}
	for _, d := range keepawake.Presets {
		items = append(items, strconv.Itoa(int(d/time.Minute)))
	}
	return items
}

// awakeRequest is what the hotkey of the Keep Awake item starts, ok false
// for Keep Awake itself (a toggle) or an item unknown.
func awakeRequest(item string) (r keepawake.Request, ok bool) {
	m, err := strconv.Atoi(item)
	if err != nil || m <= 0 {
		return r, false
	}
	return keepawake.Request{Kind: keepawake.For, Duration: time.Duration(m) * time.Minute}, true
}

// awakeItemName is the name of Keep Awake's row of the item, which stays
// as it is while Keep Awake is on (design.md 4.4).
func awakeItemName(item string) string {
	if r, ok := awakeRequest(item); ok {
		return awakeWords().Title(r, time.Time{})
	}
	return locales.AwakeTitle()
}

// awakeHotkeyPressed runs the hotkey of a Keep Awake row (design.md 4.4):
// Keep Awake's own starts it until stopped, or stops it, whatever runs; a
// length's starts it, in place of what runs. No window shows: the tray
// icon says it.
func (g *gocast) awakeHotkeyPressed(item string) {
	if g.awake == nil {
		return
	}
	r, ok := awakeRequest(item)
	switch {
	case ok:
		g.keepAwake(r)
	case item != settings.AwakeToggle:
	case g.awakeActive():
		g.stopAwake()
	default:
		g.keepAwake(keepawake.Request{Kind: keepawake.Forever})
	}
}

// ownHotkeyPressed runs the hotkey of Gocast's own item: what choosing
// its row in the search panel does (design.md 4.4).
func (g *gocast) ownHotkeyPressed(item string) {
	if g.launcher.visible() {
		g.launcher.hideForAction()
	}
	switch item {
	case settings.OwnAddPreset:
		g.addPreset()
	case settings.OwnSettings:
		g.openSettings("")
	}
}

// ownHotkeyName is the name of Gocast's own item of GocastHotkeys.
func ownHotkeyName(item string) string {
	if item == settings.OwnSettings {
		return locales.TraySettings()
	}
	return locales.PresetsAdd()
}

// ownHotkeyItemID is the search item of Gocast's own item of
// GocastHotkeys.
func ownHotkeyItemID(item string) string {
	if item == settings.OwnSettings {
		return settingsItemID
	}
	return addPresetItemID
}

// searchHidden reports whether the search leaves out the item with the
// ID, an app's at path.
func (g *gocast) searchHidden(id, path string) bool {
	return hideable(id) && g.settings.Hidden(id, itemGroup(id, path))
}

// setSearchHidden hides the item with the ID, of group, in the search, or
// shows it, at once (design.md 4.4).
func (g *gocast) setSearchHidden(id, group string, hidden bool) {
	g.settings.SetHidden(id, group, hidden)
	g.saveSettings()
	g.searchChanged()
}

// setGroupHidden hides the group whole in the search, or shows it again.
func (g *gocast) setGroupHidden(group string, hidden bool) {
	g.settings.SetGroupHidden(group, hidden)
	g.saveSettings()
	g.searchChanged()
}

// searchChanged finds the search panel's rows again, what it leaves out
// changed: in effect at once, as the next key typed (design.md 4.4).
func (g *gocast) searchChanged() {
	if l := g.launcher; l != nil && l.itemAt != nil {
		l.rebuild()
		l.redraw()
	}
}

// --- The search panel ---

// itemHidden reports whether the search leaves out the search item.
func (l *launcher) itemHidden(item int) bool {
	path := ""
	if a := l.entries[item].app; a != nil {
		path = a.Path
	}
	return l.g.searchHidden(l.items[item].ID, path)
}

// suggestable reports whether the item with the ID may be suggested:
// there to open, and not hidden.
func (l *launcher) suggestable(id string) bool {
	if !l.exists(id) {
		return false
	}
	if i, ok := l.itemAt[id]; ok {
		return !l.itemHidden(i)
	}
	return !l.g.searchHidden(id, "")
}

// shownOwn are Gocast's own items the search does not leave out, as the
// list under an empty query shows them.
func (l *launcher) shownOwn(id string) bool { return !l.g.searchHidden(id, "") }

// withoutHidden leaves out of the rows the items hidden.
func (l *launcher) withoutHidden(rows []panelRow) []panelRow {
	return slices.DeleteFunc(rows, func(r panelRow) bool { return r.item >= 0 && l.itemHidden(r.item) })
}

// hideAction is the Cmd-K menu's last item, Hide from Search, for the
// row chosen when it may be hidden (design.md 2.5).
func (l *launcher) hideAction() (action, bool) {
	id := l.currentID()
	if !hideable(id) {
		return action{}, false
	}
	path := ""
	if e, ok := l.current(); ok && e.app != nil {
		path = e.app.Path
	}
	return action{"eye.slash", locales.ActionHideFromSearch(), nil, func() { l.hideItem(id, path) }}, true
}

// hideItem hides the item with the ID, an app's at path, in the search
// (design.md 2.5): its row goes, the next one chosen in its place (the
// one above when it was the last); the panel stays, saying where the
// item comes back from, for longer than other notices.
func (l *launcher) hideItem(id, path string) {
	at := l.chosen
	l.g.setSearchHidden(id, itemGroup(id, path), true)
	l.choose(nearRow(l.rows, at))
	l.moved = true
	l.sayAction(actionNotice{icon: "eye.slash", text: locales.HiddenFromSearchNotice(), long: true})
}

// nearRow is the row to choose in place of the one at i, gone: the next,
// else the one before; -1 for none.
func nearRow(rows []panelRow, i int) int {
	if len(rows) == 0 {
		return -1
	}
	return min(max(i, 0), len(rows)-1)
}
