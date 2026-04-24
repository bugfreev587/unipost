package handler

import (
	"context"
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
	ID                string    `json:"id"`
	ProfileID         string    `json:"profile_id"`
	ProfileName       string    `json:"profile_name"`
	Platform          string    `json:"platform"`
	AccountName       *string   `json:"account_name"`
	ConnectedAt       time.Time `json:"connected_at"`
	Status            string    `json:"status"`
	ConnectionType    string    `json:"connection_type"`
	ExternalUserID    *string   `json:"external_user_id,omitempty"`
	ExternalUserEmail *string   `json:"external_user_email,omitempty"`
}

func toSocialAccountResponse(a db.SocialAccount, profileName ...string) socialAccountResponse {
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
	pName := ""
	if len(profileName) > 0 {
		pName = profileName[0]
	}
	return socialAccountResponse{
		ID:                a.ID,
		ProfileID:         a.ProfileID,
		ProfileName:       pName,
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
	var body struct {
		ProfileID   string            `json:"profile_id"`
		Platform    string            `json:"platform"`
		Credentials map[string]string `json:"credentials"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	profileID := h.resolveProfileID(r, body.ProfileID)
	if profileID == "" {
		if workspaceID := auth.GetWorkspaceID(r.Context()); workspaceID != "" {
			if profileErr := h.resolveAPIProfileError(r.Context(), workspaceID, body.ProfileID); profileErr != nil {
				writeError(w, profileErr.status, profileErr.code, profileErr.msg)
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing profile context")
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

	// Dedup: check if this platform account is already connected in the workspace
	if result.ExternalAccountID != "" {
		profile, err := h.queries.GetProfile(r.Context(), profileID)
		if err == nil {
			existing, err := h.queries.FindSocialAccountByExternalID(r.Context(), db.FindSocialAccountByExternalIDParams{
				Platform:          body.Platform,
				ExternalAccountID: result.ExternalAccountID,
				WorkspaceID:       profile.WorkspaceID,
			})
			if err == nil && existing.ID != "" {
				existingName := ""
				if existing.AccountName.Valid {
					existingName = existing.AccountName.String
				}
				writeError(w, http.StatusConflict, "ACCOUNT_ALREADY_CONNECTED",
					"This "+body.Platform+" account ("+existingName+") is already connected in your workspace. Disconnect the existing one first if you want to reconnect.")
				return
			}
		}
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

	wsID := profileID
	if prof, pErr := h.queries.GetProfile(r.Context(), profileID); pErr == nil {
		wsID = prof.WorkspaceID
	}
	h.bus.Publish(r.Context(), wsID, events.EventAccountConnected, map[string]any{
		"social_account_id": account.ID,
		"profile_id":        profileID,
		"platform":          body.Platform,
		"account_name":      result.AccountName,
		"connection_type":   account.ConnectionType,
	})
}

// List handles GET /v1/social-accounts (API key) and
// GET /v1/profiles/{profileID}/social-accounts (dashboard).
//
// API key path: lists all accounts across all profiles in the workspace.
// Dashboard path: lists accounts for a specific profile.
func (h *SocialAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	extUserID := r.URL.Query().Get("external_user_id")
	platformFilter := r.URL.Query().Get("platform")
	profileIDFilter := r.URL.Query().Get("profile_id")

	var accounts []db.SocialAccount
	var err error

	// API key path — workspace-scoped (all profiles)
	if workspaceID := auth.GetWorkspaceID(r.Context()); workspaceID != "" {
		if extUserID == "" && platformFilter == "" && profileIDFilter == "" {
			accounts, err = h.queries.ListSocialAccountsByWorkspace(r.Context(), workspaceID)
		} else {
			accounts, err = h.queries.ListSocialAccountsByWorkspaceFiltered(r.Context(), db.ListSocialAccountsByWorkspaceFilteredParams{
				WorkspaceID:    workspaceID,
				ProfileID:      pgtype.Text{String: profileIDFilter, Valid: profileIDFilter != ""},
				ExternalUserID: pgtype.Text{String: extUserID, Valid: extUserID != ""},
				Platform:       pgtype.Text{String: platformFilter, Valid: platformFilter != ""},
			})
		}
	} else {
		// Dashboard path — profile-scoped
		profileID := h.getProfileID(r)
		if profileID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing profile context")
			return
		}
		if extUserID == "" && platformFilter == "" {
			accounts, err = h.queries.ListSocialAccountsByProfile(r.Context(), profileID)
		} else {
			accounts, err = h.queries.ListSocialAccountsByProfileFiltered(r.Context(), db.ListSocialAccountsByProfileFilteredParams{
				ProfileID:      profileID,
				ExternalUserID: pgtype.Text{String: extUserID, Valid: extUserID != ""},
				Platform:       pgtype.Text{String: platformFilter, Valid: platformFilter != ""},
			})
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list accounts")
		return
	}

	// Build profile name map for denormalized response
	profileNames := make(map[string]string)
	for _, a := range accounts {
		if _, ok := profileNames[a.ProfileID]; !ok {
			if p, pErr := h.queries.GetProfile(r.Context(), a.ProfileID); pErr == nil {
				profileNames[p.ID] = p.Name
			}
		}
	}

	result := make([]socialAccountResponse, len(accounts))
	for i, a := range accounts {
		result[i] = toSocialAccountResponse(a, profileNames[a.ProfileID])
	}

	writeSuccessWithListMeta(w, result, len(result), len(result))
}

// Disconnect handles DELETE /v1/social-accounts/{id}
func (h *SocialAccountHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	// Resolve the profile_id for the account — API key path verifies via
	// workspace, dashboard path verifies via profile URL param.
	var profileID string
	if workspaceID := auth.GetWorkspaceID(r.Context()); workspaceID != "" {
		// API key path: verify the account belongs to this workspace
		acc, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
			ID:          accountID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
			return
		}
		profileID = acc.ProfileID
	} else {
		profileID = h.getProfileID(r)
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
		"profile_id":        profileID,
		"platform":          disconnected.Platform,
		"account_name":      accountName,
		"disconnected_at":   time.Now().UTC().Format(time.RFC3339),
		"reason":            "user_initiated",
	})

	writeSuccess(w, map[string]bool{"disconnected": true})
}

// getProfileID extracts profile ID from URL param (dashboard routes only).
func (h *SocialAccountHandler) getProfileID(r *http.Request) string {
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

func (h *SocialAccountHandler) resolveAPIProfileError(ctx context.Context, workspaceID, requestedID string) *httpError {
	if _, profileErr := resolveProfileForWorkspace(ctx, h.queries, workspaceID, requestedID); profileErr != nil {
		return profileErr
	}
	return nil
}

// resolveProfileID returns a profile ID for creating accounts.
// API key path: explicit profile_id or single-profile fallback.
// Dashboard path: uses the profile ID from the URL.
func (h *SocialAccountHandler) resolveProfileID(r *http.Request, requestedID string) string {
	if workspaceID := auth.GetWorkspaceID(r.Context()); workspaceID != "" {
		profileID, profileErr := resolveProfileForWorkspace(r.Context(), h.queries, workspaceID, requestedID)
		if profileErr != nil {
			return ""
		}
		return profileID
	}
	return h.getProfileID(r)
}
