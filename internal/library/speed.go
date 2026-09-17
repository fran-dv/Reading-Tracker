package library

import (
	"context"
	"errors"
	"sort"
	"time"
)

// MinSpeedEvidence is how much positioned reading a week needs before any
// speed is shown for it or a speed ramp judges it (spec §8.6).
const MinSpeedEvidence = 120 * time.Minute

// ErrNoBaseline is returned by StartSpeedRamp when no band has a measured
// speed in the pace window: a target set blind means nothing (spec §8.6).
var ErrNoBaseline = errors.New("no measured speed to start from")

// BandSpeed is one band's reading over a period: the progress and the time
// of its finished sessions that recorded positions.
type BandSpeed struct {
	Band  Band
	Units int // progress in the band's unit
	Time  time.Duration
}

// PerHour is the band's speed in its own unit. ok is false when nothing
// usable was read or the positions went backwards overall.
func (b BandSpeed) PerHour() (float64, bool) {
	if b.Units <= 0 || b.Time <= 0 {
		return 0, false
	}
	return float64(b.Units) / b.Time.Hours(), true
}

// bandSpeeds groups the positioned sessions started in [from, to) by band.
// Bands measured in minutes have no speed and are left out.
func bandSpeeds(items map[string]Item, sessions []Session, from, to time.Time) map[Band]BandSpeed {
	out := map[Band]BandSpeed{}
	for _, s := range sessions {
		delta, ok := s.ProgressDelta()
		if !ok || s.Duration() <= 0 || s.StartedAt.Before(from) || !s.StartedAt.Before(to) {
			continue
		}
		item, found := items[s.ItemID]
		if !found || item.SizeUnit == UnitMinutes {
			continue
		}
		b := out[item.Band()]
		b.Band = item.Band()
		b.Units += delta
		b.Time += s.Duration()
		out[item.Band()] = b
	}
	return out
}

// Mix is the share of reading time one kind of material took (spec §9.1).
type Mix struct {
	Format      Format
	FocusDemand FocusDemand
	Time        time.Duration
	Share       float64 // 0–1
}

// WeekSpeed is how fast the reading went over one closed week, with the
// material it was measured on.
type WeekSpeed struct {
	From, To     time.Time     // calendar days, To exclusive
	Measured     time.Duration // positioned reading in bands with a speed
	PagesPerHour float64       // words counted as pages by settings.WordsPerPage
	Mix          []Mix         // largest share first
}

// Enough reports whether the week has the evidence to show a speed.
func (w WeekSpeed) Enough() bool { return w.Measured >= MinSpeedEvidence }

func weekSpeed(bands map[Band]BandSpeed, from, to time.Time, wordsPerPage int) WeekSpeed {
	w := WeekSpeed{From: from, To: to}
	var pages float64
	mix := map[[2]string]time.Duration{}
	for _, b := range bands {
		if _, ok := b.PerHour(); !ok {
			continue
		}
		w.Measured += b.Time
		units := float64(b.Units)
		if b.Band.SizeUnit == UnitWords {
			units /= float64(wordsPerPage)
		}
		pages += units
		mix[[2]string{string(b.Band.Format), string(b.Band.FocusDemand)}] += b.Time
	}
	if w.Measured > 0 {
		w.PagesPerHour = pages / w.Measured.Hours()
	}
	for key, t := range mix {
		w.Mix = append(w.Mix, Mix{Format(key[0]), FocusDemand(key[1]), t, float64(t) / float64(w.Measured)})
	}
	sort.Slice(w.Mix, func(i, j int) bool {
		a, b := w.Mix[i], w.Mix[j]
		if a.Time != b.Time {
			return a.Time > b.Time
		}
		return string(a.Format)+string(a.FocusDemand) < string(b.Format)+string(b.FocusDemand)
	})
	return w
}

// SpeedRamp is the decision to raise reading speed week by week (spec §2.6).
// Its target starts at 100%.
type SpeedRamp struct {
	StartedOn        time.Time  `json:"started_on"` // a calendar day
	IncrementPercent int        `json:"increment_percent"`
	CeilingPercent   int        `json:"ceiling_percent"`
	StoppedOn        *time.Time `json:"stopped_on"` // a calendar day; nil while it runs
}

// Baseline is a band's speed over the pace window before a ramp would
// start: what shows a ramp has measured speed to start from.
type Baseline struct {
	Band    Band
	PerHour float64
}

// IndexRow is one item's part in a week's speed index, worked through.
type IndexRow struct {
	Item    Item
	Before  float64 // units per hour in the period compared with; 0 when not read then
	PerHour float64 // units per hour this week
	Time    time.Duration
	Ratio   float64 // PerHour ÷ Before
	Counted bool    // read in both periods; otherwise it counts from next week
}

// SpeedIndex is a week's speed index (spec §8.6): each item compared only
// with itself in the last measured week before it, weighted by the time
// spent on it, and the chain of those steps since the ramp began.
type SpeedIndex struct {
	From, To    time.Time
	BeforeFrom  time.Time // the period compared with
	BeforeTo    time.Time
	Rows        []IndexRow
	Measured    time.Duration // time on counted items this week
	Step        float64       // this week against the one before; 0 when nothing counted
	Index       float64       // the chain since the ramp began: 1.0 is where it started
	IndexBefore float64       // the index before this week's step
}

// Enough reports whether the week has the evidence to be judged.
func (x SpeedIndex) Enough() bool { return x.Measured >= MinSpeedEvidence }

// SpeedCheck is the outcome of a week-boundary check that was due.
type SpeedCheck struct {
	On       time.Time
	Target   int // percent the judged week needed
	Index    float64
	Measured time.Duration
	Enough   bool
	Advanced bool
}

// SpeedRampState is where a speed ramp stands today.
type SpeedRampState struct {
	Ramp      SpeedRamp
	Running   bool         // false once stopped or ended at its ceiling
	Target    int          // percent
	ReachedOn time.Time    // when it reached its ceiling; zero otherwise
	NextCheck time.Time    // zero when not running
	Checks    []SpeedCheck // every check that was due, oldest first
	LastCheck *SpeedCheck  // the last of Checks; nil before the first
	LastWeek  *SpeedIndex  // the last closed week since the ramp began
	ThisWeek  *SpeedIndex  // the current week so far, while running
}

// Speed is the speed side of the plan: last week's reading speed and the
// speed ramp, if one was ever started.
type Speed struct {
	LastWeek  WeekSpeed
	Ramp      *SpeedRampState // the latest ramp; nil before the first
	StartFrom []Baseline      // the speed a ramp started today would start from
}

// ReplaySpeed measures the last closed week and replays the latest speed
// ramp up to today. items is keyed by ID; ramps are ordered by StartedOn.
func ReplaySpeed(items map[string]Item, sessions []Session, ramps []SpeedRamp, st Settings, loc *time.Location, now time.Time) Speed {
	today := dayOf(now, loc)
	weekStart := weekStartOf(today, st.ReviewWeekday)
	lastFrom := weekStart.AddDate(0, 0, -7)
	sp := Speed{
		LastWeek:  weekSpeed(bandSpeeds(items, sessions, dayStart(lastFrom, loc), dayStart(weekStart, loc)), lastFrom, weekStart, st.WordsPerPage),
		StartFrom: startBaselines(items, sessions, today, st, loc),
	}
	var ramp *SpeedRamp
	for i := range ramps {
		if !ramps[i].StartedOn.After(today) {
			ramp = &ramps[i]
		}
	}
	if ramp != nil {
		sp.Ramp = replaySpeedRamp(*ramp, items, sessions, st, loc, now)
	}
	return sp
}

// endedRamps is every ramp with the day it ended: its own stop, or the day
// before the next ramp started, since starting one ends the one before.
// ramps are ordered by StartedOn.
func endedRamps(ramps []SpeedRamp) []SpeedRamp {
	out := make([]SpeedRamp, len(ramps))
	copy(out, ramps)
	for i := range out[:max(len(out)-1, 0)] {
		last := out[i+1].StartedOn.AddDate(0, 0, -1)
		if out[i].StoppedOn == nil || out[i].StoppedOn.After(last) {
			out[i].StoppedOn = &last
		}
	}
	return out
}

// period is the positioned reading of each item over a span of days, the
// span a week's index is compared with.
type period struct {
	From, To time.Time // calendar days, To exclusive
	Items    map[string]BandSpeed
	Time     time.Duration
}

// measure gathers a period's reading item by item. Items measured in
// minutes have no speed; a session far out of line with its item's own
// pace is left out, as a typo rather than reading.
func measure(items map[string]Item, sessions []Session, paces map[string]float64, from, to time.Time, loc *time.Location, until time.Time) period {
	p := period{From: from, To: to, Items: map[string]BandSpeed{}}
	end := dayStart(to, loc)
	if until.Before(end) {
		end = until
	}
	for _, s := range sessions {
		delta, ok := s.ProgressDelta()
		if !ok || s.Duration() <= 0 || s.StartedAt.Before(dayStart(from, loc)) || !s.StartedAt.Before(end) {
			continue
		}
		item, found := items[s.ItemID]
		if !found || item.SizeUnit == UnitMinutes {
			continue
		}
		if pace, ok := paces[s.ItemID]; ok {
			if speed := float64(delta) / s.Duration().Hours(); speed > 3*pace || speed < pace/3 {
				continue
			}
		}
		r := p.Items[item.ID]
		r.Band = item.Band()
		r.Units += delta
		r.Time += s.Duration()
		p.Items[item.ID] = r
		p.Time += s.Duration()
	}
	return p
}

// guardPaces is each item's pace over all its positioned sessions, for
// items with at least three of them: what a single session is checked
// against before it counts toward the index.
func guardPaces(sessions []Session) map[string]float64 {
	out := map[string]float64{}
	for id, history := range sessionsByItem(sessions) {
		n := 0
		for _, s := range history {
			if _, ok := s.ProgressDelta(); ok && s.Duration() > 0 {
				n++
			}
		}
		if pace, ok := ItemPace(history); ok && n >= 3 {
			out[id] = pace
		}
	}
	return out
}

// step compares a week with the period before it, item by item: items read
// in both count, weighted by this week's time on them.
func step(items map[string]Item, week, before period) SpeedIndex {
	x := SpeedIndex{From: week.From, To: week.To, BeforeFrom: before.From, BeforeTo: before.To}
	var weighted float64
	for _, id := range sortedItems(week.Items) {
		now := week.Items[id]
		perHour, ok := now.PerHour()
		if !ok {
			continue
		}
		row := IndexRow{Item: items[id], PerHour: perHour, Time: now.Time}
		if then, ok := before.Items[id].PerHour(); ok {
			row.Before, row.Ratio, row.Counted = then, perHour/then, true
			x.Measured += now.Time
			weighted += row.Ratio * now.Time.Hours()
		}
		x.Rows = append(x.Rows, row)
	}
	if x.Measured > 0 {
		x.Step = weighted / x.Measured.Hours()
	}
	return x
}

// sortedItems lists a period's items, most time first.
func sortedItems(m map[string]BandSpeed) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if m[ids[i]].Time != m[ids[j]].Time {
			return m[ids[i]].Time > m[ids[j]].Time
		}
		return ids[i] < ids[j]
	})
	return ids
}

// replaySpeedRamp replays one speed ramp from its first day up to today.
// Each closed week is compared with the last well-measured period before
// it — at first, the pace window before the ramp — and the index chains
// those steps (spec §8.6).
func replaySpeedRamp(ramp SpeedRamp, items map[string]Item, sessions []Session, st Settings, loc *time.Location, now time.Time) *SpeedRampState {
	today := dayOf(now, loc)
	weekStart := weekStartOf(today, st.ReviewWeekday)
	start := ramp.StartedOn
	state := &SpeedRampState{Ramp: ramp, Running: true, Target: 100}
	paces := guardPaces(sessions)
	before := measure(items, sessions, paces, start.AddDate(0, 0, -st.PaceWindowDays), start, loc, now)
	index := 1.0

	since := start // day the target last changed
	for b := weekStartOf(start, st.ReviewWeekday).AddDate(0, 0, 7); !b.After(today); b = b.AddDate(0, 0, 7) {
		if ramp.StoppedOn != nil && b.After(*ramp.StoppedOn) {
			break
		}
		week := measure(items, sessions, paces, b.AddDate(0, 0, -7), b, loc, now)
		x := step(items, week, before)
		x.IndexBefore = index
		if x.Enough() {
			index *= x.Step
		}
		x.Index = index
		if week.Time >= MinSpeedEvidence {
			before = week // the next week is compared with this one
		}
		if b.Equal(weekStart) {
			state.LastWeek = &x
		}
		if !b.Before(since.AddDate(0, 0, 7)) {
			check := SpeedCheck{On: b, Target: state.Target, Index: index, Measured: x.Measured, Enough: x.Enough()}
			if check.Enough && index*100 >= float64(state.Target)-1e-9 {
				state.Target = min(state.Target+ramp.IncrementPercent, ramp.CeilingPercent)
				since = b
				check.Advanced = true
			}
			state.Checks = append(state.Checks, check)
			if state.Target == ramp.CeilingPercent {
				state.Running, state.ReachedOn = false, b
				break
			}
		}
	}
	if ramp.StoppedOn != nil && !ramp.StoppedOn.After(today) {
		state.Running = false
	}
	if n := len(state.Checks); n > 0 {
		state.LastCheck = &state.Checks[n-1]
	}
	if state.Running {
		soFar := step(items, measure(items, sessions, paces, weekStart, weekStart.AddDate(0, 0, 7), loc, now), before)
		soFar.IndexBefore = index
		soFar.Index = index
		if soFar.Measured > 0 {
			soFar.Index = index * soFar.Step
		}
		state.ThisWeek = &soFar
		state.NextCheck = weekStart.AddDate(0, 0, 7)
		for state.NextCheck.Before(since.AddDate(0, 0, 7)) {
			state.NextCheck = state.NextCheck.AddDate(0, 0, 7)
		}
	}
	return state
}

// startBaselines measures every band over the pace window before a ramp's
// first day.
func startBaselines(items map[string]Item, sessions []Session, start time.Time, st Settings, loc *time.Location) []Baseline {
	var out []Baseline
	bands := bandSpeeds(items, sessions, dayStart(start.AddDate(0, 0, -st.PaceWindowDays), loc), dayStart(start, loc))
	for _, b := range sortedBands(bands) {
		if perHour, ok := b.PerHour(); ok {
			out = append(out, Baseline{Band: b.Band, PerHour: perHour})
		}
	}
	return out
}

// sortedBands lists bands in a stable order: most time first.
func sortedBands(m map[Band]BandSpeed) []BandSpeed {
	out := make([]BandSpeed, 0, len(m))
	for _, b := range m {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Time != out[j].Time {
			return out[i].Time > out[j].Time
		}
		a, c := out[i].Band, out[j].Band
		return string(a.Format)+string(a.FocusDemand)+string(a.SizeUnit) < string(c.Format)+string(c.FocusDemand)+string(c.SizeUnit)
	})
	return out
}

// StartSpeedRamp starts a speed ramp today at 100% of the current
// baselines, replacing any ramp before it. A second start on the same day
// replaces that day's ramp.
func (s *Service) StartSpeedRamp(ctx context.Context, incrementPercent, ceilingPercent int) error {
	if incrementPercent < 1 {
		return &ValidationError{"increment_percent", "must be at least 1"}
	}
	if ceilingPercent <= 100 || ceilingPercent > 1000 {
		return &ValidationError{"ceiling_percent", "must be above 100 and at most 1000"}
	}
	return s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		today := dayOf(sn.now, loc)
		if len(ReplaySpeed(sn.itemsByID(), sn.sessions, nil, *sn.settings, loc, sn.now).StartFrom) == 0 {
			return ErrNoBaseline
		}
		return r.PutSpeedRamp(&SpeedRamp{StartedOn: today, IncrementPercent: incrementPercent, CeilingPercent: ceilingPercent})
	})
}

// StopSpeedRamp stops the running speed ramp today. Without one it does
// nothing, so a second tap from a stale page is harmless.
func (s *Service) StopSpeedRamp(ctx context.Context) error {
	return s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		sp, err := sn.speed()
		if err != nil {
			return err
		}
		if sp.Ramp == nil || !sp.Ramp.Running {
			return nil
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		ramp, today := sp.Ramp.Ramp, dayOf(sn.now, loc)
		ramp.StoppedOn = &today
		return r.PutSpeedRamp(&ramp)
	})
}
