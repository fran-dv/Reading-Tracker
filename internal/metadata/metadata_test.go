package metadata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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

// newTestClient serves mux behind a Client whose requests all land on it.
func newTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	return New(rewrite{target})
}

func TestSearchBooks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("title") != "go programming" || q.Get("limit") != "5" || !strings.Contains(q.Get("fields"), "cover_i") {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.UserAgent() != userAgent {
			t.Errorf("user agent = %q", r.UserAgent())
		}
		http.ServeFile(w, r, "testdata/search.json")
	})
	books, err := newTestClient(t, mux).SearchBooks(ctx, "  go programming ")
	if err != nil {
		t.Fatal(err)
	}
	want := []Book{
		{Title: "The Go Programming Language", Author: "Alan A. A. Donovan", Year: 2015, Pages: 380,
			CoverURL: "https://covers.openlibrary.org/b/id/8231856-M.jpg"},
		{Title: "Go in Action", Author: "William Kennedy", Year: 2015},
	}
	if len(books) != len(want) {
		t.Fatalf("got %d books, want %d", len(books), len(want))
	}
	for i := range want {
		if books[i] != want[i] {
			t.Errorf("book %d = %+v, want %+v", i, books[i], want[i])
		}
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
