package app

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/usage"
	"gocast/locales"
)

// blank reports whether the frame is transparent throughout: the window
// shows nothing of it.
func blank(img *image.RGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			return false
		}
	}
	return true
}

// panelTester draws the search panel as its window does, the tester
// standing in for the window: hidden at first, as it is created.
func panelTester(t *testing.T) (*launcher, *ui.Tester) {
	t.Helper()
	inLocale(t, "en")
	l := testLauncher(t, usage.New(), []presets.Preset{{ID: "p", Name: "pi", Dir: "~"}})
	l.clip = newClipView(l)
	for _, a := range l.apps {
		l.icons[a.Path] = nil // drawn without: icons need the app running
	}
	l.g.clip = &clipService{g: l.g, noAX: true}
	return l, ui.NewTester(l.view, windowWidth, windowHeight)
}

// What the window keeps as it hides, and shows first as it shows again
// until the frame of what it shows now draws, is nothing: neither the
// search panel as the clipboard shortcut opens the clipboard view, nor
// the reverse (design.md 2.8.1).
func TestPanelKeepsNothingHidden(t *testing.T) {
	l, tt := panelTester(t)
	search, clip := locales.SearchSuggestions(), locales.ClipPlaceholder()
	has := func(s string) bool { return slices.Contains(tt.Texts(), s) }

	// Created hidden, its first frame (which the window keeps) is empty.
	tt.Frame()
	if !blank(tt.Image()) || len(tt.Texts()) > 0 {
		t.Fatalf("first frame, not shown yet: %q", tt.Texts())
	}

	// The search shortcut: the search panel.
	now := time.Now()
	l.enterSearch(now)
	tt.Frame()
	if !has(search) || has(clip) || blank(tt.Image()) {
		t.Fatalf("search panel: %q", tt.Texts())
	}

	// Hidden: as the window leaves the screen, a frame of nothing, which
	// the window keeps.
	l.leave(now)
	tt.SetWindowHidden(true)
	if !blank(tt.Image()) || len(tt.Texts()) > 0 {
		t.Fatalf("hidden after the search panel: %q", tt.Texts())
	}

	// The clipboard shortcut while hidden: the state changes before the
	// window shows, and no frame draws out of sight; as it shows, its
	// first frame is the clipboard view.
	l.enterClip(pasteTarget{}, false)
	tt.Frame()
	if !blank(tt.Image()) {
		t.Fatal("a frame drawn out of sight")
	}
	tt.SetWindowHidden(false)
	if !has(clip) || has(search) || blank(tt.Image()) {
		t.Fatalf("clipboard view as it shows: %q", tt.Texts())
	}

	// The reverse: hidden from the clipboard view, then the search
	// shortcut.
	l.leave(now)
	tt.SetWindowHidden(true)
	if !blank(tt.Image()) || len(tt.Texts()) > 0 {
		t.Fatalf("hidden after the clipboard view: %q", tt.Texts())
	}
	l.enterSearch(now.Add(10 * time.Second))
	tt.SetWindowHidden(false)
	if !has(search) || has(clip) {
		t.Fatalf("search panel as it shows: %q", tt.Texts())
	}
}

// Hidden and shown again within 90 seconds, the search panel is as it was
// (design.md 2.6), though it drew nothing meanwhile.
func TestPanelRestoredAfterHiding(t *testing.T) {
	l, tt := panelTester(t)
	now := time.Now()
	l.enterSearch(now)
	l.query = "saf"
	l.filter()
	tt.Frame()
	l.leave(now)
	tt.SetWindowHidden(true)

	l.enterSearch(now.Add(30 * time.Second))
	if l.query != "saf" || !slices.Equal(shown(l), []string{"> app:com.apple.Safari"}) || !l.selectAll {
		t.Errorf("restored: %q %q, select all %v", l.query, shown(l), l.selectAll)
	}
	tt.SetWindowHidden(false)
	if !slices.Contains(tt.Texts(), "Safari") {
		t.Errorf("restored frame: %q", tt.Texts())
	}

	// After 90 seconds: empty, the suggestions.
	l.leave(now.Add(40 * time.Second))
	tt.SetWindowHidden(true)
	l.enterSearch(now.Add(40*time.Second + restoreFor))
	tt.SetWindowHidden(false)
	if l.query != "" || !slices.Contains(tt.Texts(), locales.SearchSuggestions()) {
		t.Errorf("after 90 s: %q, %q", l.query, tt.Texts())
	}

	// The clipboard view is never restored: the search panel shows.
	l.enterClip(pasteTarget{}, false)
	tt.Frame()
	l.leave(now.Add(50 * time.Second))
	tt.SetWindowHidden(true)
	l.enterSearch(now.Add(55 * time.Second))
	tt.SetWindowHidden(false)
	if l.clip.open || slices.Contains(tt.Texts(), locales.ClipPlaceholder()) {
		t.Errorf("after the clipboard view: %q", tt.Texts())
	}
}
