package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestIsImageFile(t *testing.T) {
	for path, want := range map[string]bool{
		"/a/CleanShot 2026-11-13 at 15.01.35@2x.png": true,
		"/a/photo.JPG": true, "/a/x.jpeg": true, "/a/x.webp": true, "/a/x.tiff": true,
		"/a/notes.txt": false, "/a/folder": false, "/a/x.heic": false,
	} {
		if got := IsImageFile(path); got != want {
			t.Errorf("IsImageFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestImageFileThumbnail(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{200, 10, 10, 255})
		}
	}
	dir := t.TempDir()
	for name, enc := range map[string]func(*bytes.Buffer) error{
		"shot.png": func(b *bytes.Buffer) error { return png.Encode(b, img) },
		"shot.jpg": func(b *bytes.Buffer) error { return jpeg.Encode(b, img, nil) },
	} {
		var b bytes.Buffer
		if err := enc(&b); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		thumb, w, h, err := ImageFileThumbnail(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if w != 400 || h != 100 {
			t.Errorf("%s: size %d×%d, want 400×100", name, w, h)
		}
		ti, err := png.Decode(bytes.NewReader(thumb))
		if err != nil {
			t.Fatalf("%s: thumbnail: %v", name, err)
		}
		if b := ti.Bounds(); b.Dx() != ThumbnailSize || b.Dy() != ThumbnailSize/4 {
			t.Errorf("%s: thumbnail %d×%d", name, b.Dx(), b.Dy())
		}
	}
	if _, _, _, err := ImageFileThumbnail(filepath.Join(dir, "gone.png")); err == nil {
		t.Error("a missing file has a thumbnail")
	}
	bad := filepath.Join(dir, "bad.png")
	os.WriteFile(bad, []byte("not an image"), 0o600)
	if _, _, _, err := ImageFileThumbnail(bad); err == nil {
		t.Error("a file that is no image has a thumbnail")
	}
}
