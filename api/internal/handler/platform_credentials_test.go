package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestPlatformCredentials_BasicRejectsNonSlotPlatform(t *testing.T) {
	store := &platformCredentialTestDB{
		planID:             "basic",
		customPlatformSlot: "tiktok",
	}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "linkedin",
		"client_id": "linkedin-client",
		"client_secret": "linkedin-secret"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createCalls != 0 {
		t.Fatalf("CreatePlatformCredential calls = %d, want 0", store.createCalls)
	}
}

func TestPlatformCredentials_BasicClaimsEmptyCustomPlatformSlot(t *testing.T) {
	store := &platformCredentialTestDB{planID: "basic"}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "linkedin",
		"client_id": "linkedin-client",
		"client_secret": "linkedin-secret"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.customPlatformSlot != "linkedin" {
		t.Fatalf("customPlatformSlot = %q, want linkedin", store.customPlatformSlot)
	}
	if store.createCalls != 1 {
		t.Fatalf("CreatePlatformCredential calls = %d, want 1", store.createCalls)
	}
}

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

func TestPlatformCredentials_DeleteDoesNotReportSuccessOnDatabaseFailure(t *testing.T) {
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
	store := &platformCredentialTestDB{planID: "team", auditErr: fmt.Errorf("audit unavailable")}
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

type platformCredentialTestDB struct {
	planID              string
	customPlatformSlot  string
	createCalls         int
	deleteErr           error
	auditErr            error
	auditWriteAttempts  int
	auditWrites         [][]any
	lastEncryptedSecret string
}

func (f *platformCredentialTestDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: DeletePlatformCredential") {
		return pgconn.CommandTag{}, f.deleteErr
	}
	if strings.Contains(query, "-- name: WriteAuditLog") {
		f.auditWriteAttempts++
		f.auditWrites = append(f.auditWrites, append([]any(nil), args...))
		return pgconn.CommandTag{}, f.auditErr
	}
	return pgconn.CommandTag{}, nil
}

func (f *platformCredentialTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListPlatformCredentialsByWorkspace"):
		return emptyScheduledIdempotencyRows{}, nil
	default:
		return nil, pgx.ErrNoRows
	}
}

func (f *platformCredentialTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		planID := f.planID
		if planID == "" {
			planID = "free"
		}
		return scanRow{values: []any{
			"sub_1",
			planID,
			pgtype.Text{},
			pgtype.Text{},
			"active",
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			pgtype.Bool{},
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			false,
			"ws_1",
		}}
	case strings.Contains(query, "-- name: GetWorkspace"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		customPlatformSlot := pgtype.Text{}
		if f.customPlatformSlot != "" {
			customPlatformSlot = pgtype.Text{String: f.customPlatformSlot, Valid: true}
		}
		return scanRow{values: []any{
			"ws_1",
			"user_1",
			"Workspace",
			pgtype.Int4{},
			now,
			now,
			[]string{"publishing"},
			customPlatformSlot,
		}}
	case strings.Contains(query, "-- name: ClaimWorkspaceCustomPlatformSlot"):
		nextSlot, _ := args[1].(pgtype.Text)
		if f.customPlatformSlot != "" && nextSlot.Valid && f.customPlatformSlot != nextSlot.String {
			return scanRow{err: pgx.ErrNoRows}
		}
		if nextSlot.Valid {
			f.customPlatformSlot = nextSlot.String
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"ws_1",
			"user_1",
			"Workspace",
			pgtype.Int4{},
			now,
			now,
			[]string{"publishing"},
			pgtype.Text{String: f.customPlatformSlot, Valid: f.customPlatformSlot != ""},
		}}
	case strings.Contains(query, "-- name: CreatePlatformCredential"):
		f.createCalls++
		platform, _ := args[1].(string)
		clientID, _ := args[2].(string)
		clientSecret, _ := args[3].(string)
		f.lastEncryptedSecret = clientSecret
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"pc_1",
			platform,
			clientID,
			clientSecret,
			now,
			"ws_1",
		}}
	default:
		return scanRow{err: pgx.ErrNoRows}
	}
}
