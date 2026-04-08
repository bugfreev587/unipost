package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// PlatformHandler exposes the static publish-side capability map and the
// per-account capability lookup. Both endpoints are pure reads of an
// in-memory map (no DB hits for the global one) so we can serve them
// fast and cache them aggressively at the edge.
type PlatformHandler struct {
	queries *db.Queries
}

func NewPlatformHandler(queries *db.Queries) *PlatformHandler {
	return &PlatformHandler{queries: queries}
}

// capabilitiesEnvelope is what the global endpoint returns. We wrap the
// map under a "platforms" key + a schema_version sibling so clients can
// detect drift without parsing every entry.
type capabilitiesEnvelope struct {
	SchemaVersion string                          `json:"schema_version"`
	Platforms     map[string]platform.Capability  `json:"platforms"`
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
	SchemaVersion string              `json:"schema_version"`
	AccountID     string              `json:"account_id"`
	Platform      string              `json:"platform"`
	Capability    platform.Capability `json:"capability"`
}

// GetAccountCapabilities handles GET /v1/social-accounts/{id}/capabilities.
//
// API key auth (project-scoped). Returns the capability for the
// account's platform, scoped to the calling project so a customer
// can't enumerate accounts they don't own.
//
// Per-account quirks (e.g. business vs creator IG) are NOT yet
// reflected in the returned struct — Sprint 1 returns the platform-
// level defaults. The endpoint exists today so the schema is stable
// for clients; Sprint 2 will add the account-specific overrides.
func (h *PlatformHandler) GetAccountCapabilities(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetProjectID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, err := h.queries.GetSocialAccountByIDAndProject(r.Context(), db.GetSocialAccountByIDAndProjectParams{
		ID:        accountID,
		ProjectID: projectID,
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

	writeSuccess(w, accountCapabilityResponse{
		SchemaVersion: platform.CapabilitiesSchemaVersion,
		AccountID:     acc.ID,
		Platform:      acc.Platform,
		Capability:    cap,
	})
}
