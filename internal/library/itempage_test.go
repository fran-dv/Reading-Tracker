package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// An item's page adds up its sessions, gives its own pace and what is left,
// and dates the finish at the rate it was read over the last two weeks.
func TestItemPage(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "Novels")
	item := startItem(t, svc, newItem(t, svc, shelf.ID, "Stoner", func(it *library.Item) { it.SizeValue = ptr(300) }).ID)
	setTags(t, svc, item.ID, "classics")
	now := clk.Now() // Tuesday 15 Sep 2026, 10:00 UTC

	// Three hours in the last fortnight, up to page 90.
	for daysAgo, reached := range map[int]int{5: 30, 3: 60, 1: 90} {
		start := now.AddDate(0, 0, -daysAgo)
		if _, err := svc.AddRetroactiveSession(ctx, item.ID, start, start.Add(time.Hour), ptr(reached), ""); err != nil {
			t.Fatal(err)
		}
	}
	// A session long before the fortnight still counts in the totals.
	old := now.AddDate(0, 0, -30)
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, old, old.Add(time.Hour), ptr(15), ""); err != nil {
		t.Fatal(err)
	}

	p, err := svc.ItemPage(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.ShelfName != "Novels" || len(p.Tags) != 1 || len(p.Sessions) != 4 || p.Days != 4 || p.Total != 4*time.Hour {
		t.Fatalf("page %+v", p)
	}
	if p.Position != 90 {
		t.Fatalf("position %d, want 90", p.Position)
	}
	// 90 pages in 4 h.
	if p.Pace != 22.5 || p.PaceTime != 4*time.Hour || p.Remaining.Basis != library.BasisItem {
		t.Fatalf("pace %v over %v, basis %s", p.Pace, p.PaceTime, p.Remaining.Basis)
	}
	// 210 pages at 22.5/h is 9 h 20 min; 3 h over 14 days is 12 min 51 s a day: 44 days.
	if p.Recent != 3*time.Hour/14 {
		t.Fatalf("recent %v", p.Recent)
	}
	want := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 44)
	if !p.FinishOn.Equal(want) {
		t.Fatalf("finish on %v, want %v", p.FinishOn, want)
	}
}
