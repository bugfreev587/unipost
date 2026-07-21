package auth

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
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
				ctx, failure := AuthenticateAPIKeyToken(r.Context(), queries, token)
				if failure != nil {
					writeJSON(w, failure.Status, map[string]any{
						"error": map[string]any{"code": failure.Code, "message": failure.Message},
					})
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
			// Self-heal: a regression in the RBAC migration 060 rollout
			// (May 2026) and a webhook delivery race could both leave a
			// user with a workspace row but no workspace_members row,
			// blocking every authenticated request with NO_WORKSPACE.
			// If a workspace exists for this user, grant them owner
			// membership now and proceed; otherwise fall through to the
			// genuine NO_WORKSPACE error.
			workspaces, wsErr := queries.ListWorkspacesByUser(ctx, claims.Subject)
			if wsErr == nil && len(workspaces) > 0 {
				ws := workspaces[0]
				// Best-effort. If the row was just inserted by another
				// concurrent request, the unique key will reject this
				// one and the follow-up GetActiveMembership succeeds
				// for both callers.
				_, _ = queries.CreateMembership(ctx, db.CreateMembershipParams{
					WorkspaceID: ws.ID,
					UserID:      claims.Subject,
					Role:        "owner",
				})
				slog.Warn("auth self-heal: created missing owner membership", "user_id", claims.Subject, "workspace_id", ws.ID)
				mem, err = queries.GetActiveMembership(ctx, claims.Subject)
			}
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
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]any{"code": "INTERNAL_ERROR", "message": "Failed to resolve workspace"},
			})
			return nil, false
		}
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

func SetAPIKeyID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, APIKeyIDKey, id)
}
