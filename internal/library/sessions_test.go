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
	stopped, err := svc.StopSession(ctx, session.ID, library.Stop{Reached: ptr(80), Note: " slow chapter "})
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
	if _, err := svc.StopSession(ctx, session.ID, library.Stop{}); !errors.Is(err, library.ErrInvalidTransition) {
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
	stopped, err := svc.StopSession(ctx, session.ID, library.Stop{})
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

// A session logged late starts where reading stood at its own time, and the
// sessions after it follow (spec §2.3). The audit found the late session
// starting from the item's latest position: yesterday's hour ran backwards
// and the item's pace came out at 25 pages/h instead of 40.
func TestLateSessionStartsWhereReadingStoodThen(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)

	// Today: an hour on the timer, reaching page 80.
	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Hour)
	if _, err := svc.StopSession(ctx, timer.ID, library.Stop{Reached: ptr(80)}); err != nil {
		t.Fatal(err)
	}
	// Then yesterday's hour, which reached page 50, is logged.
	yesterday := timer.StartedAt.Add(-14 * time.Hour)
	late, err := svc.AddRetroactiveSession(ctx, item.ID, yesterday, yesterday.Add(time.Hour), ptr(50), "")
	if err != nil {
		t.Fatal(err)
	}
	if delta, ok := late.ProgressDelta(); !ok || delta != 50 {
		t.Fatalf("late session delta %d/%v, want 50 from page 0", delta, ok)
	}

	sessions, err := svc.Sessions(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := *sessions[1].PositionStart; got != 50 {
		t.Fatalf("today's session now starts at %d, want 50", got)
	}
	if pace, ok := library.ItemPace(sessions); !ok || pace != 40 {
		t.Fatalf("item pace %v/%v, want 80 pages in 2 h = 40", pace, ok)
	}
}

// A position below the start is rereading: time without progress. The item
// keeps its furthest position, and the next session starts from it.
func TestRereadingAddsNoProgress(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	now := clk.Now()
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-3*time.Hour), now.Add(-2*time.Hour), ptr(100), ""); err != nil {
		t.Fatal(err)
	}
	reread, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-2*time.Hour), now.Add(-90*time.Minute), ptr(60), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reread.ProgressDelta(); ok {
		t.Fatalf("rereading must report no progress: %+v", reread)
	}
	next, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-time.Hour), now, ptr(120), "")
	if err != nil {
		t.Fatal(err)
	}
	if delta, ok := next.ProgressDelta(); !ok || delta != 20 {
		t.Fatalf("after rereading, resumes from page 100: delta %d/%v, want 20", delta, ok)
	}
	reading, err := svc.Reading(ctx)
	if err != nil || reading[0].Position != 120 {
		t.Fatalf("position %v, %v; want the furthest, 120", reading, err)
	}
}

func TestSessionsAreRefusedWhenImpossible(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	sized := startItem(t, svc, newItem(t, svc, shelf.ID, "sized", func(it *library.Item) { it.SizeValue = ptr(300) }).ID)
	other := startItem(t, svc, newItem(t, svc, shelf.ID, "other").ID)
	now := clk.Now()
	logged, err := svc.AddRetroactiveSession(ctx, sized.ID, now.Add(-2*time.Hour), now.Add(-time.Hour), nil, "")
	if err != nil {
		t.Fatal(err)
	}

	// The same hour again, on either item: reading time is one at a time.
	for _, id := range []string{sized.ID, other.ID} {
		var overlap *library.OverlapError
		_, err := svc.AddRetroactiveSession(ctx, id, now.Add(-90*time.Minute), now.Add(-30*time.Minute), nil, "")
		if !errors.As(err, &overlap) || overlap.With.ID != logged.ID {
			t.Fatalf("overlapping hour: got %v, want OverlapError with %s", err, logged.ID)
		}
	}
	if _, err := svc.AddRetroactiveSession(ctx, other.ID, now.Add(-time.Hour), now, nil, ""); err != nil {
		t.Fatalf("a session starting as another ends is fine: %v", err)
	}

	long := now.Add(-48 * time.Hour) // well before the sessions above
	if _, err := svc.AddRetroactiveSession(ctx, sized.ID, long, long.Add(library.MaxSessionLength+time.Minute), nil, ""); !errors.Is(err, library.ErrTooLong) {
		t.Fatalf("over 16 h: got %v, want ErrTooLong", err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, sized.ID, long, long.Add(library.MaxSessionLength), nil, ""); err != nil {
		t.Fatalf("exactly 16 h is allowed: %v", err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, sized.ID, now.Add(-4*time.Hour), now.Add(-3*time.Hour), ptr(301), ""); !errors.Is(err, library.ErrPastEnd) {
		t.Fatalf("page 301 of 300: got %v, want ErrPastEnd", err)
	}
}

// A timer left running can be stopped at the time reading really stopped.
func TestStopAtAnEarlierTime(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(20 * time.Hour)
	if _, err := svc.StopSession(ctx, timer.ID, library.Stop{Reached: ptr(30)}); !errors.Is(err, library.ErrTooLong) {
		t.Fatalf("stopping after 20 h: got %v, want ErrTooLong", err)
	}
	if _, err := svc.StopSession(ctx, timer.ID, library.Stop{At: timer.StartedAt.Add(-time.Minute)}); !errors.Is(err, library.ErrInvalidRange) {
		t.Fatalf("stopping before the start: got %v, want ErrInvalidRange", err)
	}
	stopped, err := svc.StopSession(ctx, timer.ID, library.Stop{Reached: ptr(30), At: timer.StartedAt.Add(45 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Duration() != 45*time.Minute || *stopped.PositionEnd != 30 {
		t.Fatalf("stopped %v at %v, want 45m at 30", stopped.Duration(), stopped.PositionEnd)
	}
}

// Finishing logs the last stretch in the same step, or stops the timer that
// is running on the item.
func TestFinishLogsTheLastStretch(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	sized := func(it *library.Item) { it.SizeValue = ptr(200) }
	logged := startItem(t, svc, newItem(t, svc, shelf.ID, "logged", sized).ID)
	timed := startItem(t, svc, newItem(t, svc, shelf.ID, "timed", sized).ID)
	now := clk.Now()

	last := &library.Stretch{Start: now.Add(-40 * time.Minute), End: now, Reached: ptr(200)}
	if _, err := svc.Finish(ctx, logged.ID, "", last); err != nil {
		t.Fatal(err)
	}
	sessions, err := svc.Sessions(ctx, logged.ID)
	if err != nil || len(sessions) != 1 || sessions[0].Duration() != 40*time.Minute || *sessions[0].PositionEnd != 200 {
		t.Fatalf("sessions %+v, %v; want the 40 min stretch to page 200", sessions, err)
	}

	timer, err := svc.StartSession(ctx, timed.ID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(25 * time.Minute)
	if _, err := svc.Reference(ctx, timed.ID, "", &library.Stretch{Reached: ptr(150)}); err != nil {
		t.Fatal(err)
	}
	if running, _ := svc.RunningSession(ctx); running != nil {
		t.Fatalf("closing an item stops its timer: %+v", running)
	}
	sessions, err = svc.Sessions(ctx, timed.ID)
	if err != nil || len(sessions) != 1 || sessions[0].ID != timer.ID || sessions[0].Duration() != 25*time.Minute || *sessions[0].PositionEnd != 150 {
		t.Fatalf("sessions %+v, %v; want the timer stopped at 25 min, page 150", sessions, err)
	}

	// A stretch that cannot be logged leaves the item as it was.
	refused := startItem(t, svc, newItem(t, svc, shelf.ID, "refused", sized).ID)
	if _, err := svc.Finish(ctx, refused.ID, "", &library.Stretch{Start: clk.Now().Add(-time.Hour), End: clk.Now(), Reached: ptr(999)}); !errors.Is(err, library.ErrPastEnd) {
		t.Fatalf("got %v, want ErrPastEnd", err)
	}
	if got, _ := svc.GetItem(ctx, refused.ID); got.State != library.StateInProgress {
		t.Fatalf("state %s, want still in progress", got.State)
	}
}

// A closed session can be corrected under the rules it was logged by; it is
// marked edited and its item's positions rechain.
func TestEditSession(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	item := startItem(t, svc, newItem(t, svc, shelf.ID, "x", func(it *library.Item) { it.SizeValue = ptr(300) }).ID)
	now := clk.Now()
	first, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-3*time.Hour), now.Add(-2*time.Hour), ptr(40), "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-time.Hour), now, ptr(70), "")
	if err != nil {
		t.Fatal(err)
	}

	clk.Advance(time.Minute)
	edited, err := svc.EditSession(ctx, first.ID, library.SessionEdit{Start: now.Add(-3 * time.Hour), End: now.Add(-150 * time.Minute), Reached: ptr(50), Note: " fixed "})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Duration() != 30*time.Minute || *edited.PositionEnd != 50 || edited.Note != "fixed" || edited.EditedAt == nil || !edited.EditedAt.Equal(clk.Now()) {
		t.Fatalf("edited %+v", edited)
	}
	sessions, _ := svc.Sessions(ctx, item.ID)
	if *sessions[1].PositionStart != 50 {
		t.Fatalf("the next session rechains from 50, got %d", *sessions[1].PositionStart)
	}

	if _, err := svc.EditSession(ctx, first.ID, library.SessionEdit{Start: now.Add(-90 * time.Minute), End: now.Add(-30 * time.Minute)}); !errors.As(err, new(*library.OverlapError)) {
		t.Fatalf("editing onto another session: got %v, want OverlapError", err)
	}
	if _, err := svc.EditSession(ctx, second.ID, library.SessionEdit{Start: now.Add(-time.Hour), End: now, Reached: ptr(301)}); !errors.Is(err, library.ErrPastEnd) {
		t.Fatalf("got %v, want ErrPastEnd", err)
	}

	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EditSession(ctx, timer.ID, library.SessionEdit{Start: now, End: clk.Now()}); !errors.Is(err, library.ErrInvalidTransition) {
		t.Fatalf("editing the running timer: got %v, want ErrInvalidTransition", err)
	}
	clk.Advance(10 * time.Minute)
	if _, err := svc.Finish(ctx, item.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EditSession(ctx, second.ID, library.SessionEdit{Start: now.Add(-time.Hour), End: now, Reached: ptr(300)}); err != nil {
		t.Fatalf("history can be fixed after finishing: %v", err)
	}
}

// Deleting a session rechains what is left; the running timer can be
// discarded the same way.
func TestDeleteSession(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	now := clk.Now()
	first, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-3*time.Hour), now.Add(-2*time.Hour), ptr(40), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-time.Hour), now, ptr(70), ""); err != nil {
		t.Fatal(err)
	}
	deleted, err := svc.DeleteSession(ctx, first.ID)
	if err != nil || deleted.ID != first.ID {
		t.Fatalf("delete = %+v, %v", deleted, err)
	}
	sessions, _ := svc.Sessions(ctx, item.ID)
	if len(sessions) != 1 || *sessions[0].PositionStart != 0 {
		t.Fatalf("sessions %+v; want one, starting from 0", sessions)
	}
	if _, err := svc.DeleteSession(ctx, first.ID); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("deleting twice: got %v, want ErrNotFound", err)
	}

	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeleteSession(ctx, timer.ID); err != nil {
		t.Fatal(err)
	}
	if running, _ := svc.RunningSession(ctx); running != nil {
		t.Fatalf("discarded timer still running: %+v", running)
	}
}

func TestSessionsBetween(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	now := clk.Now()
	before, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-5*time.Hour), now.Add(-4*time.Hour), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	across, err := svc.AddRetroactiveSession(ctx, item.ID, now.Add(-3*time.Hour), now.Add(-time.Hour), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.SessionsBetween(ctx, now.Add(-2*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != across.ID || got[1].ID != timer.ID || got[0].Item.Title != "x" {
		t.Fatalf("got %+v; want the session reaching into the window and the timer, not %s", got, before.ID)
	}
}

// Abandoning an item stops a timer running on it: a timer is never left
// running on an item nothing can be logged on.
func TestAbandonStopsTheTimer(t *testing.T) {
	svc, clk := newTestLibrary(t)
	item := inProgressItem(t, svc)
	timer, err := svc.StartSession(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(20 * time.Minute)
	if _, err := svc.Abandon(ctx, item.ID, "not for now"); err != nil {
		t.Fatal(err)
	}
	if running, _ := svc.RunningSession(ctx); running != nil {
		t.Fatalf("timer still running: %+v", running)
	}
	sessions, _ := svc.Sessions(ctx, item.ID)
	if len(sessions) != 1 || sessions[0].ID != timer.ID || sessions[0].Duration() != 20*time.Minute || sessions[0].PositionEnd != nil {
		t.Fatalf("sessions %+v; want the timer stopped at 20 min, time only", sessions)
	}
}
