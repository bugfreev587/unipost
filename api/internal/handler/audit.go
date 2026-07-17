// audit.go — read endpoint for the audit_log table. Writes happen via
// internal/audit.Log() at every mutation site; this handler is the
// dashboard's window into those rows.
//
// GET /v1/audit-log?action=...&category=...&days=N&limit=N
//
// Available to any authenticated member of a Team or Enterprise workspace.
// The router owns the plan gate; this handler owns workspace scoping.
// Filters are optional — empty values mean "no filter on that axis".

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type AuditHandler struct {
	queries *db.Queries
}

func NewAuditHandler(queries *db.Queries) *AuditHandler {
	return &AuditHandler{queries: queries}
}

type auditLogEntry struct {
	ID            int64           `json:"id"`
	ActorUserID   string          `json:"actor_user_id,omitempty"`
	ActorAPIKeyID string          `json:"actor_api_key_id,omitempty"`
	Action        string          `json:"action"`
	ResourceType  string          `json:"resource_type"`
	ResourceID    string          `json:"resource_id,omitempty"`
	Category      string          `json:"category"`
	IPAddress     string          `json:"ip_address,omitempty"`
	Before        json.RawMessage `json:"before,omitempty"`
	After         json.RawMessage `json:"after,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	q := r.URL.Query()
	action := q.Get("action")
	category := q.Get("category")
	days, _ := strconv.Atoi(q.Get("days"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if days <= 0 {
		days = 30
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	since := time.Now().AddDate(0, 0, -days)

	rows, err := h.queries.ListAuditLogByWorkspace(r.Context(), db.ListAuditLogByWorkspaceParams{
		WorkspaceID: workspaceID,
		Column2:     action,
		Column3:     category,
		CreatedAt:   pgtype.Timestamptz{Time: since, Valid: true},
		Limit:       int32(limit),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load audit log: "+err.Error())
		return
	}

	out := make([]auditLogEntry, 0, len(rows))
	for _, row := range rows {
		entry := auditLogEntry{
			ID:           row.ID,
			Action:       row.Action,
			ResourceType: row.ResourceType,
			Category:     row.Category,
			CreatedAt:    row.CreatedAt.Time,
			Before:       row.BeforeJson,
			After:        row.AfterJson,
			Metadata:     row.Metadata,
		}
		if row.ActorUserID.Valid {
			entry.ActorUserID = row.ActorUserID.String
		}
		if row.ActorApiKeyID.Valid {
			entry.ActorAPIKeyID = row.ActorApiKeyID.String
		}
		if row.ResourceID.Valid {
			entry.ResourceID = row.ResourceID.String
		}
		if row.IpAddress.Valid {
			entry.IPAddress = row.IpAddress.String
		}
		out = append(out, entry)
	}
	writeSuccess(w, out)
}
