// internal/services/comic_service.go
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Corphon/SceneIntruderMCP/internal/llm/prompts"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

var (
	ErrNotImplemented          = errors.New("not implemented")
	ErrComicRepositoryNotReady = errors.New("comic repository not initialized")
	ErrComicServiceNotReady    = errors.New("comic service dependencies not ready")
)

type nodeContentBuildResult struct {
	Content         string
	UsedNodeIDs     []string
	ConversationIDs []string
	ContentHashes   []string
	Truncated       bool
}

type ComicPromptsBuildOptions struct {
	NodeID         string
	Style          string
	ContinuityMode string
}

type ComicGenerateOptions struct {
	Resume bool
}

// ComicService 是 v2 comics 领域服务的入口。
// Phase1 只提供最小骨架与依赖注入，占位方法先返回 ErrNotImplemented。
type ComicService struct {
	Repo     *ComicRepository
	JobQueue *JobQueue
	Progress *ProgressService
	LLM      *LLMService
	Vision   *VisionService
	Scene    *SceneService
	Story    *StoryService
}

func NewComicService(
	repo *ComicRepository,
	jobQueue *JobQueue,
	progress *ProgressService,
	llm *LLMService,
	vision *VisionService,
	scene *SceneService,
	story *StoryService,
) *ComicService {
	return &ComicService{
		Repo:     repo,
		JobQueue: jobQueue,
		Progress: progress,
		LLM:      llm,
		Vision:   vision,
		Scene:    scene,
		Story:    story,
	}
}

// EnsureSceneLayout 提前创建 `data/comics/scene_<id>/...` 目录结构。
func (s *ComicService) EnsureSceneLayout(sceneID string) (string, error) {
	if s.Repo == nil {
		return "", ErrComicRepositoryNotReady
	}
	return s.Repo.EnsureSceneLayout(sceneID)
}

func normalizeBreakdown(b *models.ComicBreakdown, sceneID string, targetFrames int) {
	if b == nil {
		return
	}
	if b.SceneID == "" {
		b.SceneID = sceneID
	}
	if targetFrames <= 0 {
		targetFrames = 4
	}
	if targetFrames < 4 {
		targetFrames = 4
	}
	if targetFrames > 12 {
		targetFrames = 12
	}
	b.TargetFrames = targetFrames

	frames := b.Frames
	if len(frames) > targetFrames {
		frames = frames[:targetFrames]
	}
	for i := 0; i < len(frames); i++ {
		if frames[i].ID == "" {
			frames[i].ID = fmt.Sprintf("frame_%d", i+1)
		}
		if frames[i].Order <= 0 {
			frames[i].Order = i + 1
		}
	}
	for i := len(frames); i < targetFrames; i++ {
		frames = append(frames, models.ComicFramePlan{
			ID:          fmt.Sprintf("frame_%d", i+1),
			Order:       i + 1,
			Description: "",
		})
	}
	b.Frames = frames
}

func sanitizeTargetFrames(targetFrames int) int {
	if targetFrames <= 0 {
		return 4
	}
	if targetFrames < 4 {
		return 4
	}
	if targetFrames > 12 {
		return 12
	}
	return targetFrames
}

func (s *ComicService) ensureReady() error {
	if s.Repo == nil || s.JobQueue == nil || s.Progress == nil || s.LLM == nil || s.Scene == nil || s.Story == nil {
		return ErrComicServiceNotReady
	}
	return nil
}

func (s *ComicService) ensureLLMReady() error {
	return s.ensureReady()
}

func (s *ComicService) ensureVisionReady() error {
	if s.Repo == nil || s.JobQueue == nil || s.Progress == nil || s.Vision == nil {
		return ErrComicServiceNotReady
	}
	return nil
}

func resolveConversationNodeIDForComic(conv models.Conversation) string {
	if trimmed := strings.TrimSpace(conv.NodeID); trimmed != "" {
		return trimmed
	}
	if conv.Metadata != nil {
		if raw, ok := conv.Metadata["node_id"].(string); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func isComicStoryConversation(conv models.Conversation) bool {
	if conv.Metadata != nil {
		if convType, ok := conv.Metadata["conversation_type"].(string); ok {
			convType = strings.ToLower(strings.TrimSpace(convType))
			if convType == "story_original" || convType == "story_console" {
				return true
			}
		}
	}
	// Fallback: keep story narrator entries.
	return strings.EqualFold(strings.TrimSpace(conv.SpeakerID), "story")
}

func resolveLatestNodeIDFromConversations(convs []models.Conversation) string {
	for i := len(convs) - 1; i >= 0; i-- {
		id := resolveConversationNodeIDForComic(convs[i])
		if id != "" {
			return id
		}
	}
	return ""
}

func applyStyleToPrompt(prompt string, style string) string {
	prompt = strings.TrimSpace(prompt)
	style = strings.TrimSpace(style)
	if prompt == "" {
		return prompt
	}
	if style == "" {
		return prompt
	}
	lowerStyle := strings.ToLower(style)

	knownStyles := []string{
		"comic", "ink-wash", "cel-shaded", "line-art", "noir", "cinematic", "realistic", "oil-painting", "watercolor", "gouache",
		"pixel-art", "low-poly", "isometric", "3d-render", "clay", "papercut", "ukiyo-e", "sumi-e", "steampunk", "cyberpunk",
		"dieselpunk", "fantasy", "dark-fantasy", "sci-fi", "retro", "retro-futurism", "vaporwave", "minimal", "storybook", "chibi",
		"anime", "manga", "sketch", "charcoal", "pastel", "poster", "surreal", "expressionism",
	}

	trimmedPrompt := prompt
	if lowerStyle != "comic" {
		lowerTrimmed := strings.ToLower(trimmedPrompt)
		leadingBiases := []string{
			"comic style,",
			"comic style:",
			"comic style.",
			"comic style;",
			"in comic style,",
			"in comic style:",
			"in comic style.",
			"in comic style;",
			"comic panel of ",
			"a comic panel of ",
			"an comic panel of ",
			"comic illustration of ",
			"a comic illustration of ",
			"an comic illustration of ",
			"graphic novel style,",
			"graphic novel style:",
			"in graphic novel style,",
			"in graphic novel style:",
			"graphic novel panel of ",
			"a graphic novel panel of ",
			"western comic style,",
			"western comic style:",
			"in western comic style,",
			"in western comic style:",
			"manga panel of ",
			"a manga panel of ",
			"an manga panel of ",
		}
		for _, p := range leadingBiases {
			if strings.HasPrefix(lowerTrimmed, p) {
				trimmedPrompt = strings.TrimSpace(trimmedPrompt[len(p):])
				break
			}
		}
	}

	for _, s := range knownStyles {
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, style) {
			continue
		}
		// Remove conflicting style mentions globally, not only at the very beginning.
		quoted := regexp.QuoteMeta(strings.ToLower(s))
		patterns := []string{
			`(?i)\b(?:in\s+)?` + quoted + `\s*(?:-|\s)?\s*style\b[\s,:;.\-—]*`,
			`(?i)\bstyle\s*:\s*` + quoted + `\b[\s,:;.\-—]*`,
		}
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			trimmedPrompt = re.ReplaceAllString(trimmedPrompt, " ")
		}

		lowerCandidate := strings.ToLower(s)
		prefixes := []string{
			lowerCandidate + " style,",
			lowerCandidate + " style:",
			lowerCandidate + " style.",
			lowerCandidate + " style;",
			lowerCandidate + " style -",
			lowerCandidate + " style —",
			"in " + lowerCandidate + " style,",
			"in " + lowerCandidate + " style:",
			"in " + lowerCandidate + " style.",
			"in " + lowerCandidate + " style;",
			"style: " + lowerCandidate + ",",
			"style: " + lowerCandidate + ".",
			"style:" + lowerCandidate + ",",
			"style:" + lowerCandidate + ".",
		}
		for _, p := range prefixes {
			if strings.HasPrefix(strings.ToLower(trimmedPrompt), p) {
				trimmedPrompt = strings.TrimSpace(trimmedPrompt[len(p):])
				break
			}
		}
	}
	trimmedPrompt = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(trimmedPrompt, " "))

	if strings.Contains(strings.ToLower(trimmedPrompt), lowerStyle) {
		return trimmedPrompt
	}

	if strings.TrimSpace(trimmedPrompt) == "" {
		return fmt.Sprintf("%s style", style)
	}
	return fmt.Sprintf("%s style, %s", style, trimmedPrompt)
}

func applyStyleGuardToNegativePrompt(style string, negativePrompt string) string {
	style = strings.ToLower(strings.TrimSpace(style))
	negativePrompt = strings.TrimSpace(negativePrompt)
	if style == "" || style == "comic" {
		return negativePrompt
	}

	guardTokens := []string{
		"western comic",
		"comic book",
		"comic panel",
		"speech bubble",
		"word balloon",
		"halftone dots",
	}

	lower := strings.ToLower(negativePrompt)
	missing := make([]string, 0, len(guardTokens))
	for _, token := range guardTokens {
		if !strings.Contains(lower, token) {
			missing = append(missing, token)
		}
	}
	if len(missing) == 0 {
		return negativePrompt
	}

	if negativePrompt == "" {
		return strings.Join(missing, ", ")
	}
	return negativePrompt + ", " + strings.Join(missing, ", ")
}

func buildFrameImageSignature(fp *models.ComicFramePrompt, promptText string) string {
	if fp == nil {
		return ""
	}
	type signaturePayload struct {
		Prompt         string                 `json:"prompt"`
		NegativePrompt string                 `json:"negative_prompt,omitempty"`
		Style          string                 `json:"style,omitempty"`
		Model          string                 `json:"model,omitempty"`
		ModelParams    map[string]interface{} `json:"model_params,omitempty"`
	}
	payload := signaturePayload{
		Prompt:         strings.TrimSpace(promptText),
		NegativePrompt: strings.TrimSpace(fp.NegativePrompt),
		Style:          strings.TrimSpace(fp.Style),
		Model:          strings.TrimSpace(fp.Model),
		ModelParams:    fp.ModelParams,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *ComicService) shouldSkipExistingFrameOnResume(sceneID string, frameID string, fp *models.ComicFramePrompt, promptText string) bool {
	if s == nil || s.Repo == nil {
		return false
	}
	existing := s.bestEffortLoadGeneratedFrameImage(sceneID, frameID)
	if len(existing) == 0 {
		return false
	}
	currentSig := buildFrameImageSignature(fp, promptText)
	if currentSig == "" {
		return false
	}
	saved, err := s.Repo.LoadFrameImageSignature(sceneID, frameID)
	if err != nil || saved == nil {
		return false
	}
	return strings.TrimSpace(saved.Signature) == currentSig
}

func (s *ComicService) saveFrameImageSignature(sceneID string, frameID string, fp *models.ComicFramePrompt, promptText string) {
	if s == nil || s.Repo == nil || fp == nil {
		return
	}
	sig := buildFrameImageSignature(fp, promptText)
	if sig == "" {
		return
	}
	meta := &models.ComicFrameImageSignature{
		FrameID:      strings.TrimSpace(frameID),
		Signature:    sig,
		Prompt:       strings.TrimSpace(promptText),
		Style:        strings.TrimSpace(fp.Style),
		Model:        strings.TrimSpace(fp.Model),
		GeneratedAt:  time.Now(),
		TemplateHint: "resume_signature_v1",
	}
	if err := s.Repo.SaveFrameImageSignature(sceneID, frameID, meta); err != nil {
		utils.GetLogger().Warn("save frame image signature failed", map[string]interface{}{"scene_id": sceneID, "frame_id": frameID, "err": err})
	}
}

func buildNodeContentFromConversations(convs []models.Conversation, nodeIDs []string, maxRunes int) nodeContentBuildResult {
	if maxRunes <= 0 {
		maxRunes = 20000
	}
	seenNodes := make([]string, 0, len(nodeIDs))
	lookup := make(map[string]struct{}, len(nodeIDs))
	for _, id := range nodeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := lookup[id]; ok {
			continue
		}
		lookup[id] = struct{}{}
		seenNodes = append(seenNodes, id)
	}
	if len(lookup) == 0 {
		latest := resolveLatestNodeIDFromConversations(convs)
		if latest == "" {
			return nodeContentBuildResult{}
		}
		lookup[latest] = struct{}{}
		seenNodes = append(seenNodes, latest)
	}

	var b strings.Builder
	used := make(map[string]struct{}, len(seenNodes))
	conversationIDs := make([]string, 0, 16)
	contentHashes := make([]string, 0, 16)
	curRunes := 0
	truncated := false
	for _, conv := range convs {
		if !isComicStoryConversation(conv) {
			continue
		}
		nodeID := resolveConversationNodeIDForComic(conv)
		if nodeID == "" {
			continue
		}
		if _, ok := lookup[nodeID]; !ok {
			continue
		}
		text := strings.TrimSpace(conv.Content)
		if text == "" {
			continue
		}
		used[nodeID] = struct{}{}
		conversationIDs = append(conversationIDs, strings.TrimSpace(conv.ID))
		sum := sha256.Sum256([]byte(text))
		contentHashes = append(contentHashes, hex.EncodeToString(sum[:]))

		// Soft safeguard against absurdly large prompts.
		// We keep the limit high so typical nodes remain fully included.
		textRunes := len([]rune(text))
		if curRunes+textRunes > maxRunes {
			remaining := maxRunes - curRunes
			if remaining <= 0 {
				truncated = true
				break
			}
			r := []rune(text)
			text = strings.TrimSpace(string(r[:remaining]))
			if text == "" {
				truncated = true
				break
			}
			textRunes = len([]rune(text))
			truncated = true
		}

		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
		curRunes += textRunes
	}

	if b.Len() == 0 {
		return nodeContentBuildResult{UsedNodeIDs: seenNodes}
	}

	usedNodeIDs := make([]string, 0, len(seenNodes))
	for _, id := range seenNodes {
		if _, ok := used[id]; ok {
			usedNodeIDs = append(usedNodeIDs, id)
		}
	}
	if len(usedNodeIDs) == 0 {
		usedNodeIDs = seenNodes
	}

	return nodeContentBuildResult{
		Content:         b.String(),
		UsedNodeIDs:     usedNodeIDs,
		ConversationIDs: conversationIDs,
		ContentHashes:   contentHashes,
		Truncated:       truncated,
	}
}

func buildNodeContentFromRawText(text string, maxRunes int) nodeContentBuildResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return nodeContentBuildResult{}
	}
	if maxRunes <= 0 {
		maxRunes = 20000
	}
	runes := []rune(text)
	truncated := false
	if len(runes) > maxRunes {
		text = strings.TrimSpace(string(runes[:maxRunes]))
		truncated = true
	}
	if text == "" {
		return nodeContentBuildResult{}
	}
	sum := sha256.Sum256([]byte(text))
	return nodeContentBuildResult{
		Content:       text,
		ContentHashes: []string{hex.EncodeToString(sum[:])},
		Truncated:     truncated,
	}
}

func isLikelyStorageNotFound(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such file or directory") {
		return true
	}
	if strings.Contains(msg, "the system cannot find the path specified") {
		return true
	}
	if strings.Contains(msg, "the system cannot find the file specified") {
		return true
	}
	if strings.Contains(msg, "系统找不到指定的路径") {
		return true
	}
	if strings.Contains(msg, "系统找不到指定的文件") {
		return true
	}
	return false
}

func (s *ComicService) upsertMetrics(sceneID string, mutate func(m *models.ComicMetrics)) {
	if s.Repo == nil {
		return
	}

	m, err := s.Repo.LoadMetrics(sceneID)
	if err != nil {
		if !isLikelyStorageNotFound(err) {
			utils.GetLogger().Warn("load comic metrics failed", map[string]interface{}{"scene_id": sceneID, "err": err})
		}
		m = &models.ComicMetrics{SceneID: sceneID}
	}

	mutate(m)
	m.UpdatedAt = time.Now()
	if err := s.Repo.SaveMetrics(sceneID, m); err != nil {
		utils.GetLogger().Warn("save comic metrics failed", map[string]interface{}{"scene_id": sceneID, "err": err})
	}
}

func slugifyID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return ""
	}
	return out
}

func fillElementIDs(prefix string, elems []models.ComicKeyElement) []models.ComicKeyElement {
	used := map[string]int{}
	for i := range elems {
		id := strings.TrimSpace(elems[i].ID)
		if id == "" {
			id = slugifyID(elems[i].Name)
		}
		if id == "" {
			id = "item"
		}
		if !strings.HasPrefix(id, prefix) {
			id = prefix + id
		}
		base := id
		if n := used[base]; n > 0 {
			id = fmt.Sprintf("%s_%d", base, n+1)
		}
		used[base]++
		elems[i].ID = id
	}
	return elems
}

func parseIntFromAny(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		if i, err := strconv.Atoi(s); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int(f)
		}
		return 0
	default:
		return 0
	}
}

func parseFloatFromAny(v interface{}) float64 {
	switch x := v.(type) {
	case float32:
		return float64(x)
	case float64:
		return x
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

func parseInt64FromAny(v interface{}) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case float32:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

func parseStringFromAny(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return ""
	}
}

func parseBoolFromAny(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int32:
		return x != 0
	case int64:
		return x != 0
	case float32:
		return x != 0
	case float64:
		return x != 0
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		if s == "" {
			return false
		}
		if s == "1" || s == "true" || s == "yes" || s == "on" {
			return true
		}
		return false
	default:
		return false
	}
}

func parsePrevFrameReferenceOptions(params map[string]interface{}) (enabled bool, denoisingStrength float64) {
	if params == nil {
		return false, 0
	}
	if raw, ok := params["use_prev_frame_reference"]; ok {
		enabled = parseBoolFromAny(raw)
	}
	if raw, ok := params["prev_frame_denoising_strength"]; ok {
		denoisingStrength = parseFloatFromAny(raw)
	}
	return enabled, denoisingStrength
}

func (s *ComicService) bestEffortLoadGeneratedFrameImage(sceneID string, frameID string) []byte {
	if s == nil || s.Repo == nil {
		return nil
	}
	frameID = strings.TrimSpace(frameID)
	if frameID == "" {
		return nil
	}
	b, err := s.Repo.LoadFrameImage(sceneID, frameID)
	if err != nil {
		if isLikelyStorageNotFound(err) {
			return nil
		}
		utils.GetLogger().Warn("load previous frame image failed", map[string]interface{}{"scene_id": sceneID, "frame_id": frameID, "err": err})
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	return b
}

func (s *ComicService) findPreviousFrameID(sceneID string, frameID string) string {
	if s == nil || s.Repo == nil {
		return ""
	}
	frameID = strings.TrimSpace(frameID)
	if frameID == "" {
		return ""
	}
	analysis, err := s.Repo.LoadAnalysis(sceneID)
	if err != nil || analysis == nil || len(analysis.Frames) == 0 {
		return ""
	}
	normalizeBreakdown(analysis, sceneID, sanitizeTargetFrames(analysis.TargetFrames))
	for i := range analysis.Frames {
		if strings.TrimSpace(analysis.Frames[i].ID) != frameID {
			continue
		}
		if i <= 0 {
			return ""
		}
		return strings.TrimSpace(analysis.Frames[i-1].ID)
	}
	return ""
}

func buildContinuityAnchorFromKeyElements(keyElements *models.ComicKeyElements) string {
	if keyElements == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if len(keyElements.Characters) > 0 {
		names := make([]string, 0, 3)
		for _, c := range keyElements.Characters {
			name := strings.TrimSpace(c.Name)
			if name == "" {
				continue
			}
			names = append(names, name)
			if len(names) >= 3 {
				break
			}
		}
		if len(names) > 0 {
			parts = append(parts, "characters="+strings.Join(names, ", "))
		}
	}
	if len(keyElements.StyleTags) > 0 {
		tags := make([]string, 0, 4)
		for _, t := range keyElements.StyleTags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			tags = append(tags, t)
			if len(tags) >= 4 {
				break
			}
		}
		if len(tags) > 0 {
			parts = append(parts, "style_tags="+strings.Join(tags, ", "))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

func applyModelParams(opts *VisionGenerateOptions, params map[string]interface{}) {
	if opts == nil {
		return
	}
	// Always reset, so values never leak across frames.
	opts.Steps = 0
	opts.CFGScale = 0
	opts.Sampler = ""
	opts.Seed = 0
	opts.ClipSkip = 0
	opts.Eta = 0
	opts.Tiling = false
	opts.Stylize = 0
	opts.Chaos = 0
	opts.PromptExtend = false
	opts.DenoisingStrength = 0

	baselineNegative := "blurry, watermark, text, logo"
	currentNegative := strings.TrimSpace(opts.NegativePrompt)
	if currentNegative == "" {
		opts.NegativePrompt = baselineNegative
	} else {
		lowerCurrent := strings.ToLower(currentNegative)
		lowerBaseline := strings.ToLower(baselineNegative)
		if !strings.Contains(lowerCurrent, lowerBaseline) {
			opts.NegativePrompt = baselineNegative + ", " + currentNegative
		} else {
			opts.NegativePrompt = currentNegative
		}
	}

	if params == nil {
		return
	}
	if v, ok := params["steps"]; ok {
		opts.Steps = parseIntFromAny(v)
	}
	if v, ok := params["cfg_scale"]; ok {
		opts.CFGScale = parseFloatFromAny(v)
	}
	if v, ok := params["sampler"]; ok {
		opts.Sampler = parseStringFromAny(v)
	}
	if v, ok := params["seed"]; ok {
		opts.Seed = parseInt64FromAny(v)
	}
	if v, ok := params["clip_skip"]; ok {
		opts.ClipSkip = parseIntFromAny(v)
	}
	if v, ok := params["eta"]; ok {
		opts.Eta = parseFloatFromAny(v)
	}
	if v, ok := params["tiling"]; ok {
		opts.Tiling = parseBoolFromAny(v)
	}
	if v, ok := params["denoising_strength"]; ok {
		opts.DenoisingStrength = parseFloatFromAny(v)
	}
	if v, ok := params["stylize"]; ok {
		opts.Stylize = parseIntFromAny(v)
	}
	if v, ok := params["chaos"]; ok {
		opts.Chaos = parseIntFromAny(v)
	}
	if v, ok := params["prompt_extend"]; ok {
		opts.PromptExtend = parseBoolFromAny(v)
	}
}

func compactErrorMessageForProgress(err error) string {
	if err == nil {
		return "unknown error"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "unknown error"
	}
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > 280 {
		msg = msg[:280] + "..."
	}
	return msg
}

func (s *ComicService) resolveVisionProviderForModel(model string) string {
	if s == nil || s.Vision == nil {
		return ""
	}
	resolvedModel := strings.TrimSpace(model)
	if resolvedModel == "" {
		resolvedModel = strings.TrimSpace(s.Vision.DefaultModel)
	}
	provider := strings.TrimSpace(s.Vision.providerForModel(resolvedModel))
	if provider == "" {
		provider = strings.TrimSpace(s.Vision.DefaultProvider)
	}
	return provider
}

func (s *ComicService) buildFrameVisionFailureMessage(frameID string, opts VisionGenerateOptions, err error) string {
	frameID = strings.TrimSpace(frameID)
	provider := s.resolveVisionProviderForModel(opts.Model)
	model := strings.TrimSpace(opts.Model)
	if model == "" && s != nil && s.Vision != nil {
		model = strings.TrimSpace(s.Vision.DefaultModel)
	}

	parts := make([]string, 0, 4)
	if frameID != "" {
		parts = append(parts, "frame="+frameID)
	}
	if provider != "" {
		parts = append(parts, "provider="+provider)
	}
	if model != "" {
		parts = append(parts, "model="+model)
	}
	prefix := strings.Join(parts, ", ")
	if prefix == "" {
		return "生成图片失败: " + compactErrorMessageForProgress(err)
	}
	return fmt.Sprintf("生成图片失败（%s）: %s", prefix, compactErrorMessageForProgress(err))
}

func (s *ComicService) bestEffortLoadFirstReferenceImage(sceneID string) []byte {
	if s == nil || s.Repo == nil {
		return nil
	}
	idx, err := s.Repo.LoadReferenceIndex(sceneID)
	if err != nil {
		if isLikelyStorageNotFound(err) {
			return nil
		}
		utils.GetLogger().Warn("load reference index failed", map[string]interface{}{"scene_id": sceneID, "err": err})
		return nil
	}
	if idx == nil || len(idx.References) == 0 {
		return nil
	}

	keys := make([]string, 0, len(idx.References))
	for k := range idx.References {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	meta := idx.References[keys[0]]
	fileName := strings.TrimSpace(meta.FileName)
	if fileName == "" {
		return nil
	}

	b, err := s.Repo.LoadReference(sceneID, fileName)
	if err != nil {
		if isLikelyStorageNotFound(err) {
			return nil
		}
		utils.GetLogger().Warn("load reference image failed", map[string]interface{}{"scene_id": sceneID, "file_name": fileName, "err": err})
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	return b
}

func (s *ComicService) selectReferenceImageForFrame(sceneID string, frame *models.ComicFramePlan, promptText string, keyElements *models.ComicKeyElements, refIndex *models.ComicReferenceIndex) []byte {
	if s == nil || s.Repo == nil {
		return nil
	}
	if refIndex == nil {
		idx, err := s.Repo.LoadReferenceIndex(sceneID)
		if err != nil {
			if isLikelyStorageNotFound(err) {
				return nil
			}
			utils.GetLogger().Warn("load reference index failed", map[string]interface{}{"scene_id": sceneID, "err": err})
			return nil
		}
		refIndex = idx
	}
	if refIndex == nil || len(refIndex.References) == 0 {
		return nil
	}

	searchText := strings.TrimSpace(promptText)
	if frame != nil && strings.TrimSpace(frame.Description) != "" {
		searchText = strings.TrimSpace(searchText + "\n" + frame.Description)
	}
	searchText = strings.ToLower(searchText)

	matched := make([]string, 0, 4)
	seen := make(map[string]struct{})
	addMatch := func(elem models.ComicKeyElement) {
		id := strings.TrimSpace(elem.ID)
		if id == "" {
			return
		}
		if _, ok := refIndex.References[id]; !ok {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		name := strings.ToLower(strings.TrimSpace(elem.Name))
		idLower := strings.ToLower(id)
		if (name != "" && strings.Contains(searchText, name)) || (idLower != "" && strings.Contains(searchText, idLower)) {
			matched = append(matched, id)
			seen[id] = struct{}{}
		}
	}

	if keyElements != nil {
		for _, c := range keyElements.Characters {
			addMatch(c)
		}
		for _, o := range keyElements.Objects {
			addMatch(o)
		}
		for _, l := range keyElements.Locations {
			addMatch(l)
		}
	}

	if len(matched) == 0 {
		keys := make([]string, 0, len(refIndex.References))
		for k := range refIndex.References {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			return nil
		}
		matched = append(matched, keys[0])
	}

	meta := refIndex.References[matched[0]]
	fileName := strings.TrimSpace(meta.FileName)
	if fileName == "" {
		return nil
	}

	b, err := s.Repo.LoadReference(sceneID, fileName)
	if err != nil {
		if isLikelyStorageNotFound(err) {
			return nil
		}
		utils.GetLogger().Warn("load reference image failed", map[string]interface{}{"scene_id": sceneID, "file_name": fileName, "err": err})
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	return b
}

// AnalyzeStoryAsync：从 Story/Scene 生成分镜（Phase2 最小可用版）。
func (s *ComicService) AnalyzeStoryAsync(ctx context.Context, sceneID string) (taskID string, err error) {
	return s.AnalyzeStoryAsyncWithConfigAndNode(ctx, sceneID, 4, "")
}

// AnalyzeStoryAsyncWithConfig：启动分镜分析任务，并允许覆盖 targetFrames（会做最小 clamp）。
func (s *ComicService) AnalyzeStoryAsyncWithConfig(ctx context.Context, sceneID string, targetFrames int) (taskID string, err error) {
	return s.AnalyzeStoryAsyncWithConfigAndNode(ctx, sceneID, targetFrames, "")
}

func (s *ComicService) AnalyzeStoryAsyncWithConfigAndSourceText(ctx context.Context, sceneID string, targetFrames int, sourceText string) (taskID string, err error) {
	if err := s.ensureLLMReady(); err != nil {
		return "", err
	}
	if sceneID == "" {
		return "", errors.New("sceneID required")
	}

	sourceText = strings.TrimSpace(sourceText)
	if sourceText == "" {
		return "", errors.New("source_text required")
	}

	targetFrames = sanitizeTargetFrames(targetFrames)

	taskID = fmt.Sprintf("comic_analyze_%s_%d", sceneID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	tracker.UpdateProgress(1, "准备分析故事...")

	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		if ctx != nil {
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		defer func() {
			if r := recover(); r != nil {
				utils.GetLogger().Error("comic analyze panic", map[string]interface{}{"scene_id": sceneID, "task_id": taskID, "panic": fmt.Sprintf("%v", r)})
				tracker.Fail("任务异常崩溃")
			}
		}()

		tracker.UpdateProgress(5, "初始化目录...")
		if _, err := s.EnsureSceneLayout(sceneID); err != nil {
			tracker.Fail("初始化目录失败")
			return err
		}

		tracker.UpdateProgress(15, "加载场景数据...")
		sceneData, err := s.Scene.LoadScene(sceneID)
		if err != nil {
			tracker.Fail("加载场景失败")
			return err
		}
		if err := s.Scene.SaveOriginalTextForScene(sceneID, sourceText); err != nil {
			tracker.Fail("保存原文失败")
			return err
		}
		sceneData.OriginalText = sourceText

		tracker.UpdateProgress(35, "调用 LLM 生成分镜...")
		cfg := prompts.ComicPromptConfig{Language: "auto", TargetFrames: targetFrames}
		sys := prompts.BuildStoryAnalysisSystemPrompt(cfg)
		user := prompts.BuildStoryAnalysisPromptFromNodeContent(sceneData.Scene, "source_text", sourceText, cfg)

		var breakdown models.ComicBreakdown
		resp, dur, cached, err := s.LLM.CreateStructuredCompletionWithMetrics(jobCtx, user, sys, &breakdown)
		if err != nil {
			tracker.Fail("LLM 分镜生成失败")
			return err
		}
		if resp != nil {
			call := &models.ComicLLMCallMetrics{
				Provider:     resp.ProviderName,
				Model:        resp.ModelName,
				TokensUsed:   resp.TokensUsed,
				PromptTokens: resp.PromptTokens,
				OutputTokens: resp.OutputTokens,
				DurationMs:   dur.Milliseconds(),
				Cached:       cached,
				GeneratedAt:  time.Now(),
			}
			s.upsertMetrics(sceneID, func(m *models.ComicMetrics) {
				m.Analysis = call
			})
		}
		breakdown.GeneratedAt = time.Now()
		normalizeBreakdown(&breakdown, sceneID, cfg.TargetFrames)

		tracker.UpdateProgress(80, "写入分析结果...")
		if err := s.Repo.SaveAnalysis(sceneID, &breakdown); err != nil {
			tracker.Fail("写入分析结果失败")
			return err
		}

		tracker.Complete("分镜分析完成")
		return nil
	}); err != nil {
		tracker.Fail("任务提交失败")
		return "", err
	}

	return taskID, nil
}

// AnalyzeStoryAsyncWithConfigAndNode：启动分镜分析任务，并可指定 nodeID 作为输入来源。
// 当 nodeID 非空时，分镜分析会基于 scene/context.json 中该 node 的全部剧情内容（story_* conversations）。
func (s *ComicService) AnalyzeStoryAsyncWithConfigAndNode(ctx context.Context, sceneID string, targetFrames int, nodeID string) (taskID string, err error) {
	if err := s.ensureLLMReady(); err != nil {
		return "", err
	}
	if sceneID == "" {
		return "", errors.New("sceneID required")
	}

	nodeID = strings.TrimSpace(nodeID)

	targetFrames = sanitizeTargetFrames(targetFrames)

	taskID = fmt.Sprintf("comic_analyze_%s_%d", sceneID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	tracker.UpdateProgress(1, "准备分析故事...")

	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		// prefer job ctx; fallback to caller ctx if needed
		if ctx != nil {
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		defer func() {
			if r := recover(); r != nil {
				utils.GetLogger().Error("comic analyze panic", map[string]interface{}{"scene_id": sceneID, "task_id": taskID, "panic": fmt.Sprintf("%v", r)})
				tracker.Fail("任务异常崩溃")
			}
		}()

		tracker.UpdateProgress(5, "初始化目录...")
		if _, err := s.EnsureSceneLayout(sceneID); err != nil {
			tracker.Fail("初始化目录失败")
			return err
		}

		tracker.UpdateProgress(15, "加载场景/故事数据...")
		sceneData, err := s.Scene.LoadScene(sceneID)
		if err != nil {
			tracker.Fail("加载场景失败")
			return err
		}
		storyData, err := s.Story.GetStoryForScene(sceneID)
		if err != nil {
			tracker.Fail("加载故事失败")
			return err
		}

		tracker.UpdateProgress(35, "调用 LLM 生成分镜...")
		cfg := prompts.ComicPromptConfig{Language: "auto", TargetFrames: targetFrames}
		sys := prompts.BuildStoryAnalysisSystemPrompt(cfg)
		user := ""
		if nodeID != "" {
			build := buildNodeContentFromConversations(sceneData.Context.Conversations, []string{nodeID}, 20000)
			if strings.TrimSpace(build.Content) == "" {
				build = buildNodeContentFromRawText(sceneData.OriginalText, 20000)
			}
			if strings.TrimSpace(build.Content) == "" {
				tracker.Fail("指定 node_id 没有可用内容")
				return fmt.Errorf("no usable content for node_id=%s", nodeID)
			}
			user = prompts.BuildStoryAnalysisPromptFromNodeContent(sceneData.Scene, nodeID, build.Content, cfg)
		} else {
			user = prompts.BuildStoryAnalysisPrompt(sceneData.Scene, *storyData, cfg)
		}

		var breakdown models.ComicBreakdown
		resp, dur, cached, err := s.LLM.CreateStructuredCompletionWithMetrics(jobCtx, user, sys, &breakdown)
		if err != nil {
			tracker.Fail("LLM 分镜生成失败")
			return err
		}
		if resp != nil {
			call := &models.ComicLLMCallMetrics{
				Provider:     resp.ProviderName,
				Model:        resp.ModelName,
				TokensUsed:   resp.TokensUsed,
				PromptTokens: resp.PromptTokens,
				OutputTokens: resp.OutputTokens,
				DurationMs:   dur.Milliseconds(),
				Cached:       cached,
				GeneratedAt:  time.Now(),
			}
			s.upsertMetrics(sceneID, func(m *models.ComicMetrics) {
				m.Analysis = call
			})
		}
		breakdown.GeneratedAt = time.Now()
		normalizeBreakdown(&breakdown, sceneID, cfg.TargetFrames)
		if nodeID != "" {
			for i := range breakdown.Frames {
				breakdown.Frames[i].StoryNodeIDs = []string{nodeID}
			}
		}

		tracker.UpdateProgress(80, "写入分析结果...")
		if err := s.Repo.SaveAnalysis(sceneID, &breakdown); err != nil {
			tracker.Fail("写入分析结果失败")
			return err
		}

		tracker.Complete("分镜分析完成")
		return nil
	}); err != nil {
		tracker.Fail("任务提交失败")
		return "", err
	}

	return taskID, nil
}

// BuildPromptsAsync：为每帧生成 prompts（Phase2 最小可用版）。
// nodeID（可选）用于指定 prompts 的内容来源节点；为空时回退到 analysis 的 story_node_ids 或上下文最新 node。
func (s *ComicService) BuildPromptsAsync(ctx context.Context, sceneID string, nodeID string) (taskID string, err error) {
	return s.BuildPromptsAsyncWithOptions(ctx, sceneID, ComicPromptsBuildOptions{NodeID: nodeID})
}

func (s *ComicService) BuildPromptsAsyncWithOptions(ctx context.Context, sceneID string, options ComicPromptsBuildOptions) (taskID string, err error) {
	if err := s.ensureLLMReady(); err != nil {
		return "", err
	}
	if sceneID == "" {
		return "", errors.New("sceneID required")
	}

	taskID = fmt.Sprintf("comic_prompts_%s_%d", sceneID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	tracker.UpdateProgress(1, "准备生成提示词...")

	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		if ctx != nil {
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		defer func() {
			if r := recover(); r != nil {
				utils.GetLogger().Error("comic prompts panic", map[string]interface{}{"scene_id": sceneID, "task_id": taskID, "panic": fmt.Sprintf("%v", r)})
				tracker.Fail("任务异常崩溃")
			}
		}()

		tracker.UpdateProgress(5, "加载分析结果...")
		breakdown, err := s.Repo.LoadAnalysis(sceneID)
		if err != nil {
			tracker.Fail("加载分析结果失败")
			return err
		}
		if breakdown == nil || len(breakdown.Frames) == 0 {
			tracker.Fail("分析结果为空")
			return errors.New("analysis is empty")
		}

		normalizeBreakdown(breakdown, sceneID, sanitizeTargetFrames(breakdown.TargetFrames))

		tracker.UpdateProgress(15, "加载场景数据...")
		sceneData, err := s.Scene.LoadScene(sceneID)
		if err != nil {
			tracker.Fail("加载场景失败")
			return err
		}

		var keyElements *models.ComicKeyElements
		if s.Repo != nil {
			ke, err := s.Repo.LoadKeyElements(sceneID)
			if err != nil && !isLikelyStorageNotFound(err) {
				utils.GetLogger().Warn("load key elements failed", map[string]interface{}{"scene_id": sceneID, "err": err})
			} else {
				keyElements = ke
			}
		}

		cfg := prompts.ComicPromptConfig{Language: "auto", TargetFrames: breakdown.TargetFrames}
		cfg.Style = strings.TrimSpace(options.Style)
		cfg.ContinuityMode = strings.TrimSpace(options.ContinuityMode)
		if cfg.ContinuityMode == "" {
			cfg.ContinuityMode = "strict"
		}
		if s.Vision != nil {
			cfg.Model = strings.TrimSpace(s.Vision.DefaultModel)
		}
		cfg.FrameAnchor = buildContinuityAnchorFromKeyElements(keyElements)
		if cfg.FrameAnchor == "" && len(breakdown.Frames) > 0 {
			firstDesc := strings.TrimSpace(breakdown.Frames[0].Description)
			if firstDesc != "" {
				cfg.FrameAnchor = "opening_frame=" + firstDesc
			}
		}
		sys := prompts.BuildFramePromptSystemPrompt(cfg, keyElements)
		selectedNodeID := strings.TrimSpace(options.NodeID)
		prevFramePrompt := ""

		for i, frame := range breakdown.Frames {
			if err := jobCtx.Err(); err != nil {
				tracker.Fail("任务已取消")
				return err
			}

			progress := 20 + int(float64(i)/float64(len(breakdown.Frames))*70.0)
			tracker.UpdateProgress(progress, fmt.Sprintf("生成提示词：%s", frame.ID))

			nodeIDs := frame.StoryNodeIDs
			if selectedNodeID != "" {
				nodeIDs = []string{selectedNodeID}
			}
			build := buildNodeContentFromConversations(sceneData.Context.Conversations, nodeIDs, 20000)
			if strings.TrimSpace(build.Content) == "" {
				build = buildNodeContentFromRawText(sceneData.OriginalText, 20000)
			}
			user := prompts.BuildFramePrompt(sceneData.Scene, frame, cfg, build.UsedNodeIDs, build.Content, prevFramePrompt)
			var out models.ComicFramePrompt
			resp, dur, cached, err := s.LLM.CreateStructuredCompletionWithMetrics(jobCtx, user, sys, &out)
			if err != nil {
				tracker.Fail("LLM 提示词生成失败")
				return err
			}
			if resp != nil {
				call := models.ComicLLMCallMetrics{
					Provider:     resp.ProviderName,
					Model:        resp.ModelName,
					TokensUsed:   resp.TokensUsed,
					PromptTokens: resp.PromptTokens,
					OutputTokens: resp.OutputTokens,
					DurationMs:   dur.Milliseconds(),
					Cached:       cached,
					GeneratedAt:  time.Now(),
				}
				s.upsertMetrics(sceneID, func(m *models.ComicMetrics) {
					m.Prompts = append(m.Prompts, call)
				})
			}
			if out.FrameID == "" {
				out.FrameID = frame.ID
			}
			if strings.TrimSpace(cfg.Style) != "" {
				out.Style = cfg.Style
				out.Prompt = applyStyleToPrompt(out.Prompt, cfg.Style)
			} else if strings.TrimSpace(out.Style) != "" {
				out.Prompt = applyStyleToPrompt(out.Prompt, out.Style)
			}
			if strings.TrimSpace(out.Model) == "" && strings.TrimSpace(cfg.Model) != "" {
				out.Model = cfg.Model
			}
			out.PromptSources = &models.ComicPromptSources{
				SceneID:         sceneID,
				NodeIDs:         build.UsedNodeIDs,
				ConversationIDs: build.ConversationIDs,
				ContentHashes:   build.ContentHashes,
				Truncated:       build.Truncated,
				ContinuityMode:  cfg.ContinuityMode,
				FrameAnchor:     cfg.FrameAnchor,
				TemplateVersion: prompts.ComicFramePromptTemplateVersion,
				GeneratedAt:     time.Now(),
			}
			prevFramePrompt = strings.TrimSpace(out.Prompt)

			if err := s.Repo.SavePrompt(sceneID, frame.ID, &out); err != nil {
				tracker.Fail("写入提示词失败")
				return err
			}
		}

		tracker.Complete("提示词生成完成")
		return nil
	}); err != nil {
		tracker.Fail("任务提交失败")
		return "", err
	}

	return taskID, nil
}

// ExtractKeyElementsAsync：从 analysis + prompts 提取关键元素并落盘（Phase2 最小可用版）。
func (s *ComicService) ExtractKeyElementsAsync(ctx context.Context, sceneID string) (taskID string, err error) {
	if err := s.ensureLLMReady(); err != nil {
		return "", err
	}
	if sceneID == "" {
		return "", errors.New("sceneID required")
	}

	taskID = fmt.Sprintf("comic_elements_%s_%d", sceneID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	tracker.UpdateProgress(1, "准备提取关键元素...")

	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		if ctx != nil {
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		defer func() {
			if r := recover(); r != nil {
				utils.GetLogger().Error("comic key elements panic", map[string]interface{}{"scene_id": sceneID, "task_id": taskID, "panic": fmt.Sprintf("%v", r)})
				tracker.Fail("任务异常崩溃")
			}
		}()

		tracker.UpdateProgress(5, "加载分析与提示词...")
		breakdown, err := s.Repo.LoadAnalysis(sceneID)
		if err != nil {
			tracker.Fail("加载分析结果失败")
			return err
		}
		if breakdown == nil || len(breakdown.Frames) == 0 {
			tracker.Fail("分析结果为空")
			return errors.New("analysis is empty")
		}

		framePrompts := make([]models.ComicFramePrompt, 0, len(breakdown.Frames))
		for _, f := range breakdown.Frames {
			p, err := s.Repo.LoadPrompt(sceneID, f.ID)
			if err != nil {
				tracker.Fail("加载提示词失败")
				return err
			}
			if p != nil {
				if p.FrameID == "" {
					p.FrameID = f.ID
				}
				framePrompts = append(framePrompts, *p)
			}
		}

		sceneData, err := s.Scene.LoadScene(sceneID)
		if err != nil {
			tracker.Fail("加载场景失败")
			return err
		}

		tracker.UpdateProgress(45, "调用 LLM 提取关键元素...")
		cfg := prompts.ComicPromptConfig{Language: "auto", TargetFrames: breakdown.TargetFrames}
		sys := prompts.BuildKeyElementsSystemPrompt(cfg)
		user := prompts.BuildKeyElementsPrompt(sceneData.Scene, *breakdown, framePrompts, cfg)

		var out models.ComicKeyElements
		resp, dur, cached, err := s.LLM.CreateStructuredCompletionWithMetrics(jobCtx, user, sys, &out)
		if err != nil {
			tracker.Fail("LLM 关键元素提取失败")
			return err
		}
		if resp != nil {
			call := &models.ComicLLMCallMetrics{
				Provider:     resp.ProviderName,
				Model:        resp.ModelName,
				TokensUsed:   resp.TokensUsed,
				PromptTokens: resp.PromptTokens,
				OutputTokens: resp.OutputTokens,
				DurationMs:   dur.Milliseconds(),
				Cached:       cached,
				GeneratedAt:  time.Now(),
			}
			s.upsertMetrics(sceneID, func(m *models.ComicMetrics) {
				m.KeyElements = call
			})
		}
		if out.SceneID == "" {
			out.SceneID = sceneID
		}
		out.GeneratedAt = time.Now()
		out.Characters = fillElementIDs("char_", out.Characters)
		out.Objects = fillElementIDs("obj_", out.Objects)
		out.Locations = fillElementIDs("loc_", out.Locations)

		tracker.UpdateProgress(85, "写入关键元素...")
		if err := s.Repo.SaveKeyElements(sceneID, &out); err != nil {
			tracker.Fail("写入关键元素失败")
			return err
		}

		tracker.Complete("关键元素提取完成")
		return nil
	}); err != nil {
		tracker.Fail("任务提交失败")
		return "", err
	}

	return taskID, nil
}

// GenerateComicAsync：调用 Vision provider 生成图片（占位）。
func (s *ComicService) GenerateComicAsync(ctx context.Context, sceneID string) (taskID string, err error) {
	return s.GenerateComicAsyncWithOptions(ctx, sceneID, ComicGenerateOptions{})
}

func (s *ComicService) GenerateComicAsyncWithOptions(ctx context.Context, sceneID string, options ComicGenerateOptions) (taskID string, err error) {
	if err := s.ensureVisionReady(); err != nil {
		return "", err
	}
	if strings.TrimSpace(sceneID) == "" {
		return "", errors.New("sceneID required")
	}

	taskID = fmt.Sprintf("comic_generate_%s_%d", sceneID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	tracker.UpdateProgress(1, "准备生成图片...")

	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		if ctx != nil {
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		defer func() {
			if r := recover(); r != nil {
				utils.GetLogger().Error("comic generate panic", map[string]interface{}{"scene_id": sceneID, "task_id": taskID, "panic": fmt.Sprintf("%v", r)})
				tracker.Fail("任务异常崩溃")
			}
		}()

		tracker.UpdateProgress(5, "初始化目录...")
		if _, err := s.EnsureSceneLayout(sceneID); err != nil {
			tracker.Fail("初始化目录失败")
			return err
		}

		tracker.UpdateProgress(10, "加载分镜与提示词...")
		breakdown, err := s.Repo.LoadAnalysis(sceneID)
		if err != nil {
			tracker.Fail("加载分镜失败")
			return err
		}
		if breakdown == nil || len(breakdown.Frames) == 0 {
			tracker.Fail("分镜为空")
			return errors.New("analysis is empty")
		}

		normalizeBreakdown(breakdown, sceneID, sanitizeTargetFrames(breakdown.TargetFrames))

		var refIndex *models.ComicReferenceIndex
		if s.Repo != nil {
			idx, err := s.Repo.LoadReferenceIndex(sceneID)
			if err != nil && !isLikelyStorageNotFound(err) {
				utils.GetLogger().Warn("load reference index failed", map[string]interface{}{"scene_id": sceneID, "err": err})
			} else {
				refIndex = idx
			}
		}

		var keyElements *models.ComicKeyElements
		if s.Repo != nil {
			ke, err := s.Repo.LoadKeyElements(sceneID)
			if err != nil && !isLikelyStorageNotFound(err) {
				utils.GetLogger().Warn("load key elements failed", map[string]interface{}{"scene_id": sceneID, "err": err})
			} else {
				keyElements = ke
			}
		}

		baseSeed := time.Now().UnixNano() % 10000
		if baseSeed == 0 {
			baseSeed = 1
		}

		for i, frame := range breakdown.Frames {
			if err := jobCtx.Err(); err != nil {
				tracker.Fail("任务已取消")
				return err
			}

			progress := 15 + int(float64(i)/float64(len(breakdown.Frames))*80.0)
			tracker.UpdateProgress(progress, fmt.Sprintf("生成图片：%s", frame.ID))

			fp, err := s.Repo.LoadPrompt(sceneID, frame.ID)
			if err != nil {
				tracker.Fail("加载提示词失败")
				return err
			}
			if fp == nil || strings.TrimSpace(fp.Prompt) == "" {
				tracker.Fail("提示词为空")
				return errors.New("prompt is empty")
			}
			promptText := applyStyleToPrompt(fp.Prompt, fp.Style)
			if options.Resume && s.shouldSkipExistingFrameOnResume(sceneID, frame.ID, fp, promptText) {
				tracker.UpdateProgress(progress, fmt.Sprintf("跳过已存在且未变更图片：%s", frame.ID))
				continue
			}

			frameOpts := VisionGenerateOptions{Width: 512, Height: 512, Model: fp.Model, NegativePrompt: fp.NegativePrompt}
			applyModelParams(&frameOpts, fp.ModelParams)
			frameOpts.NegativePrompt = applyStyleGuardToNegativePrompt(fp.Style, frameOpts.NegativePrompt)
			if frameOpts.Seed == 0 {
				frameOpts.Seed = baseSeed + int64(i*100)
			}
			if promptText != strings.TrimSpace(fp.Prompt) {
				fp.Prompt = promptText
				if err := s.Repo.SavePrompt(sceneID, frame.ID, fp); err != nil {
					utils.GetLogger().Warn("save normalized prompt failed", map[string]interface{}{"scene_id": sceneID, "frame_id": frame.ID, "err": err})
				}
			}
			usePrevRef, prevRefDenoising := parsePrevFrameReferenceOptions(fp.ModelParams)
			if usePrevRef && i > 0 {
				prevFrameID := strings.TrimSpace(breakdown.Frames[i-1].ID)
				prevFrameImage := s.bestEffortLoadGeneratedFrameImage(sceneID, prevFrameID)
				if len(prevFrameImage) > 0 {
					frameOpts.ReferenceImage = prevFrameImage
					if frameOpts.DenoisingStrength <= 0 {
						if prevRefDenoising > 0 {
							frameOpts.DenoisingStrength = prevRefDenoising
						} else {
							frameOpts.DenoisingStrength = 0.35
						}
					}
				}
			}
			if frameOpts.DenoisingStrength > 0 && len(frameOpts.ReferenceImage) == 0 {
				refImage := s.selectReferenceImageForFrame(sceneID, &frame, promptText, keyElements, refIndex)
				if len(refImage) > 0 {
					frameOpts.ReferenceImage = refImage
				}
			}
			if _, _, err := s.Vision.GenerateAndSaveFrame(jobCtx, sceneID, frame.ID, promptText, frameOpts); err != nil {
				failMsg := s.buildFrameVisionFailureMessage(frame.ID, frameOpts, err)
				tracker.EmitProgressEvent(progress, failMsg, "frame_failed", frame.ID)
				tracker.Fail(failMsg)
				return err
			}
			s.saveFrameImageSignature(sceneID, frame.ID, fp, promptText)
			tracker.EmitProgressEvent(progress, fmt.Sprintf("图片已写入：%s", frame.ID), "image_written", frame.ID)
		}

		tracker.Complete("图片生成完成")
		return nil
	}); err != nil {
		tracker.Fail("任务提交失败")
		return "", err
	}

	return taskID, nil
}

// RegenerateFrameAsync regenerates a single frame image based on its saved prompt.
func (s *ComicService) RegenerateFrameAsync(ctx context.Context, sceneID string, frameID string) (taskID string, err error) {
	if err := s.ensureVisionReady(); err != nil {
		return "", err
	}
	if strings.TrimSpace(sceneID) == "" {
		return "", errors.New("sceneID required")
	}
	if strings.TrimSpace(frameID) == "" {
		return "", errors.New("frameID required")
	}

	taskID = fmt.Sprintf("comic_regen_%s_%s_%d", sceneID, frameID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	tracker.UpdateProgress(1, "准备重绘图片...")

	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		if ctx != nil {
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		defer func() {
			if r := recover(); r != nil {
				utils.GetLogger().Error("comic regenerate panic", map[string]interface{}{"scene_id": sceneID, "frame_id": frameID, "task_id": taskID, "panic": fmt.Sprintf("%v", r)})
				tracker.Fail("任务异常崩溃")
			}
		}()

		tracker.UpdateProgress(10, "加载提示词...")
		fp, err := s.Repo.LoadPrompt(sceneID, frameID)
		if err != nil {
			tracker.Fail("加载提示词失败")
			return err
		}
		if fp == nil || strings.TrimSpace(fp.Prompt) == "" {
			tracker.Fail("提示词为空")
			return errors.New("prompt is empty")
		}

		tracker.UpdateProgress(50, "生成图片...")
		opts := VisionGenerateOptions{Width: 512, Height: 512, Model: fp.Model, NegativePrompt: fp.NegativePrompt}
		applyModelParams(&opts, fp.ModelParams)
		opts.NegativePrompt = applyStyleGuardToNegativePrompt(fp.Style, opts.NegativePrompt)
		promptText := applyStyleToPrompt(fp.Prompt, fp.Style)
		if promptText != strings.TrimSpace(fp.Prompt) {
			fp.Prompt = promptText
			if err := s.Repo.SavePrompt(sceneID, frameID, fp); err != nil {
				utils.GetLogger().Warn("save normalized prompt failed", map[string]interface{}{"scene_id": sceneID, "frame_id": frameID, "err": err})
			}
		}
		usePrevRef, prevRefDenoising := parsePrevFrameReferenceOptions(fp.ModelParams)
		if usePrevRef {
			prevFrameID := s.findPreviousFrameID(sceneID, frameID)
			prevFrameImage := s.bestEffortLoadGeneratedFrameImage(sceneID, prevFrameID)
			if len(prevFrameImage) > 0 {
				opts.ReferenceImage = prevFrameImage
				if opts.DenoisingStrength <= 0 {
					if prevRefDenoising > 0 {
						opts.DenoisingStrength = prevRefDenoising
					} else {
						opts.DenoisingStrength = 0.35
					}
				}
			}
		}
		if opts.DenoisingStrength > 0 && len(opts.ReferenceImage) == 0 {
			var refIndex *models.ComicReferenceIndex
			if s.Repo != nil {
				idx, err := s.Repo.LoadReferenceIndex(sceneID)
				if err != nil && !isLikelyStorageNotFound(err) {
					utils.GetLogger().Warn("load reference index failed", map[string]interface{}{"scene_id": sceneID, "err": err})
				} else {
					refIndex = idx
				}
			}
			var keyElements *models.ComicKeyElements
			if s.Repo != nil {
				ke, err := s.Repo.LoadKeyElements(sceneID)
				if err != nil && !isLikelyStorageNotFound(err) {
					utils.GetLogger().Warn("load key elements failed", map[string]interface{}{"scene_id": sceneID, "err": err})
				} else {
					keyElements = ke
				}
			}
			refImage := s.selectReferenceImageForFrame(sceneID, nil, promptText, keyElements, refIndex)
			if len(refImage) > 0 {
				opts.ReferenceImage = refImage
			}
		}
		if _, _, err := s.Vision.GenerateAndSaveFrame(jobCtx, sceneID, frameID, promptText, opts); err != nil {
			failMsg := s.buildFrameVisionFailureMessage(frameID, opts, err)
			tracker.EmitProgressEvent(50, failMsg, "frame_failed", frameID)
			tracker.Fail(failMsg)
			return err
		}
		s.saveFrameImageSignature(sceneID, frameID, fp, promptText)
		tracker.EmitProgressEvent(95, fmt.Sprintf("图片已写入：%s", frameID), "image_written", frameID)

		tracker.Complete("重绘完成")
		return nil
	}); err != nil {
		tracker.Fail("任务提交失败")
		return "", err
	}

	return taskID, nil
}

// GenerateFramesAsync starts async jobs to generate multiple frames in parallel.
func (s *ComicService) GenerateFramesAsync(ctx context.Context, sceneID string, frameIDs []string) (map[string]string, error) {
	if strings.TrimSpace(sceneID) == "" {
		return nil, errors.New("sceneID required")
	}
	if len(frameIDs) == 0 {
		return nil, errors.New("frameIDs required")
	}

	unique := make([]string, 0, len(frameIDs))
	seen := make(map[string]struct{}, len(frameIDs))
	for _, id := range frameIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, errors.New("frameIDs required")
	}

	tasks := make(map[string]string, len(unique))
	for _, id := range unique {
		taskID, err := s.RegenerateFrameAsync(ctx, sceneID, id)
		if err != nil {
			return nil, err
		}
		tasks[id] = taskID
	}

	return tasks, nil
}
