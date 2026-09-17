package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Reading speed as the pages draw it (spec §8.6, §9.1). Every speed is drawn with the
// material it was measured on. The pages/h ↔ words/min toggle is a browser
// signal: both figures are rendered and one is shown, so switching units
// never reaches the server and resets on reload.

// speedLine is last week's reading speed.
type speedLine struct {
	Dates    string // "13–19 Sep"
	Enough   bool
	Measured string // "45 min", for a thin week
	Pages    string // "32 pages/h"
	Words    string // "160 words/min"
	Mix      string // "book · medium 70%, article · light 30%"
}

func newSpeedLine(w library.WeekSpeed, wordsPerPage int) *speedLine {
	if w.Measured == 0 {
		return nil
	}
	out := &speedLine{
		Dates:    weekDates(w.From, w.To),
		Enough:   w.Enough(),
		Measured: minutesLabel(w.Measured),
		Pages:    fmt.Sprintf("%.0f pages/h", w.PagesPerHour),
		Words:    fmt.Sprintf("%.0f words/min", w.PagesPerHour*float64(wordsPerPage)/60),
	}
	var parts []string
	for _, m := range w.Mix {
		// No-break spaces keep one kind of material and its share on one line.
		parts = append(parts, fmt.Sprintf("%s\u00a0·\u00a0%s\u00a0%.0f%%", m.Format, m.FocusDemand, m.Share*100))
	}
	out.Mix = strings.Join(parts, ", ")
	return out
}

// workedRow is one item in the worked index table.
type workedRow struct {
	Material string // "Stoner", or for a baseline "book · medium"
	ItemID   string
	Format   library.Format
	Baseline string // the speed compared with: "20 pages/h"
	LastWeek string // "22 pages/h"
	Ratio    string // "110%"; "" when it counts from next week
	Time     string
	Counted  bool
}

// worked is the speed index of the last closed week, worked through with
// the owner's own numbers.
type worked struct {
	Dates    string // "13–19 Sep"
	Before   string // "6–12 Sep", or the pace window before the ramp
	Rows     []workedRow
	Step     string // "105%": this week against the one before
	Index    string // "112%": the chain since the ramp began
	Measured string
	Enough   bool
}

func newWorked(r *library.SpeedRampState) *worked {
	x := r.LastWeek
	if x == nil {
		return nil
	}
	out := &worked{
		Dates: weekDates(x.From, x.To), Before: weekDates(x.BeforeFrom, x.BeforeTo),
		Step: percent(x.Step), Index: percent(x.Index), Measured: minutesLabel(x.Measured), Enough: x.Enough(),
	}
	for _, row := range x.Rows {
		band := row.Item.Band()
		wr := workedRow{Material: row.Item.Title, ItemID: row.Item.ID, Format: row.Item.Format,
			LastWeek: bandSpeed(band, row.PerHour), Time: minutesLabel(row.Time), Counted: row.Counted}
		if row.Counted {
			wr.Baseline, wr.Ratio = bandSpeed(band, row.Before), percent(row.Ratio)
		}
		out.Rows = append(out.Rows, wr)
	}
	return out
}

// baselineRows lists the speed a ramp started today would start from.
func baselineRows(baselines []library.Baseline) []workedRow {
	var out []workedRow
	for _, b := range baselines {
		out = append(out, workedRow{Material: material(b.Band), Baseline: bandSpeed(b.Band, b.PerHour)})
	}
	return out
}

func material(b library.Band) string { return string(b.Format) + " · " + string(b.FocusDemand) }

// bandSpeed writes a band's speed in its own unit: pages per hour, or words
// per minute, which is how reading speed in words is usually quoted.
func bandSpeed(b library.Band, perHour float64) string {
	if b.SizeUnit == library.UnitWords {
		return fmt.Sprintf("%.0f words/min", perHour/60)
	}
	return fmt.Sprintf("%.0f pages/h", perHour)
}

func percent(ratio float64) string { return fmt.Sprintf("%.0f%%", ratio*100) }

// weekDates writes a closed week as "13–19 Sep", or "27 Sep – 3 Oct".
func weekDates(from, to time.Time) string {
	last := to.AddDate(0, 0, -1)
	if from.Month() == last.Month() {
		return fmt.Sprintf("%d–%s", from.Day(), last.Format("2 Jan"))
	}
	return from.Format("2 Jan") + " – " + last.Format("2 Jan")
}
