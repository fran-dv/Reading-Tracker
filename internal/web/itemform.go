package web

import (
	"context"
	"strconv"
	"strings"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The item form is shared. Capture files a new item with it (spec §4); the
// shelf view opens the same fields in place to correct one (§6.2). Both
// render the "item-fields" template block and drive the same signals, so one
// struct describes the form wherever it appears.
//
// State lives in Datastar signals. Handlers never re-render an open form's
// inputs: morphing them would desync them from their signals. They patch
// signals instead — field values, errors, a reset.

// newShelfID is the shelf picker value that reveals the new-shelf field.
// Shelf IDs are UUIDs, so it can never collide with a real one.
const newShelfID = "new"

// itemForm mirrors the form. The same struct seeds it, reads it back on
// submit, and resets it.
type itemForm struct {
	Title       string `json:"title"` // on capture, the smart field: a title or a link
	URL         string `json:"url"`
	Author      string `json:"author"`
	Publisher   string `json:"publisher"`
	ISBN        string `json:"isbn"` // books only; normalized to ISBN-13 on save
	Format      string `json:"format"`
	ShelfID     string `json:"shelfId"`
	ShelfName   string `json:"shelfName"` // shown on the picker; the ID is what gets filed
	NewShelf    string `json:"newShelf"`
	Why         string `json:"why"`
	Tags        string `json:"tags"` // comma separated
	FocusDemand string `json:"focusDemand"`
	SizeValue   string `json:"sizeValue"` // a string, as inputs give it; "" is empty
	NeedsDesk   bool   `json:"needsDesk"`
	Text        string `json:"text"` // pasted article text, counted by POST /words; never stored
	CoverURL    string `json:"coverUrl"`

	// The staged cover (cover-management: Cover Changes Are Staged in the
	// Form Until Filed or Saved). CoverChoice mirrors library.CoverChoice
	// as a plain string, so a blank form seeds "found" without importing
	// an enum into JSON. CoverDraft is empty until an upload or a pick
	// stages one (design.md ADR-10); CoverSource is the edition link a
	// pick carries, always empty for an upload — read back from the
	// draft store itself in coverImage below, never from this signal
	// directly, so on the server it is purely informational, for the
	// cover row's own markup to show. CoverHeldURL is this item's own
	// served cover for a picked or uploaded choice already held when
	// the form opened — empty on capture, where the item does not exist
	// yet.
	CoverChoice  string `json:"coverChoice"`
	CoverDraft   string `json:"coverDraft"`
	CoverSource  string `json:"coverSource"`
	CoverHeldURL string `json:"coverHeldUrl"`

	Errors   map[string]string                         `json:"errors"`
	Defaults map[library.Format]library.FormatDefaults `json:"defaults"`
	Pencil   map[string]bool                           `json:"pencil"` // fields a lookup filled and nobody has touched

	// Underscore signals stay in the browser; the server only seeds them.
	ShowResults  bool   `json:"_showResults"`
	SearchStatus string `json:"_searchStatus"` // the live region's text (P11); set by getBooks

	// CoverLocal is the object-URL preview a file chooser, a drop, or a
	// paste shows at once, before the upload round trip answers
	// (cover-management: Local Preview Before Processing). CoverUndo,
	// CoverUndoDraft and CoverUndoSource hold the cover state immediately
	// before a found/picked revert or removal, for one quiet Undo
	// (Immediate Staged Change with Undo). CoverConfirm names which
	// action (found or remove) the shared confirmation dialog is about,
	// for discarding a held or staged uploaded cover (P13). The cover
	// row's own markup is what reads and writes every one of these; the
	// server only clears _coverLocal once an upload it started is done.
	CoverLocal      string `json:"_coverLocal"`
	CoverUndo       string `json:"_coverUndo"`
	CoverUndoDraft  string `json:"_coverUndoDraft"`
	CoverUndoSource string `json:"_coverUndoSource"`
	CoverConfirm    string `json:"_coverConfirm"`
}

// newItemForm is a blank form filed under the given shelf. A fresh item
// has not been captured yet, so its cover choice starts at the default,
// found, with nothing held (library.CreateItem agrees).
func newItemForm(shelfID, shelfName string) itemForm {
	book := library.Defaults(library.FormatBook)
	return itemForm{
		Format:      string(library.FormatBook),
		ShelfID:     shelfID,
		ShelfName:   shelfName,
		FocusDemand: string(book.FocusDemand),
		NeedsDesk:   book.NeedsDesk,
		CoverChoice: string(library.CoverFound),
		Errors:      blankErrors(),
		Defaults:    formatDefaults(),
		Pencil:      blankPencil(),
	}
}

// itemFormFor is the form seeded from an item that already exists. Nothing is
// in pencil: every value here was either confirmed or typed.
func itemFormFor(item *library.Item, tags []string, shelfName string) itemForm {
	size := ""
	if item.SizeValue != nil {
		size = strconv.Itoa(*item.SizeValue)
		if item.SizeUnit == library.UnitMinutes {
			size = minutesField(*item.SizeValue)
		}
	}
	return itemForm{
		Title:        item.Title,
		URL:          item.URL,
		Author:       item.Author,
		Publisher:    item.Publisher,
		ISBN:         item.ISBN,
		Format:       string(item.Format),
		ShelfID:      item.ShelfID,
		ShelfName:    shelfName,
		Why:          item.Why,
		Tags:         strings.Join(tags, ", "),
		FocusDemand:  string(item.FocusDemand),
		SizeValue:    size,
		NeedsDesk:    item.NeedsDesk,
		CoverURL:     item.CoverURL,
		CoverChoice:  string(item.CoverChoice),
		CoverHeldURL: coverHeldURL(item),
		Errors:       blankErrors(),
		Defaults:     formatDefaults(),
		Pencil:       blankPencil(),
	}
}

// coverHeldURL is the item's own served cover, for the form's preview
// when editing an item whose choice is already picked or uploaded —
// bytes a draft does not (yet) replace. found and removed need no URL
// of their own: found already has one in CoverURL, and removed shows
// nothing.
func coverHeldURL(item *library.Item) string {
	if item.CoverChoice != library.CoverPicked && item.CoverChoice != library.CoverUploaded {
		return ""
	}
	return coverPlateURL(item.ID, item.UpdatedAt)
}

// formatDefaults is the shape each format starts with, so picking one in the
// browser fills focus and desk without a round trip.
func formatDefaults() map[library.Format]library.FormatDefaults {
	out := make(map[library.Format]library.FormatDefaults, len(library.Formats))
	for _, f := range library.Formats {
		out[f] = library.Defaults(f)
	}
	return out
}

// blankErrors lists every error slot on the page, so one patch clears them all.
// Keys are the library's field names.
func blankErrors() map[string]string {
	return map[string]string{"title": "", "url": "", "why": "", "format": "", "shelf_id": "", "size_value": "", "new_shelf": "", "isbn": "", "cover": ""}
}

// blankPencil lists every field a lookup or a picked search result can fill.
func blankPencil() map[string]bool {
	return map[string]bool{"title": false, "author": false, "sizeValue": false, "publisher": false, "isbn": false}
}

// lookupErrors clears the slots a lookup can set, leaving the others alone.
func lookupErrors() map[string]string {
	return map[string]string{"title": "", "url": ""}
}

// fieldInputs maps an error slot to the input that fixes it, so a rejected
// entry can move focus there and bring the message into view.
var fieldInputs = map[string]string{
	"title": "title", "url": "url", "why": "why", "shelf_id": "shelf-button",
	"size_value": "size", "new_shelf": "new-shelf", "isbn": "isbn",
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
func (in itemForm) toItem() (library.Item, []string, error) {
	item := library.Item{
		Title:       in.Title,
		URL:         strings.TrimSpace(in.URL),
		Author:      strings.TrimSpace(in.Author),
		Publisher:   strings.TrimSpace(in.Publisher),
		ISBN:        strings.TrimSpace(in.ISBN),
		Format:      library.Format(in.Format),
		ShelfID:     in.ShelfID,
		Why:         in.Why,
		FocusDemand: library.FocusDemand(in.FocusDemand),
		NeedsDesk:   in.NeedsDesk,
		CoverURL:    in.CoverURL,
	}
	if s := strings.TrimSpace(in.SizeValue); s != "" {
		if library.Defaults(item.Format).SizeUnit == library.UnitMinutes {
			n, ok := parseMinutes(s)
			if !ok {
				return item, nil, &library.ValidationError{Field: "size_value", Msg: "How long? Try 1h45, 1:45 or 105."}
			}
			item.SizeValue = &n
		} else {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				return item, nil, &library.ValidationError{Field: "size_value", Msg: "Use a whole number."}
			}
			item.SizeValue = &n
		}
	}
	if item.Format == library.FormatArticle {
		item.WordCount = item.SizeValue // an article's size is its word count
	}
	return item, strings.Split(in.Tags, ","), nil
}

// coverLostMsg answers a filing or a save when the staged cover's draft
// is gone: expired past its hour, or lost to a server restart.
const coverLostMsg = "The uploaded cover was lost. Upload it again."

// coverChanged reports whether the form staged a cover that still needs
// applying against an item currently holding current: a different
// choice, or a fresh draft restaging the same one (a new upload
// replacing an old one). The one check both coverImage and applyCover
// below go by, so they can never disagree about it.
func (in itemForm) coverChanged(current library.CoverChoice) bool {
	return library.CoverChoice(in.CoverChoice) != current || in.CoverDraft != ""
}

// coverImage resolves the form's staged cover choice into what
// Service.SetCover needs, before the item itself is created or updated
// (design.md ADR-10): picked and uploaded need the draft's bytes; found
// and removed need none, so nil is correct for them too. It also
// rejects, here rather than inside SetCover, every shape SetCover
// itself would otherwise refuse — an unrecognized choice, or picked
// with no edition link — so a filing or a save never writes the item
// first and only then discovers the cover cannot follow. Every caller
// reaches this only once coverChanged has already said the cover is
// being touched at all (capture.go, shelves.go); callers that skip it
// keep the item's own choice unchanged, which this never needs to see.
func (in itemForm) coverImage(drafts *covers.Drafts) (*library.CoverImage, error) {
	choice := library.CoverChoice(in.CoverChoice)
	switch choice {
	case library.CoverFound, library.CoverRemoved:
		return nil, nil
	case library.CoverPicked, library.CoverUploaded:
		// handled below
	default:
		return nil, &library.ValidationError{Field: "cover", Msg: coverLostMsg}
	}
	img, ok := drafts.Get(in.CoverDraft)
	if in.CoverDraft == "" || !ok || (choice == library.CoverPicked && img.SourceURL == "") {
		return nil, &library.ValidationError{Field: "cover", Msg: coverLostMsg}
	}
	return &library.CoverImage{SourceURL: img.SourceURL, MediaType: "image/jpeg", Bytes: img.Bytes}, nil
}

// applyCover calls Service.SetCover only when coverChanged says the
// form's staged choice actually needs applying against item's own.
// Otherwise item is handed back unchanged: UpdateItem already carried
// CoverURL for a found item, and SetCover has nothing new to do.
//
// This runs as its own call, after CreateItem/UpdateItem's own
// transaction, not inside it: coverImage above has already resolved
// and validated everything SetCover checks, so the only way it can
// still fail here is a real fault (the database itself) or the item
// disappearing underneath between the two calls — the same residual
// risk GetShelf already carries a step later in postItem. Accepted
// for the same reason: two single-purpose transactions stay far
// simpler to read than one spanning both.
func (h *handler) applyCover(ctx context.Context, item *library.Item, in itemForm, img *library.CoverImage) (*library.Item, error) {
	if !in.coverChanged(item.CoverChoice) {
		return item, nil
	}
	return h.svc.SetCover(ctx, item.ID, library.CoverChoice(in.CoverChoice), img)
}

// formMessages turns the library's terse validation messages into copy for
// the form. Unlisted fields keep the library's message.
var formMessages = map[string]string{
	"title":    "Give it a title.",
	"why":      "Write one line on why before filing it.",
	"shelf_id": "Pick a shelf, or add the new one first.",
}

func messageFor(verr *library.ValidationError) string {
	if msg, ok := formMessages[verr.Field]; ok && verr.Msg == "required" {
		return msg
	}
	return verr.Msg
}

// itemFormData is what the "item-fields" block needs to draw itself: the
// shelves its picker offers and the formats its choice row lists.
type itemFormData struct {
	Signals string
	Shelves []library.Shelf
	Formats []library.Format
}

// newItemFormData marshals the signals and gathers the lists the block draws.
func newItemFormData(form itemForm, shelves []library.Shelf) (*itemFormData, error) {
	signals, err := marshalSignals(form)
	if err != nil {
		return nil, err
	}
	return &itemFormData{Signals: signals, Shelves: shelves, Formats: library.Formats}, nil
}
