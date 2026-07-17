package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

// PlatformHandler exposes the static publish-side capability map and the
// per-account capability lookup. Both endpoints are pure reads of an
// in-memory map (no DB hits for the global one) so we can serve them
// fast and cache them aggressively at the edge.
type PlatformHandler struct {
	queries *db.Queries
	quota   *quota.Checker
}

func NewPlatformHandler(queries *db.Queries, quotaCheckers ...*quota.Checker) *PlatformHandler {
	checker := quota.NewChecker(queries)
	if len(quotaCheckers) > 0 && quotaCheckers[0] != nil {
		checker = quotaCheckers[0]
	}
	return &PlatformHandler{queries: queries, quota: checker}
}

// capabilitiesEnvelope is what the global endpoint returns. We wrap the
// map under a "platforms" key + a schema_version sibling so clients can
// detect drift without parsing every entry.
type capabilitiesEnvelope struct {
	SchemaVersion string                         `json:"schema_version"`
	Platforms     map[string]platform.Capability `json:"platforms"`
}

// GetGlobalCapabilities handles GET /v1/platforms/capabilities.
//
// No authentication required — the data is the same for every caller and
// safe to expose publicly. We set Cache-Control so CDNs and clients
// happily cache it for an hour; the data only changes when we deploy.
func (h *PlatformHandler) GetGlobalCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeSuccess(w, capabilitiesEnvelope{
		SchemaVersion: platform.CapabilitiesSchemaVersion,
		Platforms:     platform.Capabilities,
	})
}

// accountCapabilityResponse is the per-account variant. The shape is
// almost identical to a single Capability entry, but the platform key
// is included alongside so the client can correlate without re-fetching
// the global map.
type accountCapabilityResponse struct {
	SchemaVersion string               `json:"schema_version"`
	AccountID     string               `json:"account_id"`
	Platform      string               `json:"platform"`
	Capability    platform.Capability  `json:"capability"`
	XInbox        *xinbox.Capabilities `json:"x_inbox,omitempty"`
}

// GetAccountCapabilities handles GET /v1/social-accounts/{id}/capabilities.
//
// API key auth (workspace-scoped). Returns the capability for the
// account's platform, scoped to the calling workspace so a customer
// can't enumerate accounts they don't own.
//
// Per-account quirks (e.g. business vs creator IG) are NOT yet
// reflected in the returned struct — Sprint 1 returns the platform-
// level defaults. The endpoint exists today so the schema is stable
// for clients; Sprint 2 will add the account-specific overrides.
func (h *PlatformHandler) GetAccountCapabilities(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
		ID:          accountID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return
	}

	cap, ok := platform.CapabilityFor(strings.ToLower(acc.Platform))
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "No capability data for platform "+acc.Platform)
		return
	}

	response := accountCapabilityResponse{
		SchemaVersion: platform.CapabilitiesSchemaVersion,
		AccountID:     acc.ID,
		Platform:      acc.Platform,
		Capability:    cap,
	}
	if strings.EqualFold(acc.Platform, "twitter") {
		appMode, modeErr := xinbox.NormalizePersistedAppMode(acc.XAppMode.String)
		if modeErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "X account app identity is invalid; reconnect required")
			return
		}
		input := xinbox.CapabilityInput{
			PlanAllowsInbox: h.quota == nil || h.quota.PlanAllowsInbox(r.Context(), workspaceID),
			AccountStatus:   acc.Status,
			Scopes:          acc.Scope,
			AppMode:         appMode,
		}
		if delivery, deliveryErr := h.queries.GetXInboxDeliveryResource(r.Context(), acc.ID); deliveryErr == nil {
			input.DeliveryStatus = delivery.DeliveryStatus
		} else if deliveryErr != pgx.ErrNoRows {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load X Inbox delivery state")
			return
		}
		if input.AppMode == xinbox.AppModeWorkspace {
			cred, credErr := h.queries.GetPlatformCredential(r.Context(), db.GetPlatformCredentialParams{
				WorkspaceID: workspaceID,
				Platform:    "twitter",
			})
			if credErr == nil {
				input.AppCredentials = xinbox.AppCredentials{
					ClientIDConfigured:       strings.TrimSpace(cred.ClientID) != "",
					ClientSecretConfigured:   strings.TrimSpace(cred.ClientSecret) != "",
					AppBearerTokenConfigured: cred.AppBearerToken.Valid && strings.TrimSpace(cred.AppBearerToken.String) != "",
					ConsumerSecretConfigured: cred.ConsumerSecret.Valid && strings.TrimSpace(cred.ConsumerSecret.String) != "",
				}
			} else if credErr != pgx.ErrNoRows {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load X app credentials")
				return
			}
		}
		xInbox := xinbox.EvaluateCapabilities(input)
		response.XInbox = &xInbox
	}
	writeSuccess(w, response)
}
