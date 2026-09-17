package library

import (
	"context"
	"sort"
	"time"
)

// HistoryView is one week of reading as the History screen draws it (spec
// §6.8): each day's sessions against its target, and what the week went to.
type HistoryView struct {
	WeekStart time.Time    // calendar day the week began
	Today     time.Time    // calendar day
	Earliest  time.Time    // the first week with anything to show; no going back past it
	Days      []HistoryDay // seven, in order
	Items     []ItemTime   // what the week's time went to, most first
	Logged    time.Duration
	Target    int // minutes due over the week's planned days
}

// HistoryDay is a day of the week with the sessions that started on it.
type HistoryDay struct {
	DaySheet
	Sessions []LoggedSession // oldest first
}

// ItemTime is the time a week gave one item, and the progress it made. A
// session counts whole for the day it started on.
type ItemTime struct {
	Item     Item
	Time     time.Duration
	Progress int // in the item's unit, over sessions that moved forward
	Reached  int // the furthest position its sessions got to
	Sessions int
}

// History gathers the week that contains day, a calendar day as dayOf
// carries it; the zero day is today. A week after the current one is the
// current one, and one before the first week with anything in it is that
// first week.
func (s *Service) History(ctx context.Context, day time.Time) (*HistoryView, error) {
	var view *HistoryView
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		sc, err := sn.schedule()
		if err != nil {
			return err
		}
		view = sn.week(sc, day, loc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (sn *snapshot) week(sc Schedule, day time.Time, loc *time.Location) *HistoryView {
	if day.IsZero() {
		day = sc.Today
	}
	weekStart := weekStartOf(day, sn.settings.ReviewWeekday)
	if weekStart.After(sc.WeekStart) {
		weekStart = sc.WeekStart
	}
	v := &HistoryView{WeekStart: weekStart, Today: sc.Today, Earliest: sc.WeekStart}
	if len(sn.sessions) > 0 {
		v.Earliest = weekStartOf(dayOf(sn.sessions[0].StartedAt, loc), sn.settings.ReviewWeekday)
	}
	if len(sc.planned) > 0 && sc.planned[0].Day.Before(v.Earliest) {
		v.Earliest = weekStartOf(sc.planned[0].Day, sn.settings.ReviewWeekday)
	}
	if v.WeekStart.Before(v.Earliest) {
		v.WeekStart = v.Earliest
	}

	planned := map[time.Time]DaySheet{}
	for _, p := range sc.planned {
		planned[p.Day] = p
	}
	items := sn.itemsByID()
	times := map[string]*ItemTime{}
	for i := range 7 {
		d := v.WeekStart.AddDate(0, 0, i)
		from, to := dayStart(d, loc), dayStart(d.AddDate(0, 0, 1), loc)
		hd := HistoryDay{DaySheet: DaySheet{Day: d}}
		if p, ok := planned[d]; ok {
			hd.DaySheet = p
		} else if !d.After(sc.Today) {
			hd.Logged = loggedBetween(sn.sessions, from, to, sn.now)
		}
		for _, session := range sn.sessions {
			if session.StartedAt.Before(from) || !session.StartedAt.Before(to) {
				continue
			}
			item := items[session.ItemID]
			hd.Sessions = append(hd.Sessions, LoggedSession{Session: session, Item: item})
			t := times[item.ID]
			if t == nil {
				t = &ItemTime{Item: item}
				times[item.ID] = t
			}
			t.Sessions++
			t.Time += session.Elapsed(sn.now)
			if delta, ok := session.ProgressDelta(); ok {
				t.Progress += delta
			}
			if session.PositionEnd != nil {
				t.Reached = max(t.Reached, *session.PositionEnd)
			}
		}
		v.Logged += hd.Logged
		v.Target += hd.Target
		v.Days = append(v.Days, hd)
	}
	for _, t := range times {
		v.Items = append(v.Items, *t)
	}
	sort.Slice(v.Items, func(i, j int) bool {
		if v.Items[i].Time != v.Items[j].Time {
			return v.Items[i].Time > v.Items[j].Time
		}
		return v.Items[i].Item.Title < v.Items[j].Item.Title
	})
	return v
}
