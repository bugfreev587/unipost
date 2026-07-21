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
