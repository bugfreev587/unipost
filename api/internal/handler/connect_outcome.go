package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

type hostedConnectOutcomeWriter interface {
	WriteSync(context.Context, integrationlogs.Event) error
}

type hostedConnectAttempt struct {
	WorkspaceID    string
	ProfileID      string
	Platform       string
	SessionID      string
	ExternalUserID string
	RequestID      string
}

type hostedConnectOutcome struct {
	ctx     context.Context
	writer  hostedConnectOutcomeWriter
	attempt hostedConnectAttempt

	mu    sync.Mutex
	event integrationlogs.Event
	once  sync.Once
}

var hostedConnectOutcomeWriteFailures atomic.Uint64

func newHostedConnectOutcome(ctx context.Context, writer hostedConnectOutcomeWriter, attempt hostedConnectAttempt) *hostedConnectOutcome {
	return &hostedConnectOutcome{
		ctx:     ctx,
		writer:  writer,
		attempt: attempt,
		event: integrationlogs.Event{
			WorkspaceID: attempt.WorkspaceID,
			Level:       integrationlogs.LevelError,
			Status:      integrationlogs.StatusError,
			Category:    integrationlogs.CategoryOAuth,
			Action:      integrationlogs.ActionAccountConnectCallbackFailed,
			Source:      integrationlogs.SourceOAuth,
			Message:     "Hosted Connect failed.",
			RequestID:   attempt.RequestID,
			ProfileID:   attempt.ProfileID,
			Platform:    attempt.Platform,
			ErrorCode:   "connect_callback_failed",
			Metadata:    hostedConnectMetadata(attempt, "error", nil),
		},
	}
}

func (o *hostedConnectOutcome) Success(socialAccountID, accountName string) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.event.Level = integrationlogs.LevelInfo
	o.event.Status = integrationlogs.StatusSuccess
	o.event.Action = integrationlogs.ActionAccountConnectCallbackOK
	o.event.Message = "Hosted Connect completed successfully."
	o.event.SocialAccountID = socialAccountID
	o.event.ErrorCode = ""
	extra := map[string]any{}
	if accountName != "" {
		extra["account_name"] = accountName
	}
	o.event.Metadata = hostedConnectMetadata(o.attempt, "success", extra)
}

func (o *hostedConnectOutcome) Fail(code, message string, metadata map[string]any) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.event.Level = integrationlogs.LevelError
	o.event.Status = integrationlogs.StatusError
	o.event.Action = integrationlogs.ActionAccountConnectCallbackFailed
	o.event.Message = message
	o.event.ErrorCode = code
	o.event.Metadata = hostedConnectMetadata(o.attempt, "error", metadata)
}

func (o *hostedConnectOutcome) Cancel(code string) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.event.Level = integrationlogs.LevelWarn
	o.event.Status = integrationlogs.StatusWarning
	o.event.Action = integrationlogs.ActionAccountConnectCallbackCancelled
	o.event.Message = "Hosted Connect was cancelled by the user."
	o.event.ErrorCode = code
	o.event.Metadata = hostedConnectMetadata(o.attempt, "cancelled", nil)
}

func (o *hostedConnectOutcome) persist() {
	if o == nil {
		return
	}
	o.once.Do(func() {
		if o.writer == nil {
			return
		}
		o.mu.Lock()
		event := o.event
		o.mu.Unlock()
		if err := o.writer.WriteSync(o.ctx, event); err != nil {
			slog.Error("hosted_connect_outcome_log_write_failed",
				"workspace_id", o.attempt.WorkspaceID,
				"platform", o.attempt.Platform,
				"action", event.Action,
				"request_id", o.attempt.RequestID,
				"total_failures", hostedConnectOutcomeWriteFailures.Add(1),
			)
		}
	})
}

func hostedConnectMetadata(attempt hostedConnectAttempt, status string, extra map[string]any) map[string]any {
	metadata := map[string]any{
		"connect_session_id": attempt.SessionID,
		"external_user_id":   attempt.ExternalUserID,
		"callback_status":    status,
		"connection_type":    "managed",
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

type hostedConnectResponseWriter struct {
	http.ResponseWriter
	outcome *hostedConnectOutcome
}

func wrapHostedConnectResponse(w http.ResponseWriter, outcome *hostedConnectOutcome) http.ResponseWriter {
	return &hostedConnectResponseWriter{ResponseWriter: w, outcome: outcome}
}

func (w *hostedConnectResponseWriter) WriteHeader(code int) {
	w.outcome.persist()
	w.ResponseWriter.WriteHeader(code)
}

func (w *hostedConnectResponseWriter) Write(body []byte) (int, error) {
	w.outcome.persist()
	return w.ResponseWriter.Write(body)
}

func (w *hostedConnectResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
