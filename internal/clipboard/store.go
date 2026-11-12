package clipboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrNotFound means there is no entry with that ID.
	ErrNotFound = errors.New("clipboard: no such entry")
	// ErrEmpty means the content has nothing to keep.
	ErrEmpty = errors.New("clipboard: empty content")
	// ErrTooLarge means the content is over MaxSize.
	ErrTooLarge = errors.New("clipboard: content over the size limit")
	// ErrUnreadable means the saved history cannot be read: its index
	// cannot be read or parsed. It is left as it is, and the store stays
	// empty and records nothing, until Clear deletes it and starts a fresh
	// history.
	ErrUnreadable = errors.New("clipboard: the saved history cannot be read")
	// ErrNoPasteboard means the store has no pasteboard to write to.
	ErrNoPasteboard = errors.New("clipboard: no pasteboard")
	// ErrNoText means the entry (an image) has no text to paste as plain
	// text.
	ErrNoText = errors.New("clipboard: the entry has no text")
	// ErrNoAccessibility means Gocast may not press ⌘V for the user: the
	// entry is on the pasteboard, and the user has to paste it.
	ErrNoAccessibility = errors.New("clipboard: no Accessibility permission to paste")
)

// Retention is how long unpinned entries are kept, as settings store it.
type Retention string

const (
	RetainDay     Retention = "1d"
	RetainWeek    Retention = "1w"
	RetainMonth   Retention = "1m"
	Retain3Months Retention = "3m"
	RetainForever Retention = "forever"
	// DefaultRetention is a month.
	DefaultRetention = RetainMonth
)

// Retentions are the choices, shortest first.
var Retentions = []Retention{RetainDay, RetainWeek, RetainMonth, Retain3Months, RetainForever}

// Duration is how long entries are kept, 0 for ever. An unknown value is
// the default, a month; a month is 30 days.
func (r Retention) Duration() time.Duration {
	const day = 24 * time.Hour
	switch r {
	case RetainDay:
		return day
	case RetainWeek:
		return 7 * day
	case Retain3Months:
		return 90 * day
	case RetainForever:
		return 0
	}
	return 30 * day
}

// DefaultMaxEntries is how many entries the history keeps at most, by
// default; past it the oldest unpinned ones go.
const DefaultMaxEntries = 2000

// saveDelay gathers the changes of a moment into one write of the index.
const saveDelay = 400 * time.Millisecond

// Options configure a Store.
type Options struct {
	// Retention is how long unpinned entries are kept; "" is the default.
	Retention Retention
	// MaxEntries caps the number of entries; 0 is DefaultMaxEntries.
	// Pinned entries are never dropped for it.
	MaxEntries int
	// Pasteboard is where CopyToPasteboard and Paste write, and what a
	// Watcher reads; nil is the system pasteboard (macOS only).
	Pasteboard Pasteboard
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Format2Key is the key file of a format 2 history, which Open
	// migrates (migrateFormat2), then deletes; "" is none.
	Format2Key string
}

type item struct {
	Entry
	hash   string
	folded string   // lower-cased text searched
	parts  []string // the parts saved under data/ (partRTF, …)
	data   *Content // the formatted text or image, when not saved
}

// Store is the clipboard history: entries in memory, newest first, saved
// to its directory when it has one. Its methods are safe for concurrent
// use.
type Store struct {
	opts Options

	mu     sync.Mutex
	items  []*item // newest first
	byID   map[string]*item
	byHash map[string]*item
	disk   *disk // nil keeps the history in memory only
	dir    string
	// unreadable: the saved history cannot be read (Unreadable).
	unreadable bool
	err        error
	onChange   func()
	timer      *time.Timer

	addMu  sync.Mutex
	saveMu sync.Mutex

	// Writes of the store's own to the pasteboard, which the watcher
	// does not record.
	selfWriting atomic.Int32
	selfChange  atomic.Int64
}

func newStore(opts Options) *Store {
	if opts.Retention == "" {
		opts.Retention = DefaultRetention
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = DefaultMaxEntries
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Pasteboard == nil {
		opts.Pasteboard = SystemPasteboard()
	}
	s := &Store{opts: opts, byID: map[string]*item{}, byHash: map[string]*item{}}
	s.selfChange.Store(-1)
	return s
}

// NewMemoryStore returns a store that keeps the history in memory only.
func NewMemoryStore(opts Options) *Store { return newStore(opts) }

// Open loads the history saved in dir, in the clear. First it deletes a
// history of format 1 unread, and migrates one of format 2 (or deletes
// it, without its key). It always returns a usable store; the error says
// what went wrong, and Err keeps reporting it:
//
//   - ErrUnreadable (wrapped): the index cannot be read or parsed. The
//     files are left alone, the store is empty, records nothing and
//     reports Unreadable until Clear deletes the files and starts a fresh
//     history.
//   - another error: the directory cannot be prepared. The history is
//     kept in memory only (Persistent is false).
func Open(dir string, opts Options) (*Store, error) {
	s := newStore(opts)
	s.dir = dir
	s.err = s.open()
	return s, s.err
}

func (s *Store) open() error {
	if err := removeFormat1(s.dir); err != nil {
		return err
	}
	d := &disk{dir: s.dir}
	if err := d.prepare(); err != nil {
		return err
	}
	migrateFormat2(d, s.opts.Format2Key)
	data, err := os.ReadFile(d.indexPath())
	var idx index
	if err == nil {
		err = json.Unmarshal(data, &idx)
	} else if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	if err != nil {
		s.setUnreadable()
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	s.mu.Lock()
	now := s.opts.Now()
	for _, r := range idx.Entries {
		if !validID(r.ID) || s.byID[r.ID] != nil || r.Kind == "" {
			continue
		}
		it := &item{Entry: Entry{
			ID: r.ID, Kind: r.Kind, Text: r.Text, Files: r.Files, App: r.App,
			Time: r.Time, Size: r.Size, Width: r.Width, Height: r.Height, Pinned: r.Pinned,
			Thumbnail: r.Thumbnail,
		}, hash: r.Hash, parts: r.Data}
		it.fold()
		s.items = append(s.items, it)
		s.byID[it.ID] = it
		if it.hash != "" && s.byHash[it.hash] == nil {
			s.byHash[it.hash] = it
		}
	}
	slices.SortStableFunc(s.items, func(a, b *item) int { return b.Time.Compare(a.Time) })
	// Data files of no entry: a crash between writing one and saving the
	// index.
	d.removeOrphans(func(id string) []string {
		if it := s.byID[id]; it != nil {
			return it.parts
		}
		return nil
	})
	s.disk = d
	removed := s.pruneLocked(now)
	s.mu.Unlock()
	if len(removed) > 0 {
		s.removeData(d, removed)
		return s.save()
	}
	return nil
}

func (s *Store) setUnreadable() {
	s.mu.Lock()
	s.unreadable = true
	s.mu.Unlock()
}

// Unreadable reports whether the saved history cannot be read: its index
// cannot be read or parsed. The
// store is then empty and records nothing; Clear deletes the saved
// history, pinned entries included, and starts a fresh one.
func (s *Store) Unreadable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unreadable
}

// CanClear reports whether Clear has anything to do: the saved history
// is unreadable, or there is an unpinned entry.
func (s *Store) CanClear() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unreadable {
		return true
	}
	for _, it := range s.items {
		if !it.Pinned {
			return true
		}
	}
	return false
}

// Err is the latest problem with the saved history, or nil, for the UI
// to show. ErrUnreadable (wrapped) goes with Unreadable.
func (s *Store) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Persistent reports whether the history is saved to disk.
func (s *Store) Persistent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disk != nil
}

// SetOnChange sets fn to be called after the history changes: an entry
// added, moved to the top, pinned or deleted. It may be called on any
// goroutine, without the store's lock held.
func (s *Store) SetOnChange(fn func()) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

func (s *Store) changed() {
	s.mu.Lock()
	fn := s.onChange
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// SetRetention changes how long unpinned entries are kept, and drops the
// ones now too old.
func (s *Store) SetRetention(r Retention) {
	s.mu.Lock()
	s.opts.Retention = r
	s.mu.Unlock()
	s.Prune()
}

// Retention is how long unpinned entries are kept.
func (s *Store) Retention() Retention {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opts.Retention
}

// Prune drops the unpinned entries older than the retention, and the
// oldest unpinned ones past MaxEntries. It returns how many it dropped.
func (s *Store) Prune() int {
	s.mu.Lock()
	removed := s.pruneLocked(s.opts.Now())
	d := s.disk
	s.mu.Unlock()
	if len(removed) == 0 {
		return 0
	}
	s.removeData(d, removed)
	s.saveNow()
	s.changed()
	return len(removed)
}

func (s *Store) pruneLocked(now time.Time) []*item {
	keep := s.opts.Retention.Duration()
	var removed []*item
	unpinned := 0
	s.items = slices.DeleteFunc(s.items, func(it *item) bool {
		if it.Pinned {
			return false
		}
		if keep > 0 && now.Sub(it.Time) > keep || unpinned >= s.opts.MaxEntries {
			removed = append(removed, it)
			return true
		}
		unpinned++
		return false
	})
	for _, it := range removed {
		s.forget(it)
	}
	return removed
}

func (s *Store) forget(it *item) {
	delete(s.byID, it.ID)
	if s.byHash[it.hash] == it {
		delete(s.byHash, it.hash)
	}
}

// removeData removes the data files of items.
func (s *Store) removeData(d *disk, items []*item) {
	if d == nil {
		return
	}
	for _, it := range items {
		d.removeParts(it.ID, it.parts)
	}
}

func (it *item) fold() {
	it.folded = strings.ToLower(it.Text + "\n" + it.App.Name)
}

func (it *item) entry() Entry {
	e := it.Entry
	e.Files = slices.Clone(e.Files)
	return e
}

// Add records content copied from app: a new entry at the top, or, when
// the same content is already kept (the same text, image or files), that
// entry moved to the top with the new time, source and formatting. It
// returns ErrEmpty or ErrTooLarge for content it does not keep, and
// ErrUnreadable while the saved history is unreadable.
func (s *Store) Add(c Content, app App) (Entry, error) {
	c, kind := normalize(c)
	if kind == "" {
		return Entry{}, ErrEmpty
	}
	size := c.Size()
	if size > MaxSize {
		return Entry{}, ErrTooLarge
	}
	h := hash(c, kind)
	rich := len(c.RTF) > 0 || len(c.HTML) > 0 || len(c.PNG) > 0

	var thumb []byte
	if kind == KindImage {
		thumb = thumbnail(c.PNG)
	}

	s.addMu.Lock()
	defer s.addMu.Unlock()

	s.mu.Lock()
	if s.unreadable {
		s.mu.Unlock()
		return Entry{}, ErrUnreadable
	}
	it := s.byHash[h]
	id := newID()
	if it != nil {
		id = it.ID
	}
	d := s.disk
	var old []string
	if it != nil {
		old = slices.Clone(it.parts)
	}
	s.mu.Unlock()

	// The data files go first, so that a saved index never points at a
	// missing one. An identical image has its file already, which keeps
	// its name (the path pasted into terminals).
	var saved []string
	var werr error
	if d != nil && rich {
		if it != nil && kind == KindImage && slices.Contains(old, partPNG) {
			saved = old
		} else {
			saved, werr = d.writeParts(id, c)
		}
	}

	now := s.opts.Now()
	s.mu.Lock()
	if it != nil && s.byID[it.ID] != it {
		it = nil // deleted meanwhile
	}
	var stale []string
	if it == nil {
		it = &item{Entry: Entry{ID: id}, hash: h}
		s.byID[id] = it
		s.byHash[h] = it
		s.items = append([]*item{it}, s.items...)
	} else {
		for _, p := range old {
			if !slices.Contains(saved, p) {
				stale = append(stale, p)
			}
		}
		s.toFront(it)
	}
	it.Kind, it.App, it.Time, it.Size = kind, app, now, size
	it.Text, it.Files, it.Width, it.Height, it.Thumbnail = "", nil, 0, 0, thumb
	switch kind {
	case KindFiles:
		it.Files = c.Files
		it.Text = strings.Join(c.Files, "\n")
	case KindImage:
		it.Width, it.Height = imageSize(c.PNG)
	default:
		it.Text = c.Text
	}
	it.fold()
	it.parts = saved
	it.data = nil
	if rich && saved == nil {
		cc := c
		it.data = &cc
	}
	if werr != nil {
		s.err = werr
	}
	removed := s.pruneLocked(now)
	e := it.entry()
	s.mu.Unlock()

	if d != nil {
		d.removeParts(id, stale)
	}
	s.removeData(d, removed)
	s.scheduleSave()
	s.changed()
	return e, nil
}

func (s *Store) toFront(it *item) {
	i := slices.Index(s.items, it)
	if i > 0 {
		copy(s.items[1:i+1], s.items[:i])
		s.items[0] = it
	}
}

// Touch moves the entry to the top, as copied now.
func (s *Store) Touch(id string) error {
	s.mu.Lock()
	it := s.byID[id]
	if it == nil {
		s.mu.Unlock()
		return ErrNotFound
	}
	it.Time = s.opts.Now()
	s.toFront(it)
	s.mu.Unlock()
	s.scheduleSave()
	s.changed()
	return nil
}

// SetPinned pins or unpins the entry. Pinned entries list first and never
// expire. Unpinned, the entry becomes the newest, as copied now: at the
// top of today's, not back among those of the day it was copied.
func (s *Store) SetPinned(id string, pinned bool) error {
	s.mu.Lock()
	it := s.byID[id]
	if it == nil {
		s.mu.Unlock()
		return ErrNotFound
	}
	if it.Pinned && !pinned {
		it.Time = s.opts.Now()
		s.toFront(it)
	}
	it.Pinned = pinned
	s.mu.Unlock()
	s.scheduleSave()
	s.changed()
	return nil
}

// Delete removes the entry, pinned or not, from memory and disk.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	it := s.byID[id]
	if it == nil {
		s.mu.Unlock()
		return ErrNotFound
	}
	s.items = slices.DeleteFunc(s.items, func(x *item) bool { return x == it })
	s.forget(it)
	d := s.disk
	s.mu.Unlock()
	s.removeData(d, []*item{it})
	s.saveNow()
	s.changed()
	return nil
}

// Clear clears the history: every entry but the pinned ones goes, from
// memory and disk (pinned entries go only one by one, with Delete).
// When the saved history is unreadable, Clear instead deletes all of it,
// pinned entries included, and starts a fresh, empty history.
func (s *Store) Clear() error {
	s.mu.Lock()
	if s.unreadable {
		s.mu.Unlock()
		err := s.reset()
		s.changed()
		return err
	}
	var removed []*item
	s.items = slices.DeleteFunc(s.items, func(it *item) bool {
		if it.Pinned {
			return false
		}
		removed = append(removed, it)
		return true
	})
	for _, it := range removed {
		s.forget(it)
	}
	d := s.disk
	s.mu.Unlock()
	s.removeData(d, removed)
	err := s.saveNow()
	s.changed()
	return err
}

// reset replaces an unreadable saved history with a fresh, empty one.
func (s *Store) reset() error {
	s.addMu.Lock()
	defer s.addMu.Unlock()
	if err := wipe(s.dir); err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.unreadable, s.disk = false, nil
	s.items, s.byID, s.byHash = nil, map[string]*item{}, map[string]*item{}
	s.mu.Unlock()
	err := s.open()
	if err == nil {
		err = s.save()
	}
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
	return err
}

// Len is the number of entries.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// Get returns the entry with id.
func (s *Store) Get(id string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.byID[id]
	if it == nil {
		return Entry{}, false
	}
	return it.entry(), true
}

// Entries lists the entries that pass filter: pinned ones first, then
// the rest, each newest first. Expired entries are left out even before
// they are pruned.
func (s *Store) Entries(filter Filter) []Entry { return s.Search("", filter) }

// Search lists the entries that pass filter and contain every
// space-separated word of query, ignoring case, in their text, file
// paths, URL or source app name; in the order of Entries. Words match as
// substrings, so Chinese matches without spaces; pinyin does not match.
func (s *Store) Search(query string, filter Filter) []Entry {
	terms := strings.Fields(strings.ToLower(query))
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.opts.Now()
	keep := s.opts.Retention.Duration()
	var pinned, rest []Entry
	for _, it := range s.items {
		if !filter.Matches(it.Kind) {
			continue
		}
		if !it.Pinned && keep > 0 && now.Sub(it.Time) > keep {
			continue
		}
		ok := true
		for _, t := range terms {
			if !strings.Contains(it.folded, t) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if it.Pinned {
			pinned = append(pinned, it.entry())
		} else {
			rest = append(rest, it.entry())
		}
	}
	return append(pinned, rest...)
}

// Content returns everything the entry holds, its formatted text or image
// included, read from disk.
func (s *Store) Content(id string) (Content, error) {
	s.mu.Lock()
	it := s.byID[id]
	if it == nil {
		s.mu.Unlock()
		return Content{}, ErrNotFound
	}
	e, parts, data, d := it.entry(), it.parts, it.data, s.disk
	s.mu.Unlock()
	if data != nil {
		c := *data
		return c, nil
	}
	c := Content{Files: e.Files}
	if e.Kind != KindFiles {
		c.Text = e.Text
	}
	if d != nil {
		if err := d.readParts(id, parts, &c); err != nil {
			return c, err
		}
	}
	return c, nil
}

// The index, history.json: every entry's metadata and text.
type index struct {
	Version int      `json:"version"`
	Entries []record `json:"entries"`
}

type record struct {
	ID        string    `json:"id"`
	Kind      Kind      `json:"kind"`
	Text      string    `json:"text,omitempty"`
	Files     []string  `json:"files,omitempty"`
	App       App       `json:"app"`
	Time      time.Time `json:"time"`
	Size      int64     `json:"size"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	Pinned    bool      `json:"pinned,omitempty"`
	Thumbnail []byte    `json:"thumb,omitempty"`
	Hash      string    `json:"hash"`
	Data      []string  `json:"data,omitempty"` // the parts under data/
}

func (s *Store) scheduleSave() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disk == nil || s.timer != nil {
		return
	}
	s.timer = time.AfterFunc(saveDelay, func() { s.saveNow() })
}

// Flush writes pending changes to disk now.
func (s *Store) Flush() error { return s.saveNow() }

// Close writes pending changes; the store stays usable in memory.
func (s *Store) Close() error { return s.saveNow() }

func (s *Store) saveNow() error {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	return s.save()
}

func (s *Store) save() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	d := s.disk
	if d == nil {
		s.mu.Unlock()
		return nil
	}
	idx := index{Version: indexVersion, Entries: make([]record, 0, len(s.items))}
	for _, it := range s.items {
		idx.Entries = append(idx.Entries, record{
			ID: it.ID, Kind: it.Kind, Text: it.Text, Files: it.Files, App: it.App,
			Time: it.Time, Size: it.Size, Width: it.Width, Height: it.Height,
			Pinned: it.Pinned, Thumbnail: it.Thumbnail, Hash: it.hash, Data: it.parts,
		})
	}
	s.mu.Unlock()
	data, err := json.Marshal(idx)
	if err == nil {
		err = writeAtomic(d.indexPath(), data)
	}
	s.mu.Lock()
	if err != nil {
		s.err = err
	} else if s.err != nil && s.disk == d {
		s.err = nil
	}
	s.mu.Unlock()
	return err
}
