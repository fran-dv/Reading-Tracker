package web

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// tinyJPEG is a small, already-normalized JPEG: covers.Cache now runs
// every fetched cover through covers.Normalize before writing it, so a
// fake fetch answering with arbitrary non-image bytes would come back as
// a remembered failure instead of a served cover.
func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{200, 40, 40, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode fixture jpeg: %v", err)
	}
	return buf.Bytes()
}

// coverFixture files one item with a cover link and one without. opts reach
// the library, for a frozen clock.
func coverFixture(t *testing.T, meta *fakeMeta, opts ...library.Option) (http.Handler, *library.Service, *library.Item, *library.Item, *library.Shelf) {
	t.Helper()
	h, svc := newTestServer(t, meta, opts...)
	shelf, err := svc.CreateShelf(ctx, "Statistics")
	if err != nil {
		t.Fatal(err)
	}
	with := fileItem(t, svc, library.Item{
		Title: "Thinking in Systems", Why: "loops everywhere", Format: library.FormatBook,
		ShelfID: shelf.ID, CoverURL: "https://covers.example/1.jpg",
	})
	without := fileItem(t, svc, library.Item{
		Title: "A note to myself", Why: "no cover anywhere", Format: library.FormatPaper,
		ShelfID: shelf.ID,
	})
	return h, svc, with, without, shelf
}

func TestCoverServesTheImageOnceItIsFetched(t *testing.T) {
	jpeg := tinyJPEG(t)
	meta := &fakeMeta{image: jpeg, imageType: "image/jpeg"}
	h, _, item, _, _ := coverFixture(t, meta)

	rec := get(t, h, "/items/"+item.ID+"/cover")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if got := rec.Body.Bytes(); !bytes.Equal(got, jpeg) {
		t.Errorf("body is %d bytes, want the stored bytes unchanged (%d)", len(got), len(jpeg))
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("content type %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("cache-control %q, want the bytes held", got)
	}

	// Asked for again, it comes from the database and not the network.
	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusOK {
		t.Fatalf("second ask: status %d", rec.Code)
	}
	if len(meta.fetched) != 1 {
		t.Errorf("fetched %d times, want 1", len(meta.fetched))
	}
}

func TestCoverIsNotFoundWhenThereIsNone(t *testing.T) {
	meta := &fakeMeta{image: []byte("jpeg bytes"), imageType: "image/jpeg"}
	h, _, _, without, _ := coverFixture(t, meta)

	if rec := get(t, h, "/items/"+without.ID+"/cover"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
	if len(meta.fetched) != 0 {
		t.Errorf("fetched %v, want nothing without a link", meta.fetched)
	}
}

// An entry always draws its plate; only the picture over it is conditional,
// so a lookup that failed leaves the blank plate rather than a broken page.
func TestCoverThatCannotBeHadIsNotFound(t *testing.T) {
	meta := &fakeMeta{imageErr: errors.New("404")}
	h, _, item, _, _ := coverFixture(t, meta)

	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

// Every list draws the plate; only items with a cover link carry the image.
func TestEntriesDrawTheirPlate(t *testing.T) {
	meta := &fakeMeta{image: []byte("jpeg bytes"), imageType: "image/jpeg"}
	h, _, with, without, shelf := coverFixture(t, meta)

	for _, page := range []string{"/review", "/shelves/" + shelf.ID} {
		body := get(t, h, page).Body.String()
		if !strings.Contains(body, `src="/items/`+with.ID+`/cover?v=`) {
			t.Errorf("%s: no cover for the item that has one", page)
		}
		if strings.Contains(body, `src="/items/`+without.ID+`/cover`) {
			t.Errorf("%s: asked for a cover the item does not have", page)
		}
		if !strings.Contains(body, "entry-plate cloth-paper") {
			t.Errorf("%s: the item without a cover has no blank plate", page)
		}
	}
}

// Once an item's cover choice is removed, the route answers 404 without
// asking the fetcher for anything: the choice is locked, so ShowsCover is
// false (cover-management: Automatic Lookups Never Override a Locked
// Choice).
func TestCoverIsNotFoundOnceRemoved(t *testing.T) {
	meta := &fakeMeta{image: tinyJPEG(t), imageType: "image/jpeg"}
	h, svc, item, _, _ := coverFixture(t, meta)

	// Fetched once while the choice is still found.
	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusOK {
		t.Fatalf("status %d before removal", rec.Code)
	}
	if _, err := svc.SetCover(ctx, item.ID, library.CoverRemoved, nil); err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/items/"+item.ID+"/cover")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404 once removed", rec.Code)
	}
	if len(meta.fetched) != 1 {
		t.Errorf("fetched %d times, want no further attempt once removed", len(meta.fetched))
	}
}

// The plate's cache-busting ?v= changes whenever the item's cover changes,
// since a pick, an upload or a revert can change the bytes without
// changing the link (ADR-8).
func TestCoverPlateURLChangesAfterSetCover(t *testing.T) {
	clk := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	meta := &fakeMeta{image: tinyJPEG(t), imageType: "image/jpeg"}
	h, svc, item, _, shelf := coverFixture(t, meta, library.WithClock(func() time.Time { return clk }))

	before := coverPlateSrc(t, get(t, h, "/shelves/"+shelf.ID).Body.String(), item.ID)

	clk = clk.Add(time.Minute)
	if _, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{
		MediaType: "image/jpeg", Bytes: tinyJPEG(t),
	}); err != nil {
		t.Fatal(err)
	}

	after := coverPlateSrc(t, get(t, h, "/shelves/"+shelf.ID).Body.String(), item.ID)
	if before == after {
		t.Fatalf("plate URL unchanged after SetCover: %q", before)
	}
}

// coverPlateSrc returns the full src attribute value of itemID's plate
// image in body.
func coverPlateSrc(t *testing.T, body, itemID string) string {
	t.Helper()
	marker := `src="/items/` + itemID + `/cover`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no cover image for item %s in:\n%s", itemID, body)
	}
	rest := body[i+len(`src="`):]
	return rest[:strings.Index(rest, `"`)]
}
