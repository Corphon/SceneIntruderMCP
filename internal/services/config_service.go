// internal/services/config_service.go
package services

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
)

// ConfigService 提供配置管理功能
type ConfigService struct {
	// 缓存最近获取的配置，减少反复访问底层存储
	cachedConfig *config.AppConfig

	// 配置更新时间
	lastUpdated time.Time

	// 配置变更事件订阅者
	subscribers []ConfigChangeSubscriber

	// 配置历史记录
	changeHistory []ConfigChangeRecord

	// 互斥锁保护内部状态
	mu sync.RWMutex

	// 配置访问审计
	auditEnabled bool
	auditLog     []ConfigAuditEntry
}

// ConfigChangeSubscriber 配置变更订阅者接口
type ConfigChangeSubscriber interface {
	OnConfigChanged(oldConfig, newConfig *config.AppConfig)
}

// ConfigChangeRecord 配置变更记录
type ConfigChangeRecord struct {
	Timestamp time.Time
	ChangedBy string
	Section   string
	OldValue  interface{}
	NewValue  interface{}
}

// ConfigAuditEntry 配置访问审计条目
type ConfigAuditEntry struct {
	Timestamp time.Time
	Action    string // "read", "write"
	Section   string
	User      string // 可用于记录谁访问了配置
}

// NewConfigService 创建配置服务实例
func NewConfigService() *ConfigService {
	service := &ConfigService{
		lastUpdated:   time.Now(),
		subscribers:   make([]ConfigChangeSubscriber, 0),
		changeHistory: make([]ConfigChangeRecord, 0, 100), // 预分配容量
		auditEnabled:  false,
		auditLog:      make([]ConfigAuditEntry, 0, 100), // 预分配容量
	}

	// 初始化时加载配置到缓存
	service.cachedConfig = config.GetCurrentConfig()

	return service
}

// GetCurrentConfig 获取当前配置
func (s *ConfigService) GetCurrentConfig() *config.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 记录读取操作
	s.recordAudit("read", "全局配置", "system")

	// 如果缓存为空，从底层获取
	if s.cachedConfig == nil {
		s.cachedConfig = config.GetCurrentConfig()
	}

	return s.cachedConfig
}

// UpdateLLMConfig 更新LLM提供商和配置
func (s *ConfigService) UpdateLLMConfig(provider string, configMap map[string]string, changedBy string) error {
	s.mu.Lock()
	oldConfig := s.GetCurrentConfig()
	oldProvider := oldConfig.LLMProvider
	oldConfigMap := make(map[string]string)
	for k, v := range oldConfig.LLMConfig {
		oldConfigMap[k] = v
	}
	s.mu.Unlock()

	if provider == "" {
		return errors.New("provider cannot be empty")
	}

	// 确保必需的配置项存在
	if _, ok := configMap["api_key"]; !ok {
		log.Println("Warning: LLM config missing api_key")
	}

	// 确保有默认模型
	if _, ok := configMap["default_model"]; !ok {
		// 根据提供商设置默认模型
		switch provider {
		case "openai":
			configMap["default_model"] = "gpt-4o"
		case "anthropic":
			configMap["default_model"] = "claude-3.5-sonnet"
		case "google":
			configMap["default_model"] = "gemini-2.0-pro-exp"
		case "github":
			configMap["default_model"] = "o3-mini"
		case "":
			configMap["default_model"] = ""
		default:
			configMap["default_model"] = "gpt-4o" // 默认回退到OpenAI
		}
	}

	// 记录审计
	s.recordAudit("write", "LLM配置", changedBy)

	// 调用底层配置更新函数
	err := config.UpdateLLMConfig(provider, configMap)
	if err == nil {
		// 更新缓存
		s.mu.Lock()
		s.cachedConfig = config.GetCurrentConfig()
		newConfig := s.cachedConfig
		s.mu.Unlock()

		// 记录变更
		s.recordChange("LLM提供商", oldProvider, provider, changedBy)
		s.recordChange("LLM配置", oldConfigMap, configMap, changedBy)

		// 通知订阅者
		s.notifySubscribers(oldConfig, newConfig)
	}

	return err
}

// SaveConfig 保存当前配置
func (s *ConfigService) SaveConfig() error {
	return config.SaveConfig()
}

// GetLLMProvider 获取当前LLM提供商
func (s *ConfigService) GetLLMProvider() string {
	cfg := s.GetCurrentConfig()
	return cfg.LLMProvider
}

// GetLLMConfig 获取LLM配置
func (s *ConfigService) GetLLMConfig() map[string]string {
	cfg := s.GetCurrentConfig()
	return cfg.LLMConfig
}

// ValidateAPIKey 验证API密钥是否有效
// 返回验证结果和可能的错误信息
func (s *ConfigService) ValidateAPIKey(provider string, apiKey string) (bool, string) {
	if apiKey == "" {
		return false, "API key cannot be empty!"
	}

	// 这里可以实现真正的验证逻辑，例如发送一个简单请求到API
	// 现在简单返回true作为示例
	return true, ""
}

// SetDebugMode 设置调试模式
func (s *ConfigService) SetDebugMode(enabled bool) error {
	cfg := s.GetCurrentConfig()

	// 修改调试模式
	cfg.DebugMode = enabled

	// 保存配置
	return config.SaveConfig()
}

// SubscribeToChanges 订阅配置变更事件
func (s *ConfigService) SubscribeToChanges(subscriber ConfigChangeSubscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers = append(s.subscribers, subscriber)
}

// UnsubscribeFromChanges 取消配置变更订阅
func (s *ConfigService) UnsubscribeFromChanges(subscriber ConfigChangeSubscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, sub := range s.subscribers {
		if sub == subscriber {
			// 从订阅列表中移除
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			break
		}
	}
}

// notifySubscribers 通知所有订阅者配置已变更
func (s *ConfigService) notifySubscribers(oldConfig, newConfig *config.AppConfig) {
	s.mu.RLock()
	subscribers := make([]ConfigChangeSubscriber, len(s.subscribers))
	copy(subscribers, s.subscribers)
	s.mu.RUnlock()

	for _, subscriber := range subscribers {
		go subscriber.OnConfigChanged(oldConfig, newConfig)
	}
}

// GetChangeHistory 获取配置变更历史
func (s *ConfigService) GetChangeHistory(limit int) []ConfigChangeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.changeHistory) {
		limit = len(s.changeHistory)
	}

	// 返回最近的变更记录
	history := make([]ConfigChangeRecord, limit)
	startIdx := len(s.changeHistory) - limit
	copy(history, s.changeHistory[startIdx:])

	return history
}

// recordChange 记录配置变更
func (s *ConfigService) recordChange(section string, oldValue, newValue interface{}, changedBy string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := ConfigChangeRecord{
		Timestamp: time.Now(),
		ChangedBy: changedBy,
		Section:   section,
		OldValue:  oldValue,
		NewValue:  newValue,
	}

	// 限制历史记录数量，避免无限增长
	if len(s.changeHistory) >= 1000 {
		// 移除最旧的记录
		s.changeHistory = s.changeHistory[1:]
	}

	s.changeHistory = append(s.changeHistory, record)
}

// EnableAudit 启用配置访问审计
func (s *ConfigService) EnableAudit(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditEnabled = enabled
}

// GetAuditLog 获取配置访问审计日志
func (s *ConfigService) GetAuditLog(limit int) []ConfigAuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.auditEnabled {
		return nil
	}

	if limit <= 0 || limit > len(s.auditLog) {
		limit = len(s.auditLog)
	}

	// 返回最近的审计条目
	log := make([]ConfigAuditEntry, limit)
	startIdx := len(s.auditLog) - limit
	copy(log, s.auditLog[startIdx:])

	return log
}

// recordAudit 记录配置访问
func (s *ConfigService) recordAudit(action, section, user string) {
	if !s.auditEnabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := ConfigAuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		Section:   section,
		User:      user,
	}

	// 限制审计日志数量
	if len(s.auditLog) >= 1000 {
		s.auditLog = s.auditLog[1:]
	}

	s.auditLog = append(s.auditLog, entry)
}

// StartCacheRefresher 启动一个后台goroutine定期刷新配置缓存
func (s *ConfigService) StartCacheRefresher(refreshInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			s.mu.Lock()
			s.cachedConfig = config.GetCurrentConfig()
			s.lastUpdated = time.Now()
			s.mu.Unlock()
		}
	}()
}
