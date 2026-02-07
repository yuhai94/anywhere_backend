package scheduler

import (
	"context"
	"sync"

	"github.com/yuhai94/anywhere_backend/internal/logging"
)

// Task 定义定时任务接口
type Task interface {
	Name() string
	Start(ctx context.Context)
	Stop()
}

// Scheduler 任务管理器
type Scheduler struct {
	tasks   map[string]Task
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
}

// NewScheduler 创建新的任务管理器
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		tasks:  make(map[string]Task),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Register 注册任务
func (s *Scheduler) Register(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.Name()] = task
	logging.Info(s.ctx, "Registered task: %s", task.Name())
}

// Start 启动所有任务
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, task := range s.tasks {
		logging.Info(s.ctx, "Starting task: %s", name)
		s.wg.Add(1)
		go func(t Task) {
			defer s.wg.Done()
			t.Start(s.ctx)
		}(task)
	}

	logging.Info(s.ctx, "All tasks started")
}

// Stop 停止所有任务
func (s *Scheduler) Stop() {
	logging.Info(s.ctx, "Stopping scheduler and all tasks")

	// 取消上下文，通知所有任务停止
	s.cancel()

	// 等待所有任务结束
	s.wg.Wait()

	// 显式停止每个任务
	s.mu.Lock()
	for name, task := range s.tasks {
		logging.Info(s.ctx, "Stopping task: %s", name)
		task.Stop()
	}
	s.mu.Unlock()

	logging.Info(s.ctx, "All tasks stopped")
}

// GetTask 获取指定任务
func (s *Scheduler) GetTask(name string) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.tasks[name]
}

