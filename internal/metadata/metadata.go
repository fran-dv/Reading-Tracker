// Package metadata looks things up for capture (spec §4): a web page's title
// and length, a YouTube video's title and channel, and Open Library book
// candidates. It knows nothing about the library, templates, or handlers.
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
	"strconv"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/isbn"
)

const (
	userAgent    = "readingqueue (+https://github.com/fran-dv/reading-tracker)"
	maxBody      = 5 << 20 // bytes read from any response; the rest is ignored
	timeout      = 10 * time.Second
	maxRedirects = 5
	searchLimit  = 20 // per source; PR 29b adds Google Books alongside
)

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

// Book is one Open Library candidate. Title is always the work's own title,
// never an edition's: an edition's title may be a translation. Zero Year or
// Pages means unknown. CoverURL is the large image worth keeping; ThumbURL
// is a small one for listing candidates. Both are empty when the book has
// no cover.
//
// Publisher and ISBN come from the work's single best-matching edition (see
// searchFields); ISBN is normalized to ISBN-13, preferring a source that was
// already 13 digits, and is "" when no candidate validates. ISBNs holds
// every ISBN this result is known by (the work's and the edition's),
// normalized and deduplicated; it is kept only so PR 29b's merge can
// recognise the same book found by Google Books, and is never shown.
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
// answer. Unanswered stays empty until PR 29b adds a second source; a total
// failure of the only source today is still returned as an error, as before.
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

// SearchBooks asks Open Library for books matching query, by relevance
// (spec §4) rather than a title-only match. Search.More says how many
// further matches Open Library reports beyond what is fetched, so "keep
// typing" is never confused with "there is nothing more" (SPEC §0).
func (c *Client) SearchBooks(ctx context.Context, query string) (Search, error) {
	books, numFound, err := c.openLibrarySearch(ctx, query)
	if err != nil {
		return Search{}, err
	}
	return Search{Books: books, More: max(numFound-len(books), 0)}, nil
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
		"q":      {strings.TrimSpace(query)},
		"limit":  {fmt.Sprint(searchLimit)},
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
	b := Book{Key: d.Key, Title: d.Title, Year: d.Year, Pages: d.Pages, ISBNs: knownISBNs(d.ISBN)}
	if len(d.AuthorName) > 0 {
		b.Author = d.AuthorName[0]
	}
	if d.CoverID > 0 {
		b.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", d.CoverID)
		b.ThumbURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", d.CoverID)
	}
	if len(d.Editions.Docs) == 0 {
		return b
	}
	ed := d.Editions.Docs[0]
	if len(ed.Publisher) > 0 {
		b.Publisher = ed.Publisher[0]
	}
	b.ISBN = bestISBN(ed.ISBN)
	b.ISBNs = knownISBNs(append(append([]string{}, d.ISBN...), ed.ISBN...))
	if year := yearFromDates.FindString(strings.Join(ed.PublishDate, " ")); year != "" {
		b.Year, _ = strconv.Atoi(year)
	}
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
