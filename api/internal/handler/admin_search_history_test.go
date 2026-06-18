package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type fakeAdminSearchHistoryStore struct {
	listCalls    int
	upsertCalls  int
	deleteCalls  int
	pruneCalls   int
	cleanupCalls int

	listArg   db.ListAdminSearchHistoryParams
	upsertArg db.UpsertAdminSearchHistoryParams
	deleteArg db.DeleteAdminSearchHistoryParams
	pruneArg  db.PruneAdminSearchHistoryParams

	listRows   []db.AdminSearchHistory
	upsertRow  db.AdminSearchHistory
	deleteRows int64
}

func (f *fakeAdminSearchHistoryStore) ListAdminSearchHistory(ctx context.Context, arg db.ListAdminSearchHistoryParams) ([]db.AdminSearchHistory, error) {
	f.listCalls++
	f.listArg = arg
	return f.listRows, nil
}

func (f *fakeAdminSearchHistoryStore) UpsertAdminSearchHistory(ctx context.Context, arg db.UpsertAdminSearchHistoryParams) (db.AdminSearchHistory, error) {
	f.upsertCalls++
	f.upsertArg = arg
	return f.upsertRow, nil
}

func (f *fakeAdminSearchHistoryStore) DeleteAdminSearchHistory(ctx context.Context, arg db.DeleteAdminSearchHistoryParams) (int64, error) {
	f.deleteCalls++
	f.deleteArg = arg
	return f.deleteRows, nil
}

func (f *fakeAdminSearchHistoryStore) PruneAdminSearchHistory(ctx context.Context, arg db.PruneAdminSearchHistoryParams) (int64, error) {
	f.pruneCalls++
	f.pruneArg = arg
	return 0, nil
}

func (f *fakeAdminSearchHistoryStore) CleanupExpiredAdminSearchHistory(ctx context.Context) (int64, error) {
	f.cleanupCalls++
	return 0, nil
}

func searchHistoryRow(id, adminUserID, fieldKey, value string, usageCount int32) db.AdminSearchHistory {
	return db.AdminSearchHistory{
		ID:              id,
		AdminUserID:     adminUserID,
		FieldKey:        fieldKey,
		Value:           value,
		ValueNormalized: value,
		UsageCount:      usageCount,
		LastUsedAt:      pgtype.Timestamptz{Time: time.Date(2026, 6, 18, 17, 12, 42, 0, time.UTC), Valid: true},
	}
}

func adminSearchHistoryRequest(method, target, userID string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userID))
	return req
}

func TestAdminSearchHistorySaveNormalizesAndScopesToCurrentAdmin(t *testing.T) {
	store := &fakeAdminSearchHistoryStore{
		upsertRow: searchHistoryRow("hist_1", "user_admin", "admin.users.search", "Alice EXAMPLE@example.com", 2),
	}
	h := NewAdminSearchHistoryHandler(store, nil)
	req := adminSearchHistoryRequest(
		http.MethodPost,
		"/v1/admin/search-history",
		"user_admin",
		[]byte(`{"field_key":"admin.users.search","value":"  Alice   EXAMPLE@example.com  "}`),
	)
	rr := httptest.NewRecorder()

	h.Save(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if store.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", store.upsertCalls)
	}
	if store.upsertArg.AdminUserID != "user_admin" {
		t.Fatalf("admin user id = %q, want user_admin", store.upsertArg.AdminUserID)
	}
	if store.upsertArg.FieldKey != "admin.users.search" {
		t.Fatalf("field key = %q, want admin.users.search", store.upsertArg.FieldKey)
	}
	if store.upsertArg.Value != "Alice EXAMPLE@example.com" {
		t.Fatalf("value = %q, want collapsed display value", store.upsertArg.Value)
	}
	if store.upsertArg.ValueNormalized != "alice example@example.com" {
		t.Fatalf("normalized = %q, want case-insensitive collapsed value", store.upsertArg.ValueNormalized)
	}
	if store.pruneCalls != 1 || store.cleanupCalls != 1 {
		t.Fatalf("prune/cleanup calls = %d/%d, want 1/1", store.pruneCalls, store.cleanupCalls)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := envelope["success"]; ok {
		t.Fatalf("response should use existing envelope without success boolean: %s", rr.Body.String())
	}
	if _, ok := envelope["data"]; !ok {
		t.Fatalf("response missing data envelope: %s", rr.Body.String())
	}
}

func TestAdminSearchHistoryRejectsUnsupportedField(t *testing.T) {
	store := &fakeAdminSearchHistoryStore{}
	h := NewAdminSearchHistoryHandler(store, nil)
	req := adminSearchHistoryRequest(
		http.MethodPost,
		"/v1/admin/search-history",
		"user_admin",
		[]byte(`{"field_key":"admin.not_real.search","value":"needle"}`),
	)
	rr := httptest.NewRecorder()

	h.Save(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
	}
	if store.upsertCalls != 0 {
		t.Fatalf("upsert calls = %d, want 0 for invalid field", store.upsertCalls)
	}
}

func TestAdminSearchHistoryRejectsShortValue(t *testing.T) {
	store := &fakeAdminSearchHistoryStore{}
	h := NewAdminSearchHistoryHandler(store, nil)
	req := adminSearchHistoryRequest(
		http.MethodPost,
		"/v1/admin/search-history",
		"user_admin",
		[]byte(`{"field_key":"admin.users.search","value":"x"}`),
	)
	rr := httptest.NewRecorder()

	h.Save(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
	}
	if store.upsertCalls != 0 {
		t.Fatalf("upsert calls = %d, want 0 for short value", store.upsertCalls)
	}
}

func TestAdminSearchHistoryListCapsLimitAndScopesToCurrentAdmin(t *testing.T) {
	store := &fakeAdminSearchHistoryStore{
		listRows: []db.AdminSearchHistory{
			searchHistoryRow("hist_1", "user_admin", "admin.posts.search", "quota", 4),
		},
	}
	h := NewAdminSearchHistoryHandler(store, nil)
	req := adminSearchHistoryRequest(
		http.MethodGet,
		"/v1/admin/search-history?field_key=admin.posts.search&limit=99",
		"user_admin",
		nil,
	)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if store.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", store.listCalls)
	}
	if store.listArg.AdminUserID != "user_admin" || store.listArg.FieldKey != "admin.posts.search" {
		t.Fatalf("list scope = %+v, want current admin and field", store.listArg)
	}
	if store.listArg.LimitRows != 8 {
		t.Fatalf("limit rows = %d, want capped 8", store.listArg.LimitRows)
	}
}

func TestAdminSearchHistoryLogsFieldRequiresSuperAdmin(t *testing.T) {
	t.Setenv("SUPER_ADMINS", "user_super")
	store := &fakeAdminSearchHistoryStore{}
	h := NewAdminSearchHistoryHandler(store, auth.NewSuperAdminChecker(nil))
	req := adminSearchHistoryRequest(
		http.MethodGet,
		"/v1/admin/search-history?field_key=admin.logs.q",
		"user_admin",
		nil,
	)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rr.Code, rr.Body.String())
	}
	if store.listCalls != 0 {
		t.Fatalf("list calls = %d, want 0 for non-super-admin logs field", store.listCalls)
	}
}

func TestAdminSearchHistoryDeleteForeignRowReturnsNotFound(t *testing.T) {
	store := &fakeAdminSearchHistoryStore{deleteRows: 0}
	h := NewAdminSearchHistoryHandler(store, nil)
	router := chi.NewRouter()
	router.Delete("/v1/admin/search-history/{id}", h.Delete)
	req := adminSearchHistoryRequest(
		http.MethodDelete,
		"/v1/admin/search-history/hist_foreign",
		"user_admin",
		nil,
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rr.Code, rr.Body.String())
	}
	if store.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", store.deleteCalls)
	}
	if store.deleteArg.AdminUserID != "user_admin" || store.deleteArg.ID != "hist_foreign" {
		t.Fatalf("delete scope = %+v, want current admin and row id", store.deleteArg)
	}
}
