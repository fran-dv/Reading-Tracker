package library

import (
	"context"
	"sort"
	"time"
)

// Archive is everything finished or kept for reference (spec §6.6): what
// the reading has gained, newest first, with how it adds up.
type Archive struct {
	Items     []ArchivedItem // newest first
	Books     int            // finished books
	Other     int            // finished items that are not books
	Reference int            // kept for reference
	Pages     int            // pages of the finished books with a size
	Time      time.Duration  // every session on these items
}

// ArchivedItem is one closed item with the time it took.
type ArchivedItem struct {
	Item     Item
	Time     time.Duration
	Sessions int
	Days     int // from its start to its finish, counting both
}

// Archive gathers the finished archive.
func (s *Service) Archive(ctx context.Context) (*Archive, error) {
	var a *Archive
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		a = sn.archive(loc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (sn *snapshot) archive(loc *time.Location) *Archive {
	a := &Archive{}
	for _, it := range sn.items {
		if (it.State != StateFinished && it.State != StateReference) || it.FinishedAt == nil {
			continue
		}
		ai := ArchivedItem{Item: it, Sessions: len(sn.history[it.ID])}
		for _, s := range sn.history[it.ID] {
			ai.Time += s.Duration()
		}
		if it.StartedAt != nil {
			ai.Days = DaysBetween(dayOf(*it.StartedAt, loc), dayOf(*it.FinishedAt, loc))
		}
		switch {
		case it.State == StateReference:
			a.Reference++
		case it.Format == FormatBook:
			a.Books++
			if it.SizeUnit == UnitPages && it.SizeValue != nil {
				a.Pages += *it.SizeValue
			}
		default:
			a.Other++
		}
		a.Time += ai.Time
		a.Items = append(a.Items, ai)
	}
	sort.Slice(a.Items, func(i, j int) bool { return a.Items[i].Item.FinishedAt.After(*a.Items[j].Item.FinishedAt) })
	return a
}
