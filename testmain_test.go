package main

import (
	"os"
	"strings"
	"testing"

	"gocast/internal/running"
)

// fakeAppPaths are the apps the tests have installed, by bundle ID, as
// Launch Services would find them (spelled any way): those of the
// tests' search panels. Any other is not installed.
var fakeAppPaths = map[string]string{
	"com.apple.terminal": "/System/Applications/Utilities/Terminal.app",
	"com.apple.safari":   "/Applications/Safari.app",
	"com.apple.finder":   "/System/Library/CoreServices/Finder.app",
	"com.apple.notes":    "/System/Applications/Notes.app",
}

// TestMain stands in for the running apps: no test asks the system of
// them or acts on them (a CI runner with no login session crashes, and
// a test must not open, hide or quit the user's apps). None runs, none
// is in front.
func TestMain(m *testing.M) {
	sysIsRunning = func(string) bool { return false }
	sysAppPath = func(id string) string { return fakeAppPaths[strings.ToLower(id)] }
	sysFrontmost = func() (string, bool) { return "", false }
	sysHide = func(string) bool { return false }
	sysOpen = func(id string) error {
		if sysAppPath(id) == "" {
			return running.ErrNotInstalled
		}
		return nil
	}
	sysQuit = func(string) bool { return false }
	os.Exit(m.Run())
}
