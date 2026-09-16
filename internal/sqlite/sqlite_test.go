package sqlite

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenMigratesAndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rq.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"shelves", "items", "item_tags", "shelf_ranks", "sessions", "settings", "schema_migrations"} {
		var n int
		if err := store.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("table %s missing after migration", table)
		}
	}
	var wipCap int
	if err := store.db.QueryRow(`SELECT wip_cap FROM settings WHERE id = 1`).Scan(&wipCap); err != nil || wipCap != 5 {
		t.Fatalf("settings row: wip_cap=%d err=%v, want 5", wipCap, err)
	}
	store.Close()

	store, err = Open(path) // second open must not re-apply anything
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store.Close()
	var applied int
	if err := store.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil || applied != 3 {
		t.Fatalf("schema_migrations rows=%d err=%v, want 3", applied, err)
	}
}

func TestPragmas(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var fk int
	if err := store.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys=%d err=%v, want 1", fk, err)
	}
	var journal string
	if err := store.db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil || !strings.EqualFold(journal, "wal") {
		t.Fatalf("journal_mode=%q err=%v, want wal", journal, err)
	}
}

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "rq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`INSERT INTO shelves VALUES ('s', 'S', 1, '2026-09-15T10:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}

	copyPath := filepath.Join(dir, "copy.db")
	if err := store.Backup(copyPath); err != nil {
		t.Fatal(err)
	}
	if err := store.Backup(copyPath); err == nil {
		t.Fatal("backup over an existing file must fail")
	}

	copied, err := Open(copyPath)
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	defer copied.Close()
	var name string
	if err := copied.db.QueryRow(`SELECT name FROM shelves`).Scan(&name); err != nil || name != "S" {
		t.Fatalf("copy is missing data: %q %v", name, err)
	}
}

// The schema itself guarantees a single running session, independently of the domain rule.
func TestOneRunningSessionIndex(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	exec := func(q string, args ...any) error { _, err := store.db.Exec(q, args...); return err }
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(exec(`INSERT INTO shelves VALUES ('s', 'S', 1, '2026-09-15T10:00:00.000Z')`))
	must(exec(`INSERT INTO items (id, title, format, shelf_id, why, focus_demand, size_unit, state, created_at, updated_at)
		VALUES ('i', 'T', 'book', 's', 'W', 'medium', 'pages', 'in_progress', '2026-09-15T10:00:00.000Z', '2026-09-15T10:00:00.000Z')`))
	must(exec(`INSERT INTO sessions (id, item_id, started_at) VALUES ('a', 'i', '2026-09-15T10:00:00.000Z')`))
	if err := exec(`INSERT INTO sessions (id, item_id, started_at) VALUES ('b', 'i', '2026-09-15T11:00:00.000Z')`); err == nil {
		t.Fatal("second running session must violate the unique index")
	}
	must(exec(`UPDATE sessions SET ended_at = '2026-09-15T10:30:00.000Z' WHERE id = 'a'`))
	must(exec(`INSERT INTO sessions (id, item_id, started_at) VALUES ('b', 'i', '2026-09-15T11:00:00.000Z')`))

	if err := exec(`DELETE FROM items WHERE id = 'i'`); err == nil {
		t.Fatal("deleting an item with sessions must be blocked by the foreign key")
	}
}
