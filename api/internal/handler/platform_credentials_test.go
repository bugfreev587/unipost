package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
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

func TestPlatformCredentialsTwitterEncryptsAppSecretsAndReturnsOnlyCompletenessFlags(t *testing.T) {
	store := &platformCredentialTestDB{planID: "growth"}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "twitter",
		"client_id": "twitter-client",
		"client_secret": "twitter-client-secret",
		"app_bearer_token": "twitter-bearer-secret",
		"consumer_secret": "twitter-consumer-secret"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.appBearerToken.String == "twitter-bearer-secret" || store.consumerSecret.String == "twitter-consumer-secret" {
		t.Fatal("X app-level secrets were passed to persistence in plaintext")
	}
	if !store.appBearerTokenSupplied || !store.consumerSecretSupplied {
		t.Fatalf("supplied flags = bearer:%v consumer:%v, want both true", store.appBearerTokenSupplied, store.consumerSecretSupplied)
	}
	gotBearer, err := encryptor.Decrypt(store.appBearerToken.String)
	if err != nil || gotBearer != "twitter-bearer-secret" {
		t.Fatalf("decrypt app bearer token = %q, %v", gotBearer, err)
	}
	gotConsumer, err := encryptor.Decrypt(store.consumerSecret.String)
	if err != nil || gotConsumer != "twitter-consumer-secret" {
		t.Fatalf("decrypt consumer secret = %q, %v", gotConsumer, err)
	}
	if store.webhookRouteKey.String == xinbox.WebhookRouteKey("twitter-consumer-secret", "twitter-client") {
		t.Fatal("workspace webhook route key must be random, not derived from the rotatable consumer secret")
	}
	if len(store.webhookRouteKey.String) < 32 {
		t.Fatalf("workspace webhook route key = %q, want cryptographically strong opaque token", store.webhookRouteKey.String)
	}
	if strings.Contains(rec.Body.String(), "twitter-bearer-secret") || strings.Contains(rec.Body.String(), "twitter-consumer-secret") {
		t.Fatalf("response leaked an X secret: %s", rec.Body.String())
	}
	var response struct {
		Data struct {
			AppBearerTokenConfigured  bool `json:"app_bearer_token_configured"`
			ConsumerSecretConfigured  bool `json:"consumer_secret_configured"`
			XInboxCredentialsComplete bool `json:"x_inbox_credentials_complete"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.AppBearerTokenConfigured || !response.Data.ConsumerSecretConfigured || !response.Data.XInboxCredentialsComplete {
		t.Fatalf("credential flags = %+v", response.Data)
	}
}

func TestPlatformCredentialsTwitterConsumerSecretRotationPreservesSameAppWebhookRoute(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	existingConsumer, _ := encryptor.Encrypt("old-consumer-secret")
	existingBearer, _ := encryptor.Encrypt("existing-bearer")
	existingRoute := pgtype.Text{String: "stable-workspace-route-key", Valid: true}
	store := &platformCredentialTestDB{
		planID:                  "growth",
		existingPlatform:        "twitter",
		existingAppBearerToken:  pgtype.Text{String: existingBearer, Valid: true},
		existingConsumerSecret:  pgtype.Text{String: existingConsumer, Valid: true},
		existingWebhookRouteKey: existingRoute,
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "twitter",
		"client_id": "existing-client",
		"client_secret": "rotated-client-secret",
		"consumer_secret": "rotated-consumer-secret"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.webhookRouteKey != existingRoute {
		t.Fatalf("same-app consumer-secret rotation changed route from %+v to %+v", existingRoute, store.webhookRouteKey)
	}
	gotConsumer, err := encryptor.Decrypt(store.consumerSecret.String)
	if err != nil || gotConsumer != "rotated-consumer-secret" {
		t.Fatalf("rotated consumer secret = %q, %v", gotConsumer, err)
	}
}

func TestPlatformCredentialsTwitterClientIDChangeGeneratesNewWebhookRoute(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := &platformCredentialTestDB{
		planID:                  "growth",
		existingPlatform:        "twitter",
		existingWebhookRouteKey: pgtype.Text{String: "old-workspace-route", Valid: true},
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "twitter",
		"client_id": "replacement-client",
		"client_secret": "replacement-client-secret",
		"app_bearer_token": "replacement-bearer",
		"consumer_secret": "replacement-consumer"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !store.webhookRouteKey.Valid || store.webhookRouteKey.String == "old-workspace-route" {
		t.Fatalf("replacement app route = %+v, want a new generation", store.webhookRouteKey)
	}
}

func TestPlatformCredentialsTwitterPreservesOptionalSecretsWhenOmitted(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	existingBearer, _ := encryptor.Encrypt("existing-bearer")
	existingConsumer, _ := encryptor.Encrypt("existing-consumer")
	store := &platformCredentialTestDB{
		planID:                 "growth",
		existingPlatform:       "twitter",
		existingAppBearerToken: pgtype.Text{String: existingBearer, Valid: true},
		existingConsumerSecret: pgtype.Text{String: existingConsumer, Valid: true},
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "twitter",
		"client_id": "existing-client",
		"client_secret": "updated-client-secret"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.appBearerToken != store.existingAppBearerToken {
		t.Fatal("omitted app_bearer_token did not preserve the stored ciphertext")
	}
	if store.consumerSecret != store.existingConsumerSecret {
		t.Fatal("omitted consumer_secret did not preserve the stored ciphertext")
	}
	if store.getCalls != 0 {
		t.Fatalf("GetPlatformCredential calls = %d, want 0 for atomic update", store.getCalls)
	}
	if store.appBearerTokenSupplied || store.consumerSecretSupplied {
		t.Fatalf("supplied flags = bearer:%v consumer:%v, want both false", store.appBearerTokenSupplied, store.consumerSecretSupplied)
	}
}

func TestPlatformCredentialsTwitterDoesNotCarryOptionalSecretsAcrossAppIdentity(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	existingBearer, _ := encryptor.Encrypt("existing-bearer")
	existingConsumer, _ := encryptor.Encrypt("existing-consumer")
	store := &platformCredentialTestDB{
		planID:                 "growth",
		existingPlatform:       "twitter",
		existingAppBearerToken: pgtype.Text{String: existingBearer, Valid: true},
		existingConsumerSecret: pgtype.Text{String: existingConsumer, Valid: true},
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "twitter",
		"client_id": "replacement-client",
		"client_secret": "replacement-client-secret"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.appBearerToken.Valid || store.consumerSecret.Valid {
		t.Fatalf(
			"replacement app inherited old optional secrets: bearer=%v consumer=%v",
			store.appBearerToken,
			store.consumerSecret,
		)
	}
}

func TestPlatformCredentialsRejectsBlankXSecretAndXFieldsOnOtherPlatforms(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "blank twitter bearer",
			body: `{"platform":"twitter","client_id":"id","client_secret":"secret","app_bearer_token":"   "}`,
		},
		{
			name: "X secret on LinkedIn",
			body: `{"platform":"linkedin","client_id":"id","client_secret":"secret","consumer_secret":"not-allowed"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &platformCredentialTestDB{planID: "growth"}
			encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
			if err != nil {
				t.Fatal(err)
			}
			h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
			req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(tt.body))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			rec := httptest.NewRecorder()

			h.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if store.createCalls != 0 {
				t.Fatalf("create calls = %d, want 0", store.createCalls)
			}
		})
	}
}

func TestPlatformCredentialsDeleteReportsTransactionalCleanupFailure(t *testing.T) {
	store := &platformCredentialTestDB{deleteErr: errors.New("cleanup trigger failed")}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodDelete, "/v1/platform-credentials/twitter", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("platform", "twitter")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPlatformCredentialsUpdateReportsTransactionalCleanupFailure(t *testing.T) {
	store := &platformCredentialTestDB{
		planID:    "growth",
		createErr: errors.New("credential replacement cleanup trigger failed"),
	}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	h := NewPlatformCredentialHandler(db.New(store), encryptor, quota.NewChecker(db.New(store)))
	req := httptest.NewRequest(http.MethodPost, "/v1/platform-credentials", strings.NewReader(`{
		"platform": "twitter",
		"client_id": "replacement-client",
		"client_secret": "replacement-secret",
		"app_bearer_token": "replacement-bearer",
		"consumer_secret": "replacement-consumer"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type platformCredentialTestDB struct {
	planID                  string
	customPlatformSlot      string
	createCalls             int
	getCalls                int
	existingPlatform        string
	existingAppBearerToken  pgtype.Text
	existingConsumerSecret  pgtype.Text
	existingWebhookRouteKey pgtype.Text
	appBearerToken          pgtype.Text
	consumerSecret          pgtype.Text
	webhookRouteKey         pgtype.Text
	appBearerTokenSupplied  bool
	consumerSecretSupplied  bool
	createErr               error
	deleteErr               error
	auditErr                error
	auditWriteAttempts      int
	auditWrites             [][]any
	lastEncryptedSecret     string
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
	case strings.Contains(query, "-- name: GetPlatformCredential"):
		f.getCalls++
		if f.existingPlatform == "" {
			return scanRow{err: pgx.ErrNoRows}
		}
		return platformCredentialScanRow{
			platform:        f.existingPlatform,
			clientID:        "existing-client",
			clientSecret:    "existing-encrypted-client-secret",
			appBearerToken:  f.existingAppBearerToken,
			consumerSecret:  f.existingConsumerSecret,
			webhookRouteKey: f.existingWebhookRouteKey,
		}
	case strings.Contains(query, "-- name: CreatePlatformCredential"):
		f.createCalls++
		if f.createErr != nil {
			return scanRow{err: f.createErr}
		}
		platform, _ := args[1].(string)
		clientID, _ := args[2].(string)
		clientSecret, _ := args[3].(string)
		f.lastEncryptedSecret = clientSecret
		if len(args) > 4 {
			f.appBearerToken, _ = args[4].(pgtype.Text)
		}
		if len(args) > 5 {
			f.consumerSecret, _ = args[5].(pgtype.Text)
		}
		if len(args) > 6 {
			routeKey, _ := args[6].(string)
			f.webhookRouteKey = pgtype.Text{String: routeKey, Valid: routeKey != ""}
		}
		if len(args) > 7 {
			f.appBearerTokenSupplied, _ = args[7].(bool)
		}
		if len(args) > 8 {
			f.consumerSecretSupplied, _ = args[8].(bool)
		}
		if !f.appBearerTokenSupplied {
			if clientID == "existing-client" {
				f.appBearerToken = f.existingAppBearerToken
			} else {
				f.appBearerToken = pgtype.Text{}
			}
		}
		if !f.consumerSecretSupplied {
			if clientID == "existing-client" {
				f.consumerSecret = f.existingConsumerSecret
			} else {
				f.consumerSecret = pgtype.Text{}
			}
		}
		if clientID == "existing-client" && f.existingWebhookRouteKey.Valid {
			f.webhookRouteKey = f.existingWebhookRouteKey
		} else if !f.appBearerToken.Valid || !f.consumerSecret.Valid {
			f.webhookRouteKey = pgtype.Text{}
		}
		return platformCredentialScanRow{
			platform:        platform,
			clientID:        clientID,
			clientSecret:    clientSecret,
			appBearerToken:  f.appBearerToken,
			consumerSecret:  f.consumerSecret,
			webhookRouteKey: f.webhookRouteKey,
		}
	default:
		return scanRow{err: pgx.ErrNoRows}
	}
}

type platformCredentialScanRow struct {
	platform        string
	clientID        string
	clientSecret    string
	appBearerToken  pgtype.Text
	consumerSecret  pgtype.Text
	webhookRouteKey pgtype.Text
}

func (r platformCredentialScanRow) Scan(dest ...any) error {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	values := []any{"pc_1", r.platform, r.clientID, r.clientSecret, now, "ws_1"}
	if len(dest) == 9 {
		values = []any{"pc_1", r.platform, r.clientID, r.clientSecret, now, "ws_1", r.appBearerToken, r.consumerSecret, r.webhookRouteKey}
	}
	return scanRow{values: values}.Scan(dest...)
}
