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
// Its target starts at 100% of the baselines.
type SpeedRamp struct {
	StartedOn        time.Time  `json:"started_on"` // a calendar day
	IncrementPercent int        `json:"increment_percent"`
	CeilingPercent   int        `json:"ceiling_percent"`
	StoppedOn        *time.Time `json:"stopped_on"` // a calendar day; nil while it runs
}

// Baseline is a band's reference speed for the index.
type Baseline struct {
	Band      Band
	PerHour   float64
	SetOn     time.Time // the ramp's first day, or the week a new band first appeared
	FromStart bool      // measured over the pace window before the ramp
}

// IndexRow is one band's part in a week's speed index, worked through.
type IndexRow struct {
	Band     Band
	Baseline float64 // 0 when the band had none
	PerHour  float64
	Time     time.Duration
	Ratio    float64 // PerHour ÷ Baseline
	Counted  bool    // false: its baseline was set by this very week
}

// SpeedIndex is a closed week's speed index: every band compared only with
// itself, weighted by the time spent on it (spec §8.6).
type SpeedIndex struct {
	From, To time.Time
	Rows     []IndexRow
	Measured time.Duration // time in counted bands
	Index    float64       // 1.0 is the baseline; 0 when nothing counted
}

// Enough reports whether the week has the evidence to be judged.
func (x SpeedIndex) Enough() bool { return x.Measured >= MinSpeedEvidence }

// SpeedCheck is the outcome of a week-boundary check that was due.
type SpeedCheck struct {
	On       time.Time
	Index    float64
	Enough   bool
	Advanced bool
}

// SpeedRampState is where a speed ramp stands today.
type SpeedRampState struct {
	Ramp      SpeedRamp
	Running   bool      // false once stopped or ended at its ceiling
	Target    int       // percent
	ReachedOn time.Time // when it reached its ceiling; zero otherwise
	NextCheck time.Time // zero when not running
	LastCheck *SpeedCheck
	Baselines []Baseline
	LastWeek  *SpeedIndex // the last closed week since the ramp began
}

// Speed is the speed side of the plan: last week's reading speed and the
// speed ramp, if one was ever started.
type Speed struct {
	LastWeek  WeekSpeed
	Ramp      *SpeedRampState // the latest ramp; nil before the first
	StartFrom []Baseline      // what a ramp started today would compare against
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
	if ramp == nil {
		return sp
	}

	start := ramp.StartedOn
	state := &SpeedRampState{Ramp: *ramp, Running: true, Target: 100}
	state.Baselines = startBaselines(items, sessions, start, st, loc)

	since := start // day the target last changed
	for b := weekStartOf(start, st.ReviewWeekday).AddDate(0, 0, 7); !b.After(today); b = b.AddDate(0, 0, 7) {
		if ramp.StoppedOn != nil && b.After(*ramp.StoppedOn) {
			break
		}
		from := b.AddDate(0, 0, -7)
		week := bandSpeeds(items, sessions, dayStart(from, loc), dayStart(b, loc))
		index := state.index(week, from, b)
		if b.Equal(weekStart) {
			state.LastWeek = &index
		}
		if !b.Before(since.AddDate(0, 0, 7)) {
			check := SpeedCheck{On: b, Index: index.Index, Enough: index.Enough()}
			if check.Enough && index.Index*100 >= float64(state.Target) {
				state.Target = min(state.Target+ramp.IncrementPercent, ramp.CeilingPercent)
				since = b
				check.Advanced = true
			}
			state.LastCheck = &check
			if state.Target == ramp.CeilingPercent {
				state.Running, state.ReachedOn = false, b
				break
			}
		}
	}
	if ramp.StoppedOn != nil && !ramp.StoppedOn.After(today) {
		state.Running = false
	}
	if state.Running {
		state.NextCheck = weekStart.AddDate(0, 0, 7)
		for state.NextCheck.Before(since.AddDate(0, 0, 7)) {
			state.NextCheck = state.NextCheck.AddDate(0, 0, 7)
		}
	}
	sp.Ramp = state
	return sp
}

// startBaselines measures every band over the pace window before a ramp's
// first day.
func startBaselines(items map[string]Item, sessions []Session, start time.Time, st Settings, loc *time.Location) []Baseline {
	var out []Baseline
	window := start.AddDate(0, 0, -st.PaceWindowDays)
	for _, b := range sortedBands(bandSpeeds(items, sessions, dayStart(window, loc), dayStart(start, loc))) {
		if perHour, ok := b.PerHour(); ok {
			out = append(out, Baseline{Band: b.Band, PerHour: perHour, SetOn: start, FromStart: true})
		}
	}
	return out
}

// index works a closed week through the baselines. A band read for the
// first time sets its baseline from this week and counts from the next.
func (st *SpeedRampState) index(week map[Band]BandSpeed, from, to time.Time) SpeedIndex {
	x := SpeedIndex{From: from, To: to}
	var weighted float64
	for _, b := range sortedBands(week) {
		perHour, ok := b.PerHour()
		if !ok {
			continue
		}
		row := IndexRow{Band: b.Band, PerHour: perHour, Time: b.Time}
		if base, found := st.baseline(b.Band); found {
			row.Baseline, row.Ratio, row.Counted = base, perHour/base, true
			x.Measured += b.Time
			weighted += row.Ratio * b.Time.Hours()
		} else {
			st.Baselines = append(st.Baselines, Baseline{Band: b.Band, PerHour: perHour, SetOn: from})
			row.Baseline, row.Ratio = perHour, 1
		}
		x.Rows = append(x.Rows, row)
	}
	if x.Measured > 0 {
		x.Index = weighted / x.Measured.Hours()
	}
	return x
}

func (st *SpeedRampState) baseline(band Band) (float64, bool) {
	for _, b := range st.Baselines {
		if b.Band == band {
			return b.PerHour, true
		}
	}
	return 0, false
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
