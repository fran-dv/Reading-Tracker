package web

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Time is typed the way it is said and stored in whole minutes. The same
// rules run in the browser for the live readback (static/duration.js); this
// parser is the one that counts.

var (
	clockHM   = regexp.MustCompile(`^(\d+):(\d{1,2})$`)                                                       // 1:30
	clockHMS  = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2})$`)                                             // 1:12:30
	hoursMins = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*h(?:ours?)?\s*(?:(\d+)\s*(?:m|min|mins|minutes?)?)?$`) // 1h30, 1.5h, 2 h 15 min
	minsOnly  = regexp.MustCompile(`^(\d+)\s*(?:m|min|mins|minutes?)?$`)                                      // 90, 90 min
)

// parseMinutes reads a length of time: "90", "90 min", "1h30", "1h 30m",
// "1.5h", "1:30". A bare number is minutes. ok is false for anything else.
func parseMinutes(s string) (minutes int, ok bool) {
	s = normalise(s)
	if m := clockHM.FindStringSubmatch(s); m != nil {
		return atoi(m[1])*60 + atoi(m[2]), atoi(m[2]) < 60
	}
	return hoursOrMinutes(s)
}

// parseTimestamp reads a position in something measured in minutes, the way
// a player shows it: "12:30" is 12 min 30 s, "1:12:30" is 1 h 12 min 30 s.
// Seconds are dropped. Otherwise it reads like parseMinutes.
func parseTimestamp(s string) (minutes int, ok bool) {
	s = normalise(s)
	if m := clockHMS.FindStringSubmatch(s); m != nil {
		return atoi(m[1])*60 + atoi(m[2]), atoi(m[2]) < 60 && atoi(m[3]) < 60
	}
	if m := clockHM.FindStringSubmatch(s); m != nil {
		return atoi(m[1]), atoi(m[2]) < 60
	}
	return hoursOrMinutes(s)
}

func hoursOrMinutes(s string) (int, bool) {
	if m := hoursMins.FindStringSubmatch(s); m != nil {
		hours, _ := strconv.ParseFloat(m[1], 64)
		total := int(math.Round(hours * 60))
		if m[2] != "" {
			total += atoi(m[2])
		}
		return total, true
	}
	if m := minsOnly.FindStringSubmatch(s); m != nil {
		return atoi(m[1]), true
	}
	return 0, false
}

func normalise(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), ",", ".")
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// minutesField writes minutes back into a field the way parseMinutes reads
// them: "45", "1h30", "4h".
func minutesField(m int) string {
	switch {
	case m < 60:
		return strconv.Itoa(m)
	case m%60 == 0:
		return fmt.Sprintf("%dh", m/60)
	}
	return fmt.Sprintf("%dh%02d", m/60, m%60)
}
