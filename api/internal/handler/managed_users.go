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
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// ManagedUsersHandler owns GET /v1/users and GET /v1/users/{id}.
type ManagedUsersHandler struct {
	queries *db.Queries
}

var errManagedUsersProfileInaccessible = errors.New("managed users profile inaccessible")

func NewManagedUsersHandler(queries *db.Queries) *ManagedUsersHandler {
	return &ManagedUsersHandler{queries: queries}
}

// getProfileID resolves the profile_id this Managed Users call
// should scope its query to. Same shape as the oauth handler's
// resolver after the a43437f leftover bug fix:
//
//  1. URL :profileID — explicit request, ownership-checked
//     against the auth flavor (Clerk-user join or API-key
//     workspace match).
//  2. user.default_profile_id — bare /v1/users hit by a Clerk
//     session falls back to the dashboard's default profile.
//  3. workspace's most-recent profile — last-resort path for
//     API-key callers on the bare route.
//
// Pre-fix the function returned auth.GetWorkspaceID() and treated
// it as a profile id; the SQL filter `profile_id = $1` then never
// matched real rows (workspace.id != profile.id post-Sprint-1
// data model), and the dashboard's "Managed Users" page rendered
// empty even when managed users existed.
func (h *ManagedUsersHandler) getProfileID(r *http.Request) (string, error) {
	ctx := r.Context()
	urlProfileID := chi.URLParam(r, "profileID")
	userID := auth.GetUserID(ctx)
	workspaceID := auth.GetWorkspaceID(ctx)

	if urlProfileID != "" {
		if userID != "" {
			// Clerk-session callers should be authorized via their active
			// workspace membership, not only by direct workspace ownership.
			// Otherwise invited team members hit a silent 401 here and the
			// dashboard renders the empty state even though managed users exist.
			if workspaceID == "" {
				if _, err := h.queries.GetProfileByIDAndWorkspaceOwner(ctx, db.GetProfileByIDAndWorkspaceOwnerParams{
					ID:     urlProfileID,
					UserID: userID,
				}); err == nil {
					return urlProfileID, nil
				} else if errors.Is(err, pgx.ErrNoRows) {
					return "", errManagedUsersProfileInaccessible
				} else {
					return "", err
				}
			}
			prof, err := h.queries.GetProfile(ctx, urlProfileID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && prof.WorkspaceID != workspaceID) {
				return "", errManagedUsersProfileInaccessible
			}
			if err != nil {
				return "", err
			}
			return urlProfileID, nil
		}
		if workspaceID == "" {
			return "", nil
		}
		prof, err := h.queries.GetProfile(ctx, urlProfileID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && prof.WorkspaceID != workspaceID) {
			return "", errManagedUsersProfileInaccessible
		}
		if err != nil {
			return "", err
		}
		return urlProfileID, nil
	}

	if userID != "" {
		if user, err := h.queries.GetUser(ctx, userID); err == nil && user.DefaultProfileID.Valid {
			return user.DefaultProfileID.String, nil
		}
	}

	if workspaceID != "" {
		if profiles, err := h.queries.ListProfilesByWorkspace(ctx, workspaceID); err == nil && len(profiles) > 0 {
			return profiles[0].ID, nil
		}
	}

	return "", nil
}

func (h *ManagedUsersHandler) requireProfileID(w http.ResponseWriter, r *http.Request) (string, bool) {
	profileID, err := h.getProfileID(r)
	if errors.Is(err, errManagedUsersProfileInaccessible) {
		writeError(w, http.StatusNotFound, "PROFILE_INACCESSIBLE", "Profile is unavailable")
		return "", false
	}
	if err != nil {
		slog.Error("managed users profile lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve profile context")
		return "", false
	}
	if profileID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing profile context")
		return "", false
	}
	return profileID, true
}

func writeManagedUserNotFound(w http.ResponseWriter, r *http.Request, legacyMessage string) {
	if chi.URLParam(r, "profileID") != "" {
		writeError(w, http.StatusNotFound, "MANAGED_USER_NOT_FOUND", "Managed User not found")
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", legacyMessage)
}

// managedUserListEntry is one row in the GET /v1/users response.
// Aggregate fields are derived from the profile's social_accounts
// rows on the fly via the SQL GROUP BY in
// ListManagedUsersByProject.
type managedUserListEntry struct {
	ExternalUserID    string         `json:"external_user_id"`
	ExternalUserEmail string         `json:"external_user_email,omitempty"`
	AccountCount      int            `json:"account_count"`
	PlatformCounts    map[string]int `json:"platform_counts"`
	ReconnectCount    int            `json:"reconnect_count"`
	DisconnectedCount int            `json:"disconnected_count"`
	FirstConnectedAt  time.Time      `json:"first_connected_at"`
	LastRefreshedAt   *time.Time     `json:"last_refreshed_at,omitempty"`
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
// Cursor pagination is intentionally NOT in v1 — the typical profile
// has 0–100 managed users and a single LIMIT 100 query is fast enough.
// We can add cursor support in a follow-up if anyone hits the cap.
func (h *ManagedUsersHandler) List(w http.ResponseWriter, r *http.Request) {
	profileID, ok := h.requireProfileID(w, r)
	if !ok {
		return
	}

	limit := int32(25)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}

	rows, err := h.queries.ListManagedUsersByProfile(r.Context(), db.ListManagedUsersByProfileParams{
		ProfileID: profileID,
		Limit:     limit,
	})
	if err != nil {
		slog.Error("list managed users failed", "profile_id", profileID, "err", err)
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
				"youtube":  int(row.YoutubeCount),
			},
			ReconnectCount:    int(row.ReconnectCount),
			DisconnectedCount: int(row.DisconnectedCount),
			FirstConnectedAt:  row.FirstConnectedAt.Time,
		}
		if row.LastRefreshedAt.Valid {
			t := row.LastRefreshedAt.Time
			entry.LastRefreshedAt = &t
		}
		out = append(out, entry)
	}

	// Get the total count for the meta.
	total, _ := h.queries.CountManagedUsersByProfile(r.Context(), profileID)

	writeSuccessWithListMeta(w, out, int(total), int(limit))
}

// Get handles GET /v1/users/{external_user_id}.
//
// Returns the full detail view: header info + every social account
// belonging to that end user. 404 if no managed accounts exist for
// the id within the profile.
func (h *ManagedUsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	profileID, ok := h.requireProfileID(w, r)
	if !ok {
		return
	}
	externalUserID := chi.URLParam(r, "external_user_id")
	if externalUserID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "external_user_id is required")
		return
	}

	accounts, err := h.queries.ListManagedAccountsByExternalUser(r.Context(), db.ListManagedAccountsByExternalUserParams{
		ProfileID:      profileID,
		ExternalUserID: pgtype.Text{String: externalUserID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user accounts")
		return
	}
	if len(accounts) == 0 {
		writeManagedUserNotFound(w, r, "No managed accounts found for that external_user_id")
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

// DismissDisconnected hides all disconnected managed accounts for one
// external_user_id from dashboard connection views without deleting
// historical publishing or inbox data.
func (h *ManagedUsersHandler) DismissDisconnected(w http.ResponseWriter, r *http.Request) {
	profileID, ok := h.requireProfileID(w, r)
	if !ok {
		return
	}
	externalUserID := chi.URLParam(r, "external_user_id")
	if externalUserID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "external_user_id is required")
		return
	}

	rows, err := h.queries.DismissDisconnectedManagedAccountsByExternalUser(r.Context(), db.DismissDisconnectedManagedAccountsByExternalUserParams{
		ProfileID:      profileID,
		ExternalUserID: pgtype.Text{String: externalUserID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to dismiss disconnected accounts")
		return
	}
	if rows == 0 {
		writeManagedUserNotFound(w, r, "No disconnected accounts found for that external_user_id")
		return
	}

	writeSuccess(w, map[string]bool{"dismissed": true})
}
