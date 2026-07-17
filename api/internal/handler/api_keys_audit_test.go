package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestAPIKeyCreateWritesSecretSafeAuditEvent(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "team"}
	h := NewAPIKeyHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(`{
		"name": "Editor automation",
		"environment": "test"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", rec.Code, rec.Body.String())
	}
	var response struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	write := requireSingleAuditWrite(t, store)
	assertAuditIdentity(t, write, audit.ActionAPIKeyCreated, "api_key", "ak_1")
	serialized := auditJSONPayload(write)
	if response.Data.Key == "" {
		t.Fatal("created key missing from one-time response")
	}
	if strings.Contains(serialized, response.Data.Key) || strings.Contains(serialized, "stored-key-hash") {
		t.Fatalf("audit event leaked key material: %s", serialized)
	}
	if !strings.Contains(serialized, "Editor automation") {
		t.Fatalf("audit event missing non-secret key name: %s", serialized)
	}
}

func auditJSONPayload(write []any) string {
	var payload strings.Builder
	for _, index := range []int{9, 10, 11} {
		if index < len(write) {
			if value, ok := write[index].([]byte); ok {
				payload.Write(value)
			}
		}
	}
	return payload.String()
}

func TestAPIKeyRevokeWritesAuditEvent(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "team"}
	h := NewAPIKeyHandler(db.New(store))
	router := chi.NewRouter()
	router.Delete("/{keyID}", h.Revoke)
	req := httptest.NewRequest(http.MethodDelete, "/ak_1", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", rec.Code, rec.Body.String())
	}
	write := requireSingleAuditWrite(t, store)
	assertAuditIdentity(t, write, audit.ActionAPIKeyRevoked, "api_key", "ak_1")
}

func TestAPIKeyAuditFailureDoesNotFailPrimaryMutation(t *testing.T) {
	store := &freePlanLimitsTestDB{
		planID:   "team",
		auditErr: fmt.Errorf("audit database unavailable"),
	}
	h := NewAPIKeyHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(`{
		"name": "Best effort",
		"environment": "test"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201 despite audit failure", rec.Code, rec.Body.String())
	}
	if store.auditWriteAttempts != 1 {
		t.Fatalf("audit attempts=%d, want 1", store.auditWriteAttempts)
	}
}

func requireSingleAuditWrite(t *testing.T, store *freePlanLimitsTestDB) []any {
	t.Helper()
	if len(store.auditWrites) != 1 {
		t.Fatalf("audit writes=%d, want 1", len(store.auditWrites))
	}
	return store.auditWrites[0]
}

func assertAuditIdentity(t *testing.T, write []any, action, resourceType, resourceID string) {
	t.Helper()
	if len(write) != 12 {
		t.Fatalf("audit argument count=%d, want 12", len(write))
	}
	if write[0] != "ws_1" || write[3] != action || write[4] != resourceType || write[6] != audit.CategoryConfig {
		t.Fatalf("unexpected audit identity: %#v", write)
	}
	gotResourceID, ok := write[5].(pgtype.Text)
	if !ok || !gotResourceID.Valid || gotResourceID.String != resourceID {
		t.Fatalf("audit resource_id=%#v, want %q", write[5], resourceID)
	}
}
