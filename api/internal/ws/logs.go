package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/requestevents"
)

func LogEnvelope(row db.IntegrationLog) map[string]any {
	log := map[string]any{
		"id":           row.ID,
		"workspace_id": row.WorkspaceID,
		"ts":           row.Ts.Time,
		"level":        row.Level,
		"status":       row.Status,
		"category":     row.Category,
		"action":       row.Action,
		"source":       row.Source,
		"message":      row.Message,
	}
	if row.RequestID.Valid {
		log["request_id"] = row.RequestID.String
	}
	if row.TraceID.Valid {
		log["trace_id"] = row.TraceID.String
	}
	if row.ActorUserID.Valid {
		log["actor_user_id"] = row.ActorUserID.String
	}
	if row.ActorApiKeyID.Valid {
		log["actor_api_key_id"] = row.ActorApiKeyID.String
	}
	if row.ProfileID.Valid {
		log["profile_id"] = row.ProfileID.String
	}
	if row.SocialAccountID.Valid {
		log["social_account_id"] = row.SocialAccountID.String
	}
	if row.PostID.Valid {
		log["post_id"] = row.PostID.String
	}
	if row.PlatformPostID.Valid {
		log["platform_post_id"] = row.PlatformPostID.String
	}
	if row.Platform.Valid {
		log["platform"] = row.Platform.String
	}
	if row.Endpoint.Valid {
		log["endpoint"] = row.Endpoint.String
	}
	if row.Method.Valid {
		log["method"] = row.Method.String
	}
	if row.HTTPStatusCode.Valid {
		log["http_status_code"] = row.HTTPStatusCode.Int32
	}
	if row.RemoteStatusCode.Valid {
		log["remote_status_code"] = row.RemoteStatusCode.Int32
	}
	if row.DurationMs.Valid {
		log["duration_ms"] = row.DurationMs.Int32
	}
	if row.ErrorCode.Valid {
		log["error_code"] = row.ErrorCode.String
	}

	return map[string]any{
		"type":         "logs.new",
		"workspace_id": row.WorkspaceID,
		"log":          log,
	}
}

// ProjectLogEnvelope applies the read mode at the WebSocket consumer
// boundary. Producers and SSE subscribers always retain the numeric legacy
// integration envelope, while v2 WebSockets receive source-qualified rows
// without the duplicate legacy api_request projection.
func ProjectLogEnvelope(payload []byte, useV2 bool) ([]byte, bool) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var envelope struct {
		Type        string         `json:"type"`
		WorkspaceID string         `json:"workspace_id"`
		Log         map[string]any `json:"log"`
	}
	if err := decoder.Decode(&envelope); err != nil || envelope.Type != "logs.new" || envelope.WorkspaceID == "" || envelope.Log == nil {
		return nil, false
	}
	switch id := envelope.Log["id"].(type) {
	case json.Number:
		parsed, err := id.Int64()
		if err != nil || parsed <= 0 {
			return nil, false
		}
		if !useV2 {
			return payload, true
		}
		if envelope.Log["category"] == "api_request" {
			return nil, false
		}
		envelope.Log["id"] = "integration:" + strconv.FormatInt(parsed, 10)
	case string:
		if !useV2 || !strings.HasPrefix(id, "request:") || strings.TrimPrefix(id, "request:") == "" || strings.TrimSpace(id) != id {
			return nil, false
		}
	default:
		return nil, false
	}
	projected, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}
	return projected, true
}

// RequestEventLogEnvelope projects a durably persisted canonical event into
// the same metadata-only shape as v2 list/detail reads.
func RequestEventLogEnvelope(event requestevents.Event) map[string]any {
	level := "info"
	if event.StatusCode >= 500 {
		level = "error"
	} else if event.StatusCode >= 400 {
		level = "warn"
	}
	status := "error"
	action := "api.request.failed"
	if event.Outcome == requestevents.OutcomeSuccess {
		status = "success"
		action = "api.request.succeeded"
	} else if event.Outcome == requestevents.OutcomeValidationError {
		action = "api.request.validation_failed"
	} else if event.Outcome == requestevents.OutcomeRateLimited {
		action = "api.request.rate_limited"
	}
	log := map[string]any{
		"id":               "request:" + event.ID,
		"workspace_id":     event.WorkspaceID,
		"ts":               event.OccurredAt.UTC(),
		"level":            level,
		"status":           status,
		"category":         "api_request",
		"action":           action,
		"source":           "api",
		"message":          fmt.Sprintf("%s %s returned %d.", event.Method, event.RoutePattern, event.StatusCode),
		"actor_api_key_id": event.APIKeyID,
		"endpoint":         event.RoutePattern,
		"method":           event.Method,
		"http_status_code": event.StatusCode,
		"duration_ms":      event.DurationMS,
		"has_error_detail": event.HasErrorDetail,
	}
	for key, value := range map[string]string{
		"request_id": event.RequestID, "trace_id": event.TraceID, "profile_id": event.ProfileID,
		"social_account_id": event.SocialAccountID, "post_id": event.PostID, "error_code": event.ErrorCode,
	} {
		if value != "" {
			log[key] = value
		}
	}
	return map[string]any{"type": "logs.new", "workspace_id": event.WorkspaceID, "log": log}
}
