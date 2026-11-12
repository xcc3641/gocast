//go:build darwin

package clipboard

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyFileExcludedFromBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "clipboard.key")
	if err := NewKeyFile(path).Save(bytes.Repeat([]byte{1}, KeySize)); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, filepath.Dir(path)} {
		if ok, err := excludedFromBackup(p); err != nil || !ok {
			t.Errorf("%s excluded from backup: %v, %v", p, ok, err)
		}
	}
	// A file never marked reads as backed up: the check can fail.
	other := filepath.Join(t.TempDir(), "plain")
	os.WriteFile(other, []byte("x"), 0o600)
	if ok, err := excludedFromBackup(other); err != nil || ok {
		t.Errorf("an unmarked file: %v, %v", ok, err)
	}
}
