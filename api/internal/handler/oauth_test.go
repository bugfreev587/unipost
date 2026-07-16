package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

var errOAuthPKCEExchangeObserved = errors.New("exchange observed")

func TestOAuthPKCEConnectStoresRandomVerifierAndUsesItForTwitterChallenge(t *testing.T) {
	store := &oauthPKCETestDB{}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewOAuthHandler(db.New(store), encryptor, nil)
	h.baseRedirectURL = "https://api.example"

	previous, err := platform.Get("twitter")
	if err != nil {
		previous = nil
	}
	platform.Register(platform.NewTwitterAdapter())
	t.Cleanup(func() {
		if previous != nil {
			platform.Register(previous)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/profiles/pr_1/oauth/connect/twitter", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withOAuthPKCEChiParams(req, map[string]string{
		"profileID": "pr_1",
		"platform":  "twitter",
	})
	rec := httptest.NewRecorder()

	h.Connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.pkceVerifier == "" {
		t.Fatal("CreateOAuthState did not receive a PKCE verifier")
	}
	verifierBytes, err := base64.RawURLEncoding.DecodeString(store.pkceVerifier)
	if err != nil {
		t.Fatalf("PKCE verifier is not base64url: %v", err)
	}
	if len(verifierBytes) < 64 {
		t.Fatalf("PKCE verifier entropy bytes = %d, want at least 64", len(verifierBytes))
	}
	if store.pkceVerifier == store.state {
		t.Fatal("PKCE verifier must be random and independent from state")
	}
	if !store.xAppMode.Valid || store.xAppMode.String != "unipost_managed_app" {
		t.Fatalf("stored X app mode = %+v, want unipost_managed_app", store.xAppMode)
	}

	var response struct {
		Data struct {
			AuthURL string `json:"auth_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	authURL, err := url.Parse(response.Data.AuthURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	sum := sha256.Sum256([]byte(store.pkceVerifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	query := authURL.Query()
	if got := query.Get("code_challenge"); got != wantChallenge {
		t.Fatalf("code_challenge = %q, want stored verifier challenge %q", got, wantChallenge)
	}
	if got := query.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", got)
	}
}

func TestOAuthConnectUsesProfileWorkspaceCredentialAndStoresWorkspaceMode(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedSecret, err := encryptor.Encrypt("workspace-client-secret")
	if err != nil {
		t.Fatal(err)
	}
	store := &oauthPKCETestDB{
		planID:                   "growth",
		platformCredentialID:     "workspace-client-id",
		platformCredentialSecret: encryptedSecret,
	}
	h := NewOAuthHandler(db.New(store), encryptor, nil)
	h.baseRedirectURL = "https://api.example"
	t.Setenv("TWITTER_CLIENT_ID", "global-client-id")
	t.Setenv("TWITTER_CLIENT_SECRET", "global-client-secret")

	previous, err := platform.Get("twitter")
	if err != nil {
		previous = nil
	}
	platform.Register(platform.NewTwitterAdapter())
	t.Cleanup(func() {
		if previous != nil {
			platform.Register(previous)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/profiles/pr_1/oauth/connect/twitter", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withOAuthPKCEChiParams(req, map[string]string{"profileID": "pr_1", "platform": "twitter"})
	rec := httptest.NewRecorder()
	h.Connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.platformCredentialWorkspaceID != "ws_1" {
		t.Fatalf("credential workspace lookup = %q, want ws_1 (not profile id)", store.platformCredentialWorkspaceID)
	}
	if !store.xAppMode.Valid || store.xAppMode.String != "workspace_x_app" {
		t.Fatalf("stored X app mode = %+v, want workspace_x_app", store.xAppMode)
	}
	var response struct {
		Data struct {
			AuthURL string `json:"auth_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	authURL, err := url.Parse(response.Data.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	if got := authURL.Query().Get("client_id"); got != "workspace-client-id" {
		t.Fatalf("client_id = %q, want workspace-client-id", got)
	}
}

func TestOAuthConfigForRollingNullTwitterStateUsesDeployedCompatibilityLookup(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedSecret, err := encryptor.Encrypt("workspace-client-secret")
	if err != nil {
		t.Fatal(err)
	}
	adapter := &oauthPKCESpyAdapter{TwitterAdapter: platform.NewTwitterAdapter()}

	t.Run("real workspace credential remains invisible to old profile-id lookup", func(t *testing.T) {
		store := &oauthPKCETestDB{
			planID:                              "growth",
			platformCredentialID:                "workspace-client-id",
			platformCredentialSecret:            encryptedSecret,
			platformCredentialRecordWorkspaceID: "ws_1",
		}
		h := NewOAuthHandler(db.New(store), encryptor, nil)
		config, mode, err := h.oauthConfigForCallback(
			httptest.NewRequest(http.MethodGet, "/", nil),
			"pr_1",
			"twitter",
			adapter,
			pgtype.Text{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if config.ClientID != "client-id" {
			t.Fatalf("client id = %q, want UniPost default", config.ClientID)
		}
		if !mode.Valid || mode.String != string(xinbox.AppModeUniPostManaged) {
			t.Fatalf("resolved mode = %+v, want UniPost managed", mode)
		}
		if store.platformCredentialWorkspaceID != "pr_1" {
			t.Fatalf("credential lookup workspace = %q, want legacy profile id pr_1", store.platformCredentialWorkspaceID)
		}
	})

	t.Run("no eligible workspace credential", func(t *testing.T) {
		store := &oauthPKCETestDB{}
		h := NewOAuthHandler(db.New(store), encryptor, nil)
		config, mode, err := h.oauthConfigForCallback(
			httptest.NewRequest(http.MethodGet, "/", nil),
			"pr_1",
			"twitter",
			adapter,
			pgtype.Text{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if config.ClientID != "client-id" {
			t.Fatalf("client id = %q, want UniPost default", config.ClientID)
		}
		if !mode.Valid || mode.String != string(xinbox.AppModeUniPostManaged) {
			t.Fatalf("resolved mode = %+v, want UniPost managed", mode)
		}
	})

	t.Run("legacy lookup error still uses global app", func(t *testing.T) {
		store := &oauthPKCETestDB{platformCredentialErr: errors.New("compatibility database read failed")}
		h := NewOAuthHandler(db.New(store), encryptor, nil)
		config, mode, err := h.oauthConfigForCallback(
			httptest.NewRequest(http.MethodGet, "/", nil),
			"pr_1",
			"twitter",
			adapter,
			pgtype.Text{},
		)
		if err != nil {
			t.Fatalf("legacy NULL fallback returned error: %v", err)
		}
		if config.ClientID != "client-id" {
			t.Fatalf("client id = %q, want UniPost default", config.ClientID)
		}
		if !mode.Valid || mode.String != string(xinbox.AppModeUniPostManaged) {
			t.Fatalf("resolved mode = %+v, want UniPost managed", mode)
		}
	})

	t.Run("non-null garbage is rejected", func(t *testing.T) {
		store := &oauthPKCETestDB{}
		h := NewOAuthHandler(db.New(store), encryptor, nil)
		_, _, err := h.oauthConfigForCallback(
			httptest.NewRequest(http.MethodGet, "/", nil),
			"pr_1",
			"twitter",
			adapter,
			pgtype.Text{String: "garbage", Valid: true},
		)
		if err == nil {
			t.Fatal("garbage stored mode error = nil, want rejection")
		}
	})
}

func TestOAuthCallbackPersistsResolvedModeForRollingNullTwitterState(t *testing.T) {
	store := &oauthPKCETestDB{
		state:        "csrf-state",
		pkceVerifier: "stored-random-verifier",
		planID:       "growth",
	}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	h := NewOAuthHandler(db.New(store), encryptor, nil)
	h.baseRedirectURL = "https://api.example"

	spy := &oauthPKCESpyAdapter{
		TwitterAdapter: platform.NewTwitterAdapter(),
		result: &platform.ConnectResult{
			AccessToken:       "access-token",
			RefreshToken:      "refresh-token",
			ExternalAccountID: "twitter-account-1",
			AccountName:       "UniPost",
		},
	}
	previous, err := platform.Get("twitter")
	if err != nil {
		previous = nil
	}
	platform.Register(spy)
	t.Cleanup(func() {
		if previous != nil {
			platform.Register(previous)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/oauth/callback/twitter?code=authorization-code&state=csrf-state", nil)
	req = withOAuthPKCEChiParams(req, map[string]string{"platform": "twitter"})
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if !store.savedXAppMode.Valid || store.savedXAppMode.String != string(xinbox.AppModeUniPostManaged) {
		t.Fatalf("saved X app mode = %+v, want resolved UniPost managed mode", store.savedXAppMode)
	}
}

func TestOAuthStateCallbackConsumesOnceBeforeTwitterExchange(t *testing.T) {
	store := &oauthPKCETestDB{
		state:        "csrf-state",
		pkceVerifier: "stored-random-verifier",
	}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewOAuthHandler(db.New(store), encryptor, nil)
	h.baseRedirectURL = "https://api.example"

	spy := &oauthPKCESpyAdapter{TwitterAdapter: platform.NewTwitterAdapter()}
	previous, err := platform.Get("twitter")
	if err != nil {
		previous = nil
	}
	platform.Register(spy)
	t.Cleanup(func() {
		if previous != nil {
			platform.Register(previous)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/oauth/callback/twitter?code=authorization-code&state=csrf-state", nil)
	req = withOAuthPKCEChiParams(req, map[string]string{"platform": "twitter"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if spy.exchangeConfig.PKCEVerifier != store.pkceVerifier {
		t.Fatalf("exchange PKCE verifier = %q, want stored verifier", spy.exchangeConfig.PKCEVerifier)
	}
	if store.consumeCalls != 1 {
		t.Fatalf("ConsumeOAuthState calls = %d, want 1", store.consumeCalls)
	}
	if spy.exchangeCalls != 1 {
		t.Fatalf("ExchangeCode calls = %d, want 1", spy.exchangeCalls)
	}

	replayReq := httptest.NewRequest(http.MethodGet, "/v1/oauth/callback/twitter?code=replayed-code&state=csrf-state", nil)
	replayReq = withOAuthPKCEChiParams(replayReq, map[string]string{"platform": "twitter"})
	replayRec := httptest.NewRecorder()

	h.Callback(replayRec, replayReq)

	if store.consumeCalls != 2 {
		t.Fatalf("ConsumeOAuthState calls after replay = %d, want 2", store.consumeCalls)
	}
	if spy.exchangeCalls != 1 {
		t.Fatalf("ExchangeCode calls after replay = %d, want still 1", spy.exchangeCalls)
	}
}

func TestOAuthStateCallbackRejectsConsumeErrorBeforeExchange(t *testing.T) {
	store := &oauthPKCETestDB{
		state:        "csrf-state",
		pkceVerifier: "stored-random-verifier",
		consumeErr:   errors.New("database unavailable"),
	}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	h := NewOAuthHandler(db.New(store), encryptor, nil)

	spy := &oauthPKCESpyAdapter{TwitterAdapter: platform.NewTwitterAdapter()}
	previous, err := platform.Get("twitter")
	if err != nil {
		previous = nil
	}
	platform.Register(spy)
	t.Cleanup(func() {
		if previous != nil {
			platform.Register(previous)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/oauth/callback/twitter?code=authorization-code&state=csrf-state", nil)
	req = withOAuthPKCEChiParams(req, map[string]string{"platform": "twitter"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if store.consumeCalls != 1 {
		t.Fatalf("ConsumeOAuthState calls = %d, want 1", store.consumeCalls)
	}
	if spy.exchangeCalls != 0 {
		t.Fatalf("ExchangeCode calls = %d, want 0", spy.exchangeCalls)
	}
}

type oauthPKCESpyAdapter struct {
	*platform.TwitterAdapter
	exchangeConfig platform.OAuthConfig
	exchangeCalls  int
	result         *platform.ConnectResult
}

func (a *oauthPKCESpyAdapter) DefaultOAuthConfig(baseRedirectURL string) platform.OAuthConfig {
	config := a.TwitterAdapter.DefaultOAuthConfig(baseRedirectURL)
	config.ClientID = "client-id"
	config.ClientSecret = "client-secret"
	return config
}

func (a *oauthPKCESpyAdapter) ExchangeCode(_ context.Context, config platform.OAuthConfig, _ string) (*platform.ConnectResult, error) {
	a.exchangeConfig = config
	a.exchangeCalls++
	if a.result != nil {
		return a.result, nil
	}
	return nil, errOAuthPKCEExchangeObserved
}

type oauthPKCETestDB struct {
	state                               string
	pkceVerifier                        string
	xAppMode                            pgtype.Text
	planID                              string
	platformCredentialID                string
	platformCredentialSecret            string
	platformCredentialRecordWorkspaceID string
	platformCredentialWorkspaceID       string
	platformCredentialErr               error
	consumeCalls                        int
	consumed                            bool
	consumeErr                          error
	savedXAppMode                       pgtype.Text
}

func (f *oauthPKCETestDB) Exec(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *oauthPKCETestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (f *oauthPKCETestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetConnectSessionByOAuthState"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: ConsumeOAuthState"):
		f.consumeCalls++
		if f.consumeErr != nil {
			return scanRow{err: f.consumeErr}
		}
		if f.consumed {
			return scanRow{err: pgx.ErrNoRows}
		}
		f.consumed = true
		return f.oauthStateRow()
	case strings.Contains(query, "-- name: GetProfile"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"pr_1", "Profile", now, now, pgtype.Text{}, pgtype.Text{}, pgtype.Text{}, "ws_1", false, pgtype.Text{},
		}}
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		if f.planID == "" {
			return scanRow{err: pgx.ErrNoRows}
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"sub_1", f.planID, pgtype.Text{}, pgtype.Text{}, "active",
			now, now, pgtype.Bool{}, now, now, false, "ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlatformCredential"):
		f.platformCredentialWorkspaceID, _ = args[0].(string)
		if f.platformCredentialErr != nil {
			return scanRow{err: f.platformCredentialErr}
		}
		recordWorkspaceID := f.platformCredentialRecordWorkspaceID
		if recordWorkspaceID == "" {
			recordWorkspaceID = "ws_1"
		}
		if f.platformCredentialID != "" && f.platformCredentialWorkspaceID == recordWorkspaceID {
			now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
			return scanRow{values: []any{
				"pc_1", "twitter", f.platformCredentialID, f.platformCredentialSecret,
				now, "ws_1", pgtype.Text{}, pgtype.Text{},
			}}
		}
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: FindSocialAccountByExternalID"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: CreateSocialAccount"):
		f.savedXAppMode, _ = args[10].(pgtype.Text)
		return f.socialAccountRow()
	case strings.Contains(query, "-- name: CreateOAuthState"):
		if len(args) < 6 {
			return scanRow{err: fmt.Errorf("CreateOAuthState args = %d, want PKCE verifier argument", len(args))}
		}
		f.state, _ = args[0].(string)
		verifier, _ := args[4].(pgtype.Text)
		f.pkceVerifier = verifier.String
		f.xAppMode, _ = args[5].(pgtype.Text)
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			f.state,
			"pr_1",
			"twitter",
			pgtype.Text{},
			pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
			now,
			verifier,
			f.xAppMode,
		}}
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func (f *oauthPKCETestDB) oauthStateRow() scanRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return scanRow{values: []any{
		f.state,
		"pr_1",
		"twitter",
		pgtype.Text{},
		pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
		now,
		pgtype.Text{String: f.pkceVerifier, Valid: f.pkceVerifier != ""},
		f.xAppMode,
	}}
}

func (f *oauthPKCETestDB) socialAccountRow() scanRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return scanRow{values: []any{
		"sa_1",
		"pr_1",
		"twitter",
		"encrypted-access",
		pgtype.Text{String: "encrypted-refresh", Valid: true},
		pgtype.Timestamptz{},
		"twitter-account-1",
		pgtype.Text{String: "UniPost", Valid: true},
		pgtype.Text{},
		now,
		pgtype.Timestamptz{},
		[]byte("{}"),
		[]string{},
		"active",
		"byo",
		pgtype.Text{},
		pgtype.Text{},
		pgtype.Text{},
		now,
		f.savedXAppMode,
	}}
}

func withOAuthPKCEChiParams(req *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
