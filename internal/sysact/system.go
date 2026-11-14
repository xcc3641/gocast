package sysact

import (
	"errors"
	"fmt"
	"math"
)

// System is what the system actions ask of macOS. The app holds the
// real one (NewSystem); its tests hold a fake, so that no test locks,
// sleeps or restarts the Mac, or changes its volume or appearance.
type System interface {
	LockScreen() error
	Sleep() error
	SleepDisplays() error
	StartScreenSaver() error
	// Restart, ShutDown and LogOut ask the system politely, as the Apple
	// menu does without its own dialog (Gocast asked first): apps may
	// still ask to save.
	Restart() error
	ShutDown() error
	LogOut() error

	// Volume is the default output's volume, 0 to 1, and whether it is
	// muted.
	Volume() (level float64, muted bool, err error)
	// SetVolume sets the volume, 0 to 1.
	SetVolume(level float64) error
	SetMuted(muted bool) error

	// ToggleAppearance switches between light and dark; dark is the one
	// now.
	ToggleAppearance() (dark bool, err error)
	ShowDesktop() error
	// HideOthers hides the apps but the one of the process pid (0: but
	// the frontmost) and Gocast.
	HideOthers(pid int) error
	UnhideAll() error
	// QuitAll asks the apps but Gocast and Finder to quit, as Command-Q:
	// those with unsaved documents ask.
	QuitAll() error

	OpenTrash() error
	// TrashEmpty reports whether the Trash is empty.
	TrashEmpty() (bool, error)
	EmptyTrash() error
	// EjectAll ejects the disks that can be: how many it ejected, and the
	// names of those in use it could not.
	EjectAll() (ejected int, busy []string, err error)
	// ToggleHiddenFiles shows Finder's hidden files, or hides them again;
	// shown is how they are now.
	ToggleHiddenFiles() (shown bool, err error)
}

// AutomationError is the system refusing Gocast control of App (Finder,
// System Events): the user said no in Privacy & Security › Automation
// (error -1743).
type AutomationError struct{ App string }

func (e *AutomationError) Error() string {
	return fmt.Sprintf("not allowed to control %s", e.App)
}

// AutomationSettingsURL opens System Settings › Privacy & Security ›
// Automation.
const AutomationSettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation"

// AccessibilitySettingsURL opens System Settings › Privacy & Security ›
// Accessibility.
const AccessibilitySettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

// ErrUnsupported: not on this system.
var ErrUnsupported = errors.New("sysact: not supported on this system")

// The volume's steps, as the volume keys': sixteen.
const volumeSteps = 16

// StepVolume is the volume one step up (up) or down from level, 0 to 1,
// on the sixteen steps of the volume keys: the next above or below,
// from between two as well.
func StepVolume(level float64, up bool) float64 {
	const eps = 1e-3
	n := level * volumeSteps
	if up {
		n = math.Floor(n+eps) + 1
	} else {
		n = math.Ceil(n-eps) - 1
	}
	return min(max(n, 0), volumeSteps) / volumeSteps
}

// Percent is level, 0 to 1, in whole percent, as the notice says it.
func Percent(level float64) int { return int(math.Round(level * 100)) }
