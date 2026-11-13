package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// ThumbnailSize is the most a thumbnail is wide and high, in pixels: a
// 26-point list icon at 2×.
const ThumbnailSize = 52

// thumbSamples is how many source pixels, per side, a thumbnail pixel
// averages at most.
const thumbSamples = 8

// thumbnail makes the PNG thumbnail of a PNG image: scaled down to fit
// in ThumbnailSize×ThumbnailSize, keeping its proportions, never scaled
// up. It returns nil for an image it cannot decode.
func thumbnail(b []byte) []byte {
	src, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	return thumbnailOf(src)
}

// thumbnailOf is thumbnail of a decoded image.
func thumbnailOf(src image.Image) []byte {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return nil
	}
	dw, dh := sw, sh
	if sw > ThumbnailSize || sh > ThumbnailSize {
		if sw >= sh {
			dw, dh = ThumbnailSize, max(1, (sh*ThumbnailSize+sw/2)/sw)
		} else {
			dw, dh = max(1, (sw*ThumbnailSize+sh/2)/sh), ThumbnailSize
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := sb.Min.Y+y*sh/dh, sb.Min.Y+(y+1)*sh/dh
		for x := 0; x < dw; x++ {
			x0, x1 := sb.Min.X+x*sw/dw, sb.Min.X+(x+1)*sw/dw
			dst.SetRGBA(x, y, average(src, x0, y0, max(x1, x0+1), max(y1, y0+1)))
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil
	}
	return out.Bytes()
}

// average is the mean color of up to thumbSamples² pixels spread evenly
// over the rectangle [x0, x1) × [y0, y1), premultiplied.
func average(src image.Image, x0, y0, x1, y1 int) color.RGBA {
	nx, ny := min(x1-x0, thumbSamples), min(y1-y0, thumbSamples)
	var r, g, b, a uint64
	for j := 0; j < ny; j++ {
		y := y0 + (2*j+1)*(y1-y0)/(2*ny)
		for i := 0; i < nx; i++ {
			x := x0 + (2*i+1)*(x1-x0)/(2*nx)
			cr, cg, cb, ca := src.At(x, y).RGBA()
			r, g, b, a = r+uint64(cr), g+uint64(cg), b+uint64(cb), a+uint64(ca)
		}
	}
	n := uint64(nx * ny)
	return color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(b / n >> 8), uint8(a / n >> 8)}
}
