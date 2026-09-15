package sqlite

import (
	"database/sql"

	"github.com/fran-dv/reading-tracker/internal/library"
)

const itemCols = `id, title, url, author, format, shelf_id, why, verdict, abandoned_reason,
	focus_demand, size_value, size_unit, word_count, needs_desk, state, on_shortlist,
	created_at, updated_at, started_at, finished_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(sc scanner) (*library.Item, error) {
	var (
		it                         library.Item
		sizeValue, wordCount       sql.NullInt64
		createdAt, updatedAt       string
		startedAt, finishedAt      sql.NullString
		format, focus, unit, state string
	)
	err := sc.Scan(&it.ID, &it.Title, &it.URL, &it.Author, &format, &it.ShelfID, &it.Why, &it.Verdict,
		&it.AbandonedReason, &focus, &sizeValue, &unit, &wordCount, &it.NeedsDesk, &state, &it.OnShortlist,
		&createdAt, &updatedAt, &startedAt, &finishedAt)
	if err != nil {
		return nil, notFound(err)
	}
	it.Format, it.FocusDemand = library.Format(format), library.FocusDemand(focus)
	it.SizeUnit, it.State = library.SizeUnit(unit), library.State(state)
	it.SizeValue, it.WordCount = intPtr(sizeValue), intPtr(wordCount)
	if it.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if it.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	if it.StartedAt, err = parseNullTime(startedAt); err != nil {
		return nil, err
	}
	if it.FinishedAt, err = parseNullTime(finishedAt); err != nil {
		return nil, err
	}
	return &it, nil
}

func (r *repo) InsertItem(it *library.Item) error {
	_, err := r.tx.Exec(`INSERT INTO items (`+itemCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.ID, it.Title, it.URL, it.Author, string(it.Format), it.ShelfID, it.Why, it.Verdict, it.AbandonedReason,
		string(it.FocusDemand), nullInt(it.SizeValue), string(it.SizeUnit), nullInt(it.WordCount), it.NeedsDesk,
		string(it.State), it.OnShortlist, formatTime(it.CreatedAt), formatTime(it.UpdatedAt),
		nullTime(it.StartedAt), nullTime(it.FinishedAt))
	return err
}

func (r *repo) UpdateItem(it *library.Item) error {
	return affected(r.tx.Exec(`UPDATE items SET
		title = ?, url = ?, author = ?, format = ?, shelf_id = ?, why = ?, verdict = ?, abandoned_reason = ?,
		focus_demand = ?, size_value = ?, size_unit = ?, word_count = ?, needs_desk = ?, state = ?, on_shortlist = ?,
		updated_at = ?, started_at = ?, finished_at = ?
		WHERE id = ?`,
		it.Title, it.URL, it.Author, string(it.Format), it.ShelfID, it.Why, it.Verdict, it.AbandonedReason,
		string(it.FocusDemand), nullInt(it.SizeValue), string(it.SizeUnit), nullInt(it.WordCount), it.NeedsDesk,
		string(it.State), it.OnShortlist, formatTime(it.UpdatedAt), nullTime(it.StartedAt), nullTime(it.FinishedAt),
		it.ID))
}

func (r *repo) GetItem(id string) (*library.Item, error) {
	return scanItem(r.tx.QueryRow(`SELECT `+itemCols+` FROM items WHERE id = ?`, id))
}

func (r *repo) DeleteItem(id string) error {
	return affected(r.tx.Exec(`DELETE FROM items WHERE id = ?`, id))
}

func (r *repo) CountItemsByState(state library.State) (int, error) {
	var n int
	err := r.tx.QueryRow(`SELECT count(*) FROM items WHERE state = ?`, string(state)).Scan(&n)
	return n, err
}

func (r *repo) ListItems() ([]library.Item, error) {
	return r.queryItems(`SELECT ` + itemCols + ` FROM items ORDER BY created_at`)
}

func (r *repo) ListItemsByShelf(shelfID string, states ...library.State) ([]library.Item, error) {
	filter, args := stateFilter(states)
	return r.queryItems(`SELECT `+itemCols+` FROM items WHERE shelf_id = ?`+filter+` ORDER BY created_at DESC`,
		append([]any{shelfID}, args...)...)
}

func (r *repo) ListItemsByTag(tag string, states ...library.State) ([]library.Item, error) {
	filter, args := stateFilter(states)
	return r.queryItems(`SELECT `+itemCols+` FROM items
		WHERE id IN (SELECT item_id FROM item_tags WHERE tag = ?)`+filter+` ORDER BY created_at DESC`,
		append([]any{tag}, args...)...)
}

func (r *repo) queryItems(query string, args ...any) ([]library.Item, error) {
	rows, err := r.tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []library.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *it)
	}
	return items, rows.Err()
}
