package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type OAuthHandler struct {
	queries           *db.Queries
	encryptor         *crypto.AESEncryptor
	baseRedirectURL   string
	// superAdminChecker gates the Facebook Pages detour. A nil checker
	// behaves as "no super admins configured" — every FB attempt
	// returns 403. Non-FB platforms ignore it.
	superAdminChecker *auth.SuperAdminChecker
}

func NewOAuthHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, superAdmins *auth.SuperAdminChecker) *OAuthHandler {
	baseURL := os.Getenv("OAUTH_REDIRECT_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.unipost.dev"
	}
	return &OAuthHandler{
		queries:           queries,
		encryptor:         encryptor,
		baseRedirectURL:   baseURL,
		superAdminChecker: superAdmins,
	}
}

// Connect handles GET /v1/oauth/connect/{platform}
// Returns the authorization URL for the user to redirect to.
func (h *OAuthHandler) Connect(w http.ResponseWriter, r *http.Request) {
	platformName := chi.URLParam(r, "platform")
	redirectURL := r.URL.Query().Get("redirect_url")

	// Super-admin-only gate for Facebook during App Review — only
	// users on SUPER_ADMINS can kick off OAuth so the app never mints
	// an auth URL for a regular customer while scopes are still being
	// reviewed. Non-FB platforms skip this check entirely.
	if platformName == "facebook" {
		userID := auth.GetUserID(r.Context())
		if !h.superAdminChecker.IsSuperAdmin(r.Context(), userID) {
			writeError(w, http.StatusForbidden, "FACEBOOK_DISABLED", "Facebook integration is not enabled for your account")
			return
		}
	}

	profileID := h.getProfileID(r)
	if profileID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing profile context")
		return
	}

	adapter, err := platform.Get(platformName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	oauthAdapter, ok := adapter.(platform.OAuthAdapter)
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("%s does not support OAuth", platformName))
		return
	}

	// Get OAuth config — check for White Label credentials first
	config := h.getOAuthConfig(r, profileID, platformName, oauthAdapter)

	// Generate CSRF state
	state, err := platform.GenerateState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate state")
		return
	}

	// Store state for verification on callback
	_, err = h.queries.CreateOAuthState(r.Context(), db.CreateOAuthStateParams{
		State:     state,
		ProfileID: profileID,
		Platform:    platformName,
		RedirectUrl: pgtype.Text{String: redirectURL, Valid: redirectURL != ""},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to store OAuth state")
		return
	}

	authURL := oauthAdapter.GetAuthURL(config, state)
	writeSuccess(w, map[string]string{"auth_url": authURL})
}

// Callback handles GET /v1/oauth/callback/{platform}
// This is called by the OAuth provider after user authorization.
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	platformName := chi.URLParam(r, "platform")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		slog.Warn("oauth callback error", "platform", platformName, "error", errorParam, "description", errorDesc)
		h.redirectWithError(w, r, "", "Authorization denied: "+errorDesc)
		return
	}

	if code == "" || state == "" {
		h.redirectWithError(w, r, "", "Missing code or state parameter")
		return
	}

	// Verify state (CSRF protection)
	oauthState, err := h.queries.GetOAuthState(r.Context(), state)
	if err != nil {
		slog.Warn("oauth callback: invalid or expired state", "state", state)
		h.redirectWithError(w, r, "", "Invalid or expired OAuth state")
		return
	}

	// Clean up state
	h.queries.DeleteOAuthState(r.Context(), state)

	if oauthState.Platform != platformName {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Platform mismatch")
		return
	}

	adapter, err := platform.Get(platformName)
	if err != nil {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, err.Error())
		return
	}

	oauthAdapter, ok := adapter.(platform.OAuthAdapter)
	if !ok {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Platform does not support OAuth")
		return
	}

	config := h.getOAuthConfigForProfile(r, oauthState.ProfileID, platformName, oauthAdapter)
	// Pass the original state through so PKCE-using adapters (Twitter)
	// can reconstruct their verifier on the token exchange step.
	config.State = state

	// Exchange code for tokens
	result, err := oauthAdapter.ExchangeCode(r.Context(), config, code)
	if err != nil {
		slog.Error("oauth callback: code exchange failed", "platform", platformName, "error", err)
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to exchange authorization code")
		return
	}

	// Facebook detour: ExchangeCode returned a Meta user identity +
	// short-lived User Token. We don't yet know which Page(s) the
	// user wants to connect, so instead of writing a social_accounts
	// row, we stash everything needed to finalize later into
	// pending_connections and redirect the browser to the dashboard
	// picker. See PRD §4.2.
	if platformName == "facebook" {
		// This callback is hit by Meta's browser redirect, which
		// doesn't run through Clerk middleware — so we re-derive the
		// super-admin check from profile → workspace.user_id instead
		// of reading auth.GetUserID from context.
		if !h.callerIsFacebookSuperAdmin(r, oauthState.ProfileID) {
			h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Facebook integration is not enabled")
			return
		}
		h.handleFacebookCallback(w, r, oauthState, config, result)
		return
	}

	// Encrypt tokens
	encAccess, err := h.encryptor.Encrypt(result.AccessToken)
	if err != nil {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to encrypt token")
		return
	}

	var encRefresh string
	if result.RefreshToken != "" {
		encRefresh, err = h.encryptor.Encrypt(result.RefreshToken)
		if err != nil {
			h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to encrypt token")
			return
		}
	}

	metadataJSON := []byte("{}")
	if result.Metadata != nil {
		if m, err := json.Marshal(result.Metadata); err == nil {
			metadataJSON = m
		}
	}

	// Dedup: check if this platform account is already connected in the workspace
	if result.ExternalAccountID != "" {
		profile, profErr := h.queries.GetProfile(r.Context(), oauthState.ProfileID)
		if profErr == nil {
			existing, dupErr := h.queries.FindSocialAccountByExternalID(r.Context(), db.FindSocialAccountByExternalIDParams{
				Platform:          platformName,
				ExternalAccountID: result.ExternalAccountID,
				WorkspaceID:       profile.WorkspaceID,
			})
			if dupErr == nil && existing.ID != "" {
				// Account exists (active or disconnected) — reactivate with
				// fresh tokens. This preserves the original row ID so all
				// FK references (post results, analytics, inbox) stay intact.
				wasDisconnected := existing.DisconnectedAt.Valid
				slog.Info("oauth callback: reactivating existing account",
					"platform", platformName,
					"external_id", result.ExternalAccountID,
					"account_id", existing.ID,
					"was_disconnected", wasDisconnected)
				encAccess, aErr := h.encryptor.Encrypt(result.AccessToken)
				encRefresh, rErr := h.encryptor.Encrypt(result.RefreshToken)
				if aErr != nil || rErr != nil {
					slog.Error("oauth callback: encrypt failed for token update", "err_a", aErr, "err_r", rErr)
					h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to encrypt tokens")
					return
				}
				_, _ = h.queries.ReactivateSocialAccount(r.Context(), db.ReactivateSocialAccountParams{
					ID:             existing.ID,
					AccessToken:    encAccess,
					RefreshToken:   pgtype.Text{String: encRefresh, Valid: encRefresh != ""},
					TokenExpiresAt: pgtype.Timestamptz{Time: result.TokenExpiresAt, Valid: !result.TokenExpiresAt.IsZero()},
				})
				redirectURL := oauthState.RedirectUrl.String
				if redirectURL == "" {
					redirectURL = "https://app.unipost.dev"
				}
				sep := "?"
				if strings.Contains(redirectURL, "?") {
					sep = "&"
				}
				http.Redirect(w, r, redirectURL+sep+"status=success&account_name="+result.AccountName, http.StatusFound)
				return
			}
		}
	}

	// Store account
	_, err = h.queries.CreateSocialAccount(r.Context(), db.CreateSocialAccountParams{
		ProfileID:         oauthState.ProfileID,
		Platform:          platformName,
		AccessToken:       encAccess,
		RefreshToken:      pgtype.Text{String: encRefresh, Valid: encRefresh != ""},
		TokenExpiresAt:    pgtype.Timestamptz{Time: result.TokenExpiresAt, Valid: !result.TokenExpiresAt.IsZero()},
		ExternalAccountID: result.ExternalAccountID,
		AccountName:       pgtype.Text{String: result.AccountName, Valid: result.AccountName != ""},
		AccountAvatarUrl:  pgtype.Text{String: result.AvatarURL, Valid: result.AvatarURL != ""},
		Metadata:          metadataJSON,
	})
	if err != nil {
		slog.Error("oauth callback: failed to save account", "error", err)
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to save account")
		return
	}

	slog.Info("oauth account connected", "platform", platformName, "profile_id", oauthState.ProfileID, "account", result.AccountName)

	// Redirect back to frontend
	redirectURL := oauthState.RedirectUrl.String
	if redirectURL == "" {
		redirectURL = "https://app.unipost.dev"
	}
	sep := "?"
	if strings.Contains(redirectURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, redirectURL+sep+"status=success&account_name="+result.AccountName, http.StatusFound)
}

func (h *OAuthHandler) getOAuthConfig(r *http.Request, profileID, platformName string, adapter platform.OAuthAdapter) platform.OAuthConfig {
	return h.getOAuthConfigForProfile(r, profileID, platformName, adapter)
}

func (h *OAuthHandler) getOAuthConfigForProfile(r *http.Request, profileID, platformName string, adapter platform.OAuthAdapter) platform.OAuthConfig {
	config := adapter.DefaultOAuthConfig(h.baseRedirectURL)

	// Check for White Label credentials
	creds, err := h.queries.GetPlatformCredential(r.Context(), db.GetPlatformCredentialParams{
		WorkspaceID: profileID,
		Platform:  platformName,
	})
	if err == nil {
		config.ClientID = creds.ClientID
		secret, err := h.encryptor.Decrypt(creds.ClientSecret)
		if err == nil {
			config.ClientSecret = secret
		}
	}

	return config
}

func (h *OAuthHandler) getProfileID(r *http.Request) string {
	if pid := auth.GetWorkspaceID(r.Context()); pid != "" {
		return pid
	}
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

func (h *OAuthHandler) redirectWithError(w http.ResponseWriter, r *http.Request, redirectURL, errMsg string) {
	if redirectURL == "" {
		redirectURL = "https://app.unipost.dev"
	}
	sep := "?"
	if strings.Contains(redirectURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, redirectURL+sep+"status=error&error="+errMsg, http.StatusFound)
}

