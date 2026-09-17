package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// The record keeps every goal, met or not, adds up the years, and names
// the bests as plain facts.
func TestRecord(t *testing.T) {
	svc, clk := utcLibrary(t)
	shelf := newShelf(t, svc, "S")

	// A campaign of two that is met, then one of five that is ended short.
	clk.now = sep(1, 8)
	met, err := svc.StartCampaign(ctx, "", 2, sep(1, 0), sep(30, 0))
	if err != nil {
		t.Fatal(err)
	}
	finish := func(title string, pages int, day int, hours time.Duration) {
		t.Helper()
		clk.now = sep(day, 22)
		book := startItem(t, svc, newItem(t, svc, shelf.ID, title, func(it *library.Item) { it.SizeValue = ptr(pages) }).ID)
		if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(day, 20).Add(-hours), sep(day, 20), ptr(pages), ""); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Finish(ctx, book.ID, "", nil); err != nil {
			t.Fatal(err)
		}
	}
	finish("Short", 150, 3, time.Hour)
	finish("Long", 600, 8, 5*time.Hour)
	if err := svc.EndCampaign(ctx, met.ID); err != nil {
		t.Fatal(err)
	}
	clk.now = sep(9, 8)
	short, err := svc.StartCampaign(ctx, "", 5, sep(9, 0), sep(20, 0))
	if err != nil {
		t.Fatal(err)
	}
	finish("Middling", 300, 12, 2*time.Hour)
	if err := svc.EndCampaign(ctx, short.ID); err != nil {
		t.Fatal(err)
	}
	// An article finished too, and an hours ramp still running.
	clk.now = sep(13, 22)
	article := startItem(t, svc, newItem(t, svc, shelf.ID, "Article", func(it *library.Item) { it.Format = library.FormatArticle }).ID)
	if _, err := svc.Finish(ctx, article.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitRamp, StartMinutes: 30, IncrementMinutes: 15, CeilingMinutes: 90}, false); err != nil {
		t.Fatal(err)
	}

	rec, err := svc.Record(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Books != 3 || rec.Other != 1 || rec.Pages != 1050 || rec.Time != 8*time.Hour || rec.Days != 3 || !rec.Since.Equal(sep(3, 0)) {
		t.Fatalf("totals %+v", rec)
	}
	if len(rec.Campaigns) != 2 {
		t.Fatalf("campaigns %+v", rec.Campaigns)
	}
	newest, oldest := rec.Campaigns[0], rec.Campaigns[1]
	if newest.State.Campaign.ID != short.ID || newest.State.Finished != 1 || !newest.MetOn.IsZero() || !newest.State.Over {
		t.Fatalf("the short campaign %+v", newest)
	}
	if oldest.State.Campaign.ID != met.ID || !oldest.MetOn.Equal(sep(8, 0)) {
		t.Fatalf("the met campaign %+v", oldest)
	}
	if len(rec.HoursRamps) != 1 || rec.HoursRamps[0].Start != 30 || rec.HoursRamps[0].Ceiling != 90 || !rec.HoursRamps[0].ReachedOn.IsZero() {
		t.Fatalf("hours ramps %+v", rec.HoursRamps)
	}
	if len(rec.Years) != 1 || rec.Years[0].Year != 2026 || rec.Years[0].Books != 3 || rec.Years[0].Goals != 1 {
		t.Fatalf("years %+v", rec.Years)
	}
	b := rec.Bests
	if b.LongestBook == nil || b.LongestBook.Title != "Long" || !b.Day.Equal(sep(8, 0)) || b.DayTime != 5*time.Hour ||
		!b.Week.Equal(sep(6, 0)) || b.WeekTime != 7*time.Hour {
		t.Fatalf("bests %+v", b)
	}
}
