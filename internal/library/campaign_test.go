package library_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Buenos Aires is UTC−3 all year: no daylight saving to blur a boundary.
var buenosAires = mustLoad("America/Argentina/Buenos_Aires")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func campaignSettings() library.Settings {
	return library.Settings{ReviewWeekday: time.Sunday, PaceWindowDays: 90, ProjectionWindowWeeks: 4,
		SeedPaceMedium: 30, FallbackBookPages: 300, WordsPerPage: 300}
}

// book is a book in pages; edit may adjust it.
func book(id string, state library.State, pages int, edit ...func(*library.Item)) library.Item {
	it := library.Item{ID: id, Format: library.FormatBook, FocusDemand: library.FocusMedium,
		SizeUnit: library.UnitPages, SizeValue: ptr(pages), State: state}
	for _, e := range edit {
		e(&it)
	}
	return it
}

func finishedAt(t time.Time) func(*library.Item) {
	return func(it *library.Item) { it.FinishedAt = &t }
}

func byID(items ...library.Item) map[string]library.Item {
	out := map[string]library.Item{}
	for _, it := range items {
		out[it.ID] = it
	}
	return out
}

func TestCampaignCountsBooksFinishedInItsDays(t *testing.T) {
	loc := buenosAires
	c := library.Campaign{TargetCount: 100, StartedOn: day(9, 1), Deadline: day(9, 30)}
	utc := func(m time.Month, d, hour, minute int) time.Time {
		return time.Date(2026, m, d, hour, minute, 0, 0, time.UTC)
	}
	items := byID(
		book("eve", library.StateFinished, 300, finishedAt(utc(9, 1, 2, 0))),     // 31 Aug, 23:00 local
		book("first", library.StateFinished, 300, finishedAt(utc(9, 1, 4, 0))),   // 1 Sep, 01:00 local
		book("last", library.StateFinished, 300, finishedAt(utc(10, 1, 2, 30))),  // 30 Sep, 23:30 local
		book("late", library.StateFinished, 300, finishedAt(utc(10, 1, 4, 0))),   // 1 Oct, 01:00 local
		book("ref", library.StateReference, 300, finishedAt(utc(9, 10, 12, 0))),  // reference never counts
		book("quit", library.StateAbandoned, 300, finishedAt(utc(9, 10, 12, 0))), // nor does abandoned
		book("essay", library.StateFinished, 300, finishedAt(utc(9, 10, 12, 0)), func(it *library.Item) {
			it.Format = library.FormatArticle
		}),
	)

	// The deadline day closes at midnight local: 1 Oct 03:00 UTC.
	open := library.MeasureCampaign(c, items, nil, campaignSettings(), loc, utc(10, 1, 2, 59))
	if open.Over || open.Finished != 2 {
		t.Fatalf("a minute before the deadline closes: over %v, finished %d; want open with 2", open.Over, open.Finished)
	}
	closed := library.MeasureCampaign(c, items, nil, campaignSettings(), loc, utc(10, 1, 3, 0))
	if !closed.Over || closed.Finished != 2 || closed.WeeksLeft != 0 || closed.Projection != nil {
		t.Fatalf("at the close: %+v; want over, final count 2, no projection", closed)
	}

	ended := day(9, 15)
	c.EndedOn = &ended
	early := library.MeasureCampaign(c, items, nil, campaignSettings(), loc, utc(9, 20, 12, 0))
	if !early.Over || early.Finished != 1 {
		t.Fatalf("ended on 15 Sep: over %v, finished %d; want over with 1", early.Over, early.Finished)
	}
}

func TestCampaignRequired(t *testing.T) {
	loc := time.UTC
	now := day(9, 13) // Sunday midnight; the deadline day closes 70 days later
	c := library.Campaign{TargetCount: 12, StartedOn: day(9, 1), Deadline: day(11, 21)}
	paper := library.Item{ID: "paper", Format: library.FormatPaper, FocusDemand: library.FocusDeep, SizeUnit: library.UnitPages}
	deep := book("deep", library.StateFinished, 1000, finishedAt(day(9, 5)), func(it *library.Item) { it.FocusDemand = library.FocusDeep })
	items := byID(
		book("medium", library.StateInProgress, 200),
		book("next", library.StatePool, 400),
		book("long", library.StatePool, 0, func(it *library.Item) { it.SizeUnit = library.UnitWords; it.SizeValue = ptr(90000) }),
		book("unsized", library.StatePool, 0, func(it *library.Item) { it.SizeValue = nil }),
		deep,
		book("done", library.StateFinished, 250, finishedAt(day(9, 8))),
		paper,
	)
	sessions := []library.Session{
		at(items["medium"], day(8, 20), 3*time.Hour, 30), // 90 pages
		at(deep, day(8, 22), time.Hour, 10),              // 10 pages
		at(paper, day(8, 25), 5*time.Hour, 100),          // not a book
		at(items["medium"], day(5, 1), 9*time.Hour, 90),  // outside the pace window
	}

	cs := library.MeasureCampaign(c, items, sessions, campaignSettings(), loc, now)
	r := cs.Required
	if cs.Over || cs.Finished != 2 || cs.WeeksLeft != 10 {
		t.Fatalf("finished %d, weeks left %v; want 2 and 10", cs.Finished, cs.WeeksLeft)
	}
	if r.BooksLeft != 10 || r.AvgPages != 300 || r.PagesBasis != library.PagesWaiting {
		t.Fatalf("books left %d, avg %v (%s); want 10 and 300 from waiting books", r.BooksLeft, r.AvgPages, r.PagesBasis)
	}
	if r.Pace.Provisional || !near(r.Pace.PagesPerHour, 25) || r.Pace.Measured != 4*time.Hour {
		t.Fatalf("pace %+v; want 25 pages/h pooled over 4 h", r.Pace)
	}
	if len(r.Pace.Mix) != 2 || r.Pace.Mix[0].FocusDemand != library.FocusMedium || !near(r.Pace.Mix[0].Share, 0.75) {
		t.Fatalf("pace mix %+v; want medium 75%%, deep 25%%", r.Pace.Mix)
	}
	if !near(r.HoursLeft, 120) || !near(r.WeeklyHours, 12) {
		t.Fatalf("hours left %v, weekly %v; want 120 and 12", r.HoursLeft, r.WeeklyHours)
	}

	if m, ok := cs.MatchPerDay(weekdaysMonFri); !ok || m != 144 {
		t.Fatalf("match over five days: %d %v; want 144", m, ok)
	}
	if m, ok := cs.MatchPerDay(library.AllWeekdays); !ok || m != 103 {
		t.Fatalf("match over seven days: %d %v; want 103, rounded up", m, ok)
	}
	if _, ok := cs.MatchPerDay(0); ok {
		t.Fatal("no active days has nothing to match")
	}
	c.TargetCount = 2
	if _, ok := library.MeasureCampaign(c, items, sessions, campaignSettings(), loc, now).MatchPerDay(weekdaysMonFri); ok {
		t.Fatal("a reached campaign has nothing to match")
	}
	c.TargetCount = 1000
	if _, ok := library.MeasureCampaign(c, items, sessions, campaignSettings(), loc, now).MatchPerDay(library.WeekdaysOf(time.Monday)); ok {
		t.Fatal("more than a day's minutes cannot be matched")
	}
}

func TestCampaignFallbacks(t *testing.T) {
	loc := time.UTC
	now := day(9, 13)
	c := library.Campaign{TargetCount: 10, StartedOn: day(9, 1), Deadline: day(11, 21)}
	finished := book("done", library.StateFinished, 500, finishedAt(day(9, 8)))
	thin := []library.Session{at(finished, day(9, 1), 119*time.Minute, 60)}

	cs := library.MeasureCampaign(c, byID(finished), thin, campaignSettings(), loc, now)
	if r := cs.Required; r.AvgPages != 500 || r.PagesBasis != library.PagesFinished {
		t.Fatalf("avg %v (%s); want 500 from finished books", r.AvgPages, r.PagesBasis)
	}
	if p := cs.Required.Pace; !p.Provisional || p.PagesPerHour != 30 || p.Mix != nil {
		t.Fatalf("119 min of books: pace %+v; want the medium seed, provisional", p)
	}

	cs = library.MeasureCampaign(c, nil, nil, campaignSettings(), loc, now)
	if r := cs.Required; r.AvgPages != 300 || r.PagesBasis != library.PagesSetting {
		t.Fatalf("empty library: avg %v (%s); want the setting's 300", r.AvgPages, r.PagesBasis)
	}
}

// Before two hours are measured, the pace comes from the seeds for the focus
// of the books waiting, weighted by their pages: deep books are not assumed
// to go at the medium seed.
func TestProvisionalBookPaceFollowsTheBooksWaiting(t *testing.T) {
	st := campaignSettings()
	st.SeedPaceLight, st.SeedPaceDeep = 40, 15
	c := library.Campaign{TargetCount: 10, StartedOn: day(9, 1), Deadline: day(11, 21)}
	focus := func(f library.FocusDemand) func(*library.Item) { return func(it *library.Item) { it.FocusDemand = f } }
	items := byID(
		book("deep", library.StatePool, 300, focus(library.FocusDeep)),
		book("light", library.StateInProgress, 300, focus(library.FocusLight)),
		book("done", library.StateFinished, 900, finishedAt(day(9, 8))), // not waiting
	)

	p := library.MeasureCampaign(c, items, nil, st, time.UTC, day(9, 13)).Required.Pace
	// 300 pages at 15/h is 20 h and 300 at 40/h is 7 h 30: 600 pages in 27.5 h.
	if want := 600 / 27.5; !p.Provisional || math.Abs(p.PagesPerHour-want) > 1e-9 {
		t.Fatalf("pace %+v; want %.3f pages/h, provisional", p, want)
	}
}

func TestCampaignProjection(t *testing.T) {
	loc := buenosAires
	st := campaignSettings()
	st.ReviewWeekday = time.Monday
	// Wednesday 30 Sep, midnight local. Weeks start Mondays: 31 Aug, 7, 14,
	// 21, 28 Sep. The deadline day closes 70 days later, on 9 Dec.
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, loc)
	c := library.Campaign{TargetCount: 100, StartedOn: day(9, 1), Deadline: day(12, 8)}
	items := byID(
		book("book", library.StateInProgress, 0, func(it *library.Item) { it.SizeValue = nil }),
		library.Item{ID: "item", Format: library.FormatArticle},
		library.Item{ID: "video", Format: library.FormatVideo},
	)
	local := func(m time.Month, d, hour int) time.Time { return time.Date(2026, m, d, hour, 0, 0, 0, loc) }
	reading := func(id string, start time.Time, length time.Duration) library.Session {
		return session(id, start, length, nil, nil)
	}
	sessions := []library.Session{
		reading("book", local(9, 10, 20), 2*time.Hour), // first ever: week of 7 Sep counts
		reading("book", local(9, 15, 20), 4*time.Hour), // week of 14 Sep
		reading("item", local(9, 16, 20), 2*time.Hour), // week of 14 Sep, not a book
		reading("video", local(9, 22, 20), time.Hour),  // week of 21 Sep
		reading("book", local(9, 27, 23), 2*time.Hour), // Sunday 23:00: one hour each side of the boundary
		reading("book", local(9, 29, 20), 5*time.Hour), // this week: not closed
	}

	cs := library.MeasureCampaign(c, items, sessions, st, loc, now)
	p := cs.Projection
	if p == nil || p.Weeks != 3 {
		t.Fatalf("projection %+v; want 3 closed weeks since the first session", p)
	}
	if !near(p.WeeklyBookHours, 7.0/3) || !near(p.BookShare, 0.7) {
		t.Fatalf("weekly book hours %v, share %v; want 7/3 and 0.7", p.WeeklyBookHours, p.BookShare)
	}
	if !near(cs.WeeksLeft, 10) {
		t.Fatalf("weeks left %v, want 10", cs.WeeksLeft)
	}
	// 10 weeks × 7/3 h × 30 pages/h ÷ 300 pages = 2.33 books.
	if p.Books != 2 {
		t.Fatalf("projected %d books, want 2", p.Books)
	}
	st.ProjectionWindowWeeks = 2
	if p := library.MeasureCampaign(c, items, sessions, st, loc, now).Projection; p.Weeks != 2 || !near(p.WeeklyBookHours, 2.5) || !near(p.BookShare, 0.625) {
		t.Fatalf("a two-week window: %+v; want 2 weeks, 2.5 book hours, share 0.625", p)
	}

	fresh := library.MeasureCampaign(c, items, sessions[len(sessions)-1:], st, loc, now)
	if fresh.Projection != nil {
		t.Fatalf("with reading only this week there is no closed week: %+v", fresh.Projection)
	}
}

func TestCommittedWeek(t *testing.T) {
	loc := time.UTC
	days := []library.ActiveDays{{EffectiveOn: day(9, 6), Days: weekdaysMonFri}}
	sc := library.ReplaySchedule(days, []library.Commitment{ramp(day(9, 6), 60, 30, 240)}, nil, time.Sunday, loc, noon(loc, day(9, 16)))
	if got := sc.CommittedWeek(); got != 300 {
		t.Fatalf("ramp at 60 over five days: %d, want 300", got)
	}
}

func TestStartCampaign(t *testing.T) {
	svc, clk := newTestLibrary(t)
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Timezone = "America/Argentina/Buenos_Aires"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	clk.now = time.Date(2026, 9, 15, 2, 0, 0, 0, time.UTC) // still 14 Sep locally

	var verr *library.ValidationError
	for _, tc := range []struct {
		name            string
		target          int
		start, deadline time.Time
		field           string
	}{
		{"no books", 0, day(9, 1), day(12, 31), "target_count"},
		{"starts tomorrow", 10, day(9, 15), day(12, 31), "started_on"},
		{"deadline today", 10, day(9, 1), day(9, 14), "deadline"},
		{"deadline before start", 10, day(9, 14), day(9, 13), "deadline"},
	} {
		if _, err := svc.StartCampaign(ctx, "", tc.target, tc.start, tc.deadline); !errors.As(err, &verr) || verr.Field != tc.field {
			t.Fatalf("%s: got %v, want ValidationError on %s", tc.name, err, tc.field)
		}
	}

	c, err := svc.StartCampaign(ctx, "  ", 100, day(9, 14), time.Date(2027, 3, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "100 books by 22 Mar 2027" {
		t.Fatalf("generated name %q", c.Name)
	}
	if _, err := svc.StartCampaign(ctx, "another", 5, day(9, 14), day(12, 31)); !errors.Is(err, library.ErrCampaignActive) {
		t.Fatalf("second active campaign: got %v, want ErrCampaignActive", err)
	}

	if err := svc.RenameCampaign(ctx, c.ID, " A hundred "); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Plan(ctx)
	if err != nil || view.Campaign == nil || view.Campaign.Campaign.Name != "A hundred" || view.Campaign.Over {
		t.Fatalf("plan campaign: %+v %v", view.Campaign, err)
	}

	if err := svc.EndCampaign(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	clk.Advance(48 * time.Hour)
	if err := svc.EndCampaign(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	view, err = svc.Plan(ctx)
	if err != nil || !view.Campaign.Over || view.Campaign.Campaign.EndedOn == nil || !view.Campaign.Campaign.EndedOn.Equal(day(9, 14)) {
		t.Fatalf("ended campaign: %+v %v; want ended on 14 Sep, a second end changing nothing", view.Campaign, err)
	}

	next, err := svc.StartCampaign(ctx, "", 20, day(9, 16), day(12, 31))
	if err != nil {
		t.Fatalf("a new campaign after ending: %v", err)
	}
	if view, err = svc.Plan(ctx); err != nil || view.Campaign.Campaign.ID != next.ID {
		t.Fatalf("plan should show the active campaign: %+v %v", view.Campaign, err)
	}
	if err := svc.RenameCampaign(ctx, "missing", "x"); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("rename missing: got %v, want ErrNotFound", err)
	}
}

func TestPlanCountsFinishedBooks(t *testing.T) {
	svc, _ := newTestLibrary(t)
	if view, err := svc.Plan(ctx); err != nil || view.Campaign != nil {
		t.Fatalf("before any campaign: %+v %v", view.Campaign, err)
	}
	if _, err := svc.StartCampaign(ctx, "", 3, day(9, 1), day(12, 31)); err != nil {
		t.Fatal(err)
	}
	shelf := newShelf(t, svc, "Novels")
	for _, title := range []string{"One", "Two"} {
		it := newItem(t, svc, shelf.ID, title, func(it *library.Item) { it.SizeValue = ptr(320) })
		startItem(t, svc, it.ID)
		if title == "One" {
			if _, err := svc.Finish(ctx, it.ID, "", nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	view, err := svc.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cs := view.Campaign
	if cs.Finished != 1 || cs.Required.BooksLeft != 2 || cs.Required.AvgPages != 320 || math.IsInf(cs.Required.WeeklyHours, 0) {
		t.Fatalf("campaign %+v", cs)
	}
}
