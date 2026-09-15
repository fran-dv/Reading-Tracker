package sqlite

import (
	"database/sql"
	"errors"

	"github.com/fran-dv/reading-tracker/internal/library"
)

const sessionCols = `id, item_id, started_at, ended_at, position_start, position_end, note, entered_retroactively`

func scanSession(sc scanner) (*library.Session, error) {
	var (
		s          library.Session
		startedAt  string
		endedAt    sql.NullString
		start, end sql.NullInt64
	)
	err := sc.Scan(&s.ID, &s.ItemID, &startedAt, &endedAt, &start, &end, &s.Note, &s.EnteredRetroactively)
	if err != nil {
		return nil, notFound(err)
	}
	if s.StartedAt, err = parseTime(startedAt); err != nil {
		return nil, err
	}
	if s.EndedAt, err = parseNullTime(endedAt); err != nil {
		return nil, err
	}
	s.PositionStart, s.PositionEnd = intPtr(start), intPtr(end)
	return &s, nil
}

func (r *repo) InsertSession(s *library.Session) error {
	_, err := r.tx.Exec(`INSERT INTO sessions (`+sessionCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ItemID, formatTime(s.StartedAt), nullTime(s.EndedAt),
		nullInt(s.PositionStart), nullInt(s.PositionEnd), s.Note, s.EnteredRetroactively)
	return err
}

func (r *repo) UpdateSession(s *library.Session) error {
	return affected(r.tx.Exec(`UPDATE sessions SET
		started_at = ?, ended_at = ?, position_start = ?, position_end = ?, note = ?, entered_retroactively = ?
		WHERE id = ?`,
		formatTime(s.StartedAt), nullTime(s.EndedAt), nullInt(s.PositionStart), nullInt(s.PositionEnd),
		s.Note, s.EnteredRetroactively, s.ID))
}

func (r *repo) GetSession(id string) (*library.Session, error) {
	return scanSession(r.tx.QueryRow(`SELECT `+sessionCols+` FROM sessions WHERE id = ?`, id))
}

func (r *repo) RunningSession() (*library.Session, error) {
	s, err := scanSession(r.tx.QueryRow(`SELECT ` + sessionCols + ` FROM sessions WHERE ended_at IS NULL`))
	if errors.Is(err, library.ErrNotFound) {
		return nil, nil
	}
	return s, err
}

func (r *repo) ListSessionsByItem(itemID string) ([]library.Session, error) {
	return r.querySessions(`SELECT `+sessionCols+` FROM sessions WHERE item_id = ? ORDER BY started_at`, itemID)
}

func (r *repo) ListSessions() ([]library.Session, error) {
	return r.querySessions(`SELECT ` + sessionCols + ` FROM sessions ORDER BY started_at`)
}

func (r *repo) querySessions(query string, args ...any) ([]library.Session, error) {
	rows, err := r.tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []library.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}
