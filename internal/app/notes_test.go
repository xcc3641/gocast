package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"gocast/internal/notes"
	"gocast/internal/settings"
	"gocast/internal/sysfmt"
	"gocast/locales"
)

// notesNow is the tests' clock: the notes' times are said from it.
var notesNow = time.Date(2026, 11, 13, 15, 0, 0, 0, time.Local)

// notesTest is the notes window without its window, over a folder of the
// test's, its timers held until fire runs them.
type notesTest struct {
	*notesWindow
	tt     *ui.Tester
	timers []*func()
}

// notesTester opens the notes window over the notes md (by ID, written
// with mtimes a minute apart, the first newest), in English.
func notesTester(t *testing.T, md ...string) *notesTest {
	t.Helper()
	return notesTesterIn(t, "en", md...)
}

func notesTesterIn(t *testing.T, locale string, md ...string) *notesTest {
	t.Helper()
	inLocale(t, locale)
	g := &gocast{dir: t.TempDir()}
	dir := filepath.Join(g.dir, "Notes")
	for i, s := range md {
		writeNote(t, dir, noteID(i), s, notesNow.Add(-time.Duration(i+1)*time.Minute))
	}
	n := newNotesState(g, dir)
	n.now = func() time.Time { return notesNow }
	nt := &notesTest{notesWindow: n}
	n.after = func(_ time.Duration, fn func()) func() {
		f := &fn
		nt.timers = append(nt.timers, f)
		return func() { *f = nil }
	}
	nt.tt = ui.NewTester(n.view, notesWidth, notesHeight)
	nt.tt.SetVibrancy(true)
	n.open(notesShow)
	nt.frame()
	return nt
}

func noteID(i int) string { return "2026-11-0" + string(rune('1'+i)) + "-120000" }

// writeNote writes a note's file as another app would, at mtime.
func writeNote(t *testing.T, dir, id, md string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+notes.Ext)
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// fire runs the timers due: the saves waiting, the notices fading.
func (nt *notesTest) fire() {
	ts := nt.timers
	nt.timers = nil
	for _, f := range ts {
		if *f != nil {
			(*f)()
		}
	}
	nt.frame()
}

// frame draws two frames: one for what the last did, one for what it
// asked (the focus).
func (nt *notesTest) frame() {
	nt.tt.Frame()
	nt.tt.Frame()
}

func (nt *notesTest) key(mods ui.Modifiers, key ui.Key) {
	nt.tt.Key(mods, key)
	nt.frame()
}

// typ types s a rune at a time, as a keyboard does: richtext's Markdown
// and the window's shortcuts follow what each key typed.
func (nt *notesTest) typ(s string) {
	for _, r := range s {
		nt.tt.Type(string(r))
		nt.frame()
	}
}

// files are the notes' files in the folder, as they are, by ID.
func (nt *notesTest) files(t *testing.T) map[string]string {
	t.Helper()
	got := map[string]string{}
	entries, _ := os.ReadDir(nt.store.Dir())
	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), notes.Ext); ok {
			data, err := os.ReadFile(filepath.Join(nt.store.Dir(), e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			got[id] = string(data)
		}
	}
	return got
}

// text is the note showing's text.
func (nt *notesTest) text() string {
	if nt.cur == nil {
		return "<none>"
	}
	return nt.cur.doc.Text().String()
}

// useFakeTrash moves the notes deleted to a folder of the test's, and
// returns it.
func useFakeTrash(t *testing.T) string {
	t.Helper()
	trash := t.TempDir()
	old := sysTrash
	sysTrash = func(path string) error { return os.Rename(path, filepath.Join(trash, filepath.Base(path))) }
	t.Cleanup(func() { sysTrash = old })
	return trash
}

func TestNotesFrame(t *testing.T) {
	// The window shows where it was left, else centered a fifth down the
	// display with the pointer, back there when what it was left on is
	// gone, and never larger than the work area (design.md 11.2).
	wa := mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}
	ext := mygo.Rectangle{X: 1440, Y: 0, Width: 1920, Height: 1080}
	if got, want := notesFrame(nil, wa, []mygo.Rectangle{wa}), (mygo.Rectangle{X: 440, Y: 200, Width: 560, Height: 440}); got != want {
		t.Errorf("first: %v, want %v", got, want)
	}
	left := &settings.Frame{X: 1600, Y: 100, Width: 700, Height: 500}
	if got, want := notesFrame(left, wa, []mygo.Rectangle{wa, ext}), (mygo.Rectangle{X: 1600, Y: 100, Width: 700, Height: 500}); got != want {
		t.Errorf("left on the other display: %v, want %v", got, want)
	}
	if got, want := notesFrame(left, wa, []mygo.Rectangle{wa}), (mygo.Rectangle{X: 370, Y: 200, Width: 700, Height: 500}); got != want {
		t.Errorf("its display gone: %v, want %v", got, want)
	}
	big := &settings.Frame{X: 10, Y: 30, Width: 2000, Height: 2000}
	if got, want := notesFrame(big, wa, []mygo.Rectangle{wa}), (mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}); got != want {
		t.Errorf("larger than the work area: %v, want %v", got, want)
	}
	small := &settings.Frame{X: 10, Y: 30, Width: 100, Height: 100}
	if got := notesFrame(small, wa, []mygo.Rectangle{wa}); got.Width != notesMinWidth || got.Height != notesMinHeight {
		t.Errorf("smaller than the least: %v", got)
	}
}

func TestNotesFirstOpen(t *testing.T) {
	// With no notes, the window opens to a new note, its placeholder and
	// Untitled, not the empty state; a note with no text is never
	// written (design.md 11.1, 11.8).
	nt := notesTester(t)
	if nt.cur == nil || nt.cur.id != "" {
		t.Fatalf("not a new note: %+v", nt.cur)
	}
	if !nt.tt.HasText(locales.NotesUntitled()) {
		t.Errorf("no Untitled: %q", nt.tt.Texts())
	}
	if a, ok := nt.tt.Access(locales.NotesEditorLabel(locales.NotesUntitled())); !ok || a.Placeholder != locales.NotesPlaceholder() {
		t.Errorf("placeholder: %+v", a)
	}
	if nt.tt.HasText(locales.NotesEmptyTitle()) {
		t.Error("the empty state at first")
	}
	if !nt.tt.Focused(locales.NotesEditorLabel(locales.NotesUntitled())) {
		t.Error("the text has not the focus")
	}
	nt.hide()
	nt.fire()
	if got := nt.files(t); len(got) != 0 {
		t.Errorf("an empty note written: %q", got)
	}
}

func TestNotesTypingSaves(t *testing.T) {
	// What is typed is written once the typing pauses, as Markdown; the
	// top bar's title follows the first line, without its marks
	// (design.md 11.3, copy.md 16.2).
	nt := notesTester(t)
	nt.typ("# Groceries")
	nt.key(0, ui.KeyEnter)
	nt.typ("- milk")
	if got := nt.files(t); len(got) != 0 {
		t.Fatalf("written before the pause: %q", got)
	}
	nt.fire()
	got := nt.files(t)
	if len(got) != 1 || got[nt.cur.id] != "# Groceries\n- milk\n" {
		t.Fatalf("written: %q", got)
	}
	if h, ok := nt.tt.AccessByRole(ui.RoleHeading, "Groceries"); !ok {
		t.Errorf("title: %q", nt.tt.Texts())
	} else if mid := h.Bounds.X + h.Bounds.W/2; mid < notesWidth/2-1 || mid > notesWidth/2+1 {
		t.Errorf("title centered at %v, want %v ± 1", mid, notesWidth/2)
	}
	if nt.store.Last() != nt.cur.id {
		t.Errorf("last: %q, want %q", nt.store.Last(), nt.cur.id)
	}
	// Edited while it waits: written once, as it is last.
	nt.typ(" and eggs")
	nt.typ("!")
	nt.fire()
	if got := nt.files(t)[nt.cur.id]; got != "# Groceries\n- milk and eggs!\n" {
		t.Errorf("again: %q", got)
	}
	// No notice for a save (copy.md 16: normal saves show no text).
	if nt.notice != nil {
		t.Errorf("a notice: %+v", nt.notice)
	}
}

func TestNotesEmptiedForgotten(t *testing.T) {
	// A note emptied is forgotten as it is left: its file gone, not in
	// the Trash (design.md 11.1).
	trash := useFakeTrash(t)
	nt := notesTester(t, "keep\n")
	nt.key(ui.Cmd, ui.KeyN)
	nt.typ("x")
	nt.fire()
	id := nt.cur.id
	if _, ok := nt.files(t)[id]; !ok {
		t.Fatal("not written")
	}
	nt.key(0, ui.KeyBackspace)
	nt.key(ui.Cmd, ui.KeyN) // left for a new note
	nt.fire()
	if _, ok := nt.files(t)[id]; ok || nt.store.Note(id) != nil {
		t.Error("an emptied note kept")
	}
	if entries, _ := os.ReadDir(trash); len(entries) != 0 {
		t.Errorf("in the Trash: %v", entries)
	}
	// ⌘N on a new note with no text keeps it.
	cur := nt.cur
	nt.key(ui.Cmd, ui.KeyN)
	if nt.cur != cur {
		t.Error("another new note")
	}
}

func TestNotesShowLastAndPlace(t *testing.T) {
	// The window opens to the note shown last; a note opened the first
	// time has the caret at its end, then where it was left (design.md
	// 11.4).
	nt := notesTester(t, "first\nline two\n", "second\n")
	if nt.cur.id != noteID(0) {
		t.Fatalf("showing %q, want the newest", nt.cur.id)
	}
	if a, c := nt.cur.doc.Selection(); a != c || c != nt.cur.doc.Text().Len() {
		t.Errorf("caret %d %d, want at the end", a, c)
	}
	nt.cur.doc.SetSelection(2, 4)
	nt.openPinnedOrShow(noteID(1))
	if nt.cur.id != noteID(1) || nt.store.Last() != noteID(1) {
		t.Fatalf("showing %q, last %q", nt.cur.id, nt.store.Last())
	}
	nt.openPinnedOrShow(noteID(0))
	if a, c := nt.cur.doc.Selection(); a != 2 || c != 4 {
		t.Errorf("selection %d %d, want 2 4", a, c)
	}
	nt.hide()
	nt.open(notesShow)
	nt.frame()
	if nt.cur.id != noteID(0) {
		t.Errorf("showing %q after hiding", nt.cur.id)
	}
}

// openPinnedOrShow shows the note id, as Browse Notes does.
func (nt *notesTest) openPinnedOrShow(id string) {
	nt.showNote(id)
	nt.frame()
}

func TestNotesDelete(t *testing.T) {
	// ⌃X moves the note to the Trash without asking, says so, and shows
	// the first of the list; the last gone, the empty state, where ⌘P
	// does nothing and Return makes a note (copy.md 16.5, design.md 11.8).
	trash := useFakeTrash(t)
	nt := notesTester(t, "one\n", "two\n")
	nt.key(ui.Ctrl, ui.KeyX)
	if _, err := os.Stat(filepath.Join(trash, noteID(0)+notes.Ext)); err != nil {
		t.Fatalf("not in the Trash: %v", err)
	}
	if nt.notice == nil || nt.notice.text != locales.NotesTrashed() || !nt.tt.HasText(locales.NotesTrashed()) {
		t.Errorf("notice: %+v", nt.notice)
	}
	if nt.cur == nil || nt.cur.id != noteID(1) {
		t.Fatalf("showing %+v, want the other", nt.cur)
	}
	nt.key(ui.Ctrl, ui.KeyX)
	if nt.cur != nil || !nt.tt.HasText(locales.NotesEmptyTitle()) || !nt.tt.HasText(locales.NotesEmptyBody()) {
		t.Fatalf("not the empty state: %q", nt.tt.Texts())
	}
	nt.key(ui.Cmd, ui.KeyP)
	if nt.browse.open {
		t.Error("⌘P opened Browse Notes in the empty state")
	}
	nt.key(ui.Cmd, ui.KeyK)
	as := nt.menuActions()
	for i, a := range as {
		if a.dim != (i > 0) {
			t.Errorf("%s dim %v in the empty state", a.label, a.dim)
		}
	}
	nt.key(0, ui.KeyEscape)
	nt.key(0, ui.KeyEnter)
	if nt.cur == nil || nt.cur.id != "" {
		t.Errorf("Return: %+v", nt.cur)
	}
	// The notice fades.
	nt.fire()
	if nt.notice != nil {
		t.Error("the notice stays")
	}
}

func TestNotesPin(t *testing.T) {
	// ⇧⌘P pins the note, the top bar's pin showing; ⌘1 opens the first
	// pinned; ⌘n with none says so (design.md 11.8).
	nt := notesTester(t, "one\n", "two\n")
	nt.key(ui.Cmd|ui.Shift, ui.KeyP)
	if !nt.store.Pinned(noteID(0)) || nt.notice == nil || nt.notice.text != locales.NotesPinned() {
		t.Fatalf("not pinned: %+v", nt.notice)
	}
	if _, ok := nt.tt.Access(locales.NotesPinnedTip()); !ok {
		t.Error("no pin in the top bar")
	}
	nt.showNote(noteID(1))
	nt.frame()
	nt.key(ui.Cmd, ui.Key1)
	if nt.cur.id != noteID(0) {
		t.Errorf("⌘1 showed %q", nt.cur.id)
	}
	nt.key(ui.Cmd, ui.Key2)
	if nt.notice == nil || nt.notice.text != locales.NotesNoPinned("2") {
		t.Errorf("⌘2: %+v", nt.notice)
	}
	nt.key(ui.Cmd|ui.Shift, ui.KeyP)
	if nt.store.Pinned(noteID(0)) || nt.notice.text != locales.NotesUnpinned() {
		t.Error("not unpinned")
	}
}

func TestNotesBrowse(t *testing.T) {
	// ⌘P lists the notes, the pinned under their title with their keys,
	// the note showing chosen; typing finds in titles and text, a match in
	// the text alone showing its line; Return opens; ⌘⌫ with the field
	// empty deletes (design.md 11.6).
	useFakeTrash(t)
	nt := notesTester(t, "Groceries\n- milk\n", "Trip\nbook the train\n", "Ideas\n")
	nt.store.SetPinned(noteID(2), true)
	nt.key(ui.Cmd, ui.KeyP)
	if !nt.browse.open || nt.browse.chosenID() != noteID(0) {
		t.Fatalf("open %v, chosen %q", nt.browse.open, nt.browse.chosenID())
	}
	var got []string
	for _, r := range nt.browse.rows {
		if r.title != "" {
			got = append(got, "["+r.title+"]")
		} else {
			got = append(got, r.m.Note.Title())
		}
	}
	if want := []string{"[" + locales.NotesSectionPinned() + "]", "Ideas", "[" + locales.NotesSectionRecent() + "]", "Groceries", "Trip"}; !slices.Equal(got, want) {
		t.Errorf("rows %q, want %q", got, want)
	}
	if !nt.tt.HasText("⌘") || !nt.tt.HasText("1") {
		t.Errorf("no ⌘1: %q", nt.tt.Texts())
	}
	if !nt.tt.Focused(locales.NotesSearchPlaceholder()) {
		t.Error("the field has not the focus")
	}
	nt.typ("train")
	if id := nt.browse.chosenID(); id != noteID(1) || nt.browse.rows[0].m.Snippet == "" {
		t.Fatalf("found %q %+v", id, nt.browse.rows)
	}
	if !nt.tt.HasText("book the train") {
		t.Errorf("no snippet: %q", nt.tt.Texts())
	}
	nt.key(0, ui.KeyEnter)
	if nt.browse.open || nt.cur.id != noteID(1) {
		t.Fatalf("Return: open %v, showing %q", nt.browse.open, nt.cur.id)
	}
	// Esc and ⌘P close it, back to the note.
	nt.key(ui.Cmd, ui.KeyP)
	nt.key(0, ui.KeyEscape)
	if nt.browse.open || !nt.shown {
		t.Errorf("Esc: open %v shown %v", nt.browse.open, nt.shown)
	}
	nt.key(ui.Cmd, ui.KeyP)
	nt.key(ui.Cmd, ui.KeyP)
	if nt.browse.open {
		t.Error("⌘P did not close it")
	}
	// ⌘⌫ deletes the chosen, the choice on the next row.
	nt.key(ui.Cmd, ui.KeyP)
	nt.key(ui.Cmd, ui.KeyBackspace)
	if nt.store.Note(noteID(1)) != nil {
		t.Fatal("⌘⌫ did not delete")
	}
	if !nt.browse.open || nt.browse.chosenID() == "" {
		t.Errorf("after deleting: open %v chosen %q", nt.browse.open, nt.browse.chosenID())
	}
	nt.key(0, ui.KeyEscape)
	if nt.cur == nil || nt.cur.id != noteID(0) {
		t.Errorf("closed after deleting the note showing: %+v", nt.cur)
	}
}

func TestNotesBrowseSearchesInBackground(t *testing.T) {
	// Over 1000 notes, typing in Browse Notes never waits for the search:
	// each runs off the main thread, the rows as they were until it is
	// done, and only the last query typed is shown, whatever order the
	// searches end in (design.md 11.6).
	var jobs []func() func()
	old := sysNotesBackground
	sysNotesBackground = func(_ *notesWindow, work func() func()) { jobs = append(jobs, work) }
	t.Cleanup(func() { sysNotesBackground = old })
	// run runs a search on a goroutine of its own, then its results on
	// the main thread, as the window does.
	run := func(work func() func()) {
		ch := make(chan func())
		go func() { ch <- work() }()
		(<-ch)()
	}

	nt := notesTester(t)
	dir := nt.store.Dir()
	written := time.Now()
	defer func() { t.Logf("1000 notes written in %v", time.Since(written)) }()
	for i := range 1000 {
		body := fmt.Sprintf("Note %04d\n%s needle-%04d %s\n", i, strings.Repeat("filler text 文字 ", 60), i, strings.Repeat("more 更多 ", 40))
		writeNote(t, dir, fmt.Sprintf("2026-10-01-%06d", i), body, notesNow.Add(-time.Duration(i+1)*time.Second))
	}
	nt.key(ui.Cmd, ui.KeyP)
	if !nt.browse.open || len(nt.browse.rows) != 1000 || len(jobs) != 0 {
		t.Fatalf("open %v, %d rows, %d searches: the query empty lists them at once", nt.browse.open, len(nt.browse.rows), len(jobs))
	}
	// The list builds the rows it shows, not all 1000 a frame.
	if !nt.tt.HasText("Note 0000") || nt.tt.HasText("Note 0100") {
		t.Errorf("rows built: %d texts", len(nt.tt.Texts()))
	}
	query := "needle-0777"
	typing := time.Now()
	nt.typ(query)
	t.Logf("typed %d characters in %v", len(query), time.Since(typing))
	if nt.browse.query != query || len(jobs) != len(query) {
		t.Fatalf("query %q, %d searches started", nt.browse.query, len(jobs))
	}
	if len(nt.browse.rows) != 1000 {
		t.Fatalf("%d rows before any search was done: searched on the main thread", len(nt.browse.rows))
	}
	start := time.Now()
	run(jobs[len(jobs)-1]) // the last ends first
	t.Logf("searched 1000 notes in %v", time.Since(start))
	nt.frame()
	rows := nt.browse.rows
	if len(rows) != 1 || rows[0].m.Note.Title() != "Note 0777" || !strings.Contains(rows[0].m.Snippet, query) || nt.browse.chosenID() != rows[0].m.Note.ID {
		t.Fatalf("found %d rows, chosen %q", len(rows), nt.browse.chosenID())
	}
	for _, work := range jobs[:len(jobs)-1] { // the earlier, slower ones
		run(work)
	}
	nt.frame()
	if len(nt.browse.rows) != 1 || nt.browse.rows[0].m.Note.Title() != "Note 0777" {
		t.Errorf("an earlier query's results shown: %d rows", len(nt.browse.rows))
	}
	if !nt.tt.HasText("Note 0777") {
		t.Errorf("not drawn: %q", nt.tt.Texts())
	}
	// Back to fewer characters: a new search, its results the latest.
	jobs = jobs[:0]
	nt.key(0, ui.KeyBackspace)
	if len(jobs) != 1 {
		t.Fatalf("%d searches for one key", len(jobs))
	}
	run(jobs[0])
	nt.frame()
	if len(nt.browse.rows) != 10 { // needle-0770 to needle-0779
		t.Errorf("needle-077: %d rows", len(nt.browse.rows))
	}
	// A search ending after the card closed shows nothing.
	jobs = jobs[:0]
	nt.typ("9")
	nt.key(0, ui.KeyEscape)
	run(jobs[0])
	if nt.browse.open {
		t.Error("a late search opened the card")
	}
}

func TestNotesMenu(t *testing.T) {
	// ⌘K's five rows (copy.md 16.5): those of a note with no text dim;
	// Copy as Markdown copies, the window staying.
	nt := notesTester(t)
	nt.key(ui.Cmd, ui.KeyK)
	var labels []string
	for _, a := range nt.menuActions() {
		labels = append(labels, a.label)
		if a.dim != (a.label != locales.NotesNewItem() && a.label != locales.NotesBrowse()) {
			t.Errorf("%s dim %v in an empty note", a.label, a.dim)
		}
	}
	if want := []string{locales.NotesNewItem(), locales.NotesBrowse(), locales.NotesPin(), locales.NotesCopyMarkdown(), locales.NotesDelete()}; !slices.Equal(labels, want) {
		t.Errorf("rows %q", labels)
	}
	for _, l := range labels {
		if !nt.tt.HasText(l) {
			t.Errorf("no %q", l)
		}
	}
	nt.key(0, ui.KeyEscape)
	if nt.menu.open || !nt.shown {
		t.Fatal("Esc did not close the menu alone")
	}
	nt.typ("**bold** text")
	nt.key(ui.Cmd, ui.KeyK)
	nt.typ("copy")
	nt.key(0, ui.KeyEnter)
	if got := nt.tt.Clipboard(); got != "**bold** text\n" {
		t.Errorf("copied %q", got)
	}
	if !nt.shown || nt.notice == nil || nt.notice.text != locales.NotesCopied() {
		t.Errorf("after copying: shown %v notice %+v", nt.shown, nt.notice)
	}
}

func TestNotesFind(t *testing.T) {
	// ⌘F finds in the note whatever the case, counting; Return steps,
	// wrapping; Esc selects the match and closes the bar (design.md 11.4).
	nt := notesTester(t, "Apple pie\nan apple a day\n")
	nt.key(ui.Cmd, ui.KeyF)
	if !nt.find.open || !nt.tt.Focused(locales.NotesFindPlaceholder()) {
		t.Fatal("no find bar")
	}
	nt.typ("apple")
	if _, ok := nt.tt.Access(locales.NotesFindCountSpoken("1", "2")); len(nt.find.matches) != 2 || !ok {
		t.Fatalf("matches %v, %q", nt.find.matches, nt.tt.Texts())
	}
	nt.key(0, ui.KeyEnter)
	if nt.find.match != 1 {
		t.Errorf("Return: match %d", nt.find.match)
	}
	nt.key(ui.Shift, ui.KeyEnter)
	nt.key(ui.Cmd|ui.Shift, ui.KeyG)
	if nt.find.match != 1 {
		t.Errorf("⇧↩ ⇧⌘G: match %d", nt.find.match)
	}
	nt.key(0, ui.KeyEscape)
	if nt.find.open || !nt.shown {
		t.Fatal("Esc did not close the bar alone")
	}
	if a, c := nt.cur.doc.Selection(); a != 13 || c != 18 {
		t.Errorf("selection %d %d, want the match 13 18", a, c)
	}
	nt.key(ui.Cmd, ui.KeyF)
	nt.tt.Key(ui.Cmd, ui.KeyA)
	nt.typ("pear")
	if !nt.tt.HasText(locales.NotesFindNone()) {
		t.Errorf("no match: %q", nt.tt.Texts())
	}
}

func TestNotesSaveFailed(t *testing.T) {
	// A write that fails shows the banner and says it, the text kept;
	// Try Again that succeeds takes it away and says Saved; quitting with
	// it asks (design.md 11.5, copy.md 16.7).
	nt := notesTester(t)
	tt := nt.tt
	tt.UseScreenReader()
	if err := os.WriteFile(nt.store.Dir(), nil, 0o644); err != nil { // the folder a file
		t.Fatal(err)
	}
	nt.typ("hello")
	nt.fire()
	if !tt.HasText(locales.NotesSaveFailed()) || !tt.HasText(locales.NotesTryAgain()) {
		t.Fatalf("no banner: %q", tt.Texts())
	}
	if !slices.Contains(tt.Announcements(), locales.NotesSaveFailed()) {
		t.Errorf("not said: %q", tt.Announcements())
	}
	asked := 0
	old := sysConfirmQuitUnsaved
	sysConfirmQuitUnsaved = func() bool { asked++; return false }
	t.Cleanup(func() { sysConfirmQuitUnsaved = old })
	e := &mygo.QuitEvent{}
	nt.quitting(e)
	if asked != 1 || !e.DefaultPrevented() {
		t.Errorf("quit: asked %d, prevented %v", asked, e.DefaultPrevented())
	}
	os.Remove(nt.store.Dir())
	if err := tt.Click(locales.NotesTryAgain()); err != nil {
		t.Fatal(err)
	}
	nt.frame()
	if tt.HasText(locales.NotesSaveFailed()) || nt.notice == nil || nt.notice.text != locales.NotesSaved() {
		t.Errorf("after Try Again: %q, notice %+v", tt.Texts(), nt.notice)
	}
	if got := nt.files(t)[nt.cur.id]; got != "hello\n" {
		t.Errorf("written %q", got)
	}
	e = &mygo.QuitEvent{}
	nt.quitting(e)
	if asked != 1 || e.DefaultPrevented() {
		t.Error("asked with all written")
	}
}

func TestNotesQuitWrites(t *testing.T) {
	// Quitting writes what waits to be written, at once.
	nt := notesTester(t)
	nt.typ("not yet")
	e := &mygo.QuitEvent{}
	nt.quitting(e)
	if e.DefaultPrevented() || nt.files(t)[nt.cur.id] != "not yet\n" {
		t.Errorf("prevented %v, files %q", e.DefaultPrevented(), nt.files(t))
	}
}

func TestNotesFormatKeys(t *testing.T) {
	// The formats' keys (design.md 11.9): ⌥⌘1 a heading and back, ⇧⌘7
	// a numbered list, ⇧⌘9 tasks and ⌘↩ ticks one, ⌥⌘C code, ⇧⌘B a
	// quotation, ⌘L a link to the URL on the clipboard, Paste of a URL
	// over a selection a link.
	nt := notesTester(t)
	nt.typ("title")
	para := func() richtext.ParagraphStyle { return nt.cur.doc.Text().ParagraphStyleAt(0) }
	nt.key(ui.Cmd|ui.Alt, ui.Key1)
	if para().Heading != 1 {
		t.Errorf("⌥⌘1: %+v", para())
	}
	nt.key(ui.Cmd|ui.Alt, ui.Key1)
	if para().Heading != 0 {
		t.Errorf("⌥⌘1 again: %+v", para())
	}
	nt.key(ui.Cmd|ui.Shift, ui.Key7)
	if para().List != richtext.Numbers {
		t.Errorf("⇧⌘7: %+v", para())
	}
	nt.key(ui.Cmd|ui.Shift, ui.Key9)
	nt.key(ui.Cmd, ui.KeyEnter)
	if p := para(); p.List != richtext.Tasks || !p.Checked {
		t.Errorf("⇧⌘9 ⌘↩: %+v", p)
	}
	nt.key(ui.Cmd|ui.Shift, ui.Key9)
	nt.key(ui.Cmd|ui.Shift, ui.KeyB)
	if !para().Quote {
		t.Errorf("⇧⌘B: %+v", para())
	}
	nt.key(ui.Cmd|ui.Shift, ui.KeyB)
	nt.key(ui.Cmd|ui.Alt, ui.KeyC)
	if !para().CodeBlock {
		t.Errorf("⌥⌘C: %+v", para())
	}
	nt.key(ui.Cmd|ui.Alt, ui.KeyC)
	nt.tt.SetClipboard("https://example.com")
	nt.cur.doc.SetSelection(0, 5)
	nt.key(ui.Cmd, ui.KeyL)
	if got := nt.cur.doc.Text().AttributesAt(0).Link; got != "https://example.com" {
		t.Errorf("⌘L: link %q", got)
	}
	nt.cur.doc.SetSelection(0, 5)
	nt.key(ui.Cmd, ui.KeyL) // a link at the caret, no other URL: unlinked
	nt.tt.SetClipboard("not a url")
	nt.key(ui.Cmd, ui.KeyL)
	if got := nt.cur.doc.Text().AttributesAt(0).Link; got != "" {
		t.Errorf("⌘L to unlink: %q", got)
	}
	nt.tt.SetClipboard("https://gocast.app")
	nt.cur.doc.SetSelection(0, 5)
	nt.tt.Command("paste")
	nt.frame()
	if got, text := nt.cur.doc.Text().AttributesAt(0).Link, nt.text(); got != "https://gocast.app" || text != "title" {
		t.Errorf("Paste over a selection: link %q, text %q", got, text)
	}
	nt.fire()
	if got := nt.files(t)[nt.cur.id]; got != "[title](https://gocast.app)\n" {
		t.Errorf("written %q", got)
	}
}

func TestNotesQuitKey(t *testing.T) {
	// ⌘Q is the application menu's Quit, which has the key before any
	// window: in the notes window it hides it, Gocast staying; anywhere
	// else Gocast quits (design.md 11.2).
	nt := notesTester(t, "Groceries\n")
	g := nt.g
	g.notes = nt.notesWindow
	quits := 0
	key := true
	oldQuit, oldKey := sysAppQuit, sysNotesKey
	sysAppQuit = func() { quits++ }
	sysNotesKey = func(*notesWindow) bool { return key }
	t.Cleanup(func() { sysAppQuit, sysNotesKey = oldQuit, oldKey })

	quit := g.appMenu().ItemByID("quit")
	if quit == nil || quit.Accelerator != "Cmd+Q" {
		t.Fatalf("no ⌘Q in the application menu: %+v", quit)
	}
	nt.typ("x")
	quit.Click(quit, nil)
	if nt.shown || quits != 0 {
		t.Fatalf("⌘Q in the notes window: shown %v, quit %d times", nt.shown, quits)
	}
	if got := nt.files(t)[noteID(0)]; got != "Groceriesx\n" {
		t.Errorf("not written as it hid: %q", got)
	}
	// Another window has the keyboard, the notes showing behind it.
	nt.open(notesShow)
	nt.frame()
	key = false
	quit.Click(quit, nil)
	if quits != 1 || !nt.shown {
		t.Errorf("⌘Q elsewhere: quit %d times, notes shown %v", quits, nt.shown)
	}
	// The notes hidden: whatever had the keyboard last, Gocast quits.
	nt.hide()
	key = true
	quit.Click(quit, nil)
	if quits != 2 {
		t.Errorf("⌘Q with the notes hidden: quit %d times", quits)
	}
}

// The application menu leaves the notes' keys to the window: no item
// takes ⌘=, ⌘- or ⌘0 (a View menu's zoom), and the Edit menu's Paste is
// there, which the window takes as a command (onCommand).
func TestAppMenuKeys(t *testing.T) {
	g := &gocast{}
	var accels []string
	var walk func(items []*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, it := range items {
			if it.Accelerator != "" {
				accels = append(accels, it.Accelerator)
			}
			walk(it.Submenu)
		}
	}
	walk(g.appMenu().Items())
	for _, taken := range []string{"Cmd+Plus", "CmdOrCtrl+Plus", "CmdOrCtrl+=", "Cmd+=", "CmdOrCtrl+-", "Cmd+-", "CmdOrCtrl+0", "Cmd+0"} {
		if slices.Contains(accels, taken) {
			t.Errorf("the menu takes %s: %q", taken, accels)
		}
	}
	for _, kept := range []string{"Cmd+Q", "CmdOrCtrl+V", "CmdOrCtrl+C", "CmdOrCtrl+Z", "CmdOrCtrl+W"} {
		if !slices.Contains(accels, kept) {
			t.Errorf("no %s: %q", kept, accels)
		}
	}
}

func TestNotesTypedShortcuts(t *testing.T) {
	// --- alone on a line makes a rule; [text](url) a link; a full-width
	// ＃ of an input method at the start of a line a heading's mark.
	nt := notesTester(t)
	nt.typ("a")
	nt.key(0, ui.KeyEnter)
	nt.typ("---")
	if !notes.IsRuleParagraph(slices.Collect(paragraphs(nt.cur.doc.Text()))[1]) {
		t.Errorf("no rule: %q", nt.text())
	}
	nt.typ("see [docs](https://x.y)")
	nt.typ("")
	if !strings.HasSuffix(nt.text(), "see docs") {
		t.Errorf("link: %q", nt.text())
	}
	nt.key(0, ui.KeyEnter)
	nt.typ("＃")
	nt.typ(" Title")
	t2 := nt.cur.doc.Text()
	_, caret := nt.cur.doc.Selection()
	if st := t2.ParagraphStyleAt(caret); st.Heading != 1 {
		t.Errorf("＃ then a space: %+v, %q", st, nt.text())
	}
	nt.fire()
	// The rule typed --- is written ***, not a setext heading's underline.
	if got := nt.files(t)[nt.cur.id]; got != "a\n***\nsee [docs](https://x.y)\n# Title\n" {
		t.Errorf("written %q", got)
	}
}

func paragraphs(t richtext.Text) func(func(richtext.Paragraph) bool) {
	return func(yield func(richtext.Paragraph) bool) {
		for _, p := range t.Paragraphs() {
			if !yield(p) {
				return
			}
		}
	}
}

func TestNotesEscape(t *testing.T) {
	// Esc closes the card, else the find bar, else hides the window; not
	// while the input method composes (design.md 11.2).
	nt := notesTester(t, "text\n")
	nt.key(ui.Cmd, ui.KeyF)
	nt.key(ui.Cmd, ui.KeyK)
	nt.key(0, ui.KeyEscape)
	if nt.menu.open || !nt.find.open {
		t.Fatalf("first Esc: menu %v find %v", nt.menu.open, nt.find.open)
	}
	nt.key(0, ui.KeyEscape)
	if nt.find.open || !nt.shown {
		t.Fatalf("second Esc: find %v shown %v", nt.find.open, nt.shown)
	}
	nt.tt.Compose("ni", 2)
	nt.frame()
	nt.escape() // as the window's Esc would, the input method composing
	if !nt.shown {
		t.Fatal("hidden while composing")
	}
	nt.tt.Compose("", 0)
	nt.frame()
	nt.key(0, ui.KeyEscape)
	if nt.shown {
		t.Error("third Esc did not hide")
	}
	// ⌘W hides too, and the hotkey shows it again, then hides it.
	nt.hotkey(settings.NoteOpen)
	nt.frame()
	nt.key(ui.Cmd, ui.KeyW)
	if nt.shown {
		t.Error("⌘W did not hide")
	}
	nt.hotkey(settings.NoteOpen)
	nt.hotkey(settings.NoteOpen)
	if nt.shown {
		t.Error("the hotkey did not hide it in front")
	}
	nt.hotkey(settings.NoteSearch)
	nt.frame()
	if !nt.shown || !nt.browse.open {
		t.Error("Search Notes' hotkey: Browse Notes not open")
	}
}

func TestNotesIME(t *testing.T) {
	// While the input method composes, the keys are its, find's
	// highlights wait, and what it commits is written.
	nt := notesTester(t)
	nt.tt.Compose("bei", 3)
	nt.frame()
	if !nt.composing {
		t.Fatal("not composing")
	}
	nt.key(ui.Cmd, ui.KeyN) // still a key of the window's: the editor took no text
	nt.tt.Compose("", 0)
	nt.typ("备忘")
	nt.fire()
	if got := nt.files(t)[nt.cur.id]; got != "备忘\n" {
		t.Errorf("written %q", got)
	}
}

func TestNotesZoom(t *testing.T) {
	// ⌘+ ⌘- ⌘0 step the text's size, saying it, also at either end; the
	// size is kept (design.md 11.4).
	nt := notesTester(t)
	nt.key(ui.Cmd, ui.KeyEqual)
	if nt.g.settings.Zoom() != 115 || nt.notice == nil || nt.notice.text != locales.NotesZoom("115") {
		t.Errorf("⌘+: %d %+v", nt.g.settings.Zoom(), nt.notice)
	}
	for range 10 {
		nt.key(ui.Cmd, ui.KeyMinus)
	}
	if nt.g.settings.Zoom() != 75 || nt.notice.text != locales.NotesZoom("75") {
		t.Errorf("⌘- at the end: %d %+v", nt.g.settings.Zoom(), nt.notice)
	}
	nt.key(ui.Cmd, ui.Key0)
	if nt.g.settings.Zoom() != 100 {
		t.Errorf("⌘0: %d", nt.g.settings.Zoom())
	}
	s, err := settings.Load(nt.g.settingsPath())
	if err != nil || s.Zoom() != 100 {
		t.Errorf("kept: %v %v", s.NotesZoom, err)
	}
}

func TestNotesAccess(t *testing.T) {
	// What assistive technology reads (copy.md 16.9): the editor by its
	// note, the buttons by their names, Browse Notes' rows whole.
	nt := notesTester(t, "Groceries\n")
	nt.store.SetPinned(noteID(0), true)
	if _, ok := nt.tt.Access(locales.NotesEditorLabel("Groceries")); !ok {
		t.Error("no editor label")
	}
	for _, l := range []string{locales.NotesBrowseTip(), locales.NotesNewTip()} {
		if _, ok := nt.tt.AccessByRole(ui.RoleButton, l); !ok {
			t.Errorf("no button %q", l)
		}
	}
	nt.key(ui.Cmd, ui.KeyP)
	when := nt.browseLabel(nt.browse.rows[1])
	if want := locales.NotesRowLabelPinned("Groceries", sysfmt.Date(notesNow.Add(-time.Minute), notesNow, "en")); when != want {
		t.Errorf("row label %q, want %q", when, want)
	}
}

// allBlocks is a note with each of design.md 11.4's blocks.
const allBlocks = "# Heading one\nText under it.\n## Heading two\nMore text.\n### Heading three\n- one\n- two\n1. first\n2. second\n- [ ] open task\n- [x] done task\nText after.\n```\ncode line\ncode two\n```\nText.\n> a quote\n> two\nText.\n---\nText after the rule.\n"

func TestNotesTypography(t *testing.T) {
	// design.md 11.12 (1, 2): the text 24 from the window's left; each
	// block's line height and the room between blocks as 11.4 has them,
	// within 1 point; the rule's hairline 16 from the text around it.
	nt := notesTester(t, allBlocks)
	nt.tt.SetSize(notesWidth, 1100)
	nt.frame()
	a, ok := nt.tt.Access(locales.NotesEditorLabel("Heading one"))
	if !ok {
		t.Fatal("no editor")
	}
	nt.cur.doc.SetSelection(0, 0)
	nt.frame()
	if caret, _ := nt.cur.doc.CaretRect(); a.Bounds.X+caret.X != notePadX {
		t.Errorf("text at %v from the left, want %v", a.Bounds.X+caret.X, notePadX)
	}
	ps := nt.cur.doc.VisibleParagraphs()
	if len(ps) != 20 {
		t.Fatalf("%d paragraphs", len(ps))
	}
	// Each paragraph's line height and the room above it (above its
	// line, from the line before).
	want := []struct{ line, above float32 }{
		{30, 0}, {22, 8}, {26, 16}, {22, 6}, {22, 12}, // headings
		{22, 4}, {22, 4}, {22, 4}, {22, 4}, {22, 4}, {22, 4}, // lists
		{22, 8},                // text after a list
		{19, 12 + 10}, {19, 0}, // code: 12 outside, 10 inside
		{22, 10 + 12},             // and after it
		{22, 8}, {22, 8}, {22, 8}, // a quotation, and after it
	}
	for i, w := range want {
		p := ps[i]
		above := p.Top - notePadTop // the editor's room above its text
		if i > 0 {
			above = p.Top
			above -= ps[i-1].Top + ps[i-1].Height
		}
		if abs32(p.Height-w.line) > 1 || abs32(above-w.above) > 1 {
			t.Errorf("paragraph %d: line %v, room above %v; want %v, %v", i, p.Height, above, w.line, w.above)
		}
	}
	// The rule: its hairline 16 from the lines around it.
	img := nt.tt.Image()
	top := a.Bounds.Y
	hair := float32(-1)
	for y := ps[17].Top + ps[17].Height; y < ps[19].Top; y++ {
		if img.RGBAAt(notesWidth/2, int(top+y)) != img.RGBAAt(notesWidth-10, int(top+y)) {
			hair = y + 0.5
			break
		}
	}
	if hair < 0 {
		t.Fatal("no hairline")
	}
	if above, below := hair-(ps[17].Top+ps[17].Height), ps[19].Top-hair; abs32(above-noteRuleGap) > 1 || abs32(below-noteRuleGap) > 1 {
		t.Errorf("rule: %v above its hairline, %v below, want %v", above, below, noteRuleGap)
	}
}

func TestScreensNotes(t *testing.T) {
	// design.md 11.12's screens, light and dark: the window at its size,
	// every block, Browse Notes with pinned notes and with a match in the
	// text, ⌘K, the empty state, the banner, the find bar, and an input
	// method composing.
	dir := screensDir(t)
	groceries := "购物清单\n周六上午去超市，顺便取快递。\n- 牛奶\n- 鸡蛋\n- [x] 面包\n- [ ] 咖啡豆\n记得带**购物袋**和会员卡。\n"
	blocks := "# 一级标题\n正文第一段，15 号字，行高 22。\n## 二级标题\n正文里的 `行内代码`、**粗体**、*斜体*、~~删除线~~和[链接](https://gocast.app)。\n### 三级标题\n- 无序列表\n    - 第二层\n1. 有序列表\n2. 第二项\n- [ ] 没勾的任务\n- [x] 勾上的任务\n```\nfunc main() {\n\tfmt.Println(\"你好\")\n}\n```\n> 引用的第一段\n> 引用的第二段\n---\n分割线下面的正文。\n"
	list := []string{groceries, "周报\n本周完成了笔记功能的设计和原型。\n", "读书笔记\n《设计心理学》第三章：示能和意符。\n", "想法\n给悬浮终端加一个分屏。\n", "旅行\n订周五去杭州的高铁票。\n"}
	shot := func(name string, w, h int, dark bool, md []string, setup func(nt *notesTest)) {
		t.Helper()
		nt := notesTesterIn(t, "zh-Hans", md...)
		tt := nt.tt
		tt.SetSize(w, h)
		tt.SetScale(2)
		tt.SetDark(dark)
		tt.SetVibrancy(false)
		nt.frame()
		if setup != nil {
			setup(nt)
		}
		suffix := "-light"
		if dark {
			suffix = "-dark"
		}
		saveScreen(t, tt, dir, name+suffix, image.Rect(0, 0, w, h))
	}
	for _, dark := range []bool{false, true} {
		shot("notes-default", notesWidth, notesHeight, dark, list, nil)
		shot("notes-blocks", notesWidth, 820, dark, []string{blocks}, func(nt *notesTest) {
			nt.cur.doc.ScrollToParagraph(0, ui.ScrollStart)
			nt.frame()
		})
		shot("notes-browse-pinned", notesWidth, notesHeight, dark, list, func(nt *notesTest) {
			nt.store.SetPinned(noteID(0), true)
			nt.store.SetPinned(noteID(3), true)
			nt.key(ui.Cmd, ui.KeyP)
		})
		shot("notes-browse-match", notesWidth, notesHeight, dark, list, func(nt *notesTest) {
			nt.store.SetPinned(noteID(3), true)
			nt.key(ui.Cmd, ui.KeyP)
			nt.typ("高铁")
		})
		shot("notes-menu", notesWidth, notesHeight, dark, list, func(nt *notesTest) {
			nt.key(ui.Cmd, ui.KeyK)
		})
		shot("notes-empty", notesWidth, notesHeight, dark, list[:1], func(nt *notesTest) {
			useFakeTrash(t)
			nt.key(ui.Ctrl, ui.KeyX)
			nt.fire() // the notice gone
		})
		shot("notes-save-failed", notesWidth, notesHeight, dark, nil, func(nt *notesTest) {
			if err := os.WriteFile(nt.store.Dir(), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			nt.typ("会议记录")
			nt.fire()
		})
		shot("notes-find", notesWidth, notesHeight, dark, list, func(nt *notesTest) {
			nt.key(ui.Cmd, ui.KeyF)
			nt.typ("购物")
			nt.key(0, ui.KeyEnter)
		})
		shot("notes-ime", notesWidth, notesHeight, dark, list, func(nt *notesTest) {
			nt.cur.doc.SetSelection(4, 4) // after the title
			nt.frame()
			nt.tt.Compose("ni'hao", 6)
			nt.frame()
		})
	}
}
