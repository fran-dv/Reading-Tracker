package library_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

func inProgressItem(t *testing.T, svc *library.Service) *library.Item {
	t.Helper()
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x")
	return startItem(t, svc, item.ID)
}

func TestStartSessionRequiresInProgress(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x")
	if _, err := svc.StartSession(ctx, item.ID); !errors.Is(err, library.ErrItemNotInProgress) {
		t.Fatalf("got %v, want ErrItemNotInProgress", err)
	}
	if _, err := svc.StartSession(ctx, "nope"); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestOneRunningSession(t *testing.T) {
	svc, _ := newTestLibrary(t)
	item := inProgressItem(t, svc)
	first, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	var running *library.SessionRunningError
	if _, err := svc.StartSession(ctx, item.ID); !errors.As(err, &running) || running.ID != first.ID {
		t.Fatalf("got %v, want SessionRunningError for %s", err, first.ID)
	}
	got, err := svc.RunningSession(ctx)
	if err != nil || got == nil || got.ID != first.ID {
		t.Fatalf("RunningSession = %v, %v", got, err)
	}
}

func TestTimerSessionPositions(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)

	// Earlier reading recorded retroactively up to page 50.
	earlier := clk.Now().Add(-2 * time.Hour)
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, earlier, earlier.Add(time.Hour), ptr(50), ""); err != nil {
		t.Fatal(err)
	}

	session, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session.PositionStart == nil || *session.PositionStart != 50 {
		t.Fatalf("start position should resume at 50, got %v", session.PositionStart)
	}
	if !session.Running() || session.Duration() != 0 {
		t.Fatalf("fresh session should be running with zero duration: %+v", session)
	}

	clk.Advance(25 * time.Minute)
	stopped, err := svc.StopSession(ctx, session.ID, ptr(80), " slow chapter ")
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Duration() != 25*time.Minute {
		t.Fatalf("duration %v, want 25m", stopped.Duration())
	}
	if delta, ok := stopped.ProgressDelta(); !ok || delta != 30 {
		t.Fatalf("progress delta %d/%v, want 30", delta, ok)
	}
	if stopped.Note != "slow chapter" {
		t.Fatalf("note %q", stopped.Note)
	}
	if _, err := svc.StopSession(ctx, session.ID, nil, ""); !errors.Is(err, library.ErrInvalidTransition) {
		t.Fatalf("stopping twice: got %v, want ErrInvalidTransition", err)
	}

	sessions, err := svc.Sessions(ctx, item.ID)
	if err != nil || len(sessions) != 2 || sessions[1].ID != session.ID {
		t.Fatalf("sessions = %v, %v; want 2 oldest first", sessions, err)
	}
}

func TestStopWithoutEndDropsPositions(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	earlier := clk.Now().Add(-time.Hour)
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, earlier, clk.Now(), ptr(10), ""); err != nil {
		t.Fatal(err)
	}
	session, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	stopped, err := svc.StopSession(ctx, session.ID, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stopped.PositionStart != nil || stopped.PositionEnd != nil {
		t.Fatalf("positions should both be nil: %+v", stopped)
	}
	if _, ok := stopped.ProgressDelta(); ok {
		t.Fatal("time-only session must report no progress")
	}
	if stopped.Duration() != time.Minute {
		t.Fatalf("time-only session still counts its minutes: %v", stopped.Duration())
	}
}

func TestRetroactiveValidation(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	now := clk.Now()

	tests := []struct {
		name       string
		start, end time.Time
		want       error
	}{
		{"end before start", now, now.Add(-time.Minute), library.ErrInvalidRange},
		{"zero length", now, now, library.ErrInvalidRange},
		{"ends in the future", now.Add(-time.Hour), now.Add(time.Minute), library.ErrInFuture},
		{"ok ending now", now.Add(-time.Hour), now, nil},
		{"ok earlier", now.Add(-3 * time.Hour), now.Add(-2 * time.Hour), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := svc.AddRetroactiveSession(ctx, item.ID, tc.start, tc.end, nil, "")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if err == nil && (!s.EnteredRetroactively || s.Running()) {
				t.Fatalf("retroactive session should be flagged and closed: %+v", s)
			}
		})
	}

	shelf := newShelf(t, svc, "T")
	pool := newItem(t, svc, shelf.ID, "pool")
	if _, err := svc.AddRetroactiveSession(ctx, pool.ID, now.Add(-time.Hour), now, nil, ""); !errors.Is(err, library.ErrItemNotInProgress) {
		t.Fatalf("got %v, want ErrItemNotInProgress", err)
	}
}

// Positions resume from the last one recorded, or from 0 on a fresh item, on
// both paths. Only the position reached is ever asked for.
func TestPositionsResumeFromTheLastRecorded(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	now := clk.Now()

	first, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-3*time.Hour), now.Add(-2*time.Hour), ptr(40), "")
	if err != nil {
		t.Fatal(err)
	}
	if delta, ok := first.ProgressDelta(); !ok || delta != 40 {
		t.Fatalf("a fresh item starts at 0: delta %d/%v, want 40", delta, ok)
	}
	timeOnly, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-2*time.Hour), now.Add(-90*time.Minute), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if timeOnly.PositionStart != nil || timeOnly.PositionEnd != nil {
		t.Fatalf("no position reached means no positions at all: %+v", timeOnly)
	}
	second, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-time.Hour), now, ptr(55), "")
	if err != nil {
		t.Fatal(err)
	}
	if delta, ok := second.ProgressDelta(); !ok || delta != 15 {
		t.Fatalf("resumes past the time-only session at 40: delta %d/%v, want 15", delta, ok)
	}

	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if timer.PositionStart == nil || *timer.PositionStart != 55 {
		t.Fatalf("timer start position %v, want 55", timer.PositionStart)
	}
}

func TestReading(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	untouched := startItem(t, svc, newItem(t, svc, shelf.ID, "untouched").ID)
	clk.Advance(time.Minute)
	stale := startItem(t, svc, newItem(t, svc, shelf.ID, "stale").ID)
	fresh := startItem(t, svc, newItem(t, svc, shelf.ID, "fresh").ID)
	newItem(t, svc, shelf.ID, "still in the pool")

	now := clk.Now()
	if _, err := svc.AddRetroactiveSession(ctx, stale.ID, now.Add(-4*time.Hour), now.Add(-3*time.Hour), ptr(30), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, fresh.ID, now.Add(-2*time.Hour), now.Add(-time.Hour), nil, ""); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Reading(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Item.ID != fresh.ID || got[1].Item.ID != stale.ID || got[2].Item.ID != untouched.ID {
		t.Fatalf("order = %v, want fresh, stale, untouched", titles(got))
	}
	if got[0].Position != 0 || got[1].Position != 30 || got[2].Position != 0 {
		t.Fatalf("positions %d %d %d, want 0 30 0", got[0].Position, got[1].Position, got[2].Position)
	}
	if got[0].LastReadAt == nil || !got[0].LastReadAt.Equal(now.Add(-2*time.Hour)) || got[2].LastReadAt != nil {
		t.Fatalf("last read: %v / %v", got[0].LastReadAt, got[2].LastReadAt)
	}
}

func titles(rs []library.Reading) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Item.Title
	}
	return out
}

func TestRetroactiveCannotOverlapRunning(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	running, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	startedAt := clk.Now()
	clk.Advance(30 * time.Minute)

	var conflict *library.SessionRunningError
	_, err = svc.AddRetroactiveSession(ctx, item.ID, startedAt.Add(-time.Hour), startedAt.Add(10*time.Minute), nil, "")
	if !errors.As(err, &conflict) || conflict.ID != running.ID {
		t.Fatalf("overlapping range: got %v, want SessionRunningError", err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, startedAt.Add(-2*time.Hour), startedAt, nil, ""); err != nil {
		t.Fatalf("range ending exactly at the running start must be allowed: %v", err)
	}
}
