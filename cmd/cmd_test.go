package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// harness runs the real command tree once per invocation against a temporary
// data file, mirroring how the binary behaves when it is run repeatedly.
type harness struct {
	t        *testing.T
	dataFile string
	out      bytes.Buffer
	errOut   bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{t: t, dataFile: filepath.Join(t.TempDir(), "tasks.json")}
}

func (h *harness) run(args ...string) (string, error) {
	h.t.Helper()
	h.out.Reset()
	h.errOut.Reset()

	app := NewApp(&h.out)
	app.Err = &h.errOut

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
