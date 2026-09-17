package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The session screen (spec §6.4): the timer and retroactive entry, side by
// side with equal weight. "Now" starts and stops the timer; "Earlier" logs
// reading done away from it. Both forms pick from the items in progress.
//
// Every successful action re-renders the whole "session-body" block from
// fresh state and resets the form signals. Validation problems patch error
// signals only, so nothing typed is lost.

// datetimeLocal is the value format of an <input type="datetime-local">.
const datetimeLocal = "2006-01-02T15:04"

// sessionForm mirrors both forms. The same struct seeds them, reads them
// back on submit, and resets them.
type sessionForm struct {
	Now      nowForm           `json:"now"`
	Earlier  earlierForm       `json:"earlier"`
	Edit     editForm          `json:"edit"`
	History  historyRef        `json:"history"`  // set on History, which a correction redraws
	ItemPage string            `json:"itemPage"` // set on a book page, which a correction redraws
	Errors   map[string]string `json:"errors"`
}

// nowForm is the timer: which item to start, and on stop, where it got to.
type nowForm struct {
	ItemID    string `json:"itemId"`
	ItemTitle string `json:"itemTitle"` // shown on the picker
	Reached   string `json:"reached"`   // a string, as inputs give it; "" is empty
	Note      string `json:"note"`
	StoppedAt string `json:"stoppedAt"` // datetimeLocal; "" stops now
}

// earlierForm is retroactive entry: a stretch of reading that already ended.
type earlierForm struct {
	ItemID    string `json:"itemId"`
	ItemTitle string `json:"itemTitle"`
	From      string `json:"from"` // "from page 120": the hint beside Reached
	Minutes   string `json:"minutes"`
	EndedAt   string `json:"endedAt"` // datetimeLocal, in the configured timezone
	Reached   string `json:"reached"`
	Note      string `json:"note"`
}

// sessionErrors lists every error slot on the page, so one patch clears them all.
func sessionErrors() map[string]string {
	return map[string]string{"item": "", "reached": "", "stoppedAt": "", "logItem": "", "minutes": "", "endedAt": "", "logReached": "",
		"editStart": "", "editMinutes": "", "editReached": ""}
}

// sessionInputs maps an error slot to the input that fixes it.
var sessionInputs = map[string]string{
	"item": "now-button", "reached": "reached", "stoppedAt": "stopped-at",
	"logItem": "earlier-button", "minutes": "minutes", "endedAt": "ended-at", "logReached": "log-reached",
	"editStart": "edit-start", "editMinutes": "edit-minutes", "editReached": "edit-reached",
}

// readingRow is one item the pickers offer.
type readingRow struct {
	library.Reading
	From string // "from page 120"
}

// pickerData is what the "item-picker" block needs: the form it serves and
// the rows it offers. Form is typed template.JS so that "$now.itemId" reads
// as code inside data-on:* expressions, which html/template treats as
// JavaScript; in every other attribute it is escaped like any string.
type pickerData struct {
	Form template.JS // "now" or "earlier"
	Rows []readingRow
}

// pickerFor is the template function behind the block.
func pickerFor(form string, rows []readingRow) pickerData {
	return pickerData{Form: template.JS(form), Rows: rows}
}

// runningView is the timer while it runs.
type runningView struct {
	Session library.Session
	Item    library.Item
	From    string // "from page 120"
	Since   string // "21:03", in the configured timezone
	SinceMS int64  // started_at as epoch milliseconds, for the ticking clock
	// StartedLocal is started_at as a datetime-local value: where "Stopped
	// earlier?" starts the stop time from.
	StartedLocal string
	Clock        string // elapsed so far, "42:13" or "1:02:13"
}

// sessionBody is everything an action can change.
type sessionBody struct {
	Running *runningView // nil while the timer is idle
	Reading []readingRow // what the pickers offer, most recently read first
	Today   []sessionRow // today's sessions, newest first
	Signals string
	Status  string // one line about what just happened
	Undo    string // the session Status reports, when it can be undone
}

// sessionState is what an action leaves for the screen it redraws.
type sessionState struct {
	ItemID  string // the item the pickers start on
	Status  string // one line about what just happened
	Undo    string // the session that line can undo
	Editing string // the session whose edit form is open
}

type sessionPage struct {
	shell
	Body *sessionBody
}

// getSession draws the screen. ?item=ID preselects that item in both pickers,
// which is how Home hands an item over.
func (h *handler) getSession(w http.ResponseWriter, r *http.Request) {
	body, err := h.sessionBody(r.Context(), sessionState{ItemID: r.URL.Query().Get("item")})
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.session, sessionPage{shell: h.newShell(r.Context(), "/session"), Body: body})
}

// postStartSession starts the timer on the picked item.
func (h *handler) postStartSession(w http.ResponseWriter, r *http.Request) {
	var in sessionForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := h.svc.StartSession(r.Context(), in.Now.ItemID)
	var running *library.SessionRunningError
	switch {
	case errors.Is(err, library.ErrNotFound), errors.Is(err, library.ErrItemNotInProgress):
		h.sessionError(w, r, "item", "Pick something that is in progress.")
		return
	case errors.As(err, &running):
		err = nil // a stale tab: the fresh body shows what is running
	}
	h.patchSession(w, r, sessionState{}, err)
}

// postStopSession stops the running timer, recording where it got to.
func (h *handler) postStopSession(w http.ResponseWriter, r *http.Request) {
	var in sessionForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	unit := library.UnitPages
	if running, err := h.svc.RunningSession(ctx); err == nil && running != nil {
		if item, err := h.svc.GetItem(ctx, running.ItemID); err == nil {
			unit = item.SizeUnit
		}
	}
	reached, ok := position(in.Now.Reached, unit)
	if !ok {
		h.sessionError(w, r, "reached", positionMessage(unit))
		return
	}
	loc, err := h.location(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	stop := library.Stop{Reached: reached, Note: in.Now.Note}
	if in.Now.StoppedAt != "" {
		if stop.At, err = time.ParseInLocation(datetimeLocal, in.Now.StoppedAt, loc); err != nil {
			h.sessionError(w, r, "stoppedAt", "When did you stop reading?")
			return
		}
	}
	session, err := h.svc.StopSession(ctx, r.PathValue("id"), stop)
	if errors.Is(err, library.ErrInvalidTransition) {
		err = nil // already stopped from another tab
	}
	if field, msg, ok := h.sessionProblem(ctx, err, loc); ok {
		slot := map[string]string{"time": "stoppedAt", "reached": "reached"}[field]
		h.sessionError(w, r, slot, msg)
		return
	}
	st := sessionState{}
	if err == nil && session != nil {
		st.Status, st.Undo = h.logged(ctx, session), session.ID
	}
	h.patchSession(w, r, st, err)
}

// postSession logs a session that already happened.
func (h *handler) postSession(w http.ResponseWriter, r *http.Request) {
	var in sessionForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	minutes, ok := parseMinutes(in.Earlier.Minutes)
	if !ok || minutes < 1 {
		h.sessionError(w, r, "minutes", "How long? Try 1h30, 1:30 or 90.")
		return
	}
	loc, err := h.location(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	end, err := time.ParseInLocation(datetimeLocal, in.Earlier.EndedAt, loc)
	if err != nil {
		h.sessionError(w, r, "endedAt", "When did it end?")
		return
	}
	unit := library.UnitPages
	if item, err := h.svc.GetItem(ctx, in.Earlier.ItemID); err == nil {
		unit = item.SizeUnit
	}
	reached, ok := position(in.Earlier.Reached, unit)
	if !ok {
		h.sessionError(w, r, "logReached", positionMessage(unit))
		return
	}

	start := end.Add(-time.Duration(minutes) * time.Minute)
	session, err := h.svc.AddRetroactiveSession(ctx, in.Earlier.ItemID, start, end, reached, in.Earlier.Note)
	if errors.Is(err, library.ErrNotFound) || errors.Is(err, library.ErrItemNotInProgress) {
		h.sessionError(w, r, "logItem", "Pick something that is in progress.")
		return
	}
	if field, msg, ok := h.sessionProblem(ctx, err, loc); ok {
		slot := map[string]string{"time": "endedAt", "reached": "logReached"}[field]
		if errors.Is(err, library.ErrTooLong) {
			slot = "minutes"
		}
		h.sessionError(w, r, slot, msg)
		return
	}
	st := sessionState{}
	if err == nil {
		st.Status, st.Undo = h.logged(ctx, session), session.ID
	}
	h.patchSession(w, r, st, err)
}

// logged is the status line after a session is saved.
func (h *handler) logged(ctx context.Context, session *library.Session) string {
	return "Logged " + minutesLabel(session.Duration()) + " on " + h.titleOf(ctx, session.ItemID) + "."
}

// titleOf is an item's title for a status line; a status line never fails
// a request that already succeeded.
func (h *handler) titleOf(ctx context.Context, itemID string) string {
	item, err := h.svc.GetItem(ctx, itemID)
	if err != nil {
		h.log.Error("status title", "err", err)
		return "it"
	}
	return item.Title
}

// sessionProblem puts a refused session into words (spec §2.3), and says
// whether the fix is in its time ("time") or in the position reached
// ("reached"). ok is false when err is not about the session itself.
func (h *handler) sessionProblem(ctx context.Context, err error, loc *time.Location) (field, msg string, ok bool) {
	var overlap *library.OverlapError
	var running *library.SessionRunningError
	switch {
	case err == nil:
		return "", "", false
	case errors.Is(err, library.ErrInvalidRange):
		return "time", "It has to end after it started.", true
	case errors.Is(err, library.ErrInFuture):
		return "time", "That is still to come.", true
	case errors.Is(err, library.ErrTooLong):
		return "time", "A session runs 16 h at most. Check the times, or log it in parts.", true
	case errors.Is(err, library.ErrPastEnd):
		return "reached", "That is past the end.", true
	case errors.As(err, &running):
		return "time", "Overlaps the timer that is running. Stop it first, or end this before it started.", true
	case errors.As(err, &overlap):
		title := "another session"
		if item, err := h.svc.GetItem(ctx, overlap.With.ItemID); err == nil {
			title = item.Title
		}
		return "time", "Overlaps " + spanLabel(overlap.With, loc) + " on " + title + ".", true
	}
	return "", "", false
}

// spanLabel writes when a closed session ran: "Mon 14 Sep, 20:10–21:00".
func spanLabel(s library.Session, loc *time.Location) string {
	start, end := s.StartedAt.In(loc), s.EndedAt.In(loc)
	return start.Format("Mon 2 Jan, 15:04") + "–" + end.Format("15:04")
}

// sessionError reports a validation failure in band: the message lands in
// its slot and focus moves to the field that fixes it.
func (h *handler) sessionError(w http.ResponseWriter, r *http.Request, field, msg string) {
	errs := sessionErrors()
	errs[field] = msg
	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(map[string]any{"errors": errs}); err != nil {
		h.log.Error("session errors", "err", err)
		return
	}
	if err := sse.ExecuteScript(`document.getElementById("` + sessionInputs[field] + `").focus()`); err != nil {
		h.log.Error("session error focus", "err", err)
	}
}

// patchSession answers an action: on success the body is rebuilt from fresh
// state and patched, and the forms are reset.
func (h *handler) patchSession(w http.ResponseWriter, r *http.Request, st sessionState, err error) {
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	body, err := h.sessionBody(r.Context(), st)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.session, "session-body", body); err != nil {
		h.log.Error("session body", "err", err)
		return
	}
	// The block carries the same seed; patching it again is what resets
	// fields whose seed did not change since the last render.
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("session reset", "err", err)
	}
}

// sessionBody gathers the screen as it should be drawn. The pickers start
// on st.ItemID when it names an item in progress, else on the most recently
// read.
func (h *handler) sessionBody(ctx context.Context, st sessionState) (*sessionBody, error) {
	loc, err := h.location(ctx)
	if err != nil {
		return nil, err
	}
	reading, err := h.svc.Reading(ctx)
	if err != nil {
		return nil, err
	}
	running, err := h.svc.RunningSession(ctx)
	if err != nil {
		return nil, err
	}

	body := &sessionBody{Status: st.Status, Undo: st.Undo}
	form := sessionForm{Errors: sessionErrors()}
	for _, entry := range reading {
		body.Reading = append(body.Reading, readingRow{Reading: entry, From: fromLabel(entry.Item, entry.Position)})
	}
	lead := 0 // the most recently read, unless itemID names another
	for i, row := range body.Reading {
		if row.Item.ID == st.ItemID {
			lead = i
		}
	}
	if len(reading) > 0 {
		row := body.Reading[lead]
		form.Now.ItemID, form.Now.ItemTitle = row.Item.ID, row.Item.Title
		form.Earlier.ItemID, form.Earlier.ItemTitle, form.Earlier.From = row.Item.ID, row.Item.Title, row.From
	}
	form.Earlier.EndedAt = time.Now().In(loc).Format(datetimeLocal)

	if running != nil {
		item, err := h.svc.GetItem(ctx, running.ItemID)
		if err != nil {
			return nil, err
		}
		body.Running = &runningView{
			Session:      *running,
			Item:         *item,
			From:         fromLabel(*item, *running.PositionStart),
			Since:        running.StartedAt.In(loc).Format("15:04"),
			SinceMS:      running.StartedAt.UnixMilli(),
			StartedLocal: running.StartedAt.In(loc).Format(datetimeLocal),
			Clock:        clockLabel(time.Since(running.StartedAt)),
		}
	}
	if body.Today, form.Edit, err = h.today(ctx, loc, st.Editing); err != nil {
		return nil, err
	}
	if body.Signals, err = marshalSignals(form); err != nil {
		return nil, err
	}
	return body, nil
}

// location is the configured timezone.
func (h *handler) location(ctx context.Context) (*time.Location, error) {
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	return settings.Location()
}

// position parses an optional position field: nil when empty. In something
// measured in minutes it reads the time a player shows ("1:12:30").
func position(s string, unit library.SizeUnit) (*int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, true
	}
	if unit == library.UnitMinutes {
		n, ok := parseTimestamp(s)
		return &n, ok
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return nil, false
	}
	return &n, true
}

// positionMessage is the error for a position that doesn't read.
func positionMessage(unit library.SizeUnit) string {
	if unit == library.UnitMinutes {
		return "Type the time the player shows, like 1:12:30."
	}
	return "Use a whole number."
}

// fromLabel says where an item's reading stands, in its own unit.
func fromLabel(item library.Item, position int) string {
	if item.SizeUnit == library.UnitMinutes {
		return "from " + minutesLabel(time.Duration(position)*time.Minute)
	}
	return "from " + unitWord(item.SizeUnit) + " " + grouped(position)
}

// unitWord is the singular of a size unit, for "page 120".
func unitWord(u library.SizeUnit) string {
	return strings.TrimSuffix(string(u), "s")
}

// minutesLabel writes a duration in minutes, or hours and minutes.
func minutesLabel(d time.Duration) string {
	m := int(d.Round(time.Minute).Minutes())
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d h %02d min", m/60, m%60)
}

// clockLabel writes elapsed time the way the running clock shows it:
// "42:13", or "1:02:13" past an hour.
func clockLabel(d time.Duration) string {
	s := int(d.Seconds())
	if s < 0 {
		s = 0
	}
	if s < 3600 {
		return fmt.Sprintf("%d:%02d", s/60, s%60)
	}
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}
