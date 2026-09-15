package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// timeFormat is fixed-width so that lexicographic order equals chronological order.
const timeFormat = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeFormat) }

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeFormat, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlite: bad timestamp %q: %w", s, err)
	}
	return t, nil
}

// nullTime converts an optional time for binding.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func parseNullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// nullInt converts an optional int for binding.
func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func intPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

// stateFilter builds an " AND state IN (?, ?)" clause; empty states means no filter.
func stateFilter(states []library.State) (string, []any) {
	if len(states) == 0 {
		return "", nil
	}
	args := make([]any, len(states))
	for i, st := range states {
		args[i] = string(st)
	}
	return " AND state IN (?" + strings.Repeat(", ?", len(states)-1) + ")", args
}

// notFound maps sql.ErrNoRows to library.ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return library.ErrNotFound
	}
	return err
}

// affected returns library.ErrNotFound when a write touched no rows.
func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
