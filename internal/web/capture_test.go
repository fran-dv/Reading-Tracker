package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/fran-dv/reading-tracker/internal/metadata"
)

var ctx = context.Background()

// fakeMeta answers lookups without a network.
type fakeMeta struct {
	result  metadata.Result
	books   []metadata.Book
	err     error
	queried string
}

func (f *fakeMeta) Lookup(_ context.Context, rawURL string) (metadata.Result, error) {
	f.queried = rawURL
	return f.result, f.err
}

func (f *fakeMeta) SearchBooks(_ context.Context, query string) ([]metadata.Book, error) {
	f.queried = query
	return f.books, f.err
}

// send issues a Datastar request: signals as JSON body for POST, as the
// datastar query parameter for GET.
func send(t *testing.T, h http.Handler, method, path string, signals any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(signals)
	if err != nil {
		t.Fatal(err)
	}
	var req *http.Request
	if method == http.MethodGet {
		req = httptest.NewRequest(method, path+"?datastar="+url.QueryEscape(string(raw)), nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(string(raw)))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// patchedSignals merges every datastar-patch-signals event in an SSE body,
// the way the browser would.
func patchedSignals(t *testing.T, body string) map[string]any {
	t.Helper()
	merged := map[string]any{}
	for _, line := range strings.Split(body, "\n") {
		raw, ok := strings.CutPrefix(line, "data: signals ")
		if !ok {
			continue
		}
		var patch map[string]any
		if err := json.Unmarshal([]byte(raw), &patch); err != nil {
			t.Fatalf("bad signals %q: %v", raw, err)
		}
		for k, v := range patch {
			merged[k] = v
		}
	}
	return merged
}

// pageSignals decodes the first data-signals attribute of a rendered page
// into the form struct that seeded it.
func pageSignals[F any](t *testing.T, body string) F {
	t.Helper()
	_, rest, ok := strings.Cut(body, `data-signals="`)
	if !ok {
		t.Fatal("page has no data-signals")
	}
	attr, _, _ := strings.Cut(rest, `"`)
	var sig F
	if err := json.Unmarshal([]byte(strings.ReplaceAll(attr, "&#34;", `"`)), &sig); err != nil {
		t.Fatalf("data-signals %q: %v", attr, err)
	}
	return sig
}

func validForm(shelfID string) itemForm {
	in := newItemForm(shelfID, "Reading")
	in.Title, in.Why = "Statistical Rethinking", "Bayesian thinking I can use"
	return in
}

func TestCapturePagePrefillsShelf(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})

	if sig := pageSignals[itemForm](t, get(t, h, "/capture").Body.String()); sig.ShelfID != newShelfID {
		t.Fatalf("empty library: shelfId %q, want %q so the new-shelf field opens", sig.ShelfID, newShelfID)
	}

	stats, _ := svc.CreateShelf(ctx, "Statistics")
	goShelf, _ := svc.CreateShelf(ctx, "Go")
	body := get(t, h, "/capture").Body.String()
	if sig := pageSignals[itemForm](t, body); sig.ShelfID != stats.ID || sig.ShelfName != "Statistics" {
		t.Fatalf("no items: shelf %q %q, want the first shelf", sig.ShelfID, sig.ShelfName)
	}
	// The picker's behaviour lives in static/picker.js; the page must load it
	// and the markup must follow its contract.
	for _, want := range []string{
		`src="/static/picker.js"`,
		`<div class="picker" id="shelf-picker"`, `class="field picker-button"`, `aria-expanded="false"`,
		`role="listbox" aria-labelledby="shelf-label" hidden`, `role="option"`, `data-id="new"`,
		`data-name="Statistics"`, `data-name="Go"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}

	if _, err := svc.CreateItem(ctx, library.Item{Title: "x", Why: "y", Format: library.FormatBook, ShelfID: goShelf.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if sig := pageSignals[itemForm](t, get(t, h, "/capture").Body.String()); sig.ShelfID != goShelf.ID {
		t.Fatalf("after filing on Go: shelfId %q, want %q", sig.ShelfID, goShelf.ID)
	}
}

func TestPostItemFiles(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, _ := svc.CreateShelf(ctx, "Reading")

	in := validForm(shelf.ID)
	in.Format, in.Text, in.Tags = "article", "one two three", "go, sql,"
	in.URL, in.CoverURL = " https://blog.example/post ", "https://blog.example/img.png"
	in.NeedsDesk = true
	rec := send(t, h, http.MethodPost, "/items", in)

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d %q: %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<div class="row capture-status" id="capture-status" role="status">`) ||
		!strings.Contains(body, "Statistical Rethinking, on Reading.") {
		t.Errorf("no filed confirmation:\n%s", body)
	}
	if !strings.Contains(body, "data: mode replace") {
		t.Errorf("confirmation should replace, so its ink-in replays:\n%s", body)
	}
	if !strings.Contains(body, `document.getElementById(\"title\").focus()`) && !strings.Contains(body, `document.getElementById("title").focus()`) {
		t.Errorf("no refocus script:\n%s", body)
	}
	sig := patchedSignals(t, body)
	if sig["title"] != "" || sig["why"] != "" || sig["shelfId"] != shelf.ID || sig["format"] != "book" {
		t.Errorf("reset should clear the form and keep the shelf: %v", sig)
	}

	out, err := svc.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(out.Items))
	}
	it := out.Items[0]
	if it.SizeValue == nil || *it.SizeValue != 3 || it.WordCount == nil || *it.WordCount != 3 || it.SizeUnit != library.UnitWords {
		t.Errorf("pasted text should size the article in words: size=%v words=%v unit=%s", it.SizeValue, it.WordCount, it.SizeUnit)
	}
	if it.URL != "https://blog.example/post" || it.CoverURL != "https://blog.example/img.png" || !it.NeedsDesk {
		t.Errorf("fields lost: %+v", it.Item)
	}
	if strings.Join(it.Tags, ",") != "go,sql" {
		t.Errorf("tags = %v, want [go sql]", it.Tags)
	}
}

func TestPostItemValidationInBand(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, _ := svc.CreateShelf(ctx, "Reading")

	tests := []struct {
		name  string
		edit  func(*itemForm)
		field string
	}{
		{"no why", func(in *itemForm) { in.Why = "  " }, "why"},
		{"no title", func(in *itemForm) { in.Title = "" }, "title"},
		{"size not a number", func(in *itemForm) { in.SizeValue = "12x" }, "size_value"},
		{"negative size", func(in *itemForm) { in.SizeValue = "-4" }, "size_value"},
		{"new shelf not added", func(in *itemForm) { in.ShelfID = newShelfID }, "shelf_id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := validForm(shelf.ID)
			tc.edit(&in)
			rec := send(t, h, http.MethodPost, "/items", in)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, want 200 with in-band errors", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "capture-status") {
				t.Fatal("a rejected entry must not show the filed confirmation")
			}
			errs, _ := patchedSignals(t, rec.Body.String())["errors"].(map[string]any)
			if msg, _ := errs[tc.field].(string); msg == "" {
				t.Fatalf("no message on %s: %v", tc.field, errs)
			}
			if len(errs) != len(blankErrors()) {
				t.Fatalf("errors should carry every slot so stale ones clear: %v", errs)
			}
			if want := `getElementById(\"` + fieldInputs[tc.field] + `\").focus()`; !strings.Contains(rec.Body.String(), want) &&
				!strings.Contains(rec.Body.String(), strings.ReplaceAll(want, `\"`, `"`)) {
				t.Fatalf("focus should move to the %s input:\n%s", tc.field, rec.Body.String())
			}
		})
	}

	if out, _ := svc.Export(ctx); len(out.Items) != 0 {
		t.Fatalf("rejected entries were saved: %d", len(out.Items))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader("not json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad signals: status %d, want 400", rec.Code)
	}
}

func TestPostShelf(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})

	rec := send(t, h, http.MethodPost, "/shelves", map[string]string{"newShelf": " Statistics "})
	body := rec.Body.String()
	shelves, _ := svc.ListShelves(ctx)
	if rec.Code != http.StatusOK || len(shelves) != 1 || shelves[0].Name != "Statistics" {
		t.Fatalf("status %d, shelves %v", rec.Code, shelves)
	}
	if !strings.Contains(body, `<div class="picker" id="shelf-picker"`) || !strings.Contains(body, `data-name="Statistics"`) {
		t.Errorf("picker not re-rendered:\n%s", body)
	}
	if strings.Index(body, "patch-elements") > strings.Index(body, "patch-signals") {
		t.Error("options must arrive before the signal that selects the new shelf")
	}
	if sig := patchedSignals(t, body); sig["shelfId"] != shelves[0].ID || sig["shelfName"] != "Statistics" || sig["newShelf"] != "" {
		t.Errorf("new shelf not selected: %v", sig)
	}

	for name, value := range map[string]string{"duplicate": "statistics", "blank": "  "} {
		rec := send(t, h, http.MethodPost, "/shelves", map[string]string{"newShelf": value})
		errs, _ := patchedSignals(t, rec.Body.String())["errors"].(map[string]any)
		if rec.Code != http.StatusOK || errs["new_shelf"] == "" {
			t.Errorf("%s: status %d errors %v, want in-band new_shelf message", name, rec.Code, errs)
		}
	}
}

func TestGetMetadata(t *testing.T) {
	meta := &fakeMeta{result: metadata.Result{Title: "Real Title", Author: "Jane Doe", Format: "article", WordCount: 1234,
		CoverURL: "https://x.example/c.png"}}
	h, _ := newTestServer(t, meta)

	rec := send(t, h, http.MethodGet, "/metadata", map[string]string{"title": " https://x.example/post ", "author": "Me"})
	sig := patchedSignals(t, rec.Body.String())
	if meta.queried != "https://x.example/post" {
		t.Errorf("looked up %q", meta.queried)
	}
	if sig["url"] != "https://x.example/post" || sig["title"] != "Real Title" || sig["format"] != "article" {
		t.Errorf("link should move to url and title fill in: %v", sig)
	}
	if sig["focusDemand"] != "light" || sig["needsDesk"] != false {
		t.Errorf("a found format should bring its focus and desk defaults: %v", sig)
	}
	if _, overwritten := sig["author"]; overwritten {
		t.Errorf("an author already typed must not be overwritten: %v", sig)
	}
	if sig["sizeValue"] != "1234" {
		t.Errorf("sizeValue = %#v, want the string \"1234\" (inputs hold strings)", sig["sizeValue"])
	}
	if pencil, _ := sig["pencil"].(map[string]any); pencil["title"] != true || pencil["author"] != false {
		t.Errorf("found fields should be in pencil, typed ones not: %v", pencil)
	}

	// A second link replaces what the first lookup pencilled in, including
	// values the new page doesn't have.
	meta.result = metadata.Result{Title: "A Video", Format: "video"}
	sig = patchedSignals(t, send(t, h, http.MethodGet, "/metadata", map[string]any{
		"title": "https://youtu.be/x", "author": "Jane Doe", "sizeValue": "1234", "coverUrl": "https://x.example/c.png",
		"pencil": map[string]bool{"title": false, "author": true, "sizeValue": true},
	}).Body.String())
	if sig["author"] != "" || sig["sizeValue"] != "" || sig["coverUrl"] != "" {
		t.Errorf("pencilled values from the previous link should clear: %v", sig)
	}

	meta.err = errors.New("timeout")
	sig = patchedSignals(t, send(t, h, http.MethodGet, "/metadata", map[string]string{"title": "https://x.example/slow"}).Body.String())
	if errs, _ := sig["errors"].(map[string]any); errs["url"] == "" || sig["url"] != "https://x.example/slow" || sig["title"] != "" {
		t.Errorf("failed lookup should keep the link and ask for a title: %v", sig)
	}

	if rec := send(t, h, http.MethodGet, "/metadata", map[string]string{"title": "Just a title"}); rec.Code != http.StatusNoContent {
		t.Errorf("plain title: status %d, want 204", rec.Code)
	}
}

// The shelf view's Refetch sends a title that is a title and the link in its
// own field. The lookup then fills only what nobody has typed.
func TestGetMetadataFromTheURLField(t *testing.T) {
	meta := &fakeMeta{result: metadata.Result{Title: "Real Title", Author: "Jane Doe", Format: "article", WordCount: 1234}}
	h, _ := newTestServer(t, meta)

	sig := patchedSignals(t, send(t, h, http.MethodGet, "/metadata", map[string]any{
		"title": "A title I typed", "url": "https://x.example/post", "author": "",
		"pencil": map[string]bool{"title": false, "author": false, "sizeValue": false},
	}).Body.String())

	if meta.queried != "https://x.example/post" {
		t.Errorf("looked up %q, want the URL field", meta.queried)
	}
	if _, overwritten := sig["title"]; overwritten {
		t.Errorf("a title already typed must not be overwritten: %v", sig)
	}
	if sig["author"] != "Jane Doe" || sig["sizeValue"] != "1234" {
		t.Errorf("empty fields should still fill in: %v", sig)
	}
}

func TestGetBooks(t *testing.T) {
	meta := &fakeMeta{books: []metadata.Book{
		{Title: `Go <in> "Action"`, Author: "William Kennedy", Year: 2015},
		{Title: "The Go Programming Language", Author: "Alan Donovan", Pages: 380,
			CoverURL: "https://covers.example/1-L.jpg", ThumbURL: "https://covers.example/1-M.jpg"},
	}}
	h, _ := newTestServer(t, meta)

	rec := send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"})
	body := rec.Body.String()
	if meta.queried != "go pro" {
		t.Errorf("searched %q", meta.queried)
	}
	for _, want := range []string{
		`id="search-results"`,
		`data-title="Go &lt;in&gt; &#34;Action&#34;"`,
		`data-pages="380"`,
		`data-cover="https://covers.example/1-L.jpg"`,
		`src="https://covers.example/1-M.jpg"`,
		`data-pages=""`,
		"380 pages",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("results missing %q:\n%s", want, body)
		}
	}
	if sig := patchedSignals(t, body); sig["_showResults"] != true {
		t.Errorf("results not shown: %v", sig)
	}

	for _, short := range []string{"go", "https://x.example/long-enough"} {
		if rec := send(t, h, http.MethodGet, "/books", map[string]string{"title": short}); rec.Code != http.StatusNoContent {
			t.Errorf("%q: status %d, want 204", short, rec.Code)
		}
	}

	meta.err = errors.New("down")
	sig := patchedSignals(t, send(t, h, http.MethodGet, "/books", map[string]string{"title": "go pro"}).Body.String())
	if errs, _ := sig["errors"].(map[string]any); errs["title"] == "" || sig["_showResults"] != false {
		t.Errorf("search failure should say so and hide results: %v", sig)
	}
}

func TestCaptureSignalsToItem(t *testing.T) {
	tests := []struct {
		name      string
		edit      func(*itemForm)
		wantSize  *int
		wantWords *int
		wantErr   bool
	}{
		{"book with pages", func(in *itemForm) { in.SizeValue = " 380 " }, ptr(380), nil, false},
		{"book ignores pasted text", func(in *itemForm) { in.Text = "a b c" }, nil, nil, false},
		{"article typed size wins over text", func(in *itemForm) {
			in.Format, in.SizeValue, in.Text = "article", "900", "a b c"
		}, ptr(900), ptr(900), false},
		{"article counts pasted text", func(in *itemForm) { in.Format, in.Text = "article", "a b\nc d" }, ptr(4), ptr(4), false},
		{"article with nothing", func(in *itemForm) { in.Format = "article" }, nil, nil, false},
		{"not a number", func(in *itemForm) { in.SizeValue = "3.5" }, nil, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := validForm("shelf")
			tc.edit(&in)
			item, _, err := in.toItem()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !samePtr(item.SizeValue, tc.wantSize) || !samePtr(item.WordCount, tc.wantWords) {
				t.Fatalf("size=%v words=%v, want %v %v", deref(item.SizeValue), deref(item.WordCount), deref(tc.wantSize), deref(tc.wantWords))
			}
		})
	}
}

func ptr(i int) *int { return &i }

func samePtr(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
