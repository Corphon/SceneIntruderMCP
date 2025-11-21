// internal/llm/providers/qwen/qwen.go
package qwen

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Corphon/SceneIntruderMCP/internal/llm"
)

func init() {
	llm.Register("qwen", func() llm.Provider {
		return &Provider{
			recommendedModels: []string{

				"qwen3-max",
			},
			baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", // Default to compatibility mode
		}
	})
}

type Provider struct {
	apiKey            string
	baseURL           string
	client            *http.Client
	defaultModel      string
	recommendedModels []string
	availableModels   []string
	region            string // 阿里云区域
}

func (p *Provider) Initialize(config map[string]string) error {
	apiKey, exists := config["api_key"]
	if !exists || apiKey == "" {
		return errors.New("qwen APIKey not provided")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "qwen3-max"
	}

	if baseURL, exists := config["base_url"]; exists && baseURL != "" {
		p.baseURL = baseURL
	}

	if region, exists := config["region"]; exists && region != "" {
		p.region = region
	} else {
		p.region = "cn-beijing" // 默认区域
	}

	// 如果配置中包含自定义模型列表
	if customModels, exists := config["custom_models"]; exists && customModels != "" {
		var models []string
		if err := json.Unmarshal([]byte(customModels), &models); err == nil && len(models) > 0 {
			p.availableModels = models
		}
	}

	return nil
}

func (p *Provider) GetName() string {
	return "qwen"
}

func (p *Provider) GetSupportedModels() []string {
	// 如果已经通过API获取了真实模型列表，则返回它
	if len(p.availableModels) > 0 {
		return p.availableModels
	}
	// 否则返回推荐模型列表
	return p.recommendedModels
}

// 设置自定义模型列表
func (p *Provider) SetCustomModels(models []string) {
	if len(models) > 0 {
		p.availableModels = models
	}
}

// StreamCompletion 实现流式响应
func (p *Provider) StreamCompletion(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// 构建请求
	messages := []map[string]string{
		{"role": "user", "content": req.Prompt},
	}

	if req.SystemPrompt != "" {
		messages = append([]map[string]string{
			{"role": "system", "content": req.SystemPrompt},
		}, messages...)
	}

	// 构建OpenAI兼容的请求体
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": req.Temperature,
		"stream":      true, // 启用流式输出 for OpenAI-compatible APIs
	}

	if req.MaxTokens > 0 {
		requestBody["max_tokens"] = req.MaxTokens
	}

	if req.TopP > 0 {
		requestBody["top_p"] = req.TopP
	}

	if len(req.StopWords) > 0 {
		requestBody["stop"] = req.StopWords
	}

	// 添加任何额外参数
	if req.ExtraParams != nil {
		for k, v := range req.ExtraParams {
			requestBody[k] = v
		}
	}

	// 序列化JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// 创建HTTP请求 - 使用兼容模式的正确端点
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		p.baseURL+"/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	// Only add X-DashScope-SSE header for stream requests
	httpReq.Header.Set("X-DashScope-SSE", "enable")
	// Add region header if available
	if p.region != "" {
		httpReq.Header.Set("X-DashScope-Region", p.region)
	}

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("qwen API Error(%d): %s", httpResp.StatusCode, string(body))
	}

	// 创建响应通道
	respChan := make(chan llm.StreamResponse)

	// 启动goroutine处理流式响应
	go func() {
		defer httpResp.Body.Close()
		defer close(respChan)

		reader := bufio.NewReader(httpResp.Body)
		var contentBuffer strings.Builder

		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						respChan <- llm.StreamResponse{
							Text:         contentBuffer.String(),
							FinishReason: "error",
							ModelName:    model,
							Done:         true,
						}
					}
					return
				}

				line = strings.TrimSpace(line)

				// 空行或注释
				if line == "" || strings.HasPrefix(line, ":") {
					continue
				}

				// 移除 "data: " 前缀
				line = strings.TrimPrefix(line, "data: ")

				// 解析OpenAI兼容的流式响应
				var streamResp struct {
					ID      string `json:"id"`
					Object  string `json:"object"`
					Created int64  `json:"created"`
					Model   string `json:"model"`
					Choices []struct {
						Index int `json:"index"`
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
					Usage *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					} `json:"usage"`
				}

				if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
					continue
				}

				// 从delta.content获取增量内容（流式）
				content := ""
				done := false

				if len(streamResp.Choices) > 0 {
					choice := streamResp.Choices[0]
					if choice.Delta.Content != "" {
						// 流式增量内容
						content = choice.Delta.Content
					} else if choice.Message.Content != "" {
						// 如果没有delta，使用完整content（非流式场景）
						content = choice.Message.Content
					}

					if choice.FinishReason != "" {
						done = true
					}
				}

				if content != "" {
					// 对于OpenAI兼容的流式API，delta是增量内容
					contentBuffer.WriteString(content)

					respChan <- llm.StreamResponse{
						Text:      content,
						ModelName: model,
						Done:      false,
					}
				}

				// 检查是否已完成
				if done {
					respChan <- llm.StreamResponse{
						Text:         contentBuffer.String(),
						FinishReason: "stop", // Standard finish reason
						ModelName:    model,
						Done:         true,
					}
					return
				}
			}
		}
	}()

	return respChan, nil
}

func (p *Provider) CompleteText(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// 构建请求 - 使用OpenAI兼容格式
	messages := []map[string]string{
		{"role": "user", "content": req.Prompt},
	}

	if req.SystemPrompt != "" {
		// 在前面添加系统提示
		messages = append([]map[string]string{
			{"role": "system", "content": req.SystemPrompt},
		}, messages...)
	}

	// 构建OpenAI兼容的请求体
	requestBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}

	if req.Temperature > 0 {
		requestBody["temperature"] = req.Temperature
	}

	if req.MaxTokens > 0 {
		requestBody["max_tokens"] = req.MaxTokens
	}

	if req.TopP > 0 {
		requestBody["top_p"] = req.TopP
	}

	if len(req.StopWords) > 0 {
		requestBody["stop"] = req.StopWords
	}

	// 添加任何额外参数
	if req.ExtraParams != nil {
		for k, v := range req.ExtraParams {
			requestBody[k] = v
		}
	}

	// 序列化JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// 创建HTTP请求 - 使用兼容模式的正确端点
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		p.baseURL+"/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	// Add region header if available
	if p.region != "" {
		httpReq.Header.Set("X-DashScope-Region", p.region)
	}

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("qwen API Error(%d): %s", httpResp.StatusCode, string(body))
	}

	// 解析OpenAI兼容的响应
	var response struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Choices) == 0 {
		return nil, errors.New("qwen returned no results")
	}

	text := response.Choices[0].Message.Content
	finishReason := response.Choices[0].FinishReason

	if text == "" {
		return nil, errors.New("qwen returned empty content")
	}

	return &llm.CompletionResponse{
		Text:         text,
		FinishReason: finishReason,
		TokensUsed:   response.Usage.TotalTokens,
		PromptTokens: response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
		ModelName:    model,
		ProviderName: p.GetName(),
	}, nil
}

// FetchAvailableModels 尝试获取千问平台上可用的模型列表
func (p *Provider) FetchAvailableModels(ctx context.Context) error {
	if p.apiKey == "" {
		return errors.New("api key not set, unable to fetch model list")
	}

	// For compatible mode, models are usually known in advance, so we can just validate key works
	// Instead of getting model list, let's do a simple test call to validate key
	testReq := llm.CompletionRequest{
		Model:       p.defaultModel,
		Prompt:      "Say 'test' to verify API key",
		Temperature: 0.1,
		MaxTokens:   5,
	}

	// Do a simple test request to verify the API key works
	_, err := p.CompleteText(ctx, testReq)
	if err != nil {
		return fmt.Errorf("API key authentication failed: %v", err)
	}

	// If successful, just use the recommended models
	p.availableModels = p.recommendedModels

	return nil
}
