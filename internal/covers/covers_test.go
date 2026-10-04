package covers

import (
	"context"
	"errors"
	"image/color"
	"testing"
	"time"
)

// memStore is the whole store in a map.
type memStore struct {
	held        map[string]Cover
	puts        int
	getErr      error
	putErr      error
	toNormalize []string // NormalizeHeld tests only
}

func newMemStore() *memStore { return &memStore{held: map[string]Cover{}} }

func (m *memStore) GetCover(_ context.Context, itemID string) (*Cover, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.held[itemID]
	if !ok {
		return nil, nil
	}
	return &c, nil
}

func (m *memStore) PutCover(_ context.Context, itemID string, c Cover) error {
	if m.putErr != nil {
		return m.putErr
	}
	m.puts++
	m.held[itemID] = c
	return nil
}

func (m *memStore) CoversToNormalize(context.Context, int) ([]string, error) {
	return m.toNormalize, nil
}

// fetcher counts what it was asked for and answers with what it was given.
type fetcher struct {
	data  []byte
	mime  string
	err   error
	calls []string
}

func (f *fetcher) Image(_ context.Context, rawURL string) ([]byte, string, error) {
	f.calls = append(f.calls, rawURL)
	if f.err != nil {
		return nil, "", f.err
	}
	return f.data, f.mime, nil
}

// at fixes the clock so the retry window can be crossed on purpose.
func at(c *Cache, t time.Time) { c.now = func() time.Time { return t } }

const url = "https://covers.example/1.jpg"

// fetched bytes are real (small, already-normalized) JPEGs: Cover runs
// everything it fetches through Normalize before writing it, so fake
// non-image bytes would come back as a remembered failure instead.
func fixtureA(t *testing.T) []byte { return encodeJPEG(t, 10, 10, color.RGBA{200, 50, 50, 255}) }
func fixtureB(t *testing.T) []byte { return encodeJPEG(t, 10, 10, color.RGBA{50, 50, 200, 255}) }

func TestCoverFetchesOnceAndKeepsIt(t *testing.T) {
	jpeg := fixtureA(t)
	store, fetch := newMemStore(), &fetcher{data: jpeg, mime: "image/jpeg"}
	c := New(store, fetch)

	got, err := c.Cover(context.Background(), "item", url, false)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(got.Bytes) != string(jpeg) || got.MediaType != "image/jpeg" {
		t.Fatalf("first: got %d bytes, %q", len(got.Bytes), got.MediaType)
	}

	// The second ask is answered from the store, without the network.
	got, err = c.Cover(context.Background(), "item", url, false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(got.Bytes) != string(jpeg) {
		t.Fatalf("second: got %d bytes, want the same cover", len(got.Bytes))
	}
	if len(fetch.calls) != 1 {
		t.Fatalf("fetched %d times, want 1", len(fetch.calls))
	}
}

func TestCoverRefetchesWhenTheLinkChanges(t *testing.T) {
	jpegA, jpegB := fixtureA(t), fixtureB(t)
	store, fetch := newMemStore(), &fetcher{data: jpegA, mime: "image/jpeg"}
	c := New(store, fetch)
	if _, err := c.Cover(context.Background(), "item", url, false); err != nil {
		t.Fatal(err)
	}

	fetch.data = jpegB
	got, err := c.Cover(context.Background(), "item", "https://covers.example/2.jpg", false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Bytes) != string(jpegB) {
		t.Fatalf("got %d bytes, want the new cover", len(got.Bytes))
	}
	if len(fetch.calls) != 2 {
		t.Fatalf("fetched %d times, want 2", len(fetch.calls))
	}
}

func TestCoverRemembersAFailureAndRetriesLater(t *testing.T) {
	store, fetch := newMemStore(), &fetcher{err: errors.New("404")}
	c := New(store, fetch)
	day := time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC)
	at(c, day)

	if _, err := c.Cover(context.Background(), "item", url, false); err == nil {
		t.Fatal("first: want the fetch error")
	}

	// Within the window the failure is believed: no error, nothing to draw.
	at(c, day.Add(RetryAfter-time.Minute))
	got, err := c.Cover(context.Background(), "item", url, false)
	if err != nil {
		t.Fatalf("within the window: %v", err)
	}
	if !got.Missing() {
		t.Fatalf("within the window: got %+v, want nothing to draw", got)
	}
	if len(fetch.calls) != 1 {
		t.Fatalf("fetched %d times, want 1", len(fetch.calls))
	}

	// Past it, one more try, and this time the cover is there.
	at(c, day.Add(RetryAfter+time.Minute))
	jpeg := fixtureA(t)
	fetch.err, fetch.data, fetch.mime = nil, jpeg, "image/jpeg"
	got, err = c.Cover(context.Background(), "item", url, false)
	if err != nil {
		t.Fatalf("after the window: %v", err)
	}
	if string(got.Bytes) != string(jpeg) {
		t.Fatalf("after the window: got %d bytes", len(got.Bytes))
	}
}

func TestCoverWithoutALinkFetchesNothing(t *testing.T) {
	store, fetch := newMemStore(), &fetcher{data: fixtureA(t), mime: "image/jpeg"}
	got, err := New(store, fetch).Cover(context.Background(), "item", "", false)
	if err != nil || got != nil {
		t.Fatalf("got %+v, %v; want nothing", got, err)
	}
	if len(fetch.calls) != 0 || store.puts != 0 {
		t.Fatalf("fetched %d, stored %d; want neither", len(fetch.calls), store.puts)
	}
}

func TestCoverFetchedBytesAreNormalized(t *testing.T) {
	// A fetched cover far over the normalized bound must come back
	// shrunk, not stored at source resolution (cover-management: Cover
	// Normalization on Write).
	oversized := encodeJPEG(t, 1200, 800, color.RGBA{10, 10, 10, 255})
	store, fetch := newMemStore(), &fetcher{data: oversized, mime: "image/jpeg"}
	c := New(store, fetch)

	got, err := c.Cover(context.Background(), "item", url, false)
	if err != nil {
		t.Fatalf("Cover() error = %v", err)
	}
	if len(got.Bytes) >= len(oversized) {
		t.Fatalf("stored %d bytes, want it normalized down from the fetched %d", len(got.Bytes), len(oversized))
	}
}

func TestCoverLockedNeverFetches(t *testing.T) {
	store, fetch := newMemStore(), &fetcher{data: fixtureA(t), mime: "image/jpeg"}
	c := New(store, fetch)

	// Nothing held yet: a locked ask still never reaches the network.
	got, err := c.Cover(context.Background(), "item", url, true)
	if err != nil || got != nil {
		t.Fatalf("locked, nothing held: got %+v, %v; want nothing", got, err)
	}
	if len(fetch.calls) != 0 {
		t.Fatalf("fetched %d times while locked, want 0", len(fetch.calls))
	}

	// Hold something directly (as SetCover would), bypassing the fetcher
	// entirely, then ask locked: the held bytes come back untouched, even
	// though the link below belongs to a different, unfetched image.
	held := Cover{SourceURL: "https://covers.example/held.jpg", MediaType: "image/jpeg", Bytes: fixtureB(t), FetchedAt: time.Now()}
	store.held["item"] = held

	got, err = c.Cover(context.Background(), "item", url, true)
	if err != nil {
		t.Fatalf("locked, held: %v", err)
	}
	if string(got.Bytes) != string(held.Bytes) {
		t.Fatalf("got %d bytes, want the held bytes untouched", len(got.Bytes))
	}
	if len(fetch.calls) != 0 {
		t.Fatalf("fetched %d times while locked, want 0", len(fetch.calls))
	}
}

// NormalizeHeld's behavior against real stored bytes (an oversized PNG,
// idempotency, a corrupted row that is left as-is) is covered end to
// end against a real sqlite.Store in normalizeheld_test.go. This test
// covers what only a fake store can show directly: a genuine store
// fault stops the pass instead of being silently swallowed alongside a
// merely unreadable image.
func TestNormalizeHeldStopsOnAStoreError(t *testing.T) {
	store := newMemStore()
	store.held["a"] = Cover{MediaType: "image/png", Bytes: encodePNGWithAlpha(t, 1200, 800)}
	store.held["b"] = Cover{MediaType: "image/png", Bytes: encodePNGWithAlpha(t, 1200, 800)}
	store.toNormalize = []string{"a", "b"}
	store.putErr = errors.New("disk full")

	c := New(store, &fetcher{})
	n, err := c.NormalizeHeld(context.Background())
	if err == nil {
		t.Fatal("NormalizeHeld() error = nil, want the store's write error")
	}
	if n != 0 {
		t.Fatalf("normalized = %d, want 0 (it must stop at the first write failure)", n)
	}
}
