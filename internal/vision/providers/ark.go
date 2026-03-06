// internal/vision/providers/ark.go
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
	ErrArkEndpointRequired = errors.New("ark endpoint required")
	ErrArkAPIKeyRequired   = errors.New("ark api key required")
)

// ArkImagesProvider implements a minimal Volcengine Ark image generation provider.
type ArkImagesProvider struct {
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

func NewArkImagesProvider(endpoint string, apiKey string) *ArkImagesProvider {
	return &ArkImagesProvider{
		Endpoint:       strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:         strings.TrimSpace(apiKey),
		GenerationPath: "/images/generations",
	}
}

func (p *ArkImagesProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *ArkImagesProvider) downloadClient() *http.Client {
	if p.DownloadTimeout <= 0 {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return &http.Client{Timeout: p.DownloadTimeout}
}

func (p *ArkImagesProvider) resolveModel(modelKey string) string {
	key := strings.TrimSpace(modelKey)
	if key == "" {
		key = "doubao-seedream-4.5"
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

func (p *ArkImagesProvider) resolveSize(modelKey string, w int, h int) string {
	key := strings.TrimSpace(modelKey)
	if key == "" {
		key = "doubao-seedream-4.5"
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

func (p *ArkImagesProvider) generateURL() (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrArkEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrArkEndpointRequired
	}
	gp := strings.TrimSpace(p.GenerationPath)
	if gp == "" {
		gp = "/images/generations"
	}
	u.Path = path.Join(u.Path, gp)
	return u.String(), nil
}

func (p *ArkImagesProvider) GenerateImage(ctx context.Context, prompt string, opts vision.VisionGenerateOptions) (*vision.VisionImage, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrArkAPIKeyRequired
	}
	genURL, err := p.generateURL()
	if err != nil {
		return nil, err
	}

	w, h := defaultImageDimensions(opts.Width, opts.Height, 1024, 1024)
	model := p.resolveModel(opts.Model)
	size := p.resolveSize(opts.Model, w, h)

	reqBody := map[string]interface{}{
		"model":     model,
		"prompt":    prompt,
		"watermark": false,
	}
	if size != "" {
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
		return nil, httpStatusError("ark", "request", resp.StatusCode, body)
	}

	var parsed struct {
		Data []struct {
			URL  string `json:"url"`
			Size string `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 || strings.TrimSpace(parsed.Data[0].URL) == "" {
		return nil, errors.New("ark empty image url")
	}
	imageURL := strings.TrimSpace(parsed.Data[0].URL)

	imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	imgResp, err := p.downloadClient().Do(imgReq)
	if err != nil {
		return nil, err
	}
	defer imgResp.Body.Close()
	imgBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, err
	}
	if imgResp.StatusCode < 200 || imgResp.StatusCode >= 300 {
		return nil, httpStatusError("ark", "image download", imgResp.StatusCode, imgBytes)
	}
	if len(imgBytes) == 0 {
		return nil, errors.New("ark empty image")
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
