package integrationlogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appmw "github.com/xiaoboyu/unipost-api/internal/middleware"
)

const maxCapturedBodyBytes = 64 * 1024

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rw *loggingResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush supports SSE streaming by delegating to the underlying
// ResponseWriter's Flusher interface.
func (rw *loggingResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *loggingResponseWriter) Write(p []byte) (int, error) {
	if rw.body.Len() < maxCapturedBodyBytes {
		remaining := maxCapturedBodyBytes - rw.body.Len()
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = rw.body.Write(p[:remaining])
	}
	return rw.ResponseWriter.Write(p)
}

func captureRequestBody(r *http.Request) any {
	if r.Body == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxCapturedBodyBytes+1))
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > maxCapturedBodyBytes {
		return map[string]any{
			"truncated": true,
			"payload":   string(raw[:maxCapturedBodyBytes]),
		}
	}
	var decoded any
	if err := jsonUnmarshal(raw, &decoded); err == nil {
		return decoded
	}
	return string(raw)
}

func captureResponseBody(rw *loggingResponseWriter, contentType string) any {
	if rw == nil || rw.body.Len() == 0 {
		return nil
	}
	raw := rw.body.Bytes()
	if strings.Contains(strings.ToLower(contentType), "json") {
		var decoded any
		if err := jsonUnmarshal(raw, &decoded); err == nil {
			return decoded
		}
	}
	return string(raw)
}

func sanitizeHeaders(headers http.Header) map[string]any {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]any, len(headers))
	for key, values := range headers {
		if len(values) == 1 {
			out[key] = values[0]
			continue
		}
		dup := make([]string, len(values))
		copy(dup, values)
		out[key] = dup
	}
	return out
}

func sanitizeQuery(values map[string][]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, list := range values {
		if len(list) == 1 {
			out[key] = list[0]
			continue
		}
		dup := make([]string, len(list))
		copy(dup, list)
		out[key] = dup
	}
	return out
}

func jsonUnmarshal(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

func Middleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestBody := captureRequestBody(r)
			rw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			workspaceID := auth.GetWorkspaceID(r.Context())
			if logger == nil || workspaceID == "" {
				return
			}
			if IsInternalRequestPath(r.URL.Path) {
				return
			}
			// 402 is reserved for billing/plan gates (PLAN_FEATURE_NOT_AVAILABLE,
			// PLAN_PLATFORM_NOT_ALLOWED, *_LIMIT_REACHED). The caller has already
			// been shown the upgrade message — recording every poll just bloats
			// the workspace logs view. Funnel/conversion signal belongs in
			// product analytics, not the integration log.
			if rw.status == http.StatusPaymentRequired {
				return
			}
			// Clerk-JWT (dashboard) calls are noise in workspace logs: the
			// customer wants to see what their SDK / bots / API-key callers
			// did, not what they themselves clicked in the dashboard. Skip
			// any request authenticated by user_id without an api_key_id —
			// admin can still inspect this traffic via Railway HTTP logs.
			actorUserID := auth.GetUserID(r.Context())
			actorAPIKeyID := auth.GetAPIKeyID(r.Context())
			if actorUserID != "" && actorAPIKeyID == "" {
				return
			}

			level := LevelInfo
			status := StatusSuccess
			action := ActionAPIRequestSucceeded
			errorCode := ""
			message := fmt.Sprintf("%s %s returned %d.", r.Method, r.URL.Path, rw.status)

			switch {
			case rw.status >= 500:
				level = LevelError
				status = StatusError
				action = ActionAPIRequestFailed
				errorCode = "internal_error"
			case rw.status >= 400:
				level = LevelWarn
				status = StatusError
				action = ActionAPIRequestFailed
				errorCode = "client_error"
				if rw.status == http.StatusUnprocessableEntity {
					action = ActionAPIRequestValidationFailed
					errorCode = "validation_error"
				} else if rw.status == http.StatusTooManyRequests {
					action = ActionAPIRequestRateLimited
					errorCode = "rate_limited"
				}
			}

			httpStatusCode := rw.status
			durationMs := int(time.Since(start).Milliseconds())

			logger.Write(r.Context(), Event{
				WorkspaceID:    workspaceID,
				TS:             time.Now().UTC(),
				Level:          level,
				Status:         status,
				Category:       CategoryAPIRequest,
				Action:         action,
				Source:         SourceAPI,
				Message:        message,
				RequestID:      appmw.GetRequestID(r.Context()),
				ActorUserID:    actorUserID,
				ActorAPIKeyID:  actorAPIKeyID,
				Endpoint:       r.URL.Path,
				Method:         r.Method,
				HTTPStatusCode: &httpStatusCode,
				DurationMS:     &durationMs,
				ErrorCode:      errorCode,
				RequestPayload: map[string]any{
					"protocol": r.Proto,
					"method":   r.Method,
					"path":     r.URL.Path,
					"query":    sanitizeQuery(r.URL.Query()),
					"headers":  sanitizeHeaders(r.Header),
					"payload":  requestBody,
				},
				ResponsePayload: map[string]any{
					"protocol":    r.Proto,
					"status_code": rw.status,
					"headers":     sanitizeHeaders(rw.Header()),
					"payload":     captureResponseBody(rw, rw.Header().Get("Content-Type")),
				},
			})
		})
	}
}
