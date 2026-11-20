package keepawake

import "errors"

// AssertionType is the kind of power assertion that keeps the Mac awake
// (IOPMLib.h's kIOPMAssertionType… names).
type AssertionType string

const (
	// PreventDisplaySleep keeps the screen on, and with it the Mac: the
	// default.
	PreventDisplaySleep AssertionType = "PreventUserIdleDisplaySleep"
	// PreventSystemSleep lets the screen turn off but keeps the Mac
	// awake: with "Let the screen turn off while keeping awake" on
	// (copy.md 18.4).
	PreventSystemSleep AssertionType = "PreventUserIdleSystemSleep"
)

// typeFor is the assertion to hold when the screen may turn off or not.
func typeFor(allowDisplaySleep bool) AssertionType {
	if allowDisplaySleep {
		return PreventSystemSleep
	}
	return PreventDisplaySleep
}

// AssertionName is the name `pmset -g assertions` shows Gocast's
// assertion by.
const AssertionName = "Gocast: Keep Awake"

// Assertion is a power assertion held, an IOPMAssertionID.
type Assertion uint32

// Backend creates and releases power assertions: the system's (System)
// or, in the tests, a fake. The system drops a process's assertions when
// it exits, so nothing outlives Gocast.
type Backend interface {
	Create(t AssertionType, name string) (Assertion, error)
	Release(a Assertion) error
}

// ErrUnsupported is the error of System's Create where there are no
// power assertions: anywhere but macOS.
var ErrUnsupported = errors.New("keepawake: not supported on this system")
