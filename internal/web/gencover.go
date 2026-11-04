package web

import (
	"hash/fnv"
	"strings"
)

// The generated cover of a coverless book (DESIGN.md "Generated cover";
// design.md ADR-12) is drawn by the templates and app.css. The only
// decisions made in Go are these two.

// coverCloth picks which of the eight cover cloths (.gc-0 to .gc-7 in
// app.css) a book wears, from its title alone: the capture preview has no
// item id yet, and must show the cloth the filed book will wear.
//
// The title is trimmed and A to Z lower-cased, and nothing else: full
// Unicode lower-casing differs between Go and JavaScript, and
// static/covers.js runs this same hash for the preview.
func coverCloth(title string) int {
	folded := []byte(strings.TrimSpace(title))
	for i, b := range folded {
		if 'A' <= b && b <= 'Z' {
			folded[i] = b + ('a' - 'A')
		}
	}
	hash := fnv.New32a()
	hash.Write(folded) // a hash's Write never returns an error
	return int(hash.Sum32() % 8)
}

// coverTitle is the part of a title the cover is stamped with: the main
// title, before the first ": ". The full title is always set beside the
// plate.
func coverTitle(title string) string {
	main, _, _ := strings.Cut(title, ": ")
	return main
}
