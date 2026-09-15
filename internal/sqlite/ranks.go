package sqlite

import (
	"github.com/fran-dv/reading-tracker/internal/library"
)

func (r *repo) ListRanks(shelfID string) ([]library.Rank, error) {
	return r.queryRanks(`SELECT shelf_id, item_id, slot FROM shelf_ranks WHERE shelf_id = ? ORDER BY slot`, shelfID)
}

func (r *repo) ListRanksForItem(itemID string) ([]library.Rank, error) {
	return r.queryRanks(`SELECT shelf_id, item_id, slot FROM shelf_ranks WHERE item_id = ? ORDER BY shelf_id`, itemID)
}

func (r *repo) ReplaceRanks(shelfID string, ranks []library.Rank) error {
	if _, err := r.tx.Exec(`DELETE FROM shelf_ranks WHERE shelf_id = ?`, shelfID); err != nil {
		return err
	}
	for _, rk := range ranks {
		_, err := r.tx.Exec(`INSERT INTO shelf_ranks (shelf_id, item_id, slot) VALUES (?, ?, ?)`,
			shelfID, rk.ItemID, rk.Slot)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *repo) queryRanks(query string, arg any) ([]library.Rank, error) {
	rows, err := r.tx.Query(query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ranks []library.Rank
	for rows.Next() {
		var rk library.Rank
		if err := rows.Scan(&rk.ShelfID, &rk.ItemID, &rk.Slot); err != nil {
			return nil, err
		}
		ranks = append(ranks, rk)
	}
	return ranks, rows.Err()
}
