package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// Keyboard Navigation Through Results (P1) and "None of These" (P7): the
// results carry the listbox/option roles and ids combobox.js and the
// picked-item click handlers need, and data-query for the pending guard
// (P4).
func TestGetBooksCombobox(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{
		{Title: "Go in Action", Author: "William Kennedy", ThumbURL: "https://covers.example/1-M.jpg"},
		{Title: "The Go Programming Language", Author: "Alan Donovan"},
	}}
	h, _ := newTestServer(t, meta)
	body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"}).Body.String()

	for _, want := range []string{
		`role="listbox"`,
		`data-query="go pro"`,
		`role="option" id="result-option-0"`,
		`role="option" id="result-option-1"`,
		`role="option" id="result-option-none"`,
		`class="result result-none"`,
		`None of these`,
		`alt="Cover of Go in Action"`,
		// "None of these" (P7): a signal-only change — confirms the typed
		// title, sets book, and moves focus to Author. Its runtime effect
		// (nothing server-rendered changes) is exercised by the task 3.10
		// manual pass; this only pins the markup that carries it.
		`data-on:click="$title = $title.trim(); $format = 'book'`,
		`document.getElementById('author').focus()`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("results missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<button") {
		t.Errorf("options must not carry their own interactive children:\n%s", body)
	}
}

// design.md ADR-4: a listbox's direct children must be role=option or
// role=presentation. The failure and hint rows carry a button (Try again)
// or are purely informational, so they must never render as bare options.
func TestGetBooksNonOptionRowsArePresentation(t *testing.T) {
	tests := []struct {
		name string
		meta *fakeMeta
	}{
		{"both sources failed", &fakeMeta{unanswered: []string{"Open Library", "Google Books"}}},
		{"empty, both answered", &fakeMeta{}},
		{"one source silent", &fakeMeta{books: []metadata.Book{{Title: "Go in Action"}}, unanswered: []string{"Google Books"}}},
		{"more not shown", &fakeMeta{more: 3, books: []metadata.Book{{Title: "Go in Action"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestServer(t, tc.meta)
			body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"}).Body.String()
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimPrefix(line, "data: elements ")
				if strings.Contains(line, "<li") && !strings.Contains(line, `role="option"`) &&
					!strings.Contains(line, `role="presentation"`) {
					t.Errorf("a listbox row must be an option or presentation: %q", line)
				}
			}
		})
	}
}

// Bugfix, 2026-10-04: the "no matches" and "Try again" placeholder rows
// that stand in for the options when .Books is empty must carry a stable
// id, the same one on both, distinct from any result-option-{n} or
// result-option-none id. Without it, Datastar's morph soft-matches this
// id-less row against the next render's first real option and, because
// that option carries data-preserve-attr="class", sticks the option with
// this row's own class ("hint" or "error result-note") instead of
// "result" — silently losing cursor:pointer (app.css) on the first
// result only, never the rest.
func TestGetBooksEmptyRowHasStableID(t *testing.T) {
	tests := []struct {
		name string
		meta *fakeMeta
	}{
		{"no matches, both answered", &fakeMeta{}},
		{"neither source answered", &fakeMeta{unanswered: []string{"Open Library", "Google Books"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestServer(t, tc.meta)
			body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"}).Body.String()
			if !strings.Contains(body, `id="result-empty"`) {
				t.Errorf("empty-state row missing a stable id:\n%s", body)
			}
			if strings.Contains(body, `id="result-option-0"`) {
				t.Errorf("no books, so there should be no result-option-0:\n%s", body)
			}
		})
	}
}

// Superseded Request Cancellation (P5), server side: a request whose
// context is already cancelled by the time the search returns writes
// nothing, so an aborted fetch never even reaches a stale patch.
func TestGetBooksCancelledContext(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{{Title: "Go in Action"}}}
	h, _ := newTestServer(t, meta)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	raw := `{"title":"go pro"}`
	req := httptest.NewRequest(http.MethodGet, "/books?datastar="+url.QueryEscape(raw), nil).WithContext(cancelled)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Body.Len() != 0 {
		t.Errorf("cancelled request should write nothing: %s", rec.Body.String())
	}
}

// Screen Reader and Focus for Search (P11): the live region announces the
// count, the "more not shown" count when present, and which sources
// answered — the same disclosure the visible list shows.
func TestGetBooksLiveRegion(t *testing.T) {
	tests := []struct {
		name string
		meta *fakeMeta
		want string
	}{
		{"one result, both answered", &fakeMeta{books: []metadata.Book{{Title: "Go in Action"}}}, "1 result"},
		{"several results with more", &fakeMeta{more: 12, books: []metadata.Book{{Title: "A"}, {Title: "B"}}}, "2 results, 12 more not shown"},
		{"one source silent", &fakeMeta{books: []metadata.Book{{Title: "A"}}, unanswered: []string{"Google Books"}},
			"1 result. Only Open Library answered."},
		{"both failed", &fakeMeta{unanswered: []string{"Open Library", "Google Books"}},
			"0 results. Neither Open Library nor Google Books answered. Type the details in."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestServer(t, tc.meta)
			body := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"}).Body.String()
			if sig := patchedSignals(t, body); sig["_searchStatus"] != tc.want {
				t.Errorf("_searchStatus = %v, want %q", sig["_searchStatus"], tc.want)
			}
		})
	}
}

func TestSearchAnnouncement(t *testing.T) {
	tests := []struct {
		name       string
		n, more    int
		unanswered []string
		want       string
	}{
		{"single result", 1, 0, nil, "1 result"},
		{"plural results", 3, 0, nil, "3 results"},
		{"zero results", 0, 0, nil, "0 results"},
		{"more not shown", 8, 12, nil, "8 results, 12 more not shown"},
		{"one source silent", 1, 0, []string{"Google Books"}, "1 result. Only Open Library answered."},
		{"both silent", 0, 0, []string{"Open Library", "Google Books"},
			"0 results. Neither Open Library nor Google Books answered. Type the details in."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := searchAnnouncement(tc.n, tc.more, tc.unanswered); got != tc.want {
				t.Errorf("searchAnnouncement(%d, %d, %v) = %q, want %q", tc.n, tc.more, tc.unanswered, got, tc.want)
			}
		})
	}
}

// Matched Word Highlighting (P6).
func TestMatchedTitle(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		title       string
		wantMatched bool // whether any word ends up set apart
	}{
		{"match on subtitle only", "deluxe edition", "Dune: The Deluxe Edition", true},
		{"match on canonical title, every word matches", "dune", "Dune", false},
		{"no word matches anywhere in the title", "xyz not present", "Some Other Book", false},
		{"empty query", "", "Dune", false},
		{"case and punctuation are ignored", "GARCÍA", "The García Case", true},
		{"an internal apostrophe stays inside its word", "don't", "Neuromancer: Don't Look Back", true},
		{"a hyphenated query word matches the title's own hyphen-split words", "spider-man", "The Amazing Spider-Man: No Way Home", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			segs := matchedTitle(tc.query, tc.title)
			var rebuilt string
			var anyMatched bool
			for _, s := range segs {
				rebuilt += s.Text
				anyMatched = anyMatched || s.Matched
			}
			if rebuilt != tc.title {
				t.Errorf("segments do not reconstruct the title: %q, want %q: %+v", rebuilt, tc.title, segs)
			}
			if anyMatched != tc.wantMatched {
				t.Errorf("anyMatched = %v, want %v: %+v", anyMatched, tc.wantMatched, segs)
			}
			// "Nothing set apart" comes back as exactly one unmatched
			// segment (book-search spec's "whole title treated as
			// matched" alternative): no bolding, so a full or a missing
			// match is never rendered as a diminished partial one.
			if !tc.wantMatched && (len(segs) != 1 || segs[0].Matched) {
				t.Errorf("a non-partial match must render as one plain segment: %+v", segs)
			}
			if tc.wantMatched && len(segs) < 2 {
				t.Errorf("a partial match must be split into more than one segment: %+v", segs)
			}
		})
	}
}

// ISBN Field Entry and Validation (P9): a checksum failure answers
// errors.isbn and looks nothing up.
func TestGetBooksISBNChecksumFailure(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{{Title: "Should not be reached"}}}
	h, _ := newTestServer(t, meta)
	rec := send(t, h, http.MethodGet, "/books", map[string]string{"isbn": "9780134190441"}) // bad check digit
	body := rec.Body.String()
	if sig := patchedSignals(t, body); sig["errors"] == nil {
		t.Fatalf("expected errors signals, got %s", body)
	} else if errs, _ := sig["errors"].(map[string]any); errs["isbn"] != "Those digits don't make an ISBN. Check them against the book." {
		t.Errorf("errors.isbn = %v, want the checksum message", errs["isbn"])
	}
	if meta.queried != "" {
		t.Errorf("a bad checksum must not look anything up, but queried %q", meta.queried)
	}
	if strings.Contains(body, "search-results") {
		t.Errorf("a checksum failure must not touch the results list:\n%s", body)
	}
}

// ISBN Lookup Across Sources: a complete, checksum-valid ISBN — hyphens
// and all — is normalized and looked up at once, no debounce.
func TestGetBooksISBNValid(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{{Title: "The Go Programming Language", Author: "Alan Donovan"}}}
	h, _ := newTestServer(t, meta)
	rec := send(t, h, http.MethodGet, "/books", map[string]string{"isbn": "978-0-13-419044-0"})
	body := rec.Body.String()
	if meta.queried != "9780134190440" {
		t.Errorf("looked up %q, want the normalized ISBN-13", meta.queried)
	}
	if !strings.Contains(body, "The Go Programming Language") {
		t.Errorf("results missing the found book:\n%s", body)
	}
	if !strings.Contains(body, `data-query="978-0-13-419044-0"`) {
		t.Errorf("data-query should carry the field's own raw value:\n%s", body)
	}
	if sig := patchedSignals(t, body); sig["_showResults"] != true {
		t.Errorf("results not shown: %v", sig)
	}
	// Matched Word Highlighting (P6) is a words-search idea; the ISBN's own
	// digits are not words to look for in a title, so nothing bolds here.
	if strings.Contains(body, `class="match"`) {
		t.Errorf("an ISBN result must not highlight matched words:\n%s", body)
	}
}

// Plain Failure States with Retry (P8), ISBN half: when both sources fail
// an ISBN lookup, Try again must re-run the ISBN lookup, not a words
// search (which — worse still — a blank title field would answer 204 to).
func TestGetBooksISBNTryAgain(t *testing.T) {
	meta := &fakeMeta{unanswered: []string{"Open Library", "Google Books"}}
	h, _ := newTestServer(t, meta)
	body := send(t, h, http.MethodGet, "/books", map[string]string{"isbn": "9780134190440"}).Body.String()
	if !strings.Contains(body, `data-on:click="@get('/books', {filterSignals: {include: /^isbn$/}})">Try again`) {
		t.Errorf("Try again should re-run the ISBN lookup:\n%s", body)
	}
}

// Superseded Request Cancellation (P5), server side, for the ISBN path:
// the same guarantee TestGetBooksCancelledContext proves for a words
// search.
func TestGetBooksISBNCancelledContext(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{{Title: "Go in Action"}}}
	h, _ := newTestServer(t, meta)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	raw := `{"isbn":"978-0-13-419044-0"}`
	req := httptest.NewRequest(http.MethodGet, "/books?datastar="+url.QueryEscape(raw), nil).WithContext(cancelled)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Body.Len() != 0 {
		t.Errorf("cancelled request should write nothing: %s", rec.Body.String())
	}
}

// "None of These" Path (ISBN scenario): an ISBN with no match on either
// source keeps the field (nothing to confirm as a title) and, on
// choosing to proceed, moves focus to Title rather than Author.
func TestGetBooksISBNNoMatch(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	body := send(t, h, http.MethodGet, "/books", map[string]string{"isbn": "9780134190440"}).Body.String()
	if !strings.Contains(body, `id="result-option-none"`) {
		t.Fatalf("expected a 'none of these' option even on no match:\n%s", body)
	}
	if !strings.Contains(body, "document.getElementById('title').focus()") {
		t.Errorf("an unmatched ISBN's 'none of these' should focus Title:\n%s", body)
	}
	if strings.Contains(body, "$title = $title.trim()") {
		t.Errorf("an unmatched ISBN's 'none of these' must not touch $title:\n%s", body)
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
