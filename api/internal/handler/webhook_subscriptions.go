package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type WebhookSubscriptionHandler struct {
	queries *db.Queries
}

func NewWebhookSubscriptionHandler(queries *db.Queries) *WebhookSubscriptionHandler {
	return &WebhookSubscriptionHandler{queries: queries}
}

type webhookResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Create handles POST /v1/webhooks
func (h *WebhookSubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetProjectID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	var body struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
		Secret string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.URL == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "URL is required")
		return
	}
	if body.Secret == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Secret is required")
		return
	}
	if len(body.Events) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "At least one event is required")
		return
	}

	wh, err := h.queries.CreateWebhook(r.Context(), db.CreateWebhookParams{
		ProjectID: projectID,
		Url:       body.URL,
		Secret:    body.Secret,
		Events:    body.Events,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create webhook")
		return
	}

	writeCreated(w, webhookResponse{
		ID:        wh.ID,
		URL:       wh.Url,
		Events:    wh.Events,
		Active:    wh.Active,
		CreatedAt: wh.CreatedAt.Time,
	})
}

// List handles GET /v1/webhooks
func (h *WebhookSubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetProjectID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	webhooks, err := h.queries.ListWebhooksByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list webhooks")
		return
	}

	result := make([]webhookResponse, len(webhooks))
	for i, wh := range webhooks {
		result[i] = webhookResponse{
			ID:        wh.ID,
			URL:       wh.Url,
			Events:    wh.Events,
			Active:    wh.Active,
			CreatedAt: wh.CreatedAt.Time,
		}
	}

	writeSuccessWithMeta(w, result, len(result))
}
