// internal/services/stats_service.go
package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageStats 表示API使用统计
type UsageStats struct {
	TodayRequests int            `json:"today_requests"`
	MonthlyTokens int            `json:"monthly_tokens"`
	DailyStats    map[string]int `json:"daily_stats"`
	MonthlyStats  map[string]int `json:"monthly_stats"`
	LastUpdated   time.Time      `json:"last_updated"`
}

// StatsService 提供API使用统计功能
type StatsService struct {
	BasePath    string      // 统计数据存储路径
	statsFile   string      // 统计文件名
	mutex       sync.Mutex  // 用于数据访问的互斥锁
	cachedStats *UsageStats // 缓存的统计数据
}

// NewStatsService 创建统计服务实例
func NewStatsService() *StatsService {
	basePath := "data/stats"

	// 确保统计数据目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("Warning: Failed to create stats directory: %v\n", err)
	}

	service := &StatsService{
		BasePath:  basePath,
		statsFile: filepath.Join(basePath, "usage_stats.json"),
		mutex:     sync.Mutex{},
		cachedStats: &UsageStats{
			TodayRequests: 0,
			MonthlyTokens: 0,
			DailyStats:    make(map[string]int),
			MonthlyStats:  make(map[string]int),
			LastUpdated:   time.Now(),
		},
	}

	// 初始化统计数据
	service.initStats()

	return service
}

// EnsureStatsFileExists 确保统计文件存在
func (s *StatsService) EnsureStatsFileExists() error {
	if _, err := os.Stat(s.statsFile); os.IsNotExist(err) {
		// 文件不存在，创建它
		return s.saveStats(s.cachedStats)
	}
	return nil
}

// initStats 初始化统计数据（带锁版本）
func (s *StatsService) initStats() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 调用无锁版本的初始化（它会正确设置 s.cachedStats）
	s.initStatsUnlocked()

	// 确保文件存在
	if err := s.EnsureStatsFileExists(); err != nil {
		fmt.Printf("警告: 创建统计文件失败: %v\n", err)
	}
}

// initStatsUnlocked 初始化统计数据（无锁版本）
func (s *StatsService) initStatsUnlocked() {
	// 尝试加载现有数据
	if _, err := os.Stat(s.statsFile); err == nil {
		if loadedStats, err := s.loadStats(); err == nil {
			// 检查是否需要重置每日计数
			today := time.Now().Format("2006-01-02")
			lastDate := loadedStats.LastUpdated.Format("2006-01-02")

			if today != lastDate {
				// 新的一天，重置每日统计
				loadedStats.TodayRequests = 0
				loadedStats.LastUpdated = time.Now()
			}

			// 检查是否需要重置月度统计
			thisMonth := time.Now().Format("2006-01")
			lastMonth := loadedStats.LastUpdated.Format("2006-01")

			if thisMonth != lastMonth {
				// 新的月份，重置月度统计
				loadedStats.MonthlyTokens = 0
			}

			s.cachedStats = loadedStats
			return
		}
	}

	// 如果加载失败或文件不存在，创建新的统计数据
	if s.cachedStats == nil {
		s.cachedStats = &UsageStats{
			TodayRequests: 0,
			MonthlyTokens: 0,
			DailyStats:    make(map[string]int),
			MonthlyStats:  make(map[string]int),
			LastUpdated:   time.Now(),
		}
	}
}

/*
// initStatsUnlocked 在已持有锁的情况下初始化统计数据
func (s *StatsService) initStatsUnlocked() {
	// 检查统计文件是否存在
	if _, err := os.Stat(s.statsFile); os.IsNotExist(err) {
		// 创建初始统计数据
		initialStats := &UsageStats{
			TodayRequests: 0,
			MonthlyTokens: 0,
			DailyStats:    make(map[string]int),
			MonthlyStats:  make(map[string]int),
			LastUpdated:   time.Now(),
		}

		// 保存初始数据
		if err := s.saveStats(initialStats); err != nil {
			fmt.Printf("Warning: Failed to save initial stats: %v\n", err)
		}

		s.cachedStats = initialStats
		return
	}

	// 读取现有统计数据
	stats, err := s.loadStats()
	if err != nil {
		fmt.Printf("Warning: Failed to load stats: %v\n", err)

		// 出错时创建新的统计数据
		s.cachedStats = &UsageStats{
			TodayRequests: 0,
			MonthlyTokens: 0,
			DailyStats:    make(map[string]int),
			MonthlyStats:  make(map[string]int),
			LastUpdated:   time.Now(),
		}
		return
	}

	// 检查是否需要重置每日计数
	today := time.Now().Format("2006-01-02")
	lastDate := stats.LastUpdated.Format("2006-01-02")

	if today != lastDate {
		stats.TodayRequests = 0
		stats.LastUpdated = time.Now()

		// 保存更新后的统计数据
		if err := s.saveStats(stats); err != nil {
			fmt.Printf("Warning: Failed to update stats for new day: %v\n", err)
		}
	}

	s.cachedStats = stats
}
*/
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

	return &stats, nil
}

// saveStats 保存统计数据到文件
func (s *StatsService) saveStats(stats *UsageStats) error {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize stats: %w", err)
	}

	if err := os.WriteFile(s.statsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write stats file: %w", err)
	}

	return nil
}

// GetUsageStats 获取API使用统计
func (s *StatsService) GetUsageStats() *UsageStats {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否需要初始化或刷新数据
	if s.cachedStats == nil {
		s.initStatsUnlocked() // 使用不带锁的版本
	}

	// 创建深度副本
	dailyStatsCopy := make(map[string]int)
	for k, v := range s.cachedStats.DailyStats {
		dailyStatsCopy[k] = v
	}

	monthlyStatsCopy := make(map[string]int)
	for k, v := range s.cachedStats.MonthlyStats {
		monthlyStatsCopy[k] = v
	}

	// 返回完全独立的副本
	return &UsageStats{
		TodayRequests: s.cachedStats.TodayRequests,
		MonthlyTokens: s.cachedStats.MonthlyTokens,
		DailyStats:    dailyStatsCopy,
		MonthlyStats:  monthlyStatsCopy,
		LastUpdated:   s.cachedStats.LastUpdated,
	}
}

// RecordAPIRequest 记录API请求
func (s *StatsService) RecordAPIRequest(tokens int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 确保统计数据已初始化
	if s.cachedStats == nil {
		s.initStatsUnlocked() // 使用不带锁的版本
	}

	// 更新今日请求计数
	s.cachedStats.TodayRequests++

	// 更新本月token使用量
	s.cachedStats.MonthlyTokens += tokens

	// 更新日期统计
	today := time.Now().Format("2006-01-02")
	s.cachedStats.DailyStats[today] = s.cachedStats.DailyStats[today] + 1

	// 更新月度统计
	month := time.Now().Format("2006-01")
	s.cachedStats.MonthlyStats[month] = s.cachedStats.MonthlyStats[month] + tokens

	// 更新最后更新时间
	s.cachedStats.LastUpdated = time.Now()

	// 保存更新后的统计数据
	return s.saveStats(s.cachedStats)
}

// ResetStats 重置统计数据（仅用于测试或管理目的）
func (s *StatsService) ResetStats() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 创建新的统计数据
	newStats := &UsageStats{
		TodayRequests: 0,
		MonthlyTokens: 0,
		DailyStats:    make(map[string]int),
		MonthlyStats:  make(map[string]int),
		LastUpdated:   time.Now(),
	}

	// 保存新的统计数据
	if err := s.saveStats(newStats); err != nil {
		return err
	}

	// 更新缓存
	s.cachedStats = newStats
	return nil
}
