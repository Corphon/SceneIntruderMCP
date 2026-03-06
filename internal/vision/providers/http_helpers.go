package providers

import (
	"fmt"
	"strings"
)

func defaultImageDimensions(width int, height int, defaultWidth int, defaultHeight int) (int, int) {
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	return width, height
}

func inferImageFormat(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return "jpg"
	case strings.Contains(ct, "webp"):
		return "webp"
	default:
		return "png"
	}
}

func httpStatusError(provider string, action string, status int, body []byte) error {
	msg := fmt.Sprintf("%s %s failed: status=%d", provider, action, status)
	detail := strings.Join(strings.Fields(strings.TrimSpace(string(body))), " ")
	if detail == "" {
		return fmt.Errorf("%s", msg)
	}
	if len(detail) > 240 {
		detail = detail[:240] + "..."
	}
	return fmt.Errorf("%s body=%s", msg, detail)
}
