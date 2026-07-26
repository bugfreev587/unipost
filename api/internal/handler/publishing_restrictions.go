package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type publishingRestrictionsService interface {
	WorkspaceProjection(context.Context, string) ([]publishingrestrictions.Decision, error)
	ListAdmin(context.Context) ([]publishingrestrictions.Restriction, error)
	SetEnabled(context.Context, publishingrestrictions.TransitionRequest) (publishingrestrictions.TransitionResult, error)
}

type PublishingRestrictionsHandler struct {
	service publishingRestrictionsService
}

func NewPublishingRestrictionsHandler(service publishingRestrictionsService) *PublishingRestrictionsHandler {
	return &PublishingRestrictionsHandler{service: service}
}

func (h *PublishingRestrictionsHandler) WorkspaceProjection(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restrictions are not configured")
		return
	}
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	decisions, err := h.service.WorkspaceProjection(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Publishing policy is temporarily unavailable")
		return
	}
	writeSuccess(w, map[string]any{"restrictions": decisions})
}

func (h *PublishingRestrictionsHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restrictions are not configured")
		return
	}
	restrictions, err := h.service.ListAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load publishing restrictions")
		return
	}
	writeSuccess(w, restrictions)
}

func (h *PublishingRestrictionsHandler) AdminSet(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restrictions are not configured")
		return
	}
	platform := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform")))
	if platform == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "platform is required")
		return
	}
	var body struct {
		Enabled         *bool `json:"enabled"`
		ExpectedVersion int64 `json:"expected_version"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.Enabled == nil || body.ExpectedVersion <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "enabled and a positive expected_version are required")
		return
	}
	actor := auth.GetUserID(r.Context())
	if actor == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	requestID := r.Header.Get("X-Request-Id")
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	result, err := h.service.SetEnabled(r.Context(), publishingrestrictions.TransitionRequest{
		Platform: platform, Enabled: *body.Enabled, ExpectedVersion: body.ExpectedVersion,
		ActorUserID: actor, RequestID: requestID, ActorIP: ip, ActorUserAgent: r.UserAgent(),
	})
	var conflict *publishingrestrictions.VersionConflictError
	if errors.As(err, &conflict) {
		writeErrorWithDetails(w, http.StatusConflict, "VERSION_CONFLICT", "Publishing restriction changed; reload and confirm again", ErrorDetails{
			Details: map[string]any{"current": conflict.Current},
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update publishing restriction")
		return
	}
	writeSuccess(w, result)
}
