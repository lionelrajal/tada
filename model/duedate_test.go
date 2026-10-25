package model

import (
	"strings"
	"testing"
	"time"
)

// sunday is the reference clock for these tests: 2026-10-25 is a Sunday.
var sunday = time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)

func TestParseDueDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty means no due date", "", ""},
		{"whitespace means no due date", "   ", ""},
		{"iso date passes through", "2026-11-02", "2026-11-02"},
		{"iso date is trimmed", "  2026-11-02  ", "2026-11-02"},
		{"today", "today", "2026-10-25"},
		{"tomorrow", "tomorrow", "2026-10-26"},
		{"yesterday", "yesterday", "2026-10-24"},
		{"words are case-insensitive", "ToMoRrOw", "2026-10-26"},
		{"weekday abbreviation", "mon", "2026-10-26"},
		{"weekday full name", "friday", "2026-10-30"},
		{"weekday later this week", "sat", "2026-10-31"},
		// A weekday never resolves to today: on a Sunday, "sun" is next Sunday.
		{"weekday is never today", "sun", "2026-11-01"},
		{"next weekday", "next-mon", "2026-11-02"},
		{"next weekday in the following week", "next-fri", "2026-11-06"},
		{"next of the same weekday is a week away", "next-sun", "2026-11-01"},
		{"day offset", "+3d", "2026-10-28"},
		{"week offset", "+2w", "2026-11-08"},
		{"negative day offset", "-1d", "2026-10-24"},
		{"negative week offset", "-1w", "2026-10-18"},
		{"zero offset", "+0d", "2026-10-25"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDueDate(tt.input, sunday)
			if err != nil {
				t.Fatalf("ParseDueDate(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseDueDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseDueDateOnFriday pins the rule the issue calls out: typing the
// current weekday must not produce today's date.
func TestParseDueDateOnFriday(t *testing.T) {
	friday := time.Date(2026, 10, 30, 18, 0, 0, 0, time.UTC)
	if friday.Weekday() != time.Friday {
		t.Fatalf("test clock is a %s, not a Friday", friday.Weekday())
	}

	got, err := ParseDueDate("fri", friday)
	if err != nil {
		t.Fatalf("ParseDueDate(fri) returned error: %v", err)
	}
	if got != "2026-11-06" {
		t.Fatalf("fri on a Friday = %q, want the following Friday 2026-11-06", got)
	}
}

// TestParseDueDateIgnoresTimeOfDay checks that the date comes from the calendar
// day, not from the time the command happened to run.
func TestParseDueDateIgnoresTimeOfDay(t *testing.T) {
	lateAtNight := time.Date(2026, 10, 25, 23, 59, 59, 0, time.UTC)
	got, err := ParseDueDate("tomorrow", lateAtNight)
	if err != nil {
		t.Fatalf("ParseDueDate(tomorrow) returned error: %v", err)
	}
	if got != "2026-10-26" {
		t.Fatalf("tomorrow at 23:59 = %q, want 2026-10-26", got)
	}
}

func TestParseDueDateRejectsUnknownWords(t *testing.T) {
	inputs := []string{
		"not-a-date",
		"31-12-2026",
		"2026-13-01",
		"next-blursday",
		"funday",
		"3d",    // an offset needs a sign
		"+3m",   // months are not part of the vocabulary
		"+d",    // no number
		"next-", // no weekday
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			got, err := ParseDueDate(input, sunday)
			if err == nil {
				t.Fatalf("ParseDueDate(%q) = %q, want an error", input, got)
			}
			if !strings.Contains(err.Error(), "invalid due date") {
				t.Fatalf("error %q should mention an invalid due date", err)
			}
			if got != "" {
				t.Fatalf("ParseDueDate(%q) returned %q alongside an error", input, got)
			}
		})
	}
}
