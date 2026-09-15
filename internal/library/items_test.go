package library_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

func TestCreateItemValidation(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "Go")

	tests := []struct {
		name  string
		edit  func(*library.Item)
		field string // expected ValidationError field; "" means ErrNotFound
	}{
		{"missing title", func(it *library.Item) { it.Title = "  " }, "title"},
		{"missing why", func(it *library.Item) { it.Why = "" }, "why"},
		{"unknown format", func(it *library.Item) { it.Format = "scroll" }, "format"},
		{"unknown focus", func(it *library.Item) { it.FocusDemand = "intense" }, "focus_demand"},
		{"unknown unit", func(it *library.Item) { it.SizeUnit = "chapters" }, "size_unit"},
		{"word count on a book", func(it *library.Item) { it.WordCount = ptr(90000) }, "word_count"},
		{"unknown shelf", func(it *library.Item) { it.ShelfID = "nope" }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := library.Item{Title: "T", Why: "W", Format: library.FormatBook, ShelfID: shelf.ID}
			tc.edit(&item)
			_, err := svc.CreateItem(ctx, item)
			if tc.field == "" {
				if !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("got %v, want ErrNotFound", err)
				}
				return
			}
			var verr *library.ValidationError
			if !errors.As(err, &verr) || verr.Field != tc.field {
				t.Fatalf("got %v, want ValidationError on %s", err, tc.field)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	tests := []struct {
		format library.Format
		want   library.FormatDefaults
	}{
		{library.FormatBook, library.FormatDefaults{FocusDemand: library.FocusMedium, SizeUnit: library.UnitPages}},
		{library.FormatVideo, library.FormatDefaults{FocusDemand: library.FocusLight, SizeUnit: library.UnitMinutes}},
		{library.FormatArticle, library.FormatDefaults{FocusDemand: library.FocusLight, SizeUnit: library.UnitWords}},
		{library.FormatPaper, library.FormatDefaults{FocusDemand: library.FocusDeep, SizeUnit: library.UnitPages, NeedsDesk: true}},
		{library.FormatCourse, library.FormatDefaults{FocusDemand: library.FocusMedium, SizeUnit: library.UnitMinutes, NeedsDesk: true}},
		{"scroll", library.FormatDefaults{}},
	}
	for _, tc := range tests {
		if got := library.Defaults(tc.format); got != tc.want {
			t.Errorf("Defaults(%s) = %+v, want %+v", tc.format, got, tc.want)
		}
	}
	if len(library.Formats) != 5 {
		t.Fatalf("Formats has %d entries, want 5", len(library.Formats))
	}
}

func TestCreateItemDefaultsFromFormat(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "Mixed")

	for _, format := range library.Formats {
		t.Run(string(format), func(t *testing.T) {
			item := newItem(t, svc, shelf.ID, "x", func(it *library.Item) { it.Format = format })
			want := library.Defaults(format)
			if item.FocusDemand != want.FocusDemand || item.SizeUnit != want.SizeUnit {
				t.Fatalf("got %s/%s, want %s/%s", item.FocusDemand, item.SizeUnit, want.FocusDemand, want.SizeUnit)
			}
			if item.State != library.StatePool || item.CreatedAt.IsZero() {
				t.Fatalf("new item should be in pool with timestamps: %+v", item)
			}
		})
	}

	t.Run("explicit values win", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "x", func(it *library.Item) {
			it.Format = library.FormatVideo
			it.FocusDemand = library.FocusDeep
			it.NeedsDesk = true
		})
		if item.FocusDemand != library.FocusDeep || !item.NeedsDesk {
			t.Fatalf("explicit values overridden: %s desk=%v", item.FocusDemand, item.NeedsDesk)
		}
	})

	t.Run("desk is the caller's choice", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "x", func(it *library.Item) { it.Format = library.FormatPaper })
		if item.NeedsDesk {
			t.Fatal("paper forced needs_desk; the form pre-fills it, CreateItem must not")
		}
	})
}

func TestTransitions(t *testing.T) {
	type op func(svc *library.Service, id string) error
	start := func(svc *library.Service, id string) error { _, err := svc.Start(ctx, id); return err }
	finish := func(svc *library.Service, id string) error { _, err := svc.Finish(ctx, id, "good"); return err }
	reference := func(svc *library.Service, id string) error { _, err := svc.Reference(ctx, id, ""); return err }
	abandon := func(svc *library.Service, id string) error { _, err := svc.Abandon(ctx, id, "dull"); return err }

	// setups bring a fresh pool item to the named state.
	toInProgress := func(svc *library.Service, id string) { startItem(t, svc, id) }
	toFinished := func(svc *library.Service, id string) {
		startItem(t, svc, id)
		if _, err := svc.Finish(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name  string
		setup func(*library.Service, string)
		op    op
		want  error // nil means success
		state library.State
	}{
		{"pool start", nil, start, nil, library.StateInProgress},
		{"pool finish", nil, finish, library.ErrInvalidTransition, library.StatePool},
		{"pool reference", nil, reference, library.ErrInvalidTransition, library.StatePool},
		{"pool abandon", nil, abandon, nil, library.StateAbandoned},
		{"in_progress start", toInProgress, start, library.ErrInvalidTransition, library.StateInProgress},
		{"in_progress finish", toInProgress, finish, nil, library.StateFinished},
		{"in_progress reference", toInProgress, reference, nil, library.StateReference},
		{"in_progress abandon", toInProgress, abandon, nil, library.StateAbandoned},
		{"finished start", toFinished, start, library.ErrInvalidTransition, library.StateFinished},
		{"finished abandon", toFinished, abandon, library.ErrInvalidTransition, library.StateFinished},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestLibrary(t)
			shelf := newShelf(t, svc, "S")
			item := newItem(t, svc, shelf.ID, "x")
			if tc.setup != nil {
				tc.setup(svc, item.ID)
			}
			if err := tc.op(svc, item.ID); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			got, err := svc.GetItem(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.state {
				t.Fatalf("state %s, want %s", got.State, tc.state)
			}
		})
	}
}

func TestTransitionsRecordOutcome(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")

	finished := newItem(t, svc, shelf.ID, "finished")
	startItem(t, svc, finished.ID)
	clk.Advance(time.Hour)
	got, err := svc.Finish(ctx, finished.ID, " worth it ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != "worth it" || got.FinishedAt == nil || !got.FinishedAt.Equal(clk.Now()) || got.StartedAt == nil {
		t.Fatalf("finish did not record verdict/timestamps: %+v", got)
	}

	abandoned := newItem(t, svc, shelf.ID, "abandoned")
	if _, err := svc.Abandon(ctx, abandoned.ID, "  "); !errors.Is(err, library.ErrReasonRequired) {
		t.Fatalf("got %v, want ErrReasonRequired", err)
	}
	got, err = svc.Abandon(ctx, abandoned.ID, "not for me")
	if err != nil {
		t.Fatal(err)
	}
	if got.AbandonedReason != "not for me" || got.FinishedAt == nil {
		t.Fatalf("abandon did not record reason: %+v", got)
	}
}

func TestWIPCap(t *testing.T) {
	svc, _ := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.WIPCap = 2
	if err := svc.UpdateSettings(ctx, *settings); err != nil {
		t.Fatal(err)
	}

	a := newItem(t, svc, shelf.ID, "a")
	b := newItem(t, svc, shelf.ID, "b")
	c := newItem(t, svc, shelf.ID, "c")
	startItem(t, svc, a.ID)
	startItem(t, svc, b.ID)

	if _, err := svc.Start(ctx, c.ID); !errors.Is(err, library.ErrWIPCapReached) {
		t.Fatalf("third start: got %v, want ErrWIPCapReached", err)
	}
	got, _ := svc.GetItem(ctx, c.ID)
	if got.State != library.StatePool {
		t.Fatalf("capped item must stay in pool, got %s", got.State)
	}

	if _, err := svc.Finish(ctx, a.ID, ""); err != nil {
		t.Fatal(err)
	}
	startItem(t, svc, c.ID) // one slot freed
}

func TestDeleteItem(t *testing.T) {
	svc, clk := newTestLibrary(t)
	shelf := newShelf(t, svc, "S")

	t.Run("without sessions", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "x")
		setTags(t, svc, item.ID, "go")
		if err := svc.DeleteItem(ctx, item.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.GetItem(ctx, item.ID); !errors.Is(err, library.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("with sessions", func(t *testing.T) {
		item := newItem(t, svc, shelf.ID, "y")
		startItem(t, svc, item.ID)
		start := clk.Now()
		clk.Advance(10 * time.Minute)
		if _, err := svc.AddRetroactiveSession(ctx, item.ID, start, clk.Now(), nil, nil, ""); err != nil {
			t.Fatal(err)
		}
		if err := svc.DeleteItem(ctx, item.ID); !errors.Is(err, library.ErrHasSessions) {
			t.Fatalf("got %v, want ErrHasSessions", err)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if err := svc.DeleteItem(ctx, "nope"); !errors.Is(err, library.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("ranked item shifts the shelf", func(t *testing.T) {
		a := newItem(t, svc, shelf.ID, "a")
		b := newItem(t, svc, shelf.ID, "b")
		rank(t, svc, shelf.ID, a.ID, 1)
		rank(t, svc, shelf.ID, b.ID, 2)
		if err := svc.DeleteItem(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		wantIDs(t, slotIDs(t, svc, shelf.ID), b.ID)
	})
}

func TestUpdateItem(t *testing.T) {
	svc, _ := newTestLibrary(t)
	home := newShelf(t, svc, "Home")
	other := newShelf(t, svc, "Other")

	t.Run("keeps lifecycle fields", func(t *testing.T) {
		item := newItem(t, svc, home.ID, "x")
		startItem(t, svc, item.ID)
		item.Title = "renamed"
		item.State = library.StatePool // must be ignored
		got, err := svc.UpdateItem(ctx, *item)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "renamed" || got.State != library.StateInProgress || got.StartedAt == nil {
			t.Fatalf("update touched lifecycle fields: %+v", got)
		}
	})

	t.Run("moving shelf drops the old rank", func(t *testing.T) {
		item := newItem(t, svc, home.ID, "moved")
		rank(t, svc, home.ID, item.ID, 1)
		item.ShelfID = other.ID
		if _, err := svc.UpdateItem(ctx, *item); err != nil {
			t.Fatal(err)
		}
		wantIDs(t, slotIDs(t, svc, home.ID))
	})

	t.Run("moving shelf keeps the rank when a tag borrows it back", func(t *testing.T) {
		item := newItem(t, svc, home.ID, "tagged")
		setTags(t, svc, item.ID, "home")
		rank(t, svc, home.ID, item.ID, 1)
		item.ShelfID = other.ID
		if _, err := svc.UpdateItem(ctx, *item); err != nil {
			t.Fatal(err)
		}
		wantIDs(t, slotIDs(t, svc, home.ID), item.ID)
	})

	t.Run("validates like create", func(t *testing.T) {
		item := newItem(t, svc, home.ID, "v")
		item.Why = ""
		var verr *library.ValidationError
		if _, err := svc.UpdateItem(ctx, *item); !errors.As(err, &verr) || verr.Field != "why" {
			t.Fatalf("got %v, want ValidationError on why", err)
		}
	})
}
