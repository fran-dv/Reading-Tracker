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
		`id="campaigns-title"`, "No campaign.", "Start the campaign",
		`data-bind="campaign.target"`, `type="date" data-bind="campaign.deadline"`,
		"Several campaigns can run at once.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan missing %q", want)
		}
	}
	if strings.Index(body, `id="campaigns-title"`) > strings.Index(body, `id="week-title"`) {
		t.Error("the campaigns come first on the plan")
	}
	if strings.Contains(body, "Match ") || strings.Contains(body, "Start another campaign") {
		t.Error("nothing to match, and the start form open, before a campaign")
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
	view, err := svc.Plan(ctx)
	if err != nil || len(view.Campaigns) != 1 {
		t.Fatal(err)
	}
	id := view.Campaigns[0].Campaign.ID
	for _, want := range []string{
		`<a href="/plan/campaign/` + id + `">100 books by `,
		"weeks left. Where it&#39;s heading shows once a week of reading has closed.",
		"Needs ", "of books a week; no week of reading has closed yet.",
		"Start another campaign", "Start the campaign",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan with a campaign missing %q", want)
		}
	}

	// A second campaign starts beside the first.
	send(t, h, http.MethodPost, "/plan/campaign", campaignSignals("5"))
	body = get(t, h, "/plan").Body.String()
	if strings.Count(body, `<div class="campaign-block">`) != 2 || !strings.Contains(body, "5 books by ") {
		t.Fatalf("two campaigns on the plan:\n%s", body)
	}

	page := "/plan/campaign/" + id
	body = get(t, h, page).Body.String()
	for _, want := range []string{
		`<h1 class="heading">100 books by `, `<span class="hero-figure">0</span> <span class="hero-unit">of 100 books</span>`,
		"What it needs", "Books left", "from a guess, until books have sizes",
		`<span class="provisional">30 pages/h, provisional</span>`,
		"no daily target yet", "no closed week yet",
		`aria-haspopup="dialog"`, "End the campaign…", `<dialog class="dialog" id="end-campaign"`,
		"End 100 books by ", "You have 0 of 100, with ", "Ending stops the count and the projection today, and it can&#39;t be picked up again.",
		`autofocus data-on:click="el.closest('dialog').close()">Keep it</button>`,
		"How a campaign is counted", "light 40, medium 30, deep 15 pages/h", "your last 4 weeks",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("campaign page missing %q", want)
		}
	}
	if sig := pageSignals[planForm](t, body); !strings.HasPrefix(sig.Campaign.Name, "100 books by ") {
		t.Errorf("the name field should hold the campaign's name: %+v", sig.Campaign)
	}

	rename := planForm{Campaign: campaignForm{Name: "The hundred"}}
	if body := send(t, h, http.MethodPost, page+"/rename", rename).Body.String(); !strings.Contains(body, `<h1 class="heading">The hundred</h1>`) {
		t.Fatalf("rename:\n%s", body)
	}
	body = send(t, h, http.MethodPost, page+"/end", nil).Body.String()
	if !strings.Contains(body, "Ended on ") || !strings.Contains(body, "with 0 of 100.") || strings.Contains(body, "End the campaign") {
		t.Fatalf("end:\n%s", body)
	}
	body = get(t, h, "/plan").Body.String()
	if strings.Count(body, `<div class="campaign-block">`) != 1 || strings.Contains(body, "The hundred") {
		t.Fatalf("the plan should show only the campaign still active:\n%s", body)
	}
	if rec := get(t, h, "/plan/campaign/missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing campaign: status %d", rec.Code)
	}
}

func TestPlanPreviewCampaign(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	if body := send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h")).Body.String(); strings.Contains(body, "That is") || strings.Contains(body, "Match ") {
		t.Fatalf("no campaign, no gap or match:\n%s", body)
	}

	deadline := time.Now().AddDate(1, 0, 0)
	if _, err := svc.StartCampaign(ctx, "", 200, calendarDay(time.Now()), calendarDay(deadline)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartCampaign(ctx, "October", 3, calendarDay(time.Now()), calendarDay(time.Now().AddDate(0, 1, 0))); err != nil {
		t.Fatal(err)
	}
	body := send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h")).Body.String()
	for _, want := range []string{
		"That is 7 h 00 min a week. Read in full, if all of it goes to books:",
		"<p>200 books by " + deadline.Format("2 Jan 2006") + ": ", " of 200. It needs ",
		"<p>October: 3 of 3. It needs ", `id="plan-match"`, "Match 200 books by ", "Match October: ", `data-minutes="`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview with two campaigns missing %q", want)
		}
	}
	if strings.Count(body, "That is") != 1 {
		t.Error("the week is said once, not once per campaign")
	}

	one := fixedPlan("1h")
	one.Days = map[string]bool{"mon": true}
	body = send(t, h, http.MethodPost, "/plan/preview", one).Body.String()
	if !strings.Contains(body, "On these days 200 books by ") || !strings.Contains(body, " needs more than a day's reading.") {
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
