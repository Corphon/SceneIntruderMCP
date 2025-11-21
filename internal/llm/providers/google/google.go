// internal/llm/providers/google/google.go
package google

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
	llm.Register("google", func() llm.Provider {
		return &Provider{
			models: []string{
				"gemini-2.5-pro",
				"gemini-2.5-flash",
			},
			baseURL: "https://generativelanguage.googleapis.com/v1",
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
	models            []string
	projectID         string // 可选，对于某些环境可能需要
}

func (p *Provider) Initialize(config map[string]string) error {
	apiKey, exists := config["api_key"]
	if !exists || apiKey == "" {
		return errors.New("google_api密钥未提供")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "gemini-2.0-flash"
	}

	if baseURL, exists := config["base_url"]; exists && baseURL != "" {
		p.baseURL = baseURL
	}

	if projectID, exists := config["project_id"]; exists {
		p.projectID = projectID
	}

	return nil
}

func (p *Provider) GetName() string {
	return "google gemini"
}

func (p *Provider) GetSupportedModels() []string {
	if len(p.availableModels) > 0 {
		return p.availableModels
	}
	// 否则返回推荐模型列表
	return p.recommendedModels

}

func (p *Provider) CompleteText(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// 构建Gemini请求
	contents := []map[string]interface{}{
		{"role": "user", "parts": []map[string]string{{"text": req.Prompt}}},
	}

	if req.SystemPrompt != "" {
		// 将系统提示作为上下文添加
		contents = append([]map[string]interface{}{
			{"role": "system", "parts": []map[string]string{{"text": req.SystemPrompt}}},
		}, contents...)
	}

	requestBody := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if req.MaxTokens > 0 {
		requestBody["generationConfig"].(map[string]interface{})["maxOutputTokens"] = req.MaxTokens
	}

	if req.TopP > 0 {
		requestBody["generationConfig"].(map[string]interface{})["topP"] = req.TopP
	}

	if len(req.StopWords) > 0 {
		requestBody["generationConfig"].(map[string]interface{})["stopSequences"] = req.StopWords
	}

	// 添加任何额外参数
	if req.ExtraParams != nil {
		for k, v := range req.ExtraParams {
			// 如果是generationConfig中的参数
			if k == "topK" || k == "candidateCount" {
				requestBody["generationConfig"].(map[string]interface{})[k] = v
			} else {
				requestBody[k] = v
			}
		}
	}

	// 序列化JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// 构建URL (注意Gemini API的结构与OpenAI不同)
	apiURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, model, p.apiKey)

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		apiURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		body, _ := io.ReadAll(httpResp.Body)
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if errorObj, ok := errorResp["error"].(map[string]interface{}); ok {
				return nil, fmt.Errorf("google gemini API错误(%d): %v",
					httpResp.StatusCode, errorObj["message"])
			}
		}
		return nil, fmt.Errorf("google gemini API错误(%d): %s", httpResp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Candidates) == 0 {
		return nil, errors.New("google gemini未返回任何结果")
	}

	// 提取文本内容
	var resultText string
	for _, part := range response.Candidates[0].Content.Parts {
		resultText += part.Text
	}

	return &llm.CompletionResponse{
		Text:         resultText,
		FinishReason: response.Candidates[0].FinishReason,
		TokensUsed:   response.UsageMetadata.TotalTokenCount,
		PromptTokens: response.UsageMetadata.PromptTokenCount,
		OutputTokens: response.UsageMetadata.CandidatesTokenCount,
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

	// 构建Gemini请求
	contents := []map[string]interface{}{
		{"role": "user", "parts": []map[string]string{{"text": req.Prompt}}},
	}

	if req.SystemPrompt != "" {
		contents = append([]map[string]interface{}{
			{"role": "system", "parts": []map[string]string{{"text": req.SystemPrompt}}},
		}, contents...)
	}

	requestBody := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature": req.Temperature,
		},
		"streamGenerationConfig": map[string]interface{}{
			"streamContentTypes": []string{"text"},
		},
	}

	if req.MaxTokens > 0 {
		requestBody["generationConfig"].(map[string]interface{})["maxOutputTokens"] = req.MaxTokens
	}

	if req.TopP > 0 {
		requestBody["generationConfig"].(map[string]interface{})["topP"] = req.TopP
	}

	if len(req.StopWords) > 0 {
		requestBody["generationConfig"].(map[string]interface{})["stopSequences"] = req.StopWords
	}

	// 序列化JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// 构建URL
	apiURL := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s", p.baseURL, model, p.apiKey)

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		apiURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// 检查错误
	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("google gemini API错误(%d): %s", httpResp.StatusCode, string(body))
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
				line, err := reader.ReadBytes('\n')
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

				// 跳过空行
				if len(line) <= 1 {
					continue
				}

				// 解析JSON数据
				var streamResp struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
						FinishReason string `json:"finishReason,omitempty"`
					} `json:"candidates"`
				}

				if err := json.Unmarshal(line, &streamResp); err != nil {
					continue
				}

				if len(streamResp.Candidates) > 0 {
					var text string
					for _, part := range streamResp.Candidates[0].Content.Parts {
						text += part.Text
					}

					if text != "" {
						contentBuffer.WriteString(text)
						respChan <- llm.StreamResponse{
							Text: text,
							Done: false,
						}
					}

					// 检查是否已完成
					if streamResp.Candidates[0].FinishReason != "" {
						respChan <- llm.StreamResponse{
							Text:         contentBuffer.String(),
							FinishReason: streamResp.Candidates[0].FinishReason,
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

// ：尝试获取用户账户可用的模型列表
func (p *Provider) FetchAvailableModels(ctx context.Context) error {
	if p.apiKey == "" {
		return errors.New("API密钥未设置，无法获取模型列表")
	}

	// 构建请求
	url := fmt.Sprintf("%s/models?key=%s", p.baseURL, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
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
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}
	// 提取模型名称
	p.availableModels = make([]string, 0, len(response.Models))
	for _, model := range response.Models {
		// 从完整路径中提取模型名称 (如 "models/gemini-pro" -> "gemini-pro")
		parts := strings.Split(model.Name, "/")
		modelName := parts[len(parts)-1]
		p.availableModels = append(p.availableModels, modelName)
	}

	return nil
}

// 添加设置自定义模型列表的方法
func (p *Provider) SetCustomModels(models []string) {
	if len(models) > 0 {
		p.availableModels = models
	}
}
