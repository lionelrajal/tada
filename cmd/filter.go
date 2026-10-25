package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lionelrajal/tada/model"
)

// exclusionPrefix marks a list term as "hide matching tasks". A leading dash
// cannot do that job: the flag parser consumes -term before the command sees it,
// so `tada list -milk` fails as an unknown flag.
const exclusionPrefix = "~"

// listFilter is the selection one `tada list` invocation asks for. Every set
// field narrows the result, and the zero value accepts every task.
type listFilter struct {
	terms     []string
	exclude   []string
	overdue   bool
	dueBefore string
	priority  *int
}

// newListFilter reads the positional terms and flag values into a filter.
// dueBefore goes through ParseDueDate, so `--due-before tomorrow` works as well
// as an ISO date. now is the app clock, passed in to keep the result
// reproducible.
func newListFilter(terms []string, overdue bool, dueBefore string, priority *int, now time.Time) (listFilter, error) {
	filter := listFilter{overdue: overdue, priority: priority}

	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			continue
		}
		if pattern, found := strings.CutPrefix(trimmed, exclusionPrefix); found {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				return listFilter{}, fmt.Errorf("empty exclusion term: write ~word to hide tasks matching word")
			}
			filter.exclude = append(filter.exclude, strings.ToLower(pattern))
			continue
		}
		filter.terms = append(filter.terms, strings.ToLower(trimmed))
	}

	if strings.TrimSpace(dueBefore) != "" {
		resolved, err := model.ParseDueDate(dueBefore, now)
		if err != nil {
			return listFilter{}, err
		}
		filter.dueBefore = resolved
	}

	return filter, nil
}

// match reports whether task passes every part of the filter. today is the
// current ISO date, passed in so the caller resolves the clock once. ISO dates
// compare correctly as strings, which is why the data file stores the padded
// form.
func (f listFilter) match(task *model.Task, today string) bool {
	// --overdue is about outstanding work, so a completed task never matches
	// however old its due date is.
	if f.overdue && (task.Completed || task.DueDate == "" || task.DueDate >= today) {
		return false
	}
	if f.dueBefore != "" && (task.DueDate == "" || task.DueDate >= f.dueBefore) {
		return false
	}
	if f.priority != nil && task.Priority != *f.priority {
		return false
	}

	title := strings.ToLower(task.Title)
	for _, term := range f.terms {
		if !strings.Contains(title, term) {
			return false
		}
	}
	for _, pattern := range f.exclude {
		if strings.Contains(title, pattern) {
			return false
		}
	}
	return true
}

// Sort modes accepted by `list --sort`. The empty value keeps insertion order,
// which is what existing scripts and the documented column contract rely on.
const (
	sortInsertion = ""
	sortDue       = "due"
	sortPriority  = "priority"
	sortCreated   = "created"
)

// validateSort rejects an unknown --sort value before anything is read or
// printed, so a typo cannot fall back to some default order silently.
func validateSort(mode string) (string, error) {
	switch mode {
	case sortInsertion, sortDue, sortPriority, sortCreated:
		return mode, nil
	}
	return "", fmt.Errorf("invalid --sort %q: use due, priority or created", mode)
}

// sortTasks returns tasks in the requested order, leaving the caller's slice
// untouched so the store's order is never reordered in place.
//
// Ties break by ID in every mode, which keeps the output stable between runs.
// --sort created orders by ID until completion timestamps land; that is the
// documented behaviour of the flag for now.
func sortTasks(tasks []*model.Task, mode string) []*model.Task {
	if mode == sortInsertion || len(tasks) < 2 {
		return tasks
	}

	sorted := append([]*model.Task(nil), tasks...)

	switch mode {
	case sortDue:
		sort.SliceStable(sorted, func(i, j int) bool {
			a, b := sorted[i], sorted[j]
			switch {
			case a.DueDate == "" && b.DueDate == "":
				return a.ID < b.ID
			case a.DueDate == "":
				return false // Undated tasks sort last.
			case b.DueDate == "":
				return true
			case a.DueDate != b.DueDate:
				return a.DueDate < b.DueDate
			}
			return a.ID < b.ID
		})
	case sortPriority:
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Priority != sorted[j].Priority {
				return sorted[i].Priority > sorted[j].Priority
			}
			return sorted[i].ID < sorted[j].ID
		})
	case sortCreated:
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].ID < sorted[j].ID
		})
	}

	return sorted
}
