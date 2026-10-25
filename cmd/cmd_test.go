package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testClock is the fixed "today" for command tests: 2026-10-25, a Sunday.
// Relative due dates resolve against it so the tests do not depend on the day
// they happen to run.
var testClock = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

// harness runs the real command tree once per invocation against a temporary
// data file, mirroring how the binary behaves when it is run repeatedly.
type harness struct {
	t        *testing.T
	dataFile string
	out      bytes.Buffer
	errOut   bytes.Buffer
	now      func() time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{
		t:        t,
		dataFile: filepath.Join(t.TempDir(), "tasks.json"),
		now:      func() time.Time { return testClock },
	}
}

func (h *harness) run(args ...string) (string, error) {
	h.t.Helper()
	h.out.Reset()
	h.errOut.Reset()

	app := NewApp(&h.out)
	app.Err = &h.errOut
	app.now = h.now

	root := newRootCmd(app)
	root.SetArgs(append([]string{"--data-file", h.dataFile}, args...))
	root.SetOut(&h.out)
	root.SetErr(&h.errOut)

	err := root.Execute()
	return h.out.String(), err
}

func (h *harness) mustRun(args ...string) string {
	h.t.Helper()
	out, err := h.run(args...)
	if err != nil {
		h.t.Fatalf("tada %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func TestCommandRoundTrip(t *testing.T) {
	h := newHarness(t)

	if out := h.mustRun("add", "buy", "milk", "--due", "2026-12-31", "-p", "1"); !strings.Contains(out, "Added task 1: buy milk") {
		t.Fatalf("unexpected add output: %q", out)
	}
	if out := h.mustRun("add", "write", "report"); !strings.Contains(out, "Added task 2: write report") {
		t.Fatalf("unexpected add output: %q", out)
	}

	list := h.mustRun("list")
	for _, want := range []string{"buy milk", "write report", "medium", "2026-12-31"} {
		if !strings.Contains(list, want) {
			t.Fatalf("list output %q missing %q", list, want)
		}
	}

	if out := h.mustRun("done", "1"); !strings.Contains(out, "Completed task 1: buy milk") {
		t.Fatalf("unexpected done output: %q", out)
	}
	if out := h.mustRun("done", "1"); !strings.Contains(out, "already completed") {
		t.Fatalf("expected idempotent done, got %q", out)
	}

	all := h.mustRun("list", "--all")
	if !strings.Contains(all, "[x]") || !strings.Contains(all, "[ ]") {
		t.Fatalf("list --all should show both states, got %q", all)
	}
	if pending := h.mustRun("list"); strings.Contains(pending, "buy milk") {
		t.Fatalf("completed task should not appear in plain list: %q", pending)
	}

	if out := h.mustRun("rm", "2"); !strings.Contains(out, "Removed task 2: write report") {
		t.Fatalf("unexpected rm output: %q", out)
	}
	final := h.mustRun("list", "--all")
	if strings.Contains(final, "write report") || !strings.Contains(final, "buy milk") {
		t.Fatalf("unexpected list after rm: %q", final)
	}
}

func TestEditCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantOut  []string
		wantList []string
		// wantMissing are substrings that must be gone from `list --all`, which
		// is how a cleared or replaced field is observed.
		wantMissing []string
	}{
		{
			name:     "title keeps the id",
			args:     []string{"edit", "1", "--title", "Buy oat milk"},
			wantOut:  []string{"Updated task 1: Buy oat milk"},
			wantList: []string{"1  [ ]", "Buy oat milk"},
		},
		{
			name:     "due and priority together",
			args:     []string{"edit", "2", "--due", "2026-11-02", "--priority", "2"},
			wantOut:  []string{"Updated task 2: write report"},
			wantList: []string{"high", "2026-11-02"},
		},
		{
			name:     "multiple ids take the same change",
			args:     []string{"edit", "1", "2", "--priority", "2"},
			wantOut:  []string{"Updated task 1: Buy milk", "Updated task 2: write report"},
			wantList: []string{"high"},
		},
		{
			name:     "unquoted title words",
			args:     []string{"edit", "1", "buy", "oat", "milk"},
			wantOut:  []string{"Updated task 1: buy oat milk"},
			wantList: []string{"buy oat milk"},
		},
		{
			name:     "title flag followed by unquoted words",
			args:     []string{"edit", "1", "--title", "Buy", "oat", "milk"},
			wantOut:  []string{"Updated task 1: Buy oat milk"},
			wantList: []string{"Buy oat milk"},
		},
		{
			name:        "empty due clears the date",
			args:        []string{"edit", "1", "--due", ""},
			wantOut:     []string{"Updated task 1: Buy milk"},
			wantMissing: []string{"2026-12-31"},
		},
		{
			name:     "no change is an error",
			args:     []string{"edit", "1"},
			wantErr:  "nothing to change",
			wantList: []string{"Buy milk", "2026-12-31", "medium"},
		},
		{
			name:        "unknown id changes nothing",
			args:        []string{"edit", "1", "99", "--title", "nope"},
			wantErr:     "task 99",
			wantList:    []string{"Buy milk", "2026-12-31", "medium"},
			wantMissing: []string{"nope"},
		},
		{
			name:        "invalid date is rejected before writing",
			args:        []string{"edit", "1", "--due", "31-12-2026"},
			wantErr:     "invalid due date",
			wantList:    []string{"2026-12-31", "medium"},
			wantMissing: []string{"Updated task"},
		},
		{
			name:     "priority out of range is rejected",
			args:     []string{"edit", "1", "--priority", "9"},
			wantErr:  "out of range",
			wantList: []string{"medium", "Buy milk"},
		},
		{
			name:    "id missing",
			args:    []string{"edit", "milk"},
			wantErr: "expected a task ID first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.mustRun("add", "Buy", "milk", "--due", "2026-12-31", "-p", "1")
			h.mustRun("add", "write", "report")

			out, err := h.run(tt.args...)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got none (output %q)", tt.wantErr, out)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, want := range tt.wantOut {
				if !strings.Contains(out, want) {
					t.Fatalf("output %q missing %q", out, want)
				}
			}

			list := h.mustRun("list", "--all")
			for _, want := range tt.wantList {
				if !strings.Contains(list, want) {
					t.Fatalf("list output %q missing %q", list, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(list, missing) {
					t.Fatalf("list output %q should not contain %q", list, missing)
				}
			}
		})
	}
}

// TestEditKeepsCompletion checks that editing a finished task does not reopen
// it: edit replaces fields, not state.
func TestEditKeepsCompletion(t *testing.T) {
	h := newHarness(t)
	h.mustRun("add", "old title")
	h.mustRun("done", "1")
	h.mustRun("edit", "1", "new title")

	list := h.mustRun("list", "--all")
	if !strings.Contains(list, "[x]") {
		t.Fatalf("edit reopened a completed task: %q", list)
	}
	if !strings.Contains(list, "new title") {
		t.Fatalf("edit did not apply the new title: %q", list)
	}
}

func TestCommandsCreateDataFile(t *testing.T) {
	h := newHarness(t)
	h.mustRun("add", "persisted")

	data, err := os.ReadFile(h.dataFile)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if !strings.Contains(string(data), `"title": "persisted"`) {
		t.Fatalf("unexpected data file contents: %s", data)
	}
}

func TestListEmpty(t *testing.T) {
	h := newHarness(t)

	if out := h.mustRun("list"); !strings.Contains(out, "No tasks yet") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestListWithoutPending(t *testing.T) {
	h := newHarness(t)
	h.mustRun("add", "only task")
	h.mustRun("done", "1")

	if out := h.mustRun("list"); !strings.Contains(out, "No pending tasks") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestAddRejectsInvalidInput(t *testing.T) {
	h := newHarness(t)

	if _, err := h.run("add", "bad priority", "-p", "9"); err == nil {
		t.Fatal("expected an error for an out-of-range priority")
	}
	if _, err := h.run("add", "bad date", "--due", "not-a-date"); err == nil {
		t.Fatal("expected an error for a malformed due date")
	}
	if _, err := h.run("add"); err == nil {
		t.Fatal("expected an error when no title is given")
	}
}

func TestUnknownIDErrors(t *testing.T) {
	h := newHarness(t)

	if _, err := h.run("done", "7"); err == nil {
		t.Fatal("expected an error for an unknown task ID")
	}
	if _, err := h.run("rm", "7"); err == nil {
		t.Fatal("expected an error for an unknown task ID")
	}
	if _, err := h.run("done", "abc"); err == nil {
		t.Fatal("expected an error for a non-numeric task ID")
	}
}

func TestVersionFlag(t *testing.T) {
	h := newHarness(t)

	out := h.mustRun("--version")
	if !strings.Contains(out, version) {
		t.Fatalf("version output %q does not report %q", out, version)
	}
}

func TestVerboseReportsDataFile(t *testing.T) {
	h := newHarness(t)
	h.mustRun("--verbose", "list")

	if !strings.Contains(h.errOut.String(), h.dataFile) {
		t.Fatalf("verbose output %q missing data file path", h.errOut.String())
	}
}
