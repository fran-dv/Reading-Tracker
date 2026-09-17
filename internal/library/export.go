package library

import (
	"context"
	"time"
)

// ExportVersion identifies the export format. Bump it when the shape changes.
// Version 2 added active days, commitments, speed ramps and words_per_page;
// version 3 added campaigns; version 4 added weekly reviews; version 5 added
// when a session was edited; version 6 added the moments seen. Older files still import: version 1 with no
// plan and the default words per page, and each with none of what came
// after it.
const ExportVersion = 6

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
	Campaigns   []Campaign   `json:"campaigns"`
	Reviews     []Review     `json:"reviews"`
	MomentsSeen []MomentSeen `json:"moments_seen"`
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
		Campaigns:   []Campaign{},
		Reviews:     []Review{},
		MomentsSeen: []MomentSeen{},
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
		campaigns, err := r.ListCampaigns()
		if err != nil {
			return err
		}
		out.Campaigns = append(out.Campaigns, campaigns...)
		reviews, err := r.ListReviews()
		if err != nil {
			return err
		}
		out.Reviews = append(out.Reviews, reviews...)
		seen, err := r.ListMomentsSeen()
		if err != nil {
			return err
		}
		out.MomentsSeen = append(out.MomentsSeen, seen...)
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
	if in.Version < 1 || in.Version > ExportVersion {
		return &ValidationError{"version", "unsupported export version"}
	}
	if in.Version == 1 {
		in.Settings.WordsPerPage = defaultWordsPerPage
	}
	if err := in.Settings.validate(); err != nil {
		return err
	}
	return s.store.Tx(ctx, func(r Repo) error {
		empty, err := isEmpty(r)
		if err != nil {
			return err
		}
		if !empty {
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
		for i := range in.Campaigns {
			if err := r.InsertCampaign(&in.Campaigns[i]); err != nil {
				return err
			}
		}
		for i := range in.Reviews {
			if err := r.PutReview(&in.Reviews[i]); err != nil {
				return err
			}
		}
		for i := range in.MomentsSeen {
			if err := r.PutMomentSeen(&in.MomentsSeen[i]); err != nil {
				return err
			}
		}
		return r.UpdateSettings(&in.Settings)
	})
}

// isEmpty reports whether the library holds nothing an import would merge
// into: no shelves or items, and no plan, campaign, review or moment either.
func isEmpty(r Repo) (bool, error) {
	counts := []func() (int, error){
		func() (int, error) { s, err := r.ListShelves(); return len(s), err },
		func() (int, error) { s, err := r.ListItems(); return len(s), err },
		func() (int, error) { s, err := r.ListSessions(); return len(s), err },
		func() (int, error) { s, err := r.ListActiveDays(); return len(s), err },
		func() (int, error) { s, err := r.ListCommitments(); return len(s), err },
		func() (int, error) { s, err := r.ListSpeedRamps(); return len(s), err },
		func() (int, error) { s, err := r.ListCampaigns(); return len(s), err },
		func() (int, error) { s, err := r.ListReviews(); return len(s), err },
		func() (int, error) { s, err := r.ListMomentsSeen(); return len(s), err },
	}
	for _, count := range counts {
		n, err := count()
		if err != nil || n > 0 {
			return false, err
		}
	}
	return true, nil
}
