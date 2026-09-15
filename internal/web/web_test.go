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
	"net/url"
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

func TestRoutes(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(library.New(store), slog.New(slog.NewTextHandler(io.Discard, nil)))

	tests := []struct {
		path        string
		status      int
		contentType string
		body        string
	}{
		{"/", http.StatusOK, "text/html", "<title>Reading Tracker</title>"},
		{"/", http.StatusOK, "text/html", `<script type="module" src="/static/datastar.js">`},
		{"/", http.StatusOK, "text/html", `data-on:click="@get(`},
		{"/static/datastar.js", http.StatusOK, "text/javascript", "Datastar v1.0.3"},
		{"/healthz", http.StatusOK, "text/plain", "ok"},
		{"/static/app.css", http.StatusOK, "text/css", "color-scheme"},
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

	if cc := get(t, h, "/static/app.css").Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("static Cache-Control %q, want no-cache", cc)
	}
}

func TestExport(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := library.New(store)
	if _, err := svc.CreateShelf(context.Background(), "Go"); err != nil {
		t.Fatal(err)
	}
	h := New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

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

func TestPing(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(library.New(store), slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := get(t, h, "/ping?datastar="+url.QueryEscape(`{"name":"Fran","count":2}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"event: datastar-patch-elements\ndata: elements <div id=\"ping\">Hello, Fran. Server time ",
		"event: datastar-patch-signals\ndata: signals {\"count\":3}\n\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}

	if rec := get(t, h, "/ping?datastar=not-json"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad signals: status %d, want 400", rec.Code)
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
