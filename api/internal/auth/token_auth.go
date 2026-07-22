package auth

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type TokenAuthFailure struct {
	Status  int
	Code    string
	Message string
}

const apiKeyCreatorBoundKey contextKey = "apiKeyCreatorBound"

const apiKeyLastUsedUpdateTimeout = 5 * time.Second

type clerkTokenVerifier func(context.Context, string) (string, error)

func GetAPIKeyCreatorBound(ctx context.Context) bool {
	value, _ := ctx.Value(apiKeyCreatorBoundKey).(bool)
	return value
}

func SetAPIKeyCreatorBound(ctx context.Context, bound bool) context.Context {
	return context.WithValue(ctx, apiKeyCreatorBoundKey, bound)
}

// AuthenticateClerkToken verifies a Clerk session token and stamps the same
// current active workspace membership used by DualAuthMiddleware. It returns a
// structured failure so non-HTTP middleware, including WebSocket handshakes,
// can preserve the API's status/code/message contract without writing early.
func AuthenticateClerkToken(ctx context.Context, queries *db.Queries, token string) (context.Context, *TokenAuthFailure) {
	return authenticateClerkToken(ctx, queries, token, VerifyClerkSessionToken)
}

func authenticateClerkToken(ctx context.Context, queries *db.Queries, token string, verify clerkTokenVerifier) (context.Context, *TokenAuthFailure) {
	userID, err := verify(ctx, token)
	if err != nil {
		return nil, &TokenAuthFailure{
			Status:  http.StatusUnauthorized,
			Code:    "UNAUTHORIZED",
			Message: "Invalid session token",
		}
	}

	authenticated := context.WithValue(ctx, UserIDKey, userID)
	mem, err := queries.GetActiveMembership(authenticated, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Preserve the HTTP auth self-heal for users affected by the RBAC
			// migration/webhook race: if their owned workspace still exists,
			// recreate the missing owner membership and resolve it again.
			workspaces, workspaceErr := queries.ListWorkspacesByUser(authenticated, userID)
			if workspaceErr == nil && len(workspaces) > 0 {
				workspace := workspaces[0]
				_, _ = queries.CreateMembership(authenticated, db.CreateMembershipParams{
					WorkspaceID: workspace.ID,
					UserID:      userID,
					Role:        RoleOwner,
				})
				slog.Warn("auth self-heal: created missing owner membership", "user_id", userID, "workspace_id", workspace.ID)
				mem, err = queries.GetActiveMembership(authenticated, userID)
			}
			if err != nil {
				if err == pgx.ErrNoRows {
					return nil, &TokenAuthFailure{
						Status:  http.StatusForbidden,
						Code:    "NO_WORKSPACE",
						Message: "No workspace exists for this user",
					}
				}
				return nil, &TokenAuthFailure{
					Status:  http.StatusInternalServerError,
					Code:    "INTERNAL_ERROR",
					Message: "Failed to resolve workspace",
				}
			}
		} else {
			return nil, &TokenAuthFailure{
				Status:  http.StatusInternalServerError,
				Code:    "INTERNAL_ERROR",
				Message: "Failed to resolve workspace",
			}
		}
	}

	authenticated = SetWorkspaceID(authenticated, mem.WorkspaceID)
	authenticated = SetRole(authenticated, mem.Role)
	return authenticated, nil
}

// VerifyClerkSessionToken verifies a Clerk JWT and returns only its subject.
// Callers remain responsible for choosing the workspace resolution behavior
// appropriate to their route.
func VerifyClerkSessionToken(ctx context.Context, token string) (string, error) {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	clerk.SetKey(secretKey)
	client := jwks.NewClient(&clerk.ClientConfig{
		BackendConfig: clerk.BackendConfig{Key: clerk.String(secretKey)},
	})
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{
		Token:      token,
		JWKSClient: client,
	})
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

func AuthenticateAPIKeyToken(ctx context.Context, queries *db.Queries, token string) (context.Context, *TokenAuthFailure) {
	return authenticateAPIKeyToken(ctx, queries, token, func(apiKeyID string) {
		scheduleAPIKeyLastUsedUpdate(queries, apiKeyID)
	})
}

func authenticateAPIKeyToken(ctx context.Context, queries *db.Queries, token string, scheduleLastUsed func(string)) (context.Context, *TokenAuthFailure) {
	ak, err := queries.GetAPIKeyByHash(ctx, apikey.Hash(token))
	if err != nil {
		return nil, unauthorizedTokenFailure("Invalid API key")
	}
	if ak.RevokedAt.Valid {
		return nil, unauthorizedTokenFailure("API key has been revoked")
	}
	if ak.ExpiresAt.Valid && ak.ExpiresAt.Time.Before(time.Now()) {
		return nil, unauthorizedTokenFailure("API key has expired")
	}

	role := RoleOwner
	creatorBound := ak.CreatedByUserID != ""
	if creatorBound {
		membership, err := queries.GetMembership(ctx, db.GetMembershipParams{
			WorkspaceID: ak.WorkspaceID,
			UserID:      ak.CreatedByUserID,
		})
		if err != nil || membership.Status != "active" {
			return nil, unauthorizedTokenFailure("API key is no longer authorized")
		}
		role = membership.Role
	}

	authenticated := SetWorkspaceID(ctx, ak.WorkspaceID)
	authenticated = SetAPIKeyID(authenticated, ak.ID)
	authenticated = SetAPIKeyCreatorBound(authenticated, creatorBound)
	authenticated = SetRole(authenticated, role)
	scheduleLastUsed(ak.ID)
	return authenticated, nil
}

func scheduleAPIKeyLastUsedUpdate(queries *db.Queries, apiKeyID string) {
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), apiKeyLastUsedUpdateTimeout)
		defer cancel()
		if err := queries.UpdateAPIKeyLastUsedAt(updateCtx, apiKeyID); err != nil {
			slog.Error("failed to update last_used_at", "key_id", apiKeyID, "error", err)
		}
	}()
}

func unauthorizedTokenFailure(message string) *TokenAuthFailure {
	return &TokenAuthFailure{
		Status:  http.StatusUnauthorized,
		Code:    "UNAUTHORIZED",
		Message: message,
	}
}
