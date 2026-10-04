package covers

import (
	"encoding/binary"
	"testing"
)

// buildJPEGWithExif returns a minimal byte stream — just enough of a JPEG's
// marker structure for findAPP1Exif to work on — carrying one APP1 Exif
// segment whose IFD0 holds a single orientation entry.
func buildJPEGWithExif(t *testing.T, byteOrder string, orientation uint16) []byte {
	t.Helper()
	tiff := buildTIFFOrientation(t, byteOrder, orientation)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	return wrapAPP1(payload)
}

// buildTIFFOrientation builds a TIFF header with one IFD0 entry: the
// orientation tag (0x0112), type SHORT, holding the given value.
func buildTIFFOrientation(t *testing.T, byteOrder string, orientation uint16) []byte {
	t.Helper()
	var order binary.ByteOrder
	switch byteOrder {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		t.Fatalf("bad byte order %q", byteOrder)
	}
	buf := make([]byte, 26) // header(8) + entry count(2) + one entry(12) + next-IFD offset(4)
	copy(buf[0:2], byteOrder)
	order.PutUint16(buf[2:4], 0x002A)
	order.PutUint32(buf[4:8], 8) // IFD0 right after the header
	order.PutUint16(buf[8:10], 1)
	entry := buf[10:22]
	order.PutUint16(entry[0:2], 0x0112) // tag: orientation
	order.PutUint16(entry[2:4], 3)      // type: SHORT
	order.PutUint32(entry[4:8], 1)      // count: 1
	order.PutUint16(entry[8:10], orientation)
	order.PutUint32(buf[22:26], 0) // no next IFD
	return buf
}

// wrapAPP1 wraps a payload in a minimal SOI + APP1 segment pair.
func wrapAPP1(payload []byte) []byte {
	length := len(payload) + 2 // the length field itself counts
	b := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte(length >> 8), byte(length)}
	return append(b, payload...)
}

func TestExifOrientationAllValuesBothByteOrders(t *testing.T) {
	for _, order := range []string{"II", "MM"} {
		for o := uint16(1); o <= 8; o++ {
			t.Run(order+"/"+string(rune('0'+o)), func(t *testing.T) {
				data := buildJPEGWithExif(t, order, o)
				got := exifOrientation(data)
				if got != int(o) {
					t.Fatalf("exifOrientation() = %d, want %d", got, o)
				}
				if !hasEXIF(data) {
					t.Fatalf("hasEXIF() = false, want true")
				}
			})
		}
	}
}

func TestExifOrientationNoSegment(t *testing.T) {
	// A JPEG with no APP1 at all (just SOI then straight to scan data).
	data := []byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02}
	if got := exifOrientation(data); got != 1 {
		t.Fatalf("exifOrientation() = %d, want 1 (upright)", got)
	}
	if hasEXIF(data) {
		t.Fatalf("hasEXIF() = true, want false")
	}
}

func TestExifOrientationMalformedSegment(t *testing.T) {
	cases := map[string][]byte{
		"not a JPEG at all":                      []byte("not an image"),
		"truncated after SOI":                    {0xFF, 0xD8},
		"APP1 too short to hold the Exif header": wrapAPP1([]byte("Exi")),
		"Exif header with no TIFF data that follows": wrapAPP1(
			append([]byte("Exif\x00\x00"), []byte{1, 2, 3}...),
		),
		"bad TIFF byte-order marker": wrapAPP1(
			append([]byte("Exif\x00\x00"), []byte{'X', 'X', 0, 0, 0, 0, 0, 0}...),
		),
		"IFD0 offset past the end of the data": wrapAPP1(
			append([]byte("Exif\x00\x00"), []byte{'I', 'I', 0x2A, 0x00, 0xFF, 0xFF, 0, 0}...),
		),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if got := exifOrientation(data); got != 1 {
				t.Fatalf("exifOrientation() = %d, want 1 (upright, never refused)", got)
			}
		})
	}
}

func TestExifOrientationOutOfRangeValueIsUpright(t *testing.T) {
	// A technically well-formed entry whose value isn't one of the eight
	// real orientations must still fail safe to upright, not be trusted.
	data := buildJPEGWithExif(t, "II", 9)
	if got := exifOrientation(data); got != 1 {
		t.Fatalf("exifOrientation() = %d, want 1 for an out-of-range value", got)
	}
}

func TestExifOrientationWrongTagType(t *testing.T) {
	tiff := buildTIFFOrientation(t, "II", 6)
	// Corrupt the entry's type field (offset 12 within the TIFF buffer,
	// i.e. byte 2 of the 12-byte entry that starts at offset 10) so it no
	// longer claims to be a SHORT.
	tiff[12] = 4 // LONG, not SHORT
	data := wrapAPP1(append([]byte("Exif\x00\x00"), tiff...))
	if got := exifOrientation(data); got != 1 {
		t.Fatalf("exifOrientation() = %d, want 1 when the tag isn't a SHORT", got)
	}
}
