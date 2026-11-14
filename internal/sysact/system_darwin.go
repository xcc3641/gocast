//go:build darwin

package sysact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// macSystem is the real System: NewSystem.
type macSystem struct {
	// self are the bundle IDs of Gocast's builds, which Hide Others and
	// Quit All leave alone, with its own process.
	self []string
}

// NewSystem returns macOS's system actions. self are the bundle IDs the
// actions on all apps leave alone, as they do Gocast's own process.
func NewSystem(self ...string) System { return macSystem{self: self} }

// The commands the actions run, by their paths: no PATH lookups.
const (
	pmset         = "/usr/bin/pmset"
	openCmd       = "/usr/bin/open"
	osascript     = "/usr/bin/osascript"
	defaultsCmd   = "/usr/bin/defaults"
	missionCtl    = "/System/Applications/Mission Control.app/Contents/MacOS/Mission Control"
	finderID      = "com.apple.finder"
	systemEvents  = "System Events"
	finder        = "Finder"
	scriptTimeout = 30 * time.Second
)

// run runs a command, its error with what it said.
func run(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", filepath.Base(name), err, bytes.TrimSpace(out))
	}
	return nil
}

// script runs AppleScript that controls app, returning what it returns;
// an *AutomationError when the user did not allow Gocast to control it.
func script(app, src string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, osascript, "-e", src)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if strings.Contains(msg, "-1743") || strings.Contains(msg, "-10004") && strings.Contains(msg, "privilege") {
			return "", &AutomationError{App: app}
		}
		return "", fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(msg))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// --- The screen and power ---

const (
	keyQ          = 12 // kVK_ANSI_Q
	flagControl   = 0x00040000
	flagCommand   = 0x00100000
	hidEventTap   = 0 // kCGHIDEventTap
	sourceHIDOnly = 1 // kCGEventSourceStateHIDSystemState
)

// LockScreen presses ⌃⌘Q, the system's Lock Screen, which needs the
// Accessibility permission the window actions have; without it, it turns
// the displays off, which locks the Mac as it asks for the password at
// once (the default).
func (macSystem) LockScreen() error {
	if !load() || !lib.trusted() {
		return run(pmset, "displaysleepnow")
	}
	src := lib.evSource(sourceHIDOnly)
	if src != 0 {
		defer lib.cfRelease(src)
	}
	for _, down := range []bool{true, false} {
		ev := lib.keyEvent(src, keyQ, down)
		if ev == 0 {
			return errors.New("sysact: cannot make the ⌃⌘Q event")
		}
		lib.setFlags(ev, flagControl|flagCommand)
		lib.post(hidEventTap, ev)
		lib.cfRelease(ev)
	}
	return nil
}

func (macSystem) Sleep() error         { return run(pmset, "sleepnow") }
func (macSystem) SleepDisplays() error { return run(pmset, "displaysleepnow") }

func (macSystem) StartScreenSaver() error {
	return run(openCmd, "-a", "ScreenSaverEngine")
}

func (macSystem) Restart() error {
	_, err := script(systemEvents, `tell application "System Events" to restart`)
	return err
}

func (macSystem) ShutDown() error {
	_, err := script(systemEvents, `tell application "System Events" to shut down`)
	return err
}

func (macSystem) LogOut() error {
	_, err := script(systemEvents, `tell application "System Events" to log out`)
	return err
}

// --- The volume, through Core Audio: no permission ---

var audio struct {
	once        sync.Once
	ok          bool
	getProperty func(object uint32, address unsafe.Pointer, qualSize uint32, qual uintptr, size *uint32, data unsafe.Pointer) int32
	setProperty func(object uint32, address unsafe.Pointer, qualSize uint32, qual uintptr, size uint32, data unsafe.Pointer) int32
	// The virtual main volume is AudioToolbox's.
	serviceGet func(object uint32, address unsafe.Pointer, qualSize uint32, qual uintptr, size *uint32, data unsafe.Pointer) int32
	serviceSet func(object uint32, address unsafe.Pointer, qualSize uint32, qual uintptr, size uint32, data unsafe.Pointer) int32
}

func loadAudio() bool {
	if !load() || lib.coreAudio == 0 {
		return false
	}
	audio.once.Do(func() {
		toolbox, _ := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if toolbox == 0 {
			return
		}
		purego.RegisterLibFunc(&audio.getProperty, lib.coreAudio, "AudioObjectGetPropertyData")
		purego.RegisterLibFunc(&audio.setProperty, lib.coreAudio, "AudioObjectSetPropertyData")
		purego.RegisterLibFunc(&audio.serviceGet, toolbox, "AudioHardwareServiceGetPropertyData")
		purego.RegisterLibFunc(&audio.serviceSet, toolbox, "AudioHardwareServiceSetPropertyData")
		audio.ok = true
	})
	return audio.ok
}

type audioAddress struct{ selector, scope, element uint32 }

func fourCC(s string) uint32 {
	return uint32(s[0])<<24 | uint32(s[1])<<16 | uint32(s[2])<<8 | uint32(s[3])
}

var (
	addrDefaultOutput = audioAddress{fourCC("dOut"), fourCC("glob"), 0}
	addrMainVolume    = audioAddress{fourCC("vmvc"), fourCC("outp"), 0}
	addrMute          = audioAddress{fourCC("mute"), fourCC("outp"), 0}
)

const systemObject = 1 // kAudioObjectSystemObject

// output is the default output device.
func output() (uint32, error) {
	if !loadAudio() {
		return 0, ErrUnsupported
	}
	var dev uint32
	size := uint32(4)
	a := addrDefaultOutput
	if st := audio.getProperty(systemObject, unsafe.Pointer(&a), 0, 0, &size, unsafe.Pointer(&dev)); st != 0 || dev == 0 {
		return 0, fmt.Errorf("sysact: no default output (%d)", st)
	}
	return dev, nil
}

func (macSystem) Volume() (float64, bool, error) {
	dev, err := output()
	if err != nil {
		return 0, false, err
	}
	var v float32
	size := uint32(4)
	a := addrMainVolume
	if st := audio.serviceGet(dev, unsafe.Pointer(&a), 0, 0, &size, unsafe.Pointer(&v)); st != 0 {
		return 0, false, fmt.Errorf("sysact: reading the volume (%d)", st)
	}
	var m uint32
	size = 4
	a = addrMute
	audio.getProperty(dev, unsafe.Pointer(&a), 0, 0, &size, unsafe.Pointer(&m)) // a device may have no mute
	return math.Round(float64(v)*1000) / 1000, m != 0, nil
}

func (macSystem) SetVolume(level float64) error {
	dev, err := output()
	if err != nil {
		return err
	}
	v := float32(min(max(level, 0), 1))
	a := addrMainVolume
	if st := audio.serviceSet(dev, unsafe.Pointer(&a), 0, 0, 4, unsafe.Pointer(&v)); st != 0 {
		return fmt.Errorf("sysact: setting the volume (%d)", st)
	}
	return nil
}

func (macSystem) SetMuted(muted bool) error {
	dev, err := output()
	if err != nil {
		return err
	}
	var m uint32
	if muted {
		m = 1
	}
	a := addrMute
	if st := audio.setProperty(dev, unsafe.Pointer(&a), 0, 0, 4, unsafe.Pointer(&m)); st != 0 {
		return fmt.Errorf("sysact: muting (%d)", st)
	}
	return nil
}

// --- Appearance and the desktop ---

func (macSystem) ToggleAppearance() (bool, error) {
	out, err := script(systemEvents, `tell application "System Events" to tell appearance preferences
	set dark mode to not dark mode
	return dark mode
end tell`)
	return out == "true", err
}

// ShowDesktop asks Mission Control to show the desktop, as F11 does; again
// brings the windows back.
func (macSystem) ShowDesktop() error {
	cmd := exec.Command(missionCtl, "1")
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// --- Apps, through NSWorkspace and NSRunningApplication: no permission ---

var (
	selPool   = objc.RegisterName("new")
	selDrain  = objc.RegisterName("drain")
	selShared = objc.RegisterName("sharedWorkspace")
	selApps   = objc.RegisterName("runningApplications")
	selCount  = objc.RegisterName("count")
	selAt     = objc.RegisterName("objectAtIndex:")
	selPolicy = objc.RegisterName("activationPolicy")
	selPID    = objc.RegisterName("processIdentifier")
	selBundle = objc.RegisterName("bundleIdentifier")
	selUTF8   = objc.RegisterName("UTF8String")
	selHide   = objc.RegisterName("hide")
	selUnhide = objc.RegisterName("unhide")
	selHidden = objc.RegisterName("isHidden")
	selTerm   = objc.RegisterName("terminate")
	selTermed = objc.RegisterName("isTerminated")
	selFront  = objc.RegisterName("frontmostApplication")
	// appKitOnce loads AppKit, which MyGo has in the app, once.
	appKitOnce = sync.OnceValue(func() bool {
		_, _ = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		return objc.GetClass("NSWorkspace") != 0
	})
)

// runningApp is an app running, as NSRunningApplication has it.
type runningApp struct {
	id     objc.ID
	pid    int
	bundle string
}

// eachApp calls fn with each regular app running (those in the Dock) but
// Gocast.
func (s macSystem) eachApp(fn func(a runningApp)) error {
	if !appKitOnce() {
		return ErrUnsupported
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selPool)
	defer pool.Send(selDrain)
	ws := objc.ID(objc.GetClass("NSWorkspace")).Send(selShared)
	apps := ws.Send(selApps)
	n := objc.Send[uint](apps, selCount)
	self := os.Getpid()
	for i := uint(0); i < n; i++ {
		app := apps.Send(selAt, i)
		if objc.Send[int](app, selPolicy) != 0 || objc.Send[bool](app, selTermed) { // NSApplicationActivationPolicyRegular
			continue
		}
		a := runningApp{id: app, pid: int(objc.Send[int32](app, selPID)), bundle: goString(app.Send(selBundle))}
		if a.pid == self || s.isSelf(a.bundle) {
			continue
		}
		fn(a)
	}
	return nil
}

func (s macSystem) isSelf(bundle string) bool {
	for _, id := range s.self {
		if bundle == id {
			return true
		}
	}
	return false
}

func goString(str objc.ID) string {
	if str == 0 {
		return ""
	}
	p := objc.Send[*byte](str, selUTF8)
	if p == nil {
		return ""
	}
	var b []byte
	for q := unsafe.Pointer(p); *(*byte)(q) != 0; q = unsafe.Add(q, 1) {
		b = append(b, *(*byte)(q))
	}
	return string(b)
}

func (s macSystem) HideOthers(pid int) error {
	if pid <= 0 && appKitOnce() {
		ws := objc.ID(objc.GetClass("NSWorkspace")).Send(selShared)
		if front := ws.Send(selFront); front != 0 {
			pid = int(objc.Send[int32](front, selPID))
		}
	}
	return s.eachApp(func(a runningApp) {
		if a.pid != pid {
			a.id.Send(selHide)
		}
	})
}

func (s macSystem) UnhideAll() error {
	return s.eachApp(func(a runningApp) {
		if objc.Send[bool](a.id, selHidden) {
			a.id.Send(selUnhide)
		}
	})
}

func (s macSystem) QuitAll() error {
	return s.eachApp(func(a runningApp) {
		if a.bundle != finderID {
			a.id.Send(selTerm) // as Command-Q: an app with unsaved documents asks
		}
	})
}

// --- The Trash and the disks, through Finder (Automation) ---

func (macSystem) OpenTrash() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return run(openCmd, filepath.Join(home, ".Trash"))
}

func (macSystem) TrashEmpty() (bool, error) {
	out, err := script(finder, `tell application "Finder" to return count of items of trash`)
	if err != nil {
		return false, err
	}
	n, err := strconv.Atoi(out)
	return n == 0, err
}

func (macSystem) EmptyTrash() error {
	_, err := script(finder, `tell application "Finder" to empty trash`)
	return err
}

// ejectScript ejects each disk Finder can, returning how many it ejected
// on the first line, and the names of those it could not after.
const ejectScript = `set done to 0
set busy to ""
tell application "Finder"
	repeat with d in (every disk whose ejectable is true)
		set n to name of d
		try
			eject d
			set done to done + 1
		on error
			set busy to busy & linefeed & n
		end try
	end repeat
end tell
return (done as text) & busy`

func (macSystem) EjectAll() (int, []string, error) {
	out, err := script(finder, ejectScript)
	if err != nil {
		return 0, nil, err
	}
	lines := strings.Split(out, "\n")
	n, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, nil, fmt.Errorf("sysact: eject said %q", out)
	}
	var busy []string
	for _, l := range lines[1:] {
		if l = strings.TrimSpace(l); l != "" {
			busy = append(busy, l)
		}
	}
	return n, busy, nil
}

// ToggleHiddenFiles flips Finder's AppleShowAllFiles and has Finder quit
// and open again (in the background) to read it.
func (s macSystem) ToggleHiddenFiles() (bool, error) {
	out, _ := exec.Command(defaultsCmd, "read", finderID, "AppleShowAllFiles").Output() // unset: hidden
	v := strings.ToLower(strings.TrimSpace(string(out)))
	shown := !(v == "1" || v == "yes" || v == "true")
	if err := run(defaultsCmd, "write", finderID, "AppleShowAllFiles", "-bool", strconv.FormatBool(shown)); err != nil {
		return !shown, err
	}
	if _, err := script(finder, `tell application "Finder" to quit`); err != nil {
		return shown, err
	}
	// Open again once it quit, behind the app in front.
	for range 30 {
		if !s.finderRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return shown, run(openCmd, "-g", "-b", finderID)
}

func (s macSystem) finderRunning() bool {
	if !appKitOnce() {
		return false
	}
	found := false
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selPool)
	defer pool.Send(selDrain)
	ws := objc.ID(objc.GetClass("NSWorkspace")).Send(selShared)
	apps := ws.Send(selApps)
	n := objc.Send[uint](apps, selCount)
	for i := uint(0); i < n; i++ {
		app := apps.Send(selAt, i)
		if goString(app.Send(selBundle)) == finderID && !objc.Send[bool](app, selTermed) {
			found = true
		}
	}
	return found
}
