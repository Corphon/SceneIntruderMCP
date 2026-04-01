// internal/services/video_config_apply.go
package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
)

func ApplyVideoConfig(svc *VideoService, cfg *config.AppConfig) error {
	if svc == nil {
		return fmt.Errorf("video service is nil")
	}
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	provider := strings.TrimSpace(cfg.VideoProvider)
	if provider == "" {
		provider = "dashscope"
	}
	defaultModel := strings.TrimSpace(cfg.VideoDefaultModel)
	if defaultModel == "" {
		defaultModel = "wan2.6-i2v-flash"
	}

	svc.DefaultProvider = provider
	svc.DefaultModel = defaultModel
	svc.Provider = nil
	svc.PublicBaseURL = ""
	svc.FFmpegPath = "ffmpeg"
	if cfg.VideoConfig != nil {
		if v := strings.TrimSpace(cfg.VideoConfig["poll_interval_ms"]); v != "" {
			if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
				svc.PollInterval = time.Duration(ms) * time.Millisecond
			}
		}
		if v := strings.TrimSpace(cfg.VideoConfig["max_poll_attempts"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				svc.MaxPollAttempts = n
			}
		}
		if v := strings.TrimSpace(cfg.VideoConfig["clip_retry_count"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				svc.MaxClipRetries = n
			}
		}
		if v := strings.TrimSpace(cfg.VideoConfig["clip_cache_enabled"]); v != "" {
			lower := strings.ToLower(v)
			svc.ClipCacheEnabled = lower == "1" || lower == "true" || lower == "yes" || lower == "on"
		}
		if v := strings.TrimSpace(cfg.VideoConfig["public_base_url"]); v != "" {
			svc.PublicBaseURL = strings.TrimRight(v, "/")
		}
		if v := strings.TrimSpace(cfg.VideoConfig["fallback_compose"]); v != "" {
			lower := strings.ToLower(v)
			svc.FallbackComposeEnabled = lower == "1" || lower == "true" || lower == "yes" || lower == "on"
		}
		if v := strings.TrimSpace(cfg.VideoConfig["ffmpeg_path"]); v != "" {
			svc.FFmpegPath = v
		}
	}

	switch provider {
	case "mock":
		return nil
	case "dashscope":
		endpoint := ""
		apiKey := ""
		if cfg.VideoConfig != nil {
			endpoint = strings.TrimSpace(cfg.VideoConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VideoConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VideoConfig["api_key"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("dashscope video endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("dashscope video api_key missing")
		}
		providerClient := NewDashScopeVideoProvider(endpoint, apiKey)
		providerClient.PollEvery = svc.PollInterval
		providerClient.PollAttempts = svc.MaxPollAttempts
		if cfg.VideoConfig != nil {
			if v := strings.TrimSpace(cfg.VideoConfig["video_synthesis_path"]); v != "" {
				providerClient.SynthesisPath = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["tasks_path_prefix"]); v != "" {
				providerClient.TaskPathPrefix = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["model_wan2.6-i2v-flash"]); v != "" {
				providerClient.ModelKeyMapping["wan2.6-i2v-flash"] = v
			}
		}
		svc.Provider = providerClient
		return nil
	case "kling":
		endpoint := ""
		apiKey := ""
		if cfg.VideoConfig != nil {
			endpoint = strings.TrimSpace(cfg.VideoConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VideoConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VideoConfig["api_key"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("KLING_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("kling video endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("kling video api_key missing")
		}
		providerClient := NewKlingVideoProvider(endpoint, apiKey)
		providerClient.PollEvery = svc.PollInterval
		providerClient.PollAttempts = svc.MaxPollAttempts
		if cfg.VideoConfig != nil {
			if v := strings.TrimSpace(cfg.VideoConfig["video_synthesis_path"]); v != "" {
				providerClient.ImageToVideoPath = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["model_kling-v3"]); v != "" {
				providerClient.ModelKeyMapping["kling-v3"] = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["aspect_ratio"]); v != "" {
				providerClient.DefaultAspectRatio = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["kling_mode"]); v != "" {
				providerClient.DefaultMode = v
			}
		}
		svc.Provider = providerClient
		return nil
	case "google":
		endpoint := ""
		apiKey := ""
		if cfg.VideoConfig != nil {
			endpoint = strings.TrimSpace(cfg.VideoConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VideoConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VideoConfig["api_key"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("google veo endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("google veo api_key missing")
		}
		providerClient := NewGoogleVeoVideoProvider(endpoint, apiKey)
		providerClient.PollEvery = svc.PollInterval
		providerClient.PollAttempts = svc.MaxPollAttempts
		if cfg.VideoConfig != nil {
			if v := strings.TrimSpace(cfg.VideoConfig["generate_path_template"]); v != "" {
				providerClient.GeneratePathTemplate = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["operations_path_prefix"]); v != "" {
				providerClient.OperationsPathPrefix = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["model_veo-2"]); v != "" {
				providerClient.ModelKeyMapping["veo-2"] = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["aspect_ratio"]); v != "" {
				providerClient.DefaultAspectRatio = v
			}
		}
		svc.Provider = providerClient
		return nil
	case "vertex":
		endpoint := ""
		accessToken := ""
		if cfg.VideoConfig != nil {
			endpoint = strings.TrimSpace(cfg.VideoConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VideoConfig["base_url"])
			}
			accessToken = strings.TrimSpace(cfg.VideoConfig["api_key"])
		}
		if accessToken == "" {
			accessToken = strings.TrimSpace(os.Getenv("VERTEX_AI_ACCESS_TOKEN"))
		}
		if accessToken == "" {
			accessToken = strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_ACCESS_TOKEN"))
		}
		if endpoint == "" {
			return fmt.Errorf("vertex ai veo endpoint missing")
		}
		if accessToken == "" {
			return fmt.Errorf("vertex ai veo access token missing")
		}
		providerClient := NewVertexAIVeoVideoProvider(endpoint, accessToken)
		providerClient.PollEvery = svc.PollInterval
		providerClient.PollAttempts = svc.MaxPollAttempts
		if cfg.VideoConfig != nil {
			if v := strings.TrimSpace(cfg.VideoConfig["generate_path_template"]); v != "" {
				providerClient.GeneratePathTemplate = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["operations_path_prefix"]); v != "" {
				providerClient.OperationsPathPrefix = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["model_veo-2-vertex"]); v != "" {
				providerClient.ModelKeyMapping["veo-2-vertex"] = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["aspect_ratio"]); v != "" {
				providerClient.DefaultAspectRatio = v
			}
		}
		svc.Provider = providerClient
		return nil
	case "ark":
		endpoint := ""
		apiKey := ""
		if cfg.VideoConfig != nil {
			endpoint = strings.TrimSpace(cfg.VideoConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VideoConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VideoConfig["api_key"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("ARK_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("ark video endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("ark video api_key missing")
		}
		providerClient := NewArkVideoProvider(endpoint, apiKey)
		providerClient.PollEvery = svc.PollInterval
		providerClient.PollAttempts = svc.MaxPollAttempts
		if cfg.VideoConfig != nil {
			if v := strings.TrimSpace(cfg.VideoConfig["video_synthesis_path"]); v != "" {
				providerClient.GenerationPath = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["model_doubao-seedance-1-5-pro"]); v != "" {
				providerClient.ModelKeyMapping["doubao-seedance-1-5-pro"] = v
			}
			if v := strings.TrimSpace(cfg.VideoConfig["aspect_ratio"]); v != "" {
				providerClient.DefaultRatio = v
			}
		}
		svc.Provider = providerClient
		return nil
	default:
		return fmt.Errorf("unsupported video provider: %s", provider)
	}
}
