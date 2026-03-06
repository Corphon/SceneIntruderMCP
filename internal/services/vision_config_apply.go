// internal/services/vision_config_apply.go
package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/vision/providers"
)

// ApplyVisionConfig applies AppConfig vision settings to an existing VisionService instance.
// It keeps the placeholder provider available as a safe fallback.
func ApplyVisionConfig(svc *VisionService, cfg *config.AppConfig) error {
	if svc == nil {
		return fmt.Errorf("vision service is nil")
	}
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Always keep placeholder available.
	svc.RegisterProvider("placeholder", providers.NewPlaceholderVisionProvider())

	provider := strings.TrimSpace(cfg.VisionProvider)
	if provider == "" {
		provider = "placeholder"
	}

	// Best-effort provider registration based on config.
	switch provider {
	case "placeholder":
		// nothing else
	case "sdwebui":
		endpoint := ""
		if cfg.VisionConfig != nil {
			endpoint = strings.TrimSpace(cfg.VisionConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VisionConfig["base_url"])
			}
		}
		if endpoint == "" {
			return fmt.Errorf("sdwebui endpoint missing")
		}
		svc.RegisterProvider("sdwebui", providers.NewSDWebUIProvider(endpoint))
	case "dashscope":
		endpoint := ""
		apiKey := ""
		genPath := ""
		if cfg.VisionConfig != nil {
			endpoint = strings.TrimSpace(cfg.VisionConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VisionConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VisionConfig["api_key"])
			genPath = strings.TrimSpace(cfg.VisionConfig["generation_path"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("dashscope endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("dashscope api_key missing")
		}
		ds := providers.NewDashScopeProvider(endpoint, apiKey)
		if genPath != "" {
			ds.GenerationPath = genPath
		}
		svc.RegisterProvider("dashscope", ds)
	case "gemini":
		endpoint := ""
		apiKey := ""
		authMode := ""
		defaultModel := ""
		modelNanoBananaPro := ""
		if cfg.VisionConfig != nil {
			endpoint = strings.TrimSpace(cfg.VisionConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VisionConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VisionConfig["api_key"])
			authMode = strings.TrimSpace(cfg.VisionConfig["auth_mode"])
			defaultModel = strings.TrimSpace(cfg.VisionConfig["model"])
			modelNanoBananaPro = strings.TrimSpace(cfg.VisionConfig["model_nano-banana-pro"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("gemini endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("gemini api_key missing")
		}
		gp := providers.NewGeminiProvider(endpoint, apiKey)
		if authMode != "" {
			gp.AuthMode = authMode
		}
		if defaultModel != "" {
			gp.ModelOverride = defaultModel
		}
		gp.ModelKeyOverrides = map[string]string{}
		if modelNanoBananaPro != "" {
			gp.ModelKeyOverrides["nano-banana-pro"] = modelNanoBananaPro
		}
		svc.RegisterProvider("gemini", gp)
	case "ark":
		endpoint := ""
		apiKey := ""
		genPath := ""
		modelOverride := ""
		modelDoubaoSeedream45 := ""
		sizeOverride := ""
		sizeDoubaoSeedream45 := ""
		if cfg.VisionConfig != nil {
			endpoint = strings.TrimSpace(cfg.VisionConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VisionConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VisionConfig["api_key"])
			genPath = strings.TrimSpace(cfg.VisionConfig["generation_path"])
			modelOverride = strings.TrimSpace(cfg.VisionConfig["model"])
			modelDoubaoSeedream45 = strings.TrimSpace(cfg.VisionConfig["model_doubao-seedream-4.5"])
			sizeOverride = strings.TrimSpace(cfg.VisionConfig["size"])
			sizeDoubaoSeedream45 = strings.TrimSpace(cfg.VisionConfig["size_doubao-seedream-4.5"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("ARK_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("ark endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("ark api_key missing")
		}
		ap := providers.NewArkImagesProvider(endpoint, apiKey)
		if genPath != "" {
			ap.GenerationPath = genPath
		}
		if modelOverride != "" {
			ap.ModelOverride = modelOverride
		}
		if sizeOverride != "" {
			ap.SizeOverride = sizeOverride
		}
		ap.ModelKeyOverrides = map[string]string{}
		if modelDoubaoSeedream45 != "" {
			ap.ModelKeyOverrides["doubao-seedream-4.5"] = modelDoubaoSeedream45
		}
		ap.SizeKeyOverrides = map[string]string{}
		if sizeDoubaoSeedream45 != "" {
			ap.SizeKeyOverrides["doubao-seedream-4.5"] = sizeDoubaoSeedream45
		}
		svc.RegisterProvider("ark", ap)
	case "openai":
		endpoint := ""
		apiKey := ""
		genPath := ""
		modelOverride := ""
		modelGPTImage15 := ""
		sizeOverride := ""
		sizeGPTImage15 := ""
		if cfg.VisionConfig != nil {
			endpoint = strings.TrimSpace(cfg.VisionConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VisionConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VisionConfig["api_key"])
			genPath = strings.TrimSpace(cfg.VisionConfig["generation_path"])
			modelOverride = strings.TrimSpace(cfg.VisionConfig["model"])
			modelGPTImage15 = strings.TrimSpace(cfg.VisionConfig["model_gpt-image-1.5"])
			sizeOverride = strings.TrimSpace(cfg.VisionConfig["size"])
			sizeGPTImage15 = strings.TrimSpace(cfg.VisionConfig["size_gpt-image-1.5"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("openai endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("openai api_key missing")
		}
		op := providers.NewOpenAIImagesProvider(endpoint, apiKey)
		if genPath != "" {
			op.GenerationPath = genPath
		}
		if modelOverride != "" {
			op.ModelOverride = modelOverride
		}
		if sizeOverride != "" {
			op.SizeOverride = sizeOverride
		}
		op.ModelKeyOverrides = map[string]string{}
		if modelGPTImage15 != "" {
			op.ModelKeyOverrides["gpt-image-1.5"] = modelGPTImage15
		}
		op.SizeKeyOverrides = map[string]string{}
		if sizeGPTImage15 != "" {
			op.SizeKeyOverrides["gpt-image-1.5"] = sizeGPTImage15
		}
		svc.RegisterProvider("openai", op)
	case "glm":
		endpoint := ""
		apiKey := ""
		genPath := ""
		modelOverride := ""
		modelGLMImage := ""
		sizeOverride := ""
		sizeGLMImage := ""
		if cfg.VisionConfig != nil {
			endpoint = strings.TrimSpace(cfg.VisionConfig["endpoint"])
			if endpoint == "" {
				endpoint = strings.TrimSpace(cfg.VisionConfig["base_url"])
			}
			apiKey = strings.TrimSpace(cfg.VisionConfig["api_key"])
			genPath = strings.TrimSpace(cfg.VisionConfig["generation_path"])
			modelOverride = strings.TrimSpace(cfg.VisionConfig["model"])
			modelGLMImage = strings.TrimSpace(cfg.VisionConfig["model_glm-image"])
			sizeOverride = strings.TrimSpace(cfg.VisionConfig["size"])
			sizeGLMImage = strings.TrimSpace(cfg.VisionConfig["size_glm-image"])
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GLM_API_KEY"))
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("BIGMODEL_API_KEY"))
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("ZHIPUAI_API_KEY"))
		}
		if endpoint == "" {
			return fmt.Errorf("glm endpoint missing")
		}
		if apiKey == "" {
			return fmt.Errorf("glm api_key missing")
		}
		gp := providers.NewGLMImagesProvider(endpoint, apiKey)
		if genPath != "" {
			gp.GenerationPath = genPath
		}
		if modelOverride != "" {
			gp.ModelOverride = modelOverride
		}
		if sizeOverride != "" {
			gp.SizeOverride = sizeOverride
		}
		gp.ModelKeyOverrides = map[string]string{}
		if modelGLMImage != "" {
			gp.ModelKeyOverrides["glm-image"] = modelGLMImage
		}
		gp.SizeKeyOverrides = map[string]string{}
		if sizeGLMImage != "" {
			gp.SizeKeyOverrides["glm-image"] = sizeGLMImage
		}
		svc.RegisterProvider("glm", gp)
	default:
		return fmt.Errorf("unsupported vision provider: %s", provider)
	}

	// Defaults.
	defaultModel := strings.TrimSpace(cfg.VisionDefaultModel)
	if defaultModel == "" {
		if provider == "sdwebui" {
			defaultModel = "sd"
		} else if provider == "dashscope" {
			defaultModel = "qwen-image-max"
		} else if provider == "gemini" {
			defaultModel = "nano-banana-pro"
		} else if provider == "ark" {
			defaultModel = "doubao-seedream-4.5"
		} else if provider == "openai" {
			defaultModel = "gpt-image-1.5"
		} else if provider == "glm" {
			defaultModel = "glm-image"
		} else {
			defaultModel = "placeholder"
		}
	}

	// Model provider routing.
	modelProviders := make(map[string]string)
	if cfg.VisionModelProviders != nil {
		for k, v := range cfg.VisionModelProviders {
			if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
				continue
			}
			modelProviders[k] = v
		}
	}
	if len(modelProviders) == 0 {
		modelProviders[defaultModel] = provider
	}
	// Ensure placeholder route exists.
	if _, ok := modelProviders["placeholder"]; !ok {
		modelProviders["placeholder"] = "placeholder"
	}
	if _, ok := modelProviders[defaultModel]; !ok {
		modelProviders[defaultModel] = provider
	}

	svc.ModelProviders = modelProviders
	svc.DefaultProvider = provider
	svc.DefaultModel = defaultModel

	// Optional retry knob (best-effort): vision_config.max_attempts
	// Default is 1 (no retry).
	maxAttempts := 0
	pngRecompressThresholdBytesSet := false
	pngRecompressThresholdBytes := 0
	if cfg.VisionConfig != nil {
		if v := strings.TrimSpace(cfg.VisionConfig["max_attempts"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				maxAttempts = n
			}
		}
		if v := strings.TrimSpace(cfg.VisionConfig["png_recompress_threshold_bytes"]); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				pngRecompressThresholdBytesSet = true
				pngRecompressThresholdBytes = n
			}
		}
	}
	if maxAttempts > 0 {
		svc.MaxAttempts = maxAttempts
	}
	if pngRecompressThresholdBytesSet {
		if pngRecompressThresholdBytes <= 0 {
			// Explicitly disable PNG recompression.
			svc.PNGRecompressThresholdBytes = -1
		} else {
			svc.PNGRecompressThresholdBytes = pngRecompressThresholdBytes
		}
	}

	return nil
}
