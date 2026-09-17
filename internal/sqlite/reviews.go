package sqlite

import (
	"database/sql"

	"github.com/fran-dv/reading-tracker/internal/library"
)

func (r *repo) ListReviews() ([]library.Review, error) {
	rows, err := r.tx.Query(`SELECT week_of, closed_at, campaign_id, books_left, avg_pages,
		pages_per_hour, weeks_left, weekly_hours FROM reviews ORDER BY week_of`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []library.Review
	for rows.Next() {
		var (
			rv                        library.Review
			week, closed              string
			campaign                  sql.NullString
			booksLeft                 sql.NullInt64
			pages, pace, weeks, hours sql.NullFloat64
		)
		if err := rows.Scan(&week, &closed, &campaign, &booksLeft, &pages, &pace, &weeks, &hours); err != nil {
			return nil, err
		}
		if rv.WeekOf, err = parseDay(week); err != nil {
			return nil, err
		}
		if rv.ClosedAt, err = parseTime(closed); err != nil {
			return nil, err
		}
		// The schema keeps these all set or all NULL.
		if campaign.Valid {
			rv.Needs = &library.Needs{
				CampaignID:   campaign.String,
				BooksLeft:    int(booksLeft.Int64),
				AvgPages:     pages.Float64,
				PagesPerHour: pace.Float64,
				WeeksLeft:    weeks.Float64,
				WeeklyHours:  hours.Float64,
			}
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

func (r *repo) PutReview(rv *library.Review) error {
	var campaign, booksLeft, pages, pace, weeks, hours any // NULL without a campaign
	if n := rv.Needs; n != nil {
		campaign, booksLeft, pages, pace, weeks, hours = n.CampaignID, n.BooksLeft, n.AvgPages, n.PagesPerHour, n.WeeksLeft, n.WeeklyHours
	}
	_, err := r.tx.Exec(`INSERT OR REPLACE INTO reviews (week_of, closed_at, campaign_id, books_left,
		avg_pages, pages_per_hour, weeks_left, weekly_hours) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		formatDay(rv.WeekOf), formatTime(rv.ClosedAt), campaign, booksLeft, pages, pace, weeks, hours)
	return err
}
