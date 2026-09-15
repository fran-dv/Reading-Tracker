package library

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

// Shelf is an item's single home. Flat, no nesting.
type Shelf struct {
	ID        string
	Name      string
	SortOrder int
	CreatedAt time.Time
}

// ShelfItem is an item as seen from a shelf. Borrowed is true when the item
// lives elsewhere and appears here through a tag equal to the shelf name.
type ShelfItem struct {
	Item
	Borrowed bool
}

// ShelfView is a shelf with its three rank slots and the rest of its
// visible items. Only pool and in_progress items are included.
type ShelfView struct {
	Shelf    Shelf
	Slots    [MaxSlots]*ShelfItem // nil where the slot is empty
	Unranked []ShelfItem          // newest first
}

// CreateShelf adds a shelf at the end of the order.
func (s *Service) CreateShelf(ctx context.Context, name string) (*Shelf, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &ValidationError{"name", "required"}
	}
	shelf := &Shelf{ID: newID(), Name: name, CreatedAt: s.now()}
	err := s.store.Tx(ctx, func(r Repo) error {
		if err := checkNameFree(r, name, ""); err != nil {
			return err
		}
		shelves, err := r.ListShelves()
		if err != nil {
			return err
		}
		shelf.SortOrder = len(shelves) + 1
		return r.InsertShelf(shelf)
	})
	if err != nil {
		return nil, err
	}
	return shelf, nil
}

// RenameShelf changes a shelf's name. Items borrowed via the old name stop
// being borrowed; their ranks on this shelf are released.
func (s *Service) RenameShelf(ctx context.Context, id, name string) (*Shelf, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &ValidationError{"name", "required"}
	}
	var shelf *Shelf
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		if shelf, err = r.GetShelf(id); err != nil {
			return err
		}
		if err := checkNameFree(r, name, id); err != nil {
			return err
		}
		shelf.Name = name
		if err := r.UpdateShelf(shelf); err != nil {
			return err
		}
		return dropInvisibleRanks(r, shelf)
	})
	if err != nil {
		return nil, err
	}
	return shelf, nil
}

// ReorderShelves sets the display order. ids must name every shelf exactly once.
func (s *Service) ReorderShelves(ctx context.Context, ids []string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		shelves, err := r.ListShelves()
		if err != nil {
			return err
		}
		byID := make(map[string]*Shelf, len(shelves))
		for i := range shelves {
			byID[shelves[i].ID] = &shelves[i]
		}
		if len(ids) != len(shelves) {
			return &ValidationError{"ids", "must list every shelf exactly once"}
		}
		for i, id := range ids {
			shelf, ok := byID[id]
			if !ok {
				return &ValidationError{"ids", "must list every shelf exactly once"}
			}
			delete(byID, id)
			shelf.SortOrder = i + 1
			if err := r.UpdateShelf(shelf); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteShelf removes an empty shelf. Ranks held on it by borrowed items go with it.
func (s *Service) DeleteShelf(ctx context.Context, id string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		if _, err := r.GetShelf(id); err != nil {
			return err
		}
		n, err := r.CountItemsOnShelf(id)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrShelfNotEmpty
		}
		return r.DeleteShelf(id)
	})
}

// ListShelves returns all shelves in display order.
func (s *Service) ListShelves(ctx context.Context) ([]Shelf, error) {
	var shelves []Shelf
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		shelves, err = r.ListShelves()
		return err
	})
	return shelves, err
}

// ShelfItems returns the shelf's own pool and in_progress items plus every
// item from other shelves carrying a tag equal to this shelf's name (spec §3).
func (s *Service) ShelfItems(ctx context.Context, shelfID string) (*ShelfView, error) {
	var view *ShelfView
	err := s.store.Tx(ctx, func(r Repo) error {
		shelf, err := r.GetShelf(shelfID)
		if err != nil {
			return err
		}
		own, err := r.ListItemsByShelf(shelfID, StatePool, StateInProgress)
		if err != nil {
			return err
		}
		tagged, err := r.ListItemsByTag(shelf.Name, StatePool, StateInProgress)
		if err != nil {
			return err
		}
		ranks, err := r.ListRanks(shelfID)
		if err != nil {
			return err
		}

		items := make([]ShelfItem, 0, len(own)+len(tagged))
		for _, it := range own {
			items = append(items, ShelfItem{Item: it})
		}
		for _, it := range tagged {
			if it.ShelfID != shelfID { // own items tagged with their own shelf are not borrowed
				items = append(items, ShelfItem{Item: it, Borrowed: true})
			}
		}

		view = &ShelfView{Shelf: *shelf}
		slotOf := make(map[string]int, len(ranks))
		for _, rk := range ranks {
			slotOf[rk.ItemID] = rk.Slot
		}
		for i := range items {
			if slot, ok := slotOf[items[i].ID]; ok {
				view.Slots[slot-1] = &items[i]
			} else {
				view.Unranked = append(view.Unranked, items[i])
			}
		}
		sort.SliceStable(view.Unranked, func(i, j int) bool {
			return view.Unranked[i].CreatedAt.After(view.Unranked[j].CreatedAt)
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// checkNameFree returns ErrDuplicateShelf if another shelf (not exceptID) has the name.
func checkNameFree(r Repo, name, exceptID string) error {
	other, err := r.GetShelfByName(name)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil
	case err != nil:
		return err
	case other.ID != exceptID:
		return ErrDuplicateShelf
	}
	return nil
}
