// internal/llm/providers/anthropic/anthropic.go
package anthropic

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
	llm.Register("anthropic", func() llm.Provider {
		return &Provider{
			recommendedModels: []string{
				"claude-3.5-sonnet",
				"claude-3.7-sonnet",
				"claude-3.7-sonnet-thinking",
			},
			baseURL:    "https://api.anthropic.com",
			apiVersion: "2023-06-01",
		}
	})
}

type Provider struct {
	apiKey            string
	baseURL           string
	apiVersion        string
	client            *http.Client
	defaultModel      string
	recommendedModels []string
	availableModels   []string
}

func (p *Provider) Initialize(config map[string]string) error {
	apiKey, exists := config["api_key"]
	if !exists || apiKey == "" {
		return errors.New("anthropic api密钥未提供")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "claude-3-sonnet-20240229"
	}

	if baseURL, exists := config["base_url"]; exists && baseURL != "" {
		p.baseURL = baseURL
	}

	if apiVersion, exists := config["api_version"]; exists && apiVersion != "" {
		p.apiVersion = apiVersion
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
	return "Anthropic Claude"
}

func (p *Provider) GetSupportedModels() []string {
	// 如果通过API或用户配置已获取真实模型列表，则返回它
	if len(p.availableModels) > 0 {
		return p.availableModels
	}
	// 否则返回推荐模型列表
	return p.recommendedModels
}

// 尝试获取用户账户可用的模型列表
func (p *Provider) FetchAvailableModels(ctx context.Context) error {
	if p.apiKey == "" {
		return errors.New("API密钥未设置，无法获取模型列表")
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v1/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", p.apiKey)
	req.Header.Set("Anthropic-Version", p.apiVersion)

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
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// 提取模型名称
	p.availableModels = make([]string, 0, len(response.Models))
	for _, model := range response.Models {
		p.availableModels = append(p.availableModels, model.Name)
	}

	return nil
}

// 添加设置自定义模型列表的方法
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

	// 构建Anthropic请求
	messages := []map[string]interface{}{
		{"role": "user", "content": req.Prompt},
	}

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}

	if req.SystemPrompt != "" {
		requestBody["system"] = req.SystemPrompt
	}

	if req.TopP > 0 {
		requestBody["top_p"] = req.TopP
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
		p.baseURL+"/v1/messages",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", p.apiKey)
	httpReq.Header.Set("Anthropic-Version", p.apiVersion)

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("anthropic api错误(%d): %s", httpResp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return nil, err
	}

	// 提取文本内容
	var textContent string
	for _, content := range response.Content {
		if content.Type == "text" {
			textContent = content.Text
			break
		}
	}

	if textContent == "" {
		return nil, errors.New("Anthropic未返回文本内容")
	}

	return &llm.CompletionResponse{
		Text:         textContent,
		FinishReason: response.StopReason,
		TokensUsed:   response.Usage.InputTokens + response.Usage.OutputTokens,
		PromptTokens: response.Usage.InputTokens,
		OutputTokens: response.Usage.OutputTokens,
		ModelName:    model,
		ProviderName: p.GetName(),
	}, nil
}

// StreamCompletion 实现流式响应
func (p *Provider) StreamCompletion(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// 构建Anthropic请求
	messages := []map[string]interface{}{
		{"role": "user", "content": req.Prompt},
	}

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"stream":      true,
	}

	if req.SystemPrompt != "" {
		requestBody["system"] = req.SystemPrompt
	}

	if req.TopP > 0 {
		requestBody["top_p"] = req.TopP
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
		p.baseURL+"/v1/messages",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", p.apiKey)
	httpReq.Header.Set("Anthropic-Version", p.apiVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("anthropic api错误(%d): %s", httpResp.StatusCode, string(body))
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
							FinishReason: "stop",
							ModelName:    model, // 添加模型名称
							Done:         true,
						}
					}
					return
				}

				line = strings.TrimSpace(line)

				// 空行或注释
				if line == "" || !strings.HasPrefix(line, "data: ") {
					continue
				}

				// 移除 "data: " 前缀
				line = line[6:]

				// 检查流结束
				if line == "[DONE]" {
					respChan <- llm.StreamResponse{
						Text:         contentBuffer.String(),
						FinishReason: "stop",
						Done:         true,
					}
					return
				}

				// 解析JSON数据
				var streamResp struct {
					Type       string `json:"type"`
					StopReason string `json:"stop_reason"`
					Delta      struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"delta"`
				}

				if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
					continue
				}

				// 处理内容块
				if streamResp.Type == "content_block_delta" && streamResp.Delta.Type == "text" {
					content := streamResp.Delta.Text
					if content != "" {
						contentBuffer.WriteString(content)
						respChan <- llm.StreamResponse{
							Text: content,
							Done: false,
						}
					}
				}

				// 检查是否已完成
				if streamResp.Type == "message_stop" && streamResp.StopReason != "" {
					respChan <- llm.StreamResponse{
						Text:         contentBuffer.String(),
						FinishReason: streamResp.StopReason,
						Done:         true,
					}
					return
				}
			}
		}
	}()

	return respChan, nil
}
