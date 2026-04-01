// internal/llm/reasoning_control.go
package llm

import "strings"

const (
	ExtraParamReasoningEnabled = "reasoning_enabled"
	ExtraParamEnableReasoning  = "enable_reasoning"
	ExtraParamDisableReasoning = "disable_reasoning"
)

func NormalizeReasoningRequest(provider string, model string, extra map[string]interface{}) (string, map[string]interface{}, bool) {
	cloned := cloneExtraParams(extra)
	reasoningEnabled := extractReasoningSwitch(cloned)
	if !reasoningEnabled {
		model = fallbackReasoningModel(provider, model)
	}
	return strings.TrimSpace(model), cloned, reasoningEnabled
}

func ApplyReasoningDefaults(provider string, requestBody map[string]interface{}, model string, reasoningEnabled bool) {
	if reasoningEnabled || requestBody == nil {
		return
	}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google":
		generationConfig, _ := requestBody["generationConfig"].(map[string]interface{})
		if generationConfig == nil {
			generationConfig = map[string]interface{}{}
			requestBody["generationConfig"] = generationConfig
		}
		if _, exists := generationConfig["thinkingConfig"]; !exists {
			generationConfig["thinkingConfig"] = map[string]interface{}{"thinkingBudget": 0}
		}
	case "qwen":
		if _, exists := requestBody["enable_thinking"]; exists {
			return
		}
		lowerModel := strings.ToLower(strings.TrimSpace(model))
		if strings.HasPrefix(lowerModel, "qwen") || strings.HasPrefix(lowerModel, "qwq") || strings.HasPrefix(lowerModel, "qvq") {
			requestBody["enable_thinking"] = false
		}
	case "nvidia":
		chatTemplateKwargs, _ := requestBody["chat_template_kwargs"].(map[string]interface{})
		if chatTemplateKwargs == nil {
			chatTemplateKwargs = map[string]interface{}{}
			requestBody["chat_template_kwargs"] = chatTemplateKwargs
		}
		if _, exists := chatTemplateKwargs["thinking"]; !exists {
			chatTemplateKwargs["thinking"] = false
		}
	}
}

func cloneExtraParams(extra map[string]interface{}) map[string]interface{} {
	if len(extra) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(extra))
	for k, v := range extra {
		cloned[k] = v
	}
	return cloned
}

func extractReasoningSwitch(extra map[string]interface{}) bool {
	if len(extra) == 0 {
		return false
	}

	if disabled, ok := popBoolLike(extra, ExtraParamDisableReasoning); ok {
		if disabled {
			return false
		}
	}
	if enabled, ok := popBoolLike(extra, ExtraParamReasoningEnabled); ok {
		return enabled
	}
	if enabled, ok := popBoolLike(extra, ExtraParamEnableReasoning); ok {
		return enabled
	}
	return false
}

func popBoolLike(extra map[string]interface{}, key string) (bool, bool) {
	value, exists := extra[key]
	if !exists {
		return false, false
	}
	delete(extra, key)
	return toBoolLike(value)
}

func toBoolLike(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on", "enabled":
			return true, true
		case "0", "false", "no", "off", "disabled":
			return false, true
		}
	case int:
		return v != 0, true
	case int32:
		return v != 0, true
	case int64:
		return v != 0, true
	case float32:
		return v != 0, true
	case float64:
		return v != 0, true
	}
	return false, false
}

func fallbackReasoningModel(provider string, model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return trimmed
	}

	lowerProvider := strings.ToLower(strings.TrimSpace(provider))
	lowerModel := strings.ToLower(trimmed)

	switch lowerProvider {
	case "anthropic":
		if lowerModel == "claude-3.7-sonnet-thinking" {
			return "claude-3.7-sonnet"
		}
	case "deepseek":
		if lowerModel == "deepseek-reasoner" {
			return "deepseek-chat"
		}
	case "google":
		if strings.Contains(lowerModel, "thinking") {
			return "gemini-2.5-flash"
		}
	case "qwen":
		if strings.HasPrefix(lowerModel, "qwq") || strings.HasPrefix(lowerModel, "qvq") {
			return "qwen3-max"
		}
	}

	return trimmed
}
