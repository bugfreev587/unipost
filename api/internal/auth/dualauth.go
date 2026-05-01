package auth

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// DualAuthMiddleware accepts either a workspace API key or a Clerk session
// JWT in the Authorization: Bearer <token> header. Tokens that start with
// the API-key prefix run the API-key path; everything else is treated as a
// Clerk JWT. Both paths populate workspaceID in the request context; the
// Clerk path additionally populates userID and resolves workspaceID by
// looking up the user's default workspace.
func DualAuthMiddleware(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Missing authorization header",
					},
				})
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Invalid authorization format",
					},
				})
				return
			}

			if strings.HasPrefix(token, apikey.PrefixLive) || strings.HasPrefix(token, apikey.PrefixTest) {
				ctx, ok := authenticateAPIKey(w, r, queries, token)
				if !ok {
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			ctx, ok := authenticateClerk(w, r, queries, token)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authenticateAPIKey(w http.ResponseWriter, r *http.Request, queries *db.Queries, token string) (context.Context, bool) {
	hash := apikey.Hash(token)
	ak, err := queries.GetAPIKeyByHash(r.Context(), hash)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"code": "UNAUTHORIZED", "message": "Invalid API key"},
		})
		return nil, false
	}
	if ak.RevokedAt.Valid {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"code": "UNAUTHORIZED", "message": "API key has been revoked"},
		})
		return nil, false
	}
	if ak.ExpiresAt.Valid && ak.ExpiresAt.Time.Before(time.Now()) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"code": "UNAUTHORIZED", "message": "API key has expired"},
		})
		return nil, false
	}
	go func() {
		if err := queries.UpdateAPIKeyLastUsedAt(context.Background(), ak.ID); err != nil {
			slog.Error("failed to update last_used_at", "key_id", ak.ID, "error", err)
		}
	}()
	ctx := context.WithValue(r.Context(), WorkspaceIDKey, ak.WorkspaceID)
	ctx = context.WithValue(ctx, APIKeyIDKey, ak.ID)
	// RBAC Phase 2 (May 2026): stamp owner role for API-key-auth
	// requests. Today the api_keys table doesn't track which user
	// created the key, and every key was implicitly created by the
	// workspace owner (1 user per workspace pre-invite-flow). When
	// the invite flow ships and api_keys gains a created_by_user_id
	// column, this becomes a real membership lookup; until then,
	// API keys universally carry owner privileges. Documenting the
	// assumption here so the security model is auditable.
	ctx = context.WithValue(ctx, RoleKey, RoleOwner)
	return ctx, true
}

func authenticateClerk(w http.ResponseWriter, r *http.Request, queries *db.Queries, token string) (context.Context, bool) {
	clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
	client := jwks.NewClient(&clerk.ClientConfig{
		BackendConfig: clerk.BackendConfig{Key: clerk.String(os.Getenv("CLERK_SECRET_KEY"))},
	})
	claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
		Token:      token,
		JWKSClient: client,
	})
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"code": "UNAUTHORIZED", "message": "Invalid session token"},
		})
		return nil, false
	}
	ctx := context.WithValue(r.Context(), UserIDKey, claims.Subject)

	// RBAC Phase 2 (May 2026): resolve workspace + role from the
	// membership table instead of the legacy "default workspace
	// for this user" lookup. The migration 060 backfill ensures
	// every existing user has an active 'owner' membership for
	// their workspace, so behavior is unchanged for existing users
	// — but additional members invited later will resolve to their
	// invited role automatically.
	mem, err := queries.GetActiveMembership(ctx, claims.Subject)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": map[string]any{"code": "NO_WORKSPACE", "message": "No workspace exists for this user"},
			})
			return nil, false
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"code": "INTERNAL_ERROR", "message": "Failed to resolve workspace"},
		})
		return nil, false
	}
	ctx = context.WithValue(ctx, WorkspaceIDKey, mem.WorkspaceID)
	ctx = context.WithValue(ctx, RoleKey, mem.Role)
	return ctx, true
}

const APIKeyIDKey contextKey = "apiKeyID"

func GetAPIKeyID(ctx context.Context) string {
	id, _ := ctx.Value(APIKeyIDKey).(string)
	return id
}
