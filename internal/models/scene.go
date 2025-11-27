// internal/models/scene.go
package models

import (
	"time"
)

// Scene 表示一个文学场景
type Scene struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id,omitempty"` // 所有者用户ID
	Title          string     `json:"title"`
	Name           string     `json:"name,omitempty"`
	Description    string     `json:"description"`
	Source         string     `json:"source"`
	CreatedAt      time.Time  `json:"created_at"`
	LastAccessed   time.Time  `json:"last_accessed"`
	LastUpdated    time.Time  `json:"last_updated"`
	Summary        string     `json:"summary"`
	Locations      []Location `json:"locations"`
	Themes         []string   `json:"themes"`
	Era            string     `json:"era"`
	Atmosphere     string     `json:"atmosphere,omitempty"`
	Items          []Item     `json:"items,omitempty"`
	CharacterCount int        `json:"character_count,omitempty"`
	ItemCount      int        `json:"item_count,omitempty"`
}

// Location 表示场景中的地点
type Location struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SceneMetadata 用于场景选择列表
type SceneMetadata struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id,omitempty"` // 所有者用户ID
	Name           string    `json:"name"`
	Source         string    `json:"source"`
	CreatedAt      time.Time `json:"created_at"`
	LastAccessed   time.Time `json:"last_accessed"`
	CharacterCount int       `json:"character_count"`
}

// SceneContext 场景上下文信息
type SceneContext struct {
	SceneID       string         `json:"scene_id"`
	Conversations []Conversation `json:"conversations"`
	LastUpdated   time.Time      `json:"last_updated"`
}

// SceneSettings 场景设置
type SceneSettings struct {
	SceneID            string    `json:"scene_id"`
	UserID             string    `json:"user_id,omitempty"`   // 所有者用户ID
	AllowFreeChat      bool      `json:"allow_free_chat"`     // 是否允许自由聊天
	AllowPlotTwists    bool      `json:"allow_plot_twists"`   // 是否允许剧情转折
	CreativityLevel    float32   `json:"creativity_level"`    // 创意程度 0.0-1.0
	ResponseLength     string    `json:"response_length"`     // 回复长度: short, medium, long
	InteractionStyle   string    `json:"interaction_style"`   // 互动风格: casual, formal, dramatic
	LanguageComplexity string    `json:"language_complexity"` // 语言复杂度: simple, normal, complex
	LastUpdated        time.Time `json:"last_updated"`
}
