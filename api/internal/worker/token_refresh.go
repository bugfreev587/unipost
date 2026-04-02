package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type TokenRefreshWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewTokenRefreshWorker(queries *db.Queries, encryptor *crypto.AESEncryptor) *TokenRefreshWorker {
	return &TokenRefreshWorker{queries: queries, encryptor: encryptor}
}

func (w *TokenRefreshWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	slog.Info("token refresh worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("token refresh worker stopped")
			return
		case <-ticker.C:
			w.refreshExpiring(ctx)
		}
	}
}

func (w *TokenRefreshWorker) refreshExpiring(ctx context.Context) {
	accounts, err := w.queries.GetExpiringTokens(ctx)
	if err != nil {
		slog.Error("token refresh: failed to query expiring tokens", "error", err)
		return
	}

	if len(accounts) == 0 {
		return
	}

	slog.Info("token refresh: found expiring tokens", "count", len(accounts))

	for _, acc := range accounts {
		adapter, err := platform.Get(acc.Platform)
		if err != nil {
			slog.Warn("token refresh: unsupported platform", "platform", acc.Platform, "account_id", acc.ID)
			continue
		}

		if !acc.RefreshToken.Valid {
			slog.Warn("token refresh: no refresh token", "account_id", acc.ID)
			continue
		}

		refreshToken, err := w.encryptor.Decrypt(acc.RefreshToken.String)
		if err != nil {
			slog.Error("token refresh: failed to decrypt refresh token", "account_id", acc.ID, "error", err)
			continue
		}

		newAccess, newRefresh, expiresAt, err := adapter.RefreshToken(ctx, refreshToken)
		if err != nil {
			slog.Error("token refresh: failed to refresh", "account_id", acc.ID, "error", err)
			continue
		}

		encAccess, err := w.encryptor.Encrypt(newAccess)
		if err != nil {
			slog.Error("token refresh: failed to encrypt access token", "account_id", acc.ID, "error", err)
			continue
		}

		encRefresh, err := w.encryptor.Encrypt(newRefresh)
		if err != nil {
			slog.Error("token refresh: failed to encrypt refresh token", "account_id", acc.ID, "error", err)
			continue
		}

		err = w.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
			ID:             acc.ID,
			AccessToken:    encAccess,
			RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			slog.Error("token refresh: failed to update tokens", "account_id", acc.ID, "error", err)
			continue
		}

		slog.Info("token refresh: refreshed", "account_id", acc.ID, "platform", acc.Platform)
	}
}
