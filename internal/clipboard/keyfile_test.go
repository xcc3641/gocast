package clipboard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "keys", "clipboard.key")
	k := NewKeyFile(path)
	if _, err := k.Load(); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Load before Save: %v", err)
	}

	// The store makes the key on first open, outside its directory.
	dir := filepath.Join(root, "clipboard")
	clk := newClock()
	s := openStore(t, dir, k, clk)
	s.Add(Content{Text: "kept", HTML: []byte("<b>kept</b>")}, safari)
	s.Close()
	key, err := os.ReadFile(path)
	if err != nil || len(key) != KeySize {
		t.Fatalf("key file: %d bytes, %v", len(key), err)
	}
	if fi, _ := os.Stat(filepath.Dir(path)); fi.Mode().Perm() != 0o700 {
		t.Errorf("keys dir mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("key file mode %v", fi.Mode().Perm())
	}
	if names, _ := os.ReadDir(filepath.Dir(path)); len(names) != 1 {
		t.Errorf("keys dir holds %d files", len(names))
	}

	// Reopened, it reuses the key and reads the history.
	s2 := openStore(t, dir, NewKeyFile(path), clk)
	if got := s2.Entries(FilterAll); len(got) != 1 || got[0].Text != "kept" {
		t.Fatalf("reopened: %+v", got)
	}
	if c, err := s2.Content(s2.Entries(FilterAll)[0].ID); err != nil || string(c.HTML) != "<b>kept</b>" {
		t.Errorf("content: %v", err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(again, key) {
		t.Error("the key changed on reopening")
	}

	// A file that holds no key.
	os.WriteFile(path, []byte("junk"), 0o600)
	if _, err := k.Load(); !errors.Is(err, ErrBadKey) {
		t.Errorf("Load of junk: %v", err)
	}
}

func TestKeyFileMissingOrMalformed(t *testing.T) {
	for name, spoil := range map[string]func(path string){
		"missing":   func(path string) { os.Remove(path) },
		"malformed": func(path string) { os.WriteFile(path, []byte("not a key"), 0o600) },
		"other key": func(path string) { os.WriteFile(path, bytes.Repeat([]byte{9}, KeySize), 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "keys", "clipboard.key")
			dir := filepath.Join(root, "clipboard")
			clk := newClock()
			s := openStore(t, dir, NewKeyFile(path), clk)
			p, _ := s.Add(Content{Text: "pinned"}, safari)
			s.SetPinned(p.ID, true)
			s.Close()
			spoil(path)
			before, _ := os.ReadFile(filepath.Join(dir, indexName))

			s2, err := Open(dir, NewKeyFile(path), Options{Pasteboard: &fakeBoard{}, Now: clk.now})
			if !errors.Is(err, ErrUnreadable) || !s2.Unreadable() {
				t.Fatalf("Open: %v, unreadable %v", err, s2.Unreadable())
			}
			if after, _ := os.ReadFile(filepath.Join(dir, indexName)); !bytes.Equal(before, after) {
				t.Error("the history was touched")
			}
			old, _ := os.ReadFile(path)
			if err := s2.Clear(); err != nil {
				t.Fatal(err)
			}
			key, _ := os.ReadFile(path)
			if len(key) != KeySize || bytes.Equal(key, old) {
				t.Errorf("Clear: no new key (%d bytes)", len(key))
			}
			if s2.Unreadable() || s2.Len() != 0 {
				t.Errorf("after Clear: unreadable %v, %d entries", s2.Unreadable(), s2.Len())
			}
			s2.Add(Content{Text: "fresh"}, safari)
			s2.Close()
			s3 := openStore(t, dir, NewKeyFile(path), clk)
			if got := s3.Entries(FilterAll); len(got) != 1 || got[0].Text != "fresh" {
				t.Errorf("fresh history: %+v", got)
			}
		})
	}
}

// legacyKeys fails the test if the store asks it for the key of a
// version 1 history: those files are deleted unread.
type legacyKeys struct {
	t     *testing.T
	inner KeyStore
	files []string
}

func (k *legacyKeys) Load() ([]byte, error) {
	for _, f := range k.files {
		if _, err := os.Stat(f); err == nil {
			k.t.Errorf("the key was asked for while %s was there", f)
		}
	}
	return k.inner.Load()
}

func (k *legacyKeys) Save(key []byte) error { return k.inner.Save(key) }

func TestLegacyHistoryDeleted(t *testing.T) {
	for _, withNew := range []bool{false, true} {
		root := t.TempDir()
		dir := filepath.Join(root, "clipboard")
		path := filepath.Join(root, "keys", "clipboard.key")
		clk := newClock()
		if withNew {
			s := openStore(t, dir, NewKeyFile(path), clk)
			s.Add(Content{Text: "new format"}, safari)
			s.Close()
		}
		// Files of a Keychain-era history, which nothing could read.
		os.MkdirAll(filepath.Join(dir, legacyData), 0o700)
		legacy := []string{filepath.Join(dir, legacyIndex), filepath.Join(dir, legacyData, "abcdef"+blobExt)}
		for _, f := range legacy {
			if err := os.WriteFile(f, []byte("GCB1 sealed with a Keychain key"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		keys := &legacyKeys{t: t, inner: NewKeyFile(path), files: legacy}
		s, err := Open(dir, keys, Options{Pasteboard: &fakeBoard{}, Now: clk.now})
		if err != nil || s.Unreadable() {
			t.Fatalf("with new files %v: Open %v, unreadable %v", withNew, err, s.Unreadable())
		}
		for _, f := range legacy {
			if _, err := os.Stat(f); err == nil {
				t.Errorf("%s left", f)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, legacyData)); err == nil {
			t.Error("the old data folder left")
		}
		want := 0
		if withNew {
			want = 1
		}
		if s.Len() != want {
			t.Errorf("with new files %v: %d entries", withNew, s.Len())
		}
		if _, err := s.Add(Content{Text: "after"}, safari); err != nil {
			t.Error(err)
		}
	}
}
