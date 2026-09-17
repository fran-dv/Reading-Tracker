package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// Home (spec §6.1): the board of hours and speed, what is in progress, and
// the picks that fit the moment. Nothing else. Every change — a filter tap, a Done —
// re-renders the "home-body" block from fresh state with the moment the
// browser sent along, so the filter survives every action and resets only
// on a page load.

// homeForm mirrors the page's signals: the moment, the verdict typed into
// an open Done form, and the reason typed into an open abandon form.
type homeForm struct {
	Moment  momentForm        `json:"moment"`
	Verdict string            `json:"verdict"`
	Reason  string            `json:"reason"`
	Errors  map[string]string `json:"errors"`
}

// entryForm names the one in-progress entry open as a form, if any, and
// which form it is.
type entryForm struct {
	ID      string
	Abandon bool // the abandon form; otherwise the Done form
}

type momentForm struct {
	Time  library.TimeBucket `json:"time"`
	Fried bool               `json:"fried"`
}

// defaultMoment is what a page load starts from: long, not fried.
var defaultMoment = momentForm{Time: library.TimeLong}

func (m momentForm) moment() library.Moment {
	return library.Moment{Time: m.Time, Fried: m.Fried}
}

// readingEntry is one in-progress item as Home draws it.
type readingEntry struct {
	library.Reading
	Read       float64 // share of the item behind the position, 0–1, for the margin mark
	Size       string  // "296 pages", or "" when unrecorded
	From       string  // "from page 120": where to resume
	Last       string  // "read today", "read 12 Sep", or "" before the first session
	Left       string  // "2 h 10 min left", or "" when unknown
	Controls   bool    // offers its actions: no form is open anywhere
	Done       bool    // its Done form is open
	Abandoning bool    // its abandon form is open
	Href       string  // "/items/{id}"; actions hang off it
	Cancel     string  // where an open form's Cancel goes
}

// pickEntry is one shortlisted pool item as Home draws it.
type pickEntry struct {
	library.Pick
	Size string // "296 pages", or "" when unrecorded
	Left string // "45 min", or "" when unknown
	Href string
}

// homeBody is everything an action can change.
type homeBody struct {
	ReviewDue bool   // the weekly review is overdue
	Board     *board // where the discipline stands
	Reading   []readingEntry
	Picks     []pickEntry
	Signals   string
	Status    string // one line about what just happened
}

type homePage struct {
	shell
	Body *homeBody
}

func (h *handler) getHome(w http.ResponseWriter, r *http.Request) {
	body, err := h.homeBody(r.Context(), defaultMoment, entryForm{}, "")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.home, homePage{shell: newShell("/"), Body: body})
}

// getHomeBody re-renders for the moment the browser holds. It answers a
// filter tap, and it is how a Done form is cancelled.
func (h *handler) getHomeBody(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	h.patchHome(w, r, in.Moment, entryForm{}, "", nil)
}

// getDone opens the verdict form on one in-progress entry.
func (h *handler) getDone(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	if !h.patchHome(w, r, in.Moment, entryForm{ID: r.PathValue("id")}, "", nil) {
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.ExecuteScript(`document.getElementById("verdict").focus()`); err != nil {
		h.log.Error("verdict focus", "err", err)
	}
}

// getAbandon opens the abandon form on one in-progress entry.
func (h *handler) getAbandon(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	if !h.patchHome(w, r, in.Moment, entryForm{ID: r.PathValue("id"), Abandon: true}, "", nil) {
		return
	}
	h.focusReason(datastar.NewSSE(w, r))
}

// postAbandon closes an item that no longer earns its place, with its
// reason (spec §2.1). Home offers it so the WIP cap never waits for the
// weekly review (§7.5).
func (h *handler) postAbandon(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	item, err := h.svc.Abandon(r.Context(), r.PathValue("id"), in.Reason)
	if errors.Is(err, library.ErrReasonRequired) {
		h.reasonMissing(w, r)
		return
	}
	status := ""
	if err == nil {
		status = "Abandoned " + item.Title + "."
	}
	h.patchHome(w, r, in.Moment, entryForm{}, status, err)
}

// postFinish completes an item; postReference closes it as material
// consulted rather than read through. Both take the optional verdict.
func (h *handler) postFinish(w http.ResponseWriter, r *http.Request) {
	h.complete(w, r, h.svc.Finish, "Finished %s.")
}

func (h *handler) postReference(w http.ResponseWriter, r *http.Request) {
	h.complete(w, r, h.svc.Reference, "Kept %s for reference.")
}

func (h *handler) complete(w http.ResponseWriter, r *http.Request, close func(context.Context, string, string) (*library.Item, error), done string) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	item, err := close(r.Context(), r.PathValue("id"), in.Verdict)
	status := ""
	if err == nil {
		status = fmt.Sprintf(done, item.Title)
	}
	h.patchHome(w, r, in.Moment, entryForm{}, status, err)
}

// postStartItem takes a pick into progress (spec §7.5). At the WIP cap the
// status line says so instead.
func (h *handler) postStartItem(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	item, err := h.svc.Start(ctx, r.PathValue("id"))
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
	}
	h.patchHome(w, r, in.Moment, entryForm{}, status, err)
}

// readHome reads the page's signals, answering 400 when they do not parse.
func (h *handler) readHome(w http.ResponseWriter, r *http.Request) (homeForm, bool) {
	var in homeForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return in, false
	}
	return in, true
}

// patchHome answers an action: on success the body is rebuilt from fresh
// state for the given moment and patched. It reports whether the patch was sent.
func (h *handler) patchHome(w http.ResponseWriter, r *http.Request, m momentForm, open entryForm, status string, err error) bool {
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	body, err := h.homeBody(r.Context(), m, open, status)
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.home, "home-body", body); err != nil {
		h.log.Error("home body", "err", err)
		return false
	}
	// A morph keeps an unchanged seed, so typed text is cleared by hand.
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("home reset", "err", err)
		return false
	}
	return true
}

// homeBody gathers the screen as it should be drawn for a moment. open,
// when set, names the in-progress entry whose form is open; no entry then
// shows its controls.
func (h *handler) homeBody(ctx context.Context, m momentForm, open entryForm, status string) (*homeBody, error) {
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	loc, err := settings.Location()
	if err != nil {
		return nil, err
	}
	view, err := h.svc.Home(ctx, m.moment())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	body := &homeBody{ReviewDue: view.ReviewDue, Board: newBoard(view.Schedule, view.Speed, settings.WordsPerPage), Status: status}
	for _, entry := range view.Reading {
		out := readingEntry{
			Reading:    entry,
			Read:       readShare(entry.Item, entry.Position),
			Size:       sizeLabel(entry.Item),
			From:       fromLabel(entry.Item, entry.Position),
			Controls:   open.ID == "",
			Done:       entry.Item.ID == open.ID && !open.Abandon,
			Abandoning: entry.Item.ID == open.ID && open.Abandon,
			Href:       "/items/" + entry.Item.ID,
			Cancel:     "/home/body",
		}
		if entry.LastReadAt != nil {
			out.Last = "read " + dayLabel(*entry.LastReadAt, now, loc)
		}
		if entry.Remaining.Known() {
			out.Left = minutesLabel(entry.Remaining.Remaining) + " left"
		}
		body.Reading = append(body.Reading, out)
	}
	for _, pick := range view.Picks {
		out := pickEntry{Pick: pick, Size: sizeLabel(pick.Item), Href: "/items/" + pick.Item.ID}
		if pick.Remaining.Known() {
			out.Left = minutesLabel(pick.Remaining.Remaining)
		}
		body.Picks = append(body.Picks, out)
	}
	if body.Signals, err = marshalSignals(homeForm{Moment: m, Errors: map[string]string{"reason": ""}}); err != nil {
		return nil, err
	}
	return body, nil
}

// readShare is how much of the item lies behind the position, 0–1, for the
// mark on the margin rule. Zero when the size is unknown.
func readShare(item library.Item, position int) float64 {
	if item.SizeValue == nil || *item.SizeValue <= 0 {
		return 0
	}
	return min(float64(position)/float64(*item.SizeValue), 1)
}

// dayLabel names a day relative to now: "today", "yesterday", else the date.
func dayLabel(t, now time.Time, loc *time.Location) string {
	day := t.In(loc).Format("2006-01-02")
	switch day {
	case now.In(loc).Format("2006-01-02"):
		return "today"
	case now.In(loc).AddDate(0, 0, -1).Format("2006-01-02"):
		return "yesterday"
	}
	return t.In(loc).Format("2 Jan")
}

// reasonMissing answers an abandon with no reason: the message lands under
// the open form, which keeps its place, and focus returns to the field.
func (h *handler) reasonMissing(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(map[string]any{"errors": map[string]string{"reason": "Say why, in a line."}}); err != nil {
		h.log.Error("abandon reason", "err", err)
		return
	}
	h.focusReason(sse)
}

// focusReason puts the cursor in the open abandon form.
func (h *handler) focusReason(sse *datastar.ServerSentEventGenerator) {
	if err := sse.ExecuteScript(`document.getElementById("reason").focus()`); err != nil {
		h.log.Error("reason focus", "err", err)
	}
}
