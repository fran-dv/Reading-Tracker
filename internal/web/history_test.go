package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// History draws the week of the chosen day, lists that day's sessions with
// their corrections, and a correction made there redraws History.
func TestHistoryPage(t *testing.T) {
	f := newSessionFixture(t)
	now := time.Now()
	if now.Hour() < 3 {
		t.Skip("needs a few hours of today behind it")
	}
	if _, err := f.svc.AddRetroactiveSession(ctx, f.second.ID, now.Add(-2*time.Hour), now.Add(-time.Hour), ptr(30), "slow"); err != nil {
		t.Fatal(err)
	}
	today := now.Format(dateField)

	body := get(t, f.handler, "/history").Body.String()
	for _, want := range []string{
		`aria-current="page">History`, `class="hgrid-block cloth-book"`, "Deep Work", "page 0 → 30",
		"The week went to", "1 session", "30 pages", `href="/history?day=` + today + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("history missing %q", want)
		}
	}
	if sig := pageSignals[historyForm](t, body); sig.History.Day != today {
		t.Errorf("signals name the chosen day: %+v", sig.History)
	}

	// A day with nothing logged, in a week before any reading: clamped to
	// the first week, whose first day is chosen.
	if body := get(t, f.handler, "/history?day=2001-01-01").Body.String(); strings.Contains(body, "The week before") {
		t.Error("the first week offers no week before it")
	}

	sessions, _ := f.svc.Sessions(ctx, f.second.ID)
	var in sessionForm
	in.History.Day = today
	body = send(t, f.handler, http.MethodPost, "/sessions/"+sessions[0].ID+"/delete", in).Body.String()
	if !strings.Contains(body, `id="history-body"`) || !strings.Contains(body, "Removed 1 h 00 min on Deep Work.") || !strings.Contains(body, "Nothing logged this day.") {
		t.Fatalf("a delete on History should redraw History:\n%s", body)
	}
}

// Home's day strip names link each day that has happened to History.
func TestDayStripLinksToHistory(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitFixed, MinutesPerDay: 30}, false); err != nil {
		t.Fatal(err)
	}
	if body := get(t, h, "/").Body.String(); !strings.Contains(body, `class="day-name" href="/history?day=`+time.Now().Format(dateField)+`"`) {
		t.Errorf("today's name should link to History:\n%s", body)
	}
}
