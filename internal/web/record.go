package web

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// The record (spec §6.11): what the reading has reached over the years.
// Every goal is kept, met or not, stated as it went.

type recordBody struct {
	Books     int
	Summary   []string // "3 other items", "6 480 pages", "120 h of reading on 94 days", "since 20 Aug 2026"
	Campaigns []goalRow
	Hours     []goalRow
	Speed     []goalRow
	Years     []yearRow
	Bests     []bestRow
}

// goalRow is one goal and how it went.
type goalRow struct {
	When    string  // "20 Aug 2026 – 20 Sep 2027"
	Name    string  // "100 books by 20 Sep 2027", "30 min → 4 h 00 min a day"
	Result  string  // "met 28 Aug, 2 weeks early"
	Met     bool    // the result reads as reached
	Detail  string  // "halfway 3 Mar", "held 2 weeks"
	Fill    float64 // how far it got, 0–1
	HasFill bool
}

type yearRow struct {
	Year                       int
	Books, Other, Pages, Hours string
	Goals                      string
}

type bestRow struct {
	Label, Figure, Behind string
}

type recordPage struct {
	shell
	Body *recordBody
}

func (h *handler) getRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rec, err := h.svc.Record(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	b := &recordBody{Books: rec.Books}
	if rec.Other > 0 {
		b.Summary = append(b.Summary, countLabel(rec.Other, "other item"))
	}
	if rec.Pages > 0 {
		b.Summary = append(b.Summary, grouped(rec.Pages)+" pages")
	}
	if rec.Time > 0 {
		b.Summary = append(b.Summary, minutesLabel(rec.Time)+" of reading on "+countLabel(rec.Days, "day"))
	}
	if !rec.Since.IsZero() {
		b.Summary = append(b.Summary, "since "+rec.Since.Format("2 Jan 2006"))
	}
	for _, c := range rec.Campaigns {
		b.Campaigns = append(b.Campaigns, campaignRecord(c))
	}
	for _, j := range rec.HoursRamps {
		b.Hours = append(b.Hours, hoursRecord(j))
	}
	for _, j := range rec.SpeedRamps {
		b.Speed = append(b.Speed, speedRecord(j))
	}
	for _, y := range rec.Years {
		b.Years = append(b.Years, yearRow{
			Year: y.Year, Books: fmt.Sprint(y.Books), Other: fmt.Sprint(y.Other), Pages: grouped(y.Pages),
			Hours: fmt.Sprintf("%.0f h", math.Floor(y.Time.Hours())), Goals: fmt.Sprint(y.Goals),
		})
	}
	bests := rec.Bests
	if it := bests.LongestBook; it != nil {
		b.Bests = append(b.Bests, bestRow{Label: "Longest book finished", Figure: sizeLabel(*it), Behind: it.Title})
	}
	if bests.DayTime > 0 {
		b.Bests = append(b.Bests, bestRow{Label: "Most in a day", Figure: minutesLabel(bests.DayTime), Behind: bests.Day.Format("Monday 2 Jan 2006")})
	}
	if bests.WeekTime > 0 {
		b.Bests = append(b.Bests, bestRow{Label: "Most in a week", Figure: minutesLabel(bests.WeekTime), Behind: "the week of " + bests.Week.Format("2 Jan 2006")})
	}
	if bests.Index > 1 { // at or under the baseline is no best
		b.Bests = append(b.Bests, bestRow{Label: "Best speed week", Figure: percent(bests.Index) + " of your baseline", Behind: "the week of " + bests.IndexWeek.Format("2 Jan 2006")})
	}
	h.render(w, r, h.record, recordPage{shell: h.newShell(ctx, "/record"), Body: b})
}

func campaignRecord(c library.CampaignRecord) goalRow {
	cs := c.State
	camp := cs.Campaign
	g := goalRow{
		When: camp.StartedOn.Format("2 Jan 2006") + " – " + camp.Deadline.Format("2 Jan 2006"),
		Name: camp.Name, Fill: math.Min(1, float64(cs.Finished)/float64(camp.TargetCount)), HasFill: true,
	}
	switch {
	case !c.MetOn.IsZero():
		g.Met = true
		g.Result = fmt.Sprintf("met %s, %s", c.MetOn.Format("2 Jan"), early(c.MetOn, camp.Deadline))
		if cs.Finished > camp.TargetCount {
			g.Result += fmt.Sprintf(" · %d in all", cs.Finished)
		}
	case !camp.Active():
		g.Result = fmt.Sprintf("ended %s with %d of %d", camp.EndedOn.Format("2 Jan 2006"), cs.Finished, camp.TargetCount)
	case cs.Over:
		g.Result = fmt.Sprintf("the deadline passed with %d of %d", cs.Finished, camp.TargetCount)
	default:
		g.Result = fmt.Sprintf("under way: %d of %d", cs.Finished, camp.TargetCount)
	}
	if !c.HalfwayOn.IsZero() {
		g.Detail = "halfway on " + c.HalfwayOn.Format("2 Jan 2006")
	}
	return g
}

func hoursRecord(j library.HoursJourney) goalRow {
	g := goalRow{
		Name:    minutesLabel(minutes(j.Start)) + " → " + minutesLabel(minutes(j.Ceiling)) + " a day",
		Fill:    rampFill(j.Start, j.Value, j.Ceiling),
		HasFill: true,
		Detail:  heldLabel(j.Holds),
	}
	switch {
	case !j.ReachedOn.IsZero():
		g.Met = true
		g.When = j.Began.Format("2 Jan 2006") + " – " + j.ReachedOn.Format("2 Jan 2006")
		g.Result = fmt.Sprintf("reached its top after %s", spanLabelDays(daysBetween(j.Began, j.ReachedOn)-1))
	case !j.EndedOn.IsZero():
		g.When = j.Began.Format("2 Jan 2006") + " – " + j.EndedOn.Format("2 Jan 2006")
		g.Result = "replaced at " + minutesLabel(minutes(j.Value)) + " a day"
	default:
		g.When = "since " + j.Began.Format("2 Jan 2006")
		g.Result = "rising: at " + minutesLabel(minutes(j.Value)) + " a day"
	}
	return g
}

func speedRecord(j library.SpeedJourney) goalRow {
	r := j.Ramp
	g := goalRow{
		Name:    fmt.Sprintf("100%% → %d%% of your baseline", r.CeilingPercent),
		Fill:    rampFill(100, j.Target, r.CeilingPercent),
		HasFill: true,
		Detail:  heldLabel(j.Holds),
	}
	switch {
	case !j.ReachedOn.IsZero():
		g.Met = true
		g.When = r.StartedOn.Format("2 Jan 2006") + " – " + j.ReachedOn.Format("2 Jan 2006")
		g.Result = fmt.Sprintf("reached its top after %s", spanLabelDays(daysBetween(r.StartedOn, j.ReachedOn)-1))
	case j.Running:
		g.When = "since " + r.StartedOn.Format("2 Jan 2006")
		g.Result = fmt.Sprintf("rising: at %d%%", j.Target)
	default:
		end := time.Time{}
		if r.StoppedOn != nil {
			end = *r.StoppedOn
		}
		g.When = r.StartedOn.Format("2 Jan 2006") + " – " + end.Format("2 Jan 2006")
		g.Result = fmt.Sprintf("stopped at %d%%", j.Target)
	}
	return g
}

// rampFill is how far a ramp got from its start toward its ceiling, 0–1.
func rampFill(start, value, ceiling int) float64 {
	if ceiling <= start {
		return 1
	}
	return math.Max(0, math.Min(1, float64(value-start)/float64(ceiling-start)))
}
