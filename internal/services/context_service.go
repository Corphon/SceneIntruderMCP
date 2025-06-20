// internal/services/context_service.go
package services

import (
	"fmt"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// ContextService 管理场景上下文和交互历史
type ContextService struct {
	SceneService SceneServiceInterface
}

type SceneServiceInterface interface {
	LoadScene(sceneID string) (*SceneData, error)
	UpdateContext(sceneID string, context *models.SceneContext) error
}

// NewContextService 创建上下文服务
func NewContextService(sceneService SceneServiceInterface) *ContextService {
	return &ContextService{
		SceneService: sceneService,
	}
}

// GetRecentConversations 获取最近的对话
func (s *ContextService) GetRecentConversations(sceneID string, limit int) ([]models.Conversation, error) {
	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
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
	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
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

	// 在实际应用中，可以基于过去的对话和事件更深入地构建记忆
	return memory, nil
}

// AddConversation 添加对话到场景上下文，支持角色间对话记录
func (s *ContextService) AddConversation(sceneID, speakerID, content string, metadata map[string]interface{}) error {
	// 加载场景数据
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
	return s.SceneService.UpdateContext(sceneID, &sceneData.Context)
}

// GetCharacterInteractions 获取场景中的角色间互动历史
func (s *ContextService) GetCharacterInteractions(sceneID string, filter map[string]interface{}, limit int) ([]models.Conversation, error) {
	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
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
	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
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
