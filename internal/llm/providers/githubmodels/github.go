// internal/llm/providers/githubmodels/github.go
package githubmodels

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
	llm.Register("githubmodels", func() llm.Provider {
		return &Provider{
			recommendedModels: []string{
				"gpt-4o",
				"o1",
				"o3-mini",
				"Phi-4",
				"Phi-4-multimodal-instruct",
			},
			baseURL: "https://models.inference.ai.azure.com",
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
	deploymentID      string // 部署ID
	apiVersion        string // API版本
	region            string // Azure区域
}

func (p *Provider) Initialize(config map[string]string) error {
	apiKey, exists := config["api_key"]
	if !exists || apiKey == "" {
		return errors.New("azure models api密钥未提供")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "o3-mini"
	}

	if baseURL, exists := config["base_url"]; exists && baseURL != "" {
		p.baseURL = baseURL
	}

	// 额外的Azure特定配置
	if deploymentID, exists := config["deployment_id"]; exists {
		p.deploymentID = deploymentID
	}

	if apiVersion, exists := config["api_version"]; exists {
		p.apiVersion = apiVersion
	} else {
		p.apiVersion = "2023-08-01" // 默认API版本
	}

	if region, exists := config["region"]; exists {
		p.region = region
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
	return "Azure AI Models"
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

	// Azure Models的API可能有变化，这里采用通用查询方式
	// 构建请求
	url := fmt.Sprintf("%s/models?api-version=%s", p.baseURL, p.apiVersion)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("api-key", p.apiKey)
	if p.region != "" {
		req.Header.Set("azureml-model-deployment", p.region)
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
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// 提取模型名称
	p.availableModels = make([]string, 0, len(response.Data))
	for _, model := range response.Data {
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

	// 确定要使用的端点
	var endpoint string
	if p.deploymentID != "" {
		// 如果有特定的部署ID
		endpoint = fmt.Sprintf("/deployments/%s/chat/completions", p.deploymentID)
	} else {
		// 根据模型构建端点
		modelPath := strings.ReplaceAll(model, "/", "/models/")
		endpoint = fmt.Sprintf("/models/%s/chat/completions", modelPath)
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
		"messages":    messages,
		"temperature": req.Temperature,
		"model":       model, // 有些API需要显式指定模型
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

	// 构建URL，添加API版本参数
	url := fmt.Sprintf("%s%s", p.baseURL, endpoint)
	if strings.Contains(url, "?") {
		url = fmt.Sprintf("%s&api-version=%s", url, p.apiVersion)
	} else {
		url = fmt.Sprintf("%s?api-version=%s", url, p.apiVersion)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	// 如果特定了区域，添加区域头
	if p.region != "" {
		httpReq.Header.Set("azureml-model-deployment", p.region)
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
		return nil, fmt.Errorf("azure models api错误(%d): %s", httpResp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
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
		return nil, errors.New("azure models未返回任何结果")
	}

	return &llm.CompletionResponse{
		Text:         response.Choices[0].Message.Content,
		FinishReason: response.Choices[0].FinishReason,
		TokensUsed:   response.Usage.TotalTokens,
		PromptTokens: response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
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

	// 确定要使用的端点
	var endpoint string
	if p.deploymentID != "" {
		endpoint = fmt.Sprintf("/deployments/%s/chat/completions", p.deploymentID)
	} else {
		modelPath := strings.ReplaceAll(model, "/", "/models/")
		endpoint = fmt.Sprintf("/models/%s/chat/completions", modelPath)
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
		"messages":    messages,
		"temperature": req.Temperature,
		"model":       model,
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

	// 构建URL，添加API版本参数
	url := fmt.Sprintf("%s%s", p.baseURL, endpoint)
	if strings.Contains(url, "?") {
		url = fmt.Sprintf("%s&api-version=%s", url, p.apiVersion)
	} else {
		url = fmt.Sprintf("%s?api-version=%s", url, p.apiVersion)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	if p.region != "" {
		httpReq.Header.Set("azureml-model-deployment", p.region)
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
		return nil, fmt.Errorf("azure models api错误(%d): %s", httpResp.StatusCode, string(body))
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
						Done:         true,
					}
					return
				}

				// 解析JSON数据
				var streamResp struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
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
							Text: content,
							Done: false,
						}
					}

					// 检查是否已完成
					if streamResp.Choices[0].FinishReason != nil {
						respChan <- llm.StreamResponse{
							Text:         contentBuffer.String(),
							FinishReason: *streamResp.Choices[0].FinishReason,
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
