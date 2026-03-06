// internal/vision/providers/dashscope.go
package providers

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

	"github.com/Corphon/SceneIntruderMCP/internal/vision"
)

var (
	ErrDashScopeEndpointRequired = errors.New("dashscope endpoint required")
	ErrDashScopeAPIKeyRequired   = errors.New("dashscope api key required")
)

// DashScopeProvider implements a minimal DashScope multimodal image generation provider.
//
// It expects the response to include an image URL which it then downloads.
type DashScopeProvider struct {
	Endpoint        string
	APIKey          string
	GenerationPath  string
	Client          *http.Client
	DownloadTimeout time.Duration
}

func NewDashScopeProvider(endpoint string, apiKey string) *DashScopeProvider {
	return &DashScopeProvider{
		Endpoint:       strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:         strings.TrimSpace(apiKey),
		GenerationPath: "/services/aigc/multimodal-generation/generation",
	}
}

func (p *DashScopeProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *DashScopeProvider) downloadClient() *http.Client {
	if p.DownloadTimeout <= 0 {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return &http.Client{Timeout: p.DownloadTimeout}
}

func (p *DashScopeProvider) generateURL() (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrDashScopeEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrDashScopeEndpointRequired
	}
	gp := strings.TrimSpace(p.GenerationPath)
	if gp == "" {
		gp = "/services/aigc/multimodal-generation/generation"
	}
	u.Path = path.Join(u.Path, gp)
	return u.String(), nil
}

func (p *DashScopeProvider) GenerateImage(ctx context.Context, prompt string, opts vision.VisionGenerateOptions) (*vision.VisionImage, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrDashScopeAPIKeyRequired
	}
	genURL, err := p.generateURL()
	if err != nil {
		return nil, err
	}

	w := opts.Width
	h := opts.Height
	if w <= 0 {
		w = 1024
	}
	if h <= 0 {
		h = 1024
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "qwen-image-2.0"
	}

	requestBody := map[string]interface{}{
		"model": model,
		"input": map[string]interface{}{
			"messages": []interface{}{
				map[string]interface{}{
					"role":    "user",
					"content": []interface{}{map[string]interface{}{"text": prompt}},
				},
			},
		},
		"parameters": map[string]interface{}{
			"result_format": "message",
			"stream":        false,
			"watermark":     false,
			"prompt_extend": opts.PromptExtend,
			"size":          fmt.Sprintf("%d*%d", w, h),
		},
	}
	if strings.TrimSpace(opts.NegativePrompt) != "" {
		requestBody["parameters"].(map[string]interface{})["negative_prompt"] = opts.NegativePrompt
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, genURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope request failed: status=%d", resp.StatusCode)
	}

	var parsed struct {
		StatusCode int    `json:"status_code"`
		Code       string `json:"code"`
		Message    string `json:"message"`
		Output     struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.StatusCode != 0 && parsed.StatusCode != 200 {
		if strings.TrimSpace(parsed.Code) != "" {
			return nil, fmt.Errorf("dashscope error: %s", parsed.Code)
		}
		return nil, errors.New("dashscope error")
	}

	imageURL := ""
	if len(parsed.Output.Choices) > 0 {
		for _, c := range parsed.Output.Choices {
			for _, item := range c.Message.Content {
				if strings.TrimSpace(item.Image) != "" {
					imageURL = strings.TrimSpace(item.Image)
					break
				}
			}
			if imageURL != "" {
				break
			}
		}
	}
	if imageURL == "" {
		return nil, errors.New("dashscope empty image url")
	}

	imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	imgResp, err := p.downloadClient().Do(imgReq)
	if err != nil {
		return nil, err
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode < 200 || imgResp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope image download failed: status=%d", imgResp.StatusCode)
	}
	imgBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, err
	}
	if len(imgBytes) == 0 {
		return nil, errors.New("dashscope empty image")
	}

	contentType := strings.TrimSpace(imgResp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "image/png"
	}

	format := "png"
	if strings.Contains(contentType, "jpeg") {
		format = "jpg"
	}

	return &vision.VisionImage{
		Format:      format,
		ContentType: contentType,
		Data:        imgBytes,
		Width:       w,
		Height:      h,
	}, nil
}
