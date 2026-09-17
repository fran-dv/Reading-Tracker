package web

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/sqlite"
	"github.com/starfederation/datastar-go/datastar"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// newTestServer wires the real handler to a fresh SQLite library and the
// given metadata fake. opts reach the library, for a frozen clock.
func newTestServer(t *testing.T, meta metadataClient, opts ...library.Option) (http.Handler, *library.Service) {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	svc := library.New(store, opts...)
	return New(svc, meta, slog.New(slog.NewTextHandler(io.Discard, nil))), svc
}

func TestRoutes(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})

	tests := []struct {
		path        string
		status      int
		contentType string
		body        string
	}{
		{"/capture", http.StatusOK, "text/html", "<title>Reading Tracker</title>"},
		{"/capture", http.StatusOK, "text/html", `<script type="module" src="/static/datastar.js">`},
		{"/capture", http.StatusOK, "text/html", `@post('/items')`},
		{"/static/datastar.js", http.StatusOK, "text/javascript", "Datastar v1.0.3"},
		{"/static/picker.js", http.StatusOK, "text/javascript", "addEventListener"},
		{"/static/duration.js", http.StatusOK, "text/javascript", "readDuration"},
		{"/sw.js", http.StatusOK, "text/javascript", `caches.open`},
		{"/static/manifest.json", http.StatusOK, "application/json", `"start_url": "/"`},
		{"/static/icon.svg", http.StatusOK, "image/svg+xml", "<svg"},
		{"/capture", http.StatusOK, "text/html", `<link rel="manifest" href="/static/manifest.json">`},
		{"/capture", http.StatusOK, "text/html", `serviceWorker.register("/sw.js")`},
		{"/static/fonts/AlegreyaSans-Regular.woff2", http.StatusOK, "font/woff2", ""},
		{"/healthz", http.StatusOK, "text/plain", "ok"},
		{"/static/app.css", http.StatusOK, "text/css", "color-scheme"},
		{"/ping", http.StatusNotFound, "", ""},
		{"/nope", http.StatusNotFound, "", ""},
		{"/static/missing.css", http.StatusNotFound, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			rec := get(t, h, tc.path)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, tc.contentType) {
				t.Fatalf("content-type %q, want prefix %q", ct, tc.contentType)
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tc.body)
			}
		})
	}

	if rec := get(t, h, "/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `href="/" aria-current="page"`) {
		t.Fatalf("/ = %d, want the home page", rec.Code)
	}
	for _, path := range []string{"/static/app.css", "/sw.js"} {
		if cc := get(t, h, path).Header().Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("%s Cache-Control %q, want no-cache", path, cc)
		}
	}
}

func TestExport(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	if _, err := svc.CreateShelf(context.Background(), "Go"); err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/export")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="readingqueue-`) {
		t.Fatalf("content-disposition %q", cd)
	}
	var out library.Export
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Version != library.ExportVersion || len(out.Shelves) != 1 || out.Shelves[0].Name != "Go" {
		t.Fatalf("export body wrong: %+v", out)
	}
}

func TestPatchRendersBlock(t *testing.T) {
	h := &handler{}
	tmpl := template.Must(template.New("").Parse(`{{define "x"}}<p id="x">{{.}}</p>{{end}}`))
	rec := httptest.NewRecorder()
	sse := datastar.NewSSE(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := h.patch(sse, tmpl, "x", "hi <b>"); err != nil {
		t.Fatal(err)
	}
	// The SDK ends the last data line with "\n" and then writes the "\n\n"
	// event separator, so three newlines is its real wire format.
	want := "event: datastar-patch-elements\ndata: elements <p id=\"x\">hi &lt;b&gt;</p>\n\n\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("wire:\n%q\nwant\n%q", got, want)
	}
	if err := h.patch(sse, tmpl, "missing", nil); err == nil {
		t.Fatal("missing block: want error")
	}
}

func TestStatusOf(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{library.ErrNotFound, http.StatusNotFound},
		{&library.ValidationError{Field: "title", Msg: "required"}, http.StatusUnprocessableEntity},
		{library.ErrReasonRequired, http.StatusUnprocessableEntity},
		{library.ErrInvalidRange, http.StatusUnprocessableEntity},
		{library.ErrWIPCapReached, http.StatusConflict},
		{library.ErrInvalidTransition, http.StatusConflict},
		{library.ErrHasSessions, http.StatusConflict},
		{&library.SessionRunningError{ID: "s"}, http.StatusConflict},
		{errors.New("disk on fire"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		if got := statusOf(tc.err); got != tc.status {
			t.Errorf("statusOf(%v) = %d, want %d", tc.err, got, tc.status)
		}
	}
}

func TestMiddlewareRecoversPanics(t *testing.T) {
	h := &handler{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := get(t, h.middleware(boom), "/")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", rec.Code)
	}
}

func TestHTTPErrorHidesInternalDetails(t *testing.T) {
	h := &handler{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	h.httpError(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("secret path /var/db"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("500 leaked details: %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.httpError(rec, httptest.NewRequest(http.MethodGet, "/", nil), library.ErrWIPCapReached)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), library.ErrWIPCapReached.Error()) {
		t.Fatalf("domain error not surfaced: %d %q", rec.Code, rec.Body.String())
	}
}
