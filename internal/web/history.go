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

// The grid shows at least these hours, widened to fit every session, and
// measured in rem: an hour read in is gridHourRem tall, and a run of hours
// nothing was read in anywhere that week squeezes to one quiet band, so a
// single session at 3 a.m. doesn't stretch the week by five empty hours.
const (
	gridFirstHour = 8
	gridLastHour  = 22
	gridHourRem   = 2.75
	gridQuietRem  = 1.5
	gridShortRem  = 2.3 // under this a block writes its title and length on one line
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
	Bands    []gridBand
	Height   float64 // rem the grid stands
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
	Top   float64 // rem down the grid
}

// gridBand is one stretch of the grid behind the days: an hour, ruled at
// its top, or a run of hours nothing was read in, hatched.
type gridBand struct {
	Top    float64 // rem
	Height float64 // rem
	Quiet  bool
}

type gridDay struct {
	Name    string // "Sun"
	Date    string // "13"
	Of      string // "1:07 of 0:45", "1:07", or "–" for a day to come
	Href    string // chooses the day
	Today   bool
	Chosen  bool
	Short   bool // a closed planned day that read less than its target
	Rest    bool // planned, not an active day
	Future  bool
	Blocks  []gridBlock
	Now     float64 // rem down the grid; only today, and only while it is on the grid
	Ticking bool
}

type gridBlock struct {
	ID       string
	Title    string
	Format   library.Format
	Length   string  // "25 min"
	Brief    string  // "25", "1:05": all a phone column fits
	Top      float64 // rem down the grid
	Height   float64 // rem
	Short    bool    // too short for the title and the length to sit on separate lines
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
func (h *handler) patchHistory(w http.ResponseWriter, r *http.Request, st historyState, err error) bool {
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	body, err := h.historyBody(r.Context(), st)
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.history, "history-body", body); err != nil {
		h.log.Error("history body", "err", err)
		return false
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("history reset", "err", err)
	}
	return true
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

	now := time.Now()
	axis := newGridAxis(v, loc, now)
	b.Hours, b.Bands, b.Height = axis.Hours, axis.Bands, axis.Height
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
			gd.Blocks = append(gd.Blocks, newGridBlock(s, axis, loc, now, st.Editing))
		}
		if gd.Today {
			at := now.In(loc)
			gd.Now = axis.at(float64(at.Hour()*60 + at.Minute()))
			gd.Ticking = gd.Now > 0 && gd.Now < axis.Height
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

// gridAxis lays the week's hours out down the grid. Everything it hands
// back is in rem, because a squeezed axis has no single hour height for a
// percentage to mean anything against.
type gridAxis struct {
	Hours  []hourMark
	Bands  []gridBand
	Height float64
	spans  []gridSpan
}

// gridSpan is a band's place on the clock, so a time can be put on the grid.
type gridSpan struct {
	from, to  float64 // minutes since midnight
	top, tall float64 // rem down the grid, rem tall
}

// newGridAxis covers the default day, widened to the earliest start and the
// latest end of the week's sessions, with every run of two or more hours
// nothing was read in squeezed to one quiet band. No session can begin or
// end inside such a run, so a block is never drawn against a squeezed hour.
func newGridAxis(v *library.HistoryView, loc *time.Location, now time.Time) gridAxis {
	first, last := gridFirstHour, gridLastHour
	read := map[int]bool{}
	for _, d := range v.Days {
		for _, s := range d.Sessions {
			start := s.StartedAt.In(loc)
			first = min(first, start.Hour())
			end := start.Add(s.Elapsed(now))
			endHour := 24 // a session past midnight is cut at the grid's end
			if end.Day() == start.Day() {
				endHour = end.Hour()
				if end.Minute() > 0 {
					endHour++ // the grid ends on the hour after
				}
			}
			last = max(last, endHour)
			for hr := start.Hour(); hr < endHour; hr++ {
				read[hr] = true
			}
		}
	}

	var a gridAxis
	var quiet []int
	squeeze := func() {
		switch {
		case len(quiet) == 0:
			return
		case len(quiet) > 1:
			a.add(quiet[0], len(quiet), gridQuietRem, true)
		default:
			a.add(quiet[0], 1, gridHourRem, false)
		}
		quiet = quiet[:0]
	}
	for hr := first; hr < last; hr++ {
		if !read[hr] {
			quiet = append(quiet, hr)
			continue
		}
		squeeze()
		a.add(hr, 1, gridHourRem, false)
	}
	squeeze()
	a.Hours = append(a.Hours, hourMark{Label: hourLabel(last), Top: a.Height})
	return a
}

// add lays one band under the last, labelling it unless it is quiet.
func (a *gridAxis) add(hr, hours int, height float64, quiet bool) {
	a.Bands = append(a.Bands, gridBand{Top: a.Height, Height: height, Quiet: quiet})
	a.spans = append(a.spans, gridSpan{from: float64(hr) * 60, to: float64(hr+hours) * 60, top: a.Height, tall: height})
	if !quiet {
		a.Hours = append(a.Hours, hourMark{Label: hourLabel(hr), Top: a.Height})
	}
	a.Height += height
}

// at is where a time of day, in minutes since midnight, sits on the grid.
func (a gridAxis) at(m float64) float64 {
	for _, s := range a.spans {
		if m < s.to {
			if m <= s.from {
				return s.top
			}
			return s.top + s.tall*(m-s.from)/(s.to-s.from)
		}
	}
	return a.Height
}

func hourLabel(hr int) string { return fmt.Sprintf("%02d:00", hr%24) }

// briefLabel is all a phone's day column fits: the minutes alone, or the
// clock past an hour.
func briefLabel(d time.Duration) string {
	if m := int(d.Round(time.Minute).Minutes()); m < 60 {
		return fmt.Sprintf("%d", m)
	}
	return clock(d)
}

func newGridBlock(s library.LoggedSession, a gridAxis, loc *time.Location, now time.Time, editing string) gridBlock {
	start := s.StartedAt.In(loc)
	length := s.Elapsed(now)
	from := float64(start.Hour()*60 + start.Minute())
	top := a.at(from)
	height := a.at(from+length.Minutes()) - top
	return gridBlock{
		ID: s.ID, Title: s.Item.Title, Format: s.Item.Format,
		Length: minutesLabel(length), Brief: briefLabel(length),
		Top: top, Height: height, Short: height < gridShortRem,
		Running: s.Running(), Href: dayHref(dayOfLocal(start)) + "#session-" + s.ID, Selected: s.ID == editing,
	}
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
