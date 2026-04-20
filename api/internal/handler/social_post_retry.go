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
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/debugrt"
	"github.com/xiaoboyu/unipost-api/internal/platform"
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

	// Reconstruct the platform_post input from the parent's metadata
	// blob. If metadata is missing or the account isn't listed there,
	// we fall back to a minimal input built from the stored caption —
	// good enough for text-only legacy rows that predate metadata
	// storage.
	parsed, _ := platform.DecodePostMetadata(post.Metadata, derefText(post.Caption))
	var pp *platform.PlatformPostInput
	for i := range parsed {
		if parsed[i].AccountID == existing.SocialAccountID {
			pp = &parsed[i]
			break
		}
	}
	if pp == nil {
		pp = &platform.PlatformPostInput{
			AccountID: existing.SocialAccountID,
			Caption:   existing.Caption,
		}
	}

	acc, err := h.queries.GetSocialAccount(r.Context(), existing.SocialAccountID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Target account not found")
		return
	}
	if acc.DisconnectedAt.Valid {
		writeError(w, http.StatusConflict, "ACCOUNT_DISCONNECTED", "This social account is disconnected — reconnect it before retrying.")
		return
	}

	adapter, err := platform.Get(acc.Platform)
	if err != nil {
		writeError(w, http.StatusBadRequest, "UNSUPPORTED_PLATFORM", err.Error())
		return
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt access token")
		return
	}
	// Inline refresh mirrors dispatchOne: only persist rotated tokens
	// when every step succeeds, so a flaky refresh doesn't clobber
	// good credentials.
	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
		if refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String); decErr == nil {
			if newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(r.Context(), refreshTok); refErr == nil && newAccess != "" {
				encAccess, encErr := h.encryptor.Encrypt(newAccess)
				encRefresh, encErr2 := h.encryptor.Encrypt(newRefresh)
				if encErr == nil && encErr2 == nil {
					accessToken = newAccess
					_ = h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
						ID:             acc.ID,
						AccessToken:    encAccess,
						RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
						TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
					})
				}
			}
		}
	}

	// Resolve any media_ids the same way dispatchOne does so signed
	// R2 URLs get re-minted for this retry (the original signed URLs
	// may have already expired).
	mediaURLs := append([]string(nil), pp.MediaURLs...)
	if len(pp.MediaIDs) > 0 {
		extra, mediaErr := h.resolveMediaIDsToURLs(r.Context(), pp.MediaIDs)
		if mediaErr != nil {
			writeError(w, http.StatusBadRequest, "MEDIA_UNAVAILABLE", mediaErr.Error())
			return
		}
		mediaURLs = append(mediaURLs, extra...)
	}

	// Debug recorder for the retry attempt — same plumbing as
	// dispatchOne, so per-request curls land on this row too when
	// the retry fails.
	debugRec := debugrt.NewRecorder()
	dispatchCtx := debugrt.WithRecorder(r.Context(), debugRec)
	slog.Info("retry: dispatching", "post_id", post.ID, "result_id", existing.ID, "platform", acc.Platform)
	postResult, dispatchErr := adapter.Post(
		dispatchCtx,
		accessToken,
		pp.Caption,
		platform.MediaFromURLs(mediaURLs),
		pp.PlatformOptions,
	)

	// Build the update based on outcome. debug_curl is always set to
	// the latest attempt (including NULL on success) so we never
	// leave stale failure context on a now-published row.
	var (
		status       = "published"
		externalID   pgtype.Text
		errorMessage pgtype.Text
		publishedAt  pgtype.Timestamptz
		postURL      pgtype.Text
		debugCurl    pgtype.Text
	)
	if dispatchErr != nil {
		status = "failed"
		errorMessage = pgtype.Text{String: dispatchErr.Error(), Valid: true}
		if curl := debugRec.Serialize(); curl != "" {
			debugCurl = pgtype.Text{String: curl, Valid: true}
		}
	} else if postResult != nil {
		externalID = pgtype.Text{String: postResult.ExternalID, Valid: postResult.ExternalID != ""}
		publishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		if postResult.URL != "" {
			postURL = pgtype.Text{String: postResult.URL, Valid: true}
		}
	}

	updated, err := h.queries.UpdateSocialPostResultAfterRetry(r.Context(), db.UpdateSocialPostResultAfterRetryParams{
		ID:           existing.ID,
		Status:       status,
		ExternalID:   externalID,
		ErrorMessage: errorMessage,
		PublishedAt:  publishedAt,
		Url:          postURL,
		DebugCurl:    debugCurl,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to persist retry outcome")
		return
	}

	// Recompute the parent post's status from the updated result set.
	// A retry that succeeds on the last-remaining-failed row should
	// flip "failed" → "published"; a retry that succeeds on one of
	// many still-failing rows flips "failed" → "partial".
	allResults, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)
	h.refreshParentPostStatus(r, post, allResults)

	// Shape the response the same way the Get handler does so the
	// dashboard can swap the card in place.
	rr := postResultResponse{
		SocialAccountID: updated.SocialAccountID,
		Platform:        acc.Platform,
		Caption:         updated.Caption,
		Status:          updated.Status,
	}
	if acc.AccountName.Valid {
		rr.AccountName = acc.AccountName.String
	}
	if updated.ExternalID.Valid {
		rr.ExternalID = &updated.ExternalID.String
	}
	if updated.Url.Valid {
		rr.URL = &updated.Url.String
	}
	if updated.ErrorMessage.Valid {
		rr.ErrorMessage = &updated.ErrorMessage.String
	}
	if updated.PublishedAt.Valid {
		t := updated.PublishedAt.Time.Format(time.RFC3339)
		rr.PublishedAt = &t
	}
	if updated.DebugCurl.Valid {
		rr.DebugCurl = &updated.DebugCurl.String
	}
	rr.Submitted = &submittedSettings{
		Caption:         pp.Caption,
		MediaURLs:       pp.MediaURLs,
		MediaIDs:        pp.MediaIDs,
		PlatformOptions: pp.PlatformOptions,
		FirstComment:    pp.FirstComment,
		InReplyTo:       pp.InReplyTo,
		ThreadPosition:  pp.ThreadPosition,
	}
	writeSuccess(w, rr)
}

// refreshParentPostStatus walks the full set of results for a post
// and sets the parent social_posts.status accordingly. Extracted so
// the retry + bulk-ops paths share the same derivation.
func (h *SocialPostHandler) refreshParentPostStatus(r *http.Request, post db.SocialPost, results []db.SocialPostResult) {
	if len(results) == 0 {
		return
	}
	published := 0
	failed := 0
	for _, res := range results {
		switch res.Status {
		case "published":
			published++
		case "failed":
			failed++
		}
	}
	newStatus := "failed"
	switch {
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
	_ = h.queries.UpdateSocialPostStatus(r.Context(), db.UpdateSocialPostStatusParams{
		ID:          post.ID,
		Status:      newStatus,
		PublishedAt: newPublishedAt,
	})
	// If we just flipped off of "failed", clear the metadata error
	// summary so the posts list doesn't keep showing stale copy.
	if newStatus != "failed" {
		_ = h.queries.UpdateSocialPostErrorMetadata(r.Context(), db.UpdateSocialPostErrorMetadataParams{
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
		_ = h.queries.UpdateSocialPostErrorMetadata(r.Context(), db.UpdateSocialPostErrorMetadataParams{
			ID:      post.ID,
			Column2: strings.Join(summary, "; "),
		})
	}
}
