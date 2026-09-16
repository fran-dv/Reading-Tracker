package library

import (
	"sort"
	"time"
)

// Band groups items whose reading speeds are comparable (spec §7.2). The
// unit is part of the key: pages per hour and words per hour never share a
// median.
type Band struct {
	Format      Format
	FocusDemand FocusDemand
	SizeUnit    SizeUnit
}

// Band is the band the item belongs to.
func (it Item) Band() Band {
	return Band{it.Format, it.FocusDemand, it.SizeUnit}
}

// Paces is the median item pace per band, in the band's unit per hour.
type Paces map[Band]float64

// ItemPace is progress per hour over the sessions that recorded positions:
// Σ delta ÷ Σ hours. Running sessions and sessions without positions are
// skipped. ok is false when nothing usable was recorded or the positions
// went backwards overall.
func ItemPace(history []Session) (unitsPerHour float64, ok bool) {
	var delta int
	var hours float64
	for _, s := range history {
		d, ok := s.ProgressDelta()
		if !ok || s.Duration() <= 0 {
			continue
		}
		delta += d
		hours += s.Duration().Hours()
	}
	if delta <= 0 || hours <= 0 {
		return 0, false
	}
	return float64(delta) / hours, true
}

// BandPaces computes the median item pace of every band from the sessions
// started at or after since. Items of every state take part; an item counts
// only if its recent sessions yield a pace.
func BandPaces(items []Item, sessions []Session, since time.Time) Paces {
	history := sessionsByItem(sessions)
	perBand := map[Band][]float64{}
	for _, it := range items {
		var recent []Session
		for _, s := range history[it.ID] {
			if !s.StartedAt.Before(since) {
				recent = append(recent, s)
			}
		}
		if pace, ok := ItemPace(recent); ok {
			perBand[it.Band()] = append(perBand[it.Band()], pace)
		}
	}
	out := Paces{}
	for band, paces := range perBand {
		out[band] = median(paces)
	}
	return out
}

func median(xs []float64) float64 {
	sort.Float64s(xs)
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

// Basis says where an estimate's pace came from.
type Basis string

const (
	BasisExact Basis = "exact" // size is in minutes: no pace involved
	BasisItem  Basis = "item"  // the item's own sessions
	BasisBand  Basis = "band"  // the median of comparable items
	BasisSeed  Basis = "seed"  // settings, before any data exists
)

// Estimate is how long an item still needs. The zero value means unknown:
// no size, or nothing to derive a pace from.
type Estimate struct {
	Remaining time.Duration
	Basis     Basis
}

// Known reports whether there is an estimate at all.
func (e Estimate) Known() bool { return e.Basis != "" }

// Provisional reports whether the estimate rests on seed pace only. Every UI
// that shows such an estimate labels it (spec §7.3).
func (e Estimate) Provisional() bool { return e.Basis == BasisSeed }

// TimeRemaining estimates the time left on an item from its size, the
// position its history reached, and the best pace available: the item's own,
// then its band's, then the seed from settings (spec §7.2).
func TimeRemaining(item Item, history []Session, bands Paces, st Settings) Estimate {
	if item.SizeValue == nil {
		return Estimate{}
	}
	left := max(*item.SizeValue-positionAfter(history), 0)
	if item.SizeUnit == UnitMinutes {
		return Estimate{Remaining: time.Duration(left) * time.Minute, Basis: BasisExact}
	}
	pace, basis := paceFor(item, history, bands, st)
	if basis == "" {
		return Estimate{}
	}
	hours := float64(left) / pace
	return Estimate{Remaining: time.Duration(hours * float64(time.Hour)).Round(time.Minute), Basis: basis}
}

func paceFor(item Item, history []Session, bands Paces, st Settings) (float64, Basis) {
	if pace, ok := ItemPace(history); ok {
		return pace, BasisItem
	}
	if pace, ok := bands[item.Band()]; ok {
		return pace, BasisBand
	}
	if pace, ok := seedPace(item, st); ok {
		return pace, BasisSeed
	}
	return 0, ""
}

// seedPace is the settings' guess for an item nobody has measured yet.
func seedPace(item Item, st Settings) (float64, bool) {
	switch item.SizeUnit {
	case UnitWords:
		return float64(st.SeedPaceWPM) * 60, true
	case UnitPages:
		switch item.FocusDemand {
		case FocusLight:
			return float64(st.SeedPaceLight), true
		case FocusMedium:
			return float64(st.SeedPaceMedium), true
		case FocusDeep:
			return float64(st.SeedPaceDeep), true
		}
	}
	return 0, false
}

// stalled reports whether reading has gone quiet for stallDays: nothing
// since the last session started, or since the item was started when it has
// never been read (spec §7.2).
func stalled(item Item, lastReadAt *time.Time, now time.Time, stallDays int) bool {
	anchor := lastReadAt
	if anchor == nil {
		anchor = item.StartedAt
	}
	if anchor == nil {
		return false
	}
	return anchor.Before(now.Add(-time.Duration(stallDays) * 24 * time.Hour))
}
