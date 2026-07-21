package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type TokenAuthFailure struct {
	Status  int
	Code    string
	Message string
}

const apiKeyCreatorBoundKey contextKey = "apiKeyCreatorBound"

func GetAPIKeyCreatorBound(ctx context.Context) bool {
	value, _ := ctx.Value(apiKeyCreatorBoundKey).(bool)
	return value
}

func SetAPIKeyCreatorBound(ctx context.Context, bound bool) context.Context {
	return context.WithValue(ctx, apiKeyCreatorBoundKey, bound)
}

func AuthenticateAPIKeyToken(ctx context.Context, queries *db.Queries, token string) (context.Context, *TokenAuthFailure) {
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

	go func() {
		if err := queries.UpdateAPIKeyLastUsedAt(context.Background(), ak.ID); err != nil {
			slog.Error("failed to update last_used_at", "key_id", ak.ID, "error", err)
		}
	}()

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
	return authenticated, nil
}

func unauthorizedTokenFailure(message string) *TokenAuthFailure {
	return &TokenAuthFailure{
		Status:  http.StatusUnauthorized,
		Code:    "UNAUTHORIZED",
		Message: message,
	}
}
