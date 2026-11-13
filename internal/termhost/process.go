package termhost

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
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
// folder (0700), or, when that path would be too long for a socket, in a
// folder of the user's own (0700) in the temporary folder, named after
// the data folder. That folder is made, and refused when it is not the
// user's alone.
func SocketPath(dir string) (string, error) {
	return socketPathIn(dir, os.TempDir())
}

func socketPathIn(dir, tmp string) (string, error) {
	p := filepath.Join(dir, sockName)
	if len(p) < maxSocketPath {
		return p, nil
	}
	own := filepath.Join(tmp, fmt.Sprintf("gocast-%d", os.Getuid()))
	if err := os.Mkdir(own, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if err := ownedAlone(own); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(own, fmt.Sprintf("t-%x.sock", sum[:4])), nil
}

// ownedAlone checks that the folder is the user's, a folder and not a
// link, that nobody else can write in: where a socket of another user's
// could be put.
func ownedAlone(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || !ok || int(st.Uid) != os.Getuid() || fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("termhost: %s is not the user's alone", dir)
	}
	return nil
}

// ownSocket checks that the socket at path is one of the user's.
func ownSocket(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if fi.Mode().Type() != os.ModeSocket || !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("termhost: %s is not the user's socket", path)
	}
	return nil
}

// Dial connects to the host of the data folder dir, if one runs.
func Dial(dir string) (*Client, error) {
	sock, err := SocketPath(dir)
	if err != nil {
		return nil, err
	}
	return NewClient(func() (net.Conn, error) {
		if err := ownSocket(sock); err != nil {
			return nil, err
		}
		return net.DialTimeout("unix", sock, time.Second)
	})
}

// Ensure connects to the host of the data folder dir, starting it, as
// exe, when none runs. A host still ending its terminals (after Close
// All) answers no more: the one started waits for it to exit.
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
		deadline := time.Now().Add(lockWait + time.Second)
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
	// Appended to: a host still ending writes its last lines there too.
	// Cut short once it grew past a megabyte.
	name := filepath.Join(dir, logName)
	if fi, err := os.Stat(name); err == nil && fi.Size() > 1<<20 {
		_ = os.Truncate(name, 0)
	}
	logf, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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

// lockWait is how long a host started waits for the one before it,
// which answers no more, to exit: it ends its terminals first.
const lockWait = closeGrace + 5*time.Second

// Main runs the host of the data folder dir, for build, until its last
// terminal ended (or the app was uninstalled): the executable's whole life
// when started with Flag. A host already running keeps the folder.
func Main(dir, build string) {
	// Before anything opens a file: what Gocast left open without
	// close-on-exec would go on to every program.
	closeInherited(3, maxFD())
	log.SetPrefix("terminal host: ")
	log.SetFlags(log.LstdFlags)
	sock, err := SocketPath(dir)
	if err != nil {
		log.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	heldLock = lock // kept open, as long as the process runs
	if !takeLock(lock, dir) {
		log.Print("another host runs")
		os.Exit(0)
	}
	if err := terminal.Load(); err != nil {
		log.Fatal(err)
	}
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
	stopSignals := hostSignals()
	defer stopSignals()
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

// takeLock takes the host's lock: at once, or once the host holding it
// exited, which a host that answers no more does in a few seconds (it
// ends its terminals). It reports false when another host answers, or
// still holds it after lockWait.
func takeLock(lock *os.File, dir string) bool {
	deadline := time.Now().Add(lockWait)
	for {
		if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		if c, err := Dial(dir); err == nil {
			c.Close()
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// hostSignals has the host take the hangup and broken pipes itself, and
// returns what gives them back. Taken, not ignored: a signal ignored is
// ignored by every program the host starts too, as exec keeps it, and a
// hangup would then end none of them, nor a broken pipe a pipeline's
// producer. Taken by the host, they are back to their defaults in its
// programs. (The host has no terminal, and its output is its log.)
func hostSignals() (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGPIPE)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

// maxFD is the highest number a descriptor may have, plus one, as far as
// closeInherited looks.
func maxFD() int {
	var lim syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim) != nil || lim.Cur > 1<<16 {
		return 1 << 16
	}
	return int(lim.Cur)
}

// closeInherited closes the descriptors from to before that lack
// close-on-exec: those the process got from the one that started it
// (Go opens its own with close-on-exec, and the runtime's too).
func closeInherited(from, before int) {
	for fd := from; fd < before; fd++ {
		flags, err := fcntl(fd, syscall.F_GETFD, 0)
		if err == nil && flags&syscall.FD_CLOEXEC == 0 {
			syscall.Close(fd)
		}
	}
}

// bundleOf is the app bundle the executable exe is in, or "" for one
// built by go build.
func bundleOf(exe string) string {
	i := strings.Index(exe, ".app/Contents/MacOS/")
	if i < 0 {
		return ""
	}
	return exe[:i+len(".app")]
}

// BundleID is the identifier of the app bundle the executable exe is in,
// from its Info.plist; "" for one built by go build, or when it can't be
// read.
func BundleID(exe string) string {
	if b := bundleOf(exe); b != "" {
		return bundleIDOf(b)
	}
	return ""
}

var plistID = regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]*)</string>`)

func bundleIDOf(bundle string) string {
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	b, err := os.ReadFile(plist)
	if err != nil {
		return ""
	}
	if m := plistID.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	// A binary property list.
	out, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", plist).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// unsettled tells a bundle that may go away without being uninstalled:
// run from a disk image, or translocated (opened from Downloads before
// being moved), as Gatekeeper does.
func unsettled(bundle string) bool {
	return strings.HasPrefix(bundle, "/Volumes/") || strings.Contains(bundle, "/AppTranslocation/")
}

// findBundles lists the bundles with the identifier id that Spotlight
// knows of. A fake in tests.
var findBundles = func(id string) ([]string, error) {
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("termhost: identifier %q", id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/mdfind", "kMDItemCFBundleIdentifier == '"+id+"'").Output()
	if err != nil {
		return nil, err
	}
	var found []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			found = append(found, line)
		}
	}
	return found, nil
}

var validID = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

// watchBundle ends the terminals once the app is gone: dragged to the
// Trash or removed, as an update never leaves it for long. A bundle in a
// disk image or translocated isn't watched.
func watchBundle(h *Host, bundle string) {
	if unsettled(bundle) {
		log.Printf("%s may go away: not watched", bundle)
		return
	}
	w := bundleWatch{bundle: bundle, id: bundleIDOf(bundle), exists: exists, find: findBundles}
	for {
		select {
		case <-h.Done():
			return
		case <-time.After(goneEvery):
		}
		if w.step(time.Now(), h.Clients()) {
			log.Printf("%s is gone: ending the terminals", w.bundle)
			h.CloseAll()
			return
		}
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// bundleWatch tells when the app was uninstalled: its Info.plist missing
// at two looks in a row at least goneFor apart, with no Gocast connected,
// and no bundle of its identifier anywhere else (moved, it follows it).
// An update swaps the bundle in a moment, and a Gocast running is there,
// whatever its bundle. Not knowing (Spotlight failing) is not gone.
type bundleWatch struct {
	bundle string
	id     string
	exists func(path string) bool
	find   func(id string) ([]string, error)
	g      goneCheck
}

func (w *bundleWatch) step(now time.Time, clients int) bool {
	present := w.exists(filepath.Join(w.bundle, "Contents", "Info.plist"))
	if !present && w.id != "" {
		switch found, err := w.find(w.id); {
		case err != nil:
			log.Printf("looking for %s: %v", w.id, err)
			present = true
		default:
			for _, b := range found {
				if strings.Contains(b, "/.Trash/") || unsettled(b) || !w.exists(filepath.Join(b, "Contents", "Info.plist")) {
					continue
				}
				log.Printf("%s moved to %s", w.bundle, b)
				w.bundle, present = b, true
				break
			}
		}
	}
	return w.g.step(now, present, clients)
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

func fcntl(fd, cmd, arg int) (int, error) {
	r, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	if errno != 0 {
		return 0, errno
	}
	return int(r), nil
}
