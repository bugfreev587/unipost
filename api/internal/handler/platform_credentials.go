package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PlatformCredentialHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewPlatformCredentialHandler(queries *db.Queries, encryptor *crypto.AESEncryptor) *PlatformCredentialHandler {
	return &PlatformCredentialHandler{queries: queries, encryptor: encryptor}
}

type platformCredentialResponse struct {
	Platform  string    `json:"platform"`
	ClientID  string    `json:"client_id"`
	CreatedAt time.Time `json:"created_at"`
}

// requireWorkspace resolves the workspace id from the URL and
// validates the caller is allowed to act on it. Supports both auth
// modes:
//
//	Clerk  (Dashboard):  looks up ownership via GetWorkspaceByIDAndOwner
//	API key (integrator): the middleware has already bound an API key to
//	                       one workspace; we just enforce the URL matches.
func (h *PlatformCredentialHandler) requireWorkspace(r *http.Request, w http.ResponseWriter) (string, bool) {
	workspaceID := chi.URLParam(r, "workspaceID")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing workspace id")
		return "", false
	}
	if userID := auth.GetUserID(r.Context()); userID != "" {
		_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
			ID:     workspaceID,
			UserID: userID,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
				return "", false
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify workspace")
			return "", false
		}
		return workspaceID, true
	}
	boundWsID := auth.GetWorkspaceID(r.Context())
	if boundWsID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing auth context")
		return "", false
	}
	if boundWsID != workspaceID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
		return "", false
	}
	return workspaceID, true
}

// Create handles POST /v1/workspaces/{workspaceID}/platform-credentials
func (h *PlatformCredentialHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.requireWorkspace(r, w)
	if !ok {
		return
	}

	// Native mode requires a paid plan
	sub, _ := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID)
	if sub.PlanID == "free" || sub.PlanID == "" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Native mode requires a paid plan. Please upgrade to use your own platform credentials.")
		return
	}

	var body struct {
		Platform     string `json:"platform"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Platform == "" || body.ClientID == "" || body.ClientSecret == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "platform, client_id, and client_secret are required")
		return
	}

	encSecret, err := h.encryptor.Encrypt(body.ClientSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encrypt credentials")
		return
	}

	cred, err := h.queries.CreatePlatformCredential(r.Context(), db.CreatePlatformCredentialParams{
		WorkspaceID:  workspaceID,
		Platform:     body.Platform,
		ClientID:     body.ClientID,
		ClientSecret: encSecret,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save credentials")
		return
	}

	writeCreated(w, platformCredentialResponse{
		Platform:  cred.Platform,
		ClientID:  cred.ClientID,
		CreatedAt: cred.CreatedAt.Time,
	})
}

// List handles GET /v1/workspaces/{workspaceID}/platform-credentials
func (h *PlatformCredentialHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.requireWorkspace(r, w)
	if !ok {
		return
	}

	creds, err := h.queries.ListPlatformCredentialsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list credentials")
		return
	}

	result := make([]platformCredentialResponse, len(creds))
	for i, c := range creds {
		result[i] = platformCredentialResponse{
			Platform:  c.Platform,
			ClientID:  c.ClientID,
			CreatedAt: c.CreatedAt.Time,
		}
	}

	writeSuccessWithListMeta(w, result, len(result), len(result))
}

// Delete handles DELETE /v1/workspaces/{workspaceID}/platform-credentials/{platform}
func (h *PlatformCredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.requireWorkspace(r, w)
	if !ok {
		return
	}
	platformName := chi.URLParam(r, "platform")

	h.queries.DeletePlatformCredential(r.Context(), db.DeletePlatformCredentialParams{
		WorkspaceID: workspaceID,
		Platform:    platformName,
	})

	w.WriteHeader(http.StatusNoContent)
}
