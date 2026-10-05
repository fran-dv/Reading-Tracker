package library_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// ImportFrom must land the same library ExportTo described, streamed
// instead of decoded whole, and must behave exactly like Export/Import
// used to for an owner round-tripping their data.
func TestImportFromRoundTrip(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)

	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	dst, _ := newTestLibrary(t)
	if err := dst.ImportFrom(ctx, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}

	// Both libraries export to the exact same bytes: every field of every
	// section, not just how many rows landed.
	want, err := src.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dst.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(wantRaw) != string(gotRaw) {
		t.Fatalf("ImportFrom landed a different library:\n%s\n---\n%s", gotRaw, wantRaw)
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
// doesn't understand.
func TestImportFromRefusesNewerVersion(t *testing.T) {
	dst, _ := newTestLibrary(t)
	doc := `{"version":99}`
	var verr *library.ValidationError
	err := dst.ImportFrom(ctx, strings.NewReader(doc))
	if !errors.As(err, &verr) || verr.Field != "version" {
		t.Fatalf("got %v, want a ValidationError on version", err)
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
		{"wrong type", `{"version":"10"`, "version"},
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

// Importing into a library that already holds something is refused, same
// as the in-memory Import.
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

// The version-upgrade rules ImportFrom applies while streaming must behave
// exactly like the ones Import applied after decoding whole: this is what
// -import now runs for every file older than the current version.
func TestImportFromVersionUpgrades(t *testing.T) {
	src, clk := newTestLibrary(t)
	populate(t, src, clk)
	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	t.Run("version 1 gets the default words per page", func(t *testing.T) {
		doc := decodedExport(t, buf.Bytes())
		doc.Version = 1
		doc.ActiveDays, doc.Commitments, doc.Campaigns, doc.Reviews = nil, nil, nil, nil
		doc.Settings.WordsPerPage = 0
		raw, err := json.Marshal(doc)
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
	})

	t.Run("campaigns before version 8 get the count kind", func(t *testing.T) {
		doc := decodedExport(t, buf.Bytes())
		doc.Version = 7
		for i := range doc.Campaigns {
			doc.Campaigns[i].Kind = "" // version 7 never recorded one
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		dst, _ := newTestLibrary(t)
		if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
			t.Fatalf("version 7 import: %v", err)
		}
		again, err := dst.Export(ctx)
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
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		dst, _ := newTestLibrary(t)
		if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
			t.Fatalf("version 9 import: %v", err)
		}
		again, err := dst.Export(ctx)
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
// empty: ImportFrom must refuse to merge into it, same as Import did.
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
