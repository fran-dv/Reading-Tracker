package library_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// utc is a time of day in UTC, where the set tests read their days.
func utc(m time.Month, d, hour, minute int) time.Time {
	return time.Date(2026, m, d, hour, minute, 0, 0, time.UTC)
}

// A set campaign is measured by the time left on its items: what it needs a
// week, what its own hours give it, and where that lands.
func TestMeasureSetCampaign(t *testing.T) {
	c := library.Campaign{Kind: library.KindSet, StartedOn: day(9, 1), Deadline: day(9, 30)}
	// 600 pages read at 30 pages/h: 20 h in all, 5 h of them done.
	adm := book("adm", library.StateInProgress, 600)
	done := book("done", library.StateFinished, 300, finishedAt(utc(9, 10, 12, 0)))
	other := book("other", library.StateInProgress, 300)
	members := []library.CampaignItem{
		{CampaignID: c.ID, ItemID: "adm", AddedOn: day(9, 1)},
		{CampaignID: c.ID, ItemID: "done", AddedOn: day(9, 1)},
	}
	sessions := []library.Session{
		// 5 h on the set's own book, reaching page 150: its pace is 30/h.
		session("adm", utc(9, 5, 10, 0), 5*time.Hour, ptr(0), ptr(150)),
		// 5 h on something else in the same span: half the reading is the set's.
		session("other", utc(9, 6, 10, 0), 5*time.Hour, nil, nil),
	}
	now := utc(9, 15, 12, 0)

	cs := library.MeasureCampaign(c, members, byID(adm, done, other), sessions, campaignSettings(), time.UTC, now)
	if cs.Target != 2 || cs.Finished != 1 || cs.Lost {
		t.Fatalf("target %d, finished %d, lost %v; want 2, 1, false", cs.Target, cs.Finished, cs.Lost)
	}
	if len(cs.Items) != 2 || cs.Items[0].Position != 150 || !cs.Items[1].Done {
		t.Fatalf("items %+v", cs.Items)
	}
	// 450 pages left of the unfinished book at 30 pages/h.
	if !near(cs.Required.HoursLeft, 15) {
		t.Fatalf("hours left %.2f, want 15", cs.Required.HoursLeft)
	}
	if !near(cs.Required.WeeklyHours, 15/cs.WeeksLeft) {
		t.Fatalf("needed each week %.2f over %.2f weeks", cs.Required.WeeklyHours, cs.WeeksLeft)
	}
	p := cs.Set
	if p == nil || !near(p.Share, 0.5) {
		t.Fatalf("set progress %+v, want half the reading", p)
	}
	// 5 h over the fortnight and a half since it began.
	if math.Abs(p.Weeks-14.5/7) > 1e-6 || !near(p.Hours, 5/p.Weeks) {
		t.Fatalf("weeks %.4f, hours a week %.2f", p.Weeks, p.Hours)
	}
	// At 2 h 27 min a week, 15 h needs far longer than the fortnight left.
	if !p.DoneOn.IsZero() || p.Covered >= 1 || p.Covered <= 0 {
		t.Fatalf("done on %v, covered %.2f; want no date and a part of it", p.DoneOn, p.Covered)
	}

	// Reading nothing of its own leaves it without a projection.
	bare := library.MeasureCampaign(c, members, byID(adm, done, other), sessions[1:], campaignSettings(), time.UTC, now)
	if bare.Set != nil {
		t.Fatalf("set progress without its own reading: %+v", bare.Set)
	}
}

// An abandoned item stays in the set, and says the set can no longer be met.
func TestSetLosesAnAbandonedItem(t *testing.T) {
	c := library.Campaign{Kind: library.KindSet, StartedOn: day(9, 1), Deadline: day(9, 30)}
	gone := book("gone", library.StateAbandoned, 300)
	rest := book("rest", library.StateFinished, 300, finishedAt(utc(9, 10, 12, 0)))
	members := []library.CampaignItem{
		{ItemID: "gone", AddedOn: day(9, 1)}, {ItemID: "rest", AddedOn: day(9, 1)},
	}
	cs := library.MeasureCampaign(c, members, byID(gone, rest), nil, campaignSettings(), time.UTC, utc(9, 15, 12, 0))
	if !cs.Lost || !cs.Items[0].Lost || cs.Reached() {
		t.Fatalf("abandoned item: lost %v, items %+v, reached %v", cs.Lost, cs.Items, cs.Reached())
	}
	// It is left out of the hours: what is owed is only what can still be read.
	if cs.Required.HoursLeft != 0 {
		t.Fatalf("hours left %.2f, want none: one item is finished and the other abandoned", cs.Required.HoursLeft)
	}
}

// A set is met when its last item is finished by the deadline, and a book in
// it counts toward a count campaign running beside it.
func TestSetLifecycleAndAchievements(t *testing.T) {
	svc, clk := utcLibrary(t)
	shelf := newShelf(t, svc, "S")
	clk.now = sep(1, 8)
	year, err := svc.StartCampaign(ctx, "", 4, sep(1, 0), sep(30, 0))
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for _, title := range []string{"One", "Two"} {
		ids = append(ids, newItem(t, svc, shelf.ID, title, func(it *library.Item) { it.SizeValue = ptr(100) }).ID)
	}
	set, err := svc.StartSetCampaign(ctx, "", ids[:1], sep(1, 0), sep(30, 0))
	if err != nil {
		t.Fatal(err)
	}
	if set.Name != "One by 30 Sep 2026" || set.Kind != library.KindSet || set.TargetCount != 0 {
		t.Fatalf("set %+v, want a name from its one item", set)
	}
	// Its items are shortlisted, once, so the picks show what it asks for.
	if it, err := svc.GetItem(ctx, ids[0]); err != nil || !it.OnShortlist {
		t.Fatalf("the set's item should be shortlisted: %v", err)
	}

	// A set can only grow, and only with items that can still be read.
	if err := svc.AddCampaignItems(ctx, set.ID, ids[1:]); err != nil {
		t.Fatal(err)
	}
	var verr *library.ValidationError
	if err := svc.AddCampaignItems(ctx, set.ID, nil); !errors.As(err, &verr) {
		t.Fatalf("adding nothing: %v", err)
	}

	for i, id := range ids {
		clk.now = sep(5+i*5, 8)
		startItem(t, svc, id)
		if _, err := svc.AddRetroactiveSession(ctx, id, sep(5+i*5, 6), sep(5+i*5, 8), ptr(100), ""); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Finish(ctx, id, "", nil); err != nil {
			t.Fatal(err)
		}
	}
	clk.now = sep(21, 9)

	view, err := svc.Campaign(ctx, set.ID)
	if err != nil {
		t.Fatal(err)
	}
	cs := view.State
	if cs.Target != 2 || cs.Finished != 2 || !cs.Reached() || cs.Required.HoursLeft != 0 {
		t.Fatalf("met set: %+v", cs)
	}

	all, err := svc.Achievements(ctx)
	if err != nil {
		t.Fatal(err)
	}
	items := byKind(all, library.SetItemFinished)
	if len(items) != 2 || items[0].Campaign.ID != set.ID || items[0].Count != 2 || items[0].Target != 2 {
		t.Fatalf("set items finished %+v", items)
	}
	met := byKind(all, library.CampaignMet)
	if len(met) != 1 || met[0].Campaign.ID != set.ID || !met[0].On.Equal(sep(10, 0)) {
		t.Fatalf("met %+v, want the set on 10 Sep", met)
	}
	// The same books still count toward the campaign counting books.
	books := byKind(all, library.BookFinished)
	if len(books) != 2 || len(books[0].Toward) != 1 || books[0].Toward[0].Campaign.ID != year.ID || books[0].Toward[0].Count != 2 {
		t.Fatalf("books %+v, want each counted toward the year", books)
	}

	// Nothing can be added once it is met.
	third := newItem(t, svc, shelf.ID, "Three", func(it *library.Item) { it.SizeValue = ptr(100) })
	if err := svc.AddCampaignItems(ctx, set.ID, []string{third.ID}); !errors.As(err, &verr) || verr.Field != "items" {
		t.Fatalf("adding to a met set: %v, want a ValidationError on items", err)
	}
}

// What a set needs moves with the hours left and the weeks left, and a
// change of more than a tenth names which.
func TestSetNeedsChange(t *testing.T) {
	needs := func(hours, weeks float64) library.Needs {
		return library.Needs{CampaignID: "c", Kind: library.KindSet, HoursLeft: hours, WeeksLeft: weeks, WeeklyHours: hours / weeks}
	}
	since := day(9, 6)
	steady := library.CompareNeeds(since, needs(20, 4), needs(15, 3))
	if len(steady.Causes) != 0 || !near(steady.Ratio(), 1) {
		t.Fatalf("reading to plan changes nothing: %+v", steady)
	}
	slipped := library.CompareNeeds(since, needs(20, 4), needs(19, 2))
	if len(slipped.Causes) == 0 || slipped.Causes[0] != library.CauseWeeks {
		t.Fatalf("causes %v, want the weeks remaining first", slipped.Causes)
	}
	grown := library.CompareNeeds(since, needs(20, 4), needs(40, 3.9))
	if len(grown.Causes) == 0 || grown.Causes[0] != library.CauseHours {
		t.Fatalf("causes %v, want the hours left first", grown.Causes)
	}
}
