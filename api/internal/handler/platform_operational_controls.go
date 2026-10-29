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
	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
)

var errPlatformOperationDisabled = errors.New("platform operation disabled")

func evaluatePlatformOperation(
	ctx context.Context,
	evaluator platformcontrols.Evaluator,
	platformName string,
	operation platformcontrols.Operation,
) (platformcontrols.Decision, error) {
	if evaluator == nil {
		return platformcontrols.Decision{}, nil
	}
	decision, err := evaluator.Evaluate(ctx, platformName, operation)
	if err != nil {
		return decision, err
	}
	if decision.Blocked {
		return decision, errPlatformOperationDisabled
	}
	return decision, nil
}

func writePlatformOperationUnavailable(w http.ResponseWriter, decision platformcontrols.Decision) {
	writeError(w, http.StatusServiceUnavailable, platformcontrols.APICode, decisionMessage(decision))
}

func decisionMessage(decision platformcontrols.Decision) string {
	message := strings.TrimSpace(decision.Message)
	if message == "" {
		return "This platform is temporarily unavailable."
	}
	return message
}

type platformOperationalControlsAdminService interface {
	platformcontrols.Evaluator
	List(context.Context) ([]platformcontrols.Control, error)
	SetState(context.Context, platformcontrols.TransitionRequest) (platformcontrols.TransitionResult, error)
}

type PlatformOperationalControlsHandler struct {
	service platformOperationalControlsAdminService
}

func NewPlatformOperationalControlsHandler(service platformOperationalControlsAdminService) *PlatformOperationalControlsHandler {
	return &PlatformOperationalControlsHandler{service: service}
}

func (h *PlatformOperationalControlsHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Platform operational controls are not configured")
		return
	}
	controls, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform operational controls")
		return
	}
	writeSuccess(w, controls)
}

func (h *PlatformOperationalControlsHandler) AdminSet(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Platform operational controls are not configured")
		return
	}
	platformName := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform")))
	var body struct {
		State           platformcontrols.State `json:"state"`
		ExpectedVersion int64                  `json:"expected_version"`
		ReasonCode      string                 `json:"reason_code"`
		UserMessage     string                 `json:"user_message"`
	}
	if platformName == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "platform is required")
		return
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || !body.State.Valid() || body.ExpectedVersion <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "a valid state and positive expected_version are required")
		return
	}
	actor := auth.GetUserID(r.Context())
	if actor == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	result, err := h.service.SetState(r.Context(), platformcontrols.TransitionRequest{
		Platform: platformName, State: body.State, ExpectedVersion: body.ExpectedVersion,
		ReasonCode: body.ReasonCode, UserMessage: body.UserMessage,
		ActorUserID: actor, RequestID: r.Header.Get("X-Request-Id"), ActorIP: ip, ActorUserAgent: r.UserAgent(),
	})
	var conflict *platformcontrols.VersionConflictError
	if errors.As(err, &conflict) {
		writeErrorWithDetails(w, http.StatusConflict, "VERSION_CONFLICT", "Platform operational control changed; reload and confirm again", ErrorDetails{
			Details: map[string]any{"current": conflict.Current},
		})
		return
	}
	if errors.Is(err, platformcontrols.ErrUnsupportedPlatform) || errors.Is(err, platformcontrols.ErrUnsupportedState) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update platform operational control")
		return
	}
	writeSuccess(w, result)
}
