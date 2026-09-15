// Package library holds the domain model and rules of the reading queue:
// items, shelves, tags, rank slots, sessions and settings.
//
// It knows nothing about HTTP, templates or SQL. Persistence is reached
// through the Store and Repo interfaces; internal/sqlite implements them.
package library

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Store opens a unit of work. Every rule in this package runs inside one Tx
// so that reads and writes are atomic.
type Store interface {
	Tx(ctx context.Context, fn func(Repo) error) error
}

// Repo is the persistence surface bound to a single transaction. It is
// deliberately dumb: it stores and fetches, it does not decide.
type Repo interface {
	InsertItem(*Item) error
	UpdateItem(*Item) error
	GetItem(id string) (*Item, error)
	DeleteItem(id string) error
	CountItemsByState(State) (int, error)
	// ListItems returns every item, oldest first.
	ListItems() ([]Item, error)
	ListItemsByShelf(shelfID string, states ...State) ([]Item, error)
	// ListItemsByTag matches the tag case-insensitively.
	ListItemsByTag(tag string, states ...State) ([]Item, error)

	InsertShelf(*Shelf) error
	UpdateShelf(*Shelf) error
	GetShelf(id string) (*Shelf, error)
	// GetShelfByName matches the name case-insensitively.
	GetShelfByName(name string) (*Shelf, error)
	ListShelves() ([]Shelf, error)
	DeleteShelf(id string) error
	CountItemsOnShelf(shelfID string) (int, error)

	ListTags(itemID string) ([]string, error)
	ReplaceTags(itemID string, tags []string) error

	// ListRanks returns a shelf's ranks ordered by slot.
	ListRanks(shelfID string) ([]Rank, error)
	ListRanksForItem(itemID string) ([]Rank, error)
	ReplaceRanks(shelfID string, ranks []Rank) error

	InsertSession(*Session) error
	UpdateSession(*Session) error
	GetSession(id string) (*Session, error)
	// RunningSession returns nil, nil when no session is running.
	RunningSession() (*Session, error)
	// ListSessionsByItem returns sessions ordered by started_at ascending.
	ListSessionsByItem(itemID string) ([]Session, error)
	// ListSessions returns every session, oldest first.
	ListSessions() ([]Session, error)

	GetSettings() (*Settings, error)
	UpdateSettings(*Settings) error
}

// Service applies the domain rules on top of a Store.
type Service struct {
	store Store
	now   func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces the clock. Tests use it to freeze time.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New returns a Service backed by store.
func New(store Store, opts ...Option) *Service {
	s := &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// newID returns a random UUID v4 as a string.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("library: crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
