package library_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
)

// A newly filed item always starts with the found choice, whatever cover a
// lookup found during capture (cover-management: Default choice is found).
func TestCreateItemDefaultCoverChoiceIsFound(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x", func(it *library.Item) {
		it.CoverChoice = library.CoverRemoved // must be ignored, like State
	})
	if item.CoverChoice != library.CoverFound {
		t.Fatalf("new item's choice = %q, want found", item.CoverChoice)
	}
}

func TestShowsCover(t *testing.T) {
	tests := []struct {
		choice library.CoverChoice
		url    string
		want   bool
	}{
		{library.CoverFound, "https://covers.example/1.jpg", true},
		{library.CoverFound, "", false},
		{library.CoverPicked, "", true},
		{library.CoverPicked, "https://covers.example/1.jpg", true},
		{library.CoverUploaded, "", true},
		{library.CoverRemoved, "https://covers.example/1.jpg", false},
		{library.CoverRemoved, "", false},
		{library.CoverChoice(""), "https://covers.example/1.jpg", false}, // unset, e.g. a zero-value Item
	}
	for _, tc := range tests {
		it := library.Item{CoverChoice: tc.choice, CoverURL: tc.url}
		if got := it.ShowsCover(); got != tc.want {
			t.Errorf("choice=%q url=%q: ShowsCover()=%v, want %v", tc.choice, tc.url, got, tc.want)
		}
	}
}

// SetCover's four scenarios (cover-management: One Cover Choice Per Item).
func TestSetCoverRules(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")

	t.Run("uploaded requires bytes", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "a")
		var verr *library.ValidationError
		_, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{MediaType: "image/jpeg"})
		if !errors.As(err, &verr) || verr.Field != "cover" {
			t.Fatalf("got %v, want a ValidationError on cover", err)
		}
		_, err = svc.SetCover(ctx, item.ID, library.CoverUploaded, nil)
		if !errors.As(err, &verr) || verr.Field != "cover" {
			t.Fatalf("nil image: got %v, want a ValidationError on cover", err)
		}
	})

	t.Run("uploaded with bytes succeeds", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "b")
		got, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{
			MediaType: "image/jpeg", Bytes: []byte("owner's bytes"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.CoverChoice != library.CoverUploaded {
			t.Fatalf("choice = %q, want uploaded", got.CoverChoice)
		}
	})

	t.Run("picked requires bytes and a source link", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "c")
		var verr *library.ValidationError
		_, err := svc.SetCover(ctx, item.ID, library.CoverPicked, &library.CoverImage{Bytes: []byte("x")})
		if !errors.As(err, &verr) || verr.Field != "cover" {
			t.Fatalf("no source link: got %v, want a ValidationError on cover", err)
		}
		_, err = svc.SetCover(ctx, item.ID, library.CoverPicked, &library.CoverImage{SourceURL: "https://covers.example/1.jpg"})
		if !errors.As(err, &verr) || verr.Field != "cover" {
			t.Fatalf("no bytes: got %v, want a ValidationError on cover", err)
		}
	})

	t.Run("picked with bytes and a source link succeeds", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "d")
		got, err := svc.SetCover(ctx, item.ID, library.CoverPicked, &library.CoverImage{
			SourceURL: "https://covers.example/1.jpg", MediaType: "image/jpeg", Bytes: []byte("edition bytes"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.CoverChoice != library.CoverPicked {
			t.Fatalf("choice = %q, want picked", got.CoverChoice)
		}
	})

	t.Run("found needs no image", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "e")
		got, err := svc.SetCover(ctx, item.ID, library.CoverFound, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.CoverChoice != library.CoverFound {
			t.Fatalf("choice = %q, want found", got.CoverChoice)
		}
	})

	t.Run("removed needs no image", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "f")
		got, err := svc.SetCover(ctx, item.ID, library.CoverRemoved, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.CoverChoice != library.CoverRemoved {
			t.Fatalf("choice = %q, want removed", got.CoverChoice)
		}
	})

	t.Run("unknown choice is refused", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "g")
		var verr *library.ValidationError
		_, err := svc.SetCover(ctx, item.ID, library.CoverChoice("gone"), nil)
		if !errors.As(err, &verr) || verr.Field != "cover_choice" {
			t.Fatalf("got %v, want a ValidationError on cover_choice", err)
		}
	})
}

// Every SetCover call bumps updated_at, the same as any other transition.
func TestSetCoverBumpsUpdatedAt(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x")
	before := item.UpdatedAt

	clk.Advance(time.Minute)
	got, err := svc.SetCover(ctx, item.ID, library.CoverRemoved, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.After(before) {
		t.Fatalf("updated_at %v did not advance past %v", got.UpdatedAt, before)
	}
}

// Editing an item never touches its cover choice; only SetCover does
// (mirrors TestUpdateItemKeepsTheShortlist).
func TestUpdateItemKeepsTheCoverChoice(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x")
	if _, err := svc.SetCover(ctx, item.ID, library.CoverRemoved, nil); err != nil {
		t.Fatal(err)
	}

	edited, err := svc.GetItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited.Title, edited.CoverChoice = "x, corrected", library.CoverFound // as a form without the field sends it
	got, err := svc.UpdateItem(ctx, *edited, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.CoverChoice != library.CoverRemoved {
		t.Fatalf("choice = %q after a plain edit, want removed kept", got.CoverChoice)
	}
}

// found and removed both discard any held cover bytes (cover-management:
// Remove the Cover; Automatic Lookups Never Override a Locked Choice).
func TestSetCoverDiscardsHeldBytes(t *testing.T) {
	svc, store, _ := newTestLibraryWithStore(t)
	shelf := newShelf(t, svc, "S")

	for _, choice := range []library.CoverChoice{library.CoverFound, library.CoverRemoved} {
		item := newItem(t, svc, shelf.ID, string(choice))
		if _, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{
			MediaType: "image/jpeg", Bytes: []byte("owner's bytes"),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SetCover(ctx, item.ID, choice, nil); err != nil {
			t.Fatal(err)
		}
		held, err := store.GetCover(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if held != nil {
			t.Errorf("choice %q: cover still held: %+v", choice, held)
		}
	}
}

// The lazy fetch cache's writer is guarded: it only lands while the item's
// choice is found, and is refused once the owner has locked it in
// (cover-management: Automatic Lookups Never Override a Locked Choice).
func TestGuardedPutCoverRefusedForLockedItem(t *testing.T) {
	svc, store, _ := newTestLibraryWithStore(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x")

	fetched := covers.Cover{SourceURL: "https://covers.example/1.jpg", MediaType: "image/jpeg",
		Bytes: []byte("fetched bytes"), FetchedAt: time.Now().UTC()}
	if err := store.PutCover(ctx, item.ID, fetched); err != nil {
		t.Fatal(err)
	}
	held, err := store.GetCover(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held == nil || string(held.Bytes) != "fetched bytes" {
		t.Fatalf("a found item should take a fetch: got %+v", held)
	}

	if _, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{
		MediaType: "image/jpeg", Bytes: []byte("owner's bytes"),
	}); err != nil {
		t.Fatal(err)
	}

	stale := covers.Cover{SourceURL: "https://covers.example/1.jpg", MediaType: "image/jpeg",
		Bytes: []byte("stale fetch"), FetchedAt: time.Now().UTC()}
	if err := store.PutCover(ctx, item.ID, stale); err != nil {
		t.Fatal(err)
	}
	held, err = store.GetCover(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held == nil || string(held.Bytes) != "owner's bytes" {
		t.Fatalf("a locked item's cover must survive a lazy fetch: got %+v", held)
	}
}

// Once removed, a cover stays removed through any number of "Look it up
// again" runs over multiple days (cover-management: Removed Stays Removed
// Indefinitely).
func TestRemovedCoverSurvivesRepeatedLookups(t *testing.T) {
	svc, store, clk := newTestLibraryWithStore(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x", func(it *library.Item) {
		it.CoverURL = "https://covers.example/1.jpg"
	})
	if _, err := svc.SetCover(ctx, item.ID, library.CoverRemoved, nil); err != nil {
		t.Fatal(err)
	}

	for day := 0; day < 3; day++ {
		clk.Advance(24 * time.Hour)
		candidate := covers.Cover{SourceURL: "https://covers.example/1.jpg", MediaType: "image/jpeg",
			Bytes: []byte("a fresh candidate"), FetchedAt: clk.Now()}
		if err := store.PutCover(ctx, item.ID, candidate); err != nil {
			t.Fatal(err)
		}
		held, err := store.GetCover(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if held != nil {
			t.Fatalf("day %d: a removed cover came back: %+v", day, held)
		}
		got, err := svc.GetItem(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CoverChoice != library.CoverRemoved || got.ShowsCover() {
			t.Fatalf("day %d: choice=%q ShowsCover=%v, want removed and no cover", day, got.CoverChoice, got.ShowsCover())
		}
	}
}

// An uploaded cover is never replaced by a fresh lookup candidate
// (cover-management: A fresh lookup does not replace an uploaded cover).
func TestUploadedCoverNotReplacedByFreshLookup(t *testing.T) {
	svc, store, _ := newTestLibraryWithStore(t)
	shelf := newShelf(t, svc, "S")
	item := newItem(t, svc, shelf.ID, "x")
	if _, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{
		MediaType: "image/jpeg", Bytes: []byte("owner's upload"),
	}); err != nil {
		t.Fatal(err)
	}

	candidate := covers.Cover{SourceURL: "https://covers.example/new.jpg", MediaType: "image/jpeg",
		Bytes: []byte("a new candidate"), FetchedAt: time.Now().UTC()}
	if err := store.PutCover(ctx, item.ID, candidate); err != nil {
		t.Fatal(err)
	}
	held, err := store.GetCover(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held == nil || string(held.Bytes) != "owner's upload" {
		t.Fatalf("an uploaded cover must survive a fresh lookup candidate: got %+v", held)
	}
}
