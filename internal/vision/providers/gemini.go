// internal/vision/providers/gemini.go
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/vision"
)

var (
	ErrGeminiEndpointRequired = errors.New("gemini endpoint required")
	ErrGeminiAPIKeyRequired   = errors.New("gemini api key required")
)

// GeminiProvider implements a minimal Google Gemini image generation provider.
type GeminiProvider struct {
	Endpoint string
	APIKey   string

	ModelOverride     string
	ModelKeyOverrides map[string]string

	// AuthMode:
	// - "x-goog-api-key" (default): X-Goog-Api-Key header
	// - "query": ?key=... on request URL
	// - "bearer": Authorization: Bearer ...
	AuthMode string

	Client *http.Client
}

func NewGeminiProvider(endpoint string, apiKey string) *GeminiProvider {
	return &GeminiProvider{
		Endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:   strings.TrimSpace(apiKey),
		AuthMode: "x-goog-api-key",
	}
}

func (p *GeminiProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *GeminiProvider) resolveModel(modelKey string) string {
	key := strings.TrimSpace(modelKey)
	if key == "" {
		key = "nano-banana-pro"
	}
	if p.ModelKeyOverrides != nil {
		if v := strings.TrimSpace(p.ModelKeyOverrides[key]); v != "" {
			return v
		}
	}
	if strings.TrimSpace(p.ModelOverride) != "" {
		return strings.TrimSpace(p.ModelOverride)
	}
	return key
}

func (p *GeminiProvider) generateURL(model string) (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrGeminiEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrGeminiEndpointRequired
	}
	u.Path = path.Join(u.Path, "models", model+":generateContent")
	return u.String(), nil
}

func (p *GeminiProvider) applyAuth(req *http.Request) {
	mode := strings.ToLower(strings.TrimSpace(p.AuthMode))
	switch mode {
	case "query":
		q := req.URL.Query()
		q.Set("key", p.APIKey)
		req.URL.RawQuery = q.Encode()
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	default:
		req.Header.Set("X-Goog-Api-Key", p.APIKey)
	}
}

func (p *GeminiProvider) GenerateImage(ctx context.Context, prompt string, opts vision.VisionGenerateOptions) (*vision.VisionImage, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrGeminiAPIKeyRequired
	}
	model := p.resolveModel(opts.Model)
	genURL, err := p.generateURL(model)
	if err != nil {
		return nil, err
	}

	w, h := defaultImageDimensions(opts.Width, opts.Height, 1024, 1024)

	requestBody := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"role":  "user",
				"parts": []interface{}{map[string]interface{}{"text": prompt}},
			},
		},
		"generationConfig": map[string]interface{}{
			"candidateCount":     1,
			"responseModalities": []interface{}{"IMAGE"},
			"imageSize":          map[string]interface{}{"width": w, "height": h},
		},
	}
	if strings.TrimSpace(opts.NegativePrompt) != "" {
		requestBody["generationConfig"].(map[string]interface{})["negativePrompt"] = opts.NegativePrompt
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
	p.applyAuth(req)

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
		return nil, httpStatusError("gemini", "request", resp.StatusCode, body)
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
					InlineDataAlt *struct {
						MimeType string `json:"mime_type"`
						Data     string `json:"data"`
					} `json:"inline_data"`
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	mimeType := ""
	dataB64 := ""
	for _, c := range parsed.Candidates {
		for _, part := range c.Content.Parts {
			if part.InlineData != nil && strings.TrimSpace(part.InlineData.Data) != "" {
				mimeType = strings.TrimSpace(part.InlineData.MimeType)
				dataB64 = strings.TrimSpace(part.InlineData.Data)
				break
			}
			if part.InlineDataAlt != nil && strings.TrimSpace(part.InlineDataAlt.Data) != "" {
				mimeType = strings.TrimSpace(part.InlineDataAlt.MimeType)
				dataB64 = strings.TrimSpace(part.InlineDataAlt.Data)
				break
			}
		}
		if dataB64 != "" {
			break
		}
	}
	if dataB64 == "" {
		return nil, errors.New("gemini empty image")
	}

	imgBytes, err := decodeBase64Image(dataB64)
	if err != nil {
		return nil, err
	}

	if mimeType == "" {
		mimeType = "image/png"
	}

	return &vision.VisionImage{
		Format:      inferImageFormat(mimeType),
		ContentType: mimeType,
		Data:        imgBytes,
		Width:       w,
		Height:      h,
	}, nil
}
