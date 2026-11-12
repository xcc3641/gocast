//go:build !darwin

// Package running tells whether an app is running and asks it to quit
// (macOS only).
package running

// IsRunning reports false outside macOS.
func IsRunning(string) bool { return false }

// Quit does nothing outside macOS.
func Quit(string) bool { return false }

// AppPath returns "" outside macOS.
func AppPath(string) string { return "" }
