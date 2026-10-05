package covers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// library.MaxCoverBytes (internal/library/covers.go) must equal this
// pipeline's own skipMaxBytes, not merely mirror it by comment: the
// export side filters by library.MaxCoverBytes, so a lower skipMaxBytes
// here would leave a cover the owner actually holds out of every export
// without either file changing. This is a white-box test (package
// covers) because skipMaxBytes is unexported.
func TestMaxCoverBytesMatchesSkipMaxBytes(t *testing.T) {
	if library.MaxCoverBytes != skipMaxBytes {
		t.Fatalf("library.MaxCoverBytes = %d, want %d (skipMaxBytes)", library.MaxCoverBytes, skipMaxBytes)
	}
}

// --- fixture builders -------------------------------------------------
//
// Fixtures are built in-process wherever a pure-Go encoder exists. WebP
// has none in the standard toolchain or golang.org/x/image (decode
// only), so testdata/tiny.webp is the one small (60-byte) committed
// binary this package needs.

func encodeJPEG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg fixture: %v", err)
	}
	return buf.Bytes()
}

func encodePNGWithAlpha(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 40, B: 40, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png fixture: %v", err)
	}
	return buf.Bytes()
}

func encodeGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	pal := color.Palette{color.White, color.Black}
	img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetColorIndex(x, y, 1)
		}
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode gif fixture: %v", err)
	}
	return buf.Bytes()
}

func readWebPFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/tiny.webp")
	if err != nil {
		t.Fatalf("read testdata/tiny.webp: %v", err)
	}
	return data
}

// buildBombPNG returns bytes that are a complete, checksum-valid PNG
// IHDR chunk declaring the given dimensions, with no IDAT/IEND. That is
// all image/png's DecodeConfig ever reads for a non-paletted colour type,
// so it is enough to make a convincing decompression-bomb fixture without
// the gigabytes of real pixel data the declared size implies.
func buildBombPNG(t *testing.T, w, h uint32) []byte {
	t.Helper()
	sig := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], w)
	binary.BigEndian.PutUint32(data[4:8], h)
	data[8] = 8 // bit depth
	data[9] = 2 // colour type: truecolour
	data[10] = 0
	data[11] = 0
	data[12] = 0
	chunkType := []byte("IHDR")
	crc := crc32.ChecksumIEEE(append(append([]byte{}, chunkType...), data...))

	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	var crcBytes [4]byte
	binary.BigEndian.PutUint32(crcBytes[:], crc)

	out := append([]byte{}, sig...)
	out = append(out, length[:]...)
	out = append(out, chunkType...)
	out = append(out, data...)
	out = append(out, crcBytes[:]...)
	return out
}

// --- Normalize ----------------------------------------------------------

func TestNormalizeAcceptedFormats(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"jpeg", encodeJPEG(t, 800, 400, color.RGBA{200, 40, 40, 255})},
		{"png with alpha", encodePNGWithAlpha(t, 800, 400)},
		{"gif", encodeGIF(t, 50, 50)},
		{"webp", readWebPFixture(t)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Normalize(tt.data)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("decode normalized output: %v", err)
			}
			if format != "jpeg" {
				t.Fatalf("output format = %q, want jpeg", format)
			}
			long := cfg.Width
			if cfg.Height > long {
				long = cfg.Height
			}
			if long > maxLongSide {
				t.Fatalf("output long side = %d, want <= %d", long, maxLongSide)
			}
		})
	}
}

func TestNormalizeOversizedScalesDownNeverUp(t *testing.T) {
	// Larger than the bound: must come back scaled to it exactly on the
	// long side, aspect ratio preserved.
	big := encodeJPEG(t, 1200, 600, color.RGBA{10, 10, 10, 255})
	// Splice in an Exif segment so the skip path cannot apply, exercising
	// the full decode/scale/re-encode route regardless of size.
	big = spliceExifIntoJPEG(t, big, "II", 1)

	out, err := Normalize(big)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Width != 600 || cfg.Height != 300 {
		t.Fatalf("got %dx%d, want 600x300 (scaled from 1200x600)", cfg.Width, cfg.Height)
	}

	// Smaller than the bound: dimensions must not grow.
	small := encodeJPEG(t, 20, 15, color.RGBA{10, 10, 10, 255})
	small = spliceExifIntoJPEG(t, small, "II", 1)
	out, err = Normalize(small)
	if err != nil {
		t.Fatalf("Normalize(small) error = %v", err)
	}
	cfg, _, err = image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode small: %v", err)
	}
	if cfg.Width != 20 || cfg.Height != 15 {
		t.Fatalf("got %dx%d, want the original 20x15 (never upscaled)", cfg.Width, cfg.Height)
	}
}

func TestNormalizeSkipPathIsByteIdentical(t *testing.T) {
	// A small JPEG with no Exif segment, well under the byte cap:
	// Normalize must return the exact same bytes, not a re-encode.
	data := encodeJPEG(t, 40, 30, color.RGBA{80, 80, 80, 255})
	out, err := Normalize(data)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("skip path re-encoded %d bytes into %d; want byte-identical", len(data), len(out))
	}

	// Idempotent: normalizing the already-normalized output again is
	// still a no-op (the scenario behind the startup pass and re-import).
	again, err := Normalize(out)
	if err != nil {
		t.Fatalf("second Normalize() error = %v", err)
	}
	if !bytes.Equal(again, out) {
		t.Fatalf("re-normalizing changed the bytes; want a no-op")
	}
}

func TestNormalizeWithExifAlwaysReencodes(t *testing.T) {
	// Otherwise identical to the skip-path fixture, but carrying an Exif
	// segment: must not be returned unchanged, because re-encoding is
	// what strips Exif (including any GPS position).
	data := encodeJPEG(t, 40, 30, color.RGBA{80, 80, 80, 255})
	withExif := spliceExifIntoJPEG(t, data, "II", 1)

	out, err := Normalize(withExif)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if hasEXIF(out) {
		t.Fatalf("normalized output still carries an Exif segment")
	}
	if bytes.Equal(out, withExif) {
		t.Fatalf("output was returned unchanged; an Exif-bearing JPEG must always be re-encoded")
	}
}

func TestNormalizeRotatesByOrientation(t *testing.T) {
	// A 40x20 landscape image rotated 90 degrees clockwise (orientation
	// 6) must come out portrait: dimensions swapped.
	data := encodeJPEG(t, 40, 20, color.RGBA{30, 60, 90, 255})
	rotated := spliceExifIntoJPEG(t, data, "MM", 6)

	out, err := Normalize(rotated)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Width != 20 || cfg.Height != 40 {
		t.Fatalf("got %dx%d, want 20x40 (90-degree rotation of 40x20)", cfg.Width, cfg.Height)
	}
}

func TestNormalizeDecompressionBombRefusedBeforeDecode(t *testing.T) {
	bomb := buildBombPNG(t, 100_000, 100_000)
	_, err := Normalize(bomb)
	if err == nil {
		t.Fatalf("Normalize() = nil error, want ErrTooLarge")
	}
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Normalize() error = %v, want ErrTooLarge", err)
	}
}

func TestNormalizeUnreadable(t *testing.T) {
	tests := map[string][]byte{
		"plain text":           []byte("this is not an image, just ordinary bytes"),
		"empty":                {},
		"HEIC-like ftyp box":   append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c'}, make([]byte, 12)...),
		"truncated PNG header": {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Normalize(data)
			if !errors.Is(err, ErrUnreadable) {
				t.Fatalf("Normalize() error = %v, want ErrUnreadable", err)
			}
		})
	}
}

// spliceExifIntoJPEG inserts an APP1 Exif segment carrying the given
// orientation right after a real JPEG's SOI marker, the way a camera
// does, producing a still-valid, still-decodable JPEG.
func spliceExifIntoJPEG(t *testing.T, base []byte, order string, orientation uint16) []byte {
	t.Helper()
	tiff := buildTIFFOrientation(t, order, orientation)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	length := len(payload) + 2
	seg := []byte{0xFF, 0xE1, byte(length >> 8), byte(length)}
	seg = append(seg, payload...)

	out := make([]byte, 0, len(base)+len(seg))
	out = append(out, base[:2]...)
	out = append(out, seg...)
	out = append(out, base[2:]...)
	return out
}

// --- applyOrientation (direct) ------------------------------------------

func TestApplyOrientationUpright(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	img.SetNRGBA(1, 0, color.NRGBA{0, 0, 255, 255})

	got := applyOrientation(img, 1)
	if got != image.Image(img) {
		t.Fatalf("orientation 1 must return the image unchanged")
	}
}

func TestApplyOrientationFlipHorizontal(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	blue := color.NRGBA{0, 0, 255, 255}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, red)
	img.SetNRGBA(1, 0, blue)

	got := toNRGBA(applyOrientation(img, 2))
	if got.NRGBAAt(0, 0) != blue || got.NRGBAAt(1, 0) != red {
		t.Fatalf("flip horizontal: got (%v, %v), want (blue, red)", got.NRGBAAt(0, 0), got.NRGBAAt(1, 0))
	}
}

func TestApplyOrientationRotate90Clockwise(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	blue := color.NRGBA{0, 0, 255, 255}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1)) // a 2-wide, 1-tall strip
	img.SetNRGBA(0, 0, red)
	img.SetNRGBA(1, 0, blue)

	rotated := applyOrientation(img, 6)
	b := rotated.Bounds()
	if b.Dx() != 1 || b.Dy() != 2 {
		t.Fatalf("rotated bounds = %v, want 1x2 (dimensions swapped)", b)
	}
	got := toNRGBA(rotated)
	// Rotating the strip 90 clockwise swings its left point (red) to the
	// top and its right point (blue) to the bottom.
	if got.NRGBAAt(0, 0) != red || got.NRGBAAt(0, 1) != blue {
		t.Fatalf("rotate 90 CW: got top=%v bottom=%v, want top=red bottom=blue", got.NRGBAAt(0, 0), got.NRGBAAt(0, 1))
	}
}
