package library_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Calendar used below (2026): Sun 6 Sep, Mon 7, Thu 10, Sun 13, Wed 16,
// Sun 20, Sun 27, Sun 4 Oct. Weeks start on Sunday unless a test says so.

// day is a calendar day as the library carries it: midnight UTC.
func day(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

// read is a finished session on a calendar day in loc, starting at hour.
func read(loc *time.Location, on time.Time, hour int, length time.Duration) library.Session {
	y, m, d := on.Date()
	start := time.Date(y, m, d, hour, 0, 0, 0, loc)
	return session("item", start, length, nil, nil)
}

// readDaily reads length at noon on every day in [from, to).
func readDaily(loc *time.Location, from, to time.Time, length time.Duration) []library.Session {
	var out []library.Session
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		out = append(out, read(loc, d, 12, length))
	}
	return out
}

func noon(loc *time.Location, on time.Time) time.Time {
	y, m, d := on.Date()
	return time.Date(y, m, d, 12, 0, 0, 0, loc)
}

var weekdaysMonFri = library.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)

func fixed(on time.Time, minutes int) library.Commitment {
	return library.Commitment{EffectiveOn: on, Kind: library.CommitFixed, MinutesPerDay: minutes}
}

func ramp(on time.Time, start, increment, ceiling int) library.Commitment {
	return library.Commitment{EffectiveOn: on, Kind: library.CommitRamp,
		StartMinutes: start, IncrementMinutes: increment, CeilingMinutes: ceiling}
}

func everyDay(on time.Time) []library.ActiveDays {
	return []library.ActiveDays{{EffectiveOn: on, Days: library.AllWeekdays}}
}

func TestReplayWithoutPlan(t *testing.T) {
	loc := time.UTC
	sessions := []library.Session{read(loc, day(9, 16), 8, 40*time.Minute)}
	sc := library.ReplaySchedule(nil, nil, sessions, time.Sunday, loc, noon(loc, day(9, 16)))
	if sc.Planned() || sc.RestDay() || sc.Owed != 0 || sc.TargetToday != 0 || sc.WeekTarget != 0 {
		t.Fatalf("no plan should mean no target and no debt: %+v", sc)
	}
	if sc.LoggedToday != 40*time.Minute || sc.WeekLogged != 40*time.Minute || !sc.WeekStart.Equal(day(9, 13)) {
		t.Fatalf("logged %v, week %v from %v", sc.LoggedToday, sc.WeekLogged, sc.WeekStart)
	}
}

func TestReplayDebt(t *testing.T) {
	loc := time.UTC
	days := []library.ActiveDays{{EffectiveOn: day(9, 7), Days: weekdaysMonFri}}
	commitments := []library.Commitment{fixed(day(9, 7), 60)}
	sessions := []library.Session{
		read(loc, day(9, 7), 9, 30*time.Minute),  // Mon: short 30 → owed 30
		read(loc, day(9, 8), 9, 100*time.Minute), // Tue: 40 over → floor at 0
		// Wed: nothing → owed 60
		read(loc, day(9, 10), 9, 60*time.Minute), // Thu: on target → 60
		read(loc, day(9, 11), 9, 50*time.Minute), // Fri: short 10 → 70
		read(loc, day(9, 12), 9, 45*time.Minute), // Sat, rest day: pays 45 → 25
	}
	sc := library.ReplaySchedule(days, commitments, sessions, time.Sunday, loc, noon(loc, day(9, 13)))
	if sc.OwedAtMidnight != 25*time.Minute || sc.Owed != 25*time.Minute {
		t.Fatalf("owed %v at midnight, %v now; want 25m", sc.OwedAtMidnight, sc.Owed)
	}
	if !sc.RestDay() || sc.TargetToday != 0 || sc.Value != 60 {
		t.Fatalf("Sunday is a rest day with the value still 60: %+v", sc)
	}
}

func TestReplaySurplusNeverBanks(t *testing.T) {
	loc := time.UTC
	sessions := []library.Session{
		read(loc, day(9, 7), 9, 5*time.Hour), // far over
		// Tue: nothing
	}
	sc := library.ReplaySchedule(everyDay(day(9, 7)), []library.Commitment{fixed(day(9, 7), 60)},
		sessions, time.Sunday, loc, noon(loc, day(9, 9)))
	if sc.OwedAtMidnight != time.Hour {
		t.Fatalf("owed %v, want 1h: Monday's surplus must not cover Tuesday", sc.OwedAtMidnight)
	}
}

func TestReplayOwedPaysDownLive(t *testing.T) {
	loc := time.UTC
	days, commitments := everyDay(day(9, 14)), []library.Commitment{fixed(day(9, 14), 60)}
	// Monday 14 closes 60 short.
	cases := []struct {
		name      string
		readToday time.Duration
		want      time.Duration
	}{
		{"nothing yet: today's shortfall is not added", 0, time.Hour},
		{"below target", 30 * time.Minute, time.Hour},
		{"on target", time.Hour, time.Hour},
		{"surplus pays down", 90 * time.Minute, 30 * time.Minute},
		{"floor at zero", 4 * time.Hour, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var sessions []library.Session
			if c.readToday > 0 {
				sessions = append(sessions, read(loc, day(9, 15), 6, c.readToday))
			}
			sc := library.ReplaySchedule(days, commitments, sessions, time.Sunday, loc, noon(loc, day(9, 15)))
			if sc.OwedAtMidnight != time.Hour || sc.Owed != c.want {
				t.Fatalf("owed %v at midnight, %v live; want 1h, %v", sc.OwedAtMidnight, sc.Owed, c.want)
			}
		})
	}
}

func TestReplayRunningSessionCountsToday(t *testing.T) {
	loc := time.UTC
	running := session("item", noon(loc, day(9, 15)).Add(-90*time.Minute), 0, nil, nil)
	sc := library.ReplaySchedule(everyDay(day(9, 15)), []library.Commitment{fixed(day(9, 15), 60)},
		[]library.Session{running}, time.Sunday, loc, noon(loc, day(9, 15)))
	if sc.LoggedToday != 90*time.Minute || sc.TargetToday != 60 {
		t.Fatalf("logged %v target %d", sc.LoggedToday, sc.TargetToday)
	}
}

func TestReplayWeekTarget(t *testing.T) {
	loc := time.UTC
	days := []library.ActiveDays{{EffectiveOn: day(9, 1), Days: weekdaysMonFri}}
	full := library.ReplaySchedule(days, []library.Commitment{fixed(day(9, 1), 60)}, nil, time.Sunday, loc, noon(loc, day(9, 16)))
	if full.WeekTarget != 300 {
		t.Fatalf("Mon–Fri at 60: week target %d, want 300", full.WeekTarget)
	}
	// Committed on Wednesday: Mon and Tue carried no target.
	late := library.ReplaySchedule(days, []library.Commitment{fixed(day(9, 16), 60)}, nil, time.Sunday, loc, noon(loc, day(9, 16)))
	if late.WeekTarget != 180 {
		t.Fatalf("committed Wednesday: week target %d, want 180", late.WeekTarget)
	}
	// Changed mid-week: past days keep their old value, the rest use today's.
	changed := library.ReplaySchedule(days, []library.Commitment{fixed(day(9, 1), 60), fixed(day(9, 16), 30)}, nil,
		time.Sunday, loc, noon(loc, day(9, 16)))
	if changed.WeekTarget != 60+60+30+30+30 {
		t.Fatalf("mid-week change: week target %d, want 210", changed.WeekTarget)
	}
}

func TestReplayRampWaitsForAFullWeek(t *testing.T) {
	loc := time.UTC
	// Started Thursday 10 Sep at 60, +30 a week, up to 120; always read enough.
	commitments := []library.Commitment{ramp(day(9, 10), 60, 30, 120)}
	sessions := readDaily(loc, day(9, 10), day(10, 10), 3*time.Hour)
	at := func(on time.Time) library.Schedule {
		return library.ReplaySchedule(everyDay(day(9, 10)), commitments, sessions, time.Sunday, loc, noon(loc, on))
	}

	offset := at(day(9, 13)) // first Sunday, three days in
	if offset.Ramp == nil || offset.Ramp.Current != 60 || offset.Ramp.LastCheck != nil || !offset.Ramp.NextCheck.Equal(day(9, 20)) {
		t.Fatalf("first Sunday must not check: %+v", offset.Ramp)
	}
	first := at(day(9, 20))
	if first.Ramp == nil || first.Ramp.Current != 90 || !first.Ramp.LastCheck.Advanced || !first.Ramp.NextCheck.Equal(day(9, 27)) {
		t.Fatalf("second Sunday advances: %+v", first.Ramp)
	}
	if first.TargetToday != 90 {
		t.Fatalf("target after advance %d, want 90", first.TargetToday)
	}
	done := at(day(9, 28))
	if done.Ramp != nil || done.Value != 120 || !done.ReachedCeilingOn.Equal(day(9, 27)) || done.Commitment.Kind != library.CommitRamp {
		t.Fatalf("ceiling ends the ramp at a fixed 120: %+v", done)
	}
	later := at(day(10, 5))
	if later.Ramp != nil || later.Value != 120 {
		t.Fatalf("after the ceiling it stays fixed: %+v", later)
	}
}

func TestReplayRampHoldsWhileOwed(t *testing.T) {
	loc := time.UTC
	commitments := []library.Commitment{ramp(day(9, 6), 60, 30, 240)}
	sessions := readDaily(loc, day(9, 6), day(9, 12), time.Hour) // Sat 12 unread → owed 60
	sc := library.ReplaySchedule(everyDay(day(9, 6)), commitments, sessions, time.Sunday, loc, noon(loc, day(9, 13)))
	if sc.Ramp.Current != 60 || sc.Ramp.LastCheck == nil || sc.Ramp.LastCheck.Advanced {
		t.Fatalf("owed at the check: must hold, got %+v", sc.Ramp)
	}
	if !sc.Ramp.NextCheck.Equal(day(9, 20)) {
		t.Fatalf("a held value already served its week; next check %v, want 20 Sep", sc.Ramp.NextCheck)
	}

	// Saturday's reading entered late clears the debt: the past check advances.
	late := append(sessions, read(loc, day(9, 12), 20, time.Hour))
	sc = library.ReplaySchedule(everyDay(day(9, 6)), commitments, late, time.Sunday, loc, noon(loc, day(9, 13)))
	if sc.Ramp.Current != 90 || !sc.Ramp.LastCheck.Advanced {
		t.Fatalf("a late entry must correct the check: %+v", sc.Ramp)
	}
}

func TestReplayEditingRampAtCurrentValueKeepsItsWeek(t *testing.T) {
	loc := time.UTC
	sessions := readDaily(loc, day(9, 6), day(9, 20), 5*time.Hour)
	base := ramp(day(9, 6), 60, 30, 120) // advances to 90 on Sun 13

	// Wed 16: raise the ceiling, starting at the current 90.
	same := []library.Commitment{base, ramp(day(9, 16), 90, 30, 240)}
	sc := library.ReplaySchedule(everyDay(day(9, 6)), same, sessions, time.Sunday, loc, noon(loc, day(9, 20)))
	if sc.Ramp.Current != 120 || !sc.Ramp.StartedOn.Equal(day(9, 16)) {
		t.Fatalf("value held since 13 Sep, so 20 Sep checks: %+v", sc.Ramp)
	}

	// Wed 16: a new ramp at a different value waits for its own week.
	other := []library.Commitment{base, ramp(day(9, 16), 100, 30, 240)}
	sc = library.ReplaySchedule(everyDay(day(9, 6)), other, sessions, time.Sunday, loc, noon(loc, day(9, 20)))
	if sc.Ramp.Current != 100 || sc.Ramp.LastCheck != nil || !sc.Ramp.NextCheck.Equal(day(9, 27)) {
		t.Fatalf("new value started Wednesday must wait: %+v", sc.Ramp)
	}
}

func TestReplayFixedEndsRamp(t *testing.T) {
	loc := time.UTC
	commitments := []library.Commitment{ramp(day(9, 6), 60, 30, 240), fixed(day(9, 16), 45)}
	sc := library.ReplaySchedule(everyDay(day(9, 6)), commitments, nil, time.Sunday, loc, noon(loc, day(9, 16)))
	if sc.Ramp != nil || sc.Value != 45 || sc.Commitment.Kind != library.CommitFixed {
		t.Fatalf("latest decision wins: %+v", sc)
	}
}

func TestReplayDaysChangeKeepsRamp(t *testing.T) {
	loc := time.UTC
	days := []library.ActiveDays{
		{EffectiveOn: day(9, 6), Days: weekdaysMonFri},
		{EffectiveOn: day(9, 16), Days: weekdaysMonFri | library.WeekdaysOf(time.Saturday)},
	}
	commitments := []library.Commitment{ramp(day(9, 6), 60, 30, 240)}
	sessions := readDaily(loc, day(9, 6), day(9, 20), 5*time.Hour)
	sc := library.ReplaySchedule(days, commitments, sessions, time.Sunday, loc, noon(loc, day(9, 20)))
	if sc.Ramp.Current != 120 || !sc.Ramp.StartedOn.Equal(day(9, 6)) {
		t.Fatalf("changing days must not restart the ramp: %+v", sc.Ramp)
	}
	if sc.WeekTarget != 6*120 {
		t.Fatalf("Mon–Sat at 120: week target %d, want 720", sc.WeekTarget)
	}
}

func TestReplayUsesTimezoneDays(t *testing.T) {
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Skip("tzdata unavailable:", err)
	}
	// Clocks go back on Sunday 25 Oct 2026, so that day lasts 25 hours.
	days, commitments := everyDay(day(10, 24)), []library.Commitment{fixed(day(10, 24), 60)}
	sessions := []library.Session{
		// 00:30–01:30 Sunday in Madrid is still Saturday in UTC.
		read(madrid, day(10, 25), 0, time.Hour),
		// 23:30 Sunday → 00:30 Monday: half on each day.
		session("item", time.Date(2026, 10, 25, 23, 30, 0, 0, madrid), time.Hour, nil, nil),
	}
	sc := library.ReplaySchedule(days, commitments, sessions, time.Monday, madrid, time.Date(2026, 10, 26, 0, 45, 0, 0, madrid))
	// Sat: 60 short. Sun: read 90 → pays 30. Owed 30.
	if sc.OwedAtMidnight != 30*time.Minute {
		t.Fatalf("owed %v, want 30m: days must follow Madrid, not UTC", sc.OwedAtMidnight)
	}
	if !sc.Today.Equal(day(10, 26)) || !sc.WeekStart.Equal(day(10, 26)) || sc.LoggedToday != 30*time.Minute {
		t.Fatalf("today %v, week from %v, logged %v", sc.Today, sc.WeekStart, sc.LoggedToday)
	}
	// Owed 30, today's target 60, 30 read: nothing extra paid yet.
	if sc.Owed != 30*time.Minute {
		t.Fatalf("live owed %v", sc.Owed)
	}
}

func TestReplayReviewWeekdayIsTheBoundary(t *testing.T) {
	loc := time.UTC
	// Weeks start Wednesday. Started Wed 2 Sep: checks on 9 Sep and 16 Sep.
	commitments := []library.Commitment{ramp(day(9, 2), 60, 30, 240)}
	sessions := readDaily(loc, day(9, 2), day(9, 20), 5*time.Hour)
	sc := library.ReplaySchedule(everyDay(day(9, 2)), commitments, sessions, time.Wednesday, loc, noon(loc, day(9, 15)))
	if sc.Ramp.Current != 90 || !sc.Ramp.NextCheck.Equal(day(9, 16)) || !sc.WeekStart.Equal(day(9, 9)) {
		t.Fatalf("Wednesday weeks: %+v from %v", sc.Ramp, sc.WeekStart)
	}
}

// Service-level: saving a plan through SQLite.

func planLibrary(t *testing.T) (*library.Service, *clock) {
	t.Helper()
	svc, clk := newTestLibrary(t)
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Timezone = "UTC"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	return svc, clk
}

func plan(t *testing.T, svc *library.Service) library.Schedule {
	t.Helper()
	view, err := svc.Plan(ctx)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return view.Schedule
}

func history(t *testing.T, svc *library.Service) (int, int) {
	t.Helper()
	out, err := svc.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return len(out.ActiveDays), len(out.Commitments)
}

func TestSavePlan(t *testing.T) {
	svc, clk := planLibrary(t) // Tuesday 15 Sep, 10:00 UTC

	if err := svc.SavePlan(ctx, weekdaysMonFri, fixed(time.Time{}, 60), false); err != nil {
		t.Fatalf("first plan: %v", err)
	}
	sc := plan(t, svc)
	if !sc.Planned() || sc.TargetToday != 60 || sc.Days != weekdaysMonFri || !sc.Commitment.EffectiveOn.Equal(day(9, 15)) {
		t.Fatalf("plan takes effect today: %+v", sc)
	}

	// Saving the same plan writes nothing.
	if err := svc.SavePlan(ctx, weekdaysMonFri, fixed(time.Time{}, 60), false); err != nil {
		t.Fatal(err)
	}
	if d, c := history(t, svc); d != 1 || c != 1 {
		t.Fatalf("unchanged save wrote rows: %d days, %d commitments", d, c)
	}

	// Raising today needs no confirmation; a same-day save replaces the row.
	if err := svc.SavePlan(ctx, weekdaysMonFri, fixed(time.Time{}, 90), false); err != nil {
		t.Fatal(err)
	}
	if d, c := history(t, svc); d != 1 || c != 1 || plan(t, svc).TargetToday != 90 {
		t.Fatalf("same-day save should replace: %d days, %d commitments", d, c)
	}

	// Next day, a days change alone writes only a days row.
	clk.Advance(24 * time.Hour)
	if err := svc.SavePlan(ctx, library.AllWeekdays, fixed(time.Time{}, 90), false); err != nil {
		t.Fatal(err)
	}
	if d, c := history(t, svc); d != 2 || c != 1 {
		t.Fatalf("days change: %d days, %d commitments; want 2, 1", d, c)
	}
}

func TestSavePlanLoweringTodayNeedsConfirmation(t *testing.T) {
	svc, _ := planLibrary(t) // Tuesday
	if err := svc.SavePlan(ctx, weekdaysMonFri, fixed(time.Time{}, 90), false); err != nil {
		t.Fatal(err)
	}

	var lower *library.LowerTargetError
	err := svc.SavePlan(ctx, weekdaysMonFri, ramp(time.Time{}, 30, 15, 120), false)
	if !errors.As(err, &lower) || lower.From != 90 || lower.To != 30 {
		t.Fatalf("lowering by a ramp: got %v, want LowerTargetError 90→30", err)
	}
	if plan(t, svc).TargetToday != 90 {
		t.Fatal("an unconfirmed lowering must write nothing")
	}

	// Taking Tuesday out of the days lowers today to zero.
	err = svc.SavePlan(ctx, library.WeekdaysOf(time.Monday), fixed(time.Time{}, 90), false)
	if !errors.As(err, &lower) || lower.To != 0 {
		t.Fatalf("removing today: got %v, want LowerTargetError to 0", err)
	}

	if err := svc.SavePlan(ctx, weekdaysMonFri, ramp(time.Time{}, 30, 15, 120), true); err != nil {
		t.Fatalf("confirmed: %v", err)
	}
	if sc := plan(t, svc); sc.Ramp == nil || sc.TargetToday != 30 {
		t.Fatalf("confirmed lowering saved: %+v", sc)
	}
}

func TestSavePlanValidation(t *testing.T) {
	svc, _ := planLibrary(t)
	cases := []struct {
		name  string
		days  library.Weekdays
		c     library.Commitment
		field string
	}{
		{"no days", 0, fixed(time.Time{}, 60), "days"},
		{"fixed zero", weekdaysMonFri, fixed(time.Time{}, 0), "minutes_per_day"},
		{"fixed over a day", weekdaysMonFri, fixed(time.Time{}, 1441), "minutes_per_day"},
		{"ramp no increment", weekdaysMonFri, ramp(time.Time{}, 60, 0, 120), "increment_minutes"},
		{"ramp ceiling at start", weekdaysMonFri, ramp(time.Time{}, 60, 30, 60), "ceiling_minutes"},
		{"unknown kind", weekdaysMonFri, library.Commitment{Kind: "campaign"}, "kind"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var verr *library.ValidationError
			if err := svc.SavePlan(ctx, c.days, c.c, true); !errors.As(err, &verr) || verr.Field != c.field {
				t.Fatalf("got %v, want ValidationError on %s", err, c.field)
			}
		})
	}
}

func TestSavePlanKeepsRunningRampAtCurrentValue(t *testing.T) {
	svc, clk := planLibrary(t) // Tuesday 15 Sep
	if err := svc.SavePlan(ctx, library.AllWeekdays, ramp(time.Time{}, 60, 30, 240), false); err != nil {
		t.Fatal(err)
	}
	clk.Advance(24 * time.Hour)
	// The form shows the running ramp at its current value: an unchanged save.
	if err := svc.SavePlan(ctx, library.AllWeekdays, ramp(time.Time{}, 60, 30, 240), false); err != nil {
		t.Fatal(err)
	}
	if _, c := history(t, svc); c != 1 {
		t.Fatalf("unchanged ramp wrote %d commitments", c)
	}
}
