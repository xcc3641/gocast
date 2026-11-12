package clipboard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImageFile(t *testing.T) {
	root := t.TempDir()
	imgDir := filepath.Join(root, "gocast-clipboard")
	dir := filepath.Join(root, "clipboard")
	b, clk := &fakeBoard{}, newClock()
	opts := Options{Pasteboard: b, Now: clk.now, ImageDir: imgDir}
	s, err := Open(dir, &MemoryKeys{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	img := testPNG(t, 5, 4)
	pic, _ := s.Add(Content{PNG: img}, safari)
	clk.t = clk.t.Add(time.Minute)
	pinned, _ := s.Add(Content{PNG: testPNG(t, 6, 4)}, safari)
	s.SetPinned(pinned.ID, true)
	text, _ := s.Add(Content{Text: "words"}, safari)

	if _, err := s.ImageFile(text.ID); !errors.Is(err, ErrNotImage) {
		t.Errorf("ImageFile of text: %v", err)
	}
	if _, err := s.ImageFile("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ImageFile of nothing: %v", err)
	}
	path, err := s.ImageFile(pic.ID)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(imgDir, pic.ID+".png") {
		t.Errorf("path %s", path)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, img) {
		t.Error("the file is not the image, decrypted")
	}
	if fi, _ := os.Stat(imgDir); fi.Mode().Perm() != 0o700 {
		t.Errorf("folder mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode %v", fi.Mode().Perm())
	}
	// Reused: not written again.
	mod := time.Now().Add(-time.Hour)
	os.Chtimes(path, mod, mod)
	if again, err := s.ImageFile(pic.ID); err != nil || again != path {
		t.Fatalf("again: %s, %v", again, err)
	}
	if fi, _ := os.Stat(path); !fi.ModTime().Equal(mod) {
		t.Error("the file was written again")
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
	if got := s.Entries(FilterAll); got[1].ID != pic.ID || !got[1].Time.Equal(clk.t) {
		t.Errorf("not moved to the top: %+v", got)
	}

	// Delete removes its file; Clear the others; a launch the rest.
	pinnedPath, _ := s.ImageFile(pinned.ID)
	stray := filepath.Join(imgDir, "stray.png")
	os.WriteFile(stray, img, 0o600)
	s.Delete(pic.ID)
	if _, err := os.Stat(path); err == nil {
		t.Error("Delete left the image file")
	}
	if _, err := os.Stat(pinnedPath); err != nil {
		t.Error("Delete removed another's file")
	}
	pic2, _ := s.Add(Content{PNG: img}, safari)
	path2, _ := s.ImageFile(pic2.ID)
	s.Clear()
	if _, err := os.Stat(path2); err == nil {
		t.Error("Clear left the image file")
	}
	s.Close()
	s2, err := Open(dir, &MemoryKeys{}, opts)
	_ = err // another key: unreadable, which does not matter here
	if _, err := os.Stat(stray); err == nil {
		t.Error("Open left an earlier run's file")
	}
	if _, err := os.Stat(pinnedPath); err == nil {
		t.Error("Open left an earlier run's file")
	}
	s2.Close()
}

func TestImageFileNoFolder(t *testing.T) {
	s, _, _ := memStore(t)
	pic, _ := s.Add(Content{PNG: testPNG(t, 2, 2)}, safari)
	if _, err := s.ImageFile(pic.ID); err == nil {
		t.Error("an image file with no folder")
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

	// Pinning keeps its time; unpinning makes it the newest.
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
