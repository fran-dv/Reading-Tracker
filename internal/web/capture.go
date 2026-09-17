package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/metadata"
	"github.com/starfederation/datastar-go/datastar"
)

// Capture (spec §4): paste a link, search Open Library, or type an item in,
// then file it on a shelf with one line of why. The fields themselves are the
// shared item form (itemform.go); this file is the filing screen around them.

// metadataClient is what capture needs from the metadata package. Tests
// substitute a fake.
type metadataClient interface {
	Lookup(ctx context.Context, rawURL string) (metadata.Result, error)
	SearchBooks(ctx context.Context, query string) ([]metadata.Book, error)
}

type capturePage struct {
	shell
	*itemFormData
}

// saved is what the confirmation line shows after filing.
type saved struct {
	Item  *library.Item
	Shelf *library.Shelf
}

func (h *handler) getCapture(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shelves, err := h.svc.ListShelves(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	shelfID, err := h.svc.LastUsedShelfID(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	shelfName := ""
	for _, s := range shelves {
		if s.ID == shelfID {
			shelfName = s.Name
		}
	}
	if shelfID == "" {
		shelfID = newShelfID // an empty library starts by naming a shelf
	}
	form, err := newItemFormData(newItemForm(shelfID, shelfName), shelves)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.capture, capturePage{shell: newShell("/capture"), itemFormData: form})
}

// postItem files the item. Validation problems are an expected outcome, so
// they answer 200 with error signals; anything else is a real HTTP error.
func (h *handler) postItem(w http.ResponseWriter, r *http.Request) {
	var in itemForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	item, tags, err := in.toItem()
	if err == nil && item.ShelfID == newShelfID {
		err = &library.ValidationError{Field: "shelf_id", Msg: "required"}
	}
	var created *library.Item
	if err == nil {
		created, err = h.svc.CreateItem(ctx, item, tags)
	}
	if h.formError(w, r, err) {
		return
	}
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	shelf, err := h.svc.GetShelf(ctx, created.ShelfID)
	if err != nil {
		h.httpError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.capture, "capture-status", saved{Item: created, Shelf: shelf}, datastar.WithModeReplace()); err != nil {
		h.log.Error("capture status", "err", err)
		return
	}
	if err := sse.MarshalAndPatchSignals(newItemForm(shelf.ID, shelf.Name)); err != nil {
		h.log.Error("capture reset", "err", err)
		return
	}
	if err := sse.ExecuteScript(`document.getElementById("title").focus()`); err != nil {
		h.log.Error("capture focus", "err", err)
	}
}

// postShelf adds a shelf from either form and selects it.
func (h *handler) postShelf(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NewShelf string `json:"newShelf"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	shelf, err := h.svc.CreateShelf(ctx, in.NewShelf)
	var msg string
	var verr *library.ValidationError
	if errors.As(err, &verr) {
		msg = "Name the shelf."
	} else if errors.Is(err, library.ErrDuplicateShelf) {
		msg = "A shelf with that name already exists."
	}
	if msg != "" {
		sse := datastar.NewSSE(w, r)
		if err := sse.MarshalAndPatchSignals(map[string]any{"errors": fieldError("new_shelf", msg)}); err != nil {
			h.log.Error("shelf errors", "err", err)
			return
		}
		if err := focusField(sse, "new_shelf"); err != nil {
			h.log.Error("shelf error focus", "err", err)
		}
		return
	}
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	shelves, err := h.svc.ListShelves(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	// Options first, so the new shelf exists when the signal selects it.
	if err := h.patch(sse, h.capture, "shelf-picker", shelves); err != nil {
		h.log.Error("shelf picker", "err", err)
		return
	}
	reset := map[string]any{"shelfId": shelf.ID, "shelfName": shelf.Name, "newShelf": "", "errors": blankErrors()}
	if err := sse.MarshalAndPatchSignals(reset); err != nil {
		h.log.Error("shelf select", "err", err)
	}
}

// getMetadata looks up a link and fills what it finds. The link is whatever
// was typed into capture's smart field, or else the form's own URL field —
// which is how the shelf view's Refetch reaches this handler.
//
// Found values land in pencil and stay there until the owner touches them.
// A typed value is never overwritten; a pencilled one may be replaced by a
// later lookup.
func (h *handler) getMetadata(w http.ResponseWriter, r *http.Request) {
	var in itemForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	link := strings.TrimSpace(in.Title)
	pasted := strings.Contains(link, "://") // typed into the smart field
	if !pasted {
		link = strings.TrimSpace(in.URL)
	}
	if !strings.Contains(link, "://") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	res, err := h.meta.Lookup(r.Context(), link)
	out := map[string]any{"url": link, "_showResults": false}
	if pasted {
		out["title"] = "" // the link moves out of the smart field
	}
	pencil := blankPencil()
	switch {
	case errors.Is(err, metadata.ErrInvalidURL):
		out = map[string]any{"errors": map[string]string{"title": "That link doesn't look right.", "url": ""}}
	case err != nil:
		h.log.Info("lookup failed", "url", link, "err", err)
		out["errors"] = map[string]string{"title": "", "url": "Couldn't read this page. Fill in the title by hand."}
	default:
		out["errors"] = lookupErrors()
		if res.Title != "" && (pasted || in.Title == "" || in.Pencil["title"]) {
			out["title"], pencil["title"] = res.Title, true
		}
		format := library.Format(in.Format)
		if d := library.Defaults(library.Format(res.Format)); pasted && d.SizeUnit != "" && res.Format != in.Format {
			// A link pasted to capture brings its format and that format's
			// shape, as picking one by hand does. A refetch on an item already
			// filed never changes what it is.
			format = library.Format(res.Format)
			out["format"], out["focusDemand"], out["needsDesk"] = res.Format, d.FocusDemand, d.NeedsDesk
		}
		if in.Author == "" || in.Pencil["author"] {
			out["author"], pencil["author"] = res.Author, res.Author != ""
		}
		if in.SizeValue == "" || in.Pencil["sizeValue"] {
			out["sizeValue"], pencil["sizeValue"] = "", false
			if res.WordCount > 0 && library.Defaults(format).SizeUnit == library.UnitWords {
				out["sizeValue"], pencil["sizeValue"] = strconv.Itoa(res.WordCount), true
			}
		}
		out["coverUrl"] = res.CoverURL // never typed, so always the latest lookup's
		out["pencil"] = pencil
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(out); err != nil {
		h.log.Error("lookup signals", "err", err)
	}
}

// getBooks searches Open Library for the title typed so far.
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

	books, err := h.meta.SearchBooks(r.Context(), query)
	sse := datastar.NewSSE(w, r)
	if err != nil {
		h.log.Info("book search failed", "query", query, "err", err)
		out := map[string]any{"_showResults": false, "errors": map[string]string{"title": "Open Library didn't answer. Type the details in."}}
		if err := sse.MarshalAndPatchSignals(out); err != nil {
			h.log.Error("book search signals", "err", err)
		}
		return
	}
	if err := h.patch(sse, h.capture, "search-results", books); err != nil {
		h.log.Error("book results", "err", err)
		return
	}
	if err := sse.MarshalAndPatchSignals(map[string]any{"_showResults": true, "errors": lookupErrors()}); err != nil {
		h.log.Error("book results signals", "err", err)
	}
}
