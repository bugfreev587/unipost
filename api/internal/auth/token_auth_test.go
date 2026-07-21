package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestAuthenticateAPIKeyTokenCreatorBoundAdmin(t *testing.T) {
	store := &apiKeyAuthTestDB{membershipRole: RoleAdmin, membershipStatus: "active"}

	ctx, failure := AuthenticateAPIKeyToken(context.Background(), db.New(store), apiKeyAuthTestToken)

	if failure != nil {
		t.Fatalf("AuthenticateAPIKeyToken failure = %#v, want nil", failure)
	}
	if got := GetWorkspaceID(ctx); got != "workspace_1" {
		t.Fatalf("workspace ID = %q, want %q", got, "workspace_1")
	}
	if got := GetAPIKeyID(ctx); got != "key_1" {
		t.Fatalf("API key ID = %q, want %q", got, "key_1")
	}
	if got := GetRole(ctx); got != RoleAdmin {
		t.Fatalf("role = %q, want %q", got, RoleAdmin)
	}
	if !GetAPIKeyCreatorBound(ctx) {
		t.Fatal("creator-bound marker = false, want true")
	}
}

func TestAuthenticateAPIKeyTokenPreservesStableFailures(t *testing.T) {
	tests := []struct {
		name    string
		store   *apiKeyAuthTestDB
		message string
	}{
		{
			name:    "invalid",
			store:   &apiKeyAuthTestDB{apiKeyErr: pgx.ErrNoRows},
			message: "Invalid API key",
		},
		{
			name:    "revoked",
			store:   &apiKeyAuthTestDB{revokedAt: time.Now()},
			message: "API key has been revoked",
		},
		{
			name:    "expired",
			store:   &apiKeyAuthTestDB{expiresAt: time.Now().Add(-time.Minute)},
			message: "API key has expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduledKeys := make([]string, 0, 1)
			ctx, failure := authenticateAPIKeyToken(context.Background(), db.New(tt.store), apiKeyAuthTestToken, func(keyID string) {
				scheduledKeys = append(scheduledKeys, keyID)
			})

			if ctx != nil {
				t.Fatalf("context = %#v, want nil", ctx)
			}
			if failure == nil {
				t.Fatal("failure = nil, want unauthorized failure")
			}
			if failure.Status != http.StatusUnauthorized || failure.Code != "UNAUTHORIZED" || failure.Message != tt.message {
				t.Fatalf("failure = %#v, want message %q", failure, tt.message)
			}
			if len(scheduledKeys) != 0 {
				t.Fatalf("scheduled last-used updates = %v, want none", scheduledKeys)
			}
		})
	}
}

func TestAuthenticateAPIKeyTokenUsesDetachedLastUsedContext(t *testing.T) {
	execContexts := make(chan context.Context, 1)
	execRelease := make(chan struct{})
	store := &apiKeyAuthTestDB{execContexts: execContexts, execRelease: execRelease}
	requestContext, cancel := context.WithCancel(context.Background())

	ctx, failure := AuthenticateAPIKeyToken(requestContext, db.New(store), apiKeyAuthTestToken)
	cancel()

	if failure != nil || ctx == nil {
		t.Fatalf("authentication result = (%#v, %#v), want success", ctx, failure)
	}
	select {
	case updateContext := <-execContexts:
		if err := updateContext.Err(); err != nil {
			t.Fatalf("last-used context error = %v, want detached context", err)
		}
		deadline, ok := updateContext.Deadline()
		if !ok {
			t.Fatal("last-used context has no deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Minute {
			t.Fatalf("last-used context deadline remaining = %v, want a small positive bound", remaining)
		}
		close(execRelease)
		select {
		case <-updateContext.Done():
			if !errors.Is(updateContext.Err(), context.Canceled) {
				t.Fatalf("last-used context error after update = %v, want cancellation", updateContext.Err())
			}
		case <-time.After(time.Second):
			t.Fatal("last-used context was not canceled after update returned")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for last-used update")
	}
}

func TestAPIKeyMiddlewaresUseSharedAuthenticationFailure(t *testing.T) {
	middlewares := []struct {
		name       string
		middleware func(*db.Queries) func(http.Handler) http.Handler
	}{
		{name: "DualAuth", middleware: DualAuthMiddleware},
		{name: "Unkey", middleware: APIKeyMiddleware},
	}

	for _, tt := range middlewares {
		t.Run(tt.name, func(t *testing.T) {
			store := &apiKeyAuthTestDB{apiKeyErr: pgx.ErrNoRows}
			handler := tt.middleware(db.New(store))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("next handler called for invalid API key")
			}))
			req := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
			req.Header.Set("Authorization", "Bearer "+apiKeyAuthTestToken)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"error":{"code":"UNAUTHORIZED","message":"Invalid API key"}}` {
				t.Fatalf("body = %s, want stable JSON error", got)
			}
		})
	}
}

func TestAuthenticateAPIKeyTokenLegacyCreatorlessOwner(t *testing.T) {
	store := &apiKeyAuthTestDB{creatorUserID: ""}

	ctx, failure := AuthenticateAPIKeyToken(context.Background(), db.New(store), apiKeyAuthTestToken)

	if failure != nil {
		t.Fatalf("AuthenticateAPIKeyToken failure = %#v, want nil", failure)
	}
	if got := GetRole(ctx); got != RoleOwner {
		t.Fatalf("role = %q, want %q", got, RoleOwner)
	}
	if GetAPIKeyCreatorBound(ctx) {
		t.Fatal("creator-bound marker = true, want false")
	}
	if store.membershipQueries != 0 {
		t.Fatalf("membership queries = %d, want 0", store.membershipQueries)
	}
}

func TestAuthenticateAPIKeyTokenRejectsMissingOrInactiveCreatorMembership(t *testing.T) {
	tests := []struct {
		name             string
		membershipStatus string
		membershipErr    error
	}{
		{name: "membership missing", membershipErr: pgx.ErrNoRows},
		{name: "membership inactive", membershipStatus: "inactive"},
		{name: "membership lookup error", membershipErr: errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &apiKeyAuthTestDB{
				membershipRole:   RoleAdmin,
				membershipStatus: tt.membershipStatus,
				membershipErr:    tt.membershipErr,
			}
			scheduledKeys := make([]string, 0, 1)

			ctx, failure := authenticateAPIKeyToken(context.Background(), db.New(store), apiKeyAuthTestToken, func(keyID string) {
				scheduledKeys = append(scheduledKeys, keyID)
			})

			if ctx != nil {
				t.Fatalf("context = %#v, want nil", ctx)
			}
			if failure == nil {
				t.Fatal("failure = nil, want unauthorized failure")
			}
			if failure.Status != 401 || failure.Code != "UNAUTHORIZED" || failure.Message != "API key is no longer authorized" {
				t.Fatalf("failure = %#v, want stable unauthorized response", failure)
			}
			if len(scheduledKeys) != 0 {
				t.Fatalf("scheduled last-used updates = %v, want none", scheduledKeys)
			}
		})
	}
}

func TestAuthenticateClerkTokenUsesCurrentActiveMembership(t *testing.T) {
	store := &clerkTokenAuthTestDB{}

	ctx, failure := authenticateClerkToken(context.Background(), db.New(store), "clerk-token", func(_ context.Context, token string) (string, error) {
		if token != "clerk-token" {
			t.Fatalf("token = %q, want injected Clerk token", token)
		}
		return "user_1", nil
	})

	if failure != nil {
		t.Fatalf("authenticateClerkToken failure = %#v, want nil", failure)
	}
	if got := GetUserID(ctx); got != "user_1" {
		t.Fatalf("user ID = %q, want user_1", got)
	}
	if got := GetWorkspaceID(ctx); got != "workspace_1" {
		t.Fatalf("workspace ID = %q, want workspace_1", got)
	}
	if got := GetRole(ctx); got != RoleAdmin {
		t.Fatalf("role = %q, want admin", got)
	}
}

func TestAuthenticateClerkTokenPreservesMembershipSelfHeal(t *testing.T) {
	store := &clerkTokenAuthTestDB{selfHeal: true}

	ctx, failure := authenticateClerkToken(context.Background(), db.New(store), "clerk-token", func(context.Context, string) (string, error) {
		return "user_1", nil
	})

	if failure != nil {
		t.Fatalf("authenticateClerkToken failure = %#v, want nil", failure)
	}
	if got := GetWorkspaceID(ctx); got != "workspace_1" {
		t.Fatalf("workspace ID = %q, want workspace_1", got)
	}
	if got := GetRole(ctx); got != RoleOwner {
		t.Fatalf("role = %q, want owner", got)
	}
	if store.activeMembershipQueries != 2 {
		t.Fatalf("active membership queries = %d, want 2", store.activeMembershipQueries)
	}
	if store.workspaceQueries != 1 || store.membershipCreates != 1 {
		t.Fatalf("workspace queries/membership creates = %d/%d, want 1/1", store.workspaceQueries, store.membershipCreates)
	}
}

func TestAuthenticateClerkTokenReturnsStructuredFailures(t *testing.T) {
	tests := []struct {
		name       string
		store      *clerkTokenAuthTestDB
		verify     func(context.Context, string) (string, error)
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "invalid JWT",
			store:      &clerkTokenAuthTestDB{},
			verify:     func(context.Context, string) (string, error) { return "", errors.New("invalid") },
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
			wantMsg:    "Invalid session token",
		},
		{
			name:       "no workspace",
			store:      &clerkTokenAuthTestDB{noMembership: true},
			verify:     func(context.Context, string) (string, error) { return "user_1", nil },
			wantStatus: http.StatusForbidden,
			wantCode:   "NO_WORKSPACE",
			wantMsg:    "No workspace exists for this user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, failure := authenticateClerkToken(context.Background(), db.New(tt.store), "clerk-token", tt.verify)

			if ctx != nil {
				t.Fatalf("context = %#v, want nil", ctx)
			}
			if failure == nil || failure.Status != tt.wantStatus || failure.Code != tt.wantCode || failure.Message != tt.wantMsg {
				t.Fatalf("failure = %#v, want status=%d code=%q message=%q", failure, tt.wantStatus, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

type clerkTokenAuthTestDB struct {
	selfHeal                bool
	noMembership            bool
	activeMembershipQueries int
	workspaceQueries        int
	membershipCreates       int
}

func (*clerkTokenAuthTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *clerkTokenAuthTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if !strings.Contains(query, "-- name: ListWorkspacesByUser") {
		return nil, errors.New("unexpected Query")
	}
	f.workspaceQueries++
	if f.noMembership {
		return &clerkTokenAuthTestRows{}, nil
	}
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return &clerkTokenAuthTestRows{rows: [][]any{{
		"workspace_1", "user_1", "Workspace", pgtype.Int4{}, now, now, []string{"direct"}, pgtype.Text{},
	}}}, nil
}

func (f *clerkTokenAuthTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	role := RoleAdmin
	if f.selfHeal && f.activeMembershipQueries > 0 {
		role = RoleOwner
	}
	membership := []any{
		"workspace_1", "user_1", role, "active", pgtype.Text{}, now, now, now, now,
	}
	switch {
	case strings.Contains(query, "-- name: GetActiveMembership"):
		f.activeMembershipQueries++
		if f.noMembership || (f.selfHeal && f.activeMembershipQueries == 1) {
			return clerkTokenAuthTestRow{err: pgx.ErrNoRows}
		}
		return clerkTokenAuthTestRow{values: membership}
	case strings.Contains(query, "-- name: CreateMembership"):
		f.membershipCreates++
		return clerkTokenAuthTestRow{values: membership}
	default:
		return clerkTokenAuthTestRow{err: errors.New("unexpected QueryRow")}
	}
}

type clerkTokenAuthTestRows struct {
	rows   [][]any
	index  int
	closed bool
}

func (r *clerkTokenAuthTestRows) Close()                                       { r.closed = true }
func (r *clerkTokenAuthTestRows) Err() error                                   { return nil }
func (r *clerkTokenAuthTestRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *clerkTokenAuthTestRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *clerkTokenAuthTestRows) Next() bool {
	if r.index >= len(r.rows) {
		r.closed = true
		return false
	}
	r.index++
	return true
}
func (r *clerkTokenAuthTestRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return assignClerkTokenAuthTestValues(dest, r.rows[r.index-1])
}
func (r *clerkTokenAuthTestRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.rows) {
		return nil, errors.New("Values called without current row")
	}
	return r.rows[r.index-1], nil
}
func (*clerkTokenAuthTestRows) RawValues() [][]byte { return nil }
func (*clerkTokenAuthTestRows) Conn() *pgx.Conn     { return nil }

type clerkTokenAuthTestRow struct {
	values []any
	err    error
}

func (r clerkTokenAuthTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignClerkTokenAuthTestValues(dest, r.values)
}

func assignClerkTokenAuthTestValues(dest, values []any) error {
	if len(dest) != len(values) {
		return errors.New("unexpected scan destination count")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = values[i].(string)
		case *pgtype.Text:
			*target = values[i].(pgtype.Text)
		case *pgtype.Int4:
			*target = values[i].(pgtype.Int4)
		case *pgtype.Timestamptz:
			*target = values[i].(pgtype.Timestamptz)
		case *[]string:
			*target = values[i].([]string)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
