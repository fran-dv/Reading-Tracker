package library_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/covers"
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
		it.Publisher, it.ISBN = "Addison-Wesley Professional", "0-306-40615-2"
	})
	setTags(t, svc, textbook.ID, "IQ", "math")
	other := newItem(t, svc, iq.ID, "Other")
	rank(t, svc, stats.ID, textbook.ID, 1)
	rank(t, svc, iq.ID, textbook.ID, 1)
	rank(t, svc, iq.ID, other.ID, 2)

	reading := newItem(t, svc, stats.ID, "Reading")
	startItem(t, svc, reading.ID)
	start := clk.Now().Add(-time.Hour)
	logged, err := svc.AddRetroactiveSession(ctx, reading.ID, start, clk.Now(), ptr(20), "note")
	if err != nil {
		t.Fatal(err)
	}
	// An edited session keeps its mark through the round trip.
	if _, err := svc.EditSession(ctx, logged.ID, library.SessionEdit{Start: start, End: clk.Now(), Reached: ptr(25), Note: "note"}); err != nil {
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

	// ExportTo's bytes go straight into ImportFrom, the same file the owner
	// would download and re-import, not through an intermediate Export value.
	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()

	var out library.Export
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Version != library.ExportVersion || len(out.Shelves) != 2 || len(out.Items) != 3 ||
		len(out.Ranks) != 3 || len(out.Sessions) != 2 || out.Settings.WIPCap != 3 ||
		len(out.ActiveDays) != 1 || len(out.Commitments) != 1 || len(out.Campaigns) != 2 || len(out.Reviews) != 1 {
		t.Fatalf("export shape wrong: %+v", out)
	}

	dst, _ := newTestLibrary(t)
	if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := dst.ExportTo(ctx, &again); err != nil {
		t.Fatal(err)
	}
	if string(raw) != again.String() {
		t.Fatalf("round trip differs:\n%s\n---\n%s", raw, again.String())
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

// A version-8 file has no publisher or isbn on its items; they import empty.
func TestImportVersion8NoPublisherISBN(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)
	out, err := exportOf(t, src)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	old["version"] = 8
	for _, it := range old["items"].([]any) {
		m := it.(map[string]any)
		delete(m, "publisher")
		delete(m, "isbn")
	}
	raw, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var in library.Export
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}

	dst, _ := newTestLibrary(t)
	if err := importOf(t, dst, &in); err != nil {
		t.Fatalf("version 8 import: %v", err)
	}
	again, err := exportOf(t, dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range again.Items {
		if it.Publisher != "" || it.ISBN != "" {
			t.Fatalf("item %q: publisher=%q isbn=%q, want both empty from a version-8 file", it.Title, it.Publisher, it.ISBN)
		}
	}
}

// Every cover choice, and every held cover's bytes, round-trips through
// export and import byte-identical: the found choice's lazy-fetch cache,
// a picked cover and an uploaded one all carry their image across. A
// remembered lookup failure holds no bytes and is never exported as a
// cover entry, and neither is a removed cover, which holds none
// (cover-management: Cover Export Includes Choice and Every Held Cover,
// Cover Import Restore).
func TestExportImportRoundTripCoverChoice(t *testing.T) {
	src, store, clk := newTestLibraryWithStore(t)
	shelf := newShelf(t, src, "Shelf")

	found := newItem(t, src, shelf.ID, "Found", func(it *library.Item) {
		it.CoverURL = "https://covers.openlibrary.org/b/id/1-M.jpg"
	})
	if err := store.PutCover(ctx, found.ID, covers.Cover{
		SourceURL: found.CoverURL, MediaType: "image/jpeg",
		Bytes: []byte("found cached bytes"), FetchedAt: clk.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	picked := newItem(t, src, shelf.ID, "Picked")
	if _, err := src.SetCover(ctx, picked.ID, library.CoverPicked, &library.CoverImage{
		SourceURL: "https://covers.openlibrary.org/b/id/2-M.jpg", MediaType: "image/jpeg", Bytes: []byte("picked bytes"),
	}); err != nil {
		t.Fatal(err)
	}
	uploaded := newItem(t, src, shelf.ID, "Uploaded")
	if _, err := src.SetCover(ctx, uploaded.ID, library.CoverUploaded, &library.CoverImage{
		MediaType: "image/jpeg", Bytes: []byte("uploaded bytes"),
	}); err != nil {
		t.Fatal(err)
	}
	removed := newItem(t, src, shelf.ID, "Removed")
	if _, err := src.SetCover(ctx, removed.ID, library.CoverRemoved, nil); err != nil {
		t.Fatal(err)
	}
	// A lookup that failed and was remembered holds no bytes: the covers
	// row exists, but with an empty (never nil — the column is NOT NULL)
	// blob, exactly as the migration's own x'' default documents. It must
	// never surface as a cover entry in the export.
	failed := newItem(t, src, shelf.ID, "Failed")
	if err := store.PutCover(ctx, failed.ID, covers.Cover{
		SourceURL: "https://covers.openlibrary.org/b/id/9-M.jpg", Bytes: []byte{}, FetchedAt: clk.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	out, err := exportOf(t, src)
	if err != nil {
		t.Fatal(err)
	}
	choices := map[string]library.CoverChoice{}
	for _, it := range out.Items {
		choices[it.Title] = it.CoverChoice
	}
	want := map[string]library.CoverChoice{
		"Found": library.CoverFound, "Picked": library.CoverPicked,
		"Uploaded": library.CoverUploaded, "Removed": library.CoverRemoved,
		"Failed": library.CoverFound,
	}
	for title, choice := range want {
		if choices[title] != choice {
			t.Errorf("%s: got choice %q, want %q", title, choices[title], choice)
		}
	}

	wantBytes := map[string]string{
		found.ID: "found cached bytes", picked.ID: "picked bytes", uploaded.ID: "uploaded bytes",
	}
	checkCovers := func(t *testing.T, imgs []library.CoverImage) {
		t.Helper()
		byItem := map[string]library.CoverImage{}
		for _, c := range imgs {
			byItem[c.ItemID] = c
		}
		if len(imgs) != len(wantBytes) {
			t.Fatalf("got %d covers, want %d (found, picked, uploaded; a remembered failure and a removal hold none)",
				len(imgs), len(wantBytes))
		}
		for id, want := range wantBytes {
			got, ok := byItem[id]
			if !ok || string(got.Bytes) != want {
				t.Errorf("item %s: cover bytes = %+v, want %q", id, got, want)
			}
		}
		if _, ok := byItem[failed.ID]; ok {
			t.Error("a remembered lookup failure must not be exported as a cover entry")
		}
		if _, ok := byItem[removed.ID]; ok {
			t.Error("a removed cover must not be exported")
		}
	}
	checkCovers(t, out.Covers)

	dst, _ := newTestLibrary(t)
	if err := importOf(t, dst, out); err != nil {
		t.Fatal(err)
	}
	again, err := exportOf(t, dst)
	if err != nil {
		t.Fatal(err)
	}
	choicesAfter := map[string]library.CoverChoice{}
	for _, it := range again.Items {
		choicesAfter[it.Title] = it.CoverChoice
	}
	for title, choice := range want {
		if choicesAfter[title] != choice {
			t.Errorf("after import, %s: got choice %q, want %q", title, choicesAfter[title], choice)
		}
	}
	checkCovers(t, again.Covers)
}

// Exports before version 7 kept one campaign's needs per review, as an
// object or null; they still read.
func TestReviewReadsOldNeeds(t *testing.T) {
	for in, want := range map[string]int{
		`{"week_of":"2026-09-13T00:00:00Z","closed_at":"2026-09-13T20:00:00Z","needs":{"campaign_id":"c","books_left":9}}`:        1,
		`{"week_of":"2026-09-13T00:00:00Z","closed_at":"2026-09-13T20:00:00Z","needs":null}`:                                      0,
		`{"week_of":"2026-09-13T00:00:00Z","closed_at":"2026-09-13T20:00:00Z","needs":[{"campaign_id":"c"},{"campaign_id":"d"}]}`: 2,
	} {
		var rv library.Review
		if err := json.Unmarshal([]byte(in), &rv); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if len(rv.Needs) != want || rv.WeekOf.Day() != 13 || rv.ClosedAt.Hour() != 20 {
			t.Fatalf("%s: got %+v, want %d needs", in, rv, want)
		}
	}
}
