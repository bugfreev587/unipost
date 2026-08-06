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

func postMediaRetentionSyncSkipped(ctx context.Context) bool {
	skip, _ := ctx.Value(skipPostMediaRetentionSyncKey{}).(bool)
	return skip
}

func shouldLogMediaPostUsageUpsertError(err error) bool {
	// Cleanup may legitimately win the usage-version/status race. In that
	// case the query returns no row and the object remains safely unavailable.
	return err != nil && !errors.Is(err, pgx.ErrNoRows)
}

// Why UpsertMediaPostUsage matched no row. Only a permanently unretainable
// media may be skipped; a transient state, a tenancy mismatch, or a failed
// diagnosis all stay fail-closed.
const (
	mediaUnretainableMissing           = "missing"
	mediaUnretainableDeleted           = "deleted"
	mediaUnretainablePending           = "pending"
	mediaUnretainableWorkspaceMismatch = "workspace_mismatch"
	mediaUnretainableRaceResolved      = "race_resolved"
	mediaUnretainableUnknownStatus     = "unknown_status"
)

// classifyUnretainableMedia explains a zero-row UpsertMediaPostUsage and
// reports whether the media may be skipped.
//
// Skippable means the retention obligation is already unfulfillable: the row
// is gone, or it is marked deleted and headed for purge. Rolling the caller's
// transaction back cannot resurrect either, so failing closed would only park
// the post in `scheduled` and replay the same unrecoverable condition on every
// scheduler pass.
//
// Everything else stays fail-closed:
//   - pending: the publish path hydrates pending rows to `uploaded` and
//     publishes them (see resolveMediaIDsToURLs), so skipping would drop a
//     retention obligation for media that does get delivered.
//   - workspace_mismatch: a cross-tenant media reference is a data-integrity
//     defect. Skipping would let the post proceed to publish it.
//   - race_resolved: the media is live again (cleanup lost, or a pending row
//     hydrated) between the upsert and this diagnosis. Genuinely transient —
//     the next scheduler pass succeeds, so the retry is bounded.
//   - unknown_status: an unmodelled state; refuse to guess.
//
// The diagnosis is a read of the latest committed state, so it can disagree
// with what the upsert saw. That is why a disagreement (race_resolved) is
// classified rather than assumed away.
func (h *SocialPostHandler) classifyUnretainableMedia(ctx context.Context, workspaceID, mediaID string) (reason string, skippable bool, err error) {
	row, getErr := h.queries.GetMediaRetentionState(ctx, mediaID)
	if errors.Is(getErr, pgx.ErrNoRows) {
		return mediaUnretainableMissing, true, nil
	}
	if getErr != nil {
		// Never let a failed diagnosis masquerade as a benign zero row.
		return "", false, getErr
	}
	if row.WorkspaceID != workspaceID {
		return mediaUnretainableWorkspaceMismatch, false, nil
	}
	switch row.Status {
	case "deleted":
		return mediaUnretainableDeleted, true, nil
	case "pending":
		return mediaUnretainablePending, false, nil
	case "uploaded", "attached":
		return mediaUnretainableRaceResolved, false, nil
	default:
		return mediaUnretainableUnknownStatus, false, nil
	}
}

// handleStrictUsageUpsertError decides a strict-path upsert failure for one
// media. A nil return means "skip this media and carry on"; a non-nil return
// must roll the caller's transaction back.
func (h *SocialPostHandler) handleStrictUsageUpsertError(
	ctx context.Context,
	post db.SocialPost,
	postStatus string,
	mediaID string,
	upsertErr error,
) error {
	if !errors.Is(upsertErr, pgx.ErrNoRows) {
		return fmt.Errorf("media retention: upsert usage for post %s media %s: %w", post.ID, mediaID, upsertErr)
	}
	reason, skippable, classifyErr := h.classifyUnretainableMedia(ctx, post.WorkspaceID, mediaID)
	if classifyErr != nil {
		return fmt.Errorf("media retention: classify unretainable media for post %s media %s: %w",
			post.ID, mediaID, classifyErr)
	}
	if !skippable {
		return fmt.Errorf("media retention: upsert usage for post %s media %s: not retainable (%s): %w",
			post.ID, mediaID, reason, upsertErr)
	}
	// Stable message + classification field: this is the aggregation signal
	// that replaces a metrics counter (no metrics SDK in this service).
	slog.Warn("media retention: strict media skipped",
		"post_id", post.ID,
		"workspace_id", post.WorkspaceID,
		"media_id", mediaID,
		"post_status", postStatus,
		"classification", reason)
	return nil
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
	sort.Strings(ids)
	return ids, true
}

func (h *SocialPostHandler) syncPostMediaRetention(ctx context.Context, post db.SocialPost, postStatus string) {
	if postMediaRetentionSyncSkipped(ctx) {
		return
	}
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
	if postMediaRetentionSyncSkipped(ctx) {
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

	// External pull objects can only be staged when object storage is
	// configured, so storage-less processes do not need this lifecycle update.
	if h.storage != nil {
		if err := h.queries.UpdatePublishingPullObjectUsagesForPost(ctx, db.UpdatePublishingPullObjectUsagesForPostParams{
			PostStatus:      postStatus,
			CleanupAfterAt:  cleanupAfter,
			RetentionReason: retentionReason,
			PostID:          post.ID,
		}); err != nil {
			if strict {
				return fmt.Errorf("publishing pull object retention: update usages for post %s: %w", post.ID, err)
			}
			slog.Warn("publishing pull object retention: usage update failed",
				"post_id", post.ID,
				"post_status", postStatus,
				"error", err)
		}
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
				// Each media is decided independently: skipping one
				// unretainable object must not block the rest of the
				// post's ledger, nor the targets that can still publish.
				if strictErr := h.handleStrictUsageUpsertError(ctx, post, postStatus, mediaID, err); strictErr != nil {
					return strictErr
				}
				continue
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
