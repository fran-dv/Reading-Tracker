package web

import (
	"strings"
	"testing"
	"time"
)

// The archive shows what was finished with its why and verdict, and a spine
// for each on the shelf; before anything is finished it says so warmly.
func TestArchivePage(t *testing.T) {
	f := newHomeFixture(t)
	if body := get(t, f.handler, "/archive").Body.String(); !strings.Contains(body, "Nothing finished yet.") {
		t.Fatalf("empty archive:\n%s", body)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := f.svc.Finish(ctx, f.book.ID, "Worth it.", nil); err != nil {
		t.Fatal(err)
	}
	body := get(t, f.handler, "/archive").Body.String()
	for _, want := range []string{
		`aria-current="page">Finished`, `archive-figure figure">1</span> book finished`,
		`class="spine cloth-book"`, "Deep Work", "“Worth it.”", "focus",
		"finished " + time.Now().Format("2 Jan"),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("archive missing %q", want)
		}
	}
}
