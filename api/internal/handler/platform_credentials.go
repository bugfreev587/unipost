package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type PlatformCredentialHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	quota     *quota.Checker
}

func NewPlatformCredentialHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, quotaChecker *quota.Checker) *PlatformCredentialHandler {
	return &PlatformCredentialHandler{queries: queries, encryptor: encryptor, quota: quotaChecker}
}

type platformCredentialResponse struct {
	Platform                  string    `json:"platform"`
	ClientID                  string    `json:"client_id"`
	AppBearerTokenConfigured  bool      `json:"app_bearer_token_configured"`
	ConsumerSecretConfigured  bool      `json:"consumer_secret_configured"`
	XInboxCredentialsComplete bool      `json:"x_inbox_credentials_complete"`
	CreatedAt                 time.Time `json:"created_at"`
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

	limit := 0
	if h.quota != nil {
		limit = h.quota.WhiteLabelPlatformLimit(r.Context(), workspaceID)
	}
	if limit == 0 {
		writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
			"Platform Credentials require the Basic plan or higher — upgrade at unipost.dev/pricing")
		return
	}

	var body struct {
		Platform       string  `json:"platform"`
		ClientID       string  `json:"client_id"`
		ClientSecret   string  `json:"client_secret"`
		AppBearerToken *string `json:"app_bearer_token"`
		ConsumerSecret *string `json:"consumer_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	body.Platform = strings.ToLower(strings.TrimSpace(body.Platform))
	if body.Platform == "" || strings.TrimSpace(body.ClientID) == "" || strings.TrimSpace(body.ClientSecret) == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "platform, client_id, and client_secret are required")
		return
	}
	if !connectablePlatforms[body.Platform] {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"platform must be one of "+connectablePlatformList)
		return
	}
	if body.Platform != "twitter" && (body.AppBearerToken != nil || body.ConsumerSecret != nil) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "app_bearer_token and consumer_secret are supported only for twitter")
		return
	}
	if body.AppBearerToken != nil && strings.TrimSpace(*body.AppBearerToken) == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "app_bearer_token cannot be blank")
		return
	}
	if body.ConsumerSecret != nil && strings.TrimSpace(*body.ConsumerSecret) == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "consumer_secret cannot be blank")
		return
	}

	if limit > 0 {
		if limit == 1 {
			_, err := h.queries.ClaimWorkspaceCustomPlatformSlot(r.Context(), db.ClaimWorkspaceCustomPlatformSlotParams{
				ID:                 workspaceID,
				CustomPlatformSlot: pgtype.Text{String: body.Platform, Valid: true},
			})
			if err == pgx.ErrNoRows {
				writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
					"Your current plan supports custom hosted branding and platform credentials for 1 platform. Use the selected platform or upgrade to Growth for all supported platforms.")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check custom platform slot")
				return
			}
		}
		creds, err := h.queries.ListPlatformCredentialsByWorkspace(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check custom platform limits")
			return
		}
		alreadyConfigured := false
		for _, cred := range creds {
			if cred.Platform == body.Platform {
				alreadyConfigured = true
				break
			}
		}
		if !alreadyConfigured && len(creds) >= limit {
			writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
				"Your current plan supports custom hosted branding and platform credentials for 1 platform. Use the selected platform or upgrade to Growth for all supported platforms.")
			return
		}
	}

	encSecret, err := h.encryptor.Encrypt(body.ClientSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encrypt credentials")
		return
	}

	appBearerToken := pgtype.Text{}
	consumerSecret := pgtype.Text{}
	if body.Platform == "twitter" {
		existing, existingErr := h.queries.GetPlatformCredential(r.Context(), db.GetPlatformCredentialParams{
			WorkspaceID: workspaceID,
			Platform:    body.Platform,
		})
		if existingErr == nil {
			appBearerToken = existing.AppBearerToken
			consumerSecret = existing.ConsumerSecret
		} else if existingErr != pgx.ErrNoRows {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load existing credentials")
			return
		}
		if body.AppBearerToken != nil {
			encrypted, encryptErr := h.encryptor.Encrypt(strings.TrimSpace(*body.AppBearerToken))
			if encryptErr != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encrypt credentials")
				return
			}
			appBearerToken = pgtype.Text{String: encrypted, Valid: true}
		}
		if body.ConsumerSecret != nil {
			encrypted, encryptErr := h.encryptor.Encrypt(strings.TrimSpace(*body.ConsumerSecret))
			if encryptErr != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encrypt credentials")
				return
			}
			consumerSecret = pgtype.Text{String: encrypted, Valid: true}
		}
	}

	cred, err := h.queries.CreatePlatformCredential(r.Context(), db.CreatePlatformCredentialParams{
		WorkspaceID:    workspaceID,
		Platform:       body.Platform,
		ClientID:       body.ClientID,
		ClientSecret:   encSecret,
		AppBearerToken: appBearerToken,
		ConsumerSecret: consumerSecret,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save credentials")
		return
	}

	writeCreated(w, platformCredentialResponseFromDB(cred))
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
		result[i] = platformCredentialResponseFromDB(c)
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

func platformCredentialResponseFromDB(cred db.PlatformCredential) platformCredentialResponse {
	appBearerConfigured := cred.AppBearerToken.Valid && cred.AppBearerToken.String != ""
	consumerSecretConfigured := cred.ConsumerSecret.Valid && cred.ConsumerSecret.String != ""
	isTwitter := strings.EqualFold(cred.Platform, "twitter")
	return platformCredentialResponse{
		Platform:                  cred.Platform,
		ClientID:                  cred.ClientID,
		AppBearerTokenConfigured:  appBearerConfigured,
		ConsumerSecretConfigured:  consumerSecretConfigured,
		XInboxCredentialsComplete: isTwitter && cred.ClientID != "" && cred.ClientSecret != "" && appBearerConfigured && consumerSecretConfigured,
		CreatedAt:                 cred.CreatedAt.Time,
	}
}
