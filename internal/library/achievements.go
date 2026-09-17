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
	// The campaign a book counted toward, or that was reached.
	Campaign *Campaign
	Count    int           // books the campaign had counted by then
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
	out = append(out, sn.bookAchievements(loc)...)
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

// bookAchievements is every finished book, and each campaign's halfway and
// met, in the order the books were finished.
func (sn *snapshot) bookAchievements(loc *time.Location) []Achievement {
	var books []Item
	for _, it := range sn.items {
		if it.Format == FormatBook && it.State == StateFinished && it.FinishedAt != nil {
			books = append(books, it)
		}
	}
	sort.Slice(books, func(i, j int) bool { return books[i].FinishedAt.Before(*books[j].FinishedAt) })

	type tally struct {
		count, pages int
		time         time.Duration
		first        *Item
	}
	tallies := map[string]*tally{}
	var out []Achievement
	for i := range books {
		book := &books[i]
		day := dayOf(*book.FinishedAt, loc)
		a := Achievement{Kind: BookFinished, Key: string(BookFinished) + ":" + book.ID, On: day, Item: book}
		a.Pages, a.Time = bookPages(*book), sn.readingTime(book.ID)
		if book.StartedAt != nil {
			a.Days = DaysBetween(dayOf(*book.StartedAt, loc), day)
		}
		for ci := range sn.campaigns {
			c := &sn.campaigns[ci]
			if !counts(*c, day) {
				continue
			}
			t := tallies[c.ID]
			if t == nil {
				t = &tally{first: book}
				tallies[c.ID] = t
			}
			t.count++
			t.pages += a.Pages
			t.time += a.Time
			if c.Active() {
				a.Campaign, a.Count = c, t.count
			}
			reached := Achievement{Item: book, Campaign: c, On: day, Count: t.count, Pages: t.pages, Time: t.time,
				Days: DaysBetween(c.StartedOn, day)}
			switch {
			case t.count == c.TargetCount:
				reached.Kind, reached.First, reached.Last = CampaignMet, t.first, book
			case c.TargetCount >= 4 && t.count == (c.TargetCount+1)/2:
				reached.Kind = CampaignHalfway
				total := float64(DaysBetween(c.StartedOn, c.Deadline))
				even := float64(c.TargetCount) * float64(reached.Days) / total
				reached.Ahead = t.count - int(math.Round(even))
			default:
				continue
			}
			reached.Key = string(reached.Kind) + ":" + c.ID
			out = append(out, reached)
		}
		out = append(out, a)
	}
	return out
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
