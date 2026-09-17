// Command readingqueue runs the reading queue server on a local SQLite file.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/fran-dv/reading-tracker/internal/backup"
	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/metadata"
	"github.com/fran-dv/reading-tracker/internal/sqlite"
	"github.com/fran-dv/reading-tracker/internal/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	dbPath := flag.String("db", defaultDBPath(), "path to the SQLite database")
	importPath := flag.String("import", "", "load a JSON export into an empty database, then exit")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println(buildVersion())
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*addr, *dbPath, *importPath, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(addr, dbPath, importPath string, log *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	store, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	svc := library.New(store)

	if importPath != "" {
		if err := importFile(svc, importPath); err != nil {
			return err
		}
		log.Info("imported", "from", importPath, "db", dbPath)
		return nil
	}

	meta := metadata.New(nil)
	server := &http.Server{
		Addr:              addr,
		Handler:           web.New(svc, meta, covers.New(store, meta), log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go backup.Run(ctx, filepath.Join(filepath.Dir(dbPath), "backups"), store, svc, log)

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", "http://"+addr, "db", dbPath)
		errc <- server.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// importFile loads a JSON export (see GET /export) into the empty database.
func importFile(svc *library.Service, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var in library.Export
	if err := json.NewDecoder(f).Decode(&in); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return svc.Import(context.Background(), &in)
}

// defaultDBPath follows the XDG data directory convention.
func defaultDBPath() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "readingqueue.db"
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "readingqueue", "readingqueue.db")
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
