// managed_users.go is the Sprint 4 PR5 Managed Users API.
//
// Two endpoints:
//
//	GET /v1/users               (API key) — list view, one row per end user
//	GET /v1/users/{external_id} (API key) — detail view, all accounts for one end user
//
// "Users" here means the customer's END users — the people whose
// social accounts were onboarded via Sprint 3 Connect. They live as
// distinct external_user_id values on social_accounts rows; the
// list endpoint groups + aggregates them. BYO accounts (which have
// NULL external_user_id) are excluded entirely — this view is for
// managed multi-tenant Connect users only.

package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// ManagedUsersHandler owns GET /v1/users and GET /v1/users/{id}.
type ManagedUsersHandler struct {
	queries *db.Queries
}

func NewManagedUsersHandler(queries *db.Queries) *ManagedUsersHandler {
	return &ManagedUsersHandler{queries: queries}
}

// getProjectID resolves the project context for either auth mode.
// API key callers carry it on the request context (set by the
// API key middleware). Dashboard callers come through the
// /v1/projects/{projectID}/users path and the project is the URL
// param — we additionally verify the requesting Clerk user owns it.
func (h *ManagedUsersHandler) getProjectID(r *http.Request) string {
	if pid := auth.GetProjectID(r.Context()); pid != "" {
		return pid
	}
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	if _, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID:      projectID,
		OwnerID: userID,
	}); err != nil {
		return ""
	}
	return projectID
}

// managedUserListEntry is one row in the GET /v1/users response.
// Aggregate fields are derived from the project's social_accounts
// rows on the fly via the SQL GROUP BY in
// ListManagedUsersByProject.
type managedUserListEntry struct {
	ExternalUserID    string                      `json:"external_user_id"`
	ExternalUserEmail string                      `json:"external_user_email,omitempty"`
	AccountCount      int                         `json:"account_count"`
	PlatformCounts    map[string]int              `json:"platform_counts"`
	ReconnectCount    int                         `json:"reconnect_count"`
	FirstConnectedAt  time.Time                   `json:"first_connected_at"`
	LastRefreshedAt   *time.Time                  `json:"last_refreshed_at,omitempty"`
}

// managedUserDetail is the GET /v1/users/{id} response shape. Same
// header info as the list entry plus the full per-account list so
// the dashboard detail page can render account cards.
type managedUserDetail struct {
	ExternalUserID    string                  `json:"external_user_id"`
	ExternalUserEmail string                  `json:"external_user_email,omitempty"`
	AccountCount      int                     `json:"account_count"`
	Accounts          []socialAccountResponse `json:"accounts"`
}

// List handles GET /v1/users.
//
// Query params:
//   - limit  (optional, 1–100, default 25)
//
// Cursor pagination is intentionally NOT in v1 — the typical project
// has 0–100 managed users and a single LIMIT 100 query is fast enough.
// We can add cursor support in a follow-up if anyone hits the cap.
func (h *ManagedUsersHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	limit := int32(25)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}

	rows, err := h.queries.ListManagedUsersByProject(r.Context(), db.ListManagedUsersByProjectParams{
		ProjectID: projectID,
		Limit:     limit,
	})
	if err != nil {
		slog.Error("list managed users failed", "project_id", projectID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list managed users")
		return
	}

	out := make([]managedUserListEntry, 0, len(rows))
	for _, row := range rows {
		entry := managedUserListEntry{
			ExternalUserID:    row.ExternalUserID,
			ExternalUserEmail: row.ExternalUserEmail,
			AccountCount:      int(row.AccountCount),
			PlatformCounts: map[string]int{
				"twitter":  int(row.TwitterCount),
				"linkedin": int(row.LinkedinCount),
				"bluesky":  int(row.BlueskyCount),
			},
			ReconnectCount:   int(row.ReconnectCount),
			FirstConnectedAt: row.FirstConnectedAt.Time,
		}
		if row.LastRefreshedAt.Valid {
			t := row.LastRefreshedAt.Time
			entry.LastRefreshedAt = &t
		}
		out = append(out, entry)
	}

	// Get the total count for the meta.
	total, _ := h.queries.CountManagedUsersByProject(r.Context(), projectID)

	writeSuccessWithMeta(w, out, int(total))
}

// Get handles GET /v1/users/{external_user_id}.
//
// Returns the full detail view: header info + every social account
// belonging to that end user. 404 if no managed accounts exist for
// the id within the project.
func (h *ManagedUsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}
	externalUserID := chi.URLParam(r, "external_user_id")
	if externalUserID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "external_user_id is required")
		return
	}

	accounts, err := h.queries.ListManagedAccountsByExternalUser(r.Context(), db.ListManagedAccountsByExternalUserParams{
		ProjectID:      projectID,
		ExternalUserID: pgtype.Text{String: externalUserID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user accounts")
		return
	}
	if len(accounts) == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No managed accounts found for that external_user_id")
		return
	}

	// Pick the most recent non-empty external_user_email across the
	// accounts (matches the ListManagedUsersByProject aggregation).
	email := ""
	for _, a := range accounts {
		if a.ExternalUserEmail.Valid && a.ExternalUserEmail.String != "" {
			email = a.ExternalUserEmail.String
			break
		}
	}

	out := managedUserDetail{
		ExternalUserID:    externalUserID,
		ExternalUserEmail: email,
		AccountCount:      len(accounts),
		Accounts:          make([]socialAccountResponse, 0, len(accounts)),
	}
	for _, a := range accounts {
		out.Accounts = append(out.Accounts, toSocialAccountResponse(a))
	}

	writeSuccess(w, out)
}
