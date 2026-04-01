// internal/services/video_prompt_options.go
package services

import (
	"strings"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

func sanitizeVideoPromptOptions(opts models.VideoPromptOptions) models.VideoPromptOptions {
	opts.Language = normalizeVideoPromptLanguage(opts.Language)
	opts.DialogueStyle = normalizeVideoPromptDialogueStyle(opts.DialogueStyle)
	opts.DialogueDensity = normalizeVideoPromptDialogueDensity(opts.DialogueDensity)
	opts.DialogueEmotion = normalizeVideoPromptStrength(opts.DialogueEmotion, "moderate")
	opts.MotionStrength = normalizeVideoPromptStrength(opts.MotionStrength, "moderate")
	opts.EnvironmentMotion = normalizeVideoPromptStrength(opts.EnvironmentMotion, "moderate")
	opts.EnvironmentLayer = normalizeVideoPromptEnvironmentLayer(opts.EnvironmentLayer)
	opts.ExpressionIntensity = normalizeVideoPromptStrength(opts.ExpressionIntensity, "moderate")
	opts.CameraStyle = normalizeVideoPromptCameraStyle(opts.CameraStyle)
	opts.CameraPacing = normalizeVideoPromptCameraPacing(opts.CameraPacing)
	opts.CameraTransition = normalizeVideoPromptCameraTransition(opts.CameraTransition)
	opts.PromptSuffix = strings.TrimSpace(opts.PromptSuffix)
	return opts
}

func normalizeVideoPromptLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto", "default":
		return "auto"
	case "zh", "zh-cn", "zh_cn", "cn", "chinese", "mandarin":
		return "zh-cn"
	case "en", "en-us", "en_us", "english":
		return "en"
	default:
		return "auto"
	}
}

func normalizeVideoPromptDialogueStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto", "default":
		return "auto"
	case "historical", "historical_drama", "history":
		return "historical_drama"
	case "serious", "serious_drama", "drama":
		return "serious_drama"
	case "tragedy", "tragic":
		return "tragedy"
	case "comedy", "comic", "humor", "humorous":
		return "comedy"
	case "epic", "heroic":
		return "epic"
	case "poetic", "literary":
		return "poetic"
	case "documentary", "documentary_voice", "doc":
		return "documentary"
	default:
		return "auto"
	}
}

func normalizeVideoPromptDialogueDensity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto", "balanced", "moderate", "normal":
		return "balanced"
	case "sparse", "low", "light", "minimal":
		return "sparse"
	case "dense", "high", "rich", "frequent":
		return "dense"
	default:
		return "balanced"
	}
}

func normalizeVideoPromptEnvironmentLayer(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto", "balanced", "mixed":
		return "auto"
	case "wind", "breeze", "gust":
		return "wind"
	case "rain", "drizzle", "storm":
		return "rain"
	case "smoke", "mist", "fog":
		return "smoke"
	case "dust", "sand", "debris":
		return "dust"
	case "crowd", "people", "pedestrian":
		return "crowd"
	default:
		return "auto"
	}
}

func normalizeVideoPromptStrength(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto":
		return fallback
	case "subtle", "light", "low", "gentle":
		return "subtle"
	case "moderate", "medium", "normal", "balanced":
		return "moderate"
	case "strong", "high", "intense", "dramatic":
		return "strong"
	default:
		return fallback
	}
}

func normalizeVideoPromptCameraStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto", "cinematic":
		return "cinematic"
	case "immersive", "immersive_camera":
		return "immersive"
	case "documentary", "doc":
		return "documentary"
	case "anime", "anime_dynamic", "dynamic_anime":
		return "anime_dynamic"
	case "poetic", "poetic_epic", "epic":
		return "poetic_epic"
	default:
		return "cinematic"
	}
}

func normalizeVideoPromptCameraPacing(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto", "balanced", "moderate", "normal":
		return "balanced"
	case "slow", "measured", "steady":
		return "slow"
	case "fast", "dynamic", "urgent", "brisk":
		return "fast"
	default:
		return "balanced"
	}
}

func normalizeVideoPromptCameraTransition(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto", "steady", "steady_cut", "stable_cut":
		return "steady_cut"
	case "hard", "hard_cut", "snap_cut":
		return "hard_cut"
	case "follow", "follow_match", "tracking_match":
		return "follow_match"
	default:
		return "steady_cut"
	}
}

func buildFallbackVideoPromptBase(imagePrompt string, frameDescription string) string {
	base := strings.TrimSpace(imagePrompt)
	frameDescription = strings.TrimSpace(frameDescription)
	parts := make([]string, 0, 4)
	if base != "" {
		parts = append(parts, base)
	}
	if frameDescription != "" {
		parts = append(parts, "preserve the exact story beat: "+frameDescription)
	}
	parts = append(parts,
		"turn the still frame into a living moment with visible subject motion and environmental motion",
		"stable character continuity and coherent spatial staging",
	)
	return joinPromptParts(parts...)
}

func buildEnhancedVideoPrompt(basePrompt string, frameDescription string, opts models.VideoPromptOptions) string {
	opts = sanitizeVideoPromptOptions(opts)
	basePrompt = strings.TrimSpace(basePrompt)
	if basePrompt == "" {
		basePrompt = buildFallbackVideoPromptBase("", frameDescription)
	}
	parts := []string{
		basePrompt,
		motionStrengthClause(opts.MotionStrength),
		environmentMotionClause(opts.EnvironmentMotion),
		environmentLayerClause(opts.EnvironmentLayer),
		expressionIntensityClause(opts.ExpressionIntensity),
		cameraStyleClause(opts.CameraStyle),
		cameraPacingClause(opts.CameraPacing),
		cameraTransitionClause(opts.CameraTransition),
		"natural body mechanics, clear silhouette changes, coherent eye focus, expressive hand performance, layered foreground and background motion, cloth and hair secondary motion",
	}
	if clause := dialogueLanguageClause(opts.Language); clause != "" {
		parts = append(parts, clause)
	}
	if clause := dialogueStyleClause(opts.DialogueStyle); clause != "" {
		parts = append(parts, clause)
	}
	if clause := dialogueDensityClause(opts.DialogueDensity); clause != "" {
		parts = append(parts, clause)
	}
	if clause := dialogueEmotionClause(opts.DialogueEmotion); clause != "" {
		parts = append(parts, clause)
	}
	if suffix := strings.TrimSpace(opts.PromptSuffix); suffix != "" {
		parts = append(parts, suffix)
	}
	return joinPromptParts(parts...)
}

func motionStrengthClause(value string) string {
	switch normalizeVideoPromptStrength(value, "moderate") {
	case "subtle":
		return "subtle natural motion with restrained but readable movement arcs"
	case "strong":
		return "dynamic motion with strong momentum, readable action arcs, and impactful follow-through"
	default:
		return "expressive motion with believable acceleration, deceleration, and clean follow-through"
	}
}

func expressionIntensityClause(value string) string {
	switch normalizeVideoPromptStrength(value, "moderate") {
	case "subtle":
		return "nuanced micro-expressions, slight gaze shifts, and restrained emotional beats"
	case "strong":
		return "highly readable facial expression changes, strong emotional beats, and animated gestures"
	default:
		return "clear facial expression shifts, reactive eye focus, and natural mouth and shoulder movement"
	}
}

func environmentMotionClause(value string) string {
	switch normalizeVideoPromptStrength(value, "moderate") {
	case "subtle":
		return "background elements should move gently with restrained wind, dust, smoke, light, or crowd motion"
	case "strong":
		return "environmental motion should be vivid and scene-driving, with strong wind, debris, atmosphere, crowd flow, and layered background activity"
	default:
		return "environmental motion should stay clearly alive through wind, atmosphere, props, crowd flow, and layered background activity"
	}
}

func environmentLayerClause(value string) string {
	switch normalizeVideoPromptEnvironmentLayer(value) {
	case "wind":
		return "prioritize wind-driven environmental layers such as cloth drag, hair flow, banners, leaves, drifting particles, and gust response"
	case "rain":
		return "prioritize rain-driven environmental layers such as rainfall streaks, wet atmosphere, splash response, and moisture haze"
	case "smoke":
		return "prioritize smoke and haze layers with drifting plumes, volumetric atmosphere, and soft depth separation"
	case "dust":
		return "prioritize dust and debris layers with kicked-up particles, ground disturbance, and suspended grit"
	case "crowd":
		return "prioritize crowd-flow layers with believable pedestrian motion, background extras, and social activity continuity"
	default:
		return "keep environmental layers context-aware, choosing the most fitting mix of wind, rain, smoke, dust, or crowd movement for the scene"
	}
}

func cameraStyleClause(value string) string {
	switch normalizeVideoPromptCameraStyle(value) {
	case "immersive":
		return "immersive camera feeling with parallax depth and viewer-in-the-scene perspective"
	case "documentary":
		return "documentary-style camera realism with grounded framing and observational movement"
	case "anime_dynamic":
		return "dynamic anime camera staging with bold composition, speed emphasis, and dramatic perspective"
	case "poetic_epic":
		return "poetic epic framing with lyrical camera rhythm and grand heroic composition"
	default:
		return "cinematic camera language with purposeful framing, depth, and clean focus transitions"
	}
}

func cameraPacingClause(value string) string {
	switch normalizeVideoPromptCameraPacing(value) {
	case "slow":
		return "camera pacing should be measured and lingering, allowing beats to breathe before the next visual change"
	case "fast":
		return "camera pacing should feel brisk and energetic, with quicker visual beats and tighter transition timing"
	default:
		return "camera pacing should stay balanced, with clear beats, readable transitions, and steady shot rhythm"
	}
}

func cameraTransitionClause(value string) string {
	switch normalizeVideoPromptCameraTransition(value) {
	case "hard_cut":
		return "shot transitions should feel crisp and decisive, with hard visual cuts and immediate momentum shifts"
	case "follow_match":
		return "shot transitions should connect through motion matching or follow-through, carrying subject movement smoothly across cuts"
	default:
		return "shot transitions should stay steady and readable, using stable cuts that preserve orientation and continuity"
	}
}

func dialogueLanguageClause(value string) string {
	switch normalizeVideoPromptLanguage(value) {
	case "zh-cn":
		return "if dialogue or vocalization is present, spoken language should be Mandarin Chinese with natural lip-sync intent"
	case "en":
		return "if dialogue or vocalization is present, spoken language should be natural English with clear lip-sync intent"
	default:
		return ""
	}
}

func dialogueStyleClause(value string) string {
	switch normalizeVideoPromptDialogueStyle(value) {
	case "historical_drama":
		return "if dialogue or narration is present, use a dignified historical-drama cadence with restrained gravitas"
	case "serious_drama":
		return "if dialogue or narration is present, use serious dramatic delivery with grounded emotional weight"
	case "tragedy":
		return "if dialogue or narration is present, use restrained tragic delivery with heavy emotional aftertaste"
	case "comedy":
		return "if dialogue or narration is present, use light comedic timing with lively rhythm and playful reaction beats"
	case "epic":
		return "if dialogue or narration is present, use heroic epic delivery with bold rhythm and elevated emotion"
	case "poetic":
		return "if dialogue or narration is present, use poetic literary cadence with elegant phrasing and lyrical pauses"
	case "documentary":
		return "if dialogue or narration is present, use concise documentary-style delivery with observational clarity"
	default:
		return ""
	}
}

func dialogueDensityClause(value string) string {
	switch normalizeVideoPromptDialogueDensity(value) {
	case "sparse":
		return "keep dialogue or vocalization sparse, prioritizing silence, breath, and reaction beats over frequent spoken lines"
	case "dense":
		return "allow denser dialogue or vocal reaction beats, with more frequent line delivery, mouth movement, and conversational rhythm when suitable"
	default:
		return "keep dialogue density balanced, mixing spoken beats with pauses, reactions, and visual storytelling"
	}
}

func dialogueEmotionClause(value string) string {
	switch normalizeVideoPromptStrength(value, "moderate") {
	case "subtle":
		return "if dialogue or vocalization is present, keep emotional delivery restrained, with small tonal lifts and controlled reaction beats"
	case "strong":
		return "if dialogue or vocalization is present, let emotional delivery land strongly, with vivid tonal swings, clear emphasis, and forceful reaction beats"
	default:
		return "if dialogue or vocalization is present, keep emotional delivery readable and grounded, with clear feeling shifts and natural emphasis"
	}
}

func applyVideoPromptOptionsPatch(current models.VideoPromptOptions, patch *models.VideoPromptOptionsPatch) models.VideoPromptOptions {
	if patch == nil {
		return sanitizeVideoPromptOptions(current)
	}
	if patch.Language != nil {
		current.Language = *patch.Language
	}
	if patch.DialogueStyle != nil {
		current.DialogueStyle = *patch.DialogueStyle
	}
	if patch.DialogueDensity != nil {
		current.DialogueDensity = *patch.DialogueDensity
	}
	if patch.DialogueEmotion != nil {
		current.DialogueEmotion = *patch.DialogueEmotion
	}
	if patch.MotionStrength != nil {
		current.MotionStrength = *patch.MotionStrength
	}
	if patch.EnvironmentMotion != nil {
		current.EnvironmentMotion = *patch.EnvironmentMotion
	}
	if patch.EnvironmentLayer != nil {
		current.EnvironmentLayer = *patch.EnvironmentLayer
	}
	if patch.ExpressionIntensity != nil {
		current.ExpressionIntensity = *patch.ExpressionIntensity
	}
	if patch.CameraStyle != nil {
		current.CameraStyle = *patch.CameraStyle
	}
	if patch.CameraPacing != nil {
		current.CameraPacing = *patch.CameraPacing
	}
	if patch.CameraTransition != nil {
		current.CameraTransition = *patch.CameraTransition
	}
	if patch.PromptSuffix != nil {
		current.PromptSuffix = *patch.PromptSuffix
	}
	return sanitizeVideoPromptOptions(current)
}

func joinPromptParts(parts ...string) string {
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return strings.Join(out, ", ")
}
