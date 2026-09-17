package sqlite

import (
	"database/sql"

	"github.com/fran-dv/reading-tracker/internal/library"
)

const campaignColumns = `id, name, target_count, started_on, deadline, ended_on`

func scanCampaign(row scanner) (*library.Campaign, error) {
	var (
		c                 library.Campaign
		started, deadline string
		ended             sql.NullString
	)
	if err := row.Scan(&c.ID, &c.Name, &c.TargetCount, &started, &deadline, &ended); err != nil {
		return nil, err
	}
	var err error
	if c.StartedOn, err = parseDay(started); err != nil {
		return nil, err
	}
	if c.Deadline, err = parseDay(deadline); err != nil {
		return nil, err
	}
	if ended.Valid {
		day, err := parseDay(ended.String)
		if err != nil {
			return nil, err
		}
		c.EndedOn = &day
	}
	return &c, nil
}

func (r *repo) ListCampaigns() ([]library.Campaign, error) {
	rows, err := r.tx.Query(`SELECT ` + campaignColumns + ` FROM campaigns ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *repo) GetCampaign(id string) (*library.Campaign, error) {
	c, err := scanCampaign(r.tx.QueryRow(`SELECT `+campaignColumns+` FROM campaigns WHERE id = ?`, id))
	return c, notFound(err)
}

func (r *repo) InsertCampaign(c *library.Campaign) error {
	_, err := r.tx.Exec(`INSERT INTO campaigns (`+campaignColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.TargetCount, formatDay(c.StartedOn), formatDay(c.Deadline), nullDay(c.EndedOn))
	return err
}

func (r *repo) UpdateCampaign(c *library.Campaign) error {
	return affected(r.tx.Exec(`UPDATE campaigns SET name = ?, ended_on = ? WHERE id = ?`,
		c.Name, nullDay(c.EndedOn), c.ID))
}
