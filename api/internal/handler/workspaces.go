package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type WorkspaceHandler struct {
	queries *db.Queries
}

func NewWorkspaceHandler(queries *db.Queries) *WorkspaceHandler {
	return &WorkspaceHandler{queries: queries}
}

type workspaceResponse struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	PerAccountMonthlyLimit *int32    `json:"per_account_monthly_limit"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func toWorkspaceResponse(w db.Workspace) workspaceResponse {
	resp := workspaceResponse{
		ID:        w.ID,
		Name:      w.Name,
		CreatedAt: w.CreatedAt.Time,
		UpdatedAt: w.UpdatedAt.Time,
	}
	if w.PerAccountMonthlyLimit.Valid {
		v := w.PerAccountMonthlyLimit.Int32
		resp.PerAccountMonthlyLimit = &v
	}
	return resp
}

// Get returns the workspace associated with the current API key.
// API key auth route: GET /v1/workspace
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	ws, err := h.queries.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get workspace")
		return
	}

	writeSuccess(w, toWorkspaceResponse(ws))
}

// Update updates the workspace associated with the current API key.
// API key auth route: PATCH /v1/workspace
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body struct {
		PerAccountMonthlyLimit *int32 `json:"per_account_monthly_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.PerAccountMonthlyLimit != nil {
		limit := *body.PerAccountMonthlyLimit
		if limit < 0 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "per_account_monthly_limit must be >= 0")
			return
		}
		if limit > 1_000_000 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "per_account_monthly_limit must be <= 1,000,000")
			return
		}
	}

	var limitParam pgtype.Int4
	if body.PerAccountMonthlyLimit != nil {
		limitParam = pgtype.Int4{Int32: *body.PerAccountMonthlyLimit, Valid: true}
	}

	ws, err := h.queries.UpdateWorkspacePerAccountQuota(r.Context(), db.UpdateWorkspacePerAccountQuotaParams{
		ID:                     workspaceID,
		PerAccountMonthlyLimit: limitParam,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update workspace")
		return
	}

	writeSuccess(w, toWorkspaceResponse(ws))
}
