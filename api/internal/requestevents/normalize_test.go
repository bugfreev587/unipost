package requestevents

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeOutcome(t *testing.T) {
	tests := []struct {
		status    int
		errorCode string
		want      Outcome
	}{
		{200, "", OutcomeSuccess},
		{302, "", OutcomeSuccess},
		{400, "VALIDATION_FAILED", OutcomeValidationError},
		{400, "bad_request", OutcomeClientError},
		{401, "", OutcomeAuthenticationError},
		{402, "PLAN_FEATURE_NOT_AVAILABLE", OutcomePlanOrQuotaGate},
		{403, "", OutcomeAuthorizationError},
		{404, "", OutcomeNotFound},
		{409, "", OutcomeConflict},
		{429, "", OutcomeRateLimited},
		{503, "", OutcomeServerError},
	}
	for _, tc := range tests {
		if got := NormalizeOutcome(tc.status, tc.errorCode); got != tc.want {
			t.Errorf("NormalizeOutcome(%d, %q) = %q, want %q", tc.status, tc.errorCode, got, tc.want)
		}
	}
}

func TestNormalizeRouteUsesPatternAndNeverStoresIdentifiersOrQuery(t *testing.T) {
	tests := []struct {
		pattern string
		raw     string
		want    string
	}{
		{"/v1/posts/{postID}/publish", "/v1/posts/42/publish?token=secret", "/v1/posts/:id/publish"},
		{"", "/v1/posts/4fe26fc0-5f25-43a6-8544-6ce32dc2dc5c?signature=secret", "/unknown"},
		{"", "/v1/abcdefghijklmnop", "/unknown"},
	}
	for _, tc := range tests {
		if got := NormalizeRoute(tc.pattern, tc.raw); got != tc.want {
			t.Errorf("NormalizeRoute(%q, %q) = %q, want %q", tc.pattern, tc.raw, got, tc.want)
		}
	}
}

func TestNormalizeMethodRejectsUnboundedCustomerControlledValues(t *testing.T) {
	if got := normalizeMethod("post"); got != "POST" {
		t.Fatalf("post normalized to %q", got)
	}
	if got := normalizeMethod(strings.Repeat("X", 200)); got != "OTHER" {
		t.Fatalf("oversized method normalized to %q", got)
	}
}

func TestAllowlistedHeadersExcludeCredentialsAndBoundValues(t *testing.T) {
	headers := http.Header{
		"Content-Type":      {"application/json; profile=opaque-content-type-secret"},
		"Accept":            {"application/json"},
		"User-Agent":        {"agent Bearer user-agent-secret " + strings.Repeat("a", 1500)},
		"Retry-After":       {"30"},
		"X-RateLimit-Reset": {"opaque-header-secret"},
		"X-Request-Id":      {"request-1"},
		"Authorization":     {"Bearer should-never-appear"},
		"Cookie":            {"session=should-never-appear"},
		"X-Api-Key":         {"should-never-appear"},
		"X-Hub-Signature":   {"should-never-appear"},
	}

	got := allowlistedHeaders(headers)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, secret := range []string{"Bearer", "user-agent-secret", "opaque-header-secret", "opaque-content-type-secret", "session=", "X-Api-Key", "X-Hub-Signature", "should-never-appear"} {
		if strings.Contains(text, secret) {
			t.Fatalf("allowlisted headers leaked %q: %s", secret, text)
		}
	}
	if got["Content-Type"] != "application/json" || got["Retry-After"] != "30" {
		t.Fatalf("safe headers missing: %#v", got)
	}
	if _, stored := got["User-Agent"]; stored {
		t.Fatalf("user-controlled user agent was stored: %#v", got)
	}
}

func TestQuerySummaryStoresKeysOnly(t *testing.T) {
	got := queryKeySummary("cursor=customer-secret&signature=signed-secret&page=2&cursor=duplicate")
	if strings.Join(got, ",") != "cursor,page" {
		t.Fatalf("query keys = %#v", got)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "customer-secret") || strings.Contains(string(encoded), "signed-secret") {
		t.Fatalf("query values leaked: %s", encoded)
	}
}

func TestSanitizeJSONPayloadRecursivelyRedactsSecrets(t *testing.T) {
	raw := []byte(`{
		"title":"safe",
		"password":"p1",
		"nested":{"access_token":"t1","clientSecret":"s1","message":"ok","provider_error":"opaque-provider-secret"},
		"items":[{"authorization":"Bearer abc","value":1}]
	}`)
	got := sanitizePayload("application/json", raw, int64(len(raw)))
	if got.OmissionReason != "" || got.StoredBytes == 0 {
		t.Fatalf("unexpected JSON capture: %#v", got)
	}
	for _, secret := range []string{"p1", "t1", "s1", "Bearer abc", "opaque-provider-secret"} {
		if strings.Contains(got.Payload, secret) {
			t.Fatalf("redacted payload leaked %q: %s", secret, got.Payload)
		}
	}
	if !strings.Contains(got.Payload, `"title":"safe"`) || !strings.Contains(got.Payload, redactedValue) {
		t.Fatalf("safe/redacted fields missing: %s", got.Payload)
	}
}

func TestSanitizePayloadOmitsBinaryAndMultipartBodies(t *testing.T) {
	for _, contentType := range []string{"video/mp4", "application/octet-stream", "multipart/form-data; boundary=x"} {
		raw := []byte("binary-secret-bytes")
		got := sanitizePayload(contentType, raw, int64(len(raw)))
		if got.Payload != "" || got.StoredBytes != 0 || got.SHA256 == "" || got.OmissionReason == "" {
			t.Fatalf("%s capture = %#v", contentType, got)
		}
	}
}

func TestSanitizePayloadBoundsTextAndMarksTruncation(t *testing.T) {
	raw := []byte(strings.Repeat("x", maxPayloadBytes+100))
	got := sanitizePayload("text/plain", raw, int64(len(raw)))
	if got.StoredBytes > maxPayloadBytes || !got.Truncated || got.OriginalBytes != int64(len(raw)) {
		t.Fatalf("bounded text capture = %#v", got)
	}
}

func TestSanitizeTextPayloadRedactsEntirePayloadWhenCredentialMarkersAppear(t *testing.T) {
	for _, raw := range []string{
		`upstream said password\":\"opaque-value\"`,
		`Authorization: Bearer opaque-token`,
		`cookie=session-value`,
		`access_token nearby opaque-value`,
		`credential response contained private-value`,
	} {
		got := sanitizePayload("text/plain", []byte(raw), int64(len(raw)))
		if got.Payload != redactedValue {
			t.Fatalf("sanitizePayload(%q) = %#v", raw, got)
		}
	}
}

func TestSanitizePayloadAllowsOnlyExplicitTextContentTypes(t *testing.T) {
	got := sanitizePayload("text/html", []byte(`<p>provider output</p>`), int64(len(`<p>provider output</p>`)))
	if got.Payload != "" || got.StoredBytes != 0 || got.OmissionReason != "unsupported_content_type" {
		t.Fatalf("text/html capture = %#v", got)
	}
}

func TestContentTypeMetadataCannotStoreArbitraryCustomerText(t *testing.T) {
	if got := normalizedMediaType("application/opaque-customer-secret"); got != "" {
		t.Fatalf("arbitrary content type normalized to %q", got)
	}
	if got := normalizedMediaType("video/mp4; profile=opaque-secret"); got != "video/*" {
		t.Fatalf("video content type normalized to %q", got)
	}
}
