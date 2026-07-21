package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// DualAuthMiddleware accepts either a workspace API key or a Clerk session
// JWT in the Authorization: Bearer <token> header. Tokens that start with
// the API-key prefix run the API-key path; everything else is treated as a
// Clerk JWT. Both paths populate workspaceID in the request context; the
// Clerk path additionally populates userID and resolves workspaceID by
// looking up the user's current active workspace membership.
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
	ctx, failure := AuthenticateClerkToken(r.Context(), queries, token)
	if failure != nil {
		writeJSON(w, failure.Status, map[string]any{
			"error": map[string]any{"code": failure.Code, "message": failure.Message},
		})
		return nil, false
	}
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
