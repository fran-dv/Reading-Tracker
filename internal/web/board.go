package web

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// The board is where the discipline stands, drawn the same way on Home and
// on the plan (spec §6.1, §6.7): tonight's time to go, today and the week as
// bars, the week day by day, and speed with its mix (§9.1). Home shows it
// without explanations; the plan adds them.

// board is everything both screens draw from the schedule and speed.
type board struct {
	Planned bool
	Logged  string // today, when there is no plan

	ToGo     string // "1 h 35 min"; "" when nothing is left today
	ToGoLine string // what reading it achieves
	Done     string // "Done for today" or "Rest day", when nothing is left
	DoneLine string

	Today *todayLine // nil on a rest day with nothing owed
	Week  weekLine
	Days  []dayCell

	Daily   string    // "1 h 30 min a day · Mon–Fri"
	Ramp    *rampLine // the hours ramp running today
	Reached string    // "27 Sep": when the ramp that set the target ended

	Speed *speedBoard // nil when nothing has been measured and no ramp exists
}

// bar is a filled track. Values are 0–1 along it.
type bar struct {
	Fill     float64
	Mark     float64 // where the due line sits
	ShowMark bool
	DebtFrom float64 // the owed zone runs from here to the end
	ShowDebt bool
}

type todayLine struct {
	Logged, Target string
	Left           string // "1 h 05 min to the target", or "target met"
	Owed           string // "" when nothing is owed
	Bar            bar
}

type weekLine struct {
	Logged, Due, InAll string
	Behind             string // "1 h 35 min behind", or "on track"
	Bar                bar
}

// dayCell is one day of the week, for the strip and the plan's table.
type dayCell struct {
	Name                                  string // "Wed"
	Date                                  string // "Wed 16"
	Fill                                  float64
	Read                                  string // "0:25", or "–"
	Of                                    string // "of 1:30", "1:30" ahead, "rest"
	Target                                string // "1 h 30 min", "rest", ""
	ReadFull                              string // "25 min", "–"
	Owed                                  string // owed after a closed day
	Today, Rest, Future, Short, Unplanned bool
	Title                                 string // "Monday 14 Sep", for its popover
	Tips                                  []tip  // what the day means, one line each
}

// tip is one line of a day's popover. Owed and Met colour it.
type tip struct {
	Text      string
	Owed, Met bool
}

type rampLine struct {
	Increment, Ceiling string
	Next               string // the value it rises to
	NextCheck          string // "Sun 20 Sep"
	LastCheck          string // "13 Sep", or ""
	Held               bool
}

func newBoard(sc library.Schedule, sp library.Speed, wordsPerPage int) *board {
	b := &board{Planned: sc.Planned(), Logged: minutesLabel(sc.LoggedToday), Speed: newSpeedBoard(sp, wordsPerPage)}
	if !b.Planned {
		return b
	}
	target := minutes(sc.TargetToday)

	if toGo := sc.ToGo(); toGo > 0 {
		b.ToGo = minutesLabel(toGo)
		left := max(0, target-sc.LoggedToday)
		switch {
		case left > 0 && sc.Owed > 0:
			b.ToGoLine = "Meets today's target and clears the " + minutesLabel(sc.Owed) + " owed."
		case left > 0:
			b.ToGoLine = "Meets today's target."
		default:
			b.ToGoLine = "Clears what is owed."
		}
	} else if sc.RestDay() {
		b.Done, b.DoneLine = "Rest day", "Nothing is due today and nothing is owed."
	} else {
		b.Done, b.DoneLine = "Done for today", "Today's target is met and nothing is owed."
	}

	if scale := target + sc.OwedAtMidnight; scale > 0 {
		t := &todayLine{Logged: minutesLabel(sc.LoggedToday), Target: minutesLabel(target)}
		if left := target - sc.LoggedToday; left > 0 {
			t.Left = minutesLabel(left) + " to the target"
		} else if target > 0 {
			t.Left = "target met"
		}
		if sc.Owed > 0 {
			t.Owed = minutesLabel(sc.Owed)
		}
		t.Bar = bar{
			Fill:     share(sc.LoggedToday, scale),
			Mark:     share(target, scale),
			ShowMark: target > 0 && sc.OwedAtMidnight > 0,
			DebtFrom: share(target, scale),
			ShowDebt: sc.OwedAtMidnight > 0,
		}
		b.Today = t
	}

	due, all := minutes(sc.DueSoFar), minutes(sc.WeekTarget)
	b.Week = weekLine{Logged: minutesLabel(sc.WeekLogged), Due: minutesLabel(due), InAll: minutesLabel(all), Behind: "on track"}
	if behind := due - sc.WeekLogged; behind > 0 {
		b.Week.Behind = minutesLabel(behind) + " behind"
	}
	if all > 0 {
		b.Week.Bar = bar{Fill: share(sc.WeekLogged, all), Mark: share(due, all), ShowMark: true}
	}

	for _, d := range sc.Week {
		c := dayCell{
			Name:     d.Day.Format("Mon"),
			Date:     d.Day.Format("Mon 2"),
			Today:    d.Day.Equal(sc.Today),
			Future:   d.Day.After(sc.Today),
			Rest:     d.Planned && !d.Active,
			Read:     "–",
			ReadFull: "–",
		}
		c.Unplanned = !d.Planned
		if !c.Future {
			c.Read, c.ReadFull = clock(d.Logged), minutesLabel(d.Logged)
		}
		switch {
		case c.Unplanned:
			c.Of = "no plan"
		case c.Rest:
			c.Of, c.Target = "rest", "rest"
		case c.Future:
			c.Of, c.Target = clock(minutes(d.Target)), minutesLabel(minutes(d.Target))
		default:
			c.Of, c.Target = "of "+clock(minutes(d.Target)), minutesLabel(minutes(d.Target))
			c.Fill = share(d.Logged, minutes(d.Target))
			c.Short = d.Closed && d.Logged < minutes(d.Target)
		}
		if d.Closed {
			c.Owed = minutesLabel(d.OwedAfter)
		}
		c.Title, c.Tips = dayTips(d, c)
		b.Days = append(b.Days, c)
	}

	b.Daily = minutesLabel(minutes(sc.Value)) + " a day · " + daysLabel(sc.Days)
	if !sc.ReachedCeilingOn.IsZero() {
		b.Reached = sc.ReachedCeilingOn.Format("2 Jan")
	}
	if rp := sc.Ramp; rp != nil {
		b.Ramp = &rampLine{
			Increment: minutesLabel(minutes(rp.Increment)),
			Ceiling:   minutesLabel(minutes(rp.Ceiling)),
			Next:      minutesLabel(minutes(min(rp.Current+rp.Increment, rp.Ceiling))),
			NextCheck: rp.NextCheck.Format("Mon 2 Jan"),
		}
		if rp.LastCheck != nil {
			b.Ramp.LastCheck, b.Ramp.Held = rp.LastCheck.On.Format("2 Jan"), !rp.LastCheck.Advanced
		}
	}
	return b
}

// speedBoard is last week's speed, its mix, and the speed ramp.
type speedBoard struct {
	Last *speedLine // nil when last week measured nothing
	Mix  []mixPart
	Ramp *speedRampView
}

type mixPart struct {
	Format  string // for its cloth
	Label   string // "book · medium focus"
	Percent string // "88%"
	Grow    float64
}

type speedRampView struct {
	Running   bool
	Target    string // "105%"
	Ceiling   string
	Increment string
	Progress  float64 // how far the target has come from 100% to the ceiling
	Started   string
	Reached   string
	Stopped   string
	NextCheck string
	Checks    []checkRow // newest first, the current week so far on top
	Index     *indexBar  // the current week so far
	Worked    *worked
}

type checkRow struct {
	When     string // "Sun 13 · 6–12 Sep", or "This week so far"
	Index    string
	Needed   string
	Measured string
	Result   string
	Rose     bool
	SoFar    bool
}

type indexBar struct {
	Fill, Baseline, Target float64
	Low, High              string
	Index, Measured        string
	Enough                 bool
}

func newSpeedBoard(sp library.Speed, wordsPerPage int) *speedBoard {
	sb := &speedBoard{Last: newSpeedLine(sp.LastWeek, wordsPerPage)}
	if sp.LastWeek.Enough() {
		for _, m := range sp.LastWeek.Mix {
			sb.Mix = append(sb.Mix, mixPart{
				Format:  string(m.Format),
				Label:   string(m.Format) + " · " + string(m.FocusDemand) + " focus",
				Percent: fmt.Sprintf("%.0f%%", m.Share*100),
				Grow:    math.Round(m.Share * 1000),
			})
		}
	}
	if r := sp.Ramp; r != nil {
		sb.Ramp = newSpeedRampView(r)
	}
	if sb.Last == nil && sb.Ramp == nil {
		return nil
	}
	return sb
}

func newSpeedRampView(r *library.SpeedRampState) *speedRampView {
	v := &speedRampView{
		Running:   r.Running,
		Target:    fmt.Sprintf("%d%%", r.Target),
		Ceiling:   fmt.Sprintf("%d%%", r.Ramp.CeilingPercent),
		Increment: fmt.Sprintf("%d%%", r.Ramp.IncrementPercent),
		Progress:  float64(r.Target-100) / float64(r.Ramp.CeilingPercent-100),
		Started:   r.Ramp.StartedOn.Format("2 Jan"),
		Worked:    newWorked(r),
	}
	if !r.ReachedOn.IsZero() {
		v.Reached = r.ReachedOn.Format("2 Jan")
	} else if r.Ramp.StoppedOn != nil && !r.Running {
		v.Stopped = r.Ramp.StoppedOn.Format("2 Jan")
	}
	if !r.NextCheck.IsZero() {
		v.NextCheck = r.NextCheck.Format("Mon 2 Jan")
	}
	if x := r.ThisWeek; x != nil {
		row := checkRow{When: "This week so far", Needed: v.Target, Measured: minutesLabel(x.Measured) + " of 2 h", SoFar: true, Result: "checked " + v.NextCheck}
		if x.Measured > 0 {
			row.Index = percent(x.Index)
		} else {
			row.Index = "–"
		}
		v.Checks = append(v.Checks, row)
		if x.Measured > 0 {
			v.Index = newIndexBar(x, r.Target)
		}
	}
	for i := len(r.Checks) - 1; i >= 0 && len(v.Checks) < 5; i-- {
		c := r.Checks[i]
		row := checkRow{
			When:     c.On.Format("Mon 2") + " · " + weekDates(c.On.AddDate(0, 0, -7), c.On),
			Index:    percent(c.Index),
			Needed:   fmt.Sprintf("%d%%", c.Target),
			Measured: minutesLabel(c.Measured),
			Rose:     c.Advanced,
		}
		switch {
		case c.Advanced:
			row.Result = fmt.Sprintf("rose to %d%%", min(c.Target+r.Ramp.IncrementPercent, r.Ramp.CeilingPercent))
		case !c.Enough:
			row.Result = "held: under 2 h measured"
			if c.Measured == 0 {
				row.Index = "–"
			}
		default:
			row.Result = "held: below target"
		}
		v.Checks = append(v.Checks, row)
	}
	return v
}

func newIndexBar(x *library.SpeedIndex, target int) *indexBar {
	high := 120.0
	for _, v := range []float64{float64(target) + 10, x.Index*100 + 10} {
		high = max(high, math.Ceil(v/10)*10)
	}
	const low = 80.0
	pos := func(p float64) float64 { return math.Max(0, math.Min(1, (p-low)/(high-low))) }
	return &indexBar{
		Fill:     pos(x.Index * 100),
		Baseline: pos(100),
		Target:   pos(float64(target)),
		Low:      fmt.Sprintf("%.0f%%", low),
		High:     fmt.Sprintf("%.0f%%", high),
		Index:    percent(x.Index),
		Measured: minutesLabel(x.Measured),
		Enough:   x.Enough(),
	}
}

// minutes converts whole minutes to a duration.
func minutes(m int) time.Duration { return time.Duration(m) * time.Minute }

// share is part ÷ whole, kept within 0–1.
func share(part, whole time.Duration) float64 {
	if whole <= 0 {
		return 0
	}
	return math.Max(0, math.Min(1, float64(part)/float64(whole)))
}

// clock writes a short day figure: "1:30", "0:25".
func clock(d time.Duration) string {
	m := int(d.Round(time.Minute).Minutes())
	return fmt.Sprintf("%d:%02d", m/60, m%60)
}

// daysLabel names a set of days briefly: "every day", "Mon–Fri", "Mon Wed Fri".
func daysLabel(days library.Weekdays) string {
	switch days {
	case library.AllWeekdays:
		return "every day"
	case library.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday):
		return "Mon–Fri"
	}
	var names []string
	for d := time.Monday; d <= time.Saturday+1; d++ {
		if days.Has(d % 7) {
			names = append(names, (d % 7).String()[:3])
		}
	}
	return strings.Join(names, " ")
}

// daysSentence names a set of days in full, for the summary: "Monday to
// Friday", "Monday, Wednesday and Friday", "every day".
func daysSentence(days library.Weekdays) string {
	switch days {
	case library.AllWeekdays:
		return "every day"
	case library.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday):
		return "Monday to Friday"
	}
	var names []string
	for d := time.Monday; d <= time.Saturday+1; d++ {
		if days.Has(d % 7) {
			names = append(names, (d % 7).String())
		}
	}
	if n := len(names); n > 1 {
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	}
	return strings.Join(names, "")
}

// dayTips explains one day of the week in up to three lines: what was read
// against the target, what that did, and what was owed in all afterwards.
// It only says reading paid something back when something was owed.
func dayTips(d library.DaySheet, c dayCell) (string, []tip) {
	title := d.Day.Format("Monday 2 Jan")
	if c.Today {
		title += " · today"
	}
	target, read, before := minutes(d.Target), d.Logged, d.OwedBefore
	owedInAll := tip{Text: "Nothing owed after this day", Met: true}
	if d.OwedAfter > 0 {
		owedInAll = tip{Text: minutesLabel(d.OwedAfter) + " owed in all after this day", Owed: true}
	}
	// paidBack says what reading beyond the target did with what was owed.
	paidBack := func(extra time.Duration) string {
		if before == 0 {
			return "extra reading isn't saved for later"
		}
		return "the extra " + minutesLabel(extra) + " paid back " + minutesLabel(min(extra, before)) + " you owed"
	}

	switch {
	case c.Unplanned:
		if read == 0 {
			return title, []tip{{Text: "Before your plan began: no target"}}
		}
		return title, []tip{{Text: "Before your plan began: no target"}, {Text: "Read " + minutesLabel(read)}}
	case c.Rest && c.Future:
		return title, []tip{{Text: "Rest day: no target"}}
	case c.Future:
		return title, []tip{{Text: "Target " + minutesLabel(target)}, {Text: "Still to come"}}
	case c.Rest:
		tips := []tip{{Text: "Rest day: no target"}}
		switch {
		case read == 0:
			tips = append(tips, tip{Text: "Nothing read"})
		case before > 0:
			tips = append(tips, tip{Text: "Read " + minutesLabel(read) + ", paying back " + minutesLabel(min(read, before)) + " you owed", Met: true})
		default:
			tips = append(tips, tip{Text: "Read " + minutesLabel(read)})
		}
		if c.Today {
			if before > 0 {
				return title, append(tips, tip{Text: "Any reading today pays back what you owe"})
			}
			return title, tips
		}
		return title, append(tips, owedInAll)
	case c.Today:
		tips := []tip{{Text: "Read " + minutesLabel(read) + " of " + minutesLabel(target) + " so far"}}
		if left := target - read; left > 0 {
			return title, append(tips,
				tip{Text: minutesLabel(left) + " left for the target"},
				tip{Text: "Whatever is still short at midnight is added to what you owe"})
		}
		tips = append(tips, tip{Text: "Target met", Met: true})
		if before > 0 {
			return title, append(tips, tip{Text: "Reading beyond it pays back what you owe"})
		}
		return title, append(tips, tip{Text: "Extra reading isn't saved for later"})
	}
	tips := []tip{{Text: "Read " + minutesLabel(read) + " of " + minutesLabel(target)}}
	switch {
	case read < target:
		tips = append(tips, tip{Text: minutesLabel(target-read) + " short, added to what you owe", Owed: true})
	case read > target:
		tips = append(tips, tip{Text: "Target met; " + paidBack(read-target), Met: true})
	default:
		tips = append(tips, tip{Text: "Target met", Met: true})
	}
	return title, append(tips, owedInAll)
}
