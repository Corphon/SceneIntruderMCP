// internal/services/video_provider_kling.go
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
	"strconv"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

var (
	ErrKlingVideoEndpointRequired = errors.New("kling video endpoint required")
	ErrKlingVideoAPIKeyRequired   = errors.New("kling video api key required")
	ErrKlingVideoTaskIDRequired   = errors.New("kling video task id required")
)

type KlingVideoProvider struct {
	Endpoint           string
	APIKey             string
	ImageToVideoPath   string
	Client             *http.Client
	PollEvery          time.Duration
	PollAttempts       int
	ModelKeyMapping    map[string]string
	DefaultMode        string
	DefaultAspectRatio string
}

func NewKlingVideoProvider(endpoint string, apiKey string) *KlingVideoProvider {
	return &KlingVideoProvider{
		Endpoint:           strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:             strings.TrimSpace(apiKey),
		ImageToVideoPath:   "/videos/image2video",
		PollEvery:          2 * time.Second,
		PollAttempts:       90,
		ModelKeyMapping:    map[string]string{},
		DefaultMode:        "pro",
		DefaultAspectRatio: "16:9",
	}
}

func (p *KlingVideoProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *KlingVideoProvider) submitURL() (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrKlingVideoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrKlingVideoEndpointRequired
	}
	pathValue := strings.TrimSpace(p.ImageToVideoPath)
	if pathValue == "" {
		pathValue = "/videos/image2video"
	}
	u.Path = path.Join(u.Path, pathValue)
	return u.String(), nil
}

func (p *KlingVideoProvider) taskURL(taskID string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", ErrKlingVideoTaskIDRequired
	}
	submitURL, err := p.submitURL()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(submitURL)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, taskID)
	return u.String(), nil
}

func (p *KlingVideoProvider) SubmitClipTask(ctx context.Context, req models.VideoClipRequest) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrKlingVideoAPIKeyRequired
	}
	u, err := p.submitURL()
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
		return nil, fmt.Errorf("kling video submit failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *KlingVideoProvider) PollTask(ctx context.Context, taskID string) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrKlingVideoAPIKeyRequired
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
		return nil, fmt.Errorf("kling video poll failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *KlingVideoProvider) buildRequestBody(req models.VideoClipRequest) map[string]interface{} {
	body := map[string]interface{}{
		"model_name":      p.resolveModel(req.Model),
		"prompt":          strings.TrimSpace(req.Prompt),
		"negative_prompt": strings.TrimSpace(req.NegativePrompt),
		"mode":            p.resolveMode(),
		"duration":        strconv.Itoa(klingDuration(req.DurationSec)),
		"sound":           klingSound(req.AudioEnabled),
	}
	if image := strings.TrimSpace(req.ReferenceImageURL); image != "" {
		body["image"] = image
	}
	if aspectRatio := p.resolveAspectRatio(req.Resolution); aspectRatio != "" {
		body["aspect_ratio"] = aspectRatio
	}
	return body
}

func (p *KlingVideoProvider) resolveModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "kling-v3"
	}
	if mapped, ok := p.ModelKeyMapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return strings.TrimSpace(mapped)
	}
	return model
}

func (p *KlingVideoProvider) resolveMode() string {
	mode := strings.TrimSpace(p.DefaultMode)
	if mode == "" {
		return "pro"
	}
	return mode
}

func (p *KlingVideoProvider) resolveAspectRatio(resolution string) string {
	resolution = strings.TrimSpace(resolution)
	if strings.Contains(resolution, ":") {
		return resolution
	}
	return strings.TrimSpace(p.DefaultAspectRatio)
}

func klingDuration(durationSec float64) int {
	duration := int(durationSec)
	if duration <= 0 {
		return 5
	}
	return duration
}

func klingSound(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func (p *KlingVideoProvider) parseTaskResponse(data []byte) (*models.VideoProviderTask, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	taskID := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"data", "task_id"}),
		providerFirstString(raw, "task_id"),
	)
	if taskID == "" {
		return nil, errors.New("kling video missing task id")
	}
	providerStatus := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"data", "task_status"}),
		providerFirstString(raw, "task_status"),
	)
	resultURL := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"data", "task_result", "videos", "0", "url"}),
		providerNestedFirstString(raw, []string{"data", "videos", "0", "url"}),
	)
	errMessage := providerFirstNonEmpty(
		providerNestedFirstString(raw, []string{"data", "task_status_msg"}),
		providerFirstString(raw, "message"),
	)
	return &models.VideoProviderTask{
		TaskID:         taskID,
		Status:         normalizeKlingTaskStatus(providerStatus),
		ResultURL:      resultURL,
		ErrorMessage:   errMessage,
		ProviderStatus: providerStatus,
		Raw:            raw,
	}, nil
}

func normalizeKlingTaskStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "submitted", "processing":
		return "running"
	case "succeed", "succeeded", "completed":
		return "completed"
	case "failed", "error", "cancelled", "canceled":
		return "failed"
	default:
		return status
	}
}
