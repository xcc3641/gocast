package shortcut

import "testing"

func TestSpotlightEntryHasCmdSpace(t *testing.T) {
	for _, c := range []struct {
		name, entry string
		want        bool
	}{
		{"on, ⌘Space", `{"enabled":true,"value":{"type":"standard","parameters":[32,49,1048576]}}`, true},
		{"off", `{"enabled":false,"value":{"type":"standard","parameters":[65535,49,1048576]}}`, false},
		{"moved to ⌃Space", `{"enabled":true,"value":{"type":"standard","parameters":[32,49,262144]}}`, false},
		{"moved to ⌥⌘Space", `{"enabled":true,"value":{"type":"standard","parameters":[32,49,1572864]}}`, false},
		{"moved to ⌘F1", `{"enabled":true,"value":{"type":"standard","parameters":[65535,122,1048576]}}`, false},
		{"no enabled key", `{"value":{"parameters":[32,49,1048576]}}`, true},
		{"no parameters", `{"enabled":true}`, true},
		{"unreadable", `<plist/>`, true},
	} {
		if got := spotlightEntryHasCmdSpace([]byte(c.entry)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSpotlightHasCmdSpace(t *testing.T) {
	// Only reads; the answer depends on the Mac.
	t.Logf("Spotlight has ⌘Space: %v", SpotlightHasCmdSpace())
}
