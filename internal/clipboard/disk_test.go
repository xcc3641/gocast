package clipboard

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func openStore(t *testing.T, dir string, clk *clock) *Store {
	t.Helper()
	s, err := Open(dir, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

// dataFiles lists the files under data/.
func dataFiles(dir string) []string {
	names, _ := os.ReadDir(filepath.Join(dir, dataDir))
	var out []string
	for _, n := range names {
		out = append(out, n.Name())
	}
	return out
}

func TestPersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "clipboard")
	clk := newClock()
	s := openStore(t, dir, clk)
	if !s.Persistent() {
		t.Fatal("not persistent")
	}
	img := testPNG(t, 30, 20)
	words := "a sentence 句子"
	text, _ := s.Add(Content{Text: words, RTF: []byte("{\\rtf1 rtf}"), HTML: []byte("<i>html</i>")}, safari)
	clk.t = clk.t.Add(time.Second)
	pic, _ := s.Add(Content{PNG: img}, App{BundleID: "com.apple.Preview", Name: "Preview"})
	clk.t = clk.t.Add(time.Second)
	files, _ := s.Add(Content{Files: []string{"/Users/me/file.txt"}}, safari)
	s.SetPinned(text.ID, true)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Plain files, private to the user, written whole.
	for _, d := range []string{dir, filepath.Join(dir, dataDir)} {
		if fi, _ := os.Stat(d); fi.Mode().Perm() != 0o700 {
			t.Errorf("%s mode %v", d, fi.Mode().Perm())
		}
	}
	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v", path, fi.Mode().Perm())
		}
		if strings.HasPrefix(fi.Name(), tmpPrefix) {
			t.Errorf("left %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{pic.ID + ".png", text.ID + ".html", text.ID + ".rtf"}
	slices.Sort(want)
	if got := dataFiles(dir); !slices.Equal(got, want) {
		t.Errorf("data files %v, want %v", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, dataDir, pic.ID+".png")); !bytes.Equal(b, img) {
		t.Error("the image file is not the PNG")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, dataDir, text.ID+".html")); string(b) != "<i>html</i>" {
		t.Errorf("the HTML file holds %q", b)
	}
	var idx index
	b, _ := os.ReadFile(filepath.Join(dir, indexName))
	if err := json.Unmarshal(b, &idx); err != nil || idx.Version != 3 || len(idx.Entries) != 3 {
		t.Fatalf("history.json: %v, version %d, %d entries", err, idx.Version, len(idx.Entries))
	}

	// Reopen.
	clk.t = clk.t.Add(time.Hour)
	s2 := openStore(t, dir, clk)
	got := s2.Entries(FilterAll)
	if len(got) != 3 || got[0].ID != text.ID || !got[0].Pinned || got[1].ID != files.ID || got[2].ID != pic.ID {
		t.Fatalf("reloaded %+v", got)
	}
	if got[2].Width != 30 || len(got[2].Thumbnail) == 0 || got[0].App != safari {
		t.Errorf("reloaded metadata %+v / %+v", got[2], got[0])
	}
	c, err := s2.Content(text.ID)
	if err != nil || c.Text != words || string(c.RTF) != "{\\rtf1 rtf}" || string(c.HTML) != "<i>html</i>" {
		t.Errorf("rich content %+v, %v", c, err)
	}
	c, err = s2.Content(pic.ID)
	if err != nil || !bytes.Equal(c.PNG, img) {
		t.Errorf("image content: %v", err)
	}
	if res := s2.Search("句子", FilterAll); len(res) != 1 {
		t.Errorf("search after reload: %d", len(res))
	}

	// The same text, plain now: its formatting files go. A delete takes
	// the image's.
	s2.Add(Content{Text: words}, safari)
	if err := s2.Delete(pic.ID); err != nil {
		t.Fatal(err)
	}
	s2.Flush()
	if n := dataFiles(dir); len(n) != 0 {
		t.Errorf("data files left: %v", n)
	}
	s3 := openStore(t, dir, clk)
	if got := s3.Entries(FilterAll); len(got) != 2 || got[0].Kind != KindText {
		t.Errorf("after delete: %+v", got)
	}

	// Retention prunes on load.
	clk.t = clk.t.Add(31 * 24 * time.Hour)
	s4 := openStore(t, dir, clk)
	if got := s4.Entries(FilterAll); len(got) != 1 || !got[0].Pinned {
		t.Errorf("after a month: %+v", got)
	}
	s5 := openStore(t, dir, clk)
	if s5.Len() != 1 {
		t.Errorf("prune not saved: %d", s5.Len())
	}
}

func TestDelayedSave(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir, newClock())
	s.Add(Content{Text: "later"}, safari)
	if _, err := os.Stat(filepath.Join(dir, indexName)); err == nil {
		t.Error("the index was saved at once")
	}
	s.Close()
	if b, _ := os.ReadFile(filepath.Join(dir, indexName)); !bytes.Contains(b, []byte("later")) {
		t.Error("Close did not save")
	}
}

func TestOrphanAndTempFilesRemoved(t *testing.T) {
	dir := t.TempDir()
	clk := newClock()
	s := openStore(t, dir, clk)
	e, _ := s.Add(Content{Text: "x", HTML: []byte("<b>x</b>")}, safari)
	s.Close()
	junk := []string{
		filepath.Join(dir, dataDir, "abcdef.png"),    // no such entry
		filepath.Join(dir, dataDir, e.ID+".rtf"),     // not one of its parts
		filepath.Join(dir, dataDir, "not-an-id.png"), // not an entry's
		filepath.Join(dir, tmpPrefix+"123"),
		filepath.Join(dir, dataDir, tmpPrefix+"456"),
	}
	for _, p := range junk {
		os.WriteFile(p, []byte("junk"), 0o600)
	}
	s2 := openStore(t, dir, clk)
	for _, p := range junk {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s not removed", p)
		}
	}
	if c, err := s2.Content(e.ID); err != nil || string(c.HTML) != "<b>x</b>" {
		t.Errorf("kept data lost: %v", err)
	}
}

func TestUnreadable(t *testing.T) {
	dir := t.TempDir()
	clk := newClock()
	s := openStore(t, dir, clk)
	p, _ := s.Add(Content{Text: "pinned"}, safari)
	s.SetPinned(p.ID, true)
	s.Add(Content{PNG: testPNG(t, 3, 3)}, safari)
	s.Close()
	idx := filepath.Join(dir, indexName)
	b, _ := os.ReadFile(idx)
	os.WriteFile(idx, b[:len(b)/2], 0o600) // cut short
	before, _ := os.ReadFile(idx)
	files := dataFiles(dir)

	s2, err := Open(dir, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
	if !errors.Is(err, ErrUnreadable) || !s2.Unreadable() || !errors.Is(s2.Err(), ErrUnreadable) {
		t.Fatalf("Open: %v, unreadable %v", err, s2.Unreadable())
	}
	if s2.Len() != 0 || !s2.CanClear() {
		t.Errorf("len %d, CanClear %v", s2.Len(), s2.CanClear())
	}
	if _, err := s2.Add(Content{Text: "new"}, safari); !errors.Is(err, ErrUnreadable) {
		t.Errorf("Add while unreadable: %v", err)
	}
	s2.Flush()
	if after, _ := os.ReadFile(idx); !bytes.Equal(before, after) {
		t.Error("the unreadable history was overwritten")
	}
	if got := dataFiles(dir); !slices.Equal(got, files) {
		t.Errorf("data files touched: %v, were %v", got, files)
	}

	if err := s2.Clear(); err != nil {
		t.Fatal(err)
	}
	if s2.Unreadable() || s2.Err() != nil || !s2.Persistent() || s2.Len() != 0 || s2.CanClear() {
		t.Errorf("after Clear: unreadable %v, err %v, len %d", s2.Unreadable(), s2.Err(), s2.Len())
	}
	if n := dataFiles(dir); len(n) != 0 {
		t.Errorf("old data files left: %v", n)
	}
	if _, err := s2.Add(Content{Text: "fresh"}, safari); err != nil {
		t.Fatal(err)
	}
	s2.Close()
	if got := openStore(t, dir, clk).Entries(FilterAll); len(got) != 1 || got[0].Text != "fresh" {
		t.Errorf("fresh history: %+v", got)
	}
}

func TestDirectoryUnusable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a file")
	os.WriteFile(file, nil, 0o600)
	s, err := Open(filepath.Join(file, "clipboard"), Options{Pasteboard: &fakeBoard{}, Now: newClock().now})
	if err == nil || s.Persistent() || s.Unreadable() {
		t.Fatalf("Open: %v, persistent %v", err, s.Persistent())
	}
	e, err := s.Add(Content{Text: "memory only", HTML: []byte("<b>m</b>")}, safari)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := s.Content(e.ID); err != nil || string(c.HTML) != "<b>m</b>" {
		t.Errorf("in-memory content: %v", err)
	}
}

// Format 1, sealed with a Keychain key: deleted unread.
func TestFormat1Deleted(t *testing.T) {
	for _, withNew := range []bool{false, true} {
		dir := t.TempDir()
		clk := newClock()
		if withNew {
			s := openStore(t, dir, clk)
			s.Add(Content{Text: "kept", HTML: []byte("<b>kept</b>")}, safari)
			s.Close()
		}
		os.MkdirAll(filepath.Join(dir, dataDir), 0o700)
		old := []string{filepath.Join(dir, "history.gcb"), filepath.Join(dir, dataDir, "abcdef.gcb")}
		for _, f := range old {
			os.WriteFile(f, []byte("GCB1 sealed with a Keychain key"), 0o600)
		}
		s, err := Open(dir, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
		if err != nil || s.Unreadable() {
			t.Fatalf("Open: %v, unreadable %v", err, s.Unreadable())
		}
		for _, f := range old {
			if _, err := os.Stat(f); err == nil {
				t.Errorf("%s left", f)
			}
		}
		if withNew {
			e := s.Entries(FilterAll)
			if len(e) != 1 {
				t.Fatalf("%d entries", len(e))
			}
			if c, err := s.Content(e[0].ID); err != nil || string(c.HTML) != "<b>kept</b>" {
				t.Errorf("format 3 data lost: %v", err)
			}
		} else if s.Len() != 0 {
			t.Errorf("%d entries", s.Len())
		}
	}
}

// --- Format 2, as the builds that encrypted the history wrote it ---

func seal2(t *testing.T, key []byte, name string, plain []byte) []byte {
	t.Helper()
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce)
	out := append([]byte(format2Magic), nonce...)
	return aead.Seal(out, nonce, plain, []byte(name))
}

func blob2(c Content) []byte {
	var out []byte
	for _, f := range [][]byte{c.RTF, c.HTML, c.PNG} {
		out = binary.AppendUvarint(out, uint64(len(f)))
		out = append(out, f...)
	}
	return out
}

// writeFormat2 writes a format 2 history in dir, its key in keyPath, and
// returns its entries.
func writeFormat2(t *testing.T, dir, keyPath string, clk *clock, img []byte) []format2Record {
	t.Helper()
	key := bytes.Repeat([]byte{5}, format2Key)
	os.MkdirAll(filepath.Dir(keyPath), 0o700)
	os.WriteFile(keyPath, key, 0o600)
	os.MkdirAll(filepath.Join(dir, format2Data), 0o700)
	notes := App{BundleID: "com.apple.Notes", Name: "备忘录"}
	recs := []format2Record{
		{record: record{ID: "aa01", Kind: KindRichText, Text: "rich 富文本", App: notes, Time: clk.t.Add(-2 * time.Hour), Size: 40, Pinned: true, Hash: "h1"}, Blob: true},
		{record: record{ID: "aa02", Kind: KindImage, App: safari, Time: clk.t.Add(-time.Hour), Size: int64(len(img)), Width: 7, Height: 5, Thumbnail: []byte("thumb"), Hash: "h2"}, Blob: true},
		{record: record{ID: "aa03", Kind: KindText, Text: "plain", App: safari, Time: clk.t.Add(-30 * time.Minute), Size: 5, Hash: hash(Content{Text: "plain"}, KindText)}},
		{record: record{ID: "aa04", Kind: KindFiles, Text: "/x/y", Files: []string{"/x/y"}, App: safari, Time: clk.t.Add(-25 * time.Hour), Size: 4, Hash: "h4"}},
	}
	write := func(name string, plain []byte) {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), seal2(t, key, name, plain), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("data2/aa01.gcb", blob2(Content{RTF: []byte("{\\rtf1 r}"), HTML: []byte("<b>r</b>")}))
	write("data2/aa02.gcb", blob2(Content{PNG: img}))
	idx, _ := json.Marshal(struct {
		Version int             `json:"version"`
		Entries []format2Record `json:"entries"`
	}{1, recs})
	write(format2Index, idx)
	return recs
}

func TestFormat2Migrated(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "clipboard")
	keyPath := filepath.Join(root, "keys", "clipboard.key")
	clk := newClock()
	img := testPNG(t, 7, 5)
	recs := writeFormat2(t, dir, keyPath, clk, img)

	s, err := Open(dir, Options{Pasteboard: &fakeBoard{}, Now: clk.now, Format2Key: keyPath})
	if err != nil || s.Unreadable() {
		t.Fatalf("Open: %v, unreadable %v", err, s.Unreadable())
	}
	for _, gone := range []string{filepath.Join(dir, format2Index), filepath.Join(dir, format2Data), keyPath, filepath.Dir(keyPath)} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("%s left", gone)
		}
	}
	got := s.Entries(FilterAll)
	if len(got) != 4 {
		t.Fatalf("%d entries: %+v", len(got), got)
	}
	// Pins, times, sources and order kept.
	order := []string{"aa01", "aa03", "aa02", "aa04"}
	for i, e := range got {
		if e.ID != order[i] {
			t.Errorf("entry %d is %s, want %s", i, e.ID, order[i])
		}
		var r record
		for _, x := range recs {
			if x.ID == e.ID {
				r = x.record
			}
		}
		if !e.Time.Equal(r.Time) || e.App != r.App || e.Pinned != r.Pinned || e.Text != r.Text || e.Kind != r.Kind {
			t.Errorf("%s: %+v, was %+v", e.ID, e, r)
		}
	}
	if got[2].Width != 7 || string(got[2].Thumbnail) != "thumb" {
		t.Errorf("image metadata %+v", got[2])
	}
	c, err := s.Content("aa01")
	if err != nil || string(c.RTF) != "{\\rtf1 r}" || string(c.HTML) != "<b>r</b>" || c.Text != "rich 富文本" {
		t.Errorf("rich text: %+v, %v", c, err)
	}
	c, err = s.Content("aa02")
	if err != nil || !bytes.Equal(c.PNG, img) {
		t.Errorf("image: %v", err)
	}
	if c, _ := s.Content("aa04"); !slices.Equal(c.Files, []string{"/x/y"}) {
		t.Errorf("files %v", c.Files)
	}
	if p, err := s.ImagePath("aa02"); err != nil || p != filepath.Join(dir, dataDir, "aa02.png") {
		t.Errorf("image path %s, %v", p, err)
	}
	// The dedup of later copies still works: the hash came along.
	if e, _ := s.Add(Content{Text: "plain"}, safari); e.ID != "aa03" {
		t.Errorf("a copy of a migrated entry is a new one: %s", e.ID)
	}
	s.Close()
	// Saved as format 3.
	s2 := openStore(t, dir, clk)
	if s2.Len() != 4 {
		t.Errorf("reopened: %d entries", s2.Len())
	}
	if c, err := s2.Content("aa01"); err != nil || string(c.HTML) != "<b>r</b>" {
		t.Errorf("reopened rich text: %v", err)
	}
}

func TestFormat2WithoutKeyDeleted(t *testing.T) {
	for name, spoil := range map[string]func(keyPath string){
		"key missing":   func(p string) { os.Remove(p) },
		"key malformed": func(p string) { os.WriteFile(p, []byte("short"), 0o600) },
		"another key":   func(p string) { os.WriteFile(p, bytes.Repeat([]byte{9}, format2Key), 0o600) },
		"no key given":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "clipboard")
			keyPath := filepath.Join(root, "keys", "clipboard.key")
			clk := newClock()
			writeFormat2(t, dir, keyPath, clk, testPNG(t, 2, 2))
			given := keyPath
			if spoil != nil {
				spoil(keyPath)
			} else {
				given = ""
			}
			s, err := Open(dir, Options{Pasteboard: &fakeBoard{}, Now: clk.now, Format2Key: given})
			if err != nil || s.Unreadable() || s.Len() != 0 {
				t.Fatalf("Open: %v, unreadable %v, %d entries", err, s.Unreadable(), s.Len())
			}
			gone := []string{filepath.Join(dir, format2Index), filepath.Join(dir, format2Data)}
			if given != "" {
				gone = append(gone, filepath.Dir(keyPath))
			}
			for _, p := range gone {
				if _, err := os.Stat(p); err == nil {
					t.Errorf("%s left", p)
				}
			}
			if _, err := s.Add(Content{Text: "fresh"}, safari); err != nil {
				t.Error(err)
			}
		})
	}
}

// A migration interrupted after writing history.json: the format 2 files
// go, the format 3 history stays.
func TestFormat2AfterMigration(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "clipboard")
	keyPath := filepath.Join(root, "keys", "clipboard.key")
	clk := newClock()
	s := openStore(t, dir, clk)
	s.Add(Content{Text: "format 3"}, safari)
	s.Close()
	writeFormat2(t, dir, keyPath, clk, testPNG(t, 2, 2))
	s2, err := Open(dir, Options{Pasteboard: &fakeBoard{}, Now: clk.now, Format2Key: keyPath})
	if err != nil || s2.Len() != 1 || s2.Entries(FilterAll)[0].Text != "format 3" {
		t.Fatalf("Open: %v, %+v", err, s2.Entries(FilterAll))
	}
	if _, err := os.Stat(filepath.Join(dir, format2Index)); err == nil {
		t.Error("format 2 index left")
	}
}

func TestImagePath(t *testing.T) {
	dir := t.TempDir()
	b, clk := &fakeBoard{}, newClock()
	s, err := Open(dir, Options{Pasteboard: b, Now: clk.now})
	if err != nil {
		t.Fatal(err)
	}
	img := testPNG(t, 5, 4)
	pic, _ := s.Add(Content{PNG: img}, safari)
	clk.t = clk.t.Add(time.Minute)
	text, _ := s.Add(Content{Text: "words"}, safari)

	if _, err := s.ImagePath(text.ID); !errors.Is(err, ErrNotImage) {
		t.Errorf("ImagePath of text: %v", err)
	}
	if _, err := s.ImagePath("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ImagePath of nothing: %v", err)
	}
	path, err := s.ImagePath(pic.ID)
	if err != nil || path != filepath.Join(dir, dataDir, pic.ID+".png") {
		t.Fatalf("ImagePath: %s, %v", path, err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, img) {
		t.Error("the file is not the image")
	}

	// Pasted as its path, plain text, the entry moved to the top.
	defer func(t func() bool, k func() error) { trusted, keystroke = t, k }(trusted, keystroke)
	pressed := 0
	trusted = func() bool { return true }
	keystroke = func() error { pressed++; return nil }
	clk.t = clk.t.Add(time.Minute)
	if err := s.PasteImagePath(pic.ID); err != nil || pressed != 1 {
		t.Fatalf("PasteImagePath: %v, %d", err, pressed)
	}
	if w := b.writes[len(b.writes)-1]; w.Text != path || w.PNG != nil {
		t.Errorf("wrote %+v", w)
	}
	if got := s.Entries(FilterAll); got[0].ID != pic.ID || !got[0].Time.Equal(clk.t) {
		t.Errorf("not moved to the top: %+v", got[0])
	}

	// The path holds while the entry moves, is pinned, copied again, and
	// the store reopens.
	s.SetPinned(pic.ID, true)
	s.SetPinned(pic.ID, false)
	s.Add(Content{PNG: img}, safari)
	s.Close()
	s2 := openStore(t, dir, clk)
	if again, err := s2.ImagePath(pic.ID); err != nil || again != path {
		t.Errorf("after moves: %s, %v", again, err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, img) {
		t.Error("the file changed")
	}

	// A store in memory has no file to paste.
	m, _, _ := memStore(t)
	mp, _ := m.Add(Content{PNG: img}, safari)
	if _, err := m.ImagePath(mp.ID); !errors.Is(err, ErrNotImage) {
		t.Errorf("in memory: %v", err)
	}
}

func TestUnpinToTop(t *testing.T) {
	s, _, clk := memStore(t)
	old, _ := s.Add(Content{Text: "pinned long ago"}, safari)
	s.SetPinned(old.ID, true)
	clk.t = clk.t.Add(72 * time.Hour)
	s.Add(Content{Text: "yesterday"}, safari)
	clk.t = clk.t.Add(24 * time.Hour)
	s.Add(Content{Text: "today"}, safari)
	clk.t = clk.t.Add(time.Minute)

	if err := s.SetPinned(old.ID, false); err != nil {
		t.Fatal(err)
	}
	got := s.Entries(FilterAll)
	if got[0].ID != old.ID || got[0].Pinned || !got[0].Time.Equal(clk.t) {
		t.Fatalf("unpinned: %+v", got[0])
	}
	if SectionOf(got[0], clk.t) != SectionToday {
		t.Errorf("section %v", SectionOf(got[0], clk.t))
	}
	if got[1].Text != "today" || got[2].Text != "yesterday" {
		t.Errorf("order %q %q", got[1].Text, got[2].Text)
	}
	// Pinning moves nothing in time.
	was := got[2]
	clk.t = clk.t.Add(time.Minute)
	s.SetPinned(was.ID, true)
	if e, _ := s.Get(was.ID); !e.Time.Equal(was.Time) {
		t.Error("pinning changed the time")
	}
	// Unpinning what is not pinned changes nothing.
	s.SetPinned(got[1].ID, false)
	if e, _ := s.Get(got[1].ID); !e.Time.Equal(got[1].Time) {
		t.Error("unpinning an unpinned entry changed its time")
	}
}
