package termhost

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
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

// poke tells a test waiting that what it waits for may have come: a
// stream read again, the host's sessions changed. Tests wait for these,
// not for time to pass; one that never comes is go test's timeout.
type poke chan struct{}

func newPoke() poke { return make(chan struct{}, 1) }

func (p poke) tell() {
	select {
	case p <- struct{}{}:
	default:
	}
}

// until waits for cond, looking again each time p is told.
func until(p poke, cond func() bool) {
	for !cond() {
		<-p
	}
}

// pokes are the test host's: its sessions changed.
var hostPokes sync.Map // *Host → poke

func changed(h *Host) poke {
	p, _ := hostPokes.Load(h)
	return p.(poke)
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
	p := newPoke()
	h.changed = p.tell
	hostPokes.Store(h, p)
	t.Cleanup(func() { hostPokes.Delete(h) })
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

// shown is a terminal of the client's showing a session, as Gocast's tab.
type shown struct {
	*terminal.Terminal
	st   *Stream
	read poke // the terminal wrote what it read
}

// waitText waits for the terminal to show s.
func (sh *shown) waitText(s string) {
	until(sh.read, func() bool { return strings.Contains(sh.Text(), s) })
}

// show attaches a terminal of the client's to the session, as Gocast's
// tab does; onExit, when set, runs as the stream ends.
func show(t *testing.T, c *Client, id string, onExit func(int)) *shown {
	t.Helper()
	st, err := c.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	sh := &shown{st: st, read: newPoke()}
	st.readHook = sh.read.tell
	term, err := terminal.New(terminal.Options{Conn: st, ResizeByConn: true, NoExitMessage: true, OnExit: onExit})
	if err != nil {
		t.Fatal(err)
	}
	sh.Terminal = term
	st.Bind(func(cols, rows int) { term.Resize(cols, rows) })
	t.Cleanup(func() { term.Close() })
	return sh
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
	term := show(t, c, id, nil)
	st := term.st
	st.Write([]byte("one\r"))
	term.waitText("got one")
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

	term2 := show(t, c, id, nil)
	term2.waitText("got one") // the output before
	term2.st.Write([]byte("two\r"))
	term2.waitText("got two")

	// The view's size goes to the host, whose frames bring it back.
	term2.st.ResizePixels(52, 14, 8, 16)
	until(term2.read, func() bool { c, r := term2.Size(); return c == 52 && r == 14 })
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
	exited := make(chan struct{})
	term := show(t, c, id, func(int) { close(exited) })
	st := term.st
	st.Write([]byte("last\r"))
	term.waitText("got last")
	st.Write([]byte("bye\r"))
	<-exited
	if ex, code, lost := st.Ended(); !ex || lost || code != 3 {
		t.Errorf("ended: exited %v, code %d (want 3), lost %v", ex, code, lost)
	}
	until(changed(h), func() bool { s, _, _ := c.List(); return len(s) == 0 }) // the session gone
	if n := clock.fire(); n != 1 {
		t.Fatalf("%d idle timers once the last terminal ended", n)
	}
	select {
	case <-h.Done():
	default:
		t.Fatal("idle: not done")
	}
}

// A program that ends with nobody attached is forgotten.
func TestExitDetached(t *testing.T) {
	h, c, _ := testHost(t)
	if _, err := c.Create(CreateRequest{Argv: []string{"/bin/sh", "-c", "exit 0"}, Dir: "/", Cols: 20, Rows: 5}); err != nil {
		t.Fatal(err)
	}
	until(changed(h), func() bool { s, _, _ := c.List(); return len(s) == 0 })
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
	until(changed(h), func() bool { // the preset's end
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
	default:
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
	term := show(t, c, id, nil)
	term.st.Write([]byte("x\r"))
	term.waitText("got x")
	text := term.Text()
	if i, j := strings.Index(text, "cut off"), strings.Index(text, "got x"); i < 0 || j < i || !strings.HasPrefix(text, "before") {
		t.Errorf("text: %q", text)
	}

	// Laid out wider than the terminal starts: a row as wide stays one.
	wide := strings.Repeat("w", 70)
	id, err = c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 100, Rows: 6, Prefill: []byte(wide + "\r\n")})
	if err != nil {
		t.Fatal(err)
	}
	term = show(t, c, id, nil)
	term.st.Write([]byte("y\r"))
	term.waitText("got y")
	if text := term.Text(); !strings.HasPrefix(text, wide+"\n") {
		t.Errorf("wide: %q", text)
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
	term := show(t, c, id, func(int) { close(ended) })
	st := term.st
	st.Write([]byte("x\r"))
	term.waitText("got x")
	kill()
	<-ended
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

// The bundle's identifier comes from its Info.plist; a build not
// packaged has none.
func TestBundleID(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Gocast Dev.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o700); err != nil {
		t.Fatal(err)
	}
	plist := "<plist><dict>\n\t<key>CFBundleName</key>\n\t<string>Gocast</string>\n\t<key>CFBundleIdentifier</key>\n\t<string>com.hugo.gocast.dev</string>\n</dict></plist>\n"
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := BundleID(filepath.Join(app, "Contents", "MacOS", "Gocast")); got != "com.hugo.gocast.dev" {
		t.Errorf("bundle: %q", got)
	}
	if got := BundleID("/Users/a/src/gocast/build/gocast"); got != "" {
		t.Errorf("go build: %q", got)
	}
}

// The app gone from where it was is uninstalled only when Spotlight
// knows of no other copy: one moved is followed; one in the Trash, in a
// disk image or translocated doesn't count; Spotlight failing is not
// knowing, never gone. An app run from a disk image or translocated
// isn't watched at all.
func TestBundleWatch(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	plists := map[string]bool{}
	var found []string
	var findErr error
	w := bundleWatch{
		bundle: "/Applications/Gocast.app",
		id:     "com.hugo.gocast",
		exists: func(p string) bool { return plists[p] },
		find:   func(string) ([]string, error) { return found, findErr },
	}
	plist := func(b string) string { return b + "/Contents/Info.plist" }
	look := func(at time.Duration) bool { return w.step(t0.Add(at), 0) }

	plists[plist("/Applications/Gocast.app")] = true
	if look(0) || look(time.Minute) || look(2*time.Minute) {
		t.Fatal("there: gone")
	}

	// Moved to ~/Applications: followed.
	delete(plists, plist("/Applications/Gocast.app"))
	moved := "/Users/a/Applications/Gocast.app"
	plists[plist(moved)] = true
	found = []string{"/Users/a/.Trash/Gocast.app", moved}
	if look(3*time.Minute) || look(4*time.Minute) || w.bundle != moved {
		t.Fatalf("moved: gone, or not followed (%s)", w.bundle)
	}

	// Spotlight failing: not known, not gone.
	delete(plists, plist(moved))
	findErr = errors.New("mdfind: no")
	if look(5*time.Minute) || look(7*time.Minute) {
		t.Fatal("Spotlight failing: gone")
	}

	// In the Trash, in a disk image, translocated, or a stale entry: gone
	// at the second look a minute after the first.
	findErr = nil
	found = []string{"/Users/a/.Trash/Gocast.app", "/Volumes/Gocast/Gocast.app", "/private/var/folders/x/AppTranslocation/y/d/Gocast.app", "/Applications/Old/Gocast.app"}
	plists[plist("/Users/a/.Trash/Gocast.app")] = true
	plists[plist("/Volumes/Gocast/Gocast.app")] = true
	if look(8 * time.Minute) {
		t.Fatal("gone at the first look")
	}
	if !look(9 * time.Minute) {
		t.Fatal("uninstalled: not gone")
	}

	for b, want := range map[string]bool{
		"/Applications/Gocast.app":                                        false,
		"/Volumes/Gocast/Gocast.app":                                      true,
		"/private/var/folders/ab/cd/T/AppTranslocation/0F1E/d/Gocast.app": true,
	} {
		if unsettled(b) != want {
			t.Errorf("unsettled(%s) = %v", b, !want)
		}
	}
}

// The socket is in the data folder, or, when that path is too long for a
// socket, in a folder of the user's alone in the temporary folder; two
// data folders never share one. A folder others can write in, or a link,
// is refused.
func TestSocketPath(t *testing.T) {
	short := "/Users/a/Library/Application Support/com.hugo.gocast"
	if got, err := SocketPath(short); err != nil || got != short+"/terminal.sock" {
		t.Errorf("short: %q %v", got, err)
	}
	tmp := t.TempDir()
	long := "/Users/a/" + strings.Repeat("x", 100)
	got, err := socketPathIn(long, tmp)
	own := filepath.Join(tmp, fmt.Sprintf("gocast-%d", os.Getuid()))
	if err != nil || filepath.Dir(got) != own {
		t.Fatalf("long: %q %v", got, err)
	}
	if other, _ := socketPathIn(long+"y", tmp); other == got {
		t.Errorf("two folders, one socket: %q", got)
	}
	if fi, err := os.Stat(own); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("the folder: %v %v", fi, err)
	}

	if err := os.Chmod(own, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := socketPathIn(long, tmp); err == nil {
		t.Error("a folder anyone writes in: taken")
	}
	tmp2 := t.TempDir()
	if err := os.Symlink(own, filepath.Join(tmp2, fmt.Sprintf("gocast-%d", os.Getuid()))); err != nil {
		t.Fatal(err)
	}
	if _, err := socketPathIn(long, tmp2); err == nil {
		t.Error("a link: taken")
	}
}

// shortDir is a folder for a socket, whose path must be under 104 bytes:
// t.TempDir's are longer on macOS.
func shortDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "th")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// noTimer stands for the idle timer, which never runs out.
func noTimer(time.Duration, func()) func() bool { return func() bool { return true } }

// Over the Unix socket: a client finds the host Main would serve, and
// none where no host runs, nor once that host is ending.
func TestDial(t *testing.T) {
	if err := terminal.Load(); err != nil {
		t.Skip("no libghostty-vt:", err)
	}
	dir := shortDir(t)
	if _, err := Dial(dir); err == nil {
		t.Fatal("dialed no host")
	}
	sock, err := SocketPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	h := New(Options{Build: "x", AfterFunc: noTimer})
	go h.Serve(l)
	c, err := Dial(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.PID() != os.Getpid() || !c.Has(CapPaste) {
		t.Errorf("hello: %+v", c.hello)
	}
	h.CloseAll()
	if _, err := Dial(dir); err == nil {
		t.Fatal("dialed a host that ends")
	}
}

// The programs the host starts get the hangup and broken pipes as
// programs do, by default, not ignored as the host would have them: else
// a hangup, as a tab closes, ends none of them. (Regression: the host
// ignored both, and every program with it.)
func TestChildSignals(t *testing.T) {
	if _, err := os.Stat("/usr/bin/perl"); err != nil {
		t.Skip("no perl:", err)
	}
	stop := hostSignals()
	defer stop()
	_, c, _ := testHost(t)
	id, err := c.Create(CreateRequest{Argv: []string{"/usr/bin/perl", "-e", `print "HUP=", ($SIG{HUP} // "default"), " PIPE=", ($SIG{PIPE} // "default"), " ok\n"; sleep 30`}, Dir: "/", Cols: 60, Rows: 5})
	if err != nil {
		t.Fatal(err)
	}
	term := show(t, c, id, nil)
	term.waitText(" ok")
	if text := term.Text(); !strings.Contains(text, "HUP=default PIPE=default ok") {
		t.Errorf("the program's signals: %q", text)
	}
}

// A terminal never starts in a host that is exiting: the idle time
// running out as it was being made, or Close All, from this connection
// or another, the client finds the host lost, for one started anew to
// take it. (Regression: it got an ID, then the host exited under it; or
// the error was taken for good, and the tab ran in Gocast.)
func TestCreateWhileExiting(t *testing.T) {
	h, c, clock := testHost(t)
	h.beforeAdd = func() { clock.fire() } // the idle time runs out now
	if _, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 20, Rows: 5}); !errors.Is(err, ErrLost) || !c.Lost() {
		t.Fatalf("created as the idle time ran out: %v, lost %v", err, c.Lost())
	}
	select {
	case <-h.Done():
	default:
		t.Fatal("idle: not done")
	}
	if n := sessionsOf(h); n != 0 {
		t.Fatalf("%d sessions in a host exiting", n)
	}

	h, c, _ = testHost(t)
	other, err := NewClient(c.dial)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	h.beforeAdd = func() { _ = other.CloseAll() }
	if _, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 20, Rows: 5}); !errors.Is(err, ErrLost) {
		t.Fatalf("created during Close All: %v", err)
	}
	if !other.Lost() {
		t.Error("after Close All: the client's host not lost")
	}

	h, c, _ = testHost(t)
	if err := c.CloseAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Create(CreateRequest{Argv: echo, Dir: "/", Cols: 20, Rows: 5}); !errors.Is(err, ErrLost) {
		t.Fatalf("created after Close All: %v", err)
	}
	if n := sessionsOf(h); n != 0 {
		t.Fatalf("%d sessions after Close All", n)
	}
}

func sessionsOf(h *Host) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}

// flakyListener fails its first Accepts, then gives conns, until closed.
type flakyListener struct {
	fails  int
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func (l *flakyListener) Accept() (net.Conn, error) {
	if l.fails > 0 {
		l.fails--
		return nil, syscall.EMFILE
	}
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *flakyListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *flakyListener) Addr() net.Addr { return &net.UnixAddr{Name: "flaky", Net: "unix"} }

// Accept failing (too many files open) is waited out, longer each time,
// and the host serves again after; it stops once closed. (Regression:
// one failure and no Gocast could reach the host again.)
func TestServeAcceptFails(t *testing.T) {
	var waits []time.Duration
	h := New(Options{AfterFunc: noTimer, Sleep: func(d time.Duration) { waits = append(waits, d) }})
	l := &flakyListener{fails: 3, conns: make(chan net.Conn), closed: make(chan struct{})}
	served := make(chan error)
	go func() { served <- h.Serve(l) }()
	a, b := net.Pipe()
	l.conns <- b
	c, err := NewClient(func() (net.Conn, error) { return a, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if want := []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond}; !slices.Equal(waits, want) {
		t.Errorf("waited %v, want %v", waits, want)
	}
	h.CloseAll()
	if err := <-served; !errors.Is(err, net.ErrClosed) {
		t.Errorf("served until %v", err)
	}
}

// A connection that never says hello is closed, and not counted as a
// Gocast connected, which would keep the host from noticing it was
// uninstalled.
func TestHelloWait(t *testing.T) {
	h := New(Options{AfterFunc: noTimer, HelloWait: time.Millisecond})
	a, b := net.Pipe()
	defer a.Close()
	done := make(chan struct{})
	go func() {
		h.ServeConn(b)
		close(done)
	}()
	<-done
	if n := h.Clients(); n != 0 {
		t.Errorf("%d clients", n)
	}
}

// A stream that reads something that is not a frame ends as cut off from
// the host, not as the program's end: the tab looks for its terminal
// again, rather than closing it.
func TestMalformedFrame(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	st := &Stream{conn: a, r: bufio.NewReader(a), ready: make(chan struct{})}
	st.Bind(nil)
	go writeMsg(b, msgFrame, []byte{1})
	if n, err := st.Read(make([]byte, 8)); n != 0 || err != io.EOF {
		t.Fatalf("read %d, %v", n, err)
	}
	if exited, _, lost := st.Ended(); exited || !lost {
		t.Errorf("exited %v, lost %v", exited, lost)
	}
}

// The host closes the descriptors it got without close-on-exec, which
// its programs would get too, and keeps its own.
func TestCloseInherited(t *testing.T) {
	inherited, err := syscall.Open("/dev/null", syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	own, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer own.Close()
	closeInherited(inherited, inherited+1)
	closeInherited(int(own.Fd()), int(own.Fd())+1)
	if _, err := fcntl(inherited, syscall.F_GETFD, 0); err != syscall.EBADF {
		syscall.Close(inherited)
		t.Errorf("inherited: %v, want closed", err)
	}
	if _, err := fcntl(int(own.Fd()), syscall.F_GETFD, 0); err != nil {
		t.Errorf("own: %v", err)
	}
}
