package web

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/fran-dv/reading-tracker/internal/metadata"
	"github.com/starfederation/datastar-go/datastar"
)

// Book search (spec §4, step 29): the title field's results list, merged
// from Open Library and Google Books (internal/metadata). getBooks is the
// one handler; searchNote and matchedTitle are the pure pieces it and the
// results template share.

// searchResults is what the "search-results" block draws: the search
// itself, plus the query it answered. The query is what lets the block
// mark matched words (Matched Word Highlighting, P6) and carry data-query
// for the pending guard (Pending State Without Flicker, P4).
type searchResults struct {
	metadata.Search
	Query string
}

// getBooks searches Open Library and Google Books for the title typed so
// far and merges what each found. Neither source's failure is an HTTP
// error: metadata.Client.SearchBooks never fails outright (design.md
// ADR-1), so the results list itself always carries whatever disclosure is
// due — which sources answered, and the plain failure line with Try again
// when neither did.
func (h *handler) getBooks(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(in.Title)
	if len([]rune(query)) < 3 || strings.Contains(query, "://") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	res := h.meta.SearchBooks(r.Context(), query)
	if len(res.Unanswered) > 0 {
		h.log.Info("book search degraded", "query", query, "unanswered", res.Unanswered)
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.capture, "search-results", searchResults{Search: res, Query: query}); err != nil {
		h.log.Error("book results", "err", err)
		return
	}
	if err := sse.MarshalAndPatchSignals(map[string]any{"_showResults": true, "errors": lookupErrors()}); err != nil {
		h.log.Error("book results signals", "err", err)
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
