package model

import "testing"

func TestNewTask(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		priority int
		due      string
		wantErr  bool
	}{
		{"valid", "Task", 1, "2026-12-31", false},
		{"empty title", "", 1, "", true},
		{"bad priority", "Task", 5, "", true},
		{"bad date", "Task", 0, "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTask(tt.title, tt.priority, tt.due)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
