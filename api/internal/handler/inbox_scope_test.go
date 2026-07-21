package handler

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
)

func TestRequireInboxAccessScopeRejectsMissingAPIKeyScopeBeforeNext(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
	ctx := auth.SetWorkspaceID(req.Context(), "workspace_1")
	ctx = auth.SetAPIKeyID(ctx, "key_1")
	ctx = auth.SetAPIKeyCreatorBound(ctx, true)
	ctx = auth.SetRole(ctx, auth.RoleOwner)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()

	RequireInboxAccessScope(nil)(next).ServeHTTP(recorder, req)

	if called {
		t.Fatal("next handler was called after scope rejection")
	}
	assertInboxScopeError(t, recorder, http.StatusBadRequest, "INBOX_SCOPE_REQUIRED")
}

func TestRequireInboxAccessScopeAddsResolvedScopeToContext(t *testing.T) {
	want := inboxaccess.Scope{
		WorkspaceID: "workspace_1",
		Mode:        inboxaccess.ModeWorkspace,
	}
	var got inboxaccess.Scope
	var found bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, found = inboxaccess.FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/inbox?inbox_scope=workspace", nil)
	ctx := auth.SetWorkspaceID(req.Context(), want.WorkspaceID)
	ctx = auth.SetAPIKeyID(ctx, "key_1")
	ctx = auth.SetAPIKeyCreatorBound(ctx, true)
	ctx = auth.SetRole(ctx, auth.RoleOwner)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()

	RequireInboxAccessScope(nil)(next).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if !found || got != want {
		t.Fatalf("scope from context = (%#v, %v), want (%#v, true)", got, found, want)
	}
}

func TestRequireInboxAccessScopePropagatesResolverFailures(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		workspace  string
		apiKeyID   string
		role       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing authenticated workspace",
			rawURL:     "/v1/inbox",
			role:       auth.RoleOwner,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:       "invalid API key scope",
			rawURL:     "/v1/inbox?inbox_scope=invalid",
			workspace:  "workspace_1",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INBOX_SCOPE_INVALID",
		},
		{
			name:       "workspace scope rejects external user",
			rawURL:     "/v1/inbox?inbox_scope=workspace&external_user_id=user_a",
			workspace:  "workspace_1",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "EXTERNAL_USER_ID_NOT_ALLOWED",
		},
		{
			name:       "managed user lookup fails closed without queries",
			rawURL:     "/v1/inbox?inbox_scope=managed_user&external_user_id=user_a",
			workspace:  "workspace_1",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INBOX_SCOPE_LOOKUP_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			})
			req := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			ctx := auth.SetWorkspaceID(req.Context(), tt.workspace)
			ctx = auth.SetRole(ctx, tt.role)
			if tt.apiKeyID != "" {
				ctx = auth.SetAPIKeyID(ctx, tt.apiKeyID)
				ctx = auth.SetAPIKeyCreatorBound(ctx, true)
			}
			req = req.WithContext(ctx)
			recorder := httptest.NewRecorder()

			RequireInboxAccessScope(nil)(next).ServeHTTP(recorder, req)

			if called {
				t.Fatal("next handler was called after scope rejection")
			}
			assertInboxScopeError(t, recorder, tt.wantStatus, tt.wantCode)
		})
	}
}

func TestRequireInboxAccessScopeRouteRegistrationContract(t *testing.T) {
	mainPath := filepath.Join("..", "..", "cmd", "api", "main.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), mainPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mainPath, err)
	}

	var inboxRoute *ast.FuncLit
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || selectorName(call.Fun) != "Route" {
			return true
		}
		path, ok := stringLiteral(call.Args[0])
		if !ok || path != "/v1/inbox" {
			return true
		}
		inboxRoute, _ = call.Args[1].(*ast.FuncLit)
		return false
	})
	if inboxRoute == nil {
		t.Fatal("/v1/inbox route group was not found")
	}

	type registration struct {
		method  string
		path    string
		handler string
	}
	registrations := make([]registration, 0, len(inboxRoute.Body.List))
	for _, statement := range inboxRoute.Body.List {
		expression, ok := statement.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expression.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		method := selectorName(call.Fun)
		switch method {
		case "Use":
			if len(call.Args) != 1 {
				t.Fatalf("Inbox middleware registration has %d arguments, want 1", len(call.Args))
			}
			registrations = append(registrations, registration{method: method, handler: selectorName(call.Args[0])})
		case "Get", "Post":
			if len(call.Args) != 2 {
				t.Fatalf("Inbox %s registration has %d arguments, want 2", method, len(call.Args))
			}
			path, ok := stringLiteral(call.Args[0])
			if !ok {
				t.Fatalf("Inbox %s route path is not a string literal", method)
			}
			registrations = append(registrations, registration{
				method:  method,
				path:    path,
				handler: selectorName(call.Args[1]),
			})
			registrations[len(registrations)-1].method = map[string]string{
				"Get":  http.MethodGet,
				"Post": http.MethodPost,
			}[method]
		}
	}

	want := []registration{
		{method: "Use", handler: "RequireInboxAccessScope"},
		{method: "Use", handler: "RequirePlanInbox"},
		{method: http.MethodGet, path: "/", handler: "List"},
		{method: http.MethodGet, path: "/unread-count", handler: "UnreadCount"},
		{method: http.MethodGet, path: "/x-outbound-operations/{requestID}", handler: "XOutboundStatus"},
		{method: http.MethodPost, path: "/mark-all-read", handler: "MarkAllRead"},
		{method: http.MethodPost, path: "/sync", handler: "Sync"},
		{method: http.MethodGet, path: "/{id}", handler: "Get"},
		{method: http.MethodGet, path: "/{id}/media-context", handler: "MediaContext"},
		{method: http.MethodPost, path: "/{id}/read", handler: "MarkRead"},
		{method: http.MethodPost, path: "/{id}/reply", handler: "Reply"},
		{method: http.MethodPost, path: "/{id}/thread-state", handler: "UpdateThreadState"},
	}
	if len(registrations) != len(want) {
		t.Fatalf("Inbox registrations = %#v (%d), want %#v (%d)", registrations, len(registrations), want, len(want))
	}
	for i := range want {
		if registrations[i] != want[i] {
			t.Fatalf("Inbox registration[%d] = %#v, want %#v; scope middleware must be exactly once and before plan middleware", i, registrations[i], want[i])
		}
	}
}

func assertInboxScopeError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, recorder.Body.String())
	}
	if response.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q; body=%s", response.Error.Code, wantCode, recorder.Body.String())
	}
	if response.Error.Message == "" {
		t.Fatalf("error message is empty; body=%s", recorder.Body.String())
	}
}

func selectorName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.CallExpr:
		return selectorName(value.Fun)
	case *ast.Ident:
		return value.Name
	default:
		return ""
	}
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}
