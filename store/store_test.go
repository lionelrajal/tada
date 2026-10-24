package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
