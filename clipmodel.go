package main

// clipmodel.go is what the clipboard history's view shows, worked out
// without drawing (design.md 2.8, copy.md 6): the list's rows under their
// section titles, the choice moving over them, each item's title, what
// assistive technology reads of it, its info, the actions it has, and the
// state the view is in. clipview.go draws it.

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/i18n"

	"gocast/internal/clipboard"
	"gocast/internal/presets"
	"gocast/internal/sysfmt"
	"gocast/locales"
)

// clipRow is a row of the list: a section's title, or an item.
type clipRow struct {
	title   bool              // a section's title
	section clipboard.Section // the section it is in, or titles
	entry   int               // the item's index in the entries, for an item
}

// clipRows lays the entries (pinned first, then newest first, as the
// store lists them) out under their sections' titles, as of now: no title
// for a section without items.
func clipRows(entries []clipboard.Entry, now time.Time) []clipRow {
	var rows []clipRow
	last := clipboard.Section(-1)
	for i, e := range entries {
		s := clipboard.SectionOf(e, now)
		if s != last {
			rows = append(rows, clipRow{title: true, section: s})
			last = s
		}
		rows = append(rows, clipRow{section: s, entry: i})
	}
	return rows
}

// sectionTitle is a section's title (copy.md 6.2).
func sectionTitle(s clipboard.Section) string {
	switch s {
	case clipboard.SectionPinned:
		return locales.ClipSectionPinned()
	case clipboard.SectionToday:
		return locales.ClipSectionToday()
	case clipboard.SectionYesterday:
		return locales.ClipSectionYesterday()
	}
	return locales.ClipSectionEarlier()
}

// rowOfEntry is the row of the entry with index i, or -1.
func rowOfEntry(rows []clipRow, i int) int {
	for r, row := range rows {
		if !row.title && row.entry == i {
			return r
		}
	}
	return -1
}

// rowOfID is the row of the entry with id, or -1.
func rowOfID(rows []clipRow, entries []clipboard.Entry, id string) int {
	for r, row := range rows {
		if !row.title && entries[row.entry].ID == id {
			return r
		}
	}
	return -1
}

// freshChoice is the row a fresh view chooses: the newest unpinned item
// (design.md 2.8.1), the first when all are pinned; -1 with none.
func freshChoice(rows []clipRow, entries []clipboard.Entry) int {
	return rowOfEntry(rows, clipboard.FirstUnpinned(entries))
}

// firstItem is the row of the first item, as a query or a filter
// chooses it; -1 with none.
func firstItem(rows []clipRow) int {
	for r, row := range rows {
		if !row.title {
			return r
		}
	}
	return -1
}

// queryChoice is the row a query chooses as it changes: the first item;
// the query cleared, the newest unpinned again, as a fresh view's
// (design.md 2.8.1).
func queryChoice(rows []clipRow, entries []clipboard.Entry, query string) int {
	if strings.TrimSpace(query) == "" {
		return freshChoice(rows, entries)
	}
	return firstItem(rows)
}

// revealRow is the row to scroll into view to show row chosen, where the
// list showed rows first to last in the last frame (last below first
// before it was laid out). The first item of a section shows with the
// section's title (design.md 2.8.1): the title when it is at or above the
// top, the list scrolling up as little as shows it whole, chosen below
// it; else chosen, the title over it once it is at the bottom.
func revealRow(rows []clipRow, chosen, first, last int) int {
	if chosen > 0 && chosen < len(rows) && rows[chosen-1].title && last >= first && chosen-1 <= first {
		return chosen - 1
	}
	return chosen
}

// moveChoice moves the choice from row by step items, over the sections'
// titles, stopping at either end without wrapping.
func moveChoice(rows []clipRow, from, step int) int {
	if from < 0 || from >= len(rows) {
		return firstItem(rows)
	}
	dir := 1
	if step < 0 {
		dir, step = -1, -step
	}
	at := from
	for r := from + dir; r >= 0 && r < len(rows) && step > 0; r += dir {
		if !rows[r].title {
			at = r
			step--
		}
	}
	return at
}

// nextAfterDelete is the ID of the item to choose as the item at row goes:
// the next, or the one before it when it is the last; "" when it was the
// only one.
func nextAfterDelete(rows []clipRow, entries []clipboard.Entry, row int) string {
	if n := moveChoice(rows, row, 1); n != row {
		return entries[rows[n].entry].ID
	}
	if p := moveChoice(rows, row, -1); p != row {
		return entries[rows[p].entry].ID
	}
	return ""
}

var spaces = regexp.MustCompile(`\s+`)

// firstLine is the first line of text that is not blank, its runs of
// white space as one space.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if l := strings.TrimSpace(spaces.ReplaceAllString(line, " ")); l != "" {
			return l
		}
	}
	return ""
}

// clipTitle is an item's title in the list (design.md 2.8.4): text's
// first line, a link whole, an image's size, files' names.
func clipTitle(e clipboard.Entry) string {
	switch e.Kind {
	case clipboard.KindLink:
		return strings.TrimSpace(e.Text)
	case clipboard.KindImage:
		return locales.ClipImageTitle(e.Width, e.Height)
	case clipboard.KindFiles:
		if len(e.Files) == 0 {
			return ""
		}
		first := filepath.Base(e.Files[0])
		if len(e.Files) == 1 {
			return first
		}
		return locales.ClipFilesTitle(first, len(e.Files))
	}
	return firstLine(e.Text)
}

// listSep is what separates the parts of what assistive technology
// reads: a Chinese comma in Chinese.
func listSep() string {
	if strings.HasPrefix(i18n.Locale(), "zh") {
		return "，"
	}
	return ", "
}

// clipLabel is what assistive technology reads of an item's row (copy.md
// 6.10): "{first line}, Text, {time}[, Pinned]", "Image, 756 by 133, …".
func clipLabel(e clipboard.Entry, when string) string {
	sep := listSep()
	var s string
	switch e.Kind {
	case clipboard.KindLink:
		s = clipTitle(e) + sep + locales.ClipLabelLink()
	case clipboard.KindImage:
		s = locales.ClipLabelImage(e.Width, e.Height)
	case clipboard.KindFiles:
		if len(e.Files) > 1 {
			s = locales.ClipLabelFiles(filepath.Base(e.Files[0]), len(e.Files))
		} else {
			s = clipTitle(e) + sep + locales.ClipLabelFile()
		}
	default:
		s = clipTitle(e) + sep + locales.ClipLabelText()
	}
	s += sep + when
	if e.Pinned {
		s += sep + locales.ClipLabelPinned()
	}
	return s
}

// clipSymbol is the symbol of an item's kind (design.md 2.8.4); images
// and files show their thumbnail and icon instead.
func clipSymbol(k clipboard.Kind) string {
	switch k {
	case clipboard.KindRichText:
		return "doc.richtext"
	case clipboard.KindLink:
		return "link"
	case clipboard.KindImage:
		return "photo"
	case clipboard.KindFiles:
		return "doc"
	}
	return "doc.plaintext"
}

// clipFilter is a choice of the type filter (design.md 2.8.2).
type clipFilter struct {
	filter clipboard.Filter
	symbol string
	label  func() string
	none   func() string // "No images"
}

var clipFilters = []clipFilter{
	{clipboard.FilterAll, "square.grid.2x2", locales.ClipFilterAll, locales.ClipEmptyTitle},
	{clipboard.FilterText, "doc.plaintext", locales.ClipFilterText, locales.ClipNoText},
	{clipboard.FilterLinks, "link", locales.ClipFilterLinks, locales.ClipNoLinks},
	{clipboard.FilterImages, "photo", locales.ClipFilterImages, locales.ClipNoImages},
	{clipboard.FilterFiles, "doc", locales.ClipFilterFiles, locales.ClipNoFiles},
}

func filterOf(f clipboard.Filter) clipFilter {
	for _, cf := range clipFilters {
		if cf.filter == f {
			return cf
		}
	}
	return clipFilters[0]
}

// linkDomain is a link's host, without "www.": what the info says of it.
func linkDomain(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(link)
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// fileExists reports whether a file is still where it was copied from.
var fileExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// missingFile reports whether any of the files is gone.
func missingFile(files []string) bool {
	for _, f := range files {
		if !fileExists(f) {
			return true
		}
	}
	return false
}

// clipInfo is a line of the info under the preview: a label and its value;
// source marks the line of the app copied from, which has its icon.
type clipInfo struct {
	label, value string
	source       bool
}

// clipInfos are the lines of an item's info (design.md 2.8.5, copy.md
// 6.3): where it was copied from and when, then what fits its kind.
// totalSize says how large files are together, false when it can't
// (a folder among them).
func clipInfos(e clipboard.Entry, now time.Time, locale string, totalSize func([]string) (int64, bool)) []clipInfo {
	app := e.App.Name
	if app == "" {
		app = locales.ClipUnknownApp()
	}
	infos := []clipInfo{
		{label: locales.ClipInfoSource(), value: app, source: true},
		{label: locales.ClipInfoCopied(), value: sysfmt.Date(e.Time, now, locale)},
	}
	switch e.Kind {
	case clipboard.KindLink:
		infos = append(infos, clipInfo{label: locales.ClipInfoLink(), value: linkDomain(e.Text)})
	case clipboard.KindImage:
		infos = append(infos,
			clipInfo{label: locales.ClipInfoDimensions(), value: locales.ClipDimensions(e.Width, e.Height)},
			clipInfo{label: locales.ClipInfoSize(), value: sysfmt.Bytes(e.Size)})
	case clipboard.KindFiles:
		if len(e.Files) > 0 {
			infos = append(infos, clipInfo{label: locales.ClipInfoLocation(), value: presets.ShortPath(filepath.Dir(e.Files[0]))})
		}
		if n, ok := totalSize(e.Files); ok {
			infos = append(infos, clipInfo{label: locales.ClipInfoSize(), value: sysfmt.Bytes(n)})
		}
	default:
		infos = append(infos, clipInfo{label: locales.ClipInfoCharacters(), value: i18n.FormatNumber(locale, e.Characters(), i18n.NumberOptions{})})
	}
	return infos
}

// filesSize is how large files are together; false with a folder among
// them, or one gone.
func filesSize(files []string) (int64, bool) {
	var n int64
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil || fi.IsDir() {
			return 0, false
		}
		n += fi.Size()
	}
	return n, len(files) > 0
}

// previewLimit is how many characters of text the preview shows.
const previewLimit = 20000

// previewText is the text the preview shows, and whether it was cut.
func previewText(text string) (string, bool) {
	if utf8.RuneCountInString(text) <= previewLimit {
		return text, false
	}
	n := 0
	for i := range text {
		if n == previewLimit {
			return text[:i], true
		}
		n++
	}
	return text, false
}

// --- Actions ---

// clipActionID names an action of an item (design.md 2.8.6).
type clipActionID int

const (
	actPaste clipActionID = iota
	actPastePlain
	actCopy
	actOpenLink
	actShowInFinder
	actPin
	actDelete
	actClear
)

// clipAction is a row of the item's Cmd-K card.
type clipAction struct {
	id      clipActionID
	symbol  string
	label   string
	caps    []string
	danger  bool
	divided bool // a divider over it
}

// clipActions are the actions of an item, the primary one first
// (design.md 2.8.6, copy.md 6.5): pasting when Gocast may paste (app is
// the app it pastes into, "" when not known), copying, opening a link,
// showing a file still there, pinning, deleting and clearing.
func clipActions(e clipboard.Entry, canPaste bool, app string) []clipAction {
	var as []clipAction
	copyCaps := []string{"⌘", "↩"}
	if canPaste {
		as = append(as, clipAction{id: actPaste, symbol: "doc.on.clipboard", label: pasteLabel(app), caps: []string{"↩"}})
		if e.Kind == clipboard.KindText || e.Kind == clipboard.KindRichText {
			as = append(as, clipAction{id: actPastePlain, symbol: "doc.plaintext", label: locales.ClipPastePlain()})
		}
	} else {
		copyCaps = []string{"↩"}
	}
	as = append(as, clipAction{id: actCopy, symbol: "doc.on.doc", label: locales.ClipCopy(), caps: copyCaps})
	if e.Kind == clipboard.KindLink {
		as = append(as, clipAction{id: actOpenLink, symbol: "safari", label: locales.ClipOpenLink(), caps: []string{"⌘", "O"}})
	}
	if e.Kind == clipboard.KindFiles && len(e.Files) > 0 && fileExists(e.Files[0]) {
		as = append(as, clipAction{id: actShowInFinder, symbol: "folder", label: locales.ActionShowInFinder()})
	}
	pin := clipAction{id: actPin, symbol: "pin", label: locales.ClipPin(), caps: []string{"⌘", "."}, divided: true}
	if e.Pinned {
		pin.symbol, pin.label = "pin.slash", locales.ClipUnpin()
	}
	return append(as, pin,
		clipAction{id: actDelete, symbol: "trash", label: locales.Delete(), caps: []string{"⌃", "X"}, danger: true, divided: true},
		clipAction{id: actClear, symbol: "xmark.bin", label: locales.ClipClear(), caps: []string{"⌃", "⇧", "X"}, danger: true},
	)
}

// pasteLabel names pasting into app: 粘贴到“{app}”, or 粘贴 when the
// app is not known.
func pasteLabel(app string) string {
	if app == "" {
		return locales.ClipPaste()
	}
	return locales.ClipPasteTo(app)
}

// fitText cuts s at its end, with an ellipsis, to at most max wide as
// measure measures it.
func fitText(s string, max float32, measure func(string) float32) string {
	if measure(s) <= max {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes) // the most runes that fit, with "…"
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(string(runes[:mid])+"…") <= max {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return strings.TrimRight(string(runes[:lo]), " ") + "…"
}

// --- The view's state ---

// clipState is what the view's body shows (design.md 2.8.8).
type clipState int

const (
	clipShowList   clipState = iota // the items
	clipLoading                     // the history being read
	clipUnreadable                  // the saved history can't be read
	clipNone                        // no history
	clipOffNone                     // recording off, and no history
	clipNoResults                   // nothing matches the query
	clipNoneOfKind                  // the filter leaves nothing
)

// clipStateOf is the view's state: loaded, whether the history was read;
// total, how many items there are; shown, how many the query and filter
// leave.
func clipStateOf(loaded, unreadable, recording bool, total int, query string, filter clipboard.Filter, shown int) clipState {
	switch {
	case !loaded:
		return clipLoading
	case unreadable:
		return clipUnreadable
	case total == 0 && !recording:
		return clipOffNone
	case total == 0:
		return clipNone
	case shown > 0:
		return clipShowList
	case strings.TrimSpace(query) != "":
		return clipNoResults
	case filter != clipboard.FilterAll:
		return clipNoneOfKind
	}
	return clipNone
}

// The banners over the view's body (design.md 2.8.7).
const (
	bannerNone  = iota
	bannerOff   // recording is off, the history kept
	bannerAX    // pasting needs Accessibility
	bannerReady // pasting was allowed
)

// clipBanner is the banner to show: off wins over Accessibility; ready
// shows for a while once allowed (ready set).
func clipBanner(state clipState, recording, asked, pasteOnReturn, trusted, ready bool) int {
	switch {
	case state == clipShowList && !recording:
		return bannerOff
	case ready:
		return bannerReady
	case asked && pasteOnReturn && !trusted && state == clipShowList:
		return bannerAX
	}
	return bannerNone
}
