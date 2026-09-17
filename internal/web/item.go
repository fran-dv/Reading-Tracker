package web

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The book page (spec §6.9): one item with what its sessions say — where
// the reading stands, how fast it goes, when it finishes at the rate it is
// read lately — and every session, correctable in place.

// bookForm mirrors the page's signals. ItemPage names the item, which is how
// a correction made here knows to redraw this page.
type bookForm struct {
	ItemPage string            `json:"itemPage"`
	Edit     editForm          `json:"edit"`
	Errors   map[string]string `json:"errors"`
}

type itemBody struct {
	Item      library.Item
	Shelf     string // its shelf's name
	ShelfHref string
	Tags      string // "classics, novels"
	State     string // "reading", "in the pool", "finished 12 Sep"…
	Closing   string // the verdict or the reason it was abandoned
	Size      string // "300 pages"
	Read      float64
	Position  string // "page 90 of 300"
	Dates     []string
	Stalled   bool
	Pace      string // "22 pages/h over 4 h 00 min"; "" when unmeasured
	NoPace    string // why there is no pace: "none: it is measured in minutes", or what to log
	Left      string // "9 h 20 min left"
	LeftBasis string // "at its own pace", "provisional, at the default pace"…
	Tentative bool   // the estimate is provisional or rough
	Finish    string // "About 29 Oct, at its last two weeks' 13 min a day."
	Time      string // "4 h 00 min over 4 days, 4 sessions"
	Reading   bool   // in progress: offers Read
	Rows      []sessionRow
	Signals   string
	Status    string
}

type itemPage struct {
	shell
	Body *itemBody
}

// itemState is what an action leaves for the book page it redraws.
type itemState struct {
	ID      string
	Status  string
	Editing string // the session whose edit form is open
}

func (h *handler) getItem(w http.ResponseWriter, r *http.Request) {
	body, err := h.itemBody(r.Context(), itemState{ID: r.PathValue("id")})
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.item, itemPage{shell: h.newShell(r.Context(), ""), Body: body})
}

// getItemBody redraws the page as it stands: it cancels an edit.
func (h *handler) getItemBody(w http.ResponseWriter, r *http.Request) {
	h.patchItem(w, r, itemState{ID: r.PathValue("id")}, nil)
}

func (h *handler) patchItem(w http.ResponseWriter, r *http.Request, st itemState, err error) bool {
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	body, err := h.itemBody(r.Context(), st)
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.item, "item-body", body); err != nil {
		h.log.Error("item body", "err", err)
		return false
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("item reset", "err", err)
	}
	return true
}

func (h *handler) itemBody(ctx context.Context, st itemState) (*itemBody, error) {
	loc, err := h.location(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.svc.ItemPage(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	it := p.Item
	now := time.Now()
	b := &itemBody{
		Item: it, Shelf: p.ShelfName, ShelfHref: "/shelves/" + it.ShelfID, Tags: strings.Join(p.Tags, ", "),
		Size: sizeLabel(it), Read: readShare(it, p.Position), Stalled: p.Stalled,
		Reading: it.State == library.StateInProgress, Status: st.Status,
	}
	b.State, b.Closing = stateLabel(it, now, loc)
	if it.SizeValue != nil {
		b.Position = positionLabel(it, p.Position) + " of " + positionNumber(it, *it.SizeValue)
	} else if p.Position > 0 {
		b.Position = positionLabel(it, p.Position)
	}
	if it.StartedAt != nil {
		b.Dates = append(b.Dates, "started "+dayLabel(*it.StartedAt, now, loc))
	}
	if p.LastReadAt != nil && it.State == library.StateInProgress {
		b.Dates = append(b.Dates, "read "+dayLabel(*p.LastReadAt, now, loc))
	}
	switch {
	case p.Pace > 0:
		b.Pace = paceLabel(it, p.Pace) + " over " + minutesLabel(p.PaceTime)
	case it.SizeUnit == library.UnitMinutes:
		b.NoPace = "none: it is measured in minutes"
	default:
		b.NoPace = "not measured: log the " + unitWord(it.SizeUnit) + " you reach"
	}
	if b.Reading && p.Remaining.Known() {
		b.Left = minutesLabel(p.Remaining.Remaining) + " left"
		b.LeftBasis = map[library.Basis]string{
			library.BasisItem: "at its own pace",
			library.BasisBand: "at the pace of similar " + string(it.Format) + "s",
			library.BasisSeed: "provisional, at the default pace for its focus",
		}[p.Remaining.Basis]
		if p.Remaining.Rough {
			b.LeftBasis += ", from under an hour: rough"
		}
		b.Tentative = p.Remaining.Provisional() || p.Remaining.Rough
	}
	if b.Reading {
		switch {
		case !p.FinishOn.IsZero():
			b.Finish = fmt.Sprintf("About %s, at the %s a day it got over the last two weeks.", p.FinishOn.Format("2 Jan"), minutesLabel(p.Recent))
		case p.Remaining.Known() && p.Remaining.Remaining == 0:
			b.Finish = "At the end: finish it from Home."
		default:
			b.Finish = "Not read in the last two weeks, so no date."
		}
	}
	if len(p.Sessions) > 0 {
		b.Time = minutesLabel(p.Total) + " over " + countLabel(p.Days, "day") + ", " + countLabel(len(p.Sessions), "session")
	}

	form := bookForm{ItemPage: it.ID, Errors: sessionErrors()}
	for _, s := range slices.Backward(p.Sessions) {
		logged := library.LoggedSession{Session: s, Item: it}
		row := newSessionRow(logged, loc)
		row.Span = s.StartedAt.In(loc).Format("2 Jan") + " · " + row.Span
		row.Cancel, row.OnItsPage = "/items/"+it.ID+"/body", true
		if s.ID == st.Editing && !s.Running() {
			row.Editing, form.Edit = true, editFormFor(logged, loc)
		}
		b.Rows = append(b.Rows, row)
	}
	if b.Signals, err = marshalSignals(form); err != nil {
		return nil, err
	}
	return b, nil
}

// stateLabel says where an item is in its life, and the closing line it was
// given, if any.
func stateLabel(it library.Item, now time.Time, loc *time.Location) (state, closing string) {
	switch it.State {
	case library.StatePool:
		return "in the pool", ""
	case library.StateInProgress:
		return "reading", ""
	case library.StateFinished:
		return "finished " + dayLabel(*it.FinishedAt, now, loc), it.Verdict
	case library.StateReference:
		return "kept for reference " + dayLabel(*it.FinishedAt, now, loc), it.Verdict
	}
	return "abandoned " + dayLabel(*it.FinishedAt, now, loc), it.AbandonedReason
}

// paceLabel writes an item's pace in its own unit: "22 pages/h", "230 words/min".
func paceLabel(it library.Item, perHour float64) string {
	if it.SizeUnit == library.UnitWords {
		return fmt.Sprintf("%.0f words/min", perHour/60)
	}
	return fmt.Sprintf("%.0f %s/h", perHour, it.SizeUnit)
}
