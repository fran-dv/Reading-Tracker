package library

import (
	"context"
	"time"
)

// ExportVersion identifies the export format. Bump it when the shape changes.
const ExportVersion = 1

// Export is the whole library as one JSON-serialisable document (spec §10).
// IDs and timestamps are preserved so an import reconstructs the data exactly.
type Export struct {
	Version    int          `json:"version"`
	ExportedAt time.Time    `json:"exported_at"`
	Settings   Settings     `json:"settings"`
	Shelves    []Shelf      `json:"shelves"`
	Items      []ExportItem `json:"items"`
	Ranks      []Rank       `json:"ranks"`
	Sessions   []Session    `json:"sessions"`
}

// ExportItem is an item with its tags inlined.
type ExportItem struct {
	Item
	Tags []string `json:"tags"`
}

// Export reads everything in one transaction.
func (s *Service) Export(ctx context.Context) (*Export, error) {
	// Slices start empty, not nil, so an empty library exports as [] rather than null.
	out := &Export{
		Version:    ExportVersion,
		ExportedAt: s.now(),
		Shelves:    []Shelf{},
		Items:      []ExportItem{},
		Ranks:      []Rank{},
		Sessions:   []Session{},
	}
	err := s.store.Tx(ctx, func(r Repo) error {
		settings, err := r.GetSettings()
		if err != nil {
			return err
		}
		out.Settings = *settings
		shelves, err := r.ListShelves()
		if err != nil {
			return err
		}
		out.Shelves = append(out.Shelves, shelves...)
		items, err := r.ListItems()
		if err != nil {
			return err
		}
		for _, it := range items {
			tags, err := r.ListTags(it.ID)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, ExportItem{Item: it, Tags: tags})
		}
		for _, sh := range out.Shelves {
			ranks, err := r.ListRanks(sh.ID)
			if err != nil {
				return err
			}
			out.Ranks = append(out.Ranks, ranks...)
		}
		sessions, err := r.ListSessions()
		if err != nil {
			return err
		}
		out.Sessions = append(out.Sessions, sessions...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Import loads an export into an empty library. Schema constraints (foreign
// keys, enums, the single running session) guard the data; the caller gets
// the database error if the file is inconsistent.
func (s *Service) Import(ctx context.Context, in *Export) error {
	if in.Version != ExportVersion {
		return &ValidationError{"version", "unsupported export version"}
	}
	if err := in.Settings.validate(); err != nil {
		return err
	}
	return s.store.Tx(ctx, func(r Repo) error {
		shelves, err := r.ListShelves()
		if err != nil {
			return err
		}
		items, err := r.ListItems()
		if err != nil {
			return err
		}
		if len(shelves) > 0 || len(items) > 0 {
			return ErrNotEmpty
		}

		for i := range in.Shelves {
			if err := r.InsertShelf(&in.Shelves[i]); err != nil {
				return err
			}
		}
		for i := range in.Items {
			if err := r.InsertItem(&in.Items[i].Item); err != nil {
				return err
			}
			if err := r.ReplaceTags(in.Items[i].ID, in.Items[i].Tags); err != nil {
				return err
			}
		}
		byShelf := map[string][]Rank{}
		for _, rk := range in.Ranks {
			byShelf[rk.ShelfID] = append(byShelf[rk.ShelfID], rk)
		}
		for shelfID, ranks := range byShelf {
			if err := r.ReplaceRanks(shelfID, ranks); err != nil {
				return err
			}
		}
		for i := range in.Sessions {
			if err := r.InsertSession(&in.Sessions[i]); err != nil {
				return err
			}
		}
		return r.UpdateSettings(&in.Settings)
	})
}
