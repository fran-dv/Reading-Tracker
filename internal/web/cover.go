package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// coverPlateURL is the one formula for an item's own served cover,
// shared by the "plate" template block (layout.html, for every filed
// item everywhere it is listed) and the item form's preview of a cover
// already held before the form staged anything new (itemform.go,
// coverHeldURL). ?v= busts the cache since a pick, an upload, or a
// revert can change the bytes without changing the link (ADR-8).
func coverPlateURL(id string, updatedAt time.Time) string {
	return fmt.Sprintf("/items/%s/cover?v=%d", id, updatedAt.UnixMilli())
}

const (
	// maxCoverBodyBytes bounds the whole multipart request, with headroom
	// over maxCoverBytes for the part's own framing.
	maxCoverBodyBytes = 16 << 20
	// maxCoverBytes is the owner-facing cap (cover-management: Upload
	// Validation — File Size), checked before any decode.
	maxCoverBytes = 15 << 20
)

// coverTooLargeMsg and coverUnreadableMsg are the two plain refusal lines
// the upload route can answer, each tied to a cover-management scenario.
const (
	coverTooLargeMsg   = "That image is over 15 MB."
	coverUnreadableMsg = "That file isn't an image this can read. Use JPEG, PNG or WebP."
)

// coverCache is what the cover route needs from the covers package. Tests
// substitute a fake.
type coverCache interface {
	Cover(ctx context.Context, itemID, url string, locked bool) (*covers.Cover, error)
}

// getCover serves an item's cover from this machine. A cover that cannot be
// had, or that the item's choice no longer shows, answers 404 and the
// entry keeps its blank plate: an image with an empty alt draws nothing
// over it.
func (h *handler) getCover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	item, err := h.svc.GetItem(ctx, r.PathValue("id"))
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	if !item.ShowsCover() {
		http.NotFound(w, r)
		return
	}
	// Locked whenever the choice isn't found: a picked or uploaded cover
	// is owner data, never refetched (cover-management: Automatic Lookups
	// Never Override a Locked Choice).
	locked := item.CoverChoice != library.CoverFound
	c, err := h.covers.Cover(ctx, item.ID, item.CoverURL, locked)
	if err != nil {
		// A cover is never worth an error page: it is a picture over a
		// plate that is already drawn.
		h.log.Warn("cover", "item", item.ID, "err", err)
		http.NotFound(w, r)
		return
	}
	if c.Missing() {
		http.NotFound(w, r)
		return
	}

	// A pick, an upload or a revert changes the bytes without changing the
	// link, so the plate URL carries ?v={updated_at} to bust the cache;
	// these bytes themselves are still held for a year.
	w.Header().Set("Content-Type", c.MediaType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+strconv.FormatInt(c.FetchedAt.UnixMilli(), 36)+`"`)
	http.ServeContent(w, r, "", c.FetchedAt, bytes.NewReader(c.Bytes))
}

// postCoverUpload stages an uploaded image as a draft, applied only when
// the form is filed or saved (itemform.go, Service.SetCover); nothing is
// written to the library here (cover-management: Cover Upload Entry
// Points, Local Preview Before Processing). A refusal answers 200 with
// the plain line in errors.cover, the same in-band convention every
// other form validation already uses, so the rest of the open form is
// never lost.
func (h *handler) postCoverUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCoverBodyBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		// Not a multipart request at all (wrong content type, no
		// boundary): that is an unreadable upload, not an oversized one.
		h.coverError(w, r, coverUnreadableMsg)
		return
	}
	part, err := mr.NextPart()
	if err != nil {
		h.coverError(w, r, coverUnreadableMsg)
		return
	}
	defer part.Close()

	// One byte over the cap is enough to know it is too big, without
	// reading the whole thing (cover-management: Decompression Bomb
	// Guard covers the decode side; this is the plain byte-count side).
	// The MaxBytesError branch below almost never fires on its own —
	// maxCoverBodyBytes leaves headroom well past maxCoverBytes+1 — but
	// it is the honest answer if a multipart boundary or header ever ate
	// into that headroom enough to trip the body cap first.
	data, err := io.ReadAll(io.LimitReader(part, maxCoverBytes+1))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.coverError(w, r, coverTooLargeMsg)
		} else {
			h.log.Warn("cover upload: read part", "err", err)
			h.coverError(w, r, coverUnreadableMsg)
		}
		return
	}
	if len(data) > maxCoverBytes {
		h.coverError(w, r, coverTooLargeMsg)
		return
	}

	normalized, err := covers.Normalize(data)
	if err != nil {
		h.coverError(w, r, coverUnreadableMsg)
		return
	}

	token := h.drafts.Put(covers.Image{Bytes: normalized})
	sse := datastar.NewSSE(w, r)
	out := map[string]any{
		"coverDraft": token, "coverChoice": string(library.CoverUploaded),
		// coverSource stays empty for an upload; the edition picker's
		// pick route sets it to the chosen edition's link instead,
		// through this same signal.
		"coverSource": "",
		"_coverLocal": "", "errors": map[string]string{"cover": ""},
		// A fresh upload replaces whatever was staged before, found or
		// picked; clearing _coverUndo retires its Undo button, so it
		// can never later restore a choice this upload has superseded.
		"_coverUndo": "",
	}
	if err := sse.MarshalAndPatchSignals(out); err != nil {
		h.log.Error("cover upload signals", "err", err)
	}
}

// coverError reports a cover refusal in band, from either postCoverUpload
// or postCoverPick: the same 200-with-a-slot convention formError uses
// for a failed validation. It also clears _coverLocal: an upload's
// chooser already showed its local preview optimistically (Local Preview
// Before Processing), and a refusal must not leave that preview, with
// its busy stroke, stuck on the plate forever. A pick never sets
// _coverLocal in the first place, so clearing it there is a harmless
// no-op, not a second meaning this function has to carry.
func (h *handler) coverError(w http.ResponseWriter, r *http.Request, msg string) {
	sse := datastar.NewSSE(w, r)
	out := map[string]any{"errors": map[string]string{"cover": msg}, "_coverLocal": ""}
	if err := sse.MarshalAndPatchSignals(out); err != nil {
		h.log.Error("cover error signals", "err", err)
	}
}

// getCoverEditions answers the edition picker's own request (design.md
// ADR-14): gathers cover-bearing candidate editions of one book from both
// sources and patches the candidates row. workKey is whatever the form
// already carries (set when a search result was picked, or echoed back
// by an earlier call to this same route); metadata.Client.Editions
// resolves one itself when it is empty, which this handler then echoes
// back so a later click reuses it instead of resolving again.
func (h *handler) getCoverEditions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WorkKey string `json:"workKey"`
		Title   string `json:"title"`
		Author  string `json:"author"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res := h.meta.Editions(r.Context(), in.WorkKey, in.Title, in.Author)
	if r.Context().Err() != nil {
		// Superseded (P5's own guard, shared by every lookup route): the
		// owner has already moved on, so there is nothing left to draw.
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.capture, "cover-candidates", res); err != nil {
		h.log.Error("cover candidates", "err", err)
		return
	}
	workKey := in.WorkKey
	if workKey == "" {
		// Every Open Library candidate here carries the same resolved
		// work key (editions.go); a Google-only candidate carries none.
		// Scanning for the first non-empty one is simpler than widening
		// Editions' own return shape just to hand this back.
		for _, b := range res.Books {
			if b.Key != "" {
				workKey = b.Key
				break
			}
		}
	}
	out := map[string]any{"workKey": workKey, "_editionsNote": searchNote(res.Unanswered)}
	if err := sse.MarshalAndPatchSignals(out); err != nil {
		h.log.Error("cover candidates signals", "err", err)
	}
}

// pickableCoverHosts are the only hosts postCoverPick may fetch from: the
// cover hosts this server's own search and editions lookups ever
// produce. A candidate's cover link always comes from one of those, never
// from anything the owner typed, so refusing every other host closes off
// a server-side fetch of an arbitrary URL without losing anything a
// picked edition's own cover link needs (design.md, "Server-side fetch of
// client-supplied links").
//
// This checks only the request's own host, not where it might redirect
// to: metadata.Client.Image follows up to 5 redirects (metadata.go,
// maxRedirects) to whatever host the server answers with, same as every
// other fetch this package makes. A listed host that redirected an
// owner-supplied cover link to an attacker's host would defeat this
// guard; none of the three listed hosts are known to do that for a
// cover path today, and Open Library's own covers do legitimately
// redirect among its own and archive.org's hosts, so capping redirect
// targets here would need its own allow-list, not this one.
var pickableCoverHosts = map[string]bool{
	"covers.openlibrary.org":      true,
	"books.google.com":            true,
	"books.googleusercontent.com": true,
}

// coverPickRefusedMsg and coverPickFetchFailedMsg are postCoverPick's own
// refusal lines, alongside the upload route's coverUnreadableMsg (reused
// here for a fetched image that does not decode).
const (
	coverPickRefusedMsg     = "That cover link isn't one this can fetch."
	coverPickFetchFailedMsg = "Couldn't fetch that cover. Try another, or upload one."
)

// postCoverPick stages an edition's cover as a draft, the same way an
// upload does (cover-management: Choosing an Edition Stages a Picked
// Cover and Fills Fields): nothing is written to the library here. url
// must be one the server's own search or editions lookup produced
// (item-form.html's candidates row sets it from the result's own data);
// any other host is refused before anything is fetched.
func (h *handler) postCoverPick(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	raw := strings.TrimSpace(in.URL)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !pickableCoverHosts[u.Hostname()] {
		h.coverError(w, r, coverPickRefusedMsg)
		return
	}
	data, _, err := h.meta.Image(r.Context(), raw)
	if err != nil {
		h.coverError(w, r, coverPickFetchFailedMsg)
		return
	}
	normalized, err := covers.Normalize(data)
	if err != nil {
		h.coverError(w, r, coverUnreadableMsg)
		return
	}

	token := h.drafts.Put(covers.Image{Bytes: normalized, SourceURL: raw})
	sse := datastar.NewSSE(w, r)
	out := map[string]any{
		"coverDraft": token, "coverChoice": string(library.CoverPicked), "coverSource": raw,
		"_coverLocal": "", "errors": map[string]string{"cover": ""},
		// A fresh pick replaces whatever was staged before, the same as a
		// fresh upload (postCoverUpload): Undo would otherwise restore a
		// choice this pick has superseded.
		"_coverUndo": "",
	}
	if err := sse.MarshalAndPatchSignals(out); err != nil {
		h.log.Error("cover pick signals", "err", err)
	}
}

// getCoverDraft serves a staged cover's bytes, for the form's own local
// preview once the upload has been processed. Cache-Control: no-store
// because a draft is short-lived, per-owner server memory, never meant
// to survive in a shared or disk cache.
func (h *handler) getCoverDraft(w http.ResponseWriter, r *http.Request) {
	img, ok := h.drafts.Get(r.PathValue("token"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(img.Bytes); err != nil {
		h.log.Warn("cover draft write", "err", err)
	}
}
