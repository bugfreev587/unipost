package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// logsStore is the subset of *db.Queries the logs handlers need. An
// interface keeps the handlers unit-testable with a fake store.
type logsStore interface {
	ListIntegrationLogs(context.Context, db.ListIntegrationLogsParams) ([]db.IntegrationLog, error)
	GetIntegrationLog(context.Context, db.GetIntegrationLogParams) (db.IntegrationLog, error)
}

type LogsHandler struct {
	queries logsStore
}

func NewLogsHandler(queries logsStore) *LogsHandler {
	return &LogsHandler{queries: queries}
}

// encodeLogCursor produces an opaque, reversible page position. It
// encodes only the last seen (ts, id), never raw SQL or workspace
// state, so it is safe to hand back to customers.
func encodeLogCursor(ts time.Time, id int64) string {
	raw := fmt.Sprintf("%d:%d", ts.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeLogCursor(s string) (time.Time, int64, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, 0, err
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, errors.New("malformed cursor")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	return time.Unix(0, nanos).UTC(), id, nil
}

type integrationLogResponse struct {
	ID               int64           `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	TS               time.Time       `json:"ts"`
	Level            string          `json:"level"`
	Status           string          `json:"status"`
	Category         string          `json:"category"`
	Action           string          `json:"action"`
	Source           string          `json:"source"`
	Message          string          `json:"message"`
	RequestID        string          `json:"request_id,omitempty"`
	TraceID          string          `json:"trace_id,omitempty"`
	ActorUserID      string          `json:"actor_user_id,omitempty"`
	ActorAPIKeyID    string          `json:"actor_api_key_id,omitempty"`
	ProfileID        string          `json:"profile_id,omitempty"`
	SocialAccountID  string          `json:"social_account_id,omitempty"`
	PostID           string          `json:"post_id,omitempty"`
	PlatformPostID   string          `json:"platform_post_id,omitempty"`
	Platform         string          `json:"platform,omitempty"`
	Endpoint         string          `json:"endpoint,omitempty"`
	Method           string          `json:"method,omitempty"`
	HTTPStatusCode   *int32          `json:"http_status_code,omitempty"`
	RemoteStatusCode *int32          `json:"remote_status_code,omitempty"`
	DurationMs       *int32          `json:"duration_ms,omitempty"`
	ErrorCode        string          `json:"error_code,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	RequestPayload   json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload  json.RawMessage `json:"response_payload,omitempty"`
}

func (h *LogsHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	from := parseLogTime(q.Get("from"), time.Now().AddDate(0, 0, -7))
	to := parseLogTime(q.Get("to"), time.Now())

	cursorTs := pgtype.Timestamptz{Valid: false}
	var cursorID int64
	if raw := strings.TrimSpace(q.Get("cursor")); raw != "" {
		ts, id, err := decodeLogCursor(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid cursor")
			return
		}
		cursorTs = pgtype.Timestamptz{Time: ts, Valid: true}
		cursorID = id
	}

	// Fetch one extra row to detect whether another page exists.
	rows, err := h.queries.ListIntegrationLogs(r.Context(), db.ListIntegrationLogsParams{
		WorkspaceID:     workspaceID,
		Category:        strings.TrimSpace(q.Get("category")),
		Action:          strings.TrimSpace(q.Get("action")),
		Source:          strings.TrimSpace(q.Get("source")),
		Level:           strings.TrimSpace(q.Get("level")),
		Status:          strings.TrimSpace(q.Get("status")),
		Platform:        strings.TrimSpace(q.Get("platform")),
		ProfileID:       strings.TrimSpace(q.Get("profile_id")),
		SocialAccountID: strings.TrimSpace(q.Get("social_account_id")),
		PostID:          strings.TrimSpace(q.Get("post_id")),
		RequestID:       strings.TrimSpace(q.Get("request_id")),
		ErrorCode:       strings.TrimSpace(q.Get("error_code")),
		Query:           strings.TrimSpace(q.Get("q")),
		FromTs:          pgtype.Timestamptz{Time: from, Valid: true},
		ToTs:            pgtype.Timestamptz{Time: to, Valid: true},
		CursorTs:        cursorTs,
		CursorID:        cursorID,
		Limit:           int32(limit + 1),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load logs: "+err.Error())
		return
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]integrationLogResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toIntegrationLogResponse(row, false))
	}

	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeLogCursor(last.Ts.Time, last.ID)
	}
	writeSuccessWithCursor(w, out, nextCursor, hasMore, limit)
}

func (h *LogsHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid log id")
		return
	}

	row, err := h.queries.GetIntegrationLog(r.Context(), db.GetIntegrationLogParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Log not found")
		return
	}

	writeSuccess(w, toIntegrationLogResponse(row, true))
}

func parseLogTime(raw string, fallback time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback.UTC()
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return fallback.UTC()
	}
	return parsed.UTC()
}

func toIntegrationLogResponse(row db.IntegrationLog, includePayloads bool) integrationLogResponse {
	resp := integrationLogResponse{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		TS:          row.Ts.Time,
		Level:       row.Level,
		Status:      row.Status,
		Category:    row.Category,
		Action:      row.Action,
		Source:      row.Source,
		Message:     row.Message,
		Metadata:    row.Metadata,
	}

	if row.RequestID.Valid {
		resp.RequestID = row.RequestID.String
	}
	if row.TraceID.Valid {
		resp.TraceID = row.TraceID.String
	}
	if row.ActorUserID.Valid {
		resp.ActorUserID = row.ActorUserID.String
	}
	if row.ActorApiKeyID.Valid {
		resp.ActorAPIKeyID = row.ActorApiKeyID.String
	}
	if row.ProfileID.Valid {
		resp.ProfileID = row.ProfileID.String
	}
	if row.SocialAccountID.Valid {
		resp.SocialAccountID = row.SocialAccountID.String
	}
	if row.PostID.Valid {
		resp.PostID = row.PostID.String
	}
	if row.PlatformPostID.Valid {
		resp.PlatformPostID = row.PlatformPostID.String
	}
	if row.Platform.Valid {
		resp.Platform = row.Platform.String
	}
	if row.Endpoint.Valid {
		resp.Endpoint = row.Endpoint.String
	}
	if row.Method.Valid {
		resp.Method = row.Method.String
	}
	if row.HTTPStatusCode.Valid {
		v := row.HTTPStatusCode.Int32
		resp.HTTPStatusCode = &v
	}
	if row.RemoteStatusCode.Valid {
		v := row.RemoteStatusCode.Int32
		resp.RemoteStatusCode = &v
	}
	if row.DurationMs.Valid {
		v := row.DurationMs.Int32
		resp.DurationMs = &v
	}
	if row.ErrorCode.Valid {
		resp.ErrorCode = row.ErrorCode.String
	}
	if includePayloads {
		resp.RequestPayload = row.RequestPayload
		resp.ResponsePayload = row.ResponsePayload
	}
	return resp
}
