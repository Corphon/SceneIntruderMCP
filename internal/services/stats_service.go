// internal/services/stats_service.go
package services

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

// UsageStats 表示API使用统计
type UsageStats struct {
	TodayRequests int            `json:"today_requests"`
	MonthlyTokens int            `json:"monthly_tokens"`
	DailyStats    map[string]int `json:"daily_stats"`
	MonthlyStats  map[string]int `json:"monthly_stats"`

	// Vision stats (Phase5 monitoring). Optional for backward compatibility.
	TodayVisionRequests int `json:"today_vision_requests,omitempty"`
	TodayVisionFailures int `json:"today_vision_failures,omitempty"`

	VisionDailyRequests map[string]int `json:"vision_daily_requests,omitempty"`
	VisionDailyFailures map[string]int `json:"vision_daily_failures,omitempty"`

	VisionMonthlyRequests map[string]int `json:"vision_monthly_requests,omitempty"`
	VisionMonthlyFailures map[string]int `json:"vision_monthly_failures,omitempty"`

	// Aggregated latency (ms) for today's vision requests.
	TodayVisionLatencyCount int64 `json:"today_vision_latency_count,omitempty"`
	TodayVisionLatencySumMs int64 `json:"today_vision_latency_sum_ms,omitempty"`
	TodayVisionLatencyMinMs int64 `json:"today_vision_latency_min_ms,omitempty"`
	TodayVisionLatencyMaxMs int64 `json:"today_vision_latency_max_ms,omitempty"`

	LastUpdated time.Time `json:"last_updated"`
}

// StatsService 提供API使用统计功能
type StatsService struct {
	BasePath    string      // 统计数据存储路径
	statsFile   string      // 统计文件名
	mutex       sync.Mutex  // 用于数据访问的互斥锁
	cachedStats *UsageStats // 缓存的统计数据

	// 缓存字段
	lastCheckDate  string
	lastCheckMonth string
	lastCheckTime  time.Time

	// 批量保存控制
	isDirty      bool
	lastSaveTime time.Time
	saveInterval time.Duration
}

// ------------------------------------
// NewStatsService 创建统计服务实例
func NewStatsService() *StatsService {
	basePath := "data/stats"

	// 确保统计数据目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		utils.GetLogger().Warn("创建统计目录失败", map[string]interface{}{"base_path": basePath, "err": err})
	}

	service := &StatsService{
		BasePath:     basePath,
		statsFile:    filepath.Join(basePath, "usage_stats.json"),
		mutex:        sync.Mutex{},
		cachedStats:  nil,
		saveInterval: 30 * time.Second,
	}

	// 初始化统计数据
	service.startPeriodicSave()

	return service
}

// initStatsUnlocked 初始化统计数据（无锁版本）
func (s *StatsService) initStatsUnlocked() {
	// 尝试加载现有数据
	if loadedStats, err := s.loadStatsFromFile(); err == nil {
		// 检查并重置过期的统计数据
		s.updateStatsForNewPeriod(loadedStats)
		s.cachedStats = loadedStats

		// 确保文件存在（如果加载成功，文件肯定存在，无需重复检查）
		return
	}

	// 加载失败或文件不存在，创建新的统计数据
	s.cachedStats = &UsageStats{
		TodayRequests:           0,
		MonthlyTokens:           0,
		DailyStats:              make(map[string]int),
		MonthlyStats:            make(map[string]int),
		TodayVisionRequests:     0,
		TodayVisionFailures:     0,
		VisionDailyRequests:     make(map[string]int),
		VisionDailyFailures:     make(map[string]int),
		VisionMonthlyRequests:   make(map[string]int),
		VisionMonthlyFailures:   make(map[string]int),
		TodayVisionLatencyCount: 0,
		TodayVisionLatencySumMs: 0,
		TodayVisionLatencyMinMs: 0,
		TodayVisionLatencyMaxMs: 0,
		LastUpdated:             time.Now(),
	}

	// 保存初始数据
	if err := s.saveStats(s.cachedStats); err != nil {
		utils.GetLogger().Warn("保存初始统计数据失败", map[string]interface{}{"stats_file": s.statsFile, "err": err})
	}
}

// 分离文件加载逻辑
func (s *StatsService) loadStatsFromFile() (*UsageStats, error) {
	// 检查文件是否存在
	if _, err := os.Stat(s.statsFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("统计文件不存在")
	}

	// 加载文件内容
	return s.loadStats()
}

// 分离时间段更新逻辑
func (s *StatsService) updateStatsForNewPeriod(stats *UsageStats) {
	now := time.Now()
	today := now.Format("2006-01-02")
	thisMonth := now.Format("2006-01")

	lastDate := stats.LastUpdated.Format("2006-01-02")
	lastMonth := stats.LastUpdated.Format("2006-01")

	updated := false

	// 检查是否需要重置每日计数
	if today != lastDate {
		stats.TodayRequests = 0
		stats.TodayVisionRequests = 0
		stats.TodayVisionFailures = 0
		stats.TodayVisionLatencyCount = 0
		stats.TodayVisionLatencySumMs = 0
		stats.TodayVisionLatencyMinMs = 0
		stats.TodayVisionLatencyMaxMs = 0
		updated = true
	}

	// 检查是否需要重置月度统计
	if thisMonth != lastMonth {
		stats.MonthlyTokens = 0
		updated = true
	}

	// 如果有更新，保存到文件
	if updated {
		stats.LastUpdated = now
		if err := s.saveStats(stats); err != nil {
			utils.GetLogger().Warn("更新时间段统计失败", map[string]interface{}{"stats_file": s.statsFile, "err": err})
		}
	}
}

// loadStats 从文件加载统计数据
func (s *StatsService) loadStats() (*UsageStats, error) {
	data, err := os.ReadFile(s.statsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read stats file: %w", err)
	}

	var stats UsageStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats data: %w", err)
	}

	// 确保映射已初始化
	if stats.DailyStats == nil {
		stats.DailyStats = make(map[string]int)
	}
	if stats.MonthlyStats == nil {
		stats.MonthlyStats = make(map[string]int)
	}
	if stats.VisionDailyRequests == nil {
		stats.VisionDailyRequests = make(map[string]int)
	}
	if stats.VisionDailyFailures == nil {
		stats.VisionDailyFailures = make(map[string]int)
	}
	if stats.VisionMonthlyRequests == nil {
		stats.VisionMonthlyRequests = make(map[string]int)
	}
	if stats.VisionMonthlyFailures == nil {
		stats.VisionMonthlyFailures = make(map[string]int)
	}

	return &stats, nil
}

// saveStats 保存统计数据到文件
func (s *StatsService) saveStats(stats *UsageStats) error {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize stats: %w", err)
	}

	// 使用临时文件确保原子性写入
	tempFile := s.statsFile + ".tmp"

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp stats file: %w", err)
	}

	// 原子性重命名
	if err := os.Rename(tempFile, s.statsFile); err != nil {
		os.Remove(tempFile) // 清理临时文件
		return fmt.Errorf("failed to replace stats file: %w", err)
	}

	return nil
}

// GetUsageStats 获取API使用统计
func (s *StatsService) GetUsageStats() *UsageStats {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 确保统计数据已初始化
	if s.cachedStats == nil {
		s.initStatsUnlocked()
	}

	// 🔧 使用缓存的时间段检查，减少频繁的时间比较
	if s.needsPeriodUpdate() {
		s.updateStatsForCurrentPeriod()
	}

	// 返回深度副本
	return s.createStatsCopy()
}

// 高效的时间段检查
func (s *StatsService) needsPeriodUpdate() bool {
	now := time.Now()

	// 如果距离上次检查不到10分钟，跳过检查
	if now.Sub(s.lastCheckTime) < 10*time.Minute {
		return false
	}

	s.lastCheckTime = now
	currentDate := now.Format("2006-01-02")
	currentMonth := now.Format("2006-01")

	needsUpdate := currentDate != s.lastCheckDate || currentMonth != s.lastCheckMonth

	if needsUpdate {
		s.lastCheckDate = currentDate
		s.lastCheckMonth = currentMonth
	}

	return needsUpdate
}

// 当前时间段的更新
func (s *StatsService) updateStatsForCurrentPeriod() {
	if s.cachedStats == nil {
		return
	}

	s.updateStatsForNewPeriod(s.cachedStats)
}

// createStatsCopy 创建统计数据的深度副本
func (s *StatsService) createStatsCopy() *UsageStats {
	if s.cachedStats == nil {
		return &UsageStats{
			TodayRequests:         0,
			MonthlyTokens:         0,
			DailyStats:            make(map[string]int),
			MonthlyStats:          make(map[string]int),
			VisionDailyRequests:   make(map[string]int),
			VisionDailyFailures:   make(map[string]int),
			VisionMonthlyRequests: make(map[string]int),
			VisionMonthlyFailures: make(map[string]int),
			LastUpdated:           time.Now(),
		}
	}

	return &UsageStats{
		TodayRequests:           s.cachedStats.TodayRequests,
		MonthlyTokens:           s.cachedStats.MonthlyTokens,
		DailyStats:              copyIntMap(s.cachedStats.DailyStats),
		MonthlyStats:            copyIntMap(s.cachedStats.MonthlyStats),
		TodayVisionRequests:     s.cachedStats.TodayVisionRequests,
		TodayVisionFailures:     s.cachedStats.TodayVisionFailures,
		VisionDailyRequests:     copyIntMap(s.cachedStats.VisionDailyRequests),
		VisionDailyFailures:     copyIntMap(s.cachedStats.VisionDailyFailures),
		VisionMonthlyRequests:   copyIntMap(s.cachedStats.VisionMonthlyRequests),
		VisionMonthlyFailures:   copyIntMap(s.cachedStats.VisionMonthlyFailures),
		TodayVisionLatencyCount: s.cachedStats.TodayVisionLatencyCount,
		TodayVisionLatencySumMs: s.cachedStats.TodayVisionLatencySumMs,
		TodayVisionLatencyMinMs: s.cachedStats.TodayVisionLatencyMinMs,
		TodayVisionLatencyMaxMs: s.cachedStats.TodayVisionLatencyMaxMs,
		LastUpdated:             s.cachedStats.LastUpdated,
	}
}

// RecordVisionRequest 记录一次 Vision provider 请求的统计（计数 + 失败数 + 今日时延聚合）。
func (s *StatsService) RecordVisionRequest(provider string, modelKey string, duration time.Duration, err error) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cachedStats == nil {
		s.initStatsUnlocked()
	}
	if s.needsPeriodUpdate() {
		s.updateStatsForCurrentPeriod()
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	month := now.Format("2006-01")

	// Ensure maps are initialized for backward compatibility.
	if s.cachedStats.VisionDailyRequests == nil {
		s.cachedStats.VisionDailyRequests = make(map[string]int)
	}
	if s.cachedStats.VisionDailyFailures == nil {
		s.cachedStats.VisionDailyFailures = make(map[string]int)
	}
	if s.cachedStats.VisionMonthlyRequests == nil {
		s.cachedStats.VisionMonthlyRequests = make(map[string]int)
	}
	if s.cachedStats.VisionMonthlyFailures == nil {
		s.cachedStats.VisionMonthlyFailures = make(map[string]int)
	}

	s.cachedStats.TodayVisionRequests++
	s.cachedStats.VisionDailyRequests[today]++
	s.cachedStats.VisionMonthlyRequests[month]++

	ms := duration.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	s.cachedStats.TodayVisionLatencyCount++
	s.cachedStats.TodayVisionLatencySumMs += ms
	if s.cachedStats.TodayVisionLatencyMinMs == 0 || ms < s.cachedStats.TodayVisionLatencyMinMs {
		s.cachedStats.TodayVisionLatencyMinMs = ms
	}
	if ms > s.cachedStats.TodayVisionLatencyMaxMs {
		s.cachedStats.TodayVisionLatencyMaxMs = ms
	}

	if err != nil {
		s.cachedStats.TodayVisionFailures++
		s.cachedStats.VisionDailyFailures[today]++
		s.cachedStats.VisionMonthlyFailures[month]++
	}

	// provider/modelKey currently unused in persistent stats to keep schema minimal.
	_ = provider
	_ = modelKey

	s.cachedStats.LastUpdated = now
	s.isDirty = true
	if now.Sub(s.lastSaveTime) > s.saveInterval {
		return s.saveStatsImmediate()
	}
	return nil
}

// 简化的映射复制
func copyIntMap(original map[string]int) map[string]int {
	if original == nil {
		return make(map[string]int)
	}

	copy := make(map[string]int, len(original))
	maps.Copy(copy, original)
	return copy
}

// RecordAPIRequest 记录API请求
func (s *StatsService) RecordAPIRequest(tokens int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 确保统计数据已初始化
	if s.cachedStats == nil {
		s.initStatsUnlocked()
	}

	// 一次性获取当前时间，避免重复调用
	now := time.Now()
	today := now.Format("2006-01-02")
	month := now.Format("2006-01")

	// 更新统计数据
	s.cachedStats.TodayRequests++
	s.cachedStats.MonthlyTokens += tokens
	s.cachedStats.DailyStats[today]++
	s.cachedStats.MonthlyStats[month] += tokens
	s.cachedStats.LastUpdated = now

	// 标记为需要保存，但不立即保存
	s.isDirty = true

	// 只在必要时立即保存（如数据重要或间隔太长）
	if now.Sub(s.lastSaveTime) > s.saveInterval {
		return s.saveStatsImmediate()
	}

	return nil
}

// 立即保存（私有方法）
func (s *StatsService) saveStatsImmediate() error {
	if !s.isDirty {
		return nil
	}

	err := s.saveStats(s.cachedStats)
	if err == nil {
		s.isDirty = false
		s.lastSaveTime = time.Now()
	}
	return err
}

// 定时保存机制
func (s *StatsService) startPeriodicSave() {
	go func() {
		ticker := time.NewTicker(s.saveInterval)
		defer ticker.Stop()

		for range ticker.C {
			s.mutex.Lock()
			if s.isDirty {
				if err := s.saveStatsImmediate(); err != nil {
					utils.GetLogger().Warn("定时保存统计数据失败", map[string]interface{}{"stats_file": s.statsFile, "err": err})
				}
			}
			s.mutex.Unlock()
		}
	}()
}

// ResetStats 重置统计数据（仅用于测试或管理目的）
func (s *StatsService) ResetStats() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 创建新的统计数据
	newStats := &UsageStats{
		TodayRequests:           0,
		MonthlyTokens:           0,
		DailyStats:              make(map[string]int),
		MonthlyStats:            make(map[string]int),
		TodayVisionRequests:     0,
		TodayVisionFailures:     0,
		VisionDailyRequests:     make(map[string]int),
		VisionDailyFailures:     make(map[string]int),
		VisionMonthlyRequests:   make(map[string]int),
		VisionMonthlyFailures:   make(map[string]int),
		TodayVisionLatencyCount: 0,
		TodayVisionLatencySumMs: 0,
		TodayVisionLatencyMinMs: 0,
		TodayVisionLatencyMaxMs: 0,
		LastUpdated:             time.Now(),
	}

	// 保存新的统计数据
	if err := s.saveStats(newStats); err != nil {
		return err
	}

	// 更新缓存
	s.cachedStats = newStats
	return nil
}

// 关闭方法，确保数据保存
func (s *StatsService) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 保存任何未保存的数据
	if s.isDirty {
		return s.saveStatsImmediate()
	}
	return nil
}
