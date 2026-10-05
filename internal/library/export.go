package library

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"time"
)

// ExportVersion identifies the export format. Bump it when the shape changes.
// Version 2 added active days, commitments, speed ramps and words_per_page;
// version 3 added campaigns; version 4 added weekly reviews; version 5 added
// when a session was edited; version 6 added the moments seen; version 7 kept a review's needs for
// each campaign active then, not for one; version 8 added the kind of each
// campaign and a set's items; version 9 added an item's publisher and isbn
// (step 29); version 10 added an item's cover_choice (step 30b). Older files
// still import: version 1 with no plan and the default words per page, and
// each with none of what came after it; an item from before version 10
// imports with cover_choice found, since it predates cover choices entirely.
const ExportVersion = 10

// defaultWordsPerPage matches the migration's default, for files older than it.
const defaultWordsPerPage = 300

// Export is the whole library as one JSON-serialisable document (spec §10),
// in the key order ExportTo writes it.
// IDs and timestamps are preserved so an import reconstructs the data exactly.
type Export struct {
	Version       int            `json:"version"`
	ExportedAt    time.Time      `json:"exported_at"`
	Settings      Settings       `json:"settings"`
	Shelves       []Shelf        `json:"shelves"`
	Items         []ExportItem   `json:"items"`
	Ranks         []Rank         `json:"ranks"`
	Sessions      []Session      `json:"sessions"`
	ActiveDays    []ActiveDays   `json:"active_days"`
	Commitments   []Commitment   `json:"commitments"`
	SpeedRamps    []SpeedRamp    `json:"speed_ramps"`
	Campaigns     []Campaign     `json:"campaigns"`
	CampaignItems []CampaignItem `json:"campaign_items"`
	Reviews       []Review       `json:"reviews"`
	MomentsSeen   []MomentSeen   `json:"moments_seen"`
}

// ExportItem is an item with its tags inlined.
type ExportItem struct {
	Item
	Tags []string `json:"tags"`
}

// docWriter hand-writes the export document's object and array framing over
// a bufio.Writer, so sections can be streamed one element at a time instead
// of built up as a single value first. field and array each encode their
// value with encoding/json, one call at a time, and docWriter writes the
// braces, brackets, keys, commas and line breaks around them — one top-level
// field per line, one array element per line, so the file still reads like
// the in-memory encoder's output used to. It keeps the first error it meets
// and every later call becomes a no-op, so call sites never check an error
// themselves.
type docWriter struct {
	w     *bufio.Writer
	first bool // nothing written at the document's top level yet
	err   error
}

// newDocWriter opens the document's "{" and returns a docWriter ready for
// field and array calls.
func newDocWriter(w io.Writer) *docWriter {
	d := &docWriter{w: bufio.NewWriter(w), first: true}
	d.write([]byte("{\n"))
	return d
}

// write is the one place docWriter touches the underlying buffer: every
// later write or encode failure ends up here and is remembered as d.err.
func (d *docWriter) write(b []byte) {
	if d.err != nil {
		return
	}
	_, d.err = d.w.Write(b)
}

// encode marshals v compactly and writes it, remembering a marshal error
// the same way write remembers a write error.
func (d *docWriter) encode(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		if d.err == nil {
			d.err = err
		}
		return
	}
	d.write(b)
}

// sep writes the comma and newline that separate every field or element
// from the one before it; the first gets neither.
func (d *docWriter) sep() {
	if !d.first {
		d.write([]byte(",\n"))
	}
	d.first = false
}

// field writes one top-level "  \"name\": value" line.
func (d *docWriter) field(name string, v any) {
	d.sep()
	d.write([]byte(`  "` + name + `": `))
	d.encode(v)
}

// array writes one top-level "  \"name\": [...]" field, one compact
// element per line. fn calls put for each element, in order.
func (d *docWriter) array(name string, fn func(put func(v any)) error) {
	d.sep()
	d.write([]byte(`  "` + name + `": [`))
	firstElem := true
	put := func(v any) {
		if firstElem {
			d.write([]byte("\n"))
		} else {
			d.write([]byte(",\n"))
		}
		firstElem = false
		d.write([]byte("    "))
		d.encode(v)
	}
	if err := fn(put); err != nil && d.err == nil {
		d.err = err
	}
	if !firstElem {
		d.write([]byte("\n  "))
	}
	d.write([]byte("]"))
}

// Err reports the first error docWriter has met, if any. Loops that fetch
// one row at a time (items' tags, a shelf's ranks) check it between rows,
// so a write failure stops the fetching too, not just the writing.
func (d *docWriter) Err() error { return d.err }

// close writes the document's closing "}" and flushes the buffer. It
// returns the first error docWriter met, write or flush alike.
func (d *docWriter) close() error {
	d.write([]byte("\n}"))
	if ferr := d.w.Flush(); d.err == nil {
		d.err = ferr
	}
	return d.err
}

// arrayOf streams a slice already held in memory, element by element. Most
// sections are small enough to list whole before writing; only items (tags
// per item) and ranks (per shelf) need their own array call.
func arrayOf[T any](dw *docWriter, name string, xs []T) {
	dw.array(name, func(put func(any)) error {
		for _, x := range xs {
			put(x)
		}
		return nil
	})
}

// ExportTo streams the whole library as one JSON document to w, in the
// shape spec §10 describes and Export always declared: version first, then
// every section in today's dependency order. It runs inside a read
// snapshot (Store.Snapshot), so the download never blocks a write, and it
// never builds the document as one value: items are encoded with their
// tags as soon as ListTags returns, rather than collected into a second
// slice, and every other section is written straight from the one slice
// its List method already returned. A write failure — for instance a
// client that disconnects — is remembered on docWriter, so every later
// write is skipped; the two sections that fetch one row at a time (items'
// tags, a shelf's ranks) also stop fetching once that happens, and the
// first error this met comes back once the snapshot ends.
func (s *Service) ExportTo(ctx context.Context, w io.Writer) error {
	dw := newDocWriter(w)
	err := s.store.Snapshot(ctx, func(r Repo) error {
		dw.field("version", ExportVersion)
		dw.field("exported_at", s.now())

		settings, err := r.GetSettings()
		if err != nil {
			return err
		}
		dw.field("settings", settings)

		shelves, err := r.ListShelves()
		if err != nil {
			return err
		}
		arrayOf(dw, "shelves", shelves)

		items, err := r.ListItems()
		if err != nil {
			return err
		}
		dw.array("items", func(put func(any)) error {
			for _, it := range items {
				if err := dw.Err(); err != nil {
					return err
				}
				tags, err := r.ListTags(it.ID)
				if err != nil {
					return err
				}
				put(ExportItem{Item: it, Tags: tags})
			}
			return nil
		})
		if err := dw.Err(); err != nil {
			return err
		}

		dw.array("ranks", func(put func(any)) error {
			for _, sh := range shelves {
				if err := dw.Err(); err != nil {
					return err
				}
				ranks, err := r.ListRanks(sh.ID)
				if err != nil {
					return err
				}
				for _, rk := range ranks {
					put(rk)
				}
			}
			return nil
		})

		sessions, err := r.ListSessions()
		if err != nil {
			return err
		}
		arrayOf(dw, "sessions", sessions)

		days, err := r.ListActiveDays()
		if err != nil {
			return err
		}
		arrayOf(dw, "active_days", days)

		commitments, err := r.ListCommitments()
		if err != nil {
			return err
		}
		arrayOf(dw, "commitments", commitments)

		ramps, err := r.ListSpeedRamps()
		if err != nil {
			return err
		}
		arrayOf(dw, "speed_ramps", ramps)

		campaigns, err := r.ListCampaigns()
		if err != nil {
			return err
		}
		arrayOf(dw, "campaigns", campaigns)

		members, err := r.ListCampaignItems()
		if err != nil {
			return err
		}
		arrayOf(dw, "campaign_items", members)

		reviews, err := r.ListReviews()
		if err != nil {
			return err
		}
		arrayOf(dw, "reviews", reviews)

		seen, err := r.ListMomentsSeen()
		if err != nil {
			return err
		}
		arrayOf(dw, "moments_seen", seen)

		return nil
	})
	if err != nil {
		return err
	}
	return dw.close()
}

// Export reads everything in one transaction.
//
// Deprecated: superseded by ExportTo, which streams the same document
// instead of building it in memory first. It stays only until its
// remaining callers, the tests, migrate onto the streaming pair.
func (s *Service) Export(ctx context.Context) (*Export, error) {
	// Slices start empty, not nil, so an empty library exports as [] rather than null.
	out := &Export{
		Version:       ExportVersion,
		ExportedAt:    s.now(),
		Shelves:       []Shelf{},
		Items:         []ExportItem{},
		Ranks:         []Rank{},
		Sessions:      []Session{},
		ActiveDays:    []ActiveDays{},
		Commitments:   []Commitment{},
		SpeedRamps:    []SpeedRamp{},
		Campaigns:     []Campaign{},
		CampaignItems: []CampaignItem{},
		Reviews:       []Review{},
		MomentsSeen:   []MomentSeen{},
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
		members, err := r.ListCampaignItems()
		if err != nil {
			return err
		}
		out.CampaignItems = append(out.CampaignItems, members...)
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
	if in.Version < 8 {
		for i := range in.Campaigns {
			in.Campaigns[i].Kind = KindCount // every campaign counted books
		}
	}
	if in.Version < 10 {
		for i := range in.Items {
			in.Items[i].CoverChoice = CoverFound // predates cover choices entirely
		}
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
		for i := range in.CampaignItems {
			if err := r.InsertCampaignItem(&in.CampaignItems[i]); err != nil {
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
