package model

import (
	"errors"
	"fmt"
	"time"
)

// Task represents a todo task.
type Task struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	Priority  int    `json:"priority"`
	DueDate   string `json:"due_date"`
}

// NewTask validates and creates a new Task.
func NewTask(title string, priority int, due string) (*Task, error) {
	if title == "" {
		return nil, errors.New("title cannot be empty")
	}
	if priority < 0 || priority > 2 {
		return nil, fmt.Errorf("priority %d out of range", priority)
	}
	if due != "" {
		if _, err := time.Parse("2006-01-02", due); err != nil {
			return nil, fmt.Errorf("invalid due date: %w", err)
		}
	}
	return &Task{Title: title, Priority: priority, DueDate: due}, nil
}
