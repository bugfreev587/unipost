// Package debugrt captures outbound HTTP requests that fail (non-2xx
// response or transport error) so the dispatcher can persist a bounded,
// redacted diagnostic when a publish fails.
//
// The collector is context-local: a publish attempt creates a Recorder,
// stashes it on the context, and the shared RoundTripper appends one
// entry per failing request. After the adapter's Post returns, the
// dispatcher reads Entries and hands the result to the observability
// writer. No entries are recorded when a Recorder is absent from the
// context — callers that don't care about debug capture (webhook
// subscribers, media downloads, etc.) pay zero cost.
//
// Sensitive fields are redacted before entries are stored. Only explicitly
// allowlisted headers are retained, all query values are omitted, structured
// bodies are recursively redacted, and unstructured bodies are omitted.
package debugrt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Entry is one failing HTTP request/response cycle. CurlCommand is what
// we show to users and admins; Status and Duration are kept for log /
// admin context.
type Entry struct {
	CurlCommand    string
	Status         int
	ResponseBody   string
	Duration       time.Duration
	RecordedAt     time.Time
	TransportError string // non-empty when the request never got a response
}

// Recorder accumulates entries from one publish attempt. Safe for
// concurrent use — a single publish can fan out media fetches that
// race against the primary Post call.
type Recorder struct {
	mu      sync.Mutex
	entries []Entry
	// maxEntries caps how many failing requests we keep to avoid
	// unbounded growth when an adapter retries aggressively. Extra
	// entries are counted via dropped so we can surface "... and N
	// more" instead of silently losing them.
	maxEntries int
	dropped    int
}

// maxResponseBodyBytes bounds how much of the response body we record
// per entry. TikTok / Meta error bodies are tiny JSON, but a misrouted
// 500 can be a 2MB HTML error page — we truncate to keep rows sane.
const maxResponseBodyBytes = 8 * 1024

const (
	maxRequestBodyBytes = 32 * 1024
	maxSerializedBytes  = 64 * 1024
	maxRecorderEntries  = 8
)

// NewRecorder returns a Recorder with a hard per-publish entry cap.
func NewRecorder() *Recorder {
	return &Recorder{maxEntries: maxRecorderEntries}
}

// Entries returns a snapshot of all entries recorded so far.
func (r *Recorder) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	return out
}

// Dropped returns how many entries exceeded the cap and weren't kept.
func (r *Recorder) Dropped() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

func (r *Recorder) append(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) >= r.maxEntries {
		r.dropped++
		return
	}
	r.entries = append(r.entries, e)
}

// Serialize renders the recorder's entries as a single string, suitable
// for storing on the debug_curl column. Returns empty string when no
// entries were captured so the caller can use a nullable column.
func (r *Recorder) Serialize() string {
	entries := r.Entries()
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "# Request %d — ", i+1)
		if transportError := sanitizeDiagnosticText(e.TransportError); transportError != "" {
			fmt.Fprintf(&b, "transport error: %s", transportError)
		} else {
			fmt.Fprintf(&b, "HTTP %d (%s)", e.Status, e.Duration.Round(time.Millisecond))
		}
		b.WriteString("\n")
		b.WriteString(sanitizeDiagnosticText(e.CurlCommand))
		if responseBody := sanitizeDiagnosticText(e.ResponseBody); responseBody != "" {
			b.WriteString("\n# Response:\n# ")
			// Prefix every line so the whole block reads as a curl
			// comment — user can paste straight into a shell.
			lines := strings.Split(responseBody, "\n")
			b.WriteString(strings.Join(lines, "\n# "))
		}
	}
	if dropped := r.Dropped(); dropped > 0 {
		fmt.Fprintf(&b, "\n\n# (%d additional failing request%s were omitted)", dropped, plural(dropped))
	}
	return boundSerialized(b.String())
}

func boundSerialized(value string) string {
	value = normalizeDiagnosticText(value)
	if len(value) <= maxSerializedBytes {
		return value
	}
	const marker = "\n\n# diagnostic truncated at 65536 bytes"
	prefix := value[:maxSerializedBytes-len(marker)]
	for !utf8.ValidString(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix + marker
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

type ctxKey struct{}

// WithRecorder returns a context carrying the given recorder. The
// transport reads it from context on every request. Passing a nil
// recorder is a no-op — useful for tests that want to disable capture.
func WithRecorder(ctx context.Context, rec *Recorder) context.Context {
	if rec == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, rec)
}

// RecorderFromContext extracts the recorder, or nil when none is set.
func RecorderFromContext(ctx context.Context) *Recorder {
	if ctx == nil {
		return nil
	}
	rec, _ := ctx.Value(ctxKey{}).(*Recorder)
	return rec
}

// Transport is an http.RoundTripper that wraps another transport and
// records entries onto any recorder it finds in the request's context.
// Use NewClient or Wrap to get one.
type Transport struct {
	base http.RoundTripper
}

// Wrap returns a Transport that delegates to base. Pass nil to wrap
// http.DefaultTransport.
func Wrap(base http.RoundTripper) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{base: base}
}

// NewClient returns an http.Client with the given timeout whose
// transport captures failing requests into any recorder found on each
// request's context. Callers that previously did
//
//	&http.Client{Timeout: 30 * time.Second}
//
// can swap in debugrt.NewClient(30 * time.Second) with no other
// changes.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: Wrap(nil),
	}
}

// RoundTrip is the hot path. A bounded wrapper observes bytes only while
// the underlying transport reads them. It never pre-reads or replaces the
// caller's replay behavior, and only a failed request is retained.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := RecorderFromContext(req.Context())

	// Fast path: no recorder, no capture. Avoids buffering the request
	// body for the 99% of traffic that doesn't care.
	if rec == nil {
		return t.base.RoundTrip(req)
	}

	var bodyCapture *requestBodyCapture
	if req.Body != nil {
		bodyCapture = newRequestBodyCapture(req.Body, req.Header.Get("Content-Type"))
		req.Body = bodyCapture
	}

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		rec.append(Entry{
			CurlCommand:    buildCurlFromCapture(req, snapshotRequestBody(bodyCapture)),
			TransportError: err.Error(),
			Duration:       elapsed,
			RecordedAt:     time.Now(),
		})
		return nil, err
	}

	// Only capture 4xx/5xx — success paths are noise.
	if resp.StatusCode < 400 {
		return resp, nil
	}

	// Tee the response body into a buffer so both the caller and the
	// recorder can read it. Bound the buffered size to avoid blowing
	// up on large error pages.
	bodyForCaller, bodyForRecorder := teeBounded(resp.Body, maxResponseBodyBytes)
	resp.Body = bodyForCaller
	rec.append(Entry{
		CurlCommand:  buildCurlFromCapture(req, snapshotRequestBody(bodyCapture)),
		Status:       resp.StatusCode,
		ResponseBody: sanitizeResponseBody(bodyForRecorder, resp.Header.Get("Content-Type")),
		Duration:     elapsed,
		RecordedAt:   time.Now(),
	})
	return resp, nil
}

// teeBounded reads up to limit bytes from src (keeping them for the
// recorder) while leaving the rest intact for the actual caller. The
// returned ReadCloser re-prepends the buffered bytes so the caller
// sees the unchanged body.
func teeBounded(src io.ReadCloser, limit int) (io.ReadCloser, []byte) {
	buf := make([]byte, limit)
	n, _ := io.ReadFull(src, buf)
	buf = buf[:n]
	combined := io.MultiReader(bytes.NewReader(buf), src)
	return readCloser{Reader: combined, Closer: src}, buf
}

type readCloser struct {
	io.Reader
	io.Closer
}

type capturedRequestBody struct {
	Data        []byte
	ContentType string
	Observed    int64
	SHA256      string
	Omitted     bool
	Truncated   bool
}

type requestBodyCapture struct {
	body              io.ReadCloser
	mu                sync.Mutex
	hash              hash.Hash
	data              []byte
	contentType       string
	observed          int64
	captureStructured bool
}

func newRequestBodyCapture(body io.ReadCloser, contentType string) *requestBodyCapture {
	mediaType := normalizedContentType(contentType)
	return &requestBodyCapture{
		body:              body,
		hash:              sha256.New(),
		data:              make([]byte, 0, maxRequestBodyBytes),
		contentType:       mediaType,
		captureStructured: isSafeStructuredContentType(mediaType),
	}
}

func (c *requestBodyCapture) Read(p []byte) (int, error) {
	n, err := c.body.Read(p)
	if n > 0 {
		chunk := p[:n]
		c.mu.Lock()
		c.observed += int64(n)
		_, _ = c.hash.Write(chunk)
		if c.captureStructured && len(c.data) < maxRequestBodyBytes {
			remaining := maxRequestBodyBytes - len(c.data)
			if remaining > n {
				remaining = n
			}
			c.data = append(c.data, chunk[:remaining]...)
		}
		c.mu.Unlock()
	}
	return n, err
}

func (c *requestBodyCapture) Close() error {
	return c.body.Close()
}

func (c *requestBodyCapture) snapshot() capturedRequestBody {
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := capturedRequestBody{
		Data:        append([]byte(nil), c.data...),
		ContentType: c.contentType,
		Observed:    c.observed,
		Omitted:     !c.captureStructured && c.observed > 0,
		Truncated:   c.captureStructured && c.observed > int64(len(c.data)),
	}
	if c.observed > 0 {
		snapshot.SHA256 = hex.EncodeToString(c.hash.Sum(nil))
	}
	return sanitizeCapturedRequestBody(snapshot)
}

func snapshotRequestBody(capture *requestBodyCapture) capturedRequestBody {
	if capture == nil {
		return capturedRequestBody{}
	}
	return capture.snapshot()
}

func normalizedContentType(value string) string {
	mediaType, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(value)), ";")
	mediaType = strings.TrimSpace(mediaType)
	if mediaType == "" {
		return "unknown"
	}
	return mediaType
}

func isSafeStructuredContentType(mediaType string) bool {
	return mediaType == "application/json" ||
		strings.HasSuffix(mediaType, "+json") ||
		mediaType == "application/x-www-form-urlencoded"
}

func sanitizeCapturedRequestBody(body capturedRequestBody) capturedRequestBody {
	if body.Observed == 0 {
		body.Data = nil
		return body
	}
	if body.Omitted || body.Truncated {
		body.Data = nil
		body.Omitted = true
		return body
	}
	sanitized, ok := sanitizeStructuredPayload(body.Data, body.ContentType)
	if !ok {
		body.Data = nil
		body.Omitted = true
		return body
	}
	body.Data = sanitized
	return body
}

func sanitizeStructuredPayload(data []byte, mediaType string) ([]byte, bool) {
	switch {
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"):
		var value any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, false
		}
		redacted, err := json.Marshal(redactJSONValue(value))
		return redacted, err == nil
	case mediaType == "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(data))
		if err != nil {
			return nil, false
		}
		for key := range values {
			if isSensitiveFieldKey(key) {
				values[key] = []string{"[REDACTED]"}
			}
		}
		return []byte(values.Encode()), true
	default:
		return nil, false
	}
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			if isSensitiveFieldKey(key) {
				out[key] = "[REDACTED]"
			} else {
				out[key] = redactJSONValue(nested)
			}
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, nested := range typed {
			out[i] = redactJSONValue(nested)
		}
		return out
	default:
		return typed
	}
}

func isSensitiveFieldKey(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, key)
	for _, marker := range []string{"token", "secret", "password", "authorization", "cookie", "apikey", "signature", "appsecretproof"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

var (
	authorizationValuePattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[^\s,;]+`)
	credentialPairPattern     = regexp.MustCompile(`(?i)\b(access[_-]?token|refresh[_-]?token|client[_-]?secret|api[_-]?key|authorization|password|cookie|signature|appsecret[_-]?proof|token)\s*[:=]\s*[^\s&,;]+`)
)

func normalizeDiagnosticText(value string) string {
	value = strings.ToValidUTF8(value, "�")
	return strings.ReplaceAll(value, "\x00", "")
}

func sanitizeDiagnosticText(value string) string {
	value = normalizeDiagnosticText(value)
	value = authorizationValuePattern.ReplaceAllString(value, "$1 [REDACTED]")
	return credentialPairPattern.ReplaceAllString(value, "$1=[REDACTED]")
}

func sanitizeResponseBody(data []byte, contentType string) string {
	if len(data) == 0 {
		return ""
	}
	mediaType := normalizedContentType(contentType)
	if sanitized, ok := sanitizeStructuredPayload(data, mediaType); ok {
		return sanitizeDiagnosticText(string(sanitized))
	}
	digest := sha256.Sum256(data)
	return fmt.Sprintf(
		"response body omitted: content_type=%s captured_bytes=%d sha256=%x",
		mediaType,
		len(data),
		digest,
	)
}

// ── Curl formatting + redaction ─────────────────────────────────────────

// buildCurl renders a copyable curl one-liner for the given request.
// Sensitive headers / query params are redacted in place; the caller's
// original request is not mutated.
func buildCurl(req *http.Request, body []byte) string {
	originalSize := len(body)
	var digest [sha256.Size]byte
	if originalSize > 0 {
		digest = sha256.Sum256(body)
	}
	if len(body) > maxRequestBodyBytes {
		body = body[:maxRequestBodyBytes]
	}
	captured := capturedRequestBody{
		Data:        body,
		ContentType: normalizedContentType(req.Header.Get("Content-Type")),
		Observed:    int64(originalSize),
		Truncated:   originalSize > len(body),
	}
	if originalSize > 0 {
		captured.SHA256 = hex.EncodeToString(digest[:])
		captured = sanitizeCapturedRequestBody(captured)
	}
	return buildCurlFromCapture(req, captured)
}

func buildCurlFromCapture(req *http.Request, body capturedRequestBody) string {
	var b strings.Builder
	b.WriteString("curl -X ")
	b.WriteString(req.Method)
	b.WriteString(" '")
	b.WriteString(redactURL(req.URL))
	b.WriteString("'")

	// Sorted-by-name header list keeps output stable across runs,
	// which matters for tests and for humans diffing failing posts.
	names := make([]string, 0, len(req.Header))
	for name := range req.Header {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		if !allowlistedDiagnosticHeaders[http.CanonicalHeaderKey(name)] {
			continue
		}
		for _, value := range req.Header[name] {
			b.WriteString(" \\\n  -H '")
			b.WriteString(name)
			b.WriteString(": ")
			b.WriteString(redactHeaderValue(name, value))
			b.WriteString("'")
		}
	}

	if len(body.Data) > 0 {
		b.WriteString(" \\\n  --data '")
		b.WriteString(escapeSingleQuotes(string(body.Data)))
		b.WriteString("'")
	}
	if body.Omitted {
		fmt.Fprintf(&b, "\n# request body omitted: content_type=%s observed_bytes=%d sha256=%s", body.ContentType, body.Observed, body.SHA256)
	} else if body.Truncated {
		fmt.Fprintf(&b, "\n# request body truncated: content_type=%s observed_bytes=%d stored_bytes=%d sha256=%s", body.ContentType, body.Observed, len(body.Data), body.SHA256)
	}
	return b.String()
}

// sortStrings — avoid pulling in sort just for one call.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// escapeSingleQuotes quotes a body for safe inclusion between single
// quotes. The classic shell trick: close, escape the quote, reopen.
func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// Only headers known to carry non-secret protocol metadata are stored.
// New provider headers remain omitted until they are explicitly reviewed.
var allowlistedDiagnosticHeaders = map[string]bool{
	"Accept":                    true,
	"Content-Range":             true,
	"Content-Type":              true,
	"File_size":                 true,
	"Linkedin-Version":          true,
	"Offset":                    true,
	"X-Restli-Protocol-Version": true,
	"X-Upload-Content-Length":   true,
	"X-Upload-Content-Type":     true,
}

func redactHeaderValue(_ string, value string) string {
	return sanitizeDiagnosticText(value)
}

func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	q := u.Query()
	for key := range q {
		q.Set(key, "[REDACTED]")
	}
	clone := *u
	clone.RawQuery = q.Encode()
	return clone.String()
}
