// internal/vision/types.go
package vision

import "context"

// VisionProvider defines an image generation backend.
//
// Note: provider implementations should avoid embedding secrets in errors/logs.
type VisionProvider interface {
	GenerateImage(ctx context.Context, prompt string, opts VisionGenerateOptions) (*VisionImage, error)
}

type VisionGenerateOptions struct {
	Model  string
	Width  int
	Height int

	// Optional, provider-specific fields.
	NegativePrompt string
	Steps          int
	CFGScale       float64
	Sampler        string
	Seed           int64
	ClipSkip       int
	Eta            float64
	Tiling         bool
	Stylize        int
	Chaos          int
	PromptExtend   bool

	// ReferenceImage enables img2img for providers that support it.
	// Providers should treat empty bytes as “no reference”.
	ReferenceImage []byte
	// DenoisingStrength is commonly used by img2img backends (0..1).
	DenoisingStrength float64
}

type VisionImage struct {
	Format      string // e.g. "png"
	ContentType string // e.g. "image/png"
	Data        []byte
	Width       int
	Height      int
}
