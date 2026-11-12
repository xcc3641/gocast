package clipboard

import (
	"errors"
	"slices"
)

// ErrNotImage is what ImagePath returns for an entry that is not an
// image saved to disk.
var ErrNotImage = errors.New("clipboard: the entry is not an image saved to disk")

// ImagePath is the file of the image of entry id, data/<id>.png in the
// history's directory: what pasting an image into a terminal pastes. It
// stays the same while the entry is kept, moved to the top or pinned.
func (s *Store) ImagePath(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.byID[id]
	if it == nil {
		return "", ErrNotFound
	}
	if it.Kind != KindImage || s.disk == nil || !slices.Contains(it.parts, partPNG) {
		return "", ErrNotImage
	}
	return s.disk.partPath(id, partPNG), nil
}

// CopyImagePath puts the path of the image of entry id (ImagePath) on the
// pasteboard, as plain text, and moves the entry to the top.
func (s *Store) CopyImagePath(id string) (string, error) {
	path, err := s.ImagePath(id)
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
