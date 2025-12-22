// internal/models/script.go
package models

import (
	"time"
)

type ScriptProject struct {
	ID                  string                     `json:"id"`
	Title               string                     `json:"title"`
	Type                string                     `json:"type"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
	Framework           map[string]interface{}     `json:"framework,omitempty"`
	RecommendedCommands []ScriptRecommendedCommand `json:"recommended_commands,omitempty"`
	State               ScriptState                `json:"state,omitempty"`
}

type ScriptRecommendedCommand struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	AssistMode string `json:"assist_mode"`
	Command    string `json:"command"`
	UserInput  string `json:"user_input,omitempty"`
}

type ScriptState struct {
	ActiveDraftID string       `json:"active_draft_id,omitempty"`
	Cursor        ScriptCursor `json:"cursor,omitempty"`
}

type ScriptCursor struct {
	Chapter int `json:"chapter"`
	Scene   int `json:"scene"`
	Segment int `json:"segment"`
}

type ScriptDraft struct {
	DraftID   string             `json:"draft_id"`
	CreatedAt time.Time          `json:"created_at"`
	Content   ScriptDraftContent `json:"content"`
	Notes     ScriptDraftNotes   `json:"notes,omitempty"`
}

type ScriptDraftContent struct {
	Chapters []ScriptChapter `json:"chapters"`
}

type ScriptChapter struct {
	Index  int           `json:"index"`
	Title  string        `json:"title,omitempty"`
	Scenes []ScriptScene `json:"scenes"`
}

type ScriptScene struct {
	Index int    `json:"index"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
}

type ScriptDraftNotes struct {
	UserPrompt string `json:"user_prompt,omitempty"`
	AISummary  string `json:"ai_summary,omitempty"`
}

// ScriptDraftMeta is a lightweight view for listing drafts (e.g. for rewind UI).
type ScriptDraftMeta struct {
	DraftID    string    `json:"draft_id"`
	CreatedAt  time.Time `json:"created_at"`
	UserPrompt string    `json:"user_prompt,omitempty"`
}

type ScriptCommandTarget struct {
	Chapter int `json:"chapter"`
	Scene   int `json:"scene"`
	Segment int `json:"segment"`
}

type ScriptCommandRequest struct {
	AssistMode string                 `json:"assist_mode"`
	UserInput  string                 `json:"user_input"`
	Command    string                 `json:"command"`
	Target     ScriptCommandTarget    `json:"target"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

type ScriptBranchOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type ScriptCommandOutput struct {
	MainText         string               `json:"main_text,omitempty"`
	Environment      string               `json:"environment,omitempty"`
	DialogueVariants []string             `json:"dialogue_variants,omitempty"`
	Subtext          string               `json:"subtext,omitempty"`
	Branches         []ScriptBranchOption `json:"branches,omitempty"`
}

type ScriptCommandResponse struct {
	DraftID        string                 `json:"draft_id"`
	WorkflowItemID string                 `json:"workflow_item_id"`
	Output         ScriptCommandOutput    `json:"output"`
	MemoryUpdate   map[string]interface{} `json:"memory_update,omitempty"`
}

type ScriptMemory struct {
	Version        int                      `json:"version"`
	UpdatedAt      time.Time                `json:"updated_at"`
	Facts          []ScriptMemoryFact       `json:"facts"`
	CharacterState map[string]interface{}   `json:"character_state,omitempty"`
	OpenThreads    []ScriptMemoryThread     `json:"open_threads"`
	Foreshadowing  []ScriptMemoryForeshadow `json:"foreshadowing"`
}

type ScriptMemoryFact struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Tags       []string `json:"tags,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
}

type ScriptMemoryThread struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type ScriptMemoryForeshadow struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type ScriptChapterSummaries struct {
	Items []ScriptChapterSummaryItem `json:"items"`
}

type ScriptChapterSummaryItem struct {
	Chapter       int                  `json:"chapter"`
	DraftID       string               `json:"draft_id"`
	Summary       string               `json:"summary"`
	ConflictState string               `json:"conflict_state"`
	NextOptions   []ScriptBranchOption `json:"next_options"`
}

type ScriptWorkflowRefs struct {
	DependsOn   []string `json:"depends_on,omitempty"`
	DerivedFrom []string `json:"derived_from,omitempty"`
	Rewrites    []string `json:"rewrites,omitempty"`
}

// ScriptWorkflowItem is a minimal workflow/command history item for scripts.
// P0: only keep a small recent window for quick UI review.
type ScriptWorkflowItem struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"` // e.g. "command"
	CreatedAt  time.Time           `json:"created_at"`
	DraftID    string              `json:"draft_id,omitempty"`
	Refs       *ScriptWorkflowRefs `json:"refs,omitempty"`
	AssistMode string              `json:"assist_mode,omitempty"`
	UserInput  string              `json:"user_input,omitempty"`
	Command    string              `json:"command,omitempty"`
	Target     ScriptCommandTarget `json:"target,omitempty"`
	Output     ScriptCommandOutput `json:"output,omitempty"`
}

type ScriptWorkflow struct {
	Items []ScriptWorkflowItem `json:"items"`
}
