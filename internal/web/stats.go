package web

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Stats (spec §6.5): hours against the target, what was owed over time,
// completions, pace every way it is measured (always with its mix, §9.1),
// and the campaign's count over time. Every chart has its table.

type statsBody struct {
	Weeks       columnChart
	WeekRows    []statsRow
	Days        columnChart
	Owed        *lineChart // nil before any closed planned day
	OwedNow     string
	Months      columnChart
	MonthRows   []statsRow
	Pace        string // "32 pages/h"; "" when too little is measured
	PaceWords   string
	PaceOver    string // "over 12 h 30 min in the last 90 days"
	Mix         []mixPart
	Bands       []statsRow
	Items       []statsRow
	Campaign    *lineChart
	CampaignOf  string // "23 of 100 since 20 Aug"
	PaceWindow  int
	HasReading  bool
}

// statsRow is a row of a stats table: a label and its figures.
type statsRow struct {
	Label   string
	Href    string
	Figures []string
	Quiet   bool
}

type statsPage struct {
	shell
	Body *statsBody
}

func (h *handler) getStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	st, err := h.svc.Stats(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	b := &statsBody{PaceWindow: settings.PaceWindowDays}
	b.Weeks, b.WeekRows = weeksChart(st.Weeks)
	b.Days = daysChart(st.Days)
	b.Owed, b.OwedNow = owedChart(st.Owed)
	b.Months, b.MonthRows = monthsChart(st.Months)
	for _, d := range st.Days {
		if d.Logged > 0 {
			b.HasReading = true
		}
	}

	if st.Pace.Enough() {
		b.Pace = fmt.Sprintf("%.0f pages/h", st.Pace.PagesPerHour)
		b.PaceWords = fmt.Sprintf("%.0f words/min", st.Pace.PagesPerHour*float64(settings.WordsPerPage)/60)
	}
	if st.Pace.Measured > 0 {
		b.PaceOver = fmt.Sprintf("over %s in the last %d days", minutesLabel(st.Pace.Measured), settings.PaceWindowDays)
	}
	for _, m := range st.Pace.Mix {
		b.Mix = append(b.Mix, mixPart{Format: string(m.Format), Label: string(m.Format) + " · " + string(m.FocusDemand) + " focus",
			Percent: fmt.Sprintf("%.0f%%", m.Share*100), Grow: math.Round(m.Share * 1000)})
	}
	for _, band := range st.Bands {
		b.Bands = append(b.Bands, statsRow{Label: material(band.Band), Figures: []string{
			bandSpeed(band.Band, band.Pace), countLabel(band.Items, "item"), minutesLabel(band.Time)}})
	}
	for _, it := range st.ItemPaces {
		b.Items = append(b.Items, statsRow{Label: it.Item.Title, Href: "/items/" + it.Item.ID, Figures: []string{
			bandSpeed(it.Item.Band(), it.Pace), material(it.Item.Band()), minutesLabel(it.Time)},
			Quiet: it.Time < time.Hour})
	}
	if cs := st.Campaign; cs != nil && cs.Campaign.Active() {
		b.Campaign = campaignChart(cs, st.Counted)
		b.CampaignOf = fmt.Sprintf("%d of %d since %s", cs.Finished, cs.Campaign.TargetCount, cs.Campaign.StartedOn.Format("2 Jan 2006"))
	}
	h.render(w, r, h.stats, statsPage{shell: h.newShell(ctx, "/stats"), Body: b})
}

// weeksChart draws each week's reading as a column with its target marked.
func weeksChart(weeks []library.StatsWeek) (columnChart, []statsRow) {
	c := columnChart{Label: "Hours read each week against the week's target"}
	var rows []statsRow
	highest := 0.0
	for _, w := range weeks {
		highest = math.Max(highest, math.Max(w.Logged.Hours(), float64(w.Target)/60))
	}
	top, step := niceMax(highest)
	c.Ticks = ticks(top, step, hoursTick)
	xs, width := columnSlots(max(len(weeks), 8))
	for i, w := range weeks {
		col := column{X: xs[i], W: width, X2: xs[i] + width,
			Title: w.Start.Format("Week of 2 Jan") + ": " + minutesLabel(w.Logged) + targetOf(w.Target)}
		class := "col-read"
		if w.Target > 0 && w.Logged < minutes(w.Target) && i < len(weeks)-1 {
			class = "col-short"
		}
		col.Parts = []columnPart{{Y: yScale(w.Logged.Hours(), top), H: chartBase - yScale(w.Logged.Hours(), top), Class: class}}
		if w.Target > 0 {
			col.Mark, col.HasMark = yScale(float64(w.Target)/60, top), true
		}
		c.Columns = append(c.Columns, col)
		if i%2 == len(weeks)%2 || len(weeks) <= 8 {
			c.XLabels = append(c.XLabels, axisLabel{X: xs[i] + width/2, Label: w.Start.Format("2 Jan")})
		}
		rows = append(rows, statsRow{Label: w.Start.Format("Week of 2 Jan"), Figures: []string{minutesLabel(w.Logged), targetFigure(w.Target)}})
	}
	return c, rows
}

// daysChart draws the last days the same way.
func daysChart(days []library.DaySheet) columnChart {
	c := columnChart{Label: "Hours read each day against the day's target"}
	highest := 0.0
	for _, d := range days {
		highest = math.Max(highest, math.Max(d.Logged.Hours(), float64(d.Target)/60))
	}
	top, step := niceMax(highest)
	c.Ticks = ticks(top, step, hoursTick)
	xs, width := columnSlots(len(days))
	for i, d := range days {
		col := column{X: xs[i], W: width, X2: xs[i] + width, Title: d.Day.Format("Mon 2 Jan") + ": " + minutesLabel(d.Logged) + targetOf(d.Target)}
		class := "col-read"
		if d.Closed && d.Active && d.Logged < minutes(d.Target) {
			class = "col-short"
		}
		col.Parts = []columnPart{{Y: yScale(d.Logged.Hours(), top), H: chartBase - yScale(d.Logged.Hours(), top), Class: class}}
		if d.Planned && d.Active {
			col.Mark, col.HasMark = yScale(float64(d.Target)/60, top), true
		}
		c.Columns = append(c.Columns, col)
		if d.Day.Weekday() == days[len(days)-1].Day.Weekday() {
			c.XLabels = append(c.XLabels, axisLabel{X: xs[i] + width/2, Label: d.Day.Format("2 Jan")})
		}
	}
	return c
}

// owedChart draws what was owed at the close of each planned day.
func owedChart(days []library.DaySheet) (*lineChart, string) {
	if len(days) == 0 {
		return nil, ""
	}
	c := &lineChart{Label: "Time owed at the close of each day"}
	highest := 0.0
	for _, d := range days {
		highest = math.Max(highest, d.OwedAfter.Hours())
	}
	top, step := niceMax(math.Max(highest, 0.5))
	c.Ticks = ticks(top, step, hoursTick)
	span := float64(len(days) - 1)
	for i, d := range days {
		x := chartLeft
		if span > 0 {
			x += (chartW - chartLeft - chartRight) * float64(i) / span
		}
		c.Points = append(c.Points, chartPoint{X: x, Y: yScale(d.OwedAfter.Hours(), top), Title: d.Day.Format("Mon 2 Jan") + ": " + minutesLabel(d.OwedAfter) + " owed"})
		if i == 0 || i == len(days)-1 || (len(days) > 14 && i == len(days)/2) {
			c.XLabels = append(c.XLabels, axisLabel{X: x, Label: d.Day.Format("2 Jan")})
		}
	}
	c.Lines = []chartLine{{Points: polyline(c.Points), Area: area(c.Points), Class: "chart-owed"}}
	return c, minutesLabel(days[len(days)-1].OwedAfter)
}

// monthsChart stacks each month's completions: books, other items, reference.
func monthsChart(months []library.StatsMonth) (columnChart, []statsRow) {
	c := columnChart{Label: "Items completed each month: books, other items and reference"}
	var rows []statsRow
	highest := 0
	for _, m := range months {
		highest = max(highest, m.Books+m.Other+m.Reference)
	}
	top, step := niceMax(float64(highest))
	step = math.Max(1, math.Round(step))
	c.Ticks = ticks(top, step, func(v float64) string { return fmt.Sprintf("%.0f", v) })
	xs, width := columnSlots(len(months))
	for i, m := range months {
		col := column{X: xs[i], W: width, X2: xs[i] + width,
			Title: fmt.Sprintf("%s: %s, %d other, %d reference", m.Month.Format("January 2006"), countLabel(m.Books, "book"), m.Other, m.Reference)}
		base := 0
		for _, part := range []struct {
			n     int
			class string
		}{{m.Books, "col-book"}, {m.Other, "col-other"}, {m.Reference, "col-reference"}} {
			if part.n == 0 {
				continue
			}
			y := yScale(float64(base+part.n), top)
			// A 2px gap of ground between stacked parts.
			col.Parts = append(col.Parts, columnPart{Y: y, H: math.Max(1, yScale(float64(base), top)-y-2), Class: part.class})
			base += part.n
		}
		c.Columns = append(c.Columns, col)
		c.XLabels = append(c.XLabels, axisLabel{X: xs[i] + width/2, Label: m.Month.Format("Jan")})
		if m.Books+m.Other+m.Reference > 0 {
			rows = append(rows, statsRow{Label: m.Month.Format("January 2006"), Figures: []string{fmt.Sprint(m.Books), fmt.Sprint(m.Other), fmt.Sprint(m.Reference)}})
		}
	}
	return c, rows
}

// campaignChart draws books counted over time against an even pace from the
// start to the target on the deadline.
func campaignChart(cs *library.CampaignState, counted []time.Time) *lineChart {
	camp := cs.Campaign
	c := &lineChart{Label: "Books counted over time, against an even pace to the target"}
	top, step := niceMax(float64(camp.TargetCount))
	c.Ticks = ticks(top, step, func(v float64) string { return fmt.Sprintf("%.0f", v) })
	total := camp.Deadline.Sub(camp.StartedOn).Hours() + 24
	x := func(t time.Time) float64 {
		return chartLeft + (chartW-chartLeft-chartRight)*math.Min(1, t.Sub(camp.StartedOn).Hours()/total)
	}
	even := []chartPoint{{X: x(camp.StartedOn), Y: yScale(0, top)}, {X: x(camp.Deadline.AddDate(0, 0, 1)), Y: yScale(float64(camp.TargetCount), top)}}
	count := []chartPoint{{X: x(camp.StartedOn), Y: yScale(0, top)}}
	for i, day := range counted {
		p := chartPoint{X: x(day), Y: yScale(float64(i+1), top), Title: fmt.Sprintf("%s: %d of %d", day.Format("2 Jan"), i+1, camp.TargetCount)}
		count = append(count, chartPoint{X: p.X, Y: yScale(float64(i), top)}, p) // a step up on the day
		c.Points = append(c.Points, p)
	}
	today := time.Now()
	count = append(count, chartPoint{X: x(today), Y: yScale(float64(len(counted)), top)})
	c.Lines = []chartLine{{Points: polyline(even), Class: "chart-even"}, {Points: polyline(count), Class: "chart-count"}}
	c.XLabels = []axisLabel{{X: x(camp.StartedOn), Label: camp.StartedOn.Format("2 Jan")}, {X: x(camp.Deadline.AddDate(0, 0, 1)), Label: camp.Deadline.Format("2 Jan")}}
	return c
}

func targetOf(target int) string {
	if target == 0 {
		return ""
	}
	return " of " + minutesLabel(minutes(target))
}

func targetFigure(target int) string {
	if target == 0 {
		return "–"
	}
	return minutesLabel(minutes(target))
}

