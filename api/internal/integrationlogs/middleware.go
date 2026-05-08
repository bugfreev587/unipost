package integrationlogs

import (
	"fmt"
	"net/http"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appmw "github.com/xiaoboyu/unipost-api/internal/middleware"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *loggingResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func Middleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			workspaceID := auth.GetWorkspaceID(r.Context())
			if logger == nil || workspaceID == "" {
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
				ActorUserID:    auth.GetUserID(r.Context()),
				ActorAPIKeyID:  auth.GetAPIKeyID(r.Context()),
				Endpoint:       r.URL.Path,
				Method:         r.Method,
				HTTPStatusCode: &httpStatusCode,
				DurationMS:     &durationMs,
				ErrorCode:      errorCode,
			})
		})
	}
}
