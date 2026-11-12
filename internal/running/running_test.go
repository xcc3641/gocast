//go:build darwin

package running

import "testing"

func TestFinder(t *testing.T) {
	if !IsRunning("com.apple.finder") {
		t.Skip("Finder is not running (no session?)")
	}
	if IsRunning("dev.gocast.no-such-app") {
		t.Fatal("an app that does not exist runs")
	}
}
