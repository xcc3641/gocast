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
		{ID: "d", Path: "/Applications/Mail.app", Name: "Mail"},
		{ID: "e", Path: "/Volumes/A/Apps/mail.app", Name: "Mail"},
		{ID: "f", Path: "/Volumes/B/Apps/Mail.app", Name: "Mail"},
		{ID: "g", Path: "/Volumes/B/Apps/Mail Pro.app", Name: "Mail"},
		{ID: "h", Path: "/Applications/X/Tool.app", Name: "Tool"},
		{ID: "i", Path: "/Applications/Y/Tool.app", Name: "Tool"},
		{ID: "j", Path: "/Applications/Z/Tool 2.app", Name: "Tool"},
	}
	want := []string{
		// The file name, none where it is the name.
		"Siri AI", "",
		// Names and file names the same whatever the case: the folder,
		// as Finder names it.
		"/Applications", "~/Applications/Chrome Apps",
		"",
		// Only the colliding file names get folders; the whole path
		// where those share theirs.
		"/Applications", "/Volumes/A/Apps", "/Volumes/B/Apps", "Mail Pro",
		"/Applications/X", "/Applications/Y", "Tool 2",
	}
	if got := Places(apps); !slices.Equal(got, want) {
		t.Errorf("Places =\n%q\nwant\n%q", got, want)
	}
	same := []App{
		{ID: "x", Path: "/A/Tool.app", Name: "Tool"},
		{ID: "y", Path: "/A/tool.app", Name: "Tool"}, // a case-sensitive volume
	}
	if got, want := Places(same), []string{"/A/Tool.app", "/A/tool.app"}; !slices.Equal(got, want) {
		t.Errorf("Places in one folder = %q, want %q", got, want)
	}
}
