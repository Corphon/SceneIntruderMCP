package nvidia

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
	llm.Register("nvidia", func() llm.Provider {
		return &Provider{
			recommendedModels: []string{
				"moonshotai/kimi-k2.5",
				"meta/llama-3.3-70b-instruct",
				"nvidia/llama-3.1-nemotron-ultra-253b-v1",
			},
			baseURL: "https://integrate.api.nvidia.com/v1",
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
}

func (p *Provider) Initialize(config map[string]string) error {
	apiKey, exists := config["api_key"]
	if !exists || apiKey == "" {
		return errors.New("NVIDIA API密钥未提供")
	}

	p.apiKey = apiKey
	p.client = &http.Client{}

	if model, exists := config["default_model"]; exists && model != "" {
		p.defaultModel = model
	} else {
		p.defaultModel = "moonshotai/kimi-k2.5"
	}

	if baseURL, exists := config["base_url"]; exists && baseURL != "" {
		p.baseURL = baseURL
	}

	if customModels, exists := config["custom_models"]; exists && customModels != "" {
		var models []string
		if err := json.Unmarshal([]byte(customModels), &models); err == nil && len(models) > 0 {
			p.availableModels = models
		}
	}

	return nil
}

func (p *Provider) GetName() string {
	return "NVIDIA"
}

func (p *Provider) GetSupportedModels() []string {
	if len(p.availableModels) > 0 {
		return p.availableModels
	}
	return p.recommendedModels
}

func (p *Provider) FetchAvailableModels(ctx context.Context) error {
	if p.apiKey == "" {
		return errors.New("API密钥未设置，无法获取模型列表")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("获取模型列表失败(%d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	p.availableModels = make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		if strings.TrimSpace(model.ID) != "" {
			p.availableModels = append(p.availableModels, model.ID)
		}
	}

	return nil
}

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
	model, extraParams, reasoningEnabled := llm.NormalizeReasoningRequest("nvidia", model, req.ExtraParams)

	messages := []map[string]string{{"role": "user", "content": req.Prompt}}
	if req.SystemPrompt != "" {
		messages = append([]map[string]string{{"role": "system", "content": req.SystemPrompt}}, messages...)
	}

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

	for k, v := range extraParams {
		requestBody[k] = v
	}
	llm.ApplyReasoningDefaults("nvidia", requestBody, model, reasoningEnabled)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("NVIDIA API错误(%d): %s", httpResp.StatusCode, string(body))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, errors.New("NVIDIA未返回任何结果")
	}

	modelName := strings.TrimSpace(response.Model)
	if modelName == "" {
		modelName = model
	}

	return &llm.CompletionResponse{
		Text:         response.Choices[0].Message.Content,
		FinishReason: response.Choices[0].FinishReason,
		TokensUsed:   response.Usage.TotalTokens,
		PromptTokens: response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
		ModelName:    modelName,
		ProviderName: p.GetName(),
	}, nil
}

func (p *Provider) StreamCompletion(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	model, extraParams, reasoningEnabled := llm.NormalizeReasoningRequest("nvidia", model, req.ExtraParams)

	messages := []map[string]string{{"role": "user", "content": req.Prompt}}
	if req.SystemPrompt != "" {
		messages = append([]map[string]string{{"role": "system", "content": req.SystemPrompt}}, messages...)
	}

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

	for k, v := range extraParams {
		requestBody[k] = v
	}
	llm.ApplyReasoningDefaults("nvidia", requestBody, model, reasoningEnabled)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("NVIDIA API错误(%d): %s", httpResp.StatusCode, string(body))
	}

	respChan := make(chan llm.StreamResponse)

	go func() {
		defer httpResp.Body.Close()
		defer close(respChan)

		reader := bufio.NewReader(httpResp.Body)
		var modelName string
		var completionSent bool

		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						respChan <- llm.StreamResponse{Done: true, FinishReason: "error", ModelName: modelName}
					}
					return
				}

				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, ":") {
					continue
				}
				line = strings.TrimPrefix(line, "data: ")

				if line == "[DONE]" {
					if !completionSent {
						respChan <- llm.StreamResponse{Done: true, FinishReason: "stop", ModelName: modelName}
					}
					return
				}

				var streamResp struct {
					Model   string `json:"model"`
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

				if modelName == "" && streamResp.Model != "" {
					modelName = streamResp.Model
				}

				if len(streamResp.Choices) == 0 {
					continue
				}

				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					respChan <- llm.StreamResponse{Text: content, ModelName: modelName, Done: false}
				}

				if streamResp.Choices[0].FinishReason != nil {
					respChan <- llm.StreamResponse{
						Done:         true,
						FinishReason: *streamResp.Choices[0].FinishReason,
						ModelName:    modelName,
					}
					completionSent = true
				}
			}
		}
	}()

	return respChan, nil
}
