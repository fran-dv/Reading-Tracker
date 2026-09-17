package web

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// History (spec §6.8): a week of sessions as a calendar. Each day is a
// column of blocks at the time they were read, headed by what it read
// against its target; the chosen day lists its sessions to correct; and the
// week closes with what its time went to. ?day=2006-01-02 picks the day and
// so the week; without it, today.

// historyForm mirrors the page's signals. Day names the chosen day, which is
// how a correction made here knows to redraw History rather than Session.
type historyForm struct {
	History historyRef        `json:"history"`
	Edit    editForm          `json:"edit"`
	Errors  map[string]string `json:"errors"`
}

// historyRef is the day History is showing; "" on any other page.
type historyRef struct {
	Day string `json:"day"` // dateField
}

// The grid shows at least these hours, widened to fit every session.
const (
	gridFirstHour = 8
	gridLastHour  = 22
)

type historyBody struct {
	Range    string // "13–19 Sep"
	Prev     string // href of the week before; "" at the first week
	Next     string // href of the week after; "" at the current week
	ThisWeek string // href back to today; "" when showing it
	Logged   string // "7 h 00 min"
	Target   string // "5 h 15 min"; "" without a plan that week
	Short    bool   // a closed week that read less than its target
	Parts    []weekPart
	Hours    []hourMark
	Span     int // hours the grid covers
	Days     []gridDay
	Chosen   chosenDay
	Signals  string
	Status   string
}

// weekPart is one item's share of the week, for the bar and its table.
type weekPart struct {
	ItemID   string
	Title    string
	Format   library.Format
	Time     string  // "2 h 47 min"
	Percent  string  // "40%"
	Grow     float64 // flex-grow for the bar
	Progress string  // "131 pages", or ""
	Sessions string  // "3 sessions"
}

type hourMark struct {
	Label string  // "09:00"
	Top   float64 // percent down the grid
}

type gridDay struct {
	Name   string // "Sun"
	Date   string // "13"
	Of     string // "1:07 of 0:45", "1:07", or "–" for a day to come
	Href   string // chooses the day
	Today  bool
	Chosen bool
	Short  bool // a closed planned day that read less than its target
	Rest   bool // planned, not an active day
	Future bool
	Blocks []gridBlock
}

type gridBlock struct {
	ID       string
	Title    string
	Format   library.Format
	Length   string  // "25 min"
	Top      float64 // percent down the grid
	Height   float64 // percent of the grid
	Running  bool
	Href     string // chooses its day and scrolls to its row
	Selected bool   // its edit form is open
}

type chosenDay struct {
	Label  string // "Thursday 17 September"
	Of     string // "1 h 15 min of 45 min", or "1 h 15 min"
	Rows   []sessionRow
	Target bool
}

type historyPage struct {
	shell
	Body *historyBody
}

// historyState is what an action leaves for the History screen it redraws.
type historyState struct {
	Day     string // dateField; "" is today
	Status  string
	Editing string // the session whose edit form is open
}

func (h *handler) getHistory(w http.ResponseWriter, r *http.Request) {
	body, err := h.historyBody(r.Context(), historyState{Day: r.URL.Query().Get("day")})
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.history, historyPage{shell: h.newShell(r.Context(), "/history"), Body: body})
}

// getHistoryBody redraws the chosen day as it stands: it cancels an edit.
func (h *handler) getHistoryBody(w http.ResponseWriter, r *http.Request) {
	var in historyForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.patchHistory(w, r, historyState{Day: in.History.Day}, nil)
}

// patchHistory answers an action taken on History.
func (h *handler) patchHistory(w http.ResponseWriter, r *http.Request, st historyState, err error) {
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	body, err := h.historyBody(r.Context(), st)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.history, "history-body", body); err != nil {
		h.log.Error("history body", "err", err)
		return
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("history reset", "err", err)
	}
}

func (h *handler) historyBody(ctx context.Context, st historyState) (*historyBody, error) {
	loc, err := h.location(ctx)
	if err != nil {
		return nil, err
	}
	chosen, _ := time.Parse(dateField, st.Day) // the zero day, today, when absent
	v, err := h.svc.History(ctx, chosen)
	if err != nil {
		return nil, err
	}
	if chosen.IsZero() {
		chosen = v.Today
	}
	weekEnd := v.WeekStart.AddDate(0, 0, 7)
	if chosen.Before(v.WeekStart) || !chosen.Before(weekEnd) {
		chosen = v.WeekStart // the week was clamped: choose its first day
	}
	if chosen.After(v.Today) {
		chosen = v.Today
	}

	b := &historyBody{
		Range:  weekDates(v.WeekStart, weekEnd),
		Logged: minutesLabel(v.Logged),
		Status: st.Status,
	}
	if v.WeekStart.After(v.Earliest) {
		b.Prev = dayHref(v.WeekStart.AddDate(0, 0, -7))
	}
	if !weekEnd.After(v.Today) {
		b.Next, b.ThisWeek = dayHref(weekEnd), "/history"
	}
	if v.Target > 0 {
		b.Target = minutesLabel(minutes(v.Target))
		b.Short = weekEnd.Before(v.Today.AddDate(0, 0, 1)) && v.Logged < minutes(v.Target)
	}
	var total time.Duration
	for _, it := range v.Items {
		total += it.Time
	}
	for _, it := range v.Items {
		p := weekPart{ItemID: it.Item.ID, Title: it.Item.Title, Format: it.Item.Format, Time: minutesLabel(it.Time),
			Percent: fmt.Sprintf("%.0f%%", 100*share(it.Time, total)), Grow: math.Round(1000 * share(it.Time, total)),
			Sessions: countLabel(it.Sessions, "session")}
		if it.Progress > 0 && it.Item.SizeUnit != library.UnitMinutes {
			p.Progress = grouped(it.Progress) + " " + string(it.Item.SizeUnit)
		}
		b.Parts = append(b.Parts, p)
	}

	first, last := gridHours(v, loc)
	b.Span = last - first
	for hr := first; hr <= last; hr++ {
		b.Hours = append(b.Hours, hourMark{Label: fmt.Sprintf("%02d:00", hr), Top: 100 * float64(hr-first) / float64(b.Span)})
	}
	form := historyForm{History: historyRef{Day: chosen.Format(dateField)}, Errors: sessionErrors()}
	for _, d := range v.Days {
		gd := gridDay{
			Name: d.Day.Format("Mon"), Date: d.Day.Format("2"), Href: dayHref(d.Day),
			Today: d.Day.Equal(v.Today), Chosen: d.Day.Equal(chosen), Future: d.Day.After(v.Today),
			Rest: d.Planned && !d.Active, Of: "–",
		}
		if !gd.Future {
			gd.Of = clock(d.Logged)
			if d.Planned && d.Active {
				gd.Of += " of " + clock(minutes(d.Target))
				gd.Short = d.Closed && d.Logged < minutes(d.Target)
			}
		}
		for _, s := range d.Sessions {
			gd.Blocks = append(gd.Blocks, newGridBlock(s, first, b.Span, loc, st.Editing))
		}
		b.Days = append(b.Days, gd)
		if gd.Chosen {
			b.Chosen = chosenDay{Label: d.Day.Format("Monday 2 January"), Of: minutesLabel(d.Logged), Target: d.Planned && d.Active}
			if b.Chosen.Target {
				b.Chosen.Of += " of " + minutesLabel(minutes(d.Target))
			}
			for _, s := range slices.Backward(d.Sessions) {
				row := newSessionRow(s, loc)
				row.Cancel = "/history/body"
				if s.ID == st.Editing && !s.Running() {
					row.Editing, form.Edit = true, editFormFor(s, loc)
				}
				b.Chosen.Rows = append(b.Chosen.Rows, row)
			}
		}
	}
	if b.Signals, err = marshalSignals(form); err != nil {
		return nil, err
	}
	return b, nil
}

// gridHours is the span of hours the week's grid covers: the default day,
// widened to the earliest start and the latest end of its sessions.
func gridHours(v *library.HistoryView, loc *time.Location) (first, last int) {
	first, last = gridFirstHour, gridLastHour
	for _, d := range v.Days {
		for _, s := range d.Sessions {
			start := s.StartedAt.In(loc)
			first = min(first, start.Hour())
			end := start.Add(sessionLength(s))
			if end.Day() != start.Day() {
				last = 24
				continue
			}
			endHour := end.Hour()
			if end.Minute() > 0 {
				endHour++ // the grid ends on the hour after
			}
			last = max(last, endHour)
		}
	}
	return first, last
}

func newGridBlock(s library.LoggedSession, first, span int, loc *time.Location, editing string) gridBlock {
	start := s.StartedAt.In(loc)
	fromTop := float64(start.Hour()-first)*60 + float64(start.Minute())
	length := sessionLength(s)
	height := min(length.Minutes(), float64(span*60)-fromTop) // a session past midnight is cut at the grid's end
	return gridBlock{
		ID: s.ID, Title: s.Item.Title, Format: s.Item.Format, Length: minutesLabel(length),
		Top: 100 * fromTop / float64(span*60), Height: 100 * height / float64(span*60),
		Running: s.Running(), Href: dayHref(dayOfLocal(start)) + "#session-" + s.ID, Selected: s.ID == editing,
	}
}

// sessionLength is how long a session has run: its duration, or the time
// since its start while it runs.
func sessionLength(s library.LoggedSession) time.Duration {
	if s.Running() {
		return time.Since(s.StartedAt)
	}
	return s.Duration()
}

// dayOfLocal is the calendar day of a local time, carried as midnight UTC
// like the library's days.
func dayOfLocal(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// dayHref is History on a day.
func dayHref(day time.Time) string { return "/history?day=" + day.Format(dateField) }

// countLabel writes "1 session", "3 sessions".
func countLabel(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
