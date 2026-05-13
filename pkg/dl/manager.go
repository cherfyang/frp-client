package dl

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type TaskState int

const (
	TaskPending TaskState = iota
	TaskDownloading
	TaskDone
	TaskError
	TaskCanceled
)

func (s TaskState) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskDownloading:
		return "downloading"
	case TaskDone:
		return "done"
	case TaskError:
		return "error"
	case TaskCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

type TaskOptions struct {
	Connections int `json:"connections"`
	MaxRetries  int `json:"maxRetries"`
}

type TaskInfo struct {
	ID       string       `json:"id"`
	URL      string       `json:"url"`
	DestPath string       `json:"destPath"`
	State    string       `json:"state"`
	Progress ProgressInfo `json:"progress"`
}

type Manager struct {
	mu    sync.Mutex
	tasks map[string]*Task
	seq   int64
}

func NewManager() *Manager {
	return &Manager{tasks: make(map[string]*Task)}
}

func (m *Manager) Start(url, destPath string, opts TaskOptions) (string, error) {
	if opts.Connections <= 0 {
		opts.Connections = 4
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 3
	}
	if opts.Connections > 32 {
		opts.Connections = 32
	}
	if opts.MaxRetries > 8 {
		opts.MaxRetries = 8
	}

	id := fmt.Sprintf("dl_%d", atomic.AddInt64(&m.seq, 1))
	task := newTask(id, url, destPath, opts)

	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	go task.run()

	return id, nil
}

func (m *Manager) GetProgress(id string) (ProgressInfo, bool) {
	m.mu.Lock()
	task, ok := m.tasks[id]
	m.mu.Unlock()
	if !ok {
		return ProgressInfo{}, false
	}
	return task.progress.Snapshot(), true
}

func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	task, ok := m.tasks[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	task.cancel()
	return nil
}

func (m *Manager) List() []TaskInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]TaskInfo, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t.info())
	}
	return result
}

func (m *Manager) RemoveCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.tasks {
		t.mu.Lock()
		state := t.state
		t.mu.Unlock()
		if state == TaskDone || state == TaskError || state == TaskCanceled {
			delete(m.tasks, id)
		}
	}
}
