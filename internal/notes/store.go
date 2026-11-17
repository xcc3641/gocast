package notes

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Ext is the extension of a note's file.
const Ext = ".md"

// indexName is the folder's small index: the notes pinned, in the order
// they were pinned, and the note shown last. Hidden, for the folder to
// show the notes alone.
const indexName = ".gocast-notes.json"

// Note is a note as the list of notes knows it: its file's name without
// .md (its ID, from when it was made: the title may change, the name
// does not), when it last changed, and its text as plain lines, for the
// title and the search.
type Note struct {
	ID       string
	Modified time.Time
	// Lines are the note's lines as plain text, without the Markdown
	// around them (Plain); lower the same in lower case, for the search.
	Lines, lower []string
	// size and stamp are what the file was when read: read again only
	// when either changed.
	size  int64
	stamp time.Time
}

// Title is the note's first line, "" for none (Untitled).
func (n *Note) Title() string {
	if len(n.Lines) == 0 {
		return ""
	}
	return n.Lines[0]
}

// Store is the folder of the notes and its index. Its methods are
// called on one goroutine (the main thread); Write, which writes a
// note's file, on any.
type Store struct {
	dir    string
	notes  map[string]*Note
	pinned []string
	last   string
	// Trash moves a file to the Trash (the system's, in Gocast), from
	// where Finder's Put Back brings it back.
	Trash func(path string) error
}

// Open is the store of the folder dir, which it does not create: the
// first note written does (design.md 11.1).
func Open(dir string) *Store {
	return &Store{dir: dir, notes: map[string]*Note{}, Trash: func(string) error { return errors.ErrUnsupported }}
}

// Dir is the folder of the notes.
func (s *Store) Dir() string { return s.dir }

// Path is the file of the note id.
func (s *Store) Path(id string) string { return filepath.Join(s.dir, id+Ext) }

type index struct {
	Pinned []string `json:"pinned,omitempty"`
	Last   string   `json:"last,omitempty"`
}

// Scan reads the folder again: the notes that came (as Put Back brings
// a note back from the Trash) and went, and those that changed, which
// it reads again; the others it keeps as they were. It reports whether
// the notes changed. A folder that is not there yet has no notes.
func (s *Store) Scan() (bool, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		changed := len(s.notes) > 0
		s.notes, s.pinned, s.last = map[string]*Note{}, nil, ""
		return changed, nil
	}
	if err != nil {
		return false, err
	}
	changed := false
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, Ext) || strings.HasPrefix(name, ".") {
			continue
		}
		id := strings.TrimSuffix(name, Ext)
		info, err := e.Info()
		if err != nil {
			continue
		}
		seen[id] = true
		if n := s.notes[id]; n != nil && n.size == info.Size() && n.stamp.Equal(info.ModTime()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		s.notes[id] = noteOf(id, string(data), info.ModTime(), info.Size())
		changed = true
	}
	for id := range s.notes {
		if !seen[id] {
			delete(s.notes, id)
			changed = true
		}
	}
	var ix index
	if data, err := os.ReadFile(filepath.Join(s.dir, indexName)); err == nil {
		_ = json.Unmarshal(data, &ix) // damaged: nothing pinned
	}
	// A note back from the Trash is no longer pinned: deleting it unpinned
	// it (copy.md 16.6).
	pinned := slices.DeleteFunc(ix.Pinned, func(id string) bool { return s.notes[id] == nil })
	pinned = slices.Compact(pinned)
	if !slices.Equal(pinned, s.pinned) {
		s.pinned, changed = pinned, true
	}
	s.last = ix.Last
	return changed, nil
}

// noteOf is the note id of the Markdown md, its file modified at stamp.
func noteOf(id, md string, stamp time.Time, size int64) *Note {
	lines := Plain(md)
	lower := make([]string, len(lines))
	for i, l := range lines {
		lower[i] = strings.ToLower(l)
	}
	return &Note{ID: id, Modified: stamp, Lines: lines, lower: lower, size: size, stamp: stamp}
}

// Note is the note id, nil if there is none.
func (s *Store) Note(id string) *Note { return s.notes[id] }

// Len is how many notes there are.
func (s *Store) Len() int { return len(s.notes) }

// List lists the notes: those pinned, in the order they were pinned,
// then the others, those changed last first (design.md 11.6).
func (s *Store) List() []*Note {
	out := make([]*Note, 0, len(s.notes))
	for _, id := range s.pinned {
		out = append(out, s.notes[id])
	}
	var rest []*Note
	for id, n := range s.notes {
		if !slices.Contains(s.pinned, id) {
			rest = append(rest, n)
		}
	}
	slices.SortFunc(rest, func(a, b *Note) int {
		if c := b.Modified.Compare(a.Modified); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	return append(out, rest...)
}

// Pinned reports whether the note id is pinned.
func (s *Store) Pinned(id string) bool { return slices.Contains(s.pinned, id) }

// PinnedIDs are the notes pinned, in the order they were pinned: ⌘1 opens
// the first.
func (s *Store) PinnedIDs() []string { return slices.Clone(s.pinned) }

// SetPinned pins the note id, last, or unpins it, and saves the index.
func (s *Store) SetPinned(id string, on bool) error {
	if s.notes[id] == nil || s.Pinned(id) == on {
		return nil
	}
	if on {
		s.pinned = append(s.pinned, id)
	} else {
		s.pinned = slices.DeleteFunc(s.pinned, func(p string) bool { return p == id })
	}
	return s.saveIndex()
}

// Last is the note shown last, "" for none or one gone.
func (s *Store) Last() string {
	if s.notes[s.last] == nil {
		return ""
	}
	return s.last
}

// SetLast notes that the note id shows, for the window to show it next
// time, Gocast quit meanwhile (design.md 11.1); kept for a note that is
// not written yet only once it is.
func (s *Store) SetLast(id string) error {
	if s.last == id || s.notes[id] == nil {
		return nil
	}
	s.last = id
	return s.saveIndex()
}

func (s *Store) saveIndex() error {
	data, err := json.MarshalIndent(index{Pinned: s.pinned, Last: s.last}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return writeFile(filepath.Join(s.dir, indexName), append(data, '\n'))
}

// NewID is the ID of a note made at now: its date and time, with a
// number after it should another have them.
func (s *Store) NewID(now time.Time) string {
	base := now.Format("2006-01-02-150405")
	id := base
	for i := 2; s.notes[id] != nil || fileExists(s.Path(id)); i++ {
		id = base + "-" + strconv.Itoa(i)
	}
	return id
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Write writes md as the note id's file, in the folder dir, creating
// the folder (0700) and the file (0600) the first time, whole or not at
// all. It is safe to call on any goroutine, for one note at a time.
func Write(dir, id, md string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, id+Ext), []byte(md))
}

// writeFile replaces path with data through a temporary file renamed
// over it.
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // after the rename, nothing
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Written notes that Write wrote md as the note id: the list knows it
// as it is now, as the next Scan would.
func (s *Store) Written(id, md string) {
	path := s.Path(id)
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	s.notes[id] = noteOf(id, md, info.ModTime(), info.Size())
}

// Remove removes the note id's file and forgets it, unpinned: a note
// emptied, which nothing is lost of.
func (s *Store) Remove(id string) error {
	if s.notes[id] == nil {
		return nil
	}
	if err := os.Remove(s.Path(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return s.forget(id)
}

// Delete moves the note id to the Trash and forgets it, unpinned.
func (s *Store) Delete(id string) error {
	if s.notes[id] == nil {
		return nil
	}
	if err := s.Trash(s.Path(id)); err != nil {
		return err
	}
	return s.forget(id)
}

// forget forgets the note id, unpinned, no longer the one shown last.
func (s *Store) forget(id string) error {
	delete(s.notes, id)
	was := len(s.pinned)
	s.pinned = slices.DeleteFunc(s.pinned, func(p string) bool { return p == id })
	if len(s.pinned) != was || s.last == id {
		if s.last == id {
			s.last = ""
		}
		return s.saveIndex()
	}
	return nil
}
