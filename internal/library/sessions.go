package library

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Session is one stretch of reading on an item. Minutes are the fundamental
// unit; positions are in the item's SizeUnit and optional.
type Session struct {
	ID                   string     `json:"id"`
	ItemID               string     `json:"item_id"`
	StartedAt            time.Time  `json:"started_at"`
	EndedAt              *time.Time `json:"ended_at"`       // nil while running
	PositionStart        *int       `json:"position_start"` // both nil when progress was not recorded
	PositionEnd          *int       `json:"position_end"`
	Note                 string     `json:"note"`
	EnteredRetroactively bool       `json:"entered_retroactively"`
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
// run at a time. The session picks up where the item's last recorded
// position left off.
func (s *Service) StartSession(ctx context.Context, itemID string) (*Session, error) {
	session := &Session{ID: newID(), ItemID: itemID, StartedAt: s.now()}
	err := s.store.Tx(ctx, func(r Repo) error {
		if err := requireInProgress(r, itemID); err != nil {
			return err
		}
		if err := requireNoneRunning(r); err != nil {
			return err
		}
		start, err := lastPosition(r, itemID)
		if err != nil {
			return err
		}
		session.PositionStart = &start
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
// done away from the timer. reached is the position it got to; when given,
// the session starts from the item's last recorded position, as the timer
// does. The range must end by now and may not overlap the running session.
func (s *Service) AddRetroactiveSession(ctx context.Context, itemID string, start, end time.Time, reached *int, note string) (*Session, error) {
	if !end.After(start) {
		return nil, ErrInvalidRange
	}
	if end.After(s.now()) {
		return nil, ErrInFuture
	}
	end = end.UTC()
	session := &Session{
		ID:                   newID(),
		ItemID:               itemID,
		StartedAt:            start.UTC(),
		EndedAt:              &end,
		PositionEnd:          reached,
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
		if reached != nil {
			from, err := lastPosition(r, itemID)
			if err != nil {
				return err
			}
			session.PositionStart = &from
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

// lastPosition is where the item's reading currently stands.
func lastPosition(r Repo, itemID string) (int, error) {
	history, err := r.ListSessionsByItem(itemID)
	if err != nil {
		return 0, err
	}
	return positionAfter(history), nil
}

// positionAfter is the end position of the most recent session that recorded
// one, or 0 when none has. history is oldest first.
func positionAfter(history []Session) int {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].PositionEnd != nil {
			return *history[i].PositionEnd
		}
	}
	return 0
}

// Reading is an in_progress item together with where its reading stands.
type Reading struct {
	Item       Item
	Position   int        // the last recorded position, in the item's SizeUnit
	LastReadAt *time.Time // when its most recent session started; nil before the first
	Remaining  Estimate   // time left, when it can be estimated
	Stalled    bool       // no reading for settings.StallDays
}

// Reading lists the items being read, most recently read first. Items with
// no session yet come last, newest started first.
func (s *Service) Reading(ctx context.Context) ([]Reading, error) {
	var out []Reading
	err := s.store.Tx(ctx, func(r Repo) error {
		items, err := r.ListItems()
		if err != nil {
			return err
		}
		sessions, err := r.ListSessions()
		if err != nil {
			return err
		}
		settings, err := r.GetSettings()
		if err != nil {
			return err
		}
		now := s.now()
		history := sessionsByItem(sessions)
		bands := BandPaces(items, sessions, now.Add(-time.Duration(settings.PaceWindowDays)*24*time.Hour))
		for _, item := range items {
			if item.State != StateInProgress {
				continue
			}
			h := history[item.ID]
			entry := Reading{
				Item:      item,
				Position:  positionAfter(h),
				Remaining: TimeRemaining(item, h, bands, *settings),
			}
			if n := len(h); n > 0 {
				entry.LastReadAt = &h[n-1].StartedAt
			}
			entry.Stalled = stalled(item, entry.LastReadAt, now, settings.StallDays)
			out = append(out, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Read items first, most recent first; then the never read, newest started first.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.LastReadAt == nil) != (b.LastReadAt == nil) {
			return a.LastReadAt != nil
		}
		if a.LastReadAt == nil {
			return a.Item.StartedAt.After(*b.Item.StartedAt)
		}
		return a.LastReadAt.After(*b.LastReadAt)
	})
	return out, nil
}

// sessionsByItem groups sessions by item, keeping their order.
func sessionsByItem(sessions []Session) map[string][]Session {
	out := map[string][]Session{}
	for _, s := range sessions {
		out[s.ItemID] = append(out[s.ItemID], s)
	}
	return out
}
