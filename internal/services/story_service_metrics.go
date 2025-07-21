// internal/services/story_service_metrics.go
package services

import (
	"sync"
	"time"
)

// StoryServiceMetrics 故事服务性能指标
type StoryServiceMetrics struct {
	mutex                sync.RWMutex
	totalChoices         int64
	averageChoiceTime    time.Duration
	concurrentOperations int32
	cacheHitRate         float64
	lastMetricsReset     time.Time
}

// RecordChoice 记录选择操作
func (m *StoryServiceMetrics) RecordChoice(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.totalChoices++
	m.averageChoiceTime = (m.averageChoiceTime*time.Duration(m.totalChoices-1) + duration) / time.Duration(m.totalChoices)
}

// GetMetrics 获取性能指标
func (m *StoryServiceMetrics) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_choices":         m.totalChoices,
		"average_choice_time":   m.averageChoiceTime.Milliseconds(),
		"concurrent_operations": m.concurrentOperations,
		"cache_hit_rate":        m.cacheHitRate,
		"last_reset":            m.lastMetricsReset,
	}
}

// ResetMetrics 重置性能指标
func (m *StoryServiceMetrics) ResetMetrics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.totalChoices = 0
	m.averageChoiceTime = 0
	m.concurrentOperations = 0
	m.cacheHitRate = 0
	m.lastMetricsReset = time.Now()
}
