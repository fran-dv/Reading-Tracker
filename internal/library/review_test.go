package library_test

import (
	"slices"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

func TestReviewDue(t *testing.T) {
	week := func(m time.Month, d int) []library.Review { return []library.Review{{WeekOf: day(m, d)}} }
	cases := []struct {
		name    string
		reviews []library.Review
		today   time.Time
		want    bool
	}{
		{"before the first review, even on review day", nil, day(9, 13), true},
		{"review day, last week reviewed", week(9, 6), day(9, 13), false},
		{"the day after, not yet reviewed", week(9, 6), day(9, 14), true},
		{"late in a reviewed week", week(9, 13), day(9, 19), false},
		{"next review day", week(9, 13), day(9, 20), false},
		{"the day after that", week(9, 13), day(9, 21), true},
	}
	for _, c := range cases {
		if got := library.ReviewDue(c.reviews, c.today, time.Sunday); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCloseReviewAndDue(t *testing.T) {
	svc, clk := newTestLibrary(t)
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Timezone = "America/Argentina/Buenos_Aires"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	due := func(when string) bool {
		t.Helper()
		view, err := svc.Home(ctx, library.Moment{})
		if err != nil {
			t.Fatalf("%s: %v", when, err)
		}
		return view.ReviewDue
	}
	closeReview := func() *library.Review {
		t.Helper()
		rv, err := svc.CloseReview(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return rv
	}

	clk.now = time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC) // Saturday
	if !due("before any review") {
		t.Fatal("the first review should be due")
	}
	if rv := closeReview(); !rv.WeekOf.Equal(day(9, 6)) || rv.Needs != nil {
		t.Fatalf("closed %+v, want week of 6 Sep with no campaign", rv)
	}
	if due("after closing") {
		t.Fatal("a closed week should not be due")
	}

	clk.now = time.Date(2026, 9, 14, 2, 30, 0, 0, time.UTC) // Sunday 23:30 local
	if due("review day, late evening") {
		t.Fatal("the review day itself is not overdue")
	}
	clk.now = time.Date(2026, 9, 14, 3, 30, 0, 0, time.UTC) // Monday 00:30 local
	if !due("the day after review day") {
		t.Fatal("the review should be overdue once the review day has passed")
	}
	if rv := closeReview(); !rv.WeekOf.Equal(day(9, 13)) {
		t.Fatalf("closed week of %v, want 13 Sep", rv.WeekOf)
	}
	clk.Advance(time.Hour)
	closeReview() // again in the same week: replaces it
	if due("after closing late") {
		t.Fatal("a late review should clear the indicator")
	}
	out, err := svc.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Reviews) != 2 || !out.Reviews[1].ClosedAt.Equal(clk.now) {
		t.Fatalf("reviews %+v, want two weeks with the second closed at %v", out.Reviews, clk.now)
	}
}

func TestCompareNeeds(t *testing.T) {
	needs := func(books int, pages, pace, weeks float64) library.Needs {
		return library.Needs{CampaignID: "c", BooksLeft: books, AvgPages: pages, PagesPerHour: pace,
			WeeksLeft: weeks, WeeklyHours: float64(books) * pages / pace / weeks}
	}
	then := needs(50, 300, 30, 25) // 20 h a week
	cases := []struct {
		name string
		now  library.Needs
		want []library.Cause
	}{
		{"within 10%", needs(49, 300, 30, 24.5), nil},
		{"pace fell", needs(49, 300, 24, 24), []library.Cause{library.CausePace}},
		{"weeks ran out and pace fell", needs(50, 300, 26, 20), []library.Cause{library.CauseWeeks, library.CausePace}},
		{"weeks outweigh a faster pace", needs(50, 300, 33, 20), []library.Cause{library.CauseWeeks}},
		{"thinner books", needs(50, 200, 30, 25), []library.Cause{library.CausePages}},
		{"books read ahead", needs(40, 300, 30, 24), []library.Cause{library.CauseBooks}},
		{"target reached", needs(0, 300, 30, 20), nil},
	}
	since := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	for _, c := range cases {
		got := library.CompareNeeds(since, then, c.now)
		if !slices.Equal(got.Causes, c.want) {
			t.Errorf("%s: causes %v, want %v (ratio %.3f)", c.name, got.Causes, c.want, got.Ratio())
		}
		if !got.Since.Equal(since) || got.Then != then || got.Now != c.now {
			t.Errorf("%s: change does not carry its inputs: %+v", c.name, got)
		}
	}
}

func TestComposition(t *testing.T) {
	loc := buenosAires
	st := campaignSettings()
	st.BucketQuickMaxMin, st.BucketHourMaxMin = 25, 75
	utc := func(m time.Month, d, hour, minute int) time.Time {
		return time.Date(2026, m, d, hour, minute, 0, 0, time.UTC)
	}
	unsized := func(it *library.Item) { it.SizeValue = nil }
	format := func(f library.Format) func(*library.Item) {
		return func(it *library.Item) { it.Format = f }
	}
	items := []library.Item{
		book("long", library.StateFinished, 300, finishedAt(utc(9, 1, 12, 0))),
		book("memo", library.StateReference, 20, finishedAt(utc(9, 5, 12, 0)), format(library.FormatPaper)),
		book("essay", library.StateFinished, 0, finishedAt(utc(8, 20, 12, 0)), unsized, format(library.FormatArticle)),
		book("clip", library.StateFinished, 0, finishedAt(utc(9, 10, 12, 0)), unsized, format(library.FormatVideo)),
		book("quit", library.StateAbandoned, 400, finishedAt(utc(8, 25, 12, 0))),
		book("eve", library.StateFinished, 200, finishedAt(utc(8, 16, 2, 30))),      // 15 Aug, 23:30 local
		book("this-week", library.StateFinished, 250, finishedAt(utc(9, 13, 4, 0))), // 13 Sep, 01:00 local
		book("no-pages", library.StateFinished, 0, finishedAt(utc(8, 1, 12, 0)), unsized),
		book("early", library.StateFinished, 500, finishedAt(utc(7, 19, 15, 0))),
		book("reading", library.StateInProgress, 300),
	}
	sessions := []library.Session{
		session("long", utc(8, 30, 12, 0), 2*time.Hour, nil, nil),
		session("long", utc(8, 31, 12, 0), time.Hour, nil, nil),
		session("memo", utc(9, 5, 11, 0), 25*time.Minute, nil, nil),   // exactly the quick bound
		session("essay", utc(8, 20, 11, 0), 75*time.Minute, nil, nil), // exactly the hour bound
		session("reading", utc(9, 14, 11, 0), 5*time.Hour, nil, nil),
	}
	now := utc(9, 15, 12, 0) // Tuesday; the window is 16 Aug – 12 Sep

	campaign := &library.Campaign{StartedOn: day(7, 20), Deadline: day(12, 31)}
	comp := library.MeasureComposition(items, sessions, campaign, st, loc, now)
	if !comp.From.Equal(day(8, 16)) || !comp.To.Equal(day(9, 13)) {
		t.Fatalf("window %v – %v, want 16 Aug – 13 Sep", comp.From, comp.To)
	}
	wantFormat := map[library.Format]library.Tally{
		library.FormatBook:    {Completed: 1},
		library.FormatPaper:   {Completed: 1, Reference: 1},
		library.FormatArticle: {Completed: 1},
		library.FormatVideo:   {Completed: 1},
	}
	for _, f := range library.Formats {
		if comp.ByFormat[f] != wantFormat[f] {
			t.Errorf("format %s: %+v, want %+v", f, comp.ByFormat[f], wantFormat[f])
		}
	}
	wantSize := map[library.SizeBucket]library.Tally{
		library.SizeShort:   {Completed: 1, Reference: 1},
		library.SizeHour:    {Completed: 1},
		library.SizeLong:    {Completed: 1},
		library.SizeUntimed: {Completed: 1},
	}
	for _, b := range library.SizeBuckets {
		if comp.BySize[b] != wantSize[b] {
			t.Errorf("size %s: %+v, want %+v", b, comp.BySize[b], wantSize[b])
		}
	}
	if comp.Abandoned != 1 {
		t.Errorf("abandoned %d, want 1", comp.Abandoned)
	}
	wantBlocks := []library.PagesBlock{
		{From: day(8, 16), To: day(9, 13), Books: 1, Sized: 1, MeanPages: 300},
		{From: day(7, 20), To: day(8, 16), Books: 2, Sized: 1, MeanPages: 200}, // clipped to the campaign
	}
	if !slices.Equal(comp.BookPages, wantBlocks) {
		t.Errorf("with a campaign: blocks %+v, want %+v", comp.BookPages, wantBlocks)
	}

	// Without a campaign the trend reaches back to the first finished book.
	comp = library.MeasureComposition(items, sessions, nil, st, loc, now)
	wantBlocks[1] = library.PagesBlock{From: day(7, 19), To: day(8, 16), Books: 3, Sized: 2, MeanPages: 350}
	if !slices.Equal(comp.BookPages, wantBlocks) {
		t.Errorf("without a campaign: blocks %+v, want %+v", comp.BookPages, wantBlocks)
	}

	// Nothing finished and no campaign: no trend at all.
	if comp := library.MeasureComposition(items[9:], nil, nil, st, loc, now); comp.BookPages != nil {
		t.Errorf("empty trend: %+v", comp.BookPages)
	}
}

func TestReviewView(t *testing.T) {
	svc, clk := newTestLibrary(t)
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Timezone = "America/Argentina/Buenos_Aires"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	shortlist := func(id string) {
		t.Helper()
		if _, err := svc.SetShortlist(ctx, id, true); err != nil {
			t.Fatal(err)
		}
	}

	a := newShelf(t, svc, "A")
	b := newShelf(t, svc, "B")
	reading := newItem(t, svc, a.ID, "Reading")
	startItem(t, svc, reading.ID)
	shortlist(reading.ID)
	lead := newItem(t, svc, a.ID, "Lead")
	setTags(t, svc, lead.ID, "B")
	rank(t, svc, a.ID, lead.ID, 1)
	rank(t, svc, b.ID, lead.ID, 1) // leads both shelves, offered once
	second := newItem(t, svc, b.ID, "Second")
	rank(t, svc, b.ID, second.ID, 2)
	ticked := newItem(t, svc, a.ID, "Ticked")
	shortlist(ticked.ID)
	zeta := newItem(t, svc, a.ID, "Zeta")
	alpha := newItem(t, svc, b.ID, "Alpha")

	view, err := svc.Review(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(entries []library.ShortlistEntry) []string {
		var out []string
		for _, e := range entries {
			out = append(out, e.Item.ID)
		}
		return out
	}
	wantIDs(t, ids(view.Shortlist), reading.ID, lead.ID, second.ID, ticked.ID)
	wantIDs(t, ids(view.Pool), zeta.ID, alpha.ID) // by shelf, not by title
	if view.Shortlisted() != 2 || view.Shortlist[2].ShelfName != "B" {
		t.Fatalf("shortlisted %d, third entry on %q; want 2 and B", view.Shortlisted(), view.Shortlist[2].ShelfName)
	}
	if len(view.Shelves) != 2 || view.Shelves[1].Slots[0].ID != lead.ID || !view.Shelves[1].Slots[0].Borrowed {
		t.Fatalf("shelves %+v, want B led by borrowed Lead", view.Shelves)
	}
	if view.Closed != nil || view.Change != nil || !view.WeekOf.Equal(day(9, 13)) {
		t.Fatalf("fresh review: closed %v, change %v, week %v", view.Closed, view.Change, view.WeekOf)
	}

	campaign, err := svc.StartCampaign(ctx, "", 10, day(9, 1), day(12, 31))
	if err != nil {
		t.Fatal(err)
	}
	closed, err := svc.CloseReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Needs == nil || closed.Needs.CampaignID != campaign.ID || closed.Needs.BooksLeft != 10 {
		t.Fatalf("closed needs %+v, want the campaign's 10 books", closed.Needs)
	}
	view, err = svc.Review(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Closed == nil || view.Change != nil {
		t.Fatalf("same week: closed %v, change %v; want closed and nothing to compare", view.Closed, view.Change)
	}

	clk.Advance(7 * 24 * time.Hour)
	view, err = svc.Review(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Closed != nil || view.Change == nil || !view.Change.Since.Equal(closed.ClosedAt) ||
		view.Change.Then != *closed.Needs {
		t.Fatalf("next week: closed %v, change %+v; want a comparison with the last review", view.Closed, view.Change)
	}
	if view.Change.Now.WeeksLeft >= view.Change.Then.WeeksLeft {
		t.Fatalf("a week later the weeks left should shrink: %+v", view.Change)
	}
}

// The review lists what was reached since the last review closed, and what
// the week just closed gave each item.
func TestReviewReachedAndLastWeek(t *testing.T) {
	svc, clk := utcLibrary(t)
	shelf := newShelf(t, svc, "S")
	book := startItem(t, svc, newItem(t, svc, shelf.ID, "Book", func(it *library.Item) { it.SizeValue = ptr(100) }).ID)
	other := startItem(t, svc, newItem(t, svc, shelf.ID, "Other", func(it *library.Item) { it.SizeValue = ptr(100) }).ID)

	// 6–12 Sep: two hours on Book, to page 60.
	clk.now = sep(12, 22)
	for d, reached := range map[int]int{7: 30, 9: 60} {
		if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(d, 20), sep(d, 21), ptr(reached), ""); err != nil {
			t.Fatal(err)
		}
	}
	// The review closes on Sunday the 13th.
	clk.now = sep(13, 9)
	if _, err := svc.CloseReview(ctx); err != nil {
		t.Fatal(err)
	}
	// Book is finished on the 14th and Other on the 20th, both after it.
	clk.now = sep(14, 9)
	if _, err := svc.AddRetroactiveSession(ctx, book.ID, sep(14, 7), sep(14, 8), ptr(100), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finish(ctx, book.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	clk.now = sep(20, 9)
	if _, err := svc.Finish(ctx, other.ID, "", nil); err != nil {
		t.Fatal(err)
	}

	v, err := svc.Review(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The week of the 20th; the last review closed on the 13th.
	if !v.ReachedFrom.Equal(sep(13, 0)) || len(v.Reached) != 2 || v.Reached[0].Item.Title != "Other" || v.Reached[1].Item.Title != "Book" {
		t.Fatalf("reached from %v: %+v", v.ReachedFrom, v.Reached)
	}
	// Last week (13–19 Sep): Book, 1 h, 40 pages to page 100.
	if len(v.LastWeek) != 1 || v.LastWeek[0].Time != time.Hour || v.LastWeek[0].Progress != 40 || v.LastWeek[0].Reached != 100 {
		t.Fatalf("last week %+v", v.LastWeek)
	}
}
