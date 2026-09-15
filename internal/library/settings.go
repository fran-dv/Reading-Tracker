package library

import (
	"context"
	"time"
)

// Settings is the single row of user-tunable values (spec §2.7). The
// migration inserts the defaults; there is no "no settings" state.
type Settings struct {
	Timezone              string // IANA name, or "Local"
	WIPCap                int
	StallDays             int
	ReviewWeekday         time.Weekday
	BucketQuickMaxMin     int
	BucketHourMinMin      int
	BucketHourMaxMin      int
	BucketLongMinMin      int
	PaceWindowDays        int
	ProjectionWindowWeeks int
	SeedPaceLight         int // pages per hour
	SeedPaceMedium        int
	SeedPaceDeep          int
	SeedPaceWPM           int
	FallbackBookPages     int
}

// Location resolves the configured timezone.
func (st Settings) Location() (*time.Location, error) {
	return time.LoadLocation(st.Timezone)
}

func (st Settings) validate() error {
	if _, err := st.Location(); err != nil {
		return &ValidationError{"timezone", "unknown location"}
	}
	positives := []struct {
		name  string
		value int
	}{
		{"wip_cap", st.WIPCap},
		{"stall_days", st.StallDays},
		{"bucket_quick_max_min", st.BucketQuickMaxMin},
		{"bucket_hour_min_min", st.BucketHourMinMin},
		{"bucket_hour_max_min", st.BucketHourMaxMin},
		{"bucket_long_min_min", st.BucketLongMinMin},
		{"pace_window_days", st.PaceWindowDays},
		{"projection_window_weeks", st.ProjectionWindowWeeks},
		{"seed_pace_light", st.SeedPaceLight},
		{"seed_pace_medium", st.SeedPaceMedium},
		{"seed_pace_deep", st.SeedPaceDeep},
		{"seed_pace_wpm", st.SeedPaceWPM},
		{"fallback_book_pages", st.FallbackBookPages},
	}
	for _, p := range positives {
		if p.value < 1 {
			return &ValidationError{p.name, "must be at least 1"}
		}
	}
	if st.ReviewWeekday < time.Sunday || st.ReviewWeekday > time.Saturday {
		return &ValidationError{"review_weekday", "must be 0 (Sunday) to 6 (Saturday)"}
	}
	if st.BucketHourMinMin > st.BucketHourMaxMin {
		return &ValidationError{"bucket_hour_min_min", "must not exceed bucket_hour_max_min"}
	}
	return nil
}

// Settings returns the current settings.
func (s *Service) Settings(ctx context.Context) (*Settings, error) {
	var st *Settings
	err := s.store.Tx(ctx, func(r Repo) error {
		var err error
		st, err = r.GetSettings()
		return err
	})
	return st, err
}

// UpdateSettings replaces the settings after validation.
func (s *Service) UpdateSettings(ctx context.Context, st Settings) error {
	if err := st.validate(); err != nil {
		return err
	}
	return s.store.Tx(ctx, func(r Repo) error { return r.UpdateSettings(&st) })
}
