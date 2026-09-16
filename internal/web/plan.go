package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The plan screen (spec §6.7): active days, the daily commitment, and where
// both stand. Saving re-renders the "plan-body" block and resets the form to
// the plan now in effect. A save that lowers today's target first swaps the
// Save button for a confirmation, patching only "plan-actions" so nothing
// typed is morphed away.

// planForm mirrors the form's signals.
type planForm struct {
	Days      map[string]bool   `json:"days"` // keyed by weekdayKeys
	Kind      string            `json:"kind"` // "fixed" or "ramp"
	Minutes   string            `json:"minutes"`
	Start     string            `json:"start"`
	Increment string            `json:"increment"`
	Ceiling   string            `json:"ceiling"`
	Errors    map[string]string `json:"errors"`
}

// weekdayKeys names each weekday's signal, indexed by time.Weekday.
var weekdayKeys = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// planErrors lists every error slot, so one patch clears them all.
func planErrors() map[string]string {
	return map[string]string{"days": "", "minutes": "", "start": "", "increment": "", "ceiling": ""}
}

// planInputs maps an error slot to the input that fixes it.
var planInputs = map[string]string{
	"days": "day-first", "minutes": "plan-minutes",
	"start": "ramp-start", "increment": "ramp-increment", "ceiling": "ramp-ceiling",
}

// planSlots maps a library validation field to its error slot and message.
var planSlots = map[string][2]string{
	"days":              {"days", "Pick at least one day."},
	"minutes_per_day":   {"minutes", "Between 1 and 1440 minutes."},
	"start_minutes":     {"start", "Between 1 and 1440 minutes."},
	"increment_minutes": {"increment", "At least 1 minute."},
	"ceiling_minutes":   {"ceiling", "Above the start, and at most 1440 minutes."},
	"kind":              {"minutes", "Choose a fixed target or a ramp."},
}

// weekdayChoice is one day pill, in week order.
type weekdayChoice struct {
	Key   string // signal name
	Label string // "Mon"
	ID    string // "day-first" on the first pill, for focus
}

// planBody is everything an action can change.
type planBody struct {
	Standing  *standing
	Lower     *lowerView // always nil here: the confirmation arrives by patch
	Weekdays  []weekdayChoice
	ReviewDay string // "Sunday"
	Signals   string
	Status    string
}

type planPage struct {
	shell
	Body *planBody
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
		h.planError(w, r, slot, "Use a whole number of minutes.")
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
		c.MinutesPerDay, ok = wholeNumber(in.Minutes)
		return c, "minutes", ok
	}
	if c.StartMinutes, ok = wholeNumber(in.Start); !ok {
		return c, "start", false
	}
	if c.IncrementMinutes, ok = wholeNumber(in.Increment); !ok {
		return c, "increment", false
	}
	c.CeilingMinutes, ok = wholeNumber(in.Ceiling)
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

	form := planForm{Days: map[string]bool{}, Kind: string(library.CommitFixed), Errors: planErrors()}
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
		form.Start = strconv.Itoa(sc.Ramp.Current)
		form.Increment = strconv.Itoa(sc.Ramp.Increment)
		form.Ceiling = strconv.Itoa(sc.Ramp.Ceiling)
	case sc.Planned():
		form.Minutes = strconv.Itoa(sc.Value)
	}

	body := &planBody{Standing: newStanding(sc), ReviewDay: settings.ReviewWeekday.String(), Status: status}
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

// standing is where the discipline stands, as the strip and the week
// section draw it on Home and on the plan.
type standing struct {
	Planned    bool
	Logged     string // today, "40 min"
	Target     string // today's target; "" on a rest day
	RestDay    bool
	Owed       string // "" when nothing is owed
	WeekLogged string
	WeekTarget string
	Daily      string // "1 h 30 min a day"
	Ramp       *rampLine
	Reached    string // "27 Sep": when a finished ramp reached its ceiling
}

// rampLine is a running hours ramp in words.
type rampLine struct {
	Increment string // "30 min"
	Ceiling   string // "4 h 00 min"
	NextCheck string // "Sun 27 Sep"
	LastCheck string // "20 Sep", or "" before the first check
	Held      bool   // the last check held because something was owed
}

func newStanding(sc library.Schedule) *standing {
	out := &standing{Planned: sc.Planned(), Logged: minutesLabel(sc.LoggedToday), RestDay: sc.RestDay()}
	if !sc.Planned() {
		return out
	}
	if !sc.RestDay() {
		out.Target = minutesLabel(time.Duration(sc.TargetToday) * time.Minute)
	}
	if sc.Owed > 0 {
		out.Owed = minutesLabel(sc.Owed)
	}
	out.WeekLogged = minutesLabel(sc.WeekLogged)
	out.WeekTarget = minutesLabel(time.Duration(sc.WeekTarget) * time.Minute)
	out.Daily = minutesLabel(time.Duration(sc.Value)*time.Minute) + " a day"
	if !sc.ReachedCeilingOn.IsZero() {
		out.Reached = sc.ReachedCeilingOn.Format("2 Jan")
	}
	if rp := sc.Ramp; rp != nil {
		out.Ramp = &rampLine{
			Increment: minutesLabel(time.Duration(rp.Increment) * time.Minute),
			Ceiling:   minutesLabel(time.Duration(rp.Ceiling) * time.Minute),
			NextCheck: rp.NextCheck.Format("Mon 2 Jan"),
		}
		if rp.LastCheck != nil {
			out.Ramp.LastCheck = rp.LastCheck.On.Format("2 Jan")
			out.Ramp.Held = !rp.LastCheck.Advanced
		}
	}
	return out
}
