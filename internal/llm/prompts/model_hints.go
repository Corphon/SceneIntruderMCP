// internal/llm/prompts/model_hints.go
package prompts

import "strings"

// ModelHints provides model-specific guidance for prompt generation.
// It is designed to be embedded into LLM system prompts.
//
// Notes:
// - Keep this guidance high-level and stable; avoid brittle vendor-specific syntax unless the model truly requires it.
// - Downstream services should remain compatible even if the LLM ignores these hints.
type ModelHints struct {
	PromptRules        string
	NegativePromptHint string
	ModelParamsHint    string
	Notes              string
}

func GetModelHints(modelKey string) ModelHints {
	k := strings.ToLower(strings.TrimSpace(modelKey))
	if k == "" {
		return ModelHints{}
	}

	switch k {
	case "sd", "sdwebui":
		return ModelHints{
			PromptRules:        "- Prefer Stable Diffusion-style keywords and composition tokens; avoid Midjourney flag syntax (no --stylize/--chaos).\n- If a reference image will be used (img2img), keep identity-consistent descriptors for the main subject.",
			NegativePromptHint: "low quality, blurry, watermark, text, logo, extra fingers, bad anatomy",
			ModelParamsHint:    "steps, cfg_scale, sampler, seed, denoising_strength, clip_skip, eta, tiling",
		}
	case "flux":
		return ModelHints{
			PromptRules:        "- Prefer clear, photographic/compositional natural-language prompts; avoid Midjourney flag syntax (no --stylize/--chaos).\n- Keep style consistent across frames (same camera language, lighting, palette).",
			NegativePromptHint: "blurry, low detail, watermark, text, logo",
		}
	case "midjourney":
		return ModelHints{
			PromptRules:     "- Use Midjourney-style prompt phrasing; optional flags may be appended at the end.\n- Keep it short; put the most important subject/scene first.",
			ModelParamsHint: "seed, stylize, chaos",
			Notes:           "The system stores negative_prompt for compatibility, but Midjourney may ignore it.",
		}
	case "qwen-image-2.0", "nano-banana-pro", "doubao-seedream-4.5", "gpt-image-1.5", "glm-image", "placeholder":
		return ModelHints{
			PromptRules:        "- Use clean English natural-language prompts with concrete visual details.\n- Avoid vendor-specific command/flag syntax.",
			NegativePromptHint: "blurry, watermark, text, logo",
		}
	default:
		return ModelHints{
			PromptRules: "- Use clean English natural-language prompts with concrete visual details.",
		}
	}
}
