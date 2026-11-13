package termhost

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// How often a stream's frames are read: nothing tells a terminal no view
// shows that it changed, so each stream polls its Mirror, often while
// output comes and less and less often while none does, the least while
// the client does not show the terminal.
const (
	frameFast   = 8 * time.Millisecond    // after a frame
	frameFirst  = 2 * time.Millisecond    // after input, the echo
	frameDue    = 16 * time.Millisecond   // a synchronized update holds one back
	frameIdle   = 250 * time.Millisecond  // shown, nothing new
	frameHidden = time.Second             // not shown
	closeGrace  = 2500 * time.Millisecond // HUP, then the plugin's kill after 2 s
	idleExit    = 10 * time.Second        // after the last terminal ended
	streamGrace = 2 * time.Second         // for a stream to take its last frame
	goneFor     = 60 * time.Second        // the app missing this long is uninstalled
	goneEvery   = 30 * time.Second        // how often the host looks
	helloWait   = 5 * time.Second         // for a connection's hello
	maxPaste    = 32 << 20                // the most a paste carries
	errNoTerm   = "termhost: no such terminal"
	errExiting  = "termhost: the host is exiting"
	pidFileMode = 0o600
)

// Options configure a Host.
type Options struct {
	// Build is the version of Gocast the host is, said in its hello.
	Build string
	// IdleExit is how long after its last terminal ended the host's Done
	// closes: 10 seconds when zero.
	IdleExit time.Duration
	// AfterFunc runs f after d, as time.AfterFunc does, and returns what
	// stops it: a fake clock in tests.
	AfterFunc func(d time.Duration, f func()) (stop func() bool)
	// Sleep waits between Accept's failures: time.Sleep when nil.
	Sleep func(time.Duration)
	// HelloWait is how long a connection has to say hello: 5 seconds
	// when zero.
	HelloWait time.Duration
}

// Host runs terminals for clients: Gocast, which shows them, as many
// times as it is started.
type Host struct {
	opts Options

	mu       sync.Mutex
	sessions []*session // in the order they were made: the tabs' order
	active   string
	conns    int
	idleStop func() bool
	idleGen  int
	exiting  bool
	ls       []net.Listener

	done     chan struct{}
	doneOnce sync.Once

	// Tests': changed is told, when set, that the sessions changed
	// (one ended, its preset's command ended); beforeAdd runs as create
	// is about to add a session.
	changed   func()
	beforeAdd func()
}

// New returns a host with no terminals, whose idle time starts now.
func New(opts Options) *Host {
	if opts.IdleExit <= 0 {
		opts.IdleExit = idleExit
	}
	if opts.AfterFunc == nil {
		opts.AfterFunc = func(d time.Duration, f func()) func() bool { return time.AfterFunc(d, f).Stop }
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}
	if opts.HelloWait <= 0 {
		opts.HelloWait = helloWait
	}
	h := &Host{opts: opts, done: make(chan struct{})}
	h.mu.Lock()
	h.armIdle()
	h.mu.Unlock()
	return h
}

// Done is closed when the host should exit: IdleExit after its last
// terminal ended, or after CloseAll.
func (h *Host) Done() <-chan struct{} { return h.done }

// stopTaking has the host take no new terminal nor connection, as it is
// about to exit: create fails, and the socket goes, for a host started
// meanwhile to take its place once it exited. With h.mu held.
func (h *Host) stopTaking() {
	h.exiting = true
	for _, l := range h.ls {
		l.Close()
	}
	h.ls = nil
}

func (h *Host) exit() {
	h.mu.Lock()
	h.stopTaking()
	h.mu.Unlock()
	h.doneOnce.Do(func() { close(h.done) })
}

func (h *Host) tell() {
	if h.changed != nil {
		h.changed()
	}
}

// Clients is how many connections are open.
func (h *Host) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conns
}

// armIdle starts the idle time when no terminal runs. With h.mu held.
func (h *Host) armIdle() {
	if len(h.sessions) > 0 || h.idleStop != nil {
		return
	}
	h.idleGen++
	gen := h.idleGen
	h.idleStop = h.opts.AfterFunc(h.opts.IdleExit, func() {
		h.mu.Lock()
		if gen != h.idleGen { // stopped, or another started
			h.mu.Unlock()
			return
		}
		h.idleStop = nil
		idle := len(h.sessions) == 0
		if idle {
			h.stopTaking() // with the look: no terminal gets in between
		}
		h.mu.Unlock()
		if idle {
			h.exit()
		}
	})
}

// disarmIdle stops the idle time, as a terminal starts. With h.mu held.
func (h *Host) disarmIdle() {
	if h.idleStop != nil {
		h.idleStop()
		h.idleStop = nil
	}
	h.idleGen++
}

// Serve serves the connections l accepts until it is closed, or the host
// exits. Failures (too many files open) are waited out, longer and longer.
func (h *Host) Serve(l net.Listener) error {
	h.mu.Lock()
	if h.exiting {
		h.mu.Unlock()
		l.Close()
		return net.ErrClosed
	}
	h.ls = append(h.ls, l)
	h.mu.Unlock()
	var wait time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			h.mu.Lock()
			exiting := h.exiting
			h.mu.Unlock()
			if exiting || errors.Is(err, net.ErrClosed) {
				return err
			}
			wait = min(max(2*wait, 5*time.Millisecond), time.Second)
			h.opts.Sleep(wait)
			continue
		}
		wait = 0
		go h.ServeConn(c)
	}
}

// ServeConn serves a client's connection, until it closes: a control
// connection, or a stream once it attaches.
func (h *Host) ServeConn(c net.Conn) {
	h.mu.Lock()
	h.conns++
	h.mu.Unlock()
	defer func() {
		c.Close()
		h.mu.Lock()
		h.conns--
		h.mu.Unlock()
	}()
	r := bufio.NewReader(c)
	// A connection that says nothing is no client.
	_ = c.SetReadDeadline(time.Now().Add(h.opts.HelloWait))
	typ, _, err := readMsg(r)
	if err != nil || typ != msgHello {
		return
	}
	_ = c.SetReadDeadline(time.Time{})
	// Any version: later ones only add (see Proto).
	if writeJSON(c, msgReply, reply{Hello: &hello{Proto: Proto, Build: h.opts.Build, PID: os.Getpid(), Caps: caps}}) != nil {
		return
	}
	for {
		typ, p, err := readMsg(r)
		if err != nil {
			return
		}
		if typ == msgAttach {
			h.attach(c, r, p)
			return
		}
		rep := h.request(typ, p)
		if writeJSON(c, msgReply, rep) != nil {
			return
		}
		if typ == msgCloseAll {
			return // the host exits (design.md 5)
		}
	}
}

// request answers a request of the control connection.
func (h *Host) request(typ byte, p []byte) reply {
	fail := func(err error) reply { return reply{Error: err.Error()} }
	switch typ {
	case msgList:
		h.mu.Lock()
		defer h.mu.Unlock()
		rep := reply{Active: h.active, Sessions: []Session{}}
		for _, s := range h.sessions {
			rep.Sessions = append(rep.Sessions, Session{ID: s.id, Meta: s.meta, Title: s.t.Title(), Done: s.done})
		}
		return rep
	case msgCreate:
		var req CreateRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return fail(err)
		}
		id, err := h.create(req)
		if err != nil {
			return fail(err)
		}
		return reply{ID: id}
	case msgClose, msgSetMeta, msgSetActive:
		var m idMsg
		if err := json.Unmarshal(p, &m); err != nil {
			return fail(err)
		}
		h.mu.Lock()
		i := slices.IndexFunc(h.sessions, func(s *session) bool { return s.id == m.ID })
		var s *session
		if i >= 0 {
			s = h.sessions[i]
		}
		switch {
		case typ == msgSetActive:
			h.active = m.ID
		case s == nil:
		case typ == msgSetMeta:
			s.meta = m.Meta
		case typ == msgClose:
			h.sessions = slices.Delete(h.sessions, i, i+1)
			if h.active == s.id {
				h.active = ""
			}
			h.armIdle()
		}
		h.mu.Unlock()
		if s == nil && typ != msgSetActive {
			return fail(errors.New(errNoTerm))
		}
		if typ == msgClose {
			s.close()
		}
		return reply{}
	case msgCloseAll:
		h.CloseAll()
		return reply{}
	}
	return fail(errors.New("termhost: unknown request"))
}

// newID returns a session's ID, never one of a host before.
func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// create starts a terminal: the program starts once the session is the
// host's, unless the host exits meanwhile.
func (h *Host) create(req CreateRequest) (string, error) {
	h.mu.Lock()
	exiting := h.exiting
	h.mu.Unlock()
	if exiting {
		return "", errors.New(errExiting)
	}
	s := &session{h: h, id: newID(), meta: req.Meta, doneTitle: req.DoneTitle, exited: make(chan struct{}), streams: map[*stream]struct{}{}}
	t, err := terminal.New(terminal.Options{
		Command: req.Argv,
		Dir:     req.Dir,
		Env:     req.Env,
		// The client's terminal says nothing either: a tab whose program
		// ended closes, and Gocast says what it has to (design.md 3.6).
		NoExitMessage: true,
		OnTitle: func(title string) {
			if s.doneTitle != "" && title == s.doneTitle {
				h.mu.Lock()
				s.done = true
				h.mu.Unlock()
				h.tell()
			}
		},
	})
	if err != nil {
		return "", err
	}
	s.t = t
	if h.beforeAdd != nil {
		h.beforeAdd()
	}
	h.mu.Lock()
	if h.exiting { // since the look above: the idle time ran out, or Close All
		h.mu.Unlock()
		t.Close() // its program never started
		return "", errors.New(errExiting)
	}
	h.sessions = append(h.sessions, s)
	h.disarmIdle()
	h.mu.Unlock()
	// What the terminal showed goes first, at the width it was laid out at
	// (History), then the program starts, under it: a size starts it, else
	// the first stream's.
	if req.Cols > 0 && req.Rows > 0 {
		if len(req.Prefill) > 0 {
			t.Feed(req.Prefill)
		}
		s.mu.Lock()
		t.ResizePixels(req.Cols, req.Rows, req.CellW, req.CellH) // starts the program
		s.mu.Unlock()
	} else if len(req.Prefill) > 0 {
		t.Feed(req.Prefill)
	}
	go s.watch()
	return s.id, nil
}

// CloseAll ends every terminal, waits for their programs to end (those
// that ignore the hangup are killed after 2 seconds), and has the host
// exit: from the start, it takes no new terminal nor connection.
func (h *Host) CloseAll() {
	h.mu.Lock()
	h.stopTaking()
	ss := h.sessions
	h.sessions = nil
	h.active = ""
	h.mu.Unlock()
	for _, s := range ss {
		s.close()
	}
	deadline := time.After(closeGrace)
wait:
	for _, s := range ss {
		select {
		case <-s.t.Done():
		case <-deadline:
			break wait
		}
	}
	h.exit()
}

// attach makes the connection a stream of the session the payload names,
// until the client closes it or the program ends.
func (h *Host) attach(c net.Conn, r *bufio.Reader, p []byte) {
	var m idMsg
	_ = json.Unmarshal(p, &m)
	h.mu.Lock()
	i := slices.IndexFunc(h.sessions, func(s *session) bool { return s.id == m.ID })
	if i < 0 {
		h.mu.Unlock()
		_ = writeJSON(c, msgReply, reply{Error: errNoTerm})
		return
	}
	s := h.sessions[i]
	mirror, err := s.t.NewMirror()
	if err != nil {
		h.mu.Unlock()
		_ = writeJSON(c, msgReply, reply{Error: err.Error()})
		return
	}
	st := &stream{s: s, c: c, m: mirror, wake: make(chan struct{}, 1), stop: make(chan struct{}), gone: make(chan struct{})}
	st.visible.Store(true)
	s.streams[st] = struct{}{}
	h.mu.Unlock()
	if writeJSON(c, msgReply, reply{ID: s.id}) == nil {
		go st.pump()
		st.read(r)
	} else {
		close(st.gone)
	}
	st.detach()
}

// session is a terminal the host runs.
type session struct {
	h         *Host
	id        string
	t         *terminal.Terminal
	doneTitle string
	exited    chan struct{} // closed once the program ended

	// With h.mu held:
	meta    json.RawMessage
	done    bool
	streams map[*stream]struct{}
	closed  bool

	// mu keeps a frame and the size it was made at together: a resize
	// waits for the frame being made.
	mu sync.Mutex
}

// watch waits for the program to end, gives the streams their last frame
// and the exit, and frees the terminal.
func (s *session) watch() {
	<-s.t.Done()
	h := s.h
	h.mu.Lock()
	if i := slices.Index(h.sessions, s); i >= 0 {
		h.sessions = slices.Delete(h.sessions, i, i+1)
		if h.active == s.id {
			h.active = ""
		}
	}
	h.armIdle()
	var streams []*stream
	for st := range s.streams {
		streams = append(streams, st)
	}
	h.mu.Unlock()
	h.tell()
	close(s.exited)
	deadline := time.After(streamGrace)
	for _, st := range streams {
		select {
		case <-st.gone:
		case <-deadline:
		}
	}
	s.t.Close()
}

// close ends the session at a client's request: its streams close with
// no exit, as the client closed its tab, and the program is hung up.
func (s *session) close() {
	s.h.mu.Lock()
	s.closed = true
	var streams []*stream
	for st := range s.streams {
		streams = append(streams, st)
	}
	s.h.mu.Unlock()
	for _, st := range streams {
		st.c.Close()
	}
	s.t.Close()
}

// frame is the mirror's next frame, with the size it is at.
func (s *session) frame(m *terminal.Mirror) (frame []byte, due bool, cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cols, rows = s.t.Size()
	frame, due = m.Frame()
	return frame, due, cols, rows
}

func (s *session) resize(z sizeMsg) {
	if z.Cols <= 0 || z.Rows <= 0 {
		return
	}
	s.mu.Lock()
	s.t.ResizePixels(z.Cols, z.Rows, z.CellW, z.CellH)
	s.mu.Unlock()
}

// stream is a client's connection to a session: frames go to it, input
// comes from it.
type stream struct {
	s       *session
	c       net.Conn
	m       *terminal.Mirror
	visible atomic.Bool
	wake    chan struct{}
	stop    chan struct{}
	gone    chan struct{} // closed once pump returned
	once    sync.Once
}

// kick has the next frame read now.
func (st *stream) kick() {
	select {
	case st.wake <- struct{}{}:
	default:
	}
}

// read takes the client's messages until the connection closes.
func (st *stream) read(r *bufio.Reader) {
	t := st.s.t
	for {
		typ, p, err := readMsg(r)
		if err != nil {
			return
		}
		switch typ {
		case msgInput:
			t.Send(p)
		case msgResize:
			var z sizeMsg
			if json.Unmarshal(p, &z) == nil {
				st.s.resize(z)
			}
		case msgTheme:
			var th themeMsg
			if json.Unmarshal(p, &th) == nil {
				t.UseTheme(th.theme())
			}
		case msgPaste:
			var pm pasteMsg
			if json.Unmarshal(p, &pm) == nil && len(pm.Formats) == len(pm.Data) {
				var reps []transfer.Representation
				for i, f := range pm.Formats {
					reps = append(reps, transfer.Bytes(transfer.Format(f), pm.Data[i]))
				}
				t.PasteData(transfer.New(transfer.NewItem(reps...)))
			}
		case msgClear:
			t.ClearScreen()
		case msgVisible:
			st.visible.Store(len(p) > 0 && p[0] == 1)
		}
		st.kick()
	}
}

// detach ends the stream: the session goes on.
func (st *stream) detach() {
	st.once.Do(func() { close(st.stop) })
	<-st.gone
	st.c.Close()
	st.m.Close()
	h := st.s.h
	h.mu.Lock()
	delete(st.s.streams, st)
	h.mu.Unlock()
}

// pump sends the frames, as often as the terminal changes (see
// frameFast), and once the program ended the last one and the exit.
func (st *stream) pump() {
	defer close(st.gone)
	var delay time.Duration
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	for {
		if delay > 0 {
			timer.Reset(delay)
			select {
			case <-st.wake:
				delay = 0
			case <-timer.C:
			case <-st.stop:
				return
			case <-st.s.exited:
				st.last()
				return
			}
			timer.Stop()
		} else {
			select {
			case <-st.stop:
				return
			case <-st.s.exited:
				st.last()
				return
			default:
			}
		}
		frame, due, cols, rows := st.s.frame(st.m)
		switch {
		case len(frame) > 0:
			if writeMsg(st.c, msgFrame, frameMsg(cols, rows, frame)) != nil {
				st.c.Close() // the client sees it end, not a terminal that stopped
				return
			}
			delay = frameFast
		case due:
			delay = frameDue
		default:
			most := frameIdle
			if !st.visible.Load() {
				most = frameHidden
			}
			delay = min(max(2*delay, frameFirst), most)
		}
	}
}

// last sends the last frame and the program's exit, and closes the
// stream.
func (st *stream) last() {
	if frame, _, cols, rows := st.s.frame(st.m); len(frame) > 0 {
		if writeMsg(st.c, msgFrame, frameMsg(cols, rows, frame)) != nil {
			return
		}
	}
	_ = writeJSON(st.c, msgExit, exitMsg{Code: st.s.t.ExitCode()})
	st.c.Close()
}

func (th themeMsg) theme() *terminal.Theme {
	c := func(v [4]uint8) ui.Color { return ui.Color{R: v[0], G: v[1], B: v[2], A: v[3]} }
	t := &terminal.Theme{Foreground: c(th.Foreground), Background: c(th.Background), Cursor: c(th.Cursor), CursorText: c(th.CursorText)}
	for i, p := range th.Palette {
		t.Palette[i] = c(p)
	}
	return t
}

func themeOf(t *terminal.Theme) themeMsg {
	c := func(v ui.Color) [4]uint8 { return [4]uint8{v.R, v.G, v.B, v.A} }
	th := themeMsg{Foreground: c(t.Foreground), Background: c(t.Background), Cursor: c(t.Cursor), CursorText: c(t.CursorText)}
	for i, p := range t.Palette {
		th.Palette[i] = c(p)
	}
	return th
}
