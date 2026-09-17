package library_test

import (
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// The archive holds what was finished or kept for reference, newest first,
// and adds up books, pages and time. Abandoned and open items stay out.
func TestArchive(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	pages := func(n int) func(*library.Item) { return func(it *library.Item) { it.SizeValue = ptr(n) } }

	first := startItem(t, svc, newItem(t, svc, shelf.ID, "First", pages(200)).ID)
	start := clk.Now()
	if _, err := svc.AddRetroactiveSession(ctx, first.ID, start.Add(-time.Hour), start, ptr(200), ""); err != nil {
		t.Fatal(err)
	}
	clk.Advance(48 * time.Hour)
	if _, err := svc.Finish(ctx, first.ID, "good", nil); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Hour)
	article := startItem(t, svc, newItem(t, svc, shelf.ID, "Article", func(it *library.Item) { it.Format = library.FormatArticle }).ID)
	if _, err := svc.Finish(ctx, article.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Hour)
	spec := startItem(t, svc, newItem(t, svc, shelf.ID, "Spec", pages(90)).ID)
	if _, err := svc.Reference(ctx, spec.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	dropped := startItem(t, svc, newItem(t, svc, shelf.ID, "Dropped").ID)
	if _, err := svc.Abandon(ctx, dropped.ID, "no"); err != nil {
		t.Fatal(err)
	}
	startItem(t, svc, newItem(t, svc, shelf.ID, "Open").ID)

	a, err := svc.Archive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Items) != 3 || a.Items[0].Item.Title != "Spec" || a.Items[2].Item.Title != "First" {
		t.Fatalf("items %+v", a.Items)
	}
	if a.Books != 1 || a.Other != 1 || a.Reference != 1 || a.Pages != 200 || a.Time != time.Hour {
		t.Fatalf("archive %+v", a)
	}
	if got := a.Items[2]; got.Time != time.Hour || got.Sessions != 1 || got.Days != 3 {
		t.Fatalf("first %+v; want 1 h, 1 session, 3 days", got)
	}
}
