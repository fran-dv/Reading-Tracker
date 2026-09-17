package web

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
	"github.com/starfederation/datastar-go/datastar"
)

// The weekly review (spec §6.3): reread the whys, prune and rank, set the
// shortlist, see where the goals stand and what was read lately, then close
// the week. Every action re-renders the whole "review-body" block from fresh
// state: pruning an item changes the slots, the shortlist and the report at
// once, and one render keeps them from disagreeing.

// readingGroup is the group in-progress items sit in, in place of a shelf.
const readingGroup = "reading"

// reviewForm mirrors the page's signals: the reason typed into an open
// abandon form.
type reviewForm struct {
	Reason string            `json:"reason"`
	Errors map[string]string `json:"errors"`
}

// reviewRow is one item under a group: in progress, or a shelf's leader.
type reviewRow struct {
	Item       library.Item
	Slot       int    // 1–3 for a leader
	From       string // the home shelf, when borrowed onto this one
	Size       string
	Last       string // "read 12 Sep"
	Stalled    bool
	AtTop      bool
	AtBottom   bool
	Deletable  bool   // nothing logged on it
	Controls   bool   // no form is open anywhere
	Abandoning bool   // its abandon form is open
	Href       string // "/review/{group}/items/{id}"; actions hang off it
	Cancel     string
	DialogID   string
}

// reviewSlot is one of a shelf's three ranks.
type reviewSlot struct {
	Number int
	Row    *reviewRow
}

// reviewShelf is a shelf's leaders and, to fill an empty slot, its pool.
type reviewShelf struct {
	Shelf   library.Shelf
	Slots   []reviewSlot
	Leaders bool        // at least one slot is taken
	Free    int         // the first empty slot, 0 when all three are taken
	Pool    []reviewRow // unranked pool items, each rankable while a slot is free
}

// listRow is one item the shortlist can take.
type listRow struct {
	Item    library.Item
	Shelf   string
	Reading bool
	Href    string // "/review/items/{id}"
}

// poolGroup is one shelf's pool, reached into for the shortlist.
type poolGroup struct {
	Shelf string
	Rows  []listRow
}

type reviewBody struct {
	Week      string // "13 Sep"
	Reading   []reviewRow
	Shelves   []reviewShelf // with leaders or a pool
	Led       []reviewShelf // those with leaders, whose whys are reread
	Shortlist []listRow
	Pools     []poolGroup
	Listed    string // "6 on it"
	ListNote  string // a calm line below 5 or above 7
	Goal      goalView
	Report    reportView
	Closed    string // "Closed today at 21:40.", or ""
	Signals   string
	Status    string
}

type reviewPage struct {
	shell
	Body *reviewBody
}

func (h *handler) getReview(w http.ResponseWriter, r *http.Request) {
	body, err := h.reviewBody(r.Context(), "", "")
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	h.render(w, r, h.review, reviewPage{shell: h.newShell(r.Context(), "/review"), Body: body})
}

// getReviewBody redraws with nothing open. It is how an abandon is cancelled.
func (h *handler) getReviewBody(w http.ResponseWriter, r *http.Request) {
	h.patchReview(w, r, "", "", nil)
}

// getReviewAbandon opens the abandon form on one row. An item can lead two
// shelves; the group says under which one it opens.
func (h *handler) getReviewAbandon(w http.ResponseWriter, r *http.Request) {
	if !h.patchReview(w, r, openKey(r.PathValue("group"), r.PathValue("id")), "", nil) {
		return
	}
	h.focusReason(datastar.NewSSE(w, r))
}

func (h *handler) postReviewAbandon(w http.ResponseWriter, r *http.Request) {
	var in reviewForm
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.svc.Abandon(r.Context(), r.PathValue("id"), in.Reason)
	if errors.Is(err, library.ErrReasonRequired) {
		h.reasonMissing(w, r)
		return
	}
	status := ""
	if err == nil {
		status = "Abandoned " + item.Title + "."
	}
	h.patchReview(w, r, "", status, err)
}

// postReviewDelete removes an item with nothing logged (spec §2.1). One with
// history must be abandoned instead, and the status line says so.
func (h *handler) postReviewDelete(w http.ResponseWriter, r *http.Request) {
	ctx, id := r.Context(), r.PathValue("id")
	item, err := h.svc.GetItem(ctx, id)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	status := "Deleted " + item.Title + "."
	err = h.svc.DeleteItem(ctx, id)
	if errors.Is(err, library.ErrHasSessions) {
		status, err = item.Title+" has reading logged. Abandon it instead.", nil
	}
	h.patchReview(w, r, "", status, err)
}

func (h *handler) postReviewUp(w http.ResponseWriter, r *http.Request) {
	h.patchReview(w, r, "", "", h.svc.MoveUp(r.Context(), r.PathValue("group"), r.PathValue("id")))
}

func (h *handler) postReviewDown(w http.ResponseWriter, r *http.Request) {
	h.patchReview(w, r, "", "", h.svc.MoveDown(r.Context(), r.PathValue("group"), r.PathValue("id")))
}

func (h *handler) postReviewUnrank(w http.ResponseWriter, r *http.Request) {
	h.patchReview(w, r, "", "", h.svc.Unrank(r.Context(), r.PathValue("group"), r.PathValue("id")))
}

// postReviewRank fills the shelf's first empty slot from its pool (§5.1).
func (h *handler) postReviewRank(w http.ResponseWriter, r *http.Request) {
	ctx, shelfID := r.Context(), r.PathValue("group")
	view, err := h.svc.ShelfItems(ctx, shelfID)
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	slot := freeSlot(view)
	if slot == 0 {
		http.Error(w, "all three slots are taken", http.StatusConflict)
		return
	}
	h.patchReview(w, r, "", "", h.svc.Rank(ctx, shelfID, r.PathValue("id"), slot))
}

// postReviewShortlist and postReviewUnshortlist write each tick at once
// (spec §5.2): the shortlist in effect is last week's carried over.
func (h *handler) postReviewShortlist(w http.ResponseWriter, r *http.Request) {
	_, err := h.svc.SetShortlist(r.Context(), r.PathValue("id"), true)
	h.patchReview(w, r, "", "", err)
}

func (h *handler) postReviewUnshortlist(w http.ResponseWriter, r *http.Request) {
	_, err := h.svc.SetShortlist(r.Context(), r.PathValue("id"), false)
	h.patchReview(w, r, "", "", err)
}

// postCloseReview records the week, with what the campaign needs today.
func (h *handler) postCloseReview(w http.ResponseWriter, r *http.Request) {
	_, err := h.svc.CloseReview(r.Context())
	h.patchReview(w, r, "", "", err)
}

// patchReview answers an action: the body is rebuilt from fresh state, with
// open naming the row whose abandon form is open, and the typed reason is
// cleared. It reports whether the patch was sent.
func (h *handler) patchReview(w http.ResponseWriter, r *http.Request, open, status string, err error) bool {
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	body, err := h.reviewBody(r.Context(), open, status)
	if err != nil {
		h.httpError(w, r, err)
		return false
	}
	sse := datastar.NewSSE(w, r)
	if err := h.patch(sse, h.review, "review-body", body); err != nil {
		h.log.Error("review body", "err", err)
		return false
	}
	if err := sse.PatchSignals([]byte(body.Signals)); err != nil {
		h.log.Error("review reset", "err", err)
		return false
	}
	return true
}

// openKey names a row: an item under a group.
func openKey(group, itemID string) string { return group + "/" + itemID }

// reviewBody gathers the review as it should be drawn.
func (h *handler) reviewBody(ctx context.Context, open, status string) (*reviewBody, error) {
	settings, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	loc, err := settings.Location()
	if err != nil {
		return nil, err
	}
	view, err := h.svc.Review(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	body := &reviewBody{Week: view.WeekOf.Format("2 Jan"), Status: status}

	names := map[string]string{}
	for _, sv := range view.Shelves {
		names[sv.Shelf.ID] = sv.Shelf.Name
	}
	row := func(item library.Item, group string) reviewRow {
		href := "/review/" + group + "/items/" + item.ID
		return reviewRow{
			Item:       item,
			Size:       sizeLabel(item),
			Controls:   open == "",
			Abandoning: open == openKey(group, item.ID),
			Href:       href,
			Cancel:     "/review/body",
			DialogID:   "delete-" + group + "-" + item.ID,
		}
	}

	for _, rd := range view.Reading {
		out := row(rd.Item, readingGroup)
		out.Stalled = rd.Stalled
		out.Deletable = rd.LastReadAt == nil
		if rd.LastReadAt != nil {
			out.Last = "read " + dayLabel(*rd.LastReadAt, now, loc)
		}
		body.Reading = append(body.Reading, out)
	}

	for _, sv := range view.Shelves {
		free := freeSlot(&sv)
		shelf := reviewShelf{Shelf: sv.Shelf, Slots: make([]reviewSlot, library.MaxSlots), Free: free}
		last := library.MaxSlots
		if free > 0 {
			last = free - 1
		}
		for i, item := range sv.Slots {
			shelf.Slots[i].Number = i + 1
			if item == nil {
				continue
			}
			out := row(item.Item, sv.Shelf.ID)
			out.Slot, out.AtTop, out.AtBottom = i+1, i == 0, i+1 == last
			out.Deletable = true // only pool items lead, and nothing is logged in the pool
			if item.Borrowed {
				out.From = names[item.ShelfID]
			}
			shelf.Slots[i].Row, shelf.Leaders = &out, true
		}
		for _, item := range sv.Unranked {
			if item.State != library.StatePool {
				continue
			}
			out := row(item.Item, sv.Shelf.ID)
			if item.Borrowed {
				out.From = names[item.ShelfID]
			}
			shelf.Pool = append(shelf.Pool, out)
		}
		if shelf.Leaders || len(shelf.Pool) > 0 {
			body.Shelves = append(body.Shelves, shelf)
		}
		if shelf.Leaders {
			body.Led = append(body.Led, shelf)
		}
	}

	list := func(e library.ShortlistEntry) listRow {
		return listRow{Item: e.Item, Shelf: e.ShelfName, Reading: e.Item.State == library.StateInProgress, Href: "/review/items/" + e.Item.ID}
	}
	for _, e := range view.Shortlist {
		body.Shortlist = append(body.Shortlist, list(e))
	}
	for _, e := range view.Pool {
		if n := len(body.Pools); n == 0 || body.Pools[n-1].Shelf != e.ShelfName {
			body.Pools = append(body.Pools, poolGroup{Shelf: e.ShelfName})
		}
		g := &body.Pools[len(body.Pools)-1]
		g.Rows = append(g.Rows, list(e))
	}
	n := view.Shortlisted()
	body.Listed = fmt.Sprintf("%d on it", n)
	switch {
	case n < 5:
		body.ListNote = "Fewer than 5 leaves little to pick from when the moment is narrow."
	case n > 7:
		body.ListNote = "More than 7 is a long list to choose from, the thing the shortlist is there to avoid."
	}

	body.Goal = newGoalView(view, settings)
	body.Report = newReportView(view.Composition, *settings)
	if c := view.Closed; c != nil {
		body.Closed = fmt.Sprintf("Closed %s at %s.", dayLabel(c.ClosedAt, now, loc), c.ClosedAt.In(loc).Format("15:04"))
	}
	if body.Signals, err = marshalSignals(reviewForm{Errors: map[string]string{"reason": ""}}); err != nil {
		return nil, err
	}
	return body, nil
}

// goalView is where the goals stand (spec §6.3 step 4).
type goalView struct {
	Campaign *campaignView // an active campaign; nil otherwise
	Change   *changeView   // nil without an earlier review to compare with
	Board    *board
	Owed     string // "" when nothing is owed
	LastWeek []dayCell
	Dates    string // "6–12 Sep"
	Speed    *speedGoal
}

// changeView sets what the campaign needs now beside what the last review showed.
type changeView struct {
	Since string // "13 Sep"
	Rows  []changeRow
	Cause string // a line naming what moved it, over 10%
}

type changeRow struct {
	Label, Then, Now string
	Strong           bool
}

// speedGoal is the running speed ramp's target and its last check.
type speedGoal struct {
	Target string
	Check  *checkRow // nil before its first check
}

func newGoalView(view *library.ReviewView, st *library.Settings) goalView {
	sc := view.Schedule
	g := goalView{
		Board:    newBoard(sc, view.Speed, st.WordsPerPage),
		LastWeek: dayCells(sc.LastWeek, sc.Today),
		Dates:    weekDates(sc.LastWeek[0].Day, sc.WeekStart),
	}
	if sc.Owed > 0 {
		g.Owed = minutesLabel(sc.Owed)
	}
	if cs := view.Campaign; cs != nil && cs.Campaign.Active() && !cs.Over {
		g.Campaign = newCampaignView(cs, sc)
	}
	if c := view.Change; c != nil {
		g.Change = newChangeView(*c)
	}
	if sp := g.Board.Speed; sp != nil && sp.Ramp != nil && sp.Ramp.Running {
		g.Speed = &speedGoal{Target: sp.Ramp.Target}
		for i := range sp.Ramp.Checks {
			if !sp.Ramp.Checks[i].SoFar {
				g.Speed.Check = &sp.Ramp.Checks[i]
				break
			}
		}
	}
	return g
}

func newChangeView(c library.NeedsChange) *changeView {
	then, now := c.Then, c.Now
	v := &changeView{
		Since: c.Since.Format("2 Jan"),
		Rows: []changeRow{
			{Label: "Books left", Then: fmt.Sprint(then.BooksLeft), Now: fmt.Sprint(now.BooksLeft)},
			{Label: "Average book", Then: fmt.Sprintf("%.0f pages", then.AvgPages), Now: fmt.Sprintf("%.0f pages", now.AvgPages)},
			{Label: "Book pace", Then: fmt.Sprintf("%.0f pages/h", then.PagesPerHour), Now: fmt.Sprintf("%.0f pages/h", now.PagesPerHour)},
			{Label: "Time left", Then: timeLeft(then.WeeksLeft), Now: timeLeft(now.WeeksLeft)},
			{Label: "Needed each week", Then: minutesLabel(hours(then.WeeklyHours)), Now: minutesLabel(hours(now.WeeklyHours)), Strong: true},
		},
	}
	if len(c.Causes) == 0 {
		return v
	}
	moved := func(cause library.Cause) string {
		switch cause {
		case library.CauseBooks:
			return fmt.Sprintf("books left %s from %d to %d", direction(float64(then.BooksLeft), float64(now.BooksLeft)), then.BooksLeft, now.BooksLeft)
		case library.CausePages:
			return fmt.Sprintf("the average book %s from %.0f to %.0f pages", direction(then.AvgPages, now.AvgPages), then.AvgPages, now.AvgPages)
		case library.CausePace:
			return fmt.Sprintf("book pace %s from %.0f to %.0f pages/h", direction(then.PagesPerHour, now.PagesPerHour), then.PagesPerHour, now.PagesPerHour)
		}
		return fmt.Sprintf("the time left %s from %s to %s", direction(then.WeeksLeft, now.WeeksLeft), timeLeft(then.WeeksLeft), timeLeft(now.WeeksLeft))
	}
	v.Cause = fmt.Sprintf("Needed each week %s from %s to %s, mostly because %s",
		direction(then.WeeklyHours, now.WeeklyHours), minutesLabel(hours(then.WeeklyHours)), minutesLabel(hours(now.WeeklyHours)), moved(c.Causes[0]))
	if len(c.Causes) > 1 {
		v.Cause += ", and " + moved(c.Causes[1])
	}
	v.Cause += "."
	return v
}

// direction says which way a figure moved.
func direction(then, now float64) string {
	if now < then {
		return "fell"
	}
	return "rose"
}

// reportView is the composition report (spec §9.2).
type reportView struct {
	Weeks     string // "last 4 weeks"
	Window    string // "16 Aug – 12 Sep"
	Headline  string // "11 short items, 0 books."
	Formats   []tallyRow
	Sizes     []tallyRow
	Abandoned int
	Blocks    []blockRow
}

type tallyRow struct {
	Label     string
	Count     int
	Reference string // "1 reference", or ""
}

type blockRow struct {
	Dates, Books, Mean, Unsized string
}

func newReportView(c library.Composition, st library.Settings) reportView {
	v := reportView{Weeks: lastWeeks(st.ProjectionWindowWeeks), Window: weekDates(c.From, c.To)}
	total := 0
	for _, f := range library.Formats {
		t := c.ByFormat[f]
		total += t.Completed
		v.Formats = append(v.Formats, tallyRow{Label: string(f), Count: t.Completed, Reference: references(t)})
	}
	quick, hour := minutesLabel(minutes(st.BucketQuickMaxMin)), minutesLabel(minutes(st.BucketHourMaxMin))
	sizeLabels := map[library.SizeBucket]string{
		library.SizeShort:   "short, up to " + quick,
		library.SizeHour:    "an hour, up to " + hour,
		library.SizeLong:    "long, over " + hour,
		library.SizeUntimed: "no time logged",
	}
	for _, b := range library.SizeBuckets {
		t := c.BySize[b]
		v.Sizes = append(v.Sizes, tallyRow{Label: sizeLabels[b], Count: t.Completed, Reference: references(t)})
	}
	if total == 0 {
		v.Headline = "Nothing completed in these weeks."
	} else {
		v.Headline = fmt.Sprintf("%s, %s.",
			plural(c.BySize[library.SizeShort].Completed, "short item", "short items"),
			plural(c.ByFormat[library.FormatBook].Completed, "book", "books"))
	}
	v.Abandoned = c.Abandoned
	for _, b := range c.BookPages {
		row := blockRow{Dates: weekDates(b.From, b.To), Books: plural(b.Books, "book", "books"), Mean: "–"}
		if b.Sized > 0 {
			row.Mean = fmt.Sprintf("%.0f pages", math.Round(b.MeanPages))
		}
		if unsized := b.Books - b.Sized; unsized > 0 {
			row.Unsized = fmt.Sprintf("%d without a page count", unsized)
		}
		v.Blocks = append(v.Blocks, row)
	}
	return v
}

func references(t library.Tally) string {
	if t.Reference == 0 {
		return ""
	}
	return fmt.Sprintf("%d reference", t.Reference)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
