package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/errortriage"
)

type ErrorTriageHandler struct {
	store       *errortriage.PostgresStore
	runner      *errortriage.Service
	emailSender *errortriage.EmailSendService
}

func NewErrorTriageHandler(store *errortriage.PostgresStore, runner *errortriage.Service, emailSender *errortriage.EmailSendService) *ErrorTriageHandler {
	return &ErrorTriageHandler{store: store, runner: runner, emailSender: emailSender}
}

func (h *ErrorTriageHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := h.store.ListRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load error triage runs: "+err.Error())
		return
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	writeSuccessWithListMeta(w, runs, len(runs), limit)
}

func (h *ErrorTriageHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(chi.URLParam(r, "id"))
	detail, err := h.store.GetRunDetail(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Error triage run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load error triage run: "+err.Error())
		return
	}
	writeSuccess(w, detail)
}

func (h *ErrorTriageHandler) CreateRun(w http.ResponseWriter, r *http.Request) {
	opts, err := h.parseManualRunOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	run, err := h.runner.Run(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to run error triage: "+err.Error())
		return
	}
	writeAccepted(w, run)
}

func (h *ErrorTriageHandler) Rerun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(chi.URLParam(r, "id"))
	detail, err := h.store.GetRunDetail(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Error triage run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load error triage run: "+err.Error())
		return
	}
	run, err := h.runner.Run(r.Context(), errortriage.RunOptions{
		RunType:         errortriage.RunTypeManual,
		WindowStart:     detail.Run.WindowStart,
		WindowEnd:       detail.Run.WindowEnd,
		AdminUserID:     auth.GetUserID(r.Context()),
		SupersedesRunID: runID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to rerun error triage: "+err.Error())
		return
	}
	writeAccepted(w, run)
}

func (h *ErrorTriageHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	itemID := strings.TrimSpace(chi.URLParam(r, "id"))
	var body struct {
		WorkflowStatus string `json:"workflow_status"`
		AdminNotes     string `json:"admin_notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}
	if err := h.store.UpdateItem(r.Context(), itemID, body.WorkflowStatus, body.AdminNotes); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update error triage item: "+err.Error())
		return
	}
	writeSuccess(w, map[string]bool{"ok": true})
}

func (h *ErrorTriageHandler) ApproveBugPlan(w http.ResponseWriter, r *http.Request) {
	itemID := strings.TrimSpace(chi.URLParam(r, "id"))
	var body struct {
		AdminNotes string `json:"admin_notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.store.UpdateItem(r.Context(), itemID, string(errortriage.WorkflowStatusCompleted), body.AdminNotes); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to approve bug plan: "+err.Error())
		return
	}
	writeSuccess(w, map[string]bool{"ok": true})
}

func (h *ErrorTriageHandler) SendEmail(w http.ResponseWriter, r *http.Request) {
	itemID := strings.TrimSpace(chi.URLParam(r, "id"))
	recipientID := strings.TrimSpace(chi.URLParam(r, "recipientID"))
	result, err := h.emailSender.SendRecipient(r.Context(), itemID, recipientID, auth.GetUserID(r.Context()))
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not configured") || err.Error() == "loops_not_configured" {
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, "SEND_FAILED", err.Error())
		return
	}
	writeSuccess(w, result)
}

func (h *ErrorTriageHandler) DismissRecipient(w http.ResponseWriter, r *http.Request) {
	recipientID := strings.TrimSpace(chi.URLParam(r, "recipientID"))
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.store.DismissRecipient(r.Context(), recipientID, auth.GetUserID(r.Context()), body.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to dismiss recipient: "+err.Error())
		return
	}
	writeSuccess(w, map[string]bool{"ok": true})
}

func (h *ErrorTriageHandler) parseManualRunOptions(r *http.Request) (errortriage.RunOptions, error) {
	now := time.Now().UTC()
	opts := errortriage.RunOptions{
		RunType:     errortriage.RunTypeManual,
		WindowStart: now.Add(-24 * time.Hour),
		WindowEnd:   now,
		AdminUserID: auth.GetUserID(r.Context()),
	}
	if r.Body == nil || r.ContentLength == 0 {
		return opts, nil
	}
	var body struct {
		WindowStart     string `json:"window_start"`
		WindowEnd       string `json:"window_end"`
		SupersedesRunID string `json:"supersedes_run_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return errortriage.RunOptions{}, errors.New("Invalid JSON body")
	}
	if strings.TrimSpace(body.WindowStart) != "" {
		parsed, err := time.Parse(time.RFC3339, body.WindowStart)
		if err != nil {
			return errortriage.RunOptions{}, errors.New("window_start must be RFC3339")
		}
		opts.WindowStart = parsed
	}
	if strings.TrimSpace(body.WindowEnd) != "" {
		parsed, err := time.Parse(time.RFC3339, body.WindowEnd)
		if err != nil {
			return errortriage.RunOptions{}, errors.New("window_end must be RFC3339")
		}
		opts.WindowEnd = parsed
	}
	if !opts.WindowEnd.After(opts.WindowStart) {
		return errortriage.RunOptions{}, errors.New("window_end must be after window_start")
	}
	opts.SupersedesRunID = strings.TrimSpace(body.SupersedesRunID)
	return opts, nil
}
