package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestPlatformCredentials_CreateWritesSecretSafeAuditEvent(t *testing.T) {
	store := &platformCredentialTestDB{planID: "team"}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "linkedin",
		"client_id": "linkedin-client",
		"client_secret": "top-secret-value"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", rec.Code, rec.Body.String())
	}
	write := requireSinglePlatformCredentialAuditWrite(t, store)
	assertAuditIdentity(t, write, audit.ActionPlatformCredentialCreated, "platform_credential", "linkedin")
	payload := auditJSONPayload(write)
	if strings.Contains(payload, "top-secret-value") || strings.Contains(payload, store.lastEncryptedSecret) {
		t.Fatalf("credential audit leaked secret material: %s", payload)
	}
	if !strings.Contains(payload, "linkedin-client") {
		t.Fatalf("credential audit missing non-secret client ID: %s", payload)
	}
}

func TestPlatformCredentials_DeleteWritesAuditEvent(t *testing.T) {
	store := &platformCredentialTestDB{planID: "team"}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	router := chi.NewRouter()
	router.Delete("/{platform}", h.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/linkedin", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", rec.Code, rec.Body.String())
	}
	write := requireSinglePlatformCredentialAuditWrite(t, store)
	assertAuditIdentity(t, write, audit.ActionPlatformCredentialDeleted, "platform_credential", "linkedin")
}

func TestPlatformCredentials_DeleteDoesNotAuditDatabaseFailure(t *testing.T) {
	store := &platformCredentialTestDB{planID: "team", deleteErr: errors.New("database unavailable")}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	router := chi.NewRouter()
	router.Delete("/{platform}", h.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/linkedin", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want 500", rec.Code, rec.Body.String())
	}
	if len(store.auditWrites) != 0 {
		t.Fatalf("audit writes=%d, want 0 for failed delete", len(store.auditWrites))
	}
}

func TestPlatformCredentials_AuditFailureDoesNotFailCreate(t *testing.T) {
	store := &platformCredentialTestDB{planID: "team", auditErr: errors.New("audit unavailable")}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "linkedin",
		"client_id": "linkedin-client",
		"client_secret": "top-secret-value"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated || store.auditWriteAttempts != 1 {
		t.Fatalf("status=%d audit attempts=%d body=%s, want 201/1", rec.Code, store.auditWriteAttempts, rec.Body.String())
	}
}

func requireSinglePlatformCredentialAuditWrite(t *testing.T, store *platformCredentialTestDB) []any {
	t.Helper()
	if len(store.auditWrites) != 1 {
		t.Fatalf("audit writes=%d, want 1", len(store.auditWrites))
	}
	return store.auditWrites[0]
}
