// internal/services/script_service.go
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

var (
	ErrScriptNotFound = errors.New("script not found")
)

type ScriptService struct {
	BasePath    string
	FileStorage *storage.FileStorage
	LLM         *LLMService
}

type scriptCommandLLMResponse struct {
	MainText         string                      `json:"main_text,omitempty"`
	Environment      string                      `json:"environment,omitempty"`
	DialogueVariants []string                    `json:"dialogue_variants,omitempty"`
	Subtext          string                      `json:"subtext,omitempty"`
	Branches         []models.ScriptBranchOption `json:"branches,omitempty"`
	MemoryUpdate     map[string]interface{}      `json:"memory_update,omitempty"`
}

func optionString(options map[string]interface{}, key string) string {
	if options == nil {
		return ""
	}
	v, ok := options[key]
	if !ok || v == nil {
		return ""
	}
	switch vv := v.(type) {
	case string:
		return strings.TrimSpace(vv)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", vv))
	}
}

func optionInt(options map[string]interface{}, key string, defaultValue int) int {
	if options == nil {
		return defaultValue
	}
	v, ok := options[key]
	if !ok || v == nil {
		return defaultValue
	}
	switch vv := v.(type) {
	case int:
		return vv
	case int64:
		return int(vv)
	case float64:
		return int(vv)
	case string:
		vv = strings.TrimSpace(vv)
		if vv == "" {
			return defaultValue
		}
		if n, err := strconv.Atoi(vv); err == nil {
			return n
		}
		return defaultValue
	default:
		asStr := strings.TrimSpace(fmt.Sprintf("%v", vv))
		if n, err := strconv.Atoi(asStr); err == nil {
			return n
		}
		return defaultValue
	}
}

func (s *ScriptService) bestEffortLoadChapterDraft(scriptID string) (*models.ScriptChapterDraft, error) {
	var cd models.ScriptChapterDraft
	err := s.FileStorage.LoadJSONFile(scriptID, "chapter_draft.json", &cd)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			cd = models.ScriptChapterDraft{Chapters: []models.ScriptChapterDraftChapter{}}
			return &cd, nil
		}
		return nil, err
	}
	if cd.Chapters == nil {
		cd.Chapters = []models.ScriptChapterDraftChapter{}
	}
	return &cd, nil
}

func (s *ScriptService) bestEffortUpsertChapterDraftUserDraft(scriptID string, chapterIndex int, userDraft string) {
	if strings.TrimSpace(scriptID) == "" || chapterIndex <= 0 {
		return
	}
	cd, err := s.bestEffortLoadChapterDraft(scriptID)
	if err != nil {
		utils.GetLogger().Warn("scripts best-effort load chapter_draft.json failed", map[string]interface{}{
			"script_id": scriptID,
			"err":       err.Error(),
		})
		return
	}
	if cd.Chapters == nil {
		cd.Chapters = []models.ScriptChapterDraftChapter{}
	}

	found := false
	for i := range cd.Chapters {
		if cd.Chapters[i].Index == chapterIndex {
			cd.Chapters[i].UserDraft = strings.TrimSpace(userDraft)
			found = true
			break
		}
	}
	if !found {
		cd.Chapters = append(cd.Chapters, models.ScriptChapterDraftChapter{
			Index:     chapterIndex,
			Title:     "",
			Summary:   "",
			Outline:   "",
			UserDraft: strings.TrimSpace(userDraft),
		})
	}

	if err := s.FileStorage.SaveJSONFile(scriptID, "chapter_draft.json", cd); err != nil {
		utils.GetLogger().Warn("scripts best-effort save chapter_draft.json failed", map[string]interface{}{
			"script_id": scriptID,
			"err":       err.Error(),
		})
	}
}

func (s *ScriptService) bestEffortUpsertChapterDraftFromOutlineRange(scriptID string, chapters []scriptOutlineChapter, startChapter int, endChapter int) {
	if strings.TrimSpace(scriptID) == "" {
		return
	}
	if startChapter <= 0 {
		startChapter = 1
	}
	if endChapter <= 0 {
		endChapter = startChapter
	}
	cd, err := s.bestEffortLoadChapterDraft(scriptID)
	if err != nil {
		utils.GetLogger().Warn("scripts best-effort load chapter_draft.json failed", map[string]interface{}{
			"script_id": scriptID,
			"err":       err.Error(),
		})
		return
	}
	if cd.Chapters == nil {
		cd.Chapters = []models.ScriptChapterDraftChapter{}
	}

	byIdx := map[int]scriptOutlineChapter{}
	for _, ch := range chapters {
		byIdx[ch.Index] = ch
	}

	for idx := startChapter; idx <= endChapter; idx++ {
		ch, ok := byIdx[idx]
		if !ok {
			continue
		}
		found := false
		for i := range cd.Chapters {
			if cd.Chapters[i].Index != idx {
				continue
			}
			if strings.TrimSpace(cd.Chapters[i].Title) == "" {
				cd.Chapters[i].Title = strings.TrimSpace(ch.Title)
			}
			if strings.TrimSpace(cd.Chapters[i].Summary) == "" {
				cd.Chapters[i].Summary = strings.TrimSpace(ch.Summary)
			}
			if strings.TrimSpace(cd.Chapters[i].Outline) == "" {
				cd.Chapters[i].Outline = strings.TrimSpace(ch.Outline)
			}
			found = true
			break
		}
		if !found {
			cd.Chapters = append(cd.Chapters, models.ScriptChapterDraftChapter{
				Index:     idx,
				Title:     strings.TrimSpace(ch.Title),
				Summary:   strings.TrimSpace(ch.Summary),
				Outline:   strings.TrimSpace(ch.Outline),
				UserDraft: "",
			})
		}
	}

	if err := s.FileStorage.SaveJSONFile(scriptID, "chapter_draft.json", cd); err != nil {
		utils.GetLogger().Warn("scripts best-effort save chapter_draft.json failed", map[string]interface{}{
			"script_id": scriptID,
			"err":       err.Error(),
		})
	}
}

func (s *ScriptService) bestEffortGetExpandSceneContext(scriptID string, currentChapter int) (string, string) {
	prevUserDraft := ""
	currOutline := ""
	if strings.TrimSpace(scriptID) == "" {
		return prevUserDraft, currOutline
	}
	if currentChapter <= 0 {
		currentChapter = 1
	}

	cd, err := s.bestEffortLoadChapterDraft(scriptID)
	if err == nil && cd != nil {
		for _, ch := range cd.Chapters {
			if ch.Index == currentChapter-1 {
				prevUserDraft = strings.TrimSpace(ch.UserDraft)
			}
			if ch.Index == currentChapter {
				currOutline = strings.TrimSpace(ch.Outline)
			}
		}
	}

	return prevUserDraft, currOutline
}

func stripCodeFences(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	// Common LLM wrappers: ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		// remove first line
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = strings.TrimSpace(s[idx+1:])
		}
		// remove ending fence
		if end := strings.LastIndex(s, "```"); end >= 0 {
			s = strings.TrimSpace(s[:end])
		}
	}
	return strings.TrimSpace(s)
}

func extractJSONObjectText(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

func extractOutlineChaptersFromTruncatedJSON(raw string) []scriptOutlineChapter {
	s := stripCodeFences(raw)
	if s == "" {
		return nil
	}

	chaptersIdx := strings.Index(s, "\"chapters\"")
	if chaptersIdx < 0 {
		return nil
	}
	arrayStart := strings.Index(s[chaptersIdx:], "[")
	if arrayStart < 0 {
		return nil
	}
	pos := chaptersIdx + arrayStart + 1

	out := make([]scriptOutlineChapter, 0)
	for pos < len(s) {
		objStartRel := strings.Index(s[pos:], "{")
		if objStartRel < 0 {
			break
		}
		objStart := pos + objStartRel

		depth := 0
		inString := false
		escaped := false
		objEnd := -1
		for i := objStart; i < len(s); i++ {
			ch := s[i]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
			if ch == '{' {
				depth++
				continue
			}
			if ch == '}' {
				depth--
				if depth == 0 {
					objEnd = i
					break
				}
			}
		}
		if objEnd < 0 {
			break
		}

		candidate := strings.TrimSpace(s[objStart : objEnd+1])
		var ch scriptOutlineChapter
		if err := json.Unmarshal([]byte(candidate), &ch); err == nil && ch.Index > 0 {
			out = append(out, ch)
		}

		pos = objEnd + 1
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func parseScriptOutlineBestEffort(raw string) (scriptOutline, bool) {
	clean := stripCodeFences(raw)
	if strings.TrimSpace(clean) == "" {
		return scriptOutline{}, false
	}

	var out scriptOutline
	if err := json.Unmarshal([]byte(clean), &out); err == nil {
		return out, true
	}

	if obj := extractJSONObjectText(clean); obj != "" {
		if err := json.Unmarshal([]byte(obj), &out); err == nil {
			return out, true
		}
	}

	chapters := extractOutlineChaptersFromTruncatedJSON(clean)
	if len(chapters) == 0 {
		return scriptOutline{}, false
	}
	version, _ := extractJSONStringField(clean, "version")
	if strings.TrimSpace(version) == "" {
		version = "v1"
	}
	return scriptOutline{Version: version, Chapters: chapters}, true
}

func looksLikeJSONObject(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

// extractJSONStringField tries to best-effort extract a JSON string field from a raw JSON-like text.
// It tolerates truncated JSON (common when LLM outputs are cut by max tokens).
func extractJSONStringField(raw string, field string) (string, bool) {
	s := stripCodeFences(raw)
	if s == "" {
		return "", false
	}
	needle := fmt.Sprintf("\"%s\"", field)
	idx := strings.Index(s, needle)
	if idx < 0 {
		return "", false
	}
	// find ':' after the field name
	colon := strings.Index(s[idx+len(needle):], ":")
	if colon < 0 {
		return "", false
	}
	i := idx + len(needle) + colon + 1
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			i++
			continue
		}
		break
	}
	if i >= len(s) || s[i] != '"' {
		return "", false
	}
	// parse JSON string value with escape handling
	start := i
	i++
	escaped := false
	for i < len(s) {
		ch := s[i]
		if escaped {
			escaped = false
			i++
			continue
		}
		if ch == '\\' {
			escaped = true
			i++
			continue
		}
		if ch == '"' {
			i++
			break
		}
		i++
	}
	if i <= start+1 {
		return "", false
	}
	quoted := s[start:i]
	var out string
	if err := json.Unmarshal([]byte(quoted), &out); err != nil {
		return "", false
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", false
	}
	return out, true
}

func scriptBilingualSystemPrompt(sampleText string) string {
	sampleText = strings.TrimSpace(sampleText)
	// Language policy:
	// - If the user's input is detected as English -> respond in English.
	// - Otherwise respond in the same language as the user's request.
	// - If unclear (empty/ambiguous) -> default to English (do NOT default to Chinese).
	isEnglish := false
	if sampleText == "" {
		isEnglish = true
	} else {
		isEnglish = isEnglishText(sampleText)
	}

	languageLine := "- Use the same language as the user's request; if unclear, use English. / 使用与用户请求一致的语言；不明确时用英文。\n"
	if isEnglish {
		languageLine = "- Respond in English. / 用英文回复。\n"
	}

	return "You are a professional creative writing assistant. Strictly follow the user's settings and constraints.\n" +
		"你是一个专业的创意写作助手。请严格遵守用户给定的设定与约束。\n\n" +
		"Output requirements / 输出要求:\n" +
		languageLine +
		"- If JSON is requested: output strict JSON only, no extra commentary. / 如要求输出 JSON：必须输出严格 JSON，不要添加解释文字。"
}

type scriptOutline struct {
	Version  string                 `json:"version"`
	Chapters []scriptOutlineChapter `json:"chapters"`
}

type scriptOutlineChapter struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Outline string `json:"outline,omitempty"`
}

func normalizeScriptOutline(outline *scriptOutline, desiredChapters int) {
	if outline == nil {
		return
	}
	if strings.TrimSpace(outline.Version) == "" {
		outline.Version = "v1"
	}
	if outline.Chapters == nil {
		outline.Chapters = []scriptOutlineChapter{}
	}

	targetCount := desiredChapters
	if targetCount <= 0 {
		// optimization.md: even for short stories, keep at least 8 chapters.
		targetCount = len(outline.Chapters)
		if targetCount < 8 {
			targetCount = 8
		}
	}
	if targetCount < 1 {
		targetCount = 1
	}
	if targetCount > 300 {
		targetCount = 300
	}

	byIndex := map[int]scriptOutlineChapter{}
	for _, ch := range outline.Chapters {
		idx := ch.Index
		if idx <= 0 {
			continue
		}
		if _, exists := byIndex[idx]; !exists {
			byIndex[idx] = ch
		}
	}

	normalized := make([]scriptOutlineChapter, 0, targetCount)
	for i := 1; i <= targetCount; i++ {
		ch, ok := byIndex[i]
		if !ok {
			ch = scriptOutlineChapter{Index: i}
		}
		if strings.TrimSpace(ch.Title) == "" {
			ch.Title = fmt.Sprintf("第%d章", i)
		}
		ch.Summary = strings.TrimSpace(ch.Summary)
		ch.Outline = strings.TrimSpace(ch.Outline)
		normalized = append(normalized, ch)
	}

	// keep stable order
	sort.SliceStable(normalized, func(i, j int) bool { return normalized[i].Index < normalized[j].Index })
	outline.Chapters = normalized
}

func readOptionalChapterCount(framework map[string]interface{}) int {
	if framework == nil {
		return 0
	}
	v, ok := framework["chapter_count"]
	if !ok || v == nil {
		return 0
	}

	count := 0
	switch vv := v.(type) {
	case float64:
		count = int(vv)
	case int:
		count = vv
	case int32:
		count = int(vv)
	case int64:
		count = int(vv)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
			count = parsed
		}
	default:
		return 0
	}

	if count <= 0 {
		return 0
	}
	if count < 1 {
		count = 1
	}
	if count > 300 {
		count = 300
	}
	return count
}

func NewScriptService(basePath string, llm *LLMService) (*ScriptService, error) {
	fs, err := storage.NewFileStorage(basePath)
	if err != nil {
		return nil, err
	}
	return &ScriptService{
		BasePath:    basePath,
		FileStorage: fs,
		LLM:         llm,
	}, nil
}

func (s *ScriptService) CreateProject(ctx context.Context, title, scriptType string, framework map[string]interface{}) (*models.ScriptProject, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(scriptType) == "" {
		scriptType = "novel"
	}
	id := fmt.Sprintf("script_%d", time.Now().UnixNano())
	now := time.Now()

	project := &models.ScriptProject{
		ID:        id,
		Title:     title,
		Type:      scriptType,
		CreatedAt: now,
		UpdatedAt: now,
		Framework: framework,
		RecommendedCommands: []models.ScriptRecommendedCommand{
			{
				ID:         "expand_scene",
				Label:      "扩写当前场景（补全）",
				AssistMode: "completion",
				Command:    "请结合当前章节大纲，编写当前章节正文（user_draft）：在不改变既定情节的前提下补全细节、动作与对白，并保持与前文风格与事实一致。",
			},
			{
				ID:         "polish_language",
				Label:      "优化语言与节奏（润色）",
				AssistMode: "polish",
				Command:    "请在不改变既定情节与关键信息的前提下，润色当前场景：删冗余、理顺句式、增强画面感与节奏，并补足必要的动作与对白（保持风格一致）。",
			},
			{
				ID:         "add_tension",
				Label:      "增强情绪张力（灵感）",
				AssistMode: "inspiration",
				Command:    "请为当前场景增加更强的情绪冲突与悬念：强化动机对立、信息差与暗示。给出 2-3 个可选走向（branches）。",
			},
		},
		State: models.ScriptState{
			Cursor: models.ScriptCursor{Chapter: 1, Scene: 1, Segment: 0},
		},
	}

	dir := id
	if err := s.FileStorage.SaveJSONFile(dir, "project.json", project); err != nil {
		return nil, err
	}

	mem := &models.ScriptMemory{
		Version:       1,
		UpdatedAt:     now,
		Facts:         []models.ScriptMemoryFact{},
		OpenThreads:   []models.ScriptMemoryThread{},
		Foreshadowing: []models.ScriptMemoryForeshadow{},
	}
	if err := s.FileStorage.SaveJSONFile(dir, "memory.json", mem); err != nil {
		utils.GetLogger().Warn("scripts CreateProject best-effort save failed", map[string]interface{}{
			"script_id": id,
			"file":      "memory.json",
			"err":       err,
		})
	}

	summaries := &models.ScriptChapterSummaries{Items: []models.ScriptChapterSummaryItem{}}
	if err := s.FileStorage.SaveJSONFile(dir, "chapter_summaries.json", summaries); err != nil {
		utils.GetLogger().Warn("scripts CreateProject best-effort save failed", map[string]interface{}{
			"script_id": id,
			"file":      "chapter_summaries.json",
			"err":       err,
		})
	}

	wf := &models.ScriptWorkflow{Items: []models.ScriptWorkflowItem{}}
	if err := s.FileStorage.SaveJSONFile(dir, "workflow_items.json", wf); err != nil {
		utils.GetLogger().Warn("scripts CreateProject best-effort save failed", map[string]interface{}{
			"script_id": id,
			"file":      "workflow_items.json",
			"err":       err,
		})
	}

	// P0: editable attachments for view-details UX
	if err := s.FileStorage.SaveJSONFile(dir, "characters.json", []map[string]interface{}{}); err != nil {
		utils.GetLogger().Warn("scripts CreateProject best-effort save failed", map[string]interface{}{
			"script_id": id,
			"file":      "characters.json",
			"err":       err,
		})
	}
	if err := s.FileStorage.SaveJSONFile(dir, "items.json", []map[string]interface{}{}); err != nil {
		utils.GetLogger().Warn("scripts CreateProject best-effort save failed", map[string]interface{}{
			"script_id": id,
			"file":      "items.json",
			"err":       err,
		})
	}

	// P0: chapter_draft.json (best-effort init; used by Outline card + future user_draft)
	if err := s.FileStorage.SaveJSONFile(dir, "chapter_draft.json", &models.ScriptChapterDraft{Chapters: []models.ScriptChapterDraftChapter{}}); err != nil {
		utils.GetLogger().Warn("scripts CreateProject best-effort save failed", map[string]interface{}{
			"script_id": id,
			"file":      "chapter_draft.json",
			"err":       err,
		})
	}

	return project, nil
}

func (s *ScriptService) LoadCharacters(ctx context.Context, scriptID string) ([]map[string]interface{}, error) {
	if strings.TrimSpace(scriptID) == "" {
		return nil, ErrScriptNotFound
	}
	// ensure script exists
	if _, err := s.GetProject(ctx, scriptID); err != nil {
		return nil, err
	}

	var chars []map[string]interface{}
	err := s.FileStorage.LoadJSONFile(scriptID, "characters.json", &chars)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			chars = []map[string]interface{}{}
			if saveErr := s.FileStorage.SaveJSONFile(scriptID, "characters.json", chars); saveErr != nil {
				utils.GetLogger().Warn("scripts LoadCharacters best-effort init failed", map[string]interface{}{
					"script_id": scriptID,
					"file":      "characters.json",
					"err":       saveErr,
				})
			}
			return chars, nil
		}
		return nil, err
	}
	if chars == nil {
		chars = []map[string]interface{}{}
	}
	return chars, nil
}

func (s *ScriptService) SaveCharacters(ctx context.Context, scriptID string, chars []map[string]interface{}) error {
	if strings.TrimSpace(scriptID) == "" {
		return ErrScriptNotFound
	}
	// ensure script exists
	if _, err := s.GetProject(ctx, scriptID); err != nil {
		return err
	}
	if chars == nil {
		chars = []map[string]interface{}{}
	}
	return s.FileStorage.SaveJSONFile(scriptID, "characters.json", chars)
}

func (s *ScriptService) LoadItems(ctx context.Context, scriptID string) ([]map[string]interface{}, error) {
	if strings.TrimSpace(scriptID) == "" {
		return nil, ErrScriptNotFound
	}
	// ensure script exists
	if _, err := s.GetProject(ctx, scriptID); err != nil {
		return nil, err
	}

	var items []map[string]interface{}
	err := s.FileStorage.LoadJSONFile(scriptID, "items.json", &items)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			items = []map[string]interface{}{}
			if saveErr := s.FileStorage.SaveJSONFile(scriptID, "items.json", items); saveErr != nil {
				utils.GetLogger().Warn("scripts LoadItems best-effort init failed", map[string]interface{}{
					"script_id": scriptID,
					"file":      "items.json",
					"err":       saveErr,
				})
			}
			return items, nil
		}
		return nil, err
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	return items, nil
}

func (s *ScriptService) SaveItems(ctx context.Context, scriptID string, items []map[string]interface{}) error {
	if strings.TrimSpace(scriptID) == "" {
		return ErrScriptNotFound
	}
	// ensure script exists
	if _, err := s.GetProject(ctx, scriptID); err != nil {
		return err
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	return s.FileStorage.SaveJSONFile(scriptID, "items.json", items)
}

func (s *ScriptService) UpdateChapterUserDraft(ctx context.Context, scriptID string, chapterIndex int, userDraft string) (*models.ScriptChapterDraft, error) {
	if strings.TrimSpace(scriptID) == "" {
		return nil, ErrScriptNotFound
	}
	if chapterIndex <= 0 {
		return nil, fmt.Errorf("chapter must be > 0")
	}

	// ensure script exists
	project, err := s.GetProject(ctx, scriptID)
	if err != nil {
		return nil, err
	}

	cd, err := s.bestEffortLoadChapterDraft(scriptID)
	if err != nil {
		return nil, err
	}
	if cd.Chapters == nil {
		cd.Chapters = []models.ScriptChapterDraftChapter{}
	}

	clean := strings.TrimSpace(userDraft)
	found := false
	for i := range cd.Chapters {
		if cd.Chapters[i].Index == chapterIndex {
			cd.Chapters[i].UserDraft = clean
			found = true
			break
		}
	}
	if !found {
		cd.Chapters = append(cd.Chapters, models.ScriptChapterDraftChapter{
			Index:     chapterIndex,
			Title:     "",
			Summary:   "",
			Outline:   "",
			UserDraft: clean,
		})
	}

	if err := s.FileStorage.SaveJSONFile(scriptID, "chapter_draft.json", cd); err != nil {
		return nil, err
	}

	// keep UpdatedAt roughly consistent for detail pages
	project.UpdatedAt = time.Now()
	_ = s.FileStorage.SaveJSONFile(scriptID, "project.json", project)

	return cd, nil
}

func (s *ScriptService) UpdateProjectBasics(ctx context.Context, scriptID string, title string, typ string, framework map[string]interface{}) (*models.ScriptProject, error) {
	project, err := s.GetProject(ctx, scriptID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(title) != "" {
		project.Title = strings.TrimSpace(title)
	}
	if strings.TrimSpace(typ) != "" {
		project.Type = strings.TrimSpace(typ)
	}
	if framework != nil {
		project.Framework = framework
	}

	project.UpdatedAt = time.Now()
	if err := s.FileStorage.SaveJSONFile(scriptID, "project.json", project); err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ScriptService) DeleteProject(ctx context.Context, scriptID string) error {
	if strings.TrimSpace(scriptID) == "" {
		return ErrScriptNotFound
	}
	if !strings.HasPrefix(scriptID, "script_") {
		return ErrScriptNotFound
	}

	// ensure exists (re-use GetProject to normalize not-found behavior)
	if _, err := s.GetProject(ctx, scriptID); err != nil {
		return err
	}

	if s.FileStorage != nil {
		if err := s.FileStorage.DeleteDir(scriptID); err != nil {
			// normalize to ErrScriptNotFound for missing directory
			if strings.Contains(err.Error(), "目录不存在") || strings.Contains(strings.ToLower(err.Error()), "not exist") {
				return ErrScriptNotFound
			}
			return err
		}
		return nil
	}

	dir := filepath.Join(s.BasePath, scriptID)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}

func (s *ScriptService) ManualEditDraft(ctx context.Context, scriptID string, baseDraftID string, target models.ScriptCommandTarget, text string, userPrompt string) (*models.ScriptProject, string, error) {
	project, err := s.GetProject(ctx, scriptID)
	if err != nil {
		return nil, "", err
	}

	resolvedBaseID := strings.TrimSpace(baseDraftID)
	if resolvedBaseID == "" {
		resolvedBaseID = strings.TrimSpace(project.State.ActiveDraftID)
	}

	var baseDraft *models.ScriptDraft
	if resolvedBaseID != "" {
		var d models.ScriptDraft
		if err := s.FileStorage.LoadJSONFile(filepath.Join(scriptID, "drafts"), resolvedBaseID+".json", &d); err == nil {
			baseDraft = &d
		}
	}

	if target.Chapter <= 0 {
		target.Chapter = 1
	}
	if target.Scene <= 0 {
		target.Scene = 1
	}

	content := updateDraftContent(baseDraft, target.Chapter, target.Scene, text)
	newDraftID := fmt.Sprintf("draft_%d", time.Now().UnixNano())
	now := time.Now()
	if strings.TrimSpace(userPrompt) == "" {
		userPrompt = "manual_edit"
	}

	draft := &models.ScriptDraft{
		DraftID:   newDraftID,
		CreatedAt: now,
		Content:   content,
		Notes:     models.ScriptDraftNotes{UserPrompt: userPrompt},
	}
	if err := s.FileStorage.SaveJSONFile(filepath.Join(scriptID, "drafts"), newDraftID+".json", draft); err != nil {
		return nil, "", err
	}

	project.State.ActiveDraftID = newDraftID
	project.State.Cursor = models.ScriptCursor{Chapter: target.Chapter, Scene: target.Scene, Segment: target.Segment}
	project.UpdatedAt = now
	if err := s.FileStorage.SaveJSONFile(scriptID, "project.json", project); err != nil {
		return nil, "", err
	}

	// P0: best-effort sync to chapter_draft.user_draft ONLY when caller explicitly marks it.
	if strings.TrimSpace(userPrompt) == "__preview_edit_user_draft__" {
		s.bestEffortUpsertChapterDraftUserDraft(scriptID, target.Chapter, text)
	}

	// best-effort workflow record (keep small summary to avoid huge history files)
	wfID := fmt.Sprintf("wf_%d", time.Now().UnixNano())
	if err := s.appendWorkflowItem(scriptID, models.ScriptWorkflowItem{
		ID:        wfID,
		Type:      "manual_edit",
		CreatedAt: now,
		DraftID:   newDraftID,
		Command:   "manual_edit",
		UserInput: userPrompt,
		Target:    target,
		Output: models.ScriptCommandOutput{
			MainText: buildBriefSummary(text, 300),
		},
	}); err != nil {
		utils.GetLogger().Warn("scripts ManualEditDraft best-effort workflow append failed", map[string]interface{}{
			"script_id":   scriptID,
			"workflow_id": wfID,
			"draft_id":    newDraftID,
			"base_draft":  resolvedBaseID,
			"err":         err,
		})
	}

	return project, newDraftID, nil
}

func (s *ScriptService) GetProject(ctx context.Context, id string) (*models.ScriptProject, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrScriptNotFound
	}
	var project models.ScriptProject
	if err := s.FileStorage.LoadJSONFile(id, "project.json", &project); err != nil {
		if os.IsNotExist(unwrapPathError(err)) {
			return nil, ErrScriptNotFound
		}
		// best-effort: also treat missing file as not found
		if strings.Contains(strings.ToLower(err.Error()), "no such file") {
			return nil, ErrScriptNotFound
		}
		return nil, err
	}
	return &project, nil
}

func (s *ScriptService) ListProjects(ctx context.Context) ([]models.ScriptProject, error) {
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, err
	}

	projects := make([]models.ScriptProject, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirName := e.Name()
		if !strings.HasPrefix(dirName, "script_") {
			continue
		}
		var p models.ScriptProject
		if err := s.FileStorage.LoadJSONFile(dirName, "project.json", &p); err != nil {
			continue
		}
		projects = append(projects, p)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].UpdatedAt.After(projects[j].UpdatedAt)
	})

	return projects, nil
}

func (s *ScriptService) ListDraftMetas(ctx context.Context, scriptID string) ([]models.ScriptDraftMeta, error) {
	if strings.TrimSpace(scriptID) == "" {
		return nil, ErrScriptNotFound
	}
	// ensure script exists
	if _, err := s.GetProject(ctx, scriptID); err != nil {
		return nil, err
	}

	draftsDir := filepath.Join(s.BasePath, scriptID, "drafts")
	entries, err := os.ReadDir(draftsDir)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			return []models.ScriptDraftMeta{}, nil
		}
		return nil, err
	}

	metas := make([]models.ScriptDraftMeta, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "draft_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var d models.ScriptDraft
		if err := s.FileStorage.LoadJSONFile(filepath.Join(scriptID, "drafts"), name, &d); err != nil {
			continue
		}
		metas = append(metas, models.ScriptDraftMeta{
			DraftID:    d.DraftID,
			CreatedAt:  d.CreatedAt,
			UserPrompt: d.Notes.UserPrompt,
		})
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas, nil
}

func (s *ScriptService) Command(ctx context.Context, id string, req models.ScriptCommandRequest) (*models.ScriptCommandResponse, error) {
	project, err := s.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.LLM == nil {
		return nil, fmt.Errorf("%w: LLM service unavailable", ErrLLMNotReady)
	}
	if !s.LLM.IsReady() {
		return nil, fmt.Errorf("%w: %s", ErrLLMNotReady, s.LLM.GetReadyState())
	}

	if strings.TrimSpace(req.AssistMode) == "" {
		req.AssistMode = "inspiration"
	}
	if req.Target.Chapter == 0 {
		req.Target.Chapter = 1
	}
	if req.Target.Scene == 0 {
		req.Target.Scene = 1
	}

	commandID := optionString(req.Options, "command_id")
	if commandID == "" {
		commandID = optionString(req.Options, "commandId")
	}
	if strings.EqualFold(commandID, "fill_outline_next") {
		start := req.Target.Chapter
		if start <= 0 {
			start = 1
		}
		batch := optionInt(req.Options, "batch_size", 16)
		return s.fillOutlineNextRange(ctx, id, project, start, batch)
	}
	isExpandScene := strings.EqualFold(commandID, "expand_scene")

	currentText := ""
	var baseDraft *models.ScriptDraft
	if project.State.ActiveDraftID != "" {
		var d models.ScriptDraft
		if err := s.FileStorage.LoadJSONFile(filepath.Join(id, "drafts"), project.State.ActiveDraftID+".json", &d); err == nil {
			baseDraft = &d
			currentText = extractDraftSceneText(&d, req.Target.Chapter, req.Target.Scene)
		}
	}

	prevChapterDraft := ""
	currChapterOutline := ""
	if isExpandScene {
		prevChapterDraft, currChapterOutline = s.bestEffortGetExpandSceneContext(id, req.Target.Chapter)
	}

	systemPrompt := scriptBilingualSystemPrompt(strings.TrimSpace(req.Command + "\n" + req.UserInput + "\n" + currentText + "\n" + prevChapterDraft + "\n" + currChapterOutline))

	frameworkJSON, _ := json.Marshal(project.Framework)
	optionsJSON, _ := json.Marshal(req.Options)

	extraContext := ""
	if isExpandScene {
		extraContext = fmt.Sprintf("\n[previous_chapter_user_draft]\n%s\n\n[current_chapter_outline]\n%s\n\n",
			prevChapterDraft,
			currChapterOutline,
		)
	}

	userPrompt := fmt.Sprintf(
		"请根据以下信息执行写作协助。\nPlease perform writing assistance based on the following information.\n\n"+
			"[assist_mode]\n%s\n\n[command]\n%s\n\n[user_input]\n%s\n\n[current_text]\n%s\n%s[framework_json]\n%s\n\n[options_json]\n%s\n\n"+
			"请输出严格 JSON（不要额外解释）。\nOutput strict JSON only (no extra text).\n\n"+
			"要求 / Requirements:\n"+
			"- 必须包含 main_text，且为用户可直接阅读的正文（不要把 JSON 当成正文写回 main_text）。\n"+
			"  main_text must be readable prose (do NOT put JSON inside main_text).\n"+
			"- 若是扩写/补全当前章节：请将 main_text 作为“当前章节正文（user_draft）”，并自然承接上一章内容，同时严格遵循本章大纲。\n"+
			"  For chapter completion: main_text should be the current chapter draft (user_draft), consistent with previous chapter and following the current outline.\n"+
			"- branches 可选，仅在适合给出走向时提供，最多 3 条，每条不超过 220 字。\n"+
			"  branches are optional; when provided: max 3 items; keep each concise.\n"+
			"- memory_update 为可选字段，且必须极短（每个数组最多 3 条）；不确定就省略。\n"+
			"  memory_update is optional and must be VERY small; omit if unsure.\n\n"+
			"JSON schema:\n{\n  \"main_text\": \"...\",\n  \"branches\": [{\"id\":\"opt_1\",\"text\":\"...\"}],\n  \"memory_update\": {\n    \"added_facts\": [\"...\"],\n    \"open_threads\": [\"...\"],\n    \"foreshadowing\": [\"...\"],\n    \"character_state\": {\"character_id\": {\"key\": \"value\"}}\n  }\n}\n",
		req.AssistMode,
		req.Command,
		req.UserInput,
		currentText,
		extraContext,
		string(frameworkJSON),
		string(optionsJSON),
	)

	model := s.LLM.GetDefaultModel()
	maxTokens := 1200
	if isExpandScene {
		maxTokens = 2400
	}
	resp, err := s.LLM.CreateChatCompletion(ctx, ChatCompletionRequest{
		Model: model,
		Messages: []ChatCompletionMessage{
			{Role: RoleSystem, Content: systemPrompt},
			{Role: RoleUser, Content: userPrompt},
		},
		Temperature: 0.8,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, err
	}

	text := ""
	if len(resp.Choices) > 0 {
		text = strings.TrimSpace(resp.Choices[0].Message.Content)
	}

	llmOut := scriptCommandLLMResponse{}
	clean := stripCodeFences(text)
	if err := json.Unmarshal([]byte(clean), &llmOut); err != nil {
		// Avoid leaking raw JSON into drafts/console when output is truncated/invalid.
		if mainText, ok := extractJSONStringField(clean, "main_text"); ok {
			llmOut.MainText = mainText
		} else if !looksLikeJSONObject(clean) {
			// Non-JSON plain text fallback.
			llmOut.MainText = strings.TrimSpace(clean)
		} else {
			llmOut.MainText = "（模型输出解析失败：请重试该指令）"
		}
	}
	out := models.ScriptCommandOutput{
		MainText:         llmOut.MainText,
		Environment:      llmOut.Environment,
		DialogueVariants: llmOut.DialogueVariants,
		Subtext:          llmOut.Subtext,
		Branches:         llmOut.Branches,
	}

	draftID := fmt.Sprintf("draft_%d", time.Now().UnixNano())
	now := time.Now()
	content := updateDraftContent(baseDraft, req.Target.Chapter, req.Target.Scene, out.MainText)
	draft := &models.ScriptDraft{
		DraftID:   draftID,
		CreatedAt: now,
		Content:   content,
		Notes:     models.ScriptDraftNotes{UserPrompt: req.UserInput},
	}

	if err := s.FileStorage.SaveJSONFile(filepath.Join(id, "drafts"), draftID+".json", draft); err != nil {
		return nil, err
	}

	project.State.ActiveDraftID = draftID
	project.State.Cursor = models.ScriptCursor{Chapter: req.Target.Chapter, Scene: req.Target.Scene, Segment: req.Target.Segment}
	project.UpdatedAt = now
	if err := s.FileStorage.SaveJSONFile(id, "project.json", project); err != nil {
		return nil, err
	}

	if len(llmOut.MemoryUpdate) > 0 {
		if _, err := s.applyMemoryUpdate(id, llmOut.MemoryUpdate); err != nil {
			return nil, err
		}
	}

	// P0: best-effort update chapter summaries using latest output
	if err := s.applyChapterSummaryUpdate(id, req.Target.Chapter, draftID, out); err != nil {
		utils.GetLogger().Warn("scripts Command best-effort chapter summary update failed", map[string]interface{}{
			"script_id": id,
			"chapter":   req.Target.Chapter,
			"draft_id":  draftID,
			"err":       err,
		})
	}

	// P0: for expand_scene, persist chapter-level user_draft to chapter_draft.json (best-effort)
	if isExpandScene {
		s.bestEffortUpsertChapterDraftUserDraft(id, req.Target.Chapter, out.MainText)
	}

	workflowID := fmt.Sprintf("wf_%d", time.Now().UnixNano())
	// Keep full workflow history on disk; UI/API can request a limited window (workflow_limit).
	if err := s.appendWorkflowItem(id, models.ScriptWorkflowItem{
		ID:         workflowID,
		Type:       "command",
		CreatedAt:  now,
		DraftID:    draftID,
		AssistMode: req.AssistMode,
		UserInput:  req.UserInput,
		Command:    req.Command,
		Target:     req.Target,
		Output:     out,
	}); err != nil {
		utils.GetLogger().Warn("scripts Command best-effort workflow append failed", map[string]interface{}{
			"script_id":   id,
			"workflow_id": workflowID,
			"err":         err,
		})
	}
	return &models.ScriptCommandResponse{
		DraftID:        draftID,
		WorkflowItemID: workflowID,
		Output:         out,
		MemoryUpdate:   llmOut.MemoryUpdate,
	}, nil
}

func (s *ScriptService) loadWorkflow(scriptID string) (*models.ScriptWorkflow, error) {
	var wf models.ScriptWorkflow
	err := s.FileStorage.LoadJSONFile(scriptID, "workflow_items.json", &wf)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			wf = models.ScriptWorkflow{Items: []models.ScriptWorkflowItem{}}
			if saveErr := s.FileStorage.SaveJSONFile(scriptID, "workflow_items.json", &wf); saveErr != nil {
				utils.GetLogger().Warn("scripts loadWorkflow best-effort init failed", map[string]interface{}{
					"script_id": scriptID,
					"file":      "workflow_items.json",
					"err":       saveErr,
				})
			}
			return &wf, nil
		}
		return nil, err
	}
	if wf.Items == nil {
		wf.Items = []models.ScriptWorkflowItem{}
	}
	return &wf, nil
}

func (s *ScriptService) saveWorkflow(scriptID string, wf *models.ScriptWorkflow) error {
	if wf == nil {
		return nil
	}
	return s.FileStorage.SaveJSONFile(scriptID, "workflow_items.json", wf)
}

func (s *ScriptService) appendWorkflowItem(scriptID string, item models.ScriptWorkflowItem) error {
	wf, err := s.loadWorkflow(scriptID)
	if err != nil {
		return err
	}
	// P0: best-effort refs for card-style workflow (6.4)
	if len(wf.Items) > 0 {
		prevID := strings.TrimSpace(wf.Items[len(wf.Items)-1].ID)
		if prevID != "" {
			if item.Refs == nil {
				item.Refs = &models.ScriptWorkflowRefs{}
			}
			if len(item.Refs.DependsOn) == 0 {
				item.Refs.DependsOn = []string{prevID}
			}
		}
	}
	wf.Items = append(wf.Items, item)
	return s.saveWorkflow(scriptID, wf)
}

func updateDraftContent(baseDraft *models.ScriptDraft, chapter int, scene int, text string) models.ScriptDraftContent {
	var content models.ScriptDraftContent
	if baseDraft != nil {
		content = cloneDraftContent(baseDraft.Content)
	} else {
		content = models.ScriptDraftContent{Chapters: []models.ScriptChapter{}}
	}

	chapterIdx := -1
	for i := range content.Chapters {
		if content.Chapters[i].Index == chapter {
			chapterIdx = i
			break
		}
	}
	if chapterIdx == -1 {
		content.Chapters = append(content.Chapters, models.ScriptChapter{Index: chapter, Scenes: []models.ScriptScene{}})
		chapterIdx = len(content.Chapters) - 1
	}
	if content.Chapters[chapterIdx].Scenes == nil {
		content.Chapters[chapterIdx].Scenes = []models.ScriptScene{}
	}

	sceneIdx := -1
	for i := range content.Chapters[chapterIdx].Scenes {
		if content.Chapters[chapterIdx].Scenes[i].Index == scene {
			sceneIdx = i
			break
		}
	}
	if sceneIdx == -1 {
		content.Chapters[chapterIdx].Scenes = append(content.Chapters[chapterIdx].Scenes, models.ScriptScene{Index: scene, Text: text})
	} else {
		content.Chapters[chapterIdx].Scenes[sceneIdx].Text = text
	}

	return content
}

func cloneDraftContent(src models.ScriptDraftContent) models.ScriptDraftContent {
	cloned := models.ScriptDraftContent{Chapters: make([]models.ScriptChapter, 0, len(src.Chapters))}
	for _, ch := range src.Chapters {
		chCopy := models.ScriptChapter{Index: ch.Index, Title: ch.Title}
		if len(ch.Scenes) > 0 {
			chCopy.Scenes = make([]models.ScriptScene, 0, len(ch.Scenes))
			for _, sc := range ch.Scenes {
				chCopy.Scenes = append(chCopy.Scenes, models.ScriptScene{Index: sc.Index, Title: sc.Title, Text: sc.Text})
			}
		} else {
			chCopy.Scenes = []models.ScriptScene{}
		}
		cloned.Chapters = append(cloned.Chapters, chCopy)
	}
	return cloned
}

func extractDraftSceneText(draft *models.ScriptDraft, chapter int, scene int) string {
	if draft == nil {
		return ""
	}
	for _, ch := range draft.Content.Chapters {
		if ch.Index != chapter {
			continue
		}
		for _, sc := range ch.Scenes {
			if sc.Index == scene {
				return sc.Text
			}
		}
	}
	if len(draft.Content.Chapters) > 0 && len(draft.Content.Chapters[0].Scenes) > 0 {
		return draft.Content.Chapters[0].Scenes[0].Text
	}
	return ""
}

func (s *ScriptService) applyMemoryUpdate(scriptID string, update map[string]interface{}) (*models.ScriptMemory, error) {
	mem, err := s.loadMemory(scriptID)
	if err != nil {
		return nil, err
	}
	if update == nil {
		return mem, nil
	}

	addedFacts := toStringSlice(update["added_facts"])
	for _, text := range addedFacts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if memoryFactExists(mem.Facts, text) {
			continue
		}
		mem.Facts = append(mem.Facts, models.ScriptMemoryFact{
			ID:         fmt.Sprintf("fact_%d", time.Now().UnixNano()),
			Text:       text,
			Tags:       []string{},
			Confidence: 0.8,
		})
	}

	openThreads := toStringSlice(update["open_threads"])
	for _, text := range openThreads {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if memoryThreadExists(mem.OpenThreads, text) {
			continue
		}
		mem.OpenThreads = append(mem.OpenThreads, models.ScriptMemoryThread{
			ID:     fmt.Sprintf("thread_%d", time.Now().UnixNano()),
			Text:   text,
			Status: "open",
		})
	}

	foreshadowing := toStringSlice(update["foreshadowing"])
	for _, text := range foreshadowing {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if memoryForeshadowExists(mem.Foreshadowing, text) {
			continue
		}
		mem.Foreshadowing = append(mem.Foreshadowing, models.ScriptMemoryForeshadow{
			ID:     fmt.Sprintf("foreshadow_%d", time.Now().UnixNano()),
			Text:   text,
			Status: "planned",
		})
	}

	if cs, ok := update["character_state"].(map[string]interface{}); ok {
		if mem.CharacterState == nil {
			mem.CharacterState = map[string]interface{}{}
		}
		for k, v := range cs {
			mem.CharacterState[k] = v
		}
	}

	mem.UpdatedAt = time.Now()
	if err := s.saveMemory(scriptID, mem); err != nil {
		return nil, err
	}
	return mem, nil
}

func (s *ScriptService) loadMemory(scriptID string) (*models.ScriptMemory, error) {
	var mem models.ScriptMemory
	err := s.FileStorage.LoadJSONFile(scriptID, "memory.json", &mem)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			mem = models.ScriptMemory{
				Version:        1,
				UpdatedAt:      time.Now(),
				Facts:          []models.ScriptMemoryFact{},
				OpenThreads:    []models.ScriptMemoryThread{},
				Foreshadowing:  []models.ScriptMemoryForeshadow{},
				CharacterState: map[string]interface{}{},
			}
			if saveErr := s.FileStorage.SaveJSONFile(scriptID, "memory.json", &mem); saveErr != nil {
				utils.GetLogger().Warn("scripts loadMemory best-effort init failed", map[string]interface{}{
					"script_id": scriptID,
					"file":      "memory.json",
					"err":       saveErr,
				})
			}
			return &mem, nil
		}
		return nil, err
	}
	if mem.Facts == nil {
		mem.Facts = []models.ScriptMemoryFact{}
	}
	if mem.OpenThreads == nil {
		mem.OpenThreads = []models.ScriptMemoryThread{}
	}
	if mem.Foreshadowing == nil {
		mem.Foreshadowing = []models.ScriptMemoryForeshadow{}
	}
	return &mem, nil
}

func (s *ScriptService) saveMemory(scriptID string, mem *models.ScriptMemory) error {
	if mem == nil {
		return nil
	}
	return s.FileStorage.SaveJSONFile(scriptID, "memory.json", mem)
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, it := range t {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func memoryFactExists(facts []models.ScriptMemoryFact, text string) bool {
	for _, f := range facts {
		if strings.TrimSpace(f.Text) == text {
			return true
		}
	}
	return false
}

func memoryThreadExists(threads []models.ScriptMemoryThread, text string) bool {
	for _, th := range threads {
		if strings.TrimSpace(th.Text) == text {
			return true
		}
	}
	return false
}

func memoryForeshadowExists(items []models.ScriptMemoryForeshadow, text string) bool {
	for _, it := range items {
		if strings.TrimSpace(it.Text) == text {
			return true
		}
	}
	return false
}

func buildGenerateInitialOutlinePrompt(frameworkJSON string) string {
	return buildGenerateInitialOutlinePromptWithChapterCount(frameworkJSON, 0)
}

func buildGenerateInitialOutlinePromptWithChapterCount(frameworkJSON string, chapterCount int) string {
	chapterLine := "- 目标 10-12 章（短篇也请至少 8 章）/ Target 10-12 chapters (even for short stories, at least 8)\n"
	if chapterCount > 0 {
		if chapterCount < 1 {
			chapterCount = 1
		}
		if chapterCount > 300 {
			chapterCount = 300
		}
		chapterLine = fmt.Sprintf("- 目标 %d 章（短篇也请至少 8 章）/ Target %d chapters (even for short stories, at least 8)\n", chapterCount, chapterCount)
	}

	return fmt.Sprintf(
		"请根据写作框架生成一个简洁但清晰的章节大纲（偏关键点，不要急于完结）。\n"+
			"Please generate a concise but clear chapter outline (key beats; do NOT rush to finish the story).\n\n"+
			"[framework_json]\n%s\n\n"+
			"要求 / Requirements:\n"+
			"- 输出严格 JSON（不要额外解释）/ Output strict JSON only (no extra text)\n"+
			"%s"+
			"- 每章 summary 用一句话描述关键冲突/转折（不要写长段正文）/ Each chapter summary should be ONE sentence describing the key conflict/turning point (no long prose)\n"+
			"- 每章 outline 用 2-4 条要点列表（用 \\n 分隔，每条一句话，尽量短）\n"+
			"  Each chapter outline should be 2-4 key beats (separated by \\n; one sentence per line; keep it short)\n"+
			"- 保持节奏“慢推进”：不要在前 3 章就收束结局 / Slow pacing: do NOT resolve the ending within the first 3 chapters\n\n"+
			"JSON schema:\n{\n  \"version\": \"v1\",\n  \"chapters\": [\n    {\"index\":1,\"title\":\"...\",\"summary\":\"...\",\"outline\":\"- ...\\n- ...\"}\n  ]\n}\n",
		frameworkJSON,
		chapterLine,
	)
}

func buildGenerateInitialOutlinePromptForRange(frameworkJSON string, startChapter int, endChapter int, totalChapters int) string {
	if startChapter < 1 {
		startChapter = 1
	}
	if endChapter < startChapter {
		endChapter = startChapter
	}
	if totalChapters < endChapter {
		totalChapters = endChapter
	}
	if totalChapters > 300 {
		totalChapters = 300
	}

	return fmt.Sprintf(
		"请根据写作框架生成章节大纲 JSON，但只输出第 %d-%d 章（共 %d 章）。\n"+
			"Please generate the outline JSON, but output ONLY chapters %d-%d (total %d chapters).\n\n"+
			"[framework_json]\n%s\n\n"+
			"要求 / Requirements:\n"+
			"- 输出严格 JSON（不要额外解释）/ Output strict JSON only (no extra text)\n"+
			"- 只输出 chapters 数组中 index=%d..%d 的条目（包含边界），不要输出其它章节\n"+
			"  Output ONLY chapters with index=%d..%d (inclusive); do not include other chapters\n"+
			"- 剧情推进要按比例：第 %d-%d 章只覆盖全书前段进度（不要在此范围内写完结局或大结局）\n"+
			"  Proportional pacing: chapters %d-%d should cover only the early portion of the whole story (do NOT conclude the ending in this range)\n"+
			"- 每章 summary 用一句话描述关键冲突/转折（不要写长段正文）\n"+
			"  Each chapter summary should be ONE sentence (no long prose)\n"+
			"- 每章 outline 用 2-4 条要点列表（用 \\\n 分隔，每条一句话，尽量短）\n"+
			"  Each chapter outline should be 2-4 key beats (separated by \\\n; one sentence per line; keep it short)\n\n"+
			"JSON schema:\n{\n  \"version\": \"v1\",\n  \"chapters\": [\n    {\"index\":1,\"title\":\"...\",\"summary\":\"...\",\"outline\":\"- ...\\n- ...\"}\n  ]\n}\n",
		startChapter,
		endChapter,
		totalChapters,
		startChapter,
		endChapter,
		totalChapters,
		frameworkJSON,
		startChapter,
		endChapter,
		startChapter,
		endChapter,
		startChapter,
		endChapter,
		startChapter,
		endChapter,
	)
}

func formatOutlineContextForPrompt(outline *scriptOutline, lastN int, beforeChapter int) string {
	if outline == nil || len(outline.Chapters) == 0 {
		return ""
	}
	if lastN <= 0 {
		lastN = 6
	}
	if beforeChapter <= 1 {
		return ""
	}

	start := beforeChapter - lastN
	if start < 1 {
		start = 1
	}
	end := beforeChapter - 1
	if end < start {
		return ""
	}

	var b strings.Builder
	for i := start; i <= end; i++ {
		var ch *scriptOutlineChapter
		for j := range outline.Chapters {
			if outline.Chapters[j].Index == i {
				ch = &outline.Chapters[j]
				break
			}
		}
		if ch == nil {
			continue
		}
		title := strings.TrimSpace(ch.Title)
		summary := strings.TrimSpace(ch.Summary)
		ol := strings.TrimSpace(ch.Outline)
		if title == "" && summary == "" && ol == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- 第%d章 %s\n", i, title))
		if summary != "" {
			b.WriteString("  summary: ")
			b.WriteString(buildBriefSummary(summary, 220))
			b.WriteString("\n")
		}
		if ol != "" {
			b.WriteString("  outline: ")
			b.WriteString(buildBriefSummary(strings.ReplaceAll(ol, "\n", " "), 260))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func buildGenerateInitialOutlinePromptForRangeWithContext(frameworkJSON string, startChapter int, endChapter int, totalChapters int, existingOutlineContext string) string {
	base := buildGenerateInitialOutlinePromptForRange(frameworkJSON, startChapter, endChapter, totalChapters)
	existingOutlineContext = strings.TrimSpace(existingOutlineContext)
	if existingOutlineContext == "" {
		return base
	}
	return fmt.Sprintf("%s\n[existing_outline_context]\n%s\n\n连续性要求 / Continuity:\n- 新生成的章节必须自然承接 existing_outline_context 的剧情推进与伏笔。\n- 不要改写已有章节的核心事件；只补全 %d-%d 章。\n",
		base,
		existingOutlineContext,
		startChapter,
		endChapter,
	)
}

func estimateOutlineBatchMaxTokens(batchSize int) int {
	// Conservative defaults to reduce truncation; providers have different caps.
	if batchSize <= 6 {
		return 1200
	}
	if batchSize <= 10 {
		return 1600
	}
	if batchSize <= 14 {
		return 2200
	}
	if batchSize <= 20 {
		return 3200
	}
	return 4096
}

func chooseOutlineBatchSize(totalChapters int) int {
	// Smaller batches are much more stable with long outlines.
	if totalChapters <= 30 {
		return 20
	}
	if totalChapters <= 80 {
		return 16
	}
	return 12
}

func countMeaningfulOutlineChaptersInRange(outline *scriptOutline, startChapter int, endChapter int) int {
	if outline == nil {
		return 0
	}
	count := 0
	for _, ch := range outline.Chapters {
		if ch.Index < startChapter || ch.Index > endChapter {
			continue
		}
		if strings.TrimSpace(ch.Title) == "" {
			continue
		}
		if strings.TrimSpace(ch.Summary) == "" {
			continue
		}
		count++
	}
	return count
}

func buildGenerateInitialSceneKeyBeatsPrompt(frameworkJSON string, ch scriptOutlineChapter) string {
	return fmt.Sprintf(
		"请根据写作框架与大纲，只输出第 1 章第 1 场的【关键剧情点】（慢演绎，不要急着写完）。\n"+
			"Please output ONLY the key beats for Chapter 1, Scene 1 (slow pacing; do NOT rush).\n\n"+
			"[framework_json]\n%s\n\n[chapter]\n%+v\n\n"+
			"输出要求 / Output requirements:\n"+
			"- 直接输出文本（不要 JSON，不要标题，不要解释）/ Output plain text only (no JSON, no title, no explanation)\n"+
			"- 用 6-10 条要点列表（每条一句话），覆盖：场景目标/冲突/信息差/动作/对白钩子/伏笔\n"+
			"  Use 6-10 bullet points (one sentence each), covering: goal/conflict/info-gap/action/dialogue hook/foreshadowing\n"+
			"- 不要总结全篇，不要写结局，不要跳到后续章节 / Do not conclude the whole story; no ending; do not skip ahead\n",
		frameworkJSON,
		ch,
	)
}

// GenerateInitial 执行“先大纲后正文”的最小生成（用于 /api/scripts/:id/generate）。
// 说明：该方法设计为可被后台 goroutine 调用，进度通过 tracker 推送。
func (s *ScriptService) GenerateInitial(ctx context.Context, id string, tracker *ProgressTracker) error {
	if tracker != nil {
		tracker.UpdateProgress(5, "加载项目")
	}

	project, err := s.GetProject(ctx, id)
	if err != nil {
		return err
	}

	if s.LLM == nil {
		return fmt.Errorf("%w: LLM service unavailable", ErrLLMNotReady)
	}
	if !s.LLM.IsReady() {
		return fmt.Errorf("%w: %s", ErrLLMNotReady, s.LLM.GetReadyState())
	}

	frameworkJSON, _ := json.Marshal(project.Framework)
	systemPrompt := scriptBilingualSystemPrompt(strings.TrimSpace(project.Title))

	if tracker != nil {
		tracker.UpdateProgress(15, "生成大纲")
	}

	desiredChapters := readOptionalChapterCount(project.Framework)
	// optimization2.md: Generate only the first 20 chapters' outline initially,
	// but keep placeholders for the full chapter count so UI shows blank chapters.
	totalChapters := desiredChapters
	if totalChapters <= 0 {
		totalChapters = 20
	}
	if totalChapters < 8 {
		totalChapters = 8
	}
	if totalChapters > 300 {
		totalChapters = 300
	}
	initialEnd := totalChapters
	if initialEnd > 20 {
		initialEnd = 20
	}
	model := s.LLM.GetDefaultModel()

	var outline scriptOutline
	start := 1
	end := initialEnd
	if tracker != nil {
		tracker.UpdateProgress(25, fmt.Sprintf("生成大纲（第%d-%d章；共%d章）", start, end, totalChapters))
	}
	outPrompt := buildGenerateInitialOutlinePromptForRange(string(frameworkJSON), start, end, totalChapters)
	var part *scriptOutline
	for attempt := 0; attempt < 3; attempt++ {
		outlineResp, err := s.LLM.CreateChatCompletion(ctx, ChatCompletionRequest{
			Model: model,
			Messages: []ChatCompletionMessage{
				{Role: RoleSystem, Content: systemPrompt},
				{Role: RoleUser, Content: outPrompt},
			},
			Temperature: 0.7,
			MaxTokens:   estimateOutlineBatchMaxTokens(end - start + 1),
		})
		if err != nil {
			return err
		}
		outlineText := ""
		if len(outlineResp.Choices) > 0 {
			outlineText = strings.TrimSpace(outlineResp.Choices[0].Message.Content)
		}
		p, ok := parseScriptOutlineBestEffort(outlineText)
		if !ok {
			continue
		}
		parsedCount := countMeaningfulOutlineChaptersInRange(&p, start, end)
		rangeSize := end - start + 1
		if rangeSize > 0 && parsedCount*100 >= rangeSize*70 {
			part = &p
			break
		}
		part = &p
	}

	byIndex := map[int]scriptOutlineChapter{}
	if part != nil {
		for _, ch := range part.Chapters {
			if ch.Index < start || ch.Index > end {
				continue
			}
			byIndex[ch.Index] = ch
		}
	}
	chapters := make([]scriptOutlineChapter, 0, totalChapters)
	for i := 1; i <= totalChapters; i++ {
		ch, ok := byIndex[i]
		if !ok {
			ch = scriptOutlineChapter{Index: i}
		}
		chapters = append(chapters, ch)
	}
	outline = scriptOutline{Version: "v1", Chapters: chapters}
	normalizeScriptOutline(&outline, totalChapters)

	// Best-effort: initialize chapter_draft.json for later per-chapter editing.
	// Do not fail the whole generation if this write fails.
	if len(outline.Chapters) > 0 {
		chapterDraft := models.ScriptChapterDraft{}
		for _, ch := range outline.Chapters {
			chapterDraft.Chapters = append(chapterDraft.Chapters, models.ScriptChapterDraftChapter{
				Index:     ch.Index,
				Title:     strings.TrimSpace(ch.Title),
				Summary:   strings.TrimSpace(ch.Summary),
				Outline:   strings.TrimSpace(ch.Outline),
				UserDraft: "",
			})
		}
		if err := s.FileStorage.SaveJSONFile(id, "chapter_draft.json", &chapterDraft); err != nil {
			utils.GetLogger().Warn("scripts GenerateInitial best-effort chapter_draft.json init failed", map[string]interface{}{
				"script_id": id,
				"err":       err.Error(),
			})
		}
	}

	if tracker != nil {
		tracker.UpdateProgress(60, "生成第1章第1场关键点")
	}

	ch1 := outline.Chapters[0]
	bodyPrompt := buildGenerateInitialSceneKeyBeatsPrompt(string(frameworkJSON), ch1)

	bodyResp, err := s.LLM.CreateChatCompletion(ctx, ChatCompletionRequest{
		Model: model,
		Messages: []ChatCompletionMessage{
			{Role: RoleSystem, Content: systemPrompt},
			{Role: RoleUser, Content: bodyPrompt},
		},
		Temperature: 0.6,
		MaxTokens:   900,
	})
	if err != nil {
		return err
	}

	mainText := ""
	if len(bodyResp.Choices) > 0 {
		mainText = strings.TrimSpace(bodyResp.Choices[0].Message.Content)
	}
	if mainText == "" {
		mainText = "（生成失败：正文为空）"
	}

	draftID := fmt.Sprintf("draft_%d", time.Now().UnixNano())
	now := time.Now()
	draft := &models.ScriptDraft{
		DraftID:   draftID,
		CreatedAt: now,
		Content: models.ScriptDraftContent{
			Chapters: []models.ScriptChapter{
				{
					Index:  1,
					Title:  ch1.Title,
					Scenes: []models.ScriptScene{{Index: 1, Text: mainText}},
				},
			},
		},
		Notes: models.ScriptDraftNotes{UserPrompt: "generate_initial"},
	}

	if err := s.FileStorage.SaveJSONFile(filepath.Join(id, "drafts"), draftID+".json", draft); err != nil {
		return err
	}

	// P0: best-effort chapter summary after initial generation
	if err := s.applyChapterSummaryUpdate(id, 1, draftID, models.ScriptCommandOutput{MainText: mainText, Branches: []models.ScriptBranchOption{}}); err != nil {
		utils.GetLogger().Warn("scripts GenerateInitial best-effort chapter summary update failed", map[string]interface{}{
			"script_id": id,
			"chapter":   1,
			"draft_id":  draftID,
			"err":       err,
		})
	}

	project.State.ActiveDraftID = draftID
	project.State.Cursor = models.ScriptCursor{Chapter: 1, Scene: 1, Segment: 0}
	project.UpdatedAt = now
	if err := s.FileStorage.SaveJSONFile(id, "project.json", project); err != nil {
		return err
	}

	if tracker != nil {
		tracker.UpdateProgress(95, "写入草稿与更新状态")
	}

	return nil
}

func (s *ScriptService) fillOutlineNextRange(ctx context.Context, id string, project *models.ScriptProject, startChapter int, batchSize int) (*models.ScriptCommandResponse, error) {
	if project == nil || project.Framework == nil {
		return nil, fmt.Errorf("script project missing framework")
	}
	desiredChapters := readOptionalChapterCount(project.Framework)
	if desiredChapters <= 0 {
		desiredChapters = 1
	}
	if batchSize <= 0 {
		batchSize = 16
	}
	if batchSize > 20 {
		batchSize = 20
	}

	// chapter_draft.json is the single source of truth for outline continuity.
	cd, err := s.bestEffortLoadChapterDraft(id)
	if err != nil {
		return nil, err
	}
	userDraftByIndex := map[int]string{}
	outline := scriptOutline{Version: "v1", Chapters: []scriptOutlineChapter{}}
	if cd != nil {
		for _, ch := range cd.Chapters {
			userDraftByIndex[ch.Index] = ch.UserDraft
			outline.Chapters = append(outline.Chapters, scriptOutlineChapter{
				Index:   ch.Index,
				Title:   strings.TrimSpace(ch.Title),
				Summary: strings.TrimSpace(ch.Summary),
				Outline: strings.TrimSpace(ch.Outline),
			})
		}
	}
	normalizeScriptOutline(&outline, desiredChapters)

	// optimization2.md: must always start from the first empty chapter.
	startChapter = 1
	for startChapter <= desiredChapters {
		filled := false
		for _, ch := range outline.Chapters {
			if ch.Index != startChapter {
				continue
			}
			if strings.TrimSpace(ch.Title) != "" && strings.TrimSpace(ch.Summary) != "" {
				filled = true
			}
			break
		}
		if !filled {
			break
		}
		startChapter++
	}
	if startChapter > desiredChapters {
		out := models.ScriptCommandOutput{MainText: fmt.Sprintf("大纲已全部补全：共 %d 章。", desiredChapters)}
		return &models.ScriptCommandResponse{DraftID: "", WorkflowItemID: "", Output: out}, nil
	}
	endChapter := startChapter + batchSize - 1
	if endChapter > desiredChapters {
		endChapter = desiredChapters
	}
	if startChapter > endChapter {
		out := models.ScriptCommandOutput{MainText: "该范围内章节已存在，无需补全。"}
		return &models.ScriptCommandResponse{DraftID: "", WorkflowItemID: "", Output: out}, nil
	}

	frameworkJSON, _ := json.Marshal(project.Framework)
	systemPrompt := scriptBilingualSystemPrompt(strings.TrimSpace(project.Title))
	existingCtx := formatOutlineContextForPrompt(&outline, 6, startChapter)
	prompt := buildGenerateInitialOutlinePromptForRangeWithContext(string(frameworkJSON), startChapter, endChapter, desiredChapters, existingCtx)

	model := s.LLM.GetDefaultModel()
	var part *scriptOutline
	for attempt := 0; attempt < 3; attempt++ {
		outlineResp, err := s.LLM.CreateChatCompletion(ctx, ChatCompletionRequest{
			Model: model,
			Messages: []ChatCompletionMessage{
				{Role: RoleSystem, Content: systemPrompt},
				{Role: RoleUser, Content: prompt},
			},
			Temperature: 0.7,
			MaxTokens:   estimateOutlineBatchMaxTokens(endChapter - startChapter + 1),
		})
		if err != nil {
			return nil, err
		}
		outlineText := ""
		if len(outlineResp.Choices) > 0 {
			outlineText = strings.TrimSpace(outlineResp.Choices[0].Message.Content)
		}
		p, ok := parseScriptOutlineBestEffort(outlineText)
		if !ok {
			continue
		}
		parsedCount := countMeaningfulOutlineChaptersInRange(&p, startChapter, endChapter)
		rangeSize := endChapter - startChapter + 1
		part = &p
		if rangeSize > 0 && parsedCount*100 >= rangeSize*70 {
			break
		}
	}
	if part == nil {
		out := models.ScriptCommandOutput{MainText: "（补全失败：模型输出解析失败，请重试）"}
		return &models.ScriptCommandResponse{DraftID: "", WorkflowItemID: "", Output: out}, nil
	}

	// Merge.
	for _, ch := range part.Chapters {
		if ch.Index < startChapter || ch.Index > endChapter {
			continue
		}
		merged := false
		for i := range outline.Chapters {
			if outline.Chapters[i].Index != ch.Index {
				continue
			}
			if strings.TrimSpace(outline.Chapters[i].Title) == "" {
				outline.Chapters[i].Title = ch.Title
			}
			if strings.TrimSpace(outline.Chapters[i].Summary) == "" {
				outline.Chapters[i].Summary = ch.Summary
			}
			if strings.TrimSpace(outline.Chapters[i].Outline) == "" {
				outline.Chapters[i].Outline = ch.Outline
			}
			merged = true
			break
		}
		if !merged {
			outline.Chapters = append(outline.Chapters, ch)
		}
	}
	normalizeScriptOutline(&outline, desiredChapters)

	// Persist back into chapter_draft.json (keep existing user_draft per chapter).
	newCD := models.ScriptChapterDraft{Chapters: make([]models.ScriptChapterDraftChapter, 0, len(outline.Chapters))}
	for _, ch := range outline.Chapters {
		newCD.Chapters = append(newCD.Chapters, models.ScriptChapterDraftChapter{
			Index:     ch.Index,
			Title:     strings.TrimSpace(ch.Title),
			Summary:   strings.TrimSpace(ch.Summary),
			Outline:   strings.TrimSpace(ch.Outline),
			UserDraft: strings.TrimSpace(userDraftByIndex[ch.Index]),
		})
	}
	if err := s.FileStorage.SaveJSONFile(id, "chapter_draft.json", &newCD); err != nil {
		return nil, err
	}

	// Update project timestamp (active draft unchanged).
	project.UpdatedAt = time.Now()
	_ = s.FileStorage.SaveJSONFile(id, "project.json", project)

	workflowID := fmt.Sprintf("wf_%d", time.Now().UnixNano())
	out := models.ScriptCommandOutput{MainText: fmt.Sprintf("已补全大纲：第%d-%d章（可重复执行继续补全）。", startChapter, endChapter)}
	_ = s.appendWorkflowItem(id, models.ScriptWorkflowItem{
		ID:         workflowID,
		Type:       "fill_outline",
		CreatedAt:  time.Now(),
		DraftID:    "",
		AssistMode: "system",
		UserInput:  "",
		Command:    "fill_outline_next",
		Target:     models.ScriptCommandTarget{Chapter: startChapter, Scene: 0, Segment: 0},
		Output:     out,
	})

	return &models.ScriptCommandResponse{
		DraftID:        "",
		WorkflowItemID: workflowID,
		Output:         out,
	}, nil
}

func (s *ScriptService) applyChapterSummaryUpdate(scriptID string, chapter int, draftID string, out models.ScriptCommandOutput) error {
	sums, err := s.loadChapterSummaries(scriptID)
	if err != nil {
		return err
	}
	if chapter <= 0 {
		chapter = 1
	}

	summary := buildBriefSummary(out.MainText, 160)
	next := out.Branches
	if len(next) > 5 {
		next = next[:5]
	}

	idx := -1
	for i := range sums.Items {
		if sums.Items[i].Chapter == chapter {
			idx = i
			break
		}
	}
	item := models.ScriptChapterSummaryItem{
		Chapter:       chapter,
		DraftID:       draftID,
		Summary:       summary,
		ConflictState: "",
		NextOptions:   next,
	}
	if idx == -1 {
		sums.Items = append(sums.Items, item)
	} else {
		sums.Items[idx] = item
	}
	return s.saveChapterSummaries(scriptID, sums)
}

func (s *ScriptService) loadChapterSummaries(scriptID string) (*models.ScriptChapterSummaries, error) {
	var sums models.ScriptChapterSummaries
	err := s.FileStorage.LoadJSONFile(scriptID, "chapter_summaries.json", &sums)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			sums = models.ScriptChapterSummaries{Items: []models.ScriptChapterSummaryItem{}}
			if saveErr := s.FileStorage.SaveJSONFile(scriptID, "chapter_summaries.json", &sums); saveErr != nil {
				utils.GetLogger().Warn("scripts loadChapterSummaries best-effort init failed", map[string]interface{}{
					"script_id": scriptID,
					"file":      "chapter_summaries.json",
					"err":       saveErr,
				})
			}
			return &sums, nil
		}
		return nil, err
	}
	if sums.Items == nil {
		sums.Items = []models.ScriptChapterSummaryItem{}
	}
	return &sums, nil
}

func (s *ScriptService) saveChapterSummaries(scriptID string, sums *models.ScriptChapterSummaries) error {
	if sums == nil {
		return nil
	}
	return s.FileStorage.SaveJSONFile(scriptID, "chapter_summaries.json", sums)
}

func buildBriefSummary(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	r := []rune(text)
	if maxRunes <= 0 || len(r) <= maxRunes {
		return text
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

func (s *ScriptService) Rewind(ctx context.Context, id string, draftID string) (*models.ScriptProject, error) {
	project, err := s.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(draftID) == "" {
		return nil, fmt.Errorf("draft_id is required")
	}

	var draft models.ScriptDraft
	if err := s.FileStorage.LoadJSONFile(filepath.Join(id, "drafts"), draftID+".json", &draft); err != nil {
		if os.IsNotExist(unwrapPathError(err)) {
			return nil, fmt.Errorf("draft not found")
		}
		return nil, err
	}

	chapter := 1
	scene := 1
	segment := 0
	if len(draft.Content.Chapters) > 0 {
		chapter = draft.Content.Chapters[0].Index
		if len(draft.Content.Chapters[0].Scenes) > 0 {
			scene = draft.Content.Chapters[0].Scenes[0].Index
		}
	}

	project.State.ActiveDraftID = draftID
	project.State.Cursor = models.ScriptCursor{Chapter: chapter, Scene: scene, Segment: segment}
	project.UpdatedAt = time.Now()
	if err := s.FileStorage.SaveJSONFile(id, "project.json", project); err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ScriptService) Export(ctx context.Context, id string, format string) (*models.ExportResult, error) {
	return s.ExportWithMeta(ctx, id, format, false)
}

func (s *ScriptService) ExportWithMeta(ctx context.Context, id string, format string, includeMeta bool) (*models.ExportResult, error) {
	project, err := s.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	if project.State.ActiveDraftID == "" {
		return nil, fmt.Errorf("no active draft")
	}

	var draft models.ScriptDraft
	if err := s.FileStorage.LoadJSONFile(filepath.Join(id, "drafts"), project.State.ActiveDraftID+".json", &draft); err != nil {
		return nil, err
	}

	ext := strings.ToLower(strings.TrimSpace(format))
	if ext == "" {
		ext = "markdown"
	}

	content := ""
	switch ext {
	case "txt":
		content = s.buildTXT(project, &draft)
		if includeMeta {
			content = s.appendExportMeta(id, "txt", content)
		}
	case "html":
		md := s.buildMarkdown(project, &draft)
		if includeMeta {
			md = s.appendExportMeta(id, "markdown", md)
		}
		content = renderTextToHTMLDocument(md)
		// includeMeta already applied on markdown before conversion
	default:
		content = s.buildMarkdown(project, &draft)
		ext = "markdown"
		if includeMeta {
			content = s.appendExportMeta(id, "markdown", content)
		}
	}

	filename := fmt.Sprintf("script_%s_%d.%s", project.ID, time.Now().UnixNano(), map[string]string{"markdown": "md", "txt": "txt", "html": "html"}[ext])
	if strings.HasSuffix(filename, ".") {
		filename = fmt.Sprintf("script_%s_%d.md", project.ID, time.Now().UnixNano())
	}

	if err := s.FileStorage.SaveTextFile(filepath.Join(id, "exports"), filename, []byte(content)); err != nil {
		return nil, err
	}

	result := &models.ExportResult{
		SceneID:     project.ID,
		Title:       project.Title,
		Format:      ext,
		Content:     content,
		GeneratedAt: time.Now(),
		ExportType:  "script",
		FilePath:    filepath.Join(id, "exports", filename),
		FileSize:    int64(len(content)),
	}
	return result, nil
}

func renderTextToHTMLDocument(text string) string {
	// No third-party markdown renderer: we wrap the exported text into a <pre> block.
	// This preserves formatting while keeping the output a valid downloadable HTML document.
	escaped := html.EscapeString(text)

	return "<!doctype html>\n" +
		"<html lang=\"zh-CN\">\n" +
		"<head>\n" +
		"  <meta charset=\"utf-8\">\n" +
		"  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n" +
		"  <title>Export</title>\n" +
		"</head>\n" +
		"<body>\n" +
		"<pre>" + escaped + "</pre>\n" +
		"</body>\n" +
		"</html>\n"
}

func (s *ScriptService) appendExportMeta(scriptID string, format string, content string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "markdown"
	}

	var mem models.ScriptMemory
	memOK := s.FileStorage.LoadJSONFile(scriptID, "memory.json", &mem) == nil
	var sums models.ScriptChapterSummaries
	sumsOK := s.FileStorage.LoadJSONFile(scriptID, "chapter_summaries.json", &sums) == nil
	var cd models.ScriptChapterDraft
	cdOK := s.FileStorage.LoadJSONFile(scriptID, "chapter_draft.json", &cd) == nil

	if !memOK && !sumsOK && !cdOK {
		return content
	}

	if format == "txt" {
		var b strings.Builder
		b.WriteString(strings.TrimRight(content, "\n"))
		b.WriteString("\n\n")
		b.WriteString("==== 附录 / Appendix ====\n")
		if memOK {
			b.WriteString("\n[memory]\n")
			b.WriteString(renderAsPrettyJSON(mem))
			b.WriteString("\n")
		}
		if sumsOK {
			b.WriteString("\n[chapter_summaries]\n")
			b.WriteString(renderAsPrettyJSON(sums))
			b.WriteString("\n")
		}
		if cdOK {
			b.WriteString("\n[chapter_draft]\n")
			b.WriteString(renderAsPrettyJSON(cd))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		return b.String()
	}

	// markdown
	var b strings.Builder
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteString("\n\n---\n\n")
	b.WriteString("## 附录 / Appendix\n\n")
	if memOK {
		b.WriteString("### memory.json\n\n```json\n")
		b.WriteString(renderAsPrettyJSON(mem))
		b.WriteString("\n```\n\n")
	}
	if sumsOK {
		b.WriteString("### chapter_summaries.json\n\n```json\n")
		b.WriteString(renderAsPrettyJSON(sums))
		b.WriteString("\n```\n\n")
	}
	if cdOK {
		b.WriteString("### chapter_draft.json\n\n```json\n")
		b.WriteString(renderAsPrettyJSON(cd))
		b.WriteString("\n```\n\n")
	}
	return b.String()
}

func renderAsPrettyJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (s *ScriptService) buildMarkdown(project *models.ScriptProject, draft *models.ScriptDraft) string {
	var b strings.Builder
	if project.Title != "" {
		b.WriteString("# ")
		b.WriteString(project.Title)
		b.WriteString("\n\n")
	}
	for _, ch := range sortedChapters(draft) {
		chTitle := ch.Title
		if strings.TrimSpace(chTitle) == "" {
			chTitle = fmt.Sprintf("第%d章", ch.Index)
		}
		b.WriteString("## ")
		b.WriteString(chTitle)
		b.WriteString("\n\n")
		for _, sc := range sortedScenes(ch) {
			scTitle := sc.Title
			if strings.TrimSpace(scTitle) == "" {
				scTitle = fmt.Sprintf("场景%d", sc.Index)
			}
			b.WriteString("### ")
			b.WriteString(scTitle)
			b.WriteString("\n\n")
			b.WriteString(strings.TrimSpace(sc.Text))
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func (s *ScriptService) buildTXT(project *models.ScriptProject, draft *models.ScriptDraft) string {
	var b strings.Builder
	if project.Title != "" {
		b.WriteString(project.Title)
		b.WriteString("\n\n")
	}
	for _, ch := range sortedChapters(draft) {
		chTitle := ch.Title
		if strings.TrimSpace(chTitle) == "" {
			chTitle = fmt.Sprintf("第%d章", ch.Index)
		}
		b.WriteString(chTitle)
		b.WriteString("\n\n")
		for _, sc := range sortedScenes(ch) {
			b.WriteString(strings.TrimSpace(sc.Text))
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func sortedChapters(draft *models.ScriptDraft) []models.ScriptChapter {
	if draft == nil || len(draft.Content.Chapters) == 0 {
		return []models.ScriptChapter{}
	}
	chs := make([]models.ScriptChapter, len(draft.Content.Chapters))
	copy(chs, draft.Content.Chapters)
	sort.Slice(chs, func(i, j int) bool { return chs[i].Index < chs[j].Index })
	return chs
}

func sortedScenes(ch models.ScriptChapter) []models.ScriptScene {
	if len(ch.Scenes) == 0 {
		return []models.ScriptScene{}
	}
	scs := make([]models.ScriptScene, len(ch.Scenes))
	copy(scs, ch.Scenes)
	sort.Slice(scs, func(i, j int) bool { return scs[i].Index < scs[j].Index })
	return scs
}

func unwrapPathError(err error) error {
	// best-effort unwrap for os.PathError
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return err
}
