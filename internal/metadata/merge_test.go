package metadata

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		name      string
		ol, gb    []Book
		olTotal   int
		limit     int
		wantBooks []Book
		wantMore  int
	}{
		{
			name: "shared-ISBN duplicate merges to one result in OL-leading order",
			ol:   []Book{{Title: "Dune", Author: "Frank Herbert", ISBNs: []string{"9780441172719"}}},
			gb: []Book{
				{Title: "Something Else", Author: "Other Author", ISBNs: []string{"9780000000001"}},
				{Title: "Dune", Author: "Frank Herbert", ISBNs: []string{"9780441172719"}, CoverURL: "https://gb/dune.jpg"},
			},
			olTotal: 1, limit: 8,
			wantBooks: []Book{
				{Title: "Dune", Author: "Frank Herbert", ISBNs: []string{"9780441172719"}, CoverURL: "https://gb/dune.jpg"},
				{Title: "Something Else", Author: "Other Author", ISBNs: []string{"9780000000001"}},
			},
			wantMore: 0,
		},
		{
			name:    "no-shared-ISBN title+author duplicate merges",
			ol:      []Book{{Title: "Dune", Author: "Frank Herbert"}},
			gb:      []Book{{Title: "Dune", Author: "Frank Herbert", Pages: 412}},
			olTotal: 1, limit: 8,
			wantBooks: []Book{{Title: "Dune", Author: "Frank Herbert", Pages: 412}},
			wantMore:  0,
		},
		{
			name:    "merge fills a field the leading result lacks while keeping OL's lead and other fields",
			ol:      []Book{{Title: "Dune", Author: "Frank Herbert", Year: 1965}},
			gb:      []Book{{Title: "Dune", Author: "Frank Herbert", Year: 1990, CoverURL: "https://gb/dune.jpg", ThumbURL: "https://gb/dune-t.jpg"}},
			olTotal: 1, limit: 8,
			wantBooks: []Book{{Title: "Dune", Author: "Frank Herbert", Year: 1965, CoverURL: "https://gb/dune.jpg", ThumbURL: "https://gb/dune-t.jpg"}},
			wantMore:  0,
		},
		{
			name:    "distinct books with similar titles but different first-author surnames stay separate",
			ol:      []Book{{Title: "Dune", Author: "Frank Herbert"}},
			gb:      []Book{{Title: "Dune", Author: "Someone Else"}},
			olTotal: 1, limit: 8,
			wantBooks: []Book{{Title: "Dune", Author: "Frank Herbert"}, {Title: "Dune", Author: "Someone Else"}},
			wantMore:  0,
		},
		{
			name:    "subtitle-only difference still merges (colon-split rule)",
			ol:      []Book{{Title: "Example: A Long Subtitle", Author: "Ann Author", Pages: 300}},
			gb:      []Book{{Title: "Example", Author: "Ann Author"}},
			olTotal: 1, limit: 8,
			wantBooks: []Book{{Title: "Example: A Long Subtitle", Author: "Ann Author", Pages: 300}},
			wantMore:  0,
		},
		{
			name:    "Google-only result appears after every OL-led result",
			ol:      []Book{{Title: "Dune", Author: "Frank Herbert"}, {Title: "Dune Messiah", Author: "Frank Herbert"}},
			gb:      []Book{{Title: "Children of Dune", Author: "Frank Herbert"}},
			olTotal: 2, limit: 8,
			wantBooks: []Book{
				{Title: "Dune", Author: "Frank Herbert"},
				{Title: "Dune Messiah", Author: "Frank Herbert"},
				{Title: "Children of Dune", Author: "Frank Herbert"},
			},
			wantMore: 0,
		},
		{
			name:    "empty OL side",
			ol:      nil,
			gb:      []Book{{Title: "Only On Google", Author: "G. Author"}},
			olTotal: 0, limit: 8,
			wantBooks: []Book{{Title: "Only On Google", Author: "G. Author"}},
			wantMore:  0,
		},
		{
			name:    "empty Google side",
			ol:      []Book{{Title: "Only On OL", Author: "O. Author"}},
			gb:      nil,
			olTotal: 1, limit: 8,
			wantBooks: []Book{{Title: "Only On OL", Author: "O. Author"}},
			wantMore:  0,
		},
		{
			name:    "more arithmetic: exact count with results cut by limit",
			ol:      []Book{{Title: "A"}, {Title: "B"}, {Title: "C"}},
			gb:      []Book{{Title: "D"}, {Title: "E"}},
			olTotal: 20, limit: 4,
			wantBooks: []Book{{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}},
			wantMore:  18, // 20 (olTotal) + 2 (googleOnly fetched) - 4 (shown)
		},
		{
			name:    "more arithmetic: all shown",
			ol:      []Book{{Title: "A"}},
			gb:      []Book{{Title: "B"}},
			olTotal: 1, limit: 8,
			wantBooks: []Book{{Title: "A"}, {Title: "B"}},
			wantMore:  0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			books, more := merge(tc.ol, tc.gb, tc.olTotal, tc.limit)
			if !reflect.DeepEqual(books, tc.wantBooks) {
				t.Errorf("books = %+v, want %+v", books, tc.wantBooks)
			}
			if more != tc.wantMore {
				t.Errorf("more = %d, want %d", more, tc.wantMore)
			}
		})
	}
}

func TestFoldedMainTitle(t *testing.T) {
	tests := map[string]string{
		"Example: A Long Subtitle": "example",
		"Example":                  "example",
		"Dune!":                    "dune",
		"":                         "",
		"  ":                       "",
	}
	for in, want := range tests {
		if got := foldedMainTitle(in); got != want {
			t.Errorf("foldedMainTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFoldedSurname(t *testing.T) {
	tests := map[string]string{
		"Frank Herbert":      "herbert",
		"Alan A. A. Donovan": "donovan",
		"Herbert":            "herbert",
		"":                   "",
		// Accented letters are kept, not silently dropped: an ASCII-only
		// fold would turn "García Márquez" into "mrquez" (the á gone),
		// which could accidentally collide with an unrelated surname that
		// also loses an accented letter.
		"Gabriel García Márquez": "márquez",
	}
	for in, want := range tests {
		if got := foldedSurname(in); got != want {
			t.Errorf("foldedSurname(%q) = %q, want %q", in, got, want)
		}
	}
}
