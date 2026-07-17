package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mediaretention"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

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
	if h == nil || h.queries == nil {
		return
	}
	ids, ok := decodeMediaIDsForRetention(post)
	if !ok {
		slog.Warn("media retention: metadata decode failed",
			"post_id", post.ID,
			"post_status", postStatus)
		return
	}
	if len(ids) == 0 {
		if err := h.queries.DeleteMediaPostUsagesForPost(ctx, post.ID); err != nil {
			slog.Warn("media retention: usage delete failed",
				"post_id", post.ID,
				"post_status", postStatus,
				"error", err)
		}
		return
	}
	if err := h.queries.DeleteMediaPostUsagesForPostExcept(ctx, db.DeleteMediaPostUsagesForPostExceptParams{
		PostID:   post.ID,
		MediaIds: ids,
	}); err != nil {
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
	if retention, ok := mediaretention.RetentionForPlanStatus(planID, postStatus); ok {
		cleanupAfter = pgtype.Timestamptz{Time: time.Now().Add(retention), Valid: true}
	}

	for _, mediaID := range ids {
		if _, err := h.queries.UpsertMediaPostUsage(ctx, db.UpsertMediaPostUsageParams{
			WorkspaceID:    post.WorkspaceID,
			MediaID:        mediaID,
			PostID:         post.ID,
			PostStatus:     postStatus,
			CleanupAfterAt: cleanupAfter,
		}); err != nil {
			slog.Warn("media retention: usage upsert failed",
				"post_id", post.ID,
				"media_id", mediaID,
				"post_status", postStatus,
				"error", err)
		}
	}
}
