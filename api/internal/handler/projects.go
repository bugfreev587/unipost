package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type ProjectHandler struct {
	queries *db.Queries
}

func NewProjectHandler(queries *db.Queries) *ProjectHandler {
	return &ProjectHandler{queries: queries}
}

type projectResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// Sprint 4 PR4: white-label Connect branding. All three are
	// optional; the hosted Connect page falls back to UniPost defaults
	// when null. Pointer types so absent vs empty-string is meaningful
	// in the JSON output (omitempty drops nulls but keeps empty strings).
	BrandingLogoURL      *string `json:"branding_logo_url,omitempty"`
	BrandingDisplayName  *string `json:"branding_display_name,omitempty"`
	BrandingPrimaryColor *string `json:"branding_primary_color,omitempty"`
}

func toProjectResponse(p db.Profile) projectResponse {
	resp := projectResponse{
		ID:          p.ID,
		WorkspaceID: p.WorkspaceID,
		Name:        p.Name,
		CreatedAt:   p.CreatedAt.Time,
		UpdatedAt:   p.UpdatedAt.Time,
	}
	if p.BrandingLogoUrl.Valid {
		v := p.BrandingLogoUrl.String
		resp.BrandingLogoURL = &v
	}
	if p.BrandingDisplayName.Valid {
		v := p.BrandingDisplayName.String
		resp.BrandingDisplayName = &v
	}
	if p.BrandingPrimaryColor.Valid {
		v := p.BrandingPrimaryColor.String
		resp.BrandingPrimaryColor = &v
	}
	return resp
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")

	profiles, err := h.queries.ListProfilesByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list profiles")
		return
	}

	result := make([]projectResponse, len(profiles))
	for i, p := range profiles {
		result[i] = toProjectResponse(p)
	}

	writeSuccessWithMeta(w, result, len(result))
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name is required")
		return
	}

	profile, err := h.queries.CreateProfile(r.Context(), db.CreateProfileParams{
		WorkspaceID: workspaceID,
		Name:        body.Name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create profile")
		return
	}

	writeCreated(w, toProjectResponse(profile))
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	profileID := chi.URLParam(r, "id")

	profile, err := h.queries.GetProfileByIDAndWorkspaceOwner(r.Context(), db.GetProfileByIDAndWorkspaceOwnerParams{
		ID:     profileID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}

	// Stamp last_profile_id as a side-effect of fetching a profile the
	// user owns. The dashboard hits this endpoint on every profile page
	// mount, so it's the natural hook for "last visited" tracking. We
	// log on failure but never block the response — the user-facing
	// page must render even if the side-effect write fails.
	if err := h.queries.SetUserLastProfile(r.Context(), db.SetUserLastProfileParams{
		ID:            userID,
		LastProfileID: pgtype.Text{String: profileID, Valid: true},
	}); err != nil {
		slog.Warn("failed to update last_profile_id", "user_id", userID, "profile_id", profileID, "error", err)
	}

	writeSuccess(w, toProjectResponse(profile))
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	profileID := chi.URLParam(r, "id")

	// Verify ownership
	_, err := h.queries.GetProfileByIDAndWorkspaceOwner(r.Context(), db.GetProfileByIDAndWorkspaceOwnerParams{
		ID:     profileID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}

	// Sprint 4 PR4: pointer fields distinguish absent (don't touch)
	// from explicit empty string (clear the value). Both are useful
	// — customers patching just the logo shouldn't have to re-supply
	// name + color, and customers wanting to remove a logo entirely
	// need a way to clear it.
	var body struct {
		Name                   *string  `json:"name"`
		BrandingLogoURL        *string  `json:"branding_logo_url"`
		BrandingDisplayName    *string  `json:"branding_display_name"`
		BrandingPrimaryColor   *string  `json:"branding_primary_color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	// Validate name if provided.
	if body.Name != nil && *body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name cannot be empty")
		return
	}

	// Validate branding fields if provided.
	if body.BrandingLogoURL != nil && *body.BrandingLogoURL != "" {
		if err := validateBrandingLogoURL(*body.BrandingLogoURL); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}
	}
	if body.BrandingDisplayName != nil && len(*body.BrandingDisplayName) > 60 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"branding_display_name must be ≤ 60 chars")
		return
	}
	if body.BrandingPrimaryColor != nil && *body.BrandingPrimaryColor != "" {
		if !isHexColor(*body.BrandingPrimaryColor) {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
				"branding_primary_color must be a 6-digit hex color (e.g. #10b981)")
			return
		}
	}

	// Apply name update first (separate query — keeps the existing
	// UpdateProfile signature unchanged for callers that don't touch
	// branding).
	if body.Name != nil {
		if _, err := h.queries.UpdateProfile(r.Context(), db.UpdateProfileParams{
			ID:   profileID,
			Name: *body.Name,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update profile")
			return
		}
	}

	// Apply branding updates if any of the three fields was provided.
	// COALESCE on the SQL side leaves untouched columns alone.
	if body.BrandingLogoURL != nil || body.BrandingDisplayName != nil || body.BrandingPrimaryColor != nil {
		if _, err := h.queries.UpdateProfileBranding(r.Context(), db.UpdateProfileBrandingParams{
			ID:           profileID,
			LogoUrl:      pgTextFromPtr(body.BrandingLogoURL),
			DisplayName:  pgTextFromPtr(body.BrandingDisplayName),
			PrimaryColor: pgTextFromPtr(body.BrandingPrimaryColor),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update branding")
			return
		}
	}

	// Re-fetch so the response reflects the latest state including
	// columns we didn't touch.
	final, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read profile")
		return
	}
	writeSuccess(w, toProjectResponse(final))
}

// pgTextFromPtr converts an optional *string into the pgtype.Text
// shape sqlc expects. nil → invalid (the SQL UPDATE leaves the
// column alone via COALESCE); non-nil → valid even when empty
// (the caller is explicitly clearing the value).
func pgTextFromPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

// validateBrandingLogoURL enforces https:// + length cap. We don't
// fetch the URL — that would slow down the dashboard PATCH and isn't
// the API's job to police.
func validateBrandingLogoURL(raw string) error {
	if len(raw) > 512 {
		return errors.New("branding_logo_url must be ≤ 512 chars")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("branding_logo_url is not a valid URL: " + err.Error())
	}
	if u.Scheme != "https" {
		return errors.New("branding_logo_url must use https://")
	}
	if u.Host == "" {
		return errors.New("branding_logo_url must have a host")
	}
	return nil
}

// isHexColor matches "#RRGGBB" — six hex digits with a leading hash.
// Three-digit shorthand and rgba() are not accepted; the dashboard
// color picker should always emit the long form.
func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	profileID := chi.URLParam(r, "id")

	// Verify ownership
	_, err := h.queries.GetProfileByIDAndWorkspaceOwner(r.Context(), db.GetProfileByIDAndWorkspaceOwnerParams{
		ID:     profileID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}

	// Refuse to drop the user's auto-created Default profile — it's the
	// guaranteed fallback the dashboard root resolver redirects to when
	// the user has nowhere else to go. Without this guard a user could
	// delete their Default and end up with no profile at all (zero-state
	// would be self-healing via /me/bootstrap, but we'd lose the "can't
	// delete the default" guarantee that documentation will rely on).
	user, err := h.queries.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user")
		return
	}
	if user.DefaultProfileID.Valid && user.DefaultProfileID.String == profileID {
		writeError(w, http.StatusConflict, "DEFAULT_PROFILE_PROTECTED", "The default profile cannot be deleted")
		return
	}

	if err := h.queries.DeleteProfile(r.Context(), profileID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
