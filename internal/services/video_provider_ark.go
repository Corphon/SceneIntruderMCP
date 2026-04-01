// internal/services/video_provider_ark.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

var (
	ErrArkVideoEndpointRequired = errors.New("ark video endpoint required")
	ErrArkVideoAPIKeyRequired   = errors.New("ark video api key required")
	ErrArkVideoTaskIDRequired   = errors.New("ark video task id required")
)

type ArkVideoProvider struct {
	Endpoint          string
	APIKey            string
	GenerationPath    string
	Client            *http.Client
	PollEvery         time.Duration
	PollAttempts      int
	ModelKeyMapping   map[string]string
	DefaultRatio      string
	DefaultResolution string
}

func NewArkVideoProvider(endpoint string, apiKey string) *ArkVideoProvider {
	return &ArkVideoProvider{
		Endpoint:          strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:            strings.TrimSpace(apiKey),
		GenerationPath:    "/contents/generations/tasks",
		PollEvery:         2 * time.Second,
		PollAttempts:      90,
		ModelKeyMapping:   map[string]string{},
		DefaultRatio:      "adaptive",
		DefaultResolution: "720p",
	}
}

func (p *ArkVideoProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *ArkVideoProvider) generationURL() (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrArkVideoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrArkVideoEndpointRequired
	}
	pathValue := strings.TrimSpace(p.GenerationPath)
	if pathValue == "" {
		pathValue = "/contents/generations/tasks"
	}
	u.Path = path.Join(u.Path, pathValue)
	return u.String(), nil
}

func (p *ArkVideoProvider) taskURL(taskID string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", ErrArkVideoTaskIDRequired
	}
	generationURL, err := p.generationURL()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(generationURL)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, taskID)
	return u.String(), nil
}

func (p *ArkVideoProvider) SubmitClipTask(ctx context.Context, req models.VideoClipRequest) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrArkVideoAPIKeyRequired
	}
	u, err := p.generationURL()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(p.buildRequestBody(req))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ark video submit failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *ArkVideoProvider) PollTask(ctx context.Context, taskID string) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrArkVideoAPIKeyRequired
	}
	u, err := p.taskURL(taskID)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ark video poll failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *ArkVideoProvider) buildRequestBody(req models.VideoClipRequest) map[string]interface{} {
	content := []map[string]interface{}{{
		"type": "text",
		"text": strings.TrimSpace(req.Prompt),
	}}
	if imageURL := strings.TrimSpace(req.ReferenceImageURL); imageURL != "" {
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": imageURL,
			},
			"role": "first_frame",
		})
	}
	body := map[string]interface{}{
		"model":          p.resolveModel(req.Model),
		"content":        content,
		"generate_audio": req.AudioEnabled,
		"duration":       arkDuration(req.DurationSec),
		"watermark":      false,
	}
	if ratio := p.resolveRatio(req.Resolution); ratio != "" {
		body["ratio"] = ratio
	}
	if resolution := p.resolveResolution(req.Resolution); resolution != "" {
		body["resolution"] = resolution
	}
	return body
}

func (p *ArkVideoProvider) resolveModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "doubao-seedance-1-5-pro"
	}
	if mapped, ok := p.ModelKeyMapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return strings.TrimSpace(mapped)
	}
	return model
}

func (p *ArkVideoProvider) resolveRatio(resolution string) string {
	resolution = strings.TrimSpace(resolution)
	if strings.Contains(resolution, ":") || strings.EqualFold(resolution, "adaptive") {
		return resolution
	}
	return strings.TrimSpace(p.DefaultRatio)
}

func (p *ArkVideoProvider) resolveResolution(resolution string) string {
	resolution = strings.TrimSpace(resolution)
	if resolution == "" || strings.Contains(resolution, ":") || strings.EqualFold(resolution, "adaptive") {
		return strings.TrimSpace(p.DefaultResolution)
	}
	return resolution
}

func arkDuration(durationSec float64) int {
	duration := int(durationSec)
	if duration <= 0 {
		return 5
	}
	return duration
}

func (p *ArkVideoProvider) parseTaskResponse(data []byte) (*models.VideoProviderTask, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	taskID := providerFirstNonEmpty(providerFirstString(raw, "id"), providerNestedFirstString(raw, []string{"data", "id"}))
	if taskID == "" {
		return nil, errors.New("ark video missing task id")
	}
	providerStatus := providerFirstNonEmpty(providerFirstString(raw, "status"), providerNestedFirstString(raw, []string{"data", "status"}))
	if providerStatus == "" && taskID != "" {
		providerStatus = "queued"
	}
	resultURL := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"content", "video_url"}),
		providerNestedFirstString(raw, []string{"data", "content", "video_url"}),
	)
	errMessage := providerFirstNonEmpty(providerFirstString(raw, "message"), providerNestedFirstString(raw, []string{"error", "message"}))
	return &models.VideoProviderTask{
		TaskID:         taskID,
		Status:         normalizeArkTaskStatus(providerStatus),
		ResultURL:      resultURL,
		ErrorMessage:   errMessage,
		ProviderStatus: providerStatus,
		Raw:            raw,
	}, nil
}

func normalizeArkTaskStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "queued", "running", "processing", "submitted":
		return "running"
	case "succeeded", "succeed", "completed":
		return "completed"
	case "failed", "expired", "cancelled", "canceled", "error":
		return "failed"
	default:
		return status
	}
}
