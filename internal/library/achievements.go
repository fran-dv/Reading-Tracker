package library

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// MomentDays is how long a big achievement waits on Home to be seen.
const MomentDays = 30

// AchievementKind names what was reached (spec §6.10).
type AchievementKind string

const (
	BookFinished    AchievementKind = "book-finished"
	SetItemFinished AchievementKind = "set-item-finished"
	CampaignHalfway AchievementKind = "campaign-halfway"
	CampaignMet     AchievementKind = "campaign-met"
	HoursRampStep   AchievementKind = "hours-step"
	HoursRampTop    AchievementKind = "hours-top"
	SpeedRampStep   AchievementKind = "speed-step"
	SpeedRampTop    AchievementKind = "speed-top"
)

// Achievement is something the user set out to reach, reached. It is
// derived by replay; only its Key is ever stored, once its moment is closed.
// Fields that do not belong to its kind are zero.
type Achievement struct {
	Kind AchievementKind
	Key  string    // stable across replays: kind, subject and day
	On   time.Time // the calendar day it happened

	// A finished book, and for campaign kinds the book that reached it.
	Item *Item
	// A finished book: each active campaign it counted toward.
	Toward []Counted
	// A campaign halfway or met, or an item finished in a set, with what
	// the campaign had counted by then and what it asks for in all.
	Campaign *Campaign
	Count    int
	Target   int
	Pages    int           // the book's pages, or the counted books' pages by then
	Time     time.Duration // reading on the book, or on the counted books by then
	Days     int           // the book from start to finish, or the campaign from its start, both days counted
	First    *Item         // a campaign met: its first and last counted books
	Last     *Item
	Ahead    int // at halfway: books ahead of (+) or behind (−) an even pace to the deadline

	From, To int       // a ramp step: minutes per day, or percent
	Began    time.Time // a ramp at its top: when the ramp began
	Start    int       // and the minutes or percent it began at
	Holds    int       // checks it held at on the way
}

// Counted is a campaign a book counted toward, its count with that book,
// and what the campaign asks for.
type Counted struct {
	Campaign *Campaign
	Count    int
	Target   int
}

// Big reports whether the achievement earns a moment on Home.
func (a Achievement) Big() bool {
	switch a.Kind {
	case CampaignHalfway, CampaignMet, HoursRampTop, SpeedRampTop:
		return true
	}
	return false
}

// MomentSeen is an achievement's moment closed on Home (spec §2.9).
type MomentSeen struct {
	Key    string    `json:"key"`
	SeenAt time.Time `json:"seen_at"`
}

// Achievements lists everything reached, newest first.
func (s *Service) Achievements(ctx context.Context) ([]Achievement, error) {
	var out []Achievement
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		out, err = sn.achievements()
		return err
	})
	return out, err
}

// CloseMoment records an achievement's moment as seen, so Home stops
// showing it. Closing twice is harmless.
func (s *Service) CloseMoment(ctx context.Context, key string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		return r.PutMomentSeen(&MomentSeen{Key: key, SeenAt: s.now()})
	})
}

// moment is the newest big achievement of the last MomentDays days whose
// moment was not closed; nil when there is none.
func (sn *snapshot) moment(r Repo) (*Achievement, error) {
	all, err := sn.achievements()
	if err != nil {
		return nil, err
	}
	seen, err := r.ListMomentsSeen()
	if err != nil {
		return nil, err
	}
	closed := map[string]bool{}
	for _, m := range seen {
		closed[m.Key] = true
	}
	loc, err := sn.settings.Location()
	if err != nil {
		return nil, err
	}
	oldest := dayOf(sn.now, loc).AddDate(0, 0, -MomentDays)
	for _, a := range all {
		if a.Big() && !closed[a.Key] && !a.On.Before(oldest) {
			return &a, nil
		}
	}
	return nil, nil
}

func (sn *snapshot) achievements() ([]Achievement, error) {
	loc, err := sn.settings.Location()
	if err != nil {
		return nil, err
	}
	var out []Achievement
	out = append(out, sn.itemAchievements(loc)...)
	sc, err := sn.schedule()
	if err != nil {
		return nil, err
	}
	for _, st := range sc.steps {
		a := Achievement{Kind: HoursRampStep, On: st.On, From: st.From, To: st.To}
		if st.Top {
			a.Kind, a.Began, a.Start, a.Holds = HoursRampTop, st.Began, st.Start, st.Holds
		}
		// Keyed by the journey's first day and the value reached, which a
		// change of time zone or review weekday leaves as they are.
		a.Key = fmt.Sprintf("%s:%s:%d", a.Kind, st.Began.Format(dayFormat), st.To)
		out = append(out, a)
	}
	items := sn.itemsByID()
	for _, ramp := range endedRamps(sn.speedRamps) {
		state := replaySpeedRamp(ramp, items, sn.sessions, *sn.settings, loc, sn.now)
		target, holds := 100, 0
		for _, check := range state.Checks {
			if !check.Advanced {
				holds++
				continue
			}
			a := Achievement{Kind: SpeedRampStep, On: check.On, From: target, To: min(target+ramp.IncrementPercent, ramp.CeilingPercent)}
			target = a.To
			if check.On.Equal(state.ReachedOn) {
				a.Kind, a.Began, a.Start, a.Holds = SpeedRampTop, ramp.StartedOn, 100, holds
			}
			a.Key = fmt.Sprintf("%s:%s:%d", a.Kind, ramp.StartedOn.Format(dayFormat), a.To)
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].On.After(out[j].On) })
	return out, nil
}

// dayFormat writes a calendar day into a key.
const dayFormat = "2006-01-02"

// itemAchievements is every finished book, every item finished in a set,
// and each campaign's halfway and met, in the order they were finished.
func (sn *snapshot) itemAchievements(loc *time.Location) []Achievement {
	var finished []Item
	for _, it := range sn.items {
		if it.State == StateFinished && it.FinishedAt != nil {
			finished = append(finished, it)
		}
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].FinishedAt.Before(*finished[j].FinishedAt) })

	type tally struct {
		count, pages int
		time         time.Duration
		first        *Item
	}
	tallies := map[string]*tally{}
	var out []Achievement
	for i := range finished {
		item := &finished[i]
		day := dayOf(*item.FinishedAt, loc)
		pages, read := bookPages(*item), sn.readingTime(item.ID)
		var book *Achievement
		if item.Format == FormatBook {
			a := Achievement{Kind: BookFinished, Key: string(BookFinished) + ":" + item.ID, On: day, Item: item, Pages: pages, Time: read}
			if item.StartedAt != nil {
				a.Days = DaysBetween(dayOf(*item.StartedAt, loc), day)
			}
			book = &a
		}
		for ci := range sn.campaigns {
			c := &sn.campaigns[ci]
			target, in := sn.asks(c, item, day)
			if !in {
				continue
			}
			t := tallies[c.ID]
			if t == nil {
				t = &tally{first: item}
				tallies[c.ID] = t
			}
			t.count++
			t.pages += pages
			t.time += read
			switch {
			case c.Kind == KindSet:
				out = append(out, Achievement{Kind: SetItemFinished, Key: string(SetItemFinished) + ":" + c.ID + ":" + item.ID,
					On: day, Item: item, Campaign: c, Count: t.count, Target: target, Pages: pages, Time: read})
			case c.Active() && book != nil:
				book.Toward = append(book.Toward, Counted{Campaign: c, Count: t.count, Target: target})
			}

			reached := Achievement{Item: item, Campaign: c, On: day, Count: t.count, Target: target, Pages: t.pages, Time: t.time,
				Days: DaysBetween(c.StartedOn, day)}
			switch {
			case t.count == target:
				reached.Kind, reached.First, reached.Last = CampaignMet, t.first, item
			case target >= 4 && t.count == (target+1)/2:
				reached.Kind = CampaignHalfway
				if c.Kind == KindCount {
					total := float64(DaysBetween(c.StartedOn, c.Deadline))
					even := float64(target) * float64(reached.Days) / total
					reached.Ahead = t.count - int(math.Round(even))
				}
			default:
				continue
			}
			reached.Key = string(reached.Kind) + ":" + c.ID
			out = append(out, reached)
		}
		if book != nil {
			out = append(out, *book)
		}
	}
	return out
}

// asks reports whether an item finished on day counts toward a campaign,
// and what the campaign asked for as it stood then: its target, or the
// items the set held by that day.
func (sn *snapshot) asks(c *Campaign, item *Item, day time.Time) (target int, in bool) {
	if c.Kind == KindCount {
		return c.TargetCount, item.Format == FormatBook && counts(*c, day)
	}
	for _, m := range sn.members {
		if m.CampaignID != c.ID || m.AddedOn.After(day) {
			continue
		}
		target++
		in = in || (m.ItemID == item.ID && !day.After(c.Deadline))
	}
	return target, in
}

// counts reports whether a book finished on day counts toward c (spec §2.4).
func counts(c Campaign, day time.Time) bool {
	last := c.Deadline
	if c.EndedOn != nil && c.EndedOn.Before(last) {
		last = *c.EndedOn
	}
	return !day.Before(c.StartedOn) && !day.After(last)
}

// bookPages is a book's size in pages, or 0 when it has none in pages.
func bookPages(it Item) int {
	if it.SizeUnit != UnitPages || it.SizeValue == nil {
		return 0
	}
	return *it.SizeValue
}

// readingTime is every closed session's time on an item.
func (sn *snapshot) readingTime(itemID string) time.Duration {
	var total time.Duration
	for _, s := range sn.history[itemID] {
		total += s.Duration()
	}
	return total
}

// DaysBetween counts the calendar days from one day to another, both
// included.
func DaysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours()/24) + 1
}
