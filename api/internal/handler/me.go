package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// MeHandler exposes the current Clerk-authenticated user's identity
// plus a flag for whether they're on the ADMIN_USERS allowlist. The
// dashboard polls this on mount to decide whether to render the Admin
// link in the sidebar — keeping the allowlist server-side avoids a
// build-time NEXT_PUBLIC_* env var on the frontend.
type MeHandler struct {
	queries      *db.Queries
	adminChecker *auth.AdminChecker
}

func NewMeHandler(queries *db.Queries, adminChecker *auth.AdminChecker) *MeHandler {
	return &MeHandler{queries: queries, adminChecker: adminChecker}
}

type meResponse struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	IsAdmin bool   `json:"is_admin"`
}

func (h *MeHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	user, err := h.queries.GetUser(r.Context(), userID)
	if err == pgx.ErrNoRows {
		// User authenticated with Clerk but the webhook hasn't synced
		// them into our DB yet. Return a minimal projection so the
		// frontend can still render — is_admin defaults to false.
		writeSuccess(w, meResponse{UserID: userID})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user: "+err.Error())
		return
	}

	writeSuccess(w, meResponse{
		UserID:  user.ID,
		Email:   user.Email,
		Name:    user.Name.String,
		IsAdmin: h.adminChecker.IsAdmin(r.Context(), userID),
	})
}

// bootstrapResponse drives the dashboard root resolver. The two IDs
// answer two distinct questions:
//
//   - default_project_id: where to fall back when the last_project_id
//     is missing or stale (it's the one project the API guarantees
//     can never be deleted, see ProjectHandler.Delete).
//   - last_project_id: where the user was last working — the dashboard
//     `/` route redirects here on every visit so users land back on
//     their most-recent project instead of a generic landing page.
//
// Both fields are nullable strings on the wire so the dashboard can
// distinguish "user has projects, here are the IDs" from "no projects,
// nothing to redirect to" without sentinel values. In practice the
// /bootstrap call lazily backfills both columns, so a successful
// 200 from this endpoint should always include a default_project_id
// for any user who is allowed to use the dashboard.
type bootstrapResponse struct {
	DefaultProfileID *string `json:"default_profile_id"`
	LastProfileID    *string `json:"last_profile_id"`
}

// Bootstrap is the dashboard root resolver. Three states it handles:
//
//  1. Fresh signup, zero projects → create a "Default" project, stamp
//     it as both default_project_id AND last_project_id, return both.
//  2. Legacy user with ≥1 project but no default_project_id (existing
//     accounts pre-dating this migration) → pick the oldest project,
//     stamp it as default_project_id, leave last_project_id alone if
//     already set or stamp it to the same default.
//  3. Returning user → just return what's already on the row, with
//     last_project_id falling back to default_project_id if a deleted
//     project nulled the FK out from under it.
//
// All three branches are idempotent and safe to call on every dashboard
// load. We intentionally don't gate on a "first login" flag — the
// existence of default_project_id IS the gate.
func (h *MeHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	user, err := h.queries.GetUser(r.Context(), userID)
	if err == pgx.ErrNoRows {
		// Clerk webhook hasn't synced this user yet. Don't try to
		// create a project — we'd violate the projects.owner_id FK.
		// Returning nulls is safe; the dashboard will retry on the
		// next page load.
		writeSuccess(w, bootstrapResponse{})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user: "+err.Error())
		return
	}

	defaultID := user.DefaultProfileID
	lastID := user.LastProfileID

	if !defaultID.Valid {
		// Branch 1 vs branch 2: do they already have any projects?
		existing, err := h.queries.ListProfilesByWorkspace(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list projects: "+err.Error())
			return
		}

		var pickedID string
		if len(existing) == 0 {
			// Fresh user — auto-create the Default project. Hard-coded
			// name "Default" per product spec; users can rename it via
			// the project settings page like any other project.
			created, err := h.queries.CreateProfile(r.Context(), db.CreateProfileParams{
				WorkspaceID: userID,
				Name:        "Default",
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create default project: "+err.Error())
				return
			}
			pickedID = created.ID
		} else {
			// Legacy user — backfill default_project_id from the oldest
			// existing project. ListProfilesByWorkspace returns rows in
			// `created_at DESC` order, so the last entry is the oldest.
			pickedID = existing[len(existing)-1].ID
		}

		if err := h.queries.SetUserDefaultProfile(r.Context(), db.SetUserDefaultProfileParams{
			ID:               userID,
			DefaultProfileID: pgtype.Text{String: pickedID, Valid: true},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to set default project: "+err.Error())
			return
		}
		defaultID = pgtype.Text{String: pickedID, Valid: true}

		// If they have no recorded last_project_id (which is always true
		// for branch 1 and usually true for branch 2), seed it to the
		// default so the dashboard root has somewhere to redirect on the
		// very next request.
		if !lastID.Valid {
			if err := h.queries.SetUserLastProfile(r.Context(), db.SetUserLastProfileParams{
				ID:            userID,
				LastProfileID: pgtype.Text{String: pickedID, Valid: true},
			}); err != nil {
				// Non-fatal: the dashboard will fall back to default_project_id.
				lastID = pgtype.Text{}
			} else {
				lastID = pgtype.Text{String: pickedID, Valid: true}
			}
		}
	}

	// last_project_id falls back to default_project_id when the
	// referenced project was deleted (the FK ON DELETE SET NULL nulled
	// it out) or was never set. The default is guaranteed to exist
	// because the Delete handler refuses to drop it.
	resp := bootstrapResponse{}
	if defaultID.Valid {
		v := defaultID.String
		resp.DefaultProfileID = &v
	}
	if lastID.Valid {
		v := lastID.String
		resp.LastProfileID = &v
	} else if defaultID.Valid {
		v := defaultID.String
		resp.LastProfileID = &v
	}
	writeSuccess(w, resp)
}
