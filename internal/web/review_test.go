package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// newReviewFixture is the shelf fixture with the textbook leading both
// Statistics and IQ (where it is borrowed), and an hour logged on the book
// being read.
func newReviewFixture(t *testing.T) shelfFixture {
	t.Helper()
	f := newShelfFixture(t)
	for _, shelf := range []*library.Shelf{f.stats, f.iq} {
		if err := f.svc.Rank(ctx, shelf.ID, f.textbook.ID, 1); err != nil {
			t.Fatal(err)
		}
	}
	end := time.Now().Add(-time.Hour)
	if _, err := f.svc.AddRetroactiveSession(ctx, f.reading.ID, end.Add(-time.Hour), end, nil, ""); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestReviewPage(t *testing.T) {
	f := newReviewFixture(t)
	body := get(t, f.handler, "/review").Body.String()
	for _, want := range []string{
		`href="/review" aria-current="page"`,
		"<h2>Whys</h2>", "<h2>Prune and rank</h2>", ">Shortlist<", ">Goal status<", ">Composition<",
		`<p class="entry-reason">the one that clicked</p>`, // reread under its title
		"from Statistics", // borrowed onto IQ
		`data-href="/review/` + f.iq.ID + `/items/` + f.textbook.ID + `/up"`,
		`id="delete-` + f.stats.ID + `-` + f.textbook.ID + `"`, // nothing logged: deletable
		"0 on it", "Fewer than 5 leaves little to pick from",
		"The Art of Doing Science", // in the pool, to fill a slot or reach for
		"Nothing completed in these weeks.",
		"No campaign running.",
		"Close the review",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("review page missing %q", want)
		}
	}
	// The book being read has history: abandon only, never delete.
	if strings.Contains(body, `id="delete-reading-`+f.reading.ID+`"`) {
		t.Error("an item with reading logged must not offer delete")
	}
	if !strings.Contains(body, `data-href="/review/reading/items/`+f.reading.ID+`/abandon"`) {
		t.Error("the book being read should offer abandon")
	}
	// The textbook leads two shelves but is offered to the shortlist once.
	if n := strings.Count(body, `data-href="/review/items/`+f.textbook.ID+`"`); n != 1 {
		t.Errorf("textbook offered to the shortlist %d times, want 1", n)
	}
}

func TestReviewAbandon(t *testing.T) {
	f := newReviewFixture(t)
	href := "/review/reading/items/" + f.reading.ID

	body := send(t, f.handler, http.MethodGet, href+"/abandon", reviewForm{}).Body.String()
	if !strings.Contains(body, `id="reason"`) || !strings.Contains(body, `data-href="/review/body"`) {
		t.Fatalf("abandon form not open:\n%s", body)
	}
	if strings.Contains(body, `/unrank"`) {
		t.Error("other rows keep their controls while a form is open")
	}

	rec := send(t, f.handler, http.MethodPost, href+"/abandon", reviewForm{Reason: "  "})
	if sig := patchedSignals(t, rec.Body.String()); !strings.Contains(rec.Body.String(), "Say why, in a line.") || sig["errors"] == nil {
		t.Fatalf("missing reason:\n%s", rec.Body.String())
	}

	body = send(t, f.handler, http.MethodPost, href+"/abandon", reviewForm{Reason: "Not for now"}).Body.String()
	if !strings.Contains(body, "Abandoned Thinking in Systems.") {
		t.Fatalf("abandon:\n%s", body)
	}
	item, err := f.svc.GetItem(ctx, f.reading.ID)
	if err != nil || item.State != library.StateAbandoned || item.AbandonedReason != "Not for now" {
		t.Fatalf("item after abandon: %+v, %v", item, err)
	}
}

func TestReviewDelete(t *testing.T) {
	f := newReviewFixture(t)
	body := send(t, f.handler, http.MethodPost, "/review/"+f.stats.ID+"/items/"+f.textbook.ID+"/delete", nil).Body.String()
	if !strings.Contains(body, "Deleted Statistics for Hackers.") || strings.Contains(body, "the one that clicked") {
		t.Fatalf("delete:\n%s", body)
	}
	// A stale dialog on an item with history: refused in words, nothing lost.
	body = send(t, f.handler, http.MethodPost, "/review/reading/items/"+f.reading.ID+"/delete", nil).Body.String()
	if !strings.Contains(body, "Thinking in Systems has reading logged. Abandon it instead.") {
		t.Fatalf("delete with history:\n%s", body)
	}
	if _, err := f.svc.GetItem(ctx, f.reading.ID); err != nil {
		t.Fatalf("item with history was deleted: %v", err)
	}
}

func TestReviewRankAndShortlist(t *testing.T) {
	f := newReviewFixture(t)
	body := send(t, f.handler, http.MethodPost, "/review/"+f.stats.ID+"/items/"+f.pool.ID+"/rank", nil).Body.String()
	if !strings.Contains(body, `data-href="/review/`+f.stats.ID+`/items/`+f.pool.ID+`/unrank"`) {
		t.Fatalf("rank from the pool:\n%s", body)
	}
	wantSlots(t, f, f.textbook.ID, f.pool.ID)

	body = send(t, f.handler, http.MethodPost, "/review/items/"+f.reading.ID+"/shortlist", nil).Body.String()
	if !strings.Contains(body, "1 on it") {
		t.Fatalf("shortlist tick:\n%s", body)
	}
	item, err := f.svc.GetItem(ctx, f.reading.ID)
	if err != nil || !item.OnShortlist {
		t.Fatalf("tick not written: %+v, %v", item, err)
	}
	body = send(t, f.handler, http.MethodPost, "/review/items/"+f.reading.ID+"/unshortlist", nil).Body.String()
	if !strings.Contains(body, "0 on it") {
		t.Fatalf("shortlist untick:\n%s", body)
	}
}

func TestReviewCloseClearsHomeIndicator(t *testing.T) {
	f := newReviewFixture(t)
	if !strings.Contains(get(t, f.handler, "/").Body.String(), `href="/review">Review due</a>`) {
		t.Fatal("home should say the first review is due")
	}
	body := send(t, f.handler, http.MethodPost, "/review/close", nil).Body.String()
	if !strings.Contains(body, "Closed today at ") || !strings.Contains(body, "Close again") {
		t.Fatalf("close:\n%s", body)
	}
	if strings.Contains(get(t, f.handler, "/").Body.String(), "Review due") {
		t.Error("home still says the review is due after closing it")
	}
}

func TestHomeAbandon(t *testing.T) {
	f := newHomeFixture(t)
	long := momentSignals(library.TimeLong, false)

	body := send(t, f.handler, http.MethodGet, "/items/"+f.paper.ID+"/abandon", long).Body.String()
	if !strings.Contains(body, `id="reason"`) || strings.Contains(body, `/done"`) {
		t.Fatalf("abandon form not open alone:\n%s", body)
	}
	if rec := send(t, f.handler, http.MethodPost, "/items/"+f.paper.ID+"/abandon", long); !strings.Contains(rec.Body.String(), "Say why, in a line.") {
		t.Fatalf("missing reason:\n%s", rec.Body.String())
	}
	in := long
	in.Reason = "Too dense for now"
	body = send(t, f.handler, http.MethodPost, "/items/"+f.paper.ID+"/abandon", in).Body.String()
	if !strings.Contains(body, "Abandoned Attention Is All You Need.") {
		t.Fatalf("abandon:\n%s", body)
	}
	item, err := f.svc.GetItem(ctx, f.paper.ID)
	if err != nil || item.State != library.StateAbandoned {
		t.Fatalf("item after abandon: %+v, %v", item, err)
	}
}

func TestShelvesManagement(t *testing.T) {
	f := newShelfFixture(t)
	poetry, err := f.svc.CreateShelf(ctx, "Poetry")
	if err != nil {
		t.Fatal(err)
	}

	body := get(t, f.handler, "/shelves").Body.String()
	if !strings.Contains(body, `id="delete-shelf-`+poetry.ID+`"`) || strings.Contains(body, `id="delete-shelf-`+f.stats.ID+`"`) {
		t.Fatal("only the empty shelf offers delete")
	}

	body = send(t, f.handler, http.MethodPost, "/shelves/"+poetry.ID+"/up", nil).Body.String()
	if strings.Index(body, ">Poetry<") > strings.Index(body, ">IQ<") {
		t.Fatalf("Poetry should now come before IQ:\n%s", body)
	}

	rename := "/shelves/" + f.iq.ID + "/rename"
	if rec := send(t, f.handler, http.MethodGet, rename, nil); !strings.Contains(rec.Body.String(), `id="shelf-name"`) || !strings.Contains(rec.Body.String(), `"name":"IQ"`) {
		t.Fatalf("rename form:\n%s", rec.Body.String())
	}
	if rec := send(t, f.handler, http.MethodPost, rename, shelvesForm{Name: "statistics"}); !strings.Contains(rec.Body.String(), "Another shelf has that name.") {
		t.Fatalf("duplicate name:\n%s", rec.Body.String())
	}
	if rec := send(t, f.handler, http.MethodPost, rename, shelvesForm{Name: " "}); !strings.Contains(rec.Body.String(), "A shelf needs a name.") {
		t.Fatalf("blank name:\n%s", rec.Body.String())
	}
	body = send(t, f.handler, http.MethodPost, rename, shelvesForm{Name: "Intelligence"}).Body.String()
	if !strings.Contains(body, "Renamed IQ to Intelligence. Its tags moved with it.") {
		t.Fatalf("rename:\n%s", body)
	}
	tags, err := f.svc.Tags(ctx, f.textbook.ID)
	if err != nil || len(tags) != 1 || tags[0] != "Intelligence" {
		t.Fatalf("textbook tags after rename: %v, %v", tags, err)
	}

	body = send(t, f.handler, http.MethodPost, "/shelves/"+poetry.ID+"/delete", nil).Body.String()
	if !strings.Contains(body, "Deleted Poetry.") || strings.Contains(body, ">Poetry<") {
		t.Fatalf("delete:\n%s", body)
	}
	body = send(t, f.handler, http.MethodPost, "/shelves/"+f.stats.ID+"/delete", nil).Body.String()
	if !strings.Contains(body, "Something is filed on Statistics now, so it stays.") {
		t.Fatalf("delete a shelf in use:\n%s", body)
	}
}

func TestChangeCause(t *testing.T) {
	then := library.Needs{BooksLeft: 50, AvgPages: 300, PagesPerHour: 30, WeeksLeft: 25, WeeklyHours: 20}
	now := library.Needs{BooksLeft: 50, AvgPages: 300, PagesPerHour: 26, WeeksLeft: 20, WeeklyHours: 50 * 300 / 26.0 / 20}
	v := newChangeView(library.CompareNeeds(time.Date(2026, 9, 13, 21, 0, 0, 0, time.UTC), then, now))
	want := "Needed each week rose from 20 h 00 min to 28 h 51 min, mostly because the time left fell from 25 weeks to 20 weeks, and book pace fell from 30 to 26 pages/h."
	if v.Cause != want || v.Since != "13 Sep" {
		t.Errorf("cause %q since %q\nwant %q", v.Cause, v.Since, want)
	}
	if small := newChangeView(library.CompareNeeds(time.Now(), then, then)); small.Cause != "" {
		t.Errorf("no change named a cause: %q", small.Cause)
	}
}

func TestReportHeadline(t *testing.T) {
	st := library.Settings{ProjectionWindowWeeks: 4, BucketQuickMaxMin: 25, BucketHourMaxMin: 75}
	c := library.Composition{
		From: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
		ByFormat: map[library.Format]library.Tally{library.FormatArticle: {Completed: 11}},
		BySize:   map[library.SizeBucket]library.Tally{library.SizeShort: {Completed: 11}},
		BookPages: []library.PagesBlock{{From: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
			Books: 3, Sized: 2, MeanPages: 312.4}},
	}
	v := newReportView(c, st)
	if v.Headline != "11 short items, 0 books." || v.Weeks != "last 4 weeks" || v.Window != "16 Aug – 12 Sep" {
		t.Errorf("headline %q, weeks %q, window %q", v.Headline, v.Weeks, v.Window)
	}
	if b := v.Blocks[0]; b.Books != "3 books" || b.Mean != "312 pages" || b.Unsized != "1 without a page count" {
		t.Errorf("block %+v", b)
	}
	if v.Sizes[1].Label != "an hour, up to 1 h 15 min" {
		t.Errorf("size label %q", v.Sizes[1].Label)
	}
}
