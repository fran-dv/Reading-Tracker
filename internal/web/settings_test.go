package web

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSettingsPage(t *testing.T) {
	h, svc := newTestServer(t, &fakeMeta{})
	body := get(t, h, "/settings").Body.String()
	for _, want := range []string{`id="setting-wip_cap"`, `data-bind="values.wip_cap"`, "Weeks start on", `href="/export"`} {
		if !strings.Contains(body, want) {
			t.Errorf("settings missing %q", want)
		}
	}
	in := pageSignals[settingsForm](t, body)
	if in.Values["wip_cap"] != "5" || in.Review != "0" {
		t.Fatalf("seed %+v", in)
	}

	in.Values["wip_cap"] = "three"
	if errs := errorsOf(t, send(t, h, http.MethodPost, "/settings", in).Body.String()); errs["wip_cap"] != "Use a whole number." {
		t.Fatalf("errors = %v", errs)
	}
	in.Values["wip_cap"], in.Timezone = "3", "Mars/Olympus"
	if errs := errorsOf(t, send(t, h, http.MethodPost, "/settings", in).Body.String()); !strings.Contains(errs["timezone"].(string), "Not a time zone") {
		t.Fatalf("errors = %v", errs)
	}
	in.Timezone, in.Review = "America/Buenos_Aires", "1"
	if body := send(t, h, http.MethodPost, "/settings", in).Body.String(); !strings.Contains(body, "Saved.") {
		t.Fatalf("save:\n%s", body)
	}
	st, err := svc.Settings(ctx)
	if err != nil || st.WIPCap != 3 || st.Timezone != "America/Buenos_Aires" || st.ReviewWeekday != time.Monday {
		t.Fatalf("settings %+v, %v", st, err)
	}
}
