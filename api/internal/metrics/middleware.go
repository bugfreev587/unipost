// Package metrics provides a chi middleware that records per-request
// API metrics to the api_metrics table. Only active on API-key-auth
// routes — dashboard (Clerk) routes are excluded.

package metrics

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// Recorder is a chi middleware that inserts one api_metrics row per
// API-key-authenticated request. The insert is fire-and-forget (async)
// so it never blocks or slows down the response.
type Recorder struct {
	queries *db.Queries
}

func NewRecorder(queries *db.Queries) *Recorder {
	return &Recorder{queries: queries}
}

// Middleware returns the chi-compatible handler wrapper.
func (rec *Recorder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspaceID := auth.GetWorkspaceID(r.Context())
		if workspaceID == "" {
			// Not an API-key request — skip metrics recording.
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		durationMs := int(time.Since(start).Milliseconds())

		// Normalize the path: strip UUIDs and IDs to group endpoints.
		normalizedPath := normalizePath(r.URL.Path)

		// Fire-and-forget insert — don't block the response.
		go func() {
			_ = rec.queries.InsertAPIMetric(r.Context(), db.InsertAPIMetricParams{
				WorkspaceID: workspaceID,
				Method:      r.Method,
				Path:        normalizedPath,
				StatusCode:  int32(sw.status),
				DurationMs:  int32(durationMs),
			})
		}()
	})
}

// statusWriter wraps ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// uuidRegex matches UUIDs and other ID-like path segments.
var uuidRegex = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizePath replaces UUID path segments with :id so that
// /v1/social-posts/abc-123 and /v1/social-posts/def-456 both
// become /v1/social-posts/:id.
func normalizePath(path string) string {
	// Replace UUIDs
	normalized := uuidRegex.ReplaceAllString(path, ":id")
	// Also replace any remaining numeric-only segments (e.g. /users/12345)
	parts := strings.Split(normalized, "/")
	for i, p := range parts {
		if len(p) > 8 && !strings.HasPrefix(p, "v1") && !strings.Contains(p, ":") {
			// Long alphanumeric segments that aren't version prefixes
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}
