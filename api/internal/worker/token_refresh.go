package worker

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// TokenRefreshWorker refreshes expiring social account tokens in the background.
type TokenRefreshWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewTokenRefreshWorker(queries *db.Queries, encryptor *crypto.AESEncryptor) *TokenRefreshWorker {
	return &TokenRefreshWorker{queries: queries, encryptor: encryptor}
}

// Start runs the token refresh loop every 15 minutes until ctx is cancelled.
func (w *TokenRefreshWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	log.Println("Token refresh worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Token refresh worker stopped")
			return
		case <-ticker.C:
			w.refreshExpiring(ctx)
		}
	}
}

func (w *TokenRefreshWorker) refreshExpiring(ctx context.Context) {
	accounts, err := w.queries.GetExpiringTokens(ctx)
	if err != nil {
		log.Printf("token refresh: failed to query expiring tokens: %v", err)
		return
	}

	if len(accounts) == 0 {
		return
	}

	log.Printf("token refresh: found %d accounts with expiring tokens", len(accounts))

	for _, acc := range accounts {
		adapter, err := platform.Get(acc.Platform)
		if err != nil {
			log.Printf("token refresh: unsupported platform %s for account %s", acc.Platform, acc.ID)
			continue
		}

		if !acc.RefreshToken.Valid {
			log.Printf("token refresh: no refresh token for account %s", acc.ID)
			continue
		}

		refreshToken, err := w.encryptor.Decrypt(acc.RefreshToken.String)
		if err != nil {
			log.Printf("token refresh: failed to decrypt refresh token for account %s: %v", acc.ID, err)
			continue
		}

		newAccess, newRefresh, expiresAt, err := adapter.RefreshToken(ctx, refreshToken)
		if err != nil {
			log.Printf("token refresh: failed to refresh token for account %s: %v", acc.ID, err)
			continue
		}

		encAccess, err := w.encryptor.Encrypt(newAccess)
		if err != nil {
			log.Printf("token refresh: failed to encrypt new access token for account %s: %v", acc.ID, err)
			continue
		}

		encRefresh, err := w.encryptor.Encrypt(newRefresh)
		if err != nil {
			log.Printf("token refresh: failed to encrypt new refresh token for account %s: %v", acc.ID, err)
			continue
		}

		err = w.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
			ID:             acc.ID,
			AccessToken:    encAccess,
			RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			log.Printf("token refresh: failed to update tokens for account %s: %v", acc.ID, err)
			continue
		}

		log.Printf("token refresh: refreshed token for account %s (%s)", acc.ID, acc.Platform)
	}
}
