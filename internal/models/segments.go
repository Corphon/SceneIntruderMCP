// internal/models/segments.go
package models

// OriginalSegment 描述原文切分后的片段
type OriginalSegment struct {
	Index          int      `json:"index"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Tags           []string `json:"tags,omitempty"`
	StartParagraph int      `json:"start_paragraph"`
	EndParagraph   int      `json:"end_paragraph"`
	OriginalText   string   `json:"original_text"`
}
