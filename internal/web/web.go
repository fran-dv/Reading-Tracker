// Package web is the HTTP layer: routing, templates and static assets.
// Handlers call library.Service and nothing else; no rules live here.
package web

import (
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

//go:embed templates/*.html static
var assets embed.FS

type handler struct {
	svc   *library.Service
	log   *slog.Logger
	index *template.Template
}

// New builds the application's HTTP handler.
func New(svc *library.Service, log *slog.Logger) http.Handler {
	layout := template.Must(template.ParseFS(assets, "templates/layout.html"))
	h := &handler{
		svc:   svc,
		log:   log,
		index: template.Must(template.Must(layout.Clone()).ParseFS(assets, "templates/index.html")),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.getIndex)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.Handle("GET /static/", noCache(http.FileServerFS(assets)))
	return h.middleware(mux)
}

func (h *handler) getIndex(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, h.index, nil)
}

// render executes a page template, mapping failures through httpError.
func (h *handler) render(w http.ResponseWriter, r *http.Request, page *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.ExecuteTemplate(w, "layout", data); err != nil {
		h.httpError(w, r, err)
	}
}

// httpError writes the HTTP status that matches a library error. Unknown
// errors are logged and reported as 500 without leaking their text.
func (h *handler) httpError(w http.ResponseWriter, r *http.Request, err error) {
	status := statusOf(err)
	if status == http.StatusInternalServerError {
		h.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	http.Error(w, err.Error(), status)
}

func statusOf(err error) int {
	var validation *library.ValidationError
	var running *library.SessionRunningError
	switch {
	case errors.Is(err, library.ErrNotFound):
		return http.StatusNotFound
	case errors.As(err, &validation),
		errors.Is(err, library.ErrInvalidRange),
		errors.Is(err, library.ErrInvalidPositions),
		errors.Is(err, library.ErrReasonRequired):
		return http.StatusUnprocessableEntity
	case errors.As(err, &running),
		errors.Is(err, library.ErrInvalidTransition),
		errors.Is(err, library.ErrWIPCapReached),
		errors.Is(err, library.ErrHasSessions),
		errors.Is(err, library.ErrShelfNotEmpty),
		errors.Is(err, library.ErrDuplicateShelf),
		errors.Is(err, library.ErrNotVisibleOnShelf),
		errors.Is(err, library.ErrItemNotInProgress):
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// middleware recovers from panics and logs one line per request.
func (h *handler) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				h.log.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", p)
				http.Error(rec, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			h.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rec, r)
	})
}

// statusRecorder remembers the status code for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer,
// which SSE handlers need for flushing.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// noCache makes browsers revalidate static files on every load. Assets are
// not versioned, so a long cache would serve stale CSS after a rebuild.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
