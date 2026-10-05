package library_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// ExportTo's output must decode strictly into Export: no field it writes
// is a surprise to the type both ImportFrom's tests and -import rely on.
func TestExportToShapeIsStrict(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)

	var buf bytes.Buffer
	if err := svc.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	dec.DisallowUnknownFields()
	var out library.Export
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("ExportTo output has a field Export doesn't declare: %v", err)
	}
}

// An empty library's sections are "[]", never "null", so an element added
// later always has somewhere to go when read back with plain JSON tools.
func TestExportToEmptyLibraryArraysNotNull(t *testing.T) {
	svc, _ := newTestLibrary(t)

	var buf bytes.Buffer
	if err := svc.ExportTo(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"shelves", "items", "ranks", "sessions", "active_days", "commitments",
		"speed_ramps", "campaigns", "campaign_items", "reviews", "moments_seen",
	} {
		if got := string(doc[key]); got != "[]" {
			t.Errorf("%s = %s, want []", key, got)
		}
	}
}

// failAfter lets the first n bytes through, then fails every write after
// that, so ExportTo must stop partway through a real document without
// panicking — not just fail before writing anything.
type failAfter struct{ n int }

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("boom")
	}
	if len(p) <= f.n {
		f.n -= len(p)
		return len(p), nil
	}
	n := f.n
	f.n = 0
	return n, errors.New("boom") // a short write must carry an error
}

func TestExportToReportsAFailingWriter(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)

	for _, n := range []int{0, 100, 500} {
		if err := svc.ExportTo(ctx, &failAfter{n: n}); err == nil {
			t.Fatalf("ExportTo with a writer failing after %d bytes: want an error, got nil", n)
		}
	}
}

// signalFirstWrite wraps a writer and closes started the moment its first
// Write is called — before delegating to w, not after — so a caller can
// tell exactly when the wrapped writer first attempted to write, even if
// that attempt then blocks.
type signalFirstWrite struct {
	w       io.Writer
	once    sync.Once
	started chan struct{}
}

func (s *signalFirstWrite) Write(p []byte) (int, error) {
	s.once.Do(func() { close(s.started) })
	return s.w.Write(p)
}

// The snapshot ExportTo runs in must never block a writer: it uses a
// second, deferred-transaction connection, not the one Tx holds the write
// lock on. Writing into an io.Pipe nobody reads yet forces ExportTo to
// stall the moment its bufio buffer first flushes, since a pipe write
// only returns once something reads. signalFirstWrite reports that moment
// precisely instead of guessing at it with a sleep, so the test knows
// ExportTo is genuinely stuck before it runs a concurrent CreateItem and
// checks that it still lands well inside busy_timeout.
func TestExportToSnapshotDoesNotBlockWriters(t *testing.T) {
	svc, clk := newTestLibrary(t)
	populate(t, svc, clk)
	shelves, err := svc.ListShelves(ctx)
	if err != nil || len(shelves) == 0 {
		t.Fatalf("populate should leave at least one shelf: %v, %v", shelves, err)
	}
	// Without a comfortable margin above bufio's default 4096-byte buffer,
	// a smaller populate() would silently move the first flush to
	// ExportTo's own final Flush instead of one mid-export, and this test
	// would stop proving anything.
	for i := range 40 {
		newItem(t, svc, shelves[0].ID, fmt.Sprintf("Padding item %d, long enough to add up fast", i))
	}
	out, err := exportOf(t, svc)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 2*4096 {
		t.Fatalf("export is only %d bytes; want comfortably more than two bufio buffers", len(raw))
	}

	pr, pw := io.Pipe()
	sw := &signalFirstWrite{w: pw, started: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		err := svc.ExportTo(ctx, sw)
		pw.Close()
		done <- err
	}()

	select {
	case <-sw.started:
	case <-time.After(5 * time.Second):
		t.Fatal("ExportTo never attempted its first write")
	}

	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := svc.CreateItem(writeCtx, library.Item{
		Title: "Written while export is stalled", Why: "because",
		Format: library.FormatBook, ShelfID: shelves[0].ID,
	}, nil); err != nil {
		t.Fatalf("CreateItem blocked behind the export snapshot: %v", err)
	}

	// Drain the pipe so the stalled export can finish, then check its error.
	if _, err := io.Copy(io.Discard, pr); err != nil {
		t.Fatalf("draining the export: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("ExportTo: %v", err)
	}
}
