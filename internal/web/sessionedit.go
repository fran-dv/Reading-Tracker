package web

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// Correcting sessions (spec §2.3, §6.4): the session screen lists today's
// sessions, each editable in place or deletable, and a status line about a
// session just logged offers Undo.

// editForm mirrors the open edit form.
type editForm struct {
	Start   string `json:"start"`   // datetimeLocal, in the configured timezone
	Minutes string `json:"minutes"` // how long, typed the way it is said
	Reached string `json:"reached"` // "" records time only
	Note    string `json:"note"`
}

// sessionRow is one logged session as a list draws it.
type sessionRow struct {
	ID        string
	ItemID    string
	Title     string
	Format    library.Format
	Unit      library.SizeUnit
	Span      string // "20:10–21:00", or "since 20:10" while running
	Length    string // "50 min"; "" while running
	Progress  string // "page 100 → 150", "back to page 60", or ""
	Note      string
	Edited    bool
	Running   bool
	Editing   bool   // its edit form is open
	Cancel    string // what closes the edit form: the page's body route
	OnItsPage bool   // drawn on its item's own page, which needs no title
}

// today lists today's sessions in loc, newest first. editing names the
// session whose form is open; its values seed the returned form.
func (h *handler) today(ctx context.Context, loc *time.Location, editing string) ([]sessionRow, editForm, error) {
	now := time.Now().In(loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	logged, err := h.svc.SessionsBetween(ctx, from, from.AddDate(0, 0, 1))
	if err != nil {
		return nil, editForm{}, err
	}
	var rows []sessionRow
	var form editForm
	for _, s := range slices.Backward(logged) {
		row := newSessionRow(s, loc)
		row.Cancel = "/session/body"
		if s.ID == editing && !s.Running() {
			row.Editing = true
			form = editFormFor(s, loc)
		}
		rows = append(rows, row)
	}
	return rows, form, nil
}

func newSessionRow(s library.LoggedSession, loc *time.Location) sessionRow {
	row := sessionRow{
		ID: s.ID, ItemID: s.Item.ID, Title: s.Item.Title, Format: s.Item.Format, Unit: s.Item.SizeUnit,
		Note: s.Note, Edited: s.EditedAt != nil, Running: s.Running(),
	}
	start := s.StartedAt.In(loc).Format("15:04")
	if row.Running {
		row.Span = "since " + start
	} else {
		row.Span = start + "–" + s.EndedAt.In(loc).Format("15:04")
		row.Length = minutesLabel(s.Duration())
	}
	if s.PositionEnd != nil && s.PositionStart != nil {
		if _, ok := s.ProgressDelta(); ok {
			row.Progress = positionLabel(s.Item, *s.PositionStart) + " → " + positionNumber(s.Item, *s.PositionEnd)
		} else {
			row.Progress = "back to " + positionLabel(s.Item, *s.PositionEnd)
		}
	}
	return row
}

// positionLabel names a position in an item's unit: "page 120", "word 2,300",
// or "1 h 12 min" in something measured in minutes.
func positionLabel(item library.Item, n int) string {
	if item.SizeUnit == library.UnitMinutes {
		return minutesLabel(time.Duration(n) * time.Minute)
	}
	return unitWord(item.SizeUnit) + " " + grouped(n)
}

// positionNumber is a position without its unit word, for the far side of
// "page 100 → 150".
func positionNumber(item library.Item, n int) string {
	if item.SizeUnit == library.UnitMinutes {
		return minutesLabel(time.Duration(n) * time.Minute)
	}
	return grouped(n)
}

// editFormFor seeds the edit form with a session as it was logged.
func editFormFor(s library.LoggedSession, loc *time.Location) editForm {
	form := editForm{
		Start:   s.StartedAt.In(loc).Format(datetimeLocal),
		Minutes: minutesField(max(1, int(s.Duration().Round(time.Minute).Minutes()))), // under half a minute still offers a length
		Note:    s.Note,
	}
	if s.PositionEnd != nil {
		form.Reached = strconv.Itoa(*s.PositionEnd)
	}
	return form
}

// getSessionBody redraws the screen as it stands; it is how an edit form is
// cancelled.
func (h *handler) getSessionBody(w http.ResponseWriter, r *http.Request) {
	h.patchSession(w, r, sessionState{}, nil)
}

// getEditSession opens a closed session's edit form in place.
func (h *handler) getEditSession(w http.ResponseWriter, r *http.Request) {
	var in sessionForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.patchCorrected(w, r, in, "", r.PathValue("id"), nil)
	sse := datastar.NewSSE(w, r)
	if err := sse.ExecuteScript(`document.getElementById("edit-minutes")?.focus()`); err != nil {
		h.log.Error("edit focus", "err", err)
	}
}

// postEditSession saves a correction under the rules the session was logged
// by, and marks it edited.
func (h *handler) postEditSession(w http.ResponseWriter, r *http.Request) {
	var in sessionForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	loc, err := h.location(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	start, err := time.ParseInLocation(datetimeLocal, in.Edit.Start, loc)
	if err != nil {
		h.sessionError(w, r, "editStart", "When did it start?")
		return
	}
	minutes, ok := parseMinutes(in.Edit.Minutes)
	if !ok || minutes < 1 {
		h.sessionError(w, r, "editMinutes", "How long? Try 1h30, 1:30 or 90.")
		return
	}
	unit := library.UnitPages
	if session, err := h.svc.GetSession(ctx, id); err == nil {
		if item, err := h.svc.GetItem(ctx, session.ItemID); err == nil {
			unit = item.SizeUnit
		}
	}
	reached, ok := position(in.Edit.Reached, unit)
	if !ok {
		h.sessionError(w, r, "editReached", positionMessage(unit))
		return
	}

	e := library.SessionEdit{Start: start, End: start.Add(time.Duration(minutes) * time.Minute), Reached: reached, Note: in.Edit.Note}
	session, err := h.svc.EditSession(ctx, id, e)
	if field, msg, ok := h.sessionProblem(ctx, err, loc); ok {
		slot := map[string]string{"time": "editStart", "reached": "editReached"}[field]
		if errors.Is(err, library.ErrTooLong) {
			slot = "editMinutes"
		}
		h.sessionError(w, r, slot, msg)
		return
	}
	status := ""
	if err == nil {
		status = "Saved " + minutesLabel(session.Duration()) + " on " + h.titleOf(ctx, session.ItemID) + "."
	}
	h.patchCorrected(w, r, in, status, "", err)
}

// postDeleteSession removes a session: from its dialog, from Undo, or a
// running timer discarded. A second tap from a stale page finds nothing and
// just redraws.
func (h *handler) postDeleteSession(w http.ResponseWriter, r *http.Request) {
	var in sessionForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	session, err := h.svc.DeleteSession(ctx, r.PathValue("id"))
	if errors.Is(err, library.ErrNotFound) {
		h.patchCorrected(w, r, in, "", "", nil)
		return
	}
	status := ""
	if err == nil {
		title := h.titleOf(ctx, session.ItemID)
		status = "Removed " + minutesLabel(session.Duration()) + " on " + title + "."
		if session.Running() {
			status = "Discarded the timer on " + title + "."
		}
	}
	h.patchCorrected(w, r, in, status, "", err)
}

// patchCorrected redraws the page a correction came from: a book page or
// History when its signals name one, else Session.
func (h *handler) patchCorrected(w http.ResponseWriter, r *http.Request, in sessionForm, status, editing string, err error) {
	if in.ItemPage != "" {
		h.patchItem(w, r, itemState{ID: in.ItemPage, Status: status, Editing: editing}, err)
		return
	}
	if in.History.Day != "" {
		h.patchHistory(w, r, historyState{Day: in.History.Day, Status: status, Editing: editing}, err)
		return
	}
	h.patchSession(w, r, sessionState{Status: status, Editing: editing}, err)
}
