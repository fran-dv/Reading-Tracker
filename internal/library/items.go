package library

import (
	"context"
	"strings"
	"time"
)

// Format is what kind of material an item is.
type Format string

const (
	FormatBook    Format = "book"
	FormatVideo   Format = "video"
	FormatArticle Format = "article"
	FormatPaper   Format = "paper"
	FormatCourse  Format = "course"
)

// FocusDemand is how much concentration an item needs.
type FocusDemand string

const (
	FocusLight  FocusDemand = "light"
	FocusMedium FocusDemand = "medium"
	FocusDeep   FocusDemand = "deep"
)

// SizeUnit is the native unit an item's size and positions are measured in.
type SizeUnit string

const (
	UnitPages   SizeUnit = "pages"
	UnitMinutes SizeUnit = "minutes"
	UnitWords   SizeUnit = "words"
)

// State is where an item is in its lifecycle.
// Transitions: pool → in_progress → {finished | reference | abandoned}, plus pool → abandoned.
type State string

const (
	StatePool       State = "pool"
	StateInProgress State = "in_progress"
	StateFinished   State = "finished"
	StateReference  State = "reference"
	StateAbandoned  State = "abandoned"
)

// Item is one thing to read, watch or study. Optional strings are "" when absent.
type Item struct {
	ID              string
	Title           string
	URL             string
	Author          string
	Format          Format
	ShelfID         string
	Why             string
	Verdict         string
	AbandonedReason string
	FocusDemand     FocusDemand
	SizeValue       *int
	SizeUnit        SizeUnit
	WordCount       *int
	NeedsDesk       bool
	State           State
	OnShortlist     bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
}

// formatDefaults pre-fills focus, unit and desk need from the format (spec §4).
var formatDefaults = map[Format]struct {
	focus FocusDemand
	unit  SizeUnit
	desk  bool
}{
	FormatBook:    {FocusMedium, UnitPages, false},
	FormatVideo:   {FocusLight, UnitMinutes, false},
	FormatArticle: {FocusLight, UnitWords, false},
	FormatPaper:   {FocusDeep, UnitPages, true},
	FormatCourse:  {FocusMedium, UnitMinutes, true},
}

// applyDefaults fills zero-value shape fields from the format. NeedsDesk is
// only defaulted on creation, since false is a legitimate edited value.
func (it *Item) applyDefaults(creating bool) {
	d, ok := formatDefaults[it.Format]
	if !ok {
		return
	}
	if it.FocusDemand == "" {
		it.FocusDemand = d.focus
	}
	if it.SizeUnit == "" {
		it.SizeUnit = d.unit
	}
	if creating {
		it.NeedsDesk = d.desk
	}
}

func (it *Item) validate() error {
	it.Title = strings.TrimSpace(it.Title)
	it.Why = strings.TrimSpace(it.Why)
	switch {
	case it.Title == "":
		return &ValidationError{"title", "required"}
	case it.Why == "":
		return &ValidationError{"why", "required"}
	case it.ShelfID == "":
		return &ValidationError{"shelf_id", "required"}
	}
	if _, ok := formatDefaults[it.Format]; !ok {
		return &ValidationError{"format", "unknown value"}
	}
	switch it.FocusDemand {
	case FocusLight, FocusMedium, FocusDeep:
	default:
		return &ValidationError{"focus_demand", "unknown value"}
	}
	switch it.SizeUnit {
	case UnitPages, UnitMinutes, UnitWords:
	default:
		return &ValidationError{"size_unit", "unknown value"}
	}
	if it.Format == FormatBook && it.WordCount != nil {
		return &ValidationError{"word_count", "never set for books"}
	}
	return nil
}

// CreateItem files a new item in the pool. ID, State and timestamps on the
// input are ignored; missing shape fields are defaulted from the format.
func (s *Service) CreateItem(ctx context.Context, item Item) (*Item, error) {
	now := s.now()
	item.ID = newID()
	item.State = StatePool
	item.Verdict, item.AbandonedReason = "", ""
	item.CreatedAt, item.UpdatedAt = now, now
	item.StartedAt, item.FinishedAt = nil, nil
	item.applyDefaults(true)
	if err := item.validate(); err != nil {
		return nil, err
	}
	err := s.store.Tx(ctx, func(r Repo) error {
		if _, err := r.GetShelf(item.ShelfID); err != nil {
			return err
		}
		return r.InsertItem(&item)
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItem edits the descriptive fields of an item. State, verdict, reason
// and lifecycle timestamps are untouched; use the transition methods for those.
// Moving the item to another shelf drops its rank on the old shelf unless a
// tag keeps it visible there as borrowed.
func (s *Service) UpdateItem(ctx context.Context, item Item) (*Item, error) {
	var updated *Item
	err := s.store.Tx(ctx, func(r Repo) error {
		cur, err := r.GetItem(item.ID)
		if err != nil {
			return err
		}
		if _, err := r.GetShelf(item.ShelfID); err != nil {
			return err
		}
		oldShelf := cur.ShelfID
		cur.Title, cur.URL, cur.Author = item.Title, item.URL, item.Author
		cur.Format, cur.ShelfID, cur.Why = item.Format, item.ShelfID, item.Why
		cur.FocusDemand, cur.SizeValue, cur.SizeUnit = item.FocusDemand, item.SizeValue, item.SizeUnit
		cur.WordCount, cur.NeedsDesk, cur.OnShortlist = item.WordCount, item.NeedsDesk, item.OnShortlist
		cur.UpdatedAt = s.now()
		cur.applyDefaults(false)
		if err := cur.validate(); err != nil {
			return err
		}
		if err := r.UpdateItem(cur); err != nil {
			return err
		}
		if cur.ShelfID != oldShelf {
			stays, err := borrowedBy(r, cur.ID, oldShelf)
			if err != nil {
				return err
			}
			if !stays {
				if err := clearRanks(r, cur.ID, oldShelf); err != nil {
					return err
				}
			}
		}
		updated = cur
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// GetItem returns one item.
func (s *Service) GetItem(ctx context.Context, id string) (*Item, error) {
	var item *Item
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		item, err = r.GetItem(id)
		return err
	})
	return item, err
}

// DeleteItem hard-deletes an item. Items with session history cannot be
// deleted (ErrHasSessions); abandon them instead. Ranks on every shelf are
// released and shifted.
func (s *Service) DeleteItem(ctx context.Context, id string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		if _, err := r.GetItem(id); err != nil {
			return err
		}
		sessions, err := r.ListSessionsByItem(id)
		if err != nil {
			return err
		}
		if len(sessions) > 0 {
			return ErrHasSessions
		}
		if err := clearRanks(r, id); err != nil {
			return err
		}
		return r.DeleteItem(id)
	})
}

// Start moves a pool item to in_progress, subject to the WIP cap. The item
// leaves every rank slot it holds.
func (s *Service) Start(ctx context.Context, id string) (*Item, error) {
	return s.transition(ctx, id, func(r Repo, it *Item) error {
		if it.State != StatePool {
			return ErrInvalidTransition
		}
		settings, err := r.GetSettings()
		if err != nil {
			return err
		}
		active, err := r.CountItemsByState(StateInProgress)
		if err != nil {
			return err
		}
		if active >= settings.WIPCap {
			return ErrWIPCapReached
		}
		now := s.now()
		it.State = StateInProgress
		it.StartedAt = &now
		return clearRanks(r, it.ID)
	})
}

// Finish completes an in_progress item. The verdict is optional.
func (s *Service) Finish(ctx context.Context, id, verdict string) (*Item, error) {
	return s.complete(ctx, id, StateFinished, verdict)
}

// Reference closes an in_progress item as material consulted rather than
// completed. Counts for hours and stats, never toward the book campaign.
func (s *Service) Reference(ctx context.Context, id, verdict string) (*Item, error) {
	return s.complete(ctx, id, StateReference, verdict)
}

func (s *Service) complete(ctx context.Context, id string, to State, verdict string) (*Item, error) {
	return s.transition(ctx, id, func(_ Repo, it *Item) error {
		if it.State != StateInProgress {
			return ErrInvalidTransition
		}
		now := s.now()
		it.State = to
		it.Verdict = strings.TrimSpace(verdict)
		it.FinishedAt = &now
		return nil
	})
}

// Abandon closes a pool or in_progress item. A reason is required. The item
// leaves every rank slot it holds.
func (s *Service) Abandon(ctx context.Context, id, reason string) (*Item, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrReasonRequired
	}
	return s.transition(ctx, id, func(r Repo, it *Item) error {
		if it.State != StatePool && it.State != StateInProgress {
			return ErrInvalidTransition
		}
		now := s.now()
		it.State = StateAbandoned
		it.AbandonedReason = reason
		it.FinishedAt = &now
		return clearRanks(r, it.ID)
	})
}

// transition loads an item, lets change mutate it, and saves it.
func (s *Service) transition(ctx context.Context, id string, change func(Repo, *Item) error) (*Item, error) {
	var item *Item
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		item, err = r.GetItem(id)
		if err != nil {
			return err
		}
		if err := change(r, item); err != nil {
			return err
		}
		item.UpdatedAt = s.now()
		return r.UpdateItem(item)
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}
