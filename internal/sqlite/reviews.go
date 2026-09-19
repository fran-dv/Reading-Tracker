package sqlite

import (
	"github.com/fran-dv/reading-tracker/internal/library"
)

func (r *repo) ListReviews() ([]library.Review, error) {
	rows, err := r.tx.Query(`SELECT week_of, closed_at FROM reviews ORDER BY week_of`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.Review
	index := map[string]int{} // week_of to its place in out
	for rows.Next() {
		var (
			rv           library.Review
			week, closed string
		)
		if err := rows.Scan(&week, &closed); err != nil {
			return nil, err
		}
		if rv.WeekOf, err = parseDay(week); err != nil {
			return nil, err
		}
		if rv.ClosedAt, err = parseTime(closed); err != nil {
			return nil, err
		}
		index[week] = len(out)
		out = append(out, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	needs, err := r.tx.Query(`SELECT week_of, campaign_id, books_left, avg_pages, pages_per_hour,
		weeks_left, weekly_hours FROM review_campaigns ORDER BY week_of, rowid`)
	if err != nil {
		return nil, err
	}
	defer needs.Close()
	for needs.Next() {
		var (
			n    library.Needs
			week string
		)
		if err := needs.Scan(&week, &n.CampaignID, &n.BooksLeft, &n.AvgPages, &n.PagesPerHour, &n.WeeksLeft, &n.WeeklyHours); err != nil {
			return nil, err
		}
		rv := &out[index[week]]
		rv.Needs = append(rv.Needs, n)
	}
	return out, needs.Err()
}

// PutReview writes a review and what it kept of each campaign, replacing
// the week's earlier review if there was one.
func (r *repo) PutReview(rv *library.Review) error {
	week := formatDay(rv.WeekOf)
	if _, err := r.tx.Exec(`DELETE FROM reviews WHERE week_of = ?`, week); err != nil {
		return err
	}
	if _, err := r.tx.Exec(`INSERT INTO reviews (week_of, closed_at) VALUES (?, ?)`, week, formatTime(rv.ClosedAt)); err != nil {
		return err
	}
	for _, n := range rv.Needs {
		_, err := r.tx.Exec(`INSERT INTO review_campaigns (week_of, campaign_id, books_left, avg_pages,
			pages_per_hour, weeks_left, weekly_hours) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			week, n.CampaignID, n.BooksLeft, n.AvgPages, n.PagesPerHour, n.WeeksLeft, n.WeeklyHours)
		if err != nil {
			return err
		}
	}
	return nil
}
