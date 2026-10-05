package metadata

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// editionLimit bounds the picker's candidate list (design.md ADR-14):
// separate from resultLimit since the picker asks a different question
// ("enough covers to choose from") than a search answers ("is this the
// book I meant").
const editionLimit = 12

// workKeyPattern is the only shape an Open Library work key ever takes
// ("/works/OL123W"). workKey arrives from the form's own workKey signal
// (design.md), so the browser controls it; openLibraryEditions builds a
// URL from it, and an unchecked value there could send the request
// somewhere else entirely (a value starting "@host" turning the path
// into part of the authority) or break the request path (a stray "?" or
// "#"). Anything not matching this shape is treated as no work key at
// all, which already has a defined, safe behaviour (no Open Library
// candidates).
var workKeyPattern = regexp.MustCompile(`^/works/OL[0-9]+W$`)

// Editions gathers cover-bearing candidate editions of one book for the
// edition picker (design.md ADR-14): Open Library's own editions of the
// work, plus Google Books volumes matching the same title and author,
// merged by the same rule SearchBooks uses (merge.go). workKey is the
// Open Library work key a search result or an already-picked edition
// carries; when the form has none (a filed item never stores one),
// Editions resolves one itself from title and author before fetching.
// Every candidate without a cover is excluded before the merge
// (cover-management: Edition Candidate Sourcing) — a picker with nothing
// to show is worth less than no candidate at all.
func (c *Client) Editions(ctx context.Context, workKey, title, author string) Search {
	workKey = strings.TrimSpace(workKey)
	if workKey == "" {
		// Bounded by the same deadline as the concurrent fetch below:
		// capture never waits on a slow source (design.md ADR-1), and a
		// resolution search is itself just one more source this picker
		// asks.
		rctx, cancel := context.WithTimeout(ctx, searchDeadline)
		workKey = c.resolveWorkKey(rctx, title, author)
		cancel()
	}
	return c.mergedSearch(ctx, editionLimit,
		func(ctx context.Context) ([]Book, int, error) {
			books, err := c.openLibraryEditions(ctx, workKey, author)
			return withCover(books), 0, err
		},
		func(ctx context.Context) ([]Book, int, error) {
			if strings.TrimSpace(title) == "" {
				return nil, 0, nil // nothing meaningful to ask Google for
			}
			books, _, err := c.googleBooks(ctx, editionsQuery(title, author))
			return withCover(books), 0, err
		},
	)
}

// resolveWorkKey finds a work's Open Library key by title and author, for
// a filed item that never stored one. "" means nothing matched, or the
// lookup itself failed — either way Editions still runs the Google side
// without it, so a failed resolution degrades rather than emptying the
// picker outright.
func (c *Client) resolveWorkKey(ctx context.Context, title, author string) string {
	q := strings.TrimSpace(title + " " + author)
	if q == "" {
		return ""
	}
	var out struct {
		Docs []struct {
			Key string `json:"key"`
		} `json:"docs"`
	}
	params := url.Values{"q": {q}, "limit": {"1"}, "fields": {"key"}}
	if err := c.getJSON(ctx, "https://openlibrary.org/search.json?"+params.Encode(), &out); err != nil || len(out.Docs) == 0 {
		return ""
	}
	return out.Docs[0].Key
}

// olEditionDoc is one entry of /works/{key}/editions.json.
//
// Task 10.1 technical check (2026-10-05): confirmed live against
// https://openlibrary.org/works/OL893414W/editions.json (Dune) that the
// field names design.md assumed are exactly these: "publishers" (array),
// "publish_date" (free-form string, the same shape search.json's own
// editions.publish_date already handles via yearFromDates), "covers"
// (array of the same numeric cover ids search.json's cover_i uses, so
// covers.openlibrary.org/b/id/<id>-L.jpg applies unchanged), "isbn_13" and
// "isbn_10" (arrays, both observed present on different entries), and
// "languages" (array of {"key": "/languages/<code>"}). Every <code>
// observed across the work's 155 editions (dut, eng, fre, heb, pol, por,
// rum, rus, spa, ukr) is a 3-letter MARC/ISO 639-2 bibliographic code, not
// a 2-letter ISO 639-1 one; languageNames below covers all ten plus other
// common codes on the same scheme. No shape differs from design.md in a
// way that changes the design — only parsing needed writing.
type olEditionDoc struct {
	Title         string   `json:"title"`
	Publishers    []string `json:"publishers"`
	PublishDate   string   `json:"publish_date"`
	NumberOfPages int      `json:"number_of_pages"`
	Covers        []int    `json:"covers"`
	ISBN13        []string `json:"isbn_13"`
	ISBN10        []string `json:"isbn_10"`
	Languages     []struct {
		Key string `json:"key"`
	} `json:"languages"`
}

// openLibraryEditions fetches one work's own editions. workKey == ""
// answers no candidates and no error: there is nothing to ask Open
// Library for, which is not the same as Open Library failing to answer.
// author is the book's own confirmed author, filled onto every edition
// here since editions.json gives no clean author name of its own (only a
// free-text by_statement, and an author key that would need a further
// fetch to resolve) and every edition of one work shares the same author
// by definition.
func (c *Client) openLibraryEditions(ctx context.Context, workKey, author string) ([]Book, error) {
	if !workKeyPattern.MatchString(workKey) {
		return nil, nil
	}
	var out struct {
		Entries []olEditionDoc `json:"entries"`
	}
	endpoint := "https://openlibrary.org" + workKey + "/editions.json?limit=50"
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	books := make([]Book, 0, len(out.Entries))
	for _, d := range out.Entries {
		books = append(books, bookFromEdition(workKey, author, d))
	}
	return books, nil
}

// bookFromEdition fills a Book from one edition. Unlike bookFromDoc
// (metadata.go, a work-level search result), Title here is the edition's
// own title: it genuinely is a different printing, possibly a
// translation.
func bookFromEdition(workKey, author string, d olEditionDoc) Book {
	b := Book{Key: workKey, Title: d.Title, Author: author, Pages: d.NumberOfPages}
	if len(d.Publishers) > 0 {
		b.Publisher = d.Publishers[0]
	}
	if year := yearFromDates.FindString(d.PublishDate); year != "" {
		b.Year, _ = strconv.Atoi(year)
	}
	if len(d.Covers) > 0 && d.Covers[0] > 0 {
		id := d.Covers[0]
		b.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", id)
		b.ThumbURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", id)
	}
	allISBNs := slices.Concat(d.ISBN13, d.ISBN10)
	b.ISBN = bestISBN(allISBNs)
	b.ISBNs = knownISBNs(allISBNs)
	if len(d.Languages) > 0 {
		b.Language = languageName(d.Languages[0].Key)
	}
	return b
}

// withCover keeps only the candidates that actually carry a cover
// (cover-management: Edition Candidate Sourcing, "editions without a
// cover excluded"). nil in, nil out: an empty side stays empty rather
// than becoming a needless empty slice.
func withCover(books []Book) []Book {
	if len(books) == 0 {
		return nil
	}
	out := make([]Book, 0, len(books))
	for _, b := range books {
		if b.CoverURL != "" {
			out = append(out, b)
		}
	}
	return out
}

// editionsQuery builds the Google Books query for a work's editions
// (design.md ADR-14): titled and, when known, authored, so Google answers
// with the same book's other printings rather than an unrelated search.
func editionsQuery(title, author string) string {
	q := fmt.Sprintf(`intitle:"%s"`, strings.TrimSpace(title))
	if author = strings.TrimSpace(author); author != "" {
		q += fmt.Sprintf(` inauthor:"%s"`, author)
	}
	return q
}

// languageNames maps an Open Library language key's code (MARC/ISO 639-2
// bibliographic, confirmed live — see olEditionDoc's doc comment) to a
// plain display name. Every code actually observed is listed, plus other
// common ones on the same scheme; an unlisted code is simply omitted
// (design.md ADR-14) rather than shown as a code nobody asked for.
var languageNames = map[string]string{
	"eng": "English",
	"fre": "French",
	"ger": "German",
	"spa": "Spanish",
	"ita": "Italian",
	"por": "Portuguese",
	"dut": "Dutch",
	"rus": "Russian",
	"pol": "Polish",
	"rum": "Romanian",
	"heb": "Hebrew",
	"ukr": "Ukrainian",
	"swe": "Swedish",
	"nor": "Norwegian",
	"dan": "Danish",
	"fin": "Finnish",
	"gre": "Greek",
	"tur": "Turkish",
	"ara": "Arabic",
	"jpn": "Japanese",
	"chi": "Chinese",
	"kor": "Korean",
	"cze": "Czech",
	"hun": "Hungarian",
	"bul": "Bulgarian",
	"hrv": "Croatian",
	"srp": "Serbian",
	"cat": "Catalan",
	"lat": "Latin",
}

// languageName turns an Open Library language key ("/languages/eng") into
// a display name, or "" for a code not in languageNames.
func languageName(key string) string {
	return languageNames[strings.TrimPrefix(key, "/languages/")]
}
