// internal/vision/providers/sdwebui.go
package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/vision"
)

var ErrSDWebUIEndpointRequired = errors.New("sdwebui endpoint required")

// SDWebUIProvider implements a minimal Stable Diffusion WebUI (AUTOMATIC1111) compatible provider.
//
// It supports:
// - text2img: POST {endpoint}/sdapi/v1/txt2img
// - img2img: POST {endpoint}/sdapi/v1/img2img (when opts.ReferenceImage is set)
type SDWebUIProvider struct {
	Endpoint string
	Client   *http.Client
}

func NewSDWebUIProvider(endpoint string) *SDWebUIProvider {
	return &SDWebUIProvider{Endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/")}
}

func (p *SDWebUIProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *SDWebUIProvider) GenerateImage(ctx context.Context, prompt string, opts vision.VisionGenerateOptions) (*vision.VisionImage, error) {
	if strings.TrimSpace(p.Endpoint) == "" {
		return nil, ErrSDWebUIEndpointRequired
	}

	w, h := defaultImageDimensions(opts.Width, opts.Height, 512, 512)

	txt2imgURL := p.Endpoint + "/sdapi/v1/txt2img"
	img2imgURL := p.Endpoint + "/sdapi/v1/img2img"

	useImg2Img := len(opts.ReferenceImage) > 0
	url := txt2imgURL
	if useImg2Img {
		url = img2imgURL
	}

	reqBody := map[string]interface{}{
		"prompt":          prompt,
		"negative_prompt": opts.NegativePrompt,
		"width":           w,
		"height":          h,
	}
	overrideSettings := map[string]interface{}{}
	if model := strings.TrimSpace(opts.Model); model != "" && model != "sd" && model != "sdwebui" {
		overrideSettings["sd_model_checkpoint"] = model
	}
	if opts.Steps > 0 {
		reqBody["steps"] = opts.Steps
	}
	if opts.CFGScale > 0 {
		reqBody["cfg_scale"] = opts.CFGScale
	}
	if strings.TrimSpace(opts.Sampler) != "" {
		reqBody["sampler_name"] = opts.Sampler
	}
	if opts.Seed != 0 {
		reqBody["seed"] = opts.Seed
	}
	if opts.Eta > 0 {
		reqBody["eta"] = opts.Eta
	}
	if opts.Tiling {
		reqBody["tiling"] = true
	}
	if opts.ClipSkip > 0 {
		overrideSettings["CLIP_stop_at_last_layers"] = opts.ClipSkip
	}
	if len(overrideSettings) > 0 {
		reqBody["override_settings"] = overrideSettings
	}

	if useImg2Img {
		encoded := base64.StdEncoding.EncodeToString(opts.ReferenceImage)
		reqBody["init_images"] = []string{encoded}
		if opts.DenoisingStrength > 0 {
			reqBody["denoising_strength"] = opts.DenoisingStrength
		}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

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
		return nil, httpStatusError("sdwebui", "request", resp.StatusCode, body)
	}

	var parsed struct {
		Images []string `json:"images"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Images) == 0 || strings.TrimSpace(parsed.Images[0]) == "" {
		return nil, errors.New("sdwebui empty images")
	}

	imgBytes, err := decodeBase64Image(parsed.Images[0])
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
