package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/metadata"
)

func TestGetBooks(t *testing.T) {
	meta := &fakeMeta{more: 12, books: []metadata.Book{
		{Title: `Go <in> "Action"`, Author: "William Kennedy", Year: 2015},
		{Title: "The Go Programming Language", Author: "Alan Donovan", Pages: 380, Publisher: "Fasttrack Press",
			ISBN: "9780134190440", CoverURL: "https://covers.example/1-L.jpg", ThumbURL: "https://covers.example/1-M.jpg"},
	}}
	h, _ := newTestServer(t, meta)

	rec := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"})
	body := rec.Body.String()
	if meta.queried != "go pro" {
		t.Errorf("searched %q", meta.queried)
	}
	for _, want := range []string{
		`id="search-results"`,
		`data-title="Go &lt;in&gt; &#34;Action&#34;"`,
		`data-pages="380"`,
		`data-cover="https://covers.example/1-L.jpg"`,
		`data-publisher="Fasttrack Press"`,
		`data-isbn="9780134190440"`,
		`src="https://covers.example/1-M.jpg"`,
		`data-pages=""`,
		"380 pages",
		"Fasttrack Press",
		"12 more not shown",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("results missing %q:\n%s", want, body)
		}
	}
	if sig := patchedSignals(t, body); sig["_showResults"] != true {
		t.Errorf("results not shown: %v", sig)
	}

	for _, short := range []string{"go", "https://x.example/long-enough"} {
		if rec := send(t, h, http.MethodGet, "/books", map[string]string{"title": short}); rec.Code != http.StatusNoContent {
			t.Errorf("%q: status %d, want 204", short, rec.Code)
		}
	}
}

// Every match is already shown: no "more not shown" line.
func TestGetBooksNoMoreLine(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{{Title: "Go in Action", Author: "William Kennedy"}}}
	h, _ := newTestServer(t, meta)
	body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go in action"}).Body.String()
	if strings.Contains(body, "more not shown") {
		t.Errorf("no more matches, but a 'more not shown' line rendered:\n%s", body)
	}
}

// One source down, the other still answers with results: the disclosure
// line names the one that answered, alongside the results themselves.
func TestGetBooksOneSourceSilent(t *testing.T) {
	meta := &fakeMeta{
		books:      []metadata.Book{{Title: "Go in Action", Author: "William Kennedy"}},
		unanswered: []string{"Google Books"},
	}
	h, _ := newTestServer(t, meta)
	body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go in action"}).Body.String()
	if !strings.Contains(body, "Only Open Library answered.") {
		t.Errorf("missing source disclosure:\n%s", body)
	}
	if !strings.Contains(body, "Go in Action") {
		t.Errorf("the answering source's results should still render:\n%s", body)
	}
}

// Both sources answered: no disclosure line at all.
func TestGetBooksBothAnswered(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{{Title: "Go in Action", Author: "William Kennedy"}}}
	h, _ := newTestServer(t, meta)
	body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go in action"}).Body.String()
	if strings.Contains(body, "answered.") {
		t.Errorf("both sources answered; no disclosure line should render:\n%s", body)
	}
}

// Zero matches, both sources having answered: the empty state names both.
func TestGetBooksEmptyNamesBothSources(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "no such book"}).Body.String()
	if !strings.Contains(body, "No matches on Open Library or Google Books.") {
		t.Errorf("empty state should name both sources:\n%s", body)
	}
}

// Neither source answered: the rubric failure line and Try again render,
// results stay shown (so the field can still be filled by hand), and the
// search re-runs on Try again.
func TestGetBooksBothFailed(t *testing.T) {
	meta := &fakeMeta{unanswered: []string{"Open Library", "Google Books"}}
	h, _ := newTestServer(t, meta)
	rec := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"})
	body := rec.Body.String()
	if !strings.Contains(body, "Neither Open Library nor Google Books answered. Type the details in.") {
		t.Errorf("missing failure line:\n%s", body)
	}
	if !strings.Contains(body, "Try again") {
		t.Errorf("missing Try again action:\n%s", body)
	}
	if sig := patchedSignals(t, body); sig["_showResults"] != true {
		t.Errorf("results (carrying the failure line) should still be shown: %v", sig)
	}
}

func TestSearchNote(t *testing.T) {
	tests := []struct {
		name       string
		unanswered []string
		want       string
	}{
		{"both answered", nil, ""},
		{"Google Books silent", []string{"Google Books"}, "Only Open Library answered."},
		{"Open Library silent", []string{"Open Library"}, "Only Google Books answered."},
		{"both silent", []string{"Open Library", "Google Books"}, "Neither Open Library nor Google Books answered. Type the details in."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := searchNote(tc.unanswered); got != tc.want {
				t.Errorf("searchNote(%v) = %q, want %q", tc.unanswered, got, tc.want)
			}
		})
	}
}
