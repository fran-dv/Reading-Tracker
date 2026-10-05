package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// shelfFixture builds two shelves: Statistics holds a textbook tagged IQ, a
// book being read, and a plain pool book; IQ borrows the textbook.
type shelfFixture struct {
	handler  http.Handler
	svc      *library.Service
	stats    *library.Shelf
	iq       *library.Shelf
	textbook *library.Item
	reading  *library.Item
	pool     *library.Item
}

func newShelfFixture(t *testing.T, opts ...library.Option) shelfFixture {
	t.Helper()
	h, svc := newTestServer(t, &fakeMeta{}, opts...)
	f := shelfFixture{handler: h, svc: svc}
	var err error
	if f.stats, err = svc.CreateShelf(ctx, "Statistics"); err != nil {
		t.Fatal(err)
	}
	if f.iq, err = svc.CreateShelf(ctx, "IQ"); err != nil {
		t.Fatal(err)
	}
	pages := 240
	f.textbook = fileItem(t, svc, library.Item{
		Title: "Statistics for Hackers", Author: "Jake VanderPlas", Why: "the one that clicked",
		Format: library.FormatBook, ShelfID: f.stats.ID, SizeValue: &pages,
	}, "IQ")
	f.reading = fileItem(t, svc, library.Item{
		Title: "Thinking in Systems", Why: "loops everywhere",
		Format: library.FormatBook, ShelfID: f.stats.ID,
	})
	if _, err := svc.Start(ctx, f.reading.ID); err != nil {
		t.Fatal(err)
	}
	f.pool = fileItem(t, svc, library.Item{
		Title: "The Art of Doing Science", Why: "for the long game",
		Format: library.FormatBook, ShelfID: f.stats.ID,
	})
	return f
}

func fileItem(t *testing.T, svc *library.Service, item library.Item, tags ...string) *library.Item {
	t.Helper()
	created, err := svc.CreateItem(ctx, item, tags)
	if err != nil {
		t.Fatalf("file %q: %v", item.Title, err)
	}
	return created
}

func TestShelvesIndex(t *testing.T) {
	f := newShelfFixture(t)
	body := get(t, f.handler, "/shelves").Body.String()
	for _, want := range []string{"Statistics", "IQ", `href="/shelves/` + f.stats.ID + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("index missing %q", want)
		}
	}
	if strings.Contains(body, "Statistics for Hackers") {
		t.Error("the index lists shelves, not their contents")
	}

	h, _ := newTestServer(t, &fakeMeta{})
	if empty := get(t, h, "/shelves").Body.String(); !strings.Contains(empty, "No shelves yet") {
		t.Error("an empty library should say so")
	}
}

// The index writes the order on the margin rule, and nothing else about a
// shelf: it is a door, not a summary (spec §0).
func TestShelvesIndexWritesTheOrder(t *testing.T) {
	f := newShelfFixture(t)
	body := get(t, f.handler, "/shelves").Body.String()
	first := `<span class="entry-slot entry-order">1</span>`
	if !strings.Contains(body, first) || !strings.Contains(body, `<span class="entry-slot entry-order">2</span>`) {
		t.Errorf("index should number the shelves down the rule:\n%s", body)
	}
	if strings.Index(body, first) > strings.Index(body, ">Statistics<") {
		t.Error("a shelf's number should be written before its name")
	}
}

// A shelf being read shows how far in it is and whether it has stalled, as
// Home and the review do: the shelf is where the next thing is chosen.
func TestShelfMarksReadingAndStall(t *testing.T) {
	now := time.Now()
	f := newShelfFixture(t, library.WithClock(func() time.Time { return now }))
	if _, err := f.svc.Start(ctx, f.textbook.ID); err != nil {
		t.Fatal(err)
	}
	session, err := f.svc.StartSession(ctx, f.textbook.ID)
	if err != nil {
		t.Fatal(err)
	}
	reached := 60 // of 240 pages
	if _, err := f.svc.StopSession(ctx, session.ID, library.Stop{Reached: &reached}); err != nil {
		t.Fatal(err)
	}

	body := get(t, f.handler, "/shelves/"+f.stats.ID).Body.String()
	if !strings.Contains(body, `--read: 0.250`) {
		t.Errorf("the position track should show a quarter read:\n%s", body)
	}
	if strings.Contains(body, "entry-stall") {
		t.Error("an item read just now has not stalled")
	}

	now = now.Add(20 * 24 * time.Hour) // past the 14-day stall
	body = get(t, f.handler, "/shelves/"+f.stats.ID).Body.String()
	if !strings.Contains(body, "entry-stall") || !strings.Contains(body, `class="mark mark-warn">stalled`) {
		t.Errorf("a stalled item should be marked on the rule and in words:\n%s", body)
	}
}

func TestOrdinal(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{1, "1st"}, {2, "2nd"}, {3, "3rd"}, {4, "4th"}, {11, "11th"}, {12, "12th"},
		{13, "13th"}, {21, "21st"}, {22, "22nd"}, {23, "23rd"}, {101, "101st"}, {111, "111th"}} {
		if got := ordinal(c.n); got != c.want {
			t.Errorf("ordinal(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestShelfPage(t *testing.T) {
	f := newShelfFixture(t)
	body := get(t, f.handler, "/shelves/"+f.stats.ID).Body.String()

	for _, want := range []string{
		"Statistics",                        // heading
		"Statistics for Hackers",            // own item
		"the one that clicked",              // its why
		"Jake VanderPlas",                   // author
		"240 pages",                         // size in its own unit
		"reading",                           // the in-progress mark
		"Empty",                             // the slots nobody has filled
		`data-href="/shelves/` + f.stats.ID, // actions hang off the shelf
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shelf page missing %q", want)
		}
	}
	// In progress leads the pool, and cannot be ranked.
	if i, j := strings.Index(body, "Thinking in Systems"), strings.Index(body, "Statistics for Hackers"); i > j {
		t.Error("the item being read should lead the pool")
	}
	if n := strings.Count(body, ">Rank<"); n != 2 {
		t.Errorf("got %d Rank controls, want 2 (the in-progress item cannot be ranked)", n)
	}

	// The borrowed item names the shelf it came from.
	borrowed := get(t, f.handler, "/shelves/"+f.iq.ID).Body.String()
	if !strings.Contains(borrowed, "from Statistics") {
		t.Errorf("IQ should mark the textbook borrowed: %s", borrowed)
	}

	if rec := get(t, f.handler, "/shelves/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown shelf: status %d, want 404", rec.Code)
	}
}

func TestShelfEmpty(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	shelf, err := svc.CreateShelf(ctx, "Empty")
	if err != nil {
		t.Fatal(err)
	}
	if body := get(t, h, "/shelves/"+shelf.ID).Body.String(); !strings.Contains(body, "Nothing filed here yet") {
		t.Error("an empty shelf should say so")
	}
}

func TestRankActions(t *testing.T) {
	f := newShelfFixture(t)
	base := "/shelves/" + f.stats.ID + "/items/"

	rec := send(t, f.handler, http.MethodPost, base+f.textbook.ID+"/rank", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="shelf-body"`) {
		t.Fatalf("rank: status %d, body %s", rec.Code, rec.Body.String())
	}
	wantSlots(t, f, f.textbook.ID)

	send(t, f.handler, http.MethodPost, base+f.pool.ID+"/rank", nil)
	wantSlots(t, f, f.textbook.ID, f.pool.ID)

	send(t, f.handler, http.MethodPost, base+f.pool.ID+"/up", nil)
	wantSlots(t, f, f.pool.ID, f.textbook.ID)

	send(t, f.handler, http.MethodPost, base+f.pool.ID+"/down", nil)
	wantSlots(t, f, f.textbook.ID, f.pool.ID)

	send(t, f.handler, http.MethodPost, base+f.textbook.ID+"/unrank", nil)
	wantSlots(t, f, f.pool.ID)

	// Ranking is per shelf: the textbook is borrowed by IQ and may lead there.
	send(t, f.handler, http.MethodPost, "/shelves/"+f.iq.ID+"/items/"+f.textbook.ID+"/rank", nil)
	view, err := f.svc.ShelfItems(ctx, f.iq.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Slots[0] == nil || view.Slots[0].ID != f.textbook.ID {
		t.Error("the borrowed textbook should hold slot 1 on IQ")
	}
}

func TestRankRefusedWhenSlotsAreFull(t *testing.T) {
	f := newShelfFixture(t)
	base := "/shelves/" + f.stats.ID + "/items/"
	third := fileItem(t, f.svc, library.Item{
		Title: "A third", Why: "w", Format: library.FormatBook, ShelfID: f.stats.ID,
	})
	for _, item := range []*library.Item{f.textbook, f.pool, third} {
		send(t, f.handler, http.MethodPost, base+item.ID+"/rank", nil)
	}
	if got := len(slotted(t, f)); got != 3 {
		t.Fatalf("filled %d slots, want 3", got)
	}

	spare := fileItem(t, f.svc, library.Item{
		Title: "spare", Why: "w", Format: library.FormatBook, ShelfID: f.stats.ID,
	})
	if rec := send(t, f.handler, http.MethodPost, base+spare.ID+"/rank", nil); rec.Code != http.StatusConflict {
		t.Errorf("rank into a full shelf: status %d, want 409", rec.Code)
	}
	body := get(t, f.handler, "/shelves/"+f.stats.ID).Body.String()
	if strings.Contains(body, ">Rank<") {
		t.Error("with every slot taken, no entry should offer Rank")
	}
	if !strings.Contains(body, "Three slots taken") {
		t.Error("the page should say why Rank is gone")
	}
}

func TestEntryFormOpensAndCancels(t *testing.T) {
	f := newShelfFixture(t)
	rec := send(t, f.handler, http.MethodGet, "/shelves/"+f.stats.ID+"/items/"+f.textbook.ID+"/edit", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `id="entry-form"`) {
		t.Fatalf("edit should open a form: %s", body)
	}
	for _, want := range []string{"Statistics for Hackers", "the one that clicked", "IQ", "Save", "Cancel", "Look it up again"} {
		if !strings.Contains(body, want) {
			t.Errorf("the open form is missing %q", want)
		}
	}
	// While one row is a form, no other row offers its controls.
	if strings.Contains(body, ">Edit<") || strings.Contains(body, ">Rank<") || strings.Contains(body, ">Unrank<") {
		t.Error("other rows must lose their controls while an entry is being edited")
	}

	cancelled := send(t, f.handler, http.MethodGet, "/shelves/"+f.stats.ID+"/body", nil).Body.String()
	if strings.Contains(cancelled, `id="entry-form"`) || !strings.Contains(cancelled, ">Edit<") {
		t.Error("cancel should close the form and bring the controls back")
	}
}

func TestPostEntrySaves(t *testing.T) {
	f := newShelfFixture(t)
	in := newItemForm(f.iq.ID, "IQ")
	in.Title, in.Why, in.Author = "Statistics, corrected", "still the one", "J. VanderPlas"
	in.Tags, in.SizeValue, in.Format = "stats, iq", "260", "book"
	in.FocusDemand = "deep"

	rec := send(t, f.handler, http.MethodPost, "/shelves/"+f.stats.ID+"/items/"+f.textbook.ID, in)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "moved to IQ") {
		t.Errorf("a move off the shelf should say where it went: %s", rec.Body.String())
	}

	saved, err := f.svc.GetItem(ctx, f.textbook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != "Statistics, corrected" || saved.ShelfID != f.iq.ID || saved.FocusDemand != library.FocusDeep {
		t.Fatalf("item not saved: %+v", saved)
	}
	if saved.SizeValue == nil || *saved.SizeValue != 260 {
		t.Fatalf("size not saved: %+v", saved.SizeValue)
	}
	tags, err := f.svc.Tags(ctx, f.textbook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tags, ","); got != "iq,stats" {
		t.Fatalf("tags not saved with the item: %v", tags)
	}
}

func TestPostEntryValidationKeepsTheForm(t *testing.T) {
	f := newShelfFixture(t)
	in := newItemForm(f.stats.ID, "Statistics")
	in.Title, in.Why = "Statistics for Hackers", "   " // a why is required (§4)

	rec := send(t, f.handler, http.MethodPost, "/shelves/"+f.stats.ID+"/items/"+f.textbook.ID, in)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: validation is an expected outcome", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `id="shelf-body"`) {
		t.Error("a rejected save must not re-render the form's inputs")
	}
	sig := patchedSignals(t, rec.Body.String())
	errs, _ := sig["errors"].(map[string]any)
	if errs["why"] == "" {
		t.Errorf("want a message in the why slot: %v", sig)
	}
}

// wantSlots checks the shelf's slots, in order.
func wantSlots(t *testing.T, f shelfFixture, want ...string) {
	t.Helper()
	got := slotted(t, f)
	if len(got) != len(want) {
		t.Fatalf("got %d slotted items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slot %d holds %s, want %s", i+1, got[i], want[i])
		}
	}
}

func slotted(t *testing.T, f shelfFixture) []string {
	t.Helper()
	view, err := f.svc.ShelfItems(ctx, f.stats.ID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, slot := range view.Slots {
		if slot != nil {
			ids = append(ids, slot.ID)
		}
	}
	return ids
}

// Editing never takes an item off the shortlist, and once it has sessions
// a format measured in another unit is refused in the form.
func TestPostEntryKeepsShortlistAndUnit(t *testing.T) {
	f := newShelfFixture(t)
	path := "/shelves/" + f.stats.ID + "/items/" + f.textbook.ID
	if _, err := f.svc.SetShortlist(ctx, f.textbook.ID, true); err != nil {
		t.Fatal(err)
	}
	in := newItemForm(f.stats.ID, "Statistics")
	in.Title, in.Why, in.Format, in.FocusDemand = "Statistics, corrected", "still the one", "book", "medium"
	if rec := send(t, f.handler, http.MethodPost, path, in); rec.Code != http.StatusOK {
		t.Fatalf("save: %d", rec.Code)
	}
	if item, _ := f.svc.GetItem(ctx, f.textbook.ID); !item.OnShortlist {
		t.Fatal("editing took the item off the shortlist")
	}

	if _, err := f.svc.Start(ctx, f.textbook.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.AddRetroactiveSession(ctx, f.textbook.ID, now.Add(-time.Hour), now, ptr(20), ""); err != nil {
		t.Fatal(err)
	}
	in.Format = "article"
	errs := errorsOf(t, send(t, f.handler, http.MethodPost, path, in).Body.String())
	if msg, _ := errs["format"].(string); !strings.Contains(msg, "measured in pages") {
		t.Fatalf("errors = %v", errs)
	}
}

// Saving applies a staged cover change to an existing item, and
// reverting it afterward (no draft needed for found) clears it again
// (cover-management: Cover Changes Are Staged in the Form Until Filed
// or Saved).
func TestPostEntryAppliesAStagedCoverChange(t *testing.T) {
	f := newShelfFixture(t)
	path := "/shelves/" + f.stats.ID + "/items/" + f.pool.ID

	tags, err := f.svc.Tags(ctx, f.pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := itemFormFor(f.pool, tags, f.stats.Name)
	in.CoverChoice, in.CoverDraft = "uploaded", draftToken(t, f.handler, tinyJPEG(t))
	if rec := send(t, f.handler, http.MethodPost, path, in); rec.Code != http.StatusOK {
		t.Fatalf("save: status %d: %s", rec.Code, rec.Body.String())
	}

	saved, err := f.svc.GetItem(ctx, f.pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.CoverChoice != library.CoverUploaded {
		t.Fatalf("cover choice = %q, want uploaded", saved.CoverChoice)
	}
	if rec := get(t, f.handler, "/items/"+f.pool.ID+"/cover"); rec.Code != http.StatusOK {
		t.Errorf("uploaded cover not served: status %d", rec.Code)
	}

	// Reverting needs no draft: found carries no bytes of its own.
	in2 := itemFormFor(saved, tags, f.stats.Name)
	in2.CoverChoice = "found"
	if rec := send(t, f.handler, http.MethodPost, path, in2); rec.Code != http.StatusOK {
		t.Fatalf("revert: status %d", rec.Code)
	}
	reverted, err := f.svc.GetItem(ctx, f.pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reverted.CoverChoice != library.CoverFound {
		t.Fatalf("cover choice = %q, want found after reverting", reverted.CoverChoice)
	}
	if rec := get(t, f.handler, "/items/"+f.pool.ID+"/cover"); rec.Code != http.StatusNotFound {
		t.Errorf("a reverted item with no found link should show no cover: status %d", rec.Code)
	}
}

// An edit that leaves an already-uploaded cover untouched (the form seeds
// CoverChoice from the item and no new draft) must save, not refuse for a
// "lost" draft that was never expected in the first place.
func TestPostEntryEditingAnUploadedCoverWithoutChangingItStillSaves(t *testing.T) {
	f := newShelfFixture(t)
	path := "/shelves/" + f.stats.ID + "/items/" + f.pool.ID
	tags, err := f.svc.Tags(ctx, f.pool.ID)
	if err != nil {
		t.Fatal(err)
	}

	in := itemFormFor(f.pool, tags, f.stats.Name)
	in.CoverChoice, in.CoverDraft = "uploaded", draftToken(t, f.handler, tinyJPEG(t))
	if rec := send(t, f.handler, http.MethodPost, path, in); rec.Code != http.StatusOK {
		t.Fatalf("stage the upload: status %d: %s", rec.Code, rec.Body.String())
	}
	withCover, err := f.svc.GetItem(ctx, f.pool.ID)
	if err != nil {
		t.Fatal(err)
	}

	// A second edit, seeded straight from the saved item (its cover choice
	// is "uploaded" and no fresh draft is staged), changing only Why.
	in2 := itemFormFor(withCover, tags, f.stats.Name)
	in2.Why = "a better reason, same cover"
	rec := send(t, f.handler, http.MethodPost, path, in2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "The uploaded cover was lost") {
		t.Fatalf("an untouched cover should not need its draft again:\n%s", rec.Body.String())
	}
	saved, err := f.svc.GetItem(ctx, f.pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Why != "a better reason, same cover" {
		t.Fatalf("why not saved: %q", saved.Why)
	}
	if saved.CoverChoice != library.CoverUploaded {
		t.Fatalf("cover choice = %q, want uploaded still", saved.CoverChoice)
	}
	if rec := get(t, f.handler, "/items/"+f.pool.ID+"/cover"); rec.Code != http.StatusOK {
		t.Errorf("the untouched uploaded cover should still be served: status %d", rec.Code)
	}
}
