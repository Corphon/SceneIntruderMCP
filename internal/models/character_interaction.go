// internal/models/character_interaction.go
package models

import "time"

// CharacterInteraction 定义角色之间的互动会话
// 表示在特定场景中发生的多个角色之间的一组对话
type CharacterInteraction struct {
	ID           string                 `json:"id"`                 // 互动唯一标识符
	SceneID      string                 `json:"scene_id"`           // 所属场景ID
	CharacterIDs []string               `json:"character_ids"`      // 参与互动的角色ID列表
	Topic        string                 `json:"topic"`              // 互动主题
	Dialogues    []InteractionDialogue  `json:"dialogues"`          // 互动中的对话内容
	Timestamp    time.Time              `json:"timestamp"`          // 互动创建时间戳
	Metadata     map[string]interface{} `json:"metadata,omitempty"` // 额外元数据
}

// InteractionDialogue 定义互动中的单条对话
// 表示角色间互动中的一个发言项
type InteractionDialogue struct {
	CharacterID   string    `json:"character_id"`      // 发言角色ID
	CharacterName string    `json:"character_name"`    // 发言角色名称
	Message       string    `json:"message"`           // 对话内容
	Emotion       string    `json:"emotion,omitempty"` // 情绪标记
	Action        string    `json:"action,omitempty"`  // 动作描述
	Timestamp     time.Time `json:"timestamp"`         // 对话时间戳
}

// InteractionTrigger 定义角色互动的触发条件
// 用于在故事节点中定义何时触发角色间的互动
type InteractionTrigger struct {
	ID                 string    `json:"id"`                  // 触发器唯一标识符
	Condition          string    `json:"condition"`           // 触发条件描述
	CharacterIDs       []string  `json:"character_ids"`       // 参与互动的角色ID列表
	Topic              string    `json:"topic"`               // 互动主题
	ContextDescription string    `json:"context_description"` // 互动背景描述
	Triggered          bool      `json:"triggered"`           // 是否已触发
	CreatedAt          time.Time `json:"created_at"`          // 创建时间
}

// NewCharacterInteraction 创建一个新的角色互动实例
func NewCharacterInteraction(sceneID string, characterIDs []string, topic string) *CharacterInteraction {
	return &CharacterInteraction{
		ID:           "interaction_" + time.Now().Format("20060102150405"),
		SceneID:      sceneID,
		CharacterIDs: characterIDs,
		Topic:        topic,
		Dialogues:    []InteractionDialogue{},
		Timestamp:    time.Now(),
		Metadata:     map[string]interface{}{},
	}
}

// AddDialogue 向互动中添加一条对话
func (ci *CharacterInteraction) AddDialogue(characterID, characterName, message string, emotion string, action string) {
	dialogue := InteractionDialogue{
		CharacterID:   characterID,
		CharacterName: characterName,
		Message:       message,
		Emotion:       emotion,
		Action:        action,
		Timestamp:     time.Now(),
	}
	ci.Dialogues = append(ci.Dialogues, dialogue)
}

// GetParticipantNames 获取所有参与互动的角色名称
func (ci *CharacterInteraction) GetParticipantNames() []string {
	// 使用map去重
	namesMap := make(map[string]bool)
	for _, dialogue := range ci.Dialogues {
		namesMap[dialogue.CharacterName] = true
	}

	// 转换为数组
	names := make([]string, 0, len(namesMap))
	for name := range namesMap {
		names = append(names, name)
	}
	return names
}

// GetSummary 获取互动摘要
func (ci *CharacterInteraction) GetSummary() string {
	if len(ci.Dialogues) == 0 {
		return "无对话内容"
	}

	participants := ci.GetParticipantNames()
	if len(participants) <= 3 {
		return ci.Topic + " - " + joinStrings(participants, "、")
	} else {
		return ci.Topic + " - " + joinStrings(participants[:3], "、") + "等"
	}
}

// joinStrings 辅助函数：使用指定分隔符连接字符串切片
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
