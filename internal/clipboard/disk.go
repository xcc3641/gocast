package clipboard

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The files of the history directory, format 3, in the clear: the index,
// history.json (every entry's metadata and text), and under data/ the
// formatted text and images of the entries that have them, one file per
// part: <id>.rtf, <id>.html, <id>.png. The directory and data/ are
// private to the user (0700), the files too (0600); each is written
// atomically.
//
// Earlier formats, which Open migrates or removes: format 1
// (history.gcb, data/*.gcb), sealed with a Keychain key, deleted unread;
// format 2 (history2.gcb, data2/), sealed with a key file, migrated by
// migrateFormat2.
const (
	indexName = "history.json"
	dataDir   = "data"
	tmpPrefix = ".tmp-"

	indexVersion = 3

	format1Index = "history.gcb"
	format1Ext   = ".gcb"
)

// The parts of an entry saved under data/, by file extension.
const (
	partRTF  = "rtf"
	partHTML = "html"
	partPNG  = "png"
)

// disk is the history directory of a store that saves its history.
type disk struct {
	dir string
}

// prepare makes the directories, private to the user, and removes
// temporary files a crash left behind.
func (d *disk) prepare() error {
	for _, p := range []string{d.dir, filepath.Join(d.dir, dataDir)} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(p, 0o700); err != nil {
			return err
		}
		removeTemp(p)
	}
	return nil
}

// removeTemp removes the temporary files a crash left in dir.
func removeTemp(dir string) {
	names, _ := os.ReadDir(dir)
	for _, n := range names {
		if strings.HasPrefix(n.Name(), tmpPrefix) {
			_ = os.Remove(filepath.Join(dir, n.Name()))
		}
	}
}

func (d *disk) indexPath() string { return filepath.Join(d.dir, indexName) }

// partPath is the file of an entry's part.
func (d *disk) partPath(id, part string) string {
	return filepath.Join(d.dir, dataDir, id+"."+part)
}

// partsOf are the parts content has to save, with their bytes.
func partsOf(c Content) (parts []string, data [][]byte) {
	for _, p := range []struct {
		name string
		b    []byte
	}{{partRTF, c.RTF}, {partHTML, c.HTML}, {partPNG, c.PNG}} {
		if len(p.b) > 0 {
			parts, data = append(parts, p.name), append(data, p.b)
		}
	}
	return parts, data
}

// writeParts saves the formatted text and image of c as entry id's, and
// returns the parts written.
func (d *disk) writeParts(id string, c Content) ([]string, error) {
	parts, data := partsOf(c)
	for i, p := range parts {
		if err := writeAtomic(d.partPath(id, p), data[i]); err != nil {
			return nil, err
		}
	}
	return parts, nil
}

// readParts reads entry id's parts into c.
func (d *disk) readParts(id string, parts []string, c *Content) error {
	for _, p := range parts {
		b, err := os.ReadFile(d.partPath(id, p))
		if err != nil {
			return err
		}
		switch p {
		case partRTF:
			c.RTF = b
		case partHTML:
			c.HTML = b
		case partPNG:
			c.PNG = b
		}
	}
	return nil
}

// removeParts removes entry id's files of parts.
func (d *disk) removeParts(id string, parts []string) {
	for _, p := range parts {
		_ = os.Remove(d.partPath(id, p))
	}
}

// removeOrphans removes the files under data/ that no entry has: a crash
// between writing one and saving the index. keep reports an entry's
// parts.
func (d *disk) removeOrphans(keep func(id string) []string) {
	names, _ := os.ReadDir(filepath.Join(d.dir, dataDir))
	for _, n := range names {
		if n.IsDir() {
			continue
		}
		id, part, ok := strings.Cut(n.Name(), ".")
		if ok && validID(id) && slices.Contains(keep(id), part) {
			continue
		}
		_ = os.Remove(filepath.Join(d.dir, dataDir, n.Name()))
	}
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

// removeFormat1 deletes the history of format 1, sealed with a Keychain
// key, without reading it (nor the key): history.gcb and data/*.gcb.
func removeFormat1(dir string) error {
	err := os.Remove(filepath.Join(dir, format1Index))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	names, _ := os.ReadDir(filepath.Join(dir, dataDir))
	for _, n := range names {
		if strings.HasSuffix(n.Name(), format1Ext) {
			if err := os.Remove(filepath.Join(dir, dataDir, n.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// wipe removes the saved history: the index and every data file.
func wipe(dir string) error {
	err := os.Remove(filepath.Join(dir, indexName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, dataDir))
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
