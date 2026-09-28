package web

import (
	"net/http"
	"strings"

	"github.com/fran-dv/reading-tracker/internal/metadata"
	"github.com/starfederation/datastar-go/datastar"
)

// Book search (spec §4, step 29): the title field's results list, merged
// from Open Library and Google Books (internal/metadata). getBooks is the
// one handler; searchNote is the pure text it and the results template
// share for Source Status Disclosure (P2) and Plain Failure States with
// Retry (P8).

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
	if err := h.patch(sse, h.capture, "search-results", res); err != nil {
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
