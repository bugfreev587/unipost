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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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

// NewRecorder returns a Recorder with a sensible cap. 16 entries covers
// the worst publish path (TikTok photo post: creator_info + init + 3x
// photo pull + status poll × 12) with headroom.
func NewRecorder() *Recorder {
	return &Recorder{maxEntries: 16}
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
	return b.String()
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

// RoundTrip is the hot path. We buffer the request body once (so we can
// log it and still forward it), fire the request, and — on failure
// only — append a redacted curl entry to the context's recorder.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := RecorderFromContext(req.Context())

	// Fast path: no recorder, no capture. Avoids buffering the request
	// body for the 99% of traffic that doesn't care.
	if rec == nil {
		return t.base.RoundTrip(req)
	}

	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			// Reading the body failed — record what we can and let
			// the caller's transport surface the real error.
			rec.append(Entry{
				CurlCommand:    buildCurl(req, nil),
				TransportError: "request body unreadable: " + err.Error(),
				RecordedAt:     time.Now(),
			})
			return nil, err
		}
		bodyBytes = b
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		// GetBody lets net/http replay the body on redirects and
		// retries. Without it, redirected requests go out empty.
		if req.GetBody == nil {
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyBytes)), nil
			}
		}
	}

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		rec.append(Entry{
			CurlCommand:    buildCurl(req, bodyBytes),
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
		CurlCommand:  buildCurl(req, bodyBytes),
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

// ── Curl formatting + redaction ─────────────────────────────────────────

// buildCurl renders a copyable curl one-liner for the given request.
// Sensitive headers / query params are redacted in place; the caller's
// original request is not mutated.
func buildCurl(req *http.Request, body []byte) string {
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

	if len(body) > 0 {
		b.WriteString(" \\\n  --data '")
		b.WriteString(escapeSingleQuotes(string(body)))
		b.WriteString("'")
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
