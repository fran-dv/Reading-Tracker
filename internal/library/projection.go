package library

import (
	"math"
	"time"
)

// Trajectory is how a daily target goes on from a day: its value, and for
// a ramp the step it rises by at each week boundary from NextRise on, up to
// its ceiling. A fixed target has no increment.
type Trajectory struct {
	Days      Weekdays
	Value     int // minutes per active day now
	Increment int
	Ceiling   int
	NextRise  time.Time // the first week boundary it can rise at
}

// TrajectoryOf is the plan in effect today, assuming a ramp rises at every
// check it can (nothing owed). ok is false before any plan.
func TrajectoryOf(sc Schedule) (t Trajectory, ok bool) {
	if !sc.Planned() {
		return Trajectory{}, false
	}
	t = Trajectory{Days: sc.Days, Value: sc.Value}
	if r := sc.Ramp; r != nil {
		t.Increment, t.Ceiling, t.NextRise = r.Increment, r.Ceiling, r.NextCheck
	}
	return t, true
}

// TrajectoryFrom is the plan a commitment saved today would start: a ramp
// rises once it has held a full week.
func TrajectoryFrom(days Weekdays, c Commitment, today time.Time, weekStart time.Weekday) Trajectory {
	t := Trajectory{Days: days, Value: c.firstValue()}
	if c.Kind == CommitRamp {
		t.Increment, t.Ceiling = c.IncrementMinutes, c.CeilingMinutes
		t.NextRise = weekStartOf(today, weekStart).AddDate(0, 0, 7)
		for t.NextRise.Before(today.AddDate(0, 0, 7)) {
			t.NextRise = t.NextRise.AddDate(0, 0, 7)
		}
	}
	return t
}

// TopOn is the day a ramp reaches its ceiling if it rises at every check;
// zero for a fixed target.
func (t Trajectory) TopOn() time.Time {
	if t.Increment == 0 || t.Value >= t.Ceiling {
		return time.Time{}
	}
	rises := int(math.Ceil(float64(t.Ceiling-t.Value) / float64(t.Increment)))
	return t.NextRise.AddDate(0, 0, 7*(rises-1))
}

// PlanProjection is where a campaign lands if the plan holds (spec §8.5):
// the daily target read in full every active day, a ramp rising at every
// check, at the recent share of reading that goes to books and the book
// pace it is measured at.
type PlanProjection struct {
	Trajectory Trajectory
	TopOn      time.Time // when a ramp reaches its ceiling; zero otherwise
	BookHours  float64   // from today to the end of the deadline
	Books      int       // finished by the deadline
	Share      float64   // of reading that goes to books, 0–1
	Assumed    bool      // no recent reading: all of it is counted as books
	PerBook    float64   // hours each book left can take under the plan
	BookNeeds  float64   // hours an average waiting book takes at the book pace

	// DueByNow is the books the plan's targets since the campaign began
	// come to, at today's share, pace and book size; set for the plan in
	// effect, not for one being typed.
	DueByNow float64
}

// ProjectPlan projects a campaign under a trajectory from today. It is nil
// for a campaign that is over or has reached its target.
func (cs CampaignState) ProjectPlan(t Trajectory, today time.Time) *PlanProjection {
	if cs.Over || cs.Reached() || cs.Required.Pace.PagesPerHour <= 0 || cs.Required.AvgPages <= 0 {
		return nil
	}
	p := &PlanProjection{Trajectory: t, TopOn: t.TopOn(), Share: 1, Assumed: true}
	if cs.Projection != nil {
		p.Share, p.Assumed = cs.Projection.BookShare, false
	}
	var minutes float64
	value, rise := t.Value, t.NextRise
	for d := today; !d.After(cs.Campaign.Deadline); d = d.AddDate(0, 0, 1) {
		if t.Increment > 0 && !d.Before(rise) {
			value = min(value+t.Increment, t.Ceiling)
			rise = rise.AddDate(0, 0, 7)
		}
		if t.Days.Has(d.Weekday()) {
			minutes += float64(value)
		}
	}
	p.BookHours = minutes / 60 * p.Share
	p.Books = cs.projectFrom(p.BookHours / cs.WeeksLeft)
	p.BookNeeds = cs.Required.AvgPages / cs.Required.Pace.PagesPerHour
	if left := cs.Required.BooksLeft; left > 0 {
		p.PerBook = p.BookHours / float64(left)
	}
	return p
}

// dueByNow is what the targets a campaign has lived under so far come to in
// books, at today's share, pace and average book (spec §8.5): a checkpoint,
// not a count.
func (cs CampaignState) dueByNow(sc Schedule, share float64) float64 {
	var minutes int
	for _, day := range sc.planned {
		if !day.Day.Before(cs.Campaign.StartedOn) && day.Day.Before(sc.Today) {
			minutes += day.Target
		}
	}
	return float64(minutes) / 60 * share * cs.Required.Pace.PagesPerHour / cs.Required.AvgPages
}
