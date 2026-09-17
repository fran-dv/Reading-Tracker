package library

import (
	"context"
	"math/bits"
	"sort"
	"time"
)

// Weekdays is a set of weekdays, one bit per time.Weekday (Sunday is bit 0).
type Weekdays uint8

// AllWeekdays is every day of the week.
const AllWeekdays Weekdays = 1<<7 - 1

// WeekdaysOf builds a set from its days.
func WeekdaysOf(days ...time.Weekday) Weekdays {
	var w Weekdays
	for _, d := range days {
		w |= 1 << d
	}
	return w
}

// Has reports whether d is in the set.
func (w Weekdays) Has(d time.Weekday) bool { return w&(1<<d) != 0 }

// Count is the number of days in the set.
func (w Weekdays) Count() int { return bits.OnesCount8(uint8(w)) }

// ActiveDays is a change to the days that carry a target (spec §2.5). It
// governs from EffectiveOn until the next change.
type ActiveDays struct {
	EffectiveOn time.Time `json:"effective_on"` // a calendar day, see dayOf
	Days        Weekdays  `json:"days"`
}

// CommitmentKind says how a commitment sets the daily target.
type CommitmentKind string

const (
	CommitFixed CommitmentKind = "fixed" // the same minutes every active day
	CommitRamp  CommitmentKind = "ramp"  // an hours ramp (spec §8.4)
)

// Commitment is a decision about the daily target (spec §2.5). It governs
// from EffectiveOn until the next commitment: the latest decision wins.
// Fields that do not belong to its kind are zero.
type Commitment struct {
	EffectiveOn      time.Time      `json:"effective_on"` // a calendar day, see dayOf
	Kind             CommitmentKind `json:"kind"`
	MinutesPerDay    int            `json:"minutes_per_day"`   // fixed
	StartMinutes     int            `json:"start_minutes"`     // ramp
	IncrementMinutes int            `json:"increment_minutes"` // ramp
	CeilingMinutes   int            `json:"ceiling_minutes"`   // ramp
}

// maxMinutesPerDay bounds any daily target: a day has no more minutes.
const maxMinutesPerDay = 24 * 60

func (c Commitment) validate() error {
	switch c.Kind {
	case CommitFixed:
		if c.MinutesPerDay < 1 || c.MinutesPerDay > maxMinutesPerDay {
			return &ValidationError{"minutes_per_day", "must be between 1 and 1440"}
		}
	case CommitRamp:
		if c.StartMinutes < 1 || c.StartMinutes > maxMinutesPerDay {
			return &ValidationError{"start_minutes", "must be between 1 and 1440"}
		}
		if c.IncrementMinutes < 1 {
			return &ValidationError{"increment_minutes", "must be at least 1"}
		}
		if c.CeilingMinutes <= c.StartMinutes || c.CeilingMinutes > maxMinutesPerDay {
			return &ValidationError{"ceiling_minutes", "must be above the start and at most 1440"}
		}
	default:
		return &ValidationError{"kind", "must be fixed or ramp"}
	}
	return nil
}

// firstValue is the daily target on the commitment's first day.
func (c Commitment) firstValue() int {
	if c.Kind == CommitRamp {
		return c.StartMinutes
	}
	return c.MinutesPerDay
}

// HoursRamp is the state of a running hours ramp on a given day.
type HoursRamp struct {
	StartedOn time.Time
	Current   int // minutes per active day
	Increment int
	Ceiling   int
	NextCheck time.Time  // the first week start at which it can advance
	LastCheck *RampCheck // nil before its first check
}

// HoursStep is a rise of the daily target at a week boundary.
type HoursStep struct {
	On       time.Time // the week boundary it rose at
	From, To int       // minutes per active day
	Top      bool      // it reached the ramp's ceiling
	Began    time.Time // the day the ramp began, kept across edits that continue it
	Start    int       // minutes the ramp began at
	Holds    int       // checks it held at between its start and this rise
}

// HoursJourney is one hours ramp from the day it began, across the edits
// that continued it, to its top or to the decision that replaced it.
type HoursJourney struct {
	Began     time.Time
	Start     int // minutes per day it began at
	Value     int // minutes per day it got to
	Ceiling   int
	Holds     int
	ReachedOn time.Time // zero unless it reached its ceiling
	EndedOn   time.Time // the last day it governed, when replaced before its top
}

// RampCheck is the outcome of a week-boundary check that was due.
type RampCheck struct {
	On       time.Time
	Advanced bool // false: held, because debt was owed
}

// Schedule is where the discipline stands today, replayed from the
// decisions and the sessions (spec §8.2, §8.4).
type Schedule struct {
	Today      time.Time   // calendar day
	Days       Weekdays    // active days in effect today
	Commitment *Commitment // governing today; nil before the first decision
	Value      int         // minutes per active day in effect today
	Ramp       *HoursRamp  // the hours ramp running today, if any
	// ReachedCeilingOn is when the governing ramp reached its ceiling and
	// became a fixed target; zero otherwise.
	ReachedCeilingOn time.Time

	TargetToday    int // minutes: Value on an active day, 0 on a rest day
	LoggedToday    time.Duration
	OwedAtMidnight time.Duration // debt from the days already closed
	Owed           time.Duration // OwedAtMidnight less today's reading beyond the target

	WeekStart  time.Time // calendar day the current week began
	WeekTarget int       // minutes, summed over the week's days
	WeekLogged time.Duration
	DueSoFar   int        // minutes: the week's targets up to and including today
	Week       []DaySheet // the current week, day by day
	LastWeek   []DaySheet // the week before it, every day closed

	// planned is every day a commitment governed, from the first decision
	// to the end of the current week, in order: what History draws.
	planned []DaySheet
	// steps is every rise of an hours ramp, in order: achievements (§6.10).
	steps []HoursStep
	// journeys is every hours ramp from its start, in order: the record.
	journeys []HoursJourney
}

// DaySheet is one day of the current week.
type DaySheet struct {
	Day        time.Time // calendar day
	Planned    bool      // a commitment governed it
	Active     bool      // one of the active days
	Target     int       // minutes
	Logged     time.Duration
	Closed     bool          // before today: its debt is settled
	OwedBefore time.Duration // debt carried into the day, from days already closed
	OwedAfter  time.Duration // debt once it closed; meaningful when Closed
}

// ToGo is what is left to read today: the rest of today's target plus
// what is owed.
func (sc Schedule) ToGo() time.Duration {
	return max(0, time.Duration(sc.TargetToday)*time.Minute-sc.LoggedToday) + sc.Owed
}

// CommittedWeek is a typical week of the plan in effect today, in minutes:
// today's daily value on each active day, a ramp at its current value
// (spec §8.1). It is what required hours are compared with.
func (sc Schedule) CommittedWeek() int { return sc.Value * sc.Days.Count() }

// Planned reports whether any commitment has been made.
func (sc Schedule) Planned() bool { return sc.Commitment != nil }

// RestDay reports whether today is outside the active days of a plan.
func (sc Schedule) RestDay() bool { return sc.Planned() && !sc.Days.Has(sc.Today.Weekday()) }

// ReplaySchedule walks every calendar day from the first decision to the
// end of the current week, in loc. Closed days (before today) settle debt:
// debt = max(0, debt + target − logged). At the start of each week the
// running hours ramp is checked. Days after today only add to the week's
// target. days and commitments must be ordered by EffectiveOn.
func ReplaySchedule(days []ActiveDays, commitments []Commitment, sessions []Session, weekStart time.Weekday, loc *time.Location, now time.Time) Schedule {
	today := dayOf(now, loc)
	sc := Schedule{Today: today, WeekStart: weekStartOf(today, weekStart)}
	todayStart, tomorrowStart := dayStart(today, loc), dayStart(today.AddDate(0, 0, 1), loc)
	sc.LoggedToday = loggedBetween(sessions, todayStart, tomorrowStart, now)
	sc.WeekLogged = loggedBetween(sessions, dayStart(sc.WeekStart, loc), tomorrowStart, now)
	lastWeekStart := sc.WeekStart.AddDate(0, 0, -7)
	for i := range 14 {
		d := lastWeekStart.AddDate(0, 0, i)
		sheet := DaySheet{Day: d}
		if !d.After(today) {
			sheet.Logged = loggedBetween(sessions, dayStart(d, loc), dayStart(d.AddDate(0, 0, 1), loc), now)
		}
		if i < 7 {
			sc.LastWeek = append(sc.LastWeek, sheet)
		} else {
			sc.Week = append(sc.Week, sheet)
		}
	}
	if len(commitments) == 0 {
		return sc
	}

	var (
		debt    time.Duration
		active  Weekdays
		current *Commitment
		value   int       // minutes per active day
		since   time.Time // day value last changed
		ramp    *HoursRamp
		reached time.Time
		di, ci  int // next unread decision in days and commitments

		// Where the running ramp's journey began: an edit that saves a new
		// ramp at the current value continues it.
		began time.Time
		start int
		holds int
	)
	first := commitments[0].EffectiveOn
	if len(days) > 0 && days[0].EffectiveOn.Before(first) {
		first = days[0].EffectiveOn
	}
	weekEnd := sc.WeekStart.AddDate(0, 0, 7)

	for d := first; d.Before(weekEnd); d = d.AddDate(0, 0, 1) {
		// A week boundary checks the ramp carried in from yesterday, whose
		// last day has already closed. It can advance once its value has
		// held for a full week.
		if ramp != nil && d.Weekday() == weekStart && !d.Before(since.AddDate(0, 0, 7)) {
			check := RampCheck{On: d}
			if debt == 0 {
				from := value
				value = min(value+ramp.Increment, ramp.Ceiling)
				since = d
				check.Advanced = true
				sc.steps = append(sc.steps, HoursStep{On: d, From: from, To: value, Top: value == ramp.Ceiling, Began: began, Start: start, Holds: holds})
			} else {
				holds++
			}
			j := &sc.journeys[len(sc.journeys)-1]
			j.Value, j.Holds = value, holds
			if value == ramp.Ceiling {
				j.ReachedOn = d
			}
			ramp.LastCheck = &check
			if value == ramp.Ceiling {
				ramp, reached = nil, d
			}
		}

		// Decisions dated today take effect.
		for di < len(days) && !days[di].EffectiveOn.After(d) {
			active = days[di].Days
			di++
		}
		for ci < len(commitments) && !commitments[ci].EffectiveOn.After(d) {
			c := commitments[ci]
			ci++
			if c.firstValue() != value {
				since = d
			}
			continues := c.Kind == CommitRamp && ramp != nil && c.firstValue() == value
			if ramp != nil && !continues {
				sc.journeys[len(sc.journeys)-1].EndedOn = d.AddDate(0, 0, -1)
			}
			if c.Kind == CommitRamp && !continues {
				began, start, holds = d, c.firstValue(), 0
				sc.journeys = append(sc.journeys, HoursJourney{Began: d, Start: start, Value: start})
			}
			if c.Kind == CommitRamp {
				sc.journeys[len(sc.journeys)-1].Ceiling = c.CeilingMinutes
			}
			current, value, ramp, reached = &c, c.firstValue(), nil, time.Time{}
			if c.Kind == CommitRamp {
				ramp = &HoursRamp{StartedOn: d, Increment: c.IncrementMinutes, Ceiling: c.CeilingMinutes}
			}
		}
		if current == nil {
			continue // only active days decided so far
		}

		target := 0
		if active.Has(d.Weekday()) {
			target = value
		}
		owedBefore := debt
		day := DaySheet{Day: d, Planned: true, Active: active.Has(d.Weekday()), Target: target}
		if !d.After(today) {
			day.Logged = loggedBetween(sessions, dayStart(d, loc), dayStart(d.AddDate(0, 0, 1), loc), now)
			day.OwedBefore = owedBefore
		}
		if d.Before(today) {
			debt = max(0, debt+time.Duration(target)*time.Minute-day.Logged)
			day.Closed, day.OwedAfter = true, debt
		}
		sc.planned = append(sc.planned, day)
		if !d.Before(sc.WeekStart) {
			sc.WeekTarget += target
			if !d.After(today) {
				sc.DueSoFar += target
			}
		}
		// The two weeks drawn day by day: the one before and this one.
		if i := int(d.Sub(lastWeekStart).Hours() / 24); i >= 7 {
			sc.Week[i-7] = day
		} else if i >= 0 {
			sc.LastWeek[i] = day
		}
		if d.Equal(today) {
			sc.Days, sc.Commitment, sc.Value, sc.ReachedCeilingOn = active, current, value, reached
			sc.TargetToday, sc.OwedAtMidnight = target, debt
			if ramp != nil {
				r := *ramp
				r.Current = value
				r.NextCheck = weekEnd
				for r.NextCheck.Before(since.AddDate(0, 0, 7)) {
					r.NextCheck = r.NextCheck.AddDate(0, 0, 7)
				}
				sc.Ramp = &r
			}
		}
	}

	surplus := max(0, sc.LoggedToday-time.Duration(sc.TargetToday)*time.Minute)
	sc.Owed = max(0, sc.OwedAtMidnight-surplus)
	return sc
}

// dayOf is the calendar day of t in loc. It is carried as midnight UTC, so
// that stepping with AddDate never meets a daylight-saving change and two
// days compare with Equal.
func dayOf(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// dayStart is the first instant of a calendar day in loc.
func dayStart(day time.Time, loc *time.Location) time.Time {
	y, m, d := day.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// weekStartOf is the calendar day the week containing day began.
func weekStartOf(day time.Time, start time.Weekday) time.Time {
	back := (int(day.Weekday()) - int(start) + 7) % 7
	return day.AddDate(0, 0, -back)
}

// PlanView is the plan screen (spec §6.7).
type PlanView struct {
	Schedule Schedule
	Speed    Speed
	Campaign *CampaignState // the active campaign, or the one ended last; nil before any
}

// Plan replays the schedule as it stands now.
func (s *Service) Plan(ctx context.Context) (*PlanView, error) {
	var view *PlanView
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		sc, err := sn.schedule()
		if err != nil {
			return err
		}
		speed, err := sn.speed()
		if err != nil {
			return err
		}
		view = &PlanView{Schedule: sc, Speed: speed}
		view.Campaign, err = sn.campaign()
		return err
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// LowerTargetError is returned by SavePlan when the change would lower
// today's target and the caller has not confirmed it.
type LowerTargetError struct {
	From, To int // minutes
}

func (e *LowerTargetError) Error() string { return "the change lowers today's target" }

// SavePlan records the active days and the commitment, effective today. Only
// what differs from today's plan is written; a second save on the same day
// replaces that day's decision. A commitment equal to the running ramp at
// its current value, or to the fixed target in effect, is not a change. When
// the result lowers today's target, confirmLower must be true.
func (s *Service) SavePlan(ctx context.Context, days Weekdays, c Commitment, confirmLower bool) error {
	if days == 0 || days > AllWeekdays {
		return &ValidationError{"days", "choose at least one day"}
	}
	if err := c.validate(); err != nil {
		return err
	}
	return s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		before, err := sn.schedule()
		if err != nil {
			return err
		}
		today := before.Today

		var newDays *ActiveDays
		if !before.Planned() || before.Days != days {
			newDays = &ActiveDays{EffectiveOn: today, Days: days}
			sn.days = putDays(sn.days, *newDays)
		}
		var newCommitment *Commitment
		if !before.keeps(c) {
			c.EffectiveOn = today
			newCommitment = &c
			sn.commitments = putCommitment(sn.commitments, c)
		}

		after, err := sn.schedule()
		if err != nil {
			return err
		}
		if after.TargetToday < before.TargetToday && !confirmLower {
			return &LowerTargetError{From: before.TargetToday, To: after.TargetToday}
		}
		if newDays != nil {
			if err := r.PutActiveDays(newDays); err != nil {
				return err
			}
		}
		if newCommitment != nil {
			return r.PutCommitment(newCommitment)
		}
		return nil
	})
}

// keeps reports whether saving c would leave today's commitment as it is.
func (sc Schedule) keeps(c Commitment) bool {
	switch {
	case sc.Ramp != nil:
		return c.Kind == CommitRamp && c.StartMinutes == sc.Ramp.Current &&
			c.IncrementMinutes == sc.Ramp.Increment && c.CeilingMinutes == sc.Ramp.Ceiling
	case sc.Planned():
		return c.Kind == CommitFixed && c.MinutesPerDay == sc.Value
	}
	return false
}

// putDays adds a decision to an ordered history, replacing one on the same day.
func putDays(history []ActiveDays, d ActiveDays) []ActiveDays {
	out := []ActiveDays{d}
	for _, h := range history {
		if !h.EffectiveOn.Equal(d.EffectiveOn) {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EffectiveOn.Before(out[j].EffectiveOn) })
	return out
}

// putCommitment adds a decision to an ordered history, replacing one on the same day.
func putCommitment(history []Commitment, c Commitment) []Commitment {
	out := []Commitment{c}
	for _, h := range history {
		if !h.EffectiveOn.Equal(c.EffectiveOn) {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EffectiveOn.Before(out[j].EffectiveOn) })
	return out
}
