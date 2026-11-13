//go:build darwin

package running

import "testing"

func TestFinder(t *testing.T) {
	if !IsRunning("com.apple.finder") {
		t.Skip("Finder is not running (no session?)")
	}
	if IsRunning("com.hugo.gocast.no-such-app") {
		t.Fatal("an app that does not exist runs")
	}
}

func TestAppPath(t *testing.T) {
	if got := AppPath("com.apple.finder"); got != "/System/Library/CoreServices/Finder.app" {
		t.Errorf("Finder is at %q", got)
	}
	if got := AppPath("com.hugo.gocast.no-such-app"); got != "" {
		t.Errorf("an app that does not exist is at %q", got)
	}
}

// Frontmost only reads: the test opens and hides no app.
func TestFrontmost(t *testing.T) {
	id, _ := Frontmost()
	if id == "" {
		t.Skip("no frontmost app (no session?)")
	}
	if !IsRunning(id) {
		t.Errorf("the frontmost app %q does not run", id)
	}
}

func TestOpenNotInstalled(t *testing.T) {
	if err := Open("com.hugo.gocast.no-such-app"); err != ErrNotInstalled {
		t.Errorf("Open of an app that does not exist = %v", err)
	}
}
