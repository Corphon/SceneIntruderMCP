// internal/services/video_provider_google_veo.go
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
	ErrGoogleVeoEndpointRequired = errors.New("google veo endpoint required")
	ErrGoogleVeoAPIKeyRequired   = errors.New("google veo api key required")
	ErrGoogleVeoTaskIDRequired   = errors.New("google veo task id required")
)

type GoogleVeoVideoProvider struct {
	Endpoint             string
	APIKey               string
	GeneratePathTemplate string
	OperationsPathPrefix string
	Client               *http.Client
	PollEvery            time.Duration
	PollAttempts         int
	ModelKeyMapping      map[string]string
	DefaultAspectRatio   string
}

func NewGoogleVeoVideoProvider(endpoint string, apiKey string) *GoogleVeoVideoProvider {
	return &GoogleVeoVideoProvider{
		Endpoint:             strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:               strings.TrimSpace(apiKey),
		GeneratePathTemplate: "/models/%s:generateVideos",
		OperationsPathPrefix: "/operations",
		PollEvery:            2 * time.Second,
		PollAttempts:         120,
		ModelKeyMapping:      map[string]string{},
		DefaultAspectRatio:   "16:9",
	}
}

func (p *GoogleVeoVideoProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *GoogleVeoVideoProvider) generateURL(model string) (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrGoogleVeoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrGoogleVeoEndpointRequired
	}
	template := strings.TrimSpace(p.GeneratePathTemplate)
	if template == "" {
		template = "/models/%s:generateVideos"
	}
	u.Path = path.Join(u.Path, fmt.Sprintf(template, p.resolveModel(model)))
	return u.String(), nil
}

func (p *GoogleVeoVideoProvider) taskURL(taskID string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", ErrGoogleVeoTaskIDRequired
	}
	if strings.HasPrefix(taskID, "http://") || strings.HasPrefix(taskID, "https://") {
		return taskID, nil
	}
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrGoogleVeoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrGoogleVeoEndpointRequired
	}
	if strings.HasPrefix(taskID, "operations/") {
		u.Path = path.Join(u.Path, taskID)
		return u.String(), nil
	}
	prefix := strings.TrimSpace(p.OperationsPathPrefix)
	if prefix == "" {
		prefix = "/operations"
	}
	u.Path = path.Join(u.Path, prefix, taskID)
	return u.String(), nil
}

func (p *GoogleVeoVideoProvider) SubmitClipTask(ctx context.Context, req models.VideoClipRequest) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrGoogleVeoAPIKeyRequired
	}
	u, err := p.generateURL(req.Model)
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
	httpReq.Header.Set("x-goog-api-key", p.APIKey)
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
		return nil, fmt.Errorf("google veo submit failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *GoogleVeoVideoProvider) PollTask(ctx context.Context, taskID string) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrGoogleVeoAPIKeyRequired
	}
	u, err := p.taskURL(taskID)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-goog-api-key", p.APIKey)
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
		return nil, fmt.Errorf("google veo poll failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *GoogleVeoVideoProvider) buildRequestBody(req models.VideoClipRequest) map[string]interface{} {
	body := map[string]interface{}{
		"prompt": strings.TrimSpace(req.Prompt),
	}
	if imageURL := strings.TrimSpace(req.ReferenceImageURL); imageURL != "" {
		body["image"] = map[string]interface{}{"uri": imageURL}
	}
	config := map[string]interface{}{}
	if duration := int(req.DurationSec); duration > 0 {
		config["durationSeconds"] = duration
	}
	if aspectRatio := p.resolveAspectRatio(req.Resolution); aspectRatio != "" {
		config["aspectRatio"] = aspectRatio
	}
	if len(config) > 0 {
		body["config"] = config
	}
	return body
}

func (p *GoogleVeoVideoProvider) resolveModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "veo-2"
	}
	if mapped, ok := p.ModelKeyMapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return strings.TrimSpace(mapped)
	}
	if model == "veo-2" {
		return "veo-2.0-generate-001"
	}
	return model
}

func (p *GoogleVeoVideoProvider) resolveAspectRatio(resolution string) string {
	resolution = strings.TrimSpace(resolution)
	if strings.Contains(resolution, ":") {
		return resolution
	}
	return strings.TrimSpace(p.DefaultAspectRatio)
}

func (p *GoogleVeoVideoProvider) parseTaskResponse(data []byte) (*models.VideoProviderTask, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	taskID := providerFirstString(raw, "name")
	if taskID == "" {
		return nil, errors.New("google veo missing operation name")
	}
	resultURL := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"response", "generatedVideos", "0", "video", "uri"}),
		providerNestedFirstString(raw, []string{"response", "generated_videos", "0", "video", "uri"}),
		providerNestedFirstString(raw, []string{"response", "videos", "0", "uri"}),
	)
	errMessage := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"error", "message"}),
		providerFirstString(raw, "message"),
	)
	providerStatus := "RUNNING"
	if done, ok := raw["done"].(bool); ok && done {
		if errMessage != "" {
			providerStatus = "FAILED"
		} else {
			providerStatus = "SUCCEEDED"
		}
	}
	return &models.VideoProviderTask{
		TaskID:         taskID,
		Status:         normalizeGoogleOperationStatus(raw),
		ResultURL:      resultURL,
		ErrorMessage:   errMessage,
		ProviderStatus: providerStatus,
		Raw:            raw,
	}, nil
}

func normalizeGoogleOperationStatus(raw map[string]interface{}) string {
	done, _ := raw["done"].(bool)
	if !done {
		return "running"
	}
	if _, hasErr := raw["error"]; hasErr {
		return "failed"
	}
	return "completed"
}
