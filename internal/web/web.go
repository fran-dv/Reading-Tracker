// Package web is the HTTP layer: routing, templates and static assets.
// Handlers call library.Service and nothing else; no rules live here.
package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

//go:embed templates/*.html static
var assets embed.FS

type handler struct {
	svc     *library.Service
	meta    metadataClient
	log     *slog.Logger
	capture *template.Template
	shelves *template.Template
	shelf   *template.Template
	session *template.Template
	home    *template.Template
	plan    *template.Template
}

// New builds the application's HTTP handler.
func New(svc *library.Service, meta metadataClient, log *slog.Logger) http.Handler {
	layout := template.Must(template.New("layout").Funcs(template.FuncMap{"pickerFor": pickerFor}).ParseFS(assets, "templates/layout.html"))
	h := &handler{
		svc:     svc,
		meta:    meta,
		log:     log,
		capture: page(layout, "templates/item-form.html", "templates/capture.html"),
		shelves: page(layout, "templates/shelves.html"),
		shelf:   page(layout, "templates/item-form.html", "templates/shelf.html"),
		session: page(layout, "templates/session.html"),
		home:    page(layout, "templates/board.html", "templates/home.html"),
		plan:    page(layout, "templates/board.html", "templates/plan.html"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.getHome)
	mux.HandleFunc("GET /home/body", h.getHomeBody)
	mux.HandleFunc("GET /items/{id}/done", h.getDone)
	mux.HandleFunc("POST /items/{id}/finish", h.postFinish)
	mux.HandleFunc("POST /items/{id}/reference", h.postReference)
	mux.HandleFunc("POST /items/{id}/start", h.postStartItem)
	mux.HandleFunc("GET /session", h.getSession)
	mux.HandleFunc("POST /sessions", h.postSession)
	mux.HandleFunc("POST /sessions/start", h.postStartSession)
	mux.HandleFunc("POST /sessions/{id}/stop", h.postStopSession)
	mux.HandleFunc("GET /plan", h.getPlan)
	mux.HandleFunc("GET /plan/body", h.getPlanBody)
	mux.HandleFunc("POST /plan", h.postPlan)
	mux.HandleFunc("POST /plan/lower", h.postPlanLower)
	mux.HandleFunc("POST /plan/preview", h.postPlanPreview)
	mux.HandleFunc("POST /plan/speed", h.postSpeedRamp)
	mux.HandleFunc("POST /plan/speed/stop", h.postStopSpeedRamp)
	mux.HandleFunc("GET /capture", h.getCapture)
	mux.HandleFunc("POST /items", h.postItem)
	mux.HandleFunc("POST /shelves", h.postShelf)
	mux.HandleFunc("GET /metadata", h.getMetadata)
	mux.HandleFunc("GET /books", h.getBooks)
	mux.HandleFunc("GET /shelves", h.getShelves)
	mux.HandleFunc("GET /shelves/{id}", h.getShelf)
	mux.HandleFunc("GET /shelves/{id}/body", h.getShelfBody)
	mux.HandleFunc("GET /shelves/{id}/items/{itemID}/edit", h.getEntryForm)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}", h.postEntry)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/start", h.postStart)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/shortlist", h.postShortlist)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/unshortlist", h.postUnshortlist)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/rank", h.postRank)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/unrank", h.postUnrank)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/up", h.postMoveUp)
	mux.HandleFunc("POST /shelves/{id}/items/{itemID}/down", h.postMoveDown)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /export", h.getExport)
	mux.Handle("GET /static/", noCache(http.FileServerFS(assets)))
	mux.HandleFunc("GET /sw.js", h.getServiceWorker)
	return h.middleware(mux)
}

// page builds a page template: the shell, then the files it is made of.
func page(layout *template.Template, files ...string) *template.Template {
	return template.Must(template.Must(layout.Clone()).ParseFS(assets, files...))
}

// navLink is one link in the running head.
type navLink struct {
	Name    string
	Href    string
	Current bool
}

// nav is the running head, in order. A route joins it when its screen exists.
var nav = []navLink{{Name: "Home", Href: "/"}, {Name: "Session", Href: "/session"}, {Name: "Capture", Href: "/capture"}, {Name: "Shelves", Href: "/shelves"}, {Name: "Plan", Href: "/plan"}}

// shell is what every page hands the layout. Pages embed it.
type shell struct{ Nav []navLink }

// newShell marks the running-head link for the route being rendered.
func newShell(current string) shell {
	links := make([]navLink, len(nav))
	copy(links, nav)
	for i := range links {
		links[i].Current = links[i].Href == current
	}
	return shell{Nav: links}
}

// marshalSignals renders a seed for a data-signals attribute.
func marshalSignals(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal signals: %w", err)
	}
	return string(b), nil
}

// formError reports a validation failure in band: the message lands in its
// slot on the open form, which keeps everything already typed, and focus
// moves to the field that fixes it. It reports whether err was one, so
// callers can fall through to httpError.
func (h *handler) formError(w http.ResponseWriter, r *http.Request, err error) bool {
	var verr *library.ValidationError
	if !errors.As(err, &verr) {
		return false
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.MarshalAndPatchSignals(map[string]any{"errors": fieldError(verr.Field, messageFor(verr))}); err != nil {
		h.log.Error("form errors", "err", err)
		return true
	}
	if err := focusField(sse, verr.Field); err != nil {
		h.log.Error("form error focus", "err", err)
	}
	return true
}

// patch renders one named template block and sends it as a
// datastar-patch-elements event. The block's top-level element must carry
// the id Datastar will morph into.
//
// html/template escaping note: Go strips the "data-" prefix when choosing
// an attribute's context, so every data-on:* attribute is treated as
// JavaScript. Static expressions pass through untouched; an interpolated
// value inside one is JS-string-escaped ("/" becomes "\/"), which Datastar
// parses fine. Never interpolate user text into a Datastar expression at
// all; pass it through a signal or element content instead.
func (h *handler) patch(sse *datastar.ServerSentEventGenerator, t *template.Template, name string, data any, opts ...datastar.PatchElementOption) error {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return sse.PatchElements(buf.String(), opts...)
}

// getExport sends the whole library as a downloadable JSON file (spec §10).
func (h *handler) getExport(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Export(r.Context())
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="readingqueue-%s.json"`, out.ExportedAt.Format("2006-01-02")))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		h.log.Error("export encode", "err", err)
	}
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
		errors.Is(err, library.ErrInFuture),
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

// getServiceWorker serves the app-shell worker from the root, which is what
// gives it the whole site as its scope. Never cached: a stale worker would
// outlive the binary that shipped it.
func (h *handler) getServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, assets, "static/sw.js")
}

// noCache makes browsers revalidate static files on every load. Assets are
// not versioned, so a long cache would serve stale CSS after a rebuild.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
