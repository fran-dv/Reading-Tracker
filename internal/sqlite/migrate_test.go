package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Migration 010 moves what each review kept of the one active campaign into
// review_campaigns, and lets a second campaign start beside the first; 011
// marks the campaigns that were there as counting books, and makes room for
// sets.
func TestMigrationKeepsReviewNeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rq.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "010") {
			break
		}
		if err := apply(db, e.Name()); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`
		INSERT INTO campaigns VALUES ('c1', 'A hundred', 100, '2026-09-01', '2027-03-22', NULL);
		INSERT INTO reviews VALUES ('2026-09-06', '2026-09-06T20:00:00.000Z', NULL, NULL, NULL, NULL, NULL, NULL);
		INSERT INTO reviews VALUES ('2026-09-13', '2026-09-13T20:00:00.000Z', 'c1', 98, 320, 30, 27.5, 7.2);`)
	if err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	err = store.Tx(context.Background(), func(r library.Repo) error {
		reviews, err := r.ListReviews()
		if err != nil {
			return err
		}
		want := library.Needs{CampaignID: "c1", Kind: library.KindCount, BooksLeft: 98, AvgPages: 320,
			PagesPerHour: 30, HoursLeft: 98 * 320.0 / 30, WeeksLeft: 27.5, WeeklyHours: 7.2}
		if len(reviews) != 2 || len(reviews[0].Needs) != 0 || len(reviews[1].Needs) != 1 || reviews[1].Needs[0] != want {
			t.Fatalf("reviews %+v, want the second keeping c1's needs", reviews)
		}
		campaigns, err := r.ListCampaigns()
		if err != nil {
			return err
		}
		if len(campaigns) != 1 || campaigns[0].Kind != library.KindCount || campaigns[0].TargetCount != 100 {
			t.Fatalf("campaigns %+v, want c1 counting 100 books", campaigns)
		}
		if _, err := r.ListCampaignItems(); err != nil {
			return err // the table a set's items live in
		}
		return r.InsertCampaign(&library.Campaign{ID: "c2", Name: "Two", Kind: library.KindCount, TargetCount: 2,
			StartedOn: reviews[0].WeekOf, Deadline: reviews[1].WeekOf})
	})
	if err != nil {
		t.Fatalf("after the migration: %v", err)
	}
}

// Migration 012 adds publisher and isbn to items (step 29); an item filed
// before it lands with both columns defaulted to the empty string.
func TestMigrationPublisherISBNDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rq.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "012") {
			break
		}
		if err := apply(db, e.Name()); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`
		INSERT INTO shelves VALUES ('s1', 'Shelf', 1, '2026-09-01T00:00:00.000Z');
		INSERT INTO items (id, title, url, author, format, shelf_id, why, verdict, abandoned_reason,
			focus_demand, size_value, size_unit, word_count, needs_desk, state, on_shortlist,
			created_at, updated_at, started_at, finished_at, cover_url)
		VALUES ('i1', 'Old Item', '', '', 'book', 's1', 'because', '', '',
			'medium', 300, 'pages', NULL, 0, 'pool', 0,
			'2026-09-01T00:00:00.000Z', '2026-09-01T00:00:00.000Z', NULL, NULL, '');`)
	if err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var publisher, isbn string
	if err := db.QueryRow(`SELECT publisher, isbn FROM items WHERE id = 'i1'`).Scan(&publisher, &isbn); err != nil {
		t.Fatal(err)
	}
	if publisher != "" || isbn != "" {
		t.Fatalf("publisher=%q isbn=%q, want both empty for a pre-existing row", publisher, isbn)
	}
}

// Migration 013 adds cover_choice to items (step 30b); an item filed
// before it lands with the default, found.
func TestMigrationCoverChoiceDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rq.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "013") {
			break
		}
		if err := apply(db, e.Name()); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`
		INSERT INTO shelves VALUES ('s1', 'Shelf', 1, '2026-09-01T00:00:00.000Z');
		INSERT INTO items (id, title, url, author, format, shelf_id, why, verdict, abandoned_reason,
			focus_demand, size_value, size_unit, word_count, needs_desk, state, on_shortlist,
			created_at, updated_at, started_at, finished_at, cover_url, publisher, isbn)
		VALUES ('i1', 'Old Item', '', '', 'book', 's1', 'because', '', '',
			'medium', 300, 'pages', NULL, 0, 'pool', 0,
			'2026-09-01T00:00:00.000Z', '2026-09-01T00:00:00.000Z', NULL, NULL, '', '', '');`)
	if err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var choice string
	if err := db.QueryRow(`SELECT cover_choice FROM items WHERE id = 'i1'`).Scan(&choice); err != nil {
		t.Fatal(err)
	}
	if choice != "found" {
		t.Fatalf("cover_choice=%q, want found for a pre-existing row", choice)
	}
}

// Closing a review again in the same week replaces what it kept.
func TestPutReviewReplacesTheWeek(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	err = store.Tx(ctx, func(r library.Repo) error {
		day, _ := parseDay("2026-09-13")
		for _, id := range []string{"a", "b"} {
			if err := r.InsertCampaign(&library.Campaign{ID: id, Name: id, Kind: library.KindCount, TargetCount: 5,
				StartedOn: day, Deadline: day.AddDate(0, 1, 0)}); err != nil {
				return err
			}
		}
		needs := func(id string) library.Needs {
			return library.Needs{CampaignID: id, Kind: library.KindCount, BooksLeft: 5, AvgPages: 300,
				PagesPerHour: 30, HoursLeft: 50, WeeksLeft: 4, WeeklyHours: 12.5}
		}
		if err := r.PutReview(&library.Review{WeekOf: day, ClosedAt: day, Needs: []library.Needs{needs("a"), needs("b")}}); err != nil {
			return err
		}
		if err := r.PutReview(&library.Review{WeekOf: day, ClosedAt: day.Add(1), Needs: []library.Needs{needs("b")}}); err != nil {
			return err
		}
		reviews, err := r.ListReviews()
		if err != nil {
			return err
		}
		if len(reviews) != 1 || len(reviews[0].Needs) != 1 || reviews[0].Needs[0].CampaignID != "b" {
			t.Fatalf("reviews %+v, want one keeping only b", reviews)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
