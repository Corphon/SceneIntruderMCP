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
			log.Println("⚠️ 警告: 未设置 CONFIG_ENCRYPTION_KEY 环境变量")
			log.Println("💡 建议: 在生产环境中设置一个安全的32字符加密密钥")
			encryptionKeyWarningShown = true
		}

		// For development only, we'll use or generate a persistent key
		if getEnv("DEBUG_MODE", "true") == "true" {
			// Try to load existing key from file, or generate a new one
			persistentKey, err := loadOrGeneratePersistentKey()
			if err != nil {
				log.Printf("⚠️ 警告: 无法加载或生成持久化密钥: %v", err)
				// Fallback to a more secure derived key if persistent key fails
				derivedKey := fmt.Sprintf("%-32s", fmt.Sprintf("dev_key_%d", time.Now().UnixNano()))[:32]
				log.Println("⚠️ 警告: 使用基于时间的开发密钥，不建议用于生产环境")
				return derivedKey
			}
			log.Println("✅ 为开发环境生成了安全的随机加密密钥")
			return persistentKey
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
		log.Println("⚠️ 警告: 现有加密密钥长度不足，将生成新密钥")
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
		Port:               baseConfig.Port,
		OpenAIAPIKey:       baseConfig.OpenAIAPIKey,
		DataDir:            baseConfig.DataDir,
		StaticDir:          baseConfig.StaticDir,
		TemplatesDir:       baseConfig.TemplatesDir,
		LogDir:             baseConfig.LogDir,
		DebugMode:          baseConfig.DebugMode,
		LLMProvider:        "",                      // No default provider
		LLMConfig:          make(map[string]string), // Empty config initially
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
				// Check if the config file is just a template (empty or default values)
				// Only load the config if it has meaningful values (not empty provider, not empty default model, has API key)
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
									log.Printf("⚠️ 警告: 无法加密旧配置中的API密钥: %v", err)
									log.Printf("💡 建议: 请通过设置页面重新配置API密钥")
								} else {
									log.Println("✅ 已自动将旧配置中的API密钥升级为加密存储")
								}
							} else {
								// If encryption is disabled, just keep it in the unencrypted config
								log.Println("💡 配置: 加密已禁用，API密钥将以明文形式存储")
							}
							// Remove api_key from the unencrypted config to avoid duplication
							// This will be handled by setEncryptedAPIKey if encryption is used
							// or will remain in unencrypted config if not used
							delete(savedConfig.LLMConfig, "api_key")
						}
					}

					currentConfig = &savedConfig
				} else {
					// The config file exists but only contains template/default values, don't load it
					log.Println("📝 配置文件仅包含模板值，使用默认配置而不加载文件")
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
			Port:               baseConfig.Port,
			OpenAIAPIKey:       baseConfig.OpenAIAPIKey,
			DataDir:            baseConfig.DataDir,
			StaticDir:          baseConfig.StaticDir,
			TemplatesDir:       baseConfig.TemplatesDir,
			LogDir:             baseConfig.LogDir,
			DebugMode:          baseConfig.DebugMode,
			LLMProvider:        "",
			LLMConfig:          make(map[string]string),
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
