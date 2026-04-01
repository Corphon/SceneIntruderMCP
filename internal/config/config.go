// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/utils"
	"github.com/joho/godotenv"
)

// 当前配置的单例实例
var (
	currentConfig *AppConfig
	configMutex   sync.RWMutex
	configFile    string
	encryptionKey string // Encryption key for API keys
	// useEncryption is determined each time it's needed from environment variable
)

// AppConfig 包含应用程序的所有配置
type AppConfig struct {
	// 基础配置
	Port         string `json:"port"`
	OpenAIAPIKey string `json:"-"` // Don't serialize to JSON to avoid plain text storage
	DataDir      string `json:"data_dir"`
	StaticDir    string `json:"static_dir"`
	TemplatesDir string `json:"templates_dir"`
	LogDir       string `json:"log_dir"`
	DebugMode    bool   `json:"debug_mode"`

	// LLM相关配置
	LLMProvider string            `json:"llm_provider"`
	LLMConfig   map[string]string `json:"llm_config"`

	// Encrypted API key storage (stored as encrypted string)
	EncryptedLLMConfig map[string]string `json:"encrypted_llm_config,omitempty"`

	// Encrypted Vision API key storage (stored as encrypted string)
	EncryptedVisionConfig map[string]string `json:"encrypted_vision_config,omitempty"`
	EncryptedVideoConfig  map[string]string `json:"encrypted_video_config,omitempty"`

	// Vision 相关配置（Phase5）
	VisionProvider       string            `json:"vision_provider,omitempty"`
	VisionDefaultModel   string            `json:"vision_default_model,omitempty"`
	VisionConfig         map[string]string `json:"vision_config,omitempty"`
	VisionModelProviders map[string]string `json:"vision_model_providers,omitempty"`
	VisionModels         []VisionModelInfo `json:"vision_models,omitempty"`

	// Video 相关配置（v2.1）
	VideoProvider       string            `json:"video_provider,omitempty"`
	VideoDefaultModel   string            `json:"video_default_model,omitempty"`
	VideoConfig         map[string]string `json:"video_config,omitempty"`
	VideoModelProviders map[string]string `json:"video_model_providers,omitempty"`
	VideoModels         []VideoModelInfo  `json:"video_models,omitempty"`
}

// VisionModelInfo describes one vision model option that can be surfaced to the frontend.
// Key is the stable identifier used in prompts.model.
type VisionModelInfo struct {
	Key                    string `json:"key"`
	Label                  string `json:"label"`
	Provider               string `json:"provider"`
	SupportsReferenceImage bool   `json:"supports_reference_image"`
}

type VideoModelInfo struct {
	Key                      string `json:"key"`
	Label                    string `json:"label"`
	Provider                 string `json:"provider"`
	SupportsImageConditioned bool   `json:"supports_image_conditioned"`
}

// Config 存储应用配置
type Config struct {
	Port         string
	OpenAIAPIKey string
	DataDir      string
	StaticDir    string
	TemplatesDir string
	LogDir       string
	DebugMode    bool
}

// generateEncryptionKey generates a secure encryption key
// Only displays warning once during initialization
var encryptionKeyWarningShown = false

func generateEncryptionKey() string {
	key := getEnv("CONFIG_ENCRYPTION_KEY", "")
	if key == "" {
		// Check if encryption should be disabled for testing
		if getEnv("DISABLE_CONFIG_ENCRYPTION", "false") == "true" {
			return "" // Return empty key when encryption is disabled
		}

		// Only show warning once
		if !encryptionKeyWarningShown {
			// In production, this should be a fatal error rather than using a default key
			utils.GetLogger().Warn("未设置 CONFIG_ENCRYPTION_KEY 环境变量", nil)
			utils.GetLogger().Info("建议在生产环境中设置安全的32字符加密密钥", nil)
			encryptionKeyWarningShown = true
		}

		// For development only, we'll use or generate a persistent key
		if getEnv("DEBUG_MODE", "true") == "true" {
			// Try to load existing key from file, or generate a new one
			persistentKey, err := loadOrGeneratePersistentKey()
			if err != nil {
				utils.GetLogger().Warn("无法加载或生成持久化密钥", map[string]interface{}{"err": err})
				// Fallback to a more secure derived key if persistent key fails
				derivedKey := fmt.Sprintf("%-32s", fmt.Sprintf("dev_key_%d", time.Now().UnixNano()))[:32]
				utils.GetLogger().Warn("使用基于时间的开发密钥，不建议用于生产环境", nil)
				return derivedKey
			}
			utils.GetLogger().Info("为开发环境生成了安全的随机加密密钥", nil)
			return persistentKey
		} else {
			utils.GetLogger().Fatal("生产环境中必须设置 CONFIG_ENCRYPTION_KEY 环境变量", nil)
		}
	}

	// Validate key length
	if len(key) < 32 {
		utils.GetLogger().Fatal("加密密钥长度不足，请使用至少32字符的密钥", map[string]interface{}{"key_len": len(key)})
	}

	return key
}

// isEncryptionEnabled returns whether encryption should be used based on environment settings
func isEncryptionEnabled() bool {
	return getEnv("DISABLE_CONFIG_ENCRYPTION", "false") != "true"
}

// loadOrGeneratePersistentKey loads an existing encryption key from file or generates a new one
func loadOrGeneratePersistentKey() (string, error) {
	dataDir := getEnv("DATA_DIR", "data")
	keyFile := filepath.Join(dataDir, ".encryption_key")

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	// Try to load existing key
	if keyData, err := os.ReadFile(keyFile); err == nil {
		key := string(keyData)
		if len(key) >= 32 {
			return key, nil
		}
		utils.GetLogger().Warn("现有加密密钥长度不足，将生成新密钥", map[string]interface{}{"key_len": len(key)})
	}

	// Generate a new secure key
	randomKey, err := utils.GenerateSecureKey(32) // 32 bytes = 256 bits
	if err != nil {
		return "", fmt.Errorf("failed to generate secure key: %w", err)
	}

	// Save the key to file for future use (with restricted permissions)
	if err := os.WriteFile(keyFile, randomKey, 0600); err != nil {
		return "", fmt.Errorf("failed to save encryption key: %w", err)
	}

	return string(randomKey), nil
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	// 尝试加载.env文件（可选）
	godotenv.Load()

	// Initialize encryption key
	encryptionKey = generateEncryptionKey()

	defaultStaticDir := filepath.Join("frontend", "dist", "assets")
	defaultTemplatesDir := filepath.Join("frontend", "dist")

	// 创建配置
	config := &Config{
		Port:         getEnv("PORT", "8080"),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		DataDir:      getEnvPath("DATA_DIR", "data"),
		StaticDir:    getEnv("STATIC_DIR", defaultStaticDir),
		TemplatesDir: getEnv("TEMPLATES_DIR", defaultTemplatesDir),
		LogDir:       getEnvPath("LOG_DIR", "logs"),
		DebugMode:    getEnvBool("DEBUG_MODE", true),
	}

	// 验证OpenAI API密钥 (这是可选的，可以通过设置页面配置)
	if config.OpenAIAPIKey == "" {
		// 只记录提示信息，不是警告 - 因为用户可以通过页面配置
		utils.GetLogger().Info("可通过设置页面配置LLM API密钥以使用AI功能", nil)
	}

	return config, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvPath 获取环境变量表示的路径，如果不存在则返回默认值
func getEnvPath(key, defaultValue string) string {
	path := getEnv(key, defaultValue)

	// 确保目录存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			utils.GetLogger().Warn("创建目录失败", map[string]interface{}{"path": path, "err": err})
		}
	}

	return path
}

// getEnvBool 获取布尔类型环境变量
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value == "true" || value == "1" || value == "yes"
}

// encryptAPIKey encrypts an API key
func encryptAPIKey(plaintext string) (string, error) {
	if !isEncryptionEnabled() {
		// If encryption is disabled, return the plaintext directly
		return plaintext, nil
	}

	if encryptionKey == "" {
		return "", fmt.Errorf("encryption key not initialized")
	}
	return utils.Encrypt(plaintext, encryptionKey)
}

// decryptAPIKey decrypts an API key
func decryptAPIKey(ciphertext string) (string, error) {
	if !isEncryptionEnabled() {
		// If encryption is disabled, return the ciphertext directly as it was stored as plaintext
		return ciphertext, nil
	}

	if encryptionKey == "" {
		return "", fmt.Errorf("encryption key not initialized")
	}
	return utils.Decrypt(ciphertext, encryptionKey)
}

// getDecryptedAPIKey gets the decrypted API key from LLMConfig
func (c *AppConfig) getDecryptedAPIKey() string {
	if !isEncryptionEnabled() {
		// If encryption is disabled, API key is stored directly in LLMConfig
		if c.LLMConfig != nil {
			return c.LLMConfig["api_key"]
		}
		return ""
	}

	if c.EncryptedLLMConfig != nil {
		encryptedKey, exists := c.EncryptedLLMConfig["api_key"]
		if exists && encryptedKey != "" {
			decryptedKey, err := decryptAPIKey(encryptedKey)
			if err == nil {
				return decryptedKey
			}
			// Decryption failed - likely due to changed encryption key
			// Clear the invalid encrypted key and fall back to unencrypted config
			utils.GetLogger().Warn("无法解密已保存的API密钥（可能是加密密钥已变更）", nil)
			utils.GetLogger().Info("请在设置页面重新配置API密钥", nil)
			delete(c.EncryptedLLMConfig, "api_key")
		}
	}
	// For backward compatibility, check the unencrypted config
	if c.LLMConfig != nil {
		return c.LLMConfig["api_key"]
	}
	return ""
}

// setEncryptedAPIKey sets the encrypted API key in LLMConfig
func (c *AppConfig) setEncryptedAPIKey(apiKey string) error {
	if !isEncryptionEnabled() {
		// If encryption is disabled, store API key directly in LLMConfig
		if c.LLMConfig == nil {
			c.LLMConfig = make(map[string]string)
		}
		c.LLMConfig["api_key"] = apiKey
		return nil
	}

	if c.EncryptedLLMConfig == nil {
		c.EncryptedLLMConfig = make(map[string]string)
	}

	encryptedKey, err := encryptAPIKey(apiKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %w", err)
	}
	c.EncryptedLLMConfig["api_key"] = encryptedKey
	return nil
}

// getDecryptedVisionAPIKey gets the decrypted API key from VisionConfig.
func (c *AppConfig) getDecryptedVisionAPIKey() string {
	if !isEncryptionEnabled() {
		if c.VisionConfig != nil {
			return c.VisionConfig["api_key"]
		}
		return ""
	}

	if c.EncryptedVisionConfig != nil {
		encryptedKey := c.EncryptedVisionConfig["api_key"]
		if encryptedKey != "" {
			decryptedKey, err := decryptAPIKey(encryptedKey)
			if err == nil {
				return decryptedKey
			}
			utils.GetLogger().Warn("无法解密已保存的Vision API密钥（可能是加密密钥已变更）", nil)
			utils.GetLogger().Info("请在设置页面重新配置Vision API密钥", nil)
			delete(c.EncryptedVisionConfig, "api_key")
		}
	}

	// Backward compatibility: check unencrypted config
	if c.VisionConfig != nil {
		return c.VisionConfig["api_key"]
	}
	return ""
}

// setEncryptedVisionAPIKey stores the vision api_key either encrypted (default) or plaintext when encryption is disabled.
func (c *AppConfig) setEncryptedVisionAPIKey(apiKey string) error {
	if !isEncryptionEnabled() {
		if c.VisionConfig == nil {
			c.VisionConfig = make(map[string]string)
		}
		c.VisionConfig["api_key"] = apiKey
		return nil
	}

	if c.EncryptedVisionConfig == nil {
		c.EncryptedVisionConfig = make(map[string]string)
	}

	encryptedKey, err := encryptAPIKey(apiKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt Vision API key: %w", err)
	}
	c.EncryptedVisionConfig["api_key"] = encryptedKey
	return nil
}

func (c *AppConfig) getDecryptedVideoAPIKey() string {
	if !isEncryptionEnabled() {
		if c.VideoConfig != nil {
			return c.VideoConfig["api_key"]
		}
		return ""
	}

	if c.EncryptedVideoConfig != nil {
		encryptedKey := c.EncryptedVideoConfig["api_key"]
		if encryptedKey != "" {
			decryptedKey, err := decryptAPIKey(encryptedKey)
			if err == nil {
				return decryptedKey
			}
			utils.GetLogger().Warn("无法解密已保存的Video API密钥（可能是加密密钥已变更）", nil)
			utils.GetLogger().Info("请在设置页面重新配置Video API密钥", nil)
			delete(c.EncryptedVideoConfig, "api_key")
		}
	}

	if c.VideoConfig != nil {
		return c.VideoConfig["api_key"]
	}
	return ""
}

func (c *AppConfig) setEncryptedVideoAPIKey(apiKey string) error {
	if !isEncryptionEnabled() {
		if c.VideoConfig == nil {
			c.VideoConfig = make(map[string]string)
		}
		c.VideoConfig["api_key"] = apiKey
		return nil
	}

	if c.EncryptedVideoConfig == nil {
		c.EncryptedVideoConfig = make(map[string]string)
	}

	encryptedKey, err := encryptAPIKey(apiKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt Video API key: %w", err)
	}
	c.EncryptedVideoConfig["api_key"] = encryptedKey
	return nil
}

func (c *AppConfig) getVideoConfig() map[string]string {
	out := make(map[string]string)
	if c.VideoConfig != nil {
		for k, v := range c.VideoConfig {
			if k == "api_key" {
				continue
			}
			out[k] = v
		}
	}
	if v := c.getDecryptedVideoAPIKey(); v != "" {
		out["api_key"] = v
	}
	return out
}

// getVisionConfig returns the current vision config with decrypted api_key injected.
func (c *AppConfig) getVisionConfig() map[string]string {
	out := make(map[string]string)
	if c.VisionConfig != nil {
		for k, v := range c.VisionConfig {
			if k == "api_key" {
				continue
			}
			out[k] = v
		}
	}
	if v := c.getDecryptedVisionAPIKey(); v != "" {
		out["api_key"] = v
	}
	return out
}

// getLLMConfig returns the current LLM config with decrypted API key
func (c *AppConfig) getLLMConfig() map[string]string {
	config := make(map[string]string)

	// Copy non-sensitive fields from LLMConfig
	if c.LLMConfig != nil {
		for k, v := range c.LLMConfig {
			if k != "api_key" { // Don't copy the api_key from unencrypted config (to avoid duplication with decrypted)
				config[k] = v
			}
		}
	}

	// Add decrypted API key
	decryptedAPIKey := c.getDecryptedAPIKey()
	if decryptedAPIKey != "" {
		config["api_key"] = decryptedAPIKey
	}

	return config
}

// InitConfig 初始化配置管理器
func InitConfig(dataDir string) error {
	configFile = filepath.Join(dataDir, "config.json")

	// 加载基础配置
	baseConfig, err := Load()
	if err != nil {
		return err
	}

	// 创建初始配置
	configMutex.Lock()
	defer configMutex.Unlock()

	currentConfig = &AppConfig{
		Port:                  baseConfig.Port,
		OpenAIAPIKey:          baseConfig.OpenAIAPIKey,
		DataDir:               baseConfig.DataDir,
		StaticDir:             baseConfig.StaticDir,
		TemplatesDir:          baseConfig.TemplatesDir,
		LogDir:                baseConfig.LogDir,
		DebugMode:             baseConfig.DebugMode,
		LLMProvider:           "",                      // No default provider
		LLMConfig:             make(map[string]string), // Empty config initially
		EncryptedLLMConfig:    make(map[string]string),
		EncryptedVisionConfig: make(map[string]string),
		EncryptedVideoConfig:  make(map[string]string),
		VisionProvider:        "placeholder",
		VisionDefaultModel:    "placeholder",
		VisionConfig:          make(map[string]string),
		VisionModelProviders: map[string]string{
			"placeholder": "placeholder",
		},
		VisionModels: []VisionModelInfo{
			{Key: "qwen-image-max", Label: "qwen-image-max", Provider: "", SupportsReferenceImage: false},
			{Key: "qwen-image-2.0", Label: "qwen-image-2.0", Provider: "", SupportsReferenceImage: false},
			{Key: "nano-banana-pro", Label: "nano-banana-pro", Provider: "", SupportsReferenceImage: false},
			{Key: "nano-banana", Label: "nano-banana", Provider: "", SupportsReferenceImage: false},
			{Key: "flux", Label: "flux", Provider: "", SupportsReferenceImage: false},
			{Key: "doubao-seedream-4.5", Label: "doubao-seedream-4.5", Provider: "", SupportsReferenceImage: false},
			{Key: "gpt-image-1.5", Label: "gpt-image-1.5", Provider: "", SupportsReferenceImage: false},
			{Key: "glm-image", Label: "glm-image", Provider: "", SupportsReferenceImage: false},
			{Key: "dalle3", Label: "dalle3", Provider: "", SupportsReferenceImage: false},
			{Key: "sd", Label: "sd", Provider: "", SupportsReferenceImage: true},
			{Key: "midjourney", Label: "midjourney", Provider: "", SupportsReferenceImage: false},
			{Key: "placeholder", Label: "Placeholder", Provider: "placeholder", SupportsReferenceImage: false},
		},
		VideoProvider:     "dashscope",
		VideoDefaultModel: "wan2.6-i2v-flash",
		VideoConfig:       make(map[string]string),
		VideoModelProviders: map[string]string{
			"wan2.6-i2v-flash":        "dashscope",
			"kling-v3":                "kling",
			"veo-2":                   "google",
			"veo-2-vertex":            "vertex",
			"doubao-seedance-1-5-pro": "ark",
		},
		VideoModels: []VideoModelInfo{
			{Key: "wan2.6-i2v-flash", Label: "wan2.6-i2v-flash", Provider: "dashscope", SupportsImageConditioned: true},
			{Key: "kling-v3", Label: "kling-v3", Provider: "kling", SupportsImageConditioned: true},
			{Key: "veo-2", Label: "veo-2", Provider: "google", SupportsImageConditioned: true},
			{Key: "veo-2-vertex", Label: "veo-2-vertex", Provider: "vertex", SupportsImageConditioned: true},
			{Key: "doubao-seedance-1-5-pro", Label: "doubao-seedance-1-5-pro", Provider: "ark", SupportsImageConditioned: true},
		},
	}

	// Set encrypted API key from base config if available
	if baseConfig.OpenAIAPIKey != "" {
		err := currentConfig.setEncryptedAPIKey(baseConfig.OpenAIAPIKey)
		if err != nil {
			utils.GetLogger().Warn("无法加密环境变量中的API密钥", map[string]interface{}{"err": err})
		}
	}

	// 尝试从文件加载已保存的配置
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		data, err := os.ReadFile(configFile)
		if err == nil {
			var savedConfig AppConfig
			if json.Unmarshal(data, &savedConfig) == nil {
				// Check if the config file is just a template (empty or default values)
				// Only load the config if it has meaningful values.
				hasMeaningfulValues := false

				// Check if there's an actual provider that's not just the default
				if savedConfig.LLMProvider != "" && savedConfig.LLMProvider != "openai" {
					hasMeaningfulValues = true
				}

				// Check if there's a custom model configured (not just the default)
				if savedConfig.LLMConfig != nil &&
					(savedConfig.LLMConfig["default_model"] != "" && savedConfig.LLMConfig["default_model"] != "gpt-4o") {
					hasMeaningfulValues = true
				}

				// Check if there's an encrypted API key
				if savedConfig.EncryptedLLMConfig != nil && savedConfig.EncryptedLLMConfig["api_key"] != "" {
					hasMeaningfulValues = true
				}

				// Check if there's an unencrypted API key (for backward compatibility or if user has manually added it)
				if savedConfig.LLMConfig != nil && savedConfig.LLMConfig["api_key"] != "" {
					hasMeaningfulValues = true
				}

				// Check if base_url is set (indicating custom configuration)
				if savedConfig.LLMConfig != nil && savedConfig.LLMConfig["base_url"] != "" {
					hasMeaningfulValues = true
				}

				// Vision meaningful checks (Phase5): allow vision-only config to be loaded even when LLM is untouched.
				if savedConfig.VisionProvider != "" && savedConfig.VisionProvider != "placeholder" {
					hasMeaningfulValues = true
				}
				if savedConfig.VisionDefaultModel != "" && savedConfig.VisionDefaultModel != "placeholder" {
					hasMeaningfulValues = true
				}
				if savedConfig.VisionConfig != nil {
					if v := savedConfig.VisionConfig["endpoint"]; v != "" {
						hasMeaningfulValues = true
					}
					if v := savedConfig.VisionConfig["base_url"]; v != "" {
						hasMeaningfulValues = true
					}
					if v := savedConfig.VisionConfig["api_key"]; v != "" {
						hasMeaningfulValues = true
					}
				}
				if savedConfig.EncryptedVisionConfig != nil && savedConfig.EncryptedVisionConfig["api_key"] != "" {
					hasMeaningfulValues = true
				}
				if len(savedConfig.VisionModelProviders) > 0 {
					hasMeaningfulValues = true
				}
				if len(savedConfig.VisionModels) > 0 {
					// Treat any explicit models list as meaningful.
					hasMeaningfulValues = true
				}

				if savedConfig.VideoProvider != "" && savedConfig.VideoProvider != "dashscope" {
					hasMeaningfulValues = true
				}
				if savedConfig.VideoDefaultModel != "" && savedConfig.VideoDefaultModel != "wan2.6-i2v-flash" {
					hasMeaningfulValues = true
				}
				if savedConfig.VideoConfig != nil {
					if v := savedConfig.VideoConfig["endpoint"]; v != "" {
						hasMeaningfulValues = true
					}
					if v := savedConfig.VideoConfig["base_url"]; v != "" {
						hasMeaningfulValues = true
					}
					if v := savedConfig.VideoConfig["api_key"]; v != "" {
						hasMeaningfulValues = true
					}
				}
				if savedConfig.EncryptedVideoConfig != nil && savedConfig.EncryptedVideoConfig["api_key"] != "" {
					hasMeaningfulValues = true
				}
				if len(savedConfig.VideoModelProviders) > 0 {
					hasMeaningfulValues = true
				}
				if len(savedConfig.VideoModels) > 0 {
					hasMeaningfulValues = true
				}

				if hasMeaningfulValues {
					// 合并配置，保留文件中的LLM设置，但使用最新的基础配置
					savedConfig.Port = baseConfig.Port
					savedConfig.DataDir = baseConfig.DataDir
					savedConfig.StaticDir = baseConfig.StaticDir
					savedConfig.TemplatesDir = baseConfig.TemplatesDir
					savedConfig.LogDir = baseConfig.LogDir
					savedConfig.DebugMode = baseConfig.DebugMode

					// Handle backward compatibility with unencrypted API keys in old configs
					if savedConfig.LLMConfig != nil {
						// If there's an unencrypted API key in the old config, handle based on encryption setting
						if apiKey := savedConfig.LLMConfig["api_key"]; apiKey != "" {
							if isEncryptionEnabled() {
								// If encryption is now enabled, encrypt the existing API key
								err := savedConfig.setEncryptedAPIKey(apiKey)
								if err != nil {
									utils.GetLogger().Warn("无法加密旧配置中的API密钥", map[string]interface{}{"err": err})
									utils.GetLogger().Info("建议通过设置页面重新配置API密钥", nil)
								} else {
									utils.GetLogger().Info("已自动将旧配置中的API密钥升级为加密存储", nil)
								}
							} else {
								// If encryption is disabled, just keep it in the unencrypted config
								utils.GetLogger().Info("配置: 加密已禁用，API密钥将以明文形式存储", nil)
							}
							// Remove api_key from the unencrypted config to avoid duplication
							// This will be handled by setEncryptedAPIKey if encryption is used
							// or will remain in unencrypted config if not used
							delete(savedConfig.LLMConfig, "api_key")
						}
					}

					// Ensure vision defaults when loading older configs.
					if savedConfig.VisionProvider == "" {
						savedConfig.VisionProvider = "placeholder"
					}
					if savedConfig.VisionDefaultModel == "" {
						savedConfig.VisionDefaultModel = "placeholder"
					}
					if savedConfig.VisionConfig == nil {
						savedConfig.VisionConfig = make(map[string]string)
					}
					if savedConfig.EncryptedVisionConfig == nil {
						savedConfig.EncryptedVisionConfig = make(map[string]string)
					}
					if savedConfig.VisionModelProviders == nil {
						savedConfig.VisionModelProviders = make(map[string]string)
					}
					if savedConfig.VisionModels == nil {
						savedConfig.VisionModels = []VisionModelInfo{}
					}
					if savedConfig.VideoProvider == "" {
						savedConfig.VideoProvider = "dashscope"
					}
					if savedConfig.VideoDefaultModel == "" {
						savedConfig.VideoDefaultModel = "wan2.6-i2v-flash"
					}
					if savedConfig.VideoConfig == nil {
						savedConfig.VideoConfig = make(map[string]string)
					}
					if savedConfig.EncryptedVideoConfig == nil {
						savedConfig.EncryptedVideoConfig = make(map[string]string)
					}
					if savedConfig.VideoModelProviders == nil {
						savedConfig.VideoModelProviders = make(map[string]string)
					}
					if savedConfig.VideoModels == nil {
						savedConfig.VideoModels = []VideoModelInfo{}
					}

					// Handle backward compatibility with unencrypted Vision API keys in old configs.
					if savedConfig.VisionConfig != nil {
						if apiKey := savedConfig.VisionConfig["api_key"]; apiKey != "" {
							if isEncryptionEnabled() {
								if err := savedConfig.setEncryptedVisionAPIKey(apiKey); err != nil {
									utils.GetLogger().Warn("无法加密旧配置中的Vision API密钥", map[string]interface{}{"err": err})
									utils.GetLogger().Info("建议通过设置页面重新配置Vision API密钥", nil)
								} else {
									utils.GetLogger().Info("已自动将旧配置中的Vision API密钥升级为加密存储", nil)
								}
							} else {
								utils.GetLogger().Info("配置: 加密已禁用，Vision API密钥将以明文形式存储", nil)
							}
							delete(savedConfig.VisionConfig, "api_key")
						}
					}

					if savedConfig.VideoConfig != nil {
						if apiKey := savedConfig.VideoConfig["api_key"]; apiKey != "" {
							if isEncryptionEnabled() {
								if err := savedConfig.setEncryptedVideoAPIKey(apiKey); err != nil {
									utils.GetLogger().Warn("无法加密旧配置中的Video API密钥", map[string]interface{}{"err": err})
									utils.GetLogger().Info("建议通过设置页面重新配置Video API密钥", nil)
								} else {
									utils.GetLogger().Info("已自动将旧配置中的Video API密钥升级为加密存储", nil)
								}
							} else {
								utils.GetLogger().Info("配置: 加密已禁用，Video API密钥将以明文形式存储", nil)
							}
							delete(savedConfig.VideoConfig, "api_key")
						}
					}

					// If config file doesn't provide an API key, fall back to environment variable key.
					// This preserves the old behavior where empty keys are filled from OPENAI_API_KEY.
					if baseConfig.OpenAIAPIKey != "" && savedConfig.getDecryptedAPIKey() == "" {
						if err := savedConfig.setEncryptedAPIKey(baseConfig.OpenAIAPIKey); err != nil {
							utils.GetLogger().Warn("无法使用环境变量中的API密钥填充配置", map[string]interface{}{"err": err})
						}
					}

					currentConfig = &savedConfig
				} else {
					// The config file exists but only contains template/default values, don't load it
					utils.GetLogger().Info("配置文件仅包含模板值，使用默认配置而不加载文件", nil)
				}
			}
		}
	}

	// 保存初始配置到文件，仅当当前配置与默认配置不同时（即用户已保存有效配置）
	return SaveConfig()
}

// GetCurrentConfig 返回当前配置的副本
func GetCurrentConfig() *AppConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if currentConfig == nil {
		// 紧急情况，返回一个基本配置
		baseConfig, _ := Load()
		appConfig := &AppConfig{
			Port:                  baseConfig.Port,
			OpenAIAPIKey:          baseConfig.OpenAIAPIKey,
			DataDir:               baseConfig.DataDir,
			StaticDir:             baseConfig.StaticDir,
			TemplatesDir:          baseConfig.TemplatesDir,
			LogDir:                baseConfig.LogDir,
			DebugMode:             baseConfig.DebugMode,
			LLMProvider:           "",
			LLMConfig:             make(map[string]string),
			EncryptedLLMConfig:    make(map[string]string),
			EncryptedVisionConfig: make(map[string]string),
			EncryptedVideoConfig:  make(map[string]string),
			VisionProvider:        "placeholder",
			VisionDefaultModel:    "placeholder",
			VisionConfig:          make(map[string]string),
			VisionModelProviders: map[string]string{
				"placeholder": "placeholder",
			},
			VisionModels: []VisionModelInfo{
				{Key: "qwen-image-max", Label: "qwen-image-max", Provider: "", SupportsReferenceImage: false},
				{Key: "qwen-image-2.0", Label: "qwen-image-2.0", Provider: "", SupportsReferenceImage: false},
				{Key: "nano-banana-pro", Label: "nano-banana-pro", Provider: "", SupportsReferenceImage: false},
				{Key: "nano-banana", Label: "nano-banana", Provider: "", SupportsReferenceImage: false},
				{Key: "flux", Label: "flux", Provider: "", SupportsReferenceImage: false},
				{Key: "doubao-seedream-4.5", Label: "doubao-seedream-4.5", Provider: "", SupportsReferenceImage: false},
				{Key: "gpt-image-1.5", Label: "gpt-image-1.5", Provider: "", SupportsReferenceImage: false},
				{Key: "glm-image", Label: "glm-image", Provider: "", SupportsReferenceImage: false},
				{Key: "dalle3", Label: "dalle3", Provider: "", SupportsReferenceImage: false},
				{Key: "sd", Label: "sd", Provider: "", SupportsReferenceImage: true},
				{Key: "midjourney", Label: "midjourney", Provider: "", SupportsReferenceImage: false},
				{Key: "placeholder", Label: "Placeholder", Provider: "placeholder", SupportsReferenceImage: false},
			},
			VideoProvider:     "dashscope",
			VideoDefaultModel: "wan2.6-i2v-flash",
			VideoConfig:       make(map[string]string),
			VideoModelProviders: map[string]string{
				"wan2.6-i2v-flash":        "dashscope",
				"kling-v3":                "kling",
				"veo-2":                   "google",
				"veo-2-vertex":            "vertex",
				"doubao-seedance-1-5-pro": "ark",
			},
			VideoModels: []VideoModelInfo{
				{Key: "wan2.6-i2v-flash", Label: "wan2.6-i2v-flash", Provider: "dashscope", SupportsImageConditioned: true},
				{Key: "kling-v3", Label: "kling-v3", Provider: "kling", SupportsImageConditioned: true},
				{Key: "veo-2", Label: "veo-2", Provider: "google", SupportsImageConditioned: true},
				{Key: "veo-2-vertex", Label: "veo-2-vertex", Provider: "vertex", SupportsImageConditioned: true},
				{Key: "doubao-seedance-1-5-pro", Label: "doubao-seedance-1-5-pro", Provider: "ark", SupportsImageConditioned: true},
			},
		}

		// Set encrypted API key if available
		if baseConfig.OpenAIAPIKey != "" {
			appConfig.setEncryptedAPIKey(baseConfig.OpenAIAPIKey)
		}

		return appConfig
	}

	// 返回配置的副本 with decrypted values where needed
	configCopy := *currentConfig
	// Return a copy with decrypted LLM config
	configCopy.LLMConfig = currentConfig.getLLMConfig()
	// Return a copy with decrypted Vision config.
	configCopy.VisionConfig = currentConfig.getVisionConfig()
	configCopy.VideoConfig = currentConfig.getVideoConfig()
	if currentConfig.VisionModelProviders != nil {
		configCopy.VisionModelProviders = make(map[string]string, len(currentConfig.VisionModelProviders))
		for k, v := range currentConfig.VisionModelProviders {
			configCopy.VisionModelProviders[k] = v
		}
	}
	if currentConfig.VisionModels != nil {
		configCopy.VisionModels = make([]VisionModelInfo, len(currentConfig.VisionModels))
		copy(configCopy.VisionModels, currentConfig.VisionModels)
	}
	if currentConfig.VideoModelProviders != nil {
		configCopy.VideoModelProviders = make(map[string]string, len(currentConfig.VideoModelProviders))
		for k, v := range currentConfig.VideoModelProviders {
			configCopy.VideoModelProviders[k] = v
		}
	}
	if currentConfig.VideoModels != nil {
		configCopy.VideoModels = make([]VideoModelInfo, len(currentConfig.VideoModels))
		copy(configCopy.VideoModels, currentConfig.VideoModels)
	}
	return &configCopy
}

// validateVisionProvider validates a supported vision provider.
func validateVisionProvider(provider string) error {
	supported := []string{"placeholder", "sdwebui", "dashscope", "gemini", "ark", "openai", "glm"}
	if slices.Contains(supported, provider) {
		return nil
	}
	return fmt.Errorf("不支持的vision提供商: %s", provider)
}

func validateVisionConfig(provider string, cfg map[string]string) error {
	switch provider {
	case "placeholder":
		return nil
	case "sdwebui":
		endpoint := ""
		if cfg != nil {
			endpoint = cfg["endpoint"]
			if endpoint == "" {
				endpoint = cfg["base_url"]
			}
		}
		if endpoint == "" {
			return fmt.Errorf("sdwebui 需要 endpoint")
		}
		return nil
	case "dashscope", "gemini", "ark", "openai", "glm":
		endpoint := ""
		if cfg != nil {
			endpoint = cfg["endpoint"]
			if endpoint == "" {
				endpoint = cfg["base_url"]
			}
		}
		if endpoint == "" {
			return fmt.Errorf("%s 需要 endpoint", provider)
		}
		// api_key can be provided via env fallback.
		return nil
	default:
		return fmt.Errorf("不支持的vision提供商: %s", provider)
	}
}

func validateVideoProvider(provider string) error {
	supported := []string{"mock", "dashscope", "kling", "google", "vertex", "ark"}
	if slices.Contains(supported, provider) {
		return nil
	}
	return fmt.Errorf("不支持的video提供商: %s", provider)
}

func validateVideoConfig(provider string, cfg map[string]string) error {
	switch provider {
	case "mock":
		return nil
	case "dashscope", "kling", "google", "vertex", "ark":
		endpoint := ""
		if cfg != nil {
			endpoint = cfg["endpoint"]
			if endpoint == "" {
				endpoint = cfg["base_url"]
			}
		}
		if endpoint == "" {
			return fmt.Errorf("%s video 需要 endpoint", provider)
		}
		return nil
	default:
		return fmt.Errorf("不支持的video提供商: %s", provider)
	}
}

// UpdateVisionConfig updates vision-related settings and persists them.
func UpdateVisionConfig(provider string, visionCfg map[string]string, defaultModel string, modelProviders map[string]string, models []VisionModelInfo) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if currentConfig == nil {
		return fmt.Errorf("配置系统未初始化")
	}
	if provider == "" {
		return fmt.Errorf("vision provider 不能为空")
	}
	if err := validateVisionProvider(provider); err != nil {
		return err
	}

	// Copy maps/slices to avoid aliasing.
	newVisionCfg := make(map[string]string)
	for k, v := range visionCfg {
		newVisionCfg[k] = v
	}

	// Reuse existing Vision api_key when not provided and provider stays the same.
	newVisionKey, hasNewKey := newVisionCfg["api_key"]
	if !hasNewKey || newVisionKey == "" {
		if currentConfig.VisionProvider == provider {
			if existing := currentConfig.getDecryptedVisionAPIKey(); existing != "" {
				newVisionCfg["api_key"] = existing
			}
		}
	}
	newModelProviders := make(map[string]string)
	for k, v := range modelProviders {
		newModelProviders[k] = v
	}
	newModels := make([]VisionModelInfo, len(models))
	copy(newModels, models)

	if err := validateVisionConfig(provider, newVisionCfg); err != nil {
		return err
	}

	if defaultModel == "" {
		// Sensible defaults.
		if provider == "sdwebui" {
			defaultModel = "sd"
		} else if provider == "dashscope" {
			defaultModel = "qwen-image-2.0"
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

	// Ensure a default model provider mapping exists for the default model.
	if defaultModel != "" {
		// newModelProviders is always non-nil after initialization above
		if _, ok := newModelProviders[defaultModel]; !ok {
			newModelProviders[defaultModel] = provider
		}
	}

	currentConfig.VisionProvider = provider
	currentConfig.VisionDefaultModel = defaultModel

	// Handle Vision api_key encryption/decryption based on useEncryption setting.
	currentConfig.VisionConfig = make(map[string]string)
	for k, v := range newVisionCfg {
		if k == "api_key" {
			if v == "" {
				continue
			}
			if err := currentConfig.setEncryptedVisionAPIKey(v); err != nil {
				return fmt.Errorf("failed to %s Vision API key: %w",
					map[bool]string{true: "encrypt", false: "store"}[isEncryptionEnabled()], err)
			}
			continue
		}
		currentConfig.VisionConfig[k] = v
	}

	currentConfig.VisionModelProviders = newModelProviders
	currentConfig.VisionModels = newModels

	return SaveConfig()
}

func UpdateVideoConfig(provider string, videoCfg map[string]string, defaultModel string, modelProviders map[string]string, models []VideoModelInfo) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if currentConfig == nil {
		return fmt.Errorf("配置系统未初始化")
	}
	if provider == "" {
		return fmt.Errorf("video provider 不能为空")
	}
	if err := validateVideoProvider(provider); err != nil {
		return err
	}

	newVideoCfg := make(map[string]string)
	for k, v := range videoCfg {
		newVideoCfg[k] = v
	}
	newVideoKey, hasNewKey := newVideoCfg["api_key"]
	if !hasNewKey || newVideoKey == "" {
		if currentConfig.VideoProvider == provider {
			if existing := currentConfig.getDecryptedVideoAPIKey(); existing != "" {
				newVideoCfg["api_key"] = existing
			}
		}
	}
	newModelProviders := make(map[string]string)
	for k, v := range modelProviders {
		newModelProviders[k] = v
	}
	newModels := make([]VideoModelInfo, len(models))
	copy(newModels, models)

	if err := validateVideoConfig(provider, newVideoCfg); err != nil {
		return err
	}
	if defaultModel == "" {
		switch provider {
		case "kling":
			defaultModel = "kling-v3"
		case "google":
			defaultModel = "veo-2"
		case "vertex":
			defaultModel = "veo-2-vertex"
		case "ark":
			defaultModel = "doubao-seedance-1-5-pro"
		default:
			defaultModel = "wan2.6-i2v-flash"
		}
	}
	if defaultModel != "" {
		if _, ok := newModelProviders[defaultModel]; !ok {
			newModelProviders[defaultModel] = provider
		}
	}

	currentConfig.VideoProvider = provider
	currentConfig.VideoDefaultModel = defaultModel
	currentConfig.VideoConfig = make(map[string]string)
	for k, v := range newVideoCfg {
		if k == "api_key" {
			if v == "" {
				continue
			}
			if err := currentConfig.setEncryptedVideoAPIKey(v); err != nil {
				return fmt.Errorf("failed to %s Video API key: %w",
					map[bool]string{true: "encrypt", false: "store"}[isEncryptionEnabled()], err)
			}
			continue
		}
		currentConfig.VideoConfig[k] = v
	}
	currentConfig.VideoModelProviders = newModelProviders
	currentConfig.VideoModels = newModels

	return SaveConfig()
}

// UpdateLLMConfig 更新LLM配置
func UpdateLLMConfig(provider string, config map[string]string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if currentConfig == nil {
		return fmt.Errorf("配置系统未初始化")
	}

	// 创建配置副本以避免修改传入的 map
	newConfig := make(map[string]string)
	for k, v := range config {
		newConfig[k] = v
	}

	// 检查是否提供了新的 API Key
	newAPIKey, hasNewKey := newConfig["api_key"]

	// 如果没有提供新 Key 或为空，检查是否已有 Key
	if !hasNewKey || newAPIKey == "" {
		// 尝试获取现有的解密 Key
		existingKey := currentConfig.getDecryptedAPIKey()
		// 只有当现有 Key 存在且当前 Provider 与请求的 Provider 一致时才复用
		// 如果切换了 Provider，则必须提供新的 Key (除非新 Provider 不需要 Key，但这由 validateLLMConfig 处理)
		if existingKey != "" && currentConfig.LLMProvider == provider {
			newConfig["api_key"] = existingKey
		}
	}

	// provider 验证
	if err := validateLLMProvider(provider); err != nil {
		return err
	}

	// 配置验证
	if err := validateLLMConfig(provider, newConfig); err != nil {
		return err
	}

	currentConfig.LLMProvider = provider

	// Handle API key encryption/decryption based on useEncryption setting
	currentConfig.LLMConfig = make(map[string]string)
	for k, v := range newConfig {
		if k == "api_key" {
			// Encrypt the API key based on encryption setting
			err := currentConfig.setEncryptedAPIKey(v)
			if err != nil {
				return fmt.Errorf("failed to %s API key: %w",
					map[bool]string{true: "encrypt", false: "store"}[isEncryptionEnabled()], err)
			}
		} else {
			currentConfig.LLMConfig[k] = v
		}
	}

	return SaveConfig()
}

// UpdateFullConfig 更新完整的配置
func UpdateFullConfig(provider string, llmConfig map[string]string, encryptedLLMConfig map[string]string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if currentConfig == nil {
		return fmt.Errorf("配置系统未初始化")
	}

	// provider 验证
	if err := validateLLMProvider(provider); err != nil {
		return err
	}

	// 配置验证
	if err := validateLLMConfig(provider, encryptedLLMConfig); err != nil {
		return err
	}

	currentConfig.LLMProvider = provider
	currentConfig.LLMConfig = llmConfig
	currentConfig.EncryptedLLMConfig = encryptedLLMConfig

	return SaveConfig()
}

// validateLLMProvider 验证 LLM 提供商是否受支持
func validateLLMProvider(provider string) error {
	supportedProviders := []string{
		"openai", "anthropic", "google", "githubmodels", "grok",
		"mistral", "qwen", "glm", "deepseek", "openrouter", "nvidia",
	}

	if slices.Contains(supportedProviders, provider) {
		return nil
	}

	return fmt.Errorf("不支持的提供商: %s", provider)
}

// validateLLMConfig 验证 LLM 配置
func validateLLMConfig(provider string, config map[string]string) error {
	// 验证必需的配置项
	apiKey, exists := config["api_key"]
	if !exists {
		return fmt.Errorf("缺少 api_key 配置")
	}

	if apiKey == "" {
		return fmt.Errorf("api_key 不能为空")
	}

	// 特定提供商的验证
	switch provider {
	case "glm":
		if _, ok := config["api_secret"]; !ok {
			return fmt.Errorf("GLM 提供商需要 api_secret")
		}
	case "google":
		// Google 可能需要 project_id
		// 可以添加特定验证
	}

	return nil
}

// SaveConfig 保存当前配置到文件
func SaveConfig() error {
	if currentConfig == nil {
		return fmt.Errorf("没有配置可保存")
	}

	// 确保目录存在
	dir := filepath.Dir(configFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}
	}

	// Create a copy of the config for serialization that excludes the plain API key
	configToSave := *currentConfig

	// Store the decrypted LLM config temporarily to avoid storing plain text API key
	originalLLMConfig := configToSave.LLMConfig
	configToSave.LLMConfig = make(map[string]string)
	for k, v := range originalLLMConfig {
		if k != "api_key" {
			configToSave.LLMConfig[k] = v
		}
	}

	// Extra safety: avoid persisting Vision api_key in plaintext when encryption is enabled.
	if isEncryptionEnabled() && configToSave.VisionConfig != nil {
		delete(configToSave.VisionConfig, "api_key")
	}
	if isEncryptionEnabled() && configToSave.VideoConfig != nil {
		delete(configToSave.VideoConfig, "api_key")
	}

	// 序列化并保存
	data, err := json.MarshalIndent(configToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(configFile, data, 0644)
}
