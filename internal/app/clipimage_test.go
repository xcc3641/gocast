package app

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gocast/internal/clipboard"
	"gocast/locales"

	"github.com/egoist/mygo/ui"
)

// clipImageFileView is the clipboard view of a history with a screenshot
// a screenshot tool copied as a file, chosen, and the view's tester.
func clipImageFileView(t *testing.T, locale string, dark bool) (*launcher, *ui.Tester, clipboard.Entry) {
	t.Helper()
	l, tt, _ := hotkeyPanel(t, locale)
	tt.SetScale(2)
	tt.SetDark(dark)
	store := clipboard.NewMemoryStore(clipboard.Options{})
	dir := t.TempDir()
	seedHistory(store, &seedClock{}, filepath.Join(dir, "files"), locale)
	t.Setenv("HOME", dir) // where the screenshot is, as ~/Desktop
	os.Mkdir(filepath.Join(dir, "Desktop"), 0o700)
	shot := filepath.Join(dir, "Desktop", "CleanShot 2026-11-13 at 15.01.35@2x.png")
	if err := os.WriteFile(shot, demoImage(1600, 1000, false), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := store.Add(clipboard.Content{Files: []string{shot}}, clipboard.App{})
	if err != nil {
		t.Fatal(err)
	}
	l.g.clip.store, l.g.clip.loaded = store, true
	l.enterClip(pasteTarget{}, false)
	tt.Frame()
	return l, tt, e
}

// A screenshot copied as a file, as CleanShot copies it, shows as the
// image: its thumbnail in the list, the image in the preview.
func TestClipImageFileShowsImage(t *testing.T) {
	l, tt, e := clipImageFileView(t, "en", false)
	v := l.clip
	if cur, ok := v.current(); !ok || cur.ID != e.ID {
		t.Fatalf("chosen: %v %v", cur.ID, ok)
	}
	path, ok := imageFileOf(e)
	if !ok {
		t.Fatal("not an image file")
	}
	ft := v.fileThumbs[path]
	if ft == nil || !ft.done || ft.bitmap == nil || ft.w != 1600 || ft.h != 1000 {
		t.Fatalf("thumbnail: %+v", ft)
	}
	tt.Frame()
	if v.image.id != e.ID || !v.image.done || v.image.bitmap == nil {
		t.Fatalf("preview image: %+v", v.image)
	}
	if w, h := v.image.bitmap.Size(); w != 1600 || h != 1000 {
		t.Errorf("preview image %d×%d", w, h)
	}

	if got, want := v.rowLabel(e), "CleanShot 2026-11-13 at 15.01.35@2x.png, Image file, 1,600 by 1,000, "; !strings.HasPrefix(got, want) {
		t.Errorf("label %q", got)
	}
	if !slices.Contains(tt.Texts(), locales.ClipDimensions(1600, 1000)) {
		t.Errorf("no dimensions: %q", tt.Texts())
	}

	// Gone, it is a file again: its icon, and that it is missing.
	os.Remove(path)
	if _, ok := imageFileOf(e); ok {
		t.Error("a missing file shows as an image")
	}
	l.g.clip.store.Flush()
	v.refresh()
	tt.Frame()
	if !slices.Contains(tt.Texts(), locales.ClipFileMissing()) {
		t.Errorf("missing: %q", tt.Texts())
	}
}

func TestImageFileOf(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "a.png")
	txt := filepath.Join(dir, "a.txt")
	for _, p := range []string{png, txt} {
		os.WriteFile(p, []byte("x"), 0o600)
	}
	for _, c := range []struct {
		e    clipboard.Entry
		want bool
	}{
		{clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{png}}, true},
		{clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{txt}}, false},
		{clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{png, png}}, false},
		{clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{filepath.Join(dir, "gone.png")}}, false},
		{clipboard.Entry{Kind: clipboard.KindText, Text: png}, false},
	} {
		if _, got := imageFileOf(c.e); got != c.want {
			t.Errorf("imageFileOf(%v) = %v", c.e.Files, got)
		}
	}
}

func TestScreensClipImageFile(t *testing.T) {
	dir := screensDir(t)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		_, tt, _ := clipImageFileView(t, "zh-Hans", dark)
		tt.Frame()
		tt.Advance(time.Second)
		saveScreen(t, tt, dir, "clip-imagefile-"+mode, image.Rect(marginX, marginTop, marginX+clipWidth, marginTop+expandedHeight))
	}
}

// The preview's image has its final size from its first frame: the box
// it fits in is worked out before layout, for any item, so it never shows
// larger and shrinks (Hugo, 0.1.4).
func TestClipPreviewBoxKnownBeforeLayout(t *testing.T) {
	l, tt, e := clipImageFileView(t, "en", false)
	v := l.clip
	for i := range 4 { // the image file, then the items after it
		tt.Frame()
		got := v.pane
		box, ok := tt.Find(locales.ClipPreview())
		if !ok {
			t.Fatalf("%d: no preview", i)
		}
		if abs32(got.W-box.W) > 0.5 || abs32(got.H-box.H) > 0.5 {
			t.Errorf("%d: worked out %v×%v, laid out %v×%v", i, got.W, got.H, box.W, box.H)
		}
		if i == 0 {
			if w, h, ok := v.imageDims(e); !ok || w != 1600 || h != 1000 {
				t.Errorf("size before the thumbnail: %d×%d %v", w, h, ok)
			}
		}
		v.choose(v.chosen + 1)
	}
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}
