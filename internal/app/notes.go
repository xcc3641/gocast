package app

// The notes window (design.md 11): a floating window, one note at a time,
// styled as it is written, saved as it pauses. notes.go is its state: the
// notes, which shows, saving them, showing and hiding the window; the
// view is in noteview.go, the editor's keys and find in noteedit.go, the
// cards (Browse Notes, Cmd-K) in notecards.go.

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"gocast/internal/notes"
	"gocast/internal/settings"
	"gocast/locales"
)

// The window's measures (design.md 11.2), in DIPs.
const (
	notesWidth, notesHeight       = 560, 440
	notesMinWidth, notesMinHeight = 360, 240
	notesTopShare                 = 0.20 // the top's place down the work area, as the search panel's
	// notesSaveDelay is how long after the last edit a note is written
	// (notes brief: about 300 ms).
	notesSaveDelay = 300 * time.Millisecond
)

// What the window opens to (design.md 11.1): the note shown last, a new
// note, or Browse Notes over the note shown last.
type notesOpen int

const (
	notesShow notesOpen = iota
	notesNew
	notesBrowse
)

var (
	// sysTrash moves a note to the system's Trash, whence Finder's Put
	// Back brings it back (copy.md 16.6). Tests move it to a folder of
	// theirs.
	sysTrash = func(path string) error { return mygo.Shell.TrashItem(path) }
	// sysConfirmQuitUnsaved asks before Gocast quits with a note not
	// saved (copy.md 16.7): the system's alert, Cancel the default; true
	// quits.
	sysConfirmQuitUnsaved = func() bool {
		mygo.App.Focus()
		r, err := mygo.Dialog.Message(mygo.MessageOptions{
			Message:            locales.NotesQuitTitle(),
			Detail:             locales.NotesQuitDetail(),
			Buttons:            []string{locales.NotesQuitAnyway(), locales.Cancel()},
			DefaultButton:      1,
			CancelButton:       1,
			DestructiveButtons: []int{0},
		})
		return err == nil && r.Button == 0
	}
	// notesRead reads a note's file.
	notesRead = func(path string) (string, error) {
		data, err := os.ReadFile(path)
		return string(data), err
	}
	// sysNotesKey reports whether the notes window has the keyboard.
	sysNotesKey = func(n *notesWindow) bool { return n.win != nil && n.win.IsFocused() }
	// sysNotesBackground runs work off the main thread, then what it
	// returns on it, in the notes window's frame: the writing of a note.
	// Tests run both at once.
	sysNotesBackground = func(n *notesWindow, work func() func()) {
		go func() {
			done := work()
			n.post(done)
		}()
	}
)

// noteDoc is a note as the window edits it: one Document a note, which
// keeps its undo while another shows (design.md 11.4).
type noteDoc struct {
	// id is its file's name, "" for a new note not written yet: one with
	// no text is never written (design.md 11.1).
	id  string
	doc *richtext.Document
	// place is where it was left, nil until it was shown in this run.
	place *notePlace
	// saved is the version of doc written last (or read); writing the
	// version being written, failed the write that failed.
	saved, writing uint64
	failed         bool
	// again: edited while being written, to write again once written.
	again bool
	// stamp is when its file changed as last read or written: changed
	// since outside Gocast, it is read again.
	stamp time.Time
}

// notePlace is where a note was left: its selection, and the paragraph
// at the top of the view with how far below the view's top its lines
// start.
type notePlace struct {
	anchor, caret int
	para          int
	top           float32
}

// dirty reports whether the note has text not written.
func (d *noteDoc) dirty() bool {
	return d.doc.Version() != d.saved && !(d.id == "" && d.doc.Text().Len() == 0)
}

// notesWindow is the notes window.
type notesWindow struct {
	g     *gocast
	win   *mygo.Window // nil in tests
	store *notes.Store
	docs  map[string]*noteDoc // the notes opened in this run, by ID
	// cur is the note showing, nil for the empty state (no notes, the
	// last deleted: design.md 11.8).
	cur *noteDoc

	shown bool
	// services are the window's (the clipboard, links), as of its last
	// frame.
	services ui.Services
	now      func() time.Time
	// after runs fn after d on the main thread, unless stopped: the
	// timers of the save and the notice. Tests hold them.
	after func(d time.Duration, fn func()) (stop func())
	// saveStop stops the save waiting.
	saveStop func()

	editor    ui.Handle
	focusText bool // focus the editor in the next frame
	composing bool // the input method composes in the editor
	cmdDown   bool // Command held as the pointer went down: a link opens
	typed     string
	// What Spans and Space read of the text, as of version textVer.
	text    richtext.Text
	textVer uint64
	textDoc *richtext.Document
	styles  []richtext.ParagraphStyle
	rules   []bool
	paraAt  []int
	design  noteDesign
	// spansVer changes as the spans do (find, appearance, zoom), for the
	// editor to lay the text out again.
	spansVer int
	scrolled bool // the text is scrolled: the top bar's hairline shows
	// The note and its version the view saw last: a change is an edit.
	seenDoc *noteDoc
	seenVer uint64

	find    noteFind
	browse  noteBrowse
	menu    noteMenu
	notice  *notice // at the bottom left, nil for none
	noticeN int
	stopN   func()
	say     string // read out in the next frame
}

func newNotesWindow(g *gocast) *notesWindow {
	n := newNotesState(g, filepath.Join(g.dir, "Notes"))
	n.after = func(d time.Duration, fn func()) func() {
		t := time.AfterFunc(d, func() { n.post(fn) })
		return func() { t.Stop() }
	}
	n.win = mygo.NewWindow(mygo.WindowOptions{
		Title:        locales.NotesItem(),
		Panel:        true, // over every app, on every Space, as the floating terminal
		Hidden:       true,
		Frameless:    true,
		Transparent:  true,
		Vibrancy:     mygo.VibrancyGlass,
		CornerRadius: panelRadius,
		Width:        notesWidth,
		Height:       notesHeight,
		MinWidth:     notesMinWidth,
		MinHeight:    notesMinHeight,
		Content:      ui.View(n.view),
	})
	// Closing hides it, as Esc and Cmd-W do; it stays as Gocast goes
	// elsewhere (design.md 11.2).
	n.win.OnClose(func(e *mygo.CloseEvent) {
		if !g.quitting {
			e.PreventDefault()
			n.hide()
		}
	})
	mygo.App.OnBeforeQuit(n.quitting)
	return n
}

// newNotesState is the notes window's state over the notes in dir,
// without its window: newNotesWindow's, and the tests'.
func newNotesState(g *gocast, dir string) *notesWindow {
	n := &notesWindow{g: g, docs: map[string]*noteDoc{}, now: time.Now}
	n.store = notes.Open(dir)
	n.store.Trash = func(path string) error { return sysTrash(path) }
	n.after = func(time.Duration, func()) func() { return func() {} }
	return n
}

// post runs fn on the main thread, in the window's next frame.
func (n *notesWindow) post(fn func()) {
	if n.win != nil {
		n.win.Update(fn)
		return
	}
	fn()
}

// redraw draws the window again, for state changed outside a frame.
func (n *notesWindow) redraw() {
	if n.win != nil {
		n.win.Invalidate()
	}
}

// --- Showing ---

// open shows the window as the search panel's rows and the hotkeys do
// (design.md 11.1): the note shown last, a new note, or Browse Notes.
func (n *notesWindow) open(how notesOpen) {
	if n == nil {
		return
	}
	n.scan()
	switch how {
	case notesNew:
		n.newNote()
	case notesBrowse:
		if n.cur == nil {
			n.showLast()
		}
	default:
		if n.cur == nil || !n.shown {
			n.showLast()
		}
	}
	n.show()
	if how == notesBrowse {
		n.openBrowse()
	} else {
		n.closeCards()
	}
}

// hotkey is what a notes' hotkey does: Notes shows the window, or hides
// it in front; Search Notes too, Browse Notes open (design.md 11.1).
func (n *notesWindow) hotkey(item string) {
	if n == nil {
		return
	}
	front := n.shown && (n.win == nil || n.win.IsFocused())
	switch item {
	case settings.NoteOpen:
		if front {
			n.hide()
			return
		}
		n.open(notesShow)
	case settings.NoteNew:
		n.open(notesNew)
	case settings.NoteSearch:
		if front && n.browse.open {
			n.hide()
			return
		}
		n.open(notesBrowse)
	}
}

// show orders the window in at once (no animation, design.md 11.2), Gocast
// in front, the focus in the text.
func (n *notesWindow) show() {
	n.focusText = true
	if n.shown {
		if n.win != nil {
			n.win.Focus()
		}
		n.redraw()
		return
	}
	n.shown = true
	if n.win == nil {
		return
	}
	n.win.SetBounds(n.frame())
	n.win.Show()
	n.win.Focus()
	n.win.Invalidate()
}

// hide hides the window, the note written first, the cards closed: the
// keyboard goes back to the app before (design.md 11.2).
func (n *notesWindow) hide() {
	if !n.shown {
		return
	}
	n.leave()
	n.closeCards()
	n.find.open = false
	n.shown = false
	if n.win == nil {
		return
	}
	n.keepFrame()
	n.win.Hide()
}

// keepFrame keeps where the window is and its size, for it to show
// there again, also after Gocast quits (design.md 11.2).
func (n *notesWindow) keepFrame() {
	b := n.win.Bounds()
	n.g.settings.NotesFrame = &settings.Frame{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
	n.g.saveSettings()
}

// frame is where the window shows: where the user left it, else on the
// display with the pointer, centered, its top a fifth down the work area
// (design.md 11.2); back there, at its size, when what it was left on is
// gone, and no larger than the work area.
func (n *notesWindow) frame() mygo.Rectangle {
	wa := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint()).WorkArea
	var areas []mygo.Rectangle
	for _, d := range mygo.Screen.Displays() {
		areas = append(areas, d.WorkArea)
	}
	return notesFrame(n.g.settings.NotesFrame, wa, areas)
}

// notesFrame is frame's: saved is where the window was left, nil for
// never; wa the work area of the display with the pointer, areas those
// of all.
func notesFrame(saved *settings.Frame, wa mygo.Rectangle, areas []mygo.Rectangle) mygo.Rectangle {
	w, h := notesWidth, notesHeight
	if saved != nil && saved.Width > 0 && saved.Height > 0 {
		w, h = max(saved.Width, notesMinWidth), max(saved.Height, notesMinHeight)
		r := mygo.Rectangle{X: saved.X, Y: saved.Y, Width: w, Height: h}
		for _, a := range areas {
			if overlaps(r, a) {
				return fitIn(r, a)
			}
		}
	}
	r := fitIn(mygo.Rectangle{X: wa.X, Y: wa.Y, Width: min(w, wa.Width), Height: min(h, wa.Height)}, wa)
	r.X = wa.X + (wa.Width-r.Width)/2
	r.Y = wa.Y + int(float64(wa.Height)*notesTopShare)
	if r.Y+r.Height > wa.Y+wa.Height {
		r.Y = wa.Y + wa.Height - r.Height
	}
	return r
}

// overlaps reports whether a and b share any of their area.
func overlaps(a, b mygo.Rectangle) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}

// fitIn shrinks r to the area a, and moves it into it.
func fitIn(r, a mygo.Rectangle) mygo.Rectangle {
	r.Width, r.Height = min(r.Width, a.Width), min(r.Height, a.Height)
	r.X = min(max(r.X, a.X), a.X+a.Width-r.Width)
	r.Y = min(max(r.Y, a.Y), a.Y+a.Height-r.Height)
	return r
}

// --- The notes ---

// scan reads the folder again, as the window opens: notes put back from
// the Trash, changed or removed outside Gocast.
func (n *notesWindow) scan() {
	if _, err := n.store.Scan(); err != nil {
		log.Printf("notes: %v", err)
	}
	for id, d := range n.docs {
		note := n.store.Note(id)
		switch {
		case note == nil && d != n.cur:
			delete(n.docs, id) // gone: opened again from its file, if it comes back
		case note != nil && !d.dirty() && d.writing == 0 && note.Modified.After(d.stamp):
			// Changed outside Gocast, not here: read again.
			if md, err := notesRead(n.store.Path(id)); err == nil {
				d.doc.SetText(notes.Decode(md, n.ruleInk))
				d.saved, d.place = d.doc.Version(), nil
			}
		}
	}
}

// showLast shows the note shown last, else the first of the list; with
// no notes, a new one (design.md 11.1: not the empty state).
func (n *notesWindow) showLast() {
	id := n.store.Last()
	if id == "" {
		if list := n.store.List(); len(list) > 0 {
			id = list[0].ID
		}
	}
	if id == "" {
		n.newNote()
		return
	}
	n.showNote(id)
}

// newNote shows a new note, the caret on its first line; the note
// showing, if it has no text, is it (design.md 11.1).
func (n *notesWindow) newNote() {
	if d := n.cur; d != nil && d.doc.Text().Len() == 0 {
		if d.id != "" {
			n.forget(d) // emptied: new again, its file gone
		}
		n.focusText = true
		return
	}
	n.leave()
	n.cur = &noteDoc{doc: richtext.NewDocument(richtext.Text{})}
	n.cur.saved = n.cur.doc.Version()
	n.afterSwitch()
}

// showNote shows the note id, where it was left in this run, else the
// caret at its end (design.md 11.4).
func (n *notesWindow) showNote(id string) {
	if n.cur != nil && n.cur.id == id {
		return
	}
	d := n.docs[id]
	if d == nil {
		md, err := notesRead(n.store.Path(id))
		if err != nil {
			log.Printf("notes: %v", err)
			return
		}
		d = &noteDoc{id: id, doc: richtext.NewDocument(notes.Decode(md, n.ruleInk))}
		d.saved = d.doc.Version()
		if note := n.store.Note(id); note != nil {
			d.stamp = note.Modified
		}
		n.docs[id] = d
	}
	n.leave()
	n.cur = d
	if d.place != nil {
		p := d.place
		d.doc.SetSelection(p.anchor, p.caret)
		d.doc.ScrollToParagraph(p.para, ui.ScrollStart)
		d.doc.ScrollBy(-p.top)
	} else {
		end := d.doc.Text().Len()
		d.doc.SetSelection(end, end)
	}
	if err := n.store.SetLast(id); err != nil {
		log.Printf("notes: %v", err)
	}
	n.afterSwitch()
}

// afterSwitch follows another note shown: the find bar finds in it, the
// editor takes the focus.
func (n *notesWindow) afterSwitch() {
	n.find.matches, n.find.match = nil, 0
	n.spansVer++
	n.focusText = true
	n.redraw()
}

// leave leaves the note showing: where it is left kept, written now;
// a note with no text forgotten, its file gone (design.md 11.1).
func (n *notesWindow) leave() {
	d := n.cur
	if d == nil {
		return
	}
	a, c := d.doc.Selection()
	p := &notePlace{anchor: a, caret: c}
	if vis := d.doc.VisibleParagraphs(); len(vis) > 0 {
		p.para, p.top = vis[0].Index, vis[0].Top-notePadTop // from the text's top
	}
	d.place = p
	if d.doc.Text().Len() == 0 {
		if d.id != "" {
			n.forget(d)
		}
		return
	}
	n.save(d)
}

// forget drops a note emptied: its file and its place in the list.
func (n *notesWindow) forget(d *noteDoc) {
	if err := n.store.Remove(d.id); err != nil {
		log.Printf("notes: %v", err)
	}
	delete(n.docs, d.id)
	d.id = ""
	d.saved = d.doc.Version()
}

// --- Saving ---

// edited follows an edit of the note showing: it is written once the
// typing pauses (notesSaveDelay).
func (n *notesWindow) edited() {
	if n.saveStop != nil {
		n.saveStop()
	}
	d := n.cur
	n.saveStop = n.after(notesSaveDelay, func() {
		n.saveStop = nil
		n.save(d)
	})
}

// save writes the note as it is now, off the main thread (a long note
// takes tens of milliseconds to encode), unless written already or being
// written, when it is written again after.
func (n *notesWindow) save(d *noteDoc) {
	if d == nil || !d.dirty() {
		return
	}
	if d.writing != 0 {
		d.again = true
		return
	}
	if d.id == "" {
		d.id = n.store.NewID(n.now())
		n.docs[d.id] = d
	}
	text, ver, id, dir := d.doc.Text(), d.doc.Version(), d.id, n.store.Dir()
	d.writing = ver
	sysNotesBackground(n, func() func() {
		md := notes.Encode(text)
		err := notes.Write(dir, id, md)
		return func() { n.written(d, ver, md, err) }
	})
}

// written follows the write of version ver of the note, as md: the list
// knows it; a write that failed shows the banner (design.md 11.5), the
// first that succeeds after one says Saved.
func (n *notesWindow) written(d *noteDoc, ver uint64, md string, err error) {
	d.writing = 0
	if err != nil {
		log.Printf("notes: writing %s: %v", d.id, err)
		if !d.failed && d == n.cur {
			n.say = locales.NotesSaveFailed()
		}
		d.failed, d.again = true, false
		n.redraw()
		return
	}
	d.saved = ver
	n.store.Written(d.id, md)
	if note := n.store.Note(d.id); note != nil {
		d.stamp = note.Modified
	}
	if d == n.cur {
		if err := n.store.SetLast(d.id); err != nil {
			log.Printf("notes: %v", err)
		}
	}
	if d.failed {
		d.failed = false
		if d == n.cur {
			n.tell(notice{icon: "checkmark.circle", text: locales.NotesSaved()})
		}
	}
	if d.again {
		d.again = false
		n.save(d)
	}
	n.redraw()
}

// retry writes the note showing again, as the banner's button asks.
func (n *notesWindow) retry() {
	if d := n.cur; d != nil {
		d.saved = 0 // whatever was written, write it
		n.save(d)
	}
}

// unsaved reports whether a note has text not written.
func (n *notesWindow) unsaved() bool {
	for _, d := range n.all() {
		if d.dirty() {
			return true
		}
	}
	return false
}

// all are the notes opened in this run, and the new one showing.
func (n *notesWindow) all() []*noteDoc {
	var out []*noteDoc
	for _, d := range n.docs {
		out = append(out, d)
	}
	if n.cur != nil && n.cur.id == "" {
		out = append(out, n.cur)
	}
	return out
}

// flush writes every note not written, on the main thread, as Gocast
// quits; it reports whether all are.
func (n *notesWindow) flush() bool {
	ok := true
	for _, d := range n.all() {
		if !d.dirty() {
			continue
		}
		if d.id == "" {
			d.id = n.store.NewID(n.now())
			n.docs[d.id] = d
		}
		md := notes.Encode(d.doc.Text())
		if err := notes.Write(n.store.Dir(), d.id, md); err != nil {
			log.Printf("notes: writing %s: %v", d.id, err)
			d.failed, ok = true, false
			continue
		}
		d.saved = d.doc.Version()
		n.store.Written(d.id, md)
	}
	return ok
}

// quitting writes the notes not written as Gocast quits, and asks before
// it quits when one could not be (copy.md 16.7).
func (n *notesWindow) quitting(e *mygo.QuitEvent) {
	if n.shown && n.win != nil {
		n.keepFrame()
	}
	if n.flush() || e.SessionEnding {
		return
	}
	if !sysConfirmQuitUnsaved() {
		e.PreventDefault()
		n.show()
	}
}

// --- Acting on notes ---

// togglePin pins or unpins the note id, saying so (design.md 11.8).
func (n *notesWindow) togglePin(id string) {
	if id == "" || n.store.Note(id) == nil {
		return
	}
	on := !n.store.Pinned(id)
	if err := n.store.SetPinned(id, on); err != nil {
		log.Printf("notes: %v", err)
		return
	}
	if on {
		n.tell(notice{icon: "pin", text: locales.NotesPinned()})
	} else {
		n.tell(notice{icon: "pin.slash", text: locales.NotesUnpinned()})
	}
	n.browse.regroup(n)
}

// pinCurrent pins or unpins the note showing, written first if new.
func (n *notesWindow) pinCurrent() {
	d := n.cur
	if d == nil || d.doc.Text().Len() == 0 {
		return
	}
	if d.id == "" || n.store.Note(d.id) == nil {
		n.flushOne(d)
	}
	n.togglePin(d.id)
}

// flushOne writes the note now, on the main thread: before it is
// pinned or deleted, for the list to have it.
func (n *notesWindow) flushOne(d *noteDoc) {
	if d.id == "" {
		d.id = n.store.NewID(n.now())
		n.docs[d.id] = d
	}
	md := notes.Encode(d.doc.Text())
	if err := notes.Write(n.store.Dir(), d.id, md); err != nil {
		log.Printf("notes: writing %s: %v", d.id, err)
		d.failed = true
		return
	}
	d.saved = d.doc.Version()
	n.store.Written(d.id, md)
}

// remove moves the note id to the Trash (copy.md 16.5, no confirmation):
// the note showing, if it was, is the first of the list next, or the
// empty state.
func (n *notesWindow) remove(id string) {
	if n.cur != nil && n.cur.id == id && n.cur.dirty() {
		n.flushOne(n.cur) // what the Trash keeps is all of it
	}
	if err := n.store.Delete(id); err != nil {
		log.Printf("notes: trashing %s: %v", id, err)
		return
	}
	delete(n.docs, id)
	n.tell(notice{icon: "trash", text: locales.NotesTrashed()})
	if n.cur != nil && n.cur.id == id {
		n.cur = nil
		if !n.browse.open {
			n.showFirst()
		}
	}
	n.browse.removed(n, id)
}

// removeCurrent deletes the note showing, unless it has no text.
func (n *notesWindow) removeCurrent() {
	d := n.cur
	if d == nil || d.doc.Text().Len() == 0 {
		return
	}
	if d.id == "" || n.store.Note(d.id) == nil {
		n.flushOne(d)
	}
	n.remove(d.id)
}

// showFirst shows the first note of Recent, the newest not pinned (design.md
// 11.6, 11.7), else the first pinned, else the empty state.
func (n *notesWindow) showFirst() {
	if list := n.store.List(); len(list) > 0 {
		id := list[0].ID
		for _, note := range list {
			if !n.store.Pinned(note.ID) {
				id = note.ID
				break
			}
		}
		n.showNote(id)
		return
	}
	n.cur = nil
	n.afterSwitch()
}

// copyMarkdown copies the note showing as Markdown; the window stays
// (design.md 11.8).
func (n *notesWindow) copyMarkdown() {
	d := n.cur
	if d == nil || d.doc.Text().Len() == 0 {
		return
	}
	n.services.WriteClipboard(notes.Encode(d.doc.Text()))
	n.tell(notice{icon: "checkmark.circle", text: locales.NotesCopied()})
}

// openPinned opens the nth pinned note (1 to 9), or says there is none
// (copy.md 16.6).
func (n *notesWindow) openPinned(nth int) {
	pinned := n.store.PinnedIDs()
	if nth > len(pinned) {
		n.tell(notice{icon: "info.circle", text: locales.NotesNoPinned(strconv.Itoa(nth))})
		return
	}
	n.closeCards()
	n.showNote(pinned[nth-1])
}

// zoom steps the text's size by step (0 back to 100%), saying the size,
// also at either end (design.md 11.4).
func (n *notesWindow) zoom(step int) {
	pct := n.g.settings.Zoom()
	if step == 0 {
		pct = 100
	} else if i := slices.Index(settings.NotesZooms, pct) + step; i >= 0 && i < len(settings.NotesZooms) {
		pct = settings.NotesZooms[i]
	}
	if pct != n.g.settings.Zoom() {
		n.g.settings.SetZoom(pct)
		n.g.saveSettings()
		n.spansVer++
	}
	n.tell(notice{icon: "textformat.size", text: locales.NotesZoom(strconv.Itoa(pct))})
}

// tell shows a notice at the bottom left for a while, and reads it out
// (design.md 11.8).
func (n *notesWindow) tell(nt notice) {
	n.notice = &nt
	n.say = nt.text
	n.noticeN++
	if n.stopN != nil {
		n.stopN()
	}
	gen := n.noticeN
	n.stopN = n.after(noticeFor, func() {
		if n.noticeN == gen {
			n.notice = nil
			n.redraw()
		}
	})
	n.redraw()
}

// ruleInk is the color of the rules: the hairline of the appearance
// shown.
func (n *notesWindow) ruleInk() ui.Color { return n.design.k.hairline }
