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
	UsageModes             []string  `json:"usage_modes"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func toWorkspaceResponse(w db.Workspace) workspaceResponse {
	modes := w.UsageModes
	if modes == nil {
		modes = []string{}
	}
	resp := workspaceResponse{
		ID:         w.ID,
		Name:       w.Name,
		UsageModes: modes,
		CreatedAt:  w.CreatedAt.Time,
		UpdatedAt:  w.UpdatedAt.Time,
	}
	if w.PerAccountMonthlyLimit.Valid {
		v := w.PerAccountMonthlyLimit.Int32
		resp.PerAccountMonthlyLimit = &v
	}
	return resp
}

// Get returns the workspace bound to the authenticated caller. Workspace
// is resolved by DualAuthMiddleware: API-key callers get the key's workspace,
// Clerk callers get the default workspace for the authenticated user.
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

// Update mutates the authenticated caller's workspace. Accepts name and
// per_account_monthly_limit; both optional.
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	var body struct {
		Name                   *string `json:"name"`
		PerAccountMonthlyLimit *int32  `json:"per_account_monthly_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.Name != nil && *body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name must not be empty")
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

	if body.Name != nil {
		if _, err := h.queries.UpdateWorkspace(r.Context(), db.UpdateWorkspaceParams{
			ID: workspaceID, Name: *body.Name,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update workspace")
			return
		}
	}
	if body.PerAccountMonthlyLimit != nil {
		var limitParam pgtype.Int4
		limitParam = pgtype.Int4{Int32: *body.PerAccountMonthlyLimit, Valid: true}
		if _, err := h.queries.UpdateWorkspacePerAccountQuota(r.Context(), db.UpdateWorkspacePerAccountQuotaParams{
			ID:                     workspaceID,
			PerAccountMonthlyLimit: limitParam,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update workspace quota")
			return
		}
	}

	ws, err := h.queries.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get workspace")
		return
	}
	writeSuccess(w, toWorkspaceResponse(ws))
}
