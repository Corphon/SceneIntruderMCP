// internal/vision/providers/base64_image.go
package providers

import (
	"encoding/base64"
	"errors"
	"strings"
)

func decodeBase64Image(s string) ([]byte, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, errors.New("empty base64 image")
	}
	if i := strings.Index(trimmed, ","); i > 0 && strings.Contains(trimmed[:i], "base64") {
		trimmed = trimmed[i+1:]
	}
	trimmed = strings.TrimSpace(trimmed)
	b, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("decoded empty image")
	}
	return b, nil
}
