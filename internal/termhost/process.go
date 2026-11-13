package termhost

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

// Flag is the argument that starts the executable as the host, before the
// data folder: `Gocast --terminal-host <dir>`.
const Flag = "--terminal-host"

// The host's files in the data folder: its lock, its process ID (which
// install.sh --uninstall stops) and its log.
const (
	lockName = "terminal-host.lock"
	PIDName  = "terminal-host.pid"
	logName  = "terminal-host.log"
	sockName = "terminal.sock"
)

// maxSocketPath is under the 104 bytes of a Unix socket's path on macOS.
const maxSocketPath = 100

// SocketPath is where the host of the data folder dir listens: in the
// folder (0700), or, when that path would be too long for a socket, in
// the user's temporary folder (also 0700), named after the user and the
// folder.
func SocketPath(dir string) string {
	p := filepath.Join(dir, sockName)
	if len(p) < maxSocketPath {
		return p
	}
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(os.TempDir(), fmt.Sprintf("gocast-term-%d-%x.sock", os.Getuid(), sum[:4]))
}

// Dial connects to the host of the data folder dir, if one runs.
func Dial(dir string) (*Client, error) {
	sock := SocketPath(dir)
	return NewClient(func() (net.Conn, error) { return net.DialTimeout("unix", sock, time.Second) })
}

// Ensure connects to the host of the data folder dir, starting it, as
// exe, when none runs.
func Ensure(dir, exe string) (*Client, error) {
	if c, err := Dial(dir); err == nil {
		return c, nil
	}
	var err error
	for range 2 {
		var exited <-chan struct{}
		if exited, err = spawn(dir, exe); err != nil {
			return nil, err
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var c *Client
			if c, err = Dial(dir); err == nil {
				return c, nil
			}
			select {
			case <-exited: // another host took the lock and is going, or it failed
				deadline = time.Now()
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	return nil, fmt.Errorf("termhost: no host: %w", err)
}

// spawn starts the host, in a session of its own, so that it outlives
// Gocast: no terminal, no MYGO_AUTOMATION (the automation plugin would
// listen there too), its output in its log.
func spawn(dir, exe string) (exited <-chan struct{}, err error) {
	logf, err := os.OpenFile(filepath.Join(dir, logName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	defer logf.Close()
	cmd := exec.Command(exe, Flag, dir)
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = logf, logf
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MYGO_AUTOMATION=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait() // no zombie while Gocast runs
		close(done)
	}()
	return done, nil
}

// Main runs the host of the data folder dir, for build, until its last
// terminal ended (or the app was uninstalled): the executable's whole life
// when started with Flag. A host already running keeps the folder.
func Main(dir, build string) {
	log.SetPrefix("terminal host: ")
	log.SetFlags(log.LstdFlags)
	lock, err := os.OpenFile(filepath.Join(dir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	heldLock = lock // kept open, as long as the process runs
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		log.Print("another host runs")
		os.Exit(0)
	}
	if err := terminal.Load(); err != nil {
		log.Fatal(err)
	}
	sock := SocketPath(dir)
	_ = os.Remove(sock) // a host that died left it: the lock is ours
	old := syscall.Umask(0o177)
	l, err := net.Listen("unix", sock)
	syscall.Umask(old)
	if err != nil {
		log.Fatal(err)
	}
	_ = os.Chmod(sock, 0o600)
	pidPath := filepath.Join(dir, PIDName)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), pidFileMode)
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	h := New(Options{Build: build})
	go h.Serve(l)
	if exe, err := os.Executable(); err == nil && bundleOf(exe) != "" {
		go watchBundle(h, bundleOf(exe))
	}
	log.Printf("pid %d, %s", os.Getpid(), sock)
	select {
	case <-h.Done():
	case s := <-sigs:
		log.Printf("%v: ending the terminals", s)
		h.CloseAll()
	}
	l.Close() // removes the socket
	_ = os.Remove(pidPath)
	log.Print("exiting")
	os.Exit(0) // the lock goes with the process
}

// heldLock is the host's lock file, kept from the garbage collector,
// which would close it.
var heldLock *os.File

// bundleOf is the app bundle the executable exe is in, or "" for one
// built by go build.
func bundleOf(exe string) string {
	i := strings.Index(exe, ".app/Contents/MacOS/")
	if i < 0 {
		return ""
	}
	return exe[:i+len(".app")]
}

// watchBundle ends the terminals once the app is gone: dragged to the
// Trash or removed, as an update never leaves it for long.
func watchBundle(h *Host, bundle string) {
	var g goneCheck
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	for {
		select {
		case <-h.Done():
			return
		case <-time.After(goneEvery):
		}
		_, err := os.Stat(plist)
		if g.step(time.Now(), err == nil, h.Clients()) {
			log.Printf("%s is gone: ending the terminals", bundle)
			h.CloseAll()
			h.exit()
			return
		}
	}
}

// goneCheck tells, from looks at the app's Info.plist, when the app was
// uninstalled: missing at two looks in a row at least goneFor apart, with
// no Gocast connected. An update swaps the bundle in a moment, and a
// Gocast running is there, whatever its bundle.
type goneCheck struct {
	since  time.Time // the first look that missed it
	misses int
}

func (g *goneCheck) step(now time.Time, present bool, clients int) bool {
	if present {
		*g = goneCheck{}
		return false
	}
	if g.misses == 0 {
		g.since = now
	}
	g.misses++
	return g.misses >= 2 && now.Sub(g.since) >= goneFor && clients == 0
}
