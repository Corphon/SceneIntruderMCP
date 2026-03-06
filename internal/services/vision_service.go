// internal/services/vision_service.go
package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/utils"
	"github.com/Corphon/SceneIntruderMCP/internal/vision/providers"
)

var ErrVisionProviderNotFound = errors.New("vision provider not found")

// VisionService wraps one or more VisionProvider implementations
// and provides comic-specific persistence helpers.
type VisionService struct {
	Repo      *ComicRepository
	Stats     *StatsService
	Providers map[string]VisionProvider
	// ModelProviders maps a model key (e.g. "sd") to a provider name (e.g. "sdwebui").
	// If a model key is not present, DefaultProvider will be used.
	ModelProviders  map[string]string
	DefaultProvider string
	DefaultModel    string
	// MaxAttempts controls how many times GenerateAndSaveFrame will attempt image generation
	// before returning an error. Default is 1 (no retry).
	MaxAttempts int
	// PNGRecompressThresholdBytes enables best-effort lossless PNG recompression when
	// the generated image bytes are larger than this threshold.
	// Default is 256 KiB.
	// If set to a negative value, PNG recompression is disabled.
	PNGRecompressThresholdBytes int
}

func NewVisionService(repo *ComicRepository) *VisionService {
	providerMap := map[string]VisionProvider{
		"placeholder": providers.NewPlaceholderVisionProvider(),
	}
	return &VisionService{
		Repo:                        repo,
		Providers:                   providerMap,
		ModelProviders:              map[string]string{"placeholder": "placeholder"},
		DefaultProvider:             "placeholder",
		DefaultModel:                "placeholder",
		MaxAttempts:                 1,
		PNGRecompressThresholdBytes: 256 << 10,
	}
}

func (s *VisionService) pngRecompressThresholdBytes() int {
	if s.PNGRecompressThresholdBytes < 0 {
		return 0
	}
	if s.PNGRecompressThresholdBytes == 0 {
		return 256 << 10
	}
	// Cap to avoid accidental always-on recompress for tiny images.
	if s.PNGRecompressThresholdBytes < 4<<10 {
		return 4 << 10
	}
	return s.PNGRecompressThresholdBytes
}

func isPNGImage(img *VisionImage) bool {
	if img == nil {
		return false
	}
	if img.ContentType == "image/png" {
		return true
	}
	return img.Format == "png"
}

func recompressPNGBestEffort(pngBytes []byte) ([]byte, error) {
	decoded, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, decoded); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *VisionService) maxAttempts() int {
	if s.MaxAttempts <= 0 {
		return 1
	}
	// Keep a conservative upper bound to avoid accidental tight loops.
	if s.MaxAttempts > 3 {
		return 3
	}
	return s.MaxAttempts
}

func isRetryableVisionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrVisionProviderNotFound) {
		return false
	}
	return true
}

func (s *VisionService) RegisterProvider(name string, provider VisionProvider) {
	if s.Providers == nil {
		s.Providers = map[string]VisionProvider{}
	}
	s.Providers[name] = provider
}

func (s *VisionService) SetModelProvider(modelKey string, providerName string) {
	if s.ModelProviders == nil {
		s.ModelProviders = map[string]string{}
	}
	s.ModelProviders[modelKey] = providerName
}

func (s *VisionService) providerForModel(modelKey string) string {
	if modelKey == "" {
		return ""
	}
	if s.ModelProviders == nil {
		return ""
	}
	return s.ModelProviders[modelKey]
}

func (s *VisionService) GenerateImage(ctx context.Context, provider string, prompt string, opts VisionGenerateOptions) (*VisionImage, error) {
	if opts.Model == "" {
		opts.Model = s.DefaultModel
	}

	// Provider selection precedence:
	// 1) explicit provider argument
	// 2) modelKey -> provider mapping
	// 3) DefaultProvider
	if provider == "" {
		provider = s.providerForModel(opts.Model)
	}
	if provider == "" {
		provider = s.DefaultProvider
	}

	p, ok := s.Providers[provider]
	if !ok {
		if s.Stats != nil {
			_ = s.Stats.RecordVisionRequest(provider, opts.Model, 0, fmt.Errorf("%w: %s", ErrVisionProviderNotFound, provider))
		}
		m := utils.GetMetricsCollector()
		m.IncrementCounter("vision_requests_total")
		m.IncrementCounter("vision_requests_" + provider)
		m.IncrementCounter("vision_failures_total")
		m.IncrementCounter("vision_failures_" + provider)
		return nil, fmt.Errorf("%w: %s", ErrVisionProviderNotFound, provider)
	}

	start := time.Now()
	img, err := p.GenerateImage(ctx, prompt, opts)
	dur := time.Since(start)

	// Persistent stats (best-effort).
	if s.Stats != nil {
		_ = s.Stats.RecordVisionRequest(provider, opts.Model, dur, err)
	}

	// Runtime metrics (always available).
	m := utils.GetMetricsCollector()
	m.IncrementCounter("vision_requests_total")
	m.IncrementCounter("vision_requests_" + provider)
	m.RecordHistogram("vision_response_time_ms", dur.Milliseconds())
	if err != nil {
		m.IncrementCounter("vision_failures_total")
		m.IncrementCounter("vision_failures_" + provider)
	}

	return img, err
}

// GenerateAndSaveFrame generates an image and persists it to:
// data/comics/scene_<id>/images/<frameID>.png
func (s *VisionService) GenerateAndSaveFrame(ctx context.Context, sceneID string, frameID string, prompt string, opts VisionGenerateOptions) (relativePath string, img *VisionImage, err error) {
	if s.Repo == nil {
		return "", nil, ErrComicRepositoryNotReady
	}

	attempts := s.maxAttempts()
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return "", nil, err
			}
		}

		img, err = s.GenerateImage(ctx, "", prompt, opts)
		if err == nil {
			if img == nil || len(img.Data) == 0 {
				err = fmt.Errorf("empty image")
			} else {
				dataToSave := img.Data
				if isPNGImage(img) {
					if ctx != nil {
						if err := ctx.Err(); err != nil {
							return "", nil, err
						}
					}
					threshold := s.pngRecompressThresholdBytes()
					if threshold > 0 && len(dataToSave) >= threshold {
						if recompressed, rerr := recompressPNGBestEffort(dataToSave); rerr == nil {
							if len(recompressed) > 0 && len(recompressed) < len(dataToSave) {
								img.Data = recompressed
								dataToSave = recompressed
							}
						}
					}
				}

				if ctx != nil {
					if err := ctx.Err(); err != nil {
						return "", nil, err
					}
				}

				relativePath, err = s.Repo.SaveFrameImage(sceneID, frameID, dataToSave)
				if err == nil {
					return relativePath, img, nil
				}
			}
		}

		if attempt < attempts && isRetryableVisionError(err) {
			continue
		}
		return "", nil, err
	}

	return "", nil, err
}
