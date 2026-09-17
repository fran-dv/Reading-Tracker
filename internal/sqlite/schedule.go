package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// dayFormat stores calendar days; lexicographic order is chronological.
const dayFormat = "2006-01-02"

func formatDay(t time.Time) string { return t.Format(dayFormat) }

// nullDay converts an optional calendar day for binding.
func nullDay(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatDay(*t)
}

func parseDay(s string) (time.Time, error) {
	t, err := time.Parse(dayFormat, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlite: bad day %q: %w", s, err)
	}
	return t, nil
}

func (r *repo) ListActiveDays() ([]library.ActiveDays, error) {
	rows, err := r.tx.Query(`SELECT effective_on, days FROM active_days ORDER BY effective_on`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.ActiveDays
	for rows.Next() {
		var (
			a  library.ActiveDays
			on string
		)
		if err := rows.Scan(&on, &a.Days); err != nil {
			return nil, err
		}
		if a.EffectiveOn, err = parseDay(on); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *repo) PutActiveDays(a *library.ActiveDays) error {
	_, err := r.tx.Exec(`INSERT OR REPLACE INTO active_days (effective_on, days) VALUES (?, ?)`,
		formatDay(a.EffectiveOn), a.Days)
	return err
}

func (r *repo) ListCommitments() ([]library.Commitment, error) {
	rows, err := r.tx.Query(`SELECT effective_on, kind, minutes_per_day, start_minutes, increment_minutes, ceiling_minutes
		FROM commitments ORDER BY effective_on`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.Commitment
	for rows.Next() {
		var (
			c  library.Commitment
			on string
		)
		if err := rows.Scan(&on, &c.Kind, &c.MinutesPerDay, &c.StartMinutes, &c.IncrementMinutes, &c.CeilingMinutes); err != nil {
			return nil, err
		}
		if c.EffectiveOn, err = parseDay(on); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repo) PutCommitment(c *library.Commitment) error {
	_, err := r.tx.Exec(`INSERT OR REPLACE INTO commitments
		(effective_on, kind, minutes_per_day, start_minutes, increment_minutes, ceiling_minutes)
		VALUES (?, ?, ?, ?, ?, ?)`,
		formatDay(c.EffectiveOn), c.Kind, c.MinutesPerDay, c.StartMinutes, c.IncrementMinutes, c.CeilingMinutes)
	return err
}

func (r *repo) ListSpeedRamps() ([]library.SpeedRamp, error) {
	rows, err := r.tx.Query(`SELECT started_on, increment_percent, ceiling_percent, stopped_on FROM speed_ramps ORDER BY started_on`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.SpeedRamp
	for rows.Next() {
		var (
			sr      library.SpeedRamp
			on      string
			stopped sql.NullString
		)
		if err := rows.Scan(&on, &sr.IncrementPercent, &sr.CeilingPercent, &stopped); err != nil {
			return nil, err
		}
		if sr.StartedOn, err = parseDay(on); err != nil {
			return nil, err
		}
		if stopped.Valid {
			day, err := parseDay(stopped.String)
			if err != nil {
				return nil, err
			}
			sr.StoppedOn = &day
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}

func (r *repo) PutSpeedRamp(sr *library.SpeedRamp) error {
	_, err := r.tx.Exec(`INSERT OR REPLACE INTO speed_ramps (started_on, increment_percent, ceiling_percent, stopped_on)
		VALUES (?, ?, ?, ?)`, formatDay(sr.StartedOn), sr.IncrementPercent, sr.CeilingPercent, nullDay(sr.StoppedOn))
	return err
}
