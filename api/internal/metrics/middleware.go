// Package metrics provides a chi middleware that records per-request
// API metrics to the api_metrics table. Only active on API-key-auth
// routes - dashboard (Clerk) routes are excluded.

package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// Recorder is a chi middleware that inserts one api_metrics row per
// API-key-authenticated request. The insert is fire-and-forget (async)
// so it never blocks or slows down the response.
type Recorder struct {
	inserter metricInserter
}

type metricInserter interface {
	InsertAPIMetric(context.Context, db.InsertAPIMetricParams) error
}

func NewRecorder(inserter metricInserter) *Recorder {
	return &Recorder{inserter: inserter}
}

// Middleware returns the chi-compatible handler wrapper.
func (rec *Recorder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec == nil || rec.inserter == nil || shouldSkipPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Only record requests authenticated via API key, not Clerk session.
		// DualAuthMiddleware stamps APIKeyID in context for API-key paths.
		apiKeyID := auth.GetAPIKeyID(r.Context())
		if apiKeyID == "" {
			next.ServeHTTP(w, r)
			return
		}
		workspaceID := auth.GetWorkspaceID(r.Context())
		if workspaceID == "" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		durationMs := int(time.Since(start).Milliseconds())

		normalizedPath := normalizeRoutePattern(chi.RouteContext(r.Context()).RoutePattern(), r.URL.Path)

		// Fire-and-forget insert: don't block the response.
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
			defer cancel()
			if err := rec.inserter.InsertAPIMetric(ctx, db.InsertAPIMetricParams{
				WorkspaceID: workspaceID,
				ApiKeyID:    pgtype.Text{String: apiKeyID, Valid: true},
				Method:      r.Method,
				Path:        normalizedPath,
				StatusCode:  int32(sw.status),
				DurationMs:  int32(durationMs),
			}); err != nil {
				slog.Debug("api metrics: insert failed", "err", err, "workspace_id", workspaceID)
			}
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

// Flush supports SSE streaming by delegating to the underlying
// ResponseWriter's Flusher interface.
func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var placeholderRegex = regexp.MustCompile(`\{[^/{}]+\}`)
var uuidSegmentRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var numericSegmentRegex = regexp.MustCompile(`^[0-9]+$`)
var longIDLikeSegmentRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{12,}$`)

func shouldSkipPath(path string) bool {
	for _, skippedPath := range []string{
		"/v1/api-metrics",
		"/v1/admin",
		"/v1/me",
		"/v1/public",
		"/health",
	} {
		if path == skippedPath || strings.HasPrefix(path, skippedPath+"/") {
			return true
		}
	}
	for _, prefix := range []string{
		"/webhooks/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return strings.HasSuffix(path, "/ws")
}

func normalizeRoutePattern(routePattern, rawPath string) string {
	routePattern = strings.TrimSpace(routePattern)
	if routePattern != "" {
		routePattern = placeholderRegex.ReplaceAllString(routePattern, ":id")
		if strings.HasPrefix(routePattern, "/") {
			return routePattern
		}
	}
	return normalizePathFallback(rawPath)
}

func normalizePathFallback(path string) string {
	path = strings.Split(path, "?")[0]
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "" || p == "v1" {
			continue
		}
		if uuidSegmentRegex.MatchString(p) || numericSegmentRegex.MatchString(p) || (longIDLikeSegmentRegex.MatchString(p) && strings.IndexFunc(p, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}
