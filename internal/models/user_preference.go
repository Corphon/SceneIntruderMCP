// internal/models/user_preference.go
package models

import "time"

type CreativityLevel string

const (
	// 严格遵循原文，最小化创造性内容
	CreativityStrict CreativityLevel = "STRICT"
	// 平衡原文与创造性内容
	CreativityBalanced CreativityLevel = "BALANCED"
	// 大胆扩展，允许更多创造性内容
	CreativityExpansive CreativityLevel = "EXPANSIVE"
)

// User 用户信息
type User struct {
	ID          string          `json:"id"`
	Username    string          `json:"username"`
	Email       string          `json:"email"`
	Avatar      string          `json:"avatar,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	LastLogin   time.Time       `json:"last_login"`
	LastUpdated time.Time       `json:"last_updated"`
	Preferences UserPreferences `json:"preferences"`
	// 可选字段
	DisplayName string   `json:"display_name,omitempty"`
	Bio         string   `json:"bio,omitempty"`
	SavedScenes []string `json:"saved_scenes,omitempty"` // 保存的场景ID列表
	// 道具和技能
	Items  []UserItem  `json:"items,omitempty"`  // 用户自定义道具
	Skills []UserSkill `json:"skills,omitempty"` // 用户自定义技能
}

// UserPreferences 用户偏好设置
type UserPreferences struct {
	CreativityLevel   CreativityLevel `json:"creativity_level"`          // 创意程度
	AllowPlotTwists   bool            `json:"allow_plot_twists"`         // 是否允许剧情转折
	ResponseLength    string          `json:"response_length"`           // 回复长度: short, medium, long
	LanguageStyle     string          `json:"language_style"`            // 语言风格: casual, formal, literary
	NotificationLevel string          `json:"notification_level"`        // 通知级别: none, important, all
	DarkMode          bool            `json:"dark_mode"`                 // 暗色模式
	PreferredModel    string          `json:"preferred_model,omitempty"` // 首选LLM模型
	AutoSave          bool            `json:"auto_save"`
}

// 在现有结构前添加新的道具和技能数据结构

// UserItem 用户自定义道具
type UserItem struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IconURL     string       `json:"icon_url,omitempty"`
	Effects     []ItemEffect `json:"effects"`
	Tags        []string     `json:"tags,omitempty"`
	Created     time.Time    `json:"created"`
	Updated     time.Time    `json:"updated"`
}

// ItemEffect 道具效果
type ItemEffect struct {
	Target      string  `json:"target"` // "self", "other", "all"
	Type        string  `json:"type"`   // "health", "mood", "stat", "special"
	Value       int     `json:"value"`  // 数值效果
	Description string  `json:"description"`
	Duration    int     `json:"duration,omitempty"` // 持续回合数，0表示立即
	Probability float64 `json:"probability"`        // 触发概率，0-1
}

// UserSkill 用户自定义技能
type UserSkill struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	IconURL      string        `json:"icon_url,omitempty"`
	Effects      []SkillEffect `json:"effects"`
	Cooldown     int           `json:"cooldown,omitempty"`     // 冷却回合数
	Requirements []string      `json:"requirements,omitempty"` // 使用需求
	Tags         []string      `json:"tags,omitempty"`
	Created      time.Time     `json:"created"`
	Updated      time.Time     `json:"updated"`
}

// SkillEffect 技能效果
type SkillEffect struct {
	Target      string  `json:"target"` // "self", "other", "all"
	Type        string  `json:"type"`   // "health", "mood", "stat", "special"
	Value       int     `json:"value"`  // 数值效果
	Description string  `json:"description"`
	Duration    int     `json:"duration,omitempty"` // 持续回合数，0表示立即
	Probability float64 `json:"probability"`        // 触发概率，0-1
}
