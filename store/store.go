// Package store persists tasks in a JSON file.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lionelrajal/tada/model"
)

// SchemaVersion is the version of the on-disk format written by this build.
// Bump it on any change that would make older binaries misinterpret the file.
const SchemaVersion = 1

// ErrNotFound is returned when no task has the requested ID.
var ErrNotFound = errors.New("task not found")

// ErrSchemaTooNew is returned when the data file was written by a newer build,
// which may store fields this binary would silently drop on the next write.
var ErrSchemaTooNew = errors.New("data file was written by a newer version of tada")

// Store is the persistence layer used by the CLI commands.
type Store interface {
	// Add stores task and assigns it the next free ID.
	Add(task *model.Task) error
	// Get returns the task with the given ID, or ErrNotFound.
	Get(id int64) (*model.Task, error)
	// List returns all tasks in insertion order.
	List() ([]*model.Task, error)
	// Update replaces the stored task that shares task.ID.
	Update(task *model.Task) error
	// Delete removes the task with the given ID, or returns ErrNotFound.
	Delete(id int64) error
}

// fileFormat is the on-disk representation of the task list.
type fileFormat struct {
	SchemaVersion int           `json:"schema_version"`
	Tasks         []*model.Task `json:"tasks"`
}

// JSONStore keeps tasks in a single JSON file.
//
// Every operation reads the file, applies its change and rewrites it: each CLI
// invocation is a short-lived process, so holding tasks in memory would go
// stale as soon as a second process runs.
type JSONStore struct {
	path string
}

var _ Store = (*JSONStore)(nil)

// NewJSONStore returns a store backed by the JSON file at path.
func NewJSONStore(path string) (*JSONStore, error) {
	if path == "" {
		return nil, errors.New("data file path cannot be empty")
	}
	return &JSONStore{path: path}, nil
}

// load reads the task list. A missing or empty file is an empty list, so the
// first command creates the data file on its first write.
func (s *JSONStore) load() ([]*model.Task, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.path, err)
	}

	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	// Files written before the store wrapped tasks in an object hold a bare
	// JSON array, which is still accepted here.
	if data[0] == '[' {
		var legacy []*model.Task
		if err := json.Unmarshal(data, &legacy); err != nil {
			return nil, fmt.Errorf("parse %s: %w", s.path, err)
		}
		return legacy, nil
	}

	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}

	// Files written before the field existed carry no version but use format 1.
	if f.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("%s: format %d, this build supports %d: %w",
			s.path, f.SchemaVersion, SchemaVersion, ErrSchemaTooNew)
	}
	return f.Tasks, nil
}

// save writes the task list to a temporary file and renames it over the data
// file, so an interrupted write cannot truncate an existing task list.
func (s *JSONStore) save(tasks []*model.Task) error {
	if tasks == nil {
		tasks = []*model.Task{}
	}

	data, err := json.MarshalIndent(fileFormat{SchemaVersion: SchemaVersion, Tasks: tasks}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tasks: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tasks-*.json")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // No-op once the rename succeeds.

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace %s: %w", s.path, err)
	}
	return nil
}

// Add appends task and assigns it the lowest unused ID above the current
// maximum, so deleting a task can never make a later ID collide with it.
func (s *JSONStore) Add(task *model.Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}
	tasks, err := s.load()
	if err != nil {
		return err
	}

	var maxID int64
	for _, t := range tasks {
		if t.ID > maxID {
			maxID = t.ID
		}
	}
	task.ID = maxID + 1

	return s.save(append(tasks, task))
}

// Get returns the task with the given ID.
func (s *JSONStore) Get(id int64) (*model.Task, error) {
	tasks, err := s.load()
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("task %d: %w", id, ErrNotFound)
}

// List returns every task in insertion order.
func (s *JSONStore) List() ([]*model.Task, error) {
	return s.load()
}

// Update replaces the stored task with the same ID.
func (s *JSONStore) Update(task *model.Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}
	tasks, err := s.load()
	if err != nil {
		return err
	}
	for i, t := range tasks {
		if t.ID == task.ID {
			tasks[i] = task
			return s.save(tasks)
		}
	}
	return fmt.Errorf("task %d: %w", task.ID, ErrNotFound)
}

// Delete removes the task with the given ID.
func (s *JSONStore) Delete(id int64) error {
	tasks, err := s.load()
	if err != nil {
		return err
	}
	for i, t := range tasks {
		if t.ID == id {
			return s.save(append(tasks[:i], tasks[i+1:]...))
		}
	}
	return fmt.Errorf("task %d: %w", id, ErrNotFound)
}
