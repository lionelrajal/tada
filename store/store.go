// Package store persists tasks in a JSON file.
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

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

// ErrLockTimeout is returned when another process holds the write lock for
// longer than the caller is willing to wait.
var ErrLockTimeout = errors.New("timed out waiting for the data file lock")

// Lock tuning for writes. The timeout is deliberately short: a tada command
// takes milliseconds, so a lock held for seconds means a stuck process, and
// hanging forever would be worse than failing loudly.
const (
	defaultLockTimeout = 5 * time.Second
	lockRetryDelay     = 20 * time.Millisecond
)

// Store is the persistence layer used by the CLI commands.
//
// Implementations must serialise their own mutations: every command is a
// short-lived process, so two tada invocations can target one data file at
// the same time, and a mutation that reads, changes and rewrites the whole
// list must not interleave with another.
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
//
// Because the whole list is rewritten, every mutation holds an exclusive lock
// across its read-modify-write, or two processes racing would each read the
// same list and the later save would silently discard the earlier change.
type JSONStore struct {
	path string

	// lockTimeout bounds the wait for the write lock; zero means
	// defaultLockTimeout. Tests shorten it to avoid slow failures.
	lockTimeout time.Duration
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
	return s.withWriteLock(func() error {
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
	})
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
	return s.withWriteLock(func() error {
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
	})
}

// Delete removes the task with the given ID.
func (s *JSONStore) Delete(id int64) error {
	return s.withWriteLock(func() error {
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
	})
}

// lockPath is the file whose lock serialises writers. It sits next to the data
// file so that one data file has exactly one lock, and it is a separate file
// because save replaces the data file with a rename: a lock held on the old
// inode would protect a file that no longer has a name.
func (s *JSONStore) lockPath() string {
	return s.path + ".lock"
}

// withWriteLock runs fn while holding an exclusive lock, so a read-modify-write
// cannot interleave with another tada process doing the same.
//
// Reads stay lock-free: save renames a complete file into place, so a reader
// sees either the old list or the new one, never a partial write.
//
// The lock is an OS advisory lock, so the kernel releases it when the process
// exits, however it exits. A killed process therefore cannot leave a stale
// lock behind that would deadlock later runs; only the empty lock file remains.
func (s *JSONStore) withWriteLock(fn func() error) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	lockPath := s.lockPath()
	timeout := s.lockTimeout
	if timeout <= 0 {
		timeout = defaultLockTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	lock := flock.New(lockPath)
	locked, err := lock.TryLockContext(ctx, lockRetryDelay)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("lock %s: %w", lockPath, err)
	}
	if !locked {
		return fmt.Errorf("%w after %s: another tada process is writing to %s",
			ErrLockTimeout, timeout, s.path)
	}

	// Unlock's error is not actionable: fn has already reported whether the
	// write itself succeeded.
	defer func() { _ = lock.Unlock() }()

	return fn()
}
