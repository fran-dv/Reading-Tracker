package sqlite

import (
	"github.com/fran-dv/reading-tracker/internal/library"
)

const shelfCols = `id, name, sort_order, created_at`

func scanShelf(sc scanner) (*library.Shelf, error) {
	var (
		sh        library.Shelf
		createdAt string
	)
	if err := sc.Scan(&sh.ID, &sh.Name, &sh.SortOrder, &createdAt); err != nil {
		return nil, notFound(err)
	}
	var err error
	if sh.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &sh, nil
}

func (r *repo) InsertShelf(sh *library.Shelf) error {
	_, err := r.tx.Exec(`INSERT INTO shelves (`+shelfCols+`) VALUES (?, ?, ?, ?)`,
		sh.ID, sh.Name, sh.SortOrder, formatTime(sh.CreatedAt))
	return err
}

func (r *repo) UpdateShelf(sh *library.Shelf) error {
	return affected(r.tx.Exec(`UPDATE shelves SET name = ?, sort_order = ? WHERE id = ?`,
		sh.Name, sh.SortOrder, sh.ID))
}

func (r *repo) GetShelf(id string) (*library.Shelf, error) {
	return scanShelf(r.tx.QueryRow(`SELECT `+shelfCols+` FROM shelves WHERE id = ?`, id))
}

func (r *repo) GetShelfByName(name string) (*library.Shelf, error) {
	return scanShelf(r.tx.QueryRow(`SELECT `+shelfCols+` FROM shelves WHERE fold(name) = fold(?)`, name))
}

func (r *repo) ListShelves() ([]library.Shelf, error) {
	rows, err := r.tx.Query(`SELECT ` + shelfCols + ` FROM shelves ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shelves []library.Shelf
	for rows.Next() {
		sh, err := scanShelf(rows)
		if err != nil {
			return nil, err
		}
		shelves = append(shelves, *sh)
	}
	return shelves, rows.Err()
}

func (r *repo) DeleteShelf(id string) error {
	return affected(r.tx.Exec(`DELETE FROM shelves WHERE id = ?`, id))
}

func (r *repo) CountItemsOnShelf(shelfID string) (int, error) {
	var n int
	err := r.tx.QueryRow(`SELECT count(*) FROM items WHERE shelf_id = ?`, shelfID).Scan(&n)
	return n, err
}
