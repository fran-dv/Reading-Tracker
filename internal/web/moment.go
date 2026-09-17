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
		m.Line = fmt.Sprintf("%s by %s, %s.", countLabel(c.TargetCount, "book"), day, early(a.On, c.Deadline))
		m.Facts = campaignFacts(a)
		m.Facts = append(m.Facts, "first: "+a.First.Title, "last: "+a.Last.Title)
	case library.CampaignHalfway:
		c := a.Campaign
		m.Headline = "Halfway."
		m.Line = fmt.Sprintf("%d of %d on %s, with %s to go.", a.Count, c.TargetCount, day, spanLabelDays(daysBetween(a.On, c.Deadline)-1))
		m.Facts = append([]string{paceAgainstEven(a.Ahead)}, campaignFacts(a)...)
	case library.HoursRampTop:
		m.Headline = minutesLabel(minutes(a.To)) + " a day."
		m.Line = fmt.Sprintf("Your daily target reached its top on %s, %s after it began at %s.",
			day, spanLabelDays(daysBetween(a.Began, a.On)-1), minutesLabel(minutes(a.Start)))
		m.Facts = []string{heldLabel(a.Holds)}
	case library.SpeedRampTop:
		m.Headline = fmt.Sprintf("%d%% of your baseline.", a.To)
		m.Line = fmt.Sprintf("Your speed target reached its top on %s, %s after the ramp began.",
			day, spanLabelDays(daysBetween(a.Began, a.On)-1))
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
	days := daysBetween(met, deadline) - 1
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

// daysBetween counts calendar days from one to another, both included.
func daysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours()/24) + 1
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
// for a book, its count toward the campaign; then what it took and the
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
			if a.Campaign != nil {
				status = fmt.Sprintf("Finished %s: %d of %d.", item.Title, a.Count, a.Campaign.TargetCount)
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
