// internal/services/Progress_service.go
package services

import (
	"fmt"
	"sync"
	"time"
)

// ProgressUpdate 表示进度更新
type ProgressUpdate struct {
	Progress int    `json:"progress"` // 进度百分比 (0-100)
	Message  string `json:"message"`  // 描述性消息
	Status   string `json:"status"`   // 状态：running, completed, failed
}

// ProgressTracker 跟踪长时间运行任务的进度
type ProgressTracker struct {
	TaskID      string                       // 任务唯一标识符
	Progress    int                          // 进度百分比 (0-100)
	Message     string                       // 当前状态描述
	Status      string                       // 状态：running, completed, failed
	StartTime   time.Time                    // 开始时间
	UpdateTime  time.Time                    // 最后更新时间
	Subscribers map[chan ProgressUpdate]bool // 订阅进度更新的通道
	Done        chan struct{}                // 任务完成信号
	mutex       sync.Mutex                   // 保护并发访问
	StatusMsg   string
}

// ProgressService 管理所有进度跟踪器
type ProgressService struct {
	trackers map[string]*ProgressTracker
	mutex    sync.RWMutex
}

// NewProgressService 创建进度服务实例
func NewProgressService() *ProgressService {
	return &ProgressService{
		trackers: make(map[string]*ProgressTracker),
	}
}

// CreateTracker 创建新的进度跟踪器
func (s *ProgressService) CreateTracker(taskID string) *ProgressTracker {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 如果已存在，返回现有追踪器
	if tracker, exists := s.trackers[taskID]; exists {
		return tracker
	}

	tracker := &ProgressTracker{
		TaskID:      taskID,
		Progress:    0,
		Message:     "任务初始化中...",
		Status:      "running",
		StartTime:   time.Now(),
		UpdateTime:  time.Now(),
		Subscribers: make(map[chan ProgressUpdate]bool),
		Done:        make(chan struct{}),
	}

	s.trackers[taskID] = tracker
	return tracker
}

// GetTracker 获取进度跟踪器
func (s *ProgressService) GetTracker(taskID string) (*ProgressTracker, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tracker, exists := s.trackers[taskID]
	return tracker, exists
}

// UpdateProgress 更新任务进度
func (t *ProgressTracker) UpdateProgress(progress int, message string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if progress > t.Progress {
		t.Progress = progress
	}
	if message != "" {
		t.Message = message
	}
	t.UpdateTime = time.Now()

	update := ProgressUpdate{
		Progress: t.Progress,
		Message:  t.Message,
		Status:   t.Status,
	}

	// 通知所有订阅者
	for subscriber := range t.Subscribers {
		// 非阻塞发送，如果通道已满则跳过
		select {
		case subscriber <- update:
		default:
		}
	}
}

// Complete 标记任务完成
func (t *ProgressTracker) Complete(message string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Progress = 100
	if message != "" {
		t.Message = message
	} else {
		t.Message = "任务已完成"
	}
	t.Status = "completed"
	t.UpdateTime = time.Now()

	// 通知所有订阅者
	update := ProgressUpdate{
		Progress: 100,
		Message:  t.Message,
		Status:   "completed",
	}

	for subscriber := range t.Subscribers {
		select {
		case subscriber <- update:
		default:
		}
	}

	// 通知Done通道
	close(t.Done)
}

// Fail 标记任务失败
func (t *ProgressTracker) Fail(errorMsg string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Message = fmt.Sprintf("任务失败: %s", errorMsg)
	t.Status = "failed"
	t.UpdateTime = time.Now()

	// 通知所有订阅者
	update := ProgressUpdate{
		Progress: t.Progress,
		Message:  t.Message,
		Status:   "failed",
	}

	for subscriber := range t.Subscribers {
		select {
		case subscriber <- update:
		default:
		}
	}

	// 通知Done通道
	close(t.Done)
}

// Subscribe 订阅进度更新
func (t *ProgressTracker) Subscribe() chan ProgressUpdate {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// 创建订阅通道，缓冲区设为10以避免阻塞
	subscriber := make(chan ProgressUpdate, 10)
	t.Subscribers[subscriber] = true

	// 立即发送当前状态
	subscriber <- ProgressUpdate{
		Progress: t.Progress,
		Message:  t.Message,
		Status:   t.Status,
	}

	return subscriber
}

// Unsubscribe 取消订阅
func (t *ProgressTracker) Unsubscribe(subscriber chan ProgressUpdate) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.Subscribers, subscriber)
	close(subscriber)
}

// CleanupCompletedTasks 清理已完成的任务
func (s *ProgressService) CleanupCompletedTasks(maxAge time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for id, tracker := range s.trackers {
		tracker.mutex.Lock()
		isCompleted := tracker.Status == "completed" || tracker.Status == "failed"
		isOld := now.Sub(tracker.UpdateTime) > maxAge
		tracker.mutex.Unlock()

		if isCompleted && isOld {
			delete(s.trackers, id)
		}
	}
}
