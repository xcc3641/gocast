//go:build !darwin

package clipboard

import (
	"errors"
	"time"
)

var errUnsupported = errors.New("clipboard: only on macOS")

// KeychainService is the Keychain service of the history's key.
const KeychainService = "dev.gocast.app.clipboard"

// AccessibilitySettingsURL opens the Accessibility settings (macOS only).
const AccessibilitySettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

// Keychain is unavailable outside macOS: Load and Save fail.
type Keychain struct {
	Service, Account string
}

// NewKeychain returns a key store that always fails outside macOS.
func NewKeychain() *Keychain { return &Keychain{Service: KeychainService} }

// Load fails outside macOS.
func (*Keychain) Load() ([]byte, error) { return nil, errUnsupported }

// Save fails outside macOS.
func (*Keychain) Save([]byte) error { return errUnsupported }

// Delete fails outside macOS.
func (*Keychain) Delete() error { return errUnsupported }

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
