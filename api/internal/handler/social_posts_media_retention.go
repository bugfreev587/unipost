package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mediaretention"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

var (
	errRetryMediaReuploadRequired = errors.New("retained media is unavailable; re-upload required")
	errInvalidPostMediaMetadata   = errors.New("invalid post media metadata")
)

type skipPostMediaRetentionSyncKey struct{}

func withoutPostMediaRetentionSync(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipPostMediaRetentionSyncKey{}, true)
}

func shouldLogMediaPostUsageUpsertError(err error) bool {
	// Cleanup may legitimately win the usage-version/status race. In that
	// case the query returns no row and the object remains safely unavailable.
	return err != nil && !errors.Is(err, pgx.ErrNoRows)
}

func mediaIDsForRetention(post db.SocialPost) []string {
	ids, _ := decodeMediaIDsForRetention(post)
	return ids
}

func decodeMediaIDsForRetention(post db.SocialPost) ([]string, bool) {
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	parsed, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, pp := range parsed {
		for _, id := range pp.MediaIDs {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, true
}

func (h *SocialPostHandler) syncPostMediaRetention(ctx context.Context, post db.SocialPost, postStatus string) {
	h.syncPostMediaRetentionAt(ctx, post, postStatus, "", time.Time{})
}

func (h *SocialPostHandler) syncPostMediaRetentionForPublishingRestrictionAt(ctx context.Context, post db.SocialPost, failedAt time.Time) {
	h.syncPostMediaRetentionForPublishingRestrictionStatusAt(ctx, post, post.Status, failedAt)
}

func (h *SocialPostHandler) syncPostMediaRetentionForPublishingRestrictionStatusAt(ctx context.Context, post db.SocialPost, postStatus string, failedAt time.Time) {
	if failedAt.IsZero() {
		failedAt = time.Now()
	}
	h.syncPostMediaRetentionAt(ctx, post, postStatus, "publishing_restriction", failedAt.UTC().Add(60*24*time.Hour))
}

func (h *SocialPostHandler) syncPostMediaRetentionForPublishingRestrictionStatusAtStrict(
	ctx context.Context,
	post db.SocialPost,
	postStatus string,
	failedAt time.Time,
) error {
	if failedAt.IsZero() {
		failedAt = time.Now()
	}
	return h.syncPostMediaRetentionAtMode(
		ctx,
		post,
		postStatus,
		"publishing_restriction",
		failedAt.UTC().Add(60*24*time.Hour),
		true,
	)
}

func (h *SocialPostHandler) syncPostMediaRetentionAfterResultTransition(
	ctx context.Context,
	post db.SocialPost,
	postStatus string,
	results []db.SocialPostResult,
) {
	if skip, _ := ctx.Value(skipPostMediaRetentionSyncKey{}).(bool); skip {
		return
	}
	hasPolicyFailure := false
	for _, result := range results {
		if result.Status == "failed" && result.ErrorCode.Valid && result.ErrorCode.String == publishingrestrictions.NormalizedCode {
			hasPolicyFailure = true
			break
		}
	}
	if !hasPolicyFailure {
		h.syncPostMediaRetention(ctx, post, postStatus)
		return
	}
	retainedUntil, err := h.queries.GetPostPublishingRestrictionMediaRetention(ctx, post.ID)
	if err == nil && retainedUntil.Valid {
		h.syncPostMediaRetentionAt(ctx, post, postStatus, "publishing_restriction", retainedUntil.Time)
		return
	}
	// Defensive repair for a policy-failed result whose ledger row is missing:
	// retain from the transition time rather than shortening to plan retention.
	h.syncPostMediaRetentionForPublishingRestrictionStatusAt(ctx, post, postStatus, time.Now())
}

func (h *SocialPostHandler) syncPostMediaRetentionAt(
	ctx context.Context,
	post db.SocialPost,
	postStatus string,
	retentionReason string,
	cleanupOverride time.Time,
) {
	_ = h.syncPostMediaRetentionAtMode(ctx, post, postStatus, retentionReason, cleanupOverride, false)
}

func (h *SocialPostHandler) syncPostMediaRetentionAtMode(
	ctx context.Context,
	post db.SocialPost,
	postStatus string,
	retentionReason string,
	cleanupOverride time.Time,
	strict bool,
) error {
	if h == nil || h.queries == nil {
		if strict {
			return errors.New("media retention: database is not configured")
		}
		return nil
	}
	ids, ok := decodeMediaIDsForRetention(post)
	if !ok {
		if strict {
			return fmt.Errorf("%w for post %s", errInvalidPostMediaMetadata, post.ID)
		}
		slog.Warn("media retention: metadata decode failed",
			"post_id", post.ID,
			"post_status", postStatus)
		return nil
	}
	if strict {
		sort.Strings(ids)
	}
	if len(ids) == 0 {
		if err := h.queries.DeleteMediaPostUsagesForPost(ctx, post.ID); err != nil {
			if strict {
				return fmt.Errorf("media retention: delete usages for post %s: %w", post.ID, err)
			}
			slog.Warn("media retention: usage delete failed",
				"post_id", post.ID,
				"post_status", postStatus,
				"error", err)
		}
		return nil
	}
	if err := h.queries.DeleteMediaPostUsagesForPostExcept(ctx, db.DeleteMediaPostUsagesForPostExceptParams{
		PostID:   post.ID,
		MediaIds: ids,
	}); err != nil {
		if strict {
			return fmt.Errorf("media retention: delete stale usages for post %s: %w", post.ID, err)
		}
		slog.Warn("media retention: stale usage delete failed",
			"post_id", post.ID,
			"post_status", postStatus,
			"error", err)
	}

	planID := "free"
	if h.quota != nil {
		planID = h.quota.PlanIDFor(ctx, post.WorkspaceID)
	}

	var cleanupAfter pgtype.Timestamptz
	if !cleanupOverride.IsZero() {
		cleanupAfter = pgtype.Timestamptz{Time: cleanupOverride.UTC(), Valid: true}
	} else if retention, ok := mediaretention.RetentionForPlanStatus(planID, postStatus); ok {
		cleanupAfter = pgtype.Timestamptz{Time: time.Now().Add(retention), Valid: true}
	}
	if retentionReason == "" {
		retentionReason = "plan_status"
		if !cleanupAfter.Valid {
			retentionReason = "active_post"
		}
	}

	for _, mediaID := range ids {
		if _, err := h.queries.UpsertMediaPostUsage(ctx, db.UpsertMediaPostUsageParams{
			WorkspaceID:     post.WorkspaceID,
			MediaID:         mediaID,
			PostID:          post.ID,
			PostStatus:      postStatus,
			CleanupAfterAt:  cleanupAfter,
			RetentionReason: retentionReason,
		}); err != nil {
			if strict {
				return fmt.Errorf("media retention: upsert usage for post %s media %s: %w", post.ID, mediaID, err)
			}
			if shouldLogMediaPostUsageUpsertError(err) {
				slog.Warn("media retention: usage upsert failed",
					"post_id", post.ID,
					"media_id", mediaID,
					"post_status", postStatus,
					"error", err)
			}
		}
	}
	return nil
}
