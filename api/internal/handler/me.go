package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

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

// bootstrapResponse drives the dashboard root resolver.
//
//   - default_profile_id: the profile the API guarantees can never be
//     deleted (see ProfileHandler.Delete).
//   - last_profile_id: where the user was last working — the dashboard
//     `/` route redirects here on every visit.
type bootstrapResponse struct {
	DefaultProfileID *string `json:"default_profile_id"`
	LastProfileID    *string `json:"last_profile_id"`
}

// Bootstrap is the dashboard root resolver. Three states:
//
//  1. Fresh signup, zero profiles → create a "Default" profile, stamp
//     it as both default_profile_id AND last_profile_id, return both.
//  2. Legacy user with ≥1 profile but no default_profile_id → pick the
//     oldest profile, stamp it as default_profile_id.
//  3. Returning user → return what's already on the row.
func (h *MeHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	user, err := h.queries.GetUser(r.Context(), userID)
	if err == pgx.ErrNoRows {
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
		// Branch 1 vs branch 2: do they already have any profiles?
		// NOTE: ListProfilesByWorkspace expects a workspace_id. During
		// bootstrap the user may not have a workspace yet — the Clerk
		// webhook + migration 025 seed one per user. We pass userID
		// here as a temporary measure; the workspace_id == user_id for
		// the default workspace created by the migration.
		existing, err := h.queries.ListProfilesByWorkspace(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list profiles: "+err.Error())
			return
		}

		var pickedID string
		if len(existing) == 0 {
			created, err := h.queries.CreateProfile(r.Context(), db.CreateProfileParams{
				WorkspaceID: userID,
				Name:        "Default",
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create default profile: "+err.Error())
				return
			}
			pickedID = created.ID
		} else {
			pickedID = existing[len(existing)-1].ID
		}

		if err := h.queries.SetUserDefaultProfile(r.Context(), db.SetUserDefaultProfileParams{
			ID:               userID,
			DefaultProfileID: pgtype.Text{String: pickedID, Valid: true},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to set default profile: "+err.Error())
			return
		}
		defaultID = pgtype.Text{String: pickedID, Valid: true}

		if !lastID.Valid {
			if err := h.queries.SetUserLastProfile(r.Context(), db.SetUserLastProfileParams{
				ID:            userID,
				LastProfileID: pgtype.Text{String: pickedID, Valid: true},
			}); err != nil {
				lastID = pgtype.Text{}
			} else {
				lastID = pgtype.Text{String: pickedID, Valid: true}
			}
		}
	}

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
