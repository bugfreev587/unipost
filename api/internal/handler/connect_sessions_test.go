package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// TestValidateReturnURL — accept https/http with a host, reject everything else.
func TestValidateReturnURL(t *testing.T) {
	cases := []struct {
		in   string
		good bool
	}{
		{"https://app.example.com/done", true},
		{"http://localhost:3000/cb", true},
		{"javascript:alert(1)", false},
		{"data:text/html,<script>", false},
		{"file:///etc/passwd", false},
		{"https://", false},  // no host
		{"not a url", false}, // no scheme
		{"ftp://example.com", false},
	}
	for _, c := range cases {
		err := validateReturnURL(c.in)
		if (err == nil) != c.good {
			t.Errorf("validateReturnURL(%q): want good=%v, got err=%v", c.in, c.good, err)
		}
	}
}

// TestRandomBase64URL — produces unique, URL-safe, padding-free strings
// of approximately the right length for the given byte count.
func TestRandomBase64URL(t *testing.T) {
	a, err := randomBase64URL(32)
	if err != nil {
		t.Fatalf("randomBase64URL err: %v", err)
	}
	b, _ := randomBase64URL(32)
	if a == b {
		t.Error("two calls returned the same string — entropy broken")
	}
	if strings.Contains(a, "=") || strings.Contains(a, "+") || strings.Contains(a, "/") {
		t.Errorf("expected URL-safe encoding without padding, got %q", a)
	}
	// 32 bytes → ceil(32*4/3) = 43 base64url chars (no padding).
	if len(a) != 43 {
		t.Errorf("32-byte input should yield 43 chars, got %d (%q)", len(a), a)
	}
	// 64 bytes → 86 chars.
	v, _ := randomBase64URL(64)
	if len(v) != 86 {
		t.Errorf("64-byte input should yield 86 chars, got %d", len(v))
	}
}

// TestBuildHostedURL — shape of the URL handed back to customers.
func TestBuildHostedURL(t *testing.T) {
	h := &ConnectSessionHandler{dashboardURL: "https://app.unipost.dev"}
	got := h.buildHostedURL("twitter", "sess_abc", "state-xyz")
	want := "https://app.unipost.dev/connect/twitter?session=sess_abc&state=state-xyz"
	if got != want {
		t.Errorf("buildHostedURL: got %q, want %q", got, want)
	}

	// Trailing slash on the dashboard URL must not produce a double slash.
	h2 := &ConnectSessionHandler{dashboardURL: "https://app.unipost.dev/"}
	got = h2.buildHostedURL("bluesky", "s", "state")
	if strings.Contains(got, "dev//connect") {
		t.Errorf("trailing slash produced double slash: %q", got)
	}
}

// TestNewConnectSessionHandler_NilQuotaOK — the constructor must
// accept a nil *quota.Checker without panic. This is the test path
// (and the legacy boot path before the plan-gate wiring) and the
// existing connect tests rely on this fall-open behavior. The Create
// handler explicitly checks h.quota != nil before invoking the gate,
// so a nil checker simply means "no plan restriction enforced here".
func TestNewConnectSessionHandler_NilQuotaOK(t *testing.T) {
	h := NewConnectSessionHandler(nil, "https://app.unipost.dev", nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.quota != nil {
		t.Errorf("expected nil quota, got %#v", h.quota)
	}
}

// TestConnectablePlatforms locks the currently supported platform allowlist.
func TestConnectablePlatforms(t *testing.T) {
	for _, p := range []string{"twitter", "linkedin", "bluesky", "youtube", "tiktok", "instagram", "threads", "facebook", "pinterest"} {
		if !connectablePlatforms[p] {
			t.Errorf("%s should be connectable", p)
		}
	}
	for _, p := range []string{"reddit"} {
		if connectablePlatforms[p] {
			t.Errorf("%s should NOT be connectable yet", p)
		}
	}
}

func TestConnectSessionPlatformUsesOAuthApp(t *testing.T) {
	for _, p := range []string{"twitter", "linkedin", "youtube", "tiktok", "instagram", "threads", "facebook", "pinterest"} {
		if !connectSessionPlatformUsesOAuthApp(p) {
			t.Errorf("%s should use OAuth app credentials", p)
		}
	}
	if connectSessionPlatformUsesOAuthApp("bluesky") {
		t.Error("bluesky should not use OAuth app credentials")
	}
}

func TestCreateConnectSession_OAuthQuickstartPlatforms(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")
	t.Setenv("FEATURE_CONNECT_SESSIONS_TIKTOK_INSTAGRAM", "true")
	t.Setenv("FEATURE_CONNECT_SESSIONS_THREADS", "true")
	t.Setenv("FEATURE_CONNECT_SESSIONS_FACEBOOK_PINTEREST", "true")

	for _, platform := range []string{"tiktok", "instagram", "threads", "facebook", "pinterest"} {
		t.Run(platform, func(t *testing.T) {
			fdb := &connectSessionTestDB{platform: platform, allowQuickstart: true}
			h := NewConnectSessionHandler(db.New(fdb), "https://app.unipost.dev", nil)
			body := fmt.Sprintf(`{
				"platform": %q,
				"profile_id": "pr_1",
				"external_user_id": "user_123",
				"allow_quickstart_creds": true
			}`, platform)
			req := httptest.NewRequest(http.MethodPost, "/v1/connect/sessions", strings.NewReader(body))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			rec := httptest.NewRecorder()

			h.Create(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var env struct {
				Data connectSessionResponse `json:"data"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if env.Data.Platform != platform || env.Data.ExternalUserID != "user_123" {
				t.Fatalf("data = %+v", env.Data)
			}
			if !strings.Contains(env.Data.URL, "/connect/"+platform) {
				t.Fatalf("hosted url = %q", env.Data.URL)
			}
			if fdb.platformCredentialLookups != 0 {
				t.Fatalf("quickstart create should not require white-label credential lookup, got %d", fdb.platformCredentialLookups)
			}
		})
	}
}

func TestCreateConnectSession_OAuthMissingWhiteLabelCreds(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")
	t.Setenv("FEATURE_CONNECT_SESSIONS_TIKTOK_INSTAGRAM", "true")
	t.Setenv("FEATURE_CONNECT_SESSIONS_FACEBOOK_PINTEREST", "true")

	for _, platform := range []string{"tiktok", "facebook", "pinterest"} {
		t.Run(platform, func(t *testing.T) {
			fdb := &connectSessionTestDB{platform: platform, credentialErr: pgx.ErrNoRows}
			h := NewConnectSessionHandler(db.New(fdb), "https://app.unipost.dev", nil)
			body := fmt.Sprintf(`{
				"platform": %q,
				"profile_id": "pr_1",
				"external_user_id": "user_123",
				"allow_quickstart_creds": false
			}`, platform)
			req := httptest.NewRequest(http.MethodPost, "/v1/connect/sessions", strings.NewReader(body))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			rec := httptest.NewRecorder()

			h.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "workspace is missing "+platform+" platform credentials") {
				t.Fatalf("unexpected body: %s", rec.Body.String())
			}
		})
	}
}

func TestConnectAuthorize_ResolvesOAuthConnectors(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")
	t.Setenv("FEATURE_CONNECT_SESSIONS_TIKTOK_INSTAGRAM", "true")
	t.Setenv("FEATURE_CONNECT_SESSIONS_THREADS", "true")
	t.Setenv("FEATURE_CONNECT_SESSIONS_FACEBOOK_PINTEREST", "true")
	t.Setenv("FEATURE_TIKTOK_ANALYTICS_SCOPES", "false")

	cases := []struct {
		platform string
		registry *connect.Registry
		wantURL  string
		wantPart string
	}{
		{
			platform: "tiktok",
			registry: connect.NewRegistry(
				connect.NewTikTokConnector("client-key", "secretXYZ", "https://api.example.com"),
			),
			wantURL:  "https://www.tiktok.com/v2/auth/authorize/",
			wantPart: "client_key=client-key",
		},
		{
			platform: "threads",
			registry: connect.NewRegistry(
				connect.NewThreadsConnector("threads-client", "secretXYZ", "https://api.example.com"),
			),
			wantURL:  "https://threads.net/oauth/authorize",
			wantPart: "client_id=threads-client",
		},
		{
			platform: "facebook",
			registry: connect.NewRegistry(
				connect.NewFacebookConnector("facebook-client", "secretXYZ", "https://api.example.com"),
			),
			wantURL:  "https://www.facebook.com/v22.0/dialog/oauth",
			wantPart: "client_id=facebook-client",
		},
		{
			platform: "pinterest",
			registry: connect.NewRegistry(
				connect.NewPinterestConnector("pinterest-client", "secretXYZ", "https://api.example.com"),
			),
			wantURL:  "https://www.pinterest.com/oauth/",
			wantPart: "consumer_id=pinterest-client",
		},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			fdb := &connectSessionTestDB{platform: tc.platform, allowQuickstart: true, credentialErr: pgx.ErrNoRows}
			h := &ConnectCallbackHandler{
				queries:  db.New(fdb),
				registry: tc.registry,
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/public/connect/sessions/cs_1/authorize?state=state_1", nil)
			req = withChiParam(req, "id", "cs_1")
			rec := httptest.NewRecorder()

			h.Authorize(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			location := rec.Header().Get("Location")
			if !strings.Contains(location, tc.wantURL) {
				t.Fatalf("location = %q", location)
			}
			if !strings.Contains(location, tc.wantPart) {
				t.Fatalf("location missing connector credentials: %q", location)
			}
		})
	}
}

func TestGetConnectSession_CompletedReturnsManagedAccountID(t *testing.T) {
	fdb := &connectSessionTestDB{
		platform:        "instagram",
		status:          "completed",
		completedAcctID: "sa_instagram_123",
		allowQuickstart: true,
	}
	h := NewConnectSessionHandler(db.New(fdb), "https://app.unipost.dev", nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/sessions/cs_1", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withChiParam(req, "id", "cs_1")
	rec := httptest.NewRecorder()

	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data connectSessionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Data.Status != "completed" {
		t.Fatalf("status = %q", env.Data.Status)
	}
	if env.Data.ManagedAccountID != "sa_instagram_123" {
		t.Fatalf("managed_account_id = %q", env.Data.ManagedAccountID)
	}
	if env.Data.CompletedSocialAccountID != "sa_instagram_123" {
		t.Fatalf("completed_social_account_id = %q", env.Data.CompletedSocialAccountID)
	}
}

type connectSessionTestDB struct {
	platform                  string
	status                    string
	completedAcctID           string
	allowQuickstart           bool
	credentialErr             error
	platformCredentialLookups int
}

func (f *connectSessionTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *connectSessionTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (f *connectSessionTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetProfile"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"pr_1", "TailTales", now, now, pgtype.Text{}, pgtype.Text{}, pgtype.Text{}, false, "ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlatformCredential"):
		f.platformCredentialLookups++
		if f.credentialErr != nil {
			return scanRow{err: f.credentialErr}
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{"pc_1", f.platform, "client-id", "encrypted-secret", now, "ws_1"}}
	case strings.Contains(query, "-- name: CreateConnectSession"):
		platform, _ := args[1].(string)
		externalUserID, _ := args[2].(string)
		externalEmail, _ := args[3].(pgtype.Text)
		returnURL, _ := args[4].(pgtype.Text)
		oauthState, _ := args[5].(string)
		pkceVerifier, _ := args[6].(pgtype.Text)
		expiresAt, _ := args[7].(pgtype.Timestamptz)
		allowQuickstart, _ := args[8].(bool)
		return f.connectSessionRow(platform, "pending", "", externalUserID, externalEmail, returnURL, oauthState, pkceVerifier, expiresAt, allowQuickstart)
	case strings.Contains(query, "-- name: GetConnectSessionByIDOnly"):
		return f.connectSessionRow(f.platform, f.statusOrDefault(), f.completedAcctID, "user_123", pgtype.Text{}, pgtype.Text{}, "state_1", pgtype.Text{}, futureTimestamptz(), f.allowQuickstart)
	case strings.Contains(query, "-- name: GetConnectSessionByOAuthState"):
		return f.connectSessionRow(f.platform, f.statusOrDefault(), f.completedAcctID, "user_123", pgtype.Text{}, pgtype.Text{}, "state_1", pgtype.Text{}, futureTimestamptz(), f.allowQuickstart)
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func (f *connectSessionTestDB) statusOrDefault() string {
	if f.status != "" {
		return f.status
	}
	return "pending"
}

func (f *connectSessionTestDB) connectSessionRow(platform, status, completedAcctID, externalUserID string, externalEmail, returnURL pgtype.Text, oauthState string, pkceVerifier pgtype.Text, expiresAt pgtype.Timestamptz, allowQuickstart bool) scanRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	completedID := pgtype.Text{}
	completedAt := pgtype.Timestamptz{}
	if completedAcctID != "" {
		completedID = pgtype.Text{String: completedAcctID, Valid: true}
		completedAt = now
	}
	return scanRow{values: []any{
		"cs_1",
		"pr_1",
		platform,
		externalUserID,
		externalEmail,
		returnURL,
		status,
		completedID,
		oauthState,
		pkceVerifier,
		expiresAt,
		now,
		completedAt,
		allowQuickstart,
	}}
}

type scanRow struct {
	values []any
	err    error
}

func (r scanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count %d != values count %d", len(dest), len(r.values))
	}
	for i, value := range r.values {
		if value == nil {
			continue
		}
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return fmt.Errorf("scan destination %d is not a pointer", i)
		}
		source := reflect.ValueOf(value)
		if source.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(source)
			continue
		}
		if source.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(source.Convert(target.Elem().Type()))
			continue
		}
		return fmt.Errorf("cannot scan %T into %T", value, dest[i])
	}
	return nil
}

func withChiParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func futureTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true}
}
