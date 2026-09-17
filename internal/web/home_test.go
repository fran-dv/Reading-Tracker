package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// homeFixture has one book in progress with a session behind it, one deep
// paper in progress never read, and two shortlisted picks: a quick article
// and a long book. A third pool item is not shortlisted.
type homeFixture struct {
	handler   http.Handler
	svc       *library.Service
	shelf     *library.Shelf
	book      *library.Item // in progress, 296 pages, read to page 100
	paper     *library.Item // in progress, deep, never read
	article   *library.Item // pick, 2 300 words: about 10 minutes
	long      *library.Item // pick, 600 pages: many hours
	pool      *library.Item // not shortlisted
	logged    time.Duration // read today: an hour, less just after midnight
	readLabel string        // "read today", or "read yesterday" just after midnight
}

func newHomeFixture(t *testing.T) homeFixture {
	t.Helper()
	h, svc := newTestServer(t, &fakeMeta{})
	f := homeFixture{handler: h, svc: svc}
	var err error
	if f.shelf, err = svc.CreateShelf(ctx, "Statistics"); err != nil {
		t.Fatal(err)
	}
	f.book = fileItem(t, svc, library.Item{Title: "Deep Work", Author: "Cal Newport", Why: "focus", Format: library.FormatBook, ShelfID: f.shelf.ID, SizeValue: ptr(296)})
	f.paper = fileItem(t, svc, library.Item{Title: "Attention Is All You Need", Why: "the paper", Format: library.FormatPaper, ShelfID: f.shelf.ID, SizeValue: ptr(15)})
	f.article = fileItem(t, svc, library.Item{Title: "A Short Article", Why: "quick one", Format: library.FormatArticle, ShelfID: f.shelf.ID, SizeValue: ptr(2300)})
	f.long = fileItem(t, svc, library.Item{Title: "War and Peace", Why: "someday", Format: library.FormatBook, ShelfID: f.shelf.ID, SizeValue: ptr(600)})
	f.pool = fileItem(t, svc, library.Item{Title: "Not Yet", Why: "later", Format: library.FormatBook, ShelfID: f.shelf.ID})
	for _, item := range []*library.Item{f.book, f.paper} {
		if _, err := svc.Start(ctx, item.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []*library.Item{f.article, f.long} {
		if _, err := svc.SetShortlist(ctx, item.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	// An hour of reading that ended an hour ago. Just after midnight it lies
	// in yesterday, so the page's day words are worked out, not assumed.
	now := time.Now()
	start, end := now.Add(-2*time.Hour), now.Add(-time.Hour)
	if _, err := svc.AddRetroactiveSession(ctx, f.book.ID, start, end, ptr(100), ""); err != nil {
		t.Fatal(err)
	}
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	f.readLabel = "read " + dayLabel(start, now, time.Local)
	if end.After(midnight) {
		f.logged = end.Sub(start)
		if start.Before(midnight) {
			f.logged = end.Sub(midnight)
		}
	}
	return f
}

func momentSignals(bucket library.TimeBucket, fried bool) homeForm {
	return homeForm{Moment: momentForm{Time: bucket, Fried: fried}}
}

func TestHomePage(t *testing.T) {
	f := newHomeFixture(t)
	body := get(t, f.handler, "/").Body.String()
	for _, want := range []string{
		`href="/" aria-current="page"`,
		`<strong>` + minutesLabel(f.logged) + `</strong> read`, `No daily target yet.`,
		`value="long" data-bind="moment.time"`, `data-bind="moment.fried"`,
		`Deep Work`, `from page 100`, f.readLabel, `style="--read: 0.338"`,
		`Attention Is All You Need`, `from page 0`,
		`A Short Article`, `War and Peace`, `Statistics`,
		`href="/session?item=` + f.book.ID + `"`,
		`data-href="/items/` + f.book.ID + `/done"`,
		`data-href="/items/` + f.article.ID + `/start"`,
		`class="mark figure provisional"`, // seed pace, labelled
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home missing %q", want)
		}
	}
	if strings.Contains(body, "Not Yet") {
		t.Error("an item off the shortlist is not a pick")
	}
	if sig := pageSignals[homeForm](t, body); sig.Moment != defaultMoment {
		t.Errorf("seed = %+v, want long and not fried", sig.Moment)
	}
	// Picks show the whole estimate; an in-progress item shows what is left.
	if !strings.Contains(body, `10 min</span>`) || !strings.Contains(body, ` left</span>`) {
		t.Error("estimates missing")
	}
}

func TestHomeEmpty(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	body := get(t, h, "/").Body.String()
	for _, want := range []string{"Nothing is in progress.", "Nothing to pick from", "<strong>0 min</strong> read"} {
		if !strings.Contains(body, want) {
			t.Errorf("empty home missing %q", want)
		}
	}
}

func TestHomeMomentFilters(t *testing.T) {
	f := newHomeFixture(t)
	cases := []struct {
		name    string
		moment  homeForm
		article bool // the quick pick shows
		long    bool // the long pick shows
		faint   []string
	}{
		{"long", momentSignals(library.TimeLong, false), true, true, nil},
		{"quick", momentSignals(library.TimeQuick, false), true, false, []string{"Deep Work", "Attention Is All You Need"}},
		{"fried", momentSignals(library.TimeLong, true), true, true, []string{"Attention Is All You Need"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := send(t, f.handler, http.MethodGet, "/home/body", c.moment)
			body := rec.Body.String()
			if rec.Code != http.StatusOK || !strings.Contains(body, `id="home-body"`) {
				t.Fatalf("status %d:\n%s", rec.Code, body)
			}
			if strings.Contains(body, "A Short Article") != c.article || strings.Contains(body, "War and Peace") != c.long {
				t.Fatalf("picks wrong for %s:\n%s", c.name, body)
			}
			// In-progress items are never hidden, only faded.
			for _, title := range []string{"Deep Work", "Attention Is All You Need"} {
				if !strings.Contains(body, title) {
					t.Fatalf("%s hidden", title)
				}
			}
			if got := strings.Count(body, "entry-faint"); got != len(c.faint) {
				t.Fatalf("%d faint entries, want %d", got, len(c.faint))
			}
			// The seed carries the moment, so a morph cannot reset it.
			if sig := pageSignals[homeForm](t, body); sig.Moment != c.moment.Moment {
				t.Fatalf("seed = %+v, want %+v", sig.Moment, c.moment.Moment)
			}
		})
	}
}

func TestHomeDone(t *testing.T) {
	f := newHomeFixture(t)
	quick := momentSignals(library.TimeQuick, false)

	body := send(t, f.handler, http.MethodGet, "/items/"+f.book.ID+"/done", quick).Body.String()
	if !strings.Contains(body, `id="verdict"`) || !strings.Contains(body, `data-href="/items/`+f.book.ID+`/reference"`) {
		t.Fatalf("done form not open:\n%s", body)
	}
	if strings.Contains(body, `/done"`) {
		t.Error("other entries keep their controls while a form is open")
	}
	if strings.Contains(body, "War and Peace") {
		t.Error("the moment was lost while opening the form")
	}

	in := quick
	in.Verdict = "  Worth it.  "
	body = send(t, f.handler, http.MethodPost, "/items/"+f.book.ID+"/finish", in).Body.String()
	if !strings.Contains(body, "Finished Deep Work.") || strings.Contains(body, "from page 100") {
		t.Fatalf("finish:\n%s", body)
	}
	item, err := f.svc.GetItem(ctx, f.book.ID)
	if err != nil || item.State != library.StateFinished || item.Verdict != "Worth it." {
		t.Fatalf("item after finish: %+v, %v", item, err)
	}

	body = send(t, f.handler, http.MethodPost, "/items/"+f.paper.ID+"/reference", quick).Body.String()
	if !strings.Contains(body, "Kept Attention Is All You Need for reference.") || !strings.Contains(body, "Nothing is in progress.") {
		t.Fatalf("reference:\n%s", body)
	}

	// Closing twice is a stale tab: 409.
	if rec := send(t, f.handler, http.MethodPost, "/items/"+f.book.ID+"/finish", quick); rec.Code != http.StatusConflict {
		t.Fatalf("second finish = %d, want 409", rec.Code)
	}
}

func TestHomeStartPick(t *testing.T) {
	f := newHomeFixture(t)
	long := momentSignals(library.TimeLong, false)

	body := send(t, f.handler, http.MethodPost, "/items/"+f.article.ID+"/start", long).Body.String()
	if !strings.Contains(body, "Started A Short Article.") || !strings.Contains(body, `href="/session?item=`+f.article.ID+`"`) {
		t.Fatalf("start:\n%s", body)
	}
	if strings.Contains(body, `data-href="/items/`+f.article.ID+`/start"`) {
		t.Error("a started item is no longer a pick")
	}

	settings, err := f.svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.WIPCap = 3
	if err := f.svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	body = send(t, f.handler, http.MethodPost, "/items/"+f.long.ID+"/start", long).Body.String()
	if !strings.Contains(body, "Already 3 in progress. Finish or abandon one first.") {
		t.Fatalf("at the cap:\n%s", body)
	}
}

func TestShelfShortlistToggle(t *testing.T) {
	f := newShelfFixture(t)
	href := "/shelves/" + f.stats.ID + "/items/" + f.pool.ID

	body := send(t, f.handler, http.MethodPost, href+"/shortlist", nil).Body.String()
	if !strings.Contains(body, "The Art of Doing Science is on the shortlist.") || !strings.Contains(body, `data-href="`+href+`/unshortlist"`) {
		t.Fatalf("shortlist:\n%s", body)
	}
	if !strings.Contains(get(t, f.handler, "/").Body.String(), "The Art of Doing Science") {
		t.Error("home does not offer the shortlisted item")
	}
	body = send(t, f.handler, http.MethodPost, href+"/unshortlist", nil).Body.String()
	if !strings.Contains(body, "is off the shortlist.") || !strings.Contains(body, `data-href="`+href+`/shortlist"`) {
		t.Fatalf("unshortlist:\n%s", body)
	}
	// An in-progress entry offers no toggle.
	if strings.Contains(body, `/items/`+f.reading.ID+`/shortlist"`) {
		t.Error("in-progress entries are shortlisted from the review, not the shelf")
	}
}

func TestSessionPreselectsItem(t *testing.T) {
	f := newSessionFixture(t)
	body := get(t, f.handler, "/session?item="+f.first.ID).Body.String()
	if sig := pageSignals[sessionForm](t, body); sig.Now.ItemID != f.first.ID || sig.Earlier.ItemID != f.first.ID {
		t.Errorf("seed = %+v, want %s in both pickers", sig, f.first.ID)
	}
	// An unknown id falls back to the default.
	body = get(t, f.handler, "/session?item=nope").Body.String()
	if sig := pageSignals[sessionForm](t, body); sig.Now.ItemID != f.second.ID {
		t.Errorf("seed = %+v, want the default", sig)
	}
}

func TestDayLabel(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Madrid")
	now := time.Date(2026, 9, 16, 0, 30, 0, 0, loc) // just past midnight, local
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-10 * time.Minute), "today"},
		{now.Add(-time.Hour), "yesterday"}, // 23:30 the day before, though only an hour ago
		{now.Add(-25 * time.Hour), "14 Sep"},
		{time.Date(2026, 9, 14, 22, 30, 0, 0, time.UTC), "yesterday"}, // 00:30 on the 15th, local
	}
	for _, c := range cases {
		if got := dayLabel(c.t, now, loc); got != c.want {
			t.Errorf("dayLabel(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}
