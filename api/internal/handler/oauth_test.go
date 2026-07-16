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
	return nil, errOAuthPKCEExchangeObserved
}

type oauthPKCETestDB struct {
	state        string
	pkceVerifier string
	consumeCalls int
	consumed     bool
	consumeErr   error
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
	case strings.Contains(query, "-- name: GetPlatformCredential"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: CreateOAuthState"):
		if len(args) < 5 {
			return scanRow{err: fmt.Errorf("CreateOAuthState args = %d, want PKCE verifier argument", len(args))}
		}
		f.state, _ = args[0].(string)
		verifier, _ := args[4].(pgtype.Text)
		f.pkceVerifier = verifier.String
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			f.state,
			"pr_1",
			"twitter",
			pgtype.Text{},
			pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
			now,
			verifier,
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
	}}
}

func withOAuthPKCEChiParams(req *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
