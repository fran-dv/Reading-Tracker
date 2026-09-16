package library_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/sqlite"
)

// clock is a frozen, manually advanced time source.
type clock struct {
	now time.Time
}

func (c *clock) Now() time.Time { return c.now }

func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

var ctx = context.Background()

// newTestLibrary returns a Service on a fresh temp-file database with a
// clock frozen at 2026-09-15 10:00 UTC.
func newTestLibrary(t *testing.T) (*library.Service, *clock) {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	clk := &clock{now: time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)}
	return library.New(store, library.WithClock(clk.Now)), clk
}

func newShelf(t *testing.T, svc *library.Service, name string) *library.Shelf {
	t.Helper()
	sh, err := svc.CreateShelf(ctx, name)
	if err != nil {
		t.Fatalf("create shelf %q: %v", name, err)
	}
	return sh
}

// newItem files an untagged pool book on the shelf. edit may adjust fields
// before saving.
func newItem(t *testing.T, svc *library.Service, shelfID, title string, edit ...func(*library.Item)) *library.Item {
	t.Helper()
	item := library.Item{Title: title, Why: "because", Format: library.FormatBook, ShelfID: shelfID}
	for _, e := range edit {
		e(&item)
	}
	created, err := svc.CreateItem(ctx, item, nil)
	if err != nil {
		t.Fatalf("create item %q: %v", title, err)
	}
	return created
}

func startItem(t *testing.T, svc *library.Service, id string) *library.Item {
	t.Helper()
	item, err := svc.Start(ctx, id)
	if err != nil {
		t.Fatalf("start item: %v", err)
	}
	return item
}

// setTags replaces an item's tags, leaving every other field as it was.
func setTags(t *testing.T, svc *library.Service, id string, tags ...string) {
	t.Helper()
	item, err := svc.GetItem(ctx, id)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if _, err := svc.UpdateItem(ctx, *item, tags); err != nil {
		t.Fatalf("set tags: %v", err)
	}
}

func rank(t *testing.T, svc *library.Service, shelfID, itemID string, slot int) {
	t.Helper()
	if err := svc.Rank(ctx, shelfID, itemID, slot); err != nil {
		t.Fatalf("rank slot %d: %v", slot, err)
	}
}

// slotIDs returns the item IDs in the shelf's slots, in order, skipping empties.
func slotIDs(t *testing.T, svc *library.Service, shelfID string) []string {
	t.Helper()
	view, err := svc.ShelfItems(ctx, shelfID)
	if err != nil {
		t.Fatalf("shelf items: %v", err)
	}
	var ids []string
	for _, s := range view.Slots {
		if s != nil {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

func wantIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d ids, want %d: %v vs %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

func ptr(i int) *int { return &i }
