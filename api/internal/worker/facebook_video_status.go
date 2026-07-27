package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/mediaretention"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
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
	bus       events.EventBus
}

func NewFacebookVideoStatusWorker(queries *db.Queries, encryptor *crypto.AESEncryptor, buses ...events.EventBus) *FacebookVideoStatusWorker {
	var bus events.EventBus = events.NoopBus{}
	if len(buses) > 0 && buses[0] != nil {
		bus = buses[0]
	}
	return &FacebookVideoStatusWorker{queries: queries, encryptor: encryptor, bus: bus}
}

type facebookVideoStatusChecker interface {
	CheckVideoStatus(context.Context, string, string) (*platform.FacebookVideoStatus, error)
}

const (
	facebookVideoStatusTick        = 2 * time.Minute
	facebookVideoStatusConcurrency = 5
	facebookVideoStatusStaleCap    = 12 * time.Hour
	// A video with a /reel/ permalink was reclassified into the Reels
	// pipeline, which UniPost's /videos + file_url path cannot drive.
	// These never finish on their own — shorten the timeout so users
	// get a clear failure long before the generic stale cap fires.
	facebookReelReclassifiedCap = 10 * time.Minute
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

func (w *FacebookVideoStatusWorker) checkOne(ctx context.Context, fb facebookVideoStatusChecker, r db.ListFacebookVideosAwaitingStatusRow) {
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

	// Reel reclassification: if FB pushed the upload onto the Reels
	// pipeline (/videos + file_url cannot drive that flow), the row
	// will never leave "uploading". Fail fast once we've waited
	// long enough that a real upload would've started making progress.
	//
	// Exception: rows explicitly submitted as mediaType=reel go
	// through the /video_reels endpoint, where a /reel/ permalink is
	// the expected outcome. Don't fast-fail those.
	intentionalReel := r.FbMediaType.Valid && strings.EqualFold(strings.TrimSpace(r.FbMediaType.String), "reel")
	if !intentionalReel && isReelReclassified(st.PermalinkURL) && r.PostCreatedAt.Valid && time.Since(r.PostCreatedAt.Time) > facebookReelReclassifiedCap {
		errMsg := "Facebook reclassified this vertical video as a Reel. Reels publishing is not yet supported — upload a horizontal or square video."
		applied, err := w.finalizeProviderPoll(ctx, r, "failed", r.ExternalID, r.Url, errMsg, "platform_status", time.Time{})
		if err != nil {
			slog.Error("facebook video status: reel-reclassified update failed",
				"result_id", r.SocialPostResultID, "error", err)
			return
		}
		if !applied {
			return
		}
		slog.Warn("facebook video status: flipped to failed (reel reclassified)",
			"result_id", r.SocialPostResultID, "video_id", videoID,
			"permalink_url", st.PermalinkURL)
		return
	}

	switch st.VideoStatus {
	case "ready":
		// Swap external_id to the feed-story id once FB hands it
		// back. Downstream Graph calls (/comments for inbox sync,
		// /insights for analytics, DELETE) only accept the Page post
		// id under a Page-scoped token — hitting them with the raw
		// video_id returns `Object with ID 'X' does not exist`.
		newURL := r.Url.String
		newExternalID := r.ExternalID
		if st.PostID != "" {
			if u := platform.FeedStoryURL(r.PageID, st.PostID); u != "" {
				newURL = u
			}
			newExternalID = pgtype.Text{String: st.PostID, Valid: true}
		}
		completedAt := time.Now().UTC()
		applied, err := w.finalizeProviderPoll(ctx, r, "published", newExternalID, pgtype.Text{String: newURL, Valid: newURL != ""}, "", "", completedAt)
		if err != nil {
			slog.Error("facebook video status: update to published failed",
				"result_id", r.SocialPostResultID, "error", err)
			return
		}
		if !applied {
			return
		}
		slog.Info("facebook video status: flipped to published",
			"result_id", r.SocialPostResultID, "video_id", videoID, "post_id", st.PostID)

	case "error", "upload_failed", "expired":
		errMsg := "Facebook rejected the video"
		if st.ErrorMessage != "" {
			errMsg = "Facebook rejected the video: " + st.ErrorMessage
		}
		applied, err := w.finalizeProviderPoll(ctx, r, "failed", r.ExternalID, r.Url, errMsg, "platform_status", time.Time{})
		if err != nil {
			slog.Error("facebook video status: update to failed failed",
				"result_id", r.SocialPostResultID, "error", err)
			return
		}
		if !applied {
			return
		}
		slog.Info("facebook video status: flipped to failed (FB error)",
			"result_id", r.SocialPostResultID, "video_id", videoID, "message", errMsg)

	default:
		// Phase-level failure with top-level still transient — FB
		// sometimes reports a phase as errored before the top-level
		// video_status flips. Promote to failed immediately so the
		// user sees a clear failure instead of a 12h silence.
		if phaseErr := firstPhaseError(st); phaseErr != "" {
			errMsg := "Facebook video upload failed: " + phaseErr
			applied, err := w.finalizeProviderPoll(ctx, r, "failed", r.ExternalID, r.Url, errMsg, "platform_status", time.Time{})
			if err != nil {
				slog.Error("facebook video status: phase-error update failed",
					"result_id", r.SocialPostResultID, "error", err)
				return
			}
			if !applied {
				return
			}
			slog.Warn("facebook video status: flipped to failed (phase error)",
				"result_id", r.SocialPostResultID, "video_id", videoID, "message", errMsg)
			return
		}
		// Still uploading/processing/publishing. Fail out only if the
		// row has been in this limbo for so long that continuing to
		// wait is clearly wasted effort — 12h is well past any
		// plausible FB processing time for videos we accept
		// (UniPost's validator caps them at 1GB).
		if r.PostCreatedAt.Valid && time.Since(r.PostCreatedAt.Time) > facebookVideoStatusStaleCap {
			errMsg := fmt.Sprintf("Facebook video stuck in %q after %s; marking as failed", st.VideoStatus, facebookVideoStatusStaleCap)
			applied, err := w.finalizeProviderPoll(ctx, r, "failed", r.ExternalID, r.Url, errMsg, "worker_timeout", time.Time{})
			if err != nil {
				slog.Error("facebook video status: stale-cap update failed",
					"result_id", r.SocialPostResultID, "error", err)
				return
			}
			if !applied {
				return
			}
			slog.Warn("facebook video status: flipped to failed (stale cap)",
				"result_id", r.SocialPostResultID, "video_id", videoID,
				"phase", st.VideoStatus, "age", time.Since(r.PostCreatedAt.Time))
		}
	}
}

type facebookStatusPendingEvent struct {
	workspaceID   string
	event         string
	data          map[string]any
	postID        string
	parentStatus  string
	parentVersion string
}

func (w *FacebookVideoStatusWorker) finalizeProviderPoll(
	ctx context.Context,
	row db.ListFacebookVideosAwaitingStatusRow,
	status string,
	externalID pgtype.Text,
	url pgtype.Text,
	errorMessage string,
	failureStage string,
	completedAt time.Time,
) (bool, error) {
	var pendingEvent *facebookStatusPendingEvent
	applied := false
	err := w.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, row.PostID); err != nil {
			return err
		}
		publishedAt := pgtype.Timestamptz{}
		if status == "published" {
			publishedAt = pgtype.Timestamptz{Time: completedAt.UTC(), Valid: true}
		}
		_, err := txQueries.UpdateProcessingSocialPostResultAfterProviderPoll(ctx, db.UpdateProcessingSocialPostResultAfterProviderPollParams{
			Status:             status,
			ExternalID:         externalID,
			ErrorMessage:       pgtype.Text{String: errorMessage, Valid: strings.TrimSpace(errorMessage) != ""},
			PublishedAt:        publishedAt,
			Url:                url,
			ID:                 row.SocialPostResultID,
			ExpectedExternalID: row.ExternalID.String,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if status == "published" {
			if err := txQueries.IncrementUsage(ctx, db.IncrementUsageParams{
				WorkspaceID: row.WorkspaceID,
				Period:      quota.PeriodForTime(completedAt),
				PostCount:   1,
			}); err != nil {
				return err
			}
		} else if status == "failed" {
			failure := postfailures.BuildParams(
				row.PostID,
				row.SocialPostResultID,
				row.WorkspaceID,
				row.SocialAccountID,
				"facebook",
				failureStage,
				errorMessage,
				errorMessage,
			)
			if err := txQueries.UpdateSocialPostResultFailureDetails(ctx, db.UpdateSocialPostResultFailureDetailsParams{
				ID:                row.SocialPostResultID,
				ErrorCode:         postfailures.ToText(failure.ErrorCode),
				FailureStage:      postfailures.ToText(failure.FailureStage),
				PlatformErrorCode: failure.PlatformErrorCode,
				IsRetriable:       pgtype.Bool{Bool: failure.IsRetriable, Valid: true},
				NextAction:        postfailures.ToText(postfailures.NextActionForErrorCode(failure.ErrorCode)),
				ErrorSource:       failure.ErrorSource,
				ErrorTemporality:  failure.ErrorTemporality,
				ProviderError:     failure.ProviderError,
			}); err != nil {
				return err
			}
			if _, err := txQueries.CreatePostFailure(ctx, failure); err != nil {
				return err
			}
		}
		pendingEvent, err = reconcileFacebookStatusParentLocked(ctx, txQueries, row.PostID)
		if err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil || !applied {
		return applied, err
	}
	w.publishParentEventIfCurrent(ctx, pendingEvent)
	w.syncTerminalRetention(ctx, row.PostID)
	return true, nil
}

func (w *FacebookVideoStatusWorker) publishParentEventIfCurrent(ctx context.Context, pending *facebookStatusPendingEvent) {
	_ = ctx
	_ = pending
}

func reconcileFacebookStatusParentLocked(ctx context.Context, queries *db.Queries, postID string) (*facebookStatusPendingEvent, error) {
	post, err := queries.GetSocialPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	results, err := queries.ListSocialPostResultsByPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	jobs, err := queries.ListPostDeliveryJobsByPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	published, failed, nonTerminal, activeJobs := 0, 0, 0, 0
	publishedResultIDs := make(map[string]bool, len(results))
	for _, result := range results {
		switch result.Status {
		case "published":
			published++
			publishedResultIDs[result.ID] = true
		case "failed":
			failed++
		default:
			nonTerminal++
		}
	}
	for _, job := range jobs {
		if (job.State == "pending" || job.State == "running" || job.State == "retrying") && !publishedResultIDs[job.SocialPostResultID] {
			activeJobs++
		}
	}
	newStatus := "failed"
	switch {
	case activeJobs > 0 || nonTerminal > 0:
		newStatus = "publishing"
	case published == len(results):
		newStatus = "published"
	case published > 0:
		newStatus = "partial"
	}
	if newStatus == post.Status {
		return nil, nil
	}
	var publishedAt pgtype.Timestamptz
	if published > 0 {
		publishedAt = post.PublishedAt
		for _, result := range results {
			if result.Status == "published" && result.PublishedAt.Valid && (!publishedAt.Valid || result.PublishedAt.Time.Before(publishedAt.Time)) {
				publishedAt = result.PublishedAt
			}
		}
	}
	if err := queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
		ID: post.ID, Status: newStatus, PublishedAt: publishedAt,
	}); err != nil {
		return nil, err
	}
	post.Status = newStatus
	post.PublishedAt = publishedAt
	summary := ""
	if newStatus == "failed" {
		messages := make([]string, 0, failed)
		for _, result := range results {
			if result.Status == "failed" && result.ErrorMessage.Valid {
				messages = append(messages, fmt.Sprintf("[%s] %s", result.SocialAccountID, result.ErrorMessage.String))
			}
		}
		summary = strings.Join(messages, "; ")
	}
	if err := queries.UpdateSocialPostErrorMetadata(ctx, db.UpdateSocialPostErrorMetadataParams{
		ID: post.ID, Column2: summary,
	}); err != nil {
		return nil, err
	}
	if newStatus != "published" && newStatus != "partial" && newStatus != "failed" {
		return nil, nil
	}
	parentVersion, err := queries.GetSocialPostProjectionVersion(ctx, post.ID)
	if err != nil {
		return nil, err
	}
	event := events.EventPostFailed
	if newStatus == "published" {
		event = events.EventPostPublished
	} else if newStatus == "partial" {
		event = events.EventPostPartial
	}
	pending := &facebookStatusPendingEvent{
		workspaceID:   post.WorkspaceID,
		event:         event,
		postID:        post.ID,
		parentStatus:  newStatus,
		parentVersion: parentVersion,
		data: map[string]any{
			"id":            post.ID,
			"caption":       post.Caption,
			"media_urls":    post.MediaUrls,
			"status":        post.Status,
			"created_at":    post.CreatedAt,
			"published_at":  post.PublishedAt,
			"source":        post.Source,
			"profile_ids":   post.ProfileIds,
			"results":       results,
			"delivery_jobs": jobs,
		},
	}
	if err := db.RecordPostStatusTransition(ctx, queries, post.ID, post.WorkspaceID, newStatus, parentVersion, event, pending.data); err != nil {
		return nil, err
	}
	return pending, nil
}

func (w *FacebookVideoStatusWorker) syncTerminalRetention(ctx context.Context, postID string) {
	err := w.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, postID); err != nil {
			return err
		}
		w.syncTerminalRetentionLocked(ctx, txQueries, postID)
		return nil
	})
	if err != nil {
		slog.Warn("facebook video status: locked retention sync failed", "post_id", postID, "error", err)
	}
}

func (w *FacebookVideoStatusWorker) syncTerminalRetentionLocked(ctx context.Context, queries *db.Queries, postID string) {
	post, err := queries.GetSocialPostByID(ctx, postID)
	if err != nil {
		slog.Warn("facebook video status: retention post reload failed", "post_id", postID, "error", err)
		return
	}
	results, err := queries.ListSocialPostResultsByPost(ctx, postID)
	if err != nil {
		slog.Warn("facebook video status: retention result reload failed", "post_id", postID, "error", err)
		return
	}
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	parsed, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil {
		slog.Warn("facebook video status: retention metadata decode failed", "post_id", postID, "error", err)
		return
	}
	seen := map[string]bool{}
	mediaIDs := make([]string, 0)
	for _, input := range parsed {
		for _, mediaID := range input.MediaIDs {
			if strings.TrimSpace(mediaID) != "" && !seen[mediaID] {
				seen[mediaID] = true
				mediaIDs = append(mediaIDs, mediaID)
			}
		}
	}
	sort.Strings(mediaIDs)
	if len(mediaIDs) == 0 {
		if err := queries.DeleteMediaPostUsagesForPost(ctx, postID); err != nil {
			slog.Warn("facebook video status: retention delete failed", "post_id", postID, "error", err)
		}
		return
	}
	if err := queries.DeleteMediaPostUsagesForPostExcept(ctx, db.DeleteMediaPostUsagesForPostExceptParams{PostID: postID, MediaIds: mediaIDs}); err != nil {
		slog.Warn("facebook video status: retention stale usage delete failed", "post_id", postID, "error", err)
	}
	planID := "free"
	if subscription, err := queries.GetSubscriptionByWorkspace(ctx, post.WorkspaceID); err == nil {
		planID = subscription.PlanID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("facebook video status: retention subscription lookup failed", "post_id", postID, "error", err)
		return
	}
	cleanupAfter := pgtype.Timestamptz{}
	retentionReason := "plan_status"
	hasPolicyFailure := false
	for _, result := range results {
		if result.Status == "failed" && result.ErrorCode.Valid && result.ErrorCode.String == publishingrestrictions.NormalizedCode {
			hasPolicyFailure = true
			break
		}
	}
	if hasPolicyFailure {
		if retainedUntil, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, postID); err == nil && retainedUntil.Valid {
			cleanupAfter = retainedUntil
			retentionReason = "publishing_restriction"
		} else {
			cleanupAfter = pgtype.Timestamptz{Time: time.Now().UTC().Add(60 * 24 * time.Hour), Valid: true}
			retentionReason = "publishing_restriction"
		}
	} else if retention, ok := mediaretention.RetentionForPlanStatus(planID, post.Status); ok {
		cleanupAfter = pgtype.Timestamptz{Time: time.Now().Add(retention), Valid: true}
	} else {
		retentionReason = "active_post"
	}
	for _, mediaID := range mediaIDs {
		if _, err := queries.UpsertMediaPostUsage(ctx, db.UpsertMediaPostUsageParams{
			MediaID: mediaID, WorkspaceID: post.WorkspaceID, PostStatus: post.Status,
			CleanupAfterAt: cleanupAfter, RetentionReason: retentionReason, PostID: post.ID,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("facebook video status: retention upsert failed", "post_id", postID, "media_id", mediaID, "error", err)
		}
	}
}

// isReelReclassified returns true when FB's permalink_url indicates
// the upload was routed into the Reels pipeline (/reel/...) — the
// /videos + file_url path cannot drive that flow, so the row will
// never leave "uploading" on its own.
func isReelReclassified(permalinkURL string) bool {
	trimmed := strings.TrimSpace(permalinkURL)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "/reel/")
}

// firstPhaseError returns the first non-empty phase-level error
// message on the status response, or "" when every phase is clean.
// Used to promote "phase=error, top-level still transient" states
// into a failed row without waiting for the stale cap.
func firstPhaseError(st *platform.FacebookVideoStatus) string {
	if st == nil {
		return ""
	}
	// The adapter's CheckVideoStatus already copies the first
	// non-empty phase error into ErrorMessage, so when we see the
	// phase status string report "error" the message is already
	// there.
	if strings.EqualFold(st.UploadingStatus, "error") ||
		strings.EqualFold(st.ProcessingStatus, "error") ||
		strings.EqualFold(st.PublishingStatus, "error") {
		if st.ErrorMessage != "" {
			return st.ErrorMessage
		}
		return fmt.Sprintf("phase failed (uploading=%s, processing=%s, publishing=%s)",
			st.UploadingStatus, st.ProcessingStatus, st.PublishingStatus)
	}
	return ""
}
