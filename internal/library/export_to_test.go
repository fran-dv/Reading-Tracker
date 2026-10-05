package library_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// ExportTo must produce exactly what Export produces, just streamed. Once
// every caller has migrated, this is the round-trip test export_test.go
// keeps; for now it only checks the two paths agree.
func TestExportToMatchesExport(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)

	want, err := svc.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := svc.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	var got library.Export
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("ExportTo did not produce valid JSON: %v\n%s", err, buf.String())
	}
	gotRaw, err := json.Marshal(&got)
	if err != nil {
		t.Fatal(err)
	}
	if string(wantRaw) != string(gotRaw) {
		t.Fatalf("ExportTo disagrees with Export:\n%s\n---\n%s", gotRaw, wantRaw)
	}
}

// An empty library's sections are "[]", never "null", so an element added
// later always has somewhere to go when read back with plain JSON tools.
func TestExportToEmptyLibraryArraysNotNull(t *testing.T) {
	svc, _ := newTestLibrary(t)

	var buf bytes.Buffer
	if err := svc.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"shelves", "items", "ranks", "sessions", "active_days", "commitments",
		"speed_ramps", "campaigns", "campaign_items", "reviews", "moments_seen",
	} {
		if got := string(doc[key]); got != "[]" {
			t.Errorf("%s = %s, want []", key, got)
		}
	}
}

// failAfter lets the first n bytes through, then fails every write after
// that, so ExportTo must stop partway through a real document without
// panicking — not just fail before writing anything.
type failAfter struct{ n int }

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("boom")
	}
	if len(p) <= f.n {
		f.n -= len(p)
		return len(p), nil
	}
	n := f.n
	f.n = 0
	return n, errors.New("boom") // a short write must carry an error
}

func TestExportToReportsAFailingWriter(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)

	for _, n := range []int{0, 100} {
		if err := svc.ExportTo(ctx, &failAfter{n: n}); err == nil {
			t.Fatalf("ExportTo with a writer failing after %d bytes: want an error, got nil", n)
		}
	}
}
