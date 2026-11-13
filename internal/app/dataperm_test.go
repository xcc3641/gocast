package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestrictDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "com.hugo.gocast")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o755)
	for _, n := range []string{"settings.json", "presets.json", "usage.json"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chmod(filepath.Join(dir, n), 0o644)
	}
	if err := os.WriteFile(filepath.Join(dir, "rates.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "clipboard"), 0o700); err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(dir, "other"), 0o755)
	os.Chmod(filepath.Join(dir, "other"), 0o755)
	restrictDataDir(dir)
	want := map[string]os.FileMode{
		".": 0o700, "settings.json": 0o600, "presets.json": 0o600, "usage.json": 0o600,
		"rates.json": 0o600, "clipboard": 0o700, "other": 0o700,
	}
	for n, m := range want {
		fi, err := os.Stat(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != m {
			t.Errorf("%s: mode %o, want %o", n, got, m)
		}
	}
}
