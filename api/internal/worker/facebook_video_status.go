package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// FacebookVideoStatusWorker flips Facebook video rows out of
// `processing` once Graph reports them ready/error, without relying on
// a dashboard Get to trigger the existing on-demand refresh.
//
// Facebook's /videos publish endpoint returns a video_id immediately,
// then processes the upload asynchronously. The adapter polls inline
// for 60s before returning Status="processing"; beyond that, the row
// used to only ever flip when the handler's Get path re-polled on
// user view. If nobody looked at the post (or if FB took longer than
// the user's attention span), the row could sit at "processing"
// indefinitely — we had 3-day-old rows on prod at the time this was
// added.
//
// Runs every 2 minutes. That's frequent enough that a typical video
// under a few minutes' processing gets flipped within one tick of
// completion, but rare enough that a backlog of 100 rows still fits
// well under Graph's rate limits at the default concurrency.
//
// Rows that have been processing for more than facebookVideoStatusStaleCap
// are marked "failed" so they don't pollute the processing queue
// forever — Facebook genuinely finishing a video after 12h is so rare
// it's safer to assume something broke upstream and let the user
// republish.
type FacebookVideoStatusWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewFacebookVideoStatusWorker(queries *db.Queries, encryptor *crypto.AESEncryptor) *FacebookVideoStatusWorker {
	return &FacebookVideoStatusWorker{queries: queries, encryptor: encryptor}
}

const (
	facebookVideoStatusTick        = 2 * time.Minute
	facebookVideoStatusConcurrency = 5
	facebookVideoStatusStaleCap    = 12 * time.Hour
)

func (w *FacebookVideoStatusWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(facebookVideoStatusTick)
	defer ticker.Stop()

	slog.Info("facebook video status worker started")

	// Run once on startup so a freshly-deployed instance resolves any
	// rows that were left processing across the deploy.
	w.sweep(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("facebook video status worker stopped")
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *FacebookVideoStatusWorker) sweep(ctx context.Context) {
	rows, err := w.queries.ListFacebookVideosAwaitingStatus(ctx)
	if err != nil {
		slog.Error("facebook video status: failed to list processing rows", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}

	adapter, err := platform.Get("facebook")
	if err != nil {
		// Adapter isn't registered — this can happen in test envs
		// that skip the Facebook wiring. Noisy enough to notice, not
		// enough to bail out of other workers.
		slog.Warn("facebook video status: adapter not registered", "error", err)
		return
	}
	fb, ok := adapter.(*platform.FacebookAdapter)
	if !ok {
		slog.Error("facebook video status: registered adapter is not *FacebookAdapter")
		return
	}

	slog.Info("facebook video status: checking processing rows", "count", len(rows))

	sem := make(chan struct{}, facebookVideoStatusConcurrency)
	var wg sync.WaitGroup
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(r db.ListFacebookVideosAwaitingStatusRow) {
			defer wg.Done()
			defer func() { <-sem }()
			w.checkOne(ctx, fb, r)
		}(row)
	}
	wg.Wait()
}

func (w *FacebookVideoStatusWorker) checkOne(ctx context.Context, fb *platform.FacebookAdapter, r db.ListFacebookVideosAwaitingStatusRow) {
	if !r.ExternalID.Valid {
		// Filtered out by the WHERE clause, but defensive.
		return
	}
	videoID := r.ExternalID.String

	accessToken, err := w.encryptor.Decrypt(r.AccessToken)
	if err != nil {
		slog.Warn("facebook video status: decrypt failed", "result_id", r.SocialPostResultID, "error", err)
		return
	}

	st, err := fb.CheckVideoStatus(ctx, accessToken, videoID)
	if err != nil {
		// Transient errors: leave the row alone and try again next tick.
		// The staleness cap below will still fire on its own schedule
		// regardless of whether status checks are working.
		slog.Warn("facebook video status: CheckVideoStatus failed",
			"result_id", r.SocialPostResultID, "video_id", videoID, "error", err)
		return
	}

	switch st.VideoStatus {
	case "ready":
		newURL := r.Url.String
		if st.PostID != "" {
			if u := platform.FeedStoryURL(r.PageID, st.PostID); u != "" {
				newURL = u
			}
		}
		if _, err := w.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
			ID:           r.SocialPostResultID,
			Status:       "published",
			ExternalID:   r.ExternalID,
			ErrorMessage: pgtype.Text{Valid: false},
			PublishedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			Url:          pgtype.Text{String: newURL, Valid: newURL != ""},
			DebugCurl:    pgtype.Text{Valid: false},
		}); err != nil {
			slog.Error("facebook video status: update to published failed",
				"result_id", r.SocialPostResultID, "error", err)
			return
		}
		slog.Info("facebook video status: flipped to published",
			"result_id", r.SocialPostResultID, "video_id", videoID, "post_id", st.PostID)

	case "error":
		errMsg := "Facebook rejected the video"
		if st.ErrorMessage != "" {
			errMsg = "Facebook rejected the video: " + st.ErrorMessage
		}
		if _, err := w.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
			ID:           r.SocialPostResultID,
			Status:       "failed",
			ExternalID:   r.ExternalID,
			ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
			PublishedAt:  pgtype.Timestamptz{Valid: false},
			Url:          r.Url,
			DebugCurl:    pgtype.Text{Valid: false},
		}); err != nil {
			slog.Error("facebook video status: update to failed failed",
				"result_id", r.SocialPostResultID, "error", err)
			return
		}
		slog.Info("facebook video status: flipped to failed (FB error)",
			"result_id", r.SocialPostResultID, "video_id", videoID, "message", errMsg)

	default:
		// Still uploading/processing/publishing. Fail out only if the
		// row has been in this limbo for so long that continuing to
		// wait is clearly wasted effort — 12h is well past any
		// plausible FB processing time for videos we accept
		// (UniPost's validator caps them at 1GB).
		if r.PostCreatedAt.Valid && time.Since(r.PostCreatedAt.Time) > facebookVideoStatusStaleCap {
			errMsg := fmt.Sprintf("Facebook video stuck in %q after %s; marking as failed", st.VideoStatus, facebookVideoStatusStaleCap)
			if _, err := w.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
				ID:           r.SocialPostResultID,
				Status:       "failed",
				ExternalID:   r.ExternalID,
				ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
				PublishedAt:  pgtype.Timestamptz{Valid: false},
				Url:          r.Url,
				DebugCurl:    pgtype.Text{Valid: false},
			}); err != nil {
				slog.Error("facebook video status: stale-cap update failed",
					"result_id", r.SocialPostResultID, "error", err)
				return
			}
			slog.Warn("facebook video status: flipped to failed (stale cap)",
				"result_id", r.SocialPostResultID, "video_id", videoID,
				"phase", st.VideoStatus, "age", time.Since(r.PostCreatedAt.Time))
		}
	}
}
