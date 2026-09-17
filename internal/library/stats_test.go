package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

func TestStats(t *testing.T) {
	svc, clk := utcLibrary(t)
	shelf := newShelf(t, svc, "S")
	clk.now = sep(1, 8)
	if _, err := svc.StartCampaign(ctx, "", 10, sep(1, 0), sep(30, 0)); err != nil {
		t.Fatal(err)
	}
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitFixed, MinutesPerDay: 60}, false); err != nil {
		t.Fatal(err)
	}
	clk.now = sep(15, 22) // Tuesday; weeks start on Sunday
	book := startItem(t, svc, newItem(t, svc, shelf.ID, "Book", func(it *library.Item) { it.SizeValue = ptr(120) }).ID)
	for d, reached := range map[int]int{2: 30, 9: 60, 14: 120} {
		if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(d, 19), sep(d, 21), ptr(reached), ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Finish(ctx, book.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	article := startItem(t, svc, newItem(t, svc, shelf.ID, "Article", func(it *library.Item) { it.Format = library.FormatArticle }).ID)
	if _, err := svc.Finish(ctx, article.ID, "", nil); err != nil {
		t.Fatal(err)
	}

	st, err := svc.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The first week with anything is the plan's, from 30 Aug.
	if len(st.Weeks) != 3 || !st.Weeks[0].Start.Equal(time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)) || st.Weeks[2].Logged != 2*time.Hour {
		t.Fatalf("weeks %+v", st.Weeks)
	}
	if len(st.Days) != 28 || !st.Days[27].Day.Equal(sep(15, 0)) || st.Days[26].Logged != 2*time.Hour || st.Days[26].Target != 60 {
		t.Fatalf("days: last two %+v %+v", st.Days[26], st.Days[27])
	}
	// Owed builds on every day not read: 14 closed days of the plan, 1–14 Sep.
	if len(st.Owed) != 14 || st.Owed[len(st.Owed)-1].OwedAfter <= 0 {
		t.Fatalf("owed %d days, last %+v", len(st.Owed), st.Owed[len(st.Owed)-1])
	}
	if m := st.Months[len(st.Months)-1]; m.Books != 1 || m.Other != 1 || !m.Month.Equal(sep(1, 0)) {
		t.Fatalf("this month %+v", m)
	}
	if !near(st.Pace.PagesPerHour, 20) || len(st.Bands) != 1 || st.Bands[0].Items != 1 || len(st.ItemPaces) != 1 || st.ItemPaces[0].Time != 6*time.Hour {
		t.Fatalf("pace %+v bands %+v items %+v", st.Pace, st.Bands, st.ItemPaces)
	}
	if st.Campaign == nil || len(st.Counted) != 1 || !st.Counted[0].Equal(sep(15, 0)) {
		t.Fatalf("campaign %+v counted %v", st.Campaign, st.Counted)
	}
}
