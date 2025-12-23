// internal/services/context_service.go
package services

import (
	"fmt"
	"strings"
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
	LoadSceneNoCache(sceneID string) (*SceneData, error)
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
	// 使用无缓存加载，避免并发写入时读到旧上下文导致覆盖
	sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
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

// GetRecentConsoleStoryEntries 获取最近的 console_story 内容
func (s *ContextService) GetRecentConsoleStoryEntries(sceneID string, limit int) ([]models.Conversation, error) {
	if limit <= 0 {
		limit = 3
	}
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return nil, err
	}
	conversations := sceneData.Context.Conversations
	if len(conversations) == 0 {
		return []models.Conversation{}, nil
	}
	target := limit * 2
	filtered := make([]models.Conversation, 0, target)
	for i := len(conversations) - 1; i >= 0 && len(filtered) < target; i-- {
		conv := conversations[i]
		if isStoryConsoleConversation(conv) {
			filtered = append(filtered, conv)
		}
	}
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	return filtered, nil
}

func isStoryConsoleConversation(conv models.Conversation) bool {
	if conv.Metadata != nil {
		if convType, ok := conv.Metadata["conversation_type"].(string); ok && strings.EqualFold(convType, "story_console") {
			return true
		}
	}
	return strings.HasPrefix(conv.SpeakerID, "console_")
}

func isStoryConsoleUserConversation(conv models.Conversation) bool {
	if conv.Metadata != nil {
		if convType, ok := conv.Metadata["conversation_type"].(string); ok && strings.EqualFold(convType, "story_console") {
			if channel, ok := conv.Metadata["channel"].(string); ok && strings.EqualFold(channel, "user") {
				return true
			}
		}
	}
	speaker := strings.ToLower(strings.TrimSpace(conv.SpeakerID))
	return speaker == "web_user" || speaker == "user"
}

func resolveConversationNodeID(conv models.Conversation) string {
	if trimmed := strings.TrimSpace(conv.NodeID); trimmed != "" {
		return trimmed
	}
	if conv.Metadata != nil {
		if raw, ok := conv.Metadata["node_id"].(string); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// GetConsoleStoryEntriesByNode 获取指定节点的 console_story 内容
func (s *ContextService) GetConsoleStoryEntriesByNode(sceneID, nodeID string, limit int) ([]models.Conversation, error) {
	if limit <= 0 {
		limit = 3
	}
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return nil, err
	}
	conversations := sceneData.Context.Conversations
	if len(conversations) == 0 || strings.TrimSpace(nodeID) == "" {
		return []models.Conversation{}, nil
	}
	result := make([]models.Conversation, 0, limit)
	for i := len(conversations) - 1; i >= 0 && len(result) < limit; i-- {
		conv := conversations[i]
		if !isStoryConsoleConversation(conv) {
			continue
		}
		if resolveConversationNodeID(conv) != nodeID {
			continue
		}
		result = append(result, conv)
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// GetUserStoryCommandsByNode 获取指定节点下最新的玩家指令
func (s *ContextService) GetUserStoryCommandsByNode(sceneID, nodeID string, limit int) ([]models.Conversation, error) {
	if limit <= 0 {
		limit = 1
	}
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return nil, err
	}
	conversations := sceneData.Context.Conversations
	if len(conversations) == 0 || strings.TrimSpace(nodeID) == "" {
		return []models.Conversation{}, nil
	}
	result := make([]models.Conversation, 0, limit)
	for i := len(conversations) - 1; i >= 0 && len(result) < limit; i-- {
		conv := conversations[i]
		if !isStoryConsoleUserConversation(conv) {
			continue
		}
		if resolveConversationNodeID(conv) != nodeID {
			continue
		}
		result = append(result, conv)
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// RemoveConversationsAfterNode 移除指定节点之后的旁白/用户对话，防止回溯后残留上下文
func (s *ContextService) RemoveConversationsAfterNode(sceneID string, removedNodeIDs []string, cutoff time.Time) error {
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
	if err != nil {
		return err
	}

	removeLookup := make(map[string]struct{}, len(removedNodeIDs))
	for _, id := range removedNodeIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			removeLookup[id] = struct{}{}
		}
	}

	original := sceneData.Context.Conversations
	filtered := make([]models.Conversation, 0, len(original))
	for _, conv := range original {
		nodeID := resolveConversationNodeID(conv)
		if _, exists := removeLookup[nodeID]; exists {
			continue
		}
		if !cutoff.IsZero() && conv.Timestamp.After(cutoff) {
			// 回溯后移除 cutoff 之后的所有上下文（含旁白/用户/console），确保定点截断
			continue
		}
		filtered = append(filtered, conv)
	}

	if len(filtered) == len(original) {
		return nil
	}

	sceneData.Context.Conversations = filtered
	if err := s.SceneService.UpdateContext(sceneID, &sceneData.Context); err != nil {
		return err
	}

	s.InvalidateSceneCache(sceneID)
	return nil
}

// HasUserStoryCommands 判断指定节点是否存在玩家指令
func (s *ContextService) HasUserStoryCommands(sceneID, nodeID string) bool {
	sceneData, err := s.loadSceneDataSafe(sceneID)
	if err != nil {
		return false
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return false
	}
	for i := len(sceneData.Context.Conversations) - 1; i >= 0; i-- {
		conv := sceneData.Context.Conversations[i]
		if resolveConversationNodeID(conv) != nodeID {
			continue
		}
		if isStoryConsoleUserConversation(conv) {
			return true
		}
	}
	return false
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
func (s *ContextService) AddConversation(sceneID, speakerID, content string, metadata map[string]interface{}, nodeID string) error {
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 在锁内强制加载最新场景数据，避免使用缓存导致旧上下文覆盖新写入
	sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
	if err != nil {
		return err
	}

	var metaCopy map[string]interface{}
	if metadata != nil {
		metaCopy = make(map[string]interface{}, len(metadata)+1)
		for k, v := range metadata {
			metaCopy[k] = v
		}
	}
	if metaCopy == nil {
		metaCopy = make(map[string]interface{})
	}
	if nodeID != "" {
		metaCopy["node_id"] = nodeID
	}

	// 创建新对话
	conversation := models.Conversation{
		ID:        fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		SceneID:   sceneID,
		SpeakerID: speakerID,
		NodeID:    nodeID,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metaCopy,
	}

	// 检查是否是角色互动对话
	if len(conversation.Metadata) > 0 {
		if interactionID, ok := conversation.Metadata["interaction_id"].(string); ok {
			conversation.Metadata["conversation_type"] = "character_interaction"
			conversation.Metadata["interaction_id"] = interactionID
		} else if simulationID, ok := conversation.Metadata["simulation_id"].(string); ok {
			conversation.Metadata["conversation_type"] = "character_simulation"
			conversation.Metadata["simulation_id"] = simulationID
		}

		// 记录目标接收者（如果有）
		if targetID, ok := conversation.Metadata["target_character_id"].(string); ok {
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

	// 🔧 更新缓存 - Invalidate cache so next read gets fresh data
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
