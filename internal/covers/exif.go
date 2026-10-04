package covers

import "encoding/binary"

// exifOrientation reads the EXIF orientation tag (IFD0 tag 0x0112) from a
// JPEG's APP1 Exif segment. It returns 1 (upright, no transform needed)
// when the segment or the tag is absent, or when anything about either
// fails to parse: a reader that cannot tell upright from rotated must
// never refuse the image over it (cover-management: EXIF Orientation
// Correction).
func exifOrientation(data []byte) int {
	tiff := findAPP1Exif(data)
	if tiff == nil {
		return 1
	}
	o, ok := parseOrientation(tiff)
	if !ok {
		return 1
	}
	return o
}

// hasEXIF reports whether data carries an APP1 Exif segment at all,
// whatever it says. Normalize's skip path (an already-normalized JPEG
// returned unchanged) requires no Exif segment, not merely an upright
// orientation, so that re-encoding still strips it — including any GPS
// position — the first time a cover with one is seen.
func hasEXIF(data []byte) bool {
	return findAPP1Exif(data) != nil
}

// findAPP1Exif scans a JPEG's marker segments for APP1 (0xFFE1) whose
// payload starts with the six-byte "Exif\0\0" header, and returns the
// TIFF data that follows it. It returns nil for anything that is not a
// well-formed run of JPEG segments ending at the start of scan data
// (0xFFDA) — Exif, when present, always comes before that — including a
// truncated or malformed segment.
func findAPP1Exif(data []byte) []byte {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return nil
		}
		marker := data[i+1]
		// Markers with no length/payload of their own.
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8) {
			i += 2
			continue
		}
		if marker == 0xDA { // start of scan: no Exif segment seen
			return nil
		}
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return nil
		}
		payload := data[i+4 : i+2+length]
		if marker == 0xE1 && len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
			return payload[6:]
		}
		i += 2 + length
	}
	return nil
}

// parseOrientation reads IFD0's orientation tag from TIFF data — the
// bytes right after an Exif segment's "Exif\0\0" header.
func parseOrientation(tiff []byte) (int, bool) {
	if len(tiff) < 8 {
		return 0, false
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0, false
	}
	if order.Uint16(tiff[2:4]) != 0x002A {
		return 0, false
	}
	ifdOffset := order.Uint32(tiff[4:8])
	if ifdOffset < 8 || int(ifdOffset)+2 > len(tiff) {
		return 0, false
	}
	entryCount := int(order.Uint16(tiff[ifdOffset : ifdOffset+2]))
	entriesStart := int(ifdOffset) + 2
	for e := range entryCount {
		off := entriesStart + e*12
		if off+12 > len(tiff) {
			return 0, false
		}
		tag := order.Uint16(tiff[off : off+2])
		if tag != 0x0112 {
			continue
		}
		typ := order.Uint16(tiff[off+2 : off+4])
		if typ != 3 { // SHORT: the only type a real orientation tag uses
			return 0, false
		}
		value := int(order.Uint16(tiff[off+8 : off+10]))
		if value < 1 || value > 8 {
			return 0, false
		}
		return value, true
	}
	return 0, false
}
