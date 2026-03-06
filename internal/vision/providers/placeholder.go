// internal/vision/providers/placeholder.go
package providers

import (
	"bytes"
	"context"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"

	"github.com/Corphon/SceneIntruderMCP/internal/vision"
)

// PlaceholderVisionProvider generates a deterministic PNG locally.
// It is useful for Phase1 end-to-end flow and tests without external credentials.
type PlaceholderVisionProvider struct{}

func NewPlaceholderVisionProvider() *PlaceholderVisionProvider {
	return &PlaceholderVisionProvider{}
}

func (p *PlaceholderVisionProvider) GenerateImage(ctx context.Context, prompt string, opts vision.VisionGenerateOptions) (*vision.VisionImage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	w := opts.Width
	h := opts.Height
	if w <= 0 {
		w = 256
	}
	if h <= 0 {
		h = 256
	}

	base := promptColor(prompt)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// simple gradient based on position to avoid a flat color
			r := uint8(int(base.R) ^ (x * 13) ^ (y * 7))
			g := uint8(int(base.G) ^ (x * 5) ^ (y * 11))
			b := uint8(int(base.B) ^ (x * 3) ^ (y * 17))
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return &vision.VisionImage{
		Format:      "png",
		ContentType: "image/png",
		Data:        buf.Bytes(),
		Width:       w,
		Height:      h,
	}, nil
}

func promptColor(prompt string) color.RGBA {
	h := fnv.New32a()
	_, _ = h.Write([]byte(prompt))
	sum := h.Sum32()
	return color.RGBA{
		R: uint8(sum & 0xFF),
		G: uint8((sum >> 8) & 0xFF),
		B: uint8((sum >> 16) & 0xFF),
		A: 255,
	}
}
