// Package debugrt captures outbound HTTP requests that fail (non-2xx
// response or transport error) so the dispatcher can persist a curl
// equivalent on the social_post_results row when a publish fails.
//
// The collector is context-local: a publish attempt creates a Recorder,
// stashes it on the context, and the shared RoundTripper appends one
// entry per failing request. After the adapter's Post returns, the
// dispatcher reads Entries and writes the result to the debug_curl
// column. No entries are recorded when a Recorder is absent from the
// context — callers that don't care about debug capture (webhook
// subscribers, media downloads, etc.) pay zero cost.
//
// Sensitive fields are redacted before entries are stored: the
// Authorization header, any bearer tokens in the URL, and a handful of
// query-string secrets that the adapters happen to use.
package debugrt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
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
		if e.TransportError != "" {
			fmt.Fprintf(&b, "transport error: %s", e.TransportError)
		} else {
			fmt.Fprintf(&b, "HTTP %d (%s)", e.Status, e.Duration.Round(time.Millisecond))
		}
		b.WriteString("\n")
		b.WriteString(e.CurlCommand)
		if e.ResponseBody != "" {
			b.WriteString("\n# Response:\n# ")
			// Prefix every line so the whole block reads as a curl
			// comment — user can paste straight into a shell.
			lines := strings.Split(e.ResponseBody, "\n")
			b.WriteString(strings.Join(lines, "\n# "))
		}
	}
	if dropped := r.Dropped(); dropped > 0 {
		fmt.Fprintf(&b, "\n\n# (%d additional failing request%s were omitted)", dropped, plural(dropped))
	}
	return boundSerialized(b.String())
}

func boundSerialized(value string) string {
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
		ResponseBody: string(bodyForRecorder),
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
	body        io.ReadCloser
	mu          sync.Mutex
	hash        hash.Hash
	data        []byte
	contentType string
	observed    int64
	captureText bool
}

func newRequestBodyCapture(body io.ReadCloser, contentType string) *requestBodyCapture {
	mediaType := normalizedContentType(contentType)
	return &requestBodyCapture{
		body:        body,
		hash:        sha256.New(),
		data:        make([]byte, 0, maxRequestBodyBytes),
		contentType: mediaType,
		captureText: isSafeTextContentType(mediaType),
	}
}

func (c *requestBodyCapture) Read(p []byte) (int, error) {
	n, err := c.body.Read(p)
	if n > 0 {
		chunk := p[:n]
		c.mu.Lock()
		c.observed += int64(n)
		_, _ = c.hash.Write(chunk)
		if c.captureText && len(c.data) < maxRequestBodyBytes {
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
		Omitted:     !c.captureText && c.observed > 0,
		Truncated:   c.captureText && c.observed > int64(len(c.data)),
	}
	if c.observed > 0 {
		snapshot.SHA256 = hex.EncodeToString(c.hash.Sum(nil))
	}
	return snapshot
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

func isSafeTextContentType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/json" ||
		strings.HasSuffix(mediaType, "+json") ||
		mediaType == "application/x-www-form-urlencoded"
}

// ── Curl formatting + redaction ─────────────────────────────────────────

// buildCurl renders a copyable curl one-liner for the given request.
// Sensitive headers / query params are redacted in place; the caller's
// original request is not mutated.
func buildCurl(req *http.Request, body []byte) string {
	return buildCurlFromCapture(req, capturedRequestBody{
		Data:        body,
		ContentType: normalizedContentType(req.Header.Get("Content-Type")),
		Observed:    int64(len(body)),
	})
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

// redactedHeaders is the set of headers whose values we replace with
// a placeholder. Matched case-insensitively against the canonical form.
var redactedHeaders = map[string]bool{
	"Authorization":       true,
	"Cookie":              true,
	"Set-Cookie":          true,
	"Proxy-Authorization": true,
	"X-Api-Key":           true,
	"X-Auth-Token":        true,
	"Client-Secret":       true,
}

func redactHeaderValue(name, value string) string {
	canonical := http.CanonicalHeaderKey(name)
	if !redactedHeaders[canonical] {
		return value
	}
	// Authorization specifically gets a richer mask so the reader can
	// tell it was a Bearer / Basic / etc. scheme — useful when the
	// bug is "we sent the wrong kind of credential".
	if canonical == "Authorization" {
		if idx := strings.IndexByte(value, ' '); idx > 0 {
			return value[:idx] + " [REDACTED]"
		}
	}
	return "[REDACTED]"
}

// redactedQueryParams — query-string equivalents of the header list.
// Platforms like Meta attach access tokens to URLs; we strip those.
var redactedQueryParams = map[string]bool{
	"access_token":  true,
	"client_secret": true,
	"refresh_token": true,
	"token":         true,
	"api_key":       true,
}

func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	q := u.Query()
	redacted := false
	for key := range q {
		if redactedQueryParams[strings.ToLower(key)] {
			q.Set(key, "[REDACTED]")
			redacted = true
		}
	}
	if !redacted {
		return u.String()
	}
	clone := *u
	clone.RawQuery = q.Encode()
	return clone.String()
}
