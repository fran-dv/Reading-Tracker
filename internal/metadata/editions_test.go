package metadata

import (
	"net/http"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/isbn"
)

// TestEditionsMergedAcrossSources covers Edition Candidate Sourcing:
// editions from both sources with covers merge and deduplicate, and the
// one edition.json entry with no cover is excluded.
func TestEditionsMergedAcrossSources(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /works/OL893414W/editions.json", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("editions limit = %q, want 50", got)
		}
		http.ServeFile(w, r, "testdata/editions.json")
	})
	mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/googlebooks_editions.json")
	})
	got := newTestClient(t, mux).Editions(ctx, "/works/OL893414W", "Dune", "Frank Herbert")

	if len(got.Unanswered) != 0 {
		t.Fatalf("Unanswered = %v, want none", got.Unanswered)
	}
	if len(got.Books) != 4 {
		t.Fatalf("got %d candidates, want 4 (the cover-less fourth edition excluded): %+v", len(got.Books), got.Books)
	}

	// The ISBN-10-only edition's ISBN-13 is computed, not hard-coded, so
	// this test does not duplicate isbn.Normalize's own arithmetic.
	wantISBN13, ok := isbn.Normalize("0441176208")
	if !ok {
		t.Fatal("test fixture's own ISBN-10 does not validate")
	}

	cases := []struct {
		publisher string
		pages     int
		isbn13    string
		language  string
	}{
		{"Ace Books", 412, "9780441013593", "English"},                // merged with Google's duplicate sharing its ISBN
		{"Ace Books, Inc.", 0, wantISBN13, ""},                        // ISBN-10 only; no language on this entry
		{"Nemira Publishing House", 734, "9786064312617", "Romanian"}, // a translated edition, own cover
		{"Ace Books", 896, "9780593098233", ""},                       // Google-only, follows every Open Library candidate
	}
	for i, c := range cases {
		b := got.Books[i]
		if b.Publisher != c.publisher || b.Pages != c.pages || b.ISBN != c.isbn13 || b.Language != c.language {
			t.Errorf("book %d = %+v, want publisher=%q pages=%d isbn=%q language=%q",
				i, b, c.publisher, c.pages, c.isbn13, c.language)
		}
		if b.CoverURL == "" {
			t.Errorf("book %d has no cover; every candidate here must carry one", i)
		}
	}
	for _, b := range got.Books {
		if b.Publisher == "No Cover Press" {
			t.Fatalf("the cover-less edition was not excluded: %+v", b)
		}
	}
}

// TestEditionsResolvesWorkKeyWhenMissing covers the "work-key resolution
// from title+author when none is supplied" scenario: a filed item that
// never stored a work key still reaches the same editions.
func TestEditionsResolvesWorkKeyWhenMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "Dune Frank Herbert" || q.Get("limit") != "1" || q.Get("fields") != "key" {
			t.Errorf("unexpected resolution query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"numFound":1,"docs":[{"key":"/works/OL893414W"}]}`))
	})
	mux.HandleFunc("GET /works/OL893414W/editions.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/editions.json")
	})
	mux.HandleFunc("GET /books/v1/volumes", emptyGoogleBooks)

	got := newTestClient(t, mux).Editions(ctx, "", "Dune", "Frank Herbert")
	if len(got.Books) == 0 {
		t.Fatal("got no candidates after resolving the work key, want the editions fixture's covered entries")
	}
	if got.Books[0].Key != "/works/OL893414W" {
		t.Errorf("Books[0].Key = %q, want the resolved work key", got.Books[0].Key)
	}
}

// TestEditionsResolutionFailureDegradesGracefully: a failed or empty
// resolution is not a disclosed source failure (Open Library answered;
// it just found nothing to resolve to), and leaves the picker merely
// empty rather than erroring.
func TestEditionsResolutionFailureDegradesGracefully(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", http.NotFound)
	mux.HandleFunc("GET /books/v1/volumes", emptyGoogleBooks)

	got := newTestClient(t, mux).Editions(ctx, "", "Unknown Book", "Nobody")
	if len(got.Books) != 0 {
		t.Fatalf("got %+v, want no candidates", got.Books)
	}
	if len(got.Unanswered) != 0 {
		t.Fatalf("Unanswered = %v, want none", got.Unanswered)
	}
}

// TestEditionsRejectsAMalformedWorkKey: a workKey that is not a plain
// "/works/OL123W" shape must never reach the editions.json request URL
// unchecked — a value starting "@host" would otherwise redirect the
// request to a different host, and one carrying "?" or "#" would break
// the request path.
func TestEditionsRejectsAMalformedWorkKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request for a malformed work key: %s", r.URL)
	})
	mux.HandleFunc("GET /books/v1/volumes", emptyGoogleBooks)

	for _, bad := range []string{"@evil.example/works/OL1W", "/works/OL1W?x=1", "/works/OL1W#frag", "not-a-key"} {
		got := newTestClient(t, mux).Editions(ctx, bad, "", "")
		if len(got.Books) != 0 {
			t.Errorf("workKey %q: got %+v, want no candidates", bad, got.Books)
		}
	}
}

func TestWithCover(t *testing.T) {
	if got := withCover(nil); got != nil {
		t.Errorf("withCover(nil) = %v, want nil", got)
	}
	got := withCover([]Book{{Title: "has one", CoverURL: "x"}, {Title: "has none"}})
	if len(got) != 1 || got[0].Title != "has one" {
		t.Fatalf("withCover = %+v, want only the covered book", got)
	}
}

func TestEditionsQuery(t *testing.T) {
	cases := []struct {
		title, author, want string
	}{
		{"Dune", "Frank Herbert", `intitle:"Dune" inauthor:"Frank Herbert"`},
		{"Dune", "", `intitle:"Dune"`},
		{"Dune", "  ", `intitle:"Dune"`},
	}
	for _, c := range cases {
		if got := editionsQuery(c.title, c.author); got != c.want {
			t.Errorf("editionsQuery(%q, %q) = %q, want %q", c.title, c.author, got, c.want)
		}
	}
}

func TestLanguageName(t *testing.T) {
	cases := []struct{ key, want string }{
		{"/languages/eng", "English"},
		{"/languages/rum", "Romanian"},
		{"/languages/xyz", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := languageName(c.key); got != c.want {
			t.Errorf("languageName(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}
