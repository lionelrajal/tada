package store

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"

	"github.com/lionelrajal/tada/model"
)

func newTestStore(t *testing.T) *JSONStore {
	t.Helper()
	s, err := NewJSONStore(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	return s
}

func TestNewJSONStoreRejectsEmptyPath(t *testing.T) {
	if _, err := NewJSONStore(""); err == nil {
		t.Fatal("expected an error for an empty path")
	}
}

func TestJSONStoreAdd(t *testing.T) {
	s := newTestStore(t)

	task := &model.Task{Title: "Test", Priority: 1}
	if err := s.Add(task); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.ID != 1 {
		t.Fatalf("expected ID 1, got %d", task.ID)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Test" {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestJSONStoreAssignsSequentialIDs(t *testing.T) {
	s := newTestStore(t)

	for i, title := range []string{"one", "two", "three"} {
		task := &model.Task{Title: title}
		if err := s.Add(task); err != nil {
			t.Fatalf("Add %q: %v", title, err)
		}
		if task.ID != int64(i+1) {
			t.Fatalf("expected ID %d for %q, got %d", i+1, title, task.ID)
		}
	}

	// Deleting the middle task must not let a later task reuse its ID.
	if err := s.Delete(2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	task := &model.Task{Title: "four"}
	if err := s.Add(task); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.ID != 4 {
		t.Fatalf("expected ID 4 after deleting 2, got %d", task.ID)
	}
}

func TestJSONStoreGetUpdateDelete(t *testing.T) {
	s := newTestStore(t)

	task := &model.Task{Title: "Original", Priority: 0}
	if err := s.Add(task); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := s.Get(task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Original" {
		t.Fatalf("Get returned %q", got.Title)
	}

	got.Completed = true
	got.Priority = 2
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := s.Get(task.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if !updated.Completed || updated.Priority != 2 {
		t.Fatalf("update not persisted: %+v", updated)
	}

	if err := s.Delete(task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestJSONStoreMissingTaskErrors(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Get(42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get: expected ErrNotFound, got %v", err)
	}
	if err := s.Update(&model.Task{ID: 42, Title: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update: expected ErrNotFound, got %v", err)
	}
	if err := s.Delete(42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete: expected ErrNotFound, got %v", err)
	}
}

func TestJSONStoreMissingFileIsEmpty(t *testing.T) {
	s := newTestStore(t)

	list, err := s.List()
	if err != nil {
		t.Fatalf("List on missing file: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no tasks, got %v", list)
	}
}

func TestJSONStoreMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	if _, err := s.List(); err == nil {
		t.Fatal("expected a parse error for malformed JSON")
	}
}

func TestJSONStorePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")

	first, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	task := &model.Task{Title: "Persisted", Priority: 1, DueDate: "2026-12-31"}
	if err := first.Add(task); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// A second store instance stands in for the next CLI invocation.
	second, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	list, err := second.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Persisted" || list[0].DueDate != "2026-12-31" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestJSONStoreWritesSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	if err := s.Add(&model.Task{Title: "versioned"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if !strings.Contains(string(data), `"schema_version": 1`) {
		t.Fatalf("data file is missing the schema version: %s", data)
	}
}

func TestJSONStoreRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	newer := `{"schema_version": 2, "tasks": [{"id":1,"title":"from the future"}]}`
	if err := os.WriteFile(path, []byte(newer), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	if _, err := s.List(); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("List: expected ErrSchemaTooNew, got %v", err)
	}
	// Writing would downgrade the file and drop fields this build cannot see,
	// so it must fail too.
	if err := s.Add(&model.Task{Title: "nope"}); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("Add: expected ErrSchemaTooNew, got %v", err)
	}
}

func TestJSONStoreLoadsLegacyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	legacy := `[{"id":1,"title":"Old format","completed":false,"priority":2,"due_date":""}]`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Old format" {
		t.Fatalf("unexpected list: %+v", list)
	}

	// Saving upgrades the file to the wrapper format without losing the task.
	if err := s.Add(&model.Task{Title: "New"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	task, err := s.Get(2)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Title != "New" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

// TestConcurrentStoresKeepEveryTask checks that writers sharing one data file
// serialise their read-modify-write instead of each rewriting the list they
// loaded before the others saved.
func TestConcurrentStoresKeepEveryTask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")

	const writers, perWriter = 8, 3

	var wg sync.WaitGroup
	errCh := make(chan error, writers*perWriter)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// A separate store per writer, so no in-memory state is shared.
			s, err := NewJSONStore(path)
			if err != nil {
				errCh <- err
				return
			}
			for j := 0; j < perWriter; j++ {
				if err := s.Add(&model.Task{Title: fmt.Sprintf("writer %d task %d", i, j)}); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("Add: %v", err)
	}

	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != writers*perWriter {
		t.Fatalf("lost tasks: got %d, want %d (%v)", len(list), writers*perWriter, taskTitles(list))
	}

	seen := make(map[int64]string, len(list))
	for _, task := range list {
		if other, dup := seen[task.ID]; dup {
			t.Fatalf("id %d assigned twice: %q and %q", task.ID, other, task.Title)
		}
		seen[task.ID] = task.Title
	}
}

// TestConcurrentAddsAcrossProcesses is the regression test for lost updates.
// It runs the real store in separate OS processes, because that is what the CLI
// does and an in-process lock would pass here while the CLI still lost data.
func TestConcurrentAddsAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns child processes")
	}

	path := filepath.Join(t.TempDir(), "tasks.json")

	const children = 6

	var wg sync.WaitGroup
	failures := make(chan string, children)
	for i := 0; i < children; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcessAdd$", "-test.count=1")
			cmd.Env = append(os.Environ(),
				"TADA_HELPER_DATA_FILE="+path,
				fmt.Sprintf("TADA_HELPER_TITLE=child %d", i),
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				failures <- fmt.Sprintf("child %d: %v\n%s", i, err, out)
			}
		}(i)
	}
	wg.Wait()
	close(failures)
	for msg := range failures {
		t.Error(msg)
	}

	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != children {
		t.Fatalf("lost tasks: got %d, want %d (%v)", len(list), children, taskTitles(list))
	}
}

// TestHelperProcessAdd is not a test of its own: it is the body that
// TestConcurrentAddsAcrossProcesses runs in a child process.
func TestHelperProcessAdd(t *testing.T) {
	path := os.Getenv("TADA_HELPER_DATA_FILE")
	if path == "" {
		t.Skip("only runs when spawned as a helper process")
	}
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	if err := s.Add(&model.Task{Title: os.Getenv("TADA_HELPER_TITLE")}); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

// TestMutationsRespectTheLock checks that every write path waits for the lock
// and fails loudly instead of writing while another process holds it.
func TestMutationsRespectTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	if err := s.Add(&model.Task{Title: "existing"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s.lockTimeout = 100 * time.Millisecond

	held := flock.New(s.lockPath())
	locked, err := held.TryLock()
	if err != nil || !locked {
		t.Fatalf("acquire lock: locked=%v err=%v", locked, err)
	}
	defer func() { _ = held.Unlock() }()

	start := time.Now()
	if err := s.Add(&model.Task{Title: "blocked"}); !errors.Is(err, ErrLockTimeout) {
		t.Errorf("Add: expected ErrLockTimeout, got %v", err)
	}
	if err := s.Update(&model.Task{ID: 1, Title: "renamed"}); !errors.Is(err, ErrLockTimeout) {
		t.Errorf("Update: expected ErrLockTimeout, got %v", err)
	}
	if err := s.Delete(1); !errors.Is(err, ErrLockTimeout) {
		t.Errorf("Delete: expected ErrLockTimeout, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("waits should be bounded, took %s", elapsed)
	}

	// Nothing may have been written while the lock was held.
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Title != "existing" {
		t.Fatalf("blocked writes reached the data file: %v", taskTitles(list))
	}
}

// taskTitles renders a task list for failure messages without depending on the
// JSON encoding.
func taskTitles(tasks []*model.Task) []string {
	titles := make([]string, len(tasks))
	for i, task := range tasks {
		titles[i] = task.Title
	}
	return titles
}
