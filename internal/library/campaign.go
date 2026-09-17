package library

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// ErrCampaignActive is returned by StartCampaign while another campaign is
// still active: end it first.
var ErrCampaignActive = errors.New("a campaign is already active")

// maxCampaignTarget bounds a campaign's target to something a person reads.
const maxCampaignTarget = 10000

// Campaign is a goal of a number of books by a deadline (spec §2.4). Only
// its name can change after it starts.
type Campaign struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TargetCount int        `json:"target_count"`
	StartedOn   time.Time  `json:"started_on"` // a calendar day, see dayOf
	Deadline    time.Time  `json:"deadline"`   // a calendar day, inclusive
	EndedOn     *time.Time `json:"ended_on"`   // a calendar day; nil while active
}

// Active reports whether the campaign has not been ended.
func (c Campaign) Active() bool { return c.EndedOn == nil }

// campaignName is the name a campaign gets when none is typed.
func campaignName(target int, deadline time.Time) string {
	return fmt.Sprintf("%d books by %s", target, deadline.Format("2 Jan 2006"))
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
// are kept apart so a change can be traced to its cause.
type Required struct {
	BooksLeft   int
	AvgPages    float64
	PagesBasis  PagesBasis
	Pace        BookPace
	HoursLeft   float64 // book hours
	WeeklyHours float64 // book hours a week until the deadline
}

// Projection is where recent reading lands by the deadline (spec §8.5).
type Projection struct {
	Weeks           int     // closed weeks averaged
	WeeklyBookHours float64 // mean over those weeks
	BookShare       float64 // share of all their reading that went to books, 0–1
	Books           int     // books finished by the deadline at that rate
}

// CampaignState is where a campaign stands now.
type CampaignState struct {
	Campaign  Campaign
	Finished  int     // books counted so far
	Over      bool    // ended, or its deadline has passed: the count is final
	WeeksLeft float64 // to the end of the deadline day; 0 when over

	// Required and Projection are meaningful only when the campaign is not
	// over. Projection is nil until a week of reading has closed.
	Required   Required
	Projection *Projection
}

// Reached reports whether the target has been met.
func (cs CampaignState) Reached() bool { return cs.Finished >= cs.Campaign.TargetCount }

// ProjectAt is the books finished by the deadline if weeklyMinutes were read
// every week, at the recent share of them going to books. assumed is true
// when there is no recent reading to take a share from, and all of it is
// counted as books.
func (cs CampaignState) ProjectAt(weeklyMinutes int) (books int, assumed bool) {
	share := 1.0
	if cs.Projection != nil {
		share = cs.Projection.BookShare
	} else {
		assumed = true
	}
	return cs.projectFrom(float64(weeklyMinutes) / 60 * share), assumed
}

// projectFrom adds to the count the books weeklyBookHours reach by the deadline.
func (cs CampaignState) projectFrom(weeklyBookHours float64) int {
	r := cs.Required
	pages := cs.WeeksLeft * weeklyBookHours * r.Pace.PagesPerHour
	// A hair of tolerance keeps 46.0 computed as 45.999… from losing a book.
	return cs.Finished + int(math.Floor(pages/r.AvgPages+1e-9))
}

// MatchPerDay is the daily target that meets the required book hours over
// the given active days, rounded up to the minute. ok is false when there
// is nothing to match (no days, over, reached) or it would not fit in a day.
func (cs CampaignState) MatchPerDay(days Weekdays) (minutes int, ok bool) {
	n := days.Count()
	if n == 0 || cs.Over || cs.Reached() {
		return 0, false
	}
	minutes = int(math.Ceil(cs.Required.WeeklyHours*60/float64(n) - 1e-9))
	return minutes, minutes >= 1 && minutes <= maxMinutesPerDay
}

// MeasureCampaign works out a campaign's count, what it requires and where
// recent reading projects it, at now in loc. items is keyed by ID.
func MeasureCampaign(c Campaign, items map[string]Item, sessions []Session, st Settings, loc *time.Location, now time.Time) CampaignState {
	today := dayOf(now, loc)
	cs := CampaignState{Campaign: c}

	last := c.Deadline // the last day a finished book counts
	if c.EndedOn != nil && c.EndedOn.Before(last) {
		last = *c.EndedOn
	}
	for _, it := range items {
		if it.Format != FormatBook || it.State != StateFinished || it.FinishedAt == nil {
			continue
		}
		if d := dayOf(*it.FinishedAt, loc); !d.Before(c.StartedOn) && !d.After(last) {
			cs.Finished++
		}
	}

	end := dayStart(c.Deadline.AddDate(0, 0, 1), loc)
	if !c.Active() || !now.Before(end) {
		cs.Over = true
		return cs
	}
	cs.WeeksLeft = end.Sub(now).Hours() / (7 * 24)

	r := &cs.Required
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
	if allTime > 0 {
		p.BookShare = float64(bookTime) / float64(allTime)
	}
	return p
}

// currentCampaign is the active campaign, or else the one ended last; nil
// before the first. campaigns are ordered oldest first.
func currentCampaign(campaigns []Campaign) *Campaign {
	for i := range campaigns {
		if campaigns[i].Active() {
			return &campaigns[i]
		}
	}
	if n := len(campaigns); n > 0 {
		return &campaigns[n-1]
	}
	return nil
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
	c := &Campaign{ID: newID(), Name: name, TargetCount: target, StartedOn: startedOn, Deadline: deadline}
	err := s.store.Tx(ctx, func(r Repo) error {
		st, err := r.GetSettings()
		if err != nil {
			return err
		}
		loc, err := st.Location()
		if err != nil {
			return err
		}
		today := dayOf(s.now(), loc)
		if startedOn.After(today) {
			return &ValidationError{"started_on", "must be today or earlier"}
		}
		if !deadline.After(today) || !deadline.After(startedOn) {
			return &ValidationError{"deadline", "must be after today and after the start"}
		}
		campaigns, err := r.ListCampaigns()
		if err != nil {
			return err
		}
		for _, other := range campaigns {
			if other.Active() {
				return ErrCampaignActive
			}
		}
		return r.InsertCampaign(c)
	})
	if err != nil {
		return nil, err
	}
	return c, nil
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
		}
		return r.UpdateCampaign(c)
	})
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
