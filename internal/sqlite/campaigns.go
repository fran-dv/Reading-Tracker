package sqlite

import (
	"database/sql"

	"github.com/fran-dv/reading-tracker/internal/library"
)

const campaignColumns = `id, name, kind, target_count, started_on, deadline, ended_on`

func scanCampaign(row scanner) (*library.Campaign, error) {
	var (
		c                 library.Campaign
		started, deadline string
		target            sql.NullInt64
		ended             sql.NullString
	)
	if err := row.Scan(&c.ID, &c.Name, &c.Kind, &target, &started, &deadline, &ended); err != nil {
		return nil, err
	}
	c.TargetCount = int(target.Int64) // NULL for a set: 0
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
	var target any // NULL for a set
	if c.Kind == library.KindCount {
		target = c.TargetCount
	}
	_, err := r.tx.Exec(`INSERT INTO campaigns (`+campaignColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Kind, target, formatDay(c.StartedOn), formatDay(c.Deadline), nullDay(c.EndedOn))
	return err
}

func (r *repo) ListCampaignItems() ([]library.CampaignItem, error) {
	rows, err := r.tx.Query(`SELECT campaign_id, item_id, added_on FROM campaign_items ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.CampaignItem
	for rows.Next() {
		var (
			m     library.CampaignItem
			added string
		)
		if err := rows.Scan(&m.CampaignID, &m.ItemID, &added); err != nil {
			return nil, err
		}
		if m.AddedOn, err = parseDay(added); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertCampaignItem adds an item to a set. Adding it twice leaves the day
// it was first added, so a second tap from a stale page changes nothing.
func (r *repo) InsertCampaignItem(m *library.CampaignItem) error {
	_, err := r.tx.Exec(`INSERT OR IGNORE INTO campaign_items (campaign_id, item_id, added_on) VALUES (?, ?, ?)`,
		m.CampaignID, m.ItemID, formatDay(m.AddedOn))
	return err
}

func (r *repo) UpdateCampaign(c *library.Campaign) error {
	return affected(r.tx.Exec(`UPDATE campaigns SET name = ?, ended_on = ? WHERE id = ?`,
		c.Name, nullDay(c.EndedOn), c.ID))
}
