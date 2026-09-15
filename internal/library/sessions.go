package library

import (
	"context"
	"strings"
	"time"
)

// Session is one stretch of reading on an item. Minutes are the fundamental
// unit; positions are in the item's SizeUnit and optional.
type Session struct {
	ID                   string
	ItemID               string
	StartedAt            time.Time
	EndedAt              *time.Time // nil while running
	PositionStart        *int       // both nil when progress was not recorded
	PositionEnd          *int
	Note                 string
	EnteredRetroactively bool
}

// Running reports whether the session has not been stopped yet.
func (s Session) Running() bool { return s.EndedAt == nil }

// Duration is the session length, or zero while running.
func (s Session) Duration() time.Duration {
	if s.EndedAt == nil {
		return 0
	}
	return s.EndedAt.Sub(s.StartedAt)
}

// ProgressDelta is PositionEnd − PositionStart. ok is false when positions
// were not recorded; such sessions count for time but not for pace.
func (s Session) ProgressDelta() (delta int, ok bool) {
	if s.PositionStart == nil || s.PositionEnd == nil {
		return 0, false
	}
	return *s.PositionEnd - *s.PositionStart, true
}

// StartSession starts the timer on an in_progress item. Only one session may
// run at a time. The start position is the item's last recorded position.
func (s *Service) StartSession(ctx context.Context, itemID string) (*Session, error) {
	session := &Session{ID: newID(), ItemID: itemID, StartedAt: s.now()}
	err := s.store.Tx(ctx, func(r Repo) error {
		if err := requireInProgress(r, itemID); err != nil {
			return err
		}
		if err := requireNoneRunning(r); err != nil {
			return err
		}
		history, err := r.ListSessionsByItem(itemID)
		if err != nil {
			return err
		}
		session.PositionStart = lastPosition(history)
		return r.InsertSession(session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// StopSession ends a running session. end is the position reached; when it
// is nil the session records time only and its start position is dropped too.
func (s *Service) StopSession(ctx context.Context, id string, end *int, note string) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		if session, err = r.GetSession(id); err != nil {
			return err
		}
		if !session.Running() {
			return ErrInvalidTransition
		}
		now := s.now()
		session.EndedAt = &now
		session.PositionEnd = end
		if end == nil {
			session.PositionStart = nil
		}
		session.Note = strings.TrimSpace(note)
		return r.UpdateSession(session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// AddRetroactiveSession records a session that already happened, for reading
// done away from the timer. Positions must be both given or both nil. The
// range may not overlap the running session.
func (s *Service) AddRetroactiveSession(ctx context.Context, itemID string, start, end time.Time, posStart, posEnd *int, note string) (*Session, error) {
	if !end.After(start) {
		return nil, ErrInvalidRange
	}
	if (posStart == nil) != (posEnd == nil) {
		return nil, ErrInvalidPositions
	}
	end = end.UTC()
	session := &Session{
		ID:                   newID(),
		ItemID:               itemID,
		StartedAt:            start.UTC(),
		EndedAt:              &end,
		PositionStart:        posStart,
		PositionEnd:          posEnd,
		Note:                 strings.TrimSpace(note),
		EnteredRetroactively: true,
	}
	err := s.store.Tx(ctx, func(r Repo) error {
		if err := requireInProgress(r, itemID); err != nil {
			return err
		}
		running, err := r.RunningSession()
		if err != nil {
			return err
		}
		if running != nil && end.After(running.StartedAt) {
			return &SessionRunningError{ID: running.ID}
		}
		return r.InsertSession(session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// RunningSession returns the running session, or nil when there is none.
func (s *Service) RunningSession(ctx context.Context) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		session, err = r.RunningSession()
		return err
	})
	return session, err
}

// Sessions returns an item's sessions, oldest first.
func (s *Service) Sessions(ctx context.Context, itemID string) ([]Session, error) {
	var sessions []Session
	err := s.store.Tx(ctx, func(r Repo) error {
		if _, err := r.GetItem(itemID); err != nil {
			return err
		}
		var err error
		sessions, err = r.ListSessionsByItem(itemID)
		return err
	})
	return sessions, err
}

func requireInProgress(r Repo, itemID string) error {
	item, err := r.GetItem(itemID)
	if err != nil {
		return err
	}
	if item.State != StateInProgress {
		return ErrItemNotInProgress
	}
	return nil
}

func requireNoneRunning(r Repo) error {
	running, err := r.RunningSession()
	if err != nil {
		return err
	}
	if running != nil {
		return &SessionRunningError{ID: running.ID}
	}
	return nil
}

// lastPosition is the end position of the most recent session that recorded
// one, or nil. history is oldest first.
func lastPosition(history []Session) *int {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].PositionEnd != nil {
			return history[i].PositionEnd
		}
	}
	return nil
}
