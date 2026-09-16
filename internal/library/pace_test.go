package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

var t0 = time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)

// session builds a finished session of the given length that read from
// start to end. Positions are dropped when start is nil.
func session(itemID string, startedAt time.Time, length time.Duration, start, end *int) library.Session {
	endedAt := startedAt.Add(length)
	s := library.Session{ItemID: itemID, StartedAt: startedAt, EndedAt: &endedAt, PositionStart: start, PositionEnd: end}
	if length == 0 {
		s.EndedAt = nil
	}
	return s
}

func TestItemPace(t *testing.T) {
	cases := []struct {
		name    string
		history []library.Session
		want    float64
		ok      bool
	}{
		{"no sessions", nil, 0, false},
		{"one session", []library.Session{session("a", t0, time.Hour, ptr(0), ptr(30))}, 30, true},
		{"sums across sessions", []library.Session{
			session("a", t0, 30*time.Minute, ptr(0), ptr(10)),
			session("a", t0.Add(time.Hour), 90*time.Minute, ptr(10), ptr(50)),
		}, 25, true},
		{"skips sessions without positions", []library.Session{
			session("a", t0, time.Hour, ptr(0), ptr(30)),
			session("a", t0.Add(2*time.Hour), 5*time.Hour, nil, nil),
		}, 30, true},
		{"skips the running session", []library.Session{
			session("a", t0, time.Hour, ptr(0), ptr(30)),
			session("a", t0.Add(2*time.Hour), 0, ptr(30), ptr(90)),
		}, 30, true},
		{"only time, no positions", []library.Session{session("a", t0, time.Hour, nil, nil)}, 0, false},
		{"correction outweighs progress", []library.Session{
			session("a", t0, time.Hour, ptr(0), ptr(30)),
			session("a", t0.Add(2*time.Hour), time.Hour, ptr(30), ptr(0)),
		}, 0, false},
		{"correction still sums", []library.Session{
			session("a", t0, time.Hour, ptr(0), ptr(40)),
			session("a", t0.Add(2*time.Hour), time.Hour, ptr(40), ptr(20)),
		}, 10, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := library.ItemPace(c.history)
			if ok != c.ok || got != c.want {
				t.Fatalf("ItemPace = %v, %v; want %v, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestBandPaces(t *testing.T) {
	book := func(id string, focus library.FocusDemand, unit library.SizeUnit) library.Item {
		return library.Item{ID: id, Format: library.FormatBook, FocusDemand: focus, SizeUnit: unit}
	}
	items := []library.Item{
		book("a", library.FocusMedium, library.UnitPages),
		book("b", library.FocusMedium, library.UnitPages),
		book("c", library.FocusMedium, library.UnitPages),
		book("d", library.FocusMedium, library.UnitPages), // no usable sessions
		book("e", library.FocusDeep, library.UnitPages),
		book("f", library.FocusMedium, library.UnitWords),
	}
	since := t0.Add(-90 * 24 * time.Hour)
	sessions := []library.Session{
		session("a", since, time.Hour, ptr(0), ptr(20)),                    // exactly on the boundary: in
		session("a", since.Add(-time.Second), time.Hour, ptr(0), ptr(200)), // just before: out
		session("b", t0.Add(-24*time.Hour), time.Hour, ptr(0), ptr(30)),
		session("c", t0.Add(-24*time.Hour), time.Hour, ptr(0), ptr(40)),
		session("d", t0.Add(-24*time.Hour), time.Hour, nil, nil),
		session("e", t0.Add(-24*time.Hour), time.Hour, ptr(0), ptr(12)),
		session("f", t0.Add(-24*time.Hour), time.Hour, ptr(0), ptr(9000)),
	}
	got := library.BandPaces(items, sessions, since)
	want := library.Paces{
		{library.FormatBook, library.FocusMedium, library.UnitPages}: 30, // median of 20, 30, 40
		{library.FormatBook, library.FocusDeep, library.UnitPages}:   12,
		{library.FormatBook, library.FocusMedium, library.UnitWords}: 9000,
	}
	if len(got) != len(want) {
		t.Fatalf("bands = %v, want %v", got, want)
	}
	for band, pace := range want {
		if got[band] != pace {
			t.Fatalf("band %v = %v, want %v", band, got[band], pace)
		}
	}

	// An even count averages the two middle paces.
	items = items[:2]
	sessions = []library.Session{
		session("a", t0, time.Hour, ptr(0), ptr(20)),
		session("b", t0, time.Hour, ptr(0), ptr(30)),
	}
	if got := library.BandPaces(items, sessions, since)[items[0].Band()]; got != 25 {
		t.Fatalf("even median = %v, want 25", got)
	}
}

func TestTimeRemaining(t *testing.T) {
	settings := library.Settings{SeedPaceLight: 40, SeedPaceMedium: 30, SeedPaceDeep: 15, SeedPaceWPM: 230}
	book := library.Item{ID: "a", Format: library.FormatBook, FocusDemand: library.FocusMedium, SizeUnit: library.UnitPages, SizeValue: ptr(300)}
	bands := library.Paces{book.Band(): 60}
	own := []library.Session{session("a", t0, time.Hour, ptr(0), ptr(100))} // 100 pages/h, at page 100

	cases := []struct {
		name    string
		item    library.Item
		history []library.Session
		bands   library.Paces
		want    library.Estimate
	}{
		{"own pace first", book, own, bands, library.Estimate{Remaining: 2 * time.Hour, Basis: library.BasisItem}},
		{"band when no own pace", book, []library.Session{session("a", t0, time.Hour, nil, nil)}, bands,
			library.Estimate{Remaining: 5 * time.Hour, Basis: library.BasisBand}},
		{"seed when no band", book, nil, nil, library.Estimate{Remaining: 10 * time.Hour, Basis: library.BasisSeed}},
		{"seed by focus", edit(book, func(it *library.Item) { it.FocusDemand = library.FocusDeep }), nil, nil,
			library.Estimate{Remaining: 20 * time.Hour, Basis: library.BasisSeed}},
		{"seed for words ignores focus", library.Item{Format: library.FormatArticle, FocusDemand: library.FocusDeep, SizeUnit: library.UnitWords, SizeValue: ptr(6900)},
			nil, nil, library.Estimate{Remaining: 30 * time.Minute, Basis: library.BasisSeed}},
		{"minutes are exact", library.Item{ID: "v", Format: library.FormatVideo, FocusDemand: library.FocusLight, SizeUnit: library.UnitMinutes, SizeValue: ptr(90)},
			[]library.Session{session("v", t0, 10*time.Minute, ptr(0), ptr(35))}, nil,
			library.Estimate{Remaining: 55 * time.Minute, Basis: library.BasisExact}},
		{"unknown size", edit(book, func(it *library.Item) { it.SizeValue = nil }), own, bands, library.Estimate{}},
		{"read past the end", edit(book, func(it *library.Item) { it.SizeValue = ptr(80) }), own, bands,
			library.Estimate{Remaining: 0, Basis: library.BasisItem}},
		{"rounds to the minute", edit(book, func(it *library.Item) { it.SizeValue = ptr(101) }), own, bands,
			library.Estimate{Remaining: time.Minute, Basis: library.BasisItem}}, // 1 page at 100/h = 36s
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := library.TimeRemaining(c.item, c.history, c.bands, settings)
			if got != c.want {
				t.Fatalf("TimeRemaining = %+v, want %+v", got, c.want)
			}
			if got.Known() != (c.want.Basis != "") || got.Provisional() != (c.want.Basis == library.BasisSeed) {
				t.Fatalf("Known/Provisional = %v/%v for %+v", got.Known(), got.Provisional(), got)
			}
		})
	}
}

func edit(item library.Item, fn func(*library.Item)) library.Item {
	fn(&item)
	return item
}

// TestReadingMeasures checks the wiring: Reading carries an estimate and the
// stall flag, computed against the clock and the stored settings.
func TestReadingMeasures(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	sized := func(n int) func(*library.Item) { return func(it *library.Item) { it.SizeValue = ptr(n) } }
	read := startItem(t, svc, newItem(t, svc, shelf.ID, "read", sized(300)).ID)
	fresh := startItem(t, svc, newItem(t, svc, shelf.ID, "fresh", sized(300)).ID)
	quiet := startItem(t, svc, newItem(t, svc, shelf.ID, "quiet", sized(300)).ID)

	// quiet was read once, then left for 15 days; read was read yesterday.
	now := clk.Now()
	if _, err := svc.AddRetroactiveSession(ctx, quiet.ID, now.Add(-15*24*time.Hour), now.Add(-15*24*time.Hour+time.Hour), ptr(20), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, read.ID, now.Add(-24*time.Hour), now.Add(-23*time.Hour), ptr(100), ""); err != nil {
		t.Fatal(err)
	}

	byTitle := func() map[string]library.Reading {
		got, err := svc.Reading(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]library.Reading{}
		for _, r := range got {
			out[r.Item.Title] = r
		}
		return out
	}
	got := byTitle()
	if r := got["read"]; r.Remaining != (library.Estimate{Remaining: 2 * time.Hour, Basis: library.BasisItem}) || r.Stalled {
		t.Fatalf("read: %+v", r)
	}
	// fresh has no sessions: band = median of read (100/h) and quiet (20/h).
	if r := got["fresh"]; r.Remaining != (library.Estimate{Remaining: 5 * time.Hour, Basis: library.BasisBand}) || r.Stalled {
		t.Fatalf("fresh: %+v", r)
	}
	if r := got["quiet"]; !r.Stalled || r.Remaining.Basis != library.BasisItem {
		t.Fatalf("quiet: %+v", r)
	}

	// 14 days later: fresh has been untouched since it started, read is quiet
	// too, and quiet's session fell out of a short pace window.
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.PaceWindowDays = 7
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	clk.Advance(14*24*time.Hour + time.Minute)
	got = byTitle()
	if !got["fresh"].Stalled || !got["read"].Stalled {
		t.Fatalf("after 14 days: fresh %v, read %v; want both stalled", got["fresh"].Stalled, got["read"].Stalled)
	}
	if got["fresh"].Remaining.Basis != library.BasisSeed {
		t.Fatalf("fresh with an empty window should fall to seed, got %+v", got["fresh"].Remaining)
	}

	// A running session makes an item current again.
	if _, err := svc.StartSession(ctx, fresh.ID); err != nil {
		t.Fatal(err)
	}
	if got = byTitle(); got["fresh"].Stalled {
		t.Fatal("running session should clear the stall")
	}
}
