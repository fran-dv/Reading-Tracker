package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Migration 010 moves what each review kept of the one active campaign
// into review_campaigns, and lets a second campaign start beside the first.
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
		want := library.Needs{CampaignID: "c1", BooksLeft: 98, AvgPages: 320, PagesPerHour: 30, WeeksLeft: 27.5, WeeklyHours: 7.2}
		if len(reviews) != 2 || len(reviews[0].Needs) != 0 || len(reviews[1].Needs) != 1 || reviews[1].Needs[0] != want {
			t.Fatalf("reviews %+v, want the second keeping c1's needs", reviews)
		}
		return r.InsertCampaign(&library.Campaign{ID: "c2", Name: "Two", TargetCount: 2,
			StartedOn: reviews[0].WeekOf, Deadline: reviews[1].WeekOf})
	})
	if err != nil {
		t.Fatalf("after the migration: %v", err)
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
			if err := r.InsertCampaign(&library.Campaign{ID: id, Name: id, TargetCount: 5, StartedOn: day, Deadline: day.AddDate(0, 1, 0)}); err != nil {
				return err
			}
		}
		needs := func(id string) library.Needs {
			return library.Needs{CampaignID: id, BooksLeft: 5, AvgPages: 300, PagesPerHour: 30, WeeksLeft: 4, WeeklyHours: 12.5}
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
