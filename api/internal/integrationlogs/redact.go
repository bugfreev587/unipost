package integrationlogs

import (
	"encoding/json"
	"strings"
)

const (
	redactedValue      = "[REDACTED]"
	maxPayloadBytes    = 16 * 1024
	truncatedFieldName = "_truncated"
)

var sensitiveKeyFragments = []string{
	"token",
	"secret",
	"authorization",
	"cookie",
	"password",
	"refresh_token",
	"access_token",
	"client_secret",
}

func RedactJSON(v any) []byte {
	if v == nil {
		return nil
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return truncateBytes(raw)
	}

	redacted := redactValue(decoded)
	out, err := json.Marshal(redacted)
	if err != nil {
		return truncateBytes(raw)
	}
	return truncateBytes(out)
}

func redactValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			if isSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = redactValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = redactValue(value)
		}
		return out
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func truncateBytes(b []byte) []byte {
	if len(b) <= maxPayloadBytes {
		return b
	}
	trimmed := b[:maxPayloadBytes]
	wrapped, err := json.Marshal(map[string]any{
		"value":            string(trimmed),
		truncatedFieldName: true,
	})
	if err != nil {
		return trimmed
	}
	return wrapped
}
