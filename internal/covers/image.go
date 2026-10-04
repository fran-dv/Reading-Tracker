package covers

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registers GIF with image.Decode/DecodeConfig
	"image/jpeg"
	_ "image/png" // registers PNG with image.Decode/DecodeConfig

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers WebP with image.Decode/DecodeConfig
)

// ErrUnreadable means the bytes are not an image this can decode: an
// unrecognized format (HEIC, AVIF, SVG, or anything else no decoder below
// claims), or a recognized format too corrupted to decode at all.
var ErrUnreadable = errors.New("covers: not an image this can read")

// ErrTooLarge means the image's declared dimensions are refused before a
// full decode is attempted, so a small file with an enormous declared
// size (a decompression bomb) never reaches a large allocation.
var ErrTooLarge = errors.New("covers: image too large")

const (
	// maxLongSide is the bound every held cover is normalized to, on its
	// long side. About 8% over a 9rem plate at 3x device pixel ratio.
	maxLongSide = 600
	// maxSidePixels and maxPixels are checked from DecodeConfig alone,
	// before any full decode, so a file that declares an unreasonable
	// size is refused without the allocation it would otherwise force.
	maxSidePixels = 12_000
	maxPixels     = 40_000_000 // 40 megapixels
	// skipMaxBytes bounds the skip path: a JPEG already at or under
	// maxLongSide, with no Exif segment, and at or under this size is
	// returned unchanged rather than needlessly re-encoded.
	skipMaxBytes = 256 * 1024
	// jpegQuality is used for every re-encode.
	jpegQuality = 82
)

// pageStock is the manifest's theme colour, composited under any
// transparency before JPEG encoding, since JPEG carries no alpha channel.
var pageStock = color.NRGBA{R: 0x12, G: 0x0f, B: 0x0c, A: 0xff}

// Normalize decodes data, which may be a JPEG, PNG, GIF, or WebP image
// (in that generality, whatever origin it came from — a lazy fetch, an
// upload, or an edition pick, go through this one pipeline), and returns
// a bounded, re-encoded JPEG: EXIF-rotated upright, scaled to at most
// maxLongSide pixels on its long side (never upscaled), with any
// transparency composited over the page stock colour, and re-encoded at
// jpegQuality. The output is always image/jpeg.
//
// An already-normalized JPEG — at or under maxLongSide, carrying no Exif
// segment, and at or under skipMaxBytes — is returned unchanged. That
// makes the pipeline idempotent: running it again on its own output (a
// repeated startup pass, or a re-import) never re-encodes, so repeated
// normalization costs nothing and loses nothing.
func Normalize(data []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrUnreadable
	}
	if cfg.Width > maxSidePixels || cfg.Height > maxSidePixels || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooLarge
	}

	longSide := max(cfg.Width, cfg.Height)
	if format == "jpeg" && longSide <= maxLongSide && len(data) <= skipMaxBytes && !hasEXIF(data) {
		return data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}

	orientation := 1
	if format == "jpeg" {
		orientation = exifOrientation(data)
	}
	img = applyOrientation(img, orientation)
	img = fitLongSide(img, maxLongSide)
	flat := flatten(img, pageStock)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("covers: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// applyOrientation returns img rotated/flipped for one of the eight EXIF
// orientation values. 1 (upright) and anything outside 1-8 are returned
// unchanged.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	src := toNRGBA(img)

	ow, oh := w, h
	if orientation >= 5 { // transpose, rotate 90/270, transverse swap the sides
		ow, oh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2: // flip horizontal
				dx, dy = w-1-x, y
			case 3: // rotate 180
				dx, dy = w-1-x, h-1-y
			case 4: // flip vertical
				dx, dy = x, h-1-y
			case 5: // transpose (mirror across the top-left/bottom-right diagonal)
				dx, dy = y, x
			case 6: // rotate 90 clockwise
				dx, dy = h-1-y, x
			case 7: // transverse (mirror across the other diagonal)
				dx, dy = h-1-y, w-1-x
			case 8: // rotate 270 clockwise
				dx, dy = y, w-1-x
			}
			dst.SetNRGBA(dx, dy, src.NRGBAAt(x, y))
		}
	}
	return dst
}

// toNRGBA returns img as an *image.NRGBA with bounds starting at (0, 0),
// converting (and copying) only when it isn't one already.
func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Bounds().Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// fitLongSide scales img down so its long side is at most bound,
// preserving aspect ratio. An image already within the bound is returned
// unchanged: this never upscales.
func fitLongSide(img image.Image, bound int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	long := max(w, h)
	if long <= bound {
		return img
	}
	scale := float64(bound) / float64(long)
	nw, nh := int(float64(w)*scale+0.5), int(float64(h)*scale+0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}

// flatten composites img over an opaque background of bg, so a
// transparent source produces a sane JPEG (which has no alpha channel of
// its own) instead of one with the source's unpremultiplied colour
// showing through as noise.
func flatten(img image.Image, bg color.Color) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	return dst
}
