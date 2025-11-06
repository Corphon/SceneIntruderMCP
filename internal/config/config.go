// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/Corphon/SceneIntruderMCP/internal/utils"
	"github.com/joho/godotenv"
)

// 当前配置的单例实例
var (
	currentConfig *AppConfig
	configMutex   sync.RWMutex
	configFile    string
	encryptionKey string // Encryption key for API keys
)

// AppConfig 包含应用程序的所有配置
type AppConfig struct {
	// 基础配置
	Port         string            `json:"port"`
	OpenAIAPIKey string            `json:"-"` // Don't serialize to JSON to avoid plain text storage
	DataDir      string            `json:"data_dir"`
	StaticDir    string            `json:"static_dir"`
	TemplatesDir string            `json:"templates_dir"`
	LogDir       string            `json:"log_dir"`
	DebugMode    bool              `json:"debug_mode"`

	// LLM相关配置
	LLMProvider string            `json:"llm_provider"`
	LLMConfig   map[string]string `json:"llm_config"`
	
	// Encrypted API key storage (stored as encrypted string)
	EncryptedLLMConfig map[string]string `json:"encrypted_llm_config,omitempty"`
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
		// Only show warning once
		if !encryptionKeyWarningShown {
			// In production, this should be a fatal error rather than using a default key
			log.Println("⚠️ 警告: 未设置 CONFIG_ENCRYPTION_KEY 环境变量")
			log.Println("💡 建议: 在生产环境中设置一个安全的32字符加密密钥")
			encryptionKeyWarningShown = true
		}

		// For development only, we'll warn and use a default, but in production this should be an error
		if getEnv("DEBUG_MODE", "true") == "true" {
			key = "SceneIntruderMCP_default_encryption_key_32_chars!"
		} else {
			log.Fatal("❌ 生产环境中必须设置 CONFIG_ENCRYPTION_KEY 环境变量")
		}
	}

	// Validate key length
	if len(key) < 32 {
		log.Fatalf("❌ 加密密钥长度不足。请使用至少32字符的密钥")
	}

	return key
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	// 尝试加载.env文件（可选）
	godotenv.Load()

	// Initialize encryption key
	encryptionKey = generateEncryptionKey()

	// 创建配置
	config := &Config{
		Port:         getEnv("PORT", "8080"),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		DataDir:      getEnvPath("DATA_DIR", "data"),
		StaticDir:    getEnvPath("STATIC_DIR", "static"),
		TemplatesDir: getEnvPath("TEMPLATES_DIR", "web/templates"),
		LogDir:       getEnvPath("LOG_DIR", "logs"),
		DebugMode:    getEnvBool("DEBUG_MODE", true),
	}

	// 验证OpenAI API密钥 (这是可选的，可以通过设置页面配置)
	if config.OpenAIAPIKey == "" {
		// 只记录提示信息，不是警告 - 因为用户可以通过页面配置
		log.Println("💡 提示: 可通过设置页面配置LLM API密钥以使用AI功能")
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
			fmt.Printf("警告: 创建目录失败 %s: %v\n", path, err)
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
	if encryptionKey == "" {
		return "", fmt.Errorf("encryption key not initialized")
	}
	return utils.Encrypt(plaintext, encryptionKey)
}

// decryptAPIKey decrypts an API key
func decryptAPIKey(ciphertext string) (string, error) {
	if encryptionKey == "" {
		return "", fmt.Errorf("encryption key not initialized")
	}
	return utils.Decrypt(ciphertext, encryptionKey)
}

// getDecryptedAPIKey gets the decrypted API key from LLMConfig
func (c *AppConfig) getDecryptedAPIKey() string {
	if c.EncryptedLLMConfig != nil {
		encryptedKey, exists := c.EncryptedLLMConfig["api_key"]
		if exists && encryptedKey != "" {
			decryptedKey, err := decryptAPIKey(encryptedKey)
			if err == nil {
				return decryptedKey
			}
			// Decryption failed - likely due to changed encryption key
			// Clear the invalid encrypted key and fall back to unencrypted config
			log.Printf("⚠️ 警告: 无法解密已保存的API密钥(可能是加密密钥已变更)")
			log.Printf("💡 提示: 请在设置页面重新配置API密钥")
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

// getLLMConfig returns the current LLM config with decrypted API key
func (c *AppConfig) getLLMConfig() map[string]string {
	config := make(map[string]string)
	
	// Copy non-sensitive fields from LLMConfig
	if c.LLMConfig != nil {
		for k, v := range c.LLMConfig {
			if k != "api_key" { // Don't copy the api_key from unencrypted config
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
		Port:         baseConfig.Port,
		OpenAIAPIKey: baseConfig.OpenAIAPIKey,
		DataDir:      baseConfig.DataDir,
		StaticDir:    baseConfig.StaticDir,
		TemplatesDir: baseConfig.TemplatesDir,
		LogDir:       baseConfig.LogDir,
		DebugMode:    baseConfig.DebugMode,
		LLMProvider:  "openai", // 默认使用OpenAI
		LLMConfig: map[string]string{
			"default_model": "gpt-4o",
		},
		EncryptedLLMConfig: make(map[string]string),
	}

	// Set encrypted API key from base config if available
	if baseConfig.OpenAIAPIKey != "" {
		err := currentConfig.setEncryptedAPIKey(baseConfig.OpenAIAPIKey)
		if err != nil {
			log.Printf("⚠️ 警告: 无法加密环境变量中的API密钥: %v", err)
		}
	}

	// 尝试从文件加载已保存的配置
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		data, err := os.ReadFile(configFile)
		if err == nil {
			var savedConfig AppConfig
			if json.Unmarshal(data, &savedConfig) == nil {
				// 合并配置，保留文件中的LLM设置，但使用最新的基础配置
				savedConfig.Port = baseConfig.Port
				savedConfig.DataDir = baseConfig.DataDir
				savedConfig.StaticDir = baseConfig.StaticDir
				savedConfig.TemplatesDir = baseConfig.TemplatesDir
				savedConfig.LogDir = baseConfig.LogDir
				savedConfig.DebugMode = baseConfig.DebugMode

				// Handle backward compatibility with unencrypted API keys in old configs
				if savedConfig.LLMConfig != nil {
					// If there's an unencrypted API key in the old config, encrypt it
					if apiKey := savedConfig.LLMConfig["api_key"]; apiKey != "" {
						// Set the encrypted version and clear the unencrypted one
						err := savedConfig.setEncryptedAPIKey(apiKey)
						if err != nil {
							log.Printf("⚠️ 警告: 无法加密旧配置中的API密钥: %v", err)
							log.Printf("💡 建议: 请通过设置页面重新配置API密钥")
						} else {
							log.Println("✅ 已自动将旧配置中的API密钥升级为加密存储")
						}
						// Remove api_key from the unencrypted config
						delete(savedConfig.LLMConfig, "api_key")
					}
				}

				currentConfig = &savedConfig
			}
		}
	}

	// 保存初始配置到文件
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
			Port:         baseConfig.Port,
			OpenAIAPIKey: baseConfig.OpenAIAPIKey,
			DataDir:      baseConfig.DataDir,
			StaticDir:    baseConfig.StaticDir,
			TemplatesDir: baseConfig.TemplatesDir,
			LogDir:       baseConfig.LogDir,
			DebugMode:    baseConfig.DebugMode,
			LLMProvider:  "openai",
			LLMConfig: map[string]string{
				"default_model": "gpt-4o",
			},
			EncryptedLLMConfig: make(map[string]string),
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
	return &configCopy
}

// UpdateLLMConfig 更新LLM配置
func UpdateLLMConfig(provider string, config map[string]string) error {
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
	if err := validateLLMConfig(provider, config); err != nil {
		return err
	}

	currentConfig.LLMProvider = provider
	
	// Handle API key encryption
	currentConfig.LLMConfig = make(map[string]string)
	for k, v := range config {
		if k == "api_key" {
			// Encrypt the API key
			err := currentConfig.setEncryptedAPIKey(v)
			if err != nil {
				return fmt.Errorf("failed to encrypt API key: %w", err)
			}
		} else {
			currentConfig.LLMConfig[k] = v
		}
	}

	return SaveConfig()
}

// validateLLMProvider 验证 LLM 提供商是否受支持
func validateLLMProvider(provider string) error {
	supportedProviders := []string{
		"openai", "anthropic", "google", "githubmodels", "grok",
		"mistral", "qwen", "glm", "deepseek", "openrouter",
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

	// 序列化并保存
	data, err := json.MarshalIndent(configToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(configFile, data, 0644)
}
