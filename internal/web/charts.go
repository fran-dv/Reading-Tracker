package web

import (
	"fmt"
	"math"
	"strings"
)

// Charts drawn as SVG on the server (spec §6.5). The Go side works out every
// coordinate in a fixed viewBox; the "chart-columns" and "chart-line"
// templates only draw them. Each mark carries a <title>, its tooltip, and
// every chart is followed by the same figures as a table.

const (
	chartW     = 600.0
	chartH     = 200.0
	chartLeft  = 40.0 // room for the y labels
	chartRight = 8.0
	chartTop   = 10.0
	chartBase  = 176.0 // the baseline; x labels sit under it
)

// axisTick is a horizontal grid line with its label.
type axisTick struct {
	Y     float64
	Label string
}

// axisLabel is a label under the baseline.
type axisLabel struct {
	X     float64
	Label string
}

// columnChart is a column per period, stacked from its parts, with an
// optional target mark across it.
type columnChart struct {
	Label   string // what it shows, for screen readers
	Columns []column
	Ticks   []axisTick
	XLabels []axisLabel
}

type column struct {
	X, W    float64
	X2      float64 // X + W, where the target mark ends
	Parts   []columnPart
	Mark    float64 // y of the target mark; 0 for none
	HasMark bool
	Title   string
}

type columnPart struct {
	Y, H  float64
	Class string // "col-read", "col-short", "cloth-book"…
}

// lineChart is one or two lines over time on one axis.
type lineChart struct {
	Label   string
	Lines   []chartLine
	Ticks   []axisTick
	XLabels []axisLabel
	Points  []chartPoint // hover targets
}

type chartLine struct {
	Points string // "x,y x,y …" for a polyline
	Area   string // the same closed along the baseline, for a fill; "" for none
	Class  string
}

type chartPoint struct {
	X, Y  float64
	Title string
}

// yScale maps a value between 0 and max to the plot's height.
func yScale(v, max float64) float64 {
	if max <= 0 {
		return chartBase
	}
	return chartBase - (chartBase-chartTop)*v/max
}

// niceMax rounds a maximum up to a step that reads well, returning the
// step: 1, 2 or 5 times a power of ten, in about four steps.
func niceMax(max float64) (top, step float64) {
	if max <= 0 {
		return 1, 1
	}
	raw := max / 4
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5, 10} {
		if step = m * mag; step >= raw {
			break
		}
	}
	return math.Ceil(max/step) * step, step
}

// ticks draws grid lines from 0 to top, each labelled by format.
func ticks(top, step float64, format func(float64) string) []axisTick {
	var out []axisTick
	for v := 0.0; v <= top+step/2; v += step {
		out = append(out, axisTick{Y: yScale(v, top), Label: format(v)})
	}
	return out
}

// hoursTick writes an hours value for an axis: "0", "2 h", "1.5 h", or
// "45 min" under an hour.
func hoursTick(h float64) string {
	switch {
	case h == 0:
		return "0"
	case h < 1:
		return fmt.Sprintf("%.0f min", h*60)
	}
	return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", h), "0"), ".") + " h"
}

// columnSlots places n columns across the plot, returning each one's x and
// width, with a gap between them.
func columnSlots(n int) (xs []float64, w float64) {
	slot := (chartW - chartLeft - chartRight) / float64(n)
	w = math.Max(2, slot*0.7)
	for i := range n {
		xs = append(xs, chartLeft+slot*float64(i)+(slot-w)/2)
	}
	return xs, w
}

// polyline writes points for an SVG polyline.
func polyline(points []chartPoint) string {
	var b strings.Builder
	for i, p := range points {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.1f,%.1f", p.X, p.Y)
	}
	return b.String()
}

// area closes a polyline along the baseline, for a fill under it.
func area(points []chartPoint) string {
	if len(points) == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f,%.1f %s %.1f,%.1f", points[0].X, chartBase, polyline(points), points[len(points)-1].X, chartBase)
}
