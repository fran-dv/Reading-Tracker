package library

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// ImportFrom loads an export (see ExportTo) into an empty library, from a
// stream rather than a decoded value, so memory stays flat on an import of
// any size. Schema constraints (foreign keys, enums, the single running
// session) guard the data; the caller gets the database error if the file
// is inconsistent.
//
// The document's first key must be "version" — every file this app has
// ever written puts it there — and its value is range-checked before any
// transaction opens. Everything else runs inside one transaction: items are
// streamed and inserted one at a time with their tags; every other section
// is small enough to decode whole; settings is validated and written last,
// as it always was. Each section is inserted as it is read, in the order
// the file lays them out — the order ExportTo always writes them in — so a
// file with, say, ranks before items fails on a foreign key rather than
// being silently reordered. Any decode error, truncation, foreign-key
// failure or validation error rolls the whole transaction back, leaving
// the database exactly as it was (ErrNotEmpty guards that it was empty to
// begin with).
func (s *Service) ImportFrom(ctx context.Context, src io.Reader) error {
	dec := json.NewDecoder(bufio.NewReader(src))

	if err := expectDelim(dec, '{'); err != nil {
		return err
	}
	key, err := decodeKey(dec)
	if err != nil {
		return err
	}
	if key != "version" {
		return &ValidationError{"version", "the export's version must come first"}
	}
	var version int
	if err := dec.Decode(&version); err != nil {
		return &ValidationError{"version", "not a number"}
	}
	if version < 1 || version > ExportVersion {
		return &ValidationError{"version", "unsupported export version"}
	}

	return s.store.Tx(ctx, func(r Repo) error {
		empty, err := isEmpty(r)
		if err != nil {
			return err
		}
		if !empty {
			return ErrNotEmpty
		}

		var settings *Settings
		for dec.More() {
			key, err := decodeKey(dec)
			if err != nil {
				return err
			}
			switch key {
			case "exported_at":
				var at string // decoded and discarded; nothing reads it back
				if err := dec.Decode(&at); err != nil {
					return err
				}
			case "settings":
				settings = &Settings{}
				if err := dec.Decode(settings); err != nil {
					return err
				}
			case "shelves":
				var shelves []Shelf
				if err := dec.Decode(&shelves); err != nil {
					return err
				}
				for i := range shelves {
					if err := r.InsertShelf(&shelves[i]); err != nil {
						return err
					}
				}
			case "items":
				if err := importItems(dec, r, version); err != nil {
					return err
				}
			case "ranks":
				var ranks []Rank
				if err := dec.Decode(&ranks); err != nil {
					return err
				}
				byShelf := map[string][]Rank{}
				for _, rk := range ranks {
					byShelf[rk.ShelfID] = append(byShelf[rk.ShelfID], rk)
				}
				for shelfID, rs := range byShelf {
					if err := r.ReplaceRanks(shelfID, rs); err != nil {
						return err
					}
				}
			case "sessions":
				var sessions []Session
				if err := dec.Decode(&sessions); err != nil {
					return err
				}
				for i := range sessions {
					if err := r.InsertSession(&sessions[i]); err != nil {
						return err
					}
				}
			case "active_days":
				var days []ActiveDays
				if err := dec.Decode(&days); err != nil {
					return err
				}
				for i := range days {
					if err := r.PutActiveDays(&days[i]); err != nil {
						return err
					}
				}
			case "commitments":
				var commitments []Commitment
				if err := dec.Decode(&commitments); err != nil {
					return err
				}
				for i := range commitments {
					if err := r.PutCommitment(&commitments[i]); err != nil {
						return err
					}
				}
			case "speed_ramps":
				var ramps []SpeedRamp
				if err := dec.Decode(&ramps); err != nil {
					return err
				}
				for i := range ramps {
					if err := r.PutSpeedRamp(&ramps[i]); err != nil {
						return err
					}
				}
			case "campaigns":
				var campaigns []Campaign
				if err := dec.Decode(&campaigns); err != nil {
					return err
				}
				if version < 8 {
					for i := range campaigns {
						campaigns[i].Kind = KindCount // every campaign counted books
					}
				}
				for i := range campaigns {
					if err := r.InsertCampaign(&campaigns[i]); err != nil {
						return err
					}
				}
			case "campaign_items":
				var members []CampaignItem
				if err := dec.Decode(&members); err != nil {
					return err
				}
				for i := range members {
					if err := r.InsertCampaignItem(&members[i]); err != nil {
						return err
					}
				}
			case "reviews":
				var reviews []Review
				if err := dec.Decode(&reviews); err != nil {
					return err
				}
				for i := range reviews {
					if err := r.PutReview(&reviews[i]); err != nil {
						return err
					}
				}
			case "moments_seen":
				var seen []MomentSeen
				if err := dec.Decode(&seen); err != nil {
					return err
				}
				for i := range seen {
					if err := r.PutMomentSeen(&seen[i]); err != nil {
						return err
					}
				}
			default:
				// Forward compatibility within one version is not a goal;
				// an unknown key is simply not kept.
				var discard json.RawMessage
				if err := dec.Decode(&discard); err != nil {
					return err
				}
			}
		}
		if err := expectDelim(dec, '}'); err != nil {
			return err
		}
		if settings == nil {
			return &ValidationError{"settings", "missing"}
		}
		if version == 1 {
			settings.WordsPerPage = defaultWordsPerPage
		}
		if err := settings.validate(); err != nil {
			return err
		}
		return r.UpdateSettings(settings)
	})
}

// importItems decodes the "items" array one element at a time — expect
// '[', then while more remain, decode, insert and tag, then expect ']' —
// so an import never holds every item in memory at once.
func importItems(dec *json.Decoder, r Repo, version int) error {
	if err := expectDelim(dec, '['); err != nil {
		return err
	}
	for dec.More() {
		var it ExportItem
		if err := dec.Decode(&it); err != nil {
			return err
		}
		if version < 10 {
			it.CoverChoice = CoverFound // predates cover choices entirely
		}
		if err := r.InsertItem(&it.Item); err != nil {
			return err
		}
		if err := r.ReplaceTags(it.ID, it.Tags); err != nil {
			return err
		}
	}
	return expectDelim(dec, ']')
}

// expectDelim reads the next JSON token from dec and requires it to be the
// given delimiter.
func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return &ValidationError{"export", fmt.Sprintf("expected %q, got %v", want, tok)}
	}
	return nil
}

// decodeKey reads the next JSON token from dec and requires it to be an
// object key.
func decodeKey(dec *json.Decoder) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", err
	}
	key, ok := tok.(string)
	if !ok {
		return "", &ValidationError{"export", fmt.Sprintf("expected an object key, got %v", tok)}
	}
	return key, nil
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
