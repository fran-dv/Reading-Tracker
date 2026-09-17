package library

import (
	"context"
	"math"
	"time"
)

// recentDays is how far back the book page looks to say when an item
// finishes at the pace it is being read.
const recentDays = 14

// ItemPage is one item with everything its sessions say about it (spec
// §6.9): where the reading stands, how fast it goes, and when it finishes
// at the rate it has been read lately.
type ItemPage struct {
	Item       Item
	Tags       []string
	ShelfName  string
	Sessions   []Session // oldest first
	Position   int       // the furthest reached, in the item's unit
	LastReadAt *time.Time
	Total      time.Duration // all its sessions
	Days       int           // calendar days it was read on
	Pace       float64       // the item's own units per hour; 0 when unmeasured
	PaceTime   time.Duration // the positioned reading the pace rests on
	Remaining  Estimate
	Stalled    bool

	// Recent is the time a day the item got over the last recentDays days,
	// and FinishOn the day the remaining time runs out at that rate. Both
	// are zero when it was not read lately or nothing is left to estimate.
	Recent   time.Duration
	FinishOn time.Time
}

// ItemPage gathers one item's page.
func (s *Service) ItemPage(ctx context.Context, id string) (*ItemPage, error) {
	var page *ItemPage
	err := s.store.Tx(ctx, func(r Repo) error {
		item, err := r.GetItem(id)
		if err != nil {
			return err
		}
		shelf, err := r.GetShelf(item.ShelfID)
		if err != nil {
			return err
		}
		tags, err := r.ListTags(id)
		if err != nil {
			return err
		}
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		page = sn.itemPage(*item, loc)
		page.Tags, page.ShelfName = tags, shelf.Name
		return nil
	})
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (sn *snapshot) itemPage(item Item, loc *time.Location) *ItemPage {
	history := sn.history[item.ID]
	p := &ItemPage{
		Item:      item,
		Sessions:  history,
		Position:  furthestPosition(history),
		Remaining: TimeRemaining(item, history, sn.bands, *sn.settings),
	}
	if pace, ok := ItemPace(history); ok {
		p.Pace = pace
	}
	today := dayOf(sn.now, loc)
	recentFrom := dayStart(today.AddDate(0, 0, 1-recentDays), loc)
	days := map[time.Time]bool{}
	for _, s := range history {
		p.Total += elapsed(s, sn.now)
		days[dayOf(s.StartedAt, loc)] = true
		if _, ok := s.ProgressDelta(); ok {
			p.PaceTime += s.Duration()
		}
		p.Recent += loggedBetween([]Session{s}, recentFrom, sn.now, sn.now)
	}
	p.Days = len(days)
	if n := len(history); n > 0 {
		p.LastReadAt = &history[n-1].StartedAt
	}
	if item.State == StateInProgress {
		p.Stalled = stalled(item, p.LastReadAt, sn.now, sn.settings.StallDays)
	}
	p.Recent /= recentDays
	if item.State == StateInProgress && p.Recent > 0 && p.Remaining.Known() && p.Remaining.Remaining > 0 {
		daysLeft := math.Ceil(float64(p.Remaining.Remaining) / float64(p.Recent))
		p.FinishOn = today.AddDate(0, 0, int(daysLeft))
	}
	return p
}

// elapsed is how long a session has run: its duration, or up to now while
// it runs.
func elapsed(s Session, now time.Time) time.Duration {
	if s.Running() {
		return now.Sub(s.StartedAt)
	}
	return s.Duration()
}
