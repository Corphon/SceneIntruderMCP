// internal/llm/prompts/comic.go
package prompts

import (
	"fmt"
	"strings"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// ComicPromptConfig controls language/style/model for comic-related prompts.
type ComicPromptConfig struct {
	Language       string
	TargetFrames   int
	Style          string
	Model          string
	ContinuityMode string
	FrameAnchor    string
}

// ComicFramePromptTemplateVersion is a stable identifier for prompt-building strategy.
// It is persisted in prompt_sources for provenance.
const ComicFramePromptTemplateVersion = "comic_frame_prompt_v2"

func clampFrames(n int) int {
	if n <= 0 {
		return 4
	}
	if n < 4 {
		return 4
	}
	if n > 12 {
		return 12
	}
	return n
}

// BuildStoryAnalysisSystemPrompt returns a system prompt that constrains output into a stable JSON schema.
func BuildStoryAnalysisSystemPrompt(cfg ComicPromptConfig) string {
	target := clampFrames(cfg.TargetFrames)
	lang := strings.TrimSpace(cfg.Language)
	if lang == "" {
		lang = "auto"
	}

	return strings.TrimSpace(fmt.Sprintf(`You are a professional visual storyboard artist.

Task:
- Analyze the story and produce a %d-frame storyboard plan.
- Return ONLY valid JSON. No markdown. No code fences.

Constraints:
- frames must contain exactly %d items.
- Each frame must have: id (frame_1..frame_%d), order (1..%d), description (1-2 sentences).
- Prefer concise, visual, camera-friendly descriptions.
- language=%s applies to descriptions.

Output schema:
{
  "scene_id": "...",
  "language": "...",
  "target_frames": %d,
  "frames": [
    {"id":"frame_1","order":1,"description":"...","story_node_ids":["..."]}
  ]
}`, target, target, target, target, lang, target))
}

// BuildStoryAnalysisPrompt builds the user prompt for story-to-frames analysis.
func BuildStoryAnalysisPrompt(scene models.Scene, story models.StoryData, cfg ComicPromptConfig) string {
	target := clampFrames(cfg.TargetFrames)

	var b strings.Builder
	b.WriteString("[scene]\n")
	b.WriteString(fmt.Sprintf("id: %s\n", scene.ID))
	if strings.TrimSpace(scene.Title) != "" {
		b.WriteString(fmt.Sprintf("title: %s\n", scene.Title))
	}
	if strings.TrimSpace(scene.Name) != "" {
		b.WriteString(fmt.Sprintf("name: %s\n", scene.Name))
	}
	if strings.TrimSpace(scene.Description) != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", scene.Description))
	}
	if strings.TrimSpace(scene.Summary) != "" {
		b.WriteString(fmt.Sprintf("summary: %s\n", scene.Summary))
	}
	b.WriteString("\n[story]\n")
	if strings.TrimSpace(story.Intro) != "" {
		b.WriteString(fmt.Sprintf("intro: %s\n", story.Intro))
	}
	if strings.TrimSpace(story.MainObjective) != "" {
		b.WriteString(fmt.Sprintf("main_objective: %s\n", story.MainObjective))
	}
	b.WriteString("\n[nodes]\n")
	maxNodes := len(story.Nodes)
	if maxNodes > 20 {
		maxNodes = 20
	}
	for i := 0; i < maxNodes; i++ {
		n := story.Nodes[i]
		content := strings.TrimSpace(n.Content)
		if content == "" {
			content = strings.TrimSpace(n.OriginalContent)
		}
		if content == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- id=%s type=%s: %s\n", n.ID, n.Type, content))
	}

	b.WriteString("\n[requirements]\n")
	b.WriteString(fmt.Sprintf("- target_frames: %d\n", target))
	b.WriteString("- Ensure frames cover beginning/middle/end and key turning points.\n")

	return b.String()
}

// BuildStoryAnalysisPromptFromNodeContent builds a story-to-frames analysis prompt grounded on
// a selected node's full content from scene context (context.json conversations).
func BuildStoryAnalysisPromptFromNodeContent(scene models.Scene, nodeID string, nodeContent string, cfg ComicPromptConfig) string {
	target := clampFrames(cfg.TargetFrames)

	nodeID = strings.TrimSpace(nodeID)
	nodeContent = strings.TrimSpace(nodeContent)

	var b strings.Builder
	b.WriteString("[scene]\n")
	b.WriteString(fmt.Sprintf("id: %s\n", scene.ID))
	if strings.TrimSpace(scene.Title) != "" {
		b.WriteString(fmt.Sprintf("title: %s\n", scene.Title))
	}
	if strings.TrimSpace(scene.Name) != "" {
		b.WriteString(fmt.Sprintf("name: %s\n", scene.Name))
	}
	if strings.TrimSpace(scene.Description) != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", scene.Description))
	}
	if strings.TrimSpace(scene.Summary) != "" {
		b.WriteString(fmt.Sprintf("summary: %s\n", scene.Summary))
	}

	b.WriteString("\n[node]\n")
	if nodeID != "" {
		b.WriteString(fmt.Sprintf("node_id: %s\n", nodeID))
	}
	b.WriteString("content:\n")
	b.WriteString(nodeContent)

	b.WriteString("\n\n[requirements]\n")
	b.WriteString(fmt.Sprintf("- target_frames: %d\n", target))
	b.WriteString("- Ensure frames cover beginning/middle/end and key turning points.\n")
	if nodeID != "" {
		b.WriteString(fmt.Sprintf("- For every frame, set story_node_ids to include: %s\n", nodeID))
	}

	return b.String()
}

// BuildFramePromptSystemPrompt returns system constraints for generating vision-friendly prompts.
func BuildFramePromptSystemPrompt(cfg ComicPromptConfig, keyElements *models.ComicKeyElements) string {
	style := strings.TrimSpace(cfg.Style)
	continuityMode := strings.TrimSpace(cfg.ContinuityMode)
	if continuityMode == "" {
		continuityMode = "strict"
	}
	frameAnchor := strings.TrimSpace(cfg.FrameAnchor)
	styleRule := "- If style is provided, reflect it consistently across all frames."
	if style != "" {
		styleRule = fmt.Sprintf("- style=%s should be reflected consistently across all frames.", style)
	}
	nonComicLexiconRule := "- Avoid conflicting style words when they contradict the requested style."
	if style != "" && !strings.EqualFold(style, "comic") {
		nonComicLexiconRule = "- Do NOT use lexical cues such as comic style, comic panel, comic book, graphic novel, speech bubble, word balloon, halftone dots unless explicitly required by the requested style."
	}
	continuityRule := fmt.Sprintf("- continuity_mode=%s: keep identity and visual continuity across adjacent frames.", continuityMode)
	if frameAnchor != "" {
		continuityRule += fmt.Sprintf(" Use frame_anchor as fixed constraints: %s.", frameAnchor)
	}
	modelKey := strings.TrimSpace(cfg.Model)
	hints := GetModelHints(modelKey)
	modelHintsBlock := ""
	if strings.TrimSpace(hints.PromptRules) != "" {
		modelHintsBlock = "\n\nModel-specific guidance:\n" + strings.TrimSpace(hints.PromptRules)
	}
	if strings.TrimSpace(hints.NegativePromptHint) != "" {
		modelHintsBlock += "\n- negative_prompt hint: " + strings.TrimSpace(hints.NegativePromptHint)
	}
	if strings.TrimSpace(hints.ModelParamsHint) != "" {
		modelHintsBlock += "\n- model_params hint: " + strings.TrimSpace(hints.ModelParamsHint)
	}
	if strings.TrimSpace(hints.Notes) != "" {
		modelHintsBlock += "\n- notes: " + strings.TrimSpace(hints.Notes)
	}
	if strings.TrimSpace(modelHintsBlock) != "" {
		modelHintsBlock = strings.TrimRight(modelHintsBlock, "\n")
	}

	anchorsBlock := ""
	if keyElements != nil {
		var b strings.Builder
		if len(keyElements.Characters) > 0 {
			b.WriteString("- Characters (keep identity consistent):\n")
			for _, c := range keyElements.Characters {
				name := strings.TrimSpace(c.Name)
				if name == "" {
					continue
				}
				b.WriteString(fmt.Sprintf("  - %s", name))
				desc := strings.TrimSpace(c.Description)
				if desc != "" {
					b.WriteString(": " + desc)
				}
				b.WriteString("\n")
			}
		}
		if len(keyElements.Objects) > 0 {
			b.WriteString("- Objects (keep appearance consistent):\n")
			for _, o := range keyElements.Objects {
				name := strings.TrimSpace(o.Name)
				if name == "" {
					continue
				}
				b.WriteString(fmt.Sprintf("  - %s", name))
				desc := strings.TrimSpace(o.Description)
				if desc != "" {
					b.WriteString(": " + desc)
				}
				b.WriteString("\n")
			}
		}
		if len(keyElements.Locations) > 0 {
			b.WriteString("- Locations (keep atmosphere consistent):\n")
			for _, l := range keyElements.Locations {
				name := strings.TrimSpace(l.Name)
				if name == "" {
					continue
				}
				b.WriteString(fmt.Sprintf("  - %s", name))
				desc := strings.TrimSpace(l.Description)
				if desc != "" {
					b.WriteString(": " + desc)
				}
				b.WriteString("\n")
			}
		}
		if len(keyElements.StyleTags) > 0 {
			b.WriteString("- Style tags (reuse across frames):\n")
			b.WriteString("  - " + strings.Join(keyElements.StyleTags, ", ") + "\n")
		}
		anchorsBlock = strings.TrimRight(b.String(), "\n")
		if anchorsBlock != "" {
			anchorsBlock = "\n\nGlobal anchors (apply to EVERY frame):\n" + anchorsBlock
		}
	}

	return strings.TrimSpace(fmt.Sprintf(`You write image-generation prompts.

Return ONLY valid JSON. No markdown.

Constraints:
- prompt MUST be in English (vision-model friendly).
- Keep it concise but specific: characters, setting, action, composition, lighting, style.
- All frames belong to the same visual narrative sequence: keep recurring characters' identity consistent (face, outfit, key props) and keep the overall art style coherent across frames.
- Prefer a consistent "style signature" across frames (repeat core style keywords rather than inventing new ones each frame).
- Keep color palette, line weight, and rendering technique consistent across frames unless the story explicitly changes them.
- Avoid re-describing the same character with conflicting appearance details.
- Prefer a stable negative_prompt across frames: start from a baseline (blurry, watermark, text, logo) and only append extra constraints when truly needed.
- %s
- %s
- %s%s%s

Output schema:
{"frame_id":"frame_1","prompt":"...","negative_prompt":"...","style":"%s","model":"%s","model_params":{}}`, styleRule, nonComicLexiconRule, continuityRule, modelHintsBlock, anchorsBlock, style, modelKey))
}

// BuildFramePrompt builds the user prompt for a single frame.
// nodeIDs + nodeContent are optional grounding inputs sourced from scene context (e.g. context.json).
func BuildFramePrompt(scene models.Scene, frame models.ComicFramePlan, cfg ComicPromptConfig, nodeIDs []string, nodeContent string, previousFramePrompt string) string {
	style := strings.TrimSpace(cfg.Style)
	continuityMode := strings.TrimSpace(cfg.ContinuityMode)
	if continuityMode == "" {
		continuityMode = "strict"
	}
	frameAnchor := strings.TrimSpace(cfg.FrameAnchor)
	previousFramePrompt = strings.TrimSpace(previousFramePrompt)

	var b strings.Builder
	b.WriteString("[scene]\n")
	b.WriteString(fmt.Sprintf("- title: %s\n", strings.TrimSpace(scene.Title)))
	b.WriteString(fmt.Sprintf("- description: %s\n", strings.TrimSpace(scene.Description)))

	b.WriteString("\n[frame]\n")
	b.WriteString(fmt.Sprintf("- id: %s\n", frame.ID))
	b.WriteString(fmt.Sprintf("- order: %d\n", frame.Order))
	b.WriteString(fmt.Sprintf("- description: %s\n", strings.TrimSpace(frame.Description)))

	if strings.TrimSpace(nodeContent) != "" {
		b.WriteString("\n[node_content]\n")
		if len(nodeIDs) > 0 {
			b.WriteString(fmt.Sprintf("- story_node_ids: %s\n", strings.Join(nodeIDs, ",")))
		}
		b.WriteString(nodeContent)
		b.WriteString("\n")
	}

	b.WriteString("\n[requirements]\n")
	b.WriteString("- output English prompt\n")
	if style != "" {
		b.WriteString(fmt.Sprintf("- include style keywords: %s\n", style))
	}
	b.WriteString(fmt.Sprintf("- continuity_mode: %s\n", continuityMode))
	if frameAnchor != "" {
		b.WriteString(fmt.Sprintf("- frame_anchor: %s\n", frameAnchor))
	}
	if previousFramePrompt != "" {
		b.WriteString("- previous_frame_prompt (for continuity only):\n")
		b.WriteString(previousFramePrompt)
		b.WriteString("\n")
	}
	b.WriteString("- keep recurring character identity consistent across frames (same face/outfit/props)\n")
	b.WriteString("- keep negative_prompt stable across frames; avoid unnecessary variations\n")
	b.WriteString("- negative_prompt baseline: blurry, watermark, text, logo")

	return strings.TrimSpace(b.String())
}

// BuildKeyElementsSystemPrompt constrains extraction output into a stable JSON schema.
func BuildKeyElementsSystemPrompt(cfg ComicPromptConfig) string {
	style := strings.TrimSpace(cfg.Style)
	styleTagExample := "..."
	if style != "" {
		styleTagExample = style
	}
	return strings.TrimSpace(fmt.Sprintf(`You extract key elements for a visual narrative production pipeline.

Return ONLY valid JSON. No markdown.

Rules:
- Extract the most important and recurring elements.
- Keep lists short and high-signal.
- Prefer proper names for characters.

Output schema:
{
  "scene_id": "...",
  "characters": [{"id":"char_x","name":"...","description":"..."}],
  "objects": [{"id":"obj_x","name":"...","description":"..."}],
  "locations": [{"id":"loc_x","name":"...","description":"..."}],
	"style_tags": ["%s"]
}`, styleTagExample))
}

// BuildKeyElementsPrompt builds the user prompt from analysis and per-frame prompts.
func BuildKeyElementsPrompt(scene models.Scene, breakdown models.ComicBreakdown, framePrompts []models.ComicFramePrompt, cfg ComicPromptConfig) string {
	style := strings.TrimSpace(cfg.Style)

	var b strings.Builder
	b.WriteString("[scene]\n")
	b.WriteString(fmt.Sprintf("id: %s\n", scene.ID))
	if strings.TrimSpace(scene.Title) != "" {
		b.WriteString(fmt.Sprintf("title: %s\n", strings.TrimSpace(scene.Title)))
	}
	if strings.TrimSpace(scene.Description) != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", strings.TrimSpace(scene.Description)))
	}
	b.WriteString("\n[analysis]\n")
	b.WriteString(fmt.Sprintf("target_frames: %d\n", breakdown.TargetFrames))

	b.WriteString("\n[frames]\n")
	maxFrames := len(breakdown.Frames)
	if maxFrames > 12 {
		maxFrames = 12
	}
	for i := 0; i < maxFrames; i++ {
		f := breakdown.Frames[i]
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.ID, strings.TrimSpace(f.Description)))
	}

	b.WriteString("\n[prompts]\n")
	maxPrompts := len(framePrompts)
	if maxPrompts > 12 {
		maxPrompts = 12
	}
	for i := 0; i < maxPrompts; i++ {
		p := framePrompts[i]
		b.WriteString(fmt.Sprintf("- %s: %s\n", p.FrameID, strings.TrimSpace(p.Prompt)))
	}

	b.WriteString("\n[requirements]\n")
	b.WriteString("- Provide stable ids; if unsure, leave id empty and we will fill it.\n")
	b.WriteString("- style_tags should be short keywords.\n")
	if style != "" {
		b.WriteString(fmt.Sprintf("- include style: %s\n", style))
	}

	return b.String()
}
