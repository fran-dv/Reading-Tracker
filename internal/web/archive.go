package web

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"slices"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// The finished archive (spec §6.6): the one screen of what was gained. A
// shelf of spines grows book by book, and the months below list each item
// with its opening why beside its closing verdict.

type archiveBody struct {
	Books   int
	Summary []string // "3 other items", "2 kept for reference", "6,480 pages", "120 h of reading"
	Spines  []spine  // oldest first, as a shelf fills
	Months  []archiveMonth
}

// spine is one closed item on the drawn shelf: as thick as it is long.
type spine struct {
	Href      string
	Title     string // for its tooltip: "Stoner · 288 pages · 12 Sep"
	Format    library.Format
	Width     float64 // rem
	Height    float64 // rem
	Reference bool
}

type archiveMonth struct {
	Label string // "September 2026"
	Count string // "3 finished"
	Items []archiveEntry
}

type archiveEntry struct {
	library.ArchivedItem
	Href      string
	Size      string // "288 pages"
	Finished  string // "finished 12 Sep" or "kept for reference 12 Sep"
	Took      string // "9 h 20 min over 3 weeks"
	Reference bool
}

type archivePage struct {
	shell
	Body *archiveBody
}

func (h *handler) getArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loc, err := h.location(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	a, err := h.svc.Archive(ctx)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	b := &archiveBody{Books: a.Books}
	if a.Other > 0 {
		b.Summary = append(b.Summary, countLabel(a.Other, "other item"))
	}
	if a.Reference > 0 {
		b.Summary = append(b.Summary, fmt.Sprintf("%d kept for reference", a.Reference))
	}
	if a.Pages > 0 {
		b.Summary = append(b.Summary, grouped(a.Pages)+" pages")
	}
	if a.Time > 0 {
		b.Summary = append(b.Summary, minutesLabel(a.Time)+" of reading")
	}
	for _, ai := range slices.Backward(a.Items) {
		b.Spines = append(b.Spines, newSpine(ai, loc))
	}
	for _, ai := range a.Items {
		finished := ai.Item.FinishedAt.In(loc)
		label := finished.Format("January 2006")
		if n := len(b.Months); n == 0 || b.Months[n-1].Label != label {
			b.Months = append(b.Months, archiveMonth{Label: label})
		}
		m := &b.Months[len(b.Months)-1]
		m.Items = append(m.Items, newArchiveEntry(ai, loc))
		m.Count = countLabel(len(m.Items), "item")
	}
	h.render(w, r, h.archive, archivePage{shell: h.newShell(ctx, "/archive"), Body: b})
}

func newArchiveEntry(ai library.ArchivedItem, loc *time.Location) archiveEntry {
	e := archiveEntry{ArchivedItem: ai, Href: "/items/" + ai.Item.ID, Size: sizeLabel(ai.Item),
		Reference: ai.Item.State == library.StateReference}
	day := ai.Item.FinishedAt.In(loc).Format("2 Jan")
	e.Finished = "finished " + day
	if e.Reference {
		e.Finished = "kept for reference " + day
	}
	if ai.Time > 0 {
		e.Took = minutesLabel(ai.Time) + " over " + spanLabelDays(ai.Days)
	}
	return e
}

// spanLabelDays writes how long an item was open: "1 day", "5 days", "3 weeks".
func spanLabelDays(days int) string {
	if days >= 14 {
		return countLabel(days/7, "week")
	}
	return countLabel(max(days, 1), "day")
}

// newSpine draws an item as a book on a shelf: its thickness follows its
// pages (words and minutes count at a page's worth), its height varies a
// little by title so the row reads as books, not bars.
func newSpine(ai library.ArchivedItem, loc *time.Location) spine {
	it := ai.Item
	pages := 60.0 // an item without a size
	if it.SizeValue != nil {
		switch it.SizeUnit {
		case library.UnitPages:
			pages = float64(*it.SizeValue)
		case library.UnitWords:
			pages = float64(*it.SizeValue) / 300
		case library.UnitMinutes:
			pages = float64(*it.SizeValue) / 2
		}
	}
	title := it.Title
	if s := sizeLabel(it); s != "" {
		title += " · " + s
	}
	title += " · " + it.FinishedAt.In(loc).Format("2 Jan 2006")
	hash := fnv.New32a()
	hash.Write([]byte(it.Title))
	return spine{
		Href: "/items/" + it.ID, Title: title, Format: it.Format,
		Width:     min(max(pages/80, 0.4), 1.6),
		Height:    4.2 + float64(hash.Sum32()%9)*0.15,
		Reference: it.State == library.StateReference,
	}
}
