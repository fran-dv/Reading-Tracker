package library

import (
	"context"
	"math"
	"sort"
	"time"
)

// Review is a week whose review was closed (spec §2.8).
type Review struct {
	WeekOf   time.Time `json:"week_of"` // calendar day the week began
	ClosedAt time.Time `json:"closed_at"`
	Needs    *Needs    `json:"needs"` // nil when no campaign was active
}

// Needs is what an active campaign required when a review closed. Required
// hours cannot be replayed, so a review keeps their inputs, apart, for the
// next review to name what moved them (spec §8.1).
type Needs struct {
	CampaignID   string  `json:"campaign_id"`
	BooksLeft    int     `json:"books_left"`
	AvgPages     float64 `json:"avg_pages"`
	PagesPerHour float64 `json:"pages_per_hour"`
	WeeksLeft    float64 `json:"weeks_left"`
	WeeklyHours  float64 `json:"weekly_hours"`
}

// needs is what the campaign requires now, in the shape a review keeps.
func (cs CampaignState) needs() Needs {
	r := cs.Required
	return Needs{
		CampaignID:   cs.Campaign.ID,
		BooksLeft:    r.BooksLeft,
		AvgPages:     r.AvgPages,
		PagesPerHour: r.Pace.PagesPerHour,
		WeeksLeft:    cs.WeeksLeft,
		WeeklyHours:  r.WeeklyHours,
	}
}

// measurable reports whether every input is above zero, so a change can be
// split among them.
func (n Needs) measurable() bool {
	return n.BooksLeft > 0 && n.AvgPages > 0 && n.PagesPerHour > 0 && n.WeeksLeft > 0
}

// Cause is an input of required hours that moved them.
type Cause string

const (
	CauseBooks Cause = "books" // books finished
	CausePages Cause = "pages" // the average book size
	CausePace  Cause = "pace"  // book pace
	CauseWeeks Cause = "weeks" // weeks remaining
)

// notableChange is how far required hours move before a cause is named.
const notableChange = 0.10

// NeedsChange compares what a campaign needs now with what an earlier review
// showed of it.
type NeedsChange struct {
	Since     time.Time // when the earlier review closed
	Then, Now Needs
	// Causes is empty unless required hours changed by more than 10%. Then
	// it names the input with the largest share of the change, and a second
	// whose share is at least half as large.
	Causes []Cause
}

// Ratio is required hours now over then; 0 when nothing was required then.
func (c NeedsChange) Ratio() float64 {
	if c.Then.WeeklyHours <= 0 {
		return 0
	}
	return c.Now.WeeklyHours / c.Then.WeeklyHours
}

// CompareNeeds works out a change in required hours and its causes.
func CompareNeeds(since time.Time, then, now Needs) NeedsChange {
	c := NeedsChange{Since: since, Then: then, Now: now}
	if !then.measurable() || !now.measurable() || math.Abs(c.Ratio()-1) <= notableChange {
		return c
	}
	// Required = books × pages ÷ pace ÷ weeks, so the log of its ratio is
	// the sum of one term per input, and each term is that input's share.
	terms := []struct {
		cause Cause
		share float64
	}{
		{CauseBooks, math.Log(float64(now.BooksLeft) / float64(then.BooksLeft))},
		{CausePages, math.Log(now.AvgPages / then.AvgPages)},
		{CausePace, -math.Log(now.PagesPerHour / then.PagesPerHour)},
		{CauseWeeks, -math.Log(now.WeeksLeft / then.WeeksLeft)},
	}
	// Turn falls into rises, so the largest term is the one that pushed
	// hardest the way required hours went.
	if c.Ratio() < 1 {
		for i := range terms {
			terms[i].share = -terms[i].share
		}
	}
	sort.SliceStable(terms, func(i, j int) bool { return terms[i].share > terms[j].share })
	c.Causes = []Cause{terms[0].cause}
	if terms[1].share >= terms[0].share/2 {
		c.Causes = append(c.Causes, terms[1].cause)
	}
	return c
}

// ReviewDue reports whether the review is overdue on today: from the day
// after the review weekday until a review closes in that week, and always
// before the first review. reviews are ordered by WeekOf.
func ReviewDue(reviews []Review, today time.Time, weekday time.Weekday) bool {
	if len(reviews) == 0 {
		return true
	}
	weekOf := weekStartOf(today, weekday)
	return today.After(weekOf) && reviews[len(reviews)-1].WeekOf.Before(weekOf)
}

// SizeBucket is how much reading an item took, against Home's time bounds.
type SizeBucket string

const (
	SizeShort   SizeBucket = "short"   // up to settings.BucketQuickMaxMin
	SizeHour    SizeBucket = "hour"    // up to settings.BucketHourMaxMin
	SizeLong    SizeBucket = "long"    // above it
	SizeUntimed SizeBucket = "untimed" // no reading logged on it
)

// SizeBuckets lists every bucket in display order.
var SizeBuckets = []SizeBucket{SizeShort, SizeHour, SizeLong, SizeUntimed}

func sizeBucket(logged time.Duration, st Settings) SizeBucket {
	switch {
	case logged <= 0:
		return SizeUntimed
	case logged <= time.Duration(st.BucketQuickMaxMin)*time.Minute:
		return SizeShort
	case logged <= time.Duration(st.BucketHourMaxMin)*time.Minute:
		return SizeHour
	}
	return SizeLong
}

// Tally counts completed items. Reference is how many of them were closed
// as reference rather than finished.
type Tally struct {
	Completed int
	Reference int
}

func (t Tally) add(state State) Tally {
	t.Completed++
	if state == StateReference {
		t.Reference++
	}
	return t
}

// PagesBlock is the mean size of the books finished in a stretch of weeks.
type PagesBlock struct {
	From, To  time.Time // calendar days; the block ends before To
	Books     int       // books finished
	Sized     int       // those with a size in pages, which the mean is over
	MeanPages float64   // 0 when none is sized
}

// Composition is what was completed lately and how big it was (spec §9.2).
type Composition struct {
	From, To  time.Time // calendar days; the window ends before To
	ByFormat  map[Format]Tally
	BySize    map[SizeBucket]Tally
	Abandoned int
	// BookPages repeats the window back to the campaign's start, or to the
	// first finished book without a campaign. Newest first; the first block
	// is the window itself.
	BookPages []PagesBlock
}

// MeasureComposition reports on the last settings.ProjectionWindowWeeks
// closed weeks, at now in loc. campaign is nil when there has been none.
func MeasureComposition(items []Item, sessions []Session, campaign *Campaign, st Settings, loc *time.Location, now time.Time) Composition {
	span := 7 * st.ProjectionWindowWeeks
	to := weekStartOf(dayOf(now, loc), st.ReviewWeekday)
	comp := Composition{
		From:     to.AddDate(0, 0, -span),
		To:       to,
		ByFormat: map[Format]Tally{},
		BySize:   map[SizeBucket]Tally{},
	}
	logged := map[string]time.Duration{}
	for _, s := range sessions {
		logged[s.ItemID] += s.Duration()
	}
	// finishedDay is the day an item closed; ok is false for open items.
	finishedDay := func(it Item) (time.Time, bool) {
		if it.FinishedAt == nil {
			return time.Time{}, false
		}
		return dayOf(*it.FinishedAt, loc), true
	}

	for _, it := range items {
		d, ok := finishedDay(it)
		if !ok || d.Before(comp.From) || !d.Before(to) {
			continue
		}
		switch it.State {
		case StateAbandoned:
			comp.Abandoned++
		case StateFinished, StateReference:
			comp.ByFormat[it.Format] = comp.ByFormat[it.Format].add(it.State)
			bucket := sizeBucket(logged[it.ID], st)
			comp.BySize[bucket] = comp.BySize[bucket].add(it.State)
		}
	}

	var books []Item
	var origin time.Time
	for _, it := range items {
		if d, ok := finishedDay(it); ok && it.Format == FormatBook && it.State == StateFinished {
			books = append(books, it)
			if origin.IsZero() || d.Before(origin) {
				origin = d
			}
		}
	}
	if campaign != nil {
		origin = campaign.StartedOn
	}
	if origin.IsZero() {
		return comp
	}
	for end := to; end.After(origin); end = end.AddDate(0, 0, -span) {
		block := PagesBlock{From: end.AddDate(0, 0, -span), To: end}
		if block.From.Before(origin) {
			block.From = origin
		}
		pages := 0
		for _, it := range books {
			if d, _ := finishedDay(it); d.Before(block.From) || !d.Before(end) {
				continue
			}
			block.Books++
			if it.SizeUnit == UnitPages && it.SizeValue != nil && *it.SizeValue > 0 {
				block.Sized++
				pages += *it.SizeValue
			}
		}
		if block.Sized > 0 {
			block.MeanPages = float64(pages) / float64(block.Sized)
		}
		comp.BookPages = append(comp.BookPages, block)
	}
	return comp
}

// ShortlistEntry is an item the shortlist can take, with its home shelf.
type ShortlistEntry struct {
	Item      Item
	ShelfName string
}

// ReviewView is the weekly review (spec §6.3).
type ReviewView struct {
	WeekOf  time.Time // calendar day the week began
	Closed  *Review   // this week's review once closed
	Reading []Reading // in progress, most recently read first
	Shelves []ShelfView
	// Shortlist is what the shortlist step offers: in-progress items, shelf
	// leaders, then anything else already on it. Pool is every other pool
	// item, by shelf, for reaching further.
	Shortlist []ShortlistEntry
	Pool      []ShortlistEntry

	Schedule    Schedule
	Speed       Speed
	Campaign    *CampaignState // the active campaign, or the one ended last
	Change      *NeedsChange   // nil without an earlier review of the active campaign
	Composition Composition

	// Reached is every achievement since the last review closed (or over
	// the last week, before any), newest first. LastWeek is what the week
	// just closed gave each item (spec §6.3).
	Reached     []Achievement
	ReachedFrom time.Time // the first day Reached covers
	LastWeek    []ItemTime
}

// Shortlisted is how many items are on the shortlist.
func (v ReviewView) Shortlisted() int {
	n := 0
	for _, e := range v.Shortlist {
		if e.Item.OnShortlist {
			n++
		}
	}
	return n
}

// Review gathers the weekly review as it stands now.
func (s *Service) Review(ctx context.Context) (*ReviewView, error) {
	var view *ReviewView
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		v := &ReviewView{WeekOf: weekStartOf(dayOf(sn.now, loc), sn.settings.ReviewWeekday)}
		for i := range sn.reviews {
			if sn.reviews[i].WeekOf.Equal(v.WeekOf) {
				v.Closed = &sn.reviews[i]
			}
		}

		v.Reading = sn.reading(Moment{})
		shelves, err := r.ListShelves()
		if err != nil {
			return err
		}
		for _, sh := range shelves {
			sv, err := shelfView(r, sh.ID)
			if err != nil {
				return err
			}
			v.Shelves = append(v.Shelves, *sv)
		}
		v.Shortlist, v.Pool = shortlistEntries(sn.items, v.Reading, v.Shelves)

		if v.Schedule, err = sn.schedule(); err != nil {
			return err
		}
		if v.Speed, err = sn.speed(); err != nil {
			return err
		}
		if v.Campaign, err = sn.campaign(); err != nil {
			return err
		}
		if cs := v.Campaign; cs != nil && !cs.Over {
			if earlier := lastNeeds(sn.reviews, cs.Campaign.ID, v.WeekOf); earlier != nil {
				change := CompareNeeds(earlier.ClosedAt, *earlier.Needs, cs.needs())
				v.Change = &change
			}
		}
		v.Composition = MeasureComposition(sn.items, sn.sessions, currentCampaign(sn.campaigns), *sn.settings, loc, sn.now)

		v.ReachedFrom = v.WeekOf.AddDate(0, 0, -7)
		for _, rv := range sn.reviews {
			if rv.WeekOf.Before(v.WeekOf) {
				v.ReachedFrom = dayOf(rv.ClosedAt, loc)
			}
		}
		all, err := sn.achievements()
		if err != nil {
			return err
		}
		for _, a := range all {
			if !a.On.Before(v.ReachedFrom) {
				v.Reached = append(v.Reached, a)
			}
		}
		// The week is clamped to the first one with anything in it, which
		// may be this one: then last week gave nothing.
		if last := sn.week(v.Schedule, v.WeekOf.AddDate(0, 0, -7), loc); last.WeekStart.Before(v.WeekOf) {
			v.LastWeek = last.Items
		}
		view = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// lastNeeds is the latest review before the week of weekOf that kept the
// campaign's needs, or nil. reviews are ordered by WeekOf.
func lastNeeds(reviews []Review, campaignID string, weekOf time.Time) *Review {
	for i := len(reviews) - 1; i >= 0; i-- {
		rv := &reviews[i]
		if rv.WeekOf.Before(weekOf) && rv.Needs != nil && rv.Needs.CampaignID == campaignID {
			return rv
		}
	}
	return nil
}

// shortlistEntries splits the items the shortlist can take into what the
// review offers first and the rest of the pools. Each item appears once,
// however many shelves rank it.
func shortlistEntries(items []Item, reading []Reading, shelves []ShelfView) (offered, pool []ShortlistEntry) {
	order := map[string]int{}
	name := map[string]string{}
	for i, sv := range shelves {
		order[sv.Shelf.ID], name[sv.Shelf.ID] = i, sv.Shelf.Name
	}
	seen := map[string]bool{}
	offer := func(it Item) {
		if !seen[it.ID] {
			seen[it.ID] = true
			offered = append(offered, ShortlistEntry{Item: it, ShelfName: name[it.ShelfID]})
		}
	}
	for _, rd := range reading {
		offer(rd.Item)
	}
	for _, sv := range shelves {
		for _, slot := range sv.Slots {
			if slot != nil {
				offer(slot.Item)
			}
		}
	}

	var rest []Item
	for _, it := range items {
		if it.State == StatePool && !seen[it.ID] {
			rest = append(rest, it)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool {
		a, b := rest[i], rest[j]
		if order[a.ShelfID] != order[b.ShelfID] {
			return order[a.ShelfID] < order[b.ShelfID]
		}
		return a.Title < b.Title
	})
	for _, it := range rest {
		if it.OnShortlist {
			offer(it)
		}
	}
	for _, it := range rest {
		if !seen[it.ID] {
			pool = append(pool, ShortlistEntry{Item: it, ShelfName: name[it.ShelfID]})
		}
	}
	return offered, pool
}

// CloseReview records this week's review, with what the active campaign
// needs today. Closing again in the same week replaces it.
func (s *Service) CloseReview(ctx context.Context) (*Review, error) {
	var rv *Review
	err := s.store.Tx(ctx, func(r Repo) error {
		sn, err := s.load(r)
		if err != nil {
			return err
		}
		loc, err := sn.settings.Location()
		if err != nil {
			return err
		}
		rv = &Review{WeekOf: weekStartOf(dayOf(sn.now, loc), sn.settings.ReviewWeekday), ClosedAt: sn.now}
		cs, err := sn.campaign()
		if err != nil {
			return err
		}
		if cs != nil && !cs.Over {
			needs := cs.needs()
			rv.Needs = &needs
		}
		return r.PutReview(rv)
	})
	if err != nil {
		return nil, err
	}
	return rv, nil
}
