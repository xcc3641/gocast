package main

// clipview.go draws the clipboard history's view, which the search panel
// turns into (design.md 2.8, copy.md 6): the search bar with the type
// filter, the list at the left, the preview and its info at the right, the
// capsules, the Cmd-K card, the banners and the confirmation of clearing.
// clipmodel.go works out what it shows; clipservice.go keeps the history.

import (
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/clipboard"
	"gocast/internal/running"
	"gocast/internal/search"
	"gocast/internal/sysfmt"
	"gocast/locales"
)

// The clipboard view's measures (design.md 2.8), in DIPs.
const (
	clipWidth      = 760
	clipListWidth  = 300
	clipPaneWidth  = clipWidth - clipListWidth - 1 // 459, past the 1 DIP divider
	clipTitleMax   = 226                           // a row's title
	clipPanePad    = 20                            // the right pane's sides
	clipBottom     = 52                            // under the list and the info, for the capsules
	clipInfoRow    = 28
	clipNoticeMax  = clipListWidth - 2*capsuleInset // 284
	clipActionsMax = clipPaneWidth - 2*capsuleInset // 443
	clipAppNameMax = 160                            // the app's name in "Paste to …"
	clipFilterW    = 200                            // the type filter's card
	clipConfirmW   = 340
	readyFor       = 3 * time.Second // "Ready to paste" shows
	imageSpinAfter = 150 * time.Millisecond
)

// clipSearchWords find the clipboard history's row in the search panel,
// besides its name (copy.md 6.1).
var clipSearchWords = []string{"Clipboard History", "剪贴板历史", "剪贴板", "粘贴板", "剪切板", "历史", "clipboard", "paste", "history"}

// clipView is the clipboard history's view. Its state is read and changed
// on the main thread.
type clipView struct {
	l    *launcher
	open bool
	// fromSearch: the view came from the search panel, which Escape goes
	// back to; else from its shortcut, and Escape hides the panel.
	fromSearch bool

	query     string
	filter    clipboard.Filter
	entries   []clipboard.Entry
	rows      []clipRow
	total     int // the items in the history
	chosen    int // the row chosen, -1 for none
	chosenID  string
	list      ui.ListState
	input     ui.Handle
	composing bool
	scroll    bool

	// target is where to paste: the app frontmost as the view opened,
	// Gocast aside, or the floating terminal when it was key.
	target  pasteTarget
	trusted bool // Gocast may press ⌘V, as of the view opening
	ready   bool // "Ready to paste" shows
	readyN  int

	filterOpen   bool
	filterChosen int

	menuOpen      bool
	menuComposing bool
	menuQuery     string
	menuChosen    int
	menuInput     ui.Handle
	menuItems     []clipAction
	menuShown     []int

	confirmClear bool

	notice       string
	noticeDanger bool
	noticeGen    int
	announce     string

	thumbs map[string]*ui.Bitmap // by entry ID; nil where it can't be read
	icons  map[string]*ui.Bitmap // by path and size
	image  clipImage
	pane   ui.Rect // the preview's content box, as last laid out
}

// clipImage is the image the preview shows, read on a goroutine.
type clipImage struct {
	id      string
	bitmap  *ui.Bitmap
	started time.Time
	done    bool
}

func newClipView(l *launcher) *clipView {
	return &clipView{l: l, chosen: -1, thumbs: map[string]*ui.Bitmap{}, icons: map[string]*ui.Bitmap{}}
}

func (v *clipView) svc() *clipService { return v.l.g.clip }

// --- Entering and leaving (design.md 2.8.1) ---

// openClip turns the panel into the clipboard view, fresh: fromSearch,
// from the search panel, which Escape goes back to; else shown at once.
func (l *launcher) openClip(fromSearch bool) {
	l.enterClip(pasteTarget{app: frontmostOther(), floating: l.g.term.focused()}, fromSearch)
	if !l.win.IsVisible() {
		d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
		wa := d.WorkArea
		l.win.SetPosition(wa.X+(wa.Width-windowWidth)/2, wa.Y+wa.Height/5-marginTop)
		l.win.Show()
	}
	l.clip.input.Focus()
	l.win.Invalidate()
}

// enterClip is openClip's state: all of it changes before the window
// shows, which then draws the clipboard view as its first frame.
func (l *launcher) enterClip(target pasteTarget, fromSearch bool) {
	v := l.clip
	v.target = target
	v.fromSearch = fromSearch
	v.open = true
	v.query, v.filter = "", clipboard.FilterAll
	v.filterOpen, v.menuOpen, v.confirmClear = false, false, false
	v.notice = ""
	v.list = ui.ListState{}
	v.chosen, v.chosenID = -1, ""
	v.image = clipImage{}
	v.checkTrust()
	v.refresh()
	v.choose(freshChoice(v.rows, v.entries))
	l.menuOpen = false
	l.gen++ // at its size at once, without animating to it
	l.shown = true
}

// toggleClip is the clipboard shortcut: it hides the view, else turns the
// panel into it.
func (l *launcher) toggleClip() {
	switch {
	case l.win.IsVisible() && l.clip.open:
		l.hide()
	case l.win.IsVisible():
		l.openClip(true)
	default:
		l.openClip(false)
	}
}

// leaveClip goes back to the search panel, empty: its suggestions, the
// first chosen, or its bar alone (design.md 2.8.1).
func (l *launcher) leaveClip() {
	l.clip.closed()
	l.query, l.filtered, l.rows, l.moved, l.expanded = "", noQuery, nil, false, false
	l.notice = ""
	l.list = ui.ListState{}
	l.filter() // the suggestions, the first chosen (design.md 2.8.1)
	l.gen++
	l.input.Focus()
	l.win.Invalidate()
}

// closed forgets the view as it closes.
func (v *clipView) closed() {
	v.open = false
	v.menuOpen, v.filterOpen, v.confirmClear = false, false, false
	v.image = clipImage{}
	v.entries, v.rows = nil, nil
}

// checkTrust asks the system whether Gocast may press ⌘V, as the view
// opens: allowed since the banner asked, it says so for a while.
func (v *clipView) checkTrust() {
	s := v.svc()
	v.trusted = s.trusted()
	if s.asked && v.trusted {
		s.asked = false
		v.ready = true
		v.announce = locales.ClipAXReady()
		v.readyN++
		n := v.readyN
		time.AfterFunc(readyFor, func() {
			v.l.win.Update(func() {
				if v.readyN == n {
					v.ready = false
				}
			})
		})
	}
}

// historyChanged shows the history as it is now, the choice on its item.
func (v *clipView) historyChanged() {
	if !v.open {
		return
	}
	was := v.chosen
	v.refresh()
	if v.chosen < 0 && len(v.rows) > 0 && was >= 0 {
		v.choose(moveChoice(v.rows, min(was, len(v.rows)-1), 0))
	}
	v.l.win.Invalidate()
}

// refresh lists the items the query and the filter leave, the choice on
// the item it was on, else on the first.
func (v *clipView) refresh() {
	st := v.svc().store
	if st == nil {
		v.entries, v.rows, v.total = nil, nil, 0
		v.chosen = -1
		return
	}
	v.entries = st.Search(v.query, v.filter)
	v.total = st.Len()
	v.rows = clipRows(v.entries, time.Now())
	r := rowOfID(v.rows, v.entries, v.chosenID)
	if r < 0 {
		r = firstItem(v.rows)
	}
	v.choose(r)
}

// choose chooses row r (-1 for none), and shows it in the next frame.
func (v *clipView) choose(r int) {
	if r >= 0 && r < len(v.rows) && v.rows[r].title {
		r = moveChoice(v.rows, r, 1)
	}
	if r < 0 || r >= len(v.rows) || v.rows[r].title {
		r = -1
	}
	v.chosen, v.chosenID = r, ""
	if r >= 0 {
		v.chosenID = v.entries[v.rows[r].entry].ID
	}
	v.list.Selected = &v.chosen
	v.list.Choose(r)
	v.scroll = true
}

// current is the item chosen.
func (v *clipView) current() (clipboard.Entry, bool) {
	if v.chosen < 0 || v.chosen >= len(v.rows) || v.rows[v.chosen].title {
		return clipboard.Entry{}, false
	}
	return v.entries[v.rows[v.chosen].entry], true
}

func (v *clipView) state() clipState {
	s := v.svc()
	unreadable := s.store != nil && s.store.Unreadable()
	return clipStateOf(s.loaded, unreadable, s.recording(), v.total, v.query, v.filter, len(v.entries))
}

// canPaste reports whether Return pastes: on in Settings, and allowed.
func (v *clipView) canPaste() bool {
	return v.l.g.settings.PasteOnReturn() && v.target.allowed(v.trusted)
}

// --- Drawing ---

func (v *clipView) view(c *ui.Context, k tokens) {
	v.shortcuts(c)
	state := v.state()
	s := v.svc()
	banner := clipBanner(state, s.recording(), s.asked, v.l.g.settings.PasteOnReturn(), v.target.allowed(v.trusted), v.ready)

	c.Key(v.l.gen)
	panel := ui.Column(c).Absolute().Left(marginX).Top(marginTop).Width(clipWidth).Height(expandedHeight).
		Material(glassOf(k, searchTint)).Clip()
	panelShadow(panelOutline(panel, k), k)
	panel.Children(func() {
		v.searchBar(c, k)
		ui.Column(c).Height(listHeight).Shrink(0).Children(func() {
			v.banner(c, k, banner)
			if state == clipShowList {
				ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
					ui.Column(c).Width(clipListWidth).Shrink(0).Children(func() { v.listPane(c, k) })
					ui.Box(c).Width(1).Shrink(0).Background(k.hairline).AccessHidden(true)
					ui.Column(c).Width(clipPaneWidth).Shrink(0).Children(func() { v.previewPane(c, k) })
				})
			} else {
				v.emptyState(c, k, state)
			}
		})
		v.footer(c, k, state)
		if v.filterOpen {
			v.filterCard(c, k)
		}
		if v.menuOpen {
			v.actionsCard(c, k)
		}
		if v.confirmClear {
			v.confirmation(c, k)
		}
	})
	if v.announce != "" {
		c.Announce(v.announce)
		v.announce = ""
	}
}

// shortcuts are the view's keys, whatever has the focus (design.md 2.8.9);
// the card's keys work with it closed.
func (v *clipView) shortcuts(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		v.l.hide()
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		v.l.hide()
		v.l.g.openSettings(pageClipboard)
	}
	if v.confirmClear {
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		v.toggleMenu()
	}
	if c.Shortcut(ui.Cmd, ui.KeyP) {
		v.toggleFilter()
	}
	e, ok := v.current()
	if c.Shortcut(ui.Cmd, ui.KeyPeriod) && ok {
		v.run(actPin, e)
	}
	if c.Shortcut(ui.Ctrl, ui.KeyX) && ok {
		v.run(actDelete, e)
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyX) {
		v.askClear()
	}
	if c.Shortcut(ui.Cmd, ui.KeyO) && ok && e.Kind == clipboard.KindLink {
		v.run(actOpenLink, e)
	}
}

// searchBar is the search panel's bar with the type filter at its right
// (design.md 2.8.2).
func (v *clipView) searchBar(c *ui.Context, k tokens) {
	ui.Row(c).Height(barHeight).Shrink(0).Padding(0, rightLine, 0, 20).Gap(12).AlignItems(ui.Center).Children(func() {
		gocastMark(c, k).Margin(1, 0, 0, 0)
		field := ui.TextInputBase(c, &v.query).Bind(&v.input).Grow(1).FontSize(20).
			Placeholder(locales.ClipPlaceholder()).Label(locales.ClipPlaceholder()).
			OnTextKey(v.textKey)
		v.composing = field.Composing()
		if field.Changed() {
			v.queryChanged()
		}
		f := filterOf(v.filter)
		b := ui.Row(c).Height(28).Radius(14).Padding(0, 10, 0, 12).Gap(6).AlignItems(ui.Center).Shrink(0).
			Background(k.keycap).Role(ui.RoleButton).Label(locales.ClipTypeLabel(f.label())).
			KeyShortcuts(ui.Cmd, ui.KeyP).HasPopup(ui.PopupMenu).Expanded(v.filterOpen).OnClick(v.toggleFilter)
		if b.Pressed() {
			b.Background(k.selection)
		}
		b.Children(func() {
			ui.Text(c, f.label()).FontSize(13).TextColor(k.text).SingleLine().AccessHidden(true)
			sfsymbols.Symbol(c, "chevron.down").Size(10).Weight(sfsymbols.Semibold).Color(k.textOnGlass)
		})
	})
}

// queryChanged lists what the new query finds, the first chosen.
func (v *clipView) queryChanged() {
	v.chosenID = ""
	v.menuOpen = false
	v.refresh()
	v.choose(queryChoice(v.rows, v.entries, v.query))
}

// textKey takes the search field's keys that move the choice and act on
// it, none while the input method composes (design.md 2.7).
func (v *clipView) textKey(key ui.TextKey) bool {
	if v.composing {
		return false
	}
	// As the search panel's: the query as it is now, the bound value put
	// back for the field to write.
	bound := v.query
	if q := key.Text(); q != v.query {
		v.query = q
		v.queryChanged()
		defer func() { v.query = bound }()
	}
	mods := key.Mods
	if v.confirmClear {
		switch {
		case key.Key == ui.KeyEnter && mods == 0:
			v.clear()
		case key.Key == ui.KeyEscape && mods == 0:
			v.confirmClear = false
		}
		return true
	}
	if v.filterOpen {
		switch {
		case key.Key == ui.KeyEscape && mods == 0:
			v.filterOpen = false
		case key.Key == ui.KeyEnter && mods == 0:
			v.setFilter(clipFilters[v.filterChosen].filter)
		case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl, key.Key == ui.KeyJ && mods == ui.Ctrl:
			v.filterChosen = min(v.filterChosen+1, len(clipFilters)-1)
		case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl, key.Key == ui.KeyK && mods == ui.Ctrl:
			v.filterChosen = max(v.filterChosen-1, 0)
		default:
			return false
		}
		return true
	}
	switch {
	case key.Key == ui.KeyEscape && mods == 0:
		switch {
		case v.query != "":
			key.SetText("")
			bound, v.query = "", ""
			v.queryChanged()
		case v.fromSearch:
			v.l.leaveClip()
		default:
			v.l.hide()
		}
		return true
	case key.Key == ui.KeyEnter && mods == 0:
		v.primary()
		return true
	case key.Key == ui.KeyEnter && mods == ui.Cmd:
		if e, ok := v.current(); ok {
			v.run(actCopy, e)
		}
		return true
	case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl, key.Key == ui.KeyJ && mods == ui.Ctrl:
		v.move(1)
		return true
	case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl, key.Key == ui.KeyK && mods == ui.Ctrl:
		v.move(-1)
		return true
	}
	return false
}

func (v *clipView) move(step int) {
	if len(v.rows) == 0 {
		return
	}
	v.choose(moveChoice(v.rows, v.chosen, step))
}

// --- The list (design.md 2.8.4) ---

func (v *clipView) listPane(c *ui.Context, k tokens) {
	v.list.Selected = &v.chosen
	v.list.Key = func(i int) any {
		if r := v.rows[i]; r.title {
			return r.section
		}
		return v.entries[v.rows[i].entry].ID
	}
	v.list.Label = func(i int) string {
		if r := v.rows[i]; r.title {
			return locales.ClipHeading(sectionTitle(r.section))
		}
		return v.rowLabel(v.entries[v.rows[i].entry])
	}
	var list ui.Element
	ui.Themed(c, listTheme(c, k)).Background(ui.Transparent).Grow(1).Children(func() {
		list = ui.List(c, &v.list, len(v.rows), func(i int) { v.row(c, k, i) }).Grow(1).
			Padding(0, listInset, clipBottom, listInset).Label(locales.ClipboardHistory())
	})
	if v.scroll && v.chosen >= 0 && v.chosen < len(v.rows) {
		v.scroll = false
		first, last := v.list.Visible()
		v.list.ScrollIntoView(revealRow(v.rows, v.chosen, first, last))
	}
	if list.Changed() {
		v.choose(v.chosen) // off a section's title
		v.scroll = false
		v.input.Focus()
	}
	if list.Submitted() {
		v.primary() // a double click
	}
}

// rowLabel is what assistive technology reads of an item's row.
func (v *clipView) rowLabel(e clipboard.Entry) string {
	return clipLabel(e, sysfmt.Date(e.Time, time.Now(), mygo.App.Locale()))
}

func (v *clipView) row(c *ui.Context, k tokens, i int) {
	r := v.rows[i]
	if r.title {
		listSection(c, k, sectionTitle(r.section), locales.ClipHeading(sectionTitle(r.section)))
		return
	}
	e := v.entries[r.entry]
	listRow(c, k, false, func() {
		v.rowIcon(c, k, e)
		t := listTitle(c, k, clipTitle(e)).MaxWidth(clipTitleMax)
		if e.Kind == clipboard.KindLink {
			t.TruncateMiddle()
		}
	}).Label(v.rowLabel(e))
}

// rowIcon is an item's icon: its kind's symbol, an image's thumbnail, or
// a file's icon.
func (v *clipView) rowIcon(c *ui.Context, k tokens, e clipboard.Entry) {
	switch e.Kind {
	case clipboard.KindImage:
		if b := v.thumb(e); b != nil {
			w, h := fitSize(float32(e.Width), float32(e.Height), iconCell, iconCell)
			listIcon(c, func() {
				ui.Image(c, b).Size(w, h).Fit(ui.Contain).Radius(4).Border(1, k.hairline).AccessHidden(true)
			})
			return
		}
	case clipboard.KindFiles:
		if len(e.Files) > 0 {
			if b := v.fileIcon(e.Files[0], iconCell); b != nil {
				listIcon(c, func() { ui.Image(c, b).Size(iconCell, iconCell).AccessHidden(true) })
				return
			}
		}
	}
	listSymbol(c, k, clipSymbol(e.Kind))
}

// fitSize is w by h scaled to fit in maxW by maxH, keeping its shape.
func fitSize(w, h, maxW, maxH float32) (float32, float32) {
	if w <= 0 || h <= 0 {
		return maxW, maxH
	}
	s := min(maxW/w, maxH/h)
	return w * s, h * s
}

// thumb is an image's thumbnail, read once.
func (v *clipView) thumb(e clipboard.Entry) *ui.Bitmap {
	if b, ok := v.thumbs[e.ID]; ok {
		return b
	}
	var b *ui.Bitmap
	if len(e.Thumbnail) > 0 {
		b, _ = ui.DecodeBitmap(e.Thumbnail)
	}
	v.thumbs[e.ID] = b
	return b
}

// fileIcon is the icon of the file or app at path, size DIPs, drawn for
// the sharpest display, and kept.
func (v *clipView) fileIcon(path string, size int) *ui.Bitmap {
	key := path + "\x00" + string(rune(size))
	if b, ok := v.icons[key]; ok {
		return b
	}
	scale := 1.0
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, d.ScaleFactor)
	}
	var b *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(path, int(float64(size)*scale)); err == nil {
		b, _ = ui.DecodeBitmap(data)
	}
	v.icons[key] = b
	return b
}

// genericAppIcon is the system's icon for an application with none of
// its own, size by size (an app not installed: design.md 4.4), or nil.
func (v *clipView) genericAppIcon(size int) *ui.Bitmap {
	key := genericAppIconKey(size)
	if b, ok := v.icons[key]; ok {
		return b
	}
	scale := 1.0
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, d.ScaleFactor)
	}
	var b *ui.Bitmap
	if data, err := mygo.Shell.TypeIcon("app", int(float64(size)*scale)); err == nil {
		b, _ = ui.DecodeBitmap(data)
	}
	v.icons[key] = b
	return b
}

// genericAppIconKey is where genericAppIcon keeps its icon of size among
// the icons: no path starts with a NUL.
func genericAppIconKey(size int) string { return "\x00app\x00" + string(rune(size)) }

// appPath is where the app with the bundle ID is: among the apps the
// search panel found, else running.
func (v *clipView) appPath(bundleID string) string {
	return appPathOf(v.l, bundleID)
}

func appPathOf(l *launcher, bundleID string) string {
	if bundleID == "" {
		return ""
	}
	for _, a := range l.apps {
		if a.ID == bundleID {
			return a.Path
		}
	}
	return running.AppPath(bundleID)
}

// --- The preview and its info (design.md 2.8.5) ---

func (v *clipView) previewPane(c *ui.Context, k tokens) {
	e, ok := v.current()
	if !ok {
		return
	}
	infos := clipInfos(e, time.Now(), mygo.App.Locale(), filesSize)
	ui.Column(c).Grow(1).Padding(0, clipPanePad, clipBottom, clipPanePad).Children(func() {
		c.Key(e.ID) // a new item's preview starts at its top
		box := ui.Column(c).Grow(1).Shrink(1).Padding(16, 0).Role(ui.RoleGroup).Label(locales.ClipPreview())
		if b := box.Bounds(); b.W > 0 {
			v.pane = b
		}
		box.Children(func() { v.preview(c, k, e) })
		ui.Box(c).Height(1).Shrink(0).Background(k.hairline).AccessHidden(true)
		for i, in := range infos {
			if i > 0 {
				ui.Box(c).Height(1).Shrink(0).Background(k.hairline).AccessHidden(true)
			}
			h := float32(clipInfoRow)
			if i > 0 {
				h--
			}
			ui.Row(c).Height(h).Shrink(0).Gap(16).AlignItems(ui.Center).Label(locales.ClipInfoLabel(in.label, in.value)).Children(func() {
				ui.Text(c, in.label).FontSize(12).TextColor(k.textOnGlass).SingleLine().Shrink(0).AccessHidden(true)
				ui.Row(c).Grow(1).Shrink(1).Gap(6).Justify(ui.End).AlignItems(ui.Center).AccessHidden(true).Children(func() {
					if in.source && e.App.Name != "" {
						if path := v.appPath(e.App.BundleID); path != "" {
							if b := v.fileIcon(path, 14); b != nil {
								ui.Image(c, b).Size(14, 14).Shrink(0)
							}
						}
					}
					t := ui.Text(c, in.value).FontSize(12).TextColor(k.text).SingleLine().Shrink(1)
					if e.Kind == clipboard.KindFiles && in.label == locales.ClipInfoLocation() {
						t.TruncateMiddle()
					}
				})
			})
		}
	})
}

func (v *clipView) preview(c *ui.Context, k tokens, e clipboard.Entry) {
	switch e.Kind {
	case clipboard.KindLink:
		ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(12).Children(func() {
			sfsymbols.Symbol(c, "link").Size(24).Color(k.textOnGlass)
			ui.Text(c, strings.TrimSpace(e.Text)).FontSize(13).TextColor(k.text).TextAlign(ui.Center).MaxLines(5).Selectable()
		})
	case clipboard.KindImage:
		v.imagePreview(c, k, e)
	case clipboard.KindFiles:
		v.filesPreview(c, k, e)
	default:
		text, cut := previewText(e.Text)
		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
			ui.Text(c, text).Font("monospace").FontSize(12).FixedLineHeight(18).TextColor(k.text).PreserveSpaces().Selectable()
			if cut {
				ui.Text(c, "…").Font("monospace").FontSize(12).FixedLineHeight(18).TextColor(k.textOnGlass)
			}
		})
	}
}

// imagePreview shows the image whole, scaled down to fit and never up,
// read on a goroutine; its thumbnail meanwhile, and a spinner once it
// takes long.
func (v *clipView) imagePreview(c *ui.Context, k tokens, e clipboard.Entry) {
	if v.image.id != e.ID {
		v.loadImage(e.ID)
	}
	b := v.image.bitmap
	if b == nil {
		b = v.thumb(e)
	}
	scale := float32(1)
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, float32(d.ScaleFactor))
	}
	boxW, boxH := v.pane.W, v.pane.H-32
	if boxW <= 0 {
		// Not laid out yet: the pane under no banner, four lines of info.
		boxW, boxH = clipPaneWidth-2*clipPanePad, listHeight-clipBottom-4*clipInfoRow-1-32
	}
	w0, h0 := float32(e.Width)/scale, float32(e.Height)/scale
	w, h := fitSize(w0, h0, min(boxW, w0), min(boxH, h0))
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(w, h).Shrink(0).Children(func() {
			if b != nil {
				ui.Image(c, b).Size(w, h).Fit(ui.Contain).Radius(6).Border(1, k.hairline).Label(clipTitle(e))
			}
			if !v.image.done && time.Since(v.image.started) >= imageSpinAfter {
				ui.Box(c).Absolute().Left(0).Top(0).Size(w, h).Center().Children(func() {
					ui.Spinner(c).Size(14, 14)
				})
			} else if !v.image.done {
				c.After(imageSpinAfter)
			}
		})
	})
}

// loadImage reads the image of the item id on a goroutine.
func (v *clipView) loadImage(id string) {
	v.image = clipImage{id: id, started: time.Now()}
	st := v.svc().store
	if st == nil {
		v.image.done = true
		return
	}
	go func() {
		var b *ui.Bitmap
		if content, err := st.Content(id); err == nil && len(content.PNG) > 0 {
			b, err = ui.DecodeBitmap(content.PNG)
			if err != nil {
				log.Printf("clipboard: image: %v", err)
			}
		}
		v.l.win.Update(func() {
			if v.image.id == id {
				v.image.bitmap, v.image.done = b, true
			}
		})
	}()
}

func (v *clipView) filesPreview(c *ui.Context, k tokens, e clipboard.Entry) {
	if len(e.Files) == 1 {
		path := e.Files[0]
		gone := !fileExists(path)
		ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			if b := v.fileIcon(path, 64); b != nil {
				img := ui.Image(c, b).Size(64, 64).AccessHidden(true)
				if gone {
					img.Opacity(0.4)
				}
			}
			ui.Text(c, clipTitle(e)).FontSize(14).FontWeight(500).TextColor(k.text).TextAlign(ui.Center).
				MaxLines(2).Margin(12, 0, 0, 0).Selectable()
			if gone {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(8, 0, 0, 0).Children(func() {
					sfsymbols.Symbol(c, "exclamationmark.triangle.fill").Size(12).Color(k.warning)
					ui.Text(c, locales.ClipFileMissing()).FontSize(12).TextColor(k.warning)
				})
			}
		})
		return
	}
	ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
		for _, path := range e.Files {
			c.Key(path)
			gone := !fileExists(path)
			ui.Row(c).Height(24).Gap(8).AlignItems(ui.Center).Children(func() {
				if b := v.fileIcon(path, 16); b != nil {
					ui.Image(c, b).Size(16, 16).Shrink(0).AccessHidden(true)
				} else { // a file gone has no icon: keep the names in line
					sfsymbols.Symbol(c, "doc").Size(16).Color(k.textOnGlass).Shrink(0)
				}
				t := ui.Text(c, baseName(path)).FontSize(13).TextColor(k.text).SingleLine().Shrink(1)
				if gone {
					t.TextColor(k.textOnGlass).Strikethrough().Label(baseName(path) + listSep() + locales.ClipFileMissing())
				}
			})
		}
	})
}

func baseName(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// --- States (design.md 2.8.8) and banners (2.8.7) ---

func (v *clipView) emptyState(c *ui.Context, k tokens, state clipState) {
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Padding(0, 24, 44, 24).Children(func() {
		if state == clipLoading {
			ui.Spinner(c).Size(14, 14)
			return
		}
		symbol, color := "doc.on.clipboard", k.textOnGlass
		title, body := locales.ClipEmptyTitle(), locales.ClipEmptyBody()
		switch state {
		case clipOffNone:
			symbol, title, body = "pause.circle", locales.ClipOffTitle(), locales.ClipOffBody()
		case clipUnreadable:
			symbol, color = "exclamationmark.triangle", k.warning
			title, body = locales.ClipUnreadableTitle(), locales.ClipUnreadableBody()
		case clipNoResults:
			symbol, title, body = "magnifyingglass", "", locales.ClipNoResultsHint()
		case clipNoneOfKind:
			f := filterOf(v.filter)
			symbol, title, body = f.symbol, f.none(), ""
		}
		sfsymbols.Symbol(c, symbol).Size(28).Color(color).Margin(0, 0, 12, 0)
		if state == clipNoResults {
			// Only the query cut, as the search panel's (design.md 2.6).
			q := strings.TrimSpace(v.query)
			before, after := around(locales.SearchNoResults(queryMark), queryMark)
			ui.Row(c).MaxWidth(clipWidth - 96).Label(locales.SearchNoResults(q)).Children(func() {
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
		} else {
			ui.Text(c, title).FontSize(14).FontWeight(500).TextColor(k.text).TextAlign(ui.Center)
		}
		if body != "" {
			ui.Text(c, body).FontSize(13).TextColor(k.textOnGlass).TextAlign(ui.Center).MaxWidth(520).Margin(8, 0, 0, 0)
		}
		if state == clipOffNone {
			clipButton(c, k, locales.ClipTurnOn(), true, func() { v.svc().setRecording(true) }).Margin(16, 0, 0, 0)
		}
	})
}

// clipButton is a button of the banners and the confirmation (design.md
// 2.8.7): 28 high, the accent's for the main one, else keycap gray.
func clipButton(c *ui.Context, k tokens, label string, primary bool, act func()) ui.Element {
	b := ui.Button(c, label).Height(28).Radius(6).Padding(0, 12).FontSize(13).OnClick(act)
	if primary {
		b.Background(c.Theme().Accent).TextColor(ui.Hex("#FFFFFF"))
	} else {
		b.Background(k.keycap).TextColor(k.text)
	}
	return b
}

func (v *clipView) banner(c *ui.Context, k tokens, which int) {
	if which == bannerNone {
		return
	}
	fade := &ui.Motion{Opacity: 0}
	tr := ui.ElementTransition{Exit: fade, ExitDuration: 150 * time.Millisecond}
	if c.Preferences().ReduceMotion {
		tr = ui.ElementTransition{}
	}
	c.Key("banner")
	block := ui.Row(c).Margin(8, 8, 8, 8).Radius(12).Background(k.keycap).Padding(12, 14).Gap(10).Shrink(0).Transition(tr)
	if which == bannerReady {
		block.Height(44).Padding(0, 14).AlignItems(ui.Center).Role(ui.RoleStatus).Children(func() {
			sfsymbols.Symbol(c, "checkmark.circle.fill").Size(16).Color(k.success)
			ui.Text(c, locales.ClipAXReady()).FontSize(13).TextColor(k.text)
		})
		return
	}
	symbol, title, body := "hand.raised.fill", locales.ClipAXTitle(), locales.ClipAXBody()
	if which == bannerOff {
		symbol, title, body = "pause.circle.fill", locales.ClipOffTitle(), locales.ClipOffBody()
	}
	block.AlignItems(ui.Center).Role(ui.RoleGroup).Label(title).Children(func() {
		ui.Box(c).AlignSelf(ui.Start).Children(func() {
			sfsymbols.Symbol(c, symbol).Size(16).Color(k.textOnGlass)
		})
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Margin(0, 6, 0, 0).Children(func() {
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
			ui.Text(c, body).FontSize(12).TextColor(k.textOnGlass)
		})
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			if which == bannerOff {
				clipButton(c, k, locales.ClipTurnOn(), true, func() { v.svc().setRecording(true) })
				return
			}
			clipButton(c, k, locales.ClipJustCopy(), false, func() { v.svc().setPasteOnReturn(false) })
			clipButton(c, k, locales.ClipOpenSettings(), true, v.svc().openAccessibility)
		})
	})
}

// --- The capsules (design.md 2.8.6) ---

func (v *clipView) footer(c *ui.Context, k tokens, state clipState) {
	_, ok := v.current()
	ok = ok && state == clipShowList
	var segs []segment
	if ok {
		segs = append(segs, segment{label: v.primaryLabel(c), color: k.text, key: ui.KeyEnter, caps: []string{"↩"}, act: v.primary})
	} else if state == clipOffNone {
		segs = append(segs, segment{label: locales.ClipTurnOn(), color: k.text, key: ui.KeyEnter, caps: []string{"↩"}, act: v.primary})
	}
	segs = append(segs, segment{label: locales.FooterActions(), color: k.textOnGlass, mods: ui.Cmd, key: ui.KeyK,
		caps: []string{"⌘", "K"}, dim: !ok, act: v.toggleMenu})
	if v.notice != "" {
		n := notice{icon: "checkmark.circle", iconColor: k.textOnGlass, color: k.text, text: v.notice}
		if v.noticeDanger {
			n.icon, n.iconColor, n.color = "exclamationmark.triangle.fill", k.danger, k.danger
		}
		noticeCapsule(c, k, capsuleInset, capsuleInset, clipNoticeMax, n)
	}
	actionsCapsule(c, k, capsuleInset, capsuleInset, segs...)
}

// primaryLabel names what Return does (copy.md 6.5): pasting into the app,
// its name cut to 160 inside the quotes, or copying.
func (v *clipView) primaryLabel(c *ui.Context) string {
	if !v.canPaste() {
		return locales.ClipCopy()
	}
	name := fitText(v.target.app.Name, clipAppNameMax, func(s string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: 13})
		return w
	})
	return v.target.label(name)
}

// say shows a notice at the bottom left for a few seconds, read out.
func (v *clipView) say(text string, danger bool) {
	v.notice, v.noticeDanger, v.announce = text, danger, text
	v.noticeGen++
	gen := v.noticeGen
	time.AfterFunc(noticeFor, func() {
		v.l.win.Update(func() {
			if v.noticeGen == gen {
				v.notice = ""
			}
		})
	})
	v.l.win.Invalidate()
}

// --- The type filter (design.md 2.8.2) ---

func (v *clipView) toggleFilter() {
	if v.filterOpen {
		v.filterOpen = false
	} else if !v.confirmClear {
		v.menuOpen = false
		v.filterOpen = true
		for i, f := range clipFilters {
			if f.filter == v.filter {
				v.filterChosen = i
			}
		}
		v.input.Focus()
	}
	v.l.win.Invalidate()
}

func (v *clipView) setFilter(f clipboard.Filter) {
	v.filterOpen = false
	v.filter = f
	v.chosenID = ""
	v.refresh()
	v.l.win.Invalidate()
}

func (v *clipView) filterCard(c *ui.Context, k tokens) {
	c.Key("filterCard")
	top := float32(barHeight-28)/2 + 28 + 6
	popupMenu(c, k, 1, 0).Width(clipFilterW).Right(capsuleInset).Top(top).
		Role(ui.RoleMenu).Label(locales.ClipTypeLabel(filterOf(v.filter).label())).Children(func() {
		ui.Column(c).Padding(menuPad).Children(func() {
			for i, f := range clipFilters {
				on := f.filter == v.filter
				bg := ui.Transparent
				if i == v.filterChosen {
					bg = k.selection
				}
				filter := f.filter
				c.Key(i)
				ui.Row(c).Height(menuRowHeight).Padding(0, menuRowPadR, 0, menuRowPad).Radius(8).Gap(8).AlignItems(ui.Center).
					Background(bg).Role(ui.RoleMenuItemRadio).Checked(on).Label(f.label()).OnClick(func() { v.setFilter(filter) }).
					Children(func() {
						sfsymbols.Symbol(c, f.symbol).Size(16).Color(k.text)
						ui.Text(c, f.label()).FontSize(13).TextColor(k.text).SingleLine().Grow(1).Shrink(1)
						if on {
							sfsymbols.Symbol(c, "checkmark").Size(12).Color(k.text)
						}
					})
			}
		})
	})
}

// --- The actions card (Cmd-K, design.md 2.8.6) ---

func (v *clipView) toggleMenu() {
	if v.menuOpen {
		v.closeMenu()
		return
	}
	e, ok := v.current()
	if !ok || v.state() != clipShowList || v.confirmClear {
		return
	}
	v.filterOpen = false
	v.menuOpen = true
	v.menuQuery = ""
	v.menuChosen = 0
	v.menuItems = clipActions(e, v.canPaste(), v.target)
	v.filterMenu()
	v.menuInput.Focus()
	v.l.win.Invalidate()
}

func (v *clipView) closeMenu() {
	v.menuOpen = false
	v.input.Focus()
	v.l.win.Invalidate()
}

func (v *clipView) filterMenu() {
	v.menuShown = v.menuShown[:0]
	for i, a := range v.menuItems {
		if search.NewKey(a.label).Rank(v.menuQuery) >= 0 {
			v.menuShown = append(v.menuShown, i)
		}
	}
	v.menuChosen = max(0, min(v.menuChosen, len(v.menuShown)-1))
}

func (v *clipView) runMenu(i int) {
	if i < 0 || i >= len(v.menuShown) {
		return
	}
	a := v.menuItems[v.menuShown[i]]
	v.menuOpen = false
	v.input.Focus()
	if e, ok := v.current(); ok {
		v.run(a.id, e)
	}
}

func (v *clipView) menuKey(key ui.TextKey) bool {
	if v.menuComposing {
		return false
	}
	if q := key.Text(); q != v.menuQuery {
		bound := v.menuQuery
		v.menuQuery = q
		v.filterMenu()
		defer func() { v.menuQuery = bound }()
	}
	mods := key.Mods
	switch {
	case key.Key == ui.KeyEscape && mods == 0:
		v.closeMenu()
	case key.Key == ui.KeyEnter && mods == 0:
		v.runMenu(v.menuChosen)
	case key.Key == ui.KeyEnter && mods == ui.Cmd:
		v.closeMenu()
		if e, ok := v.current(); ok {
			v.run(actCopy, e)
		}
	case key.Key == ui.KeyDown && mods == 0, key.Key == ui.KeyN && mods == ui.Ctrl, key.Key == ui.KeyJ && mods == ui.Ctrl:
		v.menuChosen = min(v.menuChosen+1, len(v.menuShown)-1)
	case key.Key == ui.KeyUp && mods == 0, key.Key == ui.KeyP && mods == ui.Ctrl, key.Key == ui.KeyK && mods == ui.Ctrl:
		v.menuChosen = max(v.menuChosen-1, 0)
	default:
		return false
	}
	return true
}

func (v *clipView) actionsCard(c *ui.Context, k tokens) {
	c.Key("actionsMenu")
	popupMenu(c, k, 1, 1).Right(capsuleInset).Bottom(capsuleInset + capsuleHeight + 8).Children(func() {
		menuField(c, k, func() {
			f := ui.TextInputBase(c, &v.menuQuery).Bind(&v.menuInput).Grow(1).FontSize(13).
				Placeholder(locales.ActionsPlaceholder()).Label(locales.ActionsPlaceholder()).
				OnTextKey(v.menuKey)
			v.menuComposing = f.Composing()
			if f.Changed() {
				v.filterMenu()
			}
		})
		menuItems(c, func() {
			if len(v.menuShown) == 0 {
				ui.Text(c, locales.ActionsNoMatch()).FontSize(13).TextColor(k.textOnGlass).Padding(8)
			}
			filtering := strings.TrimSpace(v.menuQuery) != ""
			for i, ai := range v.menuShown {
				a := v.menuItems[ai]
				if a.divided && !filtering && i > 0 {
					menuSeparator(c, k)
				}
				idx := i
				clipMenuRow(c, k, a, i == v.menuChosen, func() { v.runMenu(idx) })
			}
		})
	})
}

// clipMenuRow is a row of the actions card: as menuRow, in the danger
// color for deleting and clearing.
func clipMenuRow(c *ui.Context, k tokens, a clipAction, chosen bool, act func()) {
	if !a.danger {
		menuRow(c, k, a.symbol, a.label, "", a.caps, chosen, act)
		return
	}
	bg := ui.Transparent
	if chosen {
		bg = k.selection
	}
	c.Key(a.label)
	ui.Row(c).Height(menuRowHeight).Padding(0, menuRowPadR, 0, menuRowPad).Radius(8).Gap(8).AlignItems(ui.Center).Background(bg).
		Role(ui.RoleMenuItem).Label(a.label).OnClick(act).Children(func() {
		sfsymbols.Symbol(c, a.symbol).Size(16).Color(k.danger)
		ui.Text(c, a.label).FontSize(13).TextColor(k.danger).SingleLine().Shrink(1).Grow(1).Margin(0, menuNameGap-8, 0, 0)
		if len(a.caps) > 0 {
			keycaps(c, k, a.caps, 12)
		}
	})
}

// --- Acting ---

// primary is Return: pasting, or copying (design.md 2.8.6, 2.8.7); with
// recording off and no history, turning it on.
func (v *clipView) primary() {
	state := v.state()
	if state == clipOffNone {
		v.svc().setRecording(true)
		return
	}
	e, ok := v.current()
	if !ok || state != clipShowList {
		return
	}
	s := v.svc()
	switch {
	case v.canPaste():
		v.run(actPaste, e)
	case v.l.g.settings.PasteOnReturn() && !s.asked:
		// The first Return that would paste without the permission: copied,
		// the banner asks for it, the panel stays.
		if v.copyOnly(e) {
			s.asked = true
			v.say(locales.ClipCopiedPressCmdV(), false)
			v.announce = locales.ClipAXTitle()
		}
	default:
		v.run(actCopy, e)
	}
}

// copyOnly puts the item on the pasteboard, keeping the panel: false when
// it can't, which the notice says.
func (v *clipView) copyOnly(e clipboard.Entry) bool {
	if missingFile(e.Files) {
		v.say(locales.ClipFileMissing(), true)
		return false
	}
	if err := v.svc().copy(e.ID); err != nil {
		log.Printf("clipboard: copying: %v", err)
		return false
	}
	return true
}

// run takes an action on the item e.
func (v *clipView) run(id clipActionID, e clipboard.Entry) {
	s := v.svc()
	switch id {
	case actPaste, actPastePlain:
		if !v.canPaste() {
			v.run(actCopy, e)
			return
		}
		if err := s.paste(e.ID, id == actPastePlain, v.target); err != nil {
			if err == errFilesGone {
				v.say(locales.ClipFileMissing(), true)
			} else {
				log.Printf("clipboard: pasting: %v", err)
			}
			return
		}
		v.l.hide()
	case actCopy:
		if v.copyOnly(e) {
			v.l.hide()
		}
	case actOpenLink:
		if err := mygo.Shell.OpenExternal(strings.TrimSpace(e.Text)); err != nil {
			log.Printf("clipboard: opening the link: %v", err)
			return
		}
		v.l.hide()
	case actShowInFinder:
		if len(e.Files) > 0 {
			v.l.hide()
			mygo.Shell.ShowItemInFolder(e.Files[0])
		}
	case actPin:
		if s.store == nil {
			return
		}
		if err := s.store.SetPinned(e.ID, !e.Pinned); err != nil {
			log.Printf("clipboard: pinning: %v", err)
			return
		}
		v.chosenID = e.ID
		v.refresh()
		if e.Pinned {
			v.say(locales.ClipUnpinned(), false)
		} else {
			v.say(locales.ClipPinned(), false)
		}
	case actDelete:
		if s.store == nil {
			return
		}
		next := nextAfterDelete(v.rows, v.entries, v.chosen)
		if err := s.store.Delete(e.ID); err != nil {
			log.Printf("clipboard: deleting: %v", err)
			return
		}
		delete(v.thumbs, e.ID)
		v.chosenID = next
		v.refresh()
		v.say(locales.ClipDeleted(), false)
	case actClear:
		v.askClear()
	}
	v.l.win.Invalidate()
}

// --- Clearing (design.md 2.8.8) ---

func (v *clipView) askClear() {
	s := v.svc()
	if s.store == nil || s.store.Unreadable() || !s.store.CanClear() {
		return // in the view, unreadable history clears from Settings
	}
	v.menuOpen, v.filterOpen = false, false
	v.confirmClear = true
	v.input.Focus()
	v.l.win.Invalidate()
}

func (v *clipView) clear() {
	v.confirmClear = false
	v.svc().clear()
	v.chosenID = ""
	v.refresh()
	v.choose(freshChoice(v.rows, v.entries))
	v.l.win.Invalidate()
}

func (v *clipView) confirmation(c *ui.Context, k tokens) {
	c.Key("confirmClear")
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Background(k.scrim).Center().
		OnClick(func() {}).Children(func() {
		ui.Column(c).Width(clipConfirmW).Padding(20).Radius(12).Material(glassOf(k, searchTint)).
			Border(1, k.capsuleBorder).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.20)).Role(ui.RoleAlertDialog).Label(locales.ClipClearTitle()).Children(func() {
			ui.Text(c, locales.ClipClearTitle()).FontSize(14).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
			ui.Text(c, locales.ClipClearBody()).FontSize(13).TextColor(k.textOnGlass).Margin(6, 0, 0, 0)
			ui.Row(c).Gap(8).Justify(ui.End).Margin(16, 0, 0, 0).Children(func() {
				clipButton(c, k, locales.Cancel(), false, func() { v.confirmClear = false })
				clipButton(c, k, locales.ClipClearConfirm(), false, v.clear).Background(k.danger).TextColor(ui.Hex("#FFFFFF"))
			})
		})
	})
}
