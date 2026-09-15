package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func day(s string) time.Time {
	d, err := time.Parse(dayFormat, s)
	if err != nil {
		panic(err)
	}
	return d
}

// 60 consecutive days ending Tuesday 2026-09-15. The 14 newest days run from
// 09-02; the 8 newest ISO weeks are 31–38, whose newest copies outside those
// 14 days are the Sundays 08-30, 08-23, 08-16, 08-09 and 08-02.
func TestStale(t *testing.T) {
	last := day("2026-09-15")
	var dates []time.Time
	for i := 59; i >= 0; i-- { // oldest first, to prove ordering does not matter
		dates = append(dates, last.AddDate(0, 0, -i))
	}

	want := map[string]bool{}
	for i := 0; i < 14; i++ {
		want[last.AddDate(0, 0, -i).Format(dayFormat)] = true
	}
	for _, s := range []string{"2026-08-30", "2026-08-23", "2026-08-16", "2026-08-09", "2026-08-02"} {
		want[s] = true
	}

	dropped := map[string]bool{}
	for _, d := range stale(dates) {
		dropped[d.Format(dayFormat)] = true
	}
	if len(dropped) != 60-len(want) {
		t.Fatalf("dropped %d, want %d", len(dropped), 60-len(want))
	}
	for s := range want {
		if dropped[s] {
			t.Errorf("%s should be kept", s)
		}
	}
}

func TestStaleFewDates(t *testing.T) {
	if got := stale([]time.Time{day("2026-09-15"), day("2026-09-14")}); len(got) != 0 {
		t.Fatalf("nothing should be dropped with two backups, got %v", got)
	}
}

// fakeSource writes a marker file instead of a database copy.
type fakeSource struct{ calls int }

func (f *fakeSource) Backup(path string) error {
	f.calls++
	return os.WriteFile(path, []byte("db"), 0o644)
}

func TestDaily(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "backups") // does not exist yet
	src := &fakeSource{}
	today := day("2026-09-15")

	// Seed 30 older daily copies plus a foreign file that must be left alone.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 30; i++ {
		name := prefix + today.AddDate(0, 0, -i).Format(dayFormat) + suffix
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Daily(dir, src, today); err != nil {
		t.Fatal(err)
	}
	if err := Daily(dir, src, today); err != nil { // same day: no second copy
		t.Fatal(err)
	}
	if src.calls != 1 {
		t.Fatalf("source called %d times, want 1", src.calls)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, e := range entries {
		kept = append(kept, e.Name())
	}
	// 14 daily (09-02..09-15) + weekly Sundays 08-30, 08-23, 08-16 (the oldest seeded day) + notes.txt
	if len(kept) != 14+3+1 {
		t.Fatalf("kept %d files, want 18: %v", len(kept), kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatal("foreign file was removed")
	}
	if _, err := os.Stat(filepath.Join(dir, prefix+"2026-09-15"+suffix)); err != nil {
		t.Fatal("today's backup missing")
	}
}
