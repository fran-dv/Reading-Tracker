package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The shelf view (spec §6.2): three slots on top, the pool below, borrowed
// items marked, each entry showing its why.
//
// Every action — ranking and editing alike — re-renders one "shelf-body"
// block from fresh state and patches it. A rank can move an item between the
// two halves and renumber the slots, so rendering the pair together is the
// only way the two cannot disagree.

// shelfRow is one item as the shelf draws it.
type shelfRow struct {
	library.ShelfItem
	Slot     int    // 1–3 for a shelf leader, 0 in the pool
	From     string // the home shelf's name when this item is borrowed
	Size     string // "240 pages", or "" when unrecorded
	Reading  bool   // in progress: already being read
	Controls bool   // this row offers its controls: nothing else is being edited
	CanStart bool   // a pool item: it can be started
	CanRank  bool   // a pool item, and a slot is free to take
	AtTop    bool   // slot 1: cannot move up
	AtBottom bool   // the last filled slot: cannot move down
	Href     string // "/shelves/{shelf}/items/{item}"; actions hang off it

	Form   *itemFormData // non-nil when this row is open as a form
	Cancel string        // where Cancel goes, set with Form
}

// shelfSlot is one of the three ranks: its number, and its item if it has one.
type shelfSlot struct {
	Number int
	Row    *shelfRow
}

// shelfBody is everything a rank or an edit can change.
type shelfBody struct {
	Shelf     library.Shelf
	Slots     []shelfSlot // always three; Row is nil where the slot is empty
	Pool      []shelfRow
	SlotsFull bool
	Empty     bool   // nothing on the shelf at all
	Editing   bool   // a row is open as a form, so no other row offers controls
	Status    string // one line about what just happened, when it is not visible
}

type shelvesPage struct {
	shell
	Shelves []library.Shelf
}

type shelfPage struct {
	shell
	Body *shelfBody
}

// getShelves lists the shelves. Names only: a door, not a dashboard (§0).
func (h *handler) getShelves(w http.ResponseWriter, r *http.Request) {
	shelves, err := h.svc.ListShelves(r.Context())
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.shelves, shelvesPage{shell: newShell("/shelves"), Shelves: shelves})
}

func (h *handler) getShelf(w http.ResponseWriter, r *http.Request) {
	body, err := h.shelfBody(r.Context(), r.PathValue("id"), "")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.shelf, shelfPage{shell: newShell("/shelves"), Body: body})
}

// getShelfBody re-renders with nothing open. It is how an edit is cancelled.
func (h *handler) getShelfBody(w http.ResponseWriter, r *http.Request) {
	h.patchBody(w, r, r.PathValue("id"), "", "", nil)
}

// getEntryForm opens one entry as a form and puts the cursor in its title.
func (h *handler) getEntryForm(w http.ResponseWriter, r *http.Request) {
	if !h.patchBody(w, r, r.PathValue("id"), r.PathValue("itemID"), "", nil) {
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.ExecuteScript(`document.getElementById("title").focus()`); err != nil {
		h.log.Error("entry form focus", "err", err)
	}
}

// postEntry saves an edited entry. Validation problems answer 200 with error
// signals, leaving the open form exactly as it was typed.
func (h *handler) postEntry(w http.ResponseWriter, r *http.Request) {
	var in itemForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	shelfID, itemID := r.PathValue("id"), r.PathValue("itemID")

	item, tags, err := in.toItem()
	if err == nil && item.ShelfID == newShelfID {
		err = &library.ValidationError{Field: "shelf_id", Msg: "required"}
	}
	var saved *library.Item
	if err == nil {
		item.ID = itemID
		saved, err = h.svc.UpdateItem(ctx, item, tags)
	}
	if h.formError(w, r, err) {
		return
	}
	status := ""
	if err == nil && saved.ShelfID != shelfID {
		shelf, err := h.svc.GetShelf(ctx, saved.ShelfID)
		if err != nil {
			h.httpError(w, r, err)
			return
		}
		status = saved.Title + " moved to " + shelf.Name + "."
	}
	h.patchBody(w, r, shelfID, "", status, err)
}

// postStart moves a pool item to in_progress (spec §7.5). It leaves any slot
// it held, and the status line says which slot opened up: the app prompts
// to fill it, it does not auto-select (§5.1). At the WIP cap the line says
// so instead.
func (h *handler) postStart(w http.ResponseWriter, r *http.Request) {
	ctx, shelfID, itemID := r.Context(), r.PathValue("id"), r.PathValue("itemID")
	view, err := h.svc.ShelfItems(ctx, shelfID)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	held, last := 0, 0
	for i, slot := range view.Slots {
		if slot == nil {
			break
		}
		last = i + 1
		if slot.ID == itemID {
			held = last
		}
	}

	item, err := h.svc.Start(ctx, itemID)
	status := ""
	switch {
	case errors.Is(err, library.ErrWIPCapReached):
		settings, serr := h.svc.Settings(ctx)
		if serr != nil {
			h.httpError(w, r, serr)
			return
		}
		status = fmt.Sprintf("Already %d in progress. Finish or abandon one first.", settings.WIPCap)
		err = nil
	case err == nil:
		status = "Started " + item.Title + "."
		if held > 0 {
			status += fmt.Sprintf(" Slot %d is free.", last)
		}
	}
	h.patchBody(w, r, shelfID, "", status, err)
}

// postRank puts a pool item into the shelf's lowest empty slot (§5.1).
func (h *handler) postRank(w http.ResponseWriter, r *http.Request) {
	ctx, shelfID := r.Context(), r.PathValue("id")
	view, err := h.svc.ShelfItems(ctx, shelfID)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	slot := freeSlot(view)
	if slot == 0 {
		http.Error(w, "all three slots are taken", http.StatusConflict)
		return
	}
	h.patchBody(w, r, shelfID, "", "", h.svc.Rank(ctx, shelfID, r.PathValue("itemID"), slot))
}

// postUnrank releases a slot and shifts the ones below it up.
func (h *handler) postUnrank(w http.ResponseWriter, r *http.Request) {
	shelfID := r.PathValue("id")
	h.patchBody(w, r, shelfID, "", "", h.svc.Unrank(r.Context(), shelfID, r.PathValue("itemID")))
}

func (h *handler) postMoveUp(w http.ResponseWriter, r *http.Request) {
	shelfID := r.PathValue("id")
	h.patchBody(w, r, shelfID, "", "", h.svc.MoveUp(r.Context(), shelfID, r.PathValue("itemID")))
}

func (h *handler) postMoveDown(w http.ResponseWriter, r *http.Request) {
	shelfID := r.PathValue("id")
	h.patchBody(w, r, shelfID, "", "", h.svc.MoveDown(r.Context(), shelfID, r.PathValue("itemID")))
}

// patchBody answers an action: on success the shelf body is rebuilt from
// fresh state and patched, with editingID naming the row to open, if any.
// It reports whether the patch was sent.
func (h *handler) patchBody(w http.ResponseWriter, r *http.Request, shelfID, editingID, status string, err error) bool {
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	body, err := h.shelfBody(r.Context(), shelfID, editingID)
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	body.Status = status
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.shelf, "shelf-body", body); err != nil {
		h.log.Error("shelf body", "err", err)
		return false
	}
	return true
}

// shelfBody gathers the shelf as it should be drawn. editingID, when set,
// names the one row that renders as a form; every other row then shows no
// controls, so a stray click cannot discard what is being typed.
func (h *handler) shelfBody(ctx context.Context, shelfID, editingID string) (*shelfBody, error) {
	view, err := h.svc.ShelfItems(ctx, shelfID)
	if err != nil {
		return nil, err
	}
	shelves, err := h.svc.ListShelves(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(shelves))
	for _, s := range shelves {
		names[s.ID] = s.Name
	}

	free := freeSlot(view)
	body := &shelfBody{
		Shelf:     view.Shelf,
		Slots:     make([]shelfSlot, library.MaxSlots),
		SlotsFull: free == 0,
		Empty:     free == 1 && len(view.Unranked) == 0,
		Editing:   editingID != "",
	}
	last := free - 1 // the last filled slot; MaxSlots when all three are taken
	if body.SlotsFull {
		last = library.MaxSlots
	}
	row := func(item library.ShelfItem, slot int) (shelfRow, error) {
		out := shelfRow{
			ShelfItem: item,
			Slot:      slot,
			Size:      sizeLabel(item.Item),
			Reading:   item.State == library.StateInProgress,
			Controls:  editingID == "",
			CanStart:  item.State == library.StatePool && editingID == "",
			CanRank:   slot == 0 && item.State == library.StatePool && !body.SlotsFull && editingID == "",
			AtTop:     slot == 1,
			AtBottom:  slot == last,
			Href:      "/shelves/" + shelfID + "/items/" + item.ID,
		}
		if item.Borrowed {
			out.From = names[item.ShelfID]
		}
		if item.ID != editingID {
			return out, nil
		}
		tags, err := h.svc.Tags(ctx, item.ID)
		if err != nil {
			return out, err
		}
		form, err := newItemFormData(itemFormFor(&item.Item, tags, names[item.ShelfID]), shelves)
		if err != nil {
			return out, err
		}
		out.Form, out.Cancel = form, "/shelves/"+shelfID+"/body"
		return out, nil
	}

	for i := range body.Slots {
		body.Slots[i].Number = i + 1
		if view.Slots[i] == nil {
			continue
		}
		out, err := row(*view.Slots[i], i+1)
		if err != nil {
			return nil, err
		}
		body.Slots[i].Row = &out
	}
	for _, item := range view.Unranked {
		out, err := row(item, 0)
		if err != nil {
			return nil, err
		}
		body.Pool = append(body.Pool, out)
	}
	return body, nil
}

// freeSlot is the lowest empty slot, or 0 when all three are taken.
func freeSlot(view *library.ShelfView) int {
	for i, slot := range view.Slots {
		if slot == nil {
			return i + 1
		}
	}
	return 0
}

// sizeLabel writes an item's size in its own unit, or "" when unrecorded.
func sizeLabel(item library.Item) string {
	if item.SizeValue == nil {
		return ""
	}
	return grouped(*item.SizeValue) + " " + string(item.SizeUnit)
}

// grouped puts thin spaces between thousands so long figures stay readable.
func grouped(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + " " + s[i:]
	}
	return s
}
