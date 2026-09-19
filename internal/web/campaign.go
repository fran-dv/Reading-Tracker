package web

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// Campaigns (spec §6.7, §8.1, §8.5): on the plan, each active one as a
// compact block; on its own page, the count so far, where recent reading
// lands, and what it needs each week, every figure traced to what it is
// built from. The daily target never follows a campaign; the target form
// offers to match each one instead.

// campaignForm mirrors a campaign's signals, nested under "campaign": the
// plan's start form, or the name on a campaign's page.
type campaignForm struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	Deadline string `json:"deadline"` // "2006-01-02", from a date input
	Start    string `json:"start"`
}

// dateField is how a date input reads and writes a calendar day.
const dateField = "2006-01-02"

// campaignView is a campaign, as a block on the plan or on its own page.
type campaignView struct {
	ID, Href string
	Active   bool
	Over     bool // active, but its deadline has passed

	Name, Count, Target string
	Line                string  // under the count: deadline, time left, where it is heading; or how it ended
	Gap                 string  // the block's one line: recent book hours against what it needs
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

// newCampaignViews draws each campaign in order.
func newCampaignViews(states []library.CampaignState, sc library.Schedule) []*campaignView {
	var out []*campaignView
	for i := range states {
		out = append(out, newCampaignView(&states[i], sc))
	}
	return out
}

// newCampaignView draws a campaign where it stands.
func newCampaignView(cs *library.CampaignState, sc library.Schedule) *campaignView {
	c := cs.Campaign
	v := &campaignView{
		ID:     c.ID,
		Href:   "/plan/campaign/" + c.ID,
		Active: c.Active(),
		Over:   cs.Over,
		Name:   c.Name,
		Count:  fmt.Sprint(cs.Finished),
		Target: fmt.Sprint(c.TargetCount),
		Fill:   math.Min(1, float64(cs.Finished)/float64(c.TargetCount)),
	}
	if !c.Active() {
		v.Line = fmt.Sprintf("Ended on %s with %d of %d.", c.EndedOn.Format("2 Jan 2006"), cs.Finished, c.TargetCount)
		return v
	}
	deadline := c.Deadline.Format("2 Jan 2006")
	if cs.Over {
		v.Line = fmt.Sprintf("The deadline, %s, has passed. This is the final count.", deadline)
		v.EndLines = []string{
			fmt.Sprintf("Its count is final at %d of %d.", cs.Finished, c.TargetCount),
			"Ending files it away on the record.",
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
	v.Gap = fmt.Sprintf("Needs %s of books a week; no week of reading has closed yet.", v.Week.Needed)
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
		v.Gap = fmt.Sprintf("Books a week: %s of %s needed, %s.", w.Read, w.Needed, w.Gap)
	}

	pages := map[library.PagesBasis]string{
		library.PagesWaiting:  "the books waiting",
		library.PagesFinished: "books finished, none waiting has a size",
		library.PagesSetting:  "a guess, until books have sizes",
	}
	pace := campaignRow{Label: "Book pace", Figure: fmt.Sprintf("%.0f pages/h", r.Pace.PagesPerHour)}
	if r.Pace.Provisional {
		pace.Provisional = true
		pace.Behind = "until 2 h are measured, from the books waiting"
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
	var plan []campaignRow
	if pp := cs.Plan; pp != nil {
		v.Line += fmt.Sprintf(" If your plan holds, %d.", pp.Books)
		t := pp.Trajectory
		how := minutesLabel(minutes(t.Value)) + " a day now"
		if !pp.TopOn.IsZero() {
			how += ", " + minutesLabel(minutes(t.Ceiling)) + " from " + pp.TopOn.Format("2 Jan")
		}
		if pp.Assumed {
			how += ", all counted as books"
		} else {
			how += fmt.Sprintf(", %.0f%% on books", pp.Share*100)
		}
		plan = append(plan, campaignRow{Label: "If your plan holds", Figure: countLabel(pp.Books, "book"), Behind: how, Strong: true})
		if pp.PerBook > 0 {
			plan = append(plan, campaignRow{Label: "A book can take", Figure: minutesLabel(hours(pp.PerBook)),
				Behind: "under the plan; the books waiting need " + minutesLabel(hours(pp.BookNeeds)) + " each"})
		}
		if pp.DueByNow >= 0.5 {
			plan = append(plan, campaignRow{Label: "The plan so far", Figure: fmt.Sprintf("about %.0f", pp.DueByNow),
				Behind: fmt.Sprintf("books its targets since %s come to; %d finished", c.StartedOn.Format("2 Jan"), cs.Finished)})
		}
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
	v.Rows = append(v.Rows, plan...)
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

// campaignGaps is the rest of "If you save" (spec §8.1): what a week of
// the target as typed comes to, then, for each active campaign still open,
// where it lands if the plan holds — a ramp rising at every check — at the
// recent share of reading on books, and what a book can take under it.
// Both are empty without such a campaign.
func campaignGaps(states []library.CampaignState, t library.Trajectory, today time.Time) (week string, lines []string) {
	for i := range states {
		cs := &states[i]
		if !cs.Campaign.Active() {
			continue
		}
		p := cs.ProjectPlan(t, today)
		if p == nil {
			continue
		}
		if week == "" {
			week = planWeek(t, p)
		}
		lines = append(lines, gapLine(cs, p))
	}
	return week, lines
}

// planWeek is what a week of the plan comes to, and the share of it that
// the projections count as books.
func planWeek(t library.Trajectory, p *library.PlanProjection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "That is %s a week", minutesLabel(minutes(t.Value*t.Days.Count())))
	if top := p.TopOn; !top.IsZero() {
		fmt.Fprintf(&b, ", rising to %s a day by %s", minutesLabel(minutes(t.Ceiling)), top.Format("2 Jan"))
	}
	if p.Assumed {
		b.WriteString(". Read in full, if all of it goes to books:")
	} else {
		fmt.Fprintf(&b, ". Read in full, with %.0f%% of it on books as lately:", p.Share*100)
	}
	return b.String()
}

// gapLine is where the plan lands one campaign, and what it needs.
func gapLine(cs *library.CampaignState, p *library.PlanProjection) string {
	c := cs.Campaign
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d of %d. It needs %s of books a week",
		c.Name, p.Books, c.TargetCount, minutesLabel(hours(cs.Required.WeeklyHours)))
	if p.PerBook > 0 {
		fmt.Fprintf(&b, "; under this plan a book can take about %s, and the books waiting need about %s each",
			minutesLabel(hours(p.PerBook)), minutesLabel(hours(p.BookNeeds)))
	}
	b.WriteString(".")
	return b.String()
}

// matchView is the offer to set the daily target to what a campaign needs.
type matchView struct {
	Name  string // the campaign's
	Field string // "4h43", for the minutes field
	Label string // "4 h 43 min"
	Share string // "with 90% of it on books, as lately", or "if all of it goes to books"
	Over  bool   // it needs more than a day on these days
}

// newMatches offers a match for each active campaign on the days as typed.
func newMatches(states []library.CampaignState, days library.Weekdays) []*matchView {
	var out []*matchView
	for i := range states {
		if m := newMatch(&states[i], days); m != nil {
			out = append(out, m)
		}
	}
	return out
}

// newMatch offers a match for the days as typed; nil when there is nothing to match.
func newMatch(cs *library.CampaignState, days library.Weekdays) *matchView {
	if !cs.Campaign.Active() || cs.Over || cs.Reached() || days == 0 {
		return nil
	}
	m, share, ok := cs.MatchPerDay(days)
	if !ok && share == 0 && m == 0 {
		return nil // nothing read lately goes to books: no target would meet it
	}
	v := &matchView{Name: cs.Campaign.Name}
	if !ok {
		v.Over = true
		return v
	}
	v.Field, v.Label, v.Share = minutesField(m), minutesLabel(minutes(m)), "if all of it goes to books"
	if share < 1 {
		v.Share = fmt.Sprintf("with %.0f%% of it on books, as lately", share*100)
	}
	return v
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
	if errors.As(err, &verr) {
		s := campaignSlots[verr.Field]
		h.planError(w, r, s[0], s[1])
		return
	}
	h.patchPlan(w, r, "", err)
}

// campaignBody is a campaign's page: everything its block on the plan shows,
// at full size, with what it is built from, and renaming and ending it.
type campaignBody struct {
	Campaign        *campaignView
	PaceWindow      int    // days book pace is measured over
	SeedPaces       string // "light 40, medium 30, deep 15"
	ProjectionWeeks int
	Signals         string
	Status          string
}

type campaignPage struct {
	shell
	Body *campaignBody
}

func (h *handler) getCampaign(w http.ResponseWriter, r *http.Request) {
	body, err := h.campaignBody(r.Context(), r.PathValue("id"), "")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.campaign, campaignPage{shell: h.newShell(r.Context(), "/plan"), Body: body})
}

func (h *handler) campaignBody(ctx context.Context, id, status string) (*campaignBody, error) {
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	view, err := h.svc.Campaign(ctx, id)
	if err != nil {
		return nil, err
	}
	body := &campaignBody{
		Campaign:        newCampaignView(&view.State, view.Schedule),
		PaceWindow:      settings.PaceWindowDays,
		SeedPaces:       fmt.Sprintf("light %d, medium %d, deep %d", settings.SeedPaceLight, settings.SeedPaceMedium, settings.SeedPaceDeep),
		ProjectionWeeks: settings.ProjectionWindowWeeks,
		Status:          status,
	}
	form := struct {
		Campaign campaignForm `json:"campaign"`
	}{campaignForm{Name: view.State.Campaign.Name}}
	if body.Signals, err = marshalSignals(form); err != nil {
		return nil, err
	}
	return body, nil
}

// patchCampaign redraws a campaign's page after an action, or reports its error.
func (h *handler) patchCampaign(w http.ResponseWriter, r *http.Request, status string, err error) {
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	body, err := h.campaignBody(r.Context(), r.PathValue("id"), status)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.campaign, "campaign-body", body); err != nil {
		h.log.Error("campaign body", "err", err)
		return
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("campaign reset", "err", err)
	}
}

// postRenameCampaign renames a campaign.
func (h *handler) postRenameCampaign(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Campaign campaignForm `json:"campaign"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.patchCampaign(w, r, "Renamed.", h.svc.RenameCampaign(r.Context(), r.PathValue("id"), in.Campaign.Name))
}

// postEndCampaign ends a campaign.
func (h *handler) postEndCampaign(w http.ResponseWriter, r *http.Request) {
	h.patchCampaign(w, r, "Campaign ended.", h.svc.EndCampaign(r.Context(), r.PathValue("id")))
}

// dayIn is today in the configured timezone, as a date input writes it.
func dayIn(now time.Time, st *library.Settings) string {
	loc, err := st.Location()
	if err != nil {
		loc = time.UTC
	}
	return now.In(loc).Format(dateField)
}
