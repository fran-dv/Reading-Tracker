package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/fran-dv/reading-tracker/internal/covers"
)

// GetCover returns the stored cover, or nil when nothing was ever fetched
// for the item. Covers are read and written outside the library's
// transaction: they are a cache of someone else's bytes, not library state.
func (s *Store) GetCover(ctx context.Context, itemID string) (*covers.Cover, error) {
	var (
		c         covers.Cover
		fetchedAt string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT source_url, media_type, bytes, fetched_at FROM covers WHERE item_id = ?`, itemID).
		Scan(&c.SourceURL, &c.MediaType, &c.Bytes, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get cover %s: %w", itemID, err)
	}
	if c.FetchedAt, err = parseTime(fetchedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// PutCover stores what a fetch returned, replacing anything held for the
// item. Empty bytes record a failure so it is not retried on every draw.
func (s *Store) PutCover(ctx context.Context, itemID string, c covers.Cover) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO covers (item_id, source_url, media_type, bytes, fetched_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(item_id) DO UPDATE SET
		   source_url = excluded.source_url,
		   media_type = excluded.media_type,
		   bytes      = excluded.bytes,
		   fetched_at = excluded.fetched_at`,
		itemID, c.SourceURL, c.MediaType, c.Bytes, formatTime(c.FetchedAt))
	if err != nil {
		return fmt.Errorf("sqlite: put cover %s: %w", itemID, err)
	}
	return nil
}
