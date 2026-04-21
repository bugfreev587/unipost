// social_post_retry.go implements per-platform retry: when a single
// social_post_result row has status='failed', the user can kick it off
// again from the dashboard without rebuilding the whole post. The
// original platform_post metadata (caption, media, options) is read
// from the parent post's metadata blob, the adapter is invoked the
// same way dispatchOne does, and the SAME result row is overwritten
// with the new outcome — no new rows are created per retry, so a
// given post+account pair always has exactly one result.
//
// Parent post status is recomputed from ALL results after the update
// so a previously "failed" post correctly flips to "published" (every
// result now succeeds) or "partial" (some still failed).

package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// RetryResult handles
//
//	POST /v1/workspaces/{workspaceID}/social-posts/{id}/results/{resultID}/retry
//
// Only rows with status='failed' may be retried — a published or
// processing row returns 409 so we can't accidentally double-publish
// when an earlier success was misreported. No idempotency key: TikTok
// / Meta / etc. don't support them, and we rely on the failed-status
// gate to keep retries safe.
func (h *SocialPostHandler) RetryResult(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}
	resultID := chi.URLParam(r, "resultID")
	if postID == "" || resultID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing post or result id")
		return
	}

	post, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load post")
		return
	}
	existing, err := h.queries.GetSocialPostResultByIDAndPost(r.Context(), db.GetSocialPostResultByIDAndPostParams{
		ID:     resultID,
		PostID: post.ID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Result not found for this post")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load result")
		return
	}
	if existing.Status != "failed" {
		writeError(w, http.StatusConflict, "RESULT_NOT_RETRYABLE",
			fmt.Sprintf("Only failed results can be retried (current status: %s)", existing.Status))
		return
	}
	job, err := h.EnqueueRetryForResult(r.Context(), workspaceID, post.ID, existing.ID)
	if err != nil {
		if isQueueConflict(err) {
			writeError(w, http.StatusConflict, "QUEUE_JOB_ACTIVE", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	rr := postResultResponse{
		ID:              existing.ID,
		SocialAccountID: existing.SocialAccountID,
		Caption:         existing.Caption,
		Status:          "queued",
	}
	if job.Platform != "" {
		rr.Platform = job.Platform
	}
	writeSuccess(w, rr)
}

// refreshParentPostStatus walks the full set of results for a post
// and sets the parent social_posts.status accordingly. Extracted so
// the retry + bulk-ops paths share the same derivation.
func (h *SocialPostHandler) refreshParentPostStatus(r *http.Request, post db.SocialPost, results []db.SocialPostResult) {
	h.refreshParentPostStatusContext(r.Context(), post, results)
}

func (h *SocialPostHandler) refreshParentPostStatusContext(ctx context.Context, post db.SocialPost, results []db.SocialPostResult) {
	if len(results) == 0 {
		return
	}
	jobs, _ := h.queries.ListPostDeliveryJobsByPost(ctx, post.ID)
	published := 0
	failed := 0
	nonTerminal := 0
	activeJobs := 0
	for _, res := range results {
		switch res.Status {
		case "published":
			published++
		case "failed":
			failed++
		default:
			nonTerminal++
		}
	}
	for _, job := range jobs {
		if job.State == "pending" || job.State == "running" || job.State == "retrying" {
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
		return
	}
	var newPublishedAt pgtype.Timestamptz
	if published > 0 {
		// Preserve the earliest published_at if the parent already
		// had one, otherwise stamp now. "Now" is correct for a
		// post that was previously "failed" with no publish time.
		if post.PublishedAt.Valid {
			newPublishedAt = post.PublishedAt
		} else {
			newPublishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}
	}
	_ = h.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
		ID:          post.ID,
		Status:      newStatus,
		PublishedAt: newPublishedAt,
	})
	post.Status = newStatus
	post.PublishedAt = newPublishedAt
	// If we just flipped off of "failed", clear the metadata error
	// summary so the posts list doesn't keep showing stale copy.
	if newStatus != "failed" {
		_ = h.queries.UpdateSocialPostErrorMetadata(ctx, db.UpdateSocialPostErrorMetadataParams{
			ID:      post.ID,
			Column2: "",
		})
	} else {
		// Still failed but maybe fewer rows now — regenerate the
		// summary from the latest errors.
		summary := make([]string, 0, failed)
		for _, res := range results {
			if res.Status == "failed" && res.ErrorMessage.Valid {
				summary = append(summary, fmt.Sprintf("[%s] %s", res.SocialAccountID, res.ErrorMessage.String))
			}
		}
		_ = h.queries.UpdateSocialPostErrorMetadata(ctx, db.UpdateSocialPostErrorMetadataParams{
			ID:      post.ID,
			Column2: strings.Join(summary, "; "),
		})
	}
	if newStatus == "published" || newStatus == "partial" || newStatus == "failed" {
		resp := h.socialPostResponseFromData(post, results, jobs, "async")
		h.bus.Publish(ctx, post.WorkspaceID, eventForStatus(newStatus), resp)
	}
}
