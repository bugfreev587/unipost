package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type APIKeyHandler struct {
	queries *db.Queries
	quota   *quota.Checker
}

func NewAPIKeyHandler(queries *db.Queries) *APIKeyHandler {
	return &APIKeyHandler{queries: queries, quota: quota.NewChecker(queries)}
}

type apiKeyResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	Environment string     `json:"environment"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

type apiKeyCreateResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`
	Prefix      string    `json:"prefix"`
	Environment string    `json:"environment"`
	CreatedAt   time.Time `json:"created_at"`
}

func toAPIKeyResponse(k db.ApiKey) apiKeyResponse {
	resp := apiKeyResponse{
		ID:          k.ID,
		Name:        k.Name,
		Prefix:      k.Prefix,
		Environment: k.Environment,
		CreatedAt:   k.CreatedAt.Time,
	}
	if k.LastUsedAt.Valid {
		t := k.LastUsedAt.Time
		resp.LastUsedAt = &t
	}
	if k.ExpiresAt.Valid {
		t := k.ExpiresAt.Time
		resp.ExpiresAt = &t
	}
	return resp
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	keys, err := h.queries.ListAPIKeysByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list API keys")
		return
	}
	result := make([]apiKeyResponse, len(keys))
	for i, k := range keys {
		result[i] = toAPIKeyResponse(k)
	}
	writeSuccessWithListMeta(w, result, len(result), len(result))
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	// RBAC migration 063: attribute the new key to its creator. The
	// creator's role at runtime drives the key's effective permissions
	// (see dualauth.go authenticateAPIKey). For API-key-auth callers
	// (no Clerk user), fall back to a synthetic "owner" creator since
	// pre-RBAC keys all belonged to the workspace owner.
	creatorUserID := auth.GetUserID(r.Context())
	if creatorUserID == "" {
		// API-key-auth path — derive from the calling key's own
		// creator so the new key inherits the same identity.
		callingKeyID := auth.GetAPIKeyID(r.Context())
		if callingKeyID != "" {
			if calling, err := h.queries.GetAPIKey(r.Context(), callingKeyID); err == nil {
				creatorUserID = calling.CreatedByUserID
			}
		}
	}
	if creatorUserID == "" {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Could not resolve creator identity for new API key")
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Environment string  `json:"environment"`
		ExpiresAt   *string `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name is required")
		return
	}
	if body.Environment == "" {
		body.Environment = "production"
	}
	if body.Environment != "production" && body.Environment != "test" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Environment must be 'production' or 'test'")
		return
	}

	if h.quota != nil {
		if cap, hasCap := h.quota.MaxAPIKeysForPlan(r.Context(), workspaceID); hasCap {
			current, err := h.queries.CountActiveAPIKeysByWorkspace(r.Context(), workspaceID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check API key limit")
				return
			}
			if int(current) >= cap {
				writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
					"Free plan workspaces can have 1 active API key. Upgrade to create more API keys.")
				return
			}
		}
	}

	plaintext, prefix, hash, err := apikey.Generate(body.Environment)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate API key")
		return
	}

	var expiresAt pgtype.Timestamptz
	if body.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid expires_at format, use RFC3339")
			return
		}
		expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
	}

	key, err := h.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		ID:              uuid.New().String(),
		WorkspaceID:     workspaceID,
		Name:            body.Name,
		Prefix:          prefix,
		KeyHash:         hash,
		Environment:     body.Environment,
		ExpiresAt:       expiresAt,
		CreatedByUserID: creatorUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key")
		return
	}

	audit.Log(r.Context(), h.queries, audit.Event{
		WorkspaceID:   workspaceID,
		ActorUserID:   auth.GetUserID(r.Context()),
		ActorAPIKeyID: auth.GetAPIKeyID(r.Context()),
		Action:        audit.ActionAPIKeyCreated,
		ResourceType:  "api_key",
		ResourceID:    key.ID,
		Category:      audit.CategoryConfig,
		IPAddress:     r.RemoteAddr,
		UserAgent:     r.UserAgent(),
		After: map[string]any{
			"name":        key.Name,
			"prefix":      key.Prefix,
			"environment": key.Environment,
		},
	})

	writeCreated(w, apiKeyCreateResponse{
		ID:          key.ID,
		Name:        key.Name,
		Key:         plaintext,
		Prefix:      key.Prefix,
		Environment: key.Environment,
		CreatedAt:   key.CreatedAt.Time,
	})
}

func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	keyID := chi.URLParam(r, "keyID")

	key, err := h.queries.RevokeAPIKey(r.Context(), db.RevokeAPIKeyParams{
		ID:          keyID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "API key not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke API key")
		return
	}
	audit.Log(r.Context(), h.queries, audit.Event{
		WorkspaceID:   workspaceID,
		ActorUserID:   auth.GetUserID(r.Context()),
		ActorAPIKeyID: auth.GetAPIKeyID(r.Context()),
		Action:        audit.ActionAPIKeyRevoked,
		ResourceType:  "api_key",
		ResourceID:    key.ID,
		Category:      audit.CategoryConfig,
		IPAddress:     r.RemoteAddr,
		UserAgent:     r.UserAgent(),
		Before: map[string]any{
			"name":        key.Name,
			"prefix":      key.Prefix,
			"environment": key.Environment,
		},
	})
	w.WriteHeader(http.StatusNoContent)
}
