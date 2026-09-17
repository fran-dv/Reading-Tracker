package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The campaign on the plan (spec §6.7, §8.1, §8.5): the count so far, where
// recent reading lands, and what the campaign needs each week, every figure
// traced to what it is built from. The daily target never follows it; the
// target form offers to match it instead.

// campaignForm mirrors the campaign's signals, nested under "campaign".
type campaignForm struct {
	ID       string `json:"id"` // the campaign shown, for rename and end
	Name     string `json:"name"`
	Target   string `json:"target"`
	Deadline string `json:"deadline"` // "2006-01-02", from a date input
	Start    string `json:"start"`
}

// dateField is how a date input reads and writes a calendar day.
const dateField = "2006-01-02"

// campaignView is the Campaign section.
type campaignView struct {
	Active bool
	Over   bool   // active, but its deadline has passed
	Last   string // a note on the campaign ended last; "" otherwise

	Name, Count, Target string
	Line                string  // under the count: deadline, time left, where it is heading
	Fill                float64 // books finished, 0–1 of the target
	Lands               float64 // where the projection lands, 0–1
	ShowLands           bool
	ToGo, ByThen        string // "77 to go", "61 by then"

	Week     *campaignWeek // nil when over
	Rows     []campaignRow
	EndLines []string // what ending it means, for the confirmation dialog
}

// campaignWeek is recent book hours against what the campaign needs.
type campaignWeek struct {
	Read, Needed string
	Fill, Mark   float64
	Gap          string // "28 h 30 min short a week", or "meets it"
	Closed       bool   // a week of reading has closed
}

// campaignRow is one line of "What it needs": a figure and what it is built from.
type campaignRow struct {
	Label, Figure, Behind string
	Strong, Provisional   bool
}

// newCampaignView draws the current campaign; nil before the first.
func newCampaignView(cs *library.CampaignState, sc library.Schedule) *campaignView {
	if cs == nil {
		return nil
	}
	c := cs.Campaign
	if !c.Active() {
		return &campaignView{Last: fmt.Sprintf("The last campaign, %s, ended on %s with %d of %d.",
			c.Name, c.EndedOn.Format("2 Jan 2006"), cs.Finished, c.TargetCount)}
	}
	v := &campaignView{
		Active: true,
		Over:   cs.Over,
		Name:   c.Name,
		Count:  fmt.Sprint(cs.Finished),
		Target: fmt.Sprint(c.TargetCount),
		Fill:   math.Min(1, float64(cs.Finished)/float64(c.TargetCount)),
	}
	deadline := c.Deadline.Format("2 Jan 2006")
	if cs.Over {
		v.Line = fmt.Sprintf("The deadline, %s, has passed. This is the final count.", deadline)
		v.EndLines = []string{
			fmt.Sprintf("Its count is final at %d of %d.", cs.Finished, c.TargetCount),
			"Ending files it away, so you can start a new campaign.",
		}
		return v
	}

	r, p := cs.Required, cs.Projection
	v.Line = fmt.Sprintf("Deadline %s, %s left. ", deadline, timeLeft(cs.WeeksLeft))
	v.EndLines = []string{
		fmt.Sprintf("You have %d of %d, with %s left.", cs.Finished, c.TargetCount, timeLeft(cs.WeeksLeft)),
		"Ending stops the count and the projection today, and it can't be picked up again. Books already counted stay.",
	}
	v.ToGo = fmt.Sprintf("%d to go", r.BooksLeft)
	if r.BooksLeft == 0 {
		v.ToGo = "target reached"
	}
	v.Week = &campaignWeek{Needed: minutesLabel(hours(r.WeeklyHours))}
	if p == nil {
		v.Line += "Where it's heading shows once a week of reading has closed."
	} else {
		v.Line += fmt.Sprintf("At your %s of reading, %d by then.", lastWeeks(p.Weeks), p.Books)
		v.ByThen = fmt.Sprintf("%d by then", p.Books)
		v.Lands, v.ShowLands = math.Min(1, float64(p.Books)/float64(c.TargetCount)), true

		read, needed := hours(p.WeeklyBookHours), hours(r.WeeklyHours)
		scale := max(read, needed)
		w := v.Week
		w.Closed, w.Read = true, minutesLabel(read)
		w.Fill, w.Mark = share(read, scale), share(needed, scale)
		w.Gap = "meets it"
		if gap := needed - read; gap >= time.Minute {
			w.Gap = minutesLabel(gap) + " short a week"
		}
	}

	pages := map[library.PagesBasis]string{
		library.PagesWaiting:  "the books waiting",
		library.PagesFinished: "books finished, none waiting has a size",
		library.PagesSetting:  "a guess, until books have sizes",
	}
	pace := campaignRow{Label: "Book pace", Figure: fmt.Sprintf("%.0f pages/h", r.Pace.PagesPerHour)}
	if r.Pace.Provisional {
		pace.Provisional = true
		pace.Behind = "until 2 h are measured"
	} else {
		var mix []string
		for _, m := range r.Pace.Mix {
			mix = append(mix, fmt.Sprintf("%s %.0f%%", m.FocusDemand, m.Share*100))
		}
		pace.Behind = "over " + minutesLabel(r.Pace.Measured) + ": " + strings.Join(mix, ", ")
	}
	committed := campaignRow{Label: "Committed each week", Figure: "–", Behind: "no daily target yet"}
	if sc.Planned() {
		committed.Figure, committed.Behind = minutesLabel(minutes(sc.CommittedWeek())), ""
		if p != nil {
			committed.Behind = fmt.Sprintf("%.0f%% of it on books lately", p.BookShare*100)
		}
	}
	recent := campaignRow{Label: "Recent weeks", Figure: "–", Behind: "no closed week yet"}
	if p != nil {
		recent = campaignRow{Label: capitalize(lastWeeks(p.Weeks)), Figure: minutesLabel(hours(p.WeeklyBookHours)), Behind: "of books a week"}
	}
	v.Rows = []campaignRow{
		{Label: "Books left", Figure: fmt.Sprint(r.BooksLeft), Behind: fmt.Sprintf("%d finished of %d", cs.Finished, c.TargetCount)},
		{Label: "Average book", Figure: fmt.Sprintf("%.0f pages", r.AvgPages), Behind: "from " + pages[r.PagesBasis]},
		pace,
		{Label: "Book hours left", Figure: minutesLabel(hours(r.HoursLeft)), Behind: fmt.Sprintf("%d books × %.0f pages ÷ %.0f pages/h", r.BooksLeft, r.AvgPages, r.Pace.PagesPerHour)},
		{Label: "Needed each week", Figure: minutesLabel(hours(r.WeeklyHours)) + " of books", Behind: "over the " + timeLeft(cs.WeeksLeft) + " left", Strong: true},
		committed,
		recent,
	}
	return v
}

// hours turns fractional hours into a duration.
func hours(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }

// timeLeft writes what remains to a deadline: "26 weeks", or "9 days" under two weeks.
func timeLeft(weeks float64) string {
	if weeks >= 2 {
		return fmt.Sprintf("%d weeks", int(weeks))
	}
	if days := int(math.Ceil(weeks * 7)); days != 1 {
		return fmt.Sprintf("%d days", days)
	}
	return "1 day"
}

// lastWeeks names the weeks a projection averages: "last 4 weeks", "last week".
func lastWeeks(n int) string {
	if n == 1 {
		return "last week"
	}
	return fmt.Sprintf("last %d weeks", n)
}

func capitalize(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

// campaignGap is the second paragraph of "If you save" (spec §8.1): what a
// week of the target as typed comes to, and where it lands the campaign.
// It is "" without an active campaign that is still open.
func campaignGap(cs *library.CampaignState, weeklyMinutes int) string {
	if cs == nil || !cs.Campaign.Active() || cs.Over || cs.Reached() {
		return ""
	}
	c := cs.Campaign
	books, assumed := cs.ProjectAt(weeklyMinutes)
	week := minutesLabel(minutes(weeklyMinutes))
	needs := fmt.Sprintf("The campaign needs %s of books a week.", minutesLabel(hours(cs.Required.WeeklyHours)))
	by := fmt.Sprintf("%d of %d by %s", books, c.TargetCount, c.Deadline.Format("2 Jan 2006"))
	if assumed {
		return fmt.Sprintf("That is %s a week. If all of it goes to books, %s. %s", week, by, needs)
	}
	share := cs.Projection.BookShare
	return fmt.Sprintf("That is %s a week. Lately %.0f%% of your reading went to books, so about %s of them. At that, %s. %s",
		week, share*100, minutesLabel(time.Duration(float64(minutes(weeklyMinutes))*share)), by, needs)
}

// matchView is the offer to set the daily target to what the campaign needs.
type matchView struct {
	Field string // "4h43", for the minutes field
	Label string // "4 h 43 min"
	Over  bool   // it needs more than a day on these days
}

// newMatch offers a match for the days as typed; nil when there is nothing to match.
func newMatch(cs *library.CampaignState, days library.Weekdays) *matchView {
	if cs == nil || !cs.Campaign.Active() || cs.Over || cs.Reached() || days == 0 {
		return nil
	}
	m, ok := cs.MatchPerDay(days)
	if !ok {
		return &matchView{Over: true}
	}
	return &matchView{Field: minutesField(m), Label: minutesLabel(minutes(m))}
}

// campaignSlots maps a library validation field to its error slot and message.
var campaignSlots = map[string][2]string{
	"target_count": {"campaignTarget", "A whole number of books, up to 10000."},
	"started_on":   {"campaignStart", "Today or earlier."},
	"deadline":     {"campaignDeadline", "After today and after the start."},
}

// postCampaign starts a campaign.
func (h *handler) postCampaign(w http.ResponseWriter, r *http.Request) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f := in.Campaign
	target, ok := wholeNumber(f.Target)
	if !ok {
		h.planError(w, r, "campaignTarget", "Type a whole number of books.")
		return
	}
	deadline, err := time.Parse(dateField, f.Deadline)
	if err != nil {
		h.planError(w, r, "campaignDeadline", "Pick a date.")
		return
	}
	start, err := time.Parse(dateField, f.Start)
	if err != nil {
		h.planError(w, r, "campaignStart", "Pick a date.")
		return
	}
	_, err = h.svc.StartCampaign(r.Context(), f.Name, target, start, deadline)
	var verr *library.ValidationError
	switch {
	case errors.As(err, &verr):
		s := campaignSlots[verr.Field]
		h.planError(w, r, s[0], s[1])
		return
	case errors.Is(err, library.ErrCampaignActive):
		h.planError(w, r, "campaign", "A campaign is already active. Reload the plan to see it.")
		return
	}
	h.patchPlan(w, r, "", err)
}

// postRenameCampaign renames the campaign shown.
func (h *handler) postRenameCampaign(w http.ResponseWriter, r *http.Request) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.patchPlan(w, r, "", h.svc.RenameCampaign(r.Context(), in.Campaign.ID, in.Campaign.Name))
}

// postEndCampaign ends the campaign shown.
func (h *handler) postEndCampaign(w http.ResponseWriter, r *http.Request) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.patchPlan(w, r, "", h.svc.EndCampaign(r.Context(), in.Campaign.ID))
}

// dayIn is today in the configured timezone, as a date input writes it.
func dayIn(now time.Time, st *library.Settings) string {
	loc, err := st.Location()
	if err != nil {
		loc = time.UTC
	}
	return now.In(loc).Format(dateField)
}
