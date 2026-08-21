package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Task struct {
	ID      int       `json:"id,omitempty"`
	Text    string    `json:"text,omitempty"`
	Tags    []string  `json:"tags,omitempty"`
	Created time.Time `json:"created"`
	Status  Status    `json:"status,omitempty"`
}

func (t *Task) String() string {
	return fmt.Sprintf("%d: %s (%s, %s) [%s]", t.ID, t.Text, t.Created.Format("2006-01-02"), t.Status, strings.Join(t.Tags, ","))
}

type TaskList struct {
	Tasks []Task `json:"tasks,omitempty"`
}

func (l *TaskList) LatestID() int {
	var id int
	for _, t := range l.Tasks {
		if t.ID > id {
			id = t.ID
		}
	}
	return id
}

type Status string

const (
	Pending Status = "pending"
	Done    Status = "done"
)

func (l *TaskList) Add(t Task) {
	l.Tasks = append(l.Tasks, t)
}

func (l *TaskList) Remove(id int) error {
	for i, t := range l.Tasks {
		if t.ID == id {
			l.Tasks = append(l.Tasks[:i], l.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task with ID %d not found", id)
}

func (l *TaskList) List() []Task {
	return l.Tasks
}

func (l *TaskList) ListToday() []Task {
	var tasks []Task
	for _, t := range l.Tasks {
		if t.Created.Day() == time.Now().Day() {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (l *TaskList) ListOverdue() []Task {
	var tasks []Task
	for _, t := range l.Tasks {
		if t.Created.Before(time.Now()) && t.Status == Pending {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (l *TaskList) Done(id int) error {
	for i, t := range l.Tasks {
		if t.ID == id {
			t.Status = Done
			l.Tasks[i] = t
			return nil
		}
	}
	return fmt.Errorf("task with ID %d not found", id)
}

func (l *TaskList) Find(id int) (Task, bool) {
	for _, t := range l.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

func (l *TaskList) FindByTag(tag string) []Task {
	var tasks []Task
	for _, task := range l.Tasks {
		for _, t := range task.Tags {
			if t == tag {
				tasks = append(tasks, task)
			}
		}
	}
	return tasks
}

func Save(file string, l *TaskList) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task list: %w", err)
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return os.WriteFile(file, data, 0644)
}

func Load(file string) (*TaskList, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			l := &TaskList{Tasks: []Task{}}
			if err := Save(file, l); err != nil {
				return nil, fmt.Errorf("failed to save file %s: %w", file, err)
			}
			return l, nil
		}
		return nil, fmt.Errorf("failed to read file %s: %w", file, err)
	}
	var l *TaskList
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task list: %w", err)
	}
	return l, nil
}
