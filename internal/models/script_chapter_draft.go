// internal/models/script_chapter_draft.go
package models

// ScriptChapterDraft represents chapter_draft.json.
// It is designed for user-editable per-chapter drafts (user_draft) and editable outline.
type ScriptChapterDraft struct {
	Chapters []ScriptChapterDraftChapter `json:"chapters"`
}

type ScriptChapterDraftChapter struct {
	Index     int    `json:"index"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Outline   string `json:"outline"`
	UserDraft string `json:"user_draft"`
}
