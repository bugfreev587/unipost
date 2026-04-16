// activation.go implements the dashboard empty-state activation guide.
//
// The card shows 3 steps derived from real data counters:
//   1. Connect your first account      (connected_accounts_count >= 1)
//   2. Send your first post            (posts_sent_count       >= 1)
//   3. Get your API key                (api_keys_count         >= 1)
//
// Step state is never stored per-step — we always compute from the actual
// counts. This keeps the UI consistent with reality (e.g., if a user deletes
// their only account, step 1 correctly reverts to incomplete).
//
// Two lifecycle bits ARE persisted on the user row:
//   - activation_completed_at:       stamped once all three counts are met
//   - activation_guide_dismissed_at: stamped when user clicks Dismiss
//
// Either one hides the card permanently.

package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type ActivationHandler struct {
	queries *db.Queries
}

func NewActivationHandler(queries *db.Queries) *ActivationHandler {
	return &ActivationHandler{queries: queries}
}

type activationStep struct {
	ID        string `json:"id"`
	Completed bool   `json:"completed"`
	Count     int    `json:"count"`
}

type activationProgress struct {
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

type activationResponse struct {
	Completed bool               `json:"completed"`
	Dismissed bool               `json:"dismissed"`
	Steps     []activationStep   `json:"steps"`
	Progress  activationProgress `json:"progress"`
}

// Get handles GET /v1/me/activation.
// Returns the three step states + aggregate progress. When all three
// thresholds are met for the first time, stamps activation_completed_at
// so we know not to re-celebrate on the next load.
func (h *ActivationHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	counts, err := h.queries.GetUserActivationCounts(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load counts: "+err.Error())
		return
	}

	user, err := h.queries.GetUser(r.Context(), userID)
	if err != nil && err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user: "+err.Error())
		return
	}

	steps := []activationStep{
		{ID: "connect_account", Completed: counts.ConnectedAccountsCount >= 1, Count: int(counts.ConnectedAccountsCount)},
		{ID: "send_post", Completed: counts.PostsSentCount >= 1, Count: int(counts.PostsSentCount)},
		{ID: "create_api_key", Completed: counts.ApiKeysCount >= 1, Count: int(counts.ApiKeysCount)},
	}
	completedCount := 0
	for _, s := range steps {
		if s.Completed {
			completedCount++
		}
	}

	// First time all three are met → stamp completed_at. The celebration
	// state is frontend-only; the backend just marks the milestone so
	// subsequent loads know not to animate again.
	allDone := completedCount == len(steps)
	if allDone && !user.ActivationCompletedAt.Valid {
		if err := h.queries.MarkActivationCompleted(r.Context(), userID); err != nil {
			// Non-fatal — we can still return the data.
			_ = err
		}
	}

	resp := activationResponse{
		Completed: allDone || user.ActivationCompletedAt.Valid,
		Dismissed: user.ActivationGuideDismissedAt.Valid,
		Steps:     steps,
		Progress:  activationProgress{Completed: completedCount, Total: len(steps)},
	}
	writeSuccess(w, resp)
}

// Dismiss handles POST /v1/me/activation/dismiss.
// Sets the dismissed_at timestamp so the card never reappears.
func (h *ActivationHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	if err := h.queries.DismissActivationGuide(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to dismiss: "+err.Error())
		return
	}
	writeSuccess(w, map[string]string{"dismissed_at": time.Now().Format(time.RFC3339)})
}
