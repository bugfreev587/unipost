package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

// SchedulerWorker publishes scheduled posts when their scheduled_at time arrives.
type SchedulerWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	// bus fans out post.published / post.partial / post.failed events
	// once a scheduled post finishes its publish loop. Always non-nil.
	bus events.EventBus
}

func NewSchedulerWorker(queries *db.Queries, encryptor *crypto.AESEncryptor, bus events.EventBus) *SchedulerWorker {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &SchedulerWorker{queries: queries, encryptor: encryptor, bus: bus}
}

func (w *SchedulerWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	slog.Info("scheduler worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler worker stopped")
			return
		case <-ticker.C:
			w.publishDue(ctx)
		}
	}
}

func (w *SchedulerWorker) publishDue(ctx context.Context) {
	posts, err := w.queries.GetDueScheduledPosts(ctx)
	if err != nil {
		slog.Error("scheduler: failed to get due posts", "error", err)
		return
	}

	if len(posts) == 0 {
		return
	}

	slog.Info("scheduler: found due posts", "count", len(posts))

	for _, post := range posts {
		// Claim the post (optimistic lock)
		claimed, err := w.queries.ClaimScheduledPost(ctx, post.ID)
		if err != nil {
			continue // Another instance already claimed it
		}

		go w.publishPost(ctx, claimed)
	}
}

// publishPost is the per-post publish loop. Reads the v2 metadata
// (with v1 fallback) so per-platform captions stored at create time
// are preserved through to the adapter call. Each PlatformPostInput
// becomes one adapter dispatch — so threads (multiple posts to the
// same account) work without special-casing.
func (w *SchedulerWorker) publishPost(ctx context.Context, post db.SocialPost) {
	slog.Info("scheduler: publishing post", "post_id", post.ID)

	// Decode the persisted metadata back into a slice of
	// PlatformPostInput. v2 rows give us per-account captions; v1 rows
	// fall back to the parent post's caption (single string).
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	posts, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil || len(posts) == 0 {
		slog.Error("scheduler: failed to decode post metadata", "post_id", post.ID, "error", err)
		w.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
			ID: post.ID, Status: "failed",
		})
		return
	}

	// Legacy v1 rows store platform_options keyed by platform name
	// (since they didn't know per-account at create time). Resolve
	// after fetching each account's platform below. v2 rows return
	// nil here and the per-post PlatformOptions is used directly.
	v1Options := platform.LegacyV1Metadata(post.Metadata)

	// Sprint 5 PR2: per-account monthly quota also applies to the
	// scheduled path. Without this guard, a customer could schedule
	// 10000 posts past the cap and watch the worker happily blow
	// through it at the scheduled time. Build the tracker the same
	// way the immediate path does — load the project for the limit,
	// dedupe the account ids, snapshot current-month counts.
	var perAccountLimit pgtype.Int4
	if ws, wsErr := w.queries.GetWorkspace(ctx, post.WorkspaceID); wsErr == nil {
		perAccountLimit = ws.PerAccountMonthlyLimit
	}
	uniqueIDs := make([]string, 0, len(posts))
	seen := make(map[string]struct{}, len(posts))
	for _, pp := range posts {
		if _, ok := seen[pp.AccountID]; ok {
			continue
		}
		seen[pp.AccountID] = struct{}{}
		uniqueIDs = append(uniqueIDs, pp.AccountID)
	}
	tracker := quota.NewPerAccountTracker(ctx, w.queries, perAccountLimit, uniqueIDs)

	// One outcome per input post, indexed by position so the result
	// slice can be filled from goroutines without a mutex / append race.
	type outcome struct {
		input      platform.PlatformPostInput
		platform   string
		postResult *platform.PostResult
		err        error
	}
	outcomes := make([]outcome, len(posts))

	var wg sync.WaitGroup
	for i, pp := range posts {
		wg.Add(1)
		go func(idx int, pp platform.PlatformPostInput) {
			defer wg.Done()
			outcomes[idx] = w.publishOne(ctx, post, pp, v1Options, tracker)
		}(i, pp)
	}
	wg.Wait()

	// Persist results in input order with the per-post caption.
	allPublished := true
	anyPublished := false
	for i, oc := range outcomes {
		status := "published"
		var extID, errMsg pgtype.Text
		var pubAt pgtype.Timestamptz

		if oc.err != nil {
			status = "failed"
			errMsg = pgtype.Text{String: oc.err.Error(), Valid: true}
			allPublished = false
		} else if oc.postResult != nil {
			extID = pgtype.Text{String: oc.postResult.ExternalID, Valid: true}
			pubAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			anyPublished = true
		}

		w.queries.CreateSocialPostResult(ctx, db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: posts[i].AccountID,
			Caption:         posts[i].Caption,
			Status:          status,
			ExternalID:      extID,
			ErrorMessage:    errMsg,
			PublishedAt:     pubAt,
		})
	}

	postStatus := "failed"
	if allPublished {
		postStatus = "published"
	} else if anyPublished {
		postStatus = "partial"
	}

	var publishedAt pgtype.Timestamptz
	if anyPublished {
		publishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	w.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
		ID: post.ID, Status: postStatus, PublishedAt: publishedAt,
	})

	// Fan out webhook events. Best-effort — Publish recovers panics
	// internally and never blocks the worker. Payload is a minimal
	// post object so subscribers can correlate by ID without
	// re-fetching anything.
	w.bus.Publish(ctx, post.WorkspaceID, eventForStatus(postStatus), map[string]any{
		"post_id":      post.ID,
		"status":       postStatus,
		"scheduled_at": post.ScheduledAt.Time,
		"published_at": publishedAt.Time,
	})

	slog.Info("scheduler: post published", "post_id", post.ID, "status", postStatus)
}

// eventForStatus mirrors the helper in handler/social_posts.go so the
// scheduler emits the same event names. Tiny duplicate to avoid an
// import cycle (worker → handler is not allowed).
func eventForStatus(postStatus string) string {
	switch postStatus {
	case "published":
		return events.EventPostPublished
	case "partial":
		return events.EventPostPartial
	case "failed":
		return events.EventPostFailed
	default:
		return events.EventPostFailed
	}
}

// publishOne is the goroutine body for one PlatformPostInput. Pulled
// out of publishPost both for readability and so the dispatch loop
// can fill its outcomes slice by index without mutating shared state.
//
// v1Options is the legacy platform_options map (keyed by platform
// name) and is non-nil only when the parent post was created before
// Sprint 1. v2 rows pass options on the per-post struct itself.
func (w *SchedulerWorker) publishOne(
	ctx context.Context,
	parent db.SocialPost,
	pp platform.PlatformPostInput,
	v1Options map[string]map[string]any,
	tracker *quota.PerAccountTracker,
) (oc struct {
	input      platform.PlatformPostInput
	platform   string
	postResult *platform.PostResult
	err        error
}) {
	oc.input = pp

	acc, err := w.queries.GetSocialAccount(ctx, pp.AccountID)
	if err != nil {
		oc.err = err
		return
	}
	oc.platform = acc.Platform

	// Sprint 5 PR2: per-account monthly quota gate. Same semantics
	// as the immediate path in handler/social_posts.go — refusal
	// here records the deterministic error string and skips the
	// adapter call entirely.
	if tracker != nil && !tracker.Allow(acc.ID) {
		oc.err = quota.ErrPerAccountQuotaExceeded
		return
	}

	adapter, err := platform.Get(acc.Platform)
	if err != nil {
		oc.err = err
		return
	}
	accessToken, err := w.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		oc.err = err
		return
	}

	// Inline token refresh if expired.
	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
		if refreshTok, decErr := w.encryptor.Decrypt(acc.RefreshToken.String); decErr == nil {
			if newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(ctx, refreshTok); refErr == nil {
				accessToken = newAccess
				encAccess, _ := w.encryptor.Encrypt(newAccess)
				encRefresh, _ := w.encryptor.Encrypt(newRefresh)
				w.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
					ID:             acc.ID,
					AccessToken:    encAccess,
					RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
					TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
			}
		}
	}

	// Resolve platform options. v2 stores them per-post; v1 stores
	// them keyed by platform name and we look them up here.
	options := pp.PlatformOptions
	if options == nil && v1Options != nil {
		options = v1Options[acc.Platform]
	}

	// Per-platform routing log — emitted at INFO so smoke-tests can
	// verify each PlatformPostInput is reaching the right adapter
	// with the right caption. Truncate captions so the log line
	// stays narrow even when callers ship 2200-character IG bodies.
	slog.Info("scheduler: dispatching to adapter",
		"post_id", parent.ID,
		"account_id", acc.ID,
		"platform", acc.Platform,
		"caption_preview", truncateForLog(pp.Caption, 40))

	pr, err := adapter.Post(ctx, accessToken, pp.Caption, platform.MediaFromURLs(pp.MediaURLs), options)
	oc.postResult = pr
	oc.err = err
	return
}

// truncateForLog returns a copy of s shortened to at most n runes,
// appending an ellipsis if it was actually truncated. Used to keep
// log lines bounded when captions get long.
func truncateForLog(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

