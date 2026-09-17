// Package sqlite implements library.Store on a SQLite file using the pure-Go
// modernc.org/sqlite driver.
package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"

	"modernc.org/sqlite"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// SQLite's NOCASE folds ASCII letters only, so "Álgebra" and "álgebra" would
// differ. fold(x) lowercases every letter; shelf names and tags compare
// through it.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("fold", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		s, _ := args[0].(string)
		return strings.ToLower(s), nil
	})
}

// Store is a SQLite-backed library.Store.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and applies any
// pending migrations.
func Open(path string) (*Store, error) {
	dsn := "file:" + path +
		"?_txlock=immediate" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Backup writes a consistent, compacted copy of the database to path using
// SQLite's VACUUM INTO. The file must not already exist.
func (s *Store) Backup(path string) error {
	if _, err := s.db.Exec(`VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("sqlite: backup to %s: %w", path, err)
	}
	return nil
}

// Tx runs fn inside one transaction, committing when it returns nil and
// rolling back otherwise.
func (s *Store) Tx(ctx context.Context, fn func(library.Repo) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin: %w", err)
	}
	if err := fn(&repo{tx: tx}); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// repo is the library.Repo bound to one transaction.
type repo struct {
	tx *sql.Tx
}
