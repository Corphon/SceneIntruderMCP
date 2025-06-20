// internal/models/character.go
package models

import "time"

// Character 表示小说中的一个角色
type Character struct {
	ID            string            `json:"id"`
	SceneID       string            `json:"scene_id"`
	Name          string            `json:"name"`
	Role          string            `json:"role"`
	Description   string            `json:"description"`
	Novel         string            `json:"novel"`
	Personality   string            `json:"personality"`
	Background    string            `json:"background"`
	SpeechStyle   string            `json:"speech_style,omitempty"`
	Relationships map[string]string `json:"relationships"`
	Knowledge     []string          `json:"knowledge"`
	ImageURL      string            `json:"image_url,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	LastUpdated   time.Time         `json:"last_updated"`
}

// CharacterResponse 表示角色回应
type ChatResponse struct {
	Character string `json:"character"`
	Response  string `json:"response"`
}

// EmotionalResponse 包含角色回复和情绪信息
type EmotionalResponse struct {
	CharacterID       string    `json:"character_id"`
	CharacterName     string    `json:"character_name"`
	Response          string    `json:"response"`           // 角色回复文本
	Emotion           string    `json:"emotion"`            // 主要情绪类型
	Intensity         int       `json:"intensity"`          // 情绪强度1-10
	BodyLanguage      string    `json:"body_language"`      // 肢体语言描述
	FacialExpression  string    `json:"facial_expression"`  // 面部表情描述
	VoiceTone         string    `json:"voice_tone"`         // 语调描述
	SecondaryEmotions []string  `json:"secondary_emotions"` // 次要情绪
	Timestamp         time.Time `json:"timestamp"`          // 时间戳
	TokensUsed        int       `json:"tokens_used,omitempty"`
}
