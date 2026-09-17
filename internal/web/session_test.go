package web

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// sessionFixture has two items in progress and one still in the pool.
type sessionFixture struct {
	handler http.Handler
	svc     *library.Service
	shelf   *library.Shelf
	first   *library.Item // started first, never read: last in the pickers
	second  *library.Item // started second: first in the pickers
	pool    *library.Item
}

func newSessionFixture(t *testing.T, opts ...library.Option) sessionFixture {
	t.Helper()
	h, svc := newTestServer(t, &fakeMeta{}, opts...)
	f := sessionFixture{handler: h, svc: svc}
	var err error
	if f.shelf, err = svc.CreateShelf(ctx, "Statistics"); err != nil {
		t.Fatal(err)
	}
	pages := 296
	f.first = fileItem(t, svc, library.Item{Title: "Thinking in Systems", Why: "loops", Format: library.FormatBook, ShelfID: f.shelf.ID})
	f.second = fileItem(t, svc, library.Item{Title: "Deep Work", Why: "focus", Format: library.FormatBook, ShelfID: f.shelf.ID, SizeValue: &pages})
	f.pool = fileItem(t, svc, library.Item{Title: "The Art of Doing Science", Why: "later", Format: library.FormatBook, ShelfID: f.shelf.ID})
	for _, item := range []*library.Item{f.first, f.second} {
		if _, err := svc.Start(ctx, item.ID); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // started_at is stored to the millisecond; keep the order unambiguous
	}
	return f
}

func nowSignals(itemID, reached, note string) sessionForm {
	var in sessionForm
	in.Now = nowForm{ItemID: itemID, Reached: reached, Note: note}
	return in
}

func earlierSignals(itemID, minutes, endedAt, reached string) sessionForm {
	var in sessionForm
	in.Earlier = earlierForm{ItemID: itemID, Minutes: minutes, EndedAt: endedAt, Reached: reached}
	return in
}

// errorsOf reads the errors signal out of an in-band rejection.
func errorsOf(t *testing.T, body string) map[string]any {
	t.Helper()
	errs, _ := patchedSignals(t, body)["errors"].(map[string]any)
	if errs == nil {
		t.Fatalf("no errors signal in:\n%s", body)
	}
	return errs
}

func TestSessionPage(t *testing.T) {
	f := newSessionFixture(t)
	body := get(t, f.handler, "/session").Body.String()
	for _, want := range []string{
		`href="/session" aria-current="page"`,
		`id="now-picker"`, `id="earlier-picker"`, // one picker per form
		`data-title="Deep Work"`, `data-title="Thinking in Systems"`,
		`@post('/sessions/start')`, `@post('/sessions')`,
		`id="ended-at" type="datetime-local"`,
		`from page 0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("session page missing %q", want)
		}
	}
	// The most recently started item leads both pickers.
	if sig := pageSignals[sessionForm](t, body); sig.Now.ItemID != f.second.ID || sig.Earlier.ItemID != f.second.ID || sig.Earlier.From != "from page 0" {
		t.Errorf("seed = %+v", sig)
	}
	if strings.Contains(body, "The Art of Doing Science") {
		t.Error("pool items are not offered")
	}
	if strings.Contains(body, "session-clock") {
		t.Error("nothing is running yet")
	}
}

func TestSessionPageEmpty(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	body := get(t, h, "/session").Body.String()
	if !strings.Contains(body, "Nothing is in progress") || strings.Contains(body, "now-picker") {
		t.Errorf("empty page should say so and offer no form:\n%s", body)
	}
}

func TestStartAndStopTimer(t *testing.T) {
	f := newSessionFixture(t)

	rec := send(t, f.handler, http.MethodPost, "/sessions/start", nowSignals(f.first.ID, "", ""))
	body := rec.Body.String()
	running, err := f.svc.RunningSession(ctx)
	if err != nil || running == nil || running.ItemID != f.first.ID {
		t.Fatalf("running = %v, %v", running, err)
	}
	for _, want := range []string{
		`id="session-body"`, "Thinking in Systems", `class="session-clock figure"`,
		`data-since="` + strconv.FormatInt(running.StartedAt.UnixMilli(), 10) + `"`, // the clock ticks from the server's start
		`/sessions/` + running.ID + `/stop`,
		`id="earlier-picker"`, // the retro form stays available while the timer runs
	} {
		if !strings.Contains(body, want) {
			t.Errorf("running body missing %q", want)
		}
	}
	if strings.Contains(body, `@post('/sessions/start')`) {
		t.Error("the start form should be gone while running")
	}
	// Reloading shows the same running state: the timer lives server-side.
	if page := get(t, f.handler, "/session").Body.String(); !strings.Contains(page, `/sessions/`+running.ID+`/stop`) {
		t.Error("page reload lost the running timer")
	}

	// A stale tab starting again just sees the running state.
	if rec := send(t, f.handler, http.MethodPost, "/sessions/start", nowSignals(f.second.ID, "", "")); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "session-clock") {
		t.Errorf("second start: %d\n%s", rec.Code, rec.Body.String())
	}

	// A bad position is rejected in band; the timer keeps running.
	rec = send(t, f.handler, http.MethodPost, "/sessions/"+running.ID+"/stop", nowSignals("", "forty", ""))
	if errs := errorsOf(t, rec.Body.String()); errs["reached"] != "Use a whole number." {
		t.Errorf("errors = %v", errs)
	}
	if !strings.Contains(rec.Body.String(), `getElementById("reached").focus()`) {
		t.Error("focus should move to the reached field")
	}

	rec = send(t, f.handler, http.MethodPost, "/sessions/"+running.ID+"/stop", nowSignals("", " 42 ", "slow chapter"))
	body = rec.Body.String()
	if !strings.Contains(body, "Logged 0 min on Thinking in Systems.") || !strings.Contains(body, `@post('/sessions/start')`) {
		t.Errorf("after stop:\n%s", body)
	}
	if !strings.Contains(body, `"from":"from page 42"`) {
		t.Error("the earlier form should resume from page 42")
	}
	sessions, err := f.svc.Sessions(ctx, f.first.ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions = %v, %v", sessions, err)
	}
	if s := sessions[0]; s.Running() || *s.PositionStart != 0 || *s.PositionEnd != 42 || s.Note != "slow chapter" {
		t.Errorf("stopped session = %+v", s)
	}
}

func TestLogSession(t *testing.T) {
	f := newSessionFixture(t)
	yesterday := time.Now().Add(-24 * time.Hour).Format(datetimeLocal)

	rec := send(t, f.handler, http.MethodPost, "/sessions", earlierSignals(f.second.ID, "30", yesterday, "60"))
	body := rec.Body.String()
	if !strings.Contains(body, "Logged 30 min on Deep Work.") {
		t.Errorf("after log:\n%s", body)
	}
	// The forms reset: the seed goes out as a signals patch too.
	if sig := patchedSignals(t, body); sig["earlier"].(map[string]any)["minutes"] != "" {
		t.Errorf("minutes not reset: %v", sig["earlier"])
	}
	sessions, err := f.svc.Sessions(ctx, f.second.ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions = %v, %v", sessions, err)
	}
	s := sessions[0]
	end, _ := time.ParseInLocation(datetimeLocal, yesterday, time.Local)
	if !s.EnteredRetroactively || s.Duration() != 30*time.Minute || !s.EndedAt.Equal(end) {
		t.Errorf("logged session = %+v, want 30 min ending %v", s, end)
	}
	if *s.PositionStart != 0 || *s.PositionEnd != 60 {
		t.Errorf("positions %d-%d, want 0-60", *s.PositionStart, *s.PositionEnd)
	}
}

func TestLogSessionRejections(t *testing.T) {
	f := newSessionFixture(t)
	now := time.Now()
	past := now.Add(-2 * time.Hour).Format(datetimeLocal)

	tests := []struct {
		name  string
		in    sessionForm
		field string
	}{
		{"no minutes", earlierSignals(f.second.ID, "", past, ""), "minutes"},
		{"zero minutes", earlierSignals(f.second.ID, "0", past, ""), "minutes"},
		{"no end", earlierSignals(f.second.ID, "20", "", ""), "endedAt"},
		{"in the future", earlierSignals(f.second.ID, "20", now.Add(time.Hour).Format(datetimeLocal), ""), "endedAt"},
		{"bad position", earlierSignals(f.second.ID, "20", past, "x"), "logReached"},
		{"not in progress", earlierSignals(f.pool.ID, "20", past, ""), "logItem"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(t, f.handler, http.MethodPost, "/sessions", tc.in)
			errs := errorsOf(t, rec.Body.String())
			if errs[tc.field] == "" {
				t.Errorf("errors = %v, want one under %q", errs, tc.field)
			}
			for k, v := range errs {
				if k != tc.field && v != "" {
					t.Errorf("stray error %q: %v", k, v)
				}
			}
		})
	}

}

// Overlapping the running timer is refused under the end time. The clock is
// frozen so the timer can have started an hour ago.
func TestLogSessionOverlappingTheTimer(t *testing.T) {
	now := time.Date(2026, 9, 15, 22, 0, 0, 0, time.Local)
	clock := now.Add(-time.Hour)
	f := newSessionFixture(t, library.WithClock(func() time.Time { return clock }))
	if _, err := f.svc.StartSession(ctx, f.first.ID); err != nil {
		t.Fatal(err)
	}
	clock = now

	rec := send(t, f.handler, http.MethodPost, "/sessions", earlierSignals(f.second.ID, "20", now.Add(-30*time.Minute).Format(datetimeLocal), ""))
	if errs := errorsOf(t, rec.Body.String()); !strings.Contains(errs["endedAt"].(string), "Overlaps the running session") {
		t.Errorf("errors = %v", errs)
	}
	// Ending before the timer started is fine.
	rec = send(t, f.handler, http.MethodPost, "/sessions", earlierSignals(f.second.ID, "20", now.Add(-time.Hour).Format(datetimeLocal), ""))
	if body := rec.Body.String(); !strings.Contains(body, "Logged 20 min on Deep Work.") {
		t.Errorf("earlier session refused:\n%s", body)
	}
}

func TestShelfStart(t *testing.T) {
	f := newShelfFixture(t)
	href := "/shelves/" + f.stats.ID + "/items/"
	if err := f.svc.Rank(ctx, f.stats.ID, f.pool.ID, 1); err != nil {
		t.Fatal(err)
	}

	body := send(t, f.handler, http.MethodPost, href+f.pool.ID+"/start", nil).Body.String()
	if !strings.Contains(body, "Started The Art of Doing Science. Slot 1 is free.") {
		t.Errorf("status missing:\n%s", body)
	}
	item, err := f.svc.GetItem(ctx, f.pool.ID)
	if err != nil || item.State != library.StateInProgress {
		t.Fatalf("item = %v, %v", item, err)
	}
	if !strings.Contains(body, `href="/shelves/`+f.stats.ID+`/items/`+f.pool.ID+`/edit"`) && !strings.Contains(body, `data-href="`+href+f.pool.ID+`/edit"`) {
		t.Error("the started item stays on the shelf")
	}
	if strings.Contains(body, `data-href="`+href+f.pool.ID+`/start"`) {
		t.Error("an item in progress offers no Start")
	}

	// At the WIP cap the line says so and nothing changes.
	settings, err := f.svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.WIPCap = 2
	if err := f.svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
	body = send(t, f.handler, http.MethodPost, href+f.textbook.ID+"/start", nil).Body.String()
	if !strings.Contains(body, "Already 2 in progress. Finish or abandon one first.") {
		t.Errorf("cap status missing:\n%s", body)
	}
	if item, _ := f.svc.GetItem(ctx, f.textbook.ID); item.State != library.StatePool {
		t.Error("the item should still be in the pool")
	}
}

func TestLabels(t *testing.T) {
	for _, tc := range []struct {
		d     time.Duration
		clock string
		mins  string
	}{
		{0, "0:00", "0 min"},
		{42*time.Minute + 13*time.Second, "42:13", "42 min"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1:02:03", "1 h 02 min"},
		{3 * time.Hour, "3:00:00", "3 h 00 min"},
	} {
		if got := clockLabel(tc.d); got != tc.clock {
			t.Errorf("clockLabel(%v) = %q, want %q", tc.d, got, tc.clock)
		}
		if got := minutesLabel(tc.d); got != tc.mins {
			t.Errorf("minutesLabel(%v) = %q, want %q", tc.d, got, tc.mins)
		}
	}
}

func TestTimeTypedInHours(t *testing.T) {
	f := newSessionFixture(t)
	yesterday := time.Now().Add(-24 * time.Hour).Format(datetimeLocal)

	// Earlier reading: "1h30" is ninety minutes.
	rec := send(t, f.handler, http.MethodPost, "/sessions", earlierSignals(f.second.ID, "1h30", yesterday, ""))
	if !strings.Contains(rec.Body.String(), "Logged 1 h 30 min on Deep Work.") {
		t.Fatalf("hours:\n%s", rec.Body.String())
	}

	// A video reads positions the way the player shows them.
	video := fileItem(t, f.svc, library.Item{Title: "A Lecture", Why: "watch", Format: library.FormatVideo, ShelfID: f.shelf.ID})
	if _, err := f.svc.Start(ctx, video.ID); err != nil {
		t.Fatal(err)
	}
	rec = send(t, f.handler, http.MethodPost, "/sessions", earlierSignals(video.ID, "45", time.Now().Add(-3*time.Hour).Format(datetimeLocal), "1:12:30"))
	if !strings.Contains(rec.Body.String(), "Logged 45 min on A Lecture.") {
		t.Fatalf("video log:\n%s", rec.Body.String())
	}
	sessions, err := f.svc.Sessions(ctx, video.ID)
	if err != nil || len(sessions) != 1 || *sessions[0].PositionEnd != 72 {
		t.Fatalf("video position stored in minutes: %v %+v", err, sessions)
	}
	rec = send(t, f.handler, http.MethodPost, "/sessions", earlierSignals(video.ID, "10", time.Now().Add(-time.Hour).Format(datetimeLocal), "1:75:00"))
	if errorsOf(t, rec.Body.String())["logReached"] != "Type the time the player shows, like 1:12:30." {
		t.Fatalf("bad player time:\n%s", rec.Body.String())
	}
}
