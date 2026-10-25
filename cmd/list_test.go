package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lionelrajal/tada/model"
)

// seedList fills a harness with the tasks the filter and sort tests reason
// about. Today is testClock (2026-10-25), so task 1 is overdue and task 5 is
// completed:
//
//	1  Buy milk         2026-10-20  low
//	2  Shop for shoes   2026-11-30  high
//	3  Read book        -           low
//	4  Write report     2026-10-30  medium
//	5  Done thing       -           low     (completed)
func seedList(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.mustRun("add", "Buy", "milk", "--due", "2026-10-20", "-p", "0")
	h.mustRun("add", "Shop", "for", "shoes", "--due", "2026-11-30", "-p", "2")
	h.mustRun("add", "Read", "book")
	h.mustRun("add", "Write", "report", "--due", "2026-10-30", "-p", "1")
	h.mustRun("add", "Done", "thing")
	h.mustRun("done", "5")
	return h
}

func TestListFilters(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		want        []string
		wantMissing []string
	}{
		{
			name:        "term matches the title",
			args:        []string{"milk"},
			want:        []string{"Buy milk"},
			wantMissing: []string{"Write report", "Read book"},
		},
		{
			name: "term ignores case",
			args: []string{"MILK"},
			want: []string{"Buy milk"},
		},
		{
			name:        "terms are ANDed",
			args:        []string{"milk", "shoes"},
			want:        []string{"No pending tasks"},
			wantMissing: []string{"Buy milk", "Shop for shoes"},
		},
		{
			name:        "tilde excludes a term",
			args:        []string{"~buy"},
			want:        []string{"Write report", "Read book"},
			wantMissing: []string{"Buy milk"},
		},
		{
			name:        "include and exclude combine",
			args:        []string{"book", "~read"},
			want:        []string{"No pending tasks"},
			wantMissing: []string{"Read book"},
		},
		{
			name:        "priority narrows the list",
			args:        []string{"--priority", "2"},
			want:        []string{"Shop for shoes"},
			wantMissing: []string{"Buy milk", "Write report"},
		},
		{
			name:        "priority zero is not treated as unset",
			args:        []string{"--priority", "0"},
			want:        []string{"Buy milk", "Read book"},
			wantMissing: []string{"Write report", "Shop for shoes"},
		},
		{
			name:        "overdue shows only past due dates",
			args:        []string{"--overdue"},
			want:        []string{"Buy milk"},
			wantMissing: []string{"Write report", "Shop for shoes", "Read book"},
		},
		{
			name:        "due-before accepts an ISO date",
			args:        []string{"--due-before", "2026-11-01"},
			want:        []string{"Buy milk", "Write report"},
			wantMissing: []string{"Shop for shoes", "Read book"},
		},
		{
			name:        "due-before accepts a relative date",
			args:        []string{"--due-before", "tomorrow"},
			want:        []string{"Buy milk"},
			wantMissing: []string{"Write report"},
		},
		{
			name:        "due-before composes with a term",
			args:        []string{"report", "--due-before", "2026-10-31"},
			want:        []string{"Write report"},
			wantMissing: []string{"Buy milk"},
		},
		{
			name:        "overdue never shows completed tasks even with --all",
			args:        []string{"--all", "--overdue"},
			want:        []string{"Buy milk"},
			wantMissing: []string{"Done thing"},
		},
		{
			name: "all includes completed tasks",
			args: []string{"--all"},
			want: []string{"Done thing"},
		},
		{
			name:        "completed tasks stay hidden by default",
			args:        nil,
			want:        []string{"Buy milk", "Write report"},
			wantMissing: []string{"Done thing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := seedList(t)
			out := h.mustRun(append([]string{"list"}, tt.args...)...)

			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Fatalf("list %v output %q missing %q", tt.args, out, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(out, missing) {
					t.Fatalf("list %v output %q should not contain %q", tt.args, out, missing)
				}
			}
		})
	}
}

// TestListDashTermSuggestsTilde checks that the parser's rejection of -term
// points at the syntax that works instead of failing bare.
func TestListDashTermSuggestsTilde(t *testing.T) {
	h := seedList(t)

	_, err := h.run("list", "-milk")
	if err == nil {
		t.Fatal("expected an error for -milk")
	}
	for _, want := range []string{"unknown shorthand flag", "~term"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should mention %q", err, want)
		}
	}
}

func TestListRejectsEmptyExclusion(t *testing.T) {
	h := seedList(t)

	_, err := h.run("list", "~")
	if err == nil {
		t.Fatal("expected an error for a bare ~")
	}
	if !strings.Contains(err.Error(), "empty exclusion term") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// listIDs returns the ID column of list output, which is how ordering is
// asserted without depending on the padding tabwriter chooses.
func listIDs(t *testing.T, out string) []string {
	t.Helper()

	var ids []string
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		ids = append(ids, strings.Fields(line)[0])
	}
	return ids
}

func TestListSort(t *testing.T) {
	// Seeded so insertion order (1,2,3,4), due order and priority order are
	// all different, and two tasks share a due date to exercise the tie.
	seed := func(t *testing.T) *harness {
		t.Helper()
		h := newHarness(t)
		h.mustRun("add", "late", "--due", "2026-11-30", "-p", "0")
		h.mustRun("add", "soon", "--due", "2026-10-20", "-p", "2")
		h.mustRun("add", "undated", "-p", "1")
		h.mustRun("add", "tie", "--due", "2026-10-20", "-p", "2")
		return h
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"default keeps insertion order", nil, []string{"1", "2", "3", "4"}},
		{"due sorts earliest first with undated last", []string{"--sort", "due"}, []string{"2", "4", "1", "3"}},
		{"priority sorts high to low", []string{"--sort", "priority"}, []string{"2", "4", "3", "1"}},
		{"created sorts by id", []string{"--sort", "created"}, []string{"1", "2", "3", "4"}},
		{"short alias works", []string{"-s", "priority"}, []string{"2", "4", "3", "1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := seed(t)
			got := listIDs(t, h.mustRun(append([]string{"list"}, tt.args...)...))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("list %v IDs = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestListRejectsUnknownSort(t *testing.T) {
	h := seedList(t)

	_, err := h.run("list", "--sort", "size")
	if err == nil {
		t.Fatal("expected an error for an unknown sort mode")
	}
	if !strings.Contains(err.Error(), `invalid --sort "size"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestListDefaultOutputUnchanged pins the documented column layout. The exact
// string is what the v0.1.0 binary printed for the same tasks, so the flag work
// cannot drift the format scripts parse.
func TestListDefaultOutputUnchanged(t *testing.T) {
	h := seedList(t)

	want := "" +
		"1  [ ]  low     2026-10-20  Buy milk\n" +
		"2  [ ]  high    2026-11-30  Shop for shoes\n" +
		"3  [ ]  low     -           Read book\n" +
		"4  [ ]  medium  2026-10-30  Write report\n"

	if got := h.mustRun("list"); got != want {
		t.Fatalf("list output changed:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestListJSON(t *testing.T) {
	h := seedList(t)

	out := h.mustRun("list", "--json")
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		t.Fatalf("expected a JSON array, got %q", out)
	}
	if strings.Count(trimmed, "\n") != 0 {
		t.Fatalf("expected a single JSON document, got %q", out)
	}

	var tasks []model.Task
	if err := json.Unmarshal([]byte(trimmed), &tasks); err != nil {
		t.Fatalf("decode list --json: %v", err)
	}
	if len(tasks) != 4 {
		t.Fatalf("decoded %d tasks, want 4 (completed task hidden): %+v", len(tasks), tasks)
	}

	first := tasks[0]
	if first.ID != 1 || first.Title != "Buy milk" || first.Priority != 0 ||
		first.DueDate != "2026-10-20" || first.Completed {
		t.Fatalf("unexpected first task: %+v", first)
	}
}

func TestListJSONComposes(t *testing.T) {
	h := seedList(t)

	tests := []struct {
		name string
		args []string
		want []int64
	}{
		{"with --all", []string{"--all"}, []int64{1, 2, 3, 4, 5}},
		{"with a term", []string{"milk"}, []int64{1}},
		{"with sorting", []string{"--sort", "due"}, []int64{1, 4, 2, 3}},
		{"with all three", []string{"--all", "~book", "--sort", "priority"}, []int64{2, 4, 1, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := h.mustRun(append([]string{"list", "--json"}, tt.args...)...)

			var tasks []model.Task
			if err := json.Unmarshal([]byte(out), &tasks); err != nil {
				t.Fatalf("decode %q: %v", out, err)
			}

			got := make([]int64, 0, len(tasks))
			for _, task := range tasks {
				got = append(got, task.ID)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ids = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ids = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestListJSONEmptyIsArray(t *testing.T) {
	h := newHarness(t)

	if out := h.mustRun("list", "--json"); strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty list --json = %q, want []", out)
	}

	h.mustRun("add", "only task")
	h.mustRun("done", "1")
	if out := h.mustRun("list", "--json"); strings.TrimSpace(out) != "[]" {
		t.Fatalf("completed-only list --json = %q, want []", out)
	}
}

func TestAddJSON(t *testing.T) {
	h := newHarness(t)

	out := h.mustRun("add", "Buy", "milk", "--due", "tomorrow", "--priority", "2", "--json")
	if strings.Contains(out, "Added task") {
		t.Fatalf("--json should replace the human message, got %q", out)
	}

	var task model.Task
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &task); err != nil {
		t.Fatalf("decode add --json %q: %v", out, err)
	}
	if task.ID != 1 || task.Title != "Buy milk" || task.Priority != 2 || task.DueDate != "2026-10-26" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

// dataFileTasks reads the data file through the format the store writes.
func dataFileTasks(t *testing.T, path string) []model.Task {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	var file struct {
		SchemaVersion int          `json:"schema_version"`
		Tasks         []model.Task `json:"tasks"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("decode data file: %v", err)
	}
	if file.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", file.SchemaVersion)
	}
	return file.Tasks
}

func TestRelativeDueDatesOnCommands(t *testing.T) {
	h := newHarness(t)

	h.mustRun("add", "tomorrow", "--due", "tomorrow")
	h.mustRun("add", "weeks", "--due", "+2w")
	h.mustRun("add", "literal", "--due", "2026-12-31")

	tasks := dataFileTasks(t, h.dataFile)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	want := map[string]string{
		"tomorrow": "2026-10-26",
		"weeks":    "2026-11-08",
		"literal":  "2026-12-31",
	}
	for _, task := range tasks {
		if task.DueDate != want[task.Title] {
			t.Fatalf("%q stored due date %q, want %q", task.Title, task.DueDate, want[task.Title])
		}
	}

	// edit goes through the same parser and still keeps untouched fields.
	h.mustRun("edit", "3", "--due", "fri")
	tasks = dataFileTasks(t, h.dataFile)
	if tasks[2].DueDate != "2026-10-30" {
		t.Fatalf("edit stored due date %q, want 2026-10-30", tasks[2].DueDate)
	}

	h.mustRun("edit", "3", "--due", "")
	if got := dataFileTasks(t, h.dataFile)[2].DueDate; got != "" {
		t.Fatalf("edit --due \"\" left %q, want an empty date", got)
	}
}

func TestRelativeDueDateRejectsGarbage(t *testing.T) {
	h := newHarness(t)
	h.mustRun("add", "task", "--due", "2026-12-31")

	if _, err := h.run("add", "bad", "--due", "someday"); err == nil || !strings.Contains(err.Error(), "invalid due date") {
		t.Fatalf("add with a bad date: %v", err)
	}
	if _, err := h.run("edit", "1", "--due", "next-blursday"); err == nil || !strings.Contains(err.Error(), "invalid due date") {
		t.Fatalf("edit with a bad date: %v", err)
	}

	if got := dataFileTasks(t, h.dataFile)[0].DueDate; got != "2026-12-31" {
		t.Fatalf("a rejected date changed the stored date to %q", got)
	}
}
