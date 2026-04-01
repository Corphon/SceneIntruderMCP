// internal/services/video_provider_helpers.go
package services

import "strings"

func providerFirstString(raw map[string]interface{}, key string) string {
	v, ok := raw[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func providerNestedFirstString(raw map[string]interface{}, items []string) string {
	var cur interface{} = raw
	for _, item := range items {
		switch node := cur.(type) {
		case map[string]interface{}:
			cur = node[item]
		case []interface{}:
			if item != "0" || len(node) == 0 {
				return ""
			}
			cur = node[0]
		default:
			return ""
		}
	}
	s, _ := cur.(string)
	return strings.TrimSpace(s)
}

func providerFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
