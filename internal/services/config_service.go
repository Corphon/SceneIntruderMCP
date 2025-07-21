// internal/services/config_service.go
package services

import (
	"errors"
	"fmt"
	"log"
	"maps"
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

	// 版本控制防止并发更新冲突
	configVersion int64

	// 通知统计
	notificationStats struct {
		totalSent   int64
		totalFailed int64
		lastSent    time.Time
	}

	// 停止控制
	stopRefresher chan struct{}
	refresherDone chan struct{}
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

// ConfigTransaction 配置事务
type ConfigTransaction struct {
	service    *ConfigService
	changes    []ConfigChange
	committed  bool
	rollbackFn func() error
}

// ConfigChange 配置变更
type ConfigChange struct {
	Type     string // "llm_config", "debug_mode", etc.
	OldValue interface{}
	NewValue interface{}
}

// ConfigHealthCheck 配置健康检查
type ConfigHealthCheck struct {
	service *ConfigService
}

// --------------------------------------
// NewConfigService 创建配置服务实例
func NewConfigService() *ConfigService {
	service := &ConfigService{
		lastUpdated:   time.Now(),
		subscribers:   make([]ConfigChangeSubscriber, 0),
		changeHistory: make([]ConfigChangeRecord, 0, 100), // 预分配容量
		auditEnabled:  false,
		auditLog:      make([]ConfigAuditEntry, 0, 100), // 预分配容量
		stopRefresher: make(chan struct{}),
		refresherDone: make(chan struct{}),
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
	s.recordAuditUnsafe("read", "全局配置", "system")

	// 如果缓存为空，从底层获取
	if s.cachedConfig == nil {
		s.cachedConfig = config.GetCurrentConfig()
	}

	return s.cachedConfig
}

// getCurrentConfigUnsafe 获取当前配置（不安全版本，调用者需要持有锁）
func (s *ConfigService) getCurrentConfigUnsafe() *config.AppConfig {
	// 记录读取操作
	s.recordAuditUnsafe("read", "全局配置", "system")

	// 如果缓存为空，从底层获取
	if s.cachedConfig == nil {
		s.cachedConfig = config.GetCurrentConfig()
	}

	return s.cachedConfig
}

// BeginTransaction 开始配置事务
func (s *ConfigService) BeginTransaction() *ConfigTransaction {
	return &ConfigTransaction{
		service: s,
		changes: make([]ConfigChange, 0),
	}
}

// UpdateLLMConfigInTransaction 在事务中更新LLM配置
func (t *ConfigTransaction) UpdateLLMConfigInTransaction(provider string, configMap map[string]string, changedBy string) error {
	if t.committed {
		return errors.New("事务已提交")
	}

	// 记录变更
	t.changes = append(t.changes, ConfigChange{
		Type:     "llm_config",
		NewValue: map[string]interface{}{"provider": provider, "config": configMap},
	})

	return nil
}

// Commit 提交事务
func (t *ConfigTransaction) Commit() error {
	if t.committed {
		return errors.New("事务已提交")
	}

	t.service.mu.Lock()
	defer t.service.mu.Unlock()

	// 执行所有变更
	for _, change := range t.changes {
		switch change.Type {
		case "llm_config":
			data := change.NewValue.(map[string]interface{})
			provider := data["provider"].(string)
			configMap := data["config"].(map[string]string)

			if err := config.UpdateLLMConfig(provider, configMap); err != nil {
				// 回滚已执行的变更
				t.rollback()
				return err
			}
		}
	}

	t.committed = true
	t.service.cachedConfig = config.GetCurrentConfig()
	t.service.lastUpdated = time.Now()

	return nil
}

// rollback 回滚事务
func (t *ConfigTransaction) rollback() error {
	if t.rollbackFn != nil {
		return t.rollbackFn()
	}
	return nil
}

// UpdateLLMConfig 更新LLM提供商和配置
func (s *ConfigService) UpdateLLMConfig(provider string, configMap map[string]string, changedBy string) error {
	if provider == "" {
		return errors.New("provider cannot be empty")
	}

	// 验证配置参数
	if err := s.validateLLMConfig(provider, configMap); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 🔧 分步骤处理，避免复杂的锁操作
	var oldConfig *config.AppConfig
	var subscribers []ConfigChangeSubscriber

	// 步骤1：获取旧配置和订阅者
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		oldConfig = s.getCurrentConfigUnsafe()
		subscribers = make([]ConfigChangeSubscriber, len(s.subscribers))
		copy(subscribers, s.subscribers)

		s.recordAuditUnsafe("write", "LLM配置", changedBy)
		s.configVersion++
	}()

	// 步骤2：在无锁环境下更新配置
	newConfigMap := make(map[string]string)
	maps.Copy(newConfigMap, configMap)

	if _, ok := newConfigMap["default_model"]; !ok {
		newConfigMap["default_model"] = s.getDefaultModelForProvider(provider)
	}

	err := config.UpdateLLMConfig(provider, newConfigMap)
	if err != nil {
		// 回滚版本号
		s.mu.Lock()
		s.configVersion--
		s.mu.Unlock()
		return fmt.Errorf("更新配置失败: %w", err)
	}

	// 步骤3：更新缓存和记录变更
	var newConfig *config.AppConfig
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.cachedConfig = config.GetCurrentConfig()
		s.lastUpdated = time.Now()
		newConfig = s.cachedConfig

		s.recordChangeUnsafe("LLM提供商", oldConfig.LLMProvider, provider, changedBy)
		s.recordChangeUnsafe("LLM配置", oldConfig.LLMConfig, newConfigMap, changedBy)
	}()

	// 步骤4：异步通知
	s.notifySubscribersAsyncSafe(oldConfig, newConfig, subscribers)

	return nil
}

// 异步通知方法
func (s *ConfigService) notifySubscribersAsyncSafe(oldConfig, newConfig *config.AppConfig, subscribers []ConfigChangeSubscriber) {
	for _, subscriber := range subscribers {
		go func(sub ConfigChangeSubscriber) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("配置变更通知失败: %v", r)
					// 🔧 使用原子操作或单独的锁
					s.updateNotificationStats(false)
				}
			}()

			sub.OnConfigChanged(oldConfig, newConfig)

			// 🔧 使用原子操作或单独的锁
			s.updateNotificationStats(true)
		}(subscriber)
	}
}

// 线程安全的统计更新方法
func (s *ConfigService) updateNotificationStats(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if success {
		s.notificationStats.totalSent++
		s.notificationStats.lastSent = time.Now()
	} else {
		s.notificationStats.totalFailed++
	}
}

// validateLLMConfig 验证LLM配置
func (s *ConfigService) validateLLMConfig(provider string, configMap map[string]string) error {
	// 验证提供商
	supportedProviders := []string{
		"openai", "anthropic", "google", "github", "grok",
		"mistral", "qwen", "glm", "deepseek", "openrouter",
	}

	found := false
	for _, supported := range supportedProviders {
		if provider == supported {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("不支持的提供商: %s", provider)
	}

	// 验证必需的配置项
	if _, ok := configMap["api_key"]; !ok {
		return fmt.Errorf("缺少 api_key 配置")
	}

	if configMap["api_key"] == "" {
		return fmt.Errorf("api_key 不能为空")
	}
	// 验证特定提供商的配置
	switch provider {
	case "openai":
		if model, ok := configMap["default_model"]; ok {
			if !s.isValidOpenAIModel(model) {
				return fmt.Errorf("无效的 OpenAI 模型: %s", model)
			}
		}
	case "anthropic":
		if model, ok := configMap["default_model"]; ok {
			if !s.isValidAnthropicModel(model) {
				return fmt.Errorf("无效的 Anthropic 模型: %s", model)
			}
		}
		// 添加其他提供商的验证...
	}

	return nil
}

// getDefaultModelForProvider 获取提供商的默认模型
func (s *ConfigService) getDefaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-3.5-sonnet"
	case "google":
		return "gemini-2.0-pro-exp"
	case "github":
		return "gpt-4.1"
	case "grok":
		return "grok-3"
	case "mistral":
		return "mistral-large-latest"
	case "qwen":
		return "qwen2.5-max"
	case "glm":
		return "glm-4-plus"
	case "deepseek":
		return "deepseek-chat"
	case "openrouter":
		return "openai/gpt-4o"
	default:
		return "gpt-4o" // 默认回退
	}
}

// 模型验证方法
func (s *ConfigService) isValidOpenAIModel(model string) bool {
	validModels := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4", "gpt-4-turbo",
		"gpt-3.5-turbo", "o1-preview", "o1-mini",
	}
	return s.contains(validModels, model)
}

func (s *ConfigService) isValidAnthropicModel(model string) bool {
	validModels := []string{
		"claude-3.5-sonnet", "claude-3-opus", "claude-3-sonnet", "claude-3-haiku",
	}
	return s.contains(validModels, model)
}

func (s *ConfigService) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// recordChangeUnsafe 记录配置变更（不安全版本）
// 注意：此方法不获取锁，调用者必须持有 s.mu 锁
func (s *ConfigService) recordChangeUnsafe(section string, oldValue, newValue interface{}, changedBy string) {
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

// recordAuditUnsafe 记录配置访问（不安全版本）
func (s *ConfigService) recordAuditUnsafe(action, section, user string) {
	if !s.auditEnabled {
		return
	}

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

// 批量获取配置信息
func (s *ConfigService) GetLLMInfo() (string, map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 🔧 一次性获取配置，避免重复调用
	s.recordAuditUnsafe("read", "LLM配置", "system")

	if s.cachedConfig == nil {
		s.cachedConfig = config.GetCurrentConfig()
	}

	return s.cachedConfig.LLMProvider, s.cachedConfig.LLMConfig
}

// SaveConfig 保存当前配置
func (s *ConfigService) SaveConfig() error {
	return config.SaveConfig()
}

// GetLLMProvider 获取当前LLM提供商
func (s *ConfigService) GetLLMProvider() string {
	provider, _ := s.GetLLMInfo()
	return provider
}

// GetLLMConfig 获取LLM配置
func (s *ConfigService) GetLLMConfig() map[string]string {
	_, config := s.GetLLMInfo()
	return config
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
	// 获取当前完整配置
	cfg := s.GetCurrentConfig()

	// 创建新的配置映射
	newLLMConfig := make(map[string]string)
	maps.Copy(newLLMConfig, cfg.LLMConfig)

	// 通过更新LLM配置来间接更新调试模式
	// 这里需要根据实际的config包API来调整
	return s.UpdateLLMConfig(cfg.LLMProvider, newLLMConfig, "system")
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

// StartCacheRefresher 启动一个后台goroutine定期刷新配置缓存
func (s *ConfigService) StartCacheRefresher(refreshInterval time.Duration) {
	go func() {
		defer close(s.refresherDone)

		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				s.cachedConfig = config.GetCurrentConfig()
				s.lastUpdated = time.Now()
				s.mu.Unlock()
			case <-s.stopRefresher:
				return
			}
		}
	}()
}

// NewConfigHealthCheck 创建配置健康检查
func NewConfigHealthCheck(service *ConfigService) *ConfigHealthCheck {
	return &ConfigHealthCheck{service: service}
}

// CheckHealth 检查配置健康状态
func (c *ConfigHealthCheck) CheckHealth() map[string]interface{} {
	c.service.mu.RLock()
	defer c.service.mu.RUnlock()

	status := map[string]interface{}{
		"status": "healthy",
		"checks": make(map[string]interface{}),
	}

	checks := status["checks"].(map[string]interface{})

	// 检查配置是否加载
	if c.service.cachedConfig == nil {
		checks["config_loaded"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  "配置未加载",
		}
		status["status"] = "unhealthy"
	} else {
		checks["config_loaded"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	// 检查LLM配置
	if c.service.cachedConfig != nil {
		if c.service.cachedConfig.LLMProvider == "" {
			checks["llm_provider"] = map[string]interface{}{
				"status": "warning",
				"error":  "LLM提供商未配置",
			}
		} else {
			checks["llm_provider"] = map[string]interface{}{
				"status":   "healthy",
				"provider": c.service.cachedConfig.LLMProvider,
			}
		}
		if c.service.cachedConfig.LLMConfig == nil || c.service.cachedConfig.LLMConfig["api_key"] == "" {
			checks["llm_api_key"] = map[string]interface{}{
				"status": "warning",
				"error":  "LLM API密钥未配置",
			}
		} else {
			checks["llm_api_key"] = map[string]interface{}{
				"status": "healthy",
			}
		}
	}

	// 检查缓存状态
	checks["cache_status"] = map[string]interface{}{
		"status":       "healthy",
		"last_updated": c.service.lastUpdated.Format(time.RFC3339),
		"version":      c.service.configVersion,
	}

	return status
}

// GetMetrics 获取配置服务指标
func (s *ConfigService) GetMetrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"config_version":      s.configVersion,
		"last_updated":        s.lastUpdated.Format(time.RFC3339),
		"subscribers_count":   len(s.subscribers),
		"change_history_size": len(s.changeHistory),
		"audit_log_size":      len(s.auditLog),
		"audit_enabled":       s.auditEnabled,
		"cache_status":        s.cachedConfig != nil,
	}
}

// GetSubscribersCount 获取订阅者数量
func (s *ConfigService) GetSubscribersCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// ClearSubscribers 清空所有订阅者
func (s *ConfigService) ClearSubscribers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers = make([]ConfigChangeSubscriber, 0)
}

// 停止方法
func (s *ConfigService) StopCacheRefresher() {
	close(s.stopRefresher)
	<-s.refresherDone
}
