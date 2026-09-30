package metadata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()

// rewrite sends every request to the test server, keeping the path and query,
// so production code keeps its real hosts and gains no test hooks.
type rewrite struct{ target *url.URL }

func (rw rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.URL.Scheme, out.URL.Host = rw.target.Scheme, rw.target.Host
	resp, err := http.DefaultTransport.RoundTrip(out)
	if resp != nil {
		resp.Request = req // as a real transport would: callers see the URL they asked for
	}
	return resp, err
}

// newTestClient serves mux behind a keyless Client whose requests all land
// on it.
func newTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	return newTestClientWithKey(t, mux, "")
}

func newTestClientWithKey(t *testing.T, mux *http.ServeMux, googleBooksKey string) *Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	return New(rewrite{target}, googleBooksKey)
}

// emptyGoogleBooks answers Google Books' volumes.list with a valid,
// zero-result payload, for tests only interested in Open Library.
func emptyGoogleBooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"totalItems":0,"items":[]}`))
}

func TestSearchBooksMerged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "go programming" || q.Get("title") != "" || q.Get("limit") != "20" ||
			!strings.Contains(q.Get("fields"), "cover_i") || !strings.Contains(q.Get("fields"), "editions.isbn") ||
			!strings.Contains(q.Get("fields"), "editions.publish_date") {
			t.Errorf("unexpected Open Library query: %s", r.URL.RawQuery)
		}
		if r.UserAgent() != userAgent {
			t.Errorf("user agent = %q", r.UserAgent())
		}
		http.ServeFile(w, r, "testdata/search.json")
	})
	mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "go programming" || q.Get("maxResults") != "10" ||
			!strings.Contains(q.Get("fields"), "imageLinks") || !strings.Contains(q.Get("fields"), "industryIdentifiers") {
			t.Errorf("unexpected Google Books query: %s", r.URL.RawQuery)
		}
		http.ServeFile(w, r, "testdata/googlebooks_search.json")
	})
	got := newTestClient(t, mux).SearchBooks(ctx, "  go programming ")
	if len(got.Unanswered) != 0 {
		t.Fatalf("Unanswered = %v, want none: both sources answered", got.Unanswered)
	}
	want := Search{More: 18, Books: []Book{
		{
			// Shares an ISBN with a Google Books result: merged, but every
			// field is already Open Library's own, so nothing changes
			// (Result Merge and Deduplication, "fills a field" only
			// applies to what the leading result lacks).
			Key: "/works/OL893415W", Title: "The Go Programming Language", Author: "Alan A. A. Donovan",
			Year: 2015, Pages: 380, Publisher: "Addison-Wesley Professional", ISBN: "9780134190440",
			ISBNs:    []string{"9780134190440"},
			CoverURL: "https://covers.openlibrary.org/b/id/8231856-L.jpg",
			ThumbURL: "https://covers.openlibrary.org/b/id/8231856-M.jpg",
		},
		{
			Key: "/works/OL15168215W", Title: "Go in Action", Author: "William Kennedy", Year: 2015,
			ISBNs: []string{"9781617291784"},
		},
		{
			// Google-only, follows every Open Library result. Its thumbnail
			// links were http://; both are upgraded to https (Thumbnail URL
			// Upgrade), and the cover link's zoom is raised for a bigger
			// image than the thumbnail (task 2.1 finding, googlebooks.go).
			Title: "Learning Go", Author: "Jon Bodner", Year: 2021, Pages: 375, Publisher: "O'Reilly Media",
			ISBN: "9781492077213", ISBNs: []string{"9781492077213"},
			CoverURL: "https://books.google.com/books/content?id=9nNGEAAAQBAJ&img=1&printsec=frontcover&source=gbs_api&zoom=0",
			ThumbURL: "https://books.google.com/books/content?id=9nNGEAAAQBAJ&img=1&printsec=frontcover&source=gbs_api&zoom=1",
		},
		{
			// No cover offered by Google for this one: CoverURL/ThumbURL
			// stay empty rather than a guessed placeholder link (task 2.1
			// finding: no such placeholder is documented or was observed).
			Title: "Go Web Programming", Author: "Sau Sheong Chang", Year: 2016, Pages: 300,
			Publisher: "Manning Publications", ISBN: "9781617292569", ISBNs: []string{"9781617292569"},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Every match Open Library reports is already fetched and shown: More is 0.
func TestSearchBooksAllShown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/search_all_shown.json")
	})
	mux.HandleFunc("GET /books/v1/volumes", emptyGoogleBooks)
	got := newTestClient(t, mux).SearchBooks(ctx, "dune herbert")
	if got.More != 0 {
		t.Errorf("More = %d, want 0", got.More)
	}
	if len(got.Books) != 1 {
		t.Fatalf("got %d books, want 1", len(got.Books))
	}
	b := got.Books[0]
	// The best edition (Berkley, 1978) wins over the work's own huge
	// publisher list and over the work's first_publish_year (1965).
	if b.Publisher != "Berkley" || b.Year != 1978 {
		t.Errorf("got publisher=%q year=%d, want Berkley / 1978", b.Publisher, b.Year)
	}
	if b.ISBN != "9780425038185" {
		t.Errorf("got isbn=%q, want the edition ISBN normalized to ISBN-13", b.ISBN)
	}
	wantISBNs := []string{"9780425038918", "9780441172719", "9780425038185"}
	if !reflect.DeepEqual(b.ISBNs, wantISBNs) {
		t.Errorf("got ISBNs=%v, want %v", b.ISBNs, wantISBNs)
	}
}

// TestSearchBooksDegradation covers Graceful Single-Source Degradation: no
// combination of one or both sources failing ever turns into an error, or
// blocks the other source's results.
func TestSearchBooksDegradation(t *testing.T) {
	ol := func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "testdata/search_all_shown.json") }

	t.Run("Google quota refusal (429)", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /search.json", ol)
		mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`))
		})
		got := newTestClient(t, mux).SearchBooks(ctx, "dune herbert")
		if !reflect.DeepEqual(got.Unanswered, []string{"Google Books"}) {
			t.Fatalf("Unanswered = %v, want [Google Books]", got.Unanswered)
		}
		if len(got.Books) != 1 || got.Books[0].Title != "Dune" {
			t.Fatalf("want Open Library's result alone, got %+v", got.Books)
		}
	})

	t.Run("Google network timeout", func(t *testing.T) {
		old := searchDeadline
		// Short enough to keep the test fast, generous enough that Open
		// Library's own (local, immediate) response isn't at risk of also
		// missing the deadline on a loaded machine.
		searchDeadline = 250 * time.Millisecond
		defer func() { searchDeadline = old }()

		mux := http.NewServeMux()
		mux.HandleFunc("GET /search.json", ol)
		mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
				w.Write([]byte(`{"totalItems":0,"items":[]}`))
			}
		})
		got := newTestClient(t, mux).SearchBooks(ctx, "dune herbert")
		if !reflect.DeepEqual(got.Unanswered, []string{"Google Books"}) {
			t.Fatalf("Unanswered = %v, want [Google Books]", got.Unanswered)
		}
		if len(got.Books) != 1 {
			t.Fatalf("want Open Library's result alone despite Google's silence, got %+v", got.Books)
		}
	})

	t.Run("Open Library failure, Google Books available", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /search.json", http.NotFound)
		mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "testdata/googlebooks_search.json")
		})
		got := newTestClient(t, mux).SearchBooks(ctx, "go programming")
		if !reflect.DeepEqual(got.Unanswered, []string{"Open Library"}) {
			t.Fatalf("Unanswered = %v, want [Open Library]", got.Unanswered)
		}
		if len(got.Books) != 3 {
			t.Fatalf("want every Google Books result alone, got %d: %+v", len(got.Books), got.Books)
		}
	})

	t.Run("both sources fail", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /search.json", http.NotFound)
		mux.HandleFunc("GET /books/v1/volumes", http.NotFound)
		got := newTestClient(t, mux).SearchBooks(ctx, "go")
		if len(got.Books) != 0 {
			t.Errorf("Books = %+v, want none", got.Books)
		}
		if len(got.Unanswered) != 2 {
			t.Fatalf("Unanswered = %v, want both sources named", got.Unanswered)
		}
	})
}

// ISBN Lookup Across Sources: a complete, valid ISBN is looked up directly
// (search.json?isbn=, q=isbn:…), not by relevance, through the same merge
// and degradation path as SearchBooks (design.md ADR-1, ADR-5).
func TestLookupISBNFoundOnOneSource(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("isbn") != "9780425038185" || q.Get("q") != "" {
			t.Errorf("unexpected Open Library query: %s", r.URL.RawQuery)
		}
		http.ServeFile(w, r, "testdata/search_all_shown.json")
	})
	mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "isbn:9780425038185" {
			t.Errorf("unexpected Google Books query: %q", got)
		}
		w.Write([]byte(`{"items":[]}`))
	})
	got := newTestClient(t, mux).LookupISBN(ctx, "9780425038185")
	if len(got.Unanswered) != 0 {
		t.Fatalf("Unanswered = %v, want none: Open Library answered", got.Unanswered)
	}
	if len(got.Books) != 1 || got.Books[0].Title != "Dune" {
		t.Fatalf("got %+v, want the Open Library edition alone", got.Books)
	}
}

// An ISBN neither source recognizes is not an error: an empty Search, same
// as any other search nobody could answer with a match.
func TestLookupISBNNoMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"numFound":0,"docs":[]}`))
	})
	mux.HandleFunc("GET /books/v1/volumes", emptyGoogleBooks)
	got := newTestClient(t, mux).LookupISBN(ctx, "9780000000002")
	if len(got.Books) != 0 {
		t.Errorf("Books = %+v, want none", got.Books)
	}
	if len(got.Unanswered) != 0 {
		t.Errorf("Unanswered = %v, want none: both sources answered, just found nothing", got.Unanswered)
	}
}

// TestGoogleBooksKey covers the owner's optional API key (read from an
// environment variable by cmd/readingqueue, never by this package): the
// request carries key= only when the Client was given one.
func TestGoogleBooksKey(t *testing.T) {
	var gotQuery url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"items":[]}`))
	})

	newTestClientWithKey(t, mux, "").SearchBooks(ctx, "dune herbert")
	if gotQuery.Has("key") {
		t.Errorf("keyless client sent key=%q, want no key parameter", gotQuery.Get("key"))
	}

	newTestClientWithKey(t, mux, "test-api-key-123").SearchBooks(ctx, "dune herbert")
	if got := gotQuery.Get("key"); got != "test-api-key-123" {
		t.Errorf("key = %q, want the configured key", got)
	}
}

// TestRedactedURLHidesKey proves a Google Books failure's error message
// never leaks the key: Never log the key applies to any error text built
// from the request URL, not just to explicit log calls. Both ways get can
// fail are covered: a non-2xx response, and a transport-level failure
// (here, a context deadline), since Go wraps the latter in a *url.Error
// that otherwise carries the full URL, key included.
func TestRedactedURLHidesKey(t *testing.T) {
	assertRedacted := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("want an error")
		}
		if strings.Contains(err.Error(), "super-secret-key") {
			t.Fatalf("error leaked the API key: %v", err)
		}
		if !strings.Contains(err.Error(), "key=REDACTED") {
			t.Errorf("error = %v, want the key parameter visibly redacted, not silently dropped", err)
		}
	}

	t.Run("non-2xx response", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /books/v1/volumes", http.NotFound)
		c := newTestClientWithKey(t, mux, "super-secret-key")
		_, _, err := c.googleBooks(ctx, "dune herbert")
		assertRedacted(t, err)
	})

	t.Run("transport failure (deadline exceeded)", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /books/v1/volumes", func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		})
		c := newTestClientWithKey(t, mux, "super-secret-key")
		deadline, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
		defer cancel()
		_, _, err := c.googleBooks(deadline, "dune herbert")
		assertRedacted(t, err)
	})
}

func TestGoogleImageURL(t *testing.T) {
	raw := "http://books.google.com/books/content?id=abc&printsec=frontcover&img=1&zoom=1&edge=curl&source=gbs_api"

	thumb := googleImageURL(raw, false)
	if !strings.HasPrefix(thumb, "https://") {
		t.Errorf("thumb = %q, want https", thumb)
	}
	if strings.Contains(thumb, "edge=") {
		t.Errorf("thumb = %q, still carries the page-curl decoration", thumb)
	}
	if !strings.Contains(thumb, "zoom=1") {
		t.Errorf("thumb = %q, want the original zoom kept", thumb)
	}

	cover := googleImageURL(raw, true)
	if !strings.Contains(cover, "zoom=0") {
		t.Errorf("cover = %q, want zoom raised to 0 for the larger image", cover)
	}
}

// The best edition's publisher and ISBN fill the result even when it has no
// publish_date of its own: Year then keeps the work's first_publish_year.
func TestBookFromDocEditionWithoutYear(t *testing.T) {
	var d olSearchDoc
	d.Title, d.Year = "Some Book", 1990
	d.Editions.Docs = []struct {
		Publisher   []string `json:"publisher"`
		ISBN        []string `json:"isbn"`
		PublishDate []string `json:"publish_date"`
	}{{Publisher: []string{"Reprint House"}, ISBN: []string{"0134190440"}}}

	got := bookFromDoc(d)
	want := Book{Title: "Some Book", Year: 1990, Publisher: "Reprint House", ISBN: "9780134190440",
		ISBNs: []string{"9780134190440"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBestISBN(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"prefers an already-13-digit source", []string{"0134190440", "9780134190440"}, "9780134190440"},
		{"falls back to a 10-digit source", []string{"0134190440"}, "9780134190440"},
		{"skips an invalid candidate", []string{"not an isbn", "0134190440"}, "9780134190440"},
		{"nothing valid", []string{"not an isbn"}, ""},
		{"empty", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := bestISBN(tc.in); got != tc.want {
				t.Errorf("bestISBN(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLookupYouTube(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oembed", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("url"), "f6kdp27TYZs") {
			t.Errorf("oembed url param = %q", r.URL.Query().Get("url"))
		}
		http.ServeFile(w, r, "testdata/oembed.json")
	})
	c := newTestClient(t, mux)
	for _, link := range []string{"https://www.youtube.com/watch?v=f6kdp27TYZs", "https://youtu.be/f6kdp27TYZs"} {
		res, err := c.Lookup(ctx, link)
		if err != nil {
			t.Fatal(err)
		}
		want := Result{Title: "Go Concurrency Patterns", Author: "Google for Developers", Format: "video",
			CoverURL: "https://i.ytimg.com/vi/f6kdp27TYZs/hqdefault.jpg"}
		if res != want {
			t.Errorf("%s: got %+v, want %+v", link, res, want)
		}
	}
}

func TestLookupArticle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /post", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/article.html")
	})
	mux.HandleFunc("GET /moved", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/post", http.StatusFound)
	})
	c := newTestClient(t, mux)

	res, err := c.Lookup(ctx, "https://blog.example/moved")
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Title: "Real Title", Author: "Jane Doe", Format: "article", WordCount: 10,
		CoverURL: "https://blog.example/img/cover.png"}
	if res != want {
		t.Fatalf("got %+v, want %+v", res, want)
	}
}

func TestExtractArticleFallbacks(t *testing.T) {
	base, _ := url.Parse("https://x.example/a")
	tests := []struct {
		name string
		html string
		want Result
	}{
		{"main when no article",
			`<title> Only  title </title><nav>no</nav><main><p>a b c</p></main><p>outside</p>`,
			Result{Title: "Only title", Format: "article", WordCount: 3}},
		{"body when no main",
			`<title>T</title><body><header>skip</header><p>a b</p><aside>no</aside><p>c d</p></body>`,
			Result{Title: "T", Format: "article", WordCount: 4}},
		{"article:author name used, profile URL ignored",
			`<meta property="article:author" content="Ann Lee"><p>x</p>`,
			Result{Author: "Ann Lee", Format: "article", WordCount: 1}},
		{"profile URL is no author",
			`<meta property="article:author" content="https://facebook.com/ann"><p>x</p>`,
			Result{Format: "article", WordCount: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractArticle(strings.NewReader(tc.html), base)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLookupEdgeCases(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /paper.pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.7"))
	})
	mux.HandleFunc("GET /gone", http.NotFound)
	mux.HandleFunc("GET /loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	mux.HandleFunc("GET /huge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<article>"))
		w.Write([]byte(strings.Repeat("word ", 2*maxBody/5))) // 10 MB of text
	})
	c := newTestClient(t, mux)

	if res, err := c.Lookup(ctx, "https://x.example/paper.pdf"); err != nil || res != (Result{}) {
		t.Errorf("non-HTML: got %+v, %v; want empty result, no error", res, err)
	}
	if _, err := c.Lookup(ctx, "https://x.example/gone"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: got %v, want status error", err)
	}
	if _, err := c.Lookup(ctx, "https://x.example/loop"); err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Errorf("redirect loop: got %v, want redirect cap error", err)
	}
	res, err := c.Lookup(ctx, "https://x.example/huge")
	if err != nil {
		t.Fatalf("huge page: %v", err)
	}
	if res.WordCount == 0 || res.WordCount > maxBody/5 {
		t.Errorf("huge page: %d words, want the body cap to stop reading at ~%d", res.WordCount, maxBody/5)
	}
	for _, bad := range []string{"", "not a url", "ftp://x.example/f", "https://", "/relative"} {
		if _, err := c.Lookup(ctx, bad); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("Lookup(%q) = %v, want ErrInvalidURL", bad, err)
		}
	}
}

func TestCountWords(t *testing.T) {
	tests := map[string]int{"": 0, "   ": 0, "one": 1, "  a\tb\nc  ": 3, "it's a well-known fact": 4}
	for in, want := range tests {
		if got := CountWords(in); got != want {
			t.Errorf("CountWords(%q) = %d, want %d", in, got, want)
		}
	}
}
