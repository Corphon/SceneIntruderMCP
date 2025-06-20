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
				"qwen2.5-max",
				"qwen2.5-plus",
				"qwq-32b",
			},
			baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
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
		return errors.New("千问(Qwen) API密钥未提供")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "qwen2.5-max"
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
	return "Qwen"
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

	// 构建请求体
	requestBody := map[string]interface{}{
		"model": model,
		"input": map[string]interface{}{
			"messages": messages,
		},
		"parameters": map[string]interface{}{
			"temperature":        req.Temperature,
			"incremental_output": true, // 启用流式输出
		},
	}

	if req.MaxTokens > 0 {
		requestBody["parameters"].(map[string]interface{})["max_tokens"] = req.MaxTokens
	}

	if req.TopP > 0 {
		requestBody["parameters"].(map[string]interface{})["top_p"] = req.TopP
	}

	if len(req.StopWords) > 0 {
		requestBody["parameters"].(map[string]interface{})["stop"] = req.StopWords
	}

	// 添加任何额外参数
	if req.ExtraParams != nil {
		paramsMap := requestBody["parameters"].(map[string]interface{})
		for k, v := range req.ExtraParams {
			paramsMap[k] = v
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
		p.baseURL+"/services/aigc/text-generation/generation",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("X-DashScope-SSE", "enable")
	httpReq.Header.Set("X-DashScope-Region", p.region)

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("千问(Qwen) API错误(%d): %s", httpResp.StatusCode, string(body))
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

				// 解析JSON数据
				var streamResp struct {
					Output struct {
						Text    string `json:"text"`
						Choices []struct {
							FinishReason string `json:"finish_reason"`
							Message      struct {
								Content string `json:"content"`
							} `json:"message"`
						} `json:"choices"`
					} `json:"output"`
					Usage struct {
						OutputTokens int `json:"output_tokens"`
						InputTokens  int `json:"input_tokens"`
						TotalTokens  int `json:"total_tokens"`
					} `json:"usage"`
				}

				if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
					continue
				}

				// 先尝试从text字段获取
				content := streamResp.Output.Text

				// 如果text为空，尝试从choices获取
				if content == "" && len(streamResp.Output.Choices) > 0 {
					content = streamResp.Output.Choices[0].Message.Content
				}

				if content != "" {
					// 阿里云的流式响应不是增量的，而是全量文本
					// 需要计算增量部分
					currentLength := contentBuffer.Len()
					if len(content) > currentLength {
						delta := content[currentLength:]
						contentBuffer.WriteString(delta)

						respChan <- llm.StreamResponse{
							Text:      delta,
							ModelName: model,
							Done:      false,
						}
					}
				}

				// 检查是否已完成
				if len(streamResp.Output.Choices) > 0 && streamResp.Output.Choices[0].FinishReason != "" {
					respChan <- llm.StreamResponse{
						Text:         contentBuffer.String(),
						FinishReason: streamResp.Output.Choices[0].FinishReason,
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
		"model": model,
		"input": map[string]interface{}{
			"messages": messages,
		},
		"parameters": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if req.MaxTokens > 0 {
		requestBody["parameters"].(map[string]interface{})["max_tokens"] = req.MaxTokens
	}

	if req.TopP > 0 {
		requestBody["parameters"].(map[string]interface{})["top_p"] = req.TopP
	}

	if len(req.StopWords) > 0 {
		requestBody["parameters"].(map[string]interface{})["stop"] = req.StopWords
	}

	// 添加任何额外参数
	if req.ExtraParams != nil {
		paramsMap := requestBody["parameters"].(map[string]interface{})
		for k, v := range req.ExtraParams {
			paramsMap[k] = v
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
		p.baseURL+"/services/aigc/text-generation/generation",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("X-DashScope-Region", p.region)

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("千问(Qwen) API错误(%d): %s", httpResp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		RequestID string `json:"request_id"`
		Output    struct {
			Text    string `json:"text"`
			Choices []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return nil, err
	}

	// 使用output.text或choices[0].message.content
	text := response.Output.Text
	finishReason := ""

	// 从choices获取完成原因，无论text是否为空
	if len(response.Output.Choices) > 0 {
		finishReason = response.Output.Choices[0].FinishReason

		// 如果text为空，尝试从choices获取内容
		if text == "" {
			text = response.Output.Choices[0].Message.Content
		}
	}

	if text == "" {
		return nil, errors.New("千问(Qwen)未返回任何结果")
	}

	return &llm.CompletionResponse{
		Text:         text,
		FinishReason: finishReason,
		TokensUsed:   response.Usage.TotalTokens,
		PromptTokens: response.Usage.InputTokens,
		OutputTokens: response.Usage.OutputTokens,
		ModelName:    model,
		ProviderName: p.GetName(),
	}, nil
}

// FetchAvailableModels 尝试获取千问平台上可用的模型列表
func (p *Provider) FetchAvailableModels(ctx context.Context) error {
	if p.apiKey == "" {
		return errors.New("API密钥未设置，无法获取模型列表")
	}

	// 构建请求 - 修正URL路径结构，确保包含/api/v1前缀
	baseAPIURL := strings.Replace(p.baseURL, "/compatible-mode/v1", "", 1)
	url := fmt.Sprintf("%s/api/v1/services/api/models", baseAPIURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("X-DashScope-Region", p.region)

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
			ModelName  string `json:"name"`
			ModelID    string `json:"id"`
			ModelType  string `json:"type"`
			Generation string `json:"generation"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// 提取模型ID，只保留文本生成模型
	p.availableModels = make([]string, 0)
	for _, model := range response.Models {
		// 只收集文本生成相关模型
		if strings.Contains(model.ModelType, "text-generation") ||
			strings.Contains(model.ModelType, "chat") ||
			strings.HasPrefix(model.ModelID, "qwen") {
			p.availableModels = append(p.availableModels, model.ModelID)
		}
	}

	// 如果API没有返回任何模型，使用推荐模型列表
	if len(p.availableModels) == 0 {
		p.availableModels = p.recommendedModels
	}

	return nil
}
