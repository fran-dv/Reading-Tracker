package covers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// memStore is the whole store in a map.
type memStore struct {
	held   map[string]Cover
	puts   int
	getErr error
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
	m.puts++
	m.held[itemID] = c
	return nil
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

func TestCoverFetchesOnceAndKeepsIt(t *testing.T) {
	store, fetch := newMemStore(), &fetcher{data: []byte("jpeg"), mime: "image/jpeg"}
	c := New(store, fetch)

	got, err := c.Cover(context.Background(), "item", url)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(got.Bytes) != "jpeg" || got.MediaType != "image/jpeg" {
		t.Fatalf("first: got %q %q", got.Bytes, got.MediaType)
	}

	// The second ask is answered from the store, without the network.
	got, err = c.Cover(context.Background(), "item", url)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(got.Bytes) != "jpeg" {
		t.Fatalf("second: got %q", got.Bytes)
	}
	if len(fetch.calls) != 1 {
		t.Fatalf("fetched %d times, want 1", len(fetch.calls))
	}
}

func TestCoverRefetchesWhenTheLinkChanges(t *testing.T) {
	store, fetch := newMemStore(), &fetcher{data: []byte("one"), mime: "image/jpeg"}
	c := New(store, fetch)
	if _, err := c.Cover(context.Background(), "item", url); err != nil {
		t.Fatal(err)
	}

	fetch.data = []byte("two")
	got, err := c.Cover(context.Background(), "item", "https://covers.example/2.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Bytes) != "two" {
		t.Fatalf("got %q, want the new cover", got.Bytes)
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

	if _, err := c.Cover(context.Background(), "item", url); err == nil {
		t.Fatal("first: want the fetch error")
	}

	// Within the window the failure is believed: no error, nothing to draw.
	at(c, day.Add(RetryAfter-time.Minute))
	got, err := c.Cover(context.Background(), "item", url)
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
	fetch.err, fetch.data, fetch.mime = nil, []byte("jpeg"), "image/jpeg"
	got, err = c.Cover(context.Background(), "item", url)
	if err != nil {
		t.Fatalf("after the window: %v", err)
	}
	if string(got.Bytes) != "jpeg" {
		t.Fatalf("after the window: got %q", got.Bytes)
	}
}

func TestCoverWithoutALinkFetchesNothing(t *testing.T) {
	store, fetch := newMemStore(), &fetcher{data: []byte("jpeg"), mime: "image/jpeg"}
	got, err := New(store, fetch).Cover(context.Background(), "item", "")
	if err != nil || got != nil {
		t.Fatalf("got %+v, %v; want nothing", got, err)
	}
	if len(fetch.calls) != 0 || store.puts != 0 {
		t.Fatalf("fetched %d, stored %d; want neither", len(fetch.calls), store.puts)
	}
}
