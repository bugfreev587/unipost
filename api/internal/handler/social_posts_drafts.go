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
	"fmt"
	"io"
	"net/http"
	"strings"
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
	workspaceID string,
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
		WorkspaceID:    workspaceID,
		Caption:        canonicalCaption,
		MediaUrls:      canonicalMedia,
		Status:         "draft",
		Metadata:       metaJSON,
		ScheduledAt:    scheduledAt,
		IdempotencyKey: idempotencyKeyParam(parsed.IdempotencyKey),
		Source:         resolveSource(r.Context()),
		ProfileIds:     h.resolveProfileIDs(r.Context(), workspaceID, uniqueAccountIDs(parsed.Posts)),
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
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	postID := chi.URLParam(r, "id")

	// Optimistic lock — only one caller can win the draft → publishing
	// transition. The loser sees pgx.ErrNoRows.
	claimed, err := h.queries.ClaimDraftForPublish(r.Context(), db.ClaimDraftForPublishParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			// Either the post isn't a draft anymore (already
			// published or being published by another worker) or
			// it doesn't exist / belong to this workspace. Both map
			// to 409 — the resource isn't in a publishable state.
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is not a draft (already publishing, published, or not found in this workspace)")
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
	accountMap, err := h.loadValidateAccounts(r, workspaceID)
	if err != nil {
		_ = h.rollbackToDraft(r, claimed.ID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		return
	}

	// Re-run the validator with strict (fatal) gating — same as a
	// fresh immediate publish. We don't want to dispatch a draft
	// whose content was edited into something invalid after creation.
	vr := h.runPublishValidation(r, workspaceID, posts, nil, accountMap)
	if fatal := filterFatalIssues(vr.Errors); len(fatal) > 0 {
		_ = h.rollbackToDraft(r, claimed.ID)
		writeValidationErrors(w, fatal)
		return
	}

	// Reuse the existing publish loop. createImmediatePost will
	// re-create the parent row, but we already have one — instead,
	// inline the same logic against the claimed row. To minimize
	// duplication we add a thin variant that takes an existing row.
	h.publishExistingPost(w, r, workspaceID, claimed, parsed, accountMap)
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

type socialPostLifecyclePatch struct {
	Archived *bool
	Status   *string
}

func parseSocialPostLifecyclePatch(raw []byte) (socialPostLifecyclePatch, bool, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return socialPostLifecyclePatch{}, false, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return socialPostLifecyclePatch{}, false, err
	}

	archivedRaw, hasArchived := fields["archived"]
	statusRaw, hasStatus := fields["status"]
	if !hasArchived && !hasStatus {
		return socialPostLifecyclePatch{}, false, nil
	}
	if len(fields) > 1 {
		if hasArchived && len(fields) > 1 {
			return socialPostLifecyclePatch{}, true, fmt.Errorf("archived lifecycle patch cannot be combined with other fields")
		}
		if hasStatus && len(fields) > 1 {
			return socialPostLifecyclePatch{}, true, fmt.Errorf("status lifecycle patch cannot be combined with other fields")
		}
	}

	var patch socialPostLifecyclePatch
	if hasArchived {
		var archived bool
		if err := json.Unmarshal(archivedRaw, &archived); err != nil {
			return socialPostLifecyclePatch{}, true, fmt.Errorf("archived must be a boolean")
		}
		patch.Archived = &archived
		return patch, true, nil
	}

	var status string
	if err := json.Unmarshal(statusRaw, &status); err != nil {
		return socialPostLifecyclePatch{}, true, fmt.Errorf("status must be a string")
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "canceled", "cancelled":
		normalized := "canceled"
		patch.Status = &normalized
		return patch, true, nil
	default:
		return socialPostLifecyclePatch{}, true, fmt.Errorf("unsupported status transition: %s", status)
	}
}

func (h *SocialPostHandler) archiveSocialPost(w http.ResponseWriter, r *http.Request, workspaceID, postID string) {
	post, err := h.queries.ArchiveSocialPost(r.Context(), db.ArchiveSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to archive post")
		return
	}
	writeSuccess(w, socialPostResponseFromRow(post))
}

func (h *SocialPostHandler) restoreSocialPost(w http.ResponseWriter, r *http.Request, workspaceID, postID string) {
	post, err := h.queries.RestoreSocialPost(r.Context(), db.RestoreSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to restore post")
		return
	}
	writeSuccess(w, socialPostResponseFromRow(post))
}

func (h *SocialPostHandler) cancelSocialPost(w http.ResponseWriter, r *http.Request, workspaceID, postID string) {
	cancelled, err := h.queries.CancelSocialPost(r.Context(), db.CancelSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post cannot be cancelled (not a draft or scheduled post in this workspace)")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to cancel post")
		return
	}
	writeSuccess(w, socialPostResponseFromRow(cancelled))
}

func (h *SocialPostHandler) applyLifecyclePatch(w http.ResponseWriter, r *http.Request, workspaceID, postID string, patch socialPostLifecyclePatch) {
	if patch.Archived != nil {
		if *patch.Archived {
			h.archiveSocialPost(w, r, workspaceID, postID)
			return
		}
		h.restoreSocialPost(w, r, workspaceID, postID)
		return
	}
	if patch.Status != nil && *patch.Status == "canceled" {
		h.cancelSocialPost(w, r, workspaceID, postID)
		return
	}
	writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Unsupported lifecycle patch")
}

// reschedulePost handles the PATCH branch where the post is in
// status='scheduled'. Only scheduled_at is editable; the body must
// carry a future RFC3339 timestamp. Optimistic-locked: if the row
// already flipped to 'publishing' between the read and the write
// the UPDATE returns no rows and we 409.
func (h *SocialPostHandler) reschedulePost(w http.ResponseWriter, r *http.Request, workspaceID string, postID string) {
	var body struct {
		ScheduledAt *string `json:"scheduled_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.ScheduledAt == nil || *body.ScheduledAt == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"scheduled_at is required when rescheduling a scheduled post")
		return
	}
	t, err := time.Parse(time.RFC3339, *body.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"invalid scheduled_at: "+err.Error())
		return
	}
	// Buffer so the scheduler tick (which fires every minute) reliably
	// catches the new time without a race.
	if t.Before(time.Now().Add(60 * time.Second)) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"scheduled_at must be at least 60 seconds in the future")
		return
	}

	updated, err := h.queries.RescheduleSocialPost(r.Context(), db.RescheduleSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
		ScheduledAt: pgtype.Timestamptz{Time: t, Valid: true},
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is no longer scheduled (already publishing or published)")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reschedule post")
		return
	}
	writeSuccess(w, socialPostResponseFromRow(updated))
}

// CancelPost handles POST /v1/social-posts/{id}/cancel. Allowed for
// drafts and scheduled posts. Optimistic-locked the same way as
// reschedule. Cancelled rows are filtered out by the scheduler's
// WHERE status='scheduled' clause on the next tick — no further
// action required. No webhook fired (cancellation is a customer
// action, not a platform event).
func (h *SocialPostHandler) CancelPost(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	postID := chi.URLParam(r, "id")
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Sunset", "Tue, 31 Mar 2027 00:00:00 GMT")
	w.Header().Set("Link", `</v1/social-posts/`+postID+`>; rel="successor-version"`)
	h.cancelSocialPost(w, r, workspaceID, postID)
}

// UpdateDraft handles PATCH /v1/social-posts/{id} for both drafts and
// scheduled posts. The state machine:
//
//   - status='draft'     → caption / media / metadata / scheduled_at all editable
//   - status='scheduled' → ONLY scheduled_at editable (Sprint 3 PR8)
//   - any other status   → 409 (already publishing or done)
//
// The two paths use different SQL queries with their own optimistic
// locks so a row that flipped to 'publishing' between the read and
// the write loses cleanly.
func (h *SocialPostHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	postID := chi.URLParam(r, "id")
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if patch, ok, err := parseSocialPostLifecyclePatch(rawBody); ok {
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}
		h.applyLifecyclePatch(w, r, workspaceID, postID, patch)
		return
	}

	// Read current state once. We don't trust this for the actual
	// write — the locked UPDATE below has its own WHERE — but we use
	// it to dispatch between the draft-edit and reschedule branches.
	existing, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found in this workspace")
		return
	}

	// Sprint 3 PR8: scheduled-post reschedule branch. Only scheduled_at
	// is editable; all other fields are ignored.
	if existing.Status == "scheduled" {
		h.reschedulePost(w, r, workspaceID, postID)
		return
	}
	if existing.Status != "draft" {
		writeError(w, http.StatusConflict, "CONFLICT",
			"Post is "+existing.Status+" — only drafts and scheduled posts can be edited")
		return
	}

	var body publishRequestBody
	if err := json.Unmarshal(rawBody, &body); err != nil {
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
		WorkspaceID: workspaceID,
		Caption:     canonicalCaption,
		MediaUrls:   canonicalMedia,
		Metadata:    metaJSON,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is not a draft or does not exist in this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update draft")
		return
	}

	// Re-run validation against the new content so the editor sees
	// fresh diagnostics in the response.
	accountMap, _ := h.loadValidateAccounts(r, workspaceID)
	vr := h.runPublishValidation(r, workspaceID, parsed.Posts, parsed.ScheduledAt, accountMap)

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
		ID:         post.ID,
		Status:     post.Status,
		CreatedAt:  post.CreatedAt.Time,
		Source:     post.Source,
		ProfileIDs: post.ProfileIds,
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
	if post.ArchivedAt.Valid {
		t := post.ArchivedAt.Time
		resp.ArchivedAt = &t
	}
	return resp
}

// _ keeps the time import alive even when this file's helpers don't
// directly reference it; pgtype.Timestamptz embeds time.Time.
var _ = time.Time{}
