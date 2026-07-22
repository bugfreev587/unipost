package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

const WorkspaceIDKey contextKey = "workspaceID"
const RoleKey contextKey = "role"

// Role constants — match plans.workspace_members.role CHECK values.
// RoleLevel uses these for >= comparisons in the RequireRole middleware.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleEditor = "editor"
)

// RoleLevel returns a numeric level for the given role string so role
// gates can compare with `>=`. Unknown / empty roles get level 0 (the
// "no role" floor) which fails any RequireRole(min) check unless the
// minimum is also 0.
//
// To add a viewer role later: add the constant + a new case here +
// relax the workspace_members CHECK constraint. Existing call sites
// keep working because they use named constants, not numeric levels.
func RoleLevel(role string) int {
	switch role {
	case RoleOwner:
		return 3
	case RoleAdmin:
		return 2
	case RoleEditor:
		return 1
	default:
		return 0
	}
}

// SetRole / GetRole mirror the WorkspaceID context plumbing. Auth
// middleware stamps the role at the same time it stamps workspace_id;
// downstream RequireRole middleware reads it back.
func SetRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, RoleKey, role)
}

func GetRole(ctx context.Context) string {
	role, _ := ctx.Value(RoleKey).(string)
	return role
}

// APIKeyMiddleware verifies API keys by hashing the presented key and looking up
// the hash in the database. It checks revocation, expiration, and creator membership,
// updates last_used_at in a background goroutine, and injects API-key auth context.
func APIKeyMiddleware(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Missing API key",
					},
				})
				return
			}

			key := strings.TrimPrefix(authHeader, "Bearer ")
			if key == authHeader {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Invalid authorization format",
					},
				})
				return
			}

			ctx, failure := AuthenticateAPIKeyToken(r.Context(), queries, key)
			if failure != nil {
				writeJSON(w, failure.Status, map[string]any{
					"error": map[string]any{"code": failure.Code, "message": failure.Message},
				})
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetWorkspaceID(ctx context.Context) string {
	workspaceID, _ := ctx.Value(WorkspaceIDKey).(string)
	return workspaceID
}

// SetWorkspaceID injects a workspace ID into the context. Used by
// dashboard routes that pass workspace ID via URL parameter instead
// of API key resolution.
func SetWorkspaceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, WorkspaceIDKey, id)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
