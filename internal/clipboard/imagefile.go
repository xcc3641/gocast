package clipboard

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"  // image files: GIF
	_ "image/jpeg" // and JPEG, besides PNG
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// MaxImageFile is the largest image file ReadImageFile reads, in bytes.
const MaxImageFile = 64 << 20

// imageExts are the extensions of the image files that ReadImageFile and
// ImageFileThumbnail read.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".tif": true, ".tiff": true,
}

// IsImageFile reports whether path is named as an image this package
// reads, such as a screenshot that a screenshot tool copied as a file.
func IsImageFile(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))]
}

var errImageTooLarge = errors.New("image file too large")

// ReadImageFile returns the bytes of the image file at path, of at most
// MaxImageFile bytes.
func ReadImageFile(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxImageFile {
		return nil, errImageTooLarge
	}
	return os.ReadFile(path)
}

// DecodeImage decodes an image of any of the formats of IsImageFile.
func DecodeImage(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

// ImageFileThumbnail is the PNG thumbnail of the image file at path, as
// the thumbnails of images, and the image's size in pixels.
// ImageFileSize is how many pixels wide and high the image file at path
// is, from its header alone: quick enough to lay out its preview with
// before the image is read.
func ImageFileSize(path string) (w, h int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, errors.New("empty image")
	}
	return cfg.Width, cfg.Height, nil
}

func ImageFileThumbnail(path string) (thumb []byte, w, h int, err error) {
	b, err := ReadImageFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	img, err := DecodeImage(b)
	if err != nil {
		return nil, 0, 0, err
	}
	thumb = thumbnailOf(img)
	if thumb == nil {
		return nil, 0, 0, errors.New("empty image")
	}
	return thumb, img.Bounds().Dx(), img.Bounds().Dy(), nil
}
