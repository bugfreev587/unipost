package postfailures

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

const (
	ErrorSourceUnipost  = "unipost"
	ErrorSourcePlatform = "platform"
	ErrorSourceWorker   = "worker"
	ErrorSourceUnknown  = "unknown"

	ErrorTemporalityTemporary = "temporary"
	ErrorTemporalityPermanent = "permanent"
	ErrorTemporalityUnknown   = "unknown"
)

type ProviderError struct {
	Provider      string `json:"provider,omitempty"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	Code          string `json:"code,omitempty"`
	Subcode       string `json:"subcode,omitempty"`
	Type          string `json:"type,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Domain        string `json:"domain,omitempty"`
	QuotaLimit    string `json:"quota_limit,omitempty"`
	QuotaLocation string `json:"quota_location,omitempty"`
	IsTransient   *bool  `json:"is_transient,omitempty"`
}

type providerErrorCarrier interface {
	ProviderErrorFields() map[string]any
}

var (
	httpStatusPattern         = regexp.MustCompile(`\(([1-5][0-9]{2})\)`)
	keyValueStatusPattern     = regexp.MustCompile(`(?i)\bstatus=([1-5][0-9]{2})\b`)
	keyValueCodePattern       = regexp.MustCompile(`(?i)\bcode=([a-z0-9_.:-]+)\b`)
	keyValueProviderPattern   = regexp.MustCompile(`(?i)\bprovider=([a-z0-9_.:-]+)\b`)
	keyValueReasonPattern     = regexp.MustCompile(`(?i)\bprovider_reason=([a-z0-9_.:-]+)\b`)
	keyValueQuotaLimitPattern = regexp.MustCompile(`(?i)\bquota_limit=([a-z0-9_.:-]+)\b`)
	keyValueQuotaScopePattern = regexp.MustCompile(`(?i)\bquota_scope=([a-z0-9_.:-]+)\b`)
	facebookFormattedCode     = regexp.MustCompile(`(?i)\[code=([0-9]+)\b`)
	facebookFormattedTrace    = regexp.MustCompile(`(?i)\btrace=([a-z0-9_-]+)`)
	metaJSONCodePattern       = regexp.MustCompile(`"code"\s*:\s*([0-9]+|"[^"]+")`)
	metaJSONSubcodePattern    = regexp.MustCompile(`"error_subcode"\s*:\s*([0-9]+|"[^"]+")`)
	youtubeJSONReasonPattern  = regexp.MustCompile(`"reason"\s*:\s*"([^"]+)"`)
	youtubeJSONDomainPattern  = regexp.MustCompile(`"domain"\s*:\s*"([^"]+)"`)
)

func classifyError(raw string, err error) Classification {
	c := Classify(raw)
	if err == nil {
		return c
	}
	if carrier, ok := err.(providerErrorCarrier); ok {
		if pe := providerErrorFromFields(carrier.ProviderErrorFields()); pe != nil {
			c.ProviderError = pe
			if pe.Code != "" && c.PlatformErrorCode == "" {
				c.PlatformErrorCode = pe.Code
			}
			if pe.Subcode != "" && c.PlatformErrorCode == "" {
				c.PlatformErrorCode = pe.Subcode
			}
			c.ErrorSource = ErrorSourcePlatform
		}
	}
	return enrichClassification(c, raw)
}

func enrichClassification(c Classification, raw string) Classification {
	if c.ProviderError == nil {
		c.ProviderError = ExtractProviderError(raw)
	}
	if c.PlatformErrorCode == "" && c.ProviderError != nil {
		c.PlatformErrorCode = FirstNonEmpty(c.ProviderError.Code, c.ProviderError.Subcode, c.ProviderError.Reason)
	}
	c.ErrorSource = errorSourceFor(c, raw)
	c.ErrorTemporality = errorTemporalityFor(c)
	return c
}

func errorSourceFor(c Classification, raw string) string {
	switch c.ErrorCode {
	case "validation_error":
		return ErrorSourceUnipost
	case "worker_stalled":
		return ErrorSourceWorker
	case "unknown_error":
		return ErrorSourceUnknown
	case "temporary_platform_error", "rate_limit", "platform_request_invalid", "account_reconnect_required", "auth_token_invalid", "missing_permission", "target_not_found", "platform_error":
		return ErrorSourcePlatform
	case "media_error":
		if hasProviderSignal(c, raw) {
			return ErrorSourcePlatform
		}
		return ErrorSourceUnipost
	case "quota_exceeded":
		if hasProviderSignal(c, raw) {
			return ErrorSourcePlatform
		}
		return ErrorSourceUnipost
	default:
		if hasProviderSignal(c, raw) {
			return ErrorSourcePlatform
		}
		if strings.TrimSpace(c.ErrorCode) == "" {
			return ErrorSourceUnknown
		}
		return ErrorSourceUnknown
	}
}

func errorTemporalityFor(c Classification) string {
	switch c.ErrorCode {
	case "temporary_platform_error", "rate_limit", "worker_stalled":
		return ErrorTemporalityTemporary
	case "validation_error", "platform_request_invalid", "media_error", "quota_exceeded", "account_reconnect_required", "auth_token_invalid", "missing_permission", "target_not_found":
		return ErrorTemporalityPermanent
	case "platform_error", "unknown_error":
		return ErrorTemporalityUnknown
	default:
		return ErrorTemporalityUnknown
	}
}

func hasProviderSignal(c Classification, raw string) bool {
	if c.ProviderError != nil && c.ProviderError.Provider != "" {
		return true
	}
	s := strings.ToLower(raw)
	return strings.Contains(s, "provider=") ||
		strings.Contains(s, "tiktok") ||
		strings.Contains(s, "youtube") ||
		strings.Contains(s, "instagram") ||
		strings.Contains(s, "facebook") ||
		strings.Contains(s, "threads") ||
		strings.Contains(s, "graph api") ||
		strings.Contains(s, "oauthexception")
}

func ExtractProviderError(raw string) *ProviderError {
	s := strings.ToLower(raw)
	switch {
	case strings.Contains(s, "oauthexception") || strings.Contains(s, "fbtrace_id") || strings.Contains(s, "facebook publish"):
		return extractMetaProviderError(raw)
	case strings.Contains(s, "tiktok") || strings.Contains(s, "provider_error="):
		return extractTikTokProviderError(raw)
	case strings.Contains(s, "youtube") || strings.Contains(s, "provider=youtube"):
		return extractYouTubeProviderError(raw)
	default:
		return nil
	}
}

func ProviderErrorJSON(pe *ProviderError) []byte {
	if pe == nil || pe.empty() {
		return nil
	}
	b, err := json.Marshal(pe)
	if err != nil {
		return nil
	}
	return b
}

func ParseProviderErrorJSON(data []byte) *ProviderError {
	if len(data) == 0 {
		return nil
	}
	var pe ProviderError
	if err := json.Unmarshal(data, &pe); err != nil || pe.empty() {
		return nil
	}
	return &pe
}

func (pe ProviderError) empty() bool {
	return pe.Provider == "" &&
		pe.HTTPStatus == 0 &&
		pe.Code == "" &&
		pe.Subcode == "" &&
		pe.Type == "" &&
		pe.Reason == "" &&
		pe.Domain == "" &&
		pe.QuotaLimit == "" &&
		pe.QuotaLocation == "" &&
		pe.IsTransient == nil
}

func extractMetaProviderError(raw string) *ProviderError {
	status := extractHTTPStatus(raw)
	var parsed struct {
		Error struct {
			Message      string `json:"message"`
			Type         string `json:"type"`
			Code         any    `json:"code"`
			ErrorSubcode any    `json:"error_subcode"`
			IsTransient  *bool  `json:"is_transient"`
		} `json:"error"`
	}
	_ = decodeFirstJSONObject(raw, &parsed)

	pe := &ProviderError{
		Provider:    "meta",
		HTTPStatus:  status,
		Code:        scalarToString(parsed.Error.Code),
		Subcode:     scalarToString(parsed.Error.ErrorSubcode),
		Type:        strings.TrimSpace(parsed.Error.Type),
		IsTransient: parsed.Error.IsTransient,
	}
	if pe.Code == "" {
		pe.Code = regexpFirst(facebookFormattedCode, raw)
	}
	if pe.Code == "" {
		pe.Code = trimJSONScalar(regexpFirst(metaJSONCodePattern, raw))
	}
	if pe.Subcode == "" {
		pe.Subcode = trimJSONScalar(regexpFirst(metaJSONSubcodePattern, raw))
	}
	if pe.HTTPStatus == 0 {
		pe.HTTPStatus = status
	}
	_ = regexpFirst(facebookFormattedTrace, raw)
	if pe.empty() || pe.Provider == "" && pe.Code == "" {
		return nil
	}
	return pe
}

func extractTikTokProviderError(raw string) *ProviderError {
	code := extractTikTokProviderCode(raw)
	if code == "" {
		code = regexpFirst(keyValueCodePattern, raw)
	}
	pe := &ProviderError{
		Provider:   "tiktok",
		HTTPStatus: extractHTTPStatus(raw),
		Code:       code,
	}
	if pe.Code == "" && pe.HTTPStatus == 0 {
		return nil
	}
	return pe
}

func extractYouTubeProviderError(raw string) *ProviderError {
	pe := &ProviderError{
		Provider:      "youtube",
		HTTPStatus:    extractHTTPStatus(raw),
		Reason:        FirstNonEmpty(regexpFirst(keyValueReasonPattern, raw), regexpFirst(youtubeJSONReasonPattern, raw)),
		Domain:        regexpFirst(youtubeJSONDomainPattern, raw),
		QuotaLimit:    regexpFirst(keyValueQuotaLimitPattern, raw),
		QuotaLocation: regexpFirst(keyValueQuotaScopePattern, raw),
	}
	if pe.Reason == "" && pe.Domain == "" && pe.QuotaLimit == "" && pe.HTTPStatus == 0 {
		return nil
	}
	return pe
}

func providerErrorFromFields(fields map[string]any) *ProviderError {
	if len(fields) == 0 {
		return nil
	}
	pe := &ProviderError{
		Provider:      stringFromAny(fields["provider"]),
		Code:          stringFromAny(fields["code"]),
		Subcode:       stringFromAny(fields["subcode"]),
		Type:          stringFromAny(fields["type"]),
		Reason:        stringFromAny(fields["reason"]),
		Domain:        stringFromAny(fields["domain"]),
		QuotaLimit:    stringFromAny(fields["quota_limit"]),
		QuotaLocation: stringFromAny(fields["quota_location"]),
	}
	if v, ok := intFromAny(fields["http_status"]); ok {
		pe.HTTPStatus = v
	}
	if v, ok := fields["is_transient"].(bool); ok {
		pe.IsTransient = &v
	}
	if pe.empty() {
		return nil
	}
	return pe
}

func extractHTTPStatus(raw string) int {
	if status := atoi(regexpFirst(httpStatusPattern, raw)); status != 0 {
		return status
	}
	return atoi(regexpFirst(keyValueStatusPattern, raw))
}

func decodeFirstJSONObject(raw string, v any) error {
	idx := strings.Index(raw, "{")
	if idx < 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(raw[idx:]))
	return dec.Decode(v)
}

func regexpFirst(pattern *regexp.Regexp, raw string) string {
	m := pattern.FindStringSubmatch(raw)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func trimJSONScalar(v string) string {
	return strings.Trim(strings.TrimSpace(v), `"`)
}

func scalarToString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case json.Number:
		return x.String()
	default:
		return ""
	}
}

func stringFromAny(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return scalarToString(v)
	}
}

func intFromAny(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n := atoi(x)
		return n, n != 0
	default:
		return 0, false
	}
}

func atoi(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	return n
}
