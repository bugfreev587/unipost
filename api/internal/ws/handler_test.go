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

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
)

const (
	testClerkToken = "clerk-session-token"
	testAPIKey     = "up_test_11111111111111111111111111111111"
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
	handler    *Handler
	store      *webSocketTestDB
	plan       *recordingInboxPlanGate
	clerkCalls int
	apiCalls   int
	accepts    int
	serves     int
	serveWS    string
	serveScope inboxaccess.Scope
}

func newInboxWebSocketTestHandler() *webSocketTestHarness {
	store := &webSocketTestDB{managedUserExists: true}
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
	harness.handler.acceptWebSocket = func(http.ResponseWriter, *http.Request, *websocket.AcceptOptions) (*websocket.Conn, error) {
		harness.accepts++
		return nil, nil
	}
	harness.handler.serveWebSocket = func(ctx context.Context, workspaceID string, _ *websocket.Conn) {
		harness.serves++
		harness.serveWS = workspaceID
		harness.serveScope, _ = inboxaccess.FromContext(ctx)
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
			for _, secret := range []string{testAPIKey, testClerkToken} {
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
	})

	t.Run("header API key is not enabled", func(t *testing.T) {
		beforeAPICalls := harness.apiCalls
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/ws", nil)
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
		rec := httptest.NewRecorder()

		harness.handler.ServeHTTP(rec, req)

		assertWebSocketRejected(t, rec, http.StatusUnauthorized, "UNAUTHORIZED")
		if harness.apiCalls != beforeAPICalls {
			t.Fatalf("API authenticator calls changed from %d to %d", beforeAPICalls, harness.apiCalls)
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
	if harness.plan.workspaceID != want.WorkspaceID || harness.serveWS != want.WorkspaceID {
		t.Fatalf("plan/serve workspaces = %q/%q, want %q", harness.plan.workspaceID, harness.serveWS, want.WorkspaceID)
	}
	if harness.plan.scope != want || harness.serveScope != want {
		t.Fatalf("plan/serve scopes = %#v/%#v, want %#v", harness.plan.scope, harness.serveScope, want)
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
}

type webSocketTestDB struct {
	managedUserExists bool
	managedQueries    int
}

func (*webSocketTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (*webSocketTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *webSocketTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	if !strings.Contains(query, "-- name: InboxManagedUserExists") {
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
