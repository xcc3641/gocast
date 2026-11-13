package termhost

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

// fakeClock stands for the host's timers: they fire as fire runs them.
type fakeClock struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

type fakeTimer struct {
	f       func()
	stopped bool
}

func (c *fakeClock) after(_ time.Duration, f func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tm := &fakeTimer{f: f}
	c.timers = append(c.timers, tm)
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		was := !tm.stopped
		tm.stopped = true
		return was
	}
}

// fire runs the timers armed and not stopped, and reports how many.
func (c *fakeClock) fire() int {
	c.mu.Lock()
	var due []*fakeTimer
	for _, tm := range c.timers {
		if !tm.stopped {
			tm.stopped = true
			due = append(due, tm)
		}
	}
	c.timers = nil
	c.mu.Unlock()
	for _, tm := range due {
		tm.f()
	}
	return len(due)
}

// testHost is a host in the test's process, and a client of it over
// pipes.
func testHost(t *testing.T) (*Host, *Client, *fakeClock) {
	h, c, clock, _ := testHostConns(t)
	return h, c, clock
}

// testHostConns is testHost, with what breaks every connection from the
// host's side, as the host's process dying would.
func testHostConns(t *testing.T) (*Host, *Client, *fakeClock, func()) {
	t.Helper()
	if err := terminal.Load(); err != nil {
		t.Skip("no libghostty-vt:", err)
	}
	clock := &fakeClock{}
	h := New(Options{Build: "test", AfterFunc: clock.after})
	var mu sync.Mutex
	var conns []net.Conn
	kill := func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	}
	c, err := NewClient(func() (net.Conn, error) {
		a, b := net.Pipe()
		mu.Lock()
		conns = append(conns, b)
		mu.Unlock()
		go h.ServeConn(b)
		return a, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
		h.CloseAll()
	})
	return h, c, clock, kill
}

// waitFor waits for cond, failing after 10 seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// show attaches a terminal of the client's to the session, as Gocast's
// tab does.
func show(t *testing.T, c *Client, id string) (*terminal.Terminal, *Stream) {
	t.Helper()
	st, err := c.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	term, err := terminal.New(terminal.Options{Conn: st, ResizeByConn: true, NoExitMessage: true})
	if err != nil {
		t.Fatal(err)
	}
	st.Bind(func(cols, rows int) { term.Resize(cols, rows) })
	t.Cleanup(func() { term.Close() })
	return term, st
}

// echo is a program that answers each line typed, until it reads "bye".
var echo = []string{"/bin/sh", "-c", `while read l; do [ "$l" = bye ] && exit 3; echo "got $l"; done`}

// A terminal shows in the client as it does in the host, at the host's
// size; detached, it runs on, and attached again it shows all it showed,
// and what came meanwhile.
func TestAttachAgain(t *testing.T) {
	_, c, _ := testHost(t)
	id, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 40, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	term, st := show(t, c, id)
	st.Write([]byte("one\r"))
	waitFor(t, "got one", func() bool { return strings.Contains(term.Text(), "got one") })
	if cols, rows := term.Size(); cols != 40 || rows != 10 {
		t.Errorf("size %dx%d, want the host's 40x10", cols, rows)
	}

	// Gocast quits: its stream closes, the program goes on.
	term.Close()
	if _, _, lost := st.Ended(); lost {
		t.Error("closed by the client: taken as lost")
	}
	sessions, _, err := c.List()
	if err != nil || len(sessions) != 1 || sessions[0].ID != id {
		t.Fatalf("after detaching: %+v %v", sessions, err)
	}

	term2, st2 := show(t, c, id)
	waitFor(t, "the output before", func() bool { return strings.Contains(term2.Text(), "got one") })
	st2.Write([]byte("two\r"))
	waitFor(t, "got two", func() bool { return strings.Contains(term2.Text(), "got two") })

	// The view's size goes to the host, whose frames bring it back.
	st2.ResizePixels(52, 14, 8, 16)
	waitFor(t, "52x14", func() bool { c, r := term2.Size(); return c == 52 && r == 14 })
	if !strings.Contains(term2.Text(), "got one") {
		t.Errorf("resized: %q", term2.Text())
	}
}

// A host started for nothing exits after the idle time.
func TestIdleAtStart(t *testing.T) {
	h, _, clock := testHost(t)
	if n := clock.fire(); n != 1 {
		t.Fatalf("%d idle timers at the start", n)
	}
	select {
	case <-h.Done():
	default:
		t.Fatal("idle at the start: not done")
	}
}

// A program that ends while shown sends its last output, then its exit
// code; the host forgets it, and with no terminal left, exits after the
// idle time.
func TestExitCode(t *testing.T) {
	h, c, clock := testHost(t)
	id, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 40, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	if n := clock.fire(); n != 0 {
		t.Fatalf("%d idle timers with a terminal running", n)
	}
	exited := make(chan int, 1)
	st, err := c.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	term, err := terminal.New(terminal.Options{Conn: st, ResizeByConn: true, NoExitMessage: true, OnExit: func(int) {
		_, code, _ := st.Ended()
		exited <- code
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	st.Bind(func(cols, rows int) { term.Resize(cols, rows) })
	st.Write([]byte("last\r"))
	waitFor(t, "got last", func() bool { return strings.Contains(term.Text(), "got last") })
	st.Write([]byte("bye\r"))
	select {
	case code := <-exited:
		if code != 3 {
			t.Errorf("exit code %d, want 3", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no exit")
	}
	if ex, _, lost := st.Ended(); !ex || lost {
		t.Errorf("ended: exited %v, lost %v", ex, lost)
	}
	waitFor(t, "the session gone", func() bool { s, _, _ := c.List(); return len(s) == 0 })
	waitFor(t, "the idle timer", func() bool { return clock.fire() == 1 })
	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("idle: not done")
	}
}

// A program that ends with nobody attached is forgotten.
func TestExitDetached(t *testing.T) {
	_, c, _ := testHost(t)
	if _, err := c.Create(CreateRequest{Argv: []string{"/bin/sh", "-c", "exit 0"}, Dir: "/", Cols: 20, Rows: 5}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the session gone", func() bool { s, _, _ := c.List(); return len(s) == 0 })
}

// The client's meta, the tab showing and a preset's end come back in
// the list, in the order the terminals were made; closing one drops it,
// and Close All ends the rest, and the host.
func TestListAndCloseAll(t *testing.T) {
	h, c, _ := testHost(t)
	var ids []string
	for i, title := range []string{"", "gocast:ended"} {
		req := CreateRequest{Argv: []string{"/bin/sh", "-c", `printf '\033]2;%s\007' "$1"; sleep 30`, "sh", title}, Dir: "/", Cols: 20, Rows: 5}
		req.Meta = json.RawMessage(`{"n":` + string(rune('0'+i)) + `}`)
		req.DoneTitle = "gocast:ended"
		id, err := c.Create(req)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	id3, err := c.Create(CreateRequest{Argv: []string{"/bin/sleep", "30"}, Dir: "/", Cols: 20, Rows: 5})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetMeta(ids[0], json.RawMessage(`{"n":9}`)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetActive(ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := c.CloseTerminal(id3); err != nil {
		t.Fatal(err)
	}
	if err := c.CloseTerminal("nope"); err == nil {
		t.Error("closed a terminal that isn't")
	}
	waitFor(t, "the preset's end", func() bool {
		s, _, _ := c.List()
		return len(s) == 2 && s[1].Done
	})
	s, active, _ := c.List()
	if s[0].ID != ids[0] || string(s[0].Meta) != `{"n":9}` || s[0].Done || s[1].ID != ids[1] || string(s[1].Meta) != `{"n":1}` || active != ids[1] {
		t.Errorf("list: %+v, active %q", s, active)
	}

	if err := c.CloseAll(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("Close All: the host goes on")
	}
	h.mu.Lock()
	n := len(h.sessions)
	h.mu.Unlock()
	if n != 0 {
		t.Errorf("%d sessions after Close All", n)
	}
}

// A terminal cut off starts again with what it showed, at its size.
func TestPrefill(t *testing.T) {
	_, c, _ := testHost(t)
	id, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 30, Rows: 6, Prefill: []byte("before\r\n\x1b[90mcut off\x1b[0m\r\n")})
	if err != nil {
		t.Fatal(err)
	}
	term, st := show(t, c, id)
	st.Write([]byte("x\r"))
	waitFor(t, "got x", func() bool { return strings.Contains(term.Text(), "got x") })
	text := term.Text()
	if i, j := strings.Index(text, "cut off"), strings.Index(text, "got x"); i < 0 || j < i || !strings.HasPrefix(text, "before") {
		t.Errorf("text: %q", text)
	}
}

// Attaching to a terminal that is not there fails, without breaking the
// client.
func TestAttachMissing(t *testing.T) {
	_, c, _ := testHost(t)
	if _, err := c.Attach("nope"); err == nil {
		t.Fatal("attached")
	}
	if _, _, err := c.List(); err != nil {
		t.Fatal(err)
	}
}

// A host that goes away ends its streams as lost, not exited.
func TestHostLost(t *testing.T) {
	_, c, _, kill := testHostConns(t)
	id, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 30, Rows: 6})
	if err != nil {
		t.Fatal(err)
	}
	ended := make(chan struct{})
	st, err := c.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	term, err := terminal.New(terminal.Options{Conn: st, ResizeByConn: true, NoExitMessage: true, OnExit: func(int) { close(ended) }})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	st.Bind(func(cols, rows int) { term.Resize(cols, rows) })
	st.Write([]byte("x\r"))
	waitFor(t, "got x", func() bool { return strings.Contains(term.Text(), "got x") })
	kill()
	select {
	case <-ended:
	case <-time.After(10 * time.Second):
		t.Fatal("not ended")
	}
	if exited, _, lost := st.Ended(); exited || !lost {
		t.Errorf("exited %v, lost %v", exited, lost)
	}
	if _, _, err := c.List(); err == nil || !c.Lost() {
		t.Errorf("the client's requests: %v", err)
	}
}

// The app missing at looks a minute apart, with no Gocast connected, is
// uninstalled; missing for less, or with Gocast connected, it is being
// updated, or still running.
func TestGoneCheck(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	var g goneCheck
	if g.step(t0, false, 0) || g.step(t0.Add(30*time.Second), false, 0) {
		t.Fatal("gone after 30 s")
	}
	if !g.step(t0.Add(60*time.Second), false, 0) {
		t.Fatal("missing for 60 s: not gone")
	}

	g = goneCheck{}
	g.step(t0, false, 0)
	g.step(t0.Add(30*time.Second), true, 0) // the update put it back
	if g.step(t0.Add(60*time.Second), false, 0) || g.step(t0.Add(90*time.Second), false, 0) {
		t.Fatal("gone, though it came back in between")
	}
	if !g.step(t0.Add(120*time.Second), false, 0) {
		t.Fatal("missing again for 60 s: not gone")
	}

	g = goneCheck{}
	g.step(t0, false, 1)
	if g.step(t0.Add(5*time.Minute), false, 1) {
		t.Fatal("gone while a Gocast is connected")
	}
	if !g.step(t0.Add(6*time.Minute), false, 0) {
		t.Fatal("the Gocast quit: not gone")
	}
	if g := (goneCheck{}); g.step(t0, false, 0) {
		t.Fatal("gone at the first look")
	}
}

func TestBundleOf(t *testing.T) {
	for exe, want := range map[string]string{
		"/Applications/Gocast.app/Contents/MacOS/Gocast":         "/Applications/Gocast.app",
		"/Users/a/Applications/Gocast.app/Contents/MacOS/Gocast": "/Users/a/Applications/Gocast.app",
		"/Users/a/src/gocast/build/gocast":                       "",
	} {
		if got := bundleOf(exe); got != want {
			t.Errorf("bundleOf(%q) = %q, want %q", exe, got, want)
		}
	}
}

// The socket is in the data folder, or in the temporary folder when that
// path is too long for a socket; two data folders never share one.
func TestSocketPath(t *testing.T) {
	short := "/Users/a/Library/Application Support/com.hugo.gocast"
	if got := SocketPath(short); got != short+"/terminal.sock" {
		t.Errorf("short: %q", got)
	}
	long := "/Users/a/" + strings.Repeat("x", 100)
	got := SocketPath(long)
	if len(got) >= maxSocketPath || filepath.Dir(got) != filepath.Clean(os.TempDir()) || got == SocketPath(long+"y") {
		t.Errorf("long: %q", got)
	}
}

// Over the Unix socket: a client finds the host Main would serve, and
// none where no host runs.
func TestDial(t *testing.T) {
	if err := terminal.Load(); err != nil {
		t.Skip("no libghostty-vt:", err)
	}
	dir, err := os.MkdirTemp("", "th") // short: a socket's path is
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if _, err := Dial(dir); err == nil {
		t.Fatal("dialed no host")
	}
	l, err := net.Listen("unix", SocketPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	h := New(Options{Build: "x"})
	go h.Serve(l)
	c, err := Dial(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.PID() != os.Getpid() || !c.Has(CapPaste) {
		t.Errorf("hello: %+v", c.hello)
	}
}
