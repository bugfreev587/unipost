package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
)

const (
	testClerkToken        = "clerk-session-token"
	testAPIKey            = "up_test_11111111111111111111111111111111"
	testLiveAPIKey        = "up_live_22222222222222222222222222222222"
	testEncodedLiveAPIKey = "%75p_live_22222222222222222222222222222222"
)

type staticInboxPlanGate struct {
	allow bool
}

func (s staticInboxPlanGate) PlanAllowsInbox(context.Context, string) bool {
	return s.allow
}

type recordingInboxPlanGate struct {
	allow       bool
	calls       int
	workspaceID string
	scope       inboxaccess.Scope
}

func (g *recordingInboxPlanGate) PlanAllowsInbox(ctx context.Context, workspaceID string) bool {
	g.calls++
	g.workspaceID = workspaceID
	g.scope, _ = inboxaccess.FromContext(ctx)
	return g.allow
}

type webSocketTestHarness struct {
	handler           *Handler
	store             *webSocketTestDB
	plan              *recordingInboxPlanGate
	clerkCalls        int
	apiCalls          int
	legacyCalls       int
	accepts           int
	serves            int
	legacyServes      int
	scopedServes      int
	serveWS           string
	serveScope        inboxaccess.Scope
	serveContextScope inboxaccess.Scope
}

func newInboxWebSocketTestHandler() *webSocketTestHarness {
	store := &webSocketTestDB{managedUserExists: true, defaultWorkspaceExists: true}
	plan := &recordingInboxPlanGate{allow: true}
	harness := &webSocketTestHarness{store: store, plan: plan}
	harness.handler = NewHandler(NewHub(), db.New(store)).
		WithInboxPlanGate(plan).
		WithInboxScopeAuth()
	harness.handler.clerkTokenAuthenticator = func(ctx context.Context, _ *db.Queries, token string) (context.Context, *auth.TokenAuthFailure) {
		harness.clerkCalls++
		if token != testClerkToken {
			return nil, &auth.TokenAuthFailure{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Invalid session token"}
		}
		return clerkWebSocketContext(ctx, auth.RoleOwner), nil
	}
	harness.handler.apiKeyTokenAuthenticator = func(ctx context.Context, _ *db.Queries, token string) (context.Context, *auth.TokenAuthFailure) {
		harness.apiCalls++
		if token != testAPIKey {
			return nil, &auth.TokenAuthFailure{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Invalid API key"}
		}
		return apiKeyWebSocketContext(ctx, auth.RoleOwner, true), nil
	}
	harness.handler.legacyClerkTokenVerifier = func(_ context.Context, token string) (string, error) {
		harness.legacyCalls++
		if token != testClerkToken {
			return "", errors.New("invalid Clerk token")
		}
		return "user_1", nil
	}
	harness.handler.acceptWebSocket = func(http.ResponseWriter, *http.Request, *websocket.AcceptOptions) (*websocket.Conn, error) {
		harness.accepts++
		return nil, nil
	}
	harness.handler.serveWebSocket = func(ctx context.Context, workspaceID string, _ *websocket.Conn) {
		harness.serves++
		harness.legacyServes++
		harness.serveWS = workspaceID
		harness.serveContextScope, _ = inboxaccess.FromContext(ctx)
	}
	harness.handler.serveScopedWebSocket = func(ctx context.Context, scope inboxaccess.Scope, _ *websocket.Conn) {
		harness.serves++
		harness.scopedServes++
		harness.serveScope = scope
		harness.serveContextScope, _ = inboxaccess.FromContext(ctx)
	}
	return harness
}

func clerkWebSocketContext(ctx context.Context, role string) context.Context {
	ctx = auth.SetWorkspaceID(ctx, "workspace_1")
	return auth.SetRole(ctx, role)
}

func apiKeyWebSocketContext(ctx context.Context, role string, creatorBound bool) context.Context {
	ctx = clerkWebSocketContext(ctx, role)
	ctx = auth.SetAPIKeyID(ctx, "key_1")
	return auth.SetAPIKeyCreatorBound(ctx, creatorBound)
}

func TestInboxWebSocketAcceptsClerkOwnerAndAdminWorkspaceDefault(t *testing.T) {
	for _, role := range []string{auth.RoleOwner, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			harness := newInboxWebSocketTestHandler()
			harness.handler.clerkTokenAuthenticator = func(ctx context.Context, _ *db.Queries, _ string) (context.Context, *auth.TokenAuthFailure) {
				harness.clerkCalls++
				return clerkWebSocketContext(ctx, role), nil
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken, nil)
			rec := httptest.NewRecorder()

			harness.handler.ServeHTTP(rec, req)

			assertWebSocketAccepted(t, harness, inboxaccess.Scope{WorkspaceID: "workspace_1", Mode: inboxaccess.ModeWorkspace})
			if harness.clerkCalls != 1 || harness.apiCalls != 0 {
				t.Fatalf("auth calls = clerk:%d api:%d, want clerk:1 api:0", harness.clerkCalls, harness.apiCalls)
			}
		})
	}
}

func TestInboxWebSocketAcceptsClerkAdminManagedScope(t *testing.T) {
	harness := newInboxWebSocketTestHandler()
	harness.handler.clerkTokenAuthenticator = func(ctx context.Context, _ *db.Queries, _ string) (context.Context, *auth.TokenAuthFailure) {
		harness.clerkCalls++
		return clerkWebSocketContext(ctx, auth.RoleAdmin), nil
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&inbox_scope=managed_user&external_user_id=managed_a", nil)
	rec := httptest.NewRecorder()

	harness.handler.ServeHTTP(rec, req)

	assertWebSocketAccepted(t, harness, inboxaccess.Scope{WorkspaceID: "workspace_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"})
	if harness.store.managedQueries != 1 {
		t.Fatalf("managed-user queries = %d, want 1", harness.store.managedQueries)
	}
}

func TestInboxWebSocketRejectsClerkEditorBeforeAccept(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "workspace", rawURL: "/v1/inbox/ws?token=" + testClerkToken},
		{name: "managed user", rawURL: "/v1/inbox/ws?token=" + testClerkToken + "&inbox_scope=managed_user&external_user_id=managed_a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := newInboxWebSocketTestHandler()
			harness.handler.clerkTokenAuthenticator = func(ctx context.Context, _ *db.Queries, _ string) (context.Context, *auth.TokenAuthFailure) {
				harness.clerkCalls++
				return clerkWebSocketContext(ctx, auth.RoleEditor), nil
			}
			req := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			rec := httptest.NewRecorder()

			harness.handler.ServeHTTP(rec, req)

			assertWebSocketRejected(t, rec, http.StatusForbidden, "INSUFFICIENT_ROLE")
			assertNoPlanOrAccept(t, harness)
		})
	}
}

func TestInboxWebSocketAcceptsAPIKeyManagedScope(t *testing.T) {
	harness := newInboxWebSocketTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?inbox_scope=managed_user&external_user_id=managed_a", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	harness.handler.ServeHTTP(rec, req)

	assertWebSocketAccepted(t, harness, inboxaccess.Scope{WorkspaceID: "workspace_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"})
	if harness.apiCalls != 1 || harness.clerkCalls != 0 {
		t.Fatalf("auth calls = api:%d clerk:%d, want api:1 clerk:0", harness.apiCalls, harness.clerkCalls)
	}
}

func TestInboxWebSocketAcceptsCreatorBoundOwnerAndAdminAPIKeyWorkspaceScope(t *testing.T) {
	for _, role := range []string{auth.RoleOwner, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			harness := newInboxWebSocketTestHandler()
			harness.handler.apiKeyTokenAuthenticator = func(ctx context.Context, _ *db.Queries, _ string) (context.Context, *auth.TokenAuthFailure) {
				harness.apiCalls++
				return apiKeyWebSocketContext(ctx, role, true), nil
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?inbox_scope=workspace", nil)
			req.Header.Set("Authorization", "Bearer "+testAPIKey)
			rec := httptest.NewRecorder()

			harness.handler.ServeHTTP(rec, req)

			assertWebSocketAccepted(t, harness, inboxaccess.Scope{WorkspaceID: "workspace_1", Mode: inboxaccess.ModeWorkspace})
		})
	}
}

func TestInboxWebSocketRejectsInvalidAPIKeyScopesBeforePlanAndAccept(t *testing.T) {
	tests := []struct {
		name         string
		rawURL       string
		role         string
		creatorBound bool
		wantStatus   int
		wantCode     string
	}{
		{name: "missing scope", rawURL: "/v1/inbox/ws", role: auth.RoleOwner, creatorBound: true, wantStatus: http.StatusBadRequest, wantCode: "INBOX_SCOPE_REQUIRED"},
		{name: "creatorless workspace", rawURL: "/v1/inbox/ws?inbox_scope=workspace", role: auth.RoleOwner, wantStatus: http.StatusForbidden, wantCode: "API_KEY_CREATOR_REQUIRED"},
		{name: "editor workspace", rawURL: "/v1/inbox/ws?inbox_scope=workspace", role: auth.RoleEditor, creatorBound: true, wantStatus: http.StatusForbidden, wantCode: "INSUFFICIENT_ROLE"},
		{name: "unknown managed user", rawURL: "/v1/inbox/ws?inbox_scope=managed_user&external_user_id=missing", role: auth.RoleOwner, creatorBound: true, wantStatus: http.StatusNotFound, wantCode: "MANAGED_USER_NOT_FOUND"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := newInboxWebSocketTestHandler()
			if tt.name == "unknown managed user" {
				harness.store.managedUserExists = false
			}
			harness.handler.apiKeyTokenAuthenticator = func(ctx context.Context, _ *db.Queries, _ string) (context.Context, *auth.TokenAuthFailure) {
				harness.apiCalls++
				return apiKeyWebSocketContext(ctx, tt.role, tt.creatorBound), nil
			}
			req := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			req.Header.Set("Authorization", "Bearer "+testAPIKey)
			rec := httptest.NewRecorder()

			harness.handler.ServeHTTP(rec, req)

			assertWebSocketRejected(t, rec, tt.wantStatus, tt.wantCode)
			assertNoPlanOrAccept(t, harness)
			if harness.apiCalls != 1 {
				t.Fatalf("API authenticator calls = %d, want 1", harness.apiCalls)
			}
		})
	}
}

func TestInboxWebSocketRejectsAmbiguousOrMalformedCredentials(t *testing.T) {
	tests := []struct {
		name    string
		request func() *http.Request
	}{
		{
			name: "both credential forms",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&inbox_scope=workspace", nil)
				req.Header.Set("Authorization", "Bearer "+testAPIKey)
				return req
			},
		},
		{name: "neither credential form", request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws", nil) }},
		{name: "API key in token query", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testAPIKey+"&inbox_scope=workspace", nil)
		}},
		{name: "API key in obvious query param", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?api_key="+testAPIKey, nil)
		}},
		{name: "case-variant API key query param with Clerk token", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&API_KEY="+testAPIKey, nil)
		}},
		{
			name: "case-variant token alias with valid API header",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?Token="+testClerkToken+"&inbox_scope=workspace", nil)
				req.Header.Set("Authorization", "Bearer "+testAPIKey)
				return req
			},
		},
		{name: "API key under arbitrary query key", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&foo="+testAPIKey, nil)
		}},
		{name: "percent-encoded API key under arbitrary query key", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&foo="+testEncodedLiveAPIKey, nil)
		}},
		{name: "unknown benign query key", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&foo=bar", nil)
		}},
		{
			name: "Clerk token in Authorization",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?inbox_scope=workspace", nil)
				req.Header.Set("Authorization", "Bearer "+testClerkToken)
				return req
			},
		},
		{
			name: "malformed bearer",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?inbox_scope=workspace", nil)
				req.Header.Set("Authorization", "Bearer  "+testAPIKey)
				return req
			},
		},
		{
			name: "duplicate Authorization",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?inbox_scope=workspace", nil)
				req.Header.Add("Authorization", "Bearer "+testAPIKey)
				req.Header.Add("Authorization", "Bearer "+testAPIKey)
				return req
			},
		},
		{name: "duplicate token", request: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken+"&token=second", nil)
		}},
		{
			name: "malformed query",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token="+testClerkToken, nil)
				req.URL.RawQuery = "token=" + testClerkToken + "&external_user_id=%ZZ"
				return req
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := newInboxWebSocketTestHandler()
			var logs bytes.Buffer
			oldLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(oldLogger) })
			req := tt.request()
			rec := httptest.NewRecorder()

			harness.handler.ServeHTTP(rec, req)

			assertWebSocketRejected(t, rec, http.StatusUnauthorized, "UNAUTHORIZED")
			assertNoPlanOrAccept(t, harness)
			if harness.clerkCalls != 0 || harness.apiCalls != 0 {
				t.Fatalf("auth calls = clerk:%d api:%d, want none", harness.clerkCalls, harness.apiCalls)
			}
			for _, secret := range []string{testAPIKey, testLiveAPIKey, testEncodedLiveAPIKey, testClerkToken} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("logs contain credential %q", secret)
				}
			}
		})
	}
}

func TestInboxWebSocketPlanRejectionHappensBeforeAccept(t *testing.T) {
	harness := newInboxWebSocketTestHandler()
	harness.plan.allow = false
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?inbox_scope=workspace", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	harness.handler.ServeHTTP(rec, req)

	assertWebSocketRejected(t, rec, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE")
	if harness.plan.calls != 1 {
		t.Fatalf("plan calls = %d, want 1", harness.plan.calls)
	}
	if harness.accepts != 0 || harness.serves != 0 {
		t.Fatalf("accept/serve calls = %d/%d, want 0/0", harness.accepts, harness.serves)
	}
}

func TestInboxWebSocketPreservesLogsClerkOnlyMode(t *testing.T) {
	harness := newInboxWebSocketTestHandler()
	harness.handler.scopedInboxAuth = false
	harness.handler.planChecker = nil

	t.Run("query Clerk token remains accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/ws?token="+testClerkToken, nil)
		rec := httptest.NewRecorder()

		harness.handler.ServeHTTP(rec, req)

		if harness.accepts != 1 || harness.serves != 1 || harness.serveWS != "workspace_1" {
			t.Fatalf("accept/serve/workspace = %d/%d/%q, want 1/1/workspace_1", harness.accepts, harness.serves, harness.serveWS)
		}
		if harness.legacyServes != 1 || harness.scopedServes != 0 {
			t.Fatalf("legacy/scoped serves = %d/%d, want 1/0", harness.legacyServes, harness.scopedServes)
		}
		if harness.legacyCalls != 1 || harness.clerkCalls != 0 || harness.apiCalls != 0 {
			t.Fatalf("legacy/scoped Clerk/API auth calls = %d/%d/%d, want 1/0/0", harness.legacyCalls, harness.clerkCalls, harness.apiCalls)
		}
		if harness.store.defaultWorkspaceQueries != 1 {
			t.Fatalf("default workspace queries = %d, want 1", harness.store.defaultWorkspaceQueries)
		}
		if harness.store.activeMembershipQueries != 0 || harness.store.listWorkspaceQueries != 0 || harness.store.membershipCreates != 0 {
			t.Fatalf("active membership/list/self-heal queries = %d/%d/%d, want 0/0/0", harness.store.activeMembershipQueries, harness.store.listWorkspaceQueries, harness.store.membershipCreates)
		}
	})

	t.Run("header API key is not enabled", func(t *testing.T) {
		beforeAPICalls := harness.apiCalls
		beforeLegacyCalls := harness.legacyCalls
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/ws", nil)
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
		rec := httptest.NewRecorder()

		harness.handler.ServeHTTP(rec, req)

		assertWebSocketRejected(t, rec, http.StatusUnauthorized, "UNAUTHORIZED")
		if harness.apiCalls != beforeAPICalls {
			t.Fatalf("API authenticator calls changed from %d to %d", beforeAPICalls, harness.apiCalls)
		}
		if harness.legacyCalls != beforeLegacyCalls {
			t.Fatalf("legacy verifier calls changed from %d to %d", beforeLegacyCalls, harness.legacyCalls)
		}
	})
}

func TestLogsWebSocketPreservesLegacyFailureEnvelope(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		harness := newInboxWebSocketTestHandler()
		harness.handler.scopedInboxAuth = false
		harness.handler.planChecker = nil
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/ws", nil)
		rec := httptest.NewRecorder()

		harness.handler.ServeHTTP(rec, req)

		assertWebSocketRejectedMessage(t, rec, http.StatusUnauthorized, "UNAUTHORIZED", "Missing token query param")
		if harness.legacyCalls != 0 || harness.store.defaultWorkspaceQueries != 0 || harness.accepts != 0 {
			t.Fatalf("legacy/default workspace/accept calls = %d/%d/%d, want 0/0/0", harness.legacyCalls, harness.store.defaultWorkspaceQueries, harness.accepts)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		harness := newInboxWebSocketTestHandler()
		harness.handler.scopedInboxAuth = false
		harness.handler.planChecker = nil
		harness.handler.legacyClerkTokenVerifier = func(context.Context, string) (string, error) {
			harness.legacyCalls++
			return "", errors.New("invalid")
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/ws?token=invalid", nil)
		rec := httptest.NewRecorder()

		harness.handler.ServeHTTP(rec, req)

		assertWebSocketRejectedMessage(t, rec, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token")
		if harness.store.defaultWorkspaceQueries != 0 || harness.accepts != 0 {
			t.Fatalf("default workspace/accept calls = %d/%d, want 0/0", harness.store.defaultWorkspaceQueries, harness.accepts)
		}
	})

	t.Run("no default workspace", func(t *testing.T) {
		harness := newInboxWebSocketTestHandler()
		harness.handler.scopedInboxAuth = false
		harness.handler.planChecker = nil
		harness.store.defaultWorkspaceExists = false
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/ws?token="+testClerkToken, nil)
		rec := httptest.NewRecorder()

		harness.handler.ServeHTTP(rec, req)

		assertWebSocketRejectedMessage(t, rec, http.StatusForbidden, "FORBIDDEN", "No workspace found for user")
		if harness.store.defaultWorkspaceQueries != 1 || harness.accepts != 0 {
			t.Fatalf("default workspace/accept calls = %d/%d, want 1/0", harness.store.defaultWorkspaceQueries, harness.accepts)
		}
		if harness.store.activeMembershipQueries != 0 || harness.store.listWorkspaceQueries != 0 || harness.store.membershipCreates != 0 {
			t.Fatalf("active membership/list/self-heal queries = %d/%d/%d, want 0/0/0", harness.store.activeMembershipQueries, harness.store.listWorkspaceQueries, harness.store.membershipCreates)
		}
	})
}

func TestInboxWebSocketPlanGateBlocksUnavailablePlans(t *testing.T) {
	handler := NewHandler(NewHub(), nil).WithInboxPlanGate(staticInboxPlanGate{allow: false})
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws", nil)
	rr := httptest.NewRecorder()

	if handler.ensureInboxPlanAllowed(rr, req, "workspace_123") {
		t.Fatal("expected inbox websocket plan gate to block unavailable plan")
	}
	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status 402, got %d", rr.Code)
	}
}

func TestInboxWebSocketPlanGateAllowsUnlockedPlans(t *testing.T) {
	handler := NewHandler(NewHub(), nil).WithInboxPlanGate(staticInboxPlanGate{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws", nil)
	rr := httptest.NewRecorder()

	if !handler.ensureInboxPlanAllowed(rr, req, "workspace_123") {
		t.Fatal("expected inbox websocket plan gate to allow unlocked plan")
	}
}

func assertWebSocketAccepted(t *testing.T, harness *webSocketTestHarness, want inboxaccess.Scope) {
	t.Helper()
	if harness.plan.calls != 1 || harness.accepts != 1 || harness.serves != 1 {
		t.Fatalf("plan/accept/serve calls = %d/%d/%d, want 1/1/1", harness.plan.calls, harness.accepts, harness.serves)
	}
	if harness.legacyServes != 0 || harness.scopedServes != 1 {
		t.Fatalf("legacy/scoped serves = %d/%d, want 0/1", harness.legacyServes, harness.scopedServes)
	}
	if harness.plan.workspaceID != want.WorkspaceID || harness.serveScope.WorkspaceID != want.WorkspaceID {
		t.Fatalf("plan/serve workspaces = %q/%q, want %q", harness.plan.workspaceID, harness.serveScope.WorkspaceID, want.WorkspaceID)
	}
	if harness.plan.scope != want || harness.serveScope != want || harness.serveContextScope != want {
		t.Fatalf("plan/serve/context scopes = %#v/%#v/%#v, want %#v", harness.plan.scope, harness.serveScope, harness.serveContextScope, want)
	}
}

func assertNoPlanOrAccept(t *testing.T, harness *webSocketTestHarness) {
	t.Helper()
	if harness.plan.calls != 0 || harness.accepts != 0 || harness.serves != 0 {
		t.Fatalf("plan/accept/serve calls = %d/%d/%d, want 0/0/0", harness.plan.calls, harness.accepts, harness.serves)
	}
}

func assertWebSocketRejected(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	assertWebSocketRejectedMessage(t, recorder, wantStatus, wantCode, "")
}

func assertWebSocketRejectedMessage(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode, wantMessage string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var response errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	if response.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q; body=%s", response.Error.Code, wantCode, recorder.Body.String())
	}
	if wantMessage != "" && response.Error.Message != wantMessage {
		t.Fatalf("error message = %q, want %q; body=%s", response.Error.Message, wantMessage, recorder.Body.String())
	}
}

type webSocketTestDB struct {
	managedUserExists       bool
	defaultWorkspaceExists  bool
	managedQueries          int
	defaultWorkspaceQueries int
	activeMembershipQueries int
	listWorkspaceQueries    int
	membershipCreates       int
}

func (*webSocketTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *webSocketTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "-- name: ListWorkspacesByUser") {
		f.listWorkspaceQueries++
	}
	return nil, errors.New("unexpected Query")
}

func (f *webSocketTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetDefaultWorkspaceForUser"):
		f.defaultWorkspaceQueries++
		if !f.defaultWorkspaceExists {
			return webSocketWorkspaceRow{err: pgx.ErrNoRows}
		}
		if len(args) != 1 || args[0] != "user_1" {
			return webSocketWorkspaceRow{err: errors.New("default workspace lookup used unexpected user")}
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return webSocketWorkspaceRow{values: []any{
			"workspace_1", "user_1", "Workspace", pgtype.Int4{}, now, now, []string{"direct"}, pgtype.Text{},
		}}
	case strings.Contains(query, "-- name: GetActiveMembership"):
		f.activeMembershipQueries++
		return webSocketWorkspaceRow{err: errors.New("unexpected active membership lookup")}
	case strings.Contains(query, "-- name: CreateMembership"):
		f.membershipCreates++
		return webSocketWorkspaceRow{err: errors.New("unexpected membership self-heal")}
	case strings.Contains(query, "-- name: InboxManagedUserExists"):
		// Continue with the managed-user existence result below.
	default:
		return webSocketTestRow{err: errors.New("unexpected QueryRow")}
	}
	f.managedQueries++
	if len(args) != 2 || args[0] != "workspace_1" {
		return webSocketTestRow{err: errors.New("managed-user lookup used unexpected workspace")}
	}
	externalUserID, ok := args[1].(pgtype.Text)
	if !ok || !externalUserID.Valid || strings.TrimSpace(externalUserID.String) == "" {
		return webSocketTestRow{err: errors.New("managed-user lookup used invalid external user")}
	}
	return webSocketTestRow{value: f.managedUserExists}
}

type webSocketWorkspaceRow struct {
	values []any
	err    error
}

func (r webSocketWorkspaceRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected workspace scan destination count")
	}
	for index := range dest {
		switch target := dest[index].(type) {
		case *string:
			*target = r.values[index].(string)
		case *pgtype.Int4:
			*target = r.values[index].(pgtype.Int4)
		case *pgtype.Timestamptz:
			*target = r.values[index].(pgtype.Timestamptz)
		case *[]string:
			*target = r.values[index].([]string)
		case *pgtype.Text:
			*target = r.values[index].(pgtype.Text)
		default:
			return errors.New("unsupported workspace scan destination")
		}
	}
	return nil
}

type webSocketTestRow struct {
	value bool
	err   error
}

func (r webSocketTestRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("unexpected scan destination count")
	}
	value, ok := dest[0].(*bool)
	if !ok {
		return errors.New("unexpected scan destination type")
	}
	*value = r.value
	return nil
}
