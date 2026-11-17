package app

// The notes window's view (design.md 11.2 to 11.8): the top bar, the find
// bar, the banner of a note not saved, the editor or the empty state, the
// notices, and the cards over them.

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"gocast/locales"
)

// The view's measures (design.md 11.3, 11.4), in DIPs.
const (
	noteButtonSize = 28
	noteTitleInset = 76 // the title's room from either side: the buttons'
	noteTextMax    = 680
	notePadX       = 24
	notePadTop     = 4
	notePadBottom  = 24
	findBarHeight  = 40
	findButtonSize = 24
	noteNoticeIn   = 8
)

// view draws the window.
func (n *notesWindow) view(c *ui.Context) {
	c.SetTheme(themed(c))
	k := tokensFor(c)
	c.Root().Background(ui.Transparent)
	n.services = c.Services()
	n.follow(c, k)
	n.shortcuts(c)
	if d := n.cur; d != nil {
		n.snapshot(d.doc)
		n.findAll()
	}
	w, h := c.Size()
	root := ui.Column(c).Fill().Background(glassFill(c, k, terminalTint)).ObserveInput(n.observe)
	panelOutline(root, k).Children(func() {
		n.topBar(c, k)
		if n.find.open && n.cur != nil {
			n.findBar(c, k)
		}
		if n.cur != nil && n.cur.failed {
			n.banner(c, k)
		}
		if n.cur == nil {
			n.emptyState(c, k)
		} else {
			n.editorArea(c, k)
		}
		if nt := n.notice; nt != nil {
			shown := *nt
			shown.iconColor, shown.color = k.textOnGlass, k.text
			noticeCapsule(c, k, noteNoticeIn, noteNoticeIn, w-2*noteNoticeIn, shown)
		}
		if n.browse.open {
			n.browseCard(c, k, w, h)
		}
		if n.menu.open {
			n.menuCard(c, k)
		}
	})
	if n.say != "" {
		c.Announce(n.say)
		n.say = ""
	}
}

// follow follows what the text is styled by: the appearance, the accent
// and the zoom; the text is laid out again as they change.
func (n *notesWindow) follow(c *ui.Context, k tokens) {
	dark := c.Theme().Dark
	base := ui.LightTheme()
	if dark {
		base = ui.DarkTheme()
	}
	d := noteDesign{zoom: float32(n.g.settings.Zoom()) / 100, k: k, dark: dark, accent: base.WithPreferences(c.Preferences()).Accent}
	if d != n.design {
		n.design = d
		n.spansVer++
	}
}

// shortcuts are the window's keys where the editor has not the focus (it
// takes them itself, in onKey), Esc, and Return in the empty state.
func (n *notesWindow) shortcuts(c *ui.Context) {
	for _, wk := range windowKeys {
		if c.Shortcut(wk.mods, wk.key) {
			n.command(wk.key, wk.mods)
		}
	}
	if c.Shortcut(0, ui.KeyEscape) {
		n.escape()
	}
	if n.cur == nil && !n.browse.open && !n.menu.open && c.Shortcut(0, ui.KeyEnter) {
		n.newNote()
	}
}

// noteTitle is a note's title as the top bar shows it: its first line,
// without a rule, trimmed (copy.md 16.2: its marks are not in the text).
func noteTitle(t richtext.Text) string {
	_, end := t.ParagraphRange(0)
	s := strings.ReplaceAll(t.Slice(0, end).String(), string(richtext.ObjectReplacement), "")
	return strings.TrimSpace(s)
}

// title is the top bar's: the note's, Untitled for none, nothing in the
// empty state.
func (n *notesWindow) title() string {
	if n.cur == nil {
		return ""
	}
	if t := noteTitle(n.cur.doc.Text()); t != "" {
		return t
	}
	return locales.NotesUntitled()
}

// topBar draws the top bar (design.md 11.3): the title in the middle of
// the window, Browse and New at the right; a handle that moves the
// window, which takes the tint and a hairline while the text is scrolled
// under it.
func (n *notesWindow) topBar(c *ui.Context, k tokens) {
	c.Key("topBar")
	bar := ui.Box(c).Height(notesBarHeight).Shrink(0).DragWindow()
	if n.scrolled {
		bar.Background(glassFill(c, k, terminalTint))
	}
	bar.Children(func() {
		pinned := n.cur != nil && n.cur.id != "" && n.store.Pinned(n.cur.id)
		ui.Row(c).Absolute().Left(noteTitleInset).Right(noteTitleInset).Top(0).Bottom(0).
			Justify(ui.Center).AlignItems(ui.Center).Gap(4).Children(func() {
			if pinned {
				sfsymbols.Symbol(c, "pin.fill").Size(10).Color(k.textOnGlass).Tooltip(locales.NotesPinnedTip()).Label(locales.NotesPinnedTip())
			}
			ui.Text(c, n.title()).FontSize(13).FontWeight(500).TextColor(k.textOnGlass).SingleLine().Shrink(1).Role(ui.RoleHeading)
		})
		ui.Row(c).Absolute().Right(10).Top((notesBarHeight - noteButtonSize) / 2).Gap(4).Children(func() {
			browse := noteButton(c, k, "list.bullet.rectangle", locales.NotesBrowseTip(), n.browse.open).
				HasPopup(ui.PopupDialog).Expanded(n.browse.open)
			if n.cur == nil {
				browse.Opacity(0.4).AccessDisabled(true) // nothing to browse (design.md 11.8)
			} else {
				browse.OnClick(func() { n.command(ui.KeyP, ui.Cmd) })
			}
			noteButton(c, k, "plus", locales.NotesNewTip(), false).OnClick(func() { n.command(ui.KeyN, ui.Cmd) })
		})
		if n.scrolled {
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(k.hairline).AccessHidden(true)
		}
	})
}

// noteButton is roundButton at the top bar's 28 (design.md 11.3), or the
// find bar's 24 with a symbol of 12.
func noteButton(c *ui.Context, k tokens, symbol, tip string, on bool) ui.Element {
	return roundButtonSized(c, k, symbol, tip, on, noteButtonSize, 14)
}

func roundButtonSized(c *ui.Context, k tokens, symbol, tip string, on bool, size, symbolSize float32) ui.Element {
	b := ui.Box(c).Size(size, size).Radius(size / 2).Center().Shrink(0).
		Role(ui.RoleButton).Label(tip).Tooltip(tip)
	if on || b.Pressed() {
		b.Background(k.selection)
	}
	b.Children(func() { sfsymbols.Symbol(c, symbol).Size(symbolSize).Color(k.textOnGlass) })
	return b
}

// editorArea draws the note's text (design.md 11.4): 24 from the sides,
// at most 680 wide in the middle. The editor goes from edge to edge, the
// room around the text inside it (its theme's Padding), for the text to
// scroll under the top bar and the scroll bar to be at the window's
// edge; only the bottom's room is outside it, as Padding's is the same
// above and below.
func (n *notesWindow) editorArea(c *ui.Context, k tokens) {
	d := n.cur
	// The window's outline insets what is in it by its width: the text is
	// 24 from the window's edge, not the outline's.
	w, _ := c.Size()
	th := n.design.theme()
	th.Padding = [2]float32{max(notePadX-panelBorderWidth, (w-2*panelBorderWidth-noteTextMax)/2), notePadTop}
	ui.Column(c).Grow(1).Shrink(1).Padding(0, 0, notePadBottom-notePadTop, 0).Children(func() {
		opts := richtext.EditorOptions{
			Label:        locales.NotesEditorLabel(n.title()),
			Placeholder:  locales.NotesPlaceholder(),
			Font:         n.design.font(),
			Theme:        th,
			OnKey:        n.onKey,
			OnCommand:    n.onCommand,
			OnLink:       n.openLink,
			Spans:        n.spans,
			SpansVersion: n.spansVer,
			Space:        n.space,
		}
		c.Key(d) // a view a note: each keeps its scroll
		// Editor, for Padding, without its field's look.
		e := richtext.Editor(c, d.doc, opts).Bind(&n.editor).Grow(1).
			Background(ui.Transparent).Border(0, ui.Transparent).Radius(0).MinHeight(0)
		if n.focusText && !n.browse.open && !n.menu.open && !n.find.focus {
			n.focusText = false
			e.Focus(ui.FocusOptions{PreventScroll: true})
		}
		if v := d.doc.Version(); v != n.seenVer || d != n.seenDoc {
			if d == n.seenDoc {
				if e.Changed() {
					n.afterEdit(d.doc) // typed: its shortcuts, then saved soon
				} else {
					n.edited() // a format's key, undo: saved soon
				}
			}
			n.seenDoc, n.seenVer = d, d.doc.Version()
		}
		vis := d.doc.VisibleParagraphs()
		n.scrolled = len(vis) > 0 && (vis[0].Index > 0 || vis[0].Top < notePadTop-0.5)
	})
}

// onCommand takes the menus' Paste over a selection of a URL on the
// clipboard: the selection becomes a link to it (design.md 11.9), as ⌘V
// does where the menu bar does not take it.
func (n *notesWindow) onCommand(name string) bool {
	if name != "paste" || n.cur == nil {
		return false
	}
	if fn := n.formatKey(n.cur.doc, ui.KeyV, ui.Cmd); fn != nil {
		fn()
		return true
	}
	return false
}

// findBar draws the find bar (design.md 11.4): the field, the count, the
// buttons Previous, Next and Close.
func (n *notesWindow) findBar(c *ui.Context, k tokens) {
	f := &n.find
	c.Key("findBar")
	ui.Row(c).Height(findBarHeight).Shrink(0).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Height(26).Radius(7).Background(k.keycap).Grow(1).Shrink(1).Padding(0, 8).Gap(6).AlignItems(ui.Center).Children(func() {
			sfsymbols.Symbol(c, "magnifyingglass").Size(12).Color(k.textOnGlass)
			in := ui.TextInputBase(c, &f.query).Bind(&f.input).Grow(1).FontSize(13).
				Placeholder(locales.NotesFindPlaceholder()).Label(locales.NotesFindPlaceholder()).
				OnTextKey(n.findKey)
			f.composing = in.Composing()
			if in.Changed() {
				f.match = 0
				n.findAll()
				n.showMatch()
			}
			if f.focus {
				f.focus = false
				in.Focus()
			}
		})
		if f.query != "" {
			if len(f.matches) == 0 {
				ui.Text(c, locales.NotesFindNone()).FontSize(12).TextColor(k.danger).SingleLine().Shrink(0).Role(ui.RoleStatus)
			} else {
				i, all := strconv.Itoa(f.match+1), strconv.Itoa(len(f.matches))
				ui.Text(c, locales.NotesFindCount(i, all)).FontSize(12).TextColor(k.textOnGlass).FontFeatures("tnum").
					SingleLine().Shrink(0).Label(locales.NotesFindCountSpoken(i, all))
			}
		}
		ui.Row(c).Gap(2).Shrink(0).Children(func() {
			findButton(c, k, "chevron.up", locales.NotesFindPrevious()).OnClick(func() { n.stepFind(-1) })
			findButton(c, k, "chevron.down", locales.NotesFindNext()).OnClick(func() { n.stepFind(1) })
		})
		findButton(c, k, "xmark", locales.NotesFindClose()).Margin(0, 0, 0, 4).OnClick(n.closeFind)
	})
}

func findButton(c *ui.Context, k tokens, symbol, tip string) ui.Element {
	return roundButtonSized(c, k, symbol, tip, false, findButtonSize, 12)
}

// findKey takes the find field's keys: Return the next match, Shift-Return
// the one before, Esc closes the bar; none while the input method
// composes.
func (n *notesWindow) findKey(key ui.TextKey) bool {
	if n.find.composing {
		return false
	}
	switch {
	case key.Key == ui.KeyEnter && key.Mods == 0:
		n.stepFind(1)
	case key.Key == ui.KeyEnter && key.Mods == ui.Shift:
		n.stepFind(-1)
	case key.Key == ui.KeyEscape && key.Mods == 0:
		n.closeFind()
	default:
		return false
	}
	return true
}

// banner draws the block of a note not saved (design.md 11.5).
func (n *notesWindow) banner(c *ui.Context, k tokens) {
	c.Key("saveFailed")
	ui.Row(c).Margin(4, 8, 0, 8).Radius(12).Background(k.keycap).Padding(10, 14).AlignItems(ui.Center).Shrink(0).
		Role(ui.RoleGroup).Label(locales.NotesSaveFailed()).Children(func() {
		sfsymbols.Symbol(c, "exclamationmark.triangle.fill").Size(16).Color(k.danger)
		ui.Text(c, locales.NotesSaveFailed()).FontSize(13).TextColor(k.text).Grow(1).Shrink(1).Margin(0, 16, 0, 10)
		clipButton(c, k, locales.NotesTryAgain(), true, n.retry).Shrink(0)
	})
}

// emptyState is the window with no notes, the last deleted (design.md
// 11.8).
func (n *notesWindow) emptyState(c *ui.Context, k tokens) {
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Padding(0, 24, notesBarHeight, 24).Children(func() {
		sfsymbols.Symbol(c, "note.text").Size(28).Color(k.textOnGlass).Margin(0, 0, 12, 0)
		ui.Text(c, locales.NotesEmptyTitle()).FontSize(14).FontWeight(500).TextColor(k.text).TextAlign(ui.Center)
		ui.Text(c, locales.NotesEmptyBody()).FontSize(13).TextColor(k.textOnGlass).TextAlign(ui.Center).Margin(8, 0, 0, 0)
		clipButton(c, k, locales.NotesNewItem(), true, n.newNote).Margin(16, 0, 0, 0)
	})
}
