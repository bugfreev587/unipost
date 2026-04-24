package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type ProfileHandler struct {
	queries *db.Queries
}

func NewProfileHandler(queries *db.Queries) *ProfileHandler {
	return &ProfileHandler{queries: queries}
}

// resolveWorkspaceID returns the workspace ID from the URL param if
// present, otherwise looks up the user's default workspace. Returns
// empty string if no workspace found.
func (h *ProfileHandler) resolveWorkspaceID(r *http.Request) string {
	if workspaceID := auth.GetWorkspaceID(r.Context()); workspaceID != "" {
		return workspaceID
	}
	if wid := chi.URLParam(r, "workspaceID"); wid != "" {
		return wid
	}
	userID := auth.GetUserID(r.Context())
	workspaces, err := h.queries.ListWorkspacesByUser(r.Context(), userID)
	if err != nil || len(workspaces) == 0 {
		return ""
	}
	return workspaces[0].ID
}

type profileResponse struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Name         string    `json:"name"`
	AccountCount int       `json:"account_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// White-label Connect branding. All three are optional; the hosted
	// Connect page falls back to UniPost defaults when null.
	BrandingLogoURL      *string `json:"branding_logo_url,omitempty"`
	BrandingDisplayName  *string `json:"branding_display_name,omitempty"`
	BrandingPrimaryColor *string `json:"branding_primary_color,omitempty"`
}

func toProfileResponse(p db.Profile) profileResponse {
	resp := profileResponse{
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

func (h *ProfileHandler) profileAccountCount(ctx context.Context, profileID string) int {
	accounts, err := h.queries.ListAllSocialAccountsByProfile(ctx, profileID)
	if err != nil {
		return 0
	}
	return len(accounts)
}

func (h *ProfileHandler) enrichProfileResponse(ctx context.Context, profile db.Profile) profileResponse {
	resp := toProfileResponse(profile)
	resp.AccountCount = h.profileAccountCount(ctx, profile.ID)
	return resp
}

func (h *ProfileHandler) enrichProfileResponses(ctx context.Context, profiles []db.Profile) []profileResponse {
	result := make([]profileResponse, len(profiles))
	for i, p := range profiles {
		result[i] = h.enrichProfileResponse(ctx, p)
	}
	return result
}

func normalizeProfileName(name string) string {
	return strings.TrimSpace(name)
}

func (h *ProfileHandler) profileNameTaken(ctx context.Context, workspaceID, name, excludeID string) bool {
	name = normalizeProfileName(name)
	if name == "" {
		return false
	}
	profiles, err := h.queries.ListProfilesByWorkspace(ctx, workspaceID)
	if err != nil {
		return false
	}
	for _, profile := range profiles {
		if excludeID != "" && profile.ID == excludeID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(profile.Name), name) {
			return true
		}
	}
	return false
}

func (h *ProfileHandler) ensureProfileDeletable(ctx context.Context, profileID, defaultProfileID string) *httpError {
	if defaultProfileID != "" && defaultProfileID == profileID {
		return &httpError{
			status: http.StatusConflict,
			code:   "DEFAULT_PROFILE_PROTECTED",
			msg:    "The default profile cannot be deleted",
		}
	}
	if count := h.profileAccountCount(ctx, profileID); count > 0 {
		return &httpError{
			status: http.StatusConflict,
			code:   "PROFILE_NOT_EMPTY",
			msg:    "Profile has connected accounts; disconnect them before deleting the profile",
		}
	}
	return nil
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeSuccessWithListMeta(w, []profileResponse{}, 0, 0)
		return
	}

	profiles, err := h.queries.ListProfilesByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list profiles")
		return
	}

	result := h.enrichProfileResponses(r.Context(), profiles)
	writeSuccessWithListMeta(w, result, len(result), len(result))
}

func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "No workspace found for this user")
		return
	}

	var body struct {
		Name                 string  `json:"name"`
		BrandingLogoURL      *string `json:"branding_logo_url"`
		BrandingDisplayName  *string `json:"branding_display_name"`
		BrandingPrimaryColor *string `json:"branding_primary_color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	body.Name = normalizeProfileName(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name is required")
		return
	}
	if h.profileNameTaken(r.Context(), workspaceID, body.Name, "") {
		writeError(w, http.StatusConflict, "PROFILE_NAME_TAKEN", "A profile with that name already exists in this workspace")
		return
	}
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
	if body.BrandingPrimaryColor != nil && *body.BrandingPrimaryColor != "" && !isHexColor(*body.BrandingPrimaryColor) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"branding_primary_color must be a 6-digit hex color (e.g. #10b981)")
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
	if body.BrandingLogoURL != nil || body.BrandingDisplayName != nil || body.BrandingPrimaryColor != nil {
		profile, err = h.queries.UpdateProfileBranding(r.Context(), db.UpdateProfileBrandingParams{
			ID:           profile.ID,
			LogoUrl:      pgTextFromPtr(body.BrandingLogoURL),
			DisplayName:  pgTextFromPtr(body.BrandingDisplayName),
			PrimaryColor: pgTextFromPtr(body.BrandingPrimaryColor),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update branding")
			return
		}
	}

	writeCreated(w, h.enrichProfileResponse(r.Context(), profile))
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	if err := h.queries.SetUserLastProfile(r.Context(), db.SetUserLastProfileParams{
		ID:            userID,
		LastProfileID: pgtype.Text{String: profileID, Valid: true},
	}); err != nil {
		slog.Warn("failed to update last_profile_id", "user_id", userID, "profile_id", profileID, "error", err)
	}

	writeSuccess(w, h.enrichProfileResponse(r.Context(), profile))
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Name                 *string `json:"name"`
		BrandingLogoURL      *string `json:"branding_logo_url"`
		BrandingDisplayName  *string `json:"branding_display_name"`
		BrandingPrimaryColor *string `json:"branding_primary_color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Name != nil {
		trimmed := normalizeProfileName(*body.Name)
		if trimmed == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name cannot be empty")
			return
		}
		if h.profileNameTaken(r.Context(), profile.WorkspaceID, trimmed, profileID) {
			writeError(w, http.StatusConflict, "PROFILE_NAME_TAKEN", "A profile with that name already exists in this workspace")
			return
		}
		*body.Name = trimmed
	}

	if body.Name != nil && *body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name cannot be empty")
		return
	}

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

	if body.Name != nil {
		if _, err := h.queries.UpdateProfile(r.Context(), db.UpdateProfileParams{
			ID:   profileID,
			Name: *body.Name,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update profile")
			return
		}
	}

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

	final, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read profile")
		return
	}
	writeSuccess(w, h.enrichProfileResponse(r.Context(), final))
}

func pgTextFromPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

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

func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	profileID := chi.URLParam(r, "id")

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

	user, err := h.queries.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user")
		return
	}
	if user.DefaultProfileID.Valid && user.DefaultProfileID.String == profileID {
		writeError(w, http.StatusConflict, "DEFAULT_PROFILE_PROTECTED", "The default profile cannot be deleted")
		return
	}

	if profileErr := h.ensureProfileDeletable(r.Context(), profileID, user.DefaultProfileID.String); profileErr != nil {
		writeError(w, profileErr.status, profileErr.code, profileErr.msg)
		return
	}

	if err := h.queries.DeleteProfile(r.Context(), profileID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── API key auth routes ─────────────────────────────────────────────

// APIList returns profiles for the workspace associated with the API key.
// API key auth route: GET /v1/profiles
func (h *ProfileHandler) APIList(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	profiles, err := h.queries.ListProfilesByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list profiles")
		return
	}
	result := h.enrichProfileResponses(r.Context(), profiles)
	writeSuccessWithListMeta(w, result, len(result), len(result))
}

// APICreate handles POST /v1/profiles under API-key auth.
func (h *ProfileHandler) APICreate(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	h.Create(w, r.WithContext(auth.SetWorkspaceID(r.Context(), workspaceID)))
}

// APIGet returns a single profile, verified to belong to the API key's workspace.
// API key auth route: GET /v1/profiles/{id}
func (h *ProfileHandler) APIGet(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	profileID := chi.URLParam(r, "id")
	profile, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}
	if profile.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
		return
	}
	writeSuccess(w, h.enrichProfileResponse(r.Context(), profile))
}

// APIUpdate handles PATCH /v1/profiles/{id} under API-key auth. Scoped
// under API-key auth. Public API callers can update both the human
// profile name and the hosted Connect branding fields.
func (h *ProfileHandler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	profileID := chi.URLParam(r, "id")

	profile, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}
	if profile.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
		return
	}

	var body struct {
		Name                 *string `json:"name"`
		BrandingLogoURL      *string `json:"branding_logo_url"`
		BrandingDisplayName  *string `json:"branding_display_name"`
		BrandingPrimaryColor *string `json:"branding_primary_color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Name != nil {
		trimmed := normalizeProfileName(*body.Name)
		if trimmed == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name cannot be empty")
			return
		}
		if h.profileNameTaken(r.Context(), workspaceID, trimmed, profileID) {
			writeError(w, http.StatusConflict, "PROFILE_NAME_TAKEN", "A profile with that name already exists in this workspace")
			return
		}
		*body.Name = trimmed
	}

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

	if body.Name != nil {
		if _, err := h.queries.UpdateProfile(r.Context(), db.UpdateProfileParams{
			ID:   profileID,
			Name: *body.Name,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update profile")
			return
		}
	}

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

	final, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reload profile")
		return
	}
	writeSuccess(w, h.enrichProfileResponse(r.Context(), final))
}

// APIDelete handles DELETE /v1/profiles/{id} under API-key auth.
func (h *ProfileHandler) APIDelete(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	profileID := chi.URLParam(r, "id")

	profile, err := h.queries.GetProfile(r.Context(), profileID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}
	if profile.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
		return
	}

	workspace, err := h.queries.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load workspace")
		return
	}
	user, err := h.queries.GetUser(r.Context(), workspace.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load workspace owner")
		return
	}
	defaultProfileID := ""
	if user.DefaultProfileID.Valid {
		defaultProfileID = user.DefaultProfileID.String
	}
	if profileErr := h.ensureProfileDeletable(r.Context(), profileID, defaultProfileID); profileErr != nil {
		writeError(w, profileErr.status, profileErr.code, profileErr.msg)
		return
	}
	if err := h.queries.DeleteProfile(r.Context(), profileID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete profile")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
