package clipboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotImage is what the image file calls return for an entry that is
// not an image.
var ErrNotImage = errors.New("clipboard: the entry is not an image")

// DefaultImageDir is the folder of the image files pasted into terminals,
// os.TempDir()/gocast-clipboard: the app's Options.ImageDir.
func DefaultImageDir() string { return filepath.Join(os.TempDir(), "gocast-clipboard") }

// ImageFile writes the image of entry id, decrypted, to <id>.png in
// Options.ImageDir, a folder private to the user (0700, the file 0600),
// and returns its path: what pasting an image into a terminal pastes.
// The file of an earlier paste is reused. Open empties the folder,
// Delete and Clear remove the files of the entries they remove.
func (s *Store) ImageFile(id string) (string, error) {
	dir := s.opts.ImageDir
	if dir == "" {
		return "", errors.New("clipboard: no folder for image files")
	}
	e, ok := s.Get(id)
	if !ok || !validID(id) {
		return "", ErrNotFound
	}
	if e.Kind != KindImage {
		return "", ErrNotImage
	}
	path := filepath.Join(dir, id+".png")
	if err := privateDir(dir); err != nil {
		return "", err
	}
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() && fi.Size() > 0 {
		return path, nil
	}
	c, err := s.Content(id)
	if err != nil {
		return "", err
	}
	if len(c.PNG) == 0 {
		return "", ErrNotImage
	}
	if err := writeAtomic(path, c.PNG); err != nil {
		return "", err
	}
	return path, nil
}

// privateDir makes dir, a directory of the user's alone (0700), not a
// link to one elsewhere.
func privateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("clipboard: %s is not a folder", dir)
	}
	return os.Chmod(dir, 0o700)
}

// CopyImagePath puts the path of the image file of entry id (ImageFile)
// on the pasteboard, as plain text, and moves the entry to the top.
func (s *Store) CopyImagePath(id string) (string, error) {
	path, err := s.ImageFile(id)
	if err != nil {
		return "", err
	}
	if err := s.write(Content{Text: path}); err != nil {
		return "", err
	}
	return path, s.Touch(id)
}

// PasteImagePath is CopyImagePath, then ⌘V, as Paste: how an image is
// pasted into a terminal.
func (s *Store) PasteImagePath(id string) error {
	if _, err := s.CopyImagePath(id); err != nil {
		return err
	}
	return pasteNow()
}

// removeImageFiles removes the image files of items; all of them for nil.
func (s *Store) removeImageFiles(items []*item) {
	dir := s.opts.ImageDir
	if dir == "" {
		return
	}
	if items == nil {
		_ = os.RemoveAll(dir)
		return
	}
	for _, it := range items {
		if it.Kind == KindImage && validID(it.ID) {
			_ = os.Remove(filepath.Join(dir, it.ID+".png"))
		}
	}
}
