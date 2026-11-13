package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/termhost"
)

// trayLabels are the labels of the tray menu's items, "-" for a
// separator.
func trayLabels(g *gocast) []string {
	var labels []string
	for _, it := range g.trayMenu().Items() {
		if it.Label == "" {
			labels = append(labels, "-")
			continue
		}
		labels = append(labels, it.Label)
	}
	return labels
}

// Close All Terminals shows in the tray menu right above Quit, in the same
// group, only while a terminal tab is open (design.md 5).
func TestTrayCloseAllTerminals(t *testing.T) {
	inLocale(t, "zh-Hans")
	g := &gocast{}
	g.term = &termPanel{g: g, active: -1}
	if got := strings.Join(trayLabels(g), "|"); strings.Contains(got, "结束所有终端") {
		t.Fatalf("no tab: %s", got)
	}
	g.term.tabs = []*session{{key: 1}}
	labels := trayLabels(g)
	n := len(labels)
	if n < 3 || labels[n-3] != "-" || labels[n-2] != "结束所有终端" || labels[n-1] != "退出 Gocast" {
		t.Fatalf("with a tab: %s", strings.Join(labels, "|"))
	}
}

// Close All Terminals asks first: canceled, every tab stays; confirmed,
// they all close and the host is asked to end its terminals, never
// started for it.
func TestCloseAllTerminals(t *testing.T) {
	var spawns []bool
	old := [2]any{sysTermHost, sysConfirmCloseAll}
	t.Cleanup(func() {
		sysTermHost = old[0].(func(string, bool) (*termhost.Client, error))
		sysConfirmCloseAll = old[1].(func() bool)
	})
	sysTermHost = func(_ string, spawn bool) (*termhost.Client, error) {
		spawns = append(spawns, spawn)
		return nil, errors.New("no host")
	}
	confirm := false
	asked := 0
	sysConfirmCloseAll = func() bool { asked++; return confirm }

	g := &gocast{host: &termHost{dir: t.TempDir(), run: func(_ string, fn func()) { fn() }}}
	g.term = &termPanel{g: g, tabs: []*session{{key: 1, id: "a"}, {key: 2}}, active: 1}
	g.term.closeAll()
	if asked != 1 || len(g.term.tabs) != 2 || g.term.active != 1 || len(spawns) != 0 {
		t.Fatalf("canceled: asked %d, %d tabs, active %d, host %v", asked, len(g.term.tabs), g.term.active, spawns)
	}
	confirm = true
	g.term.closeAll()
	if asked != 2 || len(g.term.tabs) != 0 || g.term.active != -1 {
		t.Fatalf("confirmed: asked %d, %d tabs, active %d", asked, len(g.term.tabs), g.term.active)
	}
	if len(spawns) != 1 || spawns[0] {
		t.Fatalf("the host asked with spawn %v, want once, not started", spawns)
	}
	g.term.closeAll() // no tab: nothing to ask
	if asked != 2 {
		t.Fatalf("asked with no tab")
	}
}

// The tabs of the host's terminals come back after Gocast restarted in
// the order they were made, with their presets (renamed since as the
// presets are), folders and font sizes, the one last shown active, a
// command that ended a plain terminal, and before a tab opened meanwhile
// (design.md 3.6).
func TestRestoredTabs(t *testing.T) {
	inLocale(t, "en")
	g := &gocast{settings: settings.Settings{FontSize: 14}}
	g.presets = []presets.Preset{{ID: "p1", Name: "Agent (renamed)", Command: "pi"}}
	p := &termPanel{g: g, active: -1}
	g.term = p
	meta := func(s *session) json.RawMessage { return s.meta() }
	list := []termhost.Session{
		{ID: "t1", Meta: meta(&session{dir: "/tmp/a", fontSize: 13})},
		{ID: "t2", Meta: meta(&session{presetID: "p1", preset: presets.Preset{ID: "p1", Name: "Agent", Command: "pi"}, dir: "/tmp/b", fontSize: 16})},
		{ID: "t3", Meta: meta(&session{presetID: "p2", preset: presets.Preset{ID: "p2", Name: "Gone", Command: "x"}, dir: "/tmp/c"}), Done: true},
		{ID: "t4"}, // no meta: a plain terminal
	}
	p.tabs = []*session{{key: 99, fontSize: 14}} // opened before the host answered
	p.active = 0
	p.addRestored(list, "t2")
	if len(p.tabs) != 5 || p.tabs[4].key != 99 {
		t.Fatalf("%d tabs, the new one at the end: %+v", len(p.tabs), p.tabs)
	}
	if p.active != 4 {
		t.Fatalf("active %d, want the tab opened meanwhile", p.active)
	}
	want := []struct {
		id, title, dir string
		font           float32
	}{
		{"t1", "Terminal", "/tmp/a", 13},
		{"t2", "Agent (renamed)", "/tmp/b", 16},
		{"t3", "Terminal", "/tmp/c", 14},
		{"t4", "Terminal", "", 14},
	}
	for i, w := range want {
		s := p.tabs[i]
		if s.id != w.id || s.title() != w.title || s.dir != w.dir || s.fontSize != w.font || !s.restored {
			t.Errorf("tab %d: %s %q %q %v restored %v, want %+v", i, s.id, s.title(), s.dir, s.fontSize, s.restored, w)
		}
	}
	if p.tabs[2].plain() || !p.tabs[2].done {
		t.Errorf("the ended preset's tab: preset %q, done %v", p.tabs[2].presetID, p.tabs[2].done)
	}

	// Alone, the tab last shown is active; listed again, nothing doubles.
	p = &termPanel{g: g, active: -1}
	g.term = p
	p.addRestored(list, "t2")
	p.addRestored(list, "t2")
	if len(p.tabs) != 4 || p.active != 1 {
		t.Fatalf("%d tabs, active %d", len(p.tabs), p.active)
	}
}

// A terminal cut off from its host keeps what it showed, with a dim line
// under it that says so (copy.md 13).
func TestDisconnectedLine(t *testing.T) {
	inLocale(t, "zh-Hans")
	hist := []byte("$ make\r\nok\r\n")
	got := string(disconnected(hist))
	want := "$ make\r\nok\r\n\x1b[0;90m这个终端断开了，之前的命令已经停了。\x1b[0m\r\n"
	if got != want {
		t.Fatalf("%q, want %q", got, want)
	}
	if string(hist) != "$ make\r\nok\r\n" {
		t.Fatalf("the history changed: %q", hist)
	}
}

// hostForTest is a terminal host in the test's process, which never
// exits by itself, and what dials it, over pipes.
func hostForTest(t *testing.T) (*termhost.Host, func() (net.Conn, error)) {
	t.Helper()
	if err := terminal.Load(); err != nil {
		t.Skipf("no libghostty-vt: %v", err)
	}
	h := termhost.New(termhost.Options{AfterFunc: func(time.Duration, func()) func() bool { return func() bool { return true } }})
	t.Cleanup(h.CloseAll)
	return h, func() (net.Conn, error) {
		a, b := net.Pipe()
		go h.ServeConn(b)
		return a, nil
	}
}

// What is typed and the view's size before the host's terminal is joined
// reach it once it is, in order; a conn the host could not start ends.
func TestHostConn(t *testing.T) {
	_, dial := hostForTest(t)
	c, err := termhost.NewClient(dial)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id, err := c.Create(termhost.CreateRequest{Argv: []string{"/bin/cat"}})
	if err != nil {
		t.Fatal(err)
	}
	conn := newHostConn()
	conn.ResizePixels(50, 10, 8, 16)
	conn.Write([]byte("one "))
	conn.Write([]byte("two\r"))
	st, err := c.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	// The sizes the frames bring, as Read gives them, before the frame.
	var sizes [][2]int
	if !conn.bind(st, func(cols, rows int) { sizes = append(sizes, [2]int{cols, rows}) }) {
		t.Fatal("bind: closed")
	}
	var out []byte
	buf := make([]byte, 4096)
	for strings.Count(string(out), "one two") < 2 { // the echo, and cat's
		n, err := conn.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			t.Fatalf("read %q: %v", out, err)
		}
	}
	if len(sizes) == 0 || sizes[len(sizes)-1] != [2]int{50, 10} {
		t.Fatalf("frames at %v, want the view's 50×10 last", sizes)
	}
	conn.Close()
	if conn.lost() {
		t.Fatal("closed, said lost")
	}

	failed := newHostConn()
	failed.fail()
	if n, err := failed.Read(make([]byte, 8)); n != 0 || err == nil {
		t.Fatalf("failed conn read %d, %v", n, err)
	}
	if _, err := failed.Write([]byte("x")); err == nil {
		t.Fatal("failed conn took input")
	}
}

// A terminal whose stream can't be opened is not left running unseen in
// the host, nor started twice: only starting it is tried again with a
// host started anew. (Regression: the whole of it was, with the same
// client, so a preset's command ran twice, once unseen.)
func TestTermHostOpen(t *testing.T) {
	_, dial := hostForTest(t)
	failing := false
	flaky := func() (net.Conn, error) {
		if failing {
			return nil, errors.New("refused")
		}
		return dial()
	}
	old := sysTermHost
	t.Cleanup(func() { sysTermHost = old })
	sysTermHost = func(string, bool) (*termhost.Client, error) { return termhost.NewClient(flaky) }
	h := &termHost{dir: t.TempDir(), run: func(_ string, fn func()) { fn() }}
	req := termhost.CreateRequest{Argv: []string{"/bin/sleep", "30"}, Dir: "/", Cols: 20, Rows: 5}

	st, created, err := h.open(req, "")
	if err != nil || created == "" {
		t.Fatalf("open: %v", err)
	}
	st.Close()
	if !h.has(created) || h.has("nope") {
		t.Fatal("has: the terminal started not there, or one that isn't")
	}

	h.c.Close() // a new client, whose stream will fail
	h.c = nil
	sysTermHost = func(string, bool) (*termhost.Client, error) {
		c, err := termhost.NewClient(flaky)
		failing = true // the client connected; the stream can't
		return c, err
	}
	if _, _, err := h.open(req, ""); err == nil {
		t.Fatal("opened with no stream")
	}
	failing = false
	c, err := termhost.NewClient(dial)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	list, _, err := c.List()
	if err != nil || len(list) != 1 || list[0].ID != created {
		t.Fatalf("the host's terminals: %+v %v, want only the first", list, err)
	}
}

// Requests to the host queue without waiting, however many come while the
// one before runs (a host that answers no more holds it up), in order; a
// request with a key replaces the one with that key still waiting, in
// the place of the last.
func TestRequestQueue(t *testing.T) {
	q := newRequestQueue()
	go q.loop()
	var ran []string
	hold, held := make(chan struct{}), make(chan struct{})
	q.add("", func() { close(held); <-hold })
	<-held
	for i := range 500 {
		n := strconv.Itoa(i)
		q.add("active", func() { ran = append(ran, "active "+n) })
		q.add("", func() { ran = append(ran, "close "+n) })
	}
	q.add("meta a", func() { ran = append(ran, "meta a") })
	done := make(chan struct{})
	q.add("", func() { close(done) })
	close(hold)
	<-done
	if len(ran) != 502 || ran[0] != "close 0" || ran[498] != "close 498" || ran[499] != "active 499" || ran[500] != "close 499" || ran[501] != "meta a" {
		t.Fatalf("%d ran: %v … %v", len(ran), ran[:min(3, len(ran))], ran[max(0, len(ran)-3):])
	}
}

// Only the Gocast installed (com.hugo.gocast, built for release) runs its
// terminals in the data folder; any other build, which shares that folder
// (go build, mygo dev, a debug build), gets one of its own, so that it
// never shows nor ends the installed Gocast's terminals.
func TestTermHostDir(t *testing.T) {
	dir := t.TempDir()
	if got := termHostDir(dir, "com.hugo.gocast", false); got != dir {
		t.Fatalf("installed: %s", got)
	}
	seen := map[string]string{}
	for _, c := range []struct {
		id  string
		dev bool
	}{{"com.hugo.gocast", true}, {"com.hugo.gocast.dev", true}, {"com.hugo.gocast.dev", false}, {"", true}, {"", false}, {"x/../..", false}} {
		got := termHostDir(dir, c.id, c.dev)
		if filepath.Dir(filepath.Dir(got)) != dir || filepath.Base(filepath.Dir(got)) != "dev-terminals" {
			t.Errorf("%+v: %s", c, got)
		}
		if fi, err := os.Stat(got); err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o700 {
			t.Errorf("%+v: folder %v %v", c, fi, err)
		}
		k := fmt.Sprint(c)
		if c.id == "" {
			k = "unbundled" // go build: dev or not, the same
		}
		for other, p := range seen {
			if p == got && other != k {
				t.Errorf("%s and %s share %s", other, k, got)
			}
		}
		seen[k] = got
	}
}
