package library_test

import (
	"math"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// A ramp's trajectory rises at every check from its next one to its
// ceiling, and the plan projection reads it in full at the recent book
// share, pace and book size.
func TestProjectPlan(t *testing.T) {
	today := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC) // a Wednesday
	ramp := library.TrajectoryFrom(library.AllWeekdays, library.Commitment{Kind: library.CommitRamp, StartMinutes: 30, IncrementMinutes: 15, CeilingMinutes: 60}, today, time.Sunday)
	// Saved on a Wednesday, it first rises on the Sunday after a full week.
	if !ramp.NextRise.Equal(time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("next rise %v", ramp.NextRise)
	}
	if !ramp.TopOn().Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("top on %v", ramp.TopOn())
	}

	c := library.Campaign{TargetCount: 10, StartedOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Deadline: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}
	cs := library.MeasureCampaign(c, nil, nil, campaignSettings(), time.UTC, today.Add(12*time.Hour))
	p := cs.ProjectPlan(ramp, today)
	// 16–26 Sep: 11 days at 30; 27 Sep–3 Oct: 7 at 45; 4–10 Oct: 7 at 60.
	wantHours := float64(11*30+7*45+7*60) / 60
	if p == nil || math.Abs(p.BookHours-wantHours) > 1e-9 || !p.Assumed || p.Share != 1 {
		t.Fatalf("projection %+v; want %.2f book hours, all assumed books", p, wantHours)
	}
	// 17.75 h at the medium seed of 30 pages/h over 300-page books: 1.775 books.
	if p.Books != 1 || math.Abs(p.PerBook-wantHours/10) > 1e-9 || p.BookNeeds != 10 {
		t.Fatalf("books %d, per book %.2f, needs %.2f", p.Books, p.PerBook, p.BookNeeds)
	}

	fixed := library.TrajectoryFrom(library.WeekdaysOf(time.Monday), library.Commitment{Kind: library.CommitFixed, MinutesPerDay: 90}, today, time.Sunday)
	if !fixed.TopOn().IsZero() {
		t.Fatal("a fixed target has no top")
	}
	// Mondays 21 Sep, 28 Sep, 5 Oct: 4 h 30.
	if p := cs.ProjectPlan(fixed, today); math.Abs(p.BookHours-4.5) > 1e-9 {
		t.Fatalf("fixed: %.2f book hours, want 4.5", p.BookHours)
	}
}
