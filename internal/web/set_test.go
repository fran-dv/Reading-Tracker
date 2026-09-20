package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// setSignals is the start form filled in for a set: the given items, by a
// date a month out, counting from today.
func setSignals(items map[string]bool) planForm {
	in := planSignals("fixed")
	today := time.Now()
	in.Campaign = campaignForm{
		Kind:     string(library.KindSet),
		Deadline: today.AddDate(0, 1, 0).Format(dateField),
		Start:    today.Format(dateField),
		Items:    items,
	}
	return in
}

// A set campaign is started from the items ticked on the plan, shows what
// each of them still needs, and can be given more.
func TestSetCampaign(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, err := svc.CreateShelf(ctx, "Algorithms")
	if err != nil {
		t.Fatal(err)
	}
	pages := 600
	adm := fileItem(t, svc, library.Item{Title: "The Algorithm Design Manual", Why: "interviews",
		Format: library.FormatBook, ShelfID: shelf.ID, SizeValue: &pages})
	sicp := fileItem(t, svc, library.Item{Title: "SICP", Why: "foundations",
		Format: library.FormatBook, ShelfID: shelf.ID, SizeValue: &pages})
	unsized := fileItem(t, svc, library.Item{Title: "No Size", Why: "unknown",
		Format: library.FormatBook, ShelfID: shelf.ID})

	body := get(t, h, "/plan").Body.String()
	for _, want := range []string{
		`data-bind="campaign.kind"`, "these items", `data-bind="campaign.items.` + adm.ID + `"`,
		"The Algorithm Design Manual", "600 pages",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the start form missing %q", want)
		}
	}
	if strings.Contains(body, `campaign.items.`+unsized.ID) {
		t.Error("an item with no size cannot be in a set: there is nothing to measure")
	}

	if body := send(t, h, http.MethodPost, "/plan/campaign", setSignals(nil)).Body.String(); !strings.Contains(body, "Tick the items this campaign is for.") {
		t.Fatalf("a set with no items:\n%s", body)
	}

	body = send(t, h, http.MethodPost, "/plan/campaign", setSignals(map[string]bool{adm.ID: true})).Body.String()
	if !strings.Contains(body, "The Algorithm Design Manual by ") {
		t.Fatalf("a set is named after its one item:\n%s", body)
	}
	if !strings.Contains(body, "<span>Items</span><span><strong>0</strong> of <strong>1</strong></span>") {
		t.Fatalf("the block counts items:\n%s", body)
	}

	// Its item is on the shortlist, so the picks show what it asks for.
	if it, err := svc.GetItem(ctx, adm.ID); err != nil || !it.OnShortlist {
		t.Fatalf("the set's item should be shortlisted: %v", err)
	}

	view, err := svc.Plan(ctx)
	if err != nil || len(view.Campaigns) != 1 {
		t.Fatal(err)
	}
	page := "/plan/campaign/" + view.Campaigns[0].Campaign.ID
	body = get(t, h, page).Body.String()
	for _, want := range []string{
		`<span class="hero-unit">of 1 item</span>`, "Its items", "The Algorithm Design Manual",
		"Time left on them", "Needed each week", "Add items", "A set only grows.",
		"How this campaign is counted", "An item is done when you finish it",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the set's page missing %q", want)
		}
	}
	if strings.Contains(body, "Book pace") {
		t.Error("a set is measured by the time left on its items, not by book pace")
	}

	// It can be given more items, and says so.
	add := struct {
		Campaign campaignForm `json:"campaign"`
	}{campaignForm{Items: map[string]bool{sicp.ID: true}}}
	body = send(t, h, http.MethodPost, page+"/items", add).Body.String()
	if !strings.Contains(body, "1 item added.") || !strings.Contains(body, "SICP") || !strings.Contains(body, "of 2 items") {
		t.Fatalf("adding an item:\n%s", body)
	}
	if body := send(t, h, http.MethodPost, page+"/items", struct {
		Campaign campaignForm `json:"campaign"`
	}{campaignForm{Items: map[string]bool{}}}).Body.String(); !strings.Contains(body, "Tick the items to add.") {
		t.Fatalf("adding nothing:\n%s", body)
	}

	// The daily target form offers to match it, on its own items.
	body = send(t, h, http.MethodPost, "/plan/preview", fixedPlan("1h")).Body.String()
	if !strings.Contains(body, "Match The Algorithm Design Manual by ") || !strings.Contains(body, "on these items") {
		t.Fatalf("match a set:\n%s", body)
	}
}
