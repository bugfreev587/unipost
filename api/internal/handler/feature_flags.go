package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type featureFlagAdminStore interface {
	List(context.Context) ([]featureflags.Flag, error)
	Set(context.Context, string, bool, string) (featureflags.Flag, error)
}

type featureFlagEvaluator interface {
	WorkspaceFlags(context.Context, string) (map[string]bool, error)
	PublicFlags(context.Context) (map[string]bool, error)
}

type FeatureFlagsHandler struct {
	store     featureFlagAdminStore
	evaluator featureFlagEvaluator
}

func NewFeatureFlagsHandler(store featureFlagAdminStore, evaluator featureFlagEvaluator) *FeatureFlagsHandler {
	return &FeatureFlagsHandler{store: store, evaluator: evaluator}
}

type adminFeatureFlagResponse struct {
	featureflags.Flag
	Label           string `json:"label"`
	OwnerArea       string `json:"owner_area"`
	Internal        bool   `json:"internal"`
	ActivationReady bool   `json:"activation_ready"`
}

func (h *FeatureFlagsHandler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Feature flags are not configured")
		return
	}
	flags, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load feature flags")
		return
	}
	out := make([]adminFeatureFlagResponse, 0, len(flags))
	for _, flag := range flags {
		definition, ok := featureflags.DefinitionFor(flag.Key)
		if !ok {
			continue
		}
		out = append(out, adminFeatureFlagResponse{
			Flag:            flag,
			Label:           definition.Label,
			OwnerArea:       definition.OwnerArea,
			Internal:        definition.Internal,
			ActivationReady: definition.ActivationReady,
		})
	}
	writeSuccess(w, out)
}

func (h *FeatureFlagsHandler) UpdateAdmin(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Feature flags are not configured")
		return
	}
	key := chi.URLParam(r, "key")
	definition, ok := featureflags.DefinitionFor(key)
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown feature flag")
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.Enabled == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "enabled must be a boolean")
		return
	}
	if *body.Enabled && !definition.ActivationReady {
		writeError(w, http.StatusConflict, "FLAG_NOT_READY", "This feature flag cannot be enabled until its read path is implemented and accepted")
		return
	}
	actor := auth.GetUserID(r.Context())
	if actor == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	flag, err := h.store.Set(r.Context(), key, *body.Enabled, actor)
	if errors.Is(err, featureflags.ErrUnknownFlag) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown feature flag")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update feature flag")
		return
	}
	writeSuccess(w, adminFeatureFlagResponse{
		Flag:            flag,
		Label:           definition.Label,
		OwnerArea:       definition.OwnerArea,
		Internal:        definition.Internal,
		ActivationReady: definition.ActivationReady,
	})
}

func (h *FeatureFlagsHandler) Public(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.evaluator == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Feature flags are not configured")
		return
	}
	flags, err := h.evaluator.PublicFlags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load feature flags")
		return
	}
	writeSuccess(w, map[string]any{"flags": flags})
}
