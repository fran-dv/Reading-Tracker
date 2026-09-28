// Package metadata looks things up for capture (spec §4): a web page's title
// and length, a YouTube video's title and channel, and book candidates from
// Open Library and Google Books. It knows nothing about the library,
// templates, or handlers.
//
// Every lookup is best effort. Callers show the error as "fill in by hand"
// and never block saving on it.
package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fran-dv/reading-tracker/internal/isbn"
)

const (
	userAgent    = "readingqueue (+https://github.com/fran-dv/reading-tracker)"
	maxBody      = 5 << 20 // bytes read from any response; the rest is ignored
	timeout      = 10 * time.Second
	maxRedirects = 5
	searchLimit  = 20 // Open Library docs fetched per search; Google's own cap is googleMaxResults
	resultLimit  = 8  // merged results actually shown (design.md ADR-1)
)

// searchDeadline bounds how long SearchBooks and LookupISBN wait for Open
// Library and Google Books together (design.md ADR-1): capture never waits
// on a slow source. A var, not a const, so tests can shorten it rather than
// sleep for the real 4 seconds.
var searchDeadline = 4 * time.Second

// searchFields is what Open Library's search.json is asked to return per
// result: the plain work fields, plus editions sub-fields for the one
// best-matching edition Open Library itself picks (live-checked finding,
// see design.md ADR-1, "Publisher, ISBN and year on an Open Library work
// result"). That edition fills Publisher and ISBN; when it has a publish
// year, that year overrides the work's own first_publish_year for Year.
// The work's own publisher list is requested by neither: it names every
// edition ever published (dozens for a popular book) and cannot be
// attributed to one.
const searchFields = "key,title,author_name,first_publish_year,number_of_pages_median,cover_i,isbn," +
	"editions,editions.publisher,editions.isbn,editions.publish_date"

// yearFromDates finds the first 4-digit year in a free-form date string such
// as "1978", "Jan 1978", or "1978-01-15".
var yearFromDates = regexp.MustCompile(`\b(1[0-9]{3}|20[0-9]{2})\b`)

// ErrInvalidURL is returned by Lookup for anything that is not an absolute
// http or https URL.
var ErrInvalidURL = errors.New("not an http or https URL")

// Result is what a URL yields. Format uses the library's spelling: "article"
// or "video", or "" when the page type was not recognised.
type Result struct {
	Title     string
	Author    string
	Format    string
	CoverURL  string
	WordCount int // articles only; 0 when unknown
}

// Book is one search result, from Open Library or from Google Books
// (merge.go combines both into one list). For an Open Library result, Title
// is always the work's own title, never an edition's: an edition's title
// may be a translation. Zero Year or Pages means unknown. CoverURL is the
// large image worth keeping; ThumbURL is a small one for listing
// candidates. Both are empty when the book has no cover.
//
// Publisher and ISBN come from the work's single best-matching edition (see
// searchFields) for an Open Library result, or from Google's own fields for
// a Google Books one; ISBN is normalized to ISBN-13, preferring a source
// that was already 13 digits, and is "" when no candidate validates. ISBNs
// holds every ISBN this result is known by, normalized and deduplicated;
// it is kept only so merge (merge.go) can recognise the same book found by
// the other source, and is never shown.
type Book struct {
	Key       string // Open Library work key, e.g. "/works/OL27448W"; "" for a non-Open-Library result
	Title     string
	Author    string
	CoverURL  string
	ThumbURL  string
	Year      int
	Pages     int
	Publisher string
	ISBN      string
	ISBNs     []string
}

// Search is the outcome of a book search or ISBN lookup: what was found, how
// many further matches were not shown, and which source (if any) failed to
// answer. A search nobody answered is not an error: it is a Search with
// both names in Unanswered and an empty Books, exactly what the page must
// say (design.md ADR-1).
type Search struct {
	Books      []Book
	More       int      // matches reported beyond what is shown; never negative
	Unanswered []string // source names that failed or timed out, e.g. "Open Library"
}

// Client performs lookups over HTTP.
type Client struct {
	http *http.Client
}

// New returns a Client with the timeout, redirect cap and body cap applied.
// A nil transport means http.DefaultTransport; tests pass their own.
func New(transport http.RoundTripper) *Client {
	return &Client{http: &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}}
}

// Lookup fetches what it can about a URL. YouTube links go through oEmbed;
// any other HTML page is read as an article; other content yields an empty
// Result and no error.
func (c *Client) Lookup(ctx context.Context, rawURL string) (Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Result{}, ErrInvalidURL
	}
	if isYouTube(u.Hostname()) {
		return c.video(ctx, u)
	}
	return c.page(ctx, u)
}

func isYouTube(host string) bool {
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
		return true
	}
	return false
}

// video uses oEmbed: title and channel, no duration (entered by hand).
func (c *Client) video(ctx context.Context, u *url.URL) (Result, error) {
	var out struct {
		Title        string `json:"title"`
		AuthorName   string `json:"author_name"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	endpoint := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(u.String())
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return Result{}, err
	}
	return Result{Title: out.Title, Author: out.AuthorName, Format: "video", CoverURL: out.ThumbnailURL}, nil
}

func (c *Client) page(ctx context.Context, u *url.URL) (Result, error) {
	resp, err := c.get(ctx, u.String())
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "text/html" {
		return Result{}, nil
	}
	// The final URL after redirects is the base for relative image links.
	return extractArticle(io.LimitReader(resp.Body, maxBody), resp.Request.URL)
}

// SearchBooks asks Open Library and Google Books for books matching query,
// by relevance (spec §4) rather than a title-only match, and merges what
// each found (design.md ADR-1). Neither source's failure blocks the other:
// a source that errors, is refused, or has not answered by searchDeadline
// is named in Search.Unanswered rather than failing the whole search, so
// capture stays usable for manual entry regardless (Graceful Single-Source
// Degradation).
func (c *Client) SearchBooks(ctx context.Context, query string) Search {
	q := strings.TrimSpace(query)
	return c.mergedSearch(ctx,
		func(ctx context.Context) ([]Book, int, error) { return c.openLibrarySearch(ctx, q) },
		func(ctx context.Context) ([]Book, int, error) { return c.googleBooks(ctx, q) },
	)
}

// mergedSearch runs ol and gb concurrently under one shared deadline and
// merges whichever answered in time (design.md ADR-1). Each function is one
// source's private query method, already bound to what it is looking for.
func (c *Client) mergedSearch(ctx context.Context, ol, gb func(context.Context) ([]Book, int, error)) Search {
	ctx, cancel := context.WithTimeout(ctx, searchDeadline)
	defer cancel()

	var olBooks, gbBooks []Book
	var olTotal int
	var olErr, gbErr error

	var wg sync.WaitGroup
	wg.Go(func() { olBooks, olTotal, olErr = ol(ctx) })
	wg.Go(func() { gbBooks, _, gbErr = gb(ctx) })
	wg.Wait()

	var unanswered []string
	if olErr != nil {
		olBooks, olTotal = nil, 0
		unanswered = append(unanswered, "Open Library")
	}
	if gbErr != nil {
		gbBooks = nil
		unanswered = append(unanswered, "Google Books")
	}

	books, more := merge(olBooks, gbBooks, olTotal, resultLimit)
	return Search{Books: books, More: more, Unanswered: unanswered}
}

// olSearchDoc is one Open Library search.json result, including the single
// best-matching edition asked for by searchFields.
type olSearchDoc struct {
	Key        string   `json:"key"`
	Title      string   `json:"title"`
	AuthorName []string `json:"author_name"`
	Year       int      `json:"first_publish_year"`
	Pages      int      `json:"number_of_pages_median"`
	CoverID    int      `json:"cover_i"`
	ISBN       []string `json:"isbn"`
	Editions   struct {
		Docs []struct {
			Publisher   []string `json:"publisher"`
			ISBN        []string `json:"isbn"`
			PublishDate []string `json:"publish_date"`
		} `json:"docs"`
	} `json:"editions"`
}

// openLibrarySearch queries search.json by relevance and returns the books
// it found plus the total Open Library reports matching, independent of how
// many were fetched.
func (c *Client) openLibrarySearch(ctx context.Context, query string) ([]Book, int, error) {
	params := url.Values{
		"q":      {query}, // already trimmed by SearchBooks, the method's only caller
		"limit":  {strconv.Itoa(searchLimit)},
		"fields": {searchFields},
	}
	var out struct {
		NumFound int           `json:"numFound"`
		Docs     []olSearchDoc `json:"docs"`
	}
	if err := c.getJSON(ctx, "https://openlibrary.org/search.json?"+params.Encode(), &out); err != nil {
		return nil, 0, err
	}
	books := make([]Book, 0, len(out.Docs))
	for _, d := range out.Docs {
		books = append(books, bookFromDoc(d))
	}
	return books, out.NumFound, nil
}

// bookFromDoc fills a Book from a work-level result and its best edition.
func bookFromDoc(d olSearchDoc) Book {
	b := Book{Key: d.Key, Title: d.Title, Year: d.Year, Pages: d.Pages}
	if len(d.AuthorName) > 0 {
		b.Author = d.AuthorName[0]
	}
	if d.CoverID > 0 {
		b.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", d.CoverID)
		b.ThumbURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", d.CoverID)
	}
	editionISBNs := d.ISBN
	if len(d.Editions.Docs) > 0 {
		ed := d.Editions.Docs[0]
		if len(ed.Publisher) > 0 {
			b.Publisher = ed.Publisher[0]
		}
		b.ISBN = bestISBN(ed.ISBN)
		editionISBNs = slices.Concat(d.ISBN, ed.ISBN)
		if year := yearFromDates.FindString(strings.Join(ed.PublishDate, " ")); year != "" {
			b.Year, _ = strconv.Atoi(year)
		}
	}
	b.ISBNs = knownISBNs(editionISBNs)
	return b
}

// stripISBNPunctuation drops the hyphens and spaces an ISBN source may
// carry, to measure its own digit count (used by bestISBN below); actual
// validation still goes through isbn.Normalize.
var stripISBNPunctuation = strings.NewReplacer("-", "", " ", "").Replace

// bestISBN picks the one ISBN that fills the form: a candidate that was
// already 13 digits at the source is preferred, since it needs no
// conversion. Returns "" when nothing in candidates validates.
func bestISBN(candidates []string) string {
	var tenDigit string
	for _, c := range candidates {
		norm, ok := isbn.Normalize(c)
		if !ok {
			continue
		}
		if len(stripISBNPunctuation(c)) == 13 {
			return norm
		}
		if tenDigit == "" {
			tenDigit = norm
		}
	}
	return tenDigit
}

// knownISBNs normalizes and deduplicates every ISBN a result carries.
func knownISBNs(raw []string) []string {
	seen := make(map[string]bool, len(raw))
	var out []string
	for _, s := range raw {
		norm, ok := isbn.Normalize(s)
		if !ok || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}

// CountWords counts whitespace-separated words, as used for article size.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

// get performs a GET with the user agent set and fails on non-2xx statuses.
// The caller closes the body.
func (c *Client) get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", req.URL.Redacted(), resp.Status)
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	resp, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(v)
}
