package notes

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Notes as people write them come back byte for byte.
func TestCodecKeepsNotes(t *testing.T) {
	for _, in := range []string{
		"Title\nline two\n\nafter a blank\n\n\n\nthree blanks\n",
		"a\n---\nb\n",
		"#tag line\n#tag #other #标签\n",
		"use * and _ and snake_case and 2*3 and **real bold**\n",
		"see https://example.com/a_b?x=1 ok\n",
		"[t](https://e.com) and <u>under</u>\n",
		"中文 **粗体** 和 *斜体*，还有_下划线_和 `代码`\n",
		"- a\n    - b\n        - c\n- d\n",
		"- [ ] todo\n- [x] done\n    - [ ] nested\n",
		"1. one\n2. two\n    1. sub\n",
		"> quote\n> two\n",
		"```\nfmt.Println(1)\n\n  indented\n```\nafter\n",
		"| a | b |\n|---|---|\n| 1 | 2 |\n",
		"<div>html</div>\nx <span>y</span> <c> a<b\n",
		"\ttab start\n    four spaces\n",
		"emoji 😀 and a \\ backslash\n",
	} {
		if got := Encode(Decode(in, nil)); got != in {
			t.Errorf("%q\n came back %q", in, got)
		}
	}
}

// What the codec normalizes, forms of the same meaning, is stable.
func TestCodecNormalizes(t *testing.T) {
	for _, c := range []struct{ in, out string }{
		{"* star\n+ plus\n", "- star\n- plus\n"},
		{"3. three\n4. four\n", "1. three\n2. four\n"},
		{"___\n* * *\n", "---\n---\n"},
		{"__b__ _i_\n", "**b** *i*\n"},
		{"```go\ncode\n```\n", "```\ncode\n```\n"},
		{"no newline at the end", "no newline at the end\n"},
	} {
		if got := Encode(Decode(c.in, nil)); got != c.out {
			t.Errorf("%q → %q, want %q", c.in, got, c.out)
		}
		if got := Encode(Decode(c.out, nil)); got != c.out {
			t.Errorf("%q is not stable: %q", c.out, got)
		}
	}
}

// The title and the text a note is listed and searched by have none of
// the Markdown marks (copy.md 16.2).
func TestPlain(t *testing.T) {
	for _, c := range []struct{ md, title string }{
		{"# 周会 **记录**\nbody", "周会 记录"},
		{"- [ ] 买牛奶\n", "买牛奶"},
		{"- [x] done\n", "done"},
		{"> quoted [link](https://e.com)\n", "quoted link"},
		{"1. first\n", "first"},
		{"`code` and ~~gone~~ and 2\\*3\n", "code and gone and 2*3"},
		{"\nsecond line first", ""},
		{"", ""},
	} {
		lines := Plain(c.md)
		got := ""
		if len(lines) > 0 {
			got = lines[0]
		}
		if got != c.title {
			t.Errorf("%q: title %q, want %q", c.md, got, c.title)
		}
	}
	if got := Plain("a\n```\n# not a heading\n```\n---\nb"); !slices.Equal(got, []string{"a", "", "# not a heading", "", "", "b"}) {
		t.Errorf("code and rule: %q", got)
	}
}

// The search finds a note by its title or its text, whatever the case;
// found in the text alone, it gives the line from about 12 characters
// before what it found (design.md 11.6).
func TestSearch(t *testing.T) {
	list := []*Note{
		noteOf("a", "Shopping List\nmilk\n", time.Time{}, 0),
		noteOf("b", "周会\n这周要发布 Gocast 1.30 的笔记功能，记得先跟 Hugo 确认 milk 的事\n", time.Time{}, 0),
		noteOf("c", "nothing here\n", time.Time{}, 0),
	}
	got := Search(list, "MILK")
	if len(got) != 2 || got[0].Note.ID != "a" || got[1].Note.ID != "b" {
		t.Fatalf("found %v", got)
	}
	if got[0].InTitle || got[0].Snippet != "milk" {
		t.Errorf("a short line whole: %+v", got[0])
	}
	if want := "…得先跟 Hugo 确认 milk 的事"; got[1].Snippet != want {
		t.Errorf("snippet %q, want %q", got[1].Snippet, want)
	}
	if got := Search(list, "shop"); len(got) != 1 || !got[0].InTitle {
		t.Errorf("in the title: %+v", got)
	}
	if got := Search(list, "  "); len(got) != 3 {
		t.Errorf("empty query: %d", len(got))
	}
}

// The folder is made with the first note written, not before, 0700, the
// notes 0600 (notes brief).
func TestStoreWritesLazily(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Notes")
	s := Open(dir)
	if changed, err := s.Scan(); err != nil || changed || s.Len() != 0 {
		t.Fatalf("no folder: %v %v %d", changed, err, s.Len())
	}
	id := s.NewID(time.Date(2026, 11, 17, 9, 30, 0, 0, time.Local))
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the folder made before a note is written")
	}
	if err := Write(dir, id, "hello\n"); err != nil {
		t.Fatal(err)
	}
	s.Written(id, "hello\n")
	for path, mode := range map[string]os.FileMode{dir: 0o700, s.Path(id): 0o600} {
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Perm() != mode {
			t.Errorf("%s: %v %v", path, fi.Mode(), err)
		}
	}
	if n := s.Note(id); n == nil || n.Title() != "hello" {
		t.Errorf("written: %+v", n)
	}
	if other := s.NewID(time.Date(2026, 11, 17, 9, 30, 0, 0, time.Local)); other == id {
		t.Errorf("two notes made in one second have one ID: %q", id)
	}
}

// The list has the notes pinned first, in the order they were pinned,
// then the others, those changed last first; the index keeps the pins
// and the note shown last for the next launch.
func TestStoreOrderAndIndex(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 11, 17, 9, 0, 0, 0, time.Local)
	for i, id := range []string{"old", "mid", "new"} {
		if err := Write(dir, id, id+"\n"); err != nil {
			t.Fatal(err)
		}
		at := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(filepath.Join(dir, id+Ext), at, at); err != nil {
			t.Fatal(err)
		}
	}
	s := Open(dir)
	if _, err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	ids := func() []string {
		var out []string
		for _, n := range s.List() {
			out = append(out, n.ID)
		}
		return out
	}
	if got := ids(); !slices.Equal(got, []string{"new", "mid", "old"}) {
		t.Errorf("recent: %q", got)
	}
	_ = s.SetPinned("old", true)
	_ = s.SetPinned("mid", true)
	_ = s.SetLast("mid")
	if got := ids(); !slices.Equal(got, []string{"old", "mid", "new"}) {
		t.Errorf("pinned: %q", got)
	}
	again := Open(dir)
	if _, err := again.Scan(); err != nil {
		t.Fatal(err)
	}
	if got := again.PinnedIDs(); !slices.Equal(got, []string{"old", "mid"}) || again.Last() != "mid" {
		t.Errorf("index read back: %q, last %q", got, again.Last())
	}
}

// A note deleted goes to the Trash unpinned; put back there (the file
// back in the folder), the next scan lists it again, still unpinned
// (copy.md 16.6).
func TestStoreDeleteAndPutBack(t *testing.T) {
	dir, trash := t.TempDir(), t.TempDir()
	if err := Write(dir, "n", "note\n"); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	s.Trash = func(path string) error { return os.Rename(path, filepath.Join(trash, filepath.Base(path))) }
	if _, err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	_ = s.SetPinned("n", true)
	if err := s.Delete("n"); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 0 || s.Pinned("n") {
		t.Fatalf("deleted: %d notes, pinned %v", s.Len(), s.Pinned("n"))
	}
	if err := os.Rename(filepath.Join(trash, "n"+Ext), filepath.Join(dir, "n"+Ext)); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.Scan(); err != nil || !changed || s.Note("n") == nil || s.Pinned("n") {
		t.Errorf("put back: changed %v, err %v, note %v, pinned %v", changed, err, s.Note("n"), s.Pinned("n"))
	}
	// The Trash refusing it, the note stays.
	s.Trash = func(string) error { return errors.New("no trash") }
	if err := s.Delete("n"); err == nil || s.Note("n") == nil {
		t.Errorf("refused: %v", err)
	}
}

// A note changed outside Gocast is read again at the next scan; one not
// changed is not.
func TestStoreScanReadsChanges(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, "n", "before\n"); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	_, _ = s.Scan()
	if changed, _ := s.Scan(); changed {
		t.Error("nothing changed, yet changed")
	}
	if err := Write(dir, "n", "after, longer\n"); err != nil {
		t.Fatal(err)
	}
	if changed, _ := s.Scan(); !changed || s.Note("n").Title() != "after, longer" {
		t.Errorf("changed outside: %v %q", changed, s.Note("n").Title())
	}
}
