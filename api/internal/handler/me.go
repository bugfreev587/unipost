package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

type MeHandler struct {
	queries           *db.Queries
	adminChecker      *auth.AdminChecker
	superAdminChecker *auth.SuperAdminChecker
	quotaChecker      *quota.Checker
	loopsSyncer       loopsLifecycleSyncer
}

func NewMeHandler(queries *db.Queries, adminChecker *auth.AdminChecker, superAdminChecker *auth.SuperAdminChecker) *MeHandler {
	return &MeHandler{queries: queries, adminChecker: adminChecker, superAdminChecker: superAdminChecker}
}

func (h *MeHandler) SetQuotaChecker(quotaChecker *quota.Checker) *MeHandler {
	h.quotaChecker = quotaChecker
	return h
}

type meResponse struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	IsAdmin bool   `json:"is_admin"`
	// IsSuperAdmin flags users on SUPER_ADMINS. The dashboard uses it
	// to gate in-development features (currently just the Facebook
	// Pages entry in Connections) without a second env var.
	IsSuperAdmin bool `json:"is_super_admin"`
	// WorkspaceID / WorkspaceName surface the user's single workspace
	// so the dashboard can display its name without a separate API
	// round-trip. The Apr 27 "Remove workspace_id from the API surface"
	// refactor took the standalone /v1/workspaces list endpoint away —
	// this is the workspace-aware view that replaces it for Clerk-auth
	// callers. Empty strings when the user has no workspace yet (fresh
	// signup before /v1/me/bootstrap fires).
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	// Role in the current workspace (RBAC migration 060). Surfaced so
	// the dashboard can render role-conditional UI (member-management
	// page, billing-only-for-owner gates, etc.) without a separate
	// /v1/members lookup. Empty when no membership.
	Role string `json:"role,omitempty"`
	// Intent-collection fields. The frontend uses OnboardingShownAt to
	// decide whether to pop the Welcome modal on first dashboard load.
	// OnboardingIntent is one of: "exploring", "own_accounts",
	// "building_api", "skipped", or nil (never answered).
	OnboardingIntent  *string `json:"onboarding_intent,omitempty"`
	OnboardingShownAt *string `json:"onboarding_shown_at,omitempty"`
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
		UserID:       user.ID,
		Email:        user.Email,
		Name:         user.Name.String,
		IsAdmin:      h.adminChecker.IsAdmin(r.Context(), userID),
		IsSuperAdmin: h.superAdminChecker.IsSuperAdminByUser(userID, user.Email),
	}
	// Best-effort workspace + role lookup. RBAC migration 060 made
	// memberships first-class, so we resolve via the membership table
	// (which works for both owners and invited members). Falls back
	// to the legacy "workspace owned by this user" path if the
	// membership lookup fails — defensive for the brief window during
	// migration where backfill hadn't run yet.
	if mem, memErr := h.queries.GetActiveMembership(r.Context(), userID); memErr == nil {
		if ws, wsErr := h.queries.GetWorkspace(r.Context(), mem.WorkspaceID); wsErr == nil {
			resp.WorkspaceID = ws.ID
			resp.WorkspaceName = ws.Name
		}
		resp.Role = mem.Role
	} else if workspaces, wsErr := h.queries.ListWorkspacesByUser(r.Context(), userID); wsErr == nil && len(workspaces) > 0 {
		resp.WorkspaceID = workspaces[0].ID
		resp.WorkspaceName = workspaces[0].Name
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

type planGatesResponse struct {
	PlanGates map[string]bool `json:"plan_gates"`
}

type featureFlagsCompatResponse struct {
	Environment string          `json:"environment"`
	Provider    string          `json:"provider"`
	Flags       map[string]bool `json:"flags"`
	PlanGates   map[string]bool `json:"plan_gates,omitempty"`
}

// PlanGates returns authenticated user's product-package gates. These are plan
// entitlements, not rollout flags.
func (h *MeHandler) PlanGates(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	writeSuccess(w, planGatesResponse{PlanGates: h.planGatesForUser(r, userID)})
}

// FeatureFlagsCompat keeps the old /v1/me/features response shape stable during
// rolling deployments. It does not evaluate remote flags.
func (h *MeHandler) FeatureFlagsCompat(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	writeSuccess(w, featureFlagsCompatResponse{
		Environment: runtimeenv.Current(),
		Provider:    "removed",
		Flags:       map[string]bool{},
		PlanGates:   h.planGatesForUser(r, userID),
	})
}

func (h *MeHandler) planGatesForUser(r *http.Request, userID string) map[string]bool {
	workspaceID := ""
	if mem, err := h.queries.GetActiveMembership(r.Context(), userID); err == nil {
		workspaceID = mem.WorkspaceID
	} else if workspaces, wsErr := h.queries.ListWorkspacesByUser(r.Context(), userID); wsErr == nil && len(workspaces) > 0 {
		workspaceID = workspaces[0].ID
	}
	planGates := map[string]bool{
		"inbox":     false,
		"audit_log": false,
	}
	if workspaceID != "" {
		planGates["inbox"] = h.quotaChecker == nil || h.quotaChecker.PlanAllowsInbox(r.Context(), workspaceID)
		planGates["audit_log"] = h.quotaChecker != nil && h.quotaChecker.PlanAllowsAuditLog(r.Context(), workspaceID)
	}

	return planGates
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
		// Self-provision: the Clerk user.created webhook hasn't landed
		// yet (delivery race) or it failed silently. Fetch the user
		// from Clerk synchronously and insert the users row so the
		// rest of Bootstrap can proceed and stamp default_profile_id.
		// Without this, /welcome → / falls back to /projects empty
		// state, which on Free plan dead-ends at the 1-profile cap
		// when the user clicks "Create your first profile".
		clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
		cu, cerr := clerkuser.Get(r.Context(), userID)
		if cerr != nil {
			slog.Error("bootstrap: clerk user fetch failed", "user_id", userID, "error", cerr)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch user from auth provider")
			return
		}
		email := ""
		for _, e := range cu.EmailAddresses {
			if cu.PrimaryEmailAddressID != nil && e.ID == *cu.PrimaryEmailAddressID {
				email = e.EmailAddress
				break
			}
		}
		if email == "" && len(cu.EmailAddresses) > 0 {
			email = cu.EmailAddresses[0].EmailAddress
		}
		name := ""
		if cu.FirstName != nil {
			name = *cu.FirstName
		}
		if cu.LastName != nil && *cu.LastName != "" {
			if name != "" {
				name += " "
			}
			name += *cu.LastName
		}
		user, err = h.queries.UpsertUser(r.Context(), db.UpsertUserParams{
			ID:    userID,
			Email: email,
			Name:  pgtype.Text{String: name, Valid: name != ""},
		})
		if err != nil {
			// users.email is UNIQUE; ON CONFLICT (id) doesn't help if
			// the conflict is on email (a stale row from a previously
			// deleted Clerk account whose user.deleted webhook never
			// landed). Detect that one specific case, drop the
			// orphaned row, and retry the insert with the new id.
			var pgErr *pgconn.PgError
			if email != "" && errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
				if delErr := h.queries.DeleteUserByEmail(r.Context(), email); delErr != nil {
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to clear orphaned user row: "+delErr.Error())
					return
				}
				slog.Warn("bootstrap: dropped orphaned user row to claim email for new Clerk id", "new_user_id", userID, "email", email)
				user, err = h.queries.UpsertUser(r.Context(), db.UpsertUserParams{
					ID:    userID,
					Email: email,
					Name:  pgtype.Text{String: name, Valid: name != ""},
				})
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user row: "+err.Error())
				return
			}
		}
		slog.Info("bootstrap: self-provisioned missing user row", "user_id", userID, "email", email)
	} else if err != nil {
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

			// RBAC migration 060 — pair the new workspace with an owner
			// membership row so dualauth.GetActiveMembership succeeds on
			// the next request. Best-effort: if the row already exists
			// (race or partial prior bootstrap), the auth self-heal
			// recovers it on the subsequent call.
			if _, memErr := h.queries.CreateMembership(r.Context(), db.CreateMembershipParams{
				WorkspaceID: ws.ID,
				UserID:      userID,
				Role:        "owner",
				InvitedBy:   pgtype.Text{},
			}); memErr != nil {
				slog.Error("bootstrap: failed to create owner membership", "user_id", userID, "workspace_id", ws.ID, "error", memErr)
			}
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

	accountCanceledEvent, notifyLoops := h.prepareLoopsAccountCanceled(r.Context(), userID, time.Now())

	clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
	if _, err := clerkuser.Delete(r.Context(), userID); err != nil {
		slog.Error("delete account: clerk delete failed", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete account: "+err.Error())
		return
	}

	if notifyLoops {
		h.sendLoopsAccountCanceled(r.Context(), accountCanceledEvent)
	}

	slog.Info("delete account: clerk user deleted", "user_id", userID)
	w.WriteHeader(http.StatusNoContent)
}
