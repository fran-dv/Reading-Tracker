package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
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

// CoversToNormalize returns the item IDs of every held cover worth a
// look from the startup pass: it holds bytes, and either its media type
// isn't already image/jpeg or it is still over overBytes. A remembered
// failure (no bytes held) never matches, and neither does a cover a
// previous pass already normalized — the query alone makes the pass
// idempotent, with no marker table to keep in step.
func (s *Store) CoversToNormalize(ctx context.Context, overBytes int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id FROM covers
		 WHERE length(bytes) > 0 AND (media_type <> 'image/jpeg' OR length(bytes) > ?)`,
		overBytes)
	if err != nil {
		return nil, fmt.Errorf("sqlite: covers to normalize: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("sqlite: covers to normalize: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: covers to normalize: %w", err)
	}
	return ids, nil
}

// PutCover stores what a fetch returned, replacing anything held for the
// item. Empty bytes record a failure so it is not retried on every draw.
//
// Guarded: the write only lands while the item's cover_choice is still
// found. A picked, uploaded or removed cover is owner data, not a cache to
// refresh, so a lazy fetch landing after the owner's choice changed must
// not clobber it (cover-management: Automatic Lookups Never Override a
// Locked Choice). The SELECT guards the insert (nothing to insert when the
// item isn't found), and the ON CONFLICT WHERE guards the update the same
// way when a covers row already exists.
func (s *Store) PutCover(ctx context.Context, itemID string, c covers.Cover) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO covers (item_id, source_url, media_type, bytes, fetched_at)
		 SELECT ?, ?, ?, ?, ? FROM items WHERE id = ? AND cover_choice = 'found'
		 ON CONFLICT(item_id) DO UPDATE SET
		   source_url = excluded.source_url,
		   media_type = excluded.media_type,
		   bytes      = excluded.bytes,
		   fetched_at = excluded.fetched_at
		 WHERE (SELECT cover_choice FROM items WHERE id = excluded.item_id) = 'found'`,
		itemID, c.SourceURL, c.MediaType, c.Bytes, formatTime(c.FetchedAt), itemID)
	if err != nil {
		return fmt.Errorf("sqlite: put cover %s: %w", itemID, err)
	}
	return nil
}

// PutCoverImage stores a cover the owner chose — a pick or an upload —
// replacing anything held for the item. Unlike PutCover, this always
// writes: it runs inside the library's own transaction as the one place
// owner data is meant to change (cover-management: One Cover Choice Per
// Item).
func (r *repo) PutCoverImage(img *library.CoverImage) error {
	_, err := r.tx.Exec(`INSERT INTO covers (item_id, source_url, media_type, bytes, fetched_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(item_id) DO UPDATE SET
		   source_url = excluded.source_url,
		   media_type = excluded.media_type,
		   bytes      = excluded.bytes,
		   fetched_at = excluded.fetched_at`,
		img.ItemID, img.SourceURL, img.MediaType, img.Bytes, formatTime(img.FetchedAt))
	return err
}

// DeleteCoverImage discards whatever cover is held for the item, so a
// discarded upload or pick is not kept (cover-management: Remove the
// Cover).
func (r *repo) DeleteCoverImage(itemID string) error {
	_, err := r.tx.Exec(`DELETE FROM covers WHERE item_id = ?`, itemID)
	return err
}
