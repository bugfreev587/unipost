package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
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
	// Intent-collection fields. The frontend uses OnboardingShownAt to
	// decide whether to pop the Welcome modal on first dashboard load.
	// OnboardingIntent is one of: "exploring", "own_accounts",
	// "building_api", "skipped", or nil (never answered).
	OnboardingIntent   *string `json:"onboarding_intent,omitempty"`
	OnboardingShownAt  *string `json:"onboarding_shown_at,omitempty"`
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

	resp := meResponse{
		UserID:  user.ID,
		Email:   user.Email,
		Name:    user.Name.String,
		IsAdmin: h.adminChecker.IsAdmin(r.Context(), userID),
	}
	if user.OnboardingIntent.Valid {
		v := user.OnboardingIntent.String
		resp.OnboardingIntent = &v
	}
	if user.OnboardingShownAt.Valid {
		v := user.OnboardingShownAt.Time.Format("2006-01-02T15:04:05Z07:00")
		resp.OnboardingShownAt = &v
	}
	writeSuccess(w, resp)
}

// SetIntent handles PATCH /v1/me/intent.
// Records the user's intent selection (or "skipped") from the Welcome modal.
// Never gates any feature — this is purely for personalization/analytics.
func (h *MeHandler) SetIntent(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	var body struct {
		Intent string `json:"intent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	valid := map[string]bool{
		"exploring":    true,
		"own_accounts": true,
		"building_api": true,
		"skipped":      true,
	}
	if !valid[body.Intent] {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid intent: "+body.Intent)
		return
	}

	_, err := h.queries.SetOnboardingIntent(r.Context(), db.SetOnboardingIntentParams{
		ID:               userID,
		OnboardingIntent: pgtype.Text{String: body.Intent, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save intent")
		return
	}
	writeSuccess(w, map[string]string{"intent": body.Intent})
}

// MarkShown handles POST /v1/me/onboarding-shown.
// Stamps onboarding_shown_at on first Welcome modal render so we never
// show it again to the same user, even if they skip.
func (h *MeHandler) MarkShown(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	if err := h.queries.MarkOnboardingShown(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark shown")
		return
	}
	writeSuccess(w, map[string]bool{"ok": true})
}

// bootstrapResponse drives the dashboard root resolver.
//
//   - default_profile_id: the profile the API guarantees can never be
//     deleted (see ProfileHandler.Delete).
//   - last_profile_id: where the user was last working — the dashboard
//     `/` route redirects here on every visit.
type bootstrapResponse struct {
	DefaultProfileID    *string `json:"default_profile_id"`
	LastProfileID       *string `json:"last_profile_id"`
	OnboardingCompleted bool    `json:"onboarding_completed"`
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
		// Resolve the user's workspace, creating one if needed.
		workspaces, err := h.queries.ListWorkspacesByUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list workspaces: "+err.Error())
			return
		}
		var workspaceID string
		if len(workspaces) == 0 {
			// Create a default workspace for this user
			ws, err := h.queries.CreateWorkspace(r.Context(), db.CreateWorkspaceParams{
				UserID: userID,
				Name:   "Default Workspace",
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create workspace: "+err.Error())
				return
			}
			workspaceID = ws.ID
		} else {
			workspaceID = workspaces[0].ID
		}

		existing, err := h.queries.ListProfilesByWorkspace(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list profiles: "+err.Error())
			return
		}

		var pickedID string
		if len(existing) == 0 {
			created, err := h.queries.CreateProfile(r.Context(), db.CreateProfileParams{
				WorkspaceID: workspaceID,
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

	// Lazy-provision default notification channel + subscriptions on the
	// first Bootstrap call. Idempotent — skipped if the user already has
	// at least one channel. Uses the Clerk-verified signup email so the
	// channel is auto-verified. See migration 040 and
	// SupportedNotificationEvents in notifications.go for the defaults.
	ensureDefaultNotifications(r.Context(), h.queries, user)

	// Intent-collection redesign: onboarding is no longer a gate. Always
	// return OnboardingCompleted=true so the dashboard root resolver stops
	// redirecting users to the removed /welcome page. The new Welcome modal
	// on the dashboard handles intent collection non-blockingly.
	resp := bootstrapResponse{
		OnboardingCompleted: true,
	}
	_ = user.OnboardingCompleted // legacy column, no longer drives routing
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

// CompleteOnboarding handles PATCH /v1/me/onboarding.
// Saves the user's name, optionally renames the workspace, saves usage
// modes, and marks onboarding as completed.
func (h *MeHandler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	var body struct {
		FirstName string `json:"first_name"`
		OrgName   string `json:"org_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.FirstName == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "first_name is required")
		return
	}

	// Update user name and mark onboarding completed
	if err := h.queries.CompleteOnboarding(r.Context(), db.CompleteOnboardingParams{
		ID:   userID,
		Name: pgtype.Text{String: body.FirstName, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to complete onboarding")
		return
	}

	// Update workspace name (org name if provided, else "{FirstName}'s
	// Workspace"). The user.created webhook seeds a default
	// workspace at signup, but its name is based on whatever Clerk had at
	// that moment — often empty for email-only signups, giving "Default
	// Workspace". The welcome page is the first place we get a real name,
	// so rename here unconditionally.
	workspaces, err := h.queries.ListWorkspacesByUser(r.Context(), userID)
	if err == nil && len(workspaces) > 0 {
		ws := workspaces[0]
		wsName := body.OrgName
		if wsName == "" {
			wsName = body.FirstName + "'s Workspace"
		}
		h.queries.UpdateWorkspace(r.Context(), db.UpdateWorkspaceParams{
			ID:   ws.ID,
			Name: wsName,
		})
	}

	writeSuccess(w, map[string]bool{"completed": true})
}

// Delete handles DELETE /v1/me.
//
// Deletes the authenticated user from Clerk using the server-side SDK
// (CLERK_SECRET_KEY), which bypasses the "reauthentication required"
// check that Clerk enforces on client-side user.delete() calls.
//
// Clerk fires a user.deleted webhook after deletion, which our
// webhooks handler converts into a DeleteUser DB call. That cascades
// through workspaces/profiles/social_accounts/api_keys/posts via
// ON DELETE CASCADE foreign keys (migration 025).
func (h *MeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
	if _, err := clerkuser.Delete(r.Context(), userID); err != nil {
		slog.Error("delete account: clerk delete failed", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete account: "+err.Error())
		return
	}

	slog.Info("delete account: clerk user deleted", "user_id", userID)
	w.WriteHeader(http.StatusNoContent)
}
