package clipboard

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// KeyFile keeps the history's key in a file of its own, outside the
// history's directory (the app's is keys/clipboard.key in its folder):
// its directory private to the user (0700), the file too (0600), written
// atomically, both left out of Time Machine backups, so that a backup of
// the history never holds its key. Unlike a Keychain item, an update of
// the app reads it without asking; the key moves back into the Keychain
// once the app is signed with a Developer ID, a KeyStore of its own.
type KeyFile struct {
	Path string
}

// NewKeyFile is the key file at path.
func NewKeyFile(path string) *KeyFile { return &KeyFile{Path: path} }

// Load reads the key: ErrNoKey when there is no file, ErrBadKey when it
// holds no key.
func (k *KeyFile) Load() ([]byte, error) {
	b, err := os.ReadFile(k.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, err
	}
	if len(b) != KeySize {
		return nil, fmt.Errorf("%w: %d bytes", ErrBadKey, len(b))
	}
	return b, nil
}

// Save writes key, replacing an earlier one, and leaves the file and its
// directory out of backups (as best it can: a failure there is not one of
// Save's).
func (k *KeyFile) Save(key []byte) error {
	dir := filepath.Dir(k.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	// Temporary files a crash left behind.
	if names, err := os.ReadDir(dir); err == nil {
		for _, n := range names {
			if strings.HasPrefix(n.Name(), tmpPrefix) {
				_ = os.Remove(filepath.Join(dir, n.Name()))
			}
		}
	}
	if err := writeAtomic(k.Path, key); err != nil {
		return err
	}
	_ = excludeFromBackup(dir)
	_ = excludeFromBackup(k.Path)
	return nil
}
