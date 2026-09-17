package web

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The book page shows where the reading stands and every session, titles
// open it from elsewhere, and a correction made on it redraws it.
func TestItemPage(t *testing.T) {
	f := newSessionFixture(t)
	now := time.Now()
	if _, err := f.svc.AddRetroactiveSession(ctx, f.second.ID, now.Add(-3*time.Hour), now.Add(-2*time.Hour), ptr(30), ""); err != nil {
		t.Fatal(err)
	}
	path := "/items/" + f.second.ID

	body := get(t, f.handler, path).Body.String()
	for _, want := range []string{
		"<h1 class=\"heading\">Deep Work</h1>", "reading", "page 30 of 296", "at its own pace",
		"30 pages/h over 1 h 00 min", "1 h 00 min over 1 day, 1 session", "page 0 → 30",
		`href="/session?item=` + f.second.ID + `"`, "About ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("book page missing %q", want)
		}
	}
	if rec := get(t, f.handler, "/items/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", rec.Code)
	}
	if session := get(t, f.handler, "/session").Body.String(); !strings.Contains(session, `<a href="`+path+`">Deep Work</a>`) {
		t.Error("a session row's title should open the book page")
	}

	sessions, _ := f.svc.Sessions(ctx, f.second.ID)
	var in sessionForm
	in.ItemPage = f.second.ID
	body = send(t, f.handler, http.MethodPost, "/sessions/"+sessions[0].ID+"/delete", in).Body.String()
	if !strings.Contains(body, `id="item-body"`) || !strings.Contains(body, "Nothing logged on it yet.") {
		t.Fatalf("a delete on the book page should redraw it:\n%s", body)
	}
}
