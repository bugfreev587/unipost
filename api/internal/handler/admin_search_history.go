package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

const (
	adminSearchHistoryDefaultLimit = 8
	adminSearchHistoryMaxLimit     = 8
	adminSearchHistoryKeepRows     = 25
	adminSearchHistoryMaxValueLen  = 512
)

var adminSearchHistoryAllowedFields = map[string]bool{
	"admin.logs.q":                   true,
	"admin.logs.workspace_id":        true,
	"admin.logs.owner_email":         true,
	"admin.errors.search":            true,
	"admin.api_metrics.workspace_id": true,
	"admin.email.search":             true,
	"admin.posts.search":             true,
	"admin.users.search":             true,
}

type adminSearchHistoryStore interface {
	ListAdminSearchHistory(context.Context, db.ListAdminSearchHistoryParams) ([]db.AdminSearchHistory, error)
	UpsertAdminSearchHistory(context.Context, db.UpsertAdminSearchHistoryParams) (db.AdminSearchHistory, error)
	DeleteAdminSearchHistory(context.Context, db.DeleteAdminSearchHistoryParams) (int64, error)
	PruneAdminSearchHistory(context.Context, db.PruneAdminSearchHistoryParams) (int64, error)
	CleanupExpiredAdminSearchHistory(context.Context) (int64, error)
}

type AdminSearchHistoryHandler struct {
	store             adminSearchHistoryStore
	superAdminChecker *auth.SuperAdminChecker
}

func NewAdminSearchHistoryHandler(store adminSearchHistoryStore, superAdminChecker *auth.SuperAdminChecker) *AdminSearchHistoryHandler {
	return &AdminSearchHistoryHandler{store: store, superAdminChecker: superAdminChecker}
}

type adminSearchHistoryResponse struct {
	ID         string `json:"id"`
	FieldKey   string `json:"field_key"`
	Value      string `json:"value"`
	UsageCount int32  `json:"usage_count"`
	LastUsedAt string `json:"last_used_at"`
}

func (h *AdminSearchHistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Admin search history is not configured")
		return
	}
	adminUserID := auth.GetUserID(r.Context())
	if adminUserID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	fieldKey := strings.TrimSpace(r.URL.Query().Get("field_key"))
	if !h.allowedField(w, r, fieldKey, adminUserID) {
		return
	}
	limit := adminSearchHistoryDefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid search history limit")
			return
		}
		limit = parsed
	}
	if limit > adminSearchHistoryMaxLimit {
		limit = adminSearchHistoryMaxLimit
	}
	rows, err := h.store.ListAdminSearchHistory(r.Context(), db.ListAdminSearchHistoryParams{
		AdminUserID: adminUserID,
		FieldKey:    fieldKey,
		LimitRows:   int32(limit),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load search history")
		return
	}
	out := make([]adminSearchHistoryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminSearchHistoryFromRow(row))
	}
	writeSuccess(w, out)
}

func (h *AdminSearchHistoryHandler) Save(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Admin search history is not configured")
		return
	}
	adminUserID := auth.GetUserID(r.Context())
	if adminUserID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	var body struct {
		FieldKey string `json:"field_key"`
		Value    string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid search history payload")
		return
	}
	fieldKey := strings.TrimSpace(body.FieldKey)
	if !h.allowedField(w, r, fieldKey, adminUserID) {
		return
	}
	value, normalized, err := normalizeAdminSearchHistoryValue(body.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	row, err := h.store.UpsertAdminSearchHistory(r.Context(), db.UpsertAdminSearchHistoryParams{
		AdminUserID:     adminUserID,
		FieldKey:        fieldKey,
		Value:           value,
		ValueNormalized: normalized,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save search history")
		return
	}
	_, _ = h.store.PruneAdminSearchHistory(r.Context(), db.PruneAdminSearchHistoryParams{
		AdminUserID: adminUserID,
		FieldKey:    fieldKey,
		KeepRows:    adminSearchHistoryKeepRows,
	})
	_, _ = h.store.CleanupExpiredAdminSearchHistory(r.Context())
	writeSuccess(w, adminSearchHistoryFromRow(row))
}

func (h *AdminSearchHistoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Admin search history is not configured")
		return
	}
	adminUserID := auth.GetUserID(r.Context())
	if adminUserID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Missing search history id")
		return
	}
	rows, err := h.store.DeleteAdminSearchHistory(r.Context(), db.DeleteAdminSearchHistoryParams{
		ID:          id,
		AdminUserID: adminUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete search history")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Search history entry not found")
		return
	}
	writeSuccess(w, map[string]bool{"deleted": true})
}

func (h *AdminSearchHistoryHandler) allowedField(w http.ResponseWriter, r *http.Request, fieldKey, adminUserID string) bool {
	if fieldKey == "" || !adminSearchHistoryAllowedFields[fieldKey] {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unsupported search history field")
		return false
	}
	if strings.HasPrefix(fieldKey, "admin.logs.") && (h.superAdminChecker == nil || !h.superAdminChecker.IsSuperAdmin(r.Context(), adminUserID)) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Admin logs search history is restricted to super admins")
		return false
	}
	return true
}

var errAdminSearchHistoryValueRequired = errors.New("Search history value must be at least 2 characters")

func normalizeAdminSearchHistoryValue(raw string) (string, string, error) {
	value := collapseWhitespace(strings.TrimSpace(raw))
	if utf8.RuneCountInString(value) < 2 {
		return "", "", errAdminSearchHistoryValueRequired
	}
	if utf8.RuneCountInString(value) > adminSearchHistoryMaxValueLen {
		return "", "", errors.New("Search history value is too long")
	}
	return value, strings.ToLower(value), nil
}

func collapseWhitespace(value string) string {
	var b strings.Builder
	previousSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !previousSpace {
				b.WriteByte(' ')
				previousSpace = true
			}
			continue
		}
		b.WriteRune(r)
		previousSpace = false
	}
	return b.String()
}

func adminSearchHistoryFromRow(row db.AdminSearchHistory) adminSearchHistoryResponse {
	out := adminSearchHistoryResponse{
		ID:         row.ID,
		FieldKey:   row.FieldKey,
		Value:      row.Value,
		UsageCount: row.UsageCount,
	}
	if row.LastUsedAt.Valid {
		out.LastUsedAt = row.LastUsedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return out
}
