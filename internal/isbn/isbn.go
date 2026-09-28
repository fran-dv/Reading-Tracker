// Package isbn normalizes and validates ISBNs. It is a pure function with no
// HTTP, template, or Datastar imports (CLAUDE.md), shared by internal/metadata
// (to de-duplicate results and answer an ISBN lookup) and internal/library
// (to validate and store items.isbn).
package isbn

// Normalize accepts an ISBN-10 (a final check digit of 'X' allowed) or an
// ISBN-13, with hyphens and spaces forgiven anywhere, verifies its checksum,
// and returns the canonical ISBN-13 form: 13 digits, no separators. ok is
// false for anything that does not check out, and isbn13 is then "".
//
// An ISBN-10 is converted by replacing its own check digit with the "978"
// prefix and a check digit recomputed for the result, the standard rule. A
// 13-digit input is returned as-is once its own checksum passes: some books
// (those under the newer "979" prefix) have no ISBN-10 form at all.
func Normalize(s string) (isbn13 string, ok bool) {
	digits := strip(s)
	switch len(digits) {
	case 10:
		if !validISBN10(digits) {
			return "", false
		}
		return toISBN13(digits), true
	case 13:
		if !validISBN13(digits) {
			return "", false
		}
		return digits, true
	default:
		return "", false
	}
}

// strip drops hyphens and spaces and upper-cases a trailing check letter.
// Any other character is kept as-is, so it still fails the length check
// below rather than being silently dropped from a plausible-looking input.
func strip(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '-' || c == ' ':
			continue
		case c == 'x':
			out = append(out, 'X')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// validISBN10 checks the weighted-sum-mod-11 rule. Only the last digit may
// be the 'X' that stands for 10.
func validISBN10(d string) bool {
	sum := 0
	for i := 0; i < 10; i++ {
		v, ok := digitOrX(d[i], i == 9)
		if !ok {
			return false
		}
		sum += (10 - i) * v
	}
	return sum%11 == 0
}

// validISBN13 checks the alternating 1-3 weighted-sum-mod-10 rule.
func validISBN13(d string) bool {
	sum := 0
	for i := 0; i < 13; i++ {
		v, ok := digitOrX(d[i], false)
		if !ok {
			return false
		}
		if i%2 == 0 {
			sum += v
		} else {
			sum += 3 * v
		}
	}
	return sum%10 == 0
}

// toISBN13 keeps an ISBN-10's first nine digits under the "978" prefix and
// recomputes the check digit for the result.
func toISBN13(isbn10 string) string {
	prefix := "978" + isbn10[:9]
	sum := 0
	for i := 0; i < 12; i++ {
		v := int(prefix[i] - '0')
		if i%2 == 0 {
			sum += v
		} else {
			sum += 3 * v
		}
	}
	check := (10 - sum%10) % 10
	return prefix + string(rune('0'+check))
}

// digitOrX reads one ISBN character: a digit anywhere, or 'X' (worth 10)
// only where allowX says it may appear.
func digitOrX(c byte, allowX bool) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c == 'X' && allowX:
		return 10, true
	default:
		return 0, false
	}
}
