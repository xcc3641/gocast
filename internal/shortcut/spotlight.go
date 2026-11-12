package shortcut

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
)

// SpotlightURL opens System Settings › Keyboard, whose Keyboard Shortcuts…
// has Spotlight's ⌘Space.
const SpotlightURL = "x-apple.systempreferences:com.apple.preference.keyboard?Shortcuts"

// spotlightKey is Spotlight's "Show Spotlight search" among the system's
// symbolic hot keys, and the key code and modifiers of ⌘Space, its
// default.
const (
	spotlightKey = "64"
	spaceCode    = 49
	cmdMask      = 1 << 20 // NSEventModifierFlagCommand
)

// SpotlightHasCmdSpace reports whether Spotlight opens with ⌘Space, which
// would open it with the search panel. It only reads the preferences
// (defaults export), and says yes when it can't: ⌘Space is Spotlight's
// unless the user changed it.
//
// GOCAST_SPOTLIGHT_ENTRY names a file to read the entry from instead, in
// JSON (see spotlightEntryHasCmdSpace), for trying the notice of
// Settings › Shortcuts with the Mac's settings left as they are.
func SpotlightHasCmdSpace() bool {
	if path := os.Getenv("GOCAST_SPOTLIGHT_ENTRY"); path != "" {
		entry, err := os.ReadFile(path)
		return err != nil || spotlightEntryHasCmdSpace(entry)
	}
	plist, err := exec.Command("/usr/bin/defaults", "export", "com.apple.symbolichotkeys", "-").Output()
	if err != nil {
		return true
	}
	cmd := exec.Command("/usr/bin/plutil", "-extract", "AppleSymbolicHotKeys."+spotlightKey, "json", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(plist)
	entry, err := cmd.Output()
	if err != nil {
		return true // no entry: the default
	}
	return spotlightEntryHasCmdSpace(entry)
}

// spotlightEntryHasCmdSpace reports whether the symbolic hot key entry, in
// JSON, as {"enabled":true,"value":{"parameters":[32,49,1048576]}}, is an
// enabled ⌘Space; an entry it can't read is the default, ⌘Space.
func spotlightEntryHasCmdSpace(entry []byte) bool {
	var e struct {
		Enabled *bool `json:"enabled"`
		Value   struct {
			Parameters []int64 `json:"parameters"`
		} `json:"value"`
	}
	if err := json.Unmarshal(entry, &e); err != nil {
		return true
	}
	if e.Enabled != nil && !*e.Enabled {
		return false
	}
	p := e.Value.Parameters
	if len(p) < 3 {
		return true
	}
	// The modifiers but the device-dependent bits, which some versions
	// leave in.
	return p[1] == spaceCode && p[2]&0xFFFF0000 == cmdMask
}
