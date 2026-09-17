package web

import (
	"strings"
	"testing"
	"time"
)

// The record keeps every campaign as it went and opens empty with a hint.
func TestRecordPage(t *testing.T) {
	f := newHomeFixture(t)
	if body := get(t, f.handler, "/record").Body.String(); !strings.Contains(body, "No campaign yet.") || !strings.Contains(body, `aria-current="page">Record`) {
		t.Fatalf("record before any goal:\n%s", body)
	}
	today := time.Now()
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	c, err := f.svc.StartCampaign(ctx, "", 1, day, day.AddDate(0, 0, 30))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := f.svc.Finish(ctx, f.book.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	body := get(t, f.handler, "/record").Body.String()
	for _, want := range []string{c.Name, "met " + today.Format("2 Jan") + ", 4 weeks before the deadline", "Longest book finished", "296 pages", "Year by year"} {
		if !strings.Contains(body, want) {
			t.Errorf("record missing %q", want)
		}
	}
}
