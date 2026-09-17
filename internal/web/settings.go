package web

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// Settings (spec §2.7): the values the rules read, as one form. Every field
// is a string, as inputs give it; the library validates what they mean.

// settingsForm mirrors the form's signals, keyed as the library names them.
type settingsForm struct {
	Timezone string            `json:"timezone"`
	Review   string            `json:"review_weekday"` // "0" Sunday … "6" Saturday
	Values   map[string]string `json:"values"`         // every whole-number setting
	Errors   map[string]string `json:"errors"`
}

// settingsField is one whole-number setting as the form draws it.
type settingsField struct {
	Key, Label, Unit, Help string
}

// settingsGroup is a titled set of fields.
type settingsGroup struct {
	Title  string
	Fields []settingsField
}

// settingsGroups lays the whole-number settings out, group by group.
var settingsGroups = []settingsGroup{
	{"Discipline", []settingsField{
		{"wip_cap", "In progress at most", "items", "Starting more asks you to finish or abandon one first. Three is a good place to start."},
		{"stall_days", "Stalled after", "days", "An item not read for this long is marked stalled."},
	}},
	{"The moment", []settingsField{
		{"bucket_quick_max_min", "Quick, up to", "min", "Picks that fit “quick”. Books and courses fit any moment."},
		{"bucket_hour_max_min", "An hour, up to", "min", "Picks that fit “an hour”."},
	}},
	{"Measuring", []settingsField{
		{"pace_window_days", "Pace from the last", "days", "Book pace and speeds look back this far."},
		{"projection_window_weeks", "Recent reading over", "weeks", "Where the campaign is heading averages these closed weeks."},
		{"words_per_page", "Words on a page", "words", "Converts speeds between pages and words for display."},
	}},
	{"Before anything is measured", []settingsField{
		{"seed_pace_light", "Light reading", "pages/h", ""},
		{"seed_pace_medium", "Medium reading", "pages/h", ""},
		{"seed_pace_deep", "Deep reading", "pages/h", ""},
		{"seed_pace_wpm", "Articles", "words/min", ""},
		{"fallback_book_pages", "A book without a size", "pages", "Used for the campaign until books have sizes."},
	}},
}

type settingsBody struct {
	Groups   []settingsGroup
	Weekdays []weekdayOption
	Zone     string // the detected zone, when settings still say "Local"
	Signals  string
	Status   string
}

type weekdayOption struct {
	Value string
	Name  string
}

type settingsPage struct {
	shell
	Body *settingsBody
}

func (h *handler) getSettings(w http.ResponseWriter, r *http.Request) {
	body, err := h.settingsBody(r.Context(), "")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.settings, settingsPage{shell: h.newShell(r.Context(), "/settings"), Body: body})
}

func (h *handler) settingsBody(ctx context.Context, status string) (*settingsBody, error) {
	st, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	b := &settingsBody{Groups: settingsGroups, Status: status}
	for d := time.Sunday; d <= time.Saturday; d++ {
		b.Weekdays = append(b.Weekdays, weekdayOption{Value: strconv.Itoa(int(d)), Name: d.String()})
	}
	form := settingsForm{Timezone: st.Timezone, Review: strconv.Itoa(int(st.ReviewWeekday)), Values: settingsValues(*st), Errors: map[string]string{}}
	if st.Timezone == "Local" {
		// Stored by name, the days stay the same wherever the app runs.
		if zone := localZoneName(); zone != "" {
			form.Timezone, b.Zone = zone, zone
		}
	}
	for key := range form.Values {
		form.Errors[key] = ""
	}
	form.Errors["timezone"], form.Errors["review_weekday"] = "", ""
	if b.Signals, err = marshalSignals(form); err != nil {
		return nil, err
	}
	return b, nil
}

// settingsValues writes every whole-number setting the form shows.
func settingsValues(st library.Settings) map[string]string {
	n := map[string]int{
		"wip_cap": st.WIPCap, "stall_days": st.StallDays,
		"bucket_quick_max_min": st.BucketQuickMaxMin, "bucket_hour_max_min": st.BucketHourMaxMin,
		"pace_window_days": st.PaceWindowDays, "projection_window_weeks": st.ProjectionWindowWeeks, "words_per_page": st.WordsPerPage,
		"seed_pace_light": st.SeedPaceLight, "seed_pace_medium": st.SeedPaceMedium, "seed_pace_deep": st.SeedPaceDeep,
		"seed_pace_wpm": st.SeedPaceWPM, "fallback_book_pages": st.FallbackBookPages,
	}
	out := make(map[string]string, len(n))
	for k, v := range n {
		out[k] = strconv.Itoa(v)
	}
	return out
}

// postSettings saves the form. Each value lands in its field or is refused
// there with the reason.
func (h *handler) postSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	st, err := h.svc.Settings(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	targets := map[string]*int{
		"wip_cap": &st.WIPCap, "stall_days": &st.StallDays,
		"bucket_quick_max_min": &st.BucketQuickMaxMin, "bucket_hour_max_min": &st.BucketHourMaxMin,
		"pace_window_days": &st.PaceWindowDays, "projection_window_weeks": &st.ProjectionWindowWeeks, "words_per_page": &st.WordsPerPage,
		"seed_pace_light": &st.SeedPaceLight, "seed_pace_medium": &st.SeedPaceMedium, "seed_pace_deep": &st.SeedPaceDeep,
		"seed_pace_wpm": &st.SeedPaceWPM, "fallback_book_pages": &st.FallbackBookPages,
	}
	for key, target := range targets {
		n, ok := wholeNumber(in.Values[key])
		if !ok {
			h.settingsError(w, r, key, "Use a whole number.")
			return
		}
		*target = n
	}
	review, ok := wholeNumber(in.Review)
	if !ok {
		h.settingsError(w, r, "review_weekday", "Pick a day.")
		return
	}
	st.ReviewWeekday = time.Weekday(review)
	st.Timezone = strings.TrimSpace(in.Timezone)
	// The hour bucket starts no later than it ends; only its end filters.
	st.BucketHourMinMin = min(st.BucketHourMinMin, st.BucketHourMaxMin)

	err = h.svc.UpdateSettings(ctx, *st)
	var verr *library.ValidationError
	if errors.As(err, &verr) {
		msg := verr.Msg
		if verr.Field == "timezone" {
			msg = "Not a time zone. Try a name like America/Buenos_Aires."
		}
		h.settingsError(w, r, verr.Field, msg)
		return
	}
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	body, err := h.settingsBody(ctx, "Saved.")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.settings, "settings-body", body); err != nil {
		h.log.Error("settings body", "err", err)
		return
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("settings reset", "err", err)
	}
}

// settingsError puts a refusal in its field's slot and focuses the field.
func (h *handler) settingsError(w http.ResponseWriter, r *http.Request, key, msg string) {
	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(map[string]any{"errors": map[string]string{key: msg}}); err != nil {
		h.log.Error("settings errors", "err", err)
		return
	}
	if err := sse.ExecuteScript(`document.getElementById("setting-` + key + `")?.focus()`); err != nil {
		h.log.Error("settings error focus", "err", err)
	}
}

// localZoneName is the name of this machine's time zone, as TZ or the
// /etc/localtime link gives it; "" when it cannot be told.
func localZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" && !strings.HasPrefix(tz, ":") {
		return tz
	}
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	_, name, found := strings.Cut(target, "zoneinfo/")
	if !found {
		return ""
	}
	if _, err := time.LoadLocation(name); err != nil {
		return ""
	}
	return name
}
