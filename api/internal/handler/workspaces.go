package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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

// ── API key auth routes ─────────────────────────────────────────────

// Get returns the workspace associated with the current API key.
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

// Update updates the workspace quota via API key auth.
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

// ── Dashboard (Clerk session auth) ──────────────────────────────────

// DashboardList returns all workspaces owned by the authenticated user.
func (h *WorkspaceHandler) DashboardList(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	workspaces, err := h.queries.ListWorkspacesByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list workspaces")
		return
	}
	result := make([]workspaceResponse, len(workspaces))
	for i, ws := range workspaces {
		result[i] = toWorkspaceResponse(ws)
	}
	writeSuccessWithListMeta(w, result, len(result), len(result))
}

// DashboardGet returns a single workspace after verifying ownership.
func (h *WorkspaceHandler) DashboardGet(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	workspaceID := chi.URLParam(r, "workspaceID")
	ws, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID: workspaceID, UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get workspace")
		return
	}
	writeSuccess(w, toWorkspaceResponse(ws))
}

// DashboardUpdate updates the workspace name after verifying ownership.
func (h *WorkspaceHandler) DashboardUpdate(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	workspaceID := chi.URLParam(r, "workspaceID")
	_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID: workspaceID, UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify workspace")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name is required")
		return
	}
	ws, err := h.queries.UpdateWorkspace(r.Context(), db.UpdateWorkspaceParams{
		ID: workspaceID, Name: body.Name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update workspace")
		return
	}
	writeSuccess(w, toWorkspaceResponse(ws))
}
