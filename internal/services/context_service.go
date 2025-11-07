// internal/services/context_service.go
package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// ContextService 管理场景上下文和交互历史
type ContextService struct {
	SceneService SceneServiceInterface

	// 并发控制
	sceneLocks  sync.Map // sceneID -> *sync.RWMutex
	cacheMutex  sync.RWMutex
	sceneCache  map[string]*CachedSceneData
	cacheExpiry time.Duration
}

type SceneServiceInterface interface {
	LoadScene(sceneID string) (*SceneData, error)
	UpdateContext(sceneID string, context *models.SceneContext) error
}

// NewContextService 创建上下文服务
func NewContextService(sceneService SceneServiceInterface) *ContextService {
	service := &ContextService{
		SceneService: sceneService,
		sceneCache:   make(map[string]*CachedSceneData),
		cacheExpiry:  5 * time.Minute, // 5分钟缓存过期
	}

	// 🔧 启动后台清理协程
	go func() {
		ticker := time.NewTicker(2 * time.Minute) // 每2分钟清理一次
		defer ticker.Stop()

		for range ticker.C {
			service.cleanupExpiredCache()
		}
	}()

	return service
}

// 场景锁
func (s *ContextService) getSceneLock(sceneID string) *sync.RWMutex {
	value, _ := s.sceneLocks.LoadOrStore(sceneID, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

// 安全加载场景数据（带缓存）
func (s *ContextService) loadSceneDataSafe(sceneID string) (*SceneData, error) {
	lock := s.getSceneLock(sceneID)
	lock.RLock()

	// 检查缓存
	s.cacheMutex.RLock()
	if cached, exists := s.sceneCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			lock.RUnlock()
			return cached.SceneData, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 缓存过期或不存在，需要重新加载
	lock.RUnlock()
	lock.Lock()
	defer lock.Unlock()

	// 双重检查
	s.cacheMutex.RLock()
	if cached, exists := s.sceneCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached.SceneData, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 读取场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.sceneCache[sceneID] = &CachedSceneData{
		SceneData: sceneData,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	return sceneData, nil
}

// GetRecentConversations 获取最近的对话
func (s *ContextService) GetRecentConversations(sceneID string, limit int) ([]models.Conversation, error) {
	// 使用缓存加载场景数据
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return nil, err
	}

	// 获取对话列表
	conversations := sceneData.Context.Conversations

	// 如果对话数量少于limit，返回全部
	if len(conversations) <= limit {
		return conversations, nil
	}

	// 否则返回最近的limit条
	return conversations[len(conversations)-limit:], nil
}

// BuildCharacterMemory 构建角色记忆
func (s *ContextService) BuildCharacterMemory(sceneID, characterID string) (string, error) {
	// 使用缓存加载场景数据
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return "", err
	}

	// 查找角色
	var character *models.Character
	for _, c := range sceneData.Characters {
		if c.ID == characterID {
			character = c
			break
		}
	}

	if character == nil {
		return "", fmt.Errorf("角色不存在: %s", characterID)
	}

	// 简单内存构建
	memory := fmt.Sprintf("我是%s，我在%s场景中。我是%s。",
		character.Name, sceneData.Scene.Title, character.Description)

	return memory, nil
}

// AddConversation 添加对话到场景上下文，支持角色间对话记录
func (s *ContextService) AddConversation(sceneID, speakerID, content string, metadata map[string]interface{}) error {
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 在锁内加载最新的场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return err
	}

	// 创建新对话
	conversation := models.Conversation{
		ID:        fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		SceneID:   sceneID,
		SpeakerID: speakerID,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	// 检查是否是角色互动对话
	if metadata != nil {
		if interactionID, ok := metadata["interaction_id"].(string); ok {
			conversation.Metadata["conversation_type"] = "character_interaction"
			conversation.Metadata["interaction_id"] = interactionID
		} else if simulationID, ok := metadata["simulation_id"].(string); ok {
			conversation.Metadata["conversation_type"] = "character_simulation"
			conversation.Metadata["simulation_id"] = simulationID
		}

		// 记录目标接收者（如果有）
		if targetID, ok := metadata["target_character_id"].(string); ok {
			conversation.Metadata["target_character_id"] = targetID
		}
	}

	// 添加到上下文
	sceneData.Context.Conversations = append(sceneData.Context.Conversations, conversation)

	// 更新场景上下文
	err = s.SceneService.UpdateContext(sceneID, &sceneData.Context)
	if err != nil {
		return err
	}

	// 🔧 更新缓存
	s.cacheMutex.Lock()
	if s.sceneCache == nil {
		s.sceneCache = make(map[string]*CachedSceneData)
	}
	s.sceneCache[sceneID] = &CachedSceneData{
		SceneData: sceneData,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	// 清除缓存以强制重新加载 when context changes
	s.InvalidateSceneCache(sceneID)

	return nil
}

// GetCharacterInteractions 获取场景中的角色间互动历史
func (s *ContextService) GetCharacterInteractions(sceneID string, filter map[string]interface{}, limit int) ([]models.Conversation, error) {
	// 使用缓存加载场景数据
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return nil, err
	}

	// 获取所有对话
	conversations := sceneData.Context.Conversations

	// 筛选角色互动对话
	var interactions []models.Conversation
	for _, conv := range conversations {
		if conv.Metadata == nil {
			continue
		}

		// 检查是否是角色互动类型
		convType, hasType := conv.Metadata["conversation_type"]
		if !hasType {
			continue
		}

		isInteraction := convType == "character_interaction" || convType == "character_simulation"
		if !isInteraction {
			continue
		}

		// 应用过滤器
		matchesFilter := true
		for key, value := range filter {
			if metaValue, exists := conv.Metadata[key]; !exists || metaValue != value {
				matchesFilter = false
				break
			}
		}

		if matchesFilter {
			interactions = append(interactions, conv)
		}
	}

	// 如果结果数量少于limit，返回全部
	if len(interactions) <= limit || limit <= 0 {
		return interactions, nil
	}

	// 否则返回最近的limit条
	return interactions[len(interactions)-limit:], nil
}

// GetInteractionsByID 根据互动ID获取完整对话
func (s *ContextService) GetInteractionsByID(sceneID string, interactionID string) ([]models.Conversation, error) {
	filter := map[string]interface{}{
		"interaction_id": interactionID,
	}
	return s.GetCharacterInteractions(sceneID, filter, 0) // 0表示无限制
}

// GetSimulationByID 根据模拟ID获取完整对话
func (s *ContextService) GetSimulationByID(sceneID string, simulationID string) ([]models.Conversation, error) {
	filter := map[string]interface{}{
		"simulation_id": simulationID,
	}
	return s.GetCharacterInteractions(sceneID, filter, 0) // 0表示无限制
}

// GetCharacterToCharacterInteractions 获取特定两个角色之间的互动
func (s *ContextService) GetCharacterToCharacterInteractions(sceneID string, character1ID string, character2ID string, limit int) ([]models.Conversation, error) {
	// 使用缓存加载场景数据
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return nil, err
	}

	// 获取所有对话
	conversations := sceneData.Context.Conversations

	// 筛选两个角色之间的互动
	var interactions []models.Conversation
	for _, conv := range conversations {
		if conv.Metadata == nil {
			continue
		}

		// 首先，确认是角色互动类型
		convType, hasType := conv.Metadata["conversation_type"]
		isInteraction := hasType && (convType == "character_interaction" || convType == "character_simulation")
		if !isInteraction {
			continue
		}

		// 然后，检查是否涉及这两个角色
		speakerMatches := conv.SpeakerID == character1ID || conv.SpeakerID == character2ID

		var targetMatches bool
		if targetID, ok := conv.Metadata["target_character_id"].(string); ok {
			targetMatches = targetID == character1ID || targetID == character2ID
		}

		if speakerMatches && targetMatches {
			interactions = append(interactions, conv)
		}
	}

	// 如果结果数量少于limit，返回全部
	if len(interactions) <= limit || limit <= 0 {
		return interactions, nil
	}

	// 否则返回最近的limit条
	return interactions[len(interactions)-limit:], nil
}

// 手动清除指定场景的缓存（当场景数据更新时调用）
func (s *ContextService) InvalidateSceneCache(sceneID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	delete(s.sceneCache, sceneID)
}

// 清理过期缓存
func (s *ContextService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	for sceneID, cached := range s.sceneCache {
		if now.Sub(cached.Timestamp) > s.cacheExpiry {
			delete(s.sceneCache, sceneID)
		}
	}
}
