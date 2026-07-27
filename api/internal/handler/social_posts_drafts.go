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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/quotaemail"
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
	h.syncPostMediaRetention(r.Context(), post, post.Status)

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
	h.logPublishingEvent(r.Context(), integrationlogs.Event{
		WorkspaceID: workspaceID,
		Level:       integrationlogs.LevelInfo,
		Status:      integrationlogs.StatusSuccess,
		Action:      integrationlogs.ActionPostPublishStarted,
		Message:     "Started draft publish workflow.",
		PostID:      postID,
		Metadata: map[string]any{
			"mode": "draft_publish",
		},
	})

	// Admission — request rate first; if accepted, also count this
	// publish toward enqueue / depth. depthUnits is conservative at
	// 1 here because the per-platform fan-out count requires
	// decoding the metadata, which we do below; over-counting (i.e.
	// not penalizing fan-out enough) is preferable to paying the
	// decode cost twice. The published draft commonly fans out to
	// 1–3 jobs, so the under-count is small.
	if !h.admit(w, r, workspaceID, "POST /v1/posts/{id}/publish", admissionOpts{
		request:      true,
		enqueue:      true,
		depth:        true,
		enqueueUnits: 1,
		depthUnits:   1,
	}) {
		return
	}

	var response socialPostResponse
	var queuedPosts []platform.PlatformPostInput
	var failureKind string
	var fatalIssues []platform.Issue
	var restrictedDecision publishingrestrictions.Decision
	var blockedQuota quota.QuotaStatus
	var blockedQuotaUnits int
	var ordinaryRetentionPost db.SocialPost
	var ordinaryRetentionStatus string
	var syncOrdinaryRetention bool
	var preclaim db.SocialPost
	if h.paidSchedule != nil {
		var preclaimErr error
		preclaim, preclaimErr = h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{ID: postID, WorkspaceID: workspaceID})
		if preclaimErr != nil {
			writeError(w, http.StatusConflict, "CONFLICT", "Post is not available for publishing")
			return
		}
	}
	executionPeriod := quota.PeriodForTime(time.Now().UTC())
	err := h.queries.WithTransaction(r.Context(), func(txQueries *db.Queries) error {
		if h.paidSchedule != nil {
			for _, period := range scheduledQuotaLockPeriods(preclaim, executionPeriod) {
				if lockErr := txQueries.LockScheduledQuotaPeriod(r.Context(), workspaceID, period); lockErr != nil {
					return lockErr
				}
			}
		}
		txHandler := h.withQueueQueries(txQueries)
		claimed, claimErr := txQueries.ClaimDraftForPublish(r.Context(), db.ClaimDraftForPublishParams{
			ID:          postID,
			WorkspaceID: workspaceID,
		})
		if claimErr != nil {
			failureKind = "claim"
			return claimErr
		}
		if h.paidSchedule != nil && !sameScheduledQuotaPeriods(
			scheduledQuotaLockPeriods(preclaim, executionPeriod),
			scheduledQuotaLockPeriods(claimed, executionPeriod),
		) {
			failureKind = "claim"
			return errors.New("post period changed while acquiring quota locks")
		}

		fallbackCaption := ""
		if claimed.Caption.Valid {
			fallbackCaption = claimed.Caption.String
		}
		posts, decodeErr := platform.DecodePostMetadata(claimed.Metadata, fallbackCaption)
		if decodeErr != nil || len(posts) == 0 {
			failureKind = "metadata"
			if decodeErr != nil {
				return decodeErr
			}
			return errors.New("draft has no platform_posts")
		}
		queuedPosts = posts

		accountMap, accountErr := txHandler.loadValidateAccounts(r, workspaceID)
		if accountErr != nil {
			failureKind = "accounts"
			return accountErr
		}
		validation := txHandler.runPublishValidation(r, workspaceID, posts, nil, accountMap)
		fatalIssues = filterFatalIssues(validation.Errors)
		if len(fatalIssues) > 0 {
			failureKind = "validation"
			return errors.New("draft publish validation failed")
		}

		parsed := parsedRequest{Posts: posts}
		blockedTargets, policyErr := txHandler.evaluatePublishingRestrictions(r.Context(), workspaceID, posts, accountMap)
		if policyErr != nil {
			failureKind = "policy"
			return policyErr
		}
		if decision, fullyBlocked := fullyRestrictedDecision(posts, blockedTargets); fullyBlocked {
			failureKind = "restricted"
			restrictedDecision = decision
			return errors.New("draft publish fully restricted")
		}
		allowedTargets := allowedPublishingTargets(posts, blockedTargets)
		blockedQuotaUnits = countPublishQuotaUnits(allowedTargets, accountMap)
		additionalUnits := blockedQuotaUnits
		if claimed.QuotaHoldReason.Valid {
			additionalUnits = scheduledExecutionAdditionalQuotaUnits(claimed, posts, accountMap, blockedQuotaUnits, executionPeriod)
		}
		if status, blocked := txHandler.checkFreePlanPostQuotaForPeriod(r.Context(), workspaceID, additionalUnits, executionPeriod); blocked {
			failureKind = "quota"
			blockedQuota = status
			return errors.New("draft publish quota blocked")
		}
		if reservationErr := txQueries.SetScheduledExecutionReservationPeriod(r.Context(), claimed.ID, executionPeriod); reservationErr != nil {
			failureKind = "reservation"
			return reservationErr
		}

		claimed.ProfileIds = txHandler.ensureProfileIDsForPost(r.Context(), claimed, uniqueAccountIDs(posts))
		var enqueueErr error
		response, enqueueErr = txHandler.enqueueExistingPostDeliveriesInTransaction(
			withoutPostMediaRetentionSync(r.Context()), claimed, parsed.Posts, accountMap, blockedTargets,
		)
		if enqueueErr != nil {
			failureKind = "enqueue"
			return enqueueErr
		}
		if !hasRestrictedPublishingTarget(blockedTargets) {
			ordinaryRetentionPost = claimed
			ordinaryRetentionStatus = "publishing"
			if response.ActiveJobCount == 0 {
				ordinaryRetentionStatus = "failed"
			}
			syncOrdinaryRetention = true
		}
		return nil
	})
	if err != nil {
		switch failureKind {
		case "claim":
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusConflict, "CONFLICT", "Post is not a draft (already publishing, published, or not found in this workspace)")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to claim draft")
		case "metadata":
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Draft has no platform_posts to publish")
		case "accounts":
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		case "validation":
			writeValidationErrors(w, fatalIssues)
		case "policy":
			writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Publishing policy is temporarily unavailable")
		case "restricted":
			writePublishingRestrictionError(w, restrictedDecision)
		case "quota":
			h.maybeSendFreePlanQuotaEmail(r.Context(), workspaceID, quotaemail.Evaluation{Blocked: true, RequestedUnits: blockedQuotaUnits})
			writeFreePlanPostQuotaExceeded(w, blockedQuota, blockedQuotaUnits)
		case "reservation":
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reserve publishing quota")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	if syncOrdinaryRetention {
		h.syncPostMediaRetention(r.Context(), ordinaryRetentionPost, ordinaryRetentionStatus)
	}
	h.logQueuedPost(r.Context(), workspaceID, postID, "draft_publish", queuedPosts, len(response.Results), response.ActiveJobCount)
	writeAccepted(w, response)
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

func rollbackStatusForClaimedPost(post db.SocialPost) string {
	if post.QuotaHoldReason.Valid {
		return "quota_hold"
	}
	return "draft"
}

func (h *SocialPostHandler) rollbackClaimedPost(r *http.Request, post db.SocialPost) error {
	return h.queries.UpdateSocialPostStatus(r.Context(), db.UpdateSocialPostStatusParams{
		ID:          post.ID,
		Status:      rollbackStatusForClaimedPost(post),
		PublishedAt: pgtype.Timestamptz{},
	})
}

func (h *SocialPostHandler) rollbackDraftAndWriteFreePlanQuotaError(w http.ResponseWriter, r *http.Request, postID string, status quota.QuotaStatus, requestedUnits int) {
	h.rollbackClaimedAndWriteFreePlanQuotaError(w, r, db.SocialPost{ID: postID}, status, requestedUnits)
}

func (h *SocialPostHandler) rollbackClaimedAndWriteFreePlanQuotaError(w http.ResponseWriter, r *http.Request, post db.SocialPost, status quota.QuotaStatus, requestedUnits int) {
	if err := h.rollbackClaimedPost(r, post); err != nil {
		postID := post.ID
		slog.Error("publish draft: failed to roll back after quota rejection", "post_id", postID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to restore draft after quota check. Please refresh and contact support if the draft is not editable.")
		return
	}
	writeFreePlanPostQuotaExceeded(w, status, requestedUnits)
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
	h.syncPostMediaRetention(r.Context(), cancelled, cancelled.Status)
	h.maybeReconcileQuotaHolds(r.Context(), workspaceID, "capacity_released")
	if cancelled.ScheduledAt.Valid {
		h.maybeEvaluatePaidQuota(r.Context(), workspaceID, quota.PeriodForTime(cancelled.ScheduledAt.Time))
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
//
// The caller (UpdateDraft) has already consumed r.Body via io.ReadAll
// to drive the lifecycle / draft branches, so we decode from rawBody
// here rather than re-reading r.Body (which is now exhausted).
func (h *SocialPostHandler) reschedulePost(w http.ResponseWriter, r *http.Request, workspaceID string, postID string, rawBody []byte) {
	var body struct {
		ScheduledAt *string `json:"scheduled_at"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
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

	existing, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is no longer scheduled (already publishing or published)")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load scheduled post")
		return
	}
	if h.publishingRestrictions != nil {
		existingPosts, decodeErr := platform.DecodePostMetadata(existing.Metadata, derefText(existing.Caption))
		if decodeErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decode scheduled post")
			return
		}
		accountMap, loadErr := h.loadValidateAccounts(r, workspaceID)
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
			return
		}
		blockedTargets, policyErr := h.evaluatePublishingRestrictions(r.Context(), workspaceID, existingPosts, accountMap)
		if policyErr != nil {
			writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Publishing policy is temporarily unavailable")
			return
		}
		if decision, fullyBlocked := fullyRestrictedDecision(existingPosts, blockedTargets); fullyBlocked {
			writePublishingRestrictionError(w, decision)
			return
		}
	}

	var updated db.SocialPost
	mutate := func(queries *db.Queries) error {
		var mutationErr error
		updated, mutationErr = queries.RescheduleSocialPost(r.Context(), db.RescheduleSocialPostParams{
			ID:          postID,
			WorkspaceID: workspaceID,
			ScheduledAt: pgtype.Timestamptz{Time: t, Valid: true},
		})
		return mutationErr
	}
	if h.paidSchedule != nil && existing.ScheduledAt.Valid {
		oldPeriod := quota.PeriodForTime(existing.ScheduledAt.Time)
		newPeriod := quota.PeriodForTime(t)
		err = h.paidSchedule.MutatePlanned(
			r.Context(),
			workspaceID,
			[]string{oldPeriod, newPeriod},
			func(queries *db.Queries) ([]paidquota.PeriodDelta, error) {
				lockedPost, loadErr := queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
					ID:          postID,
					WorkspaceID: workspaceID,
				})
				if loadErr != nil {
					return nil, loadErr
				}
				accounts, loadErr := loadValidateAccountsWithQueries(r.Context(), queries, workspaceID)
				if loadErr != nil {
					return nil, loadErr
				}
				oldPosts, loadErr := platform.DecodePostMetadata(lockedPost.Metadata, derefText(lockedPost.Caption))
				if loadErr != nil {
					return nil, loadErr
				}
				deltas, loadErr := paidScheduleEditDeltas(lockedPost, oldPosts, t, accounts, lockedPost.Metadata)
				if loadErr != nil {
					return nil, loadErr
				}
				if loadErr := validateFreeScheduleEditDeltas(r.Context(), queries, workspaceID, deltas); loadErr != nil {
					return nil, loadErr
				}
				return deltas, nil
			},
			mutate,
		)
	} else {
		err = mutate(h.queries)
	}
	if err != nil {
		var freeAdmissionErr *freeScheduleEditAdmissionError
		if errors.As(err, &freeAdmissionErr) {
			writeFreePlanPostQuotaExceeded(w, freeAdmissionErr.status, freeAdmissionErr.requestedUnits)
			return
		}
		var admissionErr *paidquota.AdmissionError
		if errors.As(err, &admissionErr) {
			writePaidPlanSchedulingCapacityExceeded(w, admissionErr)
			return
		}
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is no longer scheduled (already publishing or published)")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reschedule post")
		return
	}
	h.maybeSendFreePlanQuotaEmail(r.Context(), workspaceID, quotaemail.Evaluation{
		Period: quota.PeriodForTime(t),
	})
	h.maybeReconcileQuotaHolds(r.Context(), workspaceID, "schedule_rescheduled")
	if existing.ScheduledAt.Valid {
		h.maybeEvaluatePaidQuota(r.Context(), workspaceID, quota.PeriodForTime(existing.ScheduledAt.Time))
	}
	h.maybeEvaluatePaidQuota(r.Context(), workspaceID, quota.PeriodForTime(t))
	writeSuccess(w, socialPostResponseFromRow(updated))
}

func isScheduledAtOnlyPatch(rawBody []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &fields); err != nil {
		return false
	}
	if len(fields) != 1 {
		return false
	}
	_, ok := fields["scheduled_at"]
	return ok
}

func canEditSocialPostContent(status string) bool {
	return status == "draft" || status == "scheduled" || status == "quota_hold"
}

func buildSocialPostContentUpdateParams(
	postID string,
	workspaceID string,
	posts []platform.PlatformPostInput,
	metaJSON []byte,
	scheduledAt *time.Time,
	profileIDs []string,
) db.UpdateDraftContentParams {
	canonicalCaption := pgtype.Text{}
	canonicalMedia := []string{}
	if len(posts) > 0 {
		if posts[0].Caption != "" {
			canonicalCaption = pgtype.Text{String: posts[0].Caption, Valid: true}
		}
		if posts[0].MediaURLs != nil {
			canonicalMedia = posts[0].MediaURLs
		}
	}
	scheduledAtParam := pgtype.Timestamptz{}
	if scheduledAt != nil {
		scheduledAtParam = pgtype.Timestamptz{Time: *scheduledAt, Valid: true}
	}
	return db.UpdateDraftContentParams{
		ID:          postID,
		WorkspaceID: workspaceID,
		Caption:     canonicalCaption,
		MediaUrls:   canonicalMedia,
		Metadata:    metaJSON,
		ScheduledAt: scheduledAtParam,
		ProfileIds:  profileIDs,
	}
}

func paidScheduleEditDeltas(
	existing db.SocialPost,
	newPosts []platform.PlatformPostInput,
	newScheduledAt time.Time,
	accountMap map[string]platform.ValidateAccount,
	newMetadata ...[]byte,
) ([]paidquota.PeriodDelta, error) {
	if (existing.Status != "scheduled" && existing.Status != "quota_hold") || !existing.ScheduledAt.Valid {
		return nil, fmt.Errorf("post is not an active scheduled post")
	}
	oldPosts, err := platform.DecodePostMetadata(existing.Metadata, derefText(existing.Caption))
	if err != nil {
		return nil, err
	}
	oldUnits := countPublishQuotaUnits(oldPosts, accountMap)
	if snapshotUnits, ok := scheduledQuotaUnitsFromMetadata(existing.Metadata); ok {
		oldUnits = snapshotUnits
	}
	newUnits := countPublishQuotaUnits(newPosts, accountMap)
	if len(newMetadata) > 0 {
		if snapshotUnits, ok := scheduledQuotaUnitsFromMetadata(newMetadata[0]); ok {
			newUnits = snapshotUnits
		}
	}
	oldPeriod := quota.PeriodForTime(existing.ScheduledAt.Time)
	newPeriod := quota.PeriodForTime(newScheduledAt)
	if oldPeriod == newPeriod {
		return []paidquota.PeriodDelta{{
			Period:         oldPeriod,
			ReleasedUnits:  oldUnits,
			RequestedUnits: newUnits,
		}}, nil
	}
	return []paidquota.PeriodDelta{
		{Period: oldPeriod, ReleasedUnits: oldUnits},
		{Period: newPeriod, RequestedUnits: newUnits},
	}, nil
}

type freeScheduleEditAdmissionError struct {
	status         quota.QuotaStatus
	requestedUnits int
}

func (e *freeScheduleEditAdmissionError) Error() string {
	return "free schedule edit would exceed monthly post quota"
}

func validateFreeScheduleEditDeltas(
	ctx context.Context,
	queries *db.Queries,
	workspaceID string,
	deltas []paidquota.PeriodDelta,
) error {
	checker := quota.NewChecker(queries)
	for _, delta := range deltas {
		snapshot, err := checker.MonthlySnapshotForPeriod(ctx, workspaceID, delta.Period)
		if err != nil {
			return err
		}
		if snapshot.PlanID != "free" || delta.RequestedUnits <= delta.ReleasedUnits || !snapshot.WouldExceed(delta.ReleasedUnits, delta.RequestedUnits) {
			continue
		}
		return &freeScheduleEditAdmissionError{
			status: quota.QuotaStatus{
				Usage:    snapshot.Completed,
				Reserved: snapshot.Scheduled,
				Limit:    snapshot.Limit,
			},
			requestedUnits: max(delta.RequestedUnits-delta.ReleasedUnits, 1),
		}
	}
	return nil
}

func (h *SocialPostHandler) updateEditablePostContent(
	ctx context.Context,
	workspaceID string,
	existing db.SocialPost,
	newPosts []platform.PlatformPostInput,
	newScheduledAt time.Time,
	params db.UpdateDraftContentParams,
) (db.SocialPost, error) {
	var updated db.SocialPost
	mutate := func(queries *db.Queries) error {
		var err error
		updated, err = queries.UpdateDraftContent(ctx, params)
		return err
	}
	if h.paidSchedule == nil ||
		(existing.Status != "scheduled" && existing.Status != "quota_hold") ||
		!existing.ScheduledAt.Valid {
		err := mutate(h.queries)
		return updated, err
	}

	oldPeriod := quota.PeriodForTime(existing.ScheduledAt.Time)
	newPeriod := quota.PeriodForTime(newScheduledAt)
	err := h.paidSchedule.MutatePlanned(
		ctx,
		workspaceID,
		[]string{oldPeriod, newPeriod},
		func(queries *db.Queries) ([]paidquota.PeriodDelta, error) {
			lockedPost, err := queries.GetSocialPostByIDAndWorkspace(ctx, db.GetSocialPostByIDAndWorkspaceParams{
				ID:          existing.ID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				return nil, err
			}
			accounts, err := loadValidateAccountsWithQueries(ctx, queries, workspaceID)
			if err != nil {
				return nil, err
			}
			deltas, err := paidScheduleEditDeltas(lockedPost, newPosts, newScheduledAt, accounts, params.Metadata)
			if err != nil {
				return nil, err
			}
			if err := validateFreeScheduleEditDeltas(ctx, queries, workspaceID, deltas); err != nil {
				return nil, err
			}
			return deltas, nil
		},
		mutate,
	)
	return updated, err
}

// CancelPost handles POST /v1/posts/{id}/cancel. Allowed for
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
	if !h.admit(w, r, workspaceID, "POST /v1/posts/{id}/cancel", admissionOpts{request: true}) {
		return
	}
	postID := chi.URLParam(r, "id")
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Sunset", "Tue, 31 Mar 2027 00:00:00 GMT")
	w.Header().Set("Link", `</v1/posts/`+postID+`>; rel="successor-version"`)
	h.cancelSocialPost(w, r, workspaceID, postID)
}

// UpdateDraft handles PATCH /v1/posts/{id} for drafts and scheduled
// posts. The state machine:
//
//   - status='draft'     → caption / media / metadata / scheduled_at all editable
//   - status='scheduled' → caption / media / metadata / scheduled_at all editable
//   - any other status   → 409 (already publishing or done)
//
// The content update uses an optimistic status guard so a row that
// flipped to 'publishing' between the read and the write loses cleanly.
func (h *SocialPostHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if !h.admit(w, r, workspaceID, "PATCH /v1/posts/{id}", admissionOpts{request: true}) {
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

	// Backward-compatible reschedule branch used by the list view and
	// SDK helpers. Full scheduled-post edits include platform_posts or
	// account_ids and go through the content update branch below.
	if (existing.Status == "scheduled" || existing.Status == "quota_hold") && isScheduledAtOnlyPatch(rawBody) {
		h.reschedulePost(w, r, workspaceID, postID, rawBody)
		return
	}
	if !canEditSocialPostContent(existing.Status) {
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
	accountMap, err := h.loadValidateAccounts(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		return
	}
	parsed.resolveLegacyPlatformOptions(accountMap)

	var metaJSON []byte
	if existing.Status == "scheduled" || existing.Status == "quota_hold" {
		blockedTargets, policyErr := h.evaluatePublishingRestrictions(r.Context(), workspaceID, parsed.Posts, accountMap)
		if policyErr != nil {
			writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Publishing policy is temporarily unavailable")
			return
		}
		if decision, fullyBlocked := fullyRestrictedDecision(parsed.Posts, blockedTargets); fullyBlocked {
			writePublishingRestrictionError(w, decision)
			return
		}
		allowedTargets := allowedPublishingTargets(parsed.Posts, blockedTargets)
		quotaAccountIDs := publishQuotaAccountIDs(allowedTargets, accountMap)
		metaJSON, err = encodeScheduledPostMetadataWithQuotaAccounts(parsed.Posts, len(quotaAccountIDs), quotaAccountIDs)
	} else {
		metaJSON, err = platform.EncodePostMetadata(parsed.Posts)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode metadata")
		return
	}

	scheduledAt := parsed.ScheduledAt
	if scheduledAt == nil &&
		(existing.Status == "scheduled" || existing.Status == "quota_hold") &&
		existing.ScheduledAt.Valid {
		scheduledAt = &existing.ScheduledAt.Time
	}

	updateParams := buildSocialPostContentUpdateParams(
		postID,
		workspaceID,
		parsed.Posts,
		metaJSON,
		scheduledAt,
		h.resolveProfileIDs(r.Context(), workspaceID, uniqueAccountIDs(parsed.Posts)),
	)
	var updated db.SocialPost
	if (existing.Status == "scheduled" || existing.Status == "quota_hold") && scheduledAt != nil {
		updated, err = h.updateEditablePostContent(
			r.Context(),
			workspaceID,
			existing,
			parsed.Posts,
			*scheduledAt,
			updateParams,
		)
	} else {
		updated, err = h.queries.UpdateDraftContent(r.Context(), updateParams)
	}
	if err != nil {
		var freeAdmissionErr *freeScheduleEditAdmissionError
		if errors.As(err, &freeAdmissionErr) {
			writeFreePlanPostQuotaExceeded(w, freeAdmissionErr.status, freeAdmissionErr.requestedUnits)
			return
		}
		var admissionErr *paidquota.AdmissionError
		if errors.As(err, &admissionErr) {
			writePaidPlanSchedulingCapacityExceeded(w, admissionErr)
			return
		}
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusConflict, "CONFLICT",
				"Post is no longer editable or does not exist in this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update post")
		return
	}
	if updated.Status == "scheduled" && updated.ScheduledAt.Valid {
		h.maybeSendFreePlanQuotaEmail(r.Context(), workspaceID, quotaemail.Evaluation{
			Period: quota.PeriodForTime(updated.ScheduledAt.Time),
		})
	}
	h.syncPostMediaRetention(r.Context(), updated, updated.Status)
	if existing.Status == "scheduled" || existing.Status == "quota_hold" {
		h.maybeReconcileQuotaHolds(r.Context(), workspaceID, "scheduled_destinations_changed")
		if existing.ScheduledAt.Valid {
			h.maybeEvaluatePaidQuota(r.Context(), workspaceID, quota.PeriodForTime(existing.ScheduledAt.Time))
		}
		if updated.ScheduledAt.Valid {
			h.maybeEvaluatePaidQuota(r.Context(), workspaceID, quota.PeriodForTime(updated.ScheduledAt.Time))
		}
	}

	// Re-run validation against the new content so the editor sees
	// fresh diagnostics in the response.
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
		ID:            post.ID,
		MediaURLs:     post.MediaUrls,
		Status:        post.Status,
		CreatedAt:     post.CreatedAt.Time,
		Source:        post.Source,
		ProfileIDs:    post.ProfileIds,
		PlatformPosts: buildEditablePlatformPosts(post.Metadata, derefText(post.Caption)),
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
	if post.QuotaHoldReason.Valid {
		reason := post.QuotaHoldReason.String
		resp.QuotaHoldReason = &reason
	}
	if post.QuotaHoldAt.Valid {
		t := post.QuotaHoldAt.Time
		resp.QuotaHoldAt = &t
	}
	if post.QuotaHoldOriginalScheduledAt.Valid {
		t := post.QuotaHoldOriginalScheduledAt.Time
		resp.QuotaHoldOriginalScheduledAt = &t
	}
	return resp
}

// _ keeps the time import alive even when this file's helpers don't
// directly reference it; pgtype.Timestamptz embeds time.Time.
var _ = time.Time{}
