package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"

	"gocast/internal/presets"
	"gocast/internal/termhost"
	"gocast/locales"
)

// sysTermHost connects to the terminal host of the data folder dir (see
// package termhost), starting it when spawn is set and none runs: the
// floating terminal's programs run there, and outlive Gocast. A fake in
// tests.
var sysTermHost = func(dir string, spawn bool) (*termhost.Client, error) {
	if !spawn {
		return termhost.Dial(dir)
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return termhost.Ensure(dir, exe)
}

// sysConfirmCloseAll asks before Close All Terminals ends every program
// (design.md 5): the system's alert, in the middle of the screen, Gocast
// in front, Cancel the default. A fake in tests.
var sysConfirmCloseAll = func() bool {
	mygo.App.Focus()
	r, err := mygo.Dialog.Message(mygo.MessageOptions{
		Message:            locales.TermCloseAllTitle(),
		Detail:             locales.TermCloseAllDetail(),
		Buttons:            []string{locales.TermCloseAllConfirm(), locales.Cancel()},
		DefaultButton:      1,
		CancelButton:       1,
		DestructiveButtons: []int{0},
	})
	return err == nil && r.Button == 0
}

// sysBundleID is the identifier of the bundle Gocast runs from, "" when
// not packaged.
var sysBundleID = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return termhost.BundleID(exe)
}

// termHostDir is the folder of the terminal host of the data folder dir:
// dir itself for the Gocast installed (identifier com.hugo.gocast, built
// for release, dev false). Any other build shares the data folder with
// it, but never its terminals, which it would show and end: go run and go
// build, mygo dev (com.hugo.gocast.dev), a debug build. Each gets a folder
// of its own, named after its identifier, in dev-terminals.
func termHostDir(dir, bundleID string, dev bool) string {
	if !dev && bundleID == identifier {
		return dir
	}
	name := "unbundled"
	if bundleID != "" {
		name = strings.Map(func(r rune) rune {
			if r == '.' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				return r
			}
			return '_'
		}, bundleID)
		if dev {
			name += "-dev"
		}
	}
	d := filepath.Join(dir, "dev-terminals", name)
	if err := os.MkdirAll(d, 0o700); err != nil {
		log.Printf("terminal host: %v", err)
	}
	return d
}

// termHost is Gocast's side of the terminal host: its connection, made
// when first needed, and the requests to it, which run in order off the
// main thread.
type termHost struct {
	dir string
	// run runs a request: on the host's goroutine, in order, after those
	// before it; one with a key in place of the one with the same key
	// still waiting. At once in tests.
	run func(key string, fn func())

	mu sync.Mutex
	c  *termhost.Client
}

func newTermHost(dir string) *termHost {
	q := newRequestQueue()
	go q.loop()
	return &termHost{dir: dir, run: q.add}
}

// requestQueue holds the requests to the host, in order, as many as come:
// adding one never waits, even while a host that answers no more holds up
// the one running (for as long as its timeout).
type requestQueue struct {
	mu    sync.Mutex
	items []queued
	wake  chan struct{}
}

type queued struct {
	key string
	fn  func()
}

func newRequestQueue() *requestQueue { return &requestQueue{wake: make(chan struct{}, 1)} }

// add queues fn; with a key, in place of the request with that key still
// waiting, which it makes useless (the tab showing, a tab's meta).
func (q *requestQueue) add(key string, fn func()) {
	q.mu.Lock()
	if key != "" {
		q.items = slices.DeleteFunc(q.items, func(it queued) bool { return it.key == key })
	}
	q.items = append(q.items, queued{key, fn})
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// next takes the first request, if any.
func (q *requestQueue) next() (func(), bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil, false
	}
	fn := q.items[0].fn
	q.items[0] = queued{}
	q.items = q.items[1:]
	return fn, true
}

// loop runs the requests, for good.
func (q *requestQueue) loop() {
	for range q.wake {
		for fn, ok := q.next(); ok; fn, ok = q.next() {
			fn()
		}
	}
}

// client is the connection to the host, made again when it broke; with
// spawn, a host starts when none runs.
func (h *termHost) client(spawn bool) (*termhost.Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.c != nil && !h.c.Lost() {
		return h.c, nil
	}
	if h.c != nil {
		h.c.Close()
		h.c = nil
	}
	c, err := sysTermHost(h.dir, spawn)
	if err != nil {
		return nil, err
	}
	h.c = c
	return c, nil
}

// do runs fn with the host, again with a host started anew when the one
// it had went away meanwhile (exited after Close All, or died): fn must
// be a single request, which the host never took when it went away.
func (h *termHost) do(spawn bool, fn func(c *termhost.Client) error) error {
	c, err := h.client(spawn)
	if err != nil {
		return err
	}
	if err = fn(c); errors.Is(err, termhost.ErrLost) {
		if c, err = h.client(spawn); err == nil {
			err = fn(c)
		}
	}
	return err
}

// request runs fn with the running host, if any, in order with the other
// requests, logging its error: requests whose answer nothing waits for.
// One with a key replaces the one with that key still waiting.
func (h *termHost) request(what, key string, fn func(c *termhost.Client) error) {
	h.run(key, func() {
		if err := h.do(false, fn); err != nil {
			log.Printf("terminal host: %s: %v", what, err)
		}
	})
}

// open has the host start the terminal req asks, or, with id, find its
// terminal id, and opens a stream of it; created is the terminal it
// started. Only the start is tried again with a host started anew: a
// stream that can't be opened ends the terminal started for it, which
// would run on unseen.
func (h *termHost) open(req termhost.CreateRequest, id string) (st *termhost.Stream, created string, err error) {
	var c *termhost.Client
	err = h.do(id == "", func(cl *termhost.Client) error {
		c = cl
		if id != "" {
			return nil
		}
		var err error
		created, err = cl.Create(req)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	if id == "" {
		id = created
	}
	if st, err = c.Attach(id); err != nil {
		if created != "" {
			if cerr := c.CloseTerminal(created); cerr != nil {
				log.Printf("terminal host: closing %s: %v", created, cerr)
			}
		}
		return nil, "", err
	}
	return st, created, nil
}

// has reports whether the running host still has the terminal id: one
// whose stream ended without its program, cut off from it, not the host
// gone.
func (h *termHost) has(id string) bool {
	found := false
	err := h.do(false, func(c *termhost.Client) error {
		list, _, err := c.List()
		found = slices.ContainsFunc(list, func(s termhost.Session) bool { return s.ID == id })
		return err
	})
	return err == nil && found
}

// tabMeta is what the host keeps of a tab for Gocast, to show it again
// after Gocast restarted: its preset, as when it started, its font size,
// its folder, and whether it is a plain terminal now (its command ended,
// or it took the place of one cut off).
type tabMeta struct {
	PresetID string          `json:"presetID,omitempty"`
	Preset   *presets.Preset `json:"preset,omitempty"`
	FontSize float32         `json:"fontSize,omitempty"`
	Dir      string          `json:"dir,omitempty"`
	Done     bool            `json:"done,omitempty"`
}

func (s *session) meta() json.RawMessage {
	m := tabMeta{FontSize: s.fontSize, Dir: s.dir, Done: s.done}
	if !s.plain() {
		pr := s.preset
		m.PresetID, m.Preset = s.presetID, &pr
	}
	b, _ := json.Marshal(m)
	return b
}

// hostConn is a tab's terminal in the host as its terminal.Options.Conn:
// made at once, for the view to size and the user to type into, and
// joined to the host's terminal once the host started it (bind). What
// comes before waits: the input, the size, the colors.
type hostConn struct {
	ready chan struct{}
	once  sync.Once

	mu      sync.Mutex
	st      *termhost.Stream
	failed  bool // the host could not start it: Read ends at once
	closed  bool
	input   [][]byte
	size    [4]int // cols, rows, cell width and height; cols 0 unset
	theme   *terminal.Theme
	visible int // 0 unset, 1 hidden, 2 visible
}

func newHostConn() *hostConn { return &hostConn{ready: make(chan struct{})} }

// bind joins the conn to the host's terminal, whose frames resize the
// tab's terminal with resize. It reports false, the stream closed, once
// the conn closed.
func (c *hostConn) bind(st *termhost.Stream, resize func(cols, rows int)) bool {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		st.Close()
		return false
	}
	c.st = st
	st.Bind(resize)
	if c.theme != nil {
		_ = st.UseTheme(c.theme)
	}
	if c.visible != 0 {
		st.SetVisible(c.visible == 2)
	}
	if c.size[0] > 0 {
		_ = st.ResizePixels(c.size[0], c.size[1], c.size[2], c.size[3])
	}
	for _, b := range c.input {
		_, _ = st.Write(b)
	}
	c.input = nil
	c.mu.Unlock()
	c.once.Do(func() { close(c.ready) })
	return true
}

// fail ends the conn that the host could not start.
func (c *hostConn) fail() {
	c.mu.Lock()
	c.failed = true
	c.mu.Unlock()
	c.once.Do(func() { close(c.ready) })
}

// stream is the host's terminal, nil until bind.
func (c *hostConn) stream() *termhost.Stream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.st
}

func (c *hostConn) Read(p []byte) (int, error) {
	<-c.ready
	st := c.stream()
	if st == nil {
		return 0, io.EOF
	}
	return st.Read(p)
}

func (c *hostConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	st := c.st
	if st == nil {
		defer c.mu.Unlock()
		if c.closed || c.failed {
			return 0, io.ErrClosedPipe
		}
		c.input = append(c.input, bytes.Clone(p))
		return len(p), nil
	}
	c.mu.Unlock()
	return st.Write(p)
}

func (c *hostConn) ResizePixels(cols, rows, cellWidth, cellHeight int) error {
	c.mu.Lock()
	c.size = [4]int{cols, rows, cellWidth, cellHeight}
	st := c.st
	c.mu.Unlock()
	if st == nil {
		return nil
	}
	return st.ResizePixels(cols, rows, cellWidth, cellHeight)
}

func (c *hostConn) UseTheme(theme *terminal.Theme) error {
	c.mu.Lock()
	c.theme = theme
	st := c.st
	c.mu.Unlock()
	if st == nil {
		return nil
	}
	return st.UseTheme(theme)
}

// setVisible tells the host whether the tab shows.
func (c *hostConn) setVisible(visible bool) {
	c.mu.Lock()
	v := 1
	if visible {
		v = 2
	}
	changed := v != c.visible
	c.visible = v
	st := c.st
	c.mu.Unlock()
	if st != nil && changed {
		st.SetVisible(visible)
	}
}

func (c *hostConn) Close() error {
	c.mu.Lock()
	c.closed = true
	st := c.st
	c.mu.Unlock()
	c.once.Do(func() { close(c.ready) })
	if st != nil {
		return st.Close()
	}
	return nil
}

// lost reports whether the host went away under the conn's terminal.
func (c *hostConn) lost() bool {
	st := c.stream()
	if st == nil {
		return false
	}
	_, _, lost := st.Ended()
	return lost
}

// disconnected is what a terminal whose host went away shows before the
// shell that takes its place (design.md 3.6, copy.md 13): what it showed,
// and a dim line (ANSI 90), as a command that ended says so.
func disconnected(history []byte) []byte {
	line := "\x1b[0;90m" + locales.TermDisconnected() + "\x1b[0m\r\n"
	return append(slices.Clip(history), line...)
}
