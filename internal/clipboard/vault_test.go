package clipboard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openStore(t *testing.T, dir string, keys KeyStore, clk *clock) *Store {
	t.Helper()
	s, err := Open(dir, keys, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestPersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "clipboard")
	keys := &MemoryKeys{}
	clk := newClock()
	s := openStore(t, dir, keys, clk)
	if !s.Persistent() || len(keys.Key) != KeySize {
		t.Fatalf("persistent %v, key %d bytes", s.Persistent(), len(keys.Key))
	}
	img := testPNG(t, 30, 20)
	secret := "a very secret sentence 机密"
	text, _ := s.Add(Content{Text: secret, RTF: []byte("{\\rtf1 secret-rtf}"), HTML: []byte("<i>secret-html</i>")}, safari)
	clk.t = clk.t.Add(time.Second)
	pic, _ := s.Add(Content{PNG: img}, App{BundleID: "com.apple.Preview", Name: "Preview"})
	clk.t = clk.t.Add(time.Second)
	files, _ := s.Add(Content{Files: []string{"/Users/me/secret-file.txt"}}, safari)
	s.SetPinned(text.ID, true)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Private permissions, and nothing readable in the clear.
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %v", fi.Mode().Perm())
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
		b, _ := os.ReadFile(path)
		for _, plain := range []string{"secret", "Safari", "Preview", "\x89PNG"} {
			if bytes.Contains(b, []byte(plain)) {
				t.Errorf("%s contains %q in the clear", path, plain)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Reopen.
	clk.t = clk.t.Add(time.Hour)
	s2 := openStore(t, dir, keys, clk)
	got := s2.Entries(FilterAll)
	if len(got) != 3 || got[0].ID != text.ID || !got[0].Pinned || got[1].ID != files.ID || got[2].ID != pic.ID {
		t.Fatalf("reloaded %+v", got)
	}
	if got[2].Width != 30 || len(got[2].Thumbnail) == 0 || got[0].App != (App{BundleID: safari.BundleID, Name: safari.Name}) {
		t.Errorf("reloaded metadata %+v / %+v", got[2], got[0])
	}
	c, err := s2.Content(text.ID)
	if err != nil || c.Text != secret || string(c.RTF) != "{\\rtf1 secret-rtf}" || string(c.HTML) != "<i>secret-html</i>" {
		t.Errorf("rich content %+v, %v", c, err)
	}
	c, err = s2.Content(pic.ID)
	if err != nil || !bytes.Equal(c.PNG, img) {
		t.Errorf("image content: %v", err)
	}
	if res := s2.Search("机密", FilterAll); len(res) != 1 {
		t.Errorf("search after reload: %d", len(res))
	}

	// A dedup bump and a delete persist; the data file goes with it.
	s2.Add(Content{Text: secret}, safari) // the same text, now plain
	if err := s2.Delete(pic.ID); err != nil {
		t.Fatal(err)
	}
	s2.Flush()
	if n := len((&vault{dir: dir}).blobIDs()); n != 0 {
		t.Errorf("%d data files left", n)
	}
	s3 := openStore(t, dir, keys, clk)
	if got := s3.Entries(FilterAll); len(got) != 2 || got[0].Kind != KindText {
		t.Errorf("after delete: %+v", got)
	}

	// Retention prunes on load.
	clk.t = clk.t.Add(31 * 24 * time.Hour)
	s4 := openStore(t, dir, keys, clk)
	if got := s4.Entries(FilterAll); len(got) != 1 || !got[0].Pinned {
		t.Errorf("after a month: %+v", got)
	}
	s5 := openStore(t, dir, keys, clk)
	if s5.Len() != 1 {
		t.Errorf("prune not saved: %d", s5.Len())
	}
}

func TestOrphanAndTempFilesRemoved(t *testing.T) {
	dir := t.TempDir()
	keys := &MemoryKeys{}
	clk := newClock()
	s := openStore(t, dir, keys, clk)
	s.Add(Content{Text: "x", HTML: []byte("<b>x</b>")}, safari)
	s.Close()
	orphan := filepath.Join(dir, dataDir, "abcdef"+blobExt)
	tmp := filepath.Join(dir, tmpPrefix+"123")
	os.WriteFile(orphan, []byte("junk"), 0o600)
	os.WriteFile(tmp, []byte("junk"), 0o600)
	s2 := openStore(t, dir, keys, clk)
	for _, p := range []string{orphan, tmp} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s not removed", p)
		}
	}
	if c, err := s2.Content(s2.Entries(FilterAll)[0].ID); err != nil || string(c.HTML) != "<b>x</b>" {
		t.Errorf("kept data lost: %v", err)
	}
}

func TestUnreadable(t *testing.T) {
	for name, spoil := range map[string]func(dir string, keys *MemoryKeys){
		"key missing": func(_ string, keys *MemoryKeys) { keys.Key = nil },
		"other key": func(_ string, keys *MemoryKeys) {
			keys.Key = bytes.Repeat([]byte{7}, KeySize)
		},
		"corrupt index": func(dir string, _ *MemoryKeys) {
			p := filepath.Join(dir, indexName)
			b, _ := os.ReadFile(p)
			b[len(b)-1] ^= 1
			os.WriteFile(p, b, 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			keys := &MemoryKeys{}
			clk := newClock()
			s := openStore(t, dir, keys, clk)
			p, _ := s.Add(Content{Text: "pinned"}, safari)
			s.SetPinned(p.ID, true)
			s.Add(Content{PNG: testPNG(t, 3, 3)}, safari)
			s.Close()
			spoil(dir, keys)
			before, _ := os.ReadFile(filepath.Join(dir, indexName))

			s2, err := Open(dir, keys, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
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
			after, _ := os.ReadFile(filepath.Join(dir, indexName))
			if !bytes.Equal(before, after) {
				t.Error("the unreadable history was overwritten")
			}

			if err := s2.Clear(); err != nil {
				t.Fatal(err)
			}
			if s2.Unreadable() || s2.Err() != nil || !s2.Persistent() || s2.Len() != 0 || s2.CanClear() {
				t.Errorf("after Clear: unreadable %v, err %v, len %d", s2.Unreadable(), s2.Err(), s2.Len())
			}
			if n := len((&vault{dir: dir}).blobIDs()); n != 0 {
				t.Errorf("%d old data files left", n)
			}
			if _, err := s2.Add(Content{Text: "fresh"}, safari); err != nil {
				t.Fatal(err)
			}
			s2.Close()
			s3 := openStore(t, dir, keys, clk)
			if got := s3.Entries(FilterAll); len(got) != 1 || got[0].Text != "fresh" {
				t.Errorf("fresh history: %+v", got)
			}
		})
	}
}

func TestKeychainUnavailable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "h")
	clk := newClock()
	s := openStore(t, dir, &MemoryKeys{}, clk)
	s.Add(Content{Text: "kept on disk"}, safari)
	s.Close()

	denied := &MemoryKeys{Err: errors.New("user canceled")}
	s2, err := Open(dir, denied, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
	if !errors.Is(err, ErrKeychain) || s2.Persistent() || s2.Unreadable() {
		t.Fatalf("Open: %v, persistent %v, unreadable %v", err, s2.Persistent(), s2.Unreadable())
	}
	if _, err := s2.Add(Content{Text: "memory only", HTML: []byte("<b>m</b>")}, safari); err != nil {
		t.Fatal(err)
	}
	e := s2.Search("memory", FilterAll)[0]
	if c, err := s2.Content(e.ID); err != nil || string(c.HTML) != "<b>m</b>" {
		t.Errorf("in-memory content: %v", err)
	}
	s2.Close()
	if (&vault{dir: dir}).blobIDs() != nil {
		t.Error("wrote data without a key")
	}
	if s, err := Open(dir, nil, Options{Pasteboard: &fakeBoard{}}); !errors.Is(err, ErrKeychain) || s == nil {
		t.Errorf("nil keys: %v", err)
	}
}

func TestBlobFraming(t *testing.T) {
	c := Content{RTF: []byte("r"), PNG: []byte{0, 1, 2}}
	r, h, p, err := decodeBlob(encodeBlob(c))
	if err != nil || string(r) != "r" || h != nil || !bytes.Equal(p, c.PNG) {
		t.Errorf("%q %q %v %v", r, h, p, err)
	}
	if _, _, _, err := decodeBlob([]byte{200}); err == nil {
		t.Error("decoded junk")
	}
}

func TestVaultRejectsSwappedFiles(t *testing.T) {
	v, _ := newVault(t.TempDir(), bytes.Repeat([]byte{1}, KeySize))
	v.prepare()
	v.write(blobName("aa"), []byte("one"))
	v.write(blobName("bb"), []byte("two"))
	b, _ := os.ReadFile(filepath.Join(v.dir, dataDir, "aa"+blobExt))
	os.WriteFile(filepath.Join(v.dir, dataDir, "bb"+blobExt), b, 0o600)
	if _, err := v.read(blobName("bb")); !errors.Is(err, ErrUnreadable) {
		t.Errorf("read a swapped file: %v", err)
	}
	if got, err := v.read(blobName("aa")); err != nil || string(got) != "one" {
		t.Errorf("%q %v", got, err)
	}
}
