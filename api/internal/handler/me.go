package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5"

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
