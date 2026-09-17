package web

import "testing"

func TestParseMinutes(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"90", 90, true}, {" 45 ", 45, true}, {"90m", 90, true}, {"90 min", 90, true},
		{"1h", 60, true}, {"1h30", 90, true}, {"1h 30m", 90, true}, {"2 h 15 min", 135, true},
		{"1.5h", 90, true}, {"1,5h", 90, true}, {"0.25 hours", 15, true},
		{"1:30", 90, true}, {"0:45", 45, true}, {"1:75", 0, false},
		{"", 0, false}, {"an hour", 0, false}, {"-5", 0, false}, {"1h30x", 0, false},
	}
	for _, c := range cases {
		got, ok := parseMinutes(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseMinutes(%q) = %d, %v; want %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"12:30", 12, true}, {"1:12:30", 72, true}, {"72", 72, true}, {"1h12", 72, true},
		{"12:61", 0, false}, {"1:60:00", 0, false},
	}
	for _, c := range cases {
		got, ok := parseTimestamp(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseTimestamp(%q) = %d, %v; want %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMinutesFieldRoundTrips(t *testing.T) {
	for _, m := range []int{1, 45, 59, 60, 90, 240, 255, 1440} {
		got, ok := parseMinutes(minutesField(m))
		if !ok || got != m {
			t.Errorf("minutesField(%d) = %q reads back as %d, %v", m, minutesField(m), got, ok)
		}
	}
}
