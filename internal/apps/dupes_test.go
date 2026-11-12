package apps

import (
	"os"
	"slices"
	"testing"
)

func TestUnique(t *testing.T) {
	home, _ := os.UserHomeDir()
	apps := []App{
		{ID: "com.google.Chrome", Path: home + "/Applications/Google Chrome.app", Name: "Google Chrome"},
		{ID: "com.google.Chrome", Path: "/Applications/Google Chrome.app", Name: "Google Chrome"},
		{ID: "com.apple.Siri", Path: "/System/Library/CoreServices/Siri.app", Name: "Siri"},
		{ID: "com.apple.siri.launcher", Path: "/System/Applications/Siri.app", Name: "Siri"},
		{ID: "com.apple.campo", Path: "/System/Applications/Siri AI.app", Name: "Siri"},
		{ID: "com.example.tool", Path: "/Volumes/Work/Tool.app", Name: "Tool"},
		{ID: "com.example.tool", Path: home + "/Applications/Tool.app", Name: "Tool"},
		{ID: "com.example.tool", Path: "/Volumes/Old/Tool.app", Name: "Tool"},
		{ID: "com.apple.Terminal", Path: "/System/Applications/Utilities/Terminal.app", Name: "Terminal"},
		{ID: "com.apple.Terminal", Path: "/Applications/Utilities/Terminal.app", Name: "Terminal"},
	}
	// The same ID: one copy, in /Applications or /System/Applications,
	// else ~/Applications, else the first; different IDs all kept.
	if got, want := Unique(apps), []int{1, 2, 3, 4, 6, 8}; !slices.Equal(got, want) {
		t.Errorf("Unique = %v, want %v", got, want)
	}
	if got := Unique(nil); len(got) != 0 {
		t.Errorf("Unique(nil) = %v", got)
	}
}

func TestPlaces(t *testing.T) {
	home, _ := os.UserHomeDir()
	apps := []App{
		{ID: "com.apple.campo", Path: "/System/Applications/Siri AI.app", Name: "Siri"},
		{ID: "com.apple.siri.launcher", Path: "/System/Applications/Siri.app", Name: "Siri"},
		{ID: "a", Path: "/Applications/Notes.app", Name: "Notes"},
		{ID: "b", Path: home + "/Applications/Chrome Apps.localized/Notes.app", Name: "notes"},
		{ID: "c", Path: "/Applications/Safari.app", Name: "Safari"},
	}
	want := []string{
		// One folder: the whole paths tell them apart.
		"/System/Applications/Siri AI.app", "/System/Applications/Siri.app",
		// Names the same whatever the case; the folder as Finder names it.
		"/Applications", "~/Applications/Chrome Apps",
		"",
	}
	if got := Places(apps); !slices.Equal(got, want) {
		t.Errorf("Places =\n%q\nwant\n%q", got, want)
	}
}
