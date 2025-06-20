// internal/models/export.go
package models

import (
	"time"
)

// ExportResult 导出结果
type ExportResult struct {
	SceneID          string                  `json:"scene_id"`
	Title            string                  `json:"title"`
	Format           string                  `json:"format"`
	Content          string                  `json:"content"`
	GeneratedAt      time.Time               `json:"generated_at"`
	ExportType       string                  `json:"export_type"`
	Characters       []*Character            `json:"characters"`
	Conversations    []Conversation          `json:"conversations"`
	Summary          string                  `json:"summary"`
	FilePath         string                  `json:"file_path"` // 导出文件路径
	FileSize         int64                   `json:"file_size"` // 文件大小
	InteractionStats *InteractionExportStats `json:"stats,omitempty"`
	StoryData        *StoryData              `json:"story_data,omitempty"`
	SceneMetadata    *SceneMetadata          `json:"scene_metadata,omitempty"`
}

// InteractionExportStats 交互导出统计
type InteractionExportStats struct {
	TotalInteractions   int            `json:"total_interactions"`
	TotalMessages       int            `json:"total_messages"`
	CharacterCount      int            `json:"character_count"`
	DateRange           DateRange      `json:"date_range"`
	EmotionDistribution map[string]int `json:"emotion_distribution"`
	TopKeywords         []string       `json:"top_keywords"`
}

// DateRange 日期范围
type DateRange struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// 场景元数据结构
type Metadata struct {
	BasicInfo       SceneBasicInfo       `json:"basic_info"`
	EnvironmentInfo SceneEnvironmentInfo `json:"environment_info"`
	StatisticsInfo  SceneStatisticsInfo  `json:"statistics_info"`
}

// Metadata 场景元数据
type SceneBasicInfo struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Source       string    `json:"source"`
	CreatedAt    time.Time `json:"created_at"`
	LastUpdated  time.Time `json:"last_updated"`
	LastAccessed time.Time `json:"last_accessed"`
}

type SceneEnvironmentInfo struct {
	Themes     []string   `json:"themes"`
	Era        string     `json:"era"`
	Atmosphere string     `json:"atmosphere"`
	Locations  []Location `json:"locations"`
	Items      []Item     `json:"items"`
}

type SceneStatisticsInfo struct {
	CharacterCount    int   `json:"character_count"`
	LocationCount     int   `json:"location_count"`
	ItemCount         int   `json:"item_count"`
	TotalInteractions int   `json:"total_interactions"`
	FileSize          int64 `json:"file_size"`
}

// StoryExportStats 故事导出统计
type StoryExportStats struct {
	TotalNodes           int            `json:"total_nodes"`
	RevealedNodes        int            `json:"revealed_nodes"`
	TotalTasks           int            `json:"total_tasks"`
	CompletedTasks       int            `json:"completed_tasks"`
	TotalChoices         int            `json:"total_choices"`
	SelectedChoices      int            `json:"selected_choices"`
	Progress             int            `json:"progress"`
	CurrentState         string         `json:"current_state"`
	BranchCount          int            `json:"branch_count"`
	MaxDepth             int            `json:"max_depth"`
	CompletionRate       float64        `json:"completion_rate"`
	NodesByType          map[string]int `json:"nodes_by_type"`
	TasksByStatus        map[string]int `json:"tasks_by_status"`
	CharacterInvolvement map[string]int `json:"character_involvement"`
}
