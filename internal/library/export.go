package library

import (
	"context"
	"time"
)

// ExportVersion identifies the export format. Bump it when the shape changes.
// Version 2 added active days, commitments, speed ramps and words_per_page;
// version 1 files still import, with no plan and the default words per page.
const ExportVersion = 2

// defaultWordsPerPage matches the migration's default, for files older than it.
const defaultWordsPerPage = 300

// Export is the whole library as one JSON-serialisable document (spec §10).
// IDs and timestamps are preserved so an import reconstructs the data exactly.
type Export struct {
	Version     int          `json:"version"`
	ExportedAt  time.Time    `json:"exported_at"`
	Settings    Settings     `json:"settings"`
	Shelves     []Shelf      `json:"shelves"`
	Items       []ExportItem `json:"items"`
	Ranks       []Rank       `json:"ranks"`
	Sessions    []Session    `json:"sessions"`
	ActiveDays  []ActiveDays `json:"active_days"`
	Commitments []Commitment `json:"commitments"`
	SpeedRamps  []SpeedRamp  `json:"speed_ramps"`
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
		Version:     ExportVersion,
		ExportedAt:  s.now(),
		Shelves:     []Shelf{},
		Items:       []ExportItem{},
		Ranks:       []Rank{},
		Sessions:    []Session{},
		ActiveDays:  []ActiveDays{},
		Commitments: []Commitment{},
		SpeedRamps:  []SpeedRamp{},
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
		days, err := r.ListActiveDays()
		if err != nil {
			return err
		}
		out.ActiveDays = append(out.ActiveDays, days...)
		commitments, err := r.ListCommitments()
		if err != nil {
			return err
		}
		out.Commitments = append(out.Commitments, commitments...)
		ramps, err := r.ListSpeedRamps()
		if err != nil {
			return err
		}
		out.SpeedRamps = append(out.SpeedRamps, ramps...)
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
	if in.Version != 1 && in.Version != ExportVersion {
		return &ValidationError{"version", "unsupported export version"}
	}
	if in.Version == 1 {
		in.Settings.WordsPerPage = defaultWordsPerPage
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
		for i := range in.ActiveDays {
			if err := r.PutActiveDays(&in.ActiveDays[i]); err != nil {
				return err
			}
		}
		for i := range in.Commitments {
			if err := r.PutCommitment(&in.Commitments[i]); err != nil {
				return err
			}
		}
		for i := range in.SpeedRamps {
			if err := r.PutSpeedRamp(&in.SpeedRamps[i]); err != nil {
				return err
			}
		}
		return r.UpdateSettings(&in.Settings)
	})
}
