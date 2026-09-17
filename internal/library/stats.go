package library

import (
	"context"
	"sort"
	"time"
)

// Stats is the stats screen (spec §6.5): hours against the target week by
// week and day by day, what was owed over time, completions by month, pace
// every way it is measured, and the campaign's count over time.
type Stats struct {
	Weeks  []StatsWeek // the last statsWeeks weeks with anything in them, oldest first
	Days   []DaySheet  // the last statsDays days, oldest first
	Owed   []DaySheet  // every closed planned day in the last statsDays×3 days, oldest first
	Months []StatsMonth

	Pace      WeekSpeed // everything read over the pace window, with its mix
	Bands     []StatsBand
	ItemPaces []StatsItemPace

	Campaign *CampaignState
	Counted  []time.Time // the day each book the campaign counted was finished, in order
	Today    time.Time   // calendar day
}

const (
	statsWeeks  = 16
	statsDays   = 28
	statsMonths = 12
)

// StatsWeek is one week's reading against its target.
type StatsWeek struct {
	Start  time.Time
	Logged time.Duration
	Target int // minutes; 0 without a plan
}

// StatsMonth is what was completed in one calendar month.
type StatsMonth struct {
	Month     time.Time // its first day
	Books     int
	Other     int
	Reference int
}

// StatsBand is a band's pace: the median of its items' paces (spec §7.2).
type StatsBand struct {
	Band  Band
	Pace  float64 // units per hour
	Items int
	Time  time.Duration // positioned reading behind it
}

// StatsItemPace is one item's own pace over the pace window.
type StatsItemPace struct {
	Item Item
	Pace float64
	Time time.Duration
}

// Stats gathers the stats screen.
func (s *Service) Stats(ctx context.Context) (*Stats, error) {
	var out *Stats
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		out, err = sn.stats()
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (sn *snapshot) stats() (*Stats, error) {
	loc, err := sn.settings.Location()
	if err != nil {
		return nil, err
	}
	sc, err := sn.schedule()
	if err != nil {
		return nil, err
	}
	st := &Stats{Today: sc.Today}

	// Hours: week by week back from this one, then the last days.
	for i := statsWeeks - 1; i >= 0; i-- {
		start := sc.WeekStart.AddDate(0, 0, -7*i)
		w := sn.week(sc, start, loc)
		if !w.WeekStart.Equal(start) {
			continue // before the first week with anything in it
		}
		st.Weeks = append(st.Weeks, StatsWeek{Start: w.WeekStart, Logged: w.Logged, Target: w.Target})
	}
	planned := map[time.Time]DaySheet{}
	for _, p := range sc.planned {
		planned[p.Day] = p
	}
	for i := statsDays - 1; i >= 0; i-- {
		d := sc.Today.AddDate(0, 0, -i)
		sheet, ok := planned[d]
		if !ok {
			sheet = DaySheet{Day: d, Logged: loggedBetween(sn.sessions, dayStart(d, loc), dayStart(d.AddDate(0, 0, 1), loc), sn.now)}
		}
		st.Days = append(st.Days, sheet)
	}
	owedFrom := sc.Today.AddDate(0, 0, -3*statsDays)
	for _, p := range sc.planned {
		if p.Closed && !p.Day.Before(owedFrom) {
			st.Owed = append(st.Owed, p)
		}
	}

	// Completions by month, the last statsMonths of them.
	thisMonth := time.Date(sc.Today.Year(), sc.Today.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := statsMonths - 1; i >= 0; i-- {
		st.Months = append(st.Months, StatsMonth{Month: thisMonth.AddDate(0, -i, 0)})
	}
	for _, it := range sn.items {
		if it.FinishedAt == nil || (it.State != StateFinished && it.State != StateReference) {
			continue
		}
		d := dayOf(*it.FinishedAt, loc)
		i := (d.Year()-thisMonth.Year())*12 + int(d.Month()-thisMonth.Month()) + statsMonths - 1
		if i < 0 || i >= statsMonths {
			continue
		}
		switch {
		case it.State == StateReference:
			st.Months[i].Reference++
		case it.Format == FormatBook:
			st.Months[i].Books++
		default:
			st.Months[i].Other++
		}
	}

	// Pace, every way it is measured, over the pace window.
	since := sn.now.Add(-time.Duration(sn.settings.PaceWindowDays) * 24 * time.Hour)
	items := sn.itemsByID()
	bands := bandSpeeds(items, sn.sessions, since, sn.now)
	st.Pace = weekSpeed(bands, dayOf(since, loc), sc.Today.AddDate(0, 0, 1), sn.settings.WordsPerPage)
	perBand := map[Band]*StatsBand{}
	for _, it := range sn.items {
		var recent []Session
		for _, s := range sn.history[it.ID] {
			if !s.StartedAt.Before(since) {
				recent = append(recent, s)
			}
		}
		pace, ok := ItemPace(recent)
		if !ok || it.SizeUnit == UnitMinutes {
			continue
		}
		t := positionedTime(recent)
		st.ItemPaces = append(st.ItemPaces, StatsItemPace{Item: it, Pace: pace, Time: t})
		b := perBand[it.Band()]
		if b == nil {
			b = &StatsBand{Band: it.Band()}
			perBand[it.Band()] = b
		}
		b.Items++
		b.Time += t
	}
	for band, pace := range sn.bands {
		if b := perBand[band]; b != nil {
			b.Pace = pace
			st.Bands = append(st.Bands, *b)
		}
	}
	sort.Slice(st.Bands, func(i, j int) bool { return st.Bands[i].Time > st.Bands[j].Time })
	sort.Slice(st.ItemPaces, func(i, j int) bool { return st.ItemPaces[i].Time > st.ItemPaces[j].Time })

	// The campaign's count over time.
	if st.Campaign, err = sn.campaign(); err != nil {
		return nil, err
	}
	if cs := st.Campaign; cs != nil {
		for _, a := range sn.bookAchievements(loc) {
			if a.Kind == BookFinished && counts(cs.Campaign, a.On) {
				st.Counted = append(st.Counted, a.On)
			}
		}
	}
	return st, nil
}
