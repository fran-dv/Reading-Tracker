package library_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Every export this app has ever written puts "version" first; a file that
// doesn't is refused before any transaction opens, not decoded as if the
// key were missing.
func TestImportFromRefusesVersionNotFirst(t *testing.T) {
	dst, _ := newTestLibrary(t)
	doc := `{"settings":{},"version":10}`
	var verr *library.ValidationError
	err := dst.ImportFrom(ctx, strings.NewReader(doc))
	if !errors.As(err, &verr) || verr.Field != "version" {
		t.Fatalf("got %v, want a ValidationError on version", err)
	}
}

// A version above what this binary knows is refused outright, before any
// transaction opens, so a reverted binary never silently drops data it
// doesn't understand, and the database it was refused into is left
// exactly as empty as it started.
func TestImportFromRefusesNewerVersion(t *testing.T) {
	dst, _ := newTestLibrary(t)
	doc := `{"version":99}`
	var verr *library.ValidationError
	err := dst.ImportFrom(ctx, strings.NewReader(doc))
	if !errors.As(err, &verr) || verr.Field != "version" {
		t.Fatalf("got %v, want a ValidationError on version", err)
	}
	if shelves, err := dst.ListShelves(ctx); err != nil || len(shelves) != 0 {
		t.Fatalf("a refused import must leave the library empty: %v, err=%v", shelves, err)
	}
}

// A version that is valid JSON but the wrong type gets the plain "not a
// number" message; a file simply cut off inside the version's value gets
// the real decode error instead of that same misleading message.
func TestImportFromVersionDecodeErrors(t *testing.T) {
	tests := []struct {
		name      string
		doc       string
		wantField string // "" when no ValidationError is expected
	}{
		{"wrong type", `{"version":"10"}`, "version"},
		{"cut off mid-value", `{"version":"1`, ""}, // unterminated string: a real syntax error, not a type mismatch
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst, _ := newTestLibrary(t)
			err := dst.ImportFrom(ctx, strings.NewReader(tc.doc))
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			var verr *library.ValidationError
			if got := errors.As(err, &verr); got != (tc.wantField != "") {
				t.Fatalf("errors.As ValidationError = %v, err was %v", got, err)
			}
			if verr != nil && verr.Field != tc.wantField {
				t.Fatalf("ValidationError.Field = %q, want %q", verr.Field, tc.wantField)
			}
		})
	}
}

// Importing into a library that already holds something is refused.
func TestImportFromRefusesNonEmptyLibrary(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)

	var buf bytes.Buffer
	if err := svc.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	if err := svc.ImportFrom(ctx, bytes.NewReader(buf.Bytes())); !errors.Is(err, library.ErrNotEmpty) {
		t.Fatalf("import into populated library: got %v, want ErrNotEmpty", err)
	}
}

// A file truncated mid-section fails to decode and leaves the database
// exactly as empty as it started: one transaction, rolled back whole.
func TestImportFromTruncatedFileLeavesLibraryEmpty(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)
	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	truncated := buf.String()[:len(buf.String())/2]

	dst, _ := newTestLibrary(t)
	if err := dst.ImportFrom(ctx, strings.NewReader(truncated)); err == nil {
		t.Fatal("truncated import: want an error, got nil")
	}
	if shelves, err := dst.ListShelves(ctx); err != nil || len(shelves) != 0 {
		t.Fatalf("a failed import must leave the library empty: %v, err=%v", shelves, err)
	}
}

// An unrecognised key is skipped rather than failing the import: forward
// compatibility within one version isn't a goal, but a stray key from a
// newer build of this same version shouldn't block an otherwise-good file.
func TestImportFromSkipsUnknownKeys(t *testing.T) {
	dst, _ := newTestLibrary(t)
	doc := `{"version":10,"mystery":{"anything":[1,2,3]},"settings":` + validSettingsJSON + `}`
	if err := dst.ImportFrom(ctx, strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	settings, err := dst.Settings(ctx)
	if err != nil || settings.Timezone != "UTC" {
		t.Fatalf("settings after an unknown key: %v, err=%v", settings, err)
	}
}

// A file with no settings section at all is refused: settings is written
// last but is not optional, unlike every other section.
func TestImportFromRefusesAFileWithNoSettings(t *testing.T) {
	dst, _ := newTestLibrary(t)
	doc := `{"version":10,"shelves":[]}`
	var verr *library.ValidationError
	err := dst.ImportFrom(ctx, strings.NewReader(doc))
	if !errors.As(err, &verr) || verr.Field != "settings" {
		t.Fatalf("got %v, want a ValidationError on settings", err)
	}
}

// validSettingsJSON mirrors migration 001's defaults, just with UTC instead
// of Local, so a hand-written document in these tests passes validate().
const validSettingsJSON = `{"timezone":"UTC","wip_cap":5,"stall_days":14,` +
	`"review_weekday":0,"bucket_quick_max_min":25,"bucket_hour_min_min":45,` +
	`"bucket_hour_max_min":75,"bucket_long_min_min":90,"pace_window_days":90,` +
	`"projection_window_weeks":4,"seed_pace_light":40,"seed_pace_medium":30,` +
	`"seed_pace_deep":15,"seed_pace_wpm":230,"fallback_book_pages":300,` +
	`"words_per_page":300}`

// decodedExport re-decodes the streamed bytes into an Export, so each
// subtest below starts from its own independent copy to mutate. Export's
// field order matches ExportTo's write order (version first), so
// json.Marshal on the mutated value stays a valid import document — unlike
// a map[string]any, which encoding/json always sorts by key.
func decodedExport(t *testing.T, raw []byte) library.Export {
	t.Helper()
	var doc library.Export
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// The version-upgrade rules ImportFrom applies while streaming must hold
// for every file older than the current version: this is what -import
// runs now, for every version it still accepts. Each subtest encodes with
// json.MarshalIndent rather than json.Marshal, the style the old in-memory
// Export method wrote (json.Encoder with SetIndent("", "  ")), so these
// also stand in for a real file from before this app streamed.
func TestImportFromVersionUpgrades(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)
	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	t.Run("version 1 gets the default words per page and has no plan", func(t *testing.T) {
		doc := decodedExport(t, buf.Bytes())
		doc.Version = 1
		doc.ActiveDays, doc.Commitments, doc.Campaigns, doc.Reviews = nil, nil, nil, nil
		doc.Settings.WordsPerPage = 0
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		dst, _ := newTestLibrary(t)
		if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
			t.Fatalf("version 1 import: %v", err)
		}
		got, err := dst.Settings(ctx)
		if err != nil || got.WordsPerPage != 300 {
			t.Fatalf("got %+v, err=%v, want words_per_page 300", got, err)
		}
		// A version 1 file predates the plan entirely: no active days or
		// commitments means no schedule, not an empty one.
		view, err := dst.Plan(ctx)
		if err != nil || view.Schedule.Planned() {
			t.Fatalf("version 1 import should have no plan: %v %+v", err, view)
		}
	})

	t.Run("campaigns before version 8 get the count kind", func(t *testing.T) {
		doc := decodedExport(t, buf.Bytes())
		doc.Version = 7
		for i := range doc.Campaigns {
			doc.Campaigns[i].Kind = "" // version 7 never recorded one
		}
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		dst, _ := newTestLibrary(t)
		if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
			t.Fatalf("version 7 import: %v", err)
		}
		again, err := exportOf(t, dst)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range again.Campaigns {
			if c.Kind != library.KindCount {
				t.Errorf("campaign %q: kind %q, want count", c.Name, c.Kind)
			}
		}
	})

	t.Run("items before version 10 get the found cover choice", func(t *testing.T) {
		doc := decodedExport(t, buf.Bytes())
		doc.Version = 9
		for i := range doc.Items {
			doc.Items[i].CoverChoice = "" // version 9 predates cover choices
		}
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		dst, _ := newTestLibrary(t)
		if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
			t.Fatalf("version 9 import: %v", err)
		}
		again, err := exportOf(t, dst)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range again.Items {
			if it.CoverChoice != library.CoverFound {
				t.Errorf("item %q: cover choice %q, want found", it.Title, it.CoverChoice)
			}
		}
	})
}

// A library holding only a plan, with no shelves or items, is still not
// empty: ImportFrom must refuse to merge into it.
func TestImportFromRefusesAPlannedLibrary(t *testing.T) {
	svc, _ := newTestLibrary(t)
	if err := svc.SavePlan(ctx, library.AllWeekdays, library.Commitment{Kind: library.CommitFixed, MinutesPerDay: 30}, false); err != nil {
		t.Fatal(err)
	}
	src, _ := newTestLibrary(t)
	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	if err := svc.ImportFrom(ctx, bytes.NewReader(buf.Bytes())); !errors.Is(err, library.ErrNotEmpty) {
		t.Fatalf("got %v, want ErrNotEmpty", err)
	}
}

// A file with its sections out of the order ExportTo always writes them in
// — here, ranks before the items they point to — fails on the foreign key
// rather than being silently reordered, and leaves the database empty.
func TestImportFromRefusesSectionOutOfOrder(t *testing.T) {
	dst, _ := newTestLibrary(t)
	doc := `{"version":10,"settings":` + validSettingsJSON + `,` +
		`"shelves":[{"id":"s1","name":"Shelf","sort_order":1,"created_at":"2026-01-01T00:00:00Z"}],` +
		`"ranks":[{"shelf_id":"s1","item_id":"missing-item","slot":1}],` +
		`"items":[]}`
	err := dst.ImportFrom(ctx, strings.NewReader(doc))
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("ranks before items: got %v, want a foreign-key error", err)
	}
	if shelves, err := dst.ListShelves(ctx); err != nil || len(shelves) != 0 {
		t.Fatalf("a failed import must leave the library empty: %v, err=%v", shelves, err)
	}
}
