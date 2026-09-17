package sqlite

import "github.com/fran-dv/reading-tracker/internal/library"

func (r *repo) ListMomentsSeen() ([]library.MomentSeen, error) {
	rows, err := r.tx.Query(`SELECT key, seen_at FROM moments_seen ORDER BY seen_at, key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.MomentSeen
	for rows.Next() {
		var (
			m    library.MomentSeen
			seen string
		)
		if err := rows.Scan(&m.Key, &seen); err != nil {
			return nil, err
		}
		if m.SeenAt, err = parseTime(seen); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *repo) PutMomentSeen(m *library.MomentSeen) error {
	_, err := r.tx.Exec(`INSERT OR REPLACE INTO moments_seen (key, seen_at) VALUES (?, ?)`, m.Key, formatTime(m.SeenAt))
	return err
}
