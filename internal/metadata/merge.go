package metadata

import (
	"slices"
	"strings"
	"unicode"
)

// merge combines Open Library's and Google Books' results into one
// deduplicated, ranked list (design.md ADR-1, Result Merge and
// Deduplication). It is pure: no HTTP, no template, nothing but slices and
// strings, so it is fully covered by merge_test.go alone.
//
// Open Library leads: its results keep their relevance order, and any
// Google Books result found to be the same book is folded into the
// matching Open Library entry, filling only the fields that entry lacks
// (cover, pages, publisher, ISBN) — its own fields and its position are
// never disturbed. A Google result with no match follows every Open
// Library result, in Google's own order. The combined list is cut to
// limit.
//
// more counts matches known to exist but not returned here: Open Library's
// own olTotal (numFound, exact) plus every Google-only result actually
// fetched (before the limit cut), minus how many are shown — floored at
// zero. Google's own totalItems plays no part in this arithmetic: it is an
// unreliable estimate, and counting it would make "more" dishonest about
// what "more" means (SPEC §0).
func merge(ol, gb []Book, olTotal, limit int) (books []Book, more int) {
	out := make([]Book, len(ol))
	copy(out, ol)

	used := make([]bool, len(gb))
	for i, lead := range out {
		for j, cand := range gb {
			if used[j] || !sameBook(lead, cand) {
				continue
			}
			out[i] = fill(lead, cand)
			used[j] = true
			break
		}
	}

	googleOnly := 0
	for j, cand := range gb {
		if !used[j] {
			out = append(out, cand)
			googleOnly++
		}
	}

	shown := out
	if len(shown) > limit {
		shown = shown[:limit]
	}
	return shown, max(olTotal+googleOnly-len(shown), 0)
}

// sameBook reports whether a and b name the same edition or work: a shared
// ISBN-13, or, when neither side offers one in common, the same folded main
// title and folded first-author surname (merge rule 2).
func sameBook(a, b Book) bool {
	if shareISBN(a.ISBNs, b.ISBNs) {
		return true
	}
	title := foldedMainTitle(a.Title)
	surname := foldedSurname(a.Author)
	return title != "" && surname != "" &&
		title == foldedMainTitle(b.Title) && surname == foldedSurname(b.Author)
}

// shareISBN reports whether a and b hold any normalized ISBN-13 in common.
func shareISBN(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	have := make(map[string]bool, len(a))
	for _, s := range a {
		have[s] = true
	}
	for _, s := range b {
		if have[s] {
			return true
		}
	}
	return false
}

// foldedMainTitle is the text before the first colon, lower-cased, letters
// and digits only — so "Example: A Long Subtitle" and "Example" fold the
// same (merge rule 2, subtitle scenario).
func foldedMainTitle(title string) string {
	if i := strings.IndexByte(title, ':'); i >= 0 {
		title = title[:i]
	}
	return foldAlnum(title)
}

// foldedSurname folds an author's last space-separated word, a plain
// stand-in for "surname" with no name-parsing library.
func foldedSurname(author string) string {
	fields := strings.Fields(author)
	if len(fields) == 0 {
		return ""
	}
	return foldAlnum(fields[len(fields)-1])
}

// foldAlnum lower-cases s and drops everything but letters and digits, by
// Unicode category rather than the ASCII range: an accented name (e.g.
// "García") must fold the same whichever source happens to send the accent.
func foldAlnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fill keeps lead's position and its own fields, taking from other only
// what lead lacks: cover, pages, publisher and ISBN (merge rule 3). ISBNs
// is never shown; it always gains whatever other knows that lead does not,
// so a later merge can still recognise the same book by either source's
// ISBN.
func fill(lead, other Book) Book {
	if lead.CoverURL == "" {
		lead.CoverURL, lead.ThumbURL = other.CoverURL, other.ThumbURL
	}
	if lead.Pages == 0 {
		lead.Pages = other.Pages
	}
	if lead.Publisher == "" {
		lead.Publisher = other.Publisher
	}
	if lead.ISBN == "" {
		lead.ISBN = other.ISBN
	}
	lead.ISBNs = unionISBNs(lead.ISBNs, other.ISBNs)
	return lead
}

// unionISBNs concatenates and deduplicates, keeping a's order first. Two
// empty sides stay nil rather than becoming a needless empty slice.
func unionISBNs(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range slices.Concat(a, b) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
