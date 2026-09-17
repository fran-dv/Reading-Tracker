package web

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// campaignSignals is the start form filled in: a target by a date in about
// a year, counting from today.
func campaignSignals(target string) planForm {
	in := planSignals("fixed")
	today := time.Now()
	in.Campaign = campaignForm{
		Target:   target,
		Deadline: today.AddDate(1, 0, 0).Format(dateField),
		Start:    today.Format(dateField),
	}
	return in
}

// calendarDay is a local date as the library carries it: midnight UTC.
func calendarDay(t time.Time) time.Time {
	d, _ := time.Parse(dateField, t.Format(dateField))
	return d
}

func TestPlanCampaignBeforeAny(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})
	body := get(t, h, "/plan").Body.String()
	for _, want := range []string{
		`id="campaign-title"`, "No campaign.", "Start the campaign",
		`data-bind="campaign.target"`, `type="date" data-bind="campaign.deadline"`,
		"How the campaign is counted", "light 40, medium 30, deep 15 pages/h", "your last 4 weeks",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan missing %q", want)
		}
	}
	if strings.Index(body, `id="campaign-title"`) > strings.Index(body, `id="week-title"`) {
		t.Error("the campaign comes first on the plan")
	}
	if strings.Contains(body, "Match the campaign") || strings.Contains(body, "End the campaign") {
		t.Error("nothing to match or end before a campaign")
	}
	if sig := pageSignals[planForm](t, body); sig.Campaign.Start != time.Now().Format(dateField) {
		t.Errorf("counting from should default to today: %+v", sig.Campaign)
	}
}

func TestPostCampaign(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})

	bad := campaignSignals("lots")
	if body := send(t, h, http.MethodPost, "/plan/campaign", bad).Body.String(); !strings.Contains(body, "Type a whole number of books.") {
		t.Fatalf("target not a number:\n%s", body)
	}
	past := campaignSignals("100")
	past.Campaign.Deadline = time.Now().AddDate(0, 0, -1).Format(dateField)
	if body := send(t, h, http.MethodPost, "/plan/campaign", past).Body.String(); !strings.Contains(body, "After today and after the start.") || !strings.Contains(body, "campaign-deadline") {
		t.Fatalf("deadline in the past:\n%s", body)
	}

	rec := send(t, h, http.MethodPost, "/plan/campaign", campaignSignals("100"))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `id="plan-body"`) {
		t.Fatalf("status %d:\n%s", rec.Code, body)
	}
	for _, want := range []string{
		`<span class="hero-figure">0</span> <span class="hero-unit">of 100 books</span>`,
		"weeks left. Where it&#39;s heading shows once a week of reading has closed.",
		"What it needs", "Books left", "from a guess, until books have sizes",
		`<span class="provisional">30 pages/h, provisional</span>`,
		"no daily target yet", "no closed week yet",
		`aria-haspopup="dialog"`, "End the campaign…", `<dialog class="dialog" id="end-campaign"`,
		"End 100 books by ", "You have 0 of 100, with ", "Ending stops the count and the projection today, and it can&#39;t be picked up again.",
		`autofocus data-on:click="el.closest('dialog').close()">Keep it</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("started campaign missing %q", want)
		}
	}
	if strings.Contains(body, "Start the campaign") {
		t.Error("no start form while a campaign is active")
	}
	sig := patchedSignals(t, body)
	campaign, _ := sig["campaign"].(map[string]any)
	if campaign["id"] == "" || !strings.HasPrefix(campaign["name"].(string), "100 books by ") {
		t.Fatalf("signals should carry the campaign shown: %v", sig)
	}

	if body := send(t, h, http.MethodPost, "/plan/campaign", campaignSignals("5")).Body.String(); !strings.Contains(body, "A campaign is already active.") {
		t.Fatalf("second campaign:\n%s", body)
	}

	view, err := svc.Plan(ctx)
	if err != nil || view.Campaign == nil {
		t.Fatal(err)
	}
	rename := planSignals("fixed")
	rename.Campaign = campaignForm{ID: view.Campaign.Campaign.ID, Name: "The hundred"}
	if body := send(t, h, http.MethodPost, "/plan/campaign/rename", rename).Body.String(); !strings.Contains(body, "<p>The hundred</p>") {
		t.Fatalf("rename:\n%s", body)
	}
	body = send(t, h, http.MethodPost, "/plan/campaign/end", rename).Body.String()
	if !strings.Contains(body, "The last campaign, The hundred, ended on") || !strings.Contains(body, "with 0 of 100.") || !strings.Contains(body, "Start the campaign") {
		t.Fatalf("end:\n%s", body)
	}
}

func TestPlanPreviewCampaign(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	if body := send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h")).Body.String(); strings.Contains(body, "That is") || strings.Contains(body, "Match the campaign") {
		t.Fatalf("no campaign, no gap or match:\n%s", body)
	}

	deadline := time.Now().AddDate(1, 0, 0)
	if _, err := svc.StartCampaign(ctx, "", 200, calendarDay(time.Now()), calendarDay(deadline)); err != nil {
		t.Fatal(err)
	}
	body := send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h")).Body.String()
	want := "That is 7 h 00 min a week. If all of it goes to books, "
	if !strings.Contains(body, want) || !strings.Contains(body, "of 200 by "+deadline.Format("2 Jan 2006")+". The campaign needs ") {
		t.Fatalf("gap paragraph:\n%s", body)
	}
	if !strings.Contains(body, `id="plan-match"`) || !strings.Contains(body, "Match the campaign: ") || !strings.Contains(body, `data-minutes="`) {
		t.Fatalf("match offer:\n%s", body)
	}

	one := fixedPlan("1h")
	one.Days = map[string]bool{"mon": true}
	body = send(t, h, http.MethodPost, "/plan/preview", one).Body.String()
	if !strings.Contains(body, "On these days the campaign needs more than a day's reading.") {
		t.Fatalf("200 books in a year on Mondays alone cannot fit a day:\n%s", body)
	}
}

func TestCampaignLabels(t *testing.T) {
	for weeks, want := range map[float64]string{26.9: "26 weeks", 2: "2 weeks", 1.9: "14 days", 0.1: "1 day", 0.2: "2 days"} {
		if got := timeLeft(weeks); got != want {
			t.Errorf("timeLeft(%v) = %q, want %q", weeks, got, want)
		}
	}
	if lastWeeks(1) != "last week" || lastWeeks(3) != "last 3 weeks" {
		t.Error("lastWeeks")
	}
}
