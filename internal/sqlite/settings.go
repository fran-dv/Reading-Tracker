package sqlite

import (
	"github.com/fran-dv/reading-tracker/internal/library"
)

const settingsCols = `timezone, wip_cap, stall_days, review_weekday,
	bucket_quick_max_min, bucket_hour_min_min, bucket_hour_max_min, bucket_long_min_min,
	pace_window_days, projection_window_weeks,
	seed_pace_light, seed_pace_medium, seed_pace_deep, seed_pace_wpm, fallback_book_pages`

func (r *repo) GetSettings() (*library.Settings, error) {
	var st library.Settings
	err := r.tx.QueryRow(`SELECT `+settingsCols+` FROM settings WHERE id = 1`).Scan(
		&st.Timezone, &st.WIPCap, &st.StallDays, &st.ReviewWeekday,
		&st.BucketQuickMaxMin, &st.BucketHourMinMin, &st.BucketHourMaxMin, &st.BucketLongMinMin,
		&st.PaceWindowDays, &st.ProjectionWindowWeeks,
		&st.SeedPaceLight, &st.SeedPaceMedium, &st.SeedPaceDeep, &st.SeedPaceWPM, &st.FallbackBookPages)
	if err != nil {
		return nil, notFound(err)
	}
	return &st, nil
}

func (r *repo) UpdateSettings(st *library.Settings) error {
	return affected(r.tx.Exec(`UPDATE settings SET
		timezone = ?, wip_cap = ?, stall_days = ?, review_weekday = ?,
		bucket_quick_max_min = ?, bucket_hour_min_min = ?, bucket_hour_max_min = ?, bucket_long_min_min = ?,
		pace_window_days = ?, projection_window_weeks = ?,
		seed_pace_light = ?, seed_pace_medium = ?, seed_pace_deep = ?, seed_pace_wpm = ?, fallback_book_pages = ?
		WHERE id = 1`,
		st.Timezone, st.WIPCap, st.StallDays, int(st.ReviewWeekday),
		st.BucketQuickMaxMin, st.BucketHourMinMin, st.BucketHourMaxMin, st.BucketLongMinMin,
		st.PaceWindowDays, st.ProjectionWindowWeeks,
		st.SeedPaceLight, st.SeedPaceMedium, st.SeedPaceDeep, st.SeedPaceWPM, st.FallbackBookPages))
}
