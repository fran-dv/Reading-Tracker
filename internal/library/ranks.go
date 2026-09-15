package library

import (
	"context"
	"slices"
)

// MaxSlots is the number of rank slots per shelf. Slot 1 is "next up".
const MaxSlots = 3

// Rank places an item in one of a shelf's slots. Slots are always contiguous
// from 1; removing an item shifts the ones below it up.
type Rank struct {
	ShelfID string `json:"shelf_id"`
	ItemID  string `json:"item_id"`
	Slot    int    `json:"slot"`
}

// Rank puts a pool item into a slot on a shelf where it is visible (home or
// borrowed). Items at or below that slot move down; whoever is pushed past
// slot 3 returns to the unranked pool.
func (s *Service) Rank(ctx context.Context, shelfID, itemID string, slot int) error {
	if slot < 1 || slot > MaxSlots {
		return &ValidationError{"slot", "must be between 1 and 3"}
	}
	return s.store.Tx(ctx, func(r Repo) error {
		item, err := r.GetItem(itemID)
		if err != nil {
			return err
		}
		if item.State != StatePool {
			return ErrInvalidTransition
		}
		visible, err := visibleOn(r, item, shelfID)
		if err != nil {
			return err
		}
		if !visible {
			return ErrNotVisibleOnShelf
		}
		ids, err := rankedIDs(r, shelfID)
		if err != nil {
			return err
		}
		ids = remove(ids, itemID)
		ids = slices.Insert(ids, min(slot-1, len(ids)), itemID)
		if len(ids) > MaxSlots {
			ids = ids[:MaxSlots]
		}
		return writeRanks(r, shelfID, ids)
	})
}

// Unrank removes an item from a shelf's slots and shifts the rest up.
func (s *Service) Unrank(ctx context.Context, shelfID, itemID string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		ids, err := rankedIDs(r, shelfID)
		if err != nil {
			return err
		}
		if !slices.Contains(ids, itemID) {
			return ErrNotFound
		}
		return writeRanks(r, shelfID, remove(ids, itemID))
	})
}

// MoveUp swaps a ranked item with the one above it. No-op at slot 1.
func (s *Service) MoveUp(ctx context.Context, shelfID, itemID string) error {
	return s.swap(ctx, shelfID, itemID, -1)
}

// MoveDown swaps a ranked item with the one below it. No-op at the last slot.
func (s *Service) MoveDown(ctx context.Context, shelfID, itemID string) error {
	return s.swap(ctx, shelfID, itemID, +1)
}

func (s *Service) swap(ctx context.Context, shelfID, itemID string, dir int) error {
	return s.store.Tx(ctx, func(r Repo) error {
		ids, err := rankedIDs(r, shelfID)
		if err != nil {
			return err
		}
		i := slices.Index(ids, itemID)
		if i < 0 {
			return ErrNotFound
		}
		j := i + dir
		if j < 0 || j >= len(ids) {
			return nil
		}
		ids[i], ids[j] = ids[j], ids[i]
		return writeRanks(r, shelfID, ids)
	})
}

// clearRanks releases an item's slot on the given shelves, or on every shelf
// when none are given, shifting each shelf's remaining slots up.
func clearRanks(r Repo, itemID string, shelfIDs ...string) error {
	if len(shelfIDs) == 0 {
		ranks, err := r.ListRanksForItem(itemID)
		if err != nil {
			return err
		}
		for _, rk := range ranks {
			shelfIDs = append(shelfIDs, rk.ShelfID)
		}
	}
	for _, shelfID := range shelfIDs {
		ids, err := rankedIDs(r, shelfID)
		if err != nil {
			return err
		}
		if !slices.Contains(ids, itemID) {
			continue
		}
		if err := writeRanks(r, shelfID, remove(ids, itemID)); err != nil {
			return err
		}
	}
	return nil
}

// dropInvisibleRanks releases the slots of items no longer visible on the shelf.
func dropInvisibleRanks(r Repo, shelf *Shelf) error {
	ranks, err := r.ListRanks(shelf.ID)
	if err != nil {
		return err
	}
	for _, rk := range ranks {
		item, err := r.GetItem(rk.ItemID)
		if err != nil {
			return err
		}
		visible, err := visibleOn(r, item, shelf.ID)
		if err != nil {
			return err
		}
		if !visible {
			if err := clearRanks(r, rk.ItemID, shelf.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// visibleOn reports whether the item is homed on or borrowed by the shelf.
func visibleOn(r Repo, item *Item, shelfID string) (bool, error) {
	if item.ShelfID == shelfID {
		return true, nil
	}
	return borrowedBy(r, item.ID, shelfID)
}

// borrowedBy reports whether the item carries a tag equal to the shelf's name.
func borrowedBy(r Repo, itemID, shelfID string) (bool, error) {
	shelf, err := r.GetShelf(shelfID)
	if err != nil {
		return false, err
	}
	tags, err := r.ListTags(itemID)
	if err != nil {
		return false, err
	}
	return containsFold(tags, shelf.Name), nil
}

// rankedIDs returns the shelf's item IDs ordered by slot.
func rankedIDs(r Repo, shelfID string) ([]string, error) {
	ranks, err := r.ListRanks(shelfID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(ranks))
	for i, rk := range ranks {
		ids[i] = rk.ItemID
	}
	return ids, nil
}

// writeRanks stores ids as slots 1..n on the shelf.
func writeRanks(r Repo, shelfID string, ids []string) error {
	ranks := make([]Rank, len(ids))
	for i, id := range ids {
		ranks[i] = Rank{ShelfID: shelfID, ItemID: id, Slot: i + 1}
	}
	return r.ReplaceRanks(shelfID, ranks)
}

func remove(ids []string, id string) []string {
	return slices.DeleteFunc(slices.Clone(ids), func(x string) bool { return x == id })
}
