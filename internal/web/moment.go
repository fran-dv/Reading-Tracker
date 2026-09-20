package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fran-dv/reading-tracker/internal/library"
)

// Achievements as Home acknowledges them (spec §6.10): a moment above the
// board for a goal reached, and a warm line when a book is finished.

// momentView is a big achievement drawn on Home.
type momentView struct {
	Key      string
	Headline string   // "You did it."
	Line     string   // the dated fact
	Facts    []string // the few figures behind it
	Href     string   // where it is kept
	HrefText string
}

// newMomentView puts a big achievement into words; nil for anything else.
func newMomentView(a *library.Achievement) *momentView {
	if a == nil {
		return nil
	}
	day := a.On.Format("2 Jan 2006")
	m := &momentView{Key: a.Key, Href: "/record", HrefText: "See the record"}
	switch a.Kind {
	case library.CampaignMet:
		c := a.Campaign
		m.Headline = "You did it."
		m.Line = fmt.Sprintf("%s by %s, %s.", countLabel(a.Target, asks(c)), day, early(a.On, c.Deadline))
		m.Facts = campaignFacts(a)
		if c.Kind == library.KindSet {
			m.Facts = append(m.Facts, "last: "+a.Last.Title)
		} else {
			m.Facts = append(m.Facts, "first: "+a.First.Title, "last: "+a.Last.Title)
		}
	case library.CampaignHalfway:
		c := a.Campaign
		m.Headline = "Halfway."
		m.Line = fmt.Sprintf("%d of %d on %s, with %s to go.", a.Count, a.Target, day, spanLabelDays(library.DaysBetween(a.On, c.Deadline)-1))
		m.Facts = campaignFacts(a)
		if c.Kind == library.KindCount {
			m.Facts = append([]string{paceAgainstEven(a.Ahead)}, m.Facts...)
		}
	case library.HoursRampTop:
		m.Headline = minutesLabel(minutes(a.To)) + " a day."
		m.Line = fmt.Sprintf("Your daily target reached its top on %s, %s after it began at %s.",
			day, spanLabelDays(library.DaysBetween(a.Began, a.On)-1), minutesLabel(minutes(a.Start)))
		m.Facts = []string{heldLabel(a.Holds)}
	case library.SpeedRampTop:
		m.Headline = fmt.Sprintf("%d%% of your baseline.", a.To)
		m.Line = fmt.Sprintf("Your speed target reached its top on %s, %s after the ramp began.",
			day, spanLabelDays(library.DaysBetween(a.Began, a.On)-1))
		m.Facts = []string{heldLabel(a.Holds)}
	default:
		return nil
	}
	return m
}

// campaignFacts is what the books counted so far came to.
func campaignFacts(a *library.Achievement) []string {
	var facts []string
	if a.Pages > 0 {
		facts = append(facts, grouped(a.Pages)+" pages")
	}
	if a.Time > 0 {
		facts = append(facts, minutesLabel(a.Time)+" of reading")
	}
	return append(facts, "in "+spanLabelDays(a.Days))
}

// early says how a campaign met stands against its deadline.
func early(met, deadline time.Time) string {
	days := library.DaysBetween(met, deadline) - 1
	if days == 0 {
		return "on its last day"
	}
	return spanLabelDays(days) + " before the deadline"
}

// paceAgainstEven writes books ahead of or behind an even pace.
func paceAgainstEven(ahead int) string {
	switch {
	case ahead > 0:
		return countLabel(ahead, "book") + " ahead of an even pace"
	case ahead < 0:
		return countLabel(-ahead, "book") + " behind an even pace"
	}
	return "right on an even pace"
}

func heldLabel(holds int) string {
	if holds == 0 {
		return "it never held"
	}
	return "held " + countLabel(holds, "week") + " on the way"
}

// postCloseMoment acknowledges a moment on Home; Home then shows the next.
func (h *handler) postCloseMoment(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readHome(w, r)
	if !ok {
		return
	}
	h.patchHome(w, r, in.Moment, entryForm{}, "", h.svc.CloseMoment(r.Context(), r.PathValue("key")))
}

// finishedLines is what Home says when an item is finished: its title and,
// for a book, its count toward each campaign; then what it took and the
// verdict.
func (h *handler) finishedLines(ctx context.Context, item *library.Item) (status, more string) {
	status = "Finished " + item.Title + "."
	var took []string
	if item.Format == library.FormatBook {
		all, err := h.svc.Achievements(ctx)
		if err != nil {
			h.log.Error("finished line", "err", err)
		}
		for _, a := range all {
			if a.Kind != library.BookFinished || a.Item.ID != item.ID {
				continue
			}
			if counts := towardLabel(a.Toward); counts != "" {
				status = fmt.Sprintf("Finished %s: %s.", item.Title, counts)
			}
			if a.Pages > 0 {
				took = append(took, grouped(a.Pages)+" pages")
			}
			if a.Time > 0 {
				took = append(took, minutesLabel(a.Time)+" over "+spanLabelDays(a.Days))
			}
		}
	}
	more = strings.Join(took, " in ")
	if item.Verdict != "" {
		if more != "" {
			more += " · "
		}
		more += "“" + item.Verdict + "”"
	}
	return status, more
}

// reachedView is the review's account of what was reached since the last
// one, and what last week gave each item.
type reachedView struct {
	From     string // "13 Sep"
	Lines    []reachedLine
	LastWeek []weekPart
}

type reachedLine struct {
	Text string
	Day  string // "Mon 14 Sep"
	Big  bool
}

func newReachedView(v *library.ReviewView) reachedView {
	rv := reachedView{From: v.ReachedFrom.Format("2 Jan")}
	for _, a := range v.Reached {
		rv.Lines = append(rv.Lines, reachedLine{Text: achievementLine(a), Day: a.On.Format("Mon 2 Jan"), Big: a.Big()})
	}
	for _, it := range v.LastWeek {
		p := weekPart{ItemID: it.Item.ID, Title: it.Item.Title, Format: it.Item.Format, Time: minutesLabel(it.Time), Sessions: countLabel(it.Sessions, "session")}
		if it.Progress > 0 && it.Item.SizeUnit != library.UnitMinutes {
			p.Progress = grouped(it.Progress) + " " + string(it.Item.SizeUnit) + ", to " + positionLabel(it.Item, it.Reached)
		}
		rv.LastWeek = append(rv.LastWeek, p)
	}
	return rv
}

// towardLabel is a book's count toward each campaign it counted for: "4 of
// 100", or, with more than one, "4 of 100 for A hundred · 2 of 5 for October".
func towardLabel(toward []library.Counted) string {
	var parts []string
	for _, t := range toward {
		part := fmt.Sprintf("%d of %d", t.Count, t.Target)
		if len(toward) > 1 {
			part += " for " + t.Campaign.Name
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " · ")
}

// asks is what a campaign counts, for a line that says how many.
func asks(c *library.Campaign) string {
	if c.Kind == library.KindSet {
		return "item"
	}
	return "book"
}

// achievementLine names an achievement in one line.
func achievementLine(a library.Achievement) string {
	switch a.Kind {
	case library.SetItemFinished:
		return fmt.Sprintf("Finished %s for %s: %d of %d", a.Item.Title, a.Campaign.Name, a.Count, a.Target)
	case library.BookFinished:
		if counts := towardLabel(a.Toward); counts != "" {
			return fmt.Sprintf("Finished %s: %s", a.Item.Title, counts)
		}
		return "Finished " + a.Item.Title
	case library.CampaignHalfway:
		return fmt.Sprintf("Halfway through %s: %d of %d", a.Campaign.Name, a.Count, a.Target)
	case library.CampaignMet:
		return "Met " + a.Campaign.Name
	case library.HoursRampStep:
		return "Daily target rose to " + minutesLabel(minutes(a.To))
	case library.HoursRampTop:
		return "Daily target reached its top: " + minutesLabel(minutes(a.To)) + " a day"
	case library.SpeedRampStep:
		return fmt.Sprintf("Speed target rose to %d%%", a.To)
	case library.SpeedRampTop:
		return fmt.Sprintf("Speed target reached its top: %d%%", a.To)
	}
	return string(a.Kind)
}
