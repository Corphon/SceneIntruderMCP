// internal/vision/providers/openai.go
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
	ErrOpenAIEndpointRequired = errors.New("openai endpoint required")
	ErrOpenAIAPIKeyRequired   = errors.New("openai api key required")
)

// OpenAIImagesProvider implements OpenAI Images API (generations).
type OpenAIImagesProvider struct {
	Endpoint       string
	APIKey         string
	GenerationPath string

	ModelOverride     string
	ModelKeyOverrides map[string]string

	SizeOverride     string
	SizeKeyOverrides map[string]string

	Client          *http.Client
	DownloadTimeout time.Duration
}

func NewOpenAIImagesProvider(endpoint string, apiKey string) *OpenAIImagesProvider {
	return &OpenAIImagesProvider{
		Endpoint:       strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:         strings.TrimSpace(apiKey),
		GenerationPath: "/images/generations",
	}
}

func (p *OpenAIImagesProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *OpenAIImagesProvider) downloadClient() *http.Client {
	if p.DownloadTimeout <= 0 {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return &http.Client{Timeout: p.DownloadTimeout}
}

func (p *OpenAIImagesProvider) resolveModel(modelKey string) string {
	key := strings.TrimSpace(modelKey)
	if key == "" {
		key = "gpt-image-1.5"
	}
	if p.ModelKeyOverrides != nil {
		if v := strings.TrimSpace(p.ModelKeyOverrides[key]); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(p.ModelOverride); v != "" {
		return v
	}
	return key
}

func (p *OpenAIImagesProvider) resolveSize(modelKey string, w int, h int) string {
	key := strings.TrimSpace(modelKey)
	if key == "" {
		key = "gpt-image-1.5"
	}
	if p.SizeKeyOverrides != nil {
		if v := strings.TrimSpace(p.SizeKeyOverrides[key]); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(p.SizeOverride); v != "" {
		return v
	}
	if w > 0 && h > 0 {
		return fmt.Sprintf("%dx%d", w, h)
	}
	return ""
}

func (p *OpenAIImagesProvider) generateURL() (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrOpenAIEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrOpenAIEndpointRequired
	}
	gp := strings.TrimSpace(p.GenerationPath)
	if gp == "" {
		gp = "/images/generations"
	}
	u.Path = path.Join(u.Path, gp)
	return u.String(), nil
}

func (p *OpenAIImagesProvider) GenerateImage(ctx context.Context, prompt string, opts vision.VisionGenerateOptions) (*vision.VisionImage, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrOpenAIAPIKeyRequired
	}
	genURL, err := p.generateURL()
	if err != nil {
		return nil, err
	}

	model := p.resolveModel(opts.Model)
	w, h := defaultImageDimensions(opts.Width, opts.Height, 1024, 1024)
	size := p.resolveSize(opts.Model, w, h)

	reqBody := map[string]interface{}{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
	}
	if strings.TrimSpace(size) != "" {
		reqBody["size"] = size
	}

	payload, err := json.Marshal(reqBody)
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
		return nil, httpStatusError("openai", "request", resp.StatusCode, body)
	}

	var parsed struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("openai empty data")
	}

	if b64 := strings.TrimSpace(parsed.Data[0].B64JSON); b64 != "" {
		imgBytes, err := decodeBase64Image(b64)
		if err != nil {
			return nil, err
		}
		return &vision.VisionImage{
			Format:      "png",
			ContentType: "image/png",
			Data:        imgBytes,
			Width:       w,
			Height:      h,
		}, nil
	}

	imageURL := strings.TrimSpace(parsed.Data[0].URL)
	if imageURL == "" {
		return nil, errors.New("openai empty image")
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
	imgBody, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, err
	}
	if imgResp.StatusCode < 200 || imgResp.StatusCode >= 300 {
		return nil, httpStatusError("openai", "image download", imgResp.StatusCode, imgBody)
	}
	imgBytes := imgBody
	if len(imgBytes) == 0 {
		return nil, errors.New("openai decoded empty image")
	}
	contentType := strings.TrimSpace(imgResp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "image/png"
	}
	return &vision.VisionImage{
		Format:      inferImageFormat(contentType),
		ContentType: contentType,
		Data:        imgBytes,
		Width:       w,
		Height:      h,
	}, nil
}
