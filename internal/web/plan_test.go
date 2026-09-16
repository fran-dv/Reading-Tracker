package web

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// planSignals is a form with every day active, so the tests never land on a
// rest day whatever the date they run on.
func planSignals(kind string) planForm {
	days := map[string]bool{}
	for _, key := range weekdayKeys {
		days[key] = true
	}
	return planForm{Days: days, Kind: kind}
}

func fixedPlan(minutes string) planForm {
	in := planSignals("fixed")
	in.Minutes = minutes
	return in
}

func TestPlanPageBeforeAnyPlan(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	body := get(t, h, "/plan").Body.String()
	for _, want := range []string{
		`href="/plan" aria-current="page"`,
		`Today <strong>0 min</strong>`,
		`data-bind="days.sun"`, `id="day-first"`,
		`value="ramp" data-bind="kind"`,
		`Save plan`,
		`A ramp is checked each Sunday.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan page missing %q", want)
		}
	}
	if strings.Contains(body, "This week") {
		t.Error("no week section before a plan exists")
	}
	sig := pageSignals[planForm](t, body)
	if sig.Kind != "fixed" || !sig.Days["mon"] || !sig.Days["fri"] || sig.Days["sat"] || sig.Days["sun"] {
		t.Errorf("first plan should prefill Mon–Fri, fixed: %+v", sig)
	}
}

func TestPostPlanSaves(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	rec := send(t, h, http.MethodPost, "/plan", fixedPlan("60"))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `id="plan-body"`) || !strings.Contains(body, "Plan saved.") {
		t.Fatalf("status %d:\n%s", rec.Code, body)
	}
	for _, want := range []string{"This week", "1 h 00 min a day", "of <strong>1 h 00 min</strong>"} {
		if !strings.Contains(body, want) {
			t.Errorf("saved body missing %q", want)
		}
	}
	if sig := patchedSignals(t, body); sig["minutes"] != "60" {
		t.Errorf("form reset to the saved plan: %v", sig)
	}
	view, err := svc.Plan(ctx)
	if err != nil || view.Schedule.Value != 60 {
		t.Fatalf("plan not saved: %v %+v", err, view)
	}

	// Home carries the same strip and week.
	home := get(t, h, "/").Body.String()
	for _, want := range []string{"of <strong>1 h 00 min</strong>", "This week", "href=\"/plan\""} {
		if !strings.Contains(home, want) {
			t.Errorf("home missing %q", want)
		}
	}
}

func TestPostPlanLoweringAsksFirst(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	if rec := send(t, h, http.MethodPost, "/plan", fixedPlan("90")); rec.Code != http.StatusOK {
		t.Fatalf("first save: %d", rec.Code)
	}

	rec := send(t, h, http.MethodPost, "/plan", fixedPlan("30"))
	body := rec.Body.String()
	if !strings.Contains(body, `id="plan-actions"`) || strings.Contains(body, `id="plan-body"`) {
		t.Fatalf("lowering should patch only the actions:\n%s", body)
	}
	for _, want := range []string{"drops from", "1 h 30 min", "Lower to 30 min", "Keep 1 h 30 min", "The days already closed stay as they are."} {
		if !strings.Contains(body, want) {
			t.Errorf("confirmation missing %q", want)
		}
	}
	if view, _ := svc.Plan(ctx); view.Schedule.Value != 90 {
		t.Fatal("nothing is saved before the confirmation")
	}

	rec = send(t, h, http.MethodPost, "/plan/lower", fixedPlan("30"))
	if !strings.Contains(rec.Body.String(), "Plan saved.") {
		t.Fatalf("confirmed lowering:\n%s", rec.Body.String())
	}
	if view, _ := svc.Plan(ctx); view.Schedule.Value != 30 {
		t.Fatalf("confirmed lowering not saved: %+v", view.Schedule)
	}
}

func TestPostPlanRamp(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	in := planSignals("ramp")
	in.Start, in.Increment, in.Ceiling = "60", "30", "240"
	body := send(t, h, http.MethodPost, "/plan", in).Body.String()
	for _, want := range []string{">Ramp<", "1 h 00 min a day</strong>, rising 30 min a week to 4 h 00 min", "next check "} {
		if !strings.Contains(body, want) {
			t.Errorf("ramp body missing %q", want)
		}
	}
	if view, _ := svc.Plan(ctx); view.Schedule.Ramp == nil {
		t.Fatal("ramp not saved")
	}
}

func TestPostPlanValidationInBand(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	noDays := fixedPlan("60")
	noDays.Days = map[string]bool{}
	badCeiling := planSignals("ramp")
	badCeiling.Start, badCeiling.Increment, badCeiling.Ceiling = "60", "30", "60"

	cases := []struct {
		name string
		in   planForm
		slot string
		msg  string
	}{
		{"not a number", fixedPlan("an hour"), "minutes", "Use a whole number of minutes."},
		{"zero minutes", fixedPlan("0"), "minutes", "Between 1 and 1440 minutes."},
		{"no days", noDays, "days", "Pick at least one day."},
		{"ceiling at start", badCeiling, "ceiling", "Above the start, and at most 1440 minutes."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := send(t, h, http.MethodPost, "/plan", c.in).Body.String()
			errs, _ := patchedSignals(t, body)["errors"].(map[string]any)
			if errs[c.slot] != c.msg {
				t.Fatalf("errors = %v, want %s: %q", errs, c.slot, c.msg)
			}
			if !strings.Contains(body, planInputs[c.slot]) {
				t.Fatalf("focus should move to %s:\n%s", planInputs[c.slot], body)
			}
		})
	}
}

func TestStandingStrip(t *testing.T) {
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) // a Sunday
	owed := library.Schedule{
		Today: day, Days: library.WeekdaysOf(time.Monday), Commitment: &library.Commitment{Kind: library.CommitRamp},
		Value: 90, LoggedToday: 40 * time.Minute, Owed: 80 * time.Minute, WeekTarget: 90,
		Ramp: &library.HoursRamp{Current: 90, Increment: 30, Ceiling: 240, NextCheck: day.AddDate(0, 0, 7),
			LastCheck: &library.RampCheck{On: day}},
	}
	cases := []struct {
		name string
		sc   library.Schedule
		want []string
		not  []string
	}{
		{"no plan", library.Schedule{Today: day, LoggedToday: 40 * time.Minute},
			[]string{"Today <strong>40 min</strong></span>"}, []string{"of <strong>", "Owed", "This week"}},
		{"rest day, owed, held ramp", owed,
			[]string{"<span>rest day</span>", `<span class="owed">Owed <strong>1 h 20 min</strong>`,
				"next check Sun 27 Sep", "held on 20 Sep, something was owed"},
			[]string{"Today <strong>40 min</strong> of"}},
	}
	tmpl := template.Must(template.ParseFS(assets, "templates/standing.html"))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, "standing", newStanding(c.sc, library.Speed{}, 300)); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			for _, want := range c.want {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in\n%s", want, out)
				}
			}
			for _, not := range c.not {
				if strings.Contains(out, not) {
					t.Errorf("unexpected %q in\n%s", not, out)
				}
			}
		})
	}
}

func TestPlanSpeedRamp(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	before := get(t, h, "/plan").Body.String()
	if !strings.Contains(before, "A speed ramp starts from speed you have measured.") || strings.Contains(before, "Start a speed ramp") {
		t.Fatal("without a measured speed there is nothing to start from")
	}

	shelf, err := svc.CreateShelf(ctx, "Statistics")
	if err != nil {
		t.Fatal(err)
	}
	book := fileItem(t, svc, library.Item{Title: "Deep Work", Why: "focus", Format: library.FormatBook, ShelfID: shelf.ID, SizeValue: ptr(296)})
	if _, err := svc.Start(ctx, book.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := svc.AddRetroactiveSession(ctx, book.ID, now.Add(-50*time.Hour), now.Add(-48*time.Hour), ptr(60), ""); err != nil {
		t.Fatal(err)
	}

	page := get(t, h, "/plan").Body.String()
	for _, want := range []string{"Starts from", "book · medium", "30 pages/h", "Start a speed ramp", `data-bind="speedIncrement"`} {
		if !strings.Contains(page, want) {
			t.Errorf("plan missing %q", want)
		}
	}
	if sig := pageSignals[planForm](t, page); sig.SpeedIncrement != "5" || sig.SpeedCeiling != "130" {
		t.Errorf("speed form defaults: %+v", sig)
	}

	bad := planSignals("fixed")
	bad.SpeedIncrement, bad.SpeedCeiling = "5", "100"
	errs, _ := patchedSignals(t, send(t, h, http.MethodPost, "/plan/speed", bad).Body.String())["errors"].(map[string]any)
	if errs["speedCeiling"] != "Above 100%, and at most 1000%." {
		t.Fatalf("ceiling at 100: %v", errs)
	}

	in := planSignals("fixed")
	in.SpeedIncrement, in.SpeedCeiling = "5", "130"
	body := send(t, h, http.MethodPost, "/plan/speed", in).Body.String()
	for _, want := range []string{"Speed ramp started.", "Running, target", "100%", "Stop the speed ramp", "Speed index", "no closed week yet"} {
		if !strings.Contains(body, want) {
			t.Errorf("started body missing %q", want)
		}
	}

	body = send(t, h, http.MethodPost, "/plan/speed/stop", in).Body.String()
	if !strings.Contains(body, "Speed ramp stopped.") || !strings.Contains(body, "Stopped on") || strings.Contains(body, "Speed index") {
		t.Fatalf("stopped body:\n%s", body)
	}
}

func TestSpeedLines(t *testing.T) {
	from := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	week := library.WeekSpeed{From: from, To: from.AddDate(0, 0, 7), Measured: 5 * time.Hour, PagesPerHour: 32,
		Mix: []library.Mix{
			{Format: library.FormatBook, FocusDemand: library.FocusMedium, Share: 0.7},
			{Format: library.FormatArticle, FocusDemand: library.FocusLight, Share: 0.3},
		}}
	line := newSpeedLine(week, 300)
	if line.Dates != "13–19 Sep" || line.Pages != "32 pages/h" || line.Words != "160 words/min" ||
		line.Mix != "book\u00a0·\u00a0medium\u00a070%, article\u00a0·\u00a0light\u00a030%" || !line.Enough {
		t.Fatalf("speed line %+v", line)
	}
	if newSpeedLine(library.WeekSpeed{}, 300) != nil {
		t.Fatal("nothing measured draws no line")
	}
	if got := weekDates(time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)); got != "27 Sep – 3 Oct" {
		t.Fatalf("across months: %q", got)
	}

	var buf bytes.Buffer
	tmpl := template.Must(template.ParseFS(assets, "templates/standing.html"))
	if err := tmpl.ExecuteTemplate(&buf, "standing", newStanding(library.Schedule{}, library.Speed{LastWeek: week}, 300)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"32 pages/h", "160 words/min", `data-bind="_unit"`, "13–19 Sep, from 5 h 00 min: book\u00a0·\u00a0medium\u00a070%"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("standing missing %q", want)
		}
	}
}
