// internal/models/story.go
package models

import (
	"time"
)

// ContentSourceType 表示内容来源的类型
type ContentSourceType string

const (
	// SourceExplicit 表示显式定义的内容
	SourceExplicit ContentSourceType = "EXPLICIT"
	// SourceGenerated 表示AI生成的内容
	SourceGenerated ContentSourceType = "GENERATED"
	// SourceExploration 表示通过探索发现的内容
	SourceExploration ContentSourceType = "EXPLORATION"
	// SourceSystem 表示系统生成的内容
	SourceSystem ContentSourceType = "SYSTEM"
	// SourceBranch 表示故事分支生成的内容
	SourceBranch ContentSourceType = "BRANCH"
	// SourceStory 表示通过故事节点获得的内容
	SourceStory ContentSourceType = "STORY"
)

type Choice struct {
	Text        string `json:"text"`
	NextNodeID  string `json:"next_node_id"`
	Consequence string `json:"consequence"`
}

type Requirement struct {
	Type  string `json:"type"` // item, event, dialog, etc.
	Value string `json:"value"`
}

// StoryData 表示一个场景的完整故事数据
type StoryData struct {
	SceneID       string          `json:"scene_id"`
	Intro         string          `json:"intro"`          // 故事介绍
	MainObjective string          `json:"main_objective"` // 主要目标
	CurrentState  string          `json:"current_state"`  // 当前状态
	Progress      int             `json:"progress"`       // 故事进度百分比
	Nodes         []StoryNode     `json:"nodes"`          // 故事节点
	Tasks         []Task          `json:"tasks"`          // 任务列表
	Locations     []StoryLocation `json:"locations"`      // 可探索地点
	LastUpdated   time.Time       `json:"last_updated"`   // 最后更新时间字段
}

// StoryNode 表示故事中的一个节点
type StoryNode struct {
	ID                    string                 `json:"id"`
	SceneID               string                 `json:"scene_id"`
	ParentID              string                 `json:"parent_id,omitempty"`
	Content               string                 `json:"content"`          // LLM 分析后的内容
	OriginalContent       string                 `json:"original_content"` // 原文内容
	Type                  string                 `json:"type"`             // 节点类型(main/side/hidden等)
	Choices               []StoryChoice          `json:"choices"`          // 可用选择
	IsRevealed            bool                   `json:"is_revealed"`      // 是否已显示
	CreatedAt             time.Time              `json:"created_at"`
	Source                ContentSourceType      `json:"source"` // 内容来源
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	RelatedItemIDs        []string               `json:"related_item_ids"`
	CharacterInteractions []CharacterInteraction `json:"character_interactions,omitempty"`
	InteractionTriggers   []InteractionTrigger   `json:"interaction_triggers,omitempty"`
}

// StoryChoice 表示故事节点中的一个选择
type StoryChoice struct {
	ID           string                 `json:"id"`
	Text         string                 `json:"text"`        // 选择文本
	Consequence  string                 `json:"consequence"` // 选择后果
	NextNodeID   string                 `json:"next_node_id,omitempty"`
	NextNodeHint string                 `json:"next_node_hint"` // 下一个节点提示
	Selected     bool                   `json:"selected"`       // 是否已选择
	CreatedAt    time.Time              `json:"created_at"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"`
	Impact       float64                `json:"impact"`
	Order        int                    `json:"order"`
	IsSelected   bool                   `json:"is_selected"`
	Requirements map[string]interface{} `json:"requirements,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// StoryBranch 表示故事的一个分支
type StoryBranch struct {
	ID         string    `json:"id"`
	SceneID    string    `json:"scene_id"`
	Name       string    `json:"name"`
	RootNodeID string    `json:"root_node_id"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// Task 表示故事中的一个任务
type Task struct {
	ID                  string                 `json:"id"`
	SceneID             string                 `json:"scene_id"`
	Title               string                 `json:"title"`       // 任务标题
	Description         string                 `json:"description"` // 任务描述
	Objectives          []Objective            `json:"objectives"`  // 任务目标
	Reward              string                 `json:"reward"`      // 任务奖励
	IsRevealed          bool                   `json:"is_revealed"` // 是否已显示
	Source              ContentSourceType      `json:"source"`      // 内容来源
	Type                string                 `json:"type"`        // social, exploration, relationship, main
	Status              string                 `json:"status"`      // active, completed, locked, hidden
	Priority            string                 `json:"priority"`    // low, normal, high, critical
	Completed           bool                   `json:"completed"`
	CompletedAt         time.Time              `json:"completed_at,omitempty"`
	TriggerCharacterID  string                 `json:"trigger_character_id,omitempty"`  // 触发任务的角色ID
	RelatedCharacterIDs []string               `json:"related_character_ids,omitempty"` // 相关角色ID列表
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// Objective 表示任务中的一个具体目标
type Objective struct {
	ID          string `json:"id"`
	Description string `json:"description"` // 目标描述
	Completed   bool   `json:"completed"`   // 是否已完成
}

// StoryLocation 表示故事中的一个可探索地点
type StoryLocation struct {
	ID           string            `json:"id"`
	SceneID      string            `json:"scene_id"`
	Name         string            `json:"name"`          // 地点名称
	Description  string            `json:"description"`   // 地点描述
	Accessible   bool              `json:"accessible"`    // 是否可访问
	RequiresItem string            `json:"requires_item"` // 访问所需物品
	Source       ContentSourceType `json:"source"`        // 内容来源
	ExploredAt   time.Time         `json:"explored_at,omitempty"`
}

// StoryUpdate 表示故事进展更新
type StoryUpdate struct {
	ID        string            `json:"id"`
	SceneID   string            `json:"scene_id"`
	Title     string            `json:"title"`   // 更新标题
	Content   string            `json:"content"` // 更新内容
	Type      string            `json:"type"`    // 更新类型
	CreatedAt time.Time         `json:"created_at"`
	Source    ContentSourceType `json:"source"`             // 内容来源
	NewTask   *Task             `json:"new_task,omitempty"` // 新任务
	NewClue   string            `json:"new_clue,omitempty"` // 新线索
}

// ExplorationResult 表示探索地点的结果
type ExplorationResult struct {
	LocationID   string     `json:"location_id"`
	Description  string     `json:"description"`          // 探索描述
	FoundItem    *Item      `json:"found_item,omitempty"` // 发现的物品
	StoryNode    *StoryNode `json:"story_node,omitempty"` // 触发的故事节点
	NewClue      string     `json:"new_clue,omitempty"`   // 发现的线索
	ExploredTime time.Time  `json:"explored_time"`
}

// StoryProgressStatus 表示故事进展状态
type StoryProgressStatus struct {
	SceneID             string        `json:"scene_id"`
	Progress            int           `json:"progress"`              // 总体进度
	CurrentState        string        `json:"current_state"`         // 当前状态
	CompletedTasks      int           `json:"completed_tasks"`       // 已完成任务数
	TotalTasks          int           `json:"total_tasks"`           // 总任务数
	TaskCompletionRate  float64       `json:"task_completion_rate"`  // 任务完成率
	AccessibleLocations int           `json:"accessible_locations"`  // 可访问地点数
	TotalLocations      int           `json:"total_locations"`       // 总地点数
	LocationAccessRate  float64       `json:"location_access_rate"`  // 地点访问率
	TotalStoryNodes     int           `json:"total_story_nodes"`     // 总节点数
	EstimatedCompletion time.Duration `json:"estimated_completion"`  // 预计完成时间
	IsMainObjectiveMet  bool          `json:"is_main_objective_met"` // 主目标是否完成
}
