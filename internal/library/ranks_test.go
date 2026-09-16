package library_test

import (
	"errors"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// threeRanked returns a shelf with items a, b, c in slots 1, 2, 3 and a fourth pool item d.
func threeRanked(t *testing.T, svc *library.Service) (shelf *library.Shelf, a, b, c, d *library.Item) {
	t.Helper()
	shelf = newShelf(t, svc, "S")
	a = newItem(t, svc, shelf.ID, "a")
	b = newItem(t, svc, shelf.ID, "b")
	c = newItem(t, svc, shelf.ID, "c")
	d = newItem(t, svc, shelf.ID, "d")
	rank(t, svc, shelf.ID, a.ID, 1)
	rank(t, svc, shelf.ID, b.ID, 2)
	rank(t, svc, shelf.ID, c.ID, 3)
	return shelf, a, b, c, d
}

func TestRankInsert(t *testing.T) {
	t.Run("pushes down and drops the overflow", func(t *testing.T) {
		svc, _ := newTestLibrary(t)
		shelf, a, b, _, d := threeRanked(t, svc)
		rank(t, svc, shelf.ID, d.ID, 1)
		wantIDs(t, slotIDs(t, svc, shelf.ID), d.ID, a.ID, b.ID)
	})
	t.Run("moves an already ranked item", func(t *testing.T) {
		svc, _ := newTestLibrary(t)
		shelf, a, b, c, _ := threeRanked(t, svc)
		rank(t, svc, shelf.ID, c.ID, 1)
		wantIDs(t, slotIDs(t, svc, shelf.ID), c.ID, a.ID, b.ID)
	})
	t.Run("never leaves a gap", func(t *testing.T) {
		svc, _ := newTestLibrary(t)
		shelf := newShelf(t, svc, "S")
		x := newItem(t, svc, shelf.ID, "x")
		rank(t, svc, shelf.ID, x.ID, 3)
		wantIDs(t, slotIDs(t, svc, shelf.ID), x.ID)
	})
	t.Run("rejects slot out of range", func(t *testing.T) {
		svc, _ := newTestLibrary(t)
		shelf := newShelf(t, svc, "S")
		x := newItem(t, svc, shelf.ID, "x")
		var verr *library.ValidationError
		if err := svc.Rank(ctx, shelf.ID, x.ID, 4); !errors.As(err, &verr) {
			t.Fatalf("got %v, want ValidationError", err)
		}
	})
}

func TestRankRequiresPoolAndVisibility(t *testing.T) {
	svc, _ := newTestLibrary(t)
	home := newShelf(t, svc, "Home")
	other := newShelf(t, svc, "Other")
	item := newItem(t, svc, home.ID, "x")

	if err := svc.Rank(ctx, other.ID, item.ID, 1); !errors.Is(err, library.ErrNotVisibleOnShelf) {
		t.Fatalf("got %v, want ErrNotVisibleOnShelf", err)
	}
	setTags(t, svc, item.ID, "other")
	rank(t, svc, other.ID, item.ID, 1) // borrowed now, may hold a slot

	startItem(t, svc, item.ID)
	if err := svc.Rank(ctx, home.ID, item.ID, 1); !errors.Is(err, library.ErrInvalidTransition) {
		t.Fatalf("in_progress item: got %v, want ErrInvalidTransition", err)
	}
}

func TestUnrankShiftsUp(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf, a, b, c, d := threeRanked(t, svc)
	if err := svc.Unrank(ctx, shelf.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	wantIDs(t, slotIDs(t, svc, shelf.ID), b.ID, c.ID)
	if err := svc.Unrank(ctx, shelf.ID, d.ID); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("unranking an unranked item: got %v, want ErrNotFound", err)
	}
}

func TestMoveUpDown(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf, a, b, c, d := threeRanked(t, svc)

	if err := svc.MoveUp(ctx, shelf.ID, a.ID); err != nil { // top edge: no-op
		t.Fatal(err)
	}
	if err := svc.MoveDown(ctx, shelf.ID, c.ID); err != nil { // bottom edge: no-op
		t.Fatal(err)
	}
	wantIDs(t, slotIDs(t, svc, shelf.ID), a.ID, b.ID, c.ID)

	if err := svc.MoveUp(ctx, shelf.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	wantIDs(t, slotIDs(t, svc, shelf.ID), a.ID, c.ID, b.ID)
	if err := svc.MoveDown(ctx, shelf.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	wantIDs(t, slotIDs(t, svc, shelf.ID), c.ID, a.ID, b.ID)

	if err := svc.MoveUp(ctx, shelf.ID, d.ID); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("moving an unranked item: got %v, want ErrNotFound", err)
	}
}

// Spec §5.1: when a slotted item leaves the pool, lower slots shift up on
// every shelf that ranked it, including shelves borrowing it.
func TestLeavingPoolClearsRanksEverywhere(t *testing.T) {
	setup := func(t *testing.T) (svc *library.Service, stats, iq *library.Shelf, textbook, s2, i2 *library.Item) {
		svc, _ = newTestLibrary(t)
		stats = newShelf(t, svc, "Statistics")
		iq = newShelf(t, svc, "IQ")
		textbook = newItem(t, svc, stats.ID, "textbook")
		setTags(t, svc, textbook.ID, "IQ")
		s2 = newItem(t, svc, stats.ID, "stats other")
		i2 = newItem(t, svc, iq.ID, "iq other")
		rank(t, svc, stats.ID, textbook.ID, 1)
		rank(t, svc, stats.ID, s2.ID, 2)
		rank(t, svc, iq.ID, textbook.ID, 1)
		rank(t, svc, iq.ID, i2.ID, 2)
		return svc, stats, iq, textbook, s2, i2
	}

	t.Run("start", func(t *testing.T) {
		svc, stats, iq, textbook, s2, i2 := setup(t)
		startItem(t, svc, textbook.ID)
		wantIDs(t, slotIDs(t, svc, stats.ID), s2.ID)
		wantIDs(t, slotIDs(t, svc, iq.ID), i2.ID)
	})
	t.Run("abandon from pool", func(t *testing.T) {
		svc, stats, iq, textbook, s2, i2 := setup(t)
		if _, err := svc.Abandon(ctx, textbook.ID, "no"); err != nil {
			t.Fatal(err)
		}
		wantIDs(t, slotIDs(t, svc, stats.ID), s2.ID)
		wantIDs(t, slotIDs(t, svc, iq.ID), i2.ID)
	})
	t.Run("tag removed clears only the borrowing shelf", func(t *testing.T) {
		svc, stats, iq, textbook, s2, i2 := setup(t)
		setTags(t, svc, textbook.ID)
		wantIDs(t, slotIDs(t, svc, stats.ID), textbook.ID, s2.ID)
		wantIDs(t, slotIDs(t, svc, iq.ID), i2.ID)
	})
}

func TestTagsAreWrittenWithTheItem(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")

	t.Run("create normalizes them", func(t *testing.T) {
		item := library.Item{Title: "x", Why: "w", Format: library.FormatBook, ShelfID: shelf.ID}
		created, err := svc.CreateItem(ctx, item, []string{" Go ", "go", "", "rust"})
		if err != nil {
			t.Fatal(err)
		}
		tags, err := svc.Tags(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		wantIDs(t, tags, "Go", "rust")
	})

	t.Run("update replaces them", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "y")
		setTags(t, svc, item.ID, "one", "two")
		setTags(t, svc, item.ID, "two")
		tags, err := svc.Tags(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		wantIDs(t, tags, "two")
	})

	t.Run("update of a missing item is not found", func(t *testing.T) {
		item := library.Item{ID: "nope", Title: "x", Why: "w", Format: library.FormatBook, ShelfID: shelf.ID}
		if _, err := svc.UpdateItem(ctx, item, nil); !errors.Is(err, library.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})
}
