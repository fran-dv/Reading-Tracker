package web

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/fran-dv/reading-tracker/internal/isbn"
	"github.com/fran-dv/reading-tracker/internal/metadata"
	"github.com/starfederation/datastar-go/datastar"
)

// Book search (spec §4, step 29): the title field's results list, merged
// from Open Library and Google Books (internal/metadata). getBooks is the
// one handler; searchNote, searchAnnouncement and matchedTitle are the
// pure pieces it and the results template share.

// searchResults is what the "search-results" block draws: the search
// itself, plus the query it answered and whether it was an ISBN lookup.
// Query is what lets the block mark matched words (Matched Word
// Highlighting, P6) and carry data-query for the pending guard (Pending
// State Without Flicker, P4) — the exact live value of whichever field
// searched, title's own typed words or the ISBN field's digits. ISBN scopes
// "None of these" (P7) to the ISBN field's own outcome: keep the entered
// ISBN and move focus to Title, rather than confirming the typed words as
// the title.
type searchResults struct {
	metadata.Search
	Query string
	ISBN  bool
}

// getBooks answers both the title field's words search and the ISBN
// field's own lookup: both GET this one URL (design.md ADR-3), and the
// payload says which — isbn wins when both are somehow present, since only
// one field can hold a complete-length value that triggers a request at a
// time. Neither source's failure is an HTTP error: metadata.Client never
// fails outright on a bad or slow source (design.md ADR-1), so the results
// list itself always carries whatever disclosure is due — which sources
// answered, and the plain failure line with Try again when neither did.
func (h *handler) getBooks(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
		ISBN  string `json:"isbn"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if raw := strings.TrimSpace(in.ISBN); raw != "" {
		h.getISBNBooks(w, r, raw)
		return
	}

	query := strings.TrimSpace(in.Title)
	if len([]rune(query)) < 3 || strings.Contains(query, "://") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	res := h.meta.SearchBooks(r.Context(), query)
	if r.Context().Err() != nil {
		// Superseded Request Cancellation (P5), server side: the field has
		// already moved on (Datastar aborted this request when it sent the
		// next one — see the technical-check comment in capture.html), so
		// there is nothing left to draw for a query nobody is waiting on.
		return
	}
	if len(res.Unanswered) > 0 {
		h.log.Info("book search degraded", "query", query, "unanswered", res.Unanswered)
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.capture, "search-results", searchResults{Search: res, Query: query}); err != nil {
		h.log.Error("book results", "err", err)
		return
	}
	signals := map[string]any{
		"_showResults":  true,
		"errors":        lookupErrors(),
		"_searchStatus": searchAnnouncement(len(res.Books), res.More, res.Unanswered),
	}
	if err := sse.MarshalAndPatchSignals(signals); err != nil {
		h.log.Error("book results signals", "err", err)
	}
}

// getISBNBooks answers the ISBN field's own request within getBooks (ISBN
// Field Entry and Validation, P9): raw is the field's own trimmed value at
// the moment a complete digit count triggered the request. A bad checksum
// sets errors.isbn and looks nothing up (no debounce either way — the
// field only ever asks once a value is complete-length, design.md ADR-5).
// A valid one runs LookupISBN across both sources at once (ISBN Lookup
// Across Sources) and patches the same #search-results list, sharing the
// cancellation guard and "None of these" row a words search uses, scoped
// to the ISBN outcome by searchResults.ISBN.
func (h *handler) getISBNBooks(w http.ResponseWriter, r *http.Request, raw string) {
	isbn13, ok := isbn.Normalize(raw)
	if !ok {
		sse := datastar.NewSSE(w, r)
		if err := sse.MarshalAndPatchSignals(map[string]any{
			"errors": map[string]string{"isbn": "Those digits don't make an ISBN. Check them against the book."},
		}); err != nil {
			h.log.Error("isbn checksum error", "err", err)
		}
		return
	}

	res := h.meta.LookupISBN(r.Context(), isbn13)
	if r.Context().Err() != nil {
		// Superseded (P5), server side, same as getBooks: checked before
		// datastar.NewSSE touches the response at all, so an aborted
		// request never even gets its headers set for a patch nobody
		// will read.
		return
	}
	if len(res.Unanswered) > 0 {
		h.log.Info("isbn lookup degraded", "isbn", isbn13, "unanswered", res.Unanswered)
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.capture, "search-results", searchResults{Search: res, Query: raw, ISBN: true}); err != nil {
		h.log.Error("isbn results", "err", err)
		return
	}
	signals := map[string]any{
		"_showResults":  true,
		"errors":        map[string]string{"isbn": ""},
		"_searchStatus": searchAnnouncement(len(res.Books), res.More, res.Unanswered),
	}
	if err := sse.MarshalAndPatchSignals(signals); err != nil {
		h.log.Error("isbn results signals", "err", err)
	}
}

// searchNote is the plain line "search-results" shows about which sources
// answered (Source Status Disclosure, P2) or, when neither did, the rubric
// failure line Plain Failure States with Retry (P8) asks for. It returns ""
// when both sources answered, since nothing needs saying then.
func searchNote(unanswered []string) string {
	switch len(unanswered) {
	case 0:
		return ""
	case 1:
		if unanswered[0] == metadata.SourceGoogleBooks {
			return "Only Open Library answered."
		}
		return "Only Google Books answered."
	default:
		return "Neither Open Library nor Google Books answered. Type the details in."
	}
}

// searchAnnouncement is the polite live-region text a results update
// announces (Screen Reader and Focus for Search, P11): the result count,
// the "more not shown" count when present, and which sources answered.
func searchAnnouncement(n, more int, unanswered []string) string {
	s := fmt.Sprintf("%d result", n)
	if n != 1 {
		s += "s"
	}
	if more > 0 {
		s += fmt.Sprintf(", %d more not shown", more)
	}
	if note := searchNote(unanswered); note != "" {
		s += ". " + note
	}
	return s
}

// titleSegment is one run of a result's displayed title, marked as
// matching a word from the search or not (Matched Word Highlighting, P6).
// Concatenating every segment's Text reconstructs the title exactly.
type titleSegment struct {
	Text    string
	Matched bool
}

// matchedTitle splits title into segments, marking the words that match a
// word from query. Matching is whole-word, case-insensitive, ignoring
// punctuation, and looks across the whole displayed title — not only the
// part before its first colon (that fold is merge.go's rule for spotting a
// duplicate, a different job) — because a relevance search can match a
// subtitle or alternate title that only shows up there.
//
// When every word in title matches, or none does, there is nothing to set
// apart: the whole title comes back as one unmatched segment. That keeps a
// title the query does not literally contain — matched on data the result
// doesn't carry here, such as an alternate title Open Library knows but
// this package never fetched — from being visually diminished; the
// "whole title treated as matched" scenario (book-search spec) covers the
// all-match case the same way.
func matchedTitle(query, title string) []titleSegment {
	words := make(map[string]bool)
	for _, w := range wordRuns(query) {
		words[w] = true
	}
	if len(words) == 0 {
		return []titleSegment{{Text: title}}
	}

	var segs []titleSegment
	var anyMatched, anyUnmatched bool
	runes := []rune(title)
	for i := 0; i < len(runes); {
		isWord := isWordRune(runes[i])
		j := i + 1
		for j < len(runes) && isWordRune(runes[j]) == isWord {
			j++
		}
		run := string(runes[i:j])
		matched := isWord && words[foldWord(run)]
		if isWord {
			if matched {
				anyMatched = true
			} else {
				anyUnmatched = true
			}
		}
		if n := len(segs); n > 0 && segs[n-1].Matched == matched {
			segs[n-1].Text += run
		} else {
			segs = append(segs, titleSegment{Text: run, Matched: matched})
		}
		i = j
	}
	if !anyMatched || !anyUnmatched {
		return []titleSegment{{Text: title}}
	}
	return segs
}

// isWordRune reports whether r belongs inside a word run for matchedTitle's
// tokenizer: a letter, a digit, or an apostrophe — straight or curly, so an
// internal one ("Don't", "O'Brien") keeps the word whole instead of
// splitting it at the punctuation and losing the match on either side.
// foldWord still strips the apostrophe itself when comparing.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '’'
}

// wordRuns splits s into its word runs (isWordRune-only stretches, the
// same ones matchedTitle's own tokenizer finds in the title), each
// already folded, skipping every separator. The query is tokenized the
// same way the title is so a hyphenated compound folds and splits
// identically on both sides: "spider-man" typed as the query becomes the
// two words "spider" and "man", matching a title's own "Spider-Man" word
// by word, rather than one compound no title-side run could ever equal.
func wordRuns(s string) []string {
	var words []string
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if !isWordRune(runes[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(runes) && isWordRune(runes[j]) {
			j++
		}
		if w := foldWord(string(runes[i:j])); w != "" {
			words = append(words, w)
		}
		i = j
	}
	return words
}

// foldWord lower-cases a word by Unicode category, the same rule
// merge.go's foldAlnum uses, so "García" and "GARCÍA" compare equal.
func foldWord(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
