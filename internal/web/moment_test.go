package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Finishing the book that meets a campaign says its count and what it took,
// and the moment appears above the board until it is closed.
func TestHomeMomentAndFinishedLine(t *testing.T) {
	f := newHomeFixture(t)
	today := time.Now()
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if _, err := f.svc.StartCampaign(ctx, "", 1, day, day.AddDate(0, 0, 30)); err != nil {
		t.Fatal(err)
	}
	long := momentSignals(library.TimeLong, false)
	in := long
	in.Verdict = "Worth it."
	body := send(t, f.handler, http.MethodPost, "/items/"+f.book.ID+"/finish", in).Body.String()
	for _, want := range []string{
		"Finished Deep Work: 1 of 1.", "296 pages in 1 h 00 min over 1 day · “Worth it.”",
		`class="goal-headline" id="goal-headline">You did it.</h2>`, "1 book by", "4 weeks before the deadline",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("finish missing %q:\n%s", want, body)
		}
	}

	all, err := f.svc.Achievements(ctx)
	if err != nil || len(all) == 0 {
		t.Fatalf("achievements %v, %v", all, err)
	}
	var key string
	for _, a := range all {
		if a.Kind == library.CampaignMet {
			key = a.Key
		}
	}
	closed := send(t, f.handler, http.MethodPost, "/moments/"+key+"/close", long).Body.String()
	if strings.Contains(closed, "goal-headline") {
		t.Fatal("a closed moment must not come back")
	}
	if strings.Contains(get(t, f.handler, "/").Body.String(), "goal-headline") {
		t.Fatal("a closed moment must not come back on a reload")
	}
}
