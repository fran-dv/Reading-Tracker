package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// utcLibrary is a test library whose settings read days in UTC.
func utcLibrary(t *testing.T) (*library.Service, *clock) {
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

func sep(d, h int) time.Time { return time.Date(2026, 9, d, h, 0, 0, 0, time.UTC) }

func byKind(all []library.Achievement, kind library.AchievementKind) []library.Achievement {
	var out []library.Achievement
	for _, a := range all {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

// A campaign of four: every book finished is an achievement with its count,
// the second is halfway with how it stands against an even pace, and the
// fourth meets it, with the books, pages and time it took.
func TestCampaignAchievements(t *testing.T) {
	svc, clk := utcLibrary(t)
	shelf := newShelf(t, svc, "S")
	clk.now = sep(1, 8)
	if _, err := svc.StartCampaign(ctx, "", 4, sep(1, 0), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for i, day := range []int{5, 10, 12, 20} {
		clk.now = sep(day, 8)
		book := startItem(t, svc, newItem(t, svc, shelf.ID, "Book "+string(rune('A'+i)), func(it *library.Item) { it.SizeValue = ptr(100) }).ID)
		if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(day, 6), sep(day, 8), ptr(100), ""); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Finish(ctx, book.ID, "", nil); err != nil {
			t.Fatal(err)
		}
	}
	clk.now = sep(21, 9)

	all, err := svc.Achievements(ctx)
	if err != nil {
		t.Fatal(err)
	}
	books := byKind(all, library.BookFinished)
	if len(books) != 4 || books[0].Count != 4 || books[3].Count != 1 || books[0].Pages != 100 || books[0].Time != 2*time.Hour || books[0].Days != 1 {
		t.Fatalf("books %+v", books)
	}
	half := byKind(all, library.CampaignHalfway)
	// Day 10 of 30: an even pace had 1.33 books, rounded to 1; 2 are done.
	if len(half) != 1 || half[0].Count != 2 || !half[0].On.Equal(sep(10, 0)) || half[0].Ahead != 1 || !half[0].Big() {
		t.Fatalf("halfway %+v", half)
	}
	met := byKind(all, library.CampaignMet)
	if len(met) != 1 || !met[0].On.Equal(sep(20, 0)) || met[0].Pages != 400 || met[0].Time != 8*time.Hour ||
		met[0].Days != 20 || met[0].First.Title != "Book A" || met[0].Last.Title != "Book D" {
		t.Fatalf("met %+v", met)
	}
	if !all[0].On.Equal(sep(20, 0)) {
		t.Fatalf("newest first: %v", all[0].On)
	}

	// Home shows the newest big one until it is closed, then the next.
	view, err := svc.Home(ctx, library.Moment{})
	if err != nil || view.Moment == nil || view.Moment.Kind != library.CampaignMet {
		t.Fatalf("moment %+v, %v", view.Moment, err)
	}
	if err := svc.CloseMoment(ctx, view.Moment.Key); err != nil {
		t.Fatal(err)
	}
	if view, _ = svc.Home(ctx, library.Moment{}); view.Moment == nil || view.Moment.Kind != library.CampaignHalfway {
		t.Fatalf("after closing: %+v", view.Moment)
	}
	// Thirty days on, an unseen moment has waited long enough.
	clk.now = sep(10, 9).AddDate(0, 0, library.MomentDays+1)
	if view, _ = svc.Home(ctx, library.Moment{}); view.Moment != nil {
		t.Fatalf("stale moment still shown: %+v", view.Moment)
	}
}

// An hours ramp rises week by week; the rise that reaches its ceiling is
// its top, with where and when the journey began and how often it held.
func TestHoursRampAchievements(t *testing.T) {
	svc, clk := utcLibrary(t)
	item := inProgressItem(t, svc)
	clk.now = sep(6, 6) // Sunday: weeks start on Sunday
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitRamp, StartMinutes: 30, IncrementMinutes: 15, CeilingMinutes: 60}, false); err != nil {
		t.Fatal(err)
	}
	clk.now = sep(27, 12)
	for d := 6; d <= 19; d++ {
		if d == 9 {
			continue // a missed day in the first week: owed at the check on the 13th
		}
		if _, err := svc.AddRetroactiveSession(ctx, item.ID, sep(d, 7), sep(d, 9), nil, ""); err != nil {
			t.Fatal(err)
		}
	}

	all, err := svc.Achievements(ctx)
	if err != nil {
		t.Fatal(err)
	}
	steps, tops := byKind(all, library.HoursRampStep), byKind(all, library.HoursRampTop)
	// Owed 30 min on the 9th was paid by the 10th's two hours, so the 13th rises.
	if len(steps) != 1 || !steps[0].On.Equal(sep(13, 0)) || steps[0].From != 30 || steps[0].To != 45 {
		t.Fatalf("steps %+v", steps)
	}
	if len(tops) != 1 || !tops[0].On.Equal(sep(20, 0)) || tops[0].To != 60 || !tops[0].Began.Equal(sep(6, 0)) || tops[0].Start != 30 || tops[0].Holds != 0 {
		t.Fatalf("tops %+v", tops)
	}
}

// A speed ramp whose first week beats its target by enough reaches its top
// at that check.
func TestSpeedRampAchievements(t *testing.T) {
	svc, clk := utcLibrary(t)
	shelf := newShelf(t, svc, "S")
	book := startItem(t, svc, newItem(t, svc, shelf.ID, "Book").ID)
	// A baseline of 30 pages an hour over three hours before the ramp.
	for i, d := range []int{1, 2, 3} {
		if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(d, 7), sep(d, 8), ptr(30*(i+1)), ""); err != nil {
			t.Fatal(err)
		}
	}
	clk.now = sep(6, 6) // Sunday
	if err := svc.StartSpeedRamp(ctx, 10, 110); err != nil {
		t.Fatal(err)
	}
	clk.now = sep(14, 12)
	// The week from the 6th reads 36 pages an hour: 120% of the baseline.
	for i, d := range []int{7, 8, 9} {
		if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(d, 7), sep(d, 8), ptr(90+36*(i+1)), ""); err != nil {
			t.Fatal(err)
		}
	}

	all, err := svc.Achievements(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tops := byKind(all, library.SpeedRampTop)
	if len(tops) != 1 || !tops[0].On.Equal(sep(13, 0)) || tops[0].From != 100 || tops[0].To != 110 || !tops[0].Began.Equal(sep(6, 0)) {
		t.Fatalf("tops %+v (all %+v)", tops, all)
	}
}
