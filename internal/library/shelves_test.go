package library_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Spec §3 worked example: a statistics textbook lives on Statistics and
// carries tag IQ; the IQ shelf shows it, marked borrowed.
func TestShelfItemsBorrowed(t *testing.T) {
	svc, clk := newTestLibrary(t)
	stats := newShelf(t, svc, "Statistics")
	iq := newShelf(t, svc, "IQ")

	textbook := newItem(t, svc, stats.ID, "Stats textbook")
	setTags(t, svc, textbook.ID, "iq") // case differs from the shelf name on purpose
	clk.Advance(time.Minute)
	native := newItem(t, svc, iq.ID, "IQ native")
	setTags(t, svc, native.ID, "IQ") // own shelf as tag: not borrowed, not duplicated
	clk.Advance(time.Minute)
	running := newItem(t, svc, iq.ID, "IQ in progress")
	startItem(t, svc, running.ID)
	done := newItem(t, svc, iq.ID, "IQ finished")
	startItem(t, svc, done.ID)
	if _, err := svc.Finish(ctx, done.ID, ""); err != nil {
		t.Fatal(err)
	}

	view, err := svc.ShelfItems(ctx, iq.ID)
	if err != nil {
		t.Fatal(err)
	}
	borrowed := map[string]bool{}
	for _, it := range view.Unranked {
		if _, dup := borrowed[it.ID]; dup {
			t.Fatalf("item %s listed twice", it.Title)
		}
		borrowed[it.ID] = it.Borrowed
	}
	if len(borrowed) != 3 {
		t.Fatalf("IQ shows %d items, want 3 (textbook, native, running): %v", len(borrowed), view.Unranked)
	}
	if !borrowed[textbook.ID] {
		t.Fatal("textbook should be marked borrowed on IQ")
	}
	if borrowed[native.ID] || borrowed[running.ID] {
		t.Fatal("own items must not be marked borrowed")
	}
	if _, shown := borrowed[done.ID]; shown {
		t.Fatal("finished item must not be on the shelf")
	}
	// newest first
	if view.Unranked[0].ID != running.ID || view.Unranked[2].ID != textbook.ID {
		t.Fatalf("unranked order wrong: %v", view.Unranked)
	}

	view, err = svc.ShelfItems(ctx, stats.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Unranked) != 1 || view.Unranked[0].Borrowed {
		t.Fatalf("Statistics should show only its own textbook, unborrowed: %v", view.Unranked)
	}
}

func TestShelfItemsSlots(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	a := newItem(t, svc, shelf.ID, "a")
	b := newItem(t, svc, shelf.ID, "b")
	c := newItem(t, svc, shelf.ID, "c")
	rank(t, svc, shelf.ID, b.ID, 1)
	rank(t, svc, shelf.ID, c.ID, 2)

	view, err := svc.ShelfItems(ctx, shelf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Slots[0] == nil || view.Slots[0].ID != b.ID || view.Slots[1] == nil || view.Slots[1].ID != c.ID {
		t.Fatalf("slots wrong: %+v", view.Slots)
	}
	if view.Slots[2] != nil {
		t.Fatal("slot 3 should be empty")
	}
	if len(view.Unranked) != 1 || view.Unranked[0].ID != a.ID {
		t.Fatalf("ranked items must not repeat in unranked: %v", view.Unranked)
	}
}

func TestCreateShelf(t *testing.T) {
	svc, _ := newTestLibrary(t)
	newShelf(t, svc, "Go")
	if _, err := svc.CreateShelf(ctx, " go "); !errors.Is(err, library.ErrDuplicateShelf) {
		t.Fatalf("got %v, want ErrDuplicateShelf", err)
	}
	var verr *library.ValidationError
	if _, err := svc.CreateShelf(ctx, "  "); !errors.As(err, &verr) {
		t.Fatalf("got %v, want ValidationError", err)
	}
	second := newShelf(t, svc, "Rust")
	if second.SortOrder != 2 {
		t.Fatalf("second shelf sort_order %d, want 2", second.SortOrder)
	}
}

func TestRenameShelf(t *testing.T) {
	svc, _ := newTestLibrary(t)
	a := newShelf(t, svc, "A")
	b := newShelf(t, svc, "B")
	if _, err := svc.RenameShelf(ctx, a.ID, "b"); !errors.Is(err, library.ErrDuplicateShelf) {
		t.Fatalf("got %v, want ErrDuplicateShelf", err)
	}
	if _, err := svc.RenameShelf(ctx, a.ID, "a"); err != nil {
		t.Fatalf("renaming to own name in other case: %v", err)
	}

	// An item borrowed onto B via tag "B" loses its rank there when B is renamed.
	item := newItem(t, svc, a.ID, "x")
	setTags(t, svc, item.ID, "B")
	rank(t, svc, b.ID, item.ID, 1)
	if _, err := svc.RenameShelf(ctx, b.ID, "Beta"); err != nil {
		t.Fatal(err)
	}
	wantIDs(t, slotIDs(t, svc, b.ID))
}

func TestReorderShelves(t *testing.T) {
	svc, _ := newTestLibrary(t)
	a := newShelf(t, svc, "A")
	b := newShelf(t, svc, "B")
	c := newShelf(t, svc, "C")

	if err := svc.ReorderShelves(ctx, []string{c.ID, a.ID}); err == nil {
		t.Fatal("partial reorder must fail")
	}
	if err := svc.ReorderShelves(ctx, []string{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	shelves, err := svc.ListShelves(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, []string{shelves[0].ID, shelves[1].ID, shelves[2].ID}, c.ID, a.ID, b.ID)
}

func TestDeleteShelf(t *testing.T) {
	svc, _ := newTestLibrary(t)
	full := newShelf(t, svc, "Full")
	empty := newShelf(t, svc, "Empty")
	item := newItem(t, svc, full.ID, "x")
	setTags(t, svc, item.ID, "Empty")
	rank(t, svc, empty.ID, item.ID, 1) // borrowed rank must not block deletion

	if err := svc.DeleteShelf(ctx, full.ID); !errors.Is(err, library.ErrShelfNotEmpty) {
		t.Fatalf("got %v, want ErrShelfNotEmpty", err)
	}
	if err := svc.DeleteShelf(ctx, empty.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ShelfItems(ctx, empty.ID); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestLastUsedShelfID(t *testing.T) {
	svc, clk := newTestLibrary(t)

	if id, err := svc.LastUsedShelfID(ctx); err != nil || id != "" {
		t.Fatalf("no shelves: got %q, %v; want empty", id, err)
	}

	first := newShelf(t, svc, "First")
	second := newShelf(t, svc, "Second")
	if id, _ := svc.LastUsedShelfID(ctx); id != first.ID {
		t.Fatalf("no items: got %q, want first shelf %q", id, first.ID)
	}

	newItem(t, svc, second.ID, "older")
	clk.Advance(time.Minute)
	newItem(t, svc, first.ID, "newer")
	if id, _ := svc.LastUsedShelfID(ctx); id != first.ID {
		t.Fatalf("got %q, want shelf of newest item %q", id, first.ID)
	}
	clk.Advance(time.Minute)
	newItem(t, svc, second.ID, "newest")
	if id, _ := svc.LastUsedShelfID(ctx); id != second.ID {
		t.Fatalf("got %q, want shelf of newest item %q", id, second.ID)
	}
}

func TestShelfItemsUnrankedOrder(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	oldest := newItem(t, svc, shelf.ID, "oldest")
	clk.Advance(time.Minute)
	reading := newItem(t, svc, shelf.ID, "reading")
	clk.Advance(time.Minute)
	newest := newItem(t, svc, shelf.ID, "newest")
	startItem(t, svc, reading.ID)

	view, err := svc.ShelfItems(ctx, shelf.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range view.Unranked {
		got = append(got, it.ID)
	}
	wantIDs(t, got, reading.ID, newest.ID, oldest.ID)
}
