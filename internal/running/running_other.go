//go:build !darwin

// Package running tells whether an app is running, asks it to quit, hides
// it and opens it (macOS only).
package running

import "errors"

// ErrNotInstalled is Open's error for an app that can't be found.
var ErrNotInstalled = errors.New("app not installed")

// IsRunning reports false outside macOS.
func IsRunning(string) bool { return false }

// Quit does nothing outside macOS.
func Quit(string) bool { return false }

// AppPath returns "" outside macOS.
func AppPath(string) string { return "" }

// Frontmost returns "" outside macOS.
func Frontmost() (string, bool) { return "", false }

// Hide does nothing outside macOS.
func Hide(string) bool { return false }

// Open opens nothing outside macOS.
func Open(string) error { return ErrNotInstalled }
