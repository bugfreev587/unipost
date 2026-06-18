package integrationlogs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactJSON_RedactsNestedSensitiveKeys(t *testing.T) {
	raw := RedactJSON(map[string]any{
		"access_token": "abc",
		"nested": map[string]any{
			"client_secret": "def",
			"safe":          "ok",
		},
	})

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal redacted json: %v", err)
	}

	if decoded["access_token"] != redactedValue {
		t.Fatalf("expected access_token to be redacted, got %#v", decoded["access_token"])
	}
	nested, ok := decoded["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested object, got %#v", decoded["nested"])
	}
	if nested["client_secret"] != redactedValue {
		t.Fatalf("expected client_secret to be redacted, got %#v", nested["client_secret"])
	}
	if nested["safe"] != "ok" {
		t.Fatalf("expected safe key to remain, got %#v", nested["safe"])
	}
}

func TestRedactJSON_TruncatesLargePayloads(t *testing.T) {
	raw := RedactJSON(map[string]any{
		"body": strings.Repeat("a", maxPayloadBytes+1024),
	})
	if len(raw) == 0 {
		t.Fatal("expected redacted payload bytes")
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal truncated json: %v", err)
	}
	if decoded[truncatedFieldName] != true {
		t.Fatalf("expected truncated marker, got %#v", decoded[truncatedFieldName])
	}
}

// TestRedactJSON_PublicDocKeyFragments guards every sensitive key
// fragment that public docs promise to redact. If a fragment is
// removed from sensitiveKeyFragments, the corresponding case fails so
// the docs and code cannot drift apart.
func TestRedactJSON_PublicDocKeyFragments(t *testing.T) {
	fragments := []string{
		"token",
		"secret",
		"authorization",
		"cookie",
		"password",
		"refresh_token",
		"access_token",
		"client_secret",
	}

	for _, fragment := range fragments {
		t.Run(fragment, func(t *testing.T) {
			// Exact key and a key that merely contains the fragment
			// must both be redacted (substring match).
			for _, key := range []string{fragment, "x_" + fragment + "_y"} {
				raw := RedactJSON(map[string]any{key: "sensitive-value"})
				var decoded map[string]any
				if err := json.Unmarshal(raw, &decoded); err != nil {
					t.Fatalf("unmarshal redacted json: %v", err)
				}
				if decoded[key] != redactedValue {
					t.Fatalf("key %q not redacted: got %#v", key, decoded[key])
				}
			}
		})
	}
}

func TestRedactJSON_InvalidJSONInputFallsBack(t *testing.T) {
	type invalid struct {
		Ch chan int `json:"ch"`
	}
	if got := RedactJSON(invalid{Ch: make(chan int)}); got != nil {
		t.Fatalf("expected nil for unmarshalable payload, got %q", string(got))
	}
}
