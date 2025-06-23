// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
)

// 当前配置的单例实例
var (
	currentConfig *AppConfig
	configMutex   sync.RWMutex
	configFile    string
)

// AppConfig 包含应用程序的所有配置
type AppConfig struct {
	// 基础配置
	Port         string `json:"port"`
	OpenAIAPIKey string `json:"openai_api_key,omitempty"`
	DataDir      string `json:"data_dir"`
	StaticDir    string `json:"static_dir"`
	TemplatesDir string `json:"templates_dir"`
	LogDir       string `json:"log_dir"`
	DebugMode    bool   `json:"debug_mode"`

	// LLM相关配置
	LLMProvider string            `json:"llm_provider"`
	LLMConfig   map[string]string `json:"llm_config"`
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

// Load 从环境变量加载配置
func Load() (*Config, error) {
	// 尝试加载.env文件（可选）
	godotenv.Load()

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

	// 验证OpenAI API密钥
	if config.OpenAIAPIKey == "" {
		// 只记录警告，不返回错误
		log.Println("警告: 未设置OpenAI API密钥，将需要在设置页面中配置才能使用LLM功能")
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
			"api_key":       baseConfig.OpenAIAPIKey,
			"default_model": "gpt-4o",
		},
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

				// 如果文件中没有API密钥，使用环境变量的密钥
				if savedConfig.LLMConfig != nil && savedConfig.LLMConfig["api_key"] == "" {
					savedConfig.LLMConfig["api_key"] = baseConfig.OpenAIAPIKey
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
		return &AppConfig{
			Port:         baseConfig.Port,
			OpenAIAPIKey: baseConfig.OpenAIAPIKey,
			DataDir:      baseConfig.DataDir,
			StaticDir:    baseConfig.StaticDir,
			TemplatesDir: baseConfig.TemplatesDir,
			LogDir:       baseConfig.LogDir,
			DebugMode:    baseConfig.DebugMode,
			LLMProvider:  "openai",
			LLMConfig: map[string]string{
				"api_key": baseConfig.OpenAIAPIKey,
			},
		}
	}

	// 返回配置的副本
	configCopy := *currentConfig
	return &configCopy
}

// UpdateLLMConfig 更新LLM配置
func UpdateLLMConfig(provider string, config map[string]string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if currentConfig == nil {
		return fmt.Errorf("配置系统未初始化")
	}

	currentConfig.LLMProvider = provider
	currentConfig.LLMConfig = config

	return SaveConfig()
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

	// 序列化并保存
	data, err := json.MarshalIndent(currentConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(configFile, data, 0644)
}
