package library

import (
	"context"
	"strings"
	"time"
)

// MaxSessionLength is the longest stretch a session may cover. Anything
// longer is a timer left running or a typo, never reading (spec §2.3).
const MaxSessionLength = 16 * time.Hour

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
	EditedAt             *time.Time `json:"edited_at"` // nil unless corrected
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
// were not recorded, or when the position went back (rereading); such
// sessions count for time but not for pace.
func (s Session) ProgressDelta() (delta int, ok bool) {
	if s.PositionStart == nil || s.PositionEnd == nil || *s.PositionEnd < *s.PositionStart {
		return 0, false
	}
	return *s.PositionEnd - *s.PositionStart, true
}

// Stop is how a running session ends.
type Stop struct {
	Reached *int      // the position reached; nil records time only
	Note    string    // optional
	At      time.Time // when reading stopped; the zero value means now
}

// Stretch is reading that has just ended, logged as an item is closed. With
// a zero End it carries only the position, for a timer running on the item.
type Stretch struct {
	Start, End time.Time
	Reached    *int // the position reached; nil records time only
}

// StartSession starts the timer on an in_progress item. Only one session may
// run at a time. The session picks up at the furthest position reached.
func (s *Service) StartSession(ctx context.Context, itemID string) (*Session, error) {
	session := &Session{ID: newID(), ItemID: itemID, StartedAt: s.now()}
	err := s.store.Tx(ctx, func(r Repo) error {
		if _, err := inProgress(r, itemID); err != nil {
			return err
		}
		running, err := r.RunningSession()
		if err != nil {
			return err
		}
		if running != nil {
			return &SessionRunningError{ID: running.ID}
		}
		history, err := r.ListSessionsByItem(itemID)
		if err != nil {
			return err
		}
		start := furthestPosition(history)
		session.PositionStart = &start
		return r.InsertSession(session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// StopSession ends a running session. Without a position reached the
// session records time only and its start position is dropped too.
func (s *Service) StopSession(ctx context.Context, id string, stop Stop) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		if session, err = r.GetSession(id); err != nil {
			return err
		}
		if !session.Running() {
			return ErrInvalidTransition
		}
		item, err := r.GetItem(session.ItemID)
		if err != nil {
			return err
		}
		return s.stop(r, item, session, stop)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// stop closes the running session in place.
func (s *Service) stop(r Repo, item *Item, session *Session, stop Stop) error {
	end := stop.At
	if end.IsZero() {
		end = s.now()
	}
	end = end.UTC()
	session.EndedAt = &end
	session.PositionEnd = stop.Reached
	if stop.Reached == nil {
		session.PositionStart = nil
	}
	session.Note = strings.TrimSpace(stop.Note)
	if err := s.check(r, item, session); err != nil {
		return err
	}
	if err := r.UpdateSession(session); err != nil {
		return err
	}
	return rechain(r, item.ID, session)
}

// AddRetroactiveSession records a session that already happened, for reading
// done away from the timer. reached is the position it got to; the session
// starts from the furthest position reached before it, wherever it falls
// among the item's sessions (spec §2.3).
func (s *Service) AddRetroactiveSession(ctx context.Context, itemID string, start, end time.Time, reached *int, note string) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		item, err := inProgress(r, itemID)
		if err != nil {
			return err
		}
		session, err = s.addRetroactive(r, item, Stretch{Start: start, End: end, Reached: reached}, note)
		return err
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// addRetroactive validates and inserts a finished stretch of reading.
func (s *Service) addRetroactive(r Repo, item *Item, st Stretch, note string) (*Session, error) {
	end := st.End.UTC()
	session := &Session{
		ID:                   newID(),
		ItemID:               item.ID,
		StartedAt:            st.Start.UTC(),
		EndedAt:              &end,
		PositionEnd:          st.Reached,
		Note:                 strings.TrimSpace(note),
		EnteredRetroactively: true,
	}
	if st.Reached != nil {
		start := 0 // set properly by rechain once the session is in place
		session.PositionStart = &start
	}
	if err := s.check(r, item, session); err != nil {
		return nil, err
	}
	if err := r.InsertSession(session); err != nil {
		return nil, err
	}
	if err := rechain(r, item.ID, session); err != nil {
		return nil, err
	}
	return session, nil
}

// check refuses a closed session that could not have been read: an empty or
// reversed range, one that ends in the future or runs longer than
// MaxSessionLength, a position past the item's size, or time that another
// session already covers.
func (s *Service) check(r Repo, item *Item, session *Session) error {
	end := *session.EndedAt
	switch {
	case !end.After(session.StartedAt):
		return ErrInvalidRange
	case end.After(s.now()):
		return ErrInFuture
	case end.Sub(session.StartedAt) > MaxSessionLength:
		return ErrTooLong
	case session.PositionEnd != nil && item.SizeValue != nil && *session.PositionEnd > *item.SizeValue:
		return ErrPastEnd
	}
	others, err := r.ListSessionsBetween(session.StartedAt, end)
	if err != nil {
		return err
	}
	for _, other := range others {
		switch {
		case other.ID == session.ID:
		case other.Running():
			return &SessionRunningError{ID: other.ID}
		default:
			return &OverlapError{With: other}
		}
	}
	return nil
}

// rechain sets the start of every positioned session of an item to the
// furthest position reached before it (spec §2.3), so a session logged late
// starts where reading stood at its own time and the ones after it follow.
// changed, when not nil, is refreshed from what was stored.
func rechain(r Repo, itemID string, changed *Session) error {
	history, err := r.ListSessionsByItem(itemID)
	if err != nil {
		return err
	}
	furthest := 0
	for i := range history {
		h := &history[i]
		if h.PositionStart == nil { // time only
			continue
		}
		if *h.PositionStart != furthest {
			start := furthest
			h.PositionStart = &start
			if err := r.UpdateSession(h); err != nil {
				return err
			}
		}
		if changed != nil && h.ID == changed.ID {
			*changed = *h
		}
		if h.PositionEnd != nil {
			furthest = max(furthest, *h.PositionEnd)
		}
	}
	return nil
}

// SessionEdit is a correction to a closed session: all of it, as it should
// have been logged.
type SessionEdit struct {
	Start, End time.Time
	Reached    *int // the position reached; nil records time only
	Note       string
}

// EditSession corrects a closed session under the rules it was logged by,
// and marks it edited. A running session is stopped or discarded instead
// (ErrInvalidTransition). The item may be in any state: history can be
// fixed after a book is finished.
func (s *Service) EditSession(ctx context.Context, id string, e SessionEdit) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		if session, err = r.GetSession(id); err != nil {
			return err
		}
		if session.Running() {
			return ErrInvalidTransition
		}
		item, err := r.GetItem(session.ItemID)
		if err != nil {
			return err
		}
		start, end, now := e.Start.UTC(), e.End.UTC(), s.now()
		session.StartedAt, session.EndedAt = start, &end
		session.PositionEnd, session.PositionStart = e.Reached, nil
		if e.Reached != nil {
			zero := 0 // rechain sets it
			session.PositionStart = &zero
		}
		session.Note = strings.TrimSpace(e.Note)
		session.EditedAt = &now
		if err := s.check(r, item, session); err != nil {
			return err
		}
		if err := r.UpdateSession(session); err != nil {
			return err
		}
		return rechain(r, item.ID, session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSession removes a session, running or closed, and returns it as it
// was. The item's other sessions rechain, and everything derived replays
// without it.
func (s *Service) DeleteSession(ctx context.Context, id string) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		if session, err = r.GetSession(id); err != nil {
			return err
		}
		if err := r.DeleteSession(id); err != nil {
			return err
		}
		return rechain(r, session.ItemID, nil)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// LoggedSession is a session with the item it was read on.
type LoggedSession struct {
	Session
	Item Item
}

// SessionsBetween lists the sessions that cover any time in [from, to),
// oldest first, each with its item. A running session is included.
func (s *Service) SessionsBetween(ctx context.Context, from, to time.Time) ([]LoggedSession, error) {
	var out []LoggedSession
	err := s.store.Tx(ctx, func(r Repo) error {
		sessions, err := r.ListSessionsBetween(from, to)
		if err != nil {
			return err
		}
		items := map[string]*Item{}
		for _, session := range sessions {
			item, ok := items[session.ItemID]
			if !ok {
				if item, err = r.GetItem(session.ItemID); err != nil {
					return err
				}
				items[session.ItemID] = item
			}
			out = append(out, LoggedSession{Session: session, Item: *item})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
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

// GetSession returns one session.
func (s *Service) GetSession(ctx context.Context, id string) (*Session, error) {
	var session *Session
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		session, err = r.GetSession(id)
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

// inProgress loads an item that sessions may be recorded on.
func inProgress(r Repo, itemID string) (*Item, error) {
	item, err := r.GetItem(itemID)
	if err != nil {
		return nil, err
	}
	if item.State != StateInProgress {
		return nil, ErrItemNotInProgress
	}
	return item, nil
}

// furthestPosition is the furthest position any session in history reached,
// or 0 when none recorded one. Rereading never moves it back.
func furthestPosition(history []Session) int {
	furthest := 0
	for _, s := range history {
		if s.PositionEnd != nil {
			furthest = max(furthest, *s.PositionEnd)
		}
	}
	return furthest
}

// Reading is an in_progress item together with where its reading stands.
type Reading struct {
	Item       Item
	Position   int        // the furthest position reached, in the item's SizeUnit
	AtEnd      bool       // the position has reached the item's size
	LastReadAt *time.Time // when its most recent session started; nil before the first
	Remaining  Estimate   // time left, when it can be estimated
	Stalled    bool       // no reading for settings.StallDays
	Fits       bool       // suits the moment Home was asked for; always true here
}

// Reading lists the items being read, most recently read first. Items with
// no session yet come last, newest started first.
func (s *Service) Reading(ctx context.Context) ([]Reading, error) {
	var out []Reading
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		out = sn.reading(Moment{})
		return nil
	})
	if err != nil {
		return nil, err
	}
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
