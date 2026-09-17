package library

import (
	"context"
	"slices"
	"sort"
	"time"
)

// Record is the long view of what the reading has reached (spec §6.11):
// every goal met or missed, year by year, the totals and the bests.
type Record struct {
	Since time.Time // the day of the first session; zero before any
	Books int       // finished books
	Other int       // other finished items
	Pages int       // pages of the finished books
	Time  time.Duration
	Days  int // days anything was read on

	Campaigns  []CampaignRecord // newest first
	HoursRamps []HoursJourney   // newest first
	SpeedRamps []SpeedJourney   // newest first
	Years      []YearRecord     // newest first
	Bests      Bests
}

// CampaignRecord is a campaign and how it went.
type CampaignRecord struct {
	State     CampaignState
	HalfwayOn time.Time // zero unless it got there
	MetOn     time.Time // zero unless it was met
}

// SpeedJourney is one speed ramp and how far it got.
type SpeedJourney struct {
	Ramp      SpeedRamp
	Target    int       // percent it got to
	Checks    int       // checks that were due
	Holds     int       // checks it held at
	ReachedOn time.Time // zero unless it reached its ceiling
	Running   bool
}

// YearRecord is one calendar year of reading.
type YearRecord struct {
	Year  int
	Books int
	Other int
	Pages int
	Time  time.Duration
	Goals int // campaigns met and ramps at their top
}

// Bests are plain facts from the history, never scores or runs of days.
type Bests struct {
	LongestBook *Item // the finished book with the most pages

	Day      time.Time // the day with the most reading, and how much
	DayTime  time.Duration
	Week     time.Time // the week, by its first day, with the most reading
	WeekTime time.Duration

	Index     float64   // the best closed-week speed index with enough evidence; 0 without
	IndexWeek time.Time // the week it was measured
}

// Record gathers the record.
func (s *Service) Record(ctx context.Context) (*Record, error) {
	var rec *Record
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		rec, err = sn.record()
		return err
	})
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (sn *snapshot) record() (*Record, error) {
	loc, err := sn.settings.Location()
	if err != nil {
		return nil, err
	}
	sc, err := sn.schedule()
	if err != nil {
		return nil, err
	}
	all, err := sn.achievements()
	if err != nil {
		return nil, err
	}
	rec := &Record{}
	years := map[int]*YearRecord{}
	year := func(y int) *YearRecord {
		if years[y] == nil {
			years[y] = &YearRecord{Year: y}
		}
		return years[y]
	}

	// Reading, day by day: totals, years and the best day and week.
	perDay := map[time.Time]time.Duration{}
	for _, s := range sn.sessions {
		if s.Running() {
			continue
		}
		for d := dayOf(s.StartedAt, loc); !dayStart(d, loc).After(*s.EndedAt); d = d.AddDate(0, 0, 1) {
			if t := loggedBetween([]Session{s}, dayStart(d, loc), dayStart(d.AddDate(0, 0, 1), loc), sn.now); t > 0 {
				perDay[d] += t
			}
		}
	}
	perWeek := map[time.Time]time.Duration{}
	for d, t := range perDay {
		rec.Time += t
		year(d.Year()).Time += t
		week := weekStartOf(d, sn.settings.ReviewWeekday)
		perWeek[week] += t
		if t > rec.Bests.DayTime || (t == rec.Bests.DayTime && d.Before(rec.Bests.Day)) {
			rec.Bests.Day, rec.Bests.DayTime = d, t
		}
		if rec.Since.IsZero() || d.Before(rec.Since) {
			rec.Since = d
		}
	}
	rec.Days = len(perDay)
	for w, t := range perWeek {
		if t > rec.Bests.WeekTime || (t == rec.Bests.WeekTime && w.Before(rec.Bests.Week)) {
			rec.Bests.Week, rec.Bests.WeekTime = w, t
		}
	}

	// What was finished.
	for i := range sn.items {
		it := &sn.items[i]
		if it.State != StateFinished || it.FinishedAt == nil {
			continue
		}
		y := year(dayOf(*it.FinishedAt, loc).Year())
		if it.Format != FormatBook {
			rec.Other++
			y.Other++
			continue
		}
		pages := bookPages(*it)
		rec.Books++
		rec.Pages += pages
		y.Books++
		y.Pages += pages
		if pages > 0 && (rec.Bests.LongestBook == nil || pages > bookPages(*rec.Bests.LongestBook)) {
			rec.Bests.LongestBook = it
		}
	}

	// Goals: campaigns, and both kinds of ramp.
	reached := map[string]time.Time{}
	for _, a := range all {
		switch a.Kind {
		case CampaignHalfway, CampaignMet:
			reached[string(a.Kind)+":"+a.Campaign.ID] = a.On
			if a.Kind == CampaignMet {
				year(a.On.Year()).Goals++
			}
		case HoursRampTop, SpeedRampTop:
			year(a.On.Year()).Goals++
		}
	}
	items := sn.itemsByID()
	for _, c := range sn.campaigns {
		rec.Campaigns = append(rec.Campaigns, CampaignRecord{
			State:     MeasureCampaign(c, items, sn.sessions, *sn.settings, loc, sn.now),
			HalfwayOn: reached[string(CampaignHalfway)+":"+c.ID],
			MetOn:     reached[string(CampaignMet)+":"+c.ID],
		})
	}
	slices.Reverse(rec.Campaigns)
	rec.HoursRamps = append(rec.HoursRamps, sc.journeys...)
	slices.Reverse(rec.HoursRamps)
	for _, ramp := range endedRamps(sn.speedRamps) {
		state := replaySpeedRamp(ramp, items, sn.sessions, *sn.settings, loc, sn.now)
		j := SpeedJourney{Ramp: ramp, Target: state.Target, Checks: len(state.Checks), ReachedOn: state.ReachedOn, Running: state.Running}
		for _, check := range state.Checks {
			if !check.Advanced {
				j.Holds++
			}
			if check.Enough && check.Index > rec.Bests.Index {
				rec.Bests.Index, rec.Bests.IndexWeek = check.Index, check.On.AddDate(0, 0, -7)
			}
		}
		rec.SpeedRamps = append(rec.SpeedRamps, j)
	}
	slices.Reverse(rec.SpeedRamps)

	for _, y := range years {
		rec.Years = append(rec.Years, *y)
	}
	sort.Slice(rec.Years, func(i, j int) bool { return rec.Years[i].Year > rec.Years[j].Year })
	return rec, nil
}
