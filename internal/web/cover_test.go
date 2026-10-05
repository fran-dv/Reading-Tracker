package web

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// tinyJPEG is a small, already-normalized JPEG: covers.Cache now runs
// every fetched cover through covers.Normalize before writing it, so a
// fake fetch answering with arbitrary non-image bytes would come back as
// a remembered failure instead of a served cover.
func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{200, 40, 40, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode fixture jpeg: %v", err)
	}
	return buf.Bytes()
}

// coverFixture files one item with a cover link and one without. opts reach
// the library, for a frozen clock.
func coverFixture(t *testing.T, meta *fakeMeta, opts ...library.Option) (http.Handler, *library.Service, *library.Item, *library.Item, *library.Shelf) {
	t.Helper()
	h, svc := newTestServer(t, meta, opts...)
	shelf, err := svc.CreateShelf(ctx, "Statistics")
	if err != nil {
		t.Fatal(err)
	}
	with := fileItem(t, svc, library.Item{
		Title: "Thinking in Systems", Why: "loops everywhere", Format: library.FormatBook,
		ShelfID: shelf.ID, CoverURL: "https://covers.example/1.jpg",
	})
	without := fileItem(t, svc, library.Item{
		Title: "A note to myself", Why: "no cover anywhere", Format: library.FormatPaper,
		ShelfID: shelf.ID,
	})
	return h, svc, with, without, shelf
}

func TestCoverServesTheImageOnceItIsFetched(t *testing.T) {
	jpeg := tinyJPEG(t)
	meta := &fakeMeta{image: jpeg, imageType: "image/jpeg"}
	h, _, item, _, _ := coverFixture(t, meta)

	rec := get(t, h, "/items/"+item.ID+"/cover")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if got := rec.Body.Bytes(); !bytes.Equal(got, jpeg) {
		t.Errorf("body is %d bytes, want the stored bytes unchanged (%d)", len(got), len(jpeg))
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("content type %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("cache-control %q, want the bytes held", got)
	}

	// Asked for again, it comes from the database and not the network.
	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusOK {
		t.Fatalf("second ask: status %d", rec.Code)
	}
	if len(meta.fetched) != 1 {
		t.Errorf("fetched %d times, want 1", len(meta.fetched))
	}
}

func TestCoverIsNotFoundWhenThereIsNone(t *testing.T) {
	meta := &fakeMeta{image: []byte("jpeg bytes"), imageType: "image/jpeg"}
	h, _, _, without, _ := coverFixture(t, meta)

	if rec := get(t, h, "/items/"+without.ID+"/cover"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
	if len(meta.fetched) != 0 {
		t.Errorf("fetched %v, want nothing without a link", meta.fetched)
	}
}

// An entry always draws its plate; only the picture over it is conditional,
// so a lookup that failed leaves the blank plate rather than a broken page.
func TestCoverThatCannotBeHadIsNotFound(t *testing.T) {
	meta := &fakeMeta{imageErr: errors.New("404")}
	h, _, item, _, _ := coverFixture(t, meta)

	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

// Every list draws the plate; only items with a cover link carry the image.
func TestEntriesDrawTheirPlate(t *testing.T) {
	meta := &fakeMeta{image: []byte("jpeg bytes"), imageType: "image/jpeg"}
	h, _, with, without, shelf := coverFixture(t, meta)

	for _, page := range []string{"/review", "/shelves/" + shelf.ID} {
		body := get(t, h, page).Body.String()
		if !strings.Contains(body, `src="/items/`+with.ID+`/cover?v=`) {
			t.Errorf("%s: no cover for the item that has one", page)
		}
		if strings.Contains(body, `src="/items/`+without.ID+`/cover`) {
			t.Errorf("%s: asked for a cover the item does not have", page)
		}
		if !strings.Contains(body, "entry-plate cloth-paper") {
			t.Errorf("%s: the item without a cover has no blank plate", page)
		}
	}
}

// Once an item's cover choice is removed, the route answers 404 without
// asking the fetcher for anything: the choice is locked, so ShowsCover is
// false (cover-management: Automatic Lookups Never Override a Locked
// Choice).
func TestCoverIsNotFoundOnceRemoved(t *testing.T) {
	meta := &fakeMeta{image: tinyJPEG(t), imageType: "image/jpeg"}
	h, svc, item, _, _ := coverFixture(t, meta)

	// Fetched once while the choice is still found.
	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusOK {
		t.Fatalf("status %d before removal", rec.Code)
	}
	if _, err := svc.SetCover(ctx, item.ID, library.CoverRemoved, nil); err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/items/"+item.ID+"/cover")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404 once removed", rec.Code)
	}
	if len(meta.fetched) != 1 {
		t.Errorf("fetched %d times, want no further attempt once removed", len(meta.fetched))
	}
}

// The plate's cache-busting ?v= changes whenever the item's cover changes,
// since a pick, an upload or a revert can change the bytes without
// changing the link (ADR-8).
func TestCoverPlateURLChangesAfterSetCover(t *testing.T) {
	clk := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	meta := &fakeMeta{image: tinyJPEG(t), imageType: "image/jpeg"}
	h, svc, item, _, shelf := coverFixture(t, meta, library.WithClock(func() time.Time { return clk }))

	before := coverPlateSrc(t, get(t, h, "/shelves/"+shelf.ID).Body.String(), item.ID)

	clk = clk.Add(time.Minute)
	if _, err := svc.SetCover(ctx, item.ID, library.CoverUploaded, &library.CoverImage{
		MediaType: "image/jpeg", Bytes: tinyJPEG(t),
	}); err != nil {
		t.Fatal(err)
	}

	after := coverPlateSrc(t, get(t, h, "/shelves/"+shelf.ID).Body.String(), item.ID)
	if before == after {
		t.Fatalf("plate URL unchanged after SetCover: %q", before)
	}
}

// multipartCoverUpload posts data as the single "cover" part of a
// multipart form, the way the browser's hidden #cover-upload form does
// (design.md ADR-10).
func multipartCoverUpload(t *testing.T, h http.Handler, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("cover", "cover.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/covers/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCoverUploadHappyPathReturnsADraftToken(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})

	rec := multipartCoverUpload(t, h, tinyJPEG(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	sig := patchedSignals(t, rec.Body.String())
	if sig["coverChoice"] != "uploaded" {
		t.Errorf("coverChoice = %v, want uploaded", sig["coverChoice"])
	}
	token, _ := sig["coverDraft"].(string)
	if token == "" {
		t.Fatal("no coverDraft token in the response")
	}
	errs, _ := sig["errors"].(map[string]any)
	if errs["cover"] != "" {
		t.Errorf("errors.cover = %v, want cleared", errs["cover"])
	}
	// A fresh upload supersedes whatever was staged before; its Undo,
	// if one was showing, must not survive to restore the wrong thing.
	if got, ok := sig["_coverUndo"]; !ok || got != "" {
		t.Errorf("_coverUndo = %v, want cleared by a fresh upload", got)
	}

	// The draft is now servable for the local preview.
	draft := get(t, h, "/covers/drafts/"+token)
	if draft.Code != http.StatusOK {
		t.Fatalf("draft status %d", draft.Code)
	}
	if ct := draft.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("draft Content-Type %q, want image/jpeg", ct)
	}
	if cc := draft.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("draft Cache-Control %q, want no-store", cc)
	}
	if _, err := jpeg.Decode(bytes.NewReader(draft.Body.Bytes())); err != nil {
		t.Errorf("draft body does not decode as JPEG: %v", err)
	}
}

func TestCoverUploadOversizedFileRefused(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})

	huge := make([]byte, maxCoverBytes+1)
	rec := multipartCoverUpload(t, h, huge)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	sig := patchedSignals(t, rec.Body.String())
	errs, _ := sig["errors"].(map[string]any)
	if errs["cover"] != coverTooLargeMsg {
		t.Errorf("errors.cover = %v, want %q", errs["cover"], coverTooLargeMsg)
	}
	if _, ok := sig["coverDraft"]; ok {
		t.Error("an oversized upload must not stage a draft")
	}
	if got, ok := sig["_coverLocal"]; !ok || got != "" {
		t.Errorf("_coverLocal = %v, want cleared so the refused preview does not stick to the plate", got)
	}
}

func TestCoverUploadUnreadableFileRefused(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})

	// Neither a real image nor any format Go can decode — the same path
	// HEIC bytes would take (cover-management: Upload Validation — File
	// Type, HEIC and unrecognized-file scenarios).
	rec := multipartCoverUpload(t, h, []byte("not an image, heic or otherwise"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	sig := patchedSignals(t, rec.Body.String())
	errs, _ := sig["errors"].(map[string]any)
	if errs["cover"] != coverUnreadableMsg {
		t.Errorf("errors.cover = %v, want %q", errs["cover"], coverUnreadableMsg)
	}
	if _, ok := sig["coverDraft"]; ok {
		t.Error("an unreadable upload must not stage a draft")
	}
	if got, ok := sig["_coverLocal"]; !ok || got != "" {
		t.Errorf("_coverLocal = %v, want cleared so the refused preview does not stick to the plate", got)
	}
}

// Expiry itself is covered in drafts_test.go; this only checks the
// route's own 404, which needs no expired draft to demonstrate.
func TestCoverDraftRouteIsNotFoundWhenUnknown(t *testing.T) {
	h, _ := newTestServer(t, &fakeMeta{})

	if rec := get(t, h, "/covers/drafts/does-not-exist"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404 for an unknown token", rec.Code)
	}
}

// draftToken uploads data and returns the token the form would stage,
// the way the item form's own upload round trip does.
func draftToken(t *testing.T, h http.Handler, data []byte) string {
	t.Helper()
	rec := multipartCoverUpload(t, h, data)
	sig := patchedSignals(t, rec.Body.String())
	token, _ := sig["coverDraft"].(string)
	if token == "" {
		t.Fatalf("upload did not stage a draft: %s", rec.Body.String())
	}
	return token
}

// Filing applies a staged upload to the new item (cover-management: Cover
// Changes Are Staged in the Form Until Filed or Saved).
func TestPostItemAppliesAStagedUploadedCover(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, err := svc.CreateShelf(ctx, "Reading")
	if err != nil {
		t.Fatal(err)
	}

	in := validForm(shelf.ID)
	in.CoverChoice, in.CoverDraft = "uploaded", draftToken(t, h, tinyJPEG(t))
	rec := send(t, h, http.MethodPost, "/items", in)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	out, err := exportOf(t, svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(out.Items))
	}
	item := out.Items[0]
	if item.CoverChoice != library.CoverUploaded {
		t.Fatalf("cover choice = %q, want uploaded", item.CoverChoice)
	}
	if rec := get(t, h, "/items/"+item.ID+"/cover"); rec.Code != http.StatusOK {
		t.Errorf("uploaded cover not served: status %d", rec.Code)
	}
}

// A draft gone by filing time — expired, or lost to a restart — refuses
// in band and files nothing at all, cover included.
func TestPostItemMissingDraftRefusesAndFilesNothing(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, err := svc.CreateShelf(ctx, "Reading")
	if err != nil {
		t.Fatal(err)
	}

	in := validForm(shelf.ID)
	in.CoverChoice, in.CoverDraft = "uploaded", "does-not-exist"
	rec := send(t, h, http.MethodPost, "/items", in)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "capture-status") {
		t.Error("a lost draft must not show the filed confirmation")
	}
	errs, _ := patchedSignals(t, rec.Body.String())["errors"].(map[string]any)
	if errs["cover"] != coverLostMsg {
		t.Errorf("errors.cover = %v, want %q", errs["cover"], coverLostMsg)
	}
	if out, err := exportOf(t, svc); err != nil || len(out.Items) != 0 {
		t.Fatalf("an item was filed despite the lost draft: %d, err=%v", len(out.Items), err)
	}
}

// The cover row (upload, revert/remove, confirm dialog) renders on both
// capture and the shelf edit form, sharing one markup contract
// (cover-management: Cover Upload Entry Points; Revert to the Found
// Cover; Remove the Cover).
func TestCoverRowRendersOnCaptureAndShelfEdit(t *testing.T) {
	wantMarkers := []string{
		`<div class="cover-plate" id="cover-plate"`,
		`id="cover-file" name="cover" accept="image/*" form="cover-upload" hidden`,
		`id="cover-upload-button"`, `>Upload a cover<`,
		`id="cover-use-found-button"`, `>Use the found cover<`,
		`id="cover-remove-button"`, `>Remove the cover<`,
		`id="cover-undo-button"`, `>Undo<`,
		`id="cover-confirm-dialog"`, `>Discard it<`, `>Keep it<`,
		`id="cover-error"`,
		`<form id="cover-upload" enctype="multipart/form-data" hidden>`,
	}

	h, svc := newTestServer(t, &fakeMeta{})
	capture := get(t, h, "/capture").Body.String()
	for _, want := range wantMarkers {
		if !strings.Contains(capture, want) {
			t.Errorf("capture page missing %q", want)
		}
	}

	shelf, err := svc.CreateShelf(ctx, "Reading")
	if err != nil {
		t.Fatal(err)
	}
	item := fileItem(t, svc, library.Item{
		Title: "Thinking in Systems", Why: "loops everywhere", Format: library.FormatBook, ShelfID: shelf.ID,
	})
	edit := get(t, h, "/shelves/"+shelf.ID+"/items/"+item.ID+"/edit").Body.String()
	for _, want := range wantMarkers {
		if !strings.Contains(edit, want) {
			t.Errorf("shelf edit form missing %q", want)
		}
	}

	if !strings.Contains(capture, `src="/static/covers.js"`) {
		t.Error("the page must load covers.js for drag-and-drop and paste")
	}
}

// coverPlateSrc returns the full src attribute value of itemID's plate
// image in body.
func coverPlateSrc(t *testing.T, body, itemID string) string {
	t.Helper()
	marker := `src="/items/` + itemID + `/cover`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no cover image for item %s in:\n%s", itemID, body)
	}
	rest := body[i+len(`src="`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("unterminated src attribute for item %s in:\n%s", itemID, body)
	}
	return rest[:end]
}
