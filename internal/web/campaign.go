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
	Kind     string          `json:"kind"` // "count" or "set"
	Name     string          `json:"name"`
	Target   string          `json:"target"`
	Deadline string          `json:"deadline"` // "2006-01-02", from a date input
	Start    string          `json:"start"`
	Items    map[string]bool `json:"items"` // the items ticked, by id
}

// pickRow is one item a set can be given.
type pickRow struct {
	ID, Title, Shelf, Size string
	Reading                bool // in progress, not waiting in the pool
}

// pickRows draws the items a set can take, and marks how many are ticked.
func pickRows(entries []library.ShortlistEntry) []pickRow {
	var out []pickRow
	for _, e := range entries {
		row := pickRow{ID: e.Item.ID, Title: e.Item.Title, Shelf: e.ShelfName, Reading: e.Item.State == library.StateInProgress}
		if e.Item.SizeValue != nil {
			row.Size = sizeLabel(e.Item)
		}
		out = append(out, row)
	}
	return out
}

// ticked is the ids ticked in the form, in the order the rows are offered,
// so a set keeps the order it was picked in.
func ticked(rows []pickRow, items map[string]bool) []string {
	var ids []string
	for _, row := range rows {
		if items[row.ID] {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

// dateField is how a date input reads and writes a calendar day.
const dateField = "2006-01-02"

// campaignView is a campaign, as a block on the plan or on its own page.
type campaignView struct {
	ID, Href string
	Active   bool
	Over     bool // active, but its deadline has passed

	Set                 bool   // it asks for named items, not a number of books
	Unit                string // "books" or "items", agreeing with the target
	Counts              string // the meter's label: "Books" or "Items"
	Name, Count, Target string
	Line                string  // under the count: deadline, time left, where it is heading; or how it ended
	Gap                 string  // the block's one line: recent book hours against what it needs
	Fill                float64 // books finished, 0–1 of the target
	Lands               float64 // where the projection lands, 0–1
	ShowLands           bool
	ToGo, ByThen        string // "77 to go", "61 by then"

	Week     *campaignWeek // nil when over
	Rows     []campaignRow
	Items    []campaignRow // a set's items, in the order they were added
	EndLines []string      // what ending it means, for the confirmation dialog
}

// campaignWeek is recent book hours against what the campaign needs.
type campaignWeek struct {
	Label        string // "Books a week", or "Hours a week" on its items
	None         string // what it says before there is a figure
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
		Set:    c.Kind == library.KindSet,
		Unit:   "books",
		Counts: "Books",
		Name:   c.Name,
		Count:  fmt.Sprint(cs.Finished),
		Target: fmt.Sprint(cs.Target),
		Fill:   math.Min(1, float64(cs.Finished)/float64(max(cs.Target, 1))),
	}
	if v.Set {
		v.Counts, v.Unit = "Items", "items"
		if cs.Target == 1 {
			v.Unit = "item"
		}
	}
	if !c.Active() {
		v.Line = fmt.Sprintf("Ended on %s with %d of %d.", c.EndedOn.Format("2 Jan 2006"), cs.Finished, cs.Target)
		return v
	}
	deadline := c.Deadline.Format("2 Jan 2006")
	if cs.Over {
		v.Line = fmt.Sprintf("The deadline, %s, has passed. This is the final count.", deadline)
		v.EndLines = []string{
			fmt.Sprintf("Its count is final at %d of %d.", cs.Finished, cs.Target),
			"Ending files it away on the record.",
		}
		return v
	}
	if v.Set {
		setView(v, cs, sc)
		return v
	}

	r, p := cs.Required, cs.Projection
	v.Line = fmt.Sprintf("Deadline %s, %s left. ", deadline, timeLeft(cs.WeeksLeft))
	v.EndLines = []string{
		fmt.Sprintf("You have %d of %d, with %s left.", cs.Finished, cs.Target, timeLeft(cs.WeeksLeft)),
		"Ending stops the count and the projection today, and it can't be picked up again. Books already counted stay.",
	}
	v.ToGo = fmt.Sprintf("%d to go", r.BooksLeft)
	if r.BooksLeft == 0 {
		v.ToGo = "target reached"
	}
	v.Week = &campaignWeek{Label: "Books a week", None: "no closed week yet", Needed: minutesLabel(hours(r.WeeklyHours))}
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

// setView draws a set campaign: where each of its items stands, the hours
// they still need, and what its own reading gives them (spec §8.1, §8.5).
func setView(v *campaignView, cs *library.CampaignState, sc library.Schedule) {
	c, r, p := cs.Campaign, cs.Required, cs.Set
	left := cs.Target - cs.Finished
	v.ToGo = countLabel(left, "item") + " to go"
	if left == 0 {
		v.ToGo = "all of them finished"
	}
	v.Line = fmt.Sprintf("Deadline %s, %s left. ", c.Deadline.Format("2 Jan 2006"), timeLeft(cs.WeeksLeft))
	v.EndLines = []string{
		fmt.Sprintf("You have %d of %d finished, with %s left.", cs.Finished, cs.Target, timeLeft(cs.WeeksLeft)),
		"Ending stops it today, and it can't be picked up again. The reading already done stays.",
	}

	for _, si := range cs.Items {
		row := campaignRow{Label: si.Item.Title}
		switch {
		case si.Done:
			row.Figure, row.Behind = "finished "+si.FinishedOn.Format("2 Jan"), ""
		case si.Lost:
			row.Figure, row.Behind = "abandoned", "it can no longer be met"
		case si.Estimate.Known():
			row.Figure = minutesLabel(si.Estimate.Remaining) + " left"
			row.Behind = positionLabel(si.Item, si.Position) + basisNote(si.Estimate)
			row.Provisional = si.Estimate.Provisional()
		default:
			row.Figure, row.Behind = "–", "no estimate yet"
		}
		v.Items = append(v.Items, row)
	}

	v.Week = &campaignWeek{Label: "Hours a week", None: "none of them read yet", Needed: minutesLabel(hours(r.WeeklyHours))}
	switch {
	case cs.Lost:
		v.Line += "An item was abandoned, so it can no longer be met."
		v.Gap = "It can no longer be met. Ending it files it away on the record."
		return
	case cs.Reached():
		v.Line += "Every item is finished."
		v.Gap = "Every item is finished."
		return
	case p == nil:
		v.Line += "Where it's heading shows once you have read one of its items."
		v.Gap = fmt.Sprintf("Needs %s a week on its items; none of them has been read yet.", v.Week.Needed)
	default:
		if !p.DoneOn.IsZero() {
			v.Line += "At this rate, done " + p.DoneOn.Format("2 Jan") + "."
		} else {
			v.Line += fmt.Sprintf("At this rate, %.0f%% of what it needs by then.", p.Covered*100)
		}
		read, needed := hours(p.Hours), hours(r.WeeklyHours)
		scale := max(read, needed)
		w := v.Week
		w.Closed, w.Read = true, minutesLabel(read)
		w.Fill, w.Mark = share(read, scale), share(needed, scale)
		w.Gap = "meets it"
		if gap := needed - read; gap >= time.Minute {
			w.Gap = minutesLabel(gap) + " short a week"
		}
		v.Gap = fmt.Sprintf("Hours a week: %s of %s needed, %s.", w.Read, w.Needed, w.Gap)
	}

	v.Rows = []campaignRow{
		{Label: "Items left", Figure: fmt.Sprint(left), Behind: fmt.Sprintf("%d finished of %d", cs.Finished, cs.Target)},
		{Label: "Time left on them", Figure: minutesLabel(hours(r.HoursLeft)), Behind: "at what each one is read at"},
		{Label: "Needed each week", Figure: minutesLabel(hours(r.WeeklyHours)), Behind: "over the " + timeLeft(cs.WeeksLeft) + " left", Strong: true},
	}
	committed := campaignRow{Label: "Committed each week", Figure: "–", Behind: "no daily target yet"}
	if sc.Planned() {
		committed.Figure, committed.Behind = minutesLabel(minutes(sc.CommittedWeek())), ""
		if p != nil {
			committed.Behind = fmt.Sprintf("%.0f%% of it on these items lately", p.Share*100)
		}
	}
	v.Rows = append(v.Rows, committed)
	if p != nil {
		v.Rows = append(v.Rows, campaignRow{Label: "On its items", Figure: minutesLabel(hours(p.Hours)),
			Behind: fmt.Sprintf("a week since it began, %s in all", minutesLabel(hours(p.Hours*p.Weeks)))})
	}
	if pp := cs.Plan; pp != nil {
		row := campaignRow{Label: "If your plan holds", Strong: true}
		if !pp.DoneOn.IsZero() {
			row.Figure = "done " + pp.DoneOn.Format("2 Jan")
		} else {
			row.Figure = fmt.Sprintf("%.0f%% by %s", pp.Covered*100, c.Deadline.Format("2 Jan"))
		}
		row.Behind = minutesLabel(minutes(pp.Trajectory.Value)) + " a day"
		if pp.Assumed {
			row.Behind += ", all of it on these items"
		} else {
			row.Behind += fmt.Sprintf(", %.0f%% of it on these items", pp.Share*100)
		}
		v.Rows = append(v.Rows, row)
	}
}

// basisNote says what an estimate rests on, after the position.
func basisNote(e library.Estimate) string {
	switch {
	case e.Provisional():
		return ", at the default pace"
	case e.Rough:
		return ", on under an hour of it"
	}
	return ""
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
	b.WriteString(". Read in full, at the share of it each of these lately gets:")
	return b.String()
}

// gapLine is where the plan lands one campaign, and what it needs.
func gapLine(cs *library.CampaignState, p *library.PlanProjection) string {
	c := cs.Campaign
	var b strings.Builder
	share := fmt.Sprintf("%.0f%%", p.Share*100)
	if p.Assumed {
		share = "all"
	}
	if c.Kind == library.KindSet {
		landing := fmt.Sprintf("%.0f%% of what it needs by %s", p.Covered*100, c.Deadline.Format("2 Jan 2006"))
		if !p.DoneOn.IsZero() {
			landing = "done " + p.DoneOn.Format("2 Jan 2006")
		}
		fmt.Fprintf(&b, "%s (%s of it on these items): %s. It needs %s a week on them.",
			c.Name, share, landing, minutesLabel(hours(cs.Required.WeeklyHours)))
		return b.String()
	}
	fmt.Fprintf(&b, "%s (%s of it on books): %d of %d. It needs %s of books a week",
		c.Name, share, p.Books, c.TargetCount, minutesLabel(hours(cs.Required.WeeklyHours)))
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
	on := "books"
	if cs.Campaign.Kind == library.KindSet {
		on = "its items"
	}
	v.Field, v.Label, v.Share = minutesField(m), minutesLabel(minutes(m)), "if all of it goes to "+on
	if share < 1 {
		v.Share = fmt.Sprintf("with %.0f%% of it on %s, as lately", share*100, on)
	}
	return v
}

// campaignSlots maps a library validation field to its error slot and message.
var campaignSlots = map[string][2]string{
	"target_count": {"campaignTarget", "A whole number of books, up to 10000."},
	"started_on":   {"campaignStart", "Today or earlier."},
	"deadline":     {"campaignDeadline", "After today and after the start."},
	"items":        {"campaignItems", "Pick at least one item that can still be read."},
}

// postCampaign starts a campaign.
func (h *handler) postCampaign(w http.ResponseWriter, r *http.Request) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f := in.Campaign
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
	if f.Kind == string(library.KindSet) {
		view, err := h.svc.Plan(r.Context())
		if err != nil {
			h.httpError(w, r, err)
			return
		}
		ids := ticked(pickRows(view.Pickable), f.Items)
		if len(ids) == 0 {
			h.planError(w, r, "campaignItems", "Tick the items this campaign is for.")
			return
		}
		_, err = h.svc.StartSetCampaign(r.Context(), f.Name, ids, start, deadline)
		h.campaignStarted(w, r, err)
		return
	}
	target, ok := wholeNumber(f.Target)
	if !ok {
		h.planError(w, r, "campaignTarget", "Type a whole number of books.")
		return
	}
	_, err = h.svc.StartCampaign(r.Context(), f.Name, target, start, deadline)
	h.campaignStarted(w, r, err)
}

// campaignStarted redraws the plan, or reports where the form went wrong.
func (h *handler) campaignStarted(w http.ResponseWriter, r *http.Request, err error) {
	var verr *library.ValidationError
	if errors.As(err, &verr) {
		s := campaignSlots[verr.Field]
		if verr.Field == "items" {
			h.planError(w, r, s[0], capitalize(verr.Msg)+".")
			return
		}
		h.planError(w, r, s[0], s[1])
		return
	}
	h.patchPlan(w, r, "", err)
}

// campaignBody is a campaign's page: everything its block on the plan shows,
// at full size, with what it is built from, and renaming and ending it.
type campaignBody struct {
	Campaign        *campaignView
	Pickable        []pickRow // the items this set can still be given
	PaceWindow      int       // days book pace is measured over
	SeedPaces       string    // "light 40, medium 30, deep 15"
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
		Pickable:        pickRows(view.Pickable),
		PaceWindow:      settings.PaceWindowDays,
		SeedPaces:       fmt.Sprintf("light %d, medium %d, deep %d", settings.SeedPaceLight, settings.SeedPaceMedium, settings.SeedPaceDeep),
		ProjectionWeeks: settings.ProjectionWindowWeeks,
		Status:          status,
	}
	form := struct {
		Campaign campaignForm      `json:"campaign"`
		Errors   map[string]string `json:"errors"`
	}{campaignForm{Name: view.State.Campaign.Name, Items: map[string]bool{}}, map[string]string{"campaignItems": ""}}
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

// postCampaignItems adds items to a set, which can only grow.
func (h *handler) postCampaignItems(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Campaign campaignForm `json:"campaign"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	view, err := h.svc.Campaign(r.Context(), id)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	ids := ticked(pickRows(view.Pickable), in.Campaign.Items)
	if len(ids) == 0 {
		h.campaignError(w, r, "Tick the items to add.")
		return
	}
	err = h.svc.AddCampaignItems(r.Context(), id, ids)
	var verr *library.ValidationError
	if errors.As(err, &verr) {
		h.campaignError(w, r, capitalize(verr.Msg)+".")
		return
	}
	h.patchCampaign(w, r, countLabel(len(ids), "item")+" added.", err)
}

// campaignError reports what went wrong on a campaign's page, leaving the
// ticks as they are.
func (h *handler) campaignError(w http.ResponseWriter, r *http.Request, message string) {
	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(map[string]any{"errors": map[string]string{"campaignItems": message}}); err != nil {
		h.log.Error("campaign error", "err", err)
	}
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
