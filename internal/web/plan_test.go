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
		`No daily target yet.`,
		`data-bind="days.sun"`, `id="day-first"`,
		`value="ramp" data-bind="kind"`,
		`Save the target`,
		`adds its step every Sunday`,
		`How hours are counted`, `How speed is measured`,
		`id="plan-summary"`, `Fill in the times, like 1h30, 1:30 or 90.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan page missing %q", want)
		}
	}
	if strings.Contains(body, `class="days"`) || strings.Contains(body, "to go tonight") {
		t.Error("no week drawn before a plan exists")
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
	if sig := patchedSignals(t, body); sig["minutes"] != "1h" {
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
	for _, want := range []string{"1 h 00 min a day · every day", "<span>Ramp</span>", "rises to <strong>1 h 30 min</strong> on "} {
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
		{"not a number", fixedPlan("an hour"), "minutes", "Type a time like 1h30, 1:30 or 90."},
		{"zero minutes", fixedPlan("0"), "minutes", "Between 1 minute and 24 h."},
		{"no days", noDays, "days", "Pick at least one day."},
		{"ceiling at start", badCeiling, "ceiling", "Above the start, and at most 24 h."},
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

func TestBoard(t *testing.T) {
	loc := time.UTC
	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	read := func(d, hour int, length time.Duration) library.Session {
		start := time.Date(2026, 9, d, hour, 0, 0, 0, loc)
		end := start.Add(length)
		return library.Session{ItemID: "x", StartedAt: start, EndedAt: &end}
	}
	// Mon–Fri at 1 h 30 from Mon 14 Sep; Mon 60, Tue 90, Wed (today) 25.
	sc := library.ReplaySchedule(
		[]library.ActiveDays{{EffectiveOn: day(14), Days: library.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)}},
		[]library.Commitment{{EffectiveOn: day(14), Kind: library.CommitFixed, MinutesPerDay: 90}},
		[]library.Session{read(14, 9, time.Hour), read(15, 9, 90*time.Minute), read(16, 9, 25*time.Minute)},
		time.Sunday, loc, time.Date(2026, 9, 16, 12, 0, 0, 0, loc))

	b := newBoard(sc, library.Speed{}, 300)
	if b.ToGo != "1 h 35 min" || b.ToGoLine != "Meets today's target and clears the 30 min owed." {
		t.Errorf("tonight: %q, %q", b.ToGo, b.ToGoLine)
	}
	if b.Today == nil || b.Today.Left != "1 h 05 min to the target" || b.Today.Owed != "30 min" || !b.Today.Bar.ShowDebt {
		t.Errorf("today: %+v", b.Today)
	}
	if b.Week.Due != "4 h 30 min" || b.Week.Behind != "1 h 35 min behind" || b.Week.InAll != "7 h 30 min" {
		t.Errorf("week: %+v", b.Week)
	}
	if len(b.Days) != 7 || !b.Days[0].Unplanned || !b.Days[1].Short || b.Days[1].Owed != "30 min" || !b.Days[3].Today || !b.Days[4].Future || !b.Days[6].Rest {
		t.Errorf("days: %+v", b.Days)
	}
	tips := func(c dayCell) string {
		var lines []string
		for _, tp := range c.Tips {
			lines = append(lines, tp.Text)
		}
		return c.Title + " | " + strings.Join(lines, " | ")
	}
	for i, want := range map[int]string{
		0: "Sunday 13 Sep | Before your plan began: no target",
		1: "Monday 14 Sep | Read 1 h 00 min of 1 h 30 min | 30 min short, added to what you owe | 30 min owed in all after this day",
		2: "Tuesday 15 Sep | Read 1 h 30 min of 1 h 30 min | Target met | 30 min owed in all after this day",
		3: "Wednesday 16 Sep · today | Read 25 min of 1 h 30 min so far | 1 h 05 min left for the target | Whatever is still short at midnight is added to what you owe",
		4: "Thursday 17 Sep | Target 1 h 30 min | Still to come",
		6: "Saturday 19 Sep | Rest day: no target",
	} {
		if got := tips(b.Days[i]); got != want {
			t.Errorf("day %d popover:\n got %s\nwant %s", i, got, want)
		}
	}
	if b.Daily != "1 h 30 min a day · Mon–Fri" || b.Speed != nil {
		t.Errorf("daily %q, speed %+v", b.Daily, b.Speed)
	}

	var buf bytes.Buffer
	tmpl := template.Must(template.ParseFS(assets, "templates/board.html"))
	if err := tmpl.ExecuteTemplate(&buf, "board-hours", b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<span class="hero-figure">1 h 35 min</span>`, "to go tonight", `<span class="owed">30 min owed</span>`, "4 h 30 min</strong> due so far", `aria-current="date"`, `class="bar-debt"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("hours block missing %q", want)
		}
	}

	none := newBoard(library.Schedule{Today: day(16), LoggedToday: 40 * time.Minute}, library.Speed{}, 300)
	buf.Reset()
	if err := tmpl.ExecuteTemplate(&buf, "board-hours", none); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No daily target yet.") || strings.Contains(buf.String(), "to go tonight") {
		t.Errorf("without a plan:\n%s", buf.String())
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
	for _, want := range []string{"Speed ramp started.", `<span class="hero-figure">100%</span>`, "of your baseline this week", "Stop the speed ramp", "This week so far"} {
		if !strings.Contains(body, want) {
			t.Errorf("started body missing %q", want)
		}
	}

	body = send(t, h, http.MethodPost, "/plan/speed/stop", in).Body.String()
	if !strings.Contains(body, "Speed ramp stopped.") || !strings.Contains(body, "was stopped on") || strings.Contains(body, "Stop the speed ramp") {
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
	tmpl := template.Must(template.ParseFS(assets, "templates/board.html"))
	if err := tmpl.ExecuteTemplate(&buf, "board-speed", newBoard(library.Schedule{}, library.Speed{LastWeek: week}, 300)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"32 pages/h", "160 words/min", `data-bind="_unit"`, "Last week, 13–19 Sep", "book · medium focus 70%", `class="cloth-book"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("speed block missing %q", want)
		}
	}
}

func TestPlanPreviewSummary(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	weekdays := planSignals("ramp")
	weekdays.Days = map[string]bool{"mon": true, "tue": true, "wed": true, "thu": true, "fri": true}
	weekdays.Start, weekdays.Increment, weekdays.Ceiling = "1h30", "30", "4h"
	body := send(t, h, http.MethodPost, "/plan/preview", weekdays).Body.String()
	want := "If you save: from today, 1 h 30 min on Monday to Friday, rising 30 min each Sunday while nothing is owed, up to 4 h 00 min. Today counts and closes at midnight."
	if !strings.Contains(body, `id="plan-summary"`) || !strings.Contains(body, want) {
		t.Fatalf("summary:\n%s", body)
	}

	some := fixedPlan("1:15")
	some.Days = map[string]bool{"mon": true, "wed": true, "fri": true}
	if body := send(t, h, http.MethodPost, "/plan/preview", some).Body.String(); !strings.Contains(body, "1 h 15 min on Monday, Wednesday and Friday.") {
		t.Fatalf("fixed summary:\n%s", body)
	}
}

func TestPostPlanAcceptsHours(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	in := planSignals("ramp")
	in.Start, in.Increment, in.Ceiling = "1.5h", "0:30", "4h 15m"
	if rec := send(t, h, http.MethodPost, "/plan", in); !strings.Contains(rec.Body.String(), "Plan saved.") {
		t.Fatalf("save:\n%s", rec.Body.String())
	}
	view, err := svc.Plan(ctx)
	if err != nil || view.Schedule.Ramp == nil || view.Schedule.Ramp.Current != 90 || view.Schedule.Ramp.Increment != 30 || view.Schedule.Ramp.Ceiling != 255 {
		t.Fatalf("ramp stored in minutes: %v %+v", err, view.Schedule.Ramp)
	}
	if sig := pageSignals[planForm](t, get(t, h, "/plan").Body.String()); sig.Start != "1h30" || sig.Ceiling != "4h15" {
		t.Fatalf("fields written back as hours: %+v", sig)
	}
}

// The popover only says reading paid something back when something was owed.
func TestDayTipsPayBackOnlyWhatWasOwed(t *testing.T) {
	day := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) // a Tuesday
	sheet := func(target int, read, before, after time.Duration) library.DaySheet {
		return library.DaySheet{Day: day, Planned: true, Active: target > 0, Target: target, Logged: read, Closed: true, OwedBefore: before, OwedAfter: after}
	}
	cases := []struct {
		name  string
		d     library.DaySheet
		c     dayCell
		lines string
	}{
		{"over, owing", sheet(60, 90*time.Minute, 45*time.Minute, 15*time.Minute), dayCell{},
			"Read 1 h 30 min of 1 h 00 min | Target met; the extra 30 min paid back 30 min you owed | 15 min owed in all after this day"},
		{"over, owing less than the extra", sheet(60, 2*time.Hour, 20*time.Minute, 0), dayCell{},
			"Read 2 h 00 min of 1 h 00 min | Target met; the extra 1 h 00 min paid back 20 min you owed | Nothing owed after this day"},
		{"over, owing nothing", sheet(60, 90*time.Minute, 0, 0), dayCell{},
			"Read 1 h 30 min of 1 h 00 min | Target met; extra reading isn't saved for later | Nothing owed after this day"},
		{"rest, owing", sheet(0, 20*time.Minute, time.Hour, 40*time.Minute), dayCell{Rest: true},
			"Rest day: no target | Read 20 min, paying back 20 min you owed | 40 min owed in all after this day"},
		{"rest, owing nothing", sheet(0, 20*time.Minute, 0, 0), dayCell{Rest: true},
			"Rest day: no target | Read 20 min | Nothing owed after this day"},
		{"today met, owing", sheet(60, 70*time.Minute, 30*time.Minute, 0), dayCell{Today: true},
			"Read 1 h 10 min of 1 h 00 min so far | Target met | Reading beyond it pays back what you owe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, tips := dayTips(c.d, c.c)
			var lines []string
			for _, tp := range tips {
				lines = append(lines, tp.Text)
			}
			if got := strings.Join(lines, " | "); got != c.lines {
				t.Errorf("\n got %s\nwant %s", got, c.lines)
			}
		})
	}
}

// The summary tells a target already in effect apart from a change, and the
// board says when a week ahead still owes time from a short day.
func TestPlanSaysWhatIsInEffect(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitFixed, MinutesPerDay: 60}, false); err != nil {
		t.Fatal(err)
	}
	if body := send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h")).Body.String(); !strings.Contains(body, "This is the target in effect. Saving changes nothing.") {
		t.Fatalf("unchanged plan:\n%s", body)
	}
	if body := send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h30")).Body.String(); !strings.Contains(body, "If you save: from today, 1 h 30 min") {
		t.Fatalf("changed plan:\n%s", body)
	}
}

func TestBoardSaysAWeekAheadStillOwes(t *testing.T) {
	c := library.Commitment{Kind: library.CommitRamp, StartMinutes: 45, IncrementMinutes: 15, CeilingMinutes: 240}
	today := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	sc := library.Schedule{
		Today: today, Days: library.AllWeekdays, Commitment: &c, Value: 45, TargetToday: 45,
		OwedAtMidnight: time.Minute, Owed: time.Minute, WeekStart: today.AddDate(0, 0, -4),
		WeekTarget: 315, DueSoFar: 225, WeekLogged: 240 * time.Minute,
		Ramp: &library.HoursRamp{Current: 45, Increment: 15, Ceiling: 240, NextCheck: today.AddDate(0, 0, 3)},
	}
	b := newBoard(sc, library.Speed{}, 300)
	if !b.Week.Owed || b.Week.Behind != "ahead for the week, 1 min still owed" {
		t.Fatalf("week line %+v", b.Week)
	}
	if b.Ramp.HeldBy != "the 1 min owed" || b.Ramp.Top == "" {
		t.Fatalf("ramp line %+v", b.Ramp)
	}
}
