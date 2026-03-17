package timeutil

import (
	"testing"
	"time"
)

func TestParseDOB(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		ok    bool
	}{
		{name: "dash format", input: "12-03-2000", ok: true},
		{name: "slash format", input: "12/03/2000", ok: true},
		{name: "year-month dash format", input: "2000-03", ok: true},
		{name: "year-month slash format", input: "2000/03", ok: true},
		{name: "invalid format", input: "2000-03-12", ok: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseDOB(tc.input)
			if tc.ok && err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestAgeFromDOBString(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC)

	age, err := AgeFromDOBString("12/03/2000", now)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if age != 26 {
		t.Fatalf("expected 26, got %d", age)
	}

	yearMonthAge, err := AgeFromDOBString("2000-03", now)
	if err != nil {
		t.Fatalf("unexpected parse error for year-month input: %v", err)
	}
	if yearMonthAge != 26 {
		t.Fatalf("expected 26, got %d", yearMonthAge)
	}
}
