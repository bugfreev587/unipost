// tutorials.go handles the multi-tutorial framework.
//
// Endpoints:
//   GET  /v1/me/tutorials                  — list all tutorials + per-step counts
//   POST /v1/me/tutorials/{id}/complete    — stamp completed_at
//   POST /v1/me/tutorials/{id}/dismiss     — stamp dismissed_at
//   POST /v1/me/tutorials/{id}/reopen      — clear dismissed_at (mandatory re-pop)
//
// Step completion within a tutorial is derived from real counts (see
// GetUserActivationCounts) so the UI always reflects reality. Only
// tutorial-level completed_at and dismissed_at are persisted.

package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type TutorialsHandler struct {
	queries *db.Queries
}

func NewTutorialsHandler(queries *db.Queries) *TutorialsHandler {
	return &TutorialsHandler{queries: queries}
}

// Known tutorial IDs. Kept here as a belt-and-suspenders allow-list so
// arbitrary strings can't be stamped via the complete/dismiss endpoints.
// Add new tutorial IDs here when registering a new tutorial.
var knownTutorials = map[string]bool{
	"quickstart":     true,
	"post_with_api":  true,
}

type tutorialState struct {
	ID          string  `json:"id"`
	CompletedAt *string `json:"completed_at,omitempty"`
	DismissedAt *string `json:"dismissed_at,omitempty"`
}

type tutorialsResponse struct {
	Tutorials []tutorialState `json:"tutorials"`
	// Activation counts are returned alongside so the frontend can
	// derive per-step completion in a single round-trip. These replace
	// the old /v1/me/activation endpoint's Steps payload.
	Counts tutorialsCounts `json:"counts"`
}

type tutorialsCounts struct {
	ConnectedAccounts int `json:"connected_accounts"`
	PostsSent         int `json:"posts_sent"`
	ApiKeys           int `json:"api_keys"`
}

// List handles GET /v1/me/tutorials.
func (h *TutorialsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	rows, err := h.queries.ListUserTutorials(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load tutorials: "+err.Error())
		return
	}

	counts, err := h.queries.GetUserActivationCounts(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load counts: "+err.Error())
		return
	}

	states := make([]tutorialState, 0, len(rows))
	for _, row := range rows {
		s := tutorialState{ID: row.TutorialID}
		if row.CompletedAt.Valid {
			v := row.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			s.CompletedAt = &v
		}
		if row.DismissedAt.Valid {
			v := row.DismissedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			s.DismissedAt = &v
		}
		states = append(states, s)
	}

	writeSuccess(w, tutorialsResponse{
		Tutorials: states,
		Counts: tutorialsCounts{
			ConnectedAccounts: int(counts.ConnectedAccountsCount),
			PostsSent:         int(counts.PostsSentCount),
			ApiKeys:           int(counts.ApiKeysCount),
		},
	})
}

// Complete handles POST /v1/me/tutorials/{id}/complete.
func (h *TutorialsHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	tutorialID := chi.URLParam(r, "id")
	if !knownTutorials[tutorialID] {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown tutorial: "+tutorialID)
		return
	}

	row, err := h.queries.CompleteUserTutorial(r.Context(), db.CompleteUserTutorialParams{
		UserID:     userID,
		TutorialID: tutorialID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to complete tutorial: "+err.Error())
		return
	}

	s := tutorialState{ID: row.TutorialID}
	if row.CompletedAt.Valid {
		v := row.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		s.CompletedAt = &v
	}
	writeSuccess(w, s)
}

// Dismiss handles POST /v1/me/tutorials/{id}/dismiss.
func (h *TutorialsHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	tutorialID := chi.URLParam(r, "id")
	if !knownTutorials[tutorialID] {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown tutorial: "+tutorialID)
		return
	}

	row, err := h.queries.DismissUserTutorial(r.Context(), db.DismissUserTutorialParams{
		UserID:     userID,
		TutorialID: tutorialID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to dismiss tutorial: "+err.Error())
		return
	}

	s := tutorialState{ID: row.TutorialID}
	if row.DismissedAt.Valid {
		v := row.DismissedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		s.DismissedAt = &v
	}
	writeSuccess(w, s)
}

// Reopen handles POST /v1/me/tutorials/{id}/reopen.
// Clears the dismissed_at timestamp so a mandatory tutorial can re-pop
// when the user returns to the profile page. Does not touch
// completed_at (re-opening a completed tutorial is a replay, not a
// re-dismissal reset).
func (h *TutorialsHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	tutorialID := chi.URLParam(r, "id")
	if !knownTutorials[tutorialID] {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown tutorial: "+tutorialID)
		return
	}

	if err := h.queries.ClearUserTutorialDismissal(r.Context(), db.ClearUserTutorialDismissalParams{
		UserID:     userID,
		TutorialID: tutorialID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reopen tutorial: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
