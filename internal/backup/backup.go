// Package backup keeps dated copies of the database (spec §10): one file per
// day, retaining the last 14 daily and the last 8 weekly copies.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

const (
	prefix    = "readingqueue-"
	suffix    = ".db"
	dayFormat = "2006-01-02"

	keepDaily  = 14
	keepWeekly = 8
)

// Source produces a consistent copy of the database at path.
type Source interface {
	Backup(path string) error
}

// Run backs up once now and then checks every hour whether the current day
// (in the configured timezone) still lacks a backup. It returns when ctx ends.
func Run(ctx context.Context, dir string, src Source, svc *library.Service, log *slog.Logger) {
	for {
		if err := today(ctx, dir, src, svc); err != nil {
			log.Error("backup failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour):
		}
	}
}

func today(ctx context.Context, dir string, src Source, svc *library.Service) error {
	settings, err := svc.Settings(ctx)
	if err != nil {
		return err
	}
	loc, err := settings.Location()
	if err != nil {
		return err
	}
	return Daily(dir, src, time.Now().In(loc))
}

// Daily ensures a backup for day exists in dir, then deletes copies that
// fall outside the retention window.
func Daily(dir string, src Source, day time.Time) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, prefix+day.Format(dayFormat)+suffix)
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return nil // already have today's copy
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := src.Backup(path); err != nil {
		return err
	}
	return prune(dir)
}

func prune(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var dates []time.Time
	for _, e := range entries {
		if d, ok := parseName(e.Name()); ok {
			dates = append(dates, d)
		}
	}
	for _, d := range stale(dates) {
		if err := os.Remove(filepath.Join(dir, prefix+d.Format(dayFormat)+suffix)); err != nil {
			return err
		}
	}
	return nil
}

func parseName(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return time.Time{}, false
	}
	d, err := time.Parse(dayFormat, strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix))
	return d, err == nil
}

// stale returns the backup dates to delete: those that are neither among the
// newest keepDaily days nor the newest copy of one of the newest keepWeekly
// ISO weeks.
func stale(dates []time.Time) []time.Time {
	sort.Slice(dates, func(i, j int) bool { return dates[i].After(dates[j]) })
	keep := make(map[time.Time]bool, keepDaily+keepWeekly)
	for i := 0; i < len(dates) && i < keepDaily; i++ {
		keep[dates[i]] = true
	}
	weeks := map[string]bool{}
	for _, d := range dates { // newest first, so the first hit per week is its newest copy
		y, w := d.ISOWeek()
		key := fmt.Sprintf("%d-%02d", y, w)
		if weeks[key] {
			continue
		}
		if len(weeks) == keepWeekly {
			break
		}
		weeks[key] = true
		keep[d] = true
	}
	var out []time.Time
	for _, d := range dates {
		if !keep[d] {
			out = append(out, d)
		}
	}
	return out
}
