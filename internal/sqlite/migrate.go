package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

// migrate applies every migrations/*.sql file not yet recorded in
// schema_migrations, in filename order, each in its own transaction.
// Migrations are forward-only: never edit an applied file, add a new one.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("sqlite: create schema_migrations: %w", err)
	}
	entries, err := migrations.ReadDir("migrations") // sorted by filename
	if err != nil {
		return err
	}
	for _, entry := range entries {
		version := entry.Name()
		var applied bool
		err := db.QueryRow(`SELECT count(*) > 0 FROM schema_migrations WHERE version = ?`, version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("sqlite: check migration %s: %w", version, err)
		}
		if applied {
			continue
		}
		if err := apply(db, version); err != nil {
			return fmt.Errorf("sqlite: migration %s: %w", version, err)
		}
	}
	return nil
}

func apply(db *sql.DB, version string) error {
	script, err := migrations.ReadFile("migrations/" + version)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(string(script)); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, formatTime(time.Now()))
	if err != nil {
		return err
	}
	return tx.Commit()
}
