package metadata

import (
	"context"
	"net/url"
	"strconv"
)

// Google Books' volumes.list endpoint, queried alongside Open Library
// (metadata.go) and merged with it (merge.go, design.md ADR-1). Keyless by
// default; New's googleBooksKey argument adds an owner-supplied key
// instead, since the keyless anonymous quota turned out to be low enough
// to exhaust in ordinary day-to-day use, not just in an automated sandbox
// (see below).
//
// Task 2.1 technical check findings:
//   - Any non-2xx status (429 quota refusal included) is treated as a
//     source error by get, the same as openLibrarySearch; nothing here
//     parses the status code or the error body.
//   - fields=: Google's usual partial-response syntax — a comma-separated
//     field list, with parentheses selecting sub-fields of a nested or
//     repeated field.
//   - zoom=: an undocumented query parameter on imageLinks URLs. zoom=1
//     (thumbnail) and zoom=5 (smallThumbnail, smaller despite the higher
//     number) are the only sizes the search/list endpoint returns; zoom=0
//     is the largest image obtainable by rewriting that same URL, which is
//     what googleImageURL uses for CoverURL.
//   - No "image not available" placeholder URL was found, live or
//     documented: when Google has no scan, imageLinks is simply absent.
//     bookFromVolume relies on exactly that (a cover is set only when the
//     field is present and non-empty) rather than matching a URL pattern.
//
// The live endpoint itself was unreachable while this was written: its
// shared anonymous daily quota was already exhausted, confirmed through two
// independent HTTP paths in the apply environment and, separately, on the
// owner's own machine — which is the reason New takes an optional key
// rather than staying keyless-only. The findings above come from Google's
// official API documentation and independently corroborated developer
// references instead of a live 200 response; the fixture in testdata/ is
// shaped from those, not captured live.
const (
	googleMaxResults = 10

	// googleFields: only what bookFromVolume reads. subtitle plays no part
	// in Book (merge folds Open Library's colon-joined title the same way
	// whether or not Google's title happens to carry one of its own), so it
	// is not requested; totalItems is Google's own count, an unreliable
	// estimate merge.go never uses (design.md ADR-1, rule 5), so it is not
	// requested either.
	googleFields = "items(volumeInfo(title,authors,publisher,publishedDate,pageCount," +
		"industryIdentifiers,imageLinks))"
)

// googleVolume is one Google Books search result, trimmed to googleFields.
type googleVolume struct {
	VolumeInfo struct {
		Title               string   `json:"title"`
		Authors             []string `json:"authors"`
		Publisher           string   `json:"publisher"`
		PublishedDate       string   `json:"publishedDate"`
		PageCount           int      `json:"pageCount"`
		IndustryIdentifiers []struct {
			Type       string `json:"type"`
			Identifier string `json:"identifier"`
		} `json:"industryIdentifiers"`
		ImageLinks struct {
			SmallThumbnail string `json:"smallThumbnail"`
			Thumbnail      string `json:"thumbnail"`
		} `json:"imageLinks"`
	} `json:"volumeInfo"`
}

// googleBooks queries Google Books' volumes endpoint, keyless or with the
// owner's own key (Client.googleBooksKey — see New's doc comment). q is
// passed through as Google expects it: free-text words for a title search,
// or "isbn:<isbn13>" for an ISBN lookup. The int result is always 0
// (Google's own total is not requested — see googleFields); it exists only
// so this method has the same shape as openLibrarySearch, which
// mergedSearch (metadata.go) relies on to run either source through one
// code path.
func (c *Client) googleBooks(ctx context.Context, q string) ([]Book, int, error) {
	params := url.Values{
		"q":          {q},
		"maxResults": {strconv.Itoa(googleMaxResults)},
		"fields":     {googleFields},
	}
	if c.googleBooksKey != "" {
		params.Set("key", c.googleBooksKey)
	}
	var out struct {
		Items []googleVolume `json:"items"`
	}
	if err := c.getJSON(ctx, "https://www.googleapis.com/books/v1/volumes?"+params.Encode(), &out); err != nil {
		return nil, 0, err
	}
	books := make([]Book, 0, len(out.Items))
	for _, item := range out.Items {
		books = append(books, bookFromVolume(item))
	}
	return books, 0, nil
}

// bookFromVolume fills a Book from one Google Books volume. Key stays ""
// (not an Open Library work); Title is Google's own title as given, with no
// work-versus-edition distinction to make.
func bookFromVolume(item googleVolume) Book {
	v := item.VolumeInfo
	b := Book{Title: v.Title, Publisher: v.Publisher, Pages: v.PageCount}
	if len(v.Authors) > 0 {
		b.Author = v.Authors[0]
	}
	if year := yearFromDates.FindString(v.PublishedDate); year != "" {
		b.Year, _ = strconv.Atoi(year)
	}
	var isbns []string
	for _, id := range v.IndustryIdentifiers {
		if id.Type == "ISBN_10" || id.Type == "ISBN_13" {
			isbns = append(isbns, id.Identifier)
		}
	}
	b.ISBN = bestISBN(isbns)
	b.ISBNs = knownISBNs(isbns)

	thumb := v.ImageLinks.Thumbnail
	if thumb == "" {
		thumb = v.ImageLinks.SmallThumbnail
	}
	if thumb != "" {
		b.ThumbURL = googleImageURL(thumb, false)
		b.CoverURL = googleImageURL(thumb, true)
	}
	return b
}

// googleImageURL upgrades a Google Books image link: https (Thumbnail URL
// Upgrade — a result is drawn client-side from this URL until a cover is
// filed) and, for a cover, the largest size obtainable by rewriting zoom
// (see this file's comment above the const block). edge=curl, a decorative
// page-curl Google adds in the corner, is dropped either way so a Google
// cover reads the same plainly as an Open Library one.
func googleImageURL(raw string, cover bool) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = "https"
	q := u.Query()
	q.Del("edge")
	if cover {
		q.Set("zoom", "0")
	}
	u.RawQuery = q.Encode()
	return u.String()
}
