package web

import (
	"strconv"
	"strings"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/metadata"
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
}

// newItemForm is a blank form filed under the given shelf.
func newItemForm(shelfID, shelfName string) itemForm {
	book := library.Defaults(library.FormatBook)
	return itemForm{
		Format:      string(library.FormatBook),
		ShelfID:     shelfID,
		ShelfName:   shelfName,
		FocusDemand: string(book.FocusDemand),
		NeedsDesk:   book.NeedsDesk,
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
		Title:       item.Title,
		URL:         item.URL,
		Author:      item.Author,
		Format:      string(item.Format),
		ShelfID:     item.ShelfID,
		ShelfName:   shelfName,
		Why:         item.Why,
		Tags:        strings.Join(tags, ", "),
		FocusDemand: string(item.FocusDemand),
		SizeValue:   size,
		NeedsDesk:   item.NeedsDesk,
		CoverURL:    item.CoverURL,
		Errors:      blankErrors(),
		Defaults:    formatDefaults(),
		Pencil:      blankPencil(),
	}
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
func (in itemForm) toItem() (library.Item, []string, error) {
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
		if item.SizeValue == nil && strings.TrimSpace(in.Text) != "" {
			n := metadata.CountWords(in.Text)
			item.SizeValue = &n
		}
		item.WordCount = item.SizeValue // an article's size is its word count
	}
	return item, strings.Split(in.Tags, ","), nil
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
