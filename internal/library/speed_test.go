package library_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Four kinds of material. Weeks start on Sunday: 6, 13, 20, 27 Sep 2026.
var (
	speedBook    = library.Item{ID: "book", Format: library.FormatBook, FocusDemand: library.FocusMedium, SizeUnit: library.UnitPages}
	speedArticle = library.Item{ID: "article", Format: library.FormatArticle, FocusDemand: library.FocusLight, SizeUnit: library.UnitWords}
	speedPaper   = library.Item{ID: "paper", Format: library.FormatPaper, FocusDemand: library.FocusDeep, SizeUnit: library.UnitPages}
	speedVideo   = library.Item{ID: "video", Format: library.FormatVideo, FocusDemand: library.FocusLight, SizeUnit: library.UnitMinutes}
	speedItems   = map[string]library.Item{"book": speedBook, "article": speedArticle, "paper": speedPaper, "video": speedVideo}
)

// at reads perHour units of an item for length, at noon UTC on a day.
func at(item library.Item, on time.Time, length time.Duration, perHour float64) library.Session {
	units := int(math.Round(perHour * length.Hours()))
	return session(item.ID, on.Add(12*time.Hour), length, ptr(0), ptr(units))
}

func speedSettings() library.Settings {
	return library.Settings{ReviewWeekday: time.Sunday, PaceWindowDays: 90, WordsPerPage: 300}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestReplaySpeedLastWeek(t *testing.T) {
	sessions := []library.Session{
		at(speedBook, day(9, 14), 2*time.Hour, 30),       // 60 pages
		at(speedArticle, day(9, 15), time.Hour, 9000),    // 30 pages' worth
		at(speedVideo, day(9, 16), 3*time.Hour, 60),      // minutes: no speed
		session("book", day(9, 17), time.Hour, nil, nil), // no positions
		at(speedBook, day(9, 21), time.Hour, 90),         // this week: not last week
	}
	sp := library.ReplaySpeed(speedItems, sessions, nil, speedSettings(), time.UTC, day(9, 22).Add(10*time.Hour))
	w := sp.LastWeek
	if !w.From.Equal(day(9, 13)) || !w.To.Equal(day(9, 20)) {
		t.Fatalf("last week %v–%v, want 13–20 Sep", w.From, w.To)
	}
	if w.Measured != 3*time.Hour || !w.Enough() || !near(w.PagesPerHour, 30) {
		t.Fatalf("measured %v, %v pages/h; want 3h at 30", w.Measured, w.PagesPerHour)
	}
	if len(w.Mix) != 2 || w.Mix[0].Format != library.FormatBook || !near(w.Mix[0].Share, 2.0/3) || !near(w.Mix[1].Share, 1.0/3) {
		t.Fatalf("mix %+v", w.Mix)
	}
	if sp.Ramp != nil {
		t.Fatal("no ramp was started")
	}

	thin := library.ReplaySpeed(speedItems, []library.Session{at(speedBook, day(9, 14), 100*time.Minute, 30)}, nil, speedSettings(), time.UTC, day(9, 22))
	if thin.LastWeek.Enough() {
		t.Fatal("100 minutes is not enough to show a speed")
	}
}

// baselineSessions set book at 20 pages/h and articles at 250 words/min in August.
func baselineSessions() []library.Session {
	return []library.Session{
		at(speedBook, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), 5*time.Hour, 20),
		at(speedArticle, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), 2*time.Hour, 15000),
	}
}

func speedRamp(on time.Time, increment, ceiling int) library.SpeedRamp {
	return library.SpeedRamp{StartedOn: on, IncrementPercent: increment, CeilingPercent: ceiling}
}

func TestSpeedIndexIgnoresTheMix(t *testing.T) {
	ramps := []library.SpeedRamp{speedRamp(day(9, 6), 5, 130)}
	// Same speeds as the baselines, two very different mixes.
	bookHeavy := append(baselineSessions(), at(speedBook, day(9, 7), 4*time.Hour, 20), at(speedArticle, day(9, 8), time.Hour, 15000))
	articleHeavy := append(baselineSessions(), at(speedBook, day(9, 7), time.Hour, 20), at(speedArticle, day(9, 8), 4*time.Hour, 15000))
	for name, sessions := range map[string][]library.Session{"book heavy": bookHeavy, "article heavy": articleHeavy} {
		sp := library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, day(9, 14))
		if x := sp.Ramp.LastWeek; x == nil || !near(x.Index, 1) || x.Measured != 5*time.Hour {
			t.Fatalf("%s: index %+v, want exactly 1.0 over 5h", name, x)
		}
	}
}

func TestSpeedRampAdvances(t *testing.T) {
	ramps := []library.SpeedRamp{speedRamp(day(9, 6), 5, 130)}
	sessions := append(baselineSessions(),
		at(speedBook, day(9, 7), time.Hour, 22),         // 110%
		at(speedArticle, day(9, 8), 4*time.Hour, 15000), // 100%
		at(speedVideo, day(9, 9), 5*time.Hour, 60),      // no speed, no weight
	)
	sp := library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, day(9, 14))
	r := sp.Ramp
	if len(r.Baselines) != 2 || !near(r.Baselines[0].PerHour, 20) || !near(r.Baselines[1].PerHour, 15000) {
		t.Fatalf("baselines %+v", r.Baselines)
	}
	if !near(r.LastWeek.Index, 1.02) {
		t.Fatalf("index %v, want (1×1.10 + 4×1.00) ÷ 5 = 1.02", r.LastWeek.Index)
	}
	if !r.Running || r.Target != 105 || !r.LastCheck.Advanced || !r.NextCheck.Equal(day(9, 20)) {
		t.Fatalf("met at the check: %+v %+v", r, r.LastCheck)
	}
}

func TestSpeedRampHolds(t *testing.T) {
	ramps := []library.SpeedRamp{speedRamp(day(9, 6), 5, 130)}
	cases := []struct {
		name     string
		sessions []library.Session
		enough   bool
	}{
		{"slower than target", []library.Session{at(speedBook, day(9, 7), 3*time.Hour, 19)}, true},
		{"too little evidence", []library.Session{at(speedBook, day(9, 7), 90*time.Minute, 40)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sp := library.ReplaySpeed(speedItems, append(baselineSessions(), c.sessions...), ramps, speedSettings(), time.UTC, day(9, 14))
			r := sp.Ramp
			if r.Target != 100 || r.LastCheck == nil || r.LastCheck.Advanced || r.LastCheck.Enough != c.enough {
				t.Fatalf("should hold: target %d, check %+v", r.Target, r.LastCheck)
			}
		})
	}
}

func TestSpeedRampWaitsForAFullWeek(t *testing.T) {
	ramps := []library.SpeedRamp{speedRamp(day(9, 10), 5, 130)} // Thursday
	var sessions []library.Session
	for d := day(9, 10); d.Before(day(9, 27)); d = d.AddDate(0, 0, 1) {
		sessions = append(sessions, at(speedBook, d, time.Hour, 30))
	}
	sessions = append(sessions, baselineSessions()...)

	first := library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, day(9, 13))
	if first.Ramp.Target != 100 || first.Ramp.LastCheck != nil || !first.Ramp.NextCheck.Equal(day(9, 20)) {
		t.Fatalf("first Sunday must not check: %+v", first.Ramp)
	}
	second := library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, day(9, 20))
	if second.Ramp.Target != 105 {
		t.Fatalf("second Sunday checks and advances: %+v", second.Ramp)
	}
}

func TestSpeedRampNewBandSetsItsBaseline(t *testing.T) {
	ramps := []library.SpeedRamp{speedRamp(day(9, 6), 5, 130)}
	sessions := append(baselineSessions(),
		at(speedBook, day(9, 7), 3*time.Hour, 20),
		at(speedPaper, day(9, 8), 2*time.Hour, 10), // first paper: baseline 10
		at(speedBook, day(9, 14), 3*time.Hour, 20),
		at(speedPaper, day(9, 15), time.Hour, 12), // 120% of its own baseline
	)
	sp := library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, day(9, 14))
	week1 := sp.Ramp.LastWeek
	if !near(week1.Index, 1) || week1.Measured != 3*time.Hour || len(week1.Rows) != 2 || week1.Rows[1].Counted {
		t.Fatalf("paper's first week sets a baseline and does not count: %+v", week1)
	}

	sp = library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, day(9, 21))
	week2 := sp.Ramp.LastWeek
	if !near(week2.Index, (3*1.0+1*1.2)/4) || week2.Measured != 4*time.Hour {
		t.Fatalf("paper counts from its second week: %+v", week2)
	}
}

func TestSpeedRampCeilingAndStop(t *testing.T) {
	var sessions []library.Session
	for d := day(9, 6); d.Before(day(10, 4)); d = d.AddDate(0, 0, 7) {
		sessions = append(sessions, at(speedBook, d.AddDate(0, 0, 1), 3*time.Hour, 40)) // 200% every week
	}
	sessions = append(sessions, baselineSessions()...)

	reached := library.ReplaySpeed(speedItems, sessions, []library.SpeedRamp{speedRamp(day(9, 6), 10, 120)}, speedSettings(), time.UTC, day(9, 30))
	if r := reached.Ramp; r.Running || r.Target != 120 || !r.ReachedOn.Equal(day(9, 20)) || !r.NextCheck.IsZero() {
		t.Fatalf("100 → 110 → 120 reached on 20 Sep: %+v", r)
	}

	stoppedOn := day(9, 16)
	stopped := speedRamp(day(9, 6), 10, 200)
	stopped.StoppedOn = &stoppedOn
	sp := library.ReplaySpeed(speedItems, sessions, []library.SpeedRamp{stopped}, speedSettings(), time.UTC, day(9, 30))
	if r := sp.Ramp; r.Running || r.Target != 110 || !r.NextCheck.IsZero() {
		t.Fatalf("no checks after a stop: %+v", r)
	}
}

func TestStartAndStopSpeedRamp(t *testing.T) {
	svc, clk := planLibrary(t) // Tuesday 15 Sep, UTC

	if err := svc.StartSpeedRamp(ctx, 5, 130); !errors.Is(err, library.ErrNoBaseline) {
		t.Fatalf("no reading yet: got %v, want ErrNoBaseline", err)
	}
	var verr *library.ValidationError
	if err := svc.StartSpeedRamp(ctx, 0, 130); !errors.As(err, &verr) || verr.Field != "increment_percent" {
		t.Fatalf("zero increment: %v", err)
	}
	if err := svc.StartSpeedRamp(ctx, 5, 100); !errors.As(err, &verr) || verr.Field != "ceiling_percent" {
		t.Fatalf("ceiling at 100: %v", err)
	}

	shelf := newShelf(t, svc, "S")
	book := startItem(t, svc, newItem(t, svc, shelf.ID, "book", func(it *library.Item) { it.SizeValue = ptr(300) }).ID)
	if _, err := svc.AddRetroactiveSession(ctx, book.ID, clk.Now().Add(-26*time.Hour), clk.Now().Add(-25*time.Hour), ptr(30), ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartSpeedRamp(ctx, 5, 130); err != nil {
		t.Fatalf("with a measured speed: %v", err)
	}
	view, err := svc.Plan(ctx)
	if err != nil || view.Speed.Ramp == nil || !view.Speed.Ramp.Running || len(view.Speed.Ramp.Baselines) != 1 {
		t.Fatalf("ramp not running: %v %+v", err, view.Speed.Ramp)
	}

	clk.Advance(24 * time.Hour)
	if err := svc.StopSpeedRamp(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.StopSpeedRamp(ctx); err != nil {
		t.Fatalf("a second stop is harmless: %v", err)
	}
	view, _ = svc.Plan(ctx)
	if view.Speed.Ramp.Running || !view.Speed.Ramp.Ramp.StoppedOn.Equal(day(9, 16)) {
		t.Fatalf("stopped today: %+v", view.Speed.Ramp)
	}
	out, err := svc.Export(ctx)
	if err != nil || len(out.SpeedRamps) != 1 || out.Settings.WordsPerPage != 300 {
		t.Fatalf("export: %v %+v", err, out.SpeedRamps)
	}
}

func TestSpeedRampChecksAndThisWeek(t *testing.T) {
	ramps := []library.SpeedRamp{speedRamp(day(9, 6), 5, 130)}
	sessions := append(baselineSessions(),
		at(speedBook, day(9, 7), 3*time.Hour, 20),  // week 1: 100%, target 100 → rose
		at(speedBook, day(9, 14), 1*time.Hour, 21), // this week so far: 105%
		at(speedPaper, day(9, 15), 1*time.Hour, 9), // new this week: not counted, no baseline set
	)
	now := day(9, 16).Add(18 * time.Hour)
	sp := library.ReplaySpeed(speedItems, sessions, ramps, speedSettings(), time.UTC, now)
	r := sp.Ramp
	if len(r.Checks) != 1 || r.Checks[0].Target != 100 || r.Checks[0].Measured != 3*time.Hour || !r.Checks[0].Advanced || r.LastCheck != &r.Checks[0] {
		t.Fatalf("checks %+v", r.Checks)
	}
	x := r.ThisWeek
	if x == nil || !near(x.Index, 1.05) || x.Measured != time.Hour || x.Enough() {
		t.Fatalf("this week so far %+v", x)
	}
	for _, b := range r.Baselines {
		if b.Band == speedPaper.Band() {
			t.Fatal("a partial week must not set a baseline")
		}
	}
}
