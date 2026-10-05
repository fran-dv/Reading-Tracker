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
// when a session was edited; version 6 added the moments seen; version 7
// kept a review's needs for each campaign active then, not for one;
// version 8 added the kind of each campaign and a set's items; version 9
// added an item's publisher and isbn
// (step 29); version 10 added an item's cover_choice (step 30b). Older files
// still import: version 1 with no plan and the default words per page, and
// each with none of what came after it; an item from before version 10
// imports with cover_choice found, since it predates cover choices entirely.
const ExportVersion = 10

// defaultWordsPerPage matches the migration's default, for files older than it.
const defaultWordsPerPage = 300

// Export is the whole library as one JSON-serialisable document (spec §10),
// in the key order ExportTo writes it. Nothing builds one in production
// anymore — ExportTo streams the same shape straight to a writer, and
// ImportFrom reads it back the same way — but it stays as the document's
// declared type, so tests can decode an export into one value.
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
// and every later call becomes a no-op, so call sites never check a write
// error themselves; ExportTo's two loops that fetch one row at a time read
// d.err directly, only to stop fetching early once that happens.
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
				if dw.err != nil {
					return dw.err
				}
				tags, err := r.ListTags(it.ID)
				if err != nil {
					return err
				}
				put(ExportItem{Item: it, Tags: tags})
			}
			return nil
		})

		dw.array("ranks", func(put func(any)) error {
			for _, sh := range shelves {
				if dw.err != nil {
					return dw.err
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
