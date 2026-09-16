package library

import (
	"context"
	"sort"
	"time"
)

// TimeBucket is how much time the user has right now (spec §6.1).
type TimeBucket string

const (
	TimeQuick TimeBucket = "quick"
	TimeHour  TimeBucket = "hour"
	TimeLong  TimeBucket = "long"
)

// Moment is the shape of the time at hand. The zero value fits everything:
// long, not fried. It is never inferred from the device; the phone that logs
// is not always where the book is read.
type Moment struct {
	Time  TimeBucket
	Fried bool // hides deep material
}

// fits reports whether an item with this estimate suits the moment. An
// unknown estimate fits: the app never hides what it cannot judge. Each
// bucket is an upper bound; long hides nothing.
func (m Moment) fits(item Item, est Estimate, st Settings) bool {
	if m.Fried && item.FocusDemand == FocusDeep {
		return false
	}
	if !est.Known() {
		return true
	}
	switch m.Time {
	case TimeQuick:
		return est.Remaining <= time.Duration(st.BucketQuickMaxMin)*time.Minute
	case TimeHour:
		return est.Remaining <= time.Duration(st.BucketHourMaxMin)*time.Minute
	}
	return true
}

// Pick is a shortlisted pool item as Home offers it.
type Pick struct {
	Item      Item
	ShelfName string
	Remaining Estimate // time to read it whole, when it can be estimated
	Fits      bool     // suits the moment Home was asked for
}

// HomeView is the home screen (spec §6.1): where the discipline stands,
// what is being read, and what to pick next.
type HomeView struct {
	Schedule Schedule
	Speed    Speed
	Reading  []Reading
	Picks    []Pick
}

// Home gathers the home screen for a moment. In-progress items are never
// hidden by the moment, only marked; picks that do not fit are left out.
func (s *Service) Home(ctx context.Context, m Moment) (*HomeView, error) {
	var view *HomeView
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		picks, err := sn.picks(r, m)
		if err != nil {
			return err
		}
		schedule, err := sn.schedule()
		if err != nil {
			return err
		}
		speed, err := sn.speed()
		if err != nil {
			return err
		}
		view = &HomeView{Schedule: schedule, Speed: speed, Reading: sn.reading(m), Picks: picks}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// SetShortlist puts a pool or in_progress item on the shortlist, or takes it
// off (spec §5.2). Finished items keep whatever flag they had.
func (s *Service) SetShortlist(ctx context.Context, id string, on bool) (*Item, error) {
	return s.transition(ctx, id, func(_ Repo, it *Item) error {
		if it.State != StatePool && it.State != StateInProgress {
			return ErrInvalidTransition
		}
		it.OnShortlist = on
		return nil
	})
}

// snapshot is everything the derived quantities are computed from, read
// once per request.
type snapshot struct {
	items       []Item
	sessions    []Session
	history     map[string][]Session
	days        []ActiveDays
	commitments []Commitment
	speedRamps  []SpeedRamp
	settings    *Settings
	bands       Paces
	now         time.Time
}

func (s *Service) load(r Repo) (*snapshot, error) {
	items, err := r.ListItems()
	if err != nil {
		return nil, err
	}
	sessions, err := r.ListSessions()
	if err != nil {
		return nil, err
	}
	days, err := r.ListActiveDays()
	if err != nil {
		return nil, err
	}
	commitments, err := r.ListCommitments()
	if err != nil {
		return nil, err
	}
	speedRamps, err := r.ListSpeedRamps()
	if err != nil {
		return nil, err
	}
	settings, err := r.GetSettings()
	if err != nil {
		return nil, err
	}
	now := s.now()
	return &snapshot{
		items:       items,
		sessions:    sessions,
		history:     sessionsByItem(sessions),
		days:        days,
		commitments: commitments,
		speedRamps:  speedRamps,
		settings:    settings,
		bands:       BandPaces(items, sessions, now.Add(-time.Duration(settings.PaceWindowDays)*24*time.Hour)),
		now:         now,
	}, nil
}

// reading lists the in_progress items, most recently read first. Items with
// no session yet come last, newest started first.
func (sn *snapshot) reading(m Moment) []Reading {
	var out []Reading
	for _, item := range sn.items {
		if item.State != StateInProgress {
			continue
		}
		h := sn.history[item.ID]
		entry := Reading{
			Item:      item,
			Position:  positionAfter(h),
			Remaining: TimeRemaining(item, h, sn.bands, *sn.settings),
		}
		if n := len(h); n > 0 {
			entry.LastReadAt = &h[n-1].StartedAt
		}
		entry.Stalled = stalled(item, entry.LastReadAt, sn.now, sn.settings.StallDays)
		entry.Fits = m.fits(item, entry.Remaining, *sn.settings)
		out = append(out, entry)
	}
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
	return out
}

// picks lists the shortlisted pool items that fit the moment, by shelf
// order, then the slot they hold on their home shelf, then title.
func (sn *snapshot) picks(r Repo, m Moment) ([]Pick, error) {
	shelves, err := r.ListShelves()
	if err != nil {
		return nil, err
	}
	order := make(map[string]int, len(shelves))
	name := make(map[string]string, len(shelves))
	for i, sh := range shelves {
		order[sh.ID], name[sh.ID] = i, sh.Name
	}

	type key struct{ shelf, slot int }
	keys := map[string]key{}
	var out []Pick
	for _, item := range sn.items {
		if item.State != StatePool || !item.OnShortlist {
			continue
		}
		est := TimeRemaining(item, sn.history[item.ID], sn.bands, *sn.settings)
		if !m.fits(item, est, *sn.settings) {
			continue
		}
		ranks, err := r.ListRanksForItem(item.ID)
		if err != nil {
			return nil, err
		}
		slot := MaxSlots + 1 // unranked sorts after every slot
		for _, rk := range ranks {
			if rk.ShelfID == item.ShelfID {
				slot = rk.Slot
			}
		}
		keys[item.ID] = key{order[item.ShelfID], slot}
		out = append(out, Pick{Item: item, ShelfName: name[item.ShelfID], Remaining: est, Fits: true})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := keys[out[i].Item.ID], keys[out[j].Item.ID]
		if a.shelf != b.shelf {
			return a.shelf < b.shelf
		}
		if a.slot != b.slot {
			return a.slot < b.slot
		}
		return out[i].Item.Title < out[j].Item.Title
	})
	return out, nil
}

// speed measures last week and replays the speed ramp in the configured timezone.
func (sn *snapshot) speed() (Speed, error) {
	loc, err := sn.settings.Location()
	if err != nil {
		return Speed{}, err
	}
	return ReplaySpeed(sn.itemsByID(), sn.sessions, sn.speedRamps, *sn.settings, loc, sn.now), nil
}

func (sn *snapshot) itemsByID() map[string]Item {
	out := make(map[string]Item, len(sn.items))
	for _, it := range sn.items {
		out[it.ID] = it
	}
	return out
}

// schedule replays the plan in the configured timezone.
func (sn *snapshot) schedule() (Schedule, error) {
	loc, err := sn.settings.Location()
	if err != nil {
		return Schedule{}, err
	}
	return ReplaySchedule(sn.days, sn.commitments, sn.sessions, sn.settings.ReviewWeekday, loc, sn.now), nil
}

// loggedBetween is the reading time that fell inside [from, to). Sessions
// are clipped to the window; the running one counts up to now.
func loggedBetween(sessions []Session, from, to, now time.Time) time.Duration {
	var total time.Duration
	for _, s := range sessions {
		end := now
		if s.EndedAt != nil {
			end = *s.EndedAt
		}
		start := s.StartedAt
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		if end.After(start) {
			total += end.Sub(start)
		}
	}
	return total
}
