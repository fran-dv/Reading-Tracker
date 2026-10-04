// This file tests covers.Cache.NormalizeHeld against a real SQLite
// store, so it must live in an external test package: internal/sqlite
// already imports internal/covers (for the Cover type), and an internal
// test file (package covers) importing internal/sqlite back would be an
// import cycle. package covers_test has no such problem.
package covers_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/sqlite"
)

// noFetch is a Fetcher that must never be called: the startup pass only
// ever normalizes bytes already held, never fetches anything.
type noFetch struct{}

func (noFetch) Image(context.Context, string) ([]byte, string, error) {
	panic("NormalizeHeld must not fetch")
}

func newStoreWithItem(t *testing.T) (*sqlite.Store, string) {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rq.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	svc := library.New(store)
	ctx := context.Background()
	shelf, err := svc.CreateShelf(ctx, "Statistics")
	if err != nil {
		t.Fatal(err)
	}
	item, err := svc.CreateItem(ctx, library.Item{
		Title: "Thinking in Systems", Why: "loops everywhere",
		Format: library.FormatBook, ShelfID: shelf.ID,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return store, item.ID
}

func encodeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{60, 90, 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture png: %v", err)
	}
	return buf.Bytes()
}

func TestNormalizeHeldRewritesAnOversizedCover(t *testing.T) {
	store, itemID := newStoreWithItem(t)
	ctx := context.Background()
	original := encodeTestPNG(t, 1200, 800)
	if err := store.PutCover(ctx, itemID, covers.Cover{
		SourceURL: "https://covers.example/1.png", MediaType: "image/png",
		Bytes: original, FetchedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cache := covers.New(store, noFetch{})
	n, err := cache.NormalizeHeld(ctx)
	if err != nil {
		t.Fatalf("NormalizeHeld() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("normalized %d covers, want 1", n)
	}

	held, err := store.GetCover(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if held.MediaType != "image/jpeg" {
		t.Fatalf("media type = %q, want image/jpeg", held.MediaType)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(held.Bytes))
	if err != nil {
		t.Fatalf("decode normalized bytes: %v", err)
	}
	if cfg.Width != 600 || cfg.Height != 400 {
		t.Fatalf("got %dx%d, want 600x400 (scaled from 1200x800)", cfg.Width, cfg.Height)
	}
}

func TestNormalizeHeldIsIdempotent(t *testing.T) {
	store, itemID := newStoreWithItem(t)
	ctx := context.Background()
	if err := store.PutCover(ctx, itemID, covers.Cover{
		SourceURL: "https://covers.example/1.png", MediaType: "image/png",
		Bytes: encodeTestPNG(t, 1200, 800), FetchedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cache := covers.New(store, noFetch{})
	if _, err := cache.NormalizeHeld(ctx); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	afterFirst, err := store.GetCover(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}

	n, err := cache.NormalizeHeld(ctx)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if n != 0 {
		t.Fatalf("second pass normalized %d covers, want 0 (idempotent)", n)
	}
	afterSecond, err := store.GetCover(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterFirst.Bytes, afterSecond.Bytes) {
		t.Fatalf("second pass changed the stored bytes; want them untouched")
	}
}

func TestNormalizeHeldLeavesAnUnreadableCoverAsIsAndContinues(t *testing.T) {
	store, itemID := newStoreWithItem(t)
	ctx := context.Background()
	corrupt := []byte("this is not a decodable image")
	if err := store.PutCover(ctx, itemID, covers.Cover{
		SourceURL: "https://covers.example/1.png", MediaType: "image/png",
		Bytes: corrupt, FetchedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	// A second item, with a cover that is normalized successfully, so the
	// pass is proven to continue past the corrupted row rather than
	// stopping on it.
	svc := library.New(store)
	shelf, err := svc.CreateShelf(ctx, "Poetry")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := svc.CreateItem(ctx, library.Item{
		Title: "Leaves of Grass", Why: "a classic", Format: library.FormatBook, ShelfID: shelf.ID,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutCover(ctx, ok.ID, covers.Cover{
		SourceURL: "https://covers.example/2.png", MediaType: "image/png",
		Bytes: encodeTestPNG(t, 1200, 800), FetchedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cache := covers.New(store, noFetch{})
	n, err := cache.NormalizeHeld(ctx)
	if err != nil {
		t.Fatalf("NormalizeHeld() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("normalized %d covers, want 1 (only the decodable one)", n)
	}

	held, err := store.GetCover(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if held.MediaType != "image/png" || !bytes.Equal(held.Bytes, corrupt) {
		t.Fatalf("corrupted cover was changed; want it left exactly as it was")
	}
}
