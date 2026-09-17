package web

import (
	"strings"
	"testing"
)

// Stats draws its charts as SVG with a title on every mark, and pace never
// shows without its mix.
func TestStatsPage(t *testing.T) {
	f := newHomeFixture(t) // an hour on Deep Work, to page 100 of 296
	body := get(t, f.handler, "/stats").Body.String()
	for _, want := range []string{
		`<svg class="chart" viewBox="0 0 600 200" role="img" aria-label="Hours read each week against the week&#39;s target">`,
		"<title>Week of", "As a table", "Completed", "Item by item", "Deep Work", "Faint rows rest on under an hour of reading.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stats missing %q", want)
		}
	}
	if strings.Contains(body, "pages/h</strong>") && !strings.Contains(body, "What the hours went to") {
		t.Error("a pace figure without its mix")
	}
}
