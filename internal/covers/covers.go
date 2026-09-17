// Package covers keeps item cover images beside the library, so a cover is
// served from this machine rather than fetched from Open Library or YouTube
// every time a list is drawn. It knows nothing about handlers or templates.
//
// Nothing is fetched while an item is being filed: capture stays instant. A
// cover is fetched the first time the item is drawn, and is local ever
// after. A fetch that fails is remembered as a failure, so a dead link is
// not retried on every page draw.
package covers

import (
	"context"
	"fmt"
	"time"
)

// RetryAfter is how long a failed fetch is believed. Long enough that a
// dead link costs nothing to draw, short enough that an outage on the day
// an item was captured repairs itself.
const RetryAfter = 24 * time.Hour

// Cover is one item's image. MediaType and Bytes are empty when the last
// fetch failed. SourceURL is the cover link the bytes came from, so a
// changed link is fetched again at once.
type Cover struct {
	SourceURL string
	MediaType string
	Bytes     []byte
	FetchedAt time.Time
}

// Missing reports whether there is nothing to draw.
func (c *Cover) Missing() bool { return c == nil || c.MediaType == "" || len(c.Bytes) == 0 }

// Store holds the bytes. GetCover returns nil when nothing is held.
type Store interface {
	GetCover(ctx context.Context, itemID string) (*Cover, error)
	PutCover(ctx context.Context, itemID string, c Cover) error
}

// Fetcher reads an image from the web. It is metadata.Client in the binary.
type Fetcher interface {
	Image(ctx context.Context, rawURL string) (data []byte, mediaType string, err error)
}

// Cache fetches each cover once and keeps it.
type Cache struct {
	store Store
	fetch Fetcher
	now   func() time.Time
}

// New returns a Cache over a store and a fetcher.
func New(store Store, fetch Fetcher) *Cache {
	return &Cache{store: store, fetch: fetch, now: time.Now}
}

// Cover returns the bytes to serve for an item, fetching them when nothing
// is held, when the item's cover link has changed, or when an old failure
// is worth one more try. A nil cover and no error means there is nothing to
// draw; the page falls back to the item's blank plate.
func (c *Cache) Cover(ctx context.Context, itemID, url string) (*Cover, error) {
	if url == "" {
		return nil, nil
	}
	held, err := c.store.GetCover(ctx, itemID)
	if err != nil {
		return nil, err
	}
	switch {
	case held == nil, held.SourceURL != url:
		// Nothing held, or held for a link the item no longer has.
	case !held.Missing():
		return held, nil
	case c.now().Sub(held.FetchedAt) < RetryAfter:
		return nil, nil // a recent failure, believed for now
	}

	got := Cover{SourceURL: url, FetchedAt: c.now()}
	data, mediaType, ferr := c.fetch.Image(ctx, url)
	if ferr == nil {
		got.MediaType, got.Bytes = mediaType, data
	}
	if err := c.store.PutCover(ctx, itemID, got); err != nil {
		return nil, err
	}
	if ferr != nil {
		return nil, fmt.Errorf("covers: fetch %s: %w", url, ferr)
	}
	return &got, nil
}
