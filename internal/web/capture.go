package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/metadata"
	"github.com/starfederation/datastar-go/datastar"
)

// Capture (spec §4): paste a link, search Open Library, or type an item in,
// then file it on a shelf with one line of why.
//
// The page keeps its state in Datastar signals. Handlers never re-render the
// form: morphing inputs would desync them from their signals. Instead they
// patch signals (field values, errors, a reset) and a few named blocks.

// metadataClient is what capture needs from the metadata package. Tests
// substitute a fake.
type metadataClient interface {
	Lookup(ctx context.Context, rawURL string) (metadata.Result, error)
	SearchBooks(ctx context.Context, query string) ([]metadata.Book, error)
}

// newShelfID is the shelf picker value that reveals the new-shelf field.
// Shelf IDs are UUIDs, so it can never collide with a real one.
const newShelfID = "new"

// captureSignals mirrors the capture form. The same struct seeds the page,
// reads a submitted form, and resets the form after filing.
type captureSignals struct {
	Title       string `json:"title"` // the smart field: a title or a link
	URL         string `json:"url"`
	Author      string `json:"author"`
	Format      string `json:"format"`
	ShelfID     string `json:"shelfId"`
	ShelfName   string `json:"shelfName"` // shown on the picker; the ID is what gets filed
	NewShelf    string `json:"newShelf"`
	Why         string `json:"why"`
	Tags        string `json:"tags"` // comma separated
	FocusDemand string `json:"focusDemand"`
	SizeValue   string `json:"sizeValue"` // a string, as inputs give it; "" is empty
	NeedsDesk   bool   `json:"needsDesk"`
	Text        string `json:"text"` // pasted article text; only its word count is kept
	CoverURL    string `json:"coverUrl"`

	Errors   map[string]string                         `json:"errors"`
	Defaults map[library.Format]library.FormatDefaults `json:"defaults"`
	Pencil   map[string]bool                           `json:"pencil"` // fields a lookup filled and nobody has touched

	// Underscore signals stay in the browser; the server only seeds them.
	ShowResults bool `json:"_showResults"`
	ShelfOpen   bool `json:"_shelfOpen"`
	ShelfActive int  `json:"_shelfActive"` // highlighted option while the list is open
}

// newCaptureSignals is a blank form filed under the given shelf.
func newCaptureSignals(shelfID, shelfName string) captureSignals {
	defaults := make(map[library.Format]library.FormatDefaults, len(library.Formats))
	for _, f := range library.Formats {
		defaults[f] = library.Defaults(f)
	}
	book := library.Defaults(library.FormatBook)
	return captureSignals{
		Format:      string(library.FormatBook),
		ShelfID:     shelfID,
		ShelfName:   shelfName,
		FocusDemand: string(book.FocusDemand),
		NeedsDesk:   book.NeedsDesk,
		Errors:      blankErrors(),
		Defaults:    defaults,
		Pencil:      blankPencil(),
	}
}

// blankErrors lists every error slot on the page, so one patch clears them all.
// Keys are the library's field names.
func blankErrors() map[string]string {
	return map[string]string{"title": "", "url": "", "why": "", "shelf_id": "", "size_value": "", "new_shelf": ""}
}

// blankPencil lists every field a lookup can fill.
func blankPencil() map[string]bool {
	return map[string]bool{"title": false, "author": false, "sizeValue": false}
}

// lookupErrors clears the slots a lookup can set, leaving the others alone.
func lookupErrors() map[string]string {
	return map[string]string{"title": "", "url": ""}
}

// fieldInputs maps an error slot to the input that fixes it, so a rejected
// entry can move focus there and bring the message into view.
var fieldInputs = map[string]string{
	"title": "title", "url": "url", "why": "why", "shelf_id": "shelf-button",
	"size_value": "size", "new_shelf": "new-shelf",
}

// focusField sends focus to the input behind an error slot, if there is one.
func focusField(sse *datastar.ServerSentEventGenerator, field string) error {
	id, ok := fieldInputs[field]
	if !ok {
		return nil
	}
	return sse.ExecuteScript(`document.getElementById("` + id + `").focus()`)
}

// fieldError puts one validation message in its slot and clears the rest.
func fieldError(field, msg string) map[string]string {
	errs := blankErrors()
	errs[field] = msg
	return errs
}

// toItem converts the form into an item and its tags. Only the size can fail
// here; everything else is validated by the library.
func (in captureSignals) toItem() (library.Item, []string, error) {
	item := library.Item{
		Title:       in.Title,
		URL:         strings.TrimSpace(in.URL),
		Author:      strings.TrimSpace(in.Author),
		Format:      library.Format(in.Format),
		ShelfID:     in.ShelfID,
		Why:         in.Why,
		FocusDemand: library.FocusDemand(in.FocusDemand),
		NeedsDesk:   in.NeedsDesk,
		CoverURL:    in.CoverURL,
	}
	if s := strings.TrimSpace(in.SizeValue); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return item, nil, &library.ValidationError{Field: "size_value", Msg: "Use a whole number."}
		}
		item.SizeValue = &n
	}
	if item.Format == library.FormatArticle {
		if item.SizeValue == nil && strings.TrimSpace(in.Text) != "" {
			n := metadata.CountWords(in.Text)
			item.SizeValue = &n
		}
		item.WordCount = item.SizeValue // an article's size is its word count
	}
	return item, strings.Split(in.Tags, ","), nil
}

// captureMessages turns the library's terse validation messages into copy
// for the form. Unlisted fields keep the library's message.
var captureMessages = map[string]string{
	"title":    "Give it a title.",
	"why":      "Write one line on why before filing it.",
	"shelf_id": "Pick a shelf, or add the new one first.",
}

func messageFor(verr *library.ValidationError) string {
	if msg, ok := captureMessages[verr.Field]; ok && verr.Msg == "required" {
		return msg
	}
	return verr.Msg
}

type capturePage struct {
	Signals string
	Shelves []library.Shelf
	Formats []library.Format
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
	signals, err := json.Marshal(newCaptureSignals(shelfID, shelfName))
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.capture, capturePage{Signals: string(signals), Shelves: shelves, Formats: library.Formats})
}

// postItem files the item. Validation problems are an expected outcome, so
// they answer 200 with error signals; anything else is a real HTTP error.
func (h *handler) postItem(w http.ResponseWriter, r *http.Request) {
	var in captureSignals
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
		created, err = h.svc.CreateItem(ctx, item)
	}
	var verr *library.ValidationError
	if errors.As(err, &verr) {
		sse := datastar.NewSSE(w, r)
		if err := sse.MarshalAndPatchSignals(map[string]any{"errors": fieldError(verr.Field, messageFor(verr))}); err != nil {
			h.log.Error("capture errors", "err", err)
			return
		}
		if err := focusField(sse, verr.Field); err != nil {
			h.log.Error("capture error focus", "err", err)
		}
		return
	}
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	if err := h.svc.SetTags(ctx, created.ID, tags); err != nil {
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
	if err := sse.MarshalAndPatchSignals(newCaptureSignals(shelf.ID, shelf.Name)); err != nil {
		h.log.Error("capture reset", "err", err)
		return
	}
	if err := sse.ExecuteScript(`document.getElementById("title").focus()`); err != nil {
		h.log.Error("capture focus", "err", err)
	}
}

// postShelf adds a shelf from the capture form and selects it.
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

// getMetadata looks up the link typed into the smart field. The link moves
// to the URL field and whatever was found fills the fields that are empty or
// still in pencil from an earlier lookup. Found values stay in pencil until
// the owner touches them; typed values are never overwritten.
func (h *handler) getMetadata(w http.ResponseWriter, r *http.Request) {
	var in captureSignals
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	link := strings.TrimSpace(in.Title)
	if !strings.Contains(link, "://") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	res, err := h.meta.Lookup(r.Context(), link)
	out := map[string]any{"url": link, "title": "", "_showResults": false}
	pencil := blankPencil()
	switch {
	case errors.Is(err, metadata.ErrInvalidURL):
		out = map[string]any{"errors": map[string]string{"title": "That link doesn't look right.", "url": ""}}
	case err != nil:
		h.log.Info("lookup failed", "url", link, "err", err)
		out["errors"] = map[string]string{"title": "", "url": "Couldn't read this page. Fill in the title by hand."}
	default:
		out["errors"] = lookupErrors()
		if res.Title != "" {
			out["title"], pencil["title"] = res.Title, true
		}
		if d := library.Defaults(library.Format(res.Format)); d.SizeUnit != "" {
			// A new format brings its shape with it, as picking one by hand does.
			out["format"], out["focusDemand"], out["needsDesk"] = res.Format, d.FocusDemand, d.NeedsDesk
		}
		if in.Author == "" || in.Pencil["author"] {
			out["author"], pencil["author"] = res.Author, res.Author != ""
		}
		if in.SizeValue == "" || in.Pencil["sizeValue"] {
			out["sizeValue"], pencil["sizeValue"] = "", false
			if res.WordCount > 0 {
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
