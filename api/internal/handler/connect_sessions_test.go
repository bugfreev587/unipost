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
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
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

func TestConnectablePlatformListMatchesAllowlist(t *testing.T) {
	parts := strings.Split(connectablePlatformList, ", ")
	if len(parts) != len(connectablePlatforms) {
		t.Fatalf("list length %d != allowlist length %d (%q)", len(parts), len(connectablePlatforms), connectablePlatformList)
	}
	seen := map[string]bool{}
	for _, p := range parts {
		if !connectablePlatforms[p] {
			t.Fatalf("%q is in error message list but not allowlist", p)
		}
		if seen[p] {
			t.Fatalf("%q appears more than once in error message list", p)
		}
		seen[p] = true
	}
	for p := range connectablePlatforms {
		if !seen[p] {
			t.Fatalf("%q is allowlisted but missing from error message list", p)
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

func TestCreateConnectSession_OAuthQuickstartPlatformsEnabledInProduction(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "production")

	for _, platform := range []string{"twitter", "linkedin", "youtube", "tiktok", "instagram", "threads", "facebook", "pinterest"} {
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
			if platform == "twitter" && (!fdb.xAppMode.Valid || fdb.xAppMode.String != "unipost_managed_app") {
				t.Fatalf("stored X app mode = %+v, want unipost_managed_app", fdb.xAppMode)
			}
		})
	}
}

func TestCreateConnectSessionStoresWorkspaceXAppModeWhenCredentialExists(t *testing.T) {
	fdb := &connectSessionTestDB{
		platform:        "twitter",
		planID:          "growth",
		allowQuickstart: true,
	}
	h := NewConnectSessionHandler(db.New(fdb), "https://app.unipost.dev", nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/connect/sessions", strings.NewReader(`{
		"platform": "twitter",
		"profile_id": "pr_1",
		"external_user_id": "user_123",
		"allow_quickstart_creds": true
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !fdb.xAppMode.Valid || fdb.xAppMode.String != "workspace_x_app" {
		t.Fatalf("stored X app mode = %+v, want workspace_x_app", fdb.xAppMode)
	}
}

func TestResolveConnectorUsesStoredTwitterAppMode(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedSecret, err := encryptor.Encrypt("workspace-secret")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("managed mode stays registry even when workspace credentials exist", func(t *testing.T) {
		fdb := &connectSessionTestDB{
			platform:              "twitter",
			planID:                "growth",
			credentialSecretValue: encryptedSecret,
		}
		h := NewConnectCallbackHandler(
			db.New(fdb), encryptor, events.NoopBus{},
			connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
			"https://api.example.com", nil,
		)
		connector, ok, err := h.resolveConnector(
			context.Background(), "ws_1", "twitter", true,
			pgtype.Text{String: "unipost_managed_app", Valid: true},
		)
		if err != nil || !ok || connector == nil {
			t.Fatalf("connector=%v ok=%v err=%v", connector, ok, err)
		}
		if fdb.platformCredentialLookups != 0 {
			t.Fatalf("workspace credential lookups = %d, want 0", fdb.platformCredentialLookups)
		}
	})
	t.Run("workspace mode fails when credential was removed", func(t *testing.T) {
		fdb := &connectSessionTestDB{platform: "twitter", credentialErr: pgx.ErrNoRows}
		h := NewConnectCallbackHandler(
			db.New(fdb), encryptor, events.NoopBus{},
			connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
			"https://api.example.com", nil,
		)
		connector, ok, err := h.resolveConnector(
			context.Background(), "ws_1", "twitter", true,
			pgtype.Text{String: "workspace_x_app", Valid: true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if ok || connector != nil {
			t.Fatalf("connector=%v ok=%v, want unavailable", connector, ok)
		}
	})
}

func TestResolveTwitterConnectorForRollingNullModeUsesLegacyPolicy(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedSecret, err := encryptor.Encrypt("workspace-secret")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("workspace credentials remain eligible without quickstart", func(t *testing.T) {
		fdb := &connectSessionTestDB{
			platform:              "twitter",
			planID:                "growth",
			credentialSecretValue: encryptedSecret,
		}
		h := NewConnectCallbackHandler(
			db.New(fdb), encryptor, events.NoopBus{},
			connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
			"https://api.example.com", nil,
		)
		resolved, err := h.resolveConnectorForStoredMode(
			context.Background(), "ws_1", "twitter", false, pgtype.Text{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !resolved.ok || resolved.connector == nil {
			t.Fatal("workspace connector was not resolved")
		}
		if !resolved.xAppMode.Valid || resolved.xAppMode.String != string(xinbox.AppModeWorkspace) {
			t.Fatalf("resolved mode = %+v, want workspace", resolved.xAppMode)
		}
	})

	t.Run("quickstart falls back to UniPost managed", func(t *testing.T) {
		fdb := &connectSessionTestDB{platform: "twitter"}
		h := NewConnectCallbackHandler(
			db.New(fdb), encryptor, events.NoopBus{},
			connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
			"https://api.example.com", nil,
		)
		resolved, err := h.resolveConnectorForStoredMode(
			context.Background(), "ws_1", "twitter", true, pgtype.Text{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !resolved.ok || resolved.connector == nil {
			t.Fatal("UniPost managed connector was not resolved")
		}
		if !resolved.xAppMode.Valid || resolved.xAppMode.String != string(xinbox.AppModeUniPostManaged) {
			t.Fatalf("resolved mode = %+v, want UniPost managed", resolved.xAppMode)
		}
	})

	t.Run("API no-quickstart does not gain managed credentials", func(t *testing.T) {
		fdb := &connectSessionTestDB{platform: "twitter"}
		h := NewConnectCallbackHandler(
			db.New(fdb), encryptor, events.NoopBus{},
			connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
			"https://api.example.com", nil,
		)
		resolved, err := h.resolveConnectorForStoredMode(
			context.Background(), "ws_1", "twitter", false, pgtype.Text{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.ok || resolved.connector != nil || resolved.xAppMode.Valid {
			t.Fatalf("resolved = %+v, want unavailable with no explicit mode", resolved)
		}
	})

	t.Run("non-null garbage is rejected", func(t *testing.T) {
		fdb := &connectSessionTestDB{platform: "twitter"}
		h := NewConnectCallbackHandler(
			db.New(fdb), encryptor, events.NoopBus{},
			connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
			"https://api.example.com", nil,
		)
		_, err := h.resolveConnectorForStoredMode(
			context.Background(), "ws_1", "twitter", true,
			pgtype.Text{String: "garbage", Valid: true},
		)
		if err == nil {
			t.Fatal("garbage stored mode error = nil, want rejection")
		}
		if fdb.platformCredentialLookups != 0 {
			t.Fatalf("credential lookups = %d, want 0 for rejected garbage", fdb.platformCredentialLookups)
		}
	})
}

func TestConnectCallbackPersistsResolvedModeForRollingNullTwitterSession(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	fdb := &connectSessionTestDB{
		platform:               "twitter",
		allowQuickstart:        true,
		activeAccountLookupErr: pgx.ErrNoRows,
	}
	h := NewConnectCallbackHandler(
		db.New(fdb), encryptor, events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "twitter"}),
		"https://api.example.com", nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/callback/twitter?code=auth-code&state=state_1", nil)
	req = withChiParam(req, "platform", "twitter")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !fdb.xAppMode.Valid || fdb.xAppMode.String != string(xinbox.AppModeUniPostManaged) {
		t.Fatalf("saved X app mode = %+v, want resolved UniPost managed mode", fdb.xAppMode)
	}
}

func TestCreateConnectSession_FreePlanRejectsNewExternalUserAfterManagedUserCap(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")

	fdb := &connectSessionTestDB{
		platform:                         "tiktok",
		allowQuickstart:                  true,
		managedUserCount:                 3,
		existingExternalUserAccountCount: 0,
	}
	queries := db.New(fdb)
	h := NewConnectSessionHandler(queries, "https://app.unipost.dev", quota.NewChecker(queries))
	req := httptest.NewRequest(http.MethodPost, "/v1/connect/sessions", strings.NewReader(`{
		"platform": "tiktok",
		"profile_id": "pr_1",
		"external_user_id": "user_new",
		"allow_quickstart_creds": true
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fdb.createConnectSessionCalls != 0 {
		t.Fatalf("CreateConnectSession calls = %d, want 0", fdb.createConnectSessionCalls)
	}
}

func TestCreateConnectSession_FreePlanAllowsExistingExternalUserAtManagedUserCap(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")

	fdb := &connectSessionTestDB{
		platform:                         "tiktok",
		allowQuickstart:                  true,
		managedUserCount:                 3,
		existingExternalUserAccountCount: 1,
	}
	queries := db.New(fdb)
	h := NewConnectSessionHandler(queries, "https://app.unipost.dev", quota.NewChecker(queries))
	req := httptest.NewRequest(http.MethodPost, "/v1/connect/sessions", strings.NewReader(`{
		"platform": "tiktok",
		"profile_id": "pr_1",
		"external_user_id": "user_123",
		"allow_quickstart_creds": true
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fdb.createConnectSessionCalls != 1 {
		t.Fatalf("CreateConnectSession calls = %d, want 1", fdb.createConnectSessionCalls)
	}
}

func TestCreateConnectSession_OAuthMissingWhiteLabelCreds(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")

	for _, platform := range []string{"tiktok", "facebook", "pinterest"} {
		t.Run(platform, func(t *testing.T) {
			fdb := &connectSessionTestDB{platform: platform, planID: "basic", customPlatformSlot: platform, credentialErr: pgx.ErrNoRows}
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
			wantPart: "client_id=pinterest-client",
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

func TestResolveConnector_BasicIgnoresCredentialsOutsideCustomPlatformSlot(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	encryptedSecret, err := encryptor.Encrypt("linkedin-secret")
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:              "linkedin",
		planID:                "basic",
		customPlatformSlot:    "tiktok",
		credentialSecretValue: encryptedSecret,
	}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "linkedin"}),
		"https://api.example.com",
		nil,
	)

	connector, ok, err := h.resolveConnector(context.Background(), "ws_1", "linkedin", true)

	if err != nil {
		t.Fatalf("resolveConnector: %v", err)
	}
	if !ok {
		t.Fatal("expected fallback connector")
	}
	if connector == nil {
		t.Fatal("expected fallback connector")
	}
	if fdb.platformCredentialLookups != 0 {
		t.Fatalf("credential lookups = %d, want 0", fdb.platformCredentialLookups)
	}
}

func TestResolveConnector_NoSubscriptionDoesNotUseWorkspaceCredentials(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:        "linkedin",
		subscriptionErr: pgx.ErrNoRows,
	}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "linkedin"}),
		"https://api.example.com",
		nil,
	)

	connector, ok, err := h.resolveConnector(context.Background(), "ws_1", "linkedin", true)

	if err != nil {
		t.Fatalf("resolveConnector: %v", err)
	}
	if !ok || connector == nil {
		t.Fatal("expected fallback connector")
	}
	if fdb.platformCredentialLookups != 0 {
		t.Fatalf("credential lookups = %d, want 0", fdb.platformCredentialLookups)
	}
}

func TestOAuthCallbackRedirectsConnectSessionState(t *testing.T) {
	fdb := &connectSessionTestDB{platform: "tiktok", allowQuickstart: true}
	h := &OAuthHandler{queries: db.New(fdb)}
	req := httptest.NewRequest(http.MethodGet, "/v1/oauth/callback/tiktok?code=auth-code-1&state=state_1", nil)
	req = withChiParam(req, "platform", "tiktok")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if location != "/v1/connect/callback/tiktok?code=auth-code-1&state=state_1" {
		t.Fatalf("location = %q", location)
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

func TestPublicGet_BasicBrandingRequiresMatchingCustomPlatformSlot(t *testing.T) {
	fdb := &connectSessionTestDB{
		platform:           "linkedin",
		planID:             "basic",
		customPlatformSlot: "tiktok",
		profileBranding:    true,
	}
	h := NewConnectSessionHandler(db.New(fdb), "https://app.unipost.dev", nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/public/connect/sessions/cs_1?state=state_1", nil)
	req = withChiParam(req, "id", "cs_1")
	rec := httptest.NewRecorder()

	h.PublicGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data publicConnectSessionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Data.Branding != nil {
		t.Fatalf("basic branding should be hidden for non-slot platform, got %+v", env.Data.Branding)
	}
}

func TestPublicGet_BasicBrandingShowsOnMatchingCustomPlatformSlot(t *testing.T) {
	fdb := &connectSessionTestDB{
		platform:           "tiktok",
		planID:             "basic",
		customPlatformSlot: "tiktok",
		profileBranding:    true,
	}
	h := NewConnectSessionHandler(db.New(fdb), "https://app.unipost.dev", nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/public/connect/sessions/cs_1?state=state_1", nil)
	req = withChiParam(req, "id", "cs_1")
	rec := httptest.NewRecorder()

	h.PublicGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data publicConnectSessionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Data.Branding == nil {
		t.Fatal("basic branding should show on the selected custom platform")
	}
	if env.Data.Branding.DisplayName != "TailTales Custom" {
		t.Fatalf("display name = %q", env.Data.Branding.DisplayName)
	}
}

func TestConnectCallbackReusesDisconnectedManagedAccountForExternalUser(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:               "tiktok",
		allowQuickstart:        true,
		credentialErr:          pgx.ErrNoRows,
		activeAccountLookupErr: pgx.ErrNoRows,
		createManagedErr:       fmt.Errorf("duplicate key value violates unique constraint \"social_accounts_managed_unique_idx\""),
		reusedManagedID:        "sa_disconnected_1",
	}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "tiktok"}),
		"https://api.example.com",
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/callback/tiktok?code=auth-code&state=state_1", nil)
	req = withChiParam(req, "platform", "tiktok")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fdb.createManagedCalls != 0 {
		t.Fatalf("CreateManagedSocialAccount calls = %d, want 0", fdb.createManagedCalls)
	}
	if fdb.upsertManagedCalls != 1 {
		t.Fatalf("UpsertManagedSocialAccount calls = %d, want 1", fdb.upsertManagedCalls)
	}
	if fdb.completedAcctID != "sa_disconnected_1" {
		t.Fatalf("completed social account = %q", fdb.completedAcctID)
	}
}

func TestConnectCallback_InstagramSubscribesBeforeCompleting(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:               "instagram",
		allowQuickstart:        true,
		activeAccountLookupErr: pgx.ErrNoRows,
	}
	subscriber := &fakeInstagramWebhookSubscriber{}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "instagram"}),
		"https://api.example.com",
		nil,
	)
	h.instagramWebhookSubscriber = subscriber
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/callback/instagram?code=auth-code&state=state_1", nil)
	req = withChiParam(req, "platform", "instagram")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if subscriber.calls != 1 {
		t.Fatalf("subscriber calls = %d, want 1", subscriber.calls)
	}
	if subscriber.accountID != "platform-account-new" {
		t.Fatalf("subscriber account id = %q", subscriber.accountID)
	}
	if subscriber.accessToken != "access-token" {
		t.Fatalf("subscriber access token = %q", subscriber.accessToken)
	}
	if fdb.completedAcctID != "sa_upserted_1" {
		t.Fatalf("completed social account = %q", fdb.completedAcctID)
	}
}

func TestConnectCallback_InstagramSubscriptionFailureRequiresReconnect(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:               "instagram",
		allowQuickstart:        true,
		activeAccountLookupErr: pgx.ErrNoRows,
	}
	subscriber := &fakeInstagramWebhookSubscriber{err: fmt.Errorf("meta subscription denied")}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "instagram"}),
		"https://api.example.com",
		nil,
	)
	h.instagramWebhookSubscriber = subscriber
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/callback/instagram?code=auth-code&state=state_1", nil)
	req = withChiParam(req, "platform", "instagram")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fdb.completedAcctID != "" {
		t.Fatalf("completed social account = %q, want empty", fdb.completedAcctID)
	}
	if fdb.reconnectRequiredCalls != 1 {
		t.Fatalf("reconnect required calls = %d, want 1", fdb.reconnectRequiredCalls)
	}
	if !strings.Contains(rec.Body.String(), "webhook_subscription_failed") {
		t.Fatalf("body = %q, want webhook subscription failure", rec.Body.String())
	}
}

func TestConnectCallback_FreePlanRejectsNewManagedUserAfterCap(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:                         "tiktok",
		allowQuickstart:                  true,
		activeAccountLookupErr:           pgx.ErrNoRows,
		managedUserCount:                 3,
		existingExternalUserAccountCount: 0,
		activeManagedAccountCount:        1,
	}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "tiktok"}),
		"https://api.example.com",
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/callback/tiktok?code=auth-code&state=state_1", nil)
	req = withChiParam(req, "platform", "tiktok")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fdb.upsertManagedCalls != 0 {
		t.Fatalf("UpsertManagedSocialAccount calls = %d, want 0", fdb.upsertManagedCalls)
	}
	if fdb.completedAcctID != "" {
		t.Fatalf("completed social account = %q, want empty", fdb.completedAcctID)
	}
}

func TestConnectCallback_FreePlanRejectsNewManagedAccountAfterCap(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	fdb := &connectSessionTestDB{
		platform:                                 "tiktok",
		allowQuickstart:                          true,
		activeAccountLookupErr:                   pgx.ErrNoRows,
		managedUserCount:                         1,
		existingExternalUserAccountCount:         1,
		activeManagedAccountCount:                2,
		existingExternalUserPlatformAccountCount: 0,
	}
	h := NewConnectCallbackHandler(
		db.New(fdb),
		encryptor,
		events.NoopBus{},
		connect.NewRegistry(fakeOAuthConnector{platform: "tiktok"}),
		"https://api.example.com",
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/connect/callback/tiktok?code=auth-code&state=state_1", nil)
	req = withChiParam(req, "platform", "tiktok")
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fdb.upsertManagedCalls != 0 {
		t.Fatalf("UpsertManagedSocialAccount calls = %d, want 0", fdb.upsertManagedCalls)
	}
	if fdb.completedAcctID != "" {
		t.Fatalf("completed social account = %q, want empty", fdb.completedAcctID)
	}
}

type connectSessionTestDB struct {
	platform                  string
	status                    string
	completedAcctID           string
	allowQuickstart           bool
	planID                    string
	subscriptionErr           error
	customPlatformSlot        string
	profileBranding           bool
	credentialSecretValue     string
	credentialErr             error
	platformCredentialLookups int
	activeAccountLookupErr    error
	createManagedErr          error
	reusedManagedID           string
	createManagedCalls        int
	upsertManagedCalls        int
	createConnectSessionCalls int
	reconnectRequiredCalls    int
	xAppMode                  pgtype.Text

	managedUserCount                         int32
	existingExternalUserAccountCount         int32
	activeManagedAccountCount                int32
	existingExternalUserPlatformAccountCount int32
}

func (f *connectSessionTestDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: MarkSocialAccountReconnectRequired") {
		f.reconnectRequiredCalls++
	}
	return pgconn.CommandTag{}, nil
}

func (f *connectSessionTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (f *connectSessionTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetProfile"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		logoURL := pgtype.Text{}
		displayName := pgtype.Text{}
		primaryColor := pgtype.Text{}
		if f.profileBranding {
			logoURL = pgtype.Text{String: "https://cdn.example.com/logo.png", Valid: true}
			displayName = pgtype.Text{String: "TailTales Custom", Valid: true}
			primaryColor = pgtype.Text{String: "#10b981", Valid: true}
		}
		return scanRow{values: []any{
			"pr_1", "TailTales", now, now, logoURL, displayName, primaryColor, "ws_1", false, pgtype.Text{},
		}}
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		if f.subscriptionErr != nil {
			return scanRow{err: f.subscriptionErr}
		}
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
	case strings.Contains(query, "-- name: GetPlatformCredential"):
		f.platformCredentialLookups++
		if f.credentialErr != nil {
			return scanRow{err: f.credentialErr}
		}
		secret := f.credentialSecretValue
		if secret == "" {
			secret = "encrypted-secret"
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{"pc_1", f.platform, "client-id", secret, now, "ws_1"}}
	case strings.Contains(query, "-- name: CreateConnectSession"):
		f.createConnectSessionCalls++
		platform, _ := args[1].(string)
		externalUserID, _ := args[2].(string)
		externalEmail, _ := args[3].(pgtype.Text)
		returnURL, _ := args[4].(pgtype.Text)
		oauthState, _ := args[5].(string)
		pkceVerifier, _ := args[6].(pgtype.Text)
		expiresAt, _ := args[7].(pgtype.Timestamptz)
		allowQuickstart, _ := args[8].(bool)
		if len(args) > 9 {
			f.xAppMode, _ = args[9].(pgtype.Text)
		}
		return f.connectSessionRow(platform, "pending", "", externalUserID, externalEmail, returnURL, oauthState, pkceVerifier, expiresAt, allowQuickstart)
	case strings.Contains(query, "-- name: GetConnectSessionByIDOnly"):
		return f.connectSessionRow(f.platform, f.statusOrDefault(), f.completedAcctID, "user_123", pgtype.Text{}, pgtype.Text{}, "state_1", pgtype.Text{}, futureTimestamptz(), f.allowQuickstart)
	case strings.Contains(query, "-- name: GetConnectSessionByOAuthState"):
		return f.connectSessionRow(f.platform, f.statusOrDefault(), f.completedAcctID, "user_123", pgtype.Text{}, pgtype.Text{}, "state_1", pgtype.Text{}, futureTimestamptz(), f.allowQuickstart)
	case strings.Contains(query, "-- name: FindActiveManagedSocialAccountByExternalAccount"):
		if f.activeAccountLookupErr != nil {
			return scanRow{err: f.activeAccountLookupErr}
		}
		return f.socialAccountRow("sa_active_1", f.platform, "platform-account-old", "user_123", "active")
	case strings.Contains(query, "-- name: CountManagedUsersByWorkspace"):
		return scanRow{values: []any{f.managedUserCount}}
	case strings.Contains(query, "-- name: CountManagedAccountsByWorkspaceAndExternalUser"):
		return scanRow{values: []any{f.existingExternalUserAccountCount}}
	case strings.Contains(query, "-- name: CountActiveManagedAccountsByWorkspace"):
		return scanRow{values: []any{f.activeManagedAccountCount}}
	case strings.Contains(query, "-- name: CountManagedAccountsByWorkspaceExternalUserAndPlatform"):
		return scanRow{values: []any{f.existingExternalUserPlatformAccountCount}}
	case strings.Contains(query, "-- name: CreateManagedSocialAccount"):
		f.createManagedCalls++
		if f.createManagedErr != nil {
			return scanRow{err: f.createManagedErr}
		}
		return f.socialAccountRow("sa_created_1", f.platform, stringArg(args, 5), pgTextString(args, 11), "active")
	case strings.Contains(query, "-- name: UpsertManagedSocialAccount"):
		f.upsertManagedCalls++
		if len(args) > 13 {
			f.xAppMode, _ = args[13].(pgtype.Text)
		}
		id := f.reusedManagedID
		if id == "" {
			id = "sa_upserted_1"
		}
		return f.socialAccountRow(id, f.platform, stringArg(args, 5), pgTextString(args, 11), "active")
	case strings.Contains(query, "-- name: MarkConnectSessionCompleted"):
		if len(args) > 1 {
			if completedID, ok := args[1].(pgtype.Text); ok && completedID.Valid {
				f.completedAcctID = completedID.String
			}
		}
		return f.connectSessionRow(f.platform, "completed", f.completedAcctID, "user_123", pgtype.Text{}, pgtype.Text{}, "state_1", pgtype.Text{}, futureTimestamptz(), f.allowQuickstart)
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
		f.xAppMode,
	}}
}

func (f *connectSessionTestDB) socialAccountRow(id, platform, externalAccountID, externalUserID, status string) scanRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	accountName := pgtype.Text{String: "Robyn", Valid: true}
	externalUser := pgtype.Text{String: externalUserID, Valid: externalUserID != ""}
	sessionID := pgtype.Text{String: "cs_1", Valid: true}
	return scanRow{values: []any{
		id,
		"pr_1",
		platform,
		"encrypted-access",
		pgtype.Text{String: "encrypted-refresh", Valid: true},
		futureTimestamptz(),
		externalAccountID,
		accountName,
		pgtype.Text{},
		now,
		pgtype.Timestamptz{},
		[]byte(`{"username":"Robyn"}`),
		[]string{"user.info.basic"},
		status,
		"managed",
		sessionID,
		externalUser,
		pgtype.Text{},
		now,
		f.xAppMode,
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
	if len(dest) < len(r.values) || len(dest)-len(r.values) > 2 {
		return fmt.Errorf("scan destination count %d != values count %d", len(dest), len(r.values))
	}
	for _, trailing := range dest[len(r.values):] {
		target := reflect.ValueOf(trailing)
		if target.Kind() != reflect.Ptr || target.IsNil() || target.Elem().Type() != reflect.TypeOf(pgtype.Text{}) {
			return fmt.Errorf("unexpected trailing scan destination %T", trailing)
		}
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

func stringArg(args []interface{}, index int) string {
	if index >= len(args) {
		return ""
	}
	value, _ := args[index].(string)
	return value
}

func pgTextString(args []interface{}, index int) string {
	if index >= len(args) {
		return ""
	}
	value, _ := args[index].(pgtype.Text)
	if !value.Valid {
		return ""
	}
	return value.String
}

type fakeOAuthConnector struct {
	platform string
}

type fakeInstagramWebhookSubscriber struct {
	calls       int
	accountID   string
	accessToken string
	err         error
}

func (f *fakeInstagramWebhookSubscriber) Subscribe(_ context.Context, accountID, accessToken string) error {
	f.calls++
	f.accountID = accountID
	f.accessToken = accessToken
	return f.err
}

func (f fakeOAuthConnector) Platform() string {
	return f.platform
}

func (f fakeOAuthConnector) AuthorizeURL(connect.SessionView) (string, error) {
	return "https://auth.example.com", nil
}

func (f fakeOAuthConnector) ExchangeCode(context.Context, connect.SessionView, string) (*connect.TokenSet, error) {
	return &connect.TokenSet{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
		Scopes:       []string{"user.info.basic"},
	}, nil
}

func (f fakeOAuthConnector) FetchProfile(context.Context, string) (*connect.Profile, error) {
	return &connect.Profile{
		ExternalAccountID: "platform-account-new",
		Username:          "Robyn",
		DisplayName:       "Robyn",
	}, nil
}

func (f fakeOAuthConnector) Refresh(context.Context, string) (*connect.TokenSet, error) {
	return nil, fmt.Errorf("unexpected refresh")
}
