// internal/models/comic.go
package models

import "time"

// ComicFramePlan describes a single planned comic frame derived from StoryData.
// It is persisted to data/comics/scene_<id>/analysis.json.
type ComicFramePlan struct {
	ID           string   `json:"id"`
	Order        int      `json:"order"`
	Description  string   `json:"description"`
	StoryNodeIDs []string `json:"story_node_ids,omitempty"`
}

// ComicBreakdown is the analysis result of a scene's story, mapped into a fixed number of frames.
type ComicBreakdown struct {
	SceneID      string           `json:"scene_id"`
	Language     string           `json:"language,omitempty"`
	TargetFrames int              `json:"target_frames"`
	Frames       []ComicFramePlan `json:"frames"`
	Model        string           `json:"model,omitempty"`
	GeneratedAt  time.Time        `json:"generated_at"`
}

// ComicFramePrompt is the per-frame prompt output consumed by Vision models.
type ComicFramePrompt struct {
	FrameID        string                 `json:"frame_id"`
	Prompt         string                 `json:"prompt"`
	NegativePrompt string                 `json:"negative_prompt,omitempty"`
	Style          string                 `json:"style,omitempty"`
	Model          string                 `json:"model,omitempty"`
	ModelParams    map[string]interface{} `json:"model_params,omitempty"`
	PromptSources  *ComicPromptSources    `json:"prompt_sources,omitempty"`
}

// ComicPromptSources captures the grounding inputs used to generate a prompt.
// It enables debugging and verification of prompt provenance.
type ComicPromptSources struct {
	SceneID         string    `json:"scene_id"`
	NodeIDs         []string  `json:"node_ids,omitempty"`
	ConversationIDs []string  `json:"conversation_ids,omitempty"`
	ContentHashes   []string  `json:"content_hashes,omitempty"`
	Truncated       bool      `json:"truncated,omitempty"`
	ContinuityMode  string    `json:"continuity_mode,omitempty"`
	FrameAnchor     string    `json:"frame_anchor,omitempty"`
	TemplateVersion string    `json:"template_version,omitempty"`
	GeneratedAt     time.Time `json:"generated_at,omitempty"`
}

// ComicKeyElement describes a single extracted element.
type ComicKeyElement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ComicKeyElements is persisted to data/comics/scene_<id>/key_elements.json.
type ComicKeyElements struct {
	SceneID     string            `json:"scene_id"`
	Characters  []ComicKeyElement `json:"characters,omitempty"`
	Objects     []ComicKeyElement `json:"objects,omitempty"`
	Locations   []ComicKeyElement `json:"locations,omitempty"`
	StyleTags   []string          `json:"style_tags,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// ComicLLMCallMetrics captures token usage and latency for a single LLM call.
// It is persisted to data/comics/scene_<id>/metrics.json.
type ComicLLMCallMetrics struct {
	Provider     string    `json:"provider,omitempty"`
	Model        string    `json:"model,omitempty"`
	TokensUsed   int       `json:"tokens_used,omitempty"`
	PromptTokens int       `json:"prompt_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	DurationMs   int64     `json:"duration_ms,omitempty"`
	Cached       bool      `json:"cached,omitempty"`
	GeneratedAt  time.Time `json:"generated_at,omitempty"`
}

// ComicMetrics is a lightweight per-scene metrics snapshot for Phase2 tasks.
type ComicMetrics struct {
	SceneID     string                `json:"scene_id"`
	Analysis    *ComicLLMCallMetrics  `json:"analysis,omitempty"`
	Prompts     []ComicLLMCallMetrics `json:"prompts,omitempty"`
	KeyElements *ComicLLMCallMetrics  `json:"key_elements,omitempty"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// ComicReferenceMeta describes a single uploaded reference image bound to an element.
// It is persisted under data/comics/scene_<id>/references/index.json.
type ComicReferenceMeta struct {
	ElementID   string    `json:"element_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type,omitempty"`
	SizeBytes   int64     `json:"size_bytes,omitempty"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// ComicReferenceIndex is a per-scene index of reference images.
type ComicReferenceIndex struct {
	SceneID    string                        `json:"scene_id"`
	References map[string]ComicReferenceMeta `json:"references"`
	UpdatedAt  time.Time                     `json:"updated_at"`
}

// ComicFrameImageSignature records the effective render inputs used to generate a frame image.
// It is persisted under images/<frame_id>.meta.json and used by resume generation to avoid
// skipping stale images created with old prompt/style/model settings.
type ComicFrameImageSignature struct {
	FrameID      string    `json:"frame_id"`
	Signature    string    `json:"signature"`
	Prompt       string    `json:"prompt,omitempty"`
	Style        string    `json:"style,omitempty"`
	Model        string    `json:"model,omitempty"`
	GeneratedAt  time.Time `json:"generated_at"`
	TemplateHint string    `json:"template_hint,omitempty"`
}
