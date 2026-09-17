package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The plan screen (spec §6.7): the campaign, the week day by day, the daily
// target and the speed ramp, each with how it works. Saving re-renders the "plan-body"
// block and resets the form to the plan now in effect. Typing updates only
// "plan-summary", which says in words what saving would do. A save that
// lowers today's target first swaps the Save button for a confirmation,
// patching only "plan-actions" so nothing typed is morphed away.

// planForm mirrors the form's signals.
type planForm struct {
	Days      map[string]bool `json:"days"` // keyed by weekdayKeys
	Kind      string          `json:"kind"` // "fixed" or "ramp"
	Minutes   string          `json:"minutes"`
	Start     string          `json:"start"`
	Increment string          `json:"increment"`
	Ceiling   string          `json:"ceiling"`
	// The speed ramp form, in whole percents.
	SpeedIncrement string            `json:"speedIncrement"`
	SpeedCeiling   string            `json:"speedCeiling"`
	Campaign       campaignForm      `json:"campaign"`
	Errors         map[string]string `json:"errors"`
}

// weekdayKeys names each weekday's signal, indexed by time.Weekday.
var weekdayKeys = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// planErrors lists every error slot, so one patch clears them all.
func planErrors() map[string]string {
	return map[string]string{"days": "", "minutes": "", "start": "", "increment": "", "ceiling": "", "speedIncrement": "", "speedCeiling": "",
		"campaignTarget": "", "campaignDeadline": "", "campaignStart": "", "campaign": ""}
}

// planInputs maps an error slot to the input that fixes it.
var planInputs = map[string]string{
	"days": "day-first", "minutes": "plan-minutes",
	"start": "ramp-start", "increment": "ramp-increment", "ceiling": "ramp-ceiling",
	"speedIncrement": "speed-increment", "speedCeiling": "speed-ceiling",
	"campaignTarget": "campaign-target", "campaignDeadline": "campaign-deadline", "campaignStart": "campaign-start",
	"campaign": "campaign-target",
}

// planSlots maps a library validation field to its error slot and message.
var planSlots = map[string][2]string{
	"days":              {"days", "Pick at least one day."},
	"minutes_per_day":   {"minutes", "Between 1 minute and 24 h."},
	"start_minutes":     {"start", "Between 1 minute and 24 h."},
	"increment_minutes": {"increment", "At least 1 minute."},
	"ceiling_minutes":   {"ceiling", "Above the start, and at most 24 h."},
	"kind":              {"minutes", "Choose a fixed target or a ramp."},
	"increment_percent": {"speedIncrement", "At least 1%."},
	"ceiling_percent":   {"speedCeiling", "Above 100%, and at most 1000%."},
}

// weekdayChoice is one day pill, in week order.
type weekdayChoice struct {
	Key   string // signal name
	Label string // "Mon"
	ID    string // "day-first" on the first pill, for focus
}

// planBody is everything an action can change.
type planBody struct {
	Board           *board
	Campaign        *campaignView // nil before the first campaign
	Lower           *lowerView    // always nil here: the confirmation arrives by patch
	Summary         planSummary
	Match           *matchView
	Weekdays        []weekdayChoice
	ReviewDay       string // "Sunday"
	PaceWindow      int    // days a baseline is measured over
	SeedPace        int    // pages/h a medium book is assumed to go at
	ProjectionWeeks int    // closed weeks a projection averages
	WordsPerPage    int
	StartFrom       []workedRow // baselines a ramp started today would use
	Signals         string
	Status          string
}

type planPage struct {
	shell
	Body *planBody
}

// planSummary says what saving the form as it stands would do: the target,
// then what a week of it means for the campaign.
type planSummary struct {
	Save, Campaign string
}

// lowerView is the confirmation shown in place of Save.
type lowerView struct {
	From, To string // "2 h 00 min"
	Owed     string // "" when nothing is owed
}

func (h *handler) getPlan(w http.ResponseWriter, r *http.Request) {
	body, err := h.planBody(r.Context(), "")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.plan, planPage{shell: newShell("/plan"), Body: body})
}

// getPlanBody redraws the plan as saved, discarding edits. It is how the
// confirmation's Keep button backs out.
func (h *handler) getPlanBody(w http.ResponseWriter, r *http.Request) {
	h.patchPlan(w, r, "", nil)
}

// postPlan saves the form. postPlanLower saves it after the lowering was
// confirmed.
func (h *handler) postPlan(w http.ResponseWriter, r *http.Request)      { h.savePlan(w, r, false) }
func (h *handler) postPlanLower(w http.ResponseWriter, r *http.Request) { h.savePlan(w, r, true) }

func (h *handler) savePlan(w http.ResponseWriter, r *http.Request, confirmLower bool) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	commitment, slot, ok := in.commitment()
	if !ok {
		h.planError(w, r, slot, "Type a time like 1h30, 1:30 or 90.")
		return
	}

	ctx := r.Context()
	err := h.svc.SavePlan(ctx, in.weekdays(), commitment, confirmLower)
	var verr *library.ValidationError
	var lower *library.LowerTargetError
	switch {
	case errors.As(err, &verr):
		s := planSlots[verr.Field]
		h.planError(w, r, s[0], s[1])
		return
	case errors.As(err, &lower):
		h.confirmLower(w, r, lower)
		return
	}
	h.patchPlan(w, r, "Plan saved.", err)
}

// postPlanPreview redraws the summary for the form as typed.
func (h *handler) postPlanPreview(w http.ResponseWriter, r *http.Request) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	view, err := h.svc.Plan(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.plan, "plan-summary", in.summary(settings.ReviewWeekday, view.Campaign)); err != nil {
		h.log.Error("plan summary", "err", err)
		return
	}
	if err := h.patch(sse, h.plan, "plan-match", newMatch(view.Campaign, in.weekdays())); err != nil {
		h.log.Error("plan match", "err", err)
	}
}

// summary says in words what saving the form would do. A ramp rises on the
// review weekday. With an open campaign it adds what a week of the target
// as typed means for it; a ramp counts at its starting value.
func (in planForm) summary(review time.Weekday, campaign *library.CampaignState) planSummary {
	days := in.weekdays()
	if days == 0 {
		return planSummary{Save: "Pick at least one day."}
	}
	c, _, ok := in.commitment()
	if c.Kind == library.CommitRamp && c.CeilingMinutes <= c.StartMinutes {
		ok = false
	}
	if !ok || c.MinutesPerDay+c.StartMinutes == 0 {
		return planSummary{Save: "Fill in the times, like 1h30, 1:30 or 90."}
	}
	when := daysSentence(days)
	if c.Kind == library.CommitRamp {
		return planSummary{
			Save: fmt.Sprintf("If you save: from today, %s on %s, rising %s each %s while nothing is owed, up to %s. Today counts and closes at midnight.",
				minutesLabel(minutes(c.StartMinutes)), when, minutesLabel(minutes(c.IncrementMinutes)), review, minutesLabel(minutes(c.CeilingMinutes))),
			Campaign: campaignGap(campaign, c.StartMinutes*days.Count()),
		}
	}
	return planSummary{
		Save:     fmt.Sprintf("If you save: from today, %s on %s. Today counts and closes at midnight.", minutesLabel(minutes(c.MinutesPerDay)), when),
		Campaign: campaignGap(campaign, c.MinutesPerDay*days.Count()),
	}
}

// postSpeedRamp starts a speed ramp at 100% of today's baselines.
func (h *handler) postSpeedRamp(w http.ResponseWriter, r *http.Request) {
	var in planForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	increment, ok := wholeNumber(in.SpeedIncrement)
	if !ok {
		h.planError(w, r, "speedIncrement", "Use a whole number.")
		return
	}
	ceiling, ok := wholeNumber(in.SpeedCeiling)
	if !ok {
		h.planError(w, r, "speedCeiling", "Use a whole number.")
		return
	}
	err := h.svc.StartSpeedRamp(r.Context(), increment, ceiling)
	var verr *library.ValidationError
	switch {
	case errors.As(err, &verr):
		s := planSlots[verr.Field]
		h.planError(w, r, s[0], s[1])
		return
	case errors.Is(err, library.ErrNoBaseline):
		h.patchPlan(w, r, "No measured speed yet. Log a session with the page you reached first.", nil)
		return
	}
	h.patchPlan(w, r, "Speed ramp started.", err)
}

// postStopSpeedRamp stops the running speed ramp.
func (h *handler) postStopSpeedRamp(w http.ResponseWriter, r *http.Request) {
	h.patchPlan(w, r, "Speed ramp stopped.", h.svc.StopSpeedRamp(r.Context()))
}

// confirmLower swaps Save for the confirmation, leaving the fields alone.
func (h *handler) confirmLower(w http.ResponseWriter, r *http.Request, lower *library.LowerTargetError) {
	view, err := h.svc.Plan(r.Context())
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	data := lowerView{From: minutesLabel(time.Duration(lower.From) * time.Minute), To: minutesLabel(time.Duration(lower.To) * time.Minute)}
	if owed := view.Schedule.Owed; owed > 0 {
		data.Owed = minutesLabel(owed)
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.plan, "plan-actions", &data); err != nil {
		h.log.Error("plan confirmation", "err", err)
		return
	}
	if err := sse.ExecuteScript(`document.getElementById("plan-lower").focus()`); err != nil {
		h.log.Error("plan confirmation focus", "err", err)
	}
}

// commitment reads the minutes for the chosen kind. When a number does not
// parse, it names the error slot.
func (in planForm) commitment() (c library.Commitment, slot string, ok bool) {
	c.Kind = library.CommitmentKind(in.Kind)
	if c.Kind != library.CommitRamp {
		c.MinutesPerDay, ok = parseMinutes(in.Minutes)
		return c, "minutes", ok
	}
	if c.StartMinutes, ok = parseMinutes(in.Start); !ok {
		return c, "start", false
	}
	if c.IncrementMinutes, ok = parseMinutes(in.Increment); !ok {
		return c, "increment", false
	}
	c.CeilingMinutes, ok = parseMinutes(in.Ceiling)
	return c, "ceiling", ok
}

func (in planForm) weekdays() library.Weekdays {
	var days library.Weekdays
	for d, key := range weekdayKeys {
		if in.Days[key] {
			days |= library.WeekdaysOf(time.Weekday(d))
		}
	}
	return days
}

// wholeNumber parses a required whole number.
func wholeNumber(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}

// planError reports a validation failure in band and focuses its field.
func (h *handler) planError(w http.ResponseWriter, r *http.Request, slot, msg string) {
	errs := planErrors()
	errs[slot] = msg
	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(map[string]any{"errors": errs}); err != nil {
		h.log.Error("plan errors", "err", err)
		return
	}
	if err := sse.ExecuteScript(`document.getElementById("` + planInputs[slot] + `").focus()`); err != nil {
		h.log.Error("plan error focus", "err", err)
	}
}

// patchPlan answers an action: the body is rebuilt from fresh state and the
// form is reset to the plan in effect.
func (h *handler) patchPlan(w http.ResponseWriter, r *http.Request, status string, err error) {
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	body, err := h.planBody(r.Context(), status)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.plan, "plan-body", body); err != nil {
		h.log.Error("plan body", "err", err)
		return
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("plan reset", "err", err)
	}
}

// planBody gathers the screen. The form starts from the plan in effect: a
// running ramp at its current value, so saving it unchanged changes nothing.
func (h *handler) planBody(ctx context.Context, status string) (*planBody, error) {
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	view, err := h.svc.Plan(ctx)
	if err != nil {
		return nil, err
	}
	sc := view.Schedule

	form := planForm{Days: map[string]bool{}, Kind: string(library.CommitFixed), SpeedIncrement: "5", SpeedCeiling: "130", Errors: planErrors()}
	form.Campaign = campaignForm{Start: dayIn(time.Now(), settings)}
	if cs := view.Campaign; cs != nil && cs.Campaign.Active() {
		form.Campaign.ID, form.Campaign.Name = cs.Campaign.ID, cs.Campaign.Name
	}
	days := sc.Days
	if !sc.Planned() {
		days = library.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
	}
	for d, key := range weekdayKeys {
		form.Days[key] = days.Has(time.Weekday(d))
	}
	switch {
	case sc.Ramp != nil:
		form.Kind = string(library.CommitRamp)
		form.Start = minutesField(sc.Ramp.Current)
		form.Increment = minutesField(sc.Ramp.Increment)
		form.Ceiling = minutesField(sc.Ramp.Ceiling)
	case sc.Planned():
		form.Minutes = minutesField(sc.Value)
	}

	body := &planBody{
		Board:           newBoard(sc, view.Speed, settings.WordsPerPage),
		Campaign:        newCampaignView(view.Campaign, sc),
		Summary:         form.summary(settings.ReviewWeekday, view.Campaign),
		Match:           newMatch(view.Campaign, days),
		ReviewDay:       settings.ReviewWeekday.String(),
		PaceWindow:      settings.PaceWindowDays,
		SeedPace:        settings.SeedPaceMedium,
		ProjectionWeeks: settings.ProjectionWindowWeeks,
		WordsPerPage:    settings.WordsPerPage,
		StartFrom:       baselineRows(view.Speed.StartFrom),
		Status:          status,
	}
	for i := range 7 {
		d := (int(settings.ReviewWeekday) + i) % 7
		choice := weekdayChoice{Key: weekdayKeys[d], Label: time.Weekday(d).String()[:3]}
		if i == 0 {
			choice.ID = "day-first"
		}
		body.Weekdays = append(body.Weekdays, choice)
	}
	if body.Signals, err = marshalSignals(form); err != nil {
		return nil, err
	}
	return body, nil
}
