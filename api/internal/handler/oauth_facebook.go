// oauth_facebook.go owns the Facebook-specific detour through
// pending_connections. Other platforms land in the standard callback
// path in oauth.go and write a social_accounts row immediately —
// Facebook cannot because a single OAuth consent may cover multiple
// Pages and the user has to pick which ones to connect.
//
// Flow:
//
//  1. Callback branch (in oauth.go) hands us the short-lived User
//     Token + Meta user identity from ExchangeCode.
//  2. handleFacebookCallback exchanges that for a 60-day LL User
//     Token, calls /me/accounts to enumerate managed Pages, and
//     writes a pending_connections row carrying the encrypted
//     tokens + page list. Redirect to the Dashboard picker.
//  3. The Dashboard renders the picker by hitting
//     GET /v1/pending-connections/{id} (served by PendingConnection-
//     Get below), and submits the user's selection via
//     POST /v1/pending-connections/{id}/finalize, which creates N
//     social_accounts rows + upserts meta_user_tokens.

package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// pendingFacebookPage is how we serialize each Page inside the
// pending_connections.pages_json blob. PageAccessTokenEncrypted is
// AES-encrypted so the row doesn't leak tokens at rest.
type pendingFacebookPage struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Category                 string   `json:"category"`
	PictureURL               string   `json:"picture_url"`
	Tasks                    []string `json:"tasks"`
	PageAccessTokenEncrypted string   `json:"page_access_token_enc"`
}

// pendingConnectionResponse is the payload sent to the Dashboard
// picker. Note: no token data is included — the browser only needs
// what it takes to render the pick list. Tokens stay server-side
// until finalize.
type pendingConnectionResponse struct {
	ID          string                 `json:"id"`
	Platform    string                 `json:"platform"`
	ProfileID   string                 `json:"profile_id"`
	MetaUser    metaUserDescriptor     `json:"meta_user"`
	Pages       []pendingPageDescriptor `json:"pages"`
	ExpiresAt   time.Time              `json:"expires_at"`
}

type metaUserDescriptor struct {
	MetaUserID string `json:"meta_user_id"`
}

type pendingPageDescriptor struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Category        string   `json:"category"`
	PictureURL      string   `json:"picture_url"`
	Tasks           []string `json:"tasks"`
	CanPublish      bool     `json:"can_publish"`
}

func (h *OAuthHandler) handleFacebookCallback(
	w http.ResponseWriter,
	r *http.Request,
	oauthState db.OauthState,
	config platform.OAuthConfig,
	result *platform.ConnectResult,
) {
	fbAdapter, ok := getFacebookAdapter()
	if !ok {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Facebook adapter unavailable")
		return
	}

	// Step 1: short-lived → long-lived User Token (~60 days).
	llToken, llExpiresAt, err := fbAdapter.ExchangeForLongLivedUserToken(
		r.Context(), config.ClientID, config.ClientSecret, result.AccessToken,
	)
	if err != nil {
		slog.Error("facebook: LL token exchange failed", "err", err)
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to upgrade Facebook access token")
		return
	}

	// Step 2: enumerate Pages the user manages (each carrying its
	// own Page Access Token). Empty list is legitimate — see PRD
	// §15 for the "0 Pages" message; we still write a pending row
	// so the Dashboard can render that state.
	pages, err := fbAdapter.FetchPages(r.Context(), llToken)
	if err != nil {
		slog.Error("facebook: /me/accounts failed", "err", err)
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to load Facebook Pages")
		return
	}

	// Step 3: encrypt every sensitive field before persisting. The
	// LL User Token + each Page Access Token live in pending rows
	// that are workspace-scoped, but encryption at rest is cheap
	// insurance and matches how social_accounts stores tokens.
	encLLToken, err := h.encryptor.Encrypt(llToken)
	if err != nil {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to encrypt Facebook token")
		return
	}
	pendingPages := make([]pendingFacebookPage, 0, len(pages))
	for _, p := range pages {
		encPageToken, err := h.encryptor.Encrypt(p.AccessToken)
		if err != nil {
			h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to encrypt Page token")
			return
		}
		pendingPages = append(pendingPages, pendingFacebookPage{
			ID:                       p.ID,
			Name:                     p.Name,
			Category:                 p.Category,
			PictureURL:               p.PictureURL,
			Tasks:                    p.Tasks,
			PageAccessTokenEncrypted: encPageToken,
		})
	}
	pagesJSON, err := json.Marshal(pendingPages)
	if err != nil {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to serialize Pages")
		return
	}

	// Step 4: resolve the profile's workspace so pending rows can
	// enforce workspace-scoped reads on the finalize endpoint.
	profile, err := h.queries.GetProfile(r.Context(), oauthState.ProfileID)
	if err != nil {
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Profile not found")
		return
	}

	metaUserID, _ := result.Metadata["meta_user_id"].(string)
	if metaUserID == "" {
		metaUserID = result.ExternalAccountID
	}

	row, err := h.queries.CreatePendingConnection(r.Context(), db.CreatePendingConnectionParams{
		WorkspaceID:         profile.WorkspaceID,
		ProfileID:           oauthState.ProfileID,
		Platform:            "facebook",
		MetaUserID:          metaUserID,
		UserTokenEncrypted:  encLLToken,
		UserTokenExpiresAt:  pgtype.Timestamptz{Time: llExpiresAt, Valid: true},
		PagesJson:           pagesJSON,
	})
	if err != nil {
		slog.Error("facebook: failed to write pending connection", "err", err)
		h.redirectWithError(w, r, oauthState.RedirectUrl.String, "Failed to prepare Page selection")
		return
	}

	// Step 5: redirect back to the dashboard with the pending ID.
	// Dashboard reads ?pending=<id> and opens the picker modal.
	redirectURL := oauthState.RedirectUrl.String
	if redirectURL == "" {
		redirectURL = "https://app.unipost.dev"
	}
	sep := "?"
	if strings.Contains(redirectURL, "?") {
		sep = "&"
	}
	redirectURL += sep + "pending=" + row.ID
	slog.Info("facebook oauth pending",
		"profile_id", oauthState.ProfileID,
		"workspace_id", profile.WorkspaceID,
		"pending_id", row.ID,
		"pages_count", len(pages))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// PendingConnectionGet handles
// GET /v1/workspaces/{workspaceID}/pending-connections/{id} —
// returns the stored pages list so the Dashboard picker can render.
// No tokens are returned; encrypted tokens stay server-side until
// the finalize endpoint decrypts + writes social_accounts rows.
func (h *OAuthHandler) PendingConnectionGet(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	pendingID := chi.URLParam(r, "id")
	if pendingID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing pending connection id")
		return
	}

	row, err := h.queries.GetPendingConnection(r.Context(), db.GetPendingConnectionParams{
		ID:          pendingID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Pending connection not found or expired")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load pending connection")
		return
	}

	var stored []pendingFacebookPage
	if err := json.Unmarshal(row.PagesJson, &stored); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Corrupt pending connection data")
		return
	}
	descriptors := make([]pendingPageDescriptor, 0, len(stored))
	for _, p := range stored {
		descriptors = append(descriptors, pendingPageDescriptor{
			ID:         p.ID,
			Name:       p.Name,
			Category:   p.Category,
			PictureURL: p.PictureURL,
			Tasks:      p.Tasks,
			CanPublish: platform.PageHasPublishTask(p.Tasks),
		})
	}

	writeSuccess(w, pendingConnectionResponse{
		ID:        row.ID,
		Platform:  row.Platform,
		ProfileID: row.ProfileID,
		MetaUser:  metaUserDescriptor{MetaUserID: row.MetaUserID},
		Pages:     descriptors,
		ExpiresAt: row.ExpiresAt.Time,
	})
}

// PendingConnectionFinalize handles
// POST /v1/workspaces/{workspaceID}/pending-connections/{id}/finalize
// Request body: { "page_ids": ["123", "456"] }
//
// Creates one social_accounts row per selected Page, upserts the
// workspace's meta_user_tokens row, and deletes the pending row.
// Rejects Page IDs that aren't in the stored list OR whose tasks
// don't include publishing permissions (PRD §15 "insufficient
// permissions" case).
func (h *OAuthHandler) PendingConnectionFinalize(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	pendingID := chi.URLParam(r, "id")

	var body struct {
		PageIDs []string `json:"page_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}
	if len(body.PageIDs) == 0 {
		writeError(w, http.StatusBadRequest, "NO_PAGES_SELECTED", "Pick at least one Page to connect")
		return
	}

	row, err := h.queries.GetPendingConnection(r.Context(), db.GetPendingConnectionParams{
		ID:          pendingID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Pending connection not found or expired")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load pending connection")
		return
	}

	var stored []pendingFacebookPage
	if err := json.Unmarshal(row.PagesJson, &stored); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Corrupt pending connection data")
		return
	}
	byID := make(map[string]pendingFacebookPage, len(stored))
	for _, p := range stored {
		byID[p.ID] = p
	}

	// Validate the selection BEFORE any side effects.
	selected := make([]pendingFacebookPage, 0, len(body.PageIDs))
	for _, id := range body.PageIDs {
		page, ok := byID[id]
		if !ok {
			writeError(w, http.StatusBadRequest, "UNKNOWN_PAGE",
				fmt.Sprintf("Page %s was not in the original selection", id))
			return
		}
		if !platform.PageHasPublishTask(page.Tasks) {
			writeError(w, http.StatusForbidden, "PAGE_LACKS_PUBLISH_PERMISSION",
				fmt.Sprintf("Page %q does not grant publishing permission to this user", page.Name))
			return
		}
		selected = append(selected, page)
	}

	// Upsert the meta_user_tokens row first — if any subsequent
	// account creation fails we still want "Add another Page"
	// later to work without a full re-OAuth.
	if _, err := h.queries.UpsertMetaUserToken(r.Context(), db.UpsertMetaUserTokenParams{
		WorkspaceID:              workspaceID,
		MetaUserID:               row.MetaUserID,
		LongLivedTokenEncrypted: row.UserTokenEncrypted,
		ExpiresAt:                row.UserTokenExpiresAt,
	}); err != nil {
		slog.Error("facebook finalize: upsert meta_user_tokens failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to store user token")
		return
	}

	createdAccounts := make([]string, 0, len(selected))
	for _, page := range selected {
		metadata := map[string]any{
			"meta_user_id":  row.MetaUserID,
			"page_category": page.Category,
			"picture_url":   page.PictureURL,
			"tasks":         page.Tasks,
		}
		metadataJSON, _ := json.Marshal(metadata)

		existing, findErr := h.queries.FindSocialAccountByExternalID(r.Context(), db.FindSocialAccountByExternalIDParams{
			Platform:          "facebook",
			ExternalAccountID: page.ID,
			WorkspaceID:       workspaceID,
		})
		if findErr == nil && existing.ID != "" {
			// Reactivate existing account with the fresh Page Token.
			_, _ = h.queries.ReactivateSocialAccount(r.Context(), db.ReactivateSocialAccountParams{
				ID:             existing.ID,
				AccessToken:    page.PageAccessTokenEncrypted,
				RefreshToken:   pgtype.Text{Valid: false},
				TokenExpiresAt: pgtype.Timestamptz{Valid: false},
			})
			createdAccounts = append(createdAccounts, existing.ID)
			continue
		}

		newAcc, err := h.queries.CreateSocialAccount(r.Context(), db.CreateSocialAccountParams{
			ProfileID:         row.ProfileID,
			Platform:          "facebook",
			AccessToken:       page.PageAccessTokenEncrypted,
			RefreshToken:      pgtype.Text{Valid: false},
			TokenExpiresAt:    pgtype.Timestamptz{Valid: false},
			ExternalAccountID: page.ID,
			AccountName:       pgtype.Text{String: page.Name, Valid: page.Name != ""},
			AccountAvatarUrl:  pgtype.Text{String: page.PictureURL, Valid: page.PictureURL != ""},
			Metadata:          metadataJSON,
		})
		if err != nil {
			slog.Error("facebook finalize: create social_account failed", "err", err, "page_id", page.ID)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
				fmt.Sprintf("Failed to save Page %q", page.Name))
			return
		}
		createdAccounts = append(createdAccounts, newAcc.ID)
	}

	// Swept last so a partial failure leaves the pending row around
	// for retry.
	_ = h.queries.DeletePendingConnection(r.Context(), row.ID)

	writeSuccess(w, map[string]any{
		"connected_account_ids": createdAccounts,
		"connected_count":       len(createdAccounts),
	})
}

// callerIsFacebookSuperAdmin looks up the workspace owner for the
// pending OAuth state's profile and checks whether that user is on
// SUPER_ADMINS. Used by the callback path, where Meta's redirect
// doesn't preserve a Clerk session — the safe, DB-backed derivation
// matches what the super-admin middleware does for live requests.
func (h *OAuthHandler) callerIsFacebookSuperAdmin(r *http.Request, profileID string) bool {
	if h.superAdminChecker == nil {
		return false
	}
	profile, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		return false
	}
	workspace, err := h.queries.GetWorkspace(r.Context(), profile.WorkspaceID)
	if err != nil {
		return false
	}
	user, err := h.queries.GetUser(r.Context(), workspace.UserID)
	if err != nil {
		return h.superAdminChecker.IsSuperAdmin(r.Context(), workspace.UserID)
	}
	return h.superAdminChecker.IsSuperAdminByUser(workspace.UserID, user.Email)
}

// getFacebookAdapter pulls the registered FacebookAdapter instance.
// Kept local to this file so other handlers don't accidentally reach
// into platform internals.
func getFacebookAdapter() (*platform.FacebookAdapter, bool) {
	adapter, err := platform.Get("facebook")
	if err != nil {
		return nil, false
	}
	fb, ok := adapter.(*platform.FacebookAdapter)
	return fb, ok
}
