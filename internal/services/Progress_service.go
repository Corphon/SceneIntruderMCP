// internal/services/Progress_service.go
package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/utils"
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
}

// ProgressService 管理所有进度跟踪器
type ProgressService struct {
	trackers    map[string]*ProgressTracker
	mutex       sync.RWMutex
	cleanup     *time.Ticker
	stopCleanup chan struct{}
}

func safeCloseProgressUpdateChan(ch chan ProgressUpdate) {
	defer func() {
		_ = recover() // closing a closed channel panics
	}()
	close(ch)
}

// NewProgressService 创建进度服务实例
func NewProgressService() *ProgressService {
	return &ProgressService{
		trackers:    make(map[string]*ProgressTracker),
		stopCleanup: make(chan struct{}),
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

	t.notifySubscribers(update, false)
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

	update := ProgressUpdate{
		Progress: 100,
		Message:  t.Message,
		Status:   "completed",
	}

	t.notifySubscribers(update, true)

	// 安全关闭 Done 通道
	select {
	case <-t.Done:
	default:
		close(t.Done)
	}
}

// 自动清理机制
func (s *ProgressService) StartAutoCleanup() {
	s.cleanup = time.NewTicker(10 * time.Minute)
	go func() {
		defer s.cleanup.Stop()
		for {
			select {
			case <-s.cleanup.C:
				s.CleanupCompletedTasks(30 * time.Minute)
				// Also cleanup abandoned tasks that have been running too long (e.g., 2 hours)
				s.CleanupAbandonedTrackers(2 * time.Hour)
			case <-s.stopCleanup:
				return
			}
		}
	}()
}

// Stop 停止自动清理
func (s *ProgressService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 安全检查，防止重复关闭
	if s.stopCleanup != nil {
		select {
		case <-s.stopCleanup:
			// 通道已经关闭，不需要再次关闭
		default:
			close(s.stopCleanup)
			s.stopCleanup = nil // 设置为 nil 防止重复关闭
		}
	}

	// 停止清理 ticker
	if s.cleanup != nil {
		s.cleanup.Stop()
		s.cleanup = nil
	}
}

// Fail 标记任务失败
func (t *ProgressTracker) Fail(errorMsg string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Message = fmt.Sprintf("任务失败: %s", errorMsg)
	t.Status = "failed"
	t.UpdateTime = time.Now()

	update := ProgressUpdate{
		Progress: t.Progress,
		Message:  t.Message,
		Status:   "failed",
	}

	t.notifySubscribers(update, true)

	// 安全关闭 Done 通道
	select {
	case <-t.Done:
	default:
		close(t.Done)
	}
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

	// 检查订阅者是否仍在列表中
	if _, exists := t.Subscribers[subscriber]; exists {
		delete(t.Subscribers, subscriber)
		// 关闭时不要尝试从通道读取（会吞掉缓冲中的最后一条进度，导致前端卡住）
		safeCloseProgressUpdateChan(subscriber)
	}
}

// CleanupCompletedTasks 清理已完成的任务
func (s *ProgressService) CleanupCompletedTasks(maxAge time.Duration) {
	now := time.Now()
	var toDelete []string

	// 第一阶段：只读取，避免嵌套锁
	s.mutex.RLock()
	for id, tracker := range s.trackers {
		tracker.mutex.Lock()
		isCompleted := tracker.Status == "completed" || tracker.Status == "failed"
		isOld := now.Sub(tracker.UpdateTime) > maxAge
		tracker.mutex.Unlock()

		if isCompleted && isOld {
			toDelete = append(toDelete, id)
		}
	}
	s.mutex.RUnlock()

	// 第二阶段：批量删除
	if len(toDelete) > 0 {
		s.mutex.Lock()
		for _, id := range toDelete {
			delete(s.trackers, id)
		}
		s.mutex.Unlock()

		utils.GetLogger().Info("progress cleanup completed tasks", map[string]interface{}{
			"count": len(toDelete),
		})
	}
}

// CleanupAbandonedTrackers 清理被遗弃的跟踪器（长时间未更新的运行中任务）
func (s *ProgressService) CleanupAbandonedTrackers(maxAge time.Duration) {
	now := time.Now()
	var toDelete []string

	// 第一阶段：只读取，避免嵌套锁
	s.mutex.RLock()
	for id, tracker := range s.trackers {
		tracker.mutex.Lock()
		isRunning := tracker.Status == "running"
		isOld := now.Sub(tracker.UpdateTime) > maxAge
		tracker.mutex.Unlock()

		if isRunning && isOld {
			// 标记为失败并将其加入删除列表
			if t, exists := s.GetTracker(id); exists {
				t.Fail("任务超时: 长时间未收到更新")
			}
			toDelete = append(toDelete, id)
		}
	}
	s.mutex.RUnlock()

	// 第二阶段：批量删除
	if len(toDelete) > 0 {
		s.mutex.Lock()
		for _, id := range toDelete {
			delete(s.trackers, id)
		}
		s.mutex.Unlock()

		utils.GetLogger().Info("progress cleanup abandoned trackers", map[string]interface{}{
			"count": len(toDelete),
		})
	}
}

// 提取通用通知方法
func (t *ProgressTracker) notifySubscribers(update ProgressUpdate, closeChannels bool) {
	droppedCount := 0

	for subscriber := range t.Subscribers {
		select {
		case subscriber <- update:
		default:
			droppedCount++ // 简单计数，避免过多日志
		}

		if closeChannels {
			// 关闭时不要读取通道：读会消费掉缓冲消息（可能就是 100% completed）
			safeCloseProgressUpdateChan(subscriber)
		}
	}

	// 只在有丢弃消息时记录一次日志
	if droppedCount > 0 {
		utils.GetLogger().Warn("progress update dropped (subscriber channels full)", map[string]interface{}{
			"dropped": droppedCount,
		})
	}

	if closeChannels {
		t.Subscribers = make(map[chan ProgressUpdate]bool)
	}
}

// GetActiveTaskCount 获取当前正在运行的任务数量
func (s *ProgressService) GetActiveTaskCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	count := 0
	for _, tracker := range s.trackers {
		tracker.mutex.Lock()
		if tracker.Status == "running" {
			count++
		}
		tracker.mutex.Unlock()
	}

	return count
}
