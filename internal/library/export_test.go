package library_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// populate builds a small library touching every table: two shelves, a
// borrowed item with ranks on both, a closed and a running session, a plan,
// two campaigns, a closed review and non-default settings.
func populate(t *testing.T, svc *library.Service, clk *clock) {
	t.Helper()
	stats := newShelf(t, svc, "Statistics")
	iq := newShelf(t, svc, "IQ")
	textbook := newItem(t, svc, stats.ID, "Textbook", func(it *library.Item) {
		it.SizeValue = ptr(400)
		it.CoverURL = "https://covers.openlibrary.org/b/id/1-M.jpg"
	})
	setTags(t, svc, textbook.ID, "IQ", "math")
	other := newItem(t, svc, iq.ID, "Other")
	rank(t, svc, stats.ID, textbook.ID, 1)
	rank(t, svc, iq.ID, textbook.ID, 1)
	rank(t, svc, iq.ID, other.ID, 2)

	reading := newItem(t, svc, stats.ID, "Reading")
	startItem(t, svc, reading.ID)
	start := clk.Now().Add(-time.Hour)
	if _, err := svc.AddRetroactiveSession(ctx, reading.ID, start, clk.Now(), ptr(20), "note"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartSession(ctx, reading.ID); err != nil {
		t.Fatal(err)
	}

	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitRamp,
		StartMinutes: 60, IncrementMinutes: 30, CeilingMinutes: 240}, false); err != nil {
		t.Fatal(err)
	}

	old, err := svc.StartCampaign(ctx, "", 10, clk.Now().AddDate(0, 0, -30).Truncate(24*time.Hour), clk.Now().AddDate(0, 3, 0).Truncate(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EndCampaign(ctx, old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartCampaign(ctx, "100 books", 100, clk.Now().Truncate(24*time.Hour), clk.Now().AddDate(1, 0, 0).Truncate(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CloseReview(ctx); err != nil {
		t.Fatal(err)
	}

	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.WIPCap = 3
	settings.Timezone = "Europe/Madrid"
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)

	out, err := src.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != library.ExportVersion || len(out.Shelves) != 2 || len(out.Items) != 3 ||
		len(out.Ranks) != 3 || len(out.Sessions) != 2 || out.Settings.WIPCap != 3 ||
		len(out.ActiveDays) != 1 || len(out.Commitments) != 1 || len(out.Campaigns) != 2 || len(out.Reviews) != 1 {
		t.Fatalf("export shape wrong: %+v", out)
	}

	// Through JSON, as the file on disk would be.
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var in library.Export
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}

	dst, _ := newTestLibrary(t)
	if err := dst.Import(ctx, &in); err != nil {
		t.Fatal(err)
	}
	again, err := dst.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw2, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(raw2) {
		t.Fatalf("round trip differs:\n%s\n---\n%s", raw, raw2)
	}

	// The imported library behaves: the running session is still the only one.
	running, err := dst.RunningSession(ctx)
	if err != nil || running == nil {
		t.Fatalf("running session lost: %v %v", running, err)
	}
	if _, err := dst.StartSession(ctx, running.ItemID); err == nil {
		t.Fatal("second running session should be refused after import")
	}
}

func TestImportRefusals(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)
	out, err := svc.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Import(ctx, out); !errors.Is(err, library.ErrNotEmpty) {
		t.Fatalf("import into populated library: got %v, want ErrNotEmpty", err)
	}

	empty, _ := newTestLibrary(t)
	bad := *out
	bad.Version = 99
	var verr *library.ValidationError
	if err := empty.Import(ctx, &bad); !errors.As(err, &verr) || verr.Field != "version" {
		t.Fatalf("wrong version: got %v, want ValidationError on version", err)
	}
	if shelves, _ := empty.ListShelves(ctx); len(shelves) != 0 {
		t.Fatal("failed import must leave the library empty")
	}
}

func TestImportVersion1(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)
	out, err := src.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// A version 1 file predates the plan: no active days, no commitments.
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	old["version"] = 1
	delete(old, "active_days")
	delete(old, "commitments")
	delete(old, "campaigns")
	delete(old, "reviews")
	raw, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var in library.Export
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}

	dst, _ := newTestLibrary(t)
	if err := dst.Import(ctx, &in); err != nil {
		t.Fatalf("version 1 import: %v", err)
	}
	view, err := dst.Plan(ctx)
	if err != nil || view.Schedule.Planned() {
		t.Fatalf("version 1 import should have no plan: %v %+v", err, view)
	}
}
