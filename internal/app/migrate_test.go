package app

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// node is what a test compares of a file, folder or link.
type node struct {
	mode fs.FileMode
	mod  time.Time // not for links
	data string    // a file's contents, a link's target
}

// tree records what is under root, by path relative to it ("." for root).
func tree(t *testing.T, root string) map[string]node {
	t.Helper()
	m := map[string]node{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		n := node{mode: fi.Mode()}
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			n.data = target
		case fi.Mode().IsRegular():
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			n.data, n.mod = string(b), fi.ModTime()
		default:
			n.mod = fi.ModTime()
		}
		m[rel] = n
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// legacyDir makes base/dev.gocast.app as Gocast leaves it: folders 0700,
// files 0600 (one 0644), a link, and times of days ago, set after the
// contents.
func legacyDir(t *testing.T, base string) string {
	t.Helper()
	old := filepath.Join(base, legacyIdentifier)
	clip := filepath.Join(old, "clipboard")
	data := filepath.Join(clip, "data")
	for _, d := range []string{old, clip, data} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]fs.FileMode{
		"settings.json":           0o600,
		"presets.json":            0o644,
		"rates.json":              0o600,
		"clipboard/index.json":    0o600,
		"clipboard/data/a1.png":   0o600,
		"clipboard/data/a1.plain": 0o600,
	}
	day := time.Date(2026, 11, 2, 9, 30, 15, 123456789, time.Local)
	i := 0
	for name, mode := range files {
		p := filepath.Join(old, filepath.FromSlash(name))
		if err := os.WriteFile(p, []byte("data of "+name), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil { // whatever the umask
			t.Fatal(err)
		}
		i++
		if err := os.Chtimes(p, day, day.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("data/a1.png", filepath.Join(clip, "latest")); err != nil {
		t.Fatal(err)
	}
	for i, d := range []string{data, clip, old} {
		if err := os.Chtimes(d, day, day.Add(-time.Duration(i+1)*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	return old
}

func TestMigrateDataDirCopies(t *testing.T) {
	base := t.TempDir()
	old := legacyDir(t, base)
	before := tree(t, old)

	copied, err := migrateDataDir(base, legacyIdentifier, identifier)
	if err != nil || !copied {
		t.Fatalf("migrateDataDir = %v, %v; want true, nil", copied, err)
	}
	got := tree(t, filepath.Join(base, identifier))
	if !maps.EqualFunc(got, before, func(a, b node) bool {
		return a.mode == b.mode && a.mod.Equal(b.mod) && a.data == b.data
	}) {
		t.Errorf("the copy differs:\n got %v\nwant %v", got, before)
	}
	if n := got["clipboard/latest"]; n.mode&fs.ModeSymlink == 0 || n.data != "data/a1.png" {
		t.Errorf("clipboard/latest = %v, want a link to data/a1.png", n)
	}
	if n := got["."]; n.mode != fs.ModeDir|0o700 {
		t.Errorf("the folder's mode = %v, want drwx------", n.mode)
	}
	if n := got["presets.json"]; n.mode != 0o644 {
		t.Errorf("presets.json's mode = %v, want -rw-r--r--", n.mode)
	}
	if after := tree(t, old); !maps.Equal(after, before) {
		t.Errorf("the old folder changed:\n got %v\nwant %v", after, before)
	}
	names, _ := os.ReadDir(base)
	if len(names) != 2 {
		t.Errorf("base holds %v, want the two folders only", names)
	}

	// Once copied, never again.
	if err := os.WriteFile(filepath.Join(old, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if copied, err := migrateDataDir(base, legacyIdentifier, identifier); err != nil || copied {
		t.Fatalf("second migrateDataDir = %v, %v; want false, nil", copied, err)
	}
	if b, _ := os.ReadFile(filepath.Join(base, identifier, "settings.json")); string(b) != "data of settings.json" {
		t.Errorf("settings.json = %q after a second launch", b)
	}
}

func TestMigrateDataDirKeepsNewDir(t *testing.T) {
	base := t.TempDir()
	old := legacyDir(t, base)
	before := tree(t, old)
	dst := filepath.Join(base, identifier)
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "settings.json"), []byte(`{"new":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	newBefore := tree(t, dst)

	if copied, err := migrateDataDir(base, legacyIdentifier, identifier); err != nil || copied {
		t.Fatalf("migrateDataDir = %v, %v; want false, nil", copied, err)
	}
	if got := tree(t, dst); !maps.Equal(got, newBefore) {
		t.Errorf("the new folder changed:\n got %v\nwant %v", got, newBefore)
	}
	if got := tree(t, old); !maps.Equal(got, before) {
		t.Errorf("the old folder changed")
	}
}

func TestMigrateDataDirNothingToCopy(t *testing.T) {
	base := t.TempDir()
	if copied, err := migrateDataDir(base, legacyIdentifier, identifier); err != nil || copied {
		t.Fatalf("migrateDataDir = %v, %v; want false, nil", copied, err)
	}
	if names, _ := os.ReadDir(base); len(names) != 0 {
		t.Errorf("base holds %v, want nothing (Gocast makes its folder)", names)
	}
}

// A copy cut short is removed: the next launch copies again, from the
// start.
func TestMigrateDataDirRemovesStaleCopy(t *testing.T) {
	base := t.TempDir()
	old := legacyDir(t, base)
	before := tree(t, old)
	tmp := filepath.Join(base, "."+identifier+".migrating")
	if err := os.MkdirAll(filepath.Join(tmp, "clipboard", "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "clipboard", "data", "half.png"), []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(tmp, "clipboard"), 0o500); err != nil { // as a read-only folder copied
		t.Fatal(err)
	}

	if copied, err := migrateDataDir(base, legacyIdentifier, identifier); err != nil || !copied {
		t.Fatalf("migrateDataDir = %v, %v; want true, nil", copied, err)
	}
	if _, err := os.Lstat(tmp); !os.IsNotExist(err) {
		t.Errorf("the copy cut short is still there: %v", err)
	}
	got := tree(t, filepath.Join(base, identifier))
	if _, ok := got["clipboard/data/half.png"]; ok || len(got) != len(before) {
		t.Errorf("the copy holds %v, want the old folder's files only", got)
	}

	// Removed as well when the new folder is there.
	if err := os.Mkdir(tmp, 0o700); err != nil {
		t.Fatal(err)
	}
	if copied, err := migrateDataDir(base, legacyIdentifier, identifier); err != nil || copied {
		t.Fatalf("migrateDataDir = %v, %v; want false, nil", copied, err)
	}
	if _, err := os.Lstat(tmp); !os.IsNotExist(err) {
		t.Errorf("the copy cut short is still there: %v", err)
	}
}

// A copy that fails leaves no folder behind, nor touches the old one.
func TestMigrateDataDirFails(t *testing.T) {
	base := t.TempDir()
	old := legacyDir(t, base)
	before := tree(t, old)
	locked := filepath.Join(old, "clipboard", "data", "a1.png")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o600) })
	if f, err := os.Open(locked); err == nil {
		f.Close()
		t.Skip("unreadable files can be read (root)")
	}

	copied, err := migrateDataDir(base, legacyIdentifier, identifier)
	if err == nil || copied {
		t.Fatalf("migrateDataDir = %v, %v; want false and an error", copied, err)
	}
	names, _ := os.ReadDir(base)
	if len(names) != 1 || names[0].Name() != legacyIdentifier {
		t.Errorf("base holds %v, want the old folder only", names)
	}
	if err := os.Chmod(locked, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := tree(t, old); !maps.Equal(got, before) {
		t.Errorf("the old folder changed")
	}
}
