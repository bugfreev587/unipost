package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type SocialAccountHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewSocialAccountHandler(queries *db.Queries, encryptor *crypto.AESEncryptor) *SocialAccountHandler {
	return &SocialAccountHandler{queries: queries, encryptor: encryptor}
}

type socialAccountResponse struct {
	ID          string    `json:"id"`
	Platform    string    `json:"platform"`
	AccountName *string   `json:"account_name"`
	ConnectedAt time.Time `json:"connected_at"`
	Status      string    `json:"status"`
}

func toSocialAccountResponse(a db.SocialAccount) socialAccountResponse {
	status := "active"
	if a.TokenExpiresAt.Valid && a.TokenExpiresAt.Time.Before(time.Now()) {
		status = "reconnect_required"
	}
	var name *string
	if a.AccountName.Valid {
		name = &a.AccountName.String
	}
	return socialAccountResponse{
		ID:          a.ID,
		Platform:    a.Platform,
		AccountName: name,
		ConnectedAt: a.ConnectedAt.Time,
		Status:      status,
	}
}

// Connect handles POST /v1/social-accounts/connect (API key auth)
// and POST /v1/projects/{projectID}/social-accounts/connect (Clerk auth)
func (h *SocialAccountHandler) Connect(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	var body struct {
		Platform    string            `json:"platform"`
		Credentials map[string]string `json:"credentials"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Platform == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Platform is required")
		return
	}

	adapter, err := platform.Get(body.Platform)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}

	result, err := adapter.Connect(r.Context(), body.Credentials)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Failed to connect: "+err.Error())
		return
	}

	// Encrypt tokens
	encAccess, err := h.encryptor.Encrypt(result.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encrypt token")
		return
	}
	encRefresh, err := h.encryptor.Encrypt(result.RefreshToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encrypt token")
		return
	}

	metadataJSON, _ := json.Marshal(result.Metadata)

	account, err := h.queries.CreateSocialAccount(r.Context(), db.CreateSocialAccountParams{
		ProjectID:         projectID,
		Platform:          body.Platform,
		AccessToken:       encAccess,
		RefreshToken:      pgtype.Text{String: encRefresh, Valid: true},
		TokenExpiresAt:    pgtype.Timestamptz{Time: result.TokenExpiresAt, Valid: true},
		ExternalAccountID: result.ExternalAccountID,
		AccountName:       pgtype.Text{String: result.AccountName, Valid: result.AccountName != ""},
		AccountAvatarUrl:  pgtype.Text{String: result.AvatarURL, Valid: result.AvatarURL != ""},
		Metadata:          metadataJSON,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save account")
		return
	}

	writeCreated(w, toSocialAccountResponse(account))
}

// List handles GET /v1/social-accounts
func (h *SocialAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	accounts, err := h.queries.ListSocialAccountsByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list accounts")
		return
	}

	result := make([]socialAccountResponse, len(accounts))
	for i, a := range accounts {
		result[i] = toSocialAccountResponse(a)
	}

	writeSuccessWithMeta(w, result, len(result))
}

// Disconnect handles DELETE /v1/social-accounts/{id}
func (h *SocialAccountHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	_, err := h.queries.DisconnectSocialAccount(r.Context(), db.DisconnectSocialAccountParams{
		ID:        accountID,
		ProjectID: projectID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to disconnect account")
		return
	}

	writeSuccess(w, map[string]bool{"disconnected": true})
}

// getProjectID extracts project ID from API key context or URL param (dashboard routes).
func (h *SocialAccountHandler) getProjectID(r *http.Request) string {
	if pid := auth.GetProjectID(r.Context()); pid != "" {
		return pid
	}
	// Dashboard route: verify ownership via URL param
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID:      projectID,
		OwnerID: userID,
	})
	if err != nil {
		return ""
	}
	return projectID
}
