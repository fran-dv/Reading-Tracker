package library

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// maxCampaignTarget bounds a campaign's target to something a person reads.
const maxCampaignTarget = 10000

// CampaignKind is what a campaign asks for (spec §2.4).
type CampaignKind string

const (
	KindCount CampaignKind = "count" // any book, by a deadline: an open target
	KindSet   CampaignKind = "set"   // these items, by a deadline
)

// Campaign is a goal by a deadline (spec §2.4): a number of books, or a
// named set of items. Any number can be active at once. Only its name can
// change after it starts, and a set can gain items.
type Campaign struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        CampaignKind `json:"kind"`
	TargetCount int          `json:"target_count"` // a count campaign's; 0 for a set
	StartedOn   time.Time    `json:"started_on"`   // a calendar day, see dayOf
	Deadline    time.Time    `json:"deadline"`     // a calendar day, inclusive
	EndedOn     *time.Time   `json:"ended_on"`     // a calendar day; nil while active
}

// CampaignItem is an item in a set campaign, dated by the day it was added
// so a raised bar leaves a trace.
type CampaignItem struct {
	CampaignID string    `json:"campaign_id"`
	ItemID     string    `json:"item_id"`
	AddedOn    time.Time `json:"added_on"` // a calendar day
}

// Active reports whether the campaign has not been ended.
func (c Campaign) Active() bool { return c.EndedOn == nil }

// campaignName is the name a count campaign gets when none is typed.
func campaignName(target int, deadline time.Time) string {
	return fmt.Sprintf("%d books by %s", target, deadline.Format("2 Jan 2006"))
}

// setName is the name a set gets when none is typed: its one item's title,
// or how many items it holds.
func setName(titles []string, deadline time.Time) string {
	by := " by " + deadline.Format("2 Jan 2006")
	if len(titles) == 1 {
		return titles[0] + by
	}
	return fmt.Sprintf("%d items%s", len(titles), by)
}

// PagesBasis says where the average book size came from (spec §8.1).
type PagesBasis string

const (
	PagesWaiting  PagesBasis = "waiting"  // books in the pool or in progress
	PagesFinished PagesBasis = "finished" // books already finished
	PagesSetting  PagesBasis = "setting"  // settings.fallback_book_pages
)

// BookPace is how fast books go, pooled over every focus demand in the pace
// window, with the material it was measured on (spec §8.1, §9.1).
type BookPace struct {
	PagesPerHour float64
	Measured     time.Duration // positioned book reading behind it
	Mix          []Mix         // by focus demand; empty when provisional
	Provisional  bool          // the medium seed: too little measured
}

// Required is what the campaign needs from today (spec §8.1). Its inputs
// are kept apart so a change can be traced to its cause. A set uses only
// the hours: the time left on its unfinished items.
type Required struct {
	BooksLeft   int
	AvgPages    float64
	PagesBasis  PagesBasis
	Pace        BookPace
	HoursLeft   float64 // book hours, or a set's hours
	WeeklyHours float64 // hours a week until the deadline
}

// SetItem is one item of a set campaign, where its reading stands.
type SetItem struct {
	Item       Item
	AddedOn    time.Time
	Position   int
	Estimate   Estimate // time left; unknown without a size
	Done       bool     // finished by the deadline
	Lost       bool     // abandoned: the set can no longer be met
	FinishedOn time.Time
}

// SetProgress is what a set campaign actually gets (spec §8.5): the hours
// its own items have taken since it began, and where that lands them.
type SetProgress struct {
	Weeks   float64       // elapsed, counting a first short week as a whole one
	Hours   float64       // on its items, a week
	Share   float64       // of all reading in that span, 0–1
	Read    time.Duration // all reading in that span
	DoneOn  time.Time     // the day the hours left run out; zero past the deadline
	Covered float64       // of the hours left, what the deadline allows, 0–1
}

// Projection is where recent reading lands by the deadline (spec §8.5).
type Projection struct {
	Weeks           int           // closed weeks averaged
	WeeklyBookHours float64       // mean over those weeks
	BookShare       float64       // share of all their reading that went to books, 0–1
	Read            time.Duration // all their reading; with none, BookShare says nothing
	Books           int           // books finished by the deadline at that rate
}

// share is how much of the reading the campaign can claim, for projecting a
// target: the share that goes to books, or, for a set, to its own items.
// assumed is true when no recent reading gives one, and all of it is counted.
func (cs CampaignState) share() (share float64, assumed bool) {
	if cs.Campaign.Kind == KindSet {
		if s := cs.Set; s != nil && s.Read > 0 {
			return s.Share, false
		}
		return 1, true
	}
	if p := cs.Projection; p != nil && p.Read > 0 {
		return p.BookShare, false
	}
	return 1, true
}

// CampaignState is where a campaign stands now.
type CampaignState struct {
	Campaign  Campaign
	Target    int     // books for a count campaign, items for a set
	Finished  int     // books counted, or items finished, so far
	Over      bool    // ended, or its deadline has passed: the count is final
	Lost      bool    // a set item was abandoned: it can no longer be met
	WeeksLeft float64 // to the end of the deadline day; 0 when over

	// Items and Set belong to a set campaign. Set is nil until something
	// has been read on its items.
	Items []SetItem
	Set   *SetProgress

	// Required and Projection are meaningful only when the campaign is not
	// over. Projection is nil until a week of reading has closed.
	Required   Required
	Projection *Projection
	// Plan is where the plan in effect lands if it holds; nil without a
	// plan, or when the campaign is over or its target reached.
	Plan *PlanProjection
}

// Reached reports whether the target has been met.
func (cs CampaignState) Reached() bool { return cs.Target > 0 && cs.Finished >= cs.Target }

// projectFrom adds to the count the books weeklyBookHours reach by the deadline.
func (cs CampaignState) projectFrom(weeklyBookHours float64) int {
	r := cs.Required
	pages := cs.WeeksLeft * weeklyBookHours * r.Pace.PagesPerHour
	// A hair of tolerance keeps 46.0 computed as 45.999… from losing a book.
	return cs.Finished + int(math.Floor(pages/r.AvgPages+1e-9))
}

// MatchPerDay is the daily target that meets the required book hours over
// the given active days at the recent share of reading on books (all of it
// without a recent share), rounded up to the minute; share is the one used. ok is false when there
// is nothing to match (no days, over, reached) or it would not fit in a day.
func (cs CampaignState) MatchPerDay(days Weekdays) (minutes int, share float64, ok bool) {
	n := days.Count()
	if n == 0 || cs.Over || cs.Reached() {
		return 0, 0, false
	}
	share, _ = cs.share()
	if share == 0 {
		return 0, 0, false // no reading goes to books lately: no target meets it
	}
	minutes = int(math.Ceil(cs.Required.WeeklyHours*60/share/float64(n) - 1e-9))
	return minutes, share, minutes >= 1 && minutes <= maxMinutesPerDay
}

// MeasureCampaign works out where a campaign stands, what it requires and
// where recent reading projects it, at now in loc. items is keyed by ID;
// members are the set campaign's items, empty for a count campaign.
func MeasureCampaign(c Campaign, members []CampaignItem, items map[string]Item, sessions []Session, st Settings, loc *time.Location, now time.Time) CampaignState {
	today := dayOf(now, loc)
	cs := CampaignState{Campaign: c, Target: c.TargetCount}
	if c.Kind == KindSet {
		since := now.Add(-time.Duration(st.PaceWindowDays) * 24 * time.Hour)
		all := make([]Item, 0, len(items))
		for _, it := range items {
			all = append(all, it)
		}
		cs.setItems(members, items, BandPaces(all, sessions, since), sessions, st, loc)
	} else {
		for _, it := range items {
			if it.Format == FormatBook && it.State == StateFinished && it.FinishedAt != nil && counts(c, dayOf(*it.FinishedAt, loc)) {
				cs.Finished++
			}
		}
	}

	end := dayStart(c.Deadline.AddDate(0, 0, 1), loc)
	if !c.Active() || !now.Before(end) {
		cs.Over = true
		return cs
	}
	cs.WeeksLeft = end.Sub(now).Hours() / (7 * 24)

	r := &cs.Required
	if c.Kind == KindSet {
		for _, si := range cs.Items {
			if !si.Done && !si.Lost {
				r.HoursLeft += si.Estimate.Remaining.Hours()
			}
		}
		r.WeeklyHours = r.HoursLeft / cs.WeeksLeft
		cs.Set = setProgress(cs, items, sessions, loc, today, now)
		return cs
	}

	r.BooksLeft = max(0, c.TargetCount-cs.Finished)
	r.AvgPages, r.PagesBasis = avgBookPages(items, st)
	r.Pace = bookPace(items, sessions, st, now)
	r.HoursLeft = float64(r.BooksLeft) * r.AvgPages / r.Pace.PagesPerHour
	r.WeeklyHours = r.HoursLeft / cs.WeeksLeft

	cs.Projection = recentReading(items, sessions, st, loc, today, now)
	if cs.Projection != nil {
		cs.Projection.Books = cs.projectFrom(cs.Projection.WeeklyBookHours)
	}
	return cs
}

// setItems reads where each item of a set stands, in the order they were
// added, and counts the ones finished by the deadline.
func (cs *CampaignState) setItems(members []CampaignItem, items map[string]Item, bands Paces, sessions []Session, st Settings, loc *time.Location) {
	history := sessionsByItem(sessions)
	for _, m := range members {
		it, ok := items[m.ItemID]
		if !ok {
			continue // the item was deleted, which only an item with no history can be
		}
		si := SetItem{Item: it, AddedOn: m.AddedOn, Position: furthestPosition(history[it.ID])}
		switch it.State {
		case StateFinished:
			si.FinishedOn = dayOf(*it.FinishedAt, loc)
			si.Done = !si.FinishedOn.After(cs.Campaign.Deadline)
		case StateAbandoned:
			si.Lost = true
		}
		if !si.Done && !si.Lost {
			si.Estimate = TimeRemaining(it, history[it.ID], bands, st)
		}
		if si.Done {
			cs.Finished++
		}
		cs.Lost = cs.Lost || si.Lost
		cs.Items = append(cs.Items, si)
	}
	cs.Target = len(cs.Items)
}

// setProgress measures the hours a set's own items have had since it began,
// and where they land its remaining hours. It is nil until one is read.
func setProgress(cs CampaignState, items map[string]Item, sessions []Session, loc *time.Location, today, now time.Time) *SetProgress {
	mine := map[string]bool{}
	for _, si := range cs.Items {
		mine[si.Item.ID] = true
	}
	from := dayStart(cs.Campaign.StartedOn, loc)
	var own []Session
	for _, s := range sessions {
		if mine[s.ItemID] {
			own = append(own, s)
		}
	}
	p := &SetProgress{
		Weeks: math.Max(1, now.Sub(from).Hours()/(7*24)), // a first short week counts whole
		Read:  loggedBetween(sessions, from, now, now),
	}
	hours := loggedBetween(own, from, now, now).Hours()
	if hours == 0 {
		return nil
	}
	p.Hours = hours / p.Weeks
	if p.Read > 0 {
		p.Share = hours / p.Read.Hours()
	}
	left := cs.Required.HoursLeft
	if left <= 0 {
		return p
	}
	p.Covered = math.Min(1, p.Hours*cs.WeeksLeft/left)
	if p.Covered >= 1 {
		p.DoneOn = today.AddDate(0, 0, int(math.Ceil(left/p.Hours*7)))
	}
	return p
}

// avgBookPages is the mean size of the books still to read, falling back to
// the books finished, then to the setting.
func avgBookPages(items map[string]Item, st Settings) (float64, PagesBasis) {
	var waiting, finished []int
	for _, it := range items {
		if it.Format != FormatBook || it.SizeUnit != UnitPages || it.SizeValue == nil || *it.SizeValue <= 0 {
			continue
		}
		switch it.State {
		case StatePool, StateInProgress:
			waiting = append(waiting, *it.SizeValue)
		case StateFinished:
			finished = append(finished, *it.SizeValue)
		}
	}
	if len(waiting) > 0 {
		return mean(waiting), PagesWaiting
	}
	if len(finished) > 0 {
		return mean(finished), PagesFinished
	}
	return float64(st.FallbackBookPages), PagesSetting
}

func mean(xs []int) float64 {
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}

// bookPace pools every book band in pages over the pace window. Below
// MinSpeedEvidence it is provisional: the seeds for the focus of the books
// waiting, weighted by their pages (spec §8.1).
func bookPace(items map[string]Item, sessions []Session, st Settings, now time.Time) BookPace {
	since := now.Add(-time.Duration(st.PaceWindowDays) * 24 * time.Hour)
	books := map[Band]BandSpeed{}
	for band, b := range bandSpeeds(items, sessions, since, now) {
		if band.Format == FormatBook && band.SizeUnit == UnitPages {
			books[band] = b
		}
	}
	// The same measure as a week's speed, over the pace window instead.
	w := weekSpeed(books, since, now, st.WordsPerPage)
	if w.Measured < MinSpeedEvidence {
		return BookPace{PagesPerHour: seedBookPace(items, st), Provisional: true}
	}
	return BookPace{PagesPerHour: w.PagesPerHour, Measured: w.Measured, Mix: w.Mix}
}

// seedBookPace is the pace the seeds give the books waiting, weighted by
// their pages: every page read at its focus's seed takes total pages ÷ this
// many hours. Without sized books waiting it is the medium seed.
func seedBookPace(items map[string]Item, st Settings) float64 {
	var pages, hours float64
	for _, it := range items {
		if it.Format != FormatBook || it.SizeUnit != UnitPages || it.SizeValue == nil || *it.SizeValue <= 0 {
			continue
		}
		if it.State != StatePool && it.State != StateInProgress {
			continue
		}
		seed, _ := seedPace(it, st)
		pages += float64(*it.SizeValue)
		hours += float64(*it.SizeValue) / seed
	}
	if hours == 0 {
		return float64(st.SeedPaceMedium)
	}
	return pages / hours
}

// recentReading averages the last closed weeks of reading, leaving out
// weeks that ended before the first session was ever logged. It is nil
// when no such week exists.
func recentReading(items map[string]Item, sessions []Session, st Settings, loc *time.Location, today, now time.Time) *Projection {
	if len(sessions) == 0 {
		return nil
	}
	first := sessions[0].StartedAt
	var books []Session
	for _, s := range sessions {
		if s.StartedAt.Before(first) {
			first = s.StartedAt
		}
		if items[s.ItemID].Format == FormatBook {
			books = append(books, s)
		}
	}

	p := &Projection{}
	var bookTime, allTime time.Duration
	weekStart := weekStartOf(today, st.ReviewWeekday)
	for i := 1; i <= st.ProjectionWindowWeeks; i++ {
		from, to := dayStart(weekStart.AddDate(0, 0, -7*i), loc), dayStart(weekStart.AddDate(0, 0, -7*(i-1)), loc)
		if !to.After(first) {
			break
		}
		p.Weeks++
		bookTime += loggedBetween(books, from, to, now)
		allTime += loggedBetween(sessions, from, to, now)
	}
	if p.Weeks == 0 {
		return nil
	}
	p.WeeklyBookHours = bookTime.Hours() / float64(p.Weeks)
	p.Read = allTime
	if allTime > 0 {
		p.BookShare = float64(bookTime) / float64(allTime)
	}
	return p
}

// currentCampaigns is every active campaign, oldest first, or else the one
// ended last on its own; empty before the first. campaigns are ordered
// oldest first.
func currentCampaigns(campaigns []Campaign) []Campaign {
	var active []Campaign
	for _, c := range campaigns {
		if c.Active() {
			active = append(active, c)
		}
	}
	if len(active) == 0 && len(campaigns) > 0 {
		return campaigns[len(campaigns)-1:]
	}
	return active
}

// CampaignView is one campaign's page (spec §6.7): where it stands, the
// schedule its needs are weighed against, and the items a set can still be
// given.
type CampaignView struct {
	State    CampaignState
	Schedule Schedule
	Pickable []ShortlistEntry
}

// Campaign measures one campaign, active or ended.
func (s *Service) Campaign(ctx context.Context, id string) (*CampaignView, error) {
	var view *CampaignView
	err := s.store.Tx(ctx, func(r Repo) error {
		c, err := r.GetCampaign(id)
		if err != nil {
			return err
		}
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		view = &CampaignView{}
		if view.State, err = sn.measure(*c); err != nil {
			return err
		}
		if view.Schedule, err = sn.schedule(); err != nil {
			return err
		}
		if c.Kind == KindSet && c.Active() {
			in := map[string]bool{}
			for _, si := range view.State.Items {
				in[si.Item.ID] = true
			}
			if view.Pickable, err = pickableItems(r, sn.items, in); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// pickableItems is what a set campaign can be given: every sized item that
// can still be read and is not already in it, by shelf order then title.
func pickableItems(r Repo, items []Item, taken map[string]bool) ([]ShortlistEntry, error) {
	shelves, err := r.ListShelves()
	if err != nil {
		return nil, err
	}
	order, name := map[string]int{}, map[string]string{}
	for i, sh := range shelves {
		order[sh.ID], name[sh.ID] = i, sh.Name
	}
	var out []ShortlistEntry
	for _, it := range items {
		if taken[it.ID] || it.SizeValue == nil || *it.SizeValue <= 0 {
			continue
		}
		if it.State != StatePool && it.State != StateInProgress {
			continue
		}
		out = append(out, ShortlistEntry{Item: it, ShelfName: name[it.ShelfID]})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Item, out[j].Item
		if order[a.ShelfID] != order[b.ShelfID] {
			return order[a.ShelfID] < order[b.ShelfID]
		}
		return a.Title < b.Title
	})
	return out, nil
}

// countCampaigns keeps the campaigns that count any book.
func countCampaigns(campaigns []Campaign) []Campaign {
	var out []Campaign
	for _, c := range campaigns {
		if c.Kind == KindCount {
			out = append(out, c)
		}
	}
	return out
}

// StartCampaign starts a campaign. startedOn and deadline are calendar days
// (midnight UTC). A blank name is generated from the target and deadline.
func (s *Service) StartCampaign(ctx context.Context, name string, target int, startedOn, deadline time.Time) (*Campaign, error) {
	if target < 1 || target > maxCampaignTarget {
		return nil, &ValidationError{"target_count", "must be between 1 and 10000"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = campaignName(target, deadline)
	}
	c := &Campaign{ID: newID(), Name: name, Kind: KindCount, TargetCount: target, StartedOn: startedOn, Deadline: deadline}
	err := s.store.Tx(ctx, func(r Repo) error {
		if err := checkCampaignDays(r, s.now(), startedOn, deadline); err != nil {
			return err
		}
		return r.InsertCampaign(c)
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// StartSetCampaign starts a campaign to finish the given items by the
// deadline. The items are put on the shortlist, once, so what the campaign
// asks for is among the picks. A blank name is generated from them.
func (s *Service) StartSetCampaign(ctx context.Context, name string, itemIDs []string, startedOn, deadline time.Time) (*Campaign, error) {
	if len(itemIDs) == 0 {
		return nil, &ValidationError{"items", "choose at least one item"}
	}
	c := &Campaign{ID: newID(), Name: strings.TrimSpace(name), Kind: KindSet, StartedOn: startedOn, Deadline: deadline}
	err := s.store.Tx(ctx, func(r Repo) error {
		if err := checkCampaignDays(r, s.now(), startedOn, deadline); err != nil {
			return err
		}
		items, err := checkSetItems(r, itemIDs)
		if err != nil {
			return err
		}
		if c.Name == "" {
			c.Name = setName(titlesOf(items), deadline)
		}
		if err := r.InsertCampaign(c); err != nil {
			return err
		}
		return writeSetItems(r, c.ID, items, startedOn, s.now())
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// AddCampaignItems adds items to an active set, dated today, and puts them
// on the shortlist. A set can only grow, and never after it is met.
func (s *Service) AddCampaignItems(ctx context.Context, id string, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return &ValidationError{"items", "choose at least one item"}
	}
	return s.store.Tx(ctx, func(r Repo) error {
		c, err := r.GetCampaign(id)
		if err != nil {
			return err
		}
		if c.Kind != KindSet || !c.Active() {
			return ErrInvalidTransition
		}
		met, err := setMet(r, *c)
		if err != nil {
			return err
		}
		if met {
			return &ValidationError{"items", "this set is met; start another campaign"}
		}
		st, err := r.GetSettings()
		if err != nil {
			return err
		}
		loc, err := st.Location()
		if err != nil {
			return err
		}
		items, err := checkSetItems(r, itemIDs)
		if err != nil {
			return err
		}
		return writeSetItems(r, id, items, dayOf(s.now(), loc), s.now())
	})
}

// checkSetItems reads the items a set is being given. Each must be one that
// can still be read, and sized, or there is nothing to measure.
func checkSetItems(r Repo, itemIDs []string) ([]*Item, error) {
	var items []*Item
	for _, itemID := range itemIDs {
		it, err := r.GetItem(itemID)
		if err != nil {
			return nil, err
		}
		if it.State != StatePool && it.State != StateInProgress {
			return nil, &ValidationError{"items", it.Title + " is not in the pool or being read"}
		}
		if it.SizeValue == nil || *it.SizeValue <= 0 {
			return nil, &ValidationError{"items", it.Title + " has no size, so there is no time to measure"}
		}
		items = append(items, it)
	}
	return items, nil
}

// writeSetItems puts each item into the set, dated addedOn, and on the
// shortlist, so what the campaign asks for is among the picks.
func writeSetItems(r Repo, campaignID string, items []*Item, addedOn, now time.Time) error {
	for _, it := range items {
		if err := r.InsertCampaignItem(&CampaignItem{CampaignID: campaignID, ItemID: it.ID, AddedOn: addedOn}); err != nil {
			return err
		}
		if !it.OnShortlist {
			it.OnShortlist, it.UpdatedAt = true, now
			if err := r.UpdateItem(it); err != nil {
				return err
			}
		}
	}
	return nil
}

// setMet reports whether every item of a set is finished by its deadline,
// which closes it to new items: a met campaign is done.
func setMet(r Repo, c Campaign) (bool, error) {
	members, err := r.ListCampaignItems()
	if err != nil {
		return false, err
	}
	found := false
	for _, m := range members {
		if m.CampaignID != c.ID {
			continue
		}
		found = true
		it, err := r.GetItem(m.ItemID)
		if err != nil {
			return false, err
		}
		if it.State != StateFinished || it.FinishedAt == nil || it.FinishedAt.After(c.Deadline.AddDate(0, 0, 1)) {
			return false, nil
		}
	}
	return found, nil
}

// titlesOf names items in order.
func titlesOf(items []*Item) []string {
	var titles []string
	for _, it := range items {
		titles = append(titles, it.Title)
	}
	return titles
}

// RenameCampaign changes a campaign's name, the one thing that can change.
// A blank name is generated again.
func (s *Service) RenameCampaign(ctx context.Context, id, name string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		c, err := r.GetCampaign(id)
		if err != nil {
			return err
		}
		c.Name = strings.TrimSpace(name)
		if c.Name == "" {
			c.Name = campaignName(c.TargetCount, c.Deadline)
			if c.Kind == KindSet {
				titles, err := setTitles(r, c.ID)
				if err != nil {
					return err
				}
				c.Name = setName(titles, c.Deadline)
			}
		}
		return r.UpdateCampaign(c)
	})
}

// checkCampaignDays refuses a start after today or a deadline that is not
// after both today and the start.
func checkCampaignDays(r Repo, now, startedOn, deadline time.Time) error {
	st, err := r.GetSettings()
	if err != nil {
		return err
	}
	loc, err := st.Location()
	if err != nil {
		return err
	}
	today := dayOf(now, loc)
	if startedOn.After(today) {
		return &ValidationError{"started_on", "must be today or earlier"}
	}
	if !deadline.After(today) || !deadline.After(startedOn) {
		return &ValidationError{"deadline", "must be after today and after the start"}
	}
	return nil
}

// setTitles is a set's item titles, in the order they were added.
func setTitles(r Repo, campaignID string) ([]string, error) {
	members, err := r.ListCampaignItems()
	if err != nil {
		return nil, err
	}
	var titles []string
	for _, m := range members {
		if m.CampaignID != campaignID {
			continue
		}
		it, err := r.GetItem(m.ItemID)
		if err != nil {
			return nil, err
		}
		titles = append(titles, it.Title)
	}
	return titles, nil
}

// EndCampaign ends a campaign today. An ended campaign stays as it was, so
// a second tap from a stale page is harmless.
func (s *Service) EndCampaign(ctx context.Context, id string) error {
	return s.store.Tx(ctx, func(r Repo) error {
		c, err := r.GetCampaign(id)
		if err != nil {
			return err
		}
		if !c.Active() {
			return nil
		}
		st, err := r.GetSettings()
		if err != nil {
			return err
		}
		loc, err := st.Location()
		if err != nil {
			return err
		}
		today := dayOf(s.now(), loc)
		c.EndedOn = &today
		return r.UpdateCampaign(c)
	})
}
