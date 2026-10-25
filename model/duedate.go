package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DueDateLayout is the format stored in the data file. Written dates such as
// "tomorrow" are normalised to this before they reach the store, so the file
// format never depends on what the user typed.
const DueDateLayout = "2006-01-02"

// offsetPattern matches relative offsets such as +3d, -1d and +2w.
var offsetPattern = regexp.MustCompile(`^([+-])(\d+)([dw])$`)

// weekdayNames maps the words accepted for a weekday. Both full names and the
// usual abbreviations are included; matching happens after lowercasing.
var weekdayNames = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"sun":       time.Sunday,
	"monday":    time.Monday,
	"mon":       time.Monday,
	"tuesday":   time.Tuesday,
	"tue":       time.Tuesday,
	"tues":      time.Tuesday,
	"wednesday": time.Wednesday,
	"wed":       time.Wednesday,
	"thursday":  time.Thursday,
	"thu":       time.Thursday,
	"thur":      time.Thursday,
	"thurs":     time.Thursday,
	"friday":    time.Friday,
	"fri":       time.Friday,
	"saturday":  time.Saturday,
	"sat":       time.Saturday,
}

// ParseDueDate converts a written due date into the YYYY-MM-DD form the data
// file stores. An empty input means "no due date" and returns "".
//
// Accepted input:
//
//	2026-11-02              an ISO date, kept as it is
//	today, tomorrow, yesterday
//	fri, friday             the next such weekday, never today
//	next-fri                the same weekday in the following week
//	+3d, -1d, +2w           an offset in days or weeks
//
// now is passed in rather than read from the clock so the result is
// reproducible in tests.
func ParseDueDate(input string, now time.Time) (string, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return "", nil
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch value {
	case "today":
		return formatDate(today), nil
	case "tomorrow":
		return formatDate(today.AddDate(0, 0, 1)), nil
	case "yesterday":
		return formatDate(today.AddDate(0, 0, -1)), nil
	}

	if parsed, err := time.Parse(DueDateLayout, value); err == nil {
		return parsed.Format(DueDateLayout), nil
	}

	if weekday, ok := weekdayNames[value]; ok {
		return formatDate(today.AddDate(0, 0, daysUntil(today, weekday))), nil
	}

	if name, found := strings.CutPrefix(value, "next-"); found {
		if weekday, ok := weekdayNames[name]; ok {
			days := daysUntil(today, weekday)
			// daysUntil is 1..7, so a weekday that is still ahead keeps its
			// nearer occurrence and one that is a week away stays put; every
			// other weekday moves into the following week.
			if days < 7 {
				days += 7
			}
			return formatDate(today.AddDate(0, 0, days)), nil
		}
	}

	if match := offsetPattern.FindStringSubmatch(value); match != nil {
		count, err := strconv.Atoi(match[2])
		if err != nil {
			// Unreachable: the pattern only matches digits, but a broken
			// offset must never be silently treated as today.
			return "", fmt.Errorf("invalid due date %q: %w", input, err)
		}
		if match[1] == "-" {
			count = -count
		}
		if match[3] == "w" {
			count *= 7
		}
		return formatDate(today.AddDate(0, 0, count)), nil
	}

	return "", fmt.Errorf("invalid due date %q: use YYYY-MM-DD, today, tomorrow, yesterday, a weekday like fri, next-fri, or an offset like +3d or -2w", input)
}

// daysUntil is the number of days from today to the next weekday, always 1..7.
// A weekday never means today: `--due fri` typed on a Friday means the Friday a
// week later, because a due date of today would be a surprise no-op.
func daysUntil(today time.Time, weekday time.Weekday) int {
	days := (int(weekday) - int(today.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return days
}

func formatDate(t time.Time) string {
	return t.Format(DueDateLayout)
}
