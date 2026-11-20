//go:build darwin

package keepawake

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The system's assertion shows in `pmset -g assertions` under Gocast's
// name and type, for this process, changes type, and is gone once released. It acts on the
// Mac (for a moment, it keeps it awake), so it runs only with
// GOCAST_REAL_ASSERTION=1:
//
//	GOCAST_REAL_ASSERTION=1 go test -run TestRealAssertion -v ./internal/keepawake
func TestRealAssertion(t *testing.T) {
	if os.Getenv("GOCAST_REAL_ASSERTION") != "1" {
		t.Skip("acts on the Mac: set GOCAST_REAL_ASSERTION=1")
	}
	k := New(Options{})
	defer k.Close()

	check := func(want AssertionType) {
		t.Helper()
		out, err := exec.Command("/usr/bin/pmset", "-g", "assertions").Output()
		if err != nil {
			t.Fatal(err)
		}
		var mine []string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, fmt.Sprintf("pid %d(", os.Getpid())) && strings.Contains(line, `"`+AssertionName+`"`) {
				mine = append(mine, strings.TrimSpace(line))
			}
		}
		t.Logf("pmset lists %d of Gocast's: %q", len(mine), mine)
		switch {
		case want == "" && len(mine) != 0:
			t.Errorf("still listed: %q", mine)
		case want != "" && (len(mine) != 1 || !strings.Contains(mine[0], string(want))):
			t.Errorf("want one %s, got %q", want, mine)
		}
	}

	if err := k.Start(Request{Kind: Forever}); err != nil {
		t.Fatal(err)
	}
	check(PreventDisplaySleep)
	if err := k.SetAllowDisplaySleep(true); err != nil {
		t.Fatal(err)
	}
	check(PreventSystemSleep)
	if err := k.Start(Request{Kind: For, Duration: MinDuration}); err != nil {
		t.Fatal(err)
	}
	check(PreventSystemSleep)
	k.Stop()
	check("")
}
