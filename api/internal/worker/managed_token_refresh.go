// managed_token_refresh.go is the Sprint 3 PR7 token refresh worker
// for managed (Connect-flow) social_accounts rows. It runs alongside
// the existing TokenRefreshWorker (which handles BYO rows) but uses
// a separate query path because:
//
//   1. Managed rows are routed through the internal/connect package
//      (Twitter / LinkedIn) instead of the platform adapter registry,
//      since the OAuth client id / secret used to mint the original
//      tokens are env-var-scoped to UniPost's own apps.
//
//   2. The query uses FOR UPDATE SKIP LOCKED so two API instances
//      doing simultaneous ticks pick disjoint slices and never
//      double-refresh a row. The BYO worker's lighter-weight query
//      doesn't need this because BYO refreshes are idempotent on
//      the platform side.
//
//   3. Per Sprint 3 founder decision #5, the success path is
//      INTENTIONALLY SILENT — no account.refreshed webhook is fired.
//      Customers don't want a webhook every 2 hours per Twitter
//      account. Failures still fire account.disconnected so the
//      customer can prompt the user to reconnect.
//
// Per Sprint 3 founder decision #3, refresh failure flips
// status='reconnect_required' (reusing the existing enum value)
// rather than introducing a new 'token_expired' status.

package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

// ManagedTokenRefreshWorker refreshes managed Connect tokens that
// are within 30 minutes of expiry. Tick is every 5 minutes per
// Sprint 3 PRD §3 W6.
type ManagedTokenRefreshWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	registry  *connect.Registry
	bus       events.EventBus

	// tickInterval is variable for tests; production uses 5 min.
	tickInterval time.Duration
}

func NewManagedTokenRefreshWorker(queries *db.Queries, encryptor *crypto.AESEncryptor, registry *connect.Registry, bus events.EventBus) *ManagedTokenRefreshWorker {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &ManagedTokenRefreshWorker{
		queries:      queries,
		encryptor:    encryptor,
		registry:     registry,
		bus:          bus,
		tickInterval: 5 * time.Minute,
	}
}

// Start runs the worker loop until ctx is cancelled.
func (w *ManagedTokenRefreshWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()
	slog.Info("managed token refresh worker started", "interval", w.tickInterval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("managed token refresh worker stopped")
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce processes one batch. Exposed for tests / integration runs
// that want to advance the worker without waiting for the ticker.
func (w *ManagedTokenRefreshWorker) RunOnce(ctx context.Context) {
	rows, err := w.queries.ListManagedAccountsDueForRefresh(ctx)
	if err != nil {
		slog.Error("managed token refresh: query failed", "err", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	slog.Info("managed token refresh: batch", "count", len(rows))

	for _, acc := range rows {
		w.refreshOne(ctx, acc)
	}
}

func (w *ManagedTokenRefreshWorker) refreshOne(ctx context.Context, acc db.SocialAccount) {
	connector, ok := w.registry.Get(acc.Platform)
	if !ok {
		// Should never happen — the query already filters out bluesky,
		// and the registry should always carry twitter + linkedin in
		// production. Skip rather than crash.
		slog.Warn("managed token refresh: no connector", "platform", acc.Platform, "account_id", acc.ID)
		return
	}
	if !acc.RefreshToken.Valid || acc.RefreshToken.String == "" {
		slog.Warn("managed token refresh: missing refresh token", "account_id", acc.ID)
		w.markReconnectRequired(ctx, acc, "missing_refresh_token")
		return
	}

	refreshTok, err := w.encryptor.Decrypt(acc.RefreshToken.String)
	if err != nil {
		slog.Error("managed token refresh: decrypt failed", "account_id", acc.ID, "err", err)
		w.markReconnectRequired(ctx, acc, "decrypt_failed")
		return
	}

	tokens, err := connector.Refresh(ctx, refreshTok)
	if err != nil {
		slog.Warn("managed token refresh: platform refused", "account_id", acc.ID, "platform", acc.Platform, "err", err)
		w.markReconnectRequired(ctx, acc, "refresh_failed")
		return
	}

	encAccess, err := w.encryptor.Encrypt(tokens.AccessToken)
	if err != nil {
		slog.Error("managed token refresh: encrypt access failed", "account_id", acc.ID, "err", err)
		return
	}

	// LinkedIn-style "no rotation" path: empty RefreshToken means
	// "keep the existing one". Twitter rotates → use the new one.
	encRefresh := acc.RefreshToken
	if tokens.RefreshToken != "" {
		enc, err := w.encryptor.Encrypt(tokens.RefreshToken)
		if err != nil {
			slog.Error("managed token refresh: encrypt refresh failed", "account_id", acc.ID, "err", err)
			return
		}
		encRefresh = pgtype.Text{String: enc, Valid: true}
	}

	if err := w.queries.UpdateManagedTokenRefresh(ctx, db.UpdateManagedTokenRefreshParams{
		ID:             acc.ID,
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		TokenExpiresAt: pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: !tokens.ExpiresAt.IsZero()},
	}); err != nil {
		slog.Error("managed token refresh: update failed", "account_id", acc.ID, "err", err)
		return
	}

	slog.Info("managed token refresh: success",
		"account_id", acc.ID,
		"platform", acc.Platform,
		"new_expiry", tokens.ExpiresAt.Format(time.RFC3339),
	)
	// Per Sprint 3 decision #5: NO webhook on the success path.
	// Customers don't want a steady stream of account.refreshed events
	// every 2 hours per Twitter account.
}

func (w *ManagedTokenRefreshWorker) markReconnectRequired(ctx context.Context, acc db.SocialAccount, reason string) {
	if err := w.queries.MarkSocialAccountReconnectRequired(ctx, acc.ID); err != nil {
		slog.Error("managed token refresh: mark reconnect_required failed", "account_id", acc.ID, "err", err)
		return
	}

	accountName := ""
	if acc.AccountName.Valid {
		accountName = acc.AccountName.String
	}
	externalUserID := ""
	if acc.ExternalUserID.Valid {
		externalUserID = acc.ExternalUserID.String
	}
	// Webhooks are workspace-scoped; look up workspace_id from the profile.
	workspaceID := acc.ProfileID // fallback (won't match webhooks, but is safe)
	if profile, err := w.queries.GetProfile(ctx, acc.ProfileID); err == nil {
		workspaceID = profile.WorkspaceID
	}
	w.bus.Publish(ctx, workspaceID, events.EventAccountDisconnected, map[string]any{
		"social_account_id": acc.ID,
		"platform":          acc.Platform,
		"account_name":      accountName,
		"external_user_id":  externalUserID,
		"connection_type":   "managed",
		"disconnected_at":   time.Now().UTC().Format(time.RFC3339),
		"reason":            reason,
	})
}
