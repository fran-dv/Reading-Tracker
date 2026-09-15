package sqlite

func (r *repo) ListTags(itemID string) ([]string, error) {
	rows, err := r.tx.Query(`SELECT tag FROM item_tags WHERE item_id = ? ORDER BY tag`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *repo) ReplaceTags(itemID string, tags []string) error {
	if _, err := r.tx.Exec(`DELETE FROM item_tags WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := r.tx.Exec(`INSERT INTO item_tags (item_id, tag) VALUES (?, ?)`, itemID, t); err != nil {
			return err
		}
	}
	return nil
}
