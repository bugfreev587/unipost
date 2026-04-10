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

// Create handles POST /v1/projects/{projectID}/platform-credentials
func (h *PlatformCredentialHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	projectID := chi.URLParam(r, "workspaceID")

	_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID:     projectID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify project")
		return
	}

	// Native mode requires a paid plan
	sub, _ := h.queries.GetSubscriptionByWorkspace(r.Context(), projectID)
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
		WorkspaceID:    projectID,
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

// List handles GET /v1/projects/{projectID}/platform-credentials
func (h *PlatformCredentialHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	projectID := chi.URLParam(r, "workspaceID")

	_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID:     projectID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify project")
		return
	}

	creds, err := h.queries.ListPlatformCredentialsByWorkspace(r.Context(), projectID)
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

	writeSuccessWithMeta(w, result, len(result))
}

// Delete handles DELETE /v1/projects/{projectID}/platform-credentials/{platform}
func (h *PlatformCredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	projectID := chi.URLParam(r, "workspaceID")
	platformName := chi.URLParam(r, "platform")

	_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID:     projectID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify project")
		return
	}

	h.queries.DeletePlatformCredential(r.Context(), db.DeletePlatformCredentialParams{
		WorkspaceID: projectID,
		Platform:  platformName,
	})

	w.WriteHeader(http.StatusNoContent)
}
