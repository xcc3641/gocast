package clipboard

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// KeySize is the length of the history's encryption key: AES-256.
const KeySize = 32

var (
	// ErrNoKey is what KeyStore.Load returns when no key is stored yet.
	ErrNoKey = errors.New("clipboard: no key stored")
	// ErrBadKey is what KeyStore.Load returns for a stored key that is not
	// a key: of the wrong length, say.
	ErrBadKey = errors.New("clipboard: the stored key is malformed")
)

// KeyStore keeps the key that encrypts the history on disk. The app uses
// a key file (KeyFile), which updates of an ad-hoc signed app read without
// asking, unlike a Keychain item; tests use MemoryKeys.
type KeyStore interface {
	// Load returns the stored key, or ErrNoKey when there is none.
	Load() ([]byte, error)
	// Save stores key, replacing any earlier one.
	Save(key []byte) error
}

// MemoryKeys is a KeyStore in memory, for tests.
type MemoryKeys struct {
	Key []byte
	// Err, when set, is what Load and Save return.
	Err error
}

// Load returns the key, or ErrNoKey.
func (m *MemoryKeys) Load() ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Key == nil {
		return nil, ErrNoKey
	}
	return append([]byte(nil), m.Key...), nil
}

// Save keeps key.
func (m *MemoryKeys) Save(key []byte) error {
	if m.Err != nil {
		return m.Err
	}
	m.Key = append([]byte(nil), key...)
	return nil
}

// The files of the history directory: the index (every entry's metadata
// and text) and one file per entry with formatted text or an image, under
// data2/. Each is AES-256-GCM sealed: a magic, a random nonce, then the
// ciphertext, authenticated together with the file's name so that files
// cannot be swapped.
//
// Version 2 is sealed with the key of a key file. Version 1 (history.gcb,
// data/, magic GCB1) was sealed with a key in the login Keychain, which
// the app no longer reads: removeLegacy deletes those files unread.
const (
	indexName = "history2.gcb"
	dataDir   = "data2"
	blobExt   = ".gcb"
	magic     = "GCB2"
	tmpPrefix = ".tmp-"

	legacyIndex = "history.gcb"
	legacyData  = "data"
)

type vault struct {
	dir  string
	aead cipher.AEAD
}

func newVault(dir string, key []byte) (*vault, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("clipboard: key is %d bytes, want %d", len(key), KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &vault{dir: dir, aead: aead}, nil
}

// prepare makes the directories, private to the user, and removes
// temporary files a crash left behind.
func (v *vault) prepare() error {
	for _, d := range []string{v.dir, filepath.Join(v.dir, dataDir)} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(d, 0o700); err != nil {
			return err
		}
		names, _ := os.ReadDir(d)
		for _, n := range names {
			if strings.HasPrefix(n.Name(), tmpPrefix) {
				_ = os.Remove(filepath.Join(d, n.Name()))
			}
		}
	}
	return nil
}

func (v *vault) seal(name string, plain []byte) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(magic)+len(nonce)+len(plain)+v.aead.Overhead())
	out = append(out, magic...)
	out = append(out, nonce...)
	return v.aead.Seal(out, nonce, plain, []byte(name)), nil
}

func (v *vault) open(name string, sealed []byte) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(sealed) < len(magic)+n || string(sealed[:len(magic)]) != magic {
		return nil, ErrUnreadable
	}
	nonce := sealed[len(magic) : len(magic)+n]
	plain, err := v.aead.Open(nil, nonce, sealed[len(magic)+n:], []byte(name))
	if err != nil {
		return nil, ErrUnreadable
	}
	return plain, nil
}

// write seals plain and writes it to name (relative to the directory)
// atomically: a temporary file, then a rename.
func (v *vault) write(name string, plain []byte) error {
	sealed, err := v.seal(name, plain)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(v.dir, filepath.FromSlash(name)), sealed)
}

// writeAtomic writes data to path, private to the user (0600): to a
// temporary file in the same directory, synced, then renamed over path.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), tmpPrefix+"*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// read reads and opens name; a missing file is fs.ErrNotExist.
func (v *vault) read(name string) ([]byte, error) {
	sealed, err := os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	return v.open(name, sealed)
}

func (v *vault) remove(name string) error {
	err := os.Remove(filepath.Join(v.dir, filepath.FromSlash(name)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func blobName(id string) string { return dataDir + "/" + id + blobExt }

// blobIDs lists the entries that have a file under data/.
func (v *vault) blobIDs() []string {
	names, _ := os.ReadDir(filepath.Join(v.dir, dataDir))
	var ids []string
	for _, n := range names {
		if id, ok := strings.CutSuffix(n.Name(), blobExt); ok && validID(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// removeLegacy deletes the history of version 1, sealed with a Keychain
// key, without reading it (nor the key): the history starts afresh.
func removeLegacy(dir string) error {
	err := os.Remove(filepath.Join(dir, legacyIndex))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, legacyData))
}

// hasFiles reports whether the directory holds any saved history.
func hasFiles(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, indexName)); err == nil {
		return true
	}
	names, _ := os.ReadDir(filepath.Join(dir, dataDir))
	return len(names) > 0
}

// wipe removes the saved history: the index and every data file.
func wipe(dir string) error {
	err := os.Remove(filepath.Join(dir, indexName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, dataDir))
}

// encodeBlob frames the formatted and image data of an entry: three
// fields, RTF, HTML and PNG, each a uvarint length then the bytes.
func encodeBlob(c Content) []byte {
	var out []byte
	for _, f := range [][]byte{c.RTF, c.HTML, c.PNG} {
		out = binary.AppendUvarint(out, uint64(len(f)))
		out = append(out, f...)
	}
	return out
}

func decodeBlob(b []byte) (rtf, htm, img []byte, err error) {
	var fields [3][]byte
	for i := range fields {
		n, k := binary.Uvarint(b)
		if k <= 0 || n > uint64(len(b)-k) {
			return nil, nil, nil, ErrUnreadable
		}
		b = b[k:]
		if n > 0 {
			fields[i] = append([]byte(nil), b[:n]...)
		}
		b = b[n:]
	}
	return fields[0], fields[1], fields[2], nil
}

func newKey() ([]byte, error) {
	key := make([]byte, KeySize)
	_, err := rand.Read(key)
	return key, err
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
