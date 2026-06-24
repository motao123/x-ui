package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TaskService struct{}

type Task struct {
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Logs      []string `json:"logs"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
	mu        sync.Mutex
}

var taskStore = struct {
	sync.Mutex
	items map[string]*Task
}{items: map[string]*Task{}}

func (s *TaskService) Start(name string, fn func(task *Task)) *Task {
	now := time.Now().Unix()
	task := &Task{Id: uuid.NewString(), Name: name, Status: "running", CreatedAt: now, UpdatedAt: now}
	taskStore.Lock()
	taskStore.items[task.Id] = task
	taskStore.Unlock()
	task.Log("INFO", "任务开始")
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				task.Fail(fmt.Sprintf("任务异常: %v", recovered))
			}
		}()
		fn(task)
	}()
	return task
}

func (s *TaskService) Get(id string) (*Task, bool) {
	taskStore.Lock()
	defer taskStore.Unlock()
	task, ok := taskStore.items[id]
	return task, ok
}

func (s *TaskService) List() []*Task {
	taskStore.Lock()
	defer taskStore.Unlock()
	tasks := make([]*Task, 0, len(taskStore.items))
	for _, task := range taskStore.items {
		tasks = append(tasks, task.snapshot())
	}
	return tasks
}

func (t *Task) Log(level string, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.UpdatedAt = time.Now().Unix()
	t.Logs = append(t.Logs, time.Now().Format("2006-01-02 15:04:05")+" "+level+" "+message)
	if len(t.Logs) > 300 {
		t.Logs = t.Logs[len(t.Logs)-300:]
	}
}

func (t *Task) Done(message string) {
	t.mu.Lock()
	t.Status = "success"
	t.UpdatedAt = time.Now().Unix()
	t.mu.Unlock()
	t.Log("INFO", message)
}

func (t *Task) Fail(message string) {
	t.mu.Lock()
	t.Status = "failed"
	t.UpdatedAt = time.Now().Unix()
	t.mu.Unlock()
	t.Log("ERROR", message)
}

func (t *Task) snapshot() *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	logs := append([]string(nil), t.Logs...)
	return &Task{Id: t.Id, Name: t.Name, Status: t.Status, Logs: logs, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}
