package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/changelog"
)

type changelogCandidateStore interface {
	CreateCandidate(rctx context.Context, input changelog.CreateCandidateInput) (changelog.CandidateRecord, bool, error)
	GetCandidate(rctx context.Context, id string) (changelog.CandidateRecord, error)
}

type changelogActionService interface {
	BuildActionLinks(changelog.CandidateRecord) changelog.ActionLinks
	VerifyActionLink(context.Context, string, changelog.Action, int64, string) (changelog.CandidateRecord, error)
	HandleAction(context.Context, changelog.ActionRequest) (changelog.ActionResult, error)
}

type ChangelogAutomationHandler struct {
	store           changelogCandidateStore
	actions         changelogActionService
	automationToken string
}

func NewChangelogAutomationHandler(store changelogCandidateStore, actions changelogActionService, automationToken string) *ChangelogAutomationHandler {
	return &ChangelogAutomationHandler{
		store:           store,
		actions:         actions,
		automationToken: strings.TrimSpace(automationToken),
	}
}

type changelogCandidateResponse struct {
	Candidate changelog.CandidateRecord `json:"candidate"`
	Created   bool                      `json:"created,omitempty"`
	Actions   changelog.ActionLinks     `json:"actions,omitempty"`
}

func (h *ChangelogAutomationHandler) CreateInternalCandidate(w http.ResponseWriter, r *http.Request) {
	if !h.validAutomationToken(r) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid changelog automation token")
		return
	}
	var input changelog.CreateCandidateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}
	record, created, err := h.store.CreateCandidate(r.Context(), input)
	if err != nil {
		writeChangelogError(w, err)
		return
	}
	out := changelogCandidateResponse{
		Candidate: record,
		Created:   created,
		Actions:   h.actions.BuildActionLinks(record),
	}
	if created {
		writeCreated(w, out)
		return
	}
	writeSuccess(w, out)
}

func (h *ChangelogAutomationHandler) GetInternalCandidate(w http.ResponseWriter, r *http.Request) {
	if !h.validAutomationToken(r) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid changelog automation token")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	record, err := h.store.GetCandidate(r.Context(), id)
	if err != nil {
		writeChangelogError(w, err)
		return
	}
	writeSuccess(w, record)
}

func (h *ChangelogAutomationHandler) GetAdminCandidate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	action := changelog.Action(strings.TrimSpace(r.URL.Query().Get("action")))
	expires, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("expires")), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expires must be a unix timestamp")
		return
	}
	signature := strings.TrimSpace(r.URL.Query().Get("signature"))
	record, err := h.actions.VerifyActionLink(r.Context(), id, action, expires, signature)
	if err != nil {
		writeChangelogError(w, err)
		return
	}
	writeSuccess(w, map[string]any{
		"candidate": record,
		"action":    action,
	})
}

func (h *ChangelogAutomationHandler) ConfirmAdminAction(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	var body struct {
		Action    changelog.Action `json:"action"`
		Expires   int64            `json:"expires"`
		Signature string           `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}
	result, err := h.actions.HandleAction(r.Context(), changelog.ActionRequest{
		CandidateID:  id,
		Action:       body.Action,
		ExpiresUnix:  body.Expires,
		Signature:    body.Signature,
		ActorAdminID: auth.GetUserID(r.Context()),
	})
	if err != nil {
		writeChangelogError(w, err)
		return
	}
	writeAccepted(w, result)
}

func (h *ChangelogAutomationHandler) validAutomationToken(r *http.Request) bool {
	if h == nil || h.automationToken == "" {
		return false
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	return header == "Bearer "+h.automationToken
}

func writeChangelogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, changelog.ErrCandidateInvalid):
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
	case errors.Is(err, changelog.ErrCandidateNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Changelog candidate not found")
	case errors.Is(err, changelog.ErrCandidateAlreadyHandled):
		writeError(w, http.StatusConflict, "ALREADY_HANDLED", "Changelog candidate was already handled")
	case errors.Is(err, changelog.ErrInvalidSignature), errors.Is(err, changelog.ErrExpiredSignature):
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", err.Error())
	case errors.Is(err, changelog.ErrUnsupportedAction):
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Unsupported changelog action")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
