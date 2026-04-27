package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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

// requireWorkspace returns the workspace ID stamped into the request
// context by DualAuthMiddleware (API-key path → key's workspace; Clerk
// path → user's default workspace).
func (h *PlatformCredentialHandler) requireWorkspace(r *http.Request, w http.ResponseWriter) (string, bool) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return "", false
	}
	return workspaceID, true
}

// Create handles POST /v1/platform-credentials
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

// List handles GET /v1/platform-credentials
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

// Delete handles DELETE /v1/platform-credentials/{platform}
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
