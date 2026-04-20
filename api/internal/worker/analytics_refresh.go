package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// AnalyticsRefreshWorker periodically refreshes cached metrics for published
// social_post_results so the dashboard can serve analytics from the
// post_analytics table without ever blocking on platform APIs.
//
// Refresh policy is tier-based and lives in the SQL query
// (GetDuePostAnalyticsRefresh) so the worker only loops over rows that
// actually need to be refreshed:
//
//   - new posts (< 24h)  → 1 hour TTL
//   - recent posts (1–7d)  → 6 hour TTL
//   - old posts (> 7d)   → 24 hour TTL
//
// Posts older than 90 days are excluded entirely.
//
// Sprint 2 also folds the abandoned-media sweeper into this same
// hourly tick — both are "periodic cleanup" jobs and we don't need
// a second goroutine for the handful of media rows that get garbage-
// collected each hour. The sweeper runs after the analytics refresh
// so it doesn't compete with the platform API calls for the same
// burst of CPU.
type AnalyticsRefreshWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	storage   *storage.Client // optional; nil disables the media sweeper half
}

func NewAnalyticsRefreshWorker(queries *db.Queries, encryptor *crypto.AESEncryptor, store *storage.Client) *AnalyticsRefreshWorker {
	return &AnalyticsRefreshWorker{queries: queries, encryptor: encryptor, storage: store}
}

// analyticsRefreshConcurrency caps in-flight platform API calls per tick.
// Tuned to stay well under per-platform rate limits even when the worker
// catches up after a long downtime.
const analyticsRefreshConcurrency = 5

func (w *AnalyticsRefreshWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	slog.Info("analytics refresh worker started")

	// Run once on startup so a freshly-deployed instance doesn't sit idle for
	// an hour before backfilling.
	w.refreshDue(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("analytics refresh worker stopped")
			return
		case <-ticker.C:
			w.refreshDue(ctx)
			w.sweepAbandonedMedia(ctx)
		}
	}
}

// sweepAbandonedMedia hard-deletes media rows in `pending` status
// older than 7 days, plus the matching R2 objects. Pending = the
// client called POST /v1/media but never followed up with the
// presigned PUT (or did, but never referenced the media_id in a
// publish call). Worth keeping the cleanup conservative — 7 days is
// long enough that flaky-network retries don't hit it.
//
// No-op when storage is nil so a server without R2 configured (or a
// test env) doesn't trip on a missing client.
func (w *AnalyticsRefreshWorker) sweepAbandonedMedia(ctx context.Context) {
	if w.storage == nil {
		return
	}
	rows, err := w.queries.ListAbandonedMedia(ctx)
	if err != nil {
		slog.Error("media sweeper: failed to list abandoned rows", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	slog.Info("media sweeper: cleaning abandoned uploads", "count", len(rows))

	for _, row := range rows {
		// Best-effort R2 delete first; if R2 already lost it (404),
		// the storage layer treats it as success.
		if err := w.storage.Delete(ctx, row.StorageKey); err != nil {
			slog.Warn("media sweeper: r2 delete failed", "media_id", row.ID, "error", err)
			// Don't drop the row — try again next tick.
			continue
		}
		if err := w.queries.HardDeleteMedia(ctx, row.ID); err != nil {
			slog.Warn("media sweeper: db delete failed", "media_id", row.ID, "error", err)
		}
	}
}

func (w *AnalyticsRefreshWorker) refreshDue(ctx context.Context) {
	rows, err := w.queries.GetDuePostAnalyticsRefresh(ctx)
	if err != nil {
		slog.Error("analytics refresh: failed to query due rows", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}

	slog.Info("analytics refresh: processing due rows", "count", len(rows))

	sem := make(chan struct{}, analyticsRefreshConcurrency)
	var wg sync.WaitGroup
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(r db.GetDuePostAnalyticsRefreshRow) {
			defer wg.Done()
			defer func() { <-sem }()
			w.refreshOne(ctx, r)
		}(row)
	}
	wg.Wait()
}

func (w *AnalyticsRefreshWorker) refreshOne(ctx context.Context, r db.GetDuePostAnalyticsRefreshRow) {
	adapter, err := platform.Get(r.Platform)
	if err != nil {
		slog.Warn("analytics refresh: unknown platform", "platform", r.Platform)
		return
	}
	analyticsAdapter, ok := adapter.(platform.AnalyticsAdapter)
	if !ok {
		// Platform doesn't implement analytics — nothing to do.
		return
	}

	accessToken, err := w.encryptor.Decrypt(r.AccessToken)
	if err != nil {
		slog.Warn("analytics refresh: failed to decrypt token", "result_id", r.SocialPostResultID, "error", err)
		return
	}

	// Refresh the OAuth token inline if expired. The TokenRefreshWorker also
	// handles this on its own schedule, but doing it here closes the race when
	// a token expires between the two workers' ticks.
	if r.TokenExpiresAt.Valid && r.TokenExpiresAt.Time.Before(time.Now()) && r.RefreshToken.Valid {
		refreshToken, decErr := w.encryptor.Decrypt(r.RefreshToken.String)
		if decErr == nil {
			newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(ctx, refreshToken)
			if refErr != nil {
				slog.Warn("analytics refresh: token refresh failed",
					"result_id", r.SocialPostResultID, "platform", r.Platform, "error", refErr)
				return
			}
			if newAccess == "" {
				slog.Warn("analytics refresh: upstream returned empty access token; skipping DB update",
					"result_id", r.SocialPostResultID, "platform", r.Platform)
				return
			}
			encAccess, encErr := w.encryptor.Encrypt(newAccess)
			encRefresh, encErr2 := w.encryptor.Encrypt(newRefresh)
			if encErr != nil || encErr2 != nil {
				slog.Error("analytics refresh: encrypt failed",
					"result_id", r.SocialPostResultID, "platform", r.Platform,
					"access_err", encErr, "refresh_err", encErr2)
				return
			}
			accessToken = newAccess
			if updateErr := w.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
				ID:             r.SocialAccountID,
				AccessToken:    encAccess,
				RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
				TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
			}); updateErr != nil {
				slog.Error("analytics refresh: update failed",
					"result_id", r.SocialPostResultID, "platform", r.Platform, "error", updateErr)
			}
		}
	}

	if !r.ExternalID.Valid {
		return
	}

	metrics, metErr := analyticsAdapter.GetAnalytics(ctx, accessToken, r.ExternalID.String)
	if metErr != nil {
		slog.Warn("analytics refresh: GetAnalytics failed",
			"result_id", r.SocialPostResultID, "platform", r.Platform, "error", metErr)
		// Touch fetched_at so the tier-based TTL applies and we don't
		// hammer a permanently-gone post every hour. Uses a separate
		// query that only bumps fetched_at + increments the failure
		// counter, preserving any real metrics from a prior successful
		// fetch (important for transient errors).
		_ = w.queries.TouchPostAnalyticsFetchedAt(ctx, db.TouchPostAnalyticsFetchedAtParams{
			SocialPostResultID: r.SocialPostResultID,
			LastFailureReason:  pgtype.Text{String: metErr.Error(), Valid: true},
		})
		return
	}

	// Compute unified engagement rate at the call site (PRD §9.1) — same
	// formula as the analytics handler.
	if metrics.Impressions > 0 {
		total := metrics.Likes + metrics.Comments + metrics.Shares + metrics.Saves + metrics.Clicks
		rate := float64(total) / float64(metrics.Impressions)
		metrics.EngagementRate = float64(int64(rate*10000+0.5)) / 10000
	}

	rawData, _ := json.Marshal(metrics)
	var psBytes []byte
	if metrics.PlatformSpecific != nil {
		psBytes, _ = json.Marshal(metrics.PlatformSpecific)
	}

	if _, err := w.queries.UpsertPostAnalytics(ctx, db.UpsertPostAnalyticsParams{
		SocialPostResultID: r.SocialPostResultID,
		Views:              pgtype.Int8{Int64: metrics.Views, Valid: true},
		Likes:              pgtype.Int8{Int64: metrics.Likes, Valid: true},
		Comments:           pgtype.Int8{Int64: metrics.Comments, Valid: true},
		Shares:             pgtype.Int8{Int64: metrics.Shares, Valid: true},
		Reach:              pgtype.Int8{Int64: metrics.Reach, Valid: true},
		Impressions:        pgtype.Int8{Int64: metrics.Impressions, Valid: true},
		Saves:              pgtype.Int8{Int64: metrics.Saves, Valid: true},
		Clicks:             pgtype.Int8{Int64: metrics.Clicks, Valid: true},
		VideoViews:         pgtype.Int8{Int64: metrics.VideoViews, Valid: true},
		PlatformSpecific:   psBytes,
		EngagementRate:     numericFromFloat(metrics.EngagementRate),
		RawData:            rawData,
	}); err != nil {
		slog.Warn("analytics refresh: upsert failed",
			"result_id", r.SocialPostResultID, "error", err)
	}
}

// numericFromFloat mirrors the helper in handler/analytics.go. Duplicated here
// to avoid an import cycle (handler imports worker is fine, but worker
// importing handler is not).
func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(f)
	return n
}
