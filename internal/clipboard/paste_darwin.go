//go:build darwin

package clipboard

import (
	"errors"
	"os/exec"
	"sync"

	"github.com/ebitengine/purego"
)

// AccessibilitySettingsURL opens System Settings › Privacy & Security ›
// Accessibility.
const AccessibilitySettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

var ax struct {
	once        sync.Once
	trusted     func() bool
	withOptions func(options uintptr) bool
	prompt      uintptr
}

func loadAX() bool {
	ax.once.Do(func() {
		load()
		if libs.appServices == 0 {
			return
		}
		purego.RegisterLibFunc(&ax.trusted, libs.appServices, "AXIsProcessTrusted")
		purego.RegisterLibFunc(&ax.withOptions, libs.appServices, "AXIsProcessTrustedWithOptions")
		ax.prompt = symbol(libs.appServices, "kAXTrustedCheckOptionPrompt")
	})
	return ax.trusted != nil
}

// AccessibilityTrusted reports whether the user has turned Gocast on in
// Accessibility, which pasting for them (posting ⌘V) needs.
func AccessibilityTrusted() bool {
	return loadAX() && ax.trusted()
}

// RequestAccessibility reports whether Gocast is trusted, and when it is
// not, has macOS ask the user to allow it (the system's own prompt,
// pointing to System Settings).
func RequestAccessibility() bool {
	if !loadAX() {
		return false
	}
	if ax.prompt == 0 || !loadCF() {
		return ax.trusted()
	}
	var refs cfRefs
	defer refs.release()
	return ax.withOptions(refs.dict(ax.prompt, cf.booleanTrue))
}

// OpenAccessibilitySettings opens System Settings at Privacy & Security ›
// Accessibility.
func OpenAccessibilitySettings() error {
	return exec.Command("/usr/bin/open", AccessibilitySettingsURL).Run()
}

var cg struct {
	once     sync.Once
	source   func(state int32) uintptr
	keyEvent func(source uintptr, key uint16, down bool) uintptr
	setFlags func(event uintptr, flags uint64)
	post     func(tap uint32, event uintptr)
}

const (
	cgSessionStateCombined  = 0 // kCGEventSourceStateCombinedSessionState
	cgAnnotatedSessionTap   = 2 // kCGAnnotatedSessionEventTap
	cgFlagCommand           = 0x00100000
	cgFlagLeftCommandDevice = 0x00000008 // NX_DEVICELCMDKEYMASK
	keyV                    = 9          // kVK_ANSI_V
)

var errNoCoreGraphics = errors.New("clipboard: CoreGraphics unavailable")

func loadCG() bool {
	cg.once.Do(func() {
		load()
		if libs.coreGraphics == 0 {
			return
		}
		lib := libs.coreGraphics
		purego.RegisterLibFunc(&cg.source, lib, "CGEventSourceCreate")
		purego.RegisterLibFunc(&cg.keyEvent, lib, "CGEventCreateKeyboardEvent")
		purego.RegisterLibFunc(&cg.setFlags, lib, "CGEventSetFlags")
		purego.RegisterLibFunc(&cg.post, lib, "CGEventPost")
	})
	return cg.post != nil
}

// PostPasteKeystroke presses ⌘V: key V (virtual key 9) down and up with
// the Command flag, to the frontmost app. It needs Accessibility
// permission; without it macOS drops the events.
func PostPasteKeystroke() error {
	if !loadCG() || !loadCF() {
		return errNoCoreGraphics
	}
	src := cg.source(cgSessionStateCombined)
	if src != 0 {
		defer cf.release(src)
	}
	for _, down := range []bool{true, false} {
		ev := cg.keyEvent(src, keyV, down)
		if ev == 0 {
			return errors.New("clipboard: cannot make the ⌘V event")
		}
		cg.setFlags(ev, cgFlagCommand|cgFlagLeftCommandDevice)
		cg.post(cgAnnotatedSessionTap, ev)
		cf.release(ev)
	}
	return nil
}
