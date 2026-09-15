package library

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// Tags returns an item's tags.
func (s *Service) Tags(ctx context.Context, itemID string) ([]string, error) {
	var tags []string
	err := s.store.Tx(ctx, func(r Repo) error {
		if _, err := r.GetItem(itemID); err != nil {
			return err
		}
		var err error
		tags, err = r.ListTags(itemID)
		return err
	})
	return tags, err
}

// SetTags replaces an item's tags. Tags are trimmed and de-duplicated
// case-insensitively. Removing a tag that names another shelf releases the
// item's rank on that shelf, since it is no longer borrowed there.
func (s *Service) SetTags(ctx context.Context, itemID string, tags []string) error {
	tags = normalizeTags(tags)
	return s.store.Tx(ctx, func(r Repo) error {
		item, err := r.GetItem(itemID)
		if err != nil {
			return err
		}
		old, err := r.ListTags(itemID)
		if err != nil {
			return err
		}
		if err := r.ReplaceTags(itemID, tags); err != nil {
			return err
		}
		for _, t := range old {
			if containsFold(tags, t) {
				continue
			}
			shelf, err := r.GetShelfByName(t)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if shelf.ID != item.ShelfID {
				if err := clearRanks(r, itemID, shelf.ID); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" && !containsFold(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(x string) bool { return strings.EqualFold(x, s) })
}
