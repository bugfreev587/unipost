package inboxaccess

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestResolve(t *testing.T) {
	dbFailure := errors.New("managed-user lookup failed")
	tests := []struct {
		name              string
		rawURL            string
		apiKeyID          string
		creatorBound      bool
		role              string
		managedUserExists bool
		dbErr             error
		want              Scope
		wantStatus        int
		wantCode          string
		wantQueries       int
	}{
		{
			name:       "API key requires explicit scope",
			rawURL:     "/v1/inbox",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INBOX_SCOPE_REQUIRED",
		},
		{
			name:       "API key rejects blank scope",
			rawURL:     "/v1/inbox?inbox_scope=%20",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INBOX_SCOPE_INVALID",
		},
		{
			name:       "API key rejects duplicate scope",
			rawURL:     "/v1/inbox?inbox_scope=workspace&inbox_scope=workspace",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INBOX_SCOPE_DUPLICATE",
		},
		{
			name:       "API key rejects contradictory scopes",
			rawURL:     "/v1/inbox?inbox_scope=workspace&inbox_scope=managed_user&external_user_id=user_a",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "INBOX_SCOPE_DUPLICATE",
		},
		{
			name:              "managed user exists in authenticated workspace",
			rawURL:            "/v1/inbox?inbox_scope=managed_user&external_user_id=%20user_a%20&workspace_id=workspace_attacker",
			apiKeyID:          "key_1",
			role:              auth.RoleEditor,
			creatorBound:      true,
			managedUserExists: true,
			want: Scope{
				WorkspaceID:    "workspace_auth",
				Mode:           ModeManagedUser,
				ExternalUserID: "user_a",
			},
			wantQueries: 1,
		},
		{
			name:              "unknown managed user fails closed",
			rawURL:            "/v1/inbox?inbox_scope=managed_user&external_user_id=user_unknown",
			apiKeyID:          "key_1",
			role:              auth.RoleOwner,
			managedUserExists: false,
			wantStatus:        http.StatusNotFound,
			wantCode:          "MANAGED_USER_NOT_FOUND",
			wantQueries:       1,
		},
		{
			name:        "managed user lookup error fails closed",
			rawURL:      "/v1/inbox?inbox_scope=managed_user&external_user_id=user_a",
			apiKeyID:    "key_1",
			role:        auth.RoleOwner,
			dbErr:       dbFailure,
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "INBOX_SCOPE_LOOKUP_FAILED",
			wantQueries: 1,
		},
		{
			name:         "creator-bound owner API key may aggregate workspace",
			rawURL:       "/v1/inbox?inbox_scope=workspace",
			apiKeyID:     "key_1",
			creatorBound: true,
			role:         auth.RoleOwner,
			want:         Scope{WorkspaceID: "workspace_auth", Mode: ModeWorkspace},
		},
		{
			name:         "creator-bound admin API key may aggregate workspace",
			rawURL:       "/v1/inbox?inbox_scope=workspace",
			apiKeyID:     "key_1",
			creatorBound: true,
			role:         auth.RoleAdmin,
			want:         Scope{WorkspaceID: "workspace_auth", Mode: ModeWorkspace},
		},
		{
			name:         "editor API key cannot aggregate workspace",
			rawURL:       "/v1/inbox?inbox_scope=workspace",
			apiKeyID:     "key_1",
			creatorBound: true,
			role:         auth.RoleEditor,
			wantStatus:   http.StatusForbidden,
			wantCode:     "INSUFFICIENT_ROLE",
		},
		{
			name:       "legacy API key cannot aggregate workspace",
			rawURL:     "/v1/inbox?inbox_scope=workspace",
			apiKeyID:   "key_legacy",
			role:       auth.RoleOwner,
			wantStatus: http.StatusForbidden,
			wantCode:   "API_KEY_CREATOR_REQUIRED",
		},
		{
			name:              "legacy API key may select managed user",
			rawURL:            "/v1/inbox?inbox_scope=managed_user&external_user_id=user_a",
			apiKeyID:          "key_legacy",
			role:              auth.RoleOwner,
			managedUserExists: true,
			want:              Scope{WorkspaceID: "workspace_auth", Mode: ModeManagedUser, ExternalUserID: "user_a"},
			wantQueries:       1,
		},
		{
			name:   "Clerk owner defaults to workspace",
			rawURL: "/v1/inbox",
			role:   auth.RoleOwner,
			want:   Scope{WorkspaceID: "workspace_auth", Mode: ModeWorkspace},
		},
		{
			name:   "Clerk admin may explicitly select workspace",
			rawURL: "/v1/inbox?inbox_scope=workspace",
			role:   auth.RoleAdmin,
			want:   Scope{WorkspaceID: "workspace_auth", Mode: ModeWorkspace},
		},
		{
			name:              "Clerk owner may explicitly select managed user",
			rawURL:            "/v1/inbox?inbox_scope=managed_user&external_user_id=user_a",
			role:              auth.RoleOwner,
			managedUserExists: true,
			want:              Scope{WorkspaceID: "workspace_auth", Mode: ModeManagedUser, ExternalUserID: "user_a"},
			wantQueries:       1,
		},
		{
			name:       "Clerk editor cannot use workspace mode",
			rawURL:     "/v1/inbox",
			role:       auth.RoleEditor,
			wantStatus: http.StatusForbidden,
			wantCode:   "INSUFFICIENT_ROLE",
		},
		{
			name:       "Clerk editor cannot use managed mode",
			rawURL:     "/v1/inbox?inbox_scope=managed_user&external_user_id=user_a",
			role:       auth.RoleEditor,
			wantStatus: http.StatusForbidden,
			wantCode:   "INSUFFICIENT_ROLE",
		},
		{
			name:         "workspace rejects external user ID",
			rawURL:       "/v1/inbox?inbox_scope=workspace&external_user_id=user_a",
			apiKeyID:     "key_1",
			creatorBound: true,
			role:         auth.RoleOwner,
			wantStatus:   http.StatusBadRequest,
			wantCode:     "EXTERNAL_USER_ID_NOT_ALLOWED",
		},
		{
			name:         "workspace rejects blank external user ID",
			rawURL:       "/v1/inbox?inbox_scope=workspace&external_user_id=",
			apiKeyID:     "key_1",
			creatorBound: true,
			role:         auth.RoleOwner,
			wantStatus:   http.StatusBadRequest,
			wantCode:     "EXTERNAL_USER_ID_NOT_ALLOWED",
		},
		{
			name:       "managed mode requires external user ID",
			rawURL:     "/v1/inbox?inbox_scope=managed_user",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "EXTERNAL_USER_ID_REQUIRED",
		},
		{
			name:       "managed mode rejects blank external user ID",
			rawURL:     "/v1/inbox?inbox_scope=managed_user&external_user_id=%20",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "EXTERNAL_USER_ID_REQUIRED",
		},
		{
			name:       "managed mode rejects duplicate external user IDs",
			rawURL:     "/v1/inbox?inbox_scope=managed_user&external_user_id=user_a&external_user_id=user_b",
			apiKeyID:   "key_1",
			role:       auth.RoleOwner,
			wantStatus: http.StatusBadRequest,
			wantCode:   "EXTERNAL_USER_ID_DUPLICATE",
		},
		{
			name:   "header shape cannot impersonate API-key auth",
			rawURL: "/v1/inbox",
			role:   auth.RoleOwner,
			want:   Scope{WorkspaceID: "workspace_auth", Mode: ModeWorkspace},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &scopeTestDB{exists: tt.managedUserExists, err: tt.dbErr}
			req := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			ctx := auth.SetWorkspaceID(req.Context(), "workspace_auth")
			ctx = auth.SetRole(ctx, tt.role)
			if tt.apiKeyID != "" {
				ctx = auth.SetAPIKeyID(ctx, tt.apiKeyID)
				ctx = auth.SetAPIKeyCreatorBound(ctx, tt.creatorBound)
			} else if tt.name == "header shape cannot impersonate API-key auth" {
				req.Header.Set("Authorization", "Bearer up_live_not_authenticated")
			}
			req = req.WithContext(ctx)

			got, failure := Resolve(req, db.New(store))

			if tt.wantCode == "" {
				if failure != nil {
					t.Fatalf("Resolve() failure = %#v, want nil", failure)
				}
				if got != tt.want {
					t.Fatalf("Resolve() scope = %#v, want %#v", got, tt.want)
				}
			} else {
				if failure == nil {
					t.Fatalf("Resolve() failure = nil, want status %d code %q", tt.wantStatus, tt.wantCode)
				}
				if failure.Status != tt.wantStatus || failure.Code != tt.wantCode {
					t.Fatalf("Resolve() failure = %#v, want status %d code %q", failure, tt.wantStatus, tt.wantCode)
				}
				if strings.TrimSpace(failure.Message) == "" {
					t.Fatal("Resolve() failure message is empty")
				}
				if got != (Scope{}) {
					t.Fatalf("Resolve() scope = %#v on failure, want zero value", got)
				}
			}
			if store.queries != tt.wantQueries {
				t.Fatalf("managed-user queries = %d, want %d", store.queries, tt.wantQueries)
			}
			if tt.wantQueries > 0 {
				if store.workspaceID != "workspace_auth" {
					t.Fatalf("lookup workspace = %q, want authenticated workspace", store.workspaceID)
				}
				if store.externalUserID != strings.TrimSpace(req.URL.Query().Get("external_user_id")) {
					t.Fatalf("lookup external user = %q, want trimmed request value", store.externalUserID)
				}
			}
		})
	}
}

func TestScopeContextRoundTrip(t *testing.T) {
	want := Scope{WorkspaceID: "workspace_1", Mode: ModeManagedUser, ExternalUserID: "managed_1"}
	ctx := WithContext(context.Background(), want)
	got, ok := FromContext(ctx)
	if !ok || got != want {
		t.Fatalf("FromContext() = (%#v, %v), want (%#v, true)", got, ok, want)
	}

	if got, ok := FromContext(context.Background()); ok || got != (Scope{}) {
		t.Fatalf("empty FromContext() = (%#v, %v), want zero value and false", got, ok)
	}
}

func TestScopeWorkspaceWide(t *testing.T) {
	tests := []struct {
		name  string
		scope Scope
		want  bool
	}{
		{name: "workspace", scope: Scope{Mode: ModeWorkspace}, want: true},
		{name: "managed user", scope: Scope{Mode: ModeManagedUser, ExternalUserID: "managed_1"}, want: false},
		{name: "empty mode is not workspace", scope: Scope{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.WorkspaceWide(); got != tt.want {
				t.Fatalf("WorkspaceWide() = %v, want %v", got, tt.want)
			}
		})
	}
}

type scopeTestDB struct {
	exists         bool
	err            error
	queries        int
	workspaceID    string
	externalUserID string
}

func (f *scopeTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *scopeTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *scopeTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	if !strings.Contains(query, "-- name: InboxManagedUserExists") {
		return scopeTestRow{err: errors.New("unexpected QueryRow")}
	}
	f.queries++
	if len(args) == 2 {
		f.workspaceID, _ = args[0].(string)
		switch value := args[1].(type) {
		case pgtype.Text:
			if value.Valid {
				f.externalUserID = value.String
			}
		case string:
			f.externalUserID = value
		}
	}
	return scopeTestRow{exists: f.exists, err: f.err}
}

type scopeTestRow struct {
	exists bool
	err    error
}

func (r scopeTestRow) Scan(dest ...interface{}) error {
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
	*value = r.exists
	return nil
}
