package clipboard

// MIGRATION ONLY: format 2 (Gocast builds of November 2026) kept the
// history encrypted, AES-256-GCM, with a key in keys/clipboard.key. The
// history is now kept in the clear (format 3); this file reads a format 2
// history once, writes it out as format 3 and deletes it, with its key.
// Nothing else decrypts. Remove it once no format 2 history is left.

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	format2Index = "history2.gcb"
	format2Data  = "data2"
	format2Magic = "GCB2"
	format2Key   = 32
)

// format2Record is an entry of a format 2 index: a format 3 record, its
// formatted text and image in one sealed file, data2/<id>.gcb, when Blob.
type format2Record struct {
	record
	Blob bool `json:"blob,omitempty"`
}

// migrateFormat2 turns a format 2 history in d into format 3, sealed with
// the key in keyPath, then deletes the format 2 files, the key file and
// its folder (keys/, when nothing else is in it). Without a readable key
// or index, or when a format 3 index is already there (a migration
// interrupted after writing it), the format 2 files are deleted unread:
// the history starts afresh, never unreadable. keyPath "" is no key.
func migrateFormat2(d *disk, keyPath string) {
	index2 := filepath.Join(d.dir, format2Index)
	data2 := filepath.Join(d.dir, format2Data)
	defer func() {
		_ = os.Remove(index2)
		_ = os.RemoveAll(data2)
		if keyPath != "" {
			_ = os.Remove(keyPath)
			removeTemp(filepath.Dir(keyPath))
			_ = os.Remove(filepath.Dir(keyPath)) // only when empty
		}
	}()
	sealed, err := os.ReadFile(index2)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if _, e := os.Stat(d.indexPath()); e == nil {
		return
	}
	if err != nil || keyPath == "" {
		log.Printf("clipboard: a format 2 history without its key: deleted")
		return
	}
	key, err := os.ReadFile(keyPath)
	if err == nil && len(key) != format2Key {
		err = fmt.Errorf("a key of %d bytes", len(key))
	}
	var aead cipher.AEAD
	if err == nil {
		var block cipher.Block
		if block, err = aes.NewCipher(key); err == nil {
			aead, err = cipher.NewGCM(block)
		}
	}
	if aead == nil {
		log.Printf("clipboard: a format 2 history without a readable key (%v): deleted", err)
		return
	}
	plain, err := open2(aead, format2Index, sealed)
	var idx struct {
		Entries []format2Record `json:"entries"`
	}
	if err == nil {
		err = json.Unmarshal(plain, &idx)
	}
	if err != nil {
		log.Printf("clipboard: a format 2 history that cannot be read: deleted")
		return
	}
	out := index{Version: indexVersion, Entries: make([]record, 0, len(idx.Entries))}
	for _, r := range idx.Entries {
		if !validID(r.ID) || r.Kind == "" {
			continue
		}
		rec := r.record
		rec.Data = nil
		if r.Blob {
			name := format2Data + "/" + r.ID + ".gcb"
			var c Content
			b, err := os.ReadFile(filepath.Join(d.dir, filepath.FromSlash(name)))
			if err == nil {
				b, err = open2(aead, name, b)
			}
			if err == nil {
				c.RTF, c.HTML, c.PNG, err = decodeBlob2(b)
			}
			if err == nil {
				rec.Data, err = d.writeParts(r.ID, c)
			}
			if err != nil && r.Kind == KindImage {
				continue // an image is nothing without it
			}
		}
		out.Entries = append(out.Entries, rec)
	}
	data, err := json.Marshal(out)
	if err == nil {
		err = writeAtomic(d.indexPath(), data)
	}
	if err != nil {
		log.Printf("clipboard: migrating the format 2 history: %v", err)
		return
	}
	log.Printf("clipboard: migrated %d entries of the format 2 history", len(out.Entries))
}

// open2 opens a format 2 file: the magic, a nonce, then the ciphertext,
// authenticated with the file's name.
func open2(aead cipher.AEAD, name string, sealed []byte) ([]byte, error) {
	n := aead.NonceSize()
	if len(sealed) < len(format2Magic)+n || !strings.HasPrefix(string(sealed[:len(format2Magic)]), format2Magic) {
		return nil, ErrUnreadable
	}
	nonce := sealed[len(format2Magic) : len(format2Magic)+n]
	return aead.Open(nil, nonce, sealed[len(format2Magic)+n:], []byte(name))
}

// decodeBlob2 reads the formatted text and image of a format 2 data
// file: RTF, HTML and PNG, each a uvarint length then the bytes.
func decodeBlob2(b []byte) (rtf, htm, img []byte, err error) {
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
