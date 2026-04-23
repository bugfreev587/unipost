package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// WebhookSubscriptionHandler manages the per-project webhook
// subscriptions used by the webhook delivery worker. As of Sprint 1
// (PR8), the signing secret is generated server-side and exposed
// only ONCE in the Create / Rotate response — Stripe-style. Reads
// return a secret_preview (first 8 chars) instead of the plaintext.
type WebhookSubscriptionHandler struct {
	queries *db.Queries
}

func NewWebhookSubscriptionHandler(queries *db.Queries) *WebhookSubscriptionHandler {
	return &WebhookSubscriptionHandler{queries: queries}
}

// webhookResponse is the read-shape: secret_preview only, never
// plaintext. Returned by GET / Update endpoints.
type webhookResponse struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	Active        bool      `json:"active"`
	SecretPreview string    `json:"secret_preview"`
	CreatedAt     time.Time `json:"created_at"`
}

// webhookCreateResponse is the one-time-only Create response shape.
// Includes the FULL plaintext secret (secret) AND the preview
// (secret_preview). Subsequent reads via GET will only return preview.
type webhookCreateResponse struct {
	webhookResponse
	Secret string `json:"secret"`
}

func toWebhookResponse(wh db.Webhook) webhookResponse {
	return webhookResponse{
		ID:            wh.ID,
		URL:           wh.Url,
		Events:        wh.Events,
		Active:        wh.Active,
		SecretPreview: secretPreview(wh.Secret),
		CreatedAt:     wh.CreatedAt.Time,
	}
}

// secretPreview returns the first 8 chars of the plaintext for
// display purposes ("whsec_ab…"). Long enough to disambiguate, short
// enough that it can't be used to forge a signature.
func secretPreview(secret string) string {
	if len(secret) <= 8 {
		return secret
	}
	return secret[:8] + "…"
}

// generateWebhookSecret produces a fresh signing secret in the
// "whsec_" + 32 hex char shape Stripe popularized. 32 hex chars =
// 128 bits of entropy from crypto/rand.
func generateWebhookSecret() (string, error) {
	buf := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	return "whsec_" + hex.EncodeToString(buf), nil
}

// Create handles POST /v1/webhooks.
//
// BREAKING (Sprint 1): the request body must NOT include `secret` —
// it's generated server-side and returned in the response exactly
// once. Pass {url, events} only.
func (h *WebhookSubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
		// Reject this field if present so callers learn about the
		// behavior change immediately rather than silently dropping
		// their secret.
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.URL == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "url is required")
		return
	}
	if body.Secret != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"secret is generated server-side; do not provide one (BREAKING CHANGE in Sprint 1 — see CHANGELOG)")
		return
	}
	if len(body.Events) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "at least one event is required")
		return
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate signing secret")
		return
	}

	wh, err := h.queries.CreateWebhook(r.Context(), db.CreateWebhookParams{
		WorkspaceID: workspaceID,
		Url:         body.URL,
		Secret:      secret,
		Events:      body.Events,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create webhook")
		return
	}

	writeCreated(w, webhookCreateResponse{
		webhookResponse: toWebhookResponse(wh),
		Secret:          secret,
	})
}

// List handles GET /v1/webhooks.
func (h *WebhookSubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	webhooks, err := h.queries.ListWebhooksByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list webhooks")
		return
	}
	total := len(webhooks)

	limit := total
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if limit < len(webhooks) {
		webhooks = webhooks[:limit]
	}

	result := make([]webhookResponse, len(webhooks))
	for i, wh := range webhooks {
		result[i] = toWebhookResponse(wh)
	}

	writeSuccessWithListMeta(w, result, total, limit)
}

// Get handles GET /v1/webhooks/{id}.
func (h *WebhookSubscriptionHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	id := chi.URLParam(r, "id")
	wh, err := h.queries.GetWebhookByIDAndWorkspace(r.Context(), db.GetWebhookByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get webhook")
		return
	}
	writeSuccess(w, toWebhookResponse(wh))
}

// Update handles PATCH /v1/webhooks/{id}. Allows updating url,
// events, and active. CANNOT touch the secret — use /rotate for that.
func (h *WebhookSubscriptionHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	id := chi.URLParam(r, "id")

	// Load the existing row first so we can pass through unchanged
	// fields. The patch query writes ALL three columns; sqlc doesn't
	// have great partial-update support without per-field flags.
	existing, err := h.queries.GetWebhookByIDAndWorkspace(r.Context(), db.GetWebhookByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load webhook")
		return
	}

	var body struct {
		URL    *string  `json:"url"`
		Events []string `json:"events"`
		Active *bool    `json:"active"`
		Secret string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.Secret != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"secret cannot be set via PATCH; use POST /v1/webhooks/{id}/rotate")
		return
	}

	url := existing.Url
	if body.URL != nil {
		url = *body.URL
	}
	evs := existing.Events
	if body.Events != nil {
		evs = body.Events
	}
	active := existing.Active
	if body.Active != nil {
		active = *body.Active
	}

	updated, err := h.queries.UpdateWebhookURLEventsActive(r.Context(), db.UpdateWebhookURLEventsActiveParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Url:         url,
		Events:      evs,
		Active:      active,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update webhook")
		return
	}
	writeSuccess(w, toWebhookResponse(updated))
}

// Rotate handles POST /v1/webhooks/{id}/rotate. Generates a new
// signing secret and returns it once.
func (h *WebhookSubscriptionHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	id := chi.URLParam(r, "id")

	secret, err := generateWebhookSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate signing secret")
		return
	}

	wh, err := h.queries.RotateWebhookSecret(r.Context(), db.RotateWebhookSecretParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Secret:      secret,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to rotate secret")
		return
	}

	writeSuccess(w, webhookCreateResponse{
		webhookResponse: toWebhookResponse(wh),
		Secret:          secret,
	})
}

// Delete handles DELETE /v1/webhooks/{id}. Hard delete — removes the
// row entirely. Pending deliveries cascade-delete via the FK.
func (h *WebhookSubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	id := chi.URLParam(r, "id")

	if err := h.queries.HardDeleteWebhook(r.Context(), db.HardDeleteWebhookParams{
		ID:          id,
		WorkspaceID: workspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete webhook")
		return
	}
	writeSuccess(w, map[string]bool{"deleted": true})
}
