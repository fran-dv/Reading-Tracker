package web

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// coverFixture files one item with a cover link and one without.
func coverFixture(t *testing.T, meta *fakeMeta) (http.Handler, *library.Item, *library.Item, *library.Shelf) {
	t.Helper()
	h, svc := newTestServer(t, meta)
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
	return h, with, without, shelf
}

func TestCoverServesTheImageOnceItIsFetched(t *testing.T) {
	meta := &fakeMeta{image: []byte("jpeg bytes"), imageType: "image/jpeg"}
	h, item, _, _ := coverFixture(t, meta)

	rec := get(t, h, "/items/"+item.ID+"/cover")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "jpeg bytes" {
		t.Errorf("body %q, want the stored bytes", got)
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
	h, _, without, _ := coverFixture(t, meta)

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
	h, item, _, _ := coverFixture(t, meta)

	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

// Every list draws the plate; only items with a cover link carry the image.
func TestEntriesDrawTheirPlate(t *testing.T) {
	meta := &fakeMeta{image: []byte("jpeg bytes"), imageType: "image/jpeg"}
	h, with, without, shelf := coverFixture(t, meta)

	for _, page := range []string{"/review", "/shelves/" + shelf.ID} {
		body := get(t, h, page).Body.String()
		if !strings.Contains(body, `src="/items/`+with.ID+`/cover"`) {
			t.Errorf("%s: no cover for the item that has one", page)
		}
		if strings.Contains(body, `src="/items/`+without.ID+`/cover"`) {
			t.Errorf("%s: asked for a cover the item does not have", page)
		}
		if !strings.Contains(body, "entry-plate cloth-paper") {
			t.Errorf("%s: the item without a cover has no blank plate", page)
		}
	}
}
