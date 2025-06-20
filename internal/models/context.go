// internal/models/context.go
package models

import (
	"time"
)

// DialogContext 表示角色之间的对话上下文
type DialogContext struct {
	SceneID       string         `json:"scene_id"`
	Conversations []Conversation `json:"conversations"`
	LastUpdated   time.Time      `json:"last_updated"`
}

// Conversation 表示一轮对话
type Conversation struct {
	ID        string                 `json:"id"`
	SceneID   string                 `json:"scene_id"`
	SpeakerID string                 `json:"speaker_id"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Speaker   string                 `json:"speaker"`
	Message   string                 `json:"message"`
	Emotions  []string               `json:"emotions,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Memory 表示角色记忆的关键信息
type Memory struct {
	CharacterID string    `json:"character_id"`
	Key         string    `json:"key"`        // 记忆的关键点
	Details     string    `json:"details"`    // 详细内容
	Importance  int       `json:"importance"` // 重要性 1-10
	CreatedAt   time.Time `json:"created_at"`
	References  []string  `json:"references"` // 引用的对话ID
}
