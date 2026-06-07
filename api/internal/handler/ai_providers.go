package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/aiproviders"
	"github.com/xiaoboyu/unipost-api/internal/auth"
)

type AIProviderHandler struct {
	service *aiproviders.Service
}

func NewAIProviderHandler(service *aiproviders.Service) *AIProviderHandler {
	return &AIProviderHandler{service: service}
}

func (h *AIProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	status, err := h.service.ListStatus(r.Context())
	if err != nil {
		writeAIProviderError(w, err)
		return
	}
	writeSuccess(w, status)
}

func (h *AIProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	provider := aiproviders.Provider(chi.URLParam(r, "provider"))
	var body struct {
		APIKey        string `json:"api_key"`
		BaseURL       string `json:"base_url"`
		ChatModel     string `json:"chat_model"`
		MessagesModel string `json:"messages_model"`
		Enabled       bool   `json:"enabled"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid provider payload")
		return
	}
	status, err := h.service.SaveProvider(r.Context(), aiproviders.SaveProviderInput{
		Provider:      provider,
		APIKey:        body.APIKey,
		BaseURL:       body.BaseURL,
		ChatModel:     body.ChatModel,
		MessagesModel: body.MessagesModel,
		Enabled:       body.Enabled,
		ActorAdminID:  auth.GetUserID(r.Context()),
	})
	if err != nil {
		writeAIProviderError(w, err)
		return
	}
	writeSuccess(w, status)
}

func (h *AIProviderHandler) Test(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	provider := aiproviders.Provider(chi.URLParam(r, "provider"))
	var body struct {
		APIKey        string `json:"api_key"`
		BaseURL       string `json:"base_url"`
		ChatModel     string `json:"chat_model"`
		MessagesModel string `json:"messages_model"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid provider test payload")
			return
		}
	}
	result, err := h.service.TestProvider(r.Context(), aiproviders.TestProviderInput{
		Provider:      provider,
		APIKey:        body.APIKey,
		BaseURL:       body.BaseURL,
		ChatModel:     body.ChatModel,
		MessagesModel: body.MessagesModel,
		ActorAdminID:  auth.GetUserID(r.Context()),
	})
	if err != nil && !errors.Is(err, aiproviders.ErrProviderKeyRequired) {
		writeAIProviderError(w, err)
		return
	}
	writeSuccess(w, result)
}

func (h *AIProviderHandler) Route(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	surface := aiproviders.Surface(chi.URLParam(r, "surface"))
	var body struct {
		Provider      aiproviders.Provider   `json:"provider"`
		ClientKind    aiproviders.ClientKind `json:"client_kind"`
		ModelOverride string                 `json:"model_override"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid routing payload")
		return
	}
	status, err := h.service.RouteSurface(r.Context(), aiproviders.RouteSurfaceInput{
		Surface:       surface,
		Provider:      body.Provider,
		ClientKind:    body.ClientKind,
		ModelOverride: body.ModelOverride,
		ActorAdminID:  auth.GetUserID(r.Context()),
	})
	if err != nil {
		writeAIProviderError(w, err)
		return
	}
	writeSuccess(w, status)
}

func (h *AIProviderHandler) Unroute(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	surface := aiproviders.Surface(chi.URLParam(r, "surface"))
	if err := h.service.UnrouteSurface(r.Context(), surface, auth.GetUserID(r.Context())); err != nil {
		writeAIProviderError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"surface": surface, "source": aiproviders.SourceEnv})
}

func (h *AIProviderHandler) Disable(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	provider := aiproviders.Provider(chi.URLParam(r, "provider"))
	status, err := h.service.DisableProvider(r.Context(), provider, auth.GetUserID(r.Context()))
	if err != nil {
		writeAIProviderError(w, err)
		return
	}
	writeSuccess(w, status)
}

func (h *AIProviderHandler) Events(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_PROVIDER_NOT_CONFIGURED", "AI provider registry is not configured")
		return
	}
	query := r.URL.Query()
	limit := int32(50)
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid limit")
			return
		}
		limit = int32(parsed)
	}
	var beforeID int64
	if raw := strings.TrimSpace(query.Get("cursor")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid cursor")
			return
		}
		beforeID = parsed
	}
	events, err := h.service.ListEvents(
		r.Context(),
		aiproviders.Provider(query.Get("provider")),
		query.Get("action"),
		beforeID,
		limit,
	)
	if err != nil {
		writeAIProviderError(w, err)
		return
	}
	nextCursor := ""
	if int32(len(events)) == limit && len(events) > 0 {
		nextCursor = strconv.FormatInt(events[len(events)-1].ID, 10)
	}
	writeSuccess(w, map[string]any{
		"events":      events,
		"next_cursor": nextCursor,
	})
}

func writeAIProviderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, aiproviders.ErrProviderKeyRequired):
		writeError(w, http.StatusBadRequest, "AI_PROVIDER_KEY_REQUIRED", "API key is required")
	case errors.Is(err, aiproviders.ErrProviderUnsupported):
		writeError(w, http.StatusBadRequest, "AI_PROVIDER_UNSUPPORTED", "Unsupported AI provider")
	case errors.Is(err, aiproviders.ErrSurfaceUnsupported):
		writeError(w, http.StatusBadRequest, "AI_SURFACE_UNSUPPORTED", "Unsupported AI surface")
	case errors.Is(err, aiproviders.ErrClientKindUnsupported):
		writeError(w, http.StatusBadRequest, "AI_CLIENT_KIND_UNSUPPORTED", "Unsupported AI client kind")
	case errors.Is(err, aiproviders.ErrSurfaceIncompatible):
		writeError(w, http.StatusUnprocessableEntity, "AI_SURFACE_CLIENT_KIND_INCOMPATIBLE", "AI surface does not support that client kind")
	case errors.Is(err, aiproviders.ErrProviderNotConfigured):
		writeError(w, http.StatusNotFound, "AI_PROVIDER_NOT_CONFIGURED", "AI provider is not configured")
	case errors.Is(err, aiproviders.ErrProviderDisabled):
		writeError(w, http.StatusConflict, "AI_PROVIDER_DISABLED", "AI provider is disabled")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "AI provider request failed")
	}
}
