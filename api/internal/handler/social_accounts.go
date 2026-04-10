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
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type SocialAccountHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	bus       events.EventBus
}

func NewSocialAccountHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, bus events.EventBus) *SocialAccountHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &SocialAccountHandler{queries: queries, encryptor: encryptor, bus: bus}
}

type socialAccountResponse struct {
	ID               string    `json:"id"`
	Platform         string    `json:"platform"`
	AccountName      *string   `json:"account_name"`
	ConnectedAt      time.Time `json:"connected_at"`
	Status           string    `json:"status"`
	ConnectionType   string    `json:"connection_type"`
	ExternalUserID   *string   `json:"external_user_id,omitempty"`
	ExternalUserEmail *string  `json:"external_user_email,omitempty"`
}

func toSocialAccountResponse(a db.SocialAccount) socialAccountResponse {
	// Sprint 3: status comes from the column directly. Refresh worker
	// flips it to reconnect_required when a managed token can't be
	// refreshed; the dashboard surfaces that to prompt re-Connect.
	status := a.Status
	if status == "" {
		status = "active"
	}
	if a.DisconnectedAt.Valid {
		status = "disconnected"
	}
	var name *string
	if a.AccountName.Valid {
		name = &a.AccountName.String
	}
	var extUserID *string
	if a.ExternalUserID.Valid {
		extUserID = &a.ExternalUserID.String
	}
	var extUserEmail *string
	if a.ExternalUserEmail.Valid {
		extUserEmail = &a.ExternalUserEmail.String
	}
	return socialAccountResponse{
		ID:                a.ID,
		Platform:          a.Platform,
		AccountName:       name,
		ConnectedAt:       a.ConnectedAt.Time,
		Status:            status,
		ConnectionType:    a.ConnectionType,
		ExternalUserID:    extUserID,
		ExternalUserEmail: extUserEmail,
	}
}

// Connect handles POST /v1/social-accounts/connect (API key auth)
// and POST /v1/profiles/{profileID}/social-accounts/connect (Clerk auth)
func (h *SocialAccountHandler) Connect(w http.ResponseWriter, r *http.Request) {
	profileID := h.getProfileID(r)
	if profileID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing profile context")
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
		ProfileID:         profileID,
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
//
// Sprint 3 PR1: optional query filters `external_user_id` and `platform`
// let customers find rows created by a Connect flow. Both are optional —
// passing neither preserves the existing "all accounts in project" shape.
func (h *SocialAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	profileID := h.getProfileID(r)
	if profileID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing profile context")
		return
	}

	extUserID := r.URL.Query().Get("external_user_id")
	platformFilter := r.URL.Query().Get("platform")

	var accounts []db.SocialAccount
	var err error
	if extUserID == "" && platformFilter == "" {
		accounts, err = h.queries.ListSocialAccountsByProfile(r.Context(), profileID)
	} else {
		accounts, err = h.queries.ListSocialAccountsByProfileFiltered(r.Context(), db.ListSocialAccountsByProfileFilteredParams{
			ProfileID:      profileID,
			ExternalUserID: pgtype.Text{String: extUserID, Valid: extUserID != ""},
			Platform:       pgtype.Text{String: platformFilter, Valid: platformFilter != ""},
		})
	}
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
	profileID := h.getProfileID(r)
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	disconnected, err := h.queries.DisconnectSocialAccount(r.Context(), db.DisconnectSocialAccountParams{
		ID:        accountID,
		ProfileID: profileID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to disconnect account")
		return
	}

	// Fan out account.disconnected webhook. Best-effort — never blocks.
	accountName := ""
	if disconnected.AccountName.Valid {
		accountName = disconnected.AccountName.String
	}
	// Webhooks are workspace-scoped; resolve workspace_id from profile.
	wsID := profileID
	if prof, pErr := h.queries.GetProfile(r.Context(), profileID); pErr == nil {
		wsID = prof.WorkspaceID
	}
	h.bus.Publish(r.Context(), wsID, events.EventAccountDisconnected, map[string]any{
		"social_account_id": disconnected.ID,
		"platform":          disconnected.Platform,
		"account_name":      accountName,
		"disconnected_at":   time.Now().UTC().Format(time.RFC3339),
		"reason":            "user_initiated",
	})

	writeSuccess(w, map[string]bool{"disconnected": true})
}

// getProfileID extracts profile ID from API key context or URL param (dashboard routes).
func (h *SocialAccountHandler) getProfileID(r *http.Request) string {
	if pid := auth.GetWorkspaceID(r.Context()); pid != "" {
		return pid
	}
	// Dashboard route: verify ownership via URL param
	profileID := chi.URLParam(r, "profileID")
	if profileID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	_, err := h.queries.GetProfileByIDAndWorkspaceOwner(r.Context(), db.GetProfileByIDAndWorkspaceOwnerParams{
		ID:     profileID,
		UserID: userID,
	})
	if err != nil {
		return ""
	}
	return profileID
}
