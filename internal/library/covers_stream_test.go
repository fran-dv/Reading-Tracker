package library_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
)

// A found-cover cache entry that predates the normalize pipeline, or that
// NormalizeHeld's startup pass could not fix (internal/covers' Cache.
// NormalizeHeld leaves such a cover exactly as it was), is excluded from
// the export rather than making the whole file unimportable: it is
// disposable — a found cover is refetched lazily on its next draw —
// unlike a picked or uploaded cover, which the pipeline always
// normalizes before it is ever stored. This is what keeps design.md's
// own invariant true: every export this app writes passes ImportFrom's
// bounds check (cover-management: Export Memory Stays Flat Regardless of
// Cover Count).
func TestExportExcludesAnUnnormalizedFoundCoverCache(t *testing.T) {
	src, store, clk := newTestLibraryWithStore(t)
	shelf := newShelf(t, src, "Shelf")
	stale := newItem(t, src, shelf.ID, "Stale cache", func(it *library.Item) {
		it.CoverURL = "https://covers.openlibrary.org/b/id/1-M.jpg"
	})
	if err := store.PutCover(ctx, stale.ID, covers.Cover{
		SourceURL: stale.CoverURL, MediaType: "image/png",
		Bytes: []byte("not actually a jpeg"), FetchedAt: clk.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	fine := newItem(t, src, shelf.ID, "Fine")
	if _, err := src.SetCover(ctx, fine.ID, library.CoverPicked, &library.CoverImage{
		SourceURL: "https://covers.openlibrary.org/b/id/2-M.jpg", MediaType: "image/jpeg", Bytes: []byte("fine bytes"),
	}); err != nil {
		t.Fatal(err)
	}

	out, err := exportOf(t, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range out.Covers {
		if c.ItemID == stale.ID {
			t.Fatalf("an unnormalized found-cover cache must not be exported: %+v", c)
		}
	}
	if len(out.Covers) != 1 || out.Covers[0].ItemID != fine.ID {
		t.Fatalf("got covers %+v, want only the fine picked cover", out.Covers)
	}

	dst, _ := newTestLibrary(t)
	if err := importOf(t, dst, out); err != nil {
		t.Fatalf("an unnormalized found cache must not block the whole import: %v", err)
	}
	again, err := exportOf(t, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Items) != 2 {
		t.Fatalf("both items should still import: got %d", len(again.Items))
	}
}

// Every cover streams through EachCoverImage's single open cursor
// (internal/sqlite/covers.go) rather than a []library.CoverImage, so a
// library holding many covers exports and imports exactly like one holding
// a handful: 200 is large enough that a slice build-up would be a real
// concern, not a theoretical one, and small enough that the test still
// runs fast (cover-management: Export Memory Stays Flat Regardless of
// Cover Count).
func TestExportImportManyCoversRoundTrip(t *testing.T) {
	const n = 200
	src, _ := newTestLibrary(t)
	shelf := newShelf(t, src, "Shelf")

	ids := make([]string, n)
	for i := range n {
		it := newItem(t, src, shelf.ID, fmt.Sprintf("Book %03d", i))
		if _, err := src.SetCover(ctx, it.ID, library.CoverPicked, &library.CoverImage{
			SourceURL: fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", i),
			MediaType: "image/jpeg", Bytes: []byte(fmt.Sprintf("cover bytes %03d", i)),
		}); err != nil {
			t.Fatal(err)
		}
		ids[i] = it.ID
	}

	out, err := exportOf(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Covers) != n {
		t.Fatalf("got %d covers, want %d", len(out.Covers), n)
	}
	for i, c := range out.Covers {
		if c.ItemID != ids[i] {
			t.Fatalf("cover %d: item %s, want %s (oldest item first)", i, c.ItemID, ids[i])
		}
		if want := fmt.Sprintf("cover bytes %03d", i); string(c.Bytes) != want {
			t.Errorf("cover %d: bytes %q, want %q", i, c.Bytes, want)
		}
	}

	dst, _ := newTestLibrary(t)
	if err := importOf(t, dst, out); err != nil {
		t.Fatal(err)
	}
	again, err := exportOf(t, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Covers) != n {
		t.Fatalf("after import, got %d covers, want %d", len(again.Covers), n)
	}
	for i, c := range again.Covers {
		if want := fmt.Sprintf("cover bytes %03d", i); string(c.Bytes) != want {
			t.Errorf("after import, cover %d: bytes %q, want %q", i, c.Bytes, want)
		}
	}
}

// Each held cover is bounds-checked before InsertCoverImage, inside the
// same transaction as every other section: a violation refuses the whole
// import — not just the bad cover — so there is never an orphaned cover
// row, nor a half-filed item left behind (cover-management: Cover Import
// Restore, the atomic-unit scenario).
func TestImportFromRefusesAnInvalidCoverRollingBackWhole(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*library.CoverImage)
	}{
		{"non-JPEG media type", func(img *library.CoverImage) { img.MediaType = "image/png" }},
		{"over the size bound", func(img *library.CoverImage) { img.Bytes = bytes.Repeat([]byte("x"), library.MaxCoverBytes+1) }},
		{"empty bytes", func(img *library.CoverImage) { img.Bytes = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, _ := newTestLibrary(t)
			shelf := newShelf(t, src, "Shelf")
			a := newItem(t, src, shelf.ID, "A")
			if _, err := src.SetCover(ctx, a.ID, library.CoverPicked, &library.CoverImage{
				SourceURL: "https://covers.openlibrary.org/b/id/1-M.jpg", MediaType: "image/jpeg", Bytes: []byte("a bytes"),
			}); err != nil {
				t.Fatal(err)
			}
			b := newItem(t, src, shelf.ID, "B")
			if _, err := src.SetCover(ctx, b.ID, library.CoverPicked, &library.CoverImage{
				SourceURL: "https://covers.openlibrary.org/b/id/2-M.jpg", MediaType: "image/jpeg", Bytes: []byte("b bytes"),
			}); err != nil {
				t.Fatal(err)
			}

			var buf bytes.Buffer
			if err := src.ExportTo(ctx, &buf); err != nil {
				t.Fatal(err)
			}
			doc := decodedExport(t, buf.Bytes())
			for i := range doc.Covers {
				if doc.Covers[i].ItemID == b.ID {
					tc.corrupt(&doc.Covers[i])
				}
			}
			raw, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			dst, _ := newTestLibrary(t)
			var verr *library.ValidationError
			err = dst.ImportFrom(ctx, bytes.NewReader(raw))
			if !errors.As(err, &verr) || verr.Field != "covers" {
				t.Fatalf("got %v, want a ValidationError on covers", err)
			}
			if shelves, err := dst.ListShelves(ctx); err != nil || len(shelves) != 0 {
				t.Fatalf("a failed import must leave the library empty, including item A's own valid cover: %v, err=%v", shelves, err)
			}
		})
	}
}

// A version-10 file written before covers joined the export (PR 30d, and
// every earlier version) simply has no "covers" key at all — not a null
// or empty one — and must still import cleanly, with every item's own
// cover_choice intact but nothing held (cover-management: Import Refuses
// an Export From a Newer Version, the older-export-without-covers half).
func TestImportFromV10FileWithNoCoversKeyImportsWithNoCovers(t *testing.T) {
	src, _ := newTestLibrary(t)
	shelf := newShelf(t, src, "Shelf")
	it := newItem(t, src, shelf.ID, "Picked")
	if _, err := src.SetCover(ctx, it.ID, library.CoverPicked, &library.CoverImage{
		SourceURL: "https://covers.openlibrary.org/b/id/1-M.jpg", MediaType: "image/jpeg", Bytes: []byte("picked bytes"),
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := src.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	doc := decodedExport(t, buf.Bytes())
	doc.Covers = nil // omitempty drops the key entirely, not as null
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"covers"`)) {
		t.Fatal("test setup: the covers key must be absent, not merely empty, to stand in for a pre-30e file")
	}

	dst, _ := newTestLibrary(t)
	if err := dst.ImportFrom(ctx, bytes.NewReader(raw)); err != nil {
		t.Fatalf("v10 file with no covers key: %v", err)
	}
	again, err := exportOf(t, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Covers) != 0 {
		t.Fatalf("got %d covers, want none", len(again.Covers))
	}
	if len(again.Items) != 1 || again.Items[0].CoverChoice != library.CoverPicked {
		t.Fatalf("the item should still import with its cover_choice, just nothing held: %+v", again.Items)
	}
}
