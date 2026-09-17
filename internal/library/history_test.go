package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// History draws a week: each day's sessions against its planned target,
// days before any plan with their reading only, and what the week's time
// went to. It never goes before the first week with anything in it, nor
// after the current one.
func TestHistory(t *testing.T) {
	svc, clk := newTestLibrary(t)
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Timezone = "UTC"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	shelf := newShelf(t, svc, "S")
	book := startItem(t, svc, newItem(t, svc, shelf.ID, "Book").ID)
	article := startItem(t, svc, newItem(t, svc, shelf.ID, "Article", func(it *library.Item) { it.Format = library.FormatArticle }).ID)

	// Tuesday 15 Sep 2026, 10:00 UTC. Weeks start on Sunday: 13–19 Sep.
	day := func(d, h int) time.Time { return time.Date(2026, 9, d, h, 0, 0, 0, time.UTC) }
	log := func(id string, start time.Time, length time.Duration, reached *int) {
		t.Helper()
		if _, err := svc.AddRetroactiveSession(ctx, id, start, start.Add(length), reached, ""); err != nil {
			t.Fatal(err)
		}
	}
	log(book.ID, day(6, 21), time.Hour, ptr(40))       // the week before
	log(book.ID, day(13, 21), 30*time.Minute, ptr(60)) // Sunday, before the plan
	log(book.ID, day(14, 7), 20*time.Minute, ptr(70))  // Monday, the plan's first day
	log(article.ID, day(14, 20), 15*time.Minute, nil)  // Monday evening
	log(book.ID, day(14, 22), 10*time.Minute, ptr(65)) // rereading: no progress
	clk.now = day(14, 6)
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitFixed, MinutesPerDay: 45}, false); err != nil {
		t.Fatal(err)
	}
	clk.now = day(15, 10)

	v, err := svc.History(ctx, day(15, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !v.WeekStart.Equal(day(13, 0)) || !v.Earliest.Equal(day(6, 0)) || len(v.Days) != 7 {
		t.Fatalf("week %v, earliest %v, %d days", v.WeekStart, v.Earliest, len(v.Days))
	}
	sun, mon, tue := v.Days[0], v.Days[1], v.Days[2]
	if sun.Planned || sun.Logged != 30*time.Minute || len(sun.Sessions) != 1 {
		t.Fatalf("Sunday, before the plan: %+v", sun)
	}
	if !mon.Planned || mon.Target != 45 || mon.Logged != 45*time.Minute || !mon.Closed || len(mon.Sessions) != 3 {
		t.Fatalf("Monday: %+v", mon.DaySheet)
	}
	if mon.Sessions[1].Item.Title != "Article" {
		t.Fatalf("sessions carry their item: %+v", mon.Sessions[1])
	}
	if !tue.Planned || tue.Closed || tue.Logged != 0 {
		t.Fatalf("Tuesday, today: %+v", tue.DaySheet)
	}
	// 45 min a day from Monday: six days.
	if v.Target != 6*45 || v.Logged != 75*time.Minute {
		t.Fatalf("week target %d, logged %v", v.Target, v.Logged)
	}
	if len(v.Items) != 2 || v.Items[0].Item.Title != "Book" || v.Items[0].Time != time.Hour || v.Items[0].Progress != 30 || v.Items[0].Sessions != 3 {
		t.Fatalf("items %+v", v.Items)
	}

	if v, _ := svc.History(ctx, day(1, 0)); !v.WeekStart.Equal(day(6, 0)) {
		t.Fatalf("before the first week: %v, want the week of 6 Sep", v.WeekStart)
	}
	if v, _ := svc.History(ctx, day(30, 0)); !v.WeekStart.Equal(day(13, 0)) {
		t.Fatalf("after the current week: %v, want this week", v.WeekStart)
	}
}
