package library_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// shortlist puts an item on the shortlist.
func shortlist(t *testing.T, svc *library.Service, id string) {
	t.Helper()
	if _, err := svc.SetShortlist(ctx, id, true); err != nil {
		t.Fatalf("shortlist: %v", err)
	}
}

func home(t *testing.T, svc *library.Service, m library.Moment) *library.HomeView {
	t.Helper()
	view, err := svc.Home(ctx, m)
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	return view
}

func pickTitles(picks []library.Pick) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Item.Title
	}
	return out
}

func TestSetShortlist(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	pool := newItem(t, svc, shelf.ID, "pool")
	reading := startItem(t, svc, newItem(t, svc, shelf.ID, "reading").ID)
	done := startItem(t, svc, newItem(t, svc, shelf.ID, "done").ID)
	if _, err := svc.Finish(ctx, done.ID, "", nil); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{pool.ID, reading.ID} {
		item, err := svc.SetShortlist(ctx, id, true)
		if err != nil || !item.OnShortlist {
			t.Fatalf("shortlist %s: %v, on=%v", id, err, item != nil && item.OnShortlist)
		}
	}
	item, err := svc.SetShortlist(ctx, pool.ID, false)
	if err != nil || item.OnShortlist {
		t.Fatalf("unshortlist: %v, on=%v", err, item != nil && item.OnShortlist)
	}
	if _, err := svc.SetShortlist(ctx, done.ID, true); !errors.Is(err, library.ErrInvalidTransition) {
		t.Fatalf("shortlisting a finished item: %v, want ErrInvalidTransition", err)
	}
}

func TestHomePicksAreShortlistedPoolItemsInShelfOrder(t *testing.T) {
	svc, _ := newTestLibrary(t)
	first := newShelf(t, svc, "First")
	second := newShelf(t, svc, "Second")

	// Second shelf: an unranked pick, a slot-2 pick, a slot-1 pick, filed in
	// that order so creation order cannot explain the result.
	loose := newItem(t, svc, second.ID, "Zed unranked")
	slot2 := newItem(t, svc, second.ID, "Slot two")
	slot1 := newItem(t, svc, second.ID, "Slot one")
	rank(t, svc, second.ID, slot1.ID, 1)
	rank(t, svc, second.ID, slot2.ID, 2)
	// First shelf: two unranked picks, ordered by title.
	beta := newItem(t, svc, first.ID, "Beta")
	alpha := newItem(t, svc, first.ID, "Alpha")
	// Not picks: not shortlisted, or shortlisted but in progress.
	newItem(t, svc, first.ID, "Not shortlisted")
	reading := startItem(t, svc, newItem(t, svc, first.ID, "Being read").ID)
	for _, it := range []*library.Item{loose, slot2, slot1, beta, alpha, reading} {
		shortlist(t, svc, it.ID)
	}

	view := home(t, svc, library.Moment{})
	got := pickTitles(view.Picks)
	want := []string{"Alpha", "Beta", "Slot one", "Slot two", "Zed unranked"}
	if len(got) != len(want) {
		t.Fatalf("picks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("picks = %v, want %v", got, want)
		}
	}
	if view.Picks[0].ShelfName != "First" || view.Picks[2].ShelfName != "Second" {
		t.Fatalf("shelf names %q, %q", view.Picks[0].ShelfName, view.Picks[2].ShelfName)
	}
	if len(view.Reading) != 1 || view.Reading[0].Item.ID != reading.ID || !view.Reading[0].Fits {
		t.Fatalf("reading = %v", view.Reading)
	}
}

func TestHomeMoment(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	// Papers are read in one sitting, so the time at hand filters them.
	paper := func(n int, focus library.FocusDemand) func(*library.Item) {
		return func(it *library.Item) { it.Format = library.FormatPaper; it.SizeValue = &n; it.FocusDemand = focus }
	}
	// Seed pace is 30 pages/h for medium, 15 for deep: 10 pages = 20 min,
	// 30 pages = 60 min, 300 pages = 10 h.
	short := newItem(t, svc, shelf.ID, "short", paper(10, library.FocusMedium))
	hour := newItem(t, svc, shelf.ID, "hour", paper(30, library.FocusMedium))
	long := newItem(t, svc, shelf.ID, "long", paper(300, library.FocusMedium))
	deep := newItem(t, svc, shelf.ID, "deep", paper(10, library.FocusDeep))
	unsized := newItem(t, svc, shelf.ID, "unsized", func(it *library.Item) { it.Format, it.FocusDemand = library.FormatPaper, library.FocusMedium })
	// A book is read in stretches: any time at hand suits it.
	book := newItem(t, svc, shelf.ID, "book", func(it *library.Item) { n := 600; it.SizeValue = &n })
	for _, it := range []*library.Item{short, hour, long, deep, unsized, book} {
		shortlist(t, svc, it.ID)
	}
	reading := startItem(t, svc, newItem(t, svc, shelf.ID, "reading", paper(300, library.FocusDeep)).ID)

	cases := []struct {
		name string
		m    library.Moment
		want []string
		fits bool // whether the 300-page deep item being read fits
	}{
		{"long shows all", library.Moment{Time: library.TimeLong}, []string{"book", "deep", "hour", "long", "short", "unsized"}, true},
		{"zero moment shows all", library.Moment{}, []string{"book", "deep", "hour", "long", "short", "unsized"}, true},
		{"hour is an upper bound", library.Moment{Time: library.TimeHour}, []string{"book", "deep", "hour", "short", "unsized"}, false},
		{"quick", library.Moment{Time: library.TimeQuick}, []string{"book", "short", "unsized"}, false},
		{"fried hides deep", library.Moment{Fried: true}, []string{"book", "hour", "long", "short", "unsized"}, false},
		{"fried and quick", library.Moment{Time: library.TimeQuick, Fried: true}, []string{"book", "short", "unsized"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view := home(t, svc, c.m)
			got := pickTitles(view.Picks)
			if len(got) != len(c.want) {
				t.Fatalf("picks = %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("picks = %v, want %v", got, c.want)
				}
			}
			if len(view.Reading) != 1 || view.Reading[0].Item.ID != reading.ID {
				t.Fatalf("in-progress items must never be hidden: %v", view.Reading)
			}
			if view.Reading[0].Fits != c.fits {
				t.Fatalf("reading fits = %v, want %v", view.Reading[0].Fits, c.fits)
			}
		})
	}

	view := home(t, svc, library.Moment{})
	for _, p := range view.Picks {
		switch p.Item.Title {
		case "unsized":
			if p.Remaining.Known() {
				t.Fatalf("unsized: estimate %v, want unknown", p.Remaining)
			}
		case "hour":
			if p.Remaining.Remaining != time.Hour || !p.Remaining.Provisional() {
				t.Fatalf("hour: estimate %v, want a provisional hour", p.Remaining)
			}
		}
	}
}

func TestHomeLoggedToday(t *testing.T) {
	svc, clk := newTestLibrary(t)
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Timezone = "Europe/Madrid"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	shelf := newShelf(t, svc, "S")
	item := startItem(t, svc, newItem(t, svc, shelf.ID, "book").ID)

	// 2026-03-29 is the day Madrid springs forward: 23 hours long, running
	// from 23:00Z the day before to 22:00Z.
	log := func(start, end time.Time) {
		t.Helper()
		if _, err := svc.AddRetroactiveSession(ctx, item.ID, start, end, nil, ""); err != nil {
			t.Fatal(err)
		}
	}
	clk.now = time.Date(2026, 3, 29, 21, 0, 0, 0, time.UTC)                                           // 23:00 local on the 29th
	log(time.Date(2026, 3, 28, 22, 0, 0, 0, time.UTC), time.Date(2026, 3, 29, 4, 0, 0, 0, time.UTC))  // clipped at 23:00Z: 5 h on the 29th
	log(time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC), time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)) // another day
	session, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(10 * time.Minute)
	if got := home(t, svc, library.Moment{}).Schedule.LoggedToday; got != 5*time.Hour+10*time.Minute {
		t.Fatalf("logged on the 29th = %v, want 5h10m", got)
	}

	// Past midnight local, on the 30th: only what fell after 22:00Z counts.
	// A naive from+24h would also count the half hour before it.
	if _, err := svc.StopSession(ctx, session.ID, library.Stop{}); err != nil {
		t.Fatal(err)
	}
	clk.now = time.Date(2026, 3, 29, 22, 45, 0, 0, time.UTC) // 00:45 local on the 30th
	log(time.Date(2026, 3, 29, 21, 30, 0, 0, time.UTC), time.Date(2026, 3, 29, 22, 30, 0, 0, time.UTC))
	if got := home(t, svc, library.Moment{}).Schedule.LoggedToday; got != 30*time.Minute {
		t.Fatalf("logged on the 30th = %v, want 30m", got)
	}
}
