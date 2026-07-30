package requestevents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxPayloadBytes        = 32 * 1024
	maxContentTypeBytes    = 512
	maxRawQueryBytes       = 8 * 1024
	maxQueryPairs          = 64
	redactionPolicyVersion = 1
	redactedValue          = "[REDACTED]"
)

type Outcome string

const (
	OutcomeSuccess             Outcome = "success"
	OutcomeClientError         Outcome = "client_error"
	OutcomeValidationError     Outcome = "validation_error"
	OutcomeAuthenticationError Outcome = "authentication_error"
	OutcomePlanOrQuotaGate     Outcome = "plan_or_quota_gate"
	OutcomeAuthorizationError  Outcome = "authorization_error"
	OutcomeNotFound            Outcome = "not_found"
	OutcomeConflict            Outcome = "conflict"
	OutcomeRateLimited         Outcome = "rate_limited"
	OutcomeServerError         Outcome = "server_error"
)

func NormalizeOutcome(status int, errorCode string) Outcome {
	switch {
	case status >= 200 && status < 400:
		return OutcomeSuccess
	case status == http.StatusBadRequest:
		normalized := normalizeToken(errorCode)
		if strings.Contains(normalized, "validation") || strings.Contains(normalized, "invalid") {
			return OutcomeValidationError
		}
		return OutcomeClientError
	case status == http.StatusUnauthorized:
		return OutcomeAuthenticationError
	case status == http.StatusPaymentRequired:
		return OutcomePlanOrQuotaGate
	case status == http.StatusForbidden:
		return OutcomeAuthorizationError
	case status == http.StatusNotFound:
		return OutcomeNotFound
	case status == http.StatusConflict:
		return OutcomeConflict
	case status == http.StatusTooManyRequests:
		return OutcomeRateLimited
	case status >= 500:
		return OutcomeServerError
	default:
		return OutcomeClientError
	}
}

var (
	placeholderPattern          = regexp.MustCompile(`\{[^/{}]+\}`)
	traceIDPattern              = regexp.MustCompile(`(?i)^[0-9a-f]{16,32}$`)
	applicationErrorCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	httpMethodPattern           = regexp.MustCompile(`^[A-Z]{1,16}$`)
	numericHeaderPattern        = regexp.MustCompile(`^[0-9]{1,20}$`)
	bearerPattern               = regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/=-]+`)
	sensitiveTextMarkerPattern  = regexp.MustCompile(`(?i)(authorization|cookie|password|passphrase|secret|token|api[_-]?key|signature|credential)`)
)

func NormalizeRoute(routePattern, _ string) string {
	routePattern = strings.TrimSpace(routePattern)
	if routePattern != "" {
		routePattern = placeholderPattern.ReplaceAllString(routePattern, ":id")
		if strings.HasPrefix(routePattern, "/") {
			return truncateUTF8(routePattern, 256)
		}
	}
	// An unmatched route has no trustworthy schema. Persisting any raw path
	// segment can leak customer-controlled identifiers and create unbounded
	// cardinality, so it is deliberately collapsed to one fixed bucket.
	return "/unknown"
}

func normalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if !httpMethodPattern.MatchString(method) {
		return "OTHER"
	}
	return method
}

var safeHeaderNames = map[string]string{
	"content-length":        "Content-Length",
	"content-type":          "Content-Type",
	"retry-after":           "Retry-After",
	"x-ratelimit-limit":     "X-RateLimit-Limit",
	"x-ratelimit-remaining": "X-RateLimit-Remaining",
	"x-ratelimit-reset":     "X-RateLimit-Reset",
}

func allowlistedHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for _, canonical := range safeHeaderNames {
		values := headers.Values(canonical)
		if len(values) == 0 {
			continue
		}
		value, safe := sanitizeAllowlistedHeaderValue(canonical, values[0])
		if safe {
			result[canonical] = value
		}
	}
	return result
}

func sanitizeAllowlistedHeaderValue(name, value string) (string, bool) {
	value = strings.TrimSpace(strings.ToValidUTF8(value, "�"))
	if len(value) > maxContentTypeBytes {
		return "", false
	}
	switch name {
	case "Content-Type":
		value = normalizedMediaType(value)
		return value, value != ""
	case "Content-Length", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset":
		return value, numericHeaderPattern.MatchString(value)
	case "Retry-After":
		if numericHeaderPattern.MatchString(value) {
			return value, true
		}
		if parsed, err := http.ParseTime(value); err == nil {
			return parsed.UTC().Format(http.TimeFormat), true
		}
	}
	return "", false
}

func queryKeySummary(rawQuery string) []string {
	rawQuery = truncateUTF8(rawQuery, maxRawQueryBytes)
	seen := make(map[string]struct{})
	pairs := strings.Split(rawQuery, "&")
	if len(pairs) > maxQueryPairs {
		pairs = pairs[:maxQueryPairs]
	}
	for _, pair := range pairs {
		key, _, _ := strings.Cut(pair, "=")
		key, err := url.QueryUnescape(key)
		if err != nil {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if _, allowed := safeQueryKeys[key]; allowed {
			seen[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var safeQueryKeys = map[string]struct{}{
	"cursor":   {},
	"days":     {},
	"end":      {},
	"from":     {},
	"limit":    {},
	"method":   {},
	"order":    {},
	"page":     {},
	"platform": {},
	"route":    {},
	"sort":     {},
	"start":    {},
	"status":   {},
	"to":       {},
}

type PayloadCapture struct {
	Payload        string
	OriginalBytes  int64
	StoredBytes    int
	SHA256         string
	Truncated      bool
	OmissionReason string
}

func sanitizePayload(contentType string, captured []byte, originalBytes int64) PayloadCapture {
	result := PayloadCapture{
		OriginalBytes: originalBytes,
		Truncated:     originalBytes > int64(len(captured)),
	}
	mediaType := normalizedMediaType(contentType)
	if isBinaryOrMultipart(mediaType) {
		result.OmissionReason = "binary_or_multipart"
		if originalBytes == int64(len(captured)) {
			result.SHA256 = sha256Hex(captured)
		} else {
			result.OmissionReason = appendOmissionReason(result.OmissionReason, "hash_omitted_size")
		}
		return result
	}

	var sanitized string
	switch {
	case isJSONMediaType(mediaType):
		var value any
		if err := json.Unmarshal(captured, &value); err != nil {
			result.OmissionReason = "invalid_json"
			return result
		}
		encoded, err := json.Marshal(redactJSON(value))
		if err != nil {
			result.OmissionReason = "redaction_failed"
			return result
		}
		sanitized = string(encoded)
	case isAllowedTextMediaType(mediaType):
		sanitized = sanitizeText(string(captured))
	default:
		result.OmissionReason = "unsupported_content_type"
		return result
	}

	sanitized = strings.ToValidUTF8(sanitized, "�")
	sanitized = strings.ReplaceAll(sanitized, "\x00", "")
	bounded := truncateUTF8(sanitized, maxPayloadBytes)
	if bounded != sanitized {
		result.Truncated = true
	}
	result.Payload = bounded
	result.StoredBytes = len(bounded)
	return result
}

func normalizedMediaType(contentType string) string {
	contentType = truncateUTF8(strings.ToValidUTF8(contentType, "�"), maxContentTypeBytes)
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	mediaType = strings.ToLower(mediaType)
	switch {
	case isJSONMediaType(mediaType):
		return "application/json"
	case mediaType == "text/plain", mediaType == "text/html":
		return mediaType
	case mediaType == "multipart/form-data", mediaType == "multipart/mixed":
		return mediaType
	case strings.HasPrefix(mediaType, "image/"):
		return "image/*"
	case strings.HasPrefix(mediaType, "video/"):
		return "video/*"
	case strings.HasPrefix(mediaType, "audio/"):
		return "audio/*"
	case mediaType == "application/octet-stream", mediaType == "application/zip", mediaType == "application/pdf", mediaType == "application/x-www-form-urlencoded":
		return mediaType
	default:
		return ""
	}
}

func isJSONMediaType(mediaType string) bool {
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func isAllowedTextMediaType(mediaType string) bool {
	return mediaType == "text/plain"
}

func isBinaryOrMultipart(mediaType string) bool {
	return strings.HasPrefix(mediaType, "multipart/") || strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "video/") || strings.HasPrefix(mediaType, "audio/") ||
		mediaType == "application/octet-stream" || mediaType == "application/zip" || mediaType == "application/pdf"
}

func redactJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if isSensitiveKey(key) {
				result[key] = redactedValue
			} else {
				result[key] = redactJSON(child)
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactJSON(child)
		}
		return result
	case string:
		return sanitizeText(typed)
	default:
		return typed
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	normalized = strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(normalized)
	for _, token := range []string{
		"authorization", "cookie", "password", "passphrase", "secret", "token", "apikey", "signature", "credential",
		"providererror", "rawerror", "upstreamresponse", "message", "error", "detail", "reason",
	} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func sanitizeText(value string) string {
	if sensitiveTextMarkerPattern.MatchString(value) {
		return redactedValue
	}
	return bearerPattern.ReplaceAllString(value, redactedValue)
}

func appendOmissionReason(existing, reason string) string {
	if existing == "" {
		return reason
	}
	if strings.Contains(existing, reason) {
		return existing
	}
	return existing + "," + reason
}

func removeOmissionReason(existing, reason string) string {
	parts := strings.Split(existing, ",")
	kept := parts[:0]
	for _, part := range parts {
		if part != "" && part != reason {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ",")
}

func sha256Hex(value []byte) string {
	if len(value) == 0 {
		return ""
	}
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func truncateUTF8(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.NewReplacer(" ", "_", "-", "_", ".", "_").Replace(value)
}
