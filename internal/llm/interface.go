// internal/llm/interface.go
package llm

import (
	"context"
	"errors"
)

// 错误定义
var ErrUnknownProvider = errors.New("未知的AI提供者")

// 请求参数标准化
type CompletionRequest struct {
	Prompt       string                 `json:"prompt"`
	SystemPrompt string                 `json:"system_prompt,omitempty"`
	MaxTokens    int                    `json:"max_tokens,omitempty"`
	Temperature  float32                `json:"temperature,omitempty"`
	TopP         float32                `json:"top_p,omitempty"`
	Model        string                 `json:"model,omitempty"`
	StopWords    []string               `json:"stop_words,omitempty"`
	Stream       bool                   `json:"stream,omitempty"`
	ExtraParams  map[string]interface{} `json:"extra_params,omitempty"`
}

// 响应结构标准化
type CompletionResponse struct {
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason,omitempty"`
	TokensUsed   int    `json:"tokens_used,omitempty"`
	PromptTokens int    `json:"prompt_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	ProviderName string `json:"provider_name,omitempty"`
}

// 流式响应
type StreamResponse struct {
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	Done         bool   `json:"done"`
}

// Provider 定义所有LLM提供者必须实现的接口
type Provider interface {
	// 初始化提供者，传入配置
	Initialize(config map[string]string) error

	// 获取提供者名称
	GetName() string

	// 获取支持的模型列表
	GetSupportedModels() []string

	// 文本生成
	CompleteText(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// 流式响应生成
	StreamCompletion(ctx context.Context, req CompletionRequest) (<-chan StreamResponse, error)

	// 可选：获取可用模型列表（有些提供商支持）
	FetchAvailableModels(ctx context.Context) error

	// 可选：设置自定义模型列表
	SetCustomModels(models []string)
}

// Registry 提供者注册表
type Registry struct {
	providers map[string]func() Provider
}

// 全局注册表
var DefaultRegistry = &Registry{
	providers: make(map[string]func() Provider),
}

// Register 注册一个新的LLM提供者
func (r *Registry) Register(name string, factory func() Provider) {
	r.providers[name] = factory
}

// GetProvider 获取指定名称的提供者实例
func (r *Registry) GetProvider(name string, config map[string]string) (Provider, error) {
	factory, exists := r.providers[name]
	if !exists {
		return nil, ErrUnknownProvider
	}

	provider := factory()
	if err := provider.Initialize(config); err != nil {
		return nil, err
	}

	return provider, nil
}

// GetAvailableProviders 返回所有已注册的提供者名称
func (r *Registry) GetAvailableProviders() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func GetAvailableProviders() []string {
	return DefaultRegistry.GetAvailableProviders()
}

// 注册表和工厂函数类型
type ProviderFactory func() Provider

var providers = make(map[string]ProviderFactory)

// Register 注册提供者工厂
func Register(name string, factory ProviderFactory) {
	providers[name] = factory
}

// GetProvider 创建指定名称的提供者实例
func GetProvider(name string, config map[string]string) (Provider, error) {
	factory, exists := providers[name]
	if !exists {
		return nil, errors.New("未知的提供者: " + name)
	}

	provider := factory()
	err := provider.Initialize(config)
	return provider, err
}

// ListProviders 返回所有已注册的提供者名称
func ListProviders() []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	return names
}

// GetSupportedModelsForProvider 获取指定提供商支持的模型列表
func GetSupportedModelsForProvider(name string) []string {
	factory, exists := providers[name]
	if !exists {
		return []string{}
	}

	provider := factory()
	return provider.GetSupportedModels()
}
