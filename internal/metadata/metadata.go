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
	"strings"
	"time"
)

const (
	userAgent    = "readingqueue (+https://github.com/fran-dv/reading-tracker)"
	maxBody      = 5 << 20 // bytes read from any response; the rest is ignored
	timeout      = 10 * time.Second
	maxRedirects = 5
	maxBooks     = 5
)

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

// Book is one Open Library candidate. Zero Year or Pages means unknown.
type Book struct {
	Title    string
	Author   string
	CoverURL string
	Year     int
	Pages    int
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

// SearchBooks asks Open Library for books whose title matches query.
func (c *Client) SearchBooks(ctx context.Context, query string) ([]Book, error) {
	params := url.Values{
		"title":  {strings.TrimSpace(query)},
		"limit":  {fmt.Sprint(maxBooks)},
		"fields": {"title,author_name,first_publish_year,number_of_pages_median,cover_i"},
	}
	var out struct {
		Docs []struct {
			Title      string   `json:"title"`
			AuthorName []string `json:"author_name"`
			Year       int      `json:"first_publish_year"`
			Pages      int      `json:"number_of_pages_median"`
			CoverID    int      `json:"cover_i"`
		} `json:"docs"`
	}
	if err := c.getJSON(ctx, "https://openlibrary.org/search.json?"+params.Encode(), &out); err != nil {
		return nil, err
	}
	books := make([]Book, 0, len(out.Docs))
	for _, d := range out.Docs {
		b := Book{Title: d.Title, Year: d.Year, Pages: d.Pages}
		if len(d.AuthorName) > 0 {
			b.Author = d.AuthorName[0]
		}
		if d.CoverID > 0 {
			b.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", d.CoverID)
		}
		books = append(books, b)
	}
	return books, nil
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
