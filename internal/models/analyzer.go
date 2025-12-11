// internal/models/analyzer.go
package models

// AnalysisResult 表示文本分析的结果
type AnalysisResult struct {
	Title            string                 `json:"title"`      // 分析文本的标题
	Summary          string                 `json:"summary"`    // 文本摘要
	Scenes           []Scene                `json:"scenes"`     // 提取的场景列表
	Locations        []Location             `json:"locations"`  // 提取的地点列表
	Characters       []Character            `json:"characters"` // 提取的角色列表
	Items            []Item                 `json:"items"`      // 提取的物品列表
	OriginalSegments []OriginalSegment      `json:"original_segments,omitempty"`
	TextLength       int                    `json:"text_length"`        // 文本长度
	TextType         string                 `json:"text_type"`          // 文本类型
	Keywords         []string               `json:"keywords,omitempty"` // 关键词
	Themes           []string               `json:"themes,omitempty"`   // 主题
	Era              string                 `json:"era,omitempty"`      // 时代背景
	Metadata         map[string]interface{} `json:"metadata,omitempty"` // 可选：用于存储额外分析数据
}

// ContentSource 表示内容的来源
type ContentSource struct {
	Type   string `json:"type"`   // text, image, user, ai, etc.
	Origin string `json:"origin"` // 来源描述
}
