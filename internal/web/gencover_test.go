package web

import (
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// The same vectors are copied into static/covers.js, whose coverCloth must
// agree with this one: the capture preview and the filed book wear the
// same cloth (generated-cover: Deterministic Palette Derivation From the
// Title).
func TestCoverCloth(t *testing.T) {
	tests := []struct {
		title string
		want  int
	}{
		{"Thinking in Systems", 2},
		{"  THINKING IN SYSTEMS  ", 2}, // trimmed, A to Z folded
		{"Dune", 3},
		{"Pedro Páramo", 4}, // á is left as it is
		{"Invisible Cities", 7},
		{"Ulysses", 1},
		{"Emma", 5},
		{"Stanisław Lem", 0},
		{"こころ", 1},
		{"", 5},
	}
	for _, tt := range tests {
		if got := coverCloth(tt.title); got != tt.want {
			t.Errorf("coverCloth(%q) = %d, want %d", tt.title, got, tt.want)
		}
	}
}

// Whatever the title, the cloth is one of the eight in app.css.
func TestCoverClothStaysInRange(t *testing.T) {
	for _, title := range []string{"a", "b", "Dune", "Ἰλιάς", "War and Peace", "1984"} {
		if got := coverCloth(title); got < 0 || got > 7 {
			t.Errorf("coverCloth(%q) = %d, want 0 to 7", title, got)
		}
	}
}

func TestCoverTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Example Title: A Longer Subtitle", "Example Title"},
		{"Thinking in Systems", "Thinking in Systems"},
		{"Gödel, Escher, Bach: an Eternal Golden Braid", "Gödel, Escher, Bach"},
		{"Ratio 3:2 explained", "Ratio 3:2 explained"}, // only a colon and a space start a subtitle
		{"One: Two: Three", "One"},
	}
	for _, tt := range tests {
		if got := coverTitle(tt.title); got != tt.want {
			t.Errorf("coverTitle(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

// A book without a cover draws the generated cover wherever its plate
// renders; any other format keeps the banded cloth, and a book with a
// cover shows it (generated-cover: Books-Only Generated Cover Rule).
func TestCoverlessBookDrawsTheGeneratedCover(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, err := svc.CreateShelf(ctx, "Statistics")
	if err != nil {
		t.Fatal(err)
	}
	fileItem(t, svc, library.Item{
		Title: "Dune: The Desert Planet", Author: "Frank Herbert", Publisher: "Chilton",
		Why: "sand", Format: library.FormatBook, ShelfID: shelf.ID,
	})
	fileItem(t, svc, library.Item{
		Title: "Invisible Cities", Why: "no publisher", Format: library.FormatBook, ShelfID: shelf.ID,
	})
	fileItem(t, svc, library.Item{
		Title: "A talk", Why: "no cover", Format: library.FormatVideo, ShelfID: shelf.ID,
	})
	fileItem(t, svc, library.Item{
		Title: "Thinking in Systems", Why: "has one", Format: library.FormatBook,
		ShelfID: shelf.ID, CoverURL: "https://covers.example/1.jpg",
	})

	body := get(t, h, "/shelves/"+shelf.ID).Body.String()
	for _, want := range []string{
		`<span class="entry-plate gen-cover gc-5" aria-hidden="true">`,
		`<span class="gc-author">Frank Herbert</span>`,
		`<span class="gc-title">Dune</span>`,
		`<span class="gc-publisher">Chilton</span>`,
		`<span class="entry-plate gen-cover gc-7" aria-hidden="true">`,
		`<span class="gc-title">Invisible Cities</span>`,
		`<use href="#gc-frame-full"/>`,
		`<use href="#gc-frame-simple"/>`,
		`<span class="entry-plate cloth-video" aria-hidden="true">`,
		`<symbol id="gc-frame-full"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shelf missing %q", want)
		}
	}
	if n := strings.Count(body, "entry-plate gen-cover"); n != 2 {
		t.Errorf("%d generated covers, want 2: the coverless books only", n)
	}
	if n := strings.Count(body, `class="gc-publisher"`); n != 1 {
		t.Errorf("%d publisher lines, want 1: Invisible Cities has no publisher", n)
	}

	// Removing the cover draws the generated one, not a blank plate.
	removed := fileItem(t, svc, library.Item{
		Title: "Ulysses", Why: "removed", Format: library.FormatBook, ShelfID: shelf.ID,
		CoverURL: "https://covers.example/2.jpg",
	})
	if _, err := svc.SetCover(ctx, removed.ID, library.CoverRemoved, nil); err != nil {
		t.Fatal(err)
	}
	page := get(t, h, "/items/"+removed.ID).Body.String()
	if !strings.Contains(page, `<span class="entry-plate gen-cover gc-1" aria-hidden="true">`) {
		t.Error("a removed book cover must draw the generated cover")
	}
}
