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
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, earlier, earlier.Add(time.Hour), ptr(0), ptr(50), ""); err != nil {
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
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, earlier, clk.Now(), ptr(0), ptr(10), ""); err != nil {
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
		pStart     *int
		pEnd       *int
		want       error
	}{
		{"end before start", now, now.Add(-time.Minute), nil, nil, library.ErrInvalidRange},
		{"zero length", now, now, nil, nil, library.ErrInvalidRange},
		{"only start position", now.Add(-time.Hour), now, ptr(1), nil, library.ErrInvalidPositions},
		{"only end position", now.Add(-time.Hour), now, nil, ptr(1), library.ErrInvalidPositions},
		{"ok without positions", now.Add(-time.Hour), now, nil, nil, nil},
		{"ok with positions", now.Add(-3 * time.Hour), now.Add(-2 * time.Hour), ptr(1), ptr(9), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := svc.AddRetroactiveSession(ctx, item.ID, tc.start, tc.end, tc.pStart, tc.pEnd, "")
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
	if _, err := svc.AddRetroactiveSession(ctx, pool.ID, now.Add(-time.Hour), now, nil, nil, ""); !errors.Is(err, library.ErrItemNotInProgress) {
		t.Fatalf("got %v, want ErrItemNotInProgress", err)
	}
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
	_, err = svc.AddRetroactiveSession(ctx, item.ID, startedAt.Add(-time.Hour), startedAt.Add(10*time.Minute), nil, nil, "")
	if !errors.As(err, &conflict) || conflict.ID != running.ID {
		t.Fatalf("overlapping range: got %v, want SessionRunningError", err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, startedAt.Add(-2*time.Hour), startedAt, nil, nil, ""); err != nil {
		t.Fatalf("range ending exactly at the running start must be allowed: %v", err)
	}
}
