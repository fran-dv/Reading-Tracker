package web

import (
	"math"
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

// The grid's axis covers the default day, widens to whatever was read
// outside it, and squeezes every run of two or more hours the week read
// nothing in, so one session at 3 a.m. costs a band and not five hours.
func TestGridAxisSqueezesQuietHours(t *testing.T) {
	loc := time.UTC
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	at := func(hour, min int, length time.Duration) library.LoggedSession {
		start := time.Date(2026, 9, 17, hour, min, 0, 0, time.UTC)
		end := start.Add(length)
		return library.LoggedSession{Session: library.Session{StartedAt: start, EndedAt: &end}}
	}
	v := &library.HistoryView{Days: []library.HistoryDay{{
		Sessions: []library.LoggedSession{at(3, 20, 40*time.Minute), at(9, 0, 30*time.Minute)},
	}}}
	v.Days[0].Day = day

	a := newGridAxis(v, loc, day)
	if got, want := a.Hours[0].Label, "03:00"; got != want {
		t.Errorf("the grid starts at the earliest session: %q, want %q", got, want)
	}
	if got, want := a.Hours[len(a.Hours)-1].Label, "22:00"; got != want {
		t.Errorf("the grid ends at the default last hour: %q, want %q", got, want)
	}

	// 04:00–09:00 read nothing, and 10:00–22:00 likewise: two bands, not
	// seventeen hours.
	var quiet, hours int
	for _, b := range a.Bands {
		if b.Quiet {
			quiet++
			continue
		}
		hours++
	}
	if quiet != 2 || hours != 2 {
		t.Errorf("bands: %d quiet and %d hours, want 2 and 2", quiet, hours)
	}
	if want := 2*gridHourRem + 2*gridQuietRem; a.Height != want {
		t.Errorf("the grid stands %.2frem, want %.2frem", a.Height, want)
	}

	// An hour that was read in keeps its full height, and a session inside
	// it is placed proportionally.
	if got, want := a.at(3*60+20), gridHourRem/3; math.Abs(got-want) > 0.001 {
		t.Errorf("03:20 sits at %.3frem, want %.3frem", got, want)
	}
	if got, want := a.at(9*60), gridHourRem+gridQuietRem; math.Abs(got-want) > 0.001 {
		t.Errorf("09:00 sits at %.3frem, want %.3frem", got, want)
	}
	if got := a.at(22 * 60); got != a.Height {
		t.Errorf("the grid's last hour sits at %.3frem, want the foot %.3frem", got, a.Height)
	}
}
