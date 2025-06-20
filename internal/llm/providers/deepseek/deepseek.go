// internal/llm/providers/deepseek/deepseek.go
package deepseek

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
	llm.Register("deepseek", func() llm.Provider {
		return &Provider{
			recommendedModels: []string{
				"deepseek-chat",
				"deepseek-reasoner",
			},
			baseURL: "https://api.deepseek.com/v1",
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
	organization      string // 组织ID（可选）
}

func (p *Provider) Initialize(config map[string]string) error {
	apiKey, exists := config["api_key"]
	if !exists || apiKey == "" {
		return errors.New("DeepSeek API密钥未提供")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "deepseek-reasoner" // 默认使用通用聊天模型
	}

	if baseURL, exists := config["base_url"]; exists && baseURL != "" {
		p.baseURL = baseURL
	}

	if org, exists := config["organization"]; exists {
		p.organization = org
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
	return "DeepSeek"
}

func (p *Provider) GetSupportedModels() []string {
	// 如果已经通过API获取了真实模型列表，则返回它
	if len(p.availableModels) > 0 {
		return p.availableModels
	}
	// 否则返回推荐模型列表
	return p.recommendedModels
}

// FetchAvailableModels 尝试获取DeepSeek上可用的模型列表
func (p *Provider) FetchAvailableModels(ctx context.Context) error {
	if p.apiKey == "" {
		return errors.New("API密钥未设置，无法获取模型列表")
	}

	// 构建请求
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.organization != "" {
		req.Header.Set("DeepSeek-Organization", p.organization)
	}

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("获取模型列表失败(%d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// 提取模型ID
	p.availableModels = make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		p.availableModels = append(p.availableModels, model.ID)
	}

	// 如果API没有返回任何模型，使用推荐模型列表
	if len(p.availableModels) == 0 {
		p.availableModels = p.recommendedModels
	}

	return nil
}

// SetCustomModels 设置自定义模型列表
func (p *Provider) SetCustomModels(models []string) {
	if len(models) > 0 {
		p.availableModels = models
	}
}

func (p *Provider) CompleteText(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// 构建请求
	messages := []map[string]string{
		{"role": "user", "content": req.Prompt},
	}

	if req.SystemPrompt != "" {
		// 在前面添加系统提示
		messages = append([]map[string]string{
			{"role": "system", "content": req.SystemPrompt},
		}, messages...)
	}

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": req.Temperature,
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

	// 创建HTTP请求
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
	if p.organization != "" {
		httpReq.Header.Set("DeepSeek-Organization", p.organization)
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
		return nil, fmt.Errorf("DeepSeek API错误(%d): %s", httpResp.StatusCode, string(body))
	}

	// 解析响应
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
		return nil, errors.New("DeepSeek未返回任何结果")
	}

	return &llm.CompletionResponse{
		Text:         response.Choices[0].Message.Content,
		FinishReason: response.Choices[0].FinishReason,
		TokensUsed:   response.Usage.TotalTokens,
		PromptTokens: response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
		ModelName:    response.Model,
		ProviderName: p.GetName(),
	}, nil
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

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": req.Temperature,
		"stream":      true,
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

	// 创建HTTP请求
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
	if p.organization != "" {
		httpReq.Header.Set("DeepSeek-Organization", p.organization)
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
		return nil, fmt.Errorf("DeepSeek API错误(%d): %s", httpResp.StatusCode, string(body))
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

				// 检查流结束
				if line == "[DONE]" {
					respChan <- llm.StreamResponse{
						Text:         contentBuffer.String(),
						FinishReason: "stop",
						ModelName:    model,
						Done:         true,
					}
					return
				}

				// 解析JSON数据
				var streamResp struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
							Role    string `json:"role"`
						} `json:"delta"`
						FinishReason *string `json:"finish_reason"`
					} `json:"choices"`
				}

				if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
					continue
				}

				if len(streamResp.Choices) > 0 {
					content := streamResp.Choices[0].Delta.Content
					if content != "" {
						contentBuffer.WriteString(content)
						respChan <- llm.StreamResponse{
							Text:      content,
							ModelName: model,
							Done:      false,
						}
					}

					// 检查是否已完成
					if streamResp.Choices[0].FinishReason != nil {
						respChan <- llm.StreamResponse{
							Text:         contentBuffer.String(),
							FinishReason: *streamResp.Choices[0].FinishReason,
							ModelName:    model,
							Done:         true,
						}
						return
					}
				}
			}
		}
	}()

	return respChan, nil
}
