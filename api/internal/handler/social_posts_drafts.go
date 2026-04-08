// social_posts_drafts.go houses the Sprint 2 drafts API:
//
//	createDraft           — POST /v1/social-posts with status="draft"
//	PublishDraft          — POST /v1/social-posts/{id}/publish
//	UpdateDraft           — PATCH /v1/social-posts/{id}
//	DeleteDraft           — DELETE /v1/social-posts/{id}
//
// Drafts share the social_posts table — they're rows in status='draft'
// with no rows in social_post_results yet. The Sprint 1 publish path
// is reused for the publish-from-draft transition: PublishDraft is
// just a thin wrapper that flips status atomically (optimistic lock)
// then dispatches to createImmediatePost so quota counting / event
// emission / per-result caption persistence stay in one place.

package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// draftResponse is the payload returned by createDraft + PublishDraft +
// UpdateDraft. Validation issues from the preflight are embedded so
// the editor UI can show them inline without a separate /validate
// round-trip.
type draftResponse struct {
	socialPostResponse
	Validation platform.ValidationResult `json:"validation"`
}

// createDraft persists a row in status='draft' and returns it. No
// adapter dispatch, no webhook fired, no quota charged. The validator
// is run for its diagnostic value but its errors are NEVER fatal at
// draft creation time — drafts are an editing surface where the user
// is allowed to save broken content and fix it later.
func (h *SocialPostHandler) createDraft(
	w http.ResponseWriter,
	r *http.Request,
	projectID string,
	parsed parsedRequest,
	validation platform.ValidationResult,
) {
	metaJSON, err := platform.EncodePostMetadata(parsed.Posts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode metadata")
		return
	}

	canonicalCaption := pgtype.Text{}
	canonicalMedia := []string{}
	if len(parsed.Posts) > 0 {
		if parsed.Posts[0].Caption != "" {
			canonicalCaption = pgtype.Text{String: parsed.Posts[0].Caption, Valid: true}
		}
		if parsed.Posts[0].MediaURLs != nil {
			canonicalMedia = parsed.Posts[0].MediaURLs
		}
	}

	scheduledAt := pgtype.Timestamptz{}
	if parsed.ScheduledAt != nil {
		scheduledAt = pgtype.Timestamptz{Time: *parsed.ScheduledAt, Valid: true}
	}

	post, err := h.queries.CreateSocialPost(r.Context(), db.CreateSocialPostParams{
		ProjectID:      projectID,
		Caption:        canonicalCaption,
		MediaUrls:      canonicalMedia,
		Status:         "draft",
		Metadata:       metaJSON,
		ScheduledAt:    scheduledAt,
		IdempotencyKey: idempotencyKeyParam(parsed.IdempotencyKey),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create draft")
		return
	}

	resp := draftResponse{
		socialPostResponse: socialPostResponseFromRow(post),
		Validation:         validation,
	}
	writeCreated(w, resp)
}

// PublishDraft handles POST /v1/social-posts/{id}/publish. Atomically
// flips a draft → publishing via ClaimDraftForPublish (optimistic
// lock — losers see 0 rows and get 409). Then re-decodes the v2
// metadata and runs the same publish loop the immediate path uses,
// so quota counting, event emission, and per-result caption
// persistence stay in one place.
func (h *SocialPostHandler) PublishDraft(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetProjectID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}
	postID := chi.URLParam(r, "id")

	// Optimistic lock — only one caller can win the draft → publishing
	// transition. The loser sees pgx.ErrNoRows.
	claimed, err := h.queries.ClaimDraftForPublish(r.Context(), db.ClaimDraftForPublishParams{
		ID:        postID,
		ProjectID: projectID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			// Either the post isn't a draft anymore (already
			// published or being published by another worker) or
			// it doesn't exist / belong to this project. Both map
			// to 409 — the resource isn't in a publishable state.
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is not a draft (already publishing, published, or not found in this project)")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to claim draft")
		return
	}

	// Re-hydrate the parsed request shape from the persisted v2
	// metadata so we can hand it to createImmediatePost. fallback
	// caption is the parent post's caption (used only for v1 rows
	// that pre-date Sprint 1).
	fallbackCaption := ""
	if claimed.Caption.Valid {
		fallbackCaption = claimed.Caption.String
	}
	posts, decErr := platform.DecodePostMetadata(claimed.Metadata, fallbackCaption)
	if decErr != nil || len(posts) == 0 {
		// Roll the draft back to its original status so the user can
		// edit it. Without this they'd be stuck in 'publishing' with
		// nothing to publish.
		_ = h.rollbackToDraft(r, claimed.ID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Draft has no platform_posts to publish")
		return
	}

	parsed := parsedRequest{
		Posts:       posts,
		ScheduledAt: nil, // publish-from-draft is always immediate
	}

	// Load accounts once so the validator and the publish loop see
	// the same view.
	accountMap, err := h.loadValidateAccounts(r, projectID)
	if err != nil {
		_ = h.rollbackToDraft(r, claimed.ID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		return
	}

	// Re-run the validator with strict (fatal) gating — same as a
	// fresh immediate publish. We don't want to dispatch a draft
	// whose content was edited into something invalid after creation.
	vr := platform.ValidatePlatformPosts(platform.ValidateOptions{
		Capabilities: platform.Capabilities,
		Accounts:     accountMap,
		Posts:        posts,
	})
	if fatal := filterFatalIssues(vr.Errors); len(fatal) > 0 {
		_ = h.rollbackToDraft(r, claimed.ID)
		writeValidationErrors(w, fatal)
		return
	}

	// Reuse the existing publish loop. createImmediatePost will
	// re-create the parent row, but we already have one — instead,
	// inline the same logic against the claimed row. To minimize
	// duplication we add a thin variant that takes an existing row.
	h.publishExistingPost(w, r, projectID, claimed, parsed, accountMap)
}

// rollbackToDraft is the inverse of ClaimDraftForPublish. Used when
// publish fails BEFORE we've fanned out to any platform — we want
// the user to be able to edit + retry, not be stuck in 'publishing'.
func (h *SocialPostHandler) rollbackToDraft(r *http.Request, postID string) error {
	return h.queries.UpdateSocialPostStatus(r.Context(), db.UpdateSocialPostStatusParams{
		ID:          postID,
		Status:      "draft",
		PublishedAt: pgtype.Timestamptz{},
	})
}

// UpdateDraft handles PATCH /v1/social-posts/{id}. Replaces the
// draft's platform_posts / scheduled_at in one shot. Refuses to touch
// non-draft rows (the SQL query has the same WHERE clause for safety).
func (h *SocialPostHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetProjectID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}
	postID := chi.URLParam(r, "id")

	var body publishRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	parsed, status, msg := parsePublishRequest(body)
	if status != 0 {
		writeError(w, status, "VALIDATION_ERROR", msg)
		return
	}

	metaJSON, err := platform.EncodePostMetadata(parsed.Posts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode metadata")
		return
	}

	canonicalCaption := pgtype.Text{}
	canonicalMedia := []string{}
	if len(parsed.Posts) > 0 {
		if parsed.Posts[0].Caption != "" {
			canonicalCaption = pgtype.Text{String: parsed.Posts[0].Caption, Valid: true}
		}
		if parsed.Posts[0].MediaURLs != nil {
			canonicalMedia = parsed.Posts[0].MediaURLs
		}
	}
	scheduledAt := pgtype.Timestamptz{}
	if parsed.ScheduledAt != nil {
		scheduledAt = pgtype.Timestamptz{Time: *parsed.ScheduledAt, Valid: true}
	}

	updated, err := h.queries.UpdateDraftContent(r.Context(), db.UpdateDraftContentParams{
		ID:          postID,
		ProjectID:   projectID,
		Caption:     canonicalCaption,
		MediaUrls:   canonicalMedia,
		Metadata:    metaJSON,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is not a draft or does not exist in this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update draft")
		return
	}

	// Re-run validation against the new content so the editor sees
	// fresh diagnostics in the response.
	accountMap, _ := h.loadValidateAccounts(r, projectID)
	vr := platform.ValidatePlatformPosts(platform.ValidateOptions{
		Capabilities: platform.Capabilities,
		Accounts:     accountMap,
		Posts:        parsed.Posts,
		ScheduledAt:  parsed.ScheduledAt,
	})

	resp := draftResponse{
		socialPostResponse: socialPostResponseFromRow(updated),
		Validation:         vr,
	}
	writeSuccess(w, resp)
}

// socialPostResponseFromRow hydrates the response shape from a DB row.
// Shared between draft endpoints and the publish-replay path. Doesn't
// load results — drafts have none and the publish path overwrites
// this with the live results anyway.
func socialPostResponseFromRow(post db.SocialPost) socialPostResponse {
	resp := socialPostResponse{
		ID:        post.ID,
		Status:    post.Status,
		CreatedAt: post.CreatedAt.Time,
	}
	if post.Caption.Valid {
		c := post.Caption.String
		resp.Caption = &c
	}
	if post.PublishedAt.Valid {
		t := post.PublishedAt.Time
		resp.PublishedAt = &t
	}
	if post.ScheduledAt.Valid {
		t := post.ScheduledAt.Time
		resp.ScheduledAt = &t
	}
	return resp
}

// _ keeps the time import alive even when this file's helpers don't
// directly reference it; pgtype.Timestamptz embeds time.Time.
var _ = time.Time{}
