//go:build !darwin

package clipboard

import (
	"errors"
	"time"
)

var errUnsupported = errors.New("clipboard: only on macOS")

// AccessibilitySettingsURL opens the Accessibility settings (macOS only).
const AccessibilitySettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

// SystemPasteboard is nil outside macOS.
func SystemPasteboard() Pasteboard { return nil }

// FrontmostApp is unknown outside macOS.
func FrontmostApp() App { return App{} }

// ActivateApp does nothing outside macOS.
func ActivateApp(int) bool { return false }

// ActivateBundle does nothing outside macOS.
func ActivateBundle(string) bool { return false }

// WaitFrontmost reports false outside macOS.
func WaitFrontmost(int, time.Duration) bool { return false }

// AccessibilityTrusted reports false outside macOS.
func AccessibilityTrusted() bool { return false }

// RequestAccessibility reports false outside macOS.
func RequestAccessibility() bool { return false }

// OpenAccessibilitySettings fails outside macOS.
func OpenAccessibilitySettings() error { return errUnsupported }

// PostPasteKeystroke fails outside macOS.
func PostPasteKeystroke() error { return errUnsupported }
