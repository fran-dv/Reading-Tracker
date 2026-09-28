package isbn

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"valid ISBN-13 with hyphens", "978-0-306-40615-7", "9780306406157", true},
		{"valid ISBN-10 without hyphens", "0306406152", "9780306406157", true},
		{"ISBN-10 with trailing X check character", "100000001X", "9781000000016", true},
		{"lowercase x accepted", "100000001x", "9781000000016", true},
		{"hyphens and spaces stripped from arbitrary positions", " 0-306 -40615-2 ", "9780306406157", true},
		{"bad checksum, 10 digits", "0306406151", "", false},
		{"bad checksum, 13 digits", "9780306406150", "", false},
		{"wrong length", "12345", "", false},
		{"979-prefix ISBN-13 with no ISBN-10 equivalent", "9791030230451", "9791030230451", true},
		{"X only allowed as the last character", "X306406152", "", false},
		{"empty", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Normalize(tc.in)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("Normalize(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
