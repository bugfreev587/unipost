package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/quotaemail"
)

const (
	defaultDeliveryJobMaxAttempts = 5
	staleDeliveryAttemptTimeout   = 5 * time.Minute
)

var (
	deliveryAccessTokenParamPattern = regexp.MustCompile(`(?i)(access_token=)[^&\s"'<>]+`)
	deliveryBearerTokenPattern      = regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s"'<>]+`)
)

type postQueueSummary struct {
	PendingCount   int
	RunningCount   int
	RetryingCount  int
	DeadCount      int
	CancelledCount int
	SucceededCount int
	ActiveJobCount int
	QueuedCount    int
}

func summarizePostJobs(jobs []db.PostDeliveryJob) postQueueSummary {
	var s postQueueSummary
	for _, job := range jobs {
		switch job.State {
		case "pending":
			s.PendingCount++
			s.ActiveJobCount++
			s.QueuedCount++
		case "running":
			s.RunningCount++
			s.ActiveJobCount++
		case "retrying":
			s.RetryingCount++
			s.ActiveJobCount++
		case "dead":
			s.DeadCount++
		case "cancelled":
			s.CancelledCount++
		case "succeeded":
			s.SucceededCount++
		}
	}
	return s
}

func deriveSocialPostStatus(post db.SocialPost, results []db.SocialPostResult, jobs []db.PostDeliveryJob) string {
	summary := summarizePostJobs(jobs)
	switch {
	case summary.RetryingCount > 0:
		return "retrying"
	case summary.RunningCount > 0:
		return "dispatching"
	case summary.PendingCount > 0:
		return "queued"
	}
	for _, res := range results {
		if res.Status == "processing" || res.Status == "pending" {
			return "dispatching"
		}
	}
	return post.Status
}

func applyQueueSummary(resp *socialPostResponse, jobs []db.PostDeliveryJob) {
	summary := summarizePostJobs(jobs)
	resp.QueuedResultsCount = summary.QueuedCount
	resp.ActiveJobCount = summary.ActiveJobCount
	resp.RetryingCount = summary.RetryingCount
	resp.DeadCount = summary.DeadCount
}

func platformPostInputAtIndex(post db.SocialPost, idx int) (platform.PlatformPostInput, error) {
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	parsed, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil {
		return platform.PlatformPostInput{}, err
	}
	if idx < 0 || idx >= len(parsed) {
		return platform.PlatformPostInput{}, fmt.Errorf("post input index %d out of range", idx)
	}
	return parsed[idx], nil
}

func findPostInputIndexForResult(post db.SocialPost, result db.SocialPostResult) int {
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	parsed, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil {
		return -1
	}
	for i, pp := range parsed {
		if pp.AccountID == result.SocialAccountID && pp.Caption == result.Caption {
			return i
		}
	}
	for i, pp := range parsed {
		if pp.AccountID == result.SocialAccountID {
			return i
		}
	}
	return -1
}

func (h *SocialPostHandler) loadDBAccountsByIDs(ctx context.Context, workspaceID string, ids []string) map[string]db.SocialAccount {
	out := make(map[string]db.SocialAccount, len(ids))
	for _, id := range ids {
		acc, err := h.queries.GetSocialAccountByIDAndWorkspace(ctx, db.GetSocialAccountByIDAndWorkspaceParams{
			ID:          id,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			continue
		}
		out[id] = acc
	}
	return out
}

func summarizeAccountValidation(dbAcc db.SocialAccount, ok bool, fallback platform.ValidateAccount) (platformName string, err error) {
	// Preserve an invalid admission snapshot even if the account changes while
	// this request is being persisted. Otherwise a missing/disconnected target
	// could reconnect after policy and quota checks and become publishable here.
	platformName = fallback.Platform
	if fallback.Platform == "" {
		return platformName, fmt.Errorf("account not found")
	}
	if fallback.Disconnected {
		return platformName, fmt.Errorf("account is disconnected")
	}
	if ok {
		platformName = dbAcc.Platform
		if socialAccountDisconnectedForPublish(dbAcc, true) {
			return platformName, fmt.Errorf("account is disconnected")
		}
		return platformName, nil
	}
	return platformName, fmt.Errorf("account not found")
}

type queuedDeliveryEvaluation struct {
	account        db.SocialAccount
	platformName   string
	validationErr  error
	policyDecision publishingrestrictions.Decision
	quotaFailure   bool
}

func evaluateQueuedDeliveryTargets(
	parsed []platform.PlatformPostInput,
	dbAccounts map[string]db.SocialAccount,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
) []queuedDeliveryEvaluation {
	evaluations := make([]queuedDeliveryEvaluation, 0, len(parsed))
	for _, pp := range parsed {
		account, ok := dbAccounts[pp.AccountID]
		platformName, validationErr := summarizeAccountValidation(account, ok, accountMap[pp.AccountID])
		var decision publishingrestrictions.Decision
		if validationErr == nil {
			decision = blockedTargets[pp.AccountID]
			if decision.Restricted {
				validationErr = errors.New(publishingrestrictions.UserMessage)
			}
		}
		evaluations = append(evaluations, queuedDeliveryEvaluation{
			account:        account,
			platformName:   platformName,
			validationErr:  validationErr,
			policyDecision: decision,
		})
	}
	return evaluations
}

func hasRestrictedPublishingTarget(blockedTargets map[string]publishingrestrictions.Decision) bool {
	for _, decision := range blockedTargets {
		if decision.Restricted {
			return true
		}
	}
	return false
}

func (h *SocialPostHandler) withQueueQueries(queries *db.Queries) *SocialPostHandler {
	clone := *h
	clone.queries = queries
	if h.quota != nil {
		clone.quota = quota.NewChecker(queries)
	}
	return &clone
}

func (h *SocialPostHandler) enqueueParsedPostDeliveries(
	ctx context.Context,
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
) ([]db.SocialPostResult, []db.PostDeliveryJob, error) {
	return h.enqueueParsedPostDeliveriesWithQuotaBlocks(ctx, post, parsed, accountMap, blockedTargets, nil)
}

func (h *SocialPostHandler) enqueueParsedPostDeliveriesWithQuotaBlocks(
	ctx context.Context,
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
	quotaBlockedTargets map[int]string,
) ([]db.SocialPostResult, []db.PostDeliveryJob, error) {
	dbAccounts := h.loadDBAccountsByIDs(ctx, post.WorkspaceID, uniqueAccountIDs(parsed))
	evaluations := evaluateQueuedDeliveryTargets(parsed, dbAccounts, accountMap, blockedTargets)
	for index, message := range quotaBlockedTargets {
		if index < 0 || index >= len(evaluations) || evaluations[index].validationErr != nil {
			continue
		}
		evaluations[index].validationErr = errors.New(message)
		evaluations[index].quotaFailure = true
	}
	results := make([]db.SocialPostResult, 0, len(parsed))
	jobs := make([]db.PostDeliveryJob, 0, len(parsed))
	failureSummaries := make([]string, 0)
	var policyFailureAt time.Time

	for idx, pp := range parsed {
		evaluation := evaluations[idx]
		acc := evaluation.account
		platformName := evaluation.platformName
		validationErr := evaluation.validationErr
		policyDecision := evaluation.policyDecision
		if policyDecision.Restricted && policyFailureAt.IsZero() {
			policyFailureAt = time.Now()
		}

		resultStatus := "pending"
		var errMsg pgtype.Text
		if validationErr != nil {
			resultStatus = "failed"
			errMsg = pgtype.Text{String: validationErr.Error(), Valid: true}
			failureSummaries = append(failureSummaries, fmt.Sprintf("[%s] %s", pp.AccountID, validationErr.Error()))
		}

		// Carry the Facebook mediaType choice onto the result row from
		// the moment it's first written so the async/queue publish
		// path matches the immediate-publish path. The status worker
		// needs this value to tell an intentional Reel
		// (fb_media_type='reel', /reel/ permalink expected) apart
		// from an accidental reclassification of a Feed video
		// (fb_media_type NULL → fast-fail at 10 min).
		var fbMediaType pgtype.Text
		if platformName == "facebook" {
			if mt := fbMediaTypeFromOptions(pp.PlatformOptions); mt != "" {
				fbMediaType = pgtype.Text{String: mt, Valid: true}
			}
		}

		res, err := h.queries.CreateSocialPostResult(ctx, db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: pp.AccountID,
			Caption:         pp.Caption,
			Status:          resultStatus,
			ExternalID:      pgtype.Text{},
			ErrorMessage:    errMsg,
			PublishedAt:     pgtype.Timestamptz{},
			Url:             pgtype.Text{},
			DebugCurl:       pgtype.Text{},
			FbMediaType:     fbMediaType,
		})
		if err != nil {
			return nil, nil, err
		}
		results = append(results, res)

		if validationErr != nil {
			failure := postfailures.BuildParams(post.ID, res.ID, post.WorkspaceID, pp.AccountID,
				postfailures.FirstNonEmpty(platformName, accountMap[pp.AccountID].Platform), "dispatch_prepare",
				validationErr.Error(), validationErr.Error())
			if evaluation.quotaFailure {
				failure = postfailures.BuildParams(post.ID, res.ID, post.WorkspaceID, pp.AccountID,
					postfailures.FirstNonEmpty(platformName, accountMap[pp.AccountID].Platform), "quota",
					validationErr.Error(), validationErr.Error())
			} else if policyDecision.Restricted {
				failure = publishingRestrictionFailure(post.ID, res.ID, post.WorkspaceID, pp.AccountID, platformName, policyDecision.CycleID)
			}
			h.recordPostFailure(ctx, failure)
			if err := h.queries.UpdateSocialPostResultFailureDetails(ctx, updateSocialPostResultFailureDetailsParams(res.ID, failure)); err != nil {
				return nil, nil, err
			}
			continue
		}

		job, err := h.queries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
			PostID:             post.ID,
			SocialPostResultID: res.ID,
			WorkspaceID:        post.WorkspaceID,
			SocialAccountID:    pp.AccountID,
			Platform:           acc.Platform,
			PostInputIndex:     int32(idx),
			Kind:               "dispatch",
			State:              "pending",
			Attempts:           0,
			MaxAttempts:        defaultDeliveryJobMaxAttempts,
		})
		if err != nil {
			return nil, nil, err
		}
		jobs = append(jobs, job)
	}

	newStatus := "publishing"
	if len(jobs) == 0 {
		newStatus = "failed"
	}
	if err := h.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
		ID:          post.ID,
		Status:      newStatus,
		PublishedAt: pgtype.Timestamptz{},
	}); err != nil {
		return nil, nil, err
	}
	post.Status = newStatus
	post.PublishedAt = pgtype.Timestamptz{}
	if !policyFailureAt.IsZero() {
		if err := h.syncPostMediaRetentionForPublishingRestrictionStatusAtStrict(ctx, post, newStatus, policyFailureAt); err != nil {
			return nil, nil, err
		}
	} else {
		h.syncPostMediaRetentionAfterResultTransition(ctx, post, newStatus, results)
	}
	if newStatus == "failed" && len(failureSummaries) > 0 {
		_ = h.queries.UpdateSocialPostErrorMetadata(ctx, db.UpdateSocialPostErrorMetadataParams{
			ID:      post.ID,
			Column2: strings.Join(failureSummaries, "; "),
		})
	}

	return results, jobs, nil
}

func (h *SocialPostHandler) queueImmediatePost(
	ctx context.Context,
	workspaceID string,
	parsed parsedRequest,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
) (socialPostResponse, error) {
	var response socialPostResponse
	var err error
	if hasRestrictedPublishingTarget(blockedTargets) {
		err = h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
			var txErr error
			response, txErr = h.withQueueQueries(txQueries).queueImmediatePostInTransaction(ctx, workspaceID, parsed, accountMap, blockedTargets)
			return txErr
		})
	} else {
		response, err = h.queueImmediatePostInTransaction(ctx, workspaceID, parsed, accountMap, blockedTargets)
	}
	if err == nil {
		h.logQueuedPost(ctx, workspaceID, response.ID, "immediate", parsed.Posts, len(response.Results), response.ActiveJobCount)
	}
	return response, err
}

func (h *SocialPostHandler) queueImmediatePostInTransaction(
	ctx context.Context,
	workspaceID string,
	parsed parsedRequest,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
) (socialPostResponse, error) {
	metaJSON, _ := platform.EncodePostMetadata(parsed.Posts)
	canonicalCaption := pgtype.Text{}
	if len(parsed.Posts) > 0 && parsed.Posts[0].Caption != "" {
		canonicalCaption = pgtype.Text{String: parsed.Posts[0].Caption, Valid: true}
	}
	canonicalMedia := []string{}
	if len(parsed.Posts) > 0 && parsed.Posts[0].MediaURLs != nil {
		canonicalMedia = parsed.Posts[0].MediaURLs
	}

	post, err := h.queries.CreateSocialPost(ctx, db.CreateSocialPostParams{
		WorkspaceID:    workspaceID,
		Caption:        canonicalCaption,
		MediaUrls:      canonicalMedia,
		Status:         "publishing",
		Metadata:       metaJSON,
		ScheduledAt:    pgtype.Timestamptz{},
		IdempotencyKey: idempotencyKeyParam(parsed.IdempotencyKey),
		Source:         resolveSource(ctx),
		ProfileIds:     h.resolveProfileIDs(ctx, workspaceID, uniqueAccountIDs(parsed.Posts)),
	})
	if err != nil {
		return socialPostResponse{}, fmt.Errorf("failed to create post: %w", err)
	}

	results, jobs, err := h.enqueueParsedPostDeliveries(ctx, post, parsed.Posts, accountMap, blockedTargets)
	if err != nil {
		return socialPostResponse{}, err
	}
	return h.socialPostResponseFromData(post, results, jobs, "async"), nil
}

func (h *SocialPostHandler) enqueueExistingPostDeliveries(
	ctx context.Context,
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
) (socialPostResponse, error) {
	var response socialPostResponse
	var err error
	if hasRestrictedPublishingTarget(blockedTargets) {
		err = h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
			var txErr error
			response, txErr = h.withQueueQueries(txQueries).enqueueExistingPostDeliveriesInTransaction(ctx, post, parsed, accountMap, blockedTargets)
			return txErr
		})
	} else {
		response, err = h.enqueueExistingPostDeliveriesInTransaction(ctx, post, parsed, accountMap, blockedTargets)
	}
	if err == nil {
		h.logQueuedPost(ctx, post.WorkspaceID, post.ID, "draft_publish", parsed, len(response.Results), response.ActiveJobCount)
	}
	return response, err
}

func (h *SocialPostHandler) enqueueExistingPostDeliveriesInTransaction(
	ctx context.Context,
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
	blockedTargets map[string]publishingrestrictions.Decision,
) (socialPostResponse, error) {
	results, jobs, err := h.enqueueParsedPostDeliveries(ctx, post, parsed, accountMap, blockedTargets)
	if err != nil {
		return socialPostResponse{}, err
	}
	return h.socialPostResponseFromData(post, results, jobs, "async"), nil
}

func (h *SocialPostHandler) logQueuedPost(ctx context.Context, workspaceID, postID, mode string, posts []platform.PlatformPostInput, resultCount, jobCount int) {
	message := "Queued post deliveries for publishing."
	if mode == "draft_publish" {
		message = "Queued draft deliveries for publishing."
	}
	h.logPublishingEvent(ctx, integrationlogs.Event{
		WorkspaceID: workspaceID,
		Level:       integrationlogs.LevelInfo,
		Status:      integrationlogs.StatusSuccess,
		Action:      integrationlogs.ActionPostPublishQueued,
		Message:     message,
		PostID:      postID,
		Metadata: map[string]any{
			"mode":            mode,
			"target_count":    len(posts),
			"queued_jobs":     jobCount,
			"result_count":    resultCount,
			"target_accounts": uniqueAccountIDs(posts),
		},
	})
}

func (h *SocialPostHandler) EnqueueScheduledPost(ctx context.Context, post db.SocialPost) error {
	outcome, err := h.enqueueClaimedScheduledPost(ctx, post)
	if err != nil {
		return err
	}
	h.applyScheduledEnqueueOutcome(ctx, outcome)
	return outcome.terminalErr
}

type scheduledEnqueueOutcome struct {
	terminalErr              error
	retentionPost            db.SocialPost
	retentionStatus          string
	syncOrdinaryRetention    bool
	blockedQuotaEmail        bool
	blockedQuotaRequestedQty int
}

func (h *SocialPostHandler) applyScheduledEnqueueOutcome(ctx context.Context, outcome scheduledEnqueueOutcome) {
	if outcome.syncOrdinaryRetention {
		h.syncPostMediaRetention(ctx, outcome.retentionPost, outcome.retentionStatus)
	}
	if outcome.blockedQuotaEmail {
		h.maybeSendFreePlanQuotaEmail(ctx, outcome.retentionPost.WorkspaceID, quotaemail.Evaluation{
			Blocked:        true,
			RequestedUnits: outcome.blockedQuotaRequestedQty,
		})
	}
}

// ClaimAndEnqueueScheduledPost keeps the scheduled->publishing claim, delivery
// rows, parent terminal state, and strict restriction retention in one
// transaction. Any failure leaves the post scheduled and eligible for the next
// scheduler pass.
func (h *SocialPostHandler) ClaimAndEnqueueScheduledPost(ctx context.Context, postID string) error {
	preclaim, err := h.queries.GetSocialPostByID(ctx, postID)
	if err != nil {
		return err
	}
	periods := scheduledQuotaLockPeriods(preclaim, quota.PeriodForTime(time.Now().UTC()))
	var outcome scheduledEnqueueOutcome
	err = h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		for _, period := range periods {
			if err := txQueries.LockScheduledQuotaPeriod(ctx, preclaim.WorkspaceID, period); err != nil {
				return err
			}
		}
		txHandler := h.withQueueQueries(txQueries)
		claimed, err := txQueries.ClaimScheduledPost(ctx, postID)
		if err != nil {
			return err
		}
		if !sameScheduledQuotaPeriods(periods, scheduledQuotaLockPeriods(claimed, quota.PeriodForTime(time.Now().UTC()))) {
			return errors.New("scheduled post period changed while acquiring quota locks")
		}
		outcome, err = txHandler.enqueueClaimedScheduledPost(withoutPostMediaRetentionSync(ctx), claimed)
		return err
	})
	if err != nil {
		return err
	}
	h.applyScheduledEnqueueOutcome(ctx, outcome)
	return outcome.terminalErr
}

func (h *SocialPostHandler) enqueueClaimedScheduledPost(ctx context.Context, post db.SocialPost) (scheduledEnqueueOutcome, error) {
	outcome := scheduledEnqueueOutcome{retentionPost: post}
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	parsed, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil || len(parsed) == 0 {
		if updateErr := h.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
			ID:          post.ID,
			Status:      "failed",
			PublishedAt: pgtype.Timestamptz{},
		}); updateErr != nil {
			return outcome, updateErr
		}
		post.Status = "failed"
		post.PublishedAt = pgtype.Timestamptz{}
		h.syncPostMediaRetention(ctx, post, post.Status)
		if err == nil {
			err = errors.New("post metadata contains no platform posts")
		}
		outcome.terminalErr = fmt.Errorf("decode post metadata: %w", err)
		outcome.retentionPost = post
		outcome.retentionStatus = post.Status
		outcome.syncOrdinaryRetention = postMediaRetentionSyncSkipped(ctx)
		return outcome, nil
	}

	accountMap := make(map[string]platform.ValidateAccount, len(parsed))
	dbAccounts := h.loadDBAccountsByIDs(ctx, post.WorkspaceID, uniqueAccountIDs(parsed))
	for _, pp := range parsed {
		acc, ok := dbAccounts[pp.AccountID]
		accountMap[pp.AccountID] = platform.ValidateAccount{
			Platform:     acc.Platform,
			Disconnected: socialAccountDisconnectedForPublish(acc, ok),
		}
	}
	blockedTargets, policyErr := h.evaluatePublishingRestrictions(ctx, post.WorkspaceID, parsed, accountMap)
	if policyErr != nil {
		return outcome, policyErr
	}
	allowedTargets := allowedPublishingTargets(parsed, blockedTargets)
	quotaUnits := countPublishQuotaUnits(allowedTargets, accountMap)
	executionPeriod := quota.PeriodForTime(time.Now().UTC())
	additionalQuotaUnits := scheduledExecutionAdditionalQuotaUnits(post, parsed, accountMap, quotaUnits, executionPeriod)
	var quotaBlockedTargets map[int]string
	quotaPartiallyBlocked := false
	if status, blocked := h.checkFreePlanPostQuotaForPeriod(ctx, post.WorkspaceID, additionalQuotaUnits, executionPeriod); quotaUnits > 0 && blocked {
		reservedIndexes, hasPerTargetSnapshot := scheduledQuotaReservedIndexesFromMetadata(post.Metadata)
		if hasPerTargetSnapshot {
			unreservedIndexes := make([]int, 0, additionalQuotaUnits)
			releasedCurrentReservationUnits := 0
			sameReservationPeriod := post.ScheduledAt.Valid && quota.PeriodForTime(post.ScheduledAt.Time) == executionPeriod
			for index, pp := range parsed {
				decision := blockedTargets[pp.AccountID]
				account := accountMap[pp.AccountID]
				available := !decision.Restricted && account.Platform != "" && !account.Disconnected
				if sameReservationPeriod && reservedIndexes[index] && !available {
					releasedCurrentReservationUnits++
				}
				if !available {
					continue
				}
				if sameReservationPeriod && reservedIndexes[index] {
					continue
				}
				unreservedIndexes = append(unreservedIndexes, index)
			}
			headroom := max(status.Limit-status.Usage-status.Reserved+releasedCurrentReservationUnits, 0)
			if headroom > len(unreservedIndexes) {
				headroom = len(unreservedIndexes)
			}
			blockedCount := len(unreservedIndexes) - headroom
			failureStatus := status
			failureStatus.Reserved = max(status.Reserved-releasedCurrentReservationUnits, 0) + headroom
			quotaMessage := freePlanQuotaExceededMessage(failureStatus, blockedCount)
			quotaBlockedTargets = make(map[int]string, blockedCount)
			for _, index := range unreservedIndexes[headroom:] {
				quotaBlockedTargets[index] = quotaMessage
			}
			quotaPartiallyBlocked = len(quotaBlockedTargets) > 0
		}
		if !quotaPartiallyBlocked {
			if err := h.failScheduledPostForQuota(ctx, post, parsed, accountMap, blockedTargets, status, quotaUnits); err != nil {
				return outcome, err
			}
			post.Status = "failed"
			post.PublishedAt = pgtype.Timestamptz{}
			outcome.retentionPost = post
			outcome.retentionStatus = post.Status
			outcome.syncOrdinaryRetention = !hasRestrictedPublishingTarget(blockedTargets) && postMediaRetentionSyncSkipped(ctx)
			outcome.blockedQuotaEmail = true
			outcome.blockedQuotaRequestedQty = quotaUnits
			return outcome, nil
		}
	}
	if err := h.queries.SetScheduledExecutionReservationPeriod(ctx, post.ID, executionPeriod); err != nil {
		return outcome, err
	}
	_, jobs, err := h.enqueueParsedPostDeliveriesWithQuotaBlocks(ctx, post, parsed, accountMap, blockedTargets, quotaBlockedTargets)
	if err != nil {
		return outcome, err
	}
	if quotaPartiallyBlocked {
		outcome.blockedQuotaEmail = true
		outcome.blockedQuotaRequestedQty = len(quotaBlockedTargets)
	}
	post.Status = "publishing"
	if len(jobs) == 0 {
		post.Status = "failed"
	}
	post.PublishedAt = pgtype.Timestamptz{}
	outcome.retentionPost = post
	outcome.retentionStatus = post.Status
	outcome.syncOrdinaryRetention = !hasRestrictedPublishingTarget(blockedTargets) && postMediaRetentionSyncSkipped(ctx)
	return outcome, nil
}

func scheduledExecutionAdditionalQuotaUnits(
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
	executionUnits int,
	executionPeriod string,
) int {
	if post.ScheduledAt.Valid && quota.PeriodForTime(post.ScheduledAt.Time) != executionPeriod {
		return max(executionUnits, 0)
	}
	reservedUnits := countPublishQuotaUnits(parsed, accountMap)
	if snapshotUnits, ok := scheduledQuotaUnitsFromMetadata(post.Metadata); ok {
		reservedUnits = snapshotUnits
	}
	return max(executionUnits-reservedUnits, 0)
}

func scheduledQuotaLockPeriods(post db.SocialPost, executionPeriod string) []string {
	periods := []string{executionPeriod}
	if post.ScheduledAt.Valid {
		periods = append(periods, quota.PeriodForTime(post.ScheduledAt.Time))
	}
	sort.Strings(periods)
	out := periods[:0]
	for _, period := range periods {
		if period != "" && (len(out) == 0 || out[len(out)-1] != period) {
			out = append(out, period)
		}
	}
	return out
}

func sameScheduledQuotaPeriods(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (h *SocialPostHandler) failScheduledPostForQuota(ctx context.Context, post db.SocialPost, parsed []platform.PlatformPostInput, accountMap map[string]platform.ValidateAccount, blockedTargets map[string]publishingrestrictions.Decision, status quota.QuotaStatus, requestedUnits int) error {
	quotaMessage := freePlanQuotaExceededMessage(status, requestedUnits)
	summaries := make([]string, 0, len(parsed))
	policyFailed := false

	for _, pp := range parsed {
		platformName := accountMap[pp.AccountID].Platform
		message := quotaMessage
		if decision, restricted := blockedTargets[pp.AccountID]; restricted && decision.Restricted {
			message = publishingrestrictions.UserMessage
			policyFailed = true
		}
		res, err := h.queries.CreateSocialPostResult(ctx, db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: pp.AccountID,
			Caption:         pp.Caption,
			Status:          "failed",
			ExternalID:      pgtype.Text{},
			ErrorMessage:    pgtype.Text{String: message, Valid: true},
			PublishedAt:     pgtype.Timestamptz{},
			Url:             pgtype.Text{},
			DebugCurl:       pgtype.Text{},
			FbMediaType:     pgtype.Text{},
		})
		if err != nil {
			return err
		}
		summaries = append(summaries, fmt.Sprintf("[%s] %s", pp.AccountID, message))
		failure := postfailures.BuildParams(post.ID, res.ID, post.WorkspaceID, pp.AccountID, platformName, "quota", message, message)
		if decision, restricted := blockedTargets[pp.AccountID]; restricted && decision.Restricted {
			failure = publishingRestrictionFailure(post.ID, res.ID, post.WorkspaceID, pp.AccountID, platformName, decision.CycleID)
		}
		h.recordPostFailure(ctx, failure)
		if err := h.queries.UpdateSocialPostResultFailureDetails(ctx, updateSocialPostResultFailureDetailsParams(res.ID, failure)); err != nil {
			return err
		}
	}

	if err := h.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
		ID:          post.ID,
		Status:      "failed",
		PublishedAt: pgtype.Timestamptz{},
	}); err != nil {
		return err
	}
	post.Status = "failed"
	post.PublishedAt = pgtype.Timestamptz{}
	if policyFailed {
		if err := h.syncPostMediaRetentionForPublishingRestrictionStatusAtStrict(ctx, post, post.Status, time.Now()); err != nil {
			return err
		}
	} else {
		h.syncPostMediaRetention(ctx, post, post.Status)
	}
	if len(summaries) > 0 {
		_ = h.queries.UpdateSocialPostErrorMetadata(ctx, db.UpdateSocialPostErrorMetadataParams{
			ID:      post.ID,
			Column2: strings.Join(summaries, "; "),
		})
	}
	return nil
}

func (h *SocialPostHandler) socialPostResponseFromData(
	post db.SocialPost,
	results []db.SocialPostResult,
	jobs []db.PostDeliveryJob,
	executionMode string,
) socialPostResponse {
	var caption *string
	if post.Caption.Valid {
		caption = &post.Caption.String
	}
	var publishedAt *time.Time
	if post.PublishedAt.Valid {
		publishedAt = &post.PublishedAt.Time
	}
	resp := socialPostResponse{
		ID:            post.ID,
		Caption:       caption,
		MediaURLs:     post.MediaUrls,
		Status:        deriveSocialPostStatus(post, results, jobs),
		ExecutionMode: executionMode,
		CreatedAt:     post.CreatedAt.Time,
		PublishedAt:   publishedAt,
		Source:        post.Source,
		ProfileIDs:    post.ProfileIds,
		PlatformPosts: buildEditablePlatformPosts(post.Metadata, derefText(post.Caption)),
	}
	submittedByAccount := buildSubmittedMap(post.Metadata, derefText(post.Caption))
	for _, res := range results {
		rr := postResultResponse{
			ID:              res.ID,
			SocialAccountID: res.SocialAccountID,
			Caption:         res.Caption,
			Status:          res.Status,
		}
		if res.ErrorMessage.Valid {
			rr.ErrorMessage = &res.ErrorMessage.String
		}
		if sub := submittedByAccount[res.SocialAccountID]; sub != nil {
			rr.Submitted = sub
		}
		for _, job := range jobs {
			if job.SocialPostResultID == res.ID {
				rr.Platform = job.Platform
				break
			}
		}
		resp.Results = append(resp.Results, rr)
	}
	applyQueueSummary(&resp, jobs)
	return resp
}

func (h *SocialPostHandler) buildPerAccountTracker(ctx context.Context, workspaceID, accountID string) *quota.PerAccountTracker {
	var perAccountLimit pgtype.Int4
	if ws, err := h.queries.GetWorkspace(ctx, workspaceID); err == nil {
		perAccountLimit = ws.PerAccountMonthlyLimit
	}
	return quota.NewPerAccountTracker(ctx, h.queries, perAccountLimit, []string{accountID})
}

func retryBackoff(attempt int32) time.Duration {
	switch attempt {
	case 1:
		return 2 * time.Minute
	case 2:
		return 10 * time.Minute
	case 3:
		return 30 * time.Minute
	case 4:
		return 2 * time.Hour
	default:
		return 6 * time.Hour
	}
}

func workerPublishingEvent(e integrationlogs.Event) integrationlogs.Event {
	e.Source = integrationlogs.SourceWorker
	return e
}

func markDeliveryJobPlatformStartedParams(job db.PostDeliveryJob) db.MarkPostDeliveryJobPlatformStartedParams {
	return db.MarkPostDeliveryJobPlatformStartedParams{
		ID:            job.ID,
		LeaseOwner:    job.LeaseOwner,
		LastAttemptAt: job.LastAttemptAt,
	}
}

func markDeliveryJobSucceededParams(job db.PostDeliveryJob, finishedAt time.Time) db.MarkPostDeliveryJobSucceededParams {
	return db.MarkPostDeliveryJobSucceededParams{
		FinishedAt:    pgtype.Timestamptz{Time: finishedAt, Valid: true},
		ID:            job.ID,
		LeaseOwner:    job.LeaseOwner,
		LastAttemptAt: job.LastAttemptAt,
	}
}

func markDeliveryJobFailedParams(job db.PostDeliveryJob, state string, failureStage, errorCode, platformErrorCode, lastError pgtype.Text, nextRunAt pgtype.Timestamptz) db.MarkPostDeliveryJobFailedParams {
	return db.MarkPostDeliveryJobFailedParams{
		ID:                job.ID,
		State:             state,
		FailureStage:      failureStage,
		ErrorCode:         errorCode,
		PlatformErrorCode: platformErrorCode,
		LastError:         lastError,
		NextRunAt:         nextRunAt,
		LeaseOwner:        job.LeaseOwner,
		LastAttemptAt:     job.LastAttemptAt,
	}
}

// attachPublishTokenResume wires idempotent-publish options onto pp for this
// delivery attempt: a resume token persisted by a prior attempt (if any), and
// a persistence hook the adapter calls the moment it obtains its intermediate
// platform token (IG creation_id / TikTok publish_id). Only the IG and TikTok
// adapters read these keys; every other adapter ignores them, so this is a
// no-op for them.
func (h *SocialPostHandler) attachPublishTokenResume(ctx context.Context, pp *platform.PlatformPostInput, res db.SocialPostResult, job db.PostDeliveryJob) {
	if h == nil || h.queries == nil || pp == nil {
		return
	}
	if pp.PlatformOptions == nil {
		pp.PlatformOptions = map[string]any{}
	}
	if res.PublishToken.Valid && res.PublishToken.String != "" {
		pp.PlatformOptions[platform.OptResumePublishToken] = res.PublishToken.String
	}
	resultID := res.ID
	pp.PlatformOptions[platform.OptOnPublishToken] = func(token string) error {
		if token == "" {
			return nil
		}
		affected, err := h.queries.SetSocialPostResultPublishToken(ctx, db.SetSocialPostResultPublishTokenParams{
			ID:            resultID,
			PublishToken:  pgtype.Text{String: token, Valid: true},
			JobID:         job.ID,
			LeaseOwner:    job.LeaseOwner,
			LastAttemptAt: job.LastAttemptAt,
		})
		if err != nil {
			return fmt.Errorf("persist publish token for result %s: %w", resultID, err)
		}
		if affected != 1 {
			return fmt.Errorf("persist publish token for result %s: active delivery lease was lost", resultID)
		}
		return nil
	}
}

func shouldEvaluateDeliveryPublishingRestriction(res db.SocialPostResult) bool {
	return !res.PublishToken.Valid || strings.TrimSpace(res.PublishToken.String) == ""
}

func (h *SocialPostHandler) ProcessPostDeliveryJob(ctx context.Context, job db.PostDeliveryJob) error {
	// Pre-publish guard against double-publish. A claimed job can sit in the
	// worker's serial processing queue for minutes; meanwhile the stale
	// recovery sweep (RecoverStaleDeliveryJobs) may have already marked this
	// job failed and queued a fresh retry. Publishing now would create a
	// duplicate on the platform because the retry publishes too. Re-read the
	// job's current state and only proceed if this job is still the active
	// owner (running/retrying). On any uncertainty, skip: leaving the job be
	// defers to stale recovery rather than risking a second platform call.
	if current, err := h.queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID:          job.ID,
		WorkspaceID: job.WorkspaceID,
	}); err != nil {
		slog.Warn("delivery job: pre-publish state check failed, skipping to avoid duplicate publish",
			"job_id", job.ID, "error", err)
		return nil
	} else if current.State != "running" && current.State != "retrying" {
		slog.Info("delivery job: no longer active, skipping publish",
			"job_id", job.ID, "state", current.State, "post_id", job.PostID)
		return nil
	}

	post, err := h.queries.GetSocialPostByID(ctx, job.PostID)
	if err != nil {
		return err
	}
	res, err := h.queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
		ID:     job.SocialPostResultID,
		PostID: job.PostID,
	})
	if err != nil {
		return err
	}

	// Second double-publish guard, at the result level. The job-state guard
	// above cannot catch this ordering: the original job passes it and starts
	// publishing, stale recovery then queues a retry, and the original
	// finishes and marks the result published. The retry is legitimately
	// "retrying" so it clears the job-state guard too. Re-reading the result
	// here (fresh from the DB, so it reflects the original's just-committed
	// success) lets us skip the platform call when the result is already
	// published, closing the job as succeeded instead of duplicating.
	if providerTerminalOrAccepted(res, job.Platform) {
		slog.Info("delivery job: provider result already accepted, skipping duplicate publish",
			"job_id", job.ID, "post_id", job.PostID, "result_id", res.ID)
		converged, err := h.convergeProviderAcceptedDelivery(ctx, job)
		if err != nil {
			return err
		}
		if converged {
			return nil
		}
		// The result changed after the optimistic read. Continue only from the
		// fresh durable state observed under the post finalization lock.
		res, err = h.queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: res.ID, PostID: post.ID})
		if err != nil {
			return err
		}
	}

	pp, err := platformPostInputAtIndex(post, int(job.PostInputIndex))
	if err != nil {
		return h.finalizeJobLoadFailure(ctx, job, res, post, err)
	}

	acc, err := h.queries.GetSocialAccountByIDAndWorkspace(ctx, db.GetSocialAccountByIDAndWorkspaceParams{
		ID:          pp.AccountID,
		WorkspaceID: post.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return h.handleJobDispatchFailure(ctx, post, res, job, publishOneOutcome{
				platform: job.Platform,
				err:      errors.New("account not found"),
			})
		}
		return err
	}
	unavailable := socialAccountUnavailableForDelivery(acc, true)
	dbAccounts := map[string]db.SocialAccount{pp.AccountID: acc}
	accountMap := map[string]platform.ValidateAccount{
		pp.AccountID: {
			Platform:     acc.Platform,
			Disconnected: unavailable,
		},
	}
	if unavailable {
		delete(dbAccounts, pp.AccountID)
	}
	if account := accountMap[pp.AccountID]; account.Disconnected {
		return h.handleJobDispatchFailure(ctx, post, res, job, publishOneOutcome{
			platform: account.Platform,
			err:      errors.New("account is disconnected"),
		})
	}

	if shouldEvaluateDeliveryPublishingRestriction(res) {
		policyDecision, err := h.evaluateDeliveryPublishingRestriction(ctx, post.WorkspaceID, job.Platform, accountMap[pp.AccountID])
		if err != nil {
			return err
		}
		if policyDecision.Restricted {
			return h.finalizeRestrictedDeliveryJob(ctx, job, res, post, policyDecision)
		}
	}

	// Idempotent-publish wiring (IG/TikTok). Let the adapter resume from a
	// token a prior attempt persisted, and persist the token this attempt
	// obtains, so a crash between "media uploaded" and "external_id recorded"
	// re-uses the same container/publish_id instead of duplicating the post.
	h.attachPublishTokenResume(ctx, &pp, res, job)

	startedJob, err := h.queries.MarkPostDeliveryJobPlatformStarted(ctx, markDeliveryJobPlatformStartedParams(job))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("delivery job: no longer active before platform dispatch, skipping publish",
				"job_id", job.ID, "post_id", job.PostID)
			return nil
		}
		return err
	}
	job = startedJob
	h.logPublishingEvent(ctx, workerPublishingEvent(integrationlogs.Event{
		WorkspaceID:     job.WorkspaceID,
		Level:           integrationlogs.LevelInfo,
		Status:          integrationlogs.StatusSuccess,
		Action:          integrationlogs.ActionPostPublishPlatformStarted,
		Message:         "Started platform delivery job.",
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Platform:        job.Platform,
		Metadata: map[string]any{
			"job_id":           job.ID,
			"attempt":          job.Attempts,
			"post_input_index": job.PostInputIndex,
		},
	}))

	oc := h.publishOneContext(
		ctx,
		post.WorkspaceID,
		post.ID,
		xUsageKeyForResult(res.ID),
		pp,
		dbAccounts,
		accountMap,
		h.buildPerAccountTracker(ctx, post.WorkspaceID, pp.AccountID),
		quota.NewPerPlatformDailyTracker(ctx, h.queries, dailyTargetsFor([]platform.PlatformPostInput{pp}, accountMap)),
		h.disallowedPlatformsForDispatch(ctx, post.WorkspaceID, []platform.PlatformPostInput{pp}, accountMap),
	)
	h.logPlatformDispatchEvents(ctx, post, res, job, oc.dispatchEvents)
	if oc.err != nil {
		return h.handleJobDispatchFailure(ctx, post, res, job, oc)
	}

	completedAt := time.Now()
	status := "published"
	var externalID, postURL pgtype.Text
	var publishedAt pgtype.Timestamptz
	var debugCurl pgtype.Text
	if oc.result != nil {
		if oc.result.Status != "" {
			status = oc.result.Status
		}
		externalID = pgtype.Text{String: oc.result.ExternalID, Valid: oc.result.ExternalID != ""}
		postURL = pgtype.Text{String: oc.result.URL, Valid: oc.result.URL != ""}
	}
	if status == "published" {
		publishedAt = pgtype.Timestamptz{Time: completedAt, Valid: true}
	}
	updateParams := db.UpdateSocialPostResultAfterRetryParams{
		ID:              res.ID,
		Status:          status,
		ExternalID:      externalID,
		ErrorMessage:    pgtype.Text{},
		PublishedAt:     publishedAt,
		Url:             postURL,
		DebugCurl:       debugCurl,
		XCreditsCounted: oc.xCreditsCounted,
		XCreditOperation: pgtype.Text{
			String: oc.xCreditOperation,
			Valid:  oc.xCreditOperation != "",
		},
		XCreditCatalogVersion: pgtype.Text{
			String: oc.xCreditCatalog,
			Valid:  oc.xCreditCatalog != "",
		},
		XCreditBillingMode: pgtype.Text{
			String: oc.xCreditBillingMode,
			Valid:  oc.xCreditBillingMode != "",
		},
	}
	var updated db.SocialPostResult
	newlyPublished := false
	var pendingEvent *pendingParentPostStatusEvent
	err = h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, post.ID); err != nil {
			return err
		}
		fresh, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: res.ID, PostID: post.ID})
		if err != nil {
			return err
		}
		if fresh.Status == "published" {
			updated = fresh
		} else if status == "published" {
			updated, err = txQueries.UpdateSocialPostResultAfterRetryAndIncrementUsage(ctx, db.UpdateSocialPostResultAfterRetryAndIncrementUsageParams{
				ID: updateParams.ID, Status: updateParams.Status, ExternalID: updateParams.ExternalID,
				ErrorMessage: updateParams.ErrorMessage, PublishedAt: updateParams.PublishedAt,
				Url: updateParams.Url, DebugCurl: updateParams.DebugCurl,
				XCreditsCounted: updateParams.XCreditsCounted, XCreditOperation: updateParams.XCreditOperation,
				XCreditCatalogVersion: updateParams.XCreditCatalogVersion, XCreditBillingMode: updateParams.XCreditBillingMode,
				WorkspaceID: post.WorkspaceID, Period: quota.PeriodForTime(completedAt), PostCount: 1,
			})
			if err != nil {
				return err
			}
			newlyPublished = true
		} else {
			updated, err = txQueries.UpdateSocialPostResultAfterRetry(ctx, updateParams)
			if err != nil {
				return err
			}
		}
		if _, err := txQueries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job, completedAt)); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
		return err
	})
	if err != nil {
		return err
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, post.ID)
	if newlyPublished {
		h.maybeSendFreePlanQuotaEmail(ctx, post.WorkspaceID, quotaemail.Evaluation{})
		h.maybeEvaluatePaidQuota(ctx, post.WorkspaceID, quota.PeriodForTime(completedAt))
	}
	h.logPublishingEvent(ctx, workerPublishingEvent(integrationlogs.Event{
		WorkspaceID:     post.WorkspaceID,
		Level:           integrationlogs.LevelInfo,
		Status:          integrationlogs.StatusSuccess,
		Action:          integrationlogs.ActionPostPublishPlatformSucceeded,
		Message:         "Platform delivery job succeeded.",
		PostID:          post.ID,
		SocialAccountID: res.SocialAccountID,
		Platform:        job.Platform,
		PlatformPostID:  externalID.String,
		Metadata: map[string]any{
			"job_id":    job.ID,
			"result_id": res.ID,
			"status":    updated.Status,
		},
	}))
	return nil
}

func (h *SocialPostHandler) logPlatformDispatchEvents(ctx context.Context, post db.SocialPost, res db.SocialPostResult, job db.PostDeliveryJob, events []platform.DispatchEvent) {
	for _, event := range events {
		switch event.Name {
		case integrationlogs.ActionPinterestDestinationPreflightStarted,
			integrationlogs.ActionPinterestDestinationPreflightSucceeded,
			integrationlogs.ActionPinterestDestinationPreflightFailed,
			integrationlogs.ActionPinterestCreatePinFailed,
			integrationlogs.ActionPinterestMediaPreflightFailed,
			integrationlogs.ActionPinterestMediaStaged:
		default:
			continue
		}
		level := integrationlogs.LevelInfo
		status := integrationlogs.StatusSuccess
		if event.Status == "failed" {
			level = integrationlogs.LevelWarn
			status = integrationlogs.StatusError
		}
		durationMS := int(event.Duration / time.Millisecond)
		var remoteStatus *int
		if event.HTTPStatus > 0 {
			value := event.HTTPStatus
			remoteStatus = &value
		}
		h.logPublishingEvent(ctx, workerPublishingEvent(integrationlogs.Event{
			WorkspaceID:      post.WorkspaceID,
			Level:            level,
			Status:           status,
			Action:           event.Name,
			Message:          event.Name,
			PostID:           post.ID,
			SocialAccountID:  res.SocialAccountID,
			Platform:         job.Platform,
			RemoteStatusCode: remoteStatus,
			DurationMS:       &durationMS,
			ErrorCode:        event.ErrorCode,
			Metadata: map[string]any{
				"job_id":                job.ID,
				"result_id":             res.ID,
				"board_fingerprint":     event.BoardFingerprint,
				"pinterest_environment": event.Environment,
				"provider_code":         event.ProviderCode,
				"normalized_reason":     event.Reason,
				"failure_stage":         event.FailureStage,
				"retriable":             event.Retriable,
				"media_type":            event.MediaType,
				"media_size_bytes":      event.MediaSizeBytes,
				"customer_input":        event.CustomerInput,
			},
		}))
	}
}

func (h *SocialPostHandler) evaluateDeliveryPublishingRestriction(
	ctx context.Context,
	workspaceID string,
	jobPlatform string,
	account platform.ValidateAccount,
) (publishingrestrictions.Decision, error) {
	if h == nil || h.publishingRestrictions == nil {
		return publishingrestrictions.Decision{}, nil
	}
	if account.Disconnected {
		return publishingrestrictions.Decision{}, nil
	}
	platformName := strings.ToLower(strings.TrimSpace(account.Platform))
	if platformName == "" {
		return publishingrestrictions.Decision{}, nil
	}
	return h.publishingRestrictions.Evaluate(ctx, workspaceID, platformName)
}

func (h *SocialPostHandler) finalizeRestrictedDeliveryJob(
	ctx context.Context,
	job db.PostDeliveryJob,
	res db.SocialPostResult,
	post db.SocialPost,
	decision publishingrestrictions.Decision,
) error {
	mediaIDs, mediaMetadataOK := decodeMediaIDsForRetention(post)
	if !mediaMetadataOK {
		return fmt.Errorf("%w: post %s", errInvalidPostMediaMetadata, post.ID)
	}
	finalizedAt := time.Now()
	params := db.FinalizeRestrictedPostDeliveryJobParams{
		FailureStage:     postfailures.ToText(publishingrestrictions.FailureStage),
		ErrorCode:        postfailures.ToText(publishingrestrictions.NormalizedCode),
		ErrorMessage:     publishingrestrictions.UserMessage,
		ID:               job.ID,
		LeaseOwner:       job.LeaseOwner,
		LastAttemptAt:    job.LastAttemptAt,
		NextAction:       postfailures.ToText(publishingrestrictions.NextAction),
		ErrorSource:      postfailures.ToText(postfailures.ErrorSourceUnipost),
		ErrorTemporality: postfailures.ToText(postfailures.ErrorTemporalityTemporary),
		MediaIds:         mediaIDs,
		CleanupAfterAt:   pgtype.Timestamptz{Time: finalizedAt.UTC().Add(60 * 24 * time.Hour), Valid: true},
	}
	var finalizedJob db.FinalizeRestrictedPostDeliveryJobRow
	var pendingEvent *pendingParentPostStatusEvent
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		// This must be a separate statement from finalization. A concurrent
		// waiter then evaluates all sibling jobs/results using a fresh
		// READ COMMITTED snapshot after the previous finalizer commits.
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, post.ID); err != nil {
			return err
		}
		var finalizeErr error
		finalizedJob, finalizeErr = txQueries.FinalizeRestrictedPostDeliveryJob(ctx, params)
		if finalizeErr != nil {
			return finalizeErr
		}
		if finalizedJob.ParentTransitioned && (finalizedJob.ParentCurrentStatus == "failed" || finalizedJob.ParentCurrentStatus == "partial") {
			pendingEvent, finalizeErr = h.parentStatusEventFromQueries(ctx, txQueries, post.ID, finalizedJob.ParentCurrentStatus)
		}
		return finalizeErr
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Observation only — the state machine is deliberately
			// unchanged here (see logRestrictedFinalizeNoRow).
			h.logRestrictedFinalizeNoRow(ctx, job, post, mediaIDs)
			return nil
		}
		return err
	}
	switch finalizedJob.State {
	case "dead":
		failure := publishingRestrictionFailure(post.ID, res.ID, post.WorkspaceID, res.SocialAccountID, decision.Platform, decision.CycleID)
		h.recordPostFailureHistory(ctx, failure)
		h.logPublishingEvent(ctx, workerPublishingEvent(integrationlogs.Event{
			WorkspaceID:     post.WorkspaceID,
			Level:           integrationlogs.LevelWarn,
			Status:          integrationlogs.StatusError,
			Action:          integrationlogs.ActionPostPublishPlatformFailed,
			Message:         "Platform delivery blocked by publishing restriction.",
			PostID:          post.ID,
			SocialAccountID: res.SocialAccountID,
			Platform:        decision.Platform,
			ErrorCode:       publishingrestrictions.NormalizedCode,
			Metadata: map[string]any{
				"job_id":               job.ID,
				"result_id":            res.ID,
				"restriction_cycle_id": decision.CycleID,
			},
		}))
	case "succeeded":
		// A different delivery job already committed this shared result as
		// published. Converge this duplicate job without recording a failure.
	default:
		return fmt.Errorf("unexpected restricted delivery finalization state %q", finalizedJob.State)
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	return nil
}

// logRestrictedFinalizeNoRow records why FinalizeRestrictedPostDeliveryJob
// matched no row. At least four conditions collapse into "no row": the job
// left running/retrying, the lease owner changed, last_attempt_at changed, or
// the referenced media are no longer retainable.
//
// This runs after the transaction returned, so a concurrent worker may already
// have finalized the job and moved the state we re-read. The classification is
// therefore a best-effort trend signal only — it is not exact attribution, and
// it must not drive a state transition. Deciding a user-visible terminal state
// for media_unavailable is deliberately left to a follow-up change; the
// finalize and stale-lease recovery state machines are untouched here.
func (h *SocialPostHandler) classifyRestrictedFinalizeNoRow(
	ctx context.Context,
	job db.PostDeliveryJob,
	post db.SocialPost,
	mediaIDs []string,
) (classification, detail, observedState string) {
	classification = "indeterminate"

	current, err := h.queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID:          job.ID,
		WorkspaceID: job.WorkspaceID,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "job_missing", "", ""
	case err != nil:
		return classification, err.Error(), ""
	}

	observedState = current.State
	switch {
	case current.State != "running" && current.State != "retrying":
		return "state_changed", "", observedState
	case current.LeaseOwner != job.LeaseOwner:
		return "lease_mismatch", "", observedState
	case current.LastAttemptAt != job.LastAttemptAt:
		return "attempt_mismatch", "", observedState
	}

	for _, mediaID := range mediaIDs {
		reason, _, classifyErr := h.classifyUnretainableMedia(ctx, post.WorkspaceID, mediaID)
		if classifyErr != nil {
			continue
		}
		if reason != mediaUnretainableRaceResolved {
			return "media_unavailable", reason, observedState
		}
	}
	return classification, detail, observedState
}

func (h *SocialPostHandler) logRestrictedFinalizeNoRow(
	ctx context.Context,
	job db.PostDeliveryJob,
	post db.SocialPost,
	mediaIDs []string,
) {
	classification, detail, observedState := h.classifyRestrictedFinalizeNoRow(ctx, job, post, mediaIDs)

	// Stable message + classification field: this is the aggregation signal
	// that replaces a metrics counter (no metrics SDK in this service).
	slog.Warn("restricted delivery finalize matched no row",
		"job_id", job.ID,
		"post_id", post.ID,
		"workspace_id", post.WorkspaceID,
		"classification", classification,
		"detail", detail,
		"observed_state", observedState,
		"expected_state", job.State,
		"lease_owner", job.LeaseOwner.String,
		"last_attempt_at", job.LastAttemptAt.Time,
		"best_effort", true)
}

func (h *SocialPostHandler) parentStatusEventFromQueries(ctx context.Context, queries *db.Queries, postID, status string) (*pendingParentPostStatusEvent, error) {
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
	parentVersion, err := queries.GetSocialPostProjectionVersion(ctx, postID)
	if err != nil {
		return nil, err
	}
	pending := &pendingParentPostStatusEvent{
		workspaceID:   post.WorkspaceID,
		event:         eventForStatus(status),
		response:      h.socialPostResponseFromData(post, results, jobs, "async"),
		parentStatus:  status,
		parentVersion: parentVersion,
	}
	if err := db.RecordPostStatusTransition(ctx, queries, post.ID, post.WorkspaceID, status, parentVersion, pending.event, pending.response); err != nil {
		return nil, err
	}
	return pending, nil
}

func (h *SocialPostHandler) refreshTerminalParentPostStatusContext(
	ctx context.Context,
	post db.SocialPost,
	_ []db.SocialPostResult,
) error {
	var pendingEvent *pendingParentPostStatusEvent
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, post.ID); err != nil {
			return err
		}
		var refreshErr error
		pendingEvent, refreshErr = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
		return refreshErr
	})
	if err != nil {
		return err
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, post.ID)
	return nil
}

func (h *SocialPostHandler) refreshTerminalParentPostStatusLocked(
	ctx context.Context,
	txQueries *db.Queries,
	postID string,
) (*pendingParentPostStatusEvent, error) {
	// The lock is intentionally a separate statement. Re-read the parent
	// and every sibling only after it is acquired so a waiter observes the
	// prior finalizer's committed status, published_at, and retention state.
	freshPost, err := txQueries.GetSocialPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	results, err := txQueries.ListSocialPostResultsByPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	return h.withQueueQueries(txQueries).
		refreshParentPostStatusContextWithoutPublish(withoutPostMediaRetentionSync(ctx), freshPost, results)
}

// syncTerminalPostRetentionAfterCommit deliberately runs after the terminal
// transaction. It reacquires the same post lock before re-reading the parent
// projection and updating media usage, so a manual retry cannot activate media
// between the retention snapshot and write. Retention remains best effort: a
// ledger error never rolls back or disguises the durable provider result.
func (h *SocialPostHandler) syncTerminalPostRetentionAfterCommit(ctx context.Context, postID string) {
	if postMediaRetentionSyncSkipped(ctx) {
		return
	}
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, postID); err != nil {
			return err
		}
		post, err := txQueries.GetSocialPostByID(ctx, postID)
		if err != nil {
			return err
		}
		results, err := txQueries.ListSocialPostResultsByPost(ctx, postID)
		if err != nil {
			return err
		}
		h.withQueueQueries(txQueries).syncPostMediaRetentionAfterResultTransition(ctx, post, post.Status, results)
		return nil
	})
	if err != nil {
		slog.Warn("terminal media retention: locked sync failed", "post_id", postID, "error", err)
	}
}

func providerAcceptedProcessing(result db.SocialPostResult, platformName string) bool {
	return strings.EqualFold(strings.TrimSpace(platformName), "facebook") &&
		result.Status == "processing" && result.ExternalID.Valid && strings.TrimSpace(result.ExternalID.String) != ""
}

func providerTerminalOrAccepted(result db.SocialPostResult, platformName string) bool {
	return result.Status == "published" || providerAcceptedProcessing(result, platformName)
}

func (h *SocialPostHandler) convergeProviderAcceptedDelivery(ctx context.Context, job db.PostDeliveryJob) (bool, error) {
	var pendingEvent *pendingParentPostStatusEvent
	converged := false
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, job.PostID); err != nil {
			return err
		}
		fresh, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
			ID: job.SocialPostResultID, PostID: job.PostID,
		})
		if err != nil {
			return err
		}
		if !providerTerminalOrAccepted(fresh, job.Platform) {
			return nil
		}
		if _, err := txQueries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job, time.Now())); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, job.PostID)
		if err != nil {
			return err
		}
		converged = true
		return nil
	})
	if err != nil || !converged {
		return converged, err
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, job.PostID)
	return true, nil
}

func (h *SocialPostHandler) recordPostFailureHistory(ctx context.Context, failure db.CreatePostFailureParams) {
	failure.Message = sanitizeDeliveryErrorText(failure.Message)
	failure.RawError = sanitizeDeliveryErrorTextValue(failure.RawError)
	failure.PlatformErrorCode = sanitizeDeliveryErrorTextValue(failure.PlatformErrorCode)
	if _, err := h.queries.CreatePostFailure(ctx, failure); err != nil {
		slog.Warn("failed to persist structured post failure",
			"post_id", failure.PostID,
			"platform", failure.Platform,
			"stage", failure.FailureStage,
			"error", err)
	}
}

func (h *SocialPostHandler) finalizeJobLoadFailure(ctx context.Context, job db.PostDeliveryJob, res db.SocialPostResult, post db.SocialPost, dispatchErr error) error {
	msg := sanitizeDeliveryErrorText(dispatchErr.Error())
	failure := postfailures.BuildParams(
		post.ID, res.ID, post.WorkspaceID, res.SocialAccountID, job.Platform, "dispatch_prepare", msg, msg,
	)
	var pendingEvent *pendingParentPostStatusEvent
	failureApplied := false
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, post.ID); err != nil {
			return err
		}
		freshResult, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: res.ID, PostID: post.ID})
		if err != nil {
			return err
		}
		if providerTerminalOrAccepted(freshResult, job.Platform) {
			if _, err := txQueries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job, time.Now())); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
			return err
		}
		// Claim the terminal transition before touching the shared result. If
		// this lease is stale, MarkPostDeliveryJobFailed returns no rows and the
		// former owner must not overwrite the current owner's result or parent.
		if _, err := txQueries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
			job,
			"dead",
			pgtype.Text{String: "dispatch_prepare", Valid: true},
			pgtype.Text{String: "validation_error", Valid: true},
			pgtype.Text{},
			pgtype.Text{String: msg, Valid: true},
			pgtype.Timestamptz{},
		)); err != nil {
			return err
		}
		if _, err := txQueries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
			ID:           res.ID,
			Status:       "failed",
			ExternalID:   pgtype.Text{},
			ErrorMessage: pgtype.Text{String: msg, Valid: true},
			PublishedAt:  pgtype.Timestamptz{},
			Url:          pgtype.Text{},
			DebugCurl:    pgtype.Text{},
		}); err != nil {
			return err
		}
		if err := txQueries.UpdateSocialPostResultFailureDetails(ctx, updateSocialPostResultFailureDetailsParams(res.ID, failure)); err != nil {
			return err
		}
		if _, err := txQueries.CreatePostFailure(ctx, failure); err != nil {
			return err
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
		if err == nil {
			failureApplied = true
		}
		return err
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, post.ID)
	if !failureApplied {
		return nil
	}
	h.logPublishingEvent(ctx, workerPublishingEvent(integrationlogs.Event{
		WorkspaceID:     post.WorkspaceID,
		Level:           integrationlogs.LevelError,
		Status:          integrationlogs.StatusError,
		Action:          integrationlogs.ActionPostPublishPlatformFailed,
		Message:         "Platform delivery job failed before dispatch.",
		PostID:          post.ID,
		SocialAccountID: res.SocialAccountID,
		Platform:        job.Platform,
		ErrorCode:       "validation_error",
		Metadata: map[string]any{
			"job_id":    job.ID,
			"result_id": res.ID,
			"stage":     "dispatch_prepare",
		},
		ResponsePayload: map[string]any{
			"error": msg,
		},
	}))
	h.syncLoopsPostFailed(ctx, post, res, job, failure, false)
	return nil
}

func (h *SocialPostHandler) handleJobDispatchFailure(ctx context.Context, post db.SocialPost, res db.SocialPostResult, job db.PostDeliveryJob, oc publishOneOutcome) error {
	errMsg := sanitizeDeliveryErrorText(oc.err.Error())
	sanitizedDebugCurl := sanitizeDeliveryErrorText(oc.debugCurl)
	failureStage := inferDispatchFailureStage(oc.err)

	// Classify before touching the result row. If this attempt is
	// retriable and another attempt is coming, keep the row in
	// "processing" — writing "failed" here would flash in the UI
	// even though the queue is about to re-dispatch. Only terminal
	// outcomes (non-retriable, or a retry job that just hit its
	// max-attempts ceiling) should flip the row to failed.
	failure := postfailures.BuildParamsFromError(
		post.ID,
		res.ID,
		post.WorkspaceID,
		res.SocialAccountID,
		postfailures.FirstNonEmpty(oc.platform, job.Platform),
		failureStage,
		oc.err,
		errMsg,
	)
	failure.PlatformErrorCode = sanitizeDeliveryErrorTextValue(failure.PlatformErrorCode)
	anotherAttempt := failure.IsRetriable && (job.Kind == "dispatch" || job.Attempts < job.MaxAttempts)
	resultStatus := "failed"
	if anotherAttempt {
		resultStatus = "processing"
	}

	resultUpdate := db.UpdateSocialPostResultAfterRetryParams{
		ID:           res.ID,
		Status:       resultStatus,
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		// Persist diagnostics only after this transaction commits. Keeping the
		// legacy value empty makes telemetry incapable of rolling back delivery.
		DebugCurl:       pgtype.Text{},
		XCreditsCounted: oc.xCreditsCounted,
		XCreditOperation: pgtype.Text{
			String: oc.xCreditOperation,
			Valid:  oc.xCreditOperation != "",
		},
		XCreditCatalogVersion: pgtype.Text{
			String: oc.xCreditCatalog,
			Valid:  oc.xCreditCatalog != "",
		},
		XCreditBillingMode: pgtype.Text{
			String: oc.xCreditBillingMode,
			Valid:  oc.xCreditBillingMode != "",
		},
	}
	var pendingEvent *pendingParentPostStatusEvent
	failureApplied := false
	transitionErr := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, post.ID); err != nil {
			return err
		}
		freshResult, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: res.ID, PostID: post.ID})
		if err != nil {
			return err
		}
		if providerTerminalOrAccepted(freshResult, job.Platform) {
			if _, err := txQueries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job, time.Now())); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
			return err
		}
		// Win the lease-checked job transition before mutating the result that
		// may already belong to a newer attempt. The transaction makes a lost
		// lease a zero-side-effect no-op, including retry creation.
		if failure.IsRetriable {
			if job.Kind == "dispatch" {
				if _, err := txQueries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
					job,
					"failed",
					pgtype.Text{String: failure.FailureStage, Valid: true},
					pgtype.Text{String: failure.ErrorCode, Valid: true},
					failure.PlatformErrorCode,
					pgtype.Text{String: errMsg, Valid: true},
					pgtype.Timestamptz{},
				)); err != nil {
					return err
				}
			} else {
				nextRunAt := pgtype.Timestamptz{Time: time.Now().Add(retryBackoff(job.Attempts)), Valid: true}
				state := "pending"
				if job.Attempts >= job.MaxAttempts {
					state = "dead"
					nextRunAt = pgtype.Timestamptz{}
				}
				if _, err := txQueries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
					job,
					state,
					pgtype.Text{String: failure.FailureStage, Valid: true},
					pgtype.Text{String: failure.ErrorCode, Valid: true},
					failure.PlatformErrorCode,
					pgtype.Text{String: errMsg, Valid: true},
					nextRunAt,
				)); err != nil {
					return err
				}
			}
		} else {
			if _, err := txQueries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
				job,
				"dead",
				pgtype.Text{String: failure.FailureStage, Valid: true},
				pgtype.Text{String: failure.ErrorCode, Valid: true},
				failure.PlatformErrorCode,
				pgtype.Text{String: errMsg, Valid: true},
				pgtype.Timestamptz{},
			)); err != nil {
				return err
			}
		}

		if _, err := txQueries.UpdateSocialPostResultAfterRetry(ctx, resultUpdate); err != nil {
			return err
		}
		if err := txQueries.UpdateSocialPostResultFailureDetails(ctx, updateSocialPostResultFailureDetailsParams(res.ID, failure)); err != nil {
			return err
		}
		if _, err := txQueries.CreatePostFailure(ctx, failure); err != nil {
			return err
		}
		if failure.IsRetriable && job.Kind == "dispatch" {
			if _, err := txQueries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
				PostID:             post.ID,
				SocialPostResultID: res.ID,
				WorkspaceID:        post.WorkspaceID,
				SocialAccountID:    res.SocialAccountID,
				Platform:           job.Platform,
				PostInputIndex:     job.PostInputIndex,
				Kind:               "retry",
				State:              "pending",
				Attempts:           0,
				MaxAttempts:        int32(defaultDeliveryJobMaxAttempts),
				FailureStage:       pgtype.Text{String: failure.FailureStage, Valid: true},
				ErrorCode:          pgtype.Text{String: failure.ErrorCode, Valid: true},
				PlatformErrorCode:  failure.PlatformErrorCode,
				LastError:          pgtype.Text{String: errMsg, Valid: true},
				NextRunAt:          pgtype.Timestamptz{Time: time.Now().Add(retryBackoff(1)), Valid: true},
			}); err != nil {
				return err
			}
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
		if err != nil {
			return err
		}
		failureApplied = true
		return nil
	})
	if transitionErr != nil {
		if errors.Is(transitionErr, pgx.ErrNoRows) {
			return nil
		}
		return transitionErr
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, post.ID)
	if !failureApplied {
		return nil
	}
	h.recordPostFailureDebug(res.ID, post.WorkspaceID, sanitizedDebugCurl)
	logLevel := integrationlogs.LevelWarn
	if !failure.IsRetriable || (job.Kind == "retry" && job.Attempts >= job.MaxAttempts) {
		logLevel = integrationlogs.LevelError
	}
	h.logPublishingEvent(ctx, workerPublishingEvent(integrationlogs.Event{
		WorkspaceID:     post.WorkspaceID,
		Level:           logLevel,
		Status:          integrationlogs.StatusError,
		Action:          integrationlogs.ActionPostPublishPlatformFailed,
		Message:         "Platform delivery job failed.",
		PostID:          post.ID,
		SocialAccountID: res.SocialAccountID,
		Platform:        postfailures.FirstNonEmpty(oc.platform, job.Platform),
		ErrorCode:       failure.ErrorCode,
		Metadata: map[string]any{
			"job_id":          job.ID,
			"result_id":       res.ID,
			"failure_stage":   failure.FailureStage,
			"retriable":       failure.IsRetriable,
			"another_attempt": anotherAttempt,
			"job_kind":        job.Kind,
			"attempts":        job.Attempts,
			"max_attempts":    job.MaxAttempts,
		},
		ResponsePayload: map[string]any{
			"error": errMsg,
		},
	}))
	h.syncLoopsPostFailed(ctx, post, res, job, failure, anotherAttempt)
	return nil
}

func inferDispatchFailureStage(err error) string {
	if err == nil {
		return "dispatch"
	}
	var carrier interface{ FailureStage() string }
	if errors.As(err, &carrier) {
		if stage := strings.TrimSpace(carrier.FailureStage()); stage != "" {
			return stage
		}
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "fetch_source_read"):
		return "fetch_source_read"
	case strings.Contains(msg, "fetch_source"):
		return "fetch_source"
	case strings.Contains(msg, "upload_media_status"):
		return "upload_media_status"
	case strings.Contains(msg, "upload_media_finalize"):
		return "upload_media_finalize"
	case strings.Contains(msg, "upload_media_append"):
		return "upload_media_append"
	case strings.Contains(msg, "upload_media_init"):
		return "upload_media_init"
	case strings.Contains(msg, "upload_media"):
		return "upload_media"
	case strings.Contains(msg, "create_tweet_reply"):
		return "create_tweet_reply"
	case strings.Contains(msg, "create_tweet"):
		return "create_tweet"
	default:
		return "dispatch"
	}
}

func (h *SocialPostHandler) RecoverStaleDeliveryJobs(ctx context.Context, maxAge time.Duration) error {
	staleBefore := time.Now().Add(-maxAge)
	jobs, err := h.queries.ListStaleActivePostDeliveryJobs(ctx, pgtype.Timestamptz{
		Time:  staleBefore,
		Valid: true,
	})
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := h.recoverStaleDeliveryJob(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

func (h *SocialPostHandler) recoverStaleDeliveryJob(ctx context.Context, job db.PostDeliveryJob) error {
	post, err := h.queries.GetSocialPostByID(ctx, job.PostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return h.cancelStaleDeliveryJobForDeletedPost(ctx, job)
		}
		return err
	}
	result, err := h.queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
		ID:     job.SocialPostResultID,
		PostID: job.PostID,
	})
	if err != nil {
		return err
	}

	// If a prior attempt already published this result, do not re-queue a
	// retry: doing so would duplicate the post on the platform. Close the
	// stale job out as succeeded to reflect reality instead.
	if providerTerminalOrAccepted(result, job.Platform) {
		slog.Info("stale recovery: provider result already accepted, closing job without retry",
			"job_id", job.ID, "post_id", job.PostID, "result_id", result.ID)
		_, err := h.convergeProviderAcceptedDelivery(ctx, job)
		return err
	}

	errMsg := fmt.Sprintf("delivery attempt stalled after %s and was re-queued automatically", staleDeliveryAttemptTimeout.Round(time.Second))
	if job.Kind == "retry" && job.Attempts >= job.MaxAttempts {
		errMsg = fmt.Sprintf("delivery attempt stalled after %s and exhausted retry attempts", staleDeliveryAttemptTimeout.Round(time.Second))
	}

	// Match handleJobDispatchFailure's status logic: keep the row in
	// "processing" while another attempt is queued, only flip to
	// "failed" when we've truly run out of retries. Stale dispatch
	// jobs always schedule a fresh retry; stale retry jobs only
	// schedule another attempt if they haven't hit the attempt cap.
	anotherAttempt := job.Kind == "dispatch" || job.Attempts < job.MaxAttempts
	resultStatus := "failed"
	if anotherAttempt {
		resultStatus = "processing"
	}

	resultUpdate := db.UpdateSocialPostResultAfterRetryParams{
		ID:           result.ID,
		Status:       resultStatus,
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		DebugCurl:    pgtype.Text{},
	}

	failureStage := pgtype.Text{String: "worker_timeout", Valid: true}
	errorCode := pgtype.Text{String: "worker_stalled", Valid: true}
	lastError := pgtype.Text{String: errMsg, Valid: true}
	failure := postfailures.BuildParams(
		post.ID, result.ID, post.WorkspaceID, result.SocialAccountID,
		postfailures.FirstNonEmpty(job.Platform), "worker_timeout", errMsg, errMsg,
	)

	var pendingEvent *pendingParentPostStatusEvent
	failureApplied := false
	transitionErr := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, post.ID); err != nil {
			return err
		}
		freshResult, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: result.ID, PostID: post.ID})
		if err != nil {
			return err
		}
		if providerTerminalOrAccepted(freshResult, job.Platform) {
			if _, err := txQueries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job, time.Now())); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
			return err
		}
		// A stale recovery snapshot is allowed to lose its lease. Claim the
		// lease-checked transition first so that loser cannot overwrite the
		// newer attempt's result or retry schedule.
		switch job.Kind {
		case "dispatch":
			if _, err := txQueries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
				job,
				"failed",
				failureStage,
				errorCode,
				pgtype.Text{},
				lastError,
				pgtype.Timestamptz{},
			)); err != nil {
				return err
			}
		default:
			nextRunAt := pgtype.Timestamptz{Time: time.Now().Add(retryBackoff(job.Attempts)), Valid: true}
			state := "pending"
			if job.Attempts >= job.MaxAttempts {
				state = "dead"
				nextRunAt = pgtype.Timestamptz{}
			}
			if _, err := txQueries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
				job,
				state,
				failureStage,
				errorCode,
				pgtype.Text{},
				lastError,
				nextRunAt,
			)); err != nil {
				return err
			}
		}
		if _, err := txQueries.UpdateSocialPostResultAfterRetry(ctx, resultUpdate); err != nil {
			return err
		}
		if err := txQueries.UpdateSocialPostResultFailureDetails(ctx, updateSocialPostResultFailureDetailsParams(result.ID, failure)); err != nil {
			return err
		}
		if _, err := txQueries.CreatePostFailure(ctx, failure); err != nil {
			return err
		}
		if job.Kind == "dispatch" {
			if _, err := txQueries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
				PostID:             post.ID,
				SocialPostResultID: result.ID,
				WorkspaceID:        post.WorkspaceID,
				SocialAccountID:    result.SocialAccountID,
				Platform:           job.Platform,
				PostInputIndex:     job.PostInputIndex,
				Kind:               "retry",
				State:              "pending",
				Attempts:           0,
				MaxAttempts:        int32(defaultDeliveryJobMaxAttempts),
				FailureStage:       failureStage,
				ErrorCode:          errorCode,
				PlatformErrorCode:  pgtype.Text{},
				LastError:          lastError,
				NextRunAt:          pgtype.Timestamptz{Time: time.Now().Add(retryBackoff(job.Attempts)), Valid: true},
			}); err != nil {
				return err
			}
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
		if err != nil {
			return err
		}
		failureApplied = true
		return nil
	})
	if transitionErr != nil {
		if errors.Is(transitionErr, pgx.ErrNoRows) {
			return nil
		}
		return transitionErr
	}

	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, post.ID)
	if !failureApplied {
		return nil
	}
	return nil
}

func (h *SocialPostHandler) cancelStaleDeliveryJobForDeletedPost(ctx context.Context, job db.PostDeliveryJob) error {
	msg := "delivery job cancelled because its parent post was deleted"
	_, err := h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
		job,
		"cancelled",
		pgtype.Text{String: "post_deleted", Valid: true},
		pgtype.Text{String: "post_deleted", Valid: true},
		pgtype.Text{},
		pgtype.Text{String: msg, Valid: true},
		pgtype.Timestamptz{},
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func sanitizeDeliveryErrorText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToValidUTF8(s, "")
	s = strings.ReplaceAll(s, "\x00", "")
	s = deliveryAccessTokenParamPattern.ReplaceAllString(s, `${1}[REDACTED]`)
	s = deliveryBearerTokenPattern.ReplaceAllString(s, `${1}[REDACTED]`)
	return s
}

func sanitizeDeliveryErrorTextValue(v pgtype.Text) pgtype.Text {
	if !v.Valid {
		return v
	}
	v.String = sanitizeDeliveryErrorText(v.String)
	v.Valid = strings.TrimSpace(v.String) != ""
	return v
}

func (h *SocialPostHandler) RequeueDeliveryJob(ctx context.Context, workspaceID, jobID string) (db.PostDeliveryJob, error) {
	job, err := h.queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID:          jobID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	return h.EnqueueRetryForResult(ctx, workspaceID, job.PostID, job.SocialPostResultID)
}

func (h *SocialPostHandler) EnqueueRetryForResult(ctx context.Context, workspaceID, postID, resultID string) (db.PostDeliveryJob, error) {
	var post db.SocialPost
	var queued db.PostDeliveryJob
	var pendingEvent *pendingParentPostStatusEvent
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, postID); err != nil {
			return err
		}
		var err error
		post, err = txQueries.GetSocialPostByIDAndWorkspace(ctx, db.GetSocialPostByIDAndWorkspaceParams{
			ID: postID, WorkspaceID: workspaceID,
		})
		if err != nil {
			return err
		}
		result, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
			ID: resultID, PostID: post.ID,
		})
		if err != nil {
			return err
		}
		if result.Status != "failed" {
			return fmt.Errorf("only failed deliveries can be retried")
		}
		txHandler := h.withQueueQueries(txQueries)
		decision, policyErr := txHandler.evaluateRetryPublishingRestriction(ctx, workspaceID, result)
		if policyErr != nil {
			var accountUnavailable *retrySocialAccountUnavailableError
			if errors.As(policyErr, &accountUnavailable) {
				return accountUnavailable
			}
			return &retryPolicyUnavailableError{err: policyErr}
		}
		if decision.Restricted {
			return &retryPublishingRestrictionError{decision: decision}
		}
		jobs, err := txQueries.ListPostDeliveryJobsByResult(ctx, result.ID)
		if err != nil {
			return err
		}
		postInputIndex := int32(findPostInputIndexForResult(post, result))
		platformName := ""
		for _, job := range jobs {
			if isActiveDeliveryJobState(job.State) {
				return h.queueConflictError()
			}
			postInputIndex = job.PostInputIndex
			if job.Platform != "" {
				platformName = job.Platform
			}
		}
		if postInputIndex < 0 {
			return fmt.Errorf("unable to resolve original post input for retry")
		}
		if platformName == "" {
			account, accountErr := txQueries.GetSocialAccount(ctx, result.SocialAccountID)
			if accountErr != nil {
				return accountErr
			}
			platformName = account.Platform
		}
		mediaIDs, mediaOK := decodeMediaIDsForRetention(post)
		if !mediaOK {
			return errRetryMediaReuploadRequired
		}
		queued, err = txQueries.CreateRetryPostDeliveryJobWithMediaActivation(ctx, db.CreateRetryPostDeliveryJobWithMediaActivationParams{
			PostID: post.ID, SocialPostResultID: result.ID, WorkspaceID: post.WorkspaceID,
			SocialAccountID: result.SocialAccountID, Platform: platformName, PostInputIndex: postInputIndex,
			MaxAttempts: int32(defaultDeliveryJobMaxAttempts),
			NextRunAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			MediaIds:    mediaIDs,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			decision, policyErr = txHandler.evaluateRetryPublishingRestriction(ctx, workspaceID, result)
			if policyErr != nil {
				var accountUnavailable *retrySocialAccountUnavailableError
				if errors.As(policyErr, &accountUnavailable) {
					return accountUnavailable
				}
				return &retryPolicyUnavailableError{err: policyErr}
			}
			if decision.Restricted {
				return &retryPublishingRestrictionError{decision: decision}
			}
			current, currentErr := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: result.ID, PostID: post.ID})
			if currentErr == nil && current.Status != "failed" {
				return fmt.Errorf("only failed deliveries can be retried")
			}
			currentJobs, jobsErr := txQueries.ListPostDeliveryJobsByResult(ctx, result.ID)
			if jobsErr == nil {
				for _, currentJob := range currentJobs {
					if isActiveDeliveryJobState(currentJob.State) {
						return h.queueConflictError()
					}
				}
			}
			return errRetryMediaReuploadRequired
		}
		if err != nil {
			return err
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, post.ID)
		return err
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, post.ID)
	return queued, nil
}

// DismissDeliveryJob archives a terminal (dead/failed/cancelled)
// delivery job from the queue view. The row is preserved for audit
// — analytics still sees the underlying social_post_result.failed
// row — but the queue list and summary skip dismissed rows so users
// can clear non-actionable failures from their view.
func (h *SocialPostHandler) DismissDeliveryJob(ctx context.Context, workspaceID, jobID string) (db.PostDeliveryJob, error) {
	job, err := h.queries.DismissPostDeliveryJob(ctx, db.DismissPostDeliveryJobParams{
		ID:          jobID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		// pgx.ErrNoRows wraps as "no rows in result set" — surface a
		// readable message rather than the SQL noise.
		if strings.Contains(err.Error(), "no rows") {
			return db.PostDeliveryJob{}, fmt.Errorf("only terminal (dead, failed, cancelled) jobs can be dismissed")
		}
		return db.PostDeliveryJob{}, err
	}
	return job, nil
}

func (h *SocialPostHandler) CancelDeliveryJob(ctx context.Context, workspaceID, jobID string) (db.PostDeliveryJob, error) {
	var cancelled db.PostDeliveryJob
	var pendingEvent *pendingParentPostStatusEvent
	err := h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		job, err := txQueries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
			ID:          jobID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return err
		}
		if job.State != "pending" {
			return fmt.Errorf("only queued jobs that have not started provider delivery can be cancelled")
		}
		if err := txQueries.LockPostDeliveryJobFinalization(ctx, job.PostID); err != nil {
			return err
		}
		result, err := txQueries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
			ID: job.SocialPostResultID, PostID: job.PostID,
		})
		if err != nil {
			return err
		}
		if providerTerminalOrAccepted(result, job.Platform) {
			return fmt.Errorf("provider delivery has already started and cannot be cancelled")
		}
		cancelled, err = txQueries.CancelPostDeliveryJob(ctx, db.CancelPostDeliveryJobParams{
			ID:          job.ID,
			WorkspaceID: workspaceID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("only queued jobs that have not started provider delivery can be cancelled")
		}
		if err != nil {
			return err
		}
		cancelMessage := "Delivery cancelled before provider dispatch."
		if _, err := txQueries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
			ID: result.ID, Status: "failed", ExternalID: pgtype.Text{},
			ErrorMessage: pgtype.Text{String: cancelMessage, Valid: true}, PublishedAt: pgtype.Timestamptz{},
			Url: pgtype.Text{}, DebugCurl: pgtype.Text{},
		}); err != nil {
			return err
		}
		cancelFailure := postfailures.BuildParams(
			job.PostID, result.ID, workspaceID, result.SocialAccountID, job.Platform,
			"queue_cancelled", cancelMessage, cancelMessage,
		)
		cancelFailure.ErrorCode = "delivery_cancelled"
		cancelFailure.IsRetriable = false
		cancelFailure.ErrorSource = pgtype.Text{String: postfailures.ErrorSourceUnipost, Valid: true}
		cancelFailure.ErrorTemporality = pgtype.Text{String: postfailures.ErrorTemporalityPermanent, Valid: true}
		if err := txQueries.UpdateSocialPostResultFailureDetails(ctx, updateSocialPostResultFailureDetailsParams(result.ID, cancelFailure)); err != nil {
			return err
		}
		pendingEvent, err = h.refreshTerminalParentPostStatusLocked(ctx, txQueries, cancelled.PostID)
		return err
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	h.publishParentStatusEventIfCurrent(ctx, pendingEvent)
	h.syncTerminalPostRetentionAfterCommit(ctx, cancelled.PostID)
	return cancelled, nil
}

func (h *SocialPostHandler) CleanupSucceededDeliveryJobs(ctx context.Context, maxAge time.Duration) error {
	return h.queries.DeleteOldSucceededPostDeliveryJobs(ctx, pgtype.Interval{
		Microseconds: int64(maxAge / time.Microsecond),
		Valid:        true,
	})
}

func (h *SocialPostHandler) queueConflictError() error {
	return fmt.Errorf("This delivery already has an active queue job.")
}

func isQueueConflict(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "active queue job") {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "post_delivery_jobs_one_active_per_result_idx"
}

type postDeliveryJobResponse struct {
	ID                 string     `json:"id"`
	PostID             string     `json:"post_id"`
	SocialPostResultID string     `json:"social_post_result_id"`
	SocialAccountID    string     `json:"social_account_id"`
	Platform           string     `json:"platform"`
	Kind               string     `json:"kind"`
	State              string     `json:"state"`
	DeliveryPhase      string     `json:"delivery_phase"`
	Attempts           int32      `json:"attempts"`
	MaxAttempts        int32      `json:"max_attempts"`
	FailureStage       *string    `json:"failure_stage,omitempty"`
	ErrorCode          *string    `json:"error_code,omitempty"`
	PlatformErrorCode  *string    `json:"platform_error_code,omitempty"`
	LastError          *string    `json:"last_error,omitempty"`
	NextRunAt          *time.Time `json:"next_run_at,omitempty"`
	LastAttemptAt      *time.Time `json:"last_attempt_at,omitempty"`
	FirstClaimedAt     *time.Time `json:"first_claimed_at,omitempty"`
	PlatformStartedAt  *time.Time `json:"platform_started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	QueueWaitMS        *int64     `json:"queue_wait_ms,omitempty"`
	WorkerWaitMS       *int64     `json:"worker_wait_ms,omitempty"`
	PlatformDurationMS *int64     `json:"platform_duration_ms,omitempty"`
	QueuedAt           time.Time  `json:"queued_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func postDeliveryJobResponseFromRow(row db.PostDeliveryJob) postDeliveryJobResponse {
	return postDeliveryJobResponseFromRowAt(row, time.Now())
}

func postDeliveryJobResponseFromRowAt(row db.PostDeliveryJob, now time.Time) postDeliveryJobResponse {
	resp := postDeliveryJobResponse{
		ID:                 row.ID,
		PostID:             row.PostID,
		SocialPostResultID: row.SocialPostResultID,
		SocialAccountID:    row.SocialAccountID,
		Platform:           row.Platform,
		Kind:               row.Kind,
		State:              row.State,
		DeliveryPhase:      deriveDeliveryPhase(row, now),
		Attempts:           row.Attempts,
		MaxAttempts:        row.MaxAttempts,
		QueuedAt:           row.CreatedAt.Time,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
	if row.FailureStage.Valid {
		resp.FailureStage = &row.FailureStage.String
	}
	if row.ErrorCode.Valid {
		resp.ErrorCode = &row.ErrorCode.String
	}
	if row.PlatformErrorCode.Valid {
		resp.PlatformErrorCode = &row.PlatformErrorCode.String
	}
	if row.LastError.Valid {
		resp.LastError = &row.LastError.String
	}
	if row.NextRunAt.Valid {
		resp.NextRunAt = &row.NextRunAt.Time
	}
	if row.LastAttemptAt.Valid {
		resp.LastAttemptAt = &row.LastAttemptAt.Time
	}
	if row.FirstClaimedAt.Valid {
		resp.FirstClaimedAt = &row.FirstClaimedAt.Time
	}
	if row.PlatformStartedAt.Valid {
		resp.PlatformStartedAt = &row.PlatformStartedAt.Time
	}
	if row.FinishedAt.Valid {
		resp.FinishedAt = &row.FinishedAt.Time
	}
	resp.QueueWaitMS = queueWaitMS(row, now)
	resp.WorkerWaitMS = workerWaitMS(row, now)
	resp.PlatformDurationMS = platformDurationMS(row, now)
	return resp
}

func deriveDeliveryPhase(row db.PostDeliveryJob, now time.Time) string {
	switch row.State {
	case "succeeded":
		return "published"
	case "failed", "dead":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "pending":
		if row.Kind == "retry" {
			if row.NextRunAt.Valid && row.NextRunAt.Time.After(now) {
				return "waiting_retry"
			}
			return "queued_retry"
		}
		return "queued"
	case "running":
		if row.PlatformStartedAt.Valid {
			return "dispatching"
		}
		return "reserved"
	case "retrying":
		if row.PlatformStartedAt.Valid {
			return "retrying"
		}
		return "reserved"
	default:
		return row.State
	}
}

func queueWaitMS(row db.PostDeliveryJob, now time.Time) *int64 {
	if !row.CreatedAt.Valid {
		return nil
	}
	start := row.CreatedAt.Time
	if row.Kind == "retry" && row.NextRunAt.Valid {
		start = row.NextRunAt.Time
	}
	end := now
	if row.LastAttemptAt.Valid {
		end = row.LastAttemptAt.Time
	}
	return durationMSPtr(start, end)
}

func workerWaitMS(row db.PostDeliveryJob, now time.Time) *int64 {
	if !row.LastAttemptAt.Valid {
		return nil
	}
	end := now
	if row.PlatformStartedAt.Valid {
		end = row.PlatformStartedAt.Time
	}
	return durationMSPtr(row.LastAttemptAt.Time, end)
}

func platformDurationMS(row db.PostDeliveryJob, now time.Time) *int64 {
	if !row.PlatformStartedAt.Valid {
		return nil
	}
	end := now
	if row.FinishedAt.Valid {
		end = row.FinishedAt.Time
	}
	return durationMSPtr(row.PlatformStartedAt.Time, end)
}

func durationMSPtr(start, end time.Time) *int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	ms := int64(end.Sub(start) / time.Millisecond)
	return &ms
}

func (h *SocialPostHandler) ListDeliveryJobs(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	limit := parseLimitParam(r.URL.Query().Get("limit"), 50, 200)
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		fmt.Sscanf(raw, "%d", &offset)
	}
	states := r.URL.Query().Get("states")
	rows, err := h.queries.ListPostDeliveryJobsByWorkspace(r.Context(), db.ListPostDeliveryJobsByWorkspaceParams{
		WorkspaceID: workspaceID,
		States:      pgtype.Text{String: states, Valid: states != ""},
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list queue jobs")
		return
	}
	resp := make([]postDeliveryJobResponse, 0, len(rows))
	for _, row := range rows {
		if row.State == "succeeded" || row.State == "cancelled" {
			continue
		}
		resp = append(resp, postDeliveryJobResponseFromRow(row))
	}
	writeSuccess(w, resp)
}

func (h *SocialPostHandler) GetDeliveryJobsSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	summary, err := h.queries.GetPostDeliveryJobsSummaryByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load queue summary")
		return
	}
	writeSuccess(w, summary)
}

func (h *SocialPostHandler) GetPostQueue(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}
	post, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
		return
	}
	results, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)
	jobs, err := h.queries.ListPostDeliveryJobsByPost(r.Context(), post.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load queue jobs")
		return
	}
	resp := h.socialPostResponseFromData(post, results, jobs, "async")
	writeSuccess(w, map[string]any{
		"post": resp,
		"jobs": mapJobsForQueue(jobs),
	})
}

func mapJobsForQueue(rows []db.PostDeliveryJob) []postDeliveryJobResponse {
	resp := make([]postDeliveryJobResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, postDeliveryJobResponseFromRow(row))
	}
	return resp
}

func (h *SocialPostHandler) RetryDeliveryJob(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing job id")
		return
	}
	job, err := h.RequeueDeliveryJob(r.Context(), workspaceID, jobID)
	if err != nil {
		var restrictionErr *retryPublishingRestrictionError
		if errors.As(err, &restrictionErr) {
			writePublishingRestrictionError(w, restrictionErr.decision)
			return
		}
		var policyUnavailable *retryPolicyUnavailableError
		if errors.As(err, &policyUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Publishing policy is temporarily unavailable")
			return
		}
		var accountUnavailable *retrySocialAccountUnavailableError
		if errors.As(err, &accountUnavailable) {
			writeRetrySocialAccountUnavailable(w)
			return
		}
		if isQueueConflict(err) {
			writeError(w, http.StatusConflict, "QUEUE_JOB_ACTIVE", err.Error())
			return
		}
		if errors.Is(err, errRetryMediaReuploadRequired) {
			writeError(w, http.StatusConflict, "MEDIA_REUPLOAD_REQUIRED", "The retained media is no longer available. Upload the media again before retrying.")
			return
		}
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	writeSuccess(w, postDeliveryJobResponseFromRow(job))
}

// RetryDeliveryJobNow is the legacy alias for RetryDeliveryJob.
// Retained for compatibility while clients migrate from /retry-now
// to the canonical /retry command route.
func (h *SocialPostHandler) RetryDeliveryJobNow(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Sunset", "Tue, 31 Mar 2027 00:00:00 GMT")
	w.Header().Set("Link", `</v1/post-delivery-jobs/`+chi.URLParam(r, "jobID")+`/retry>; rel="successor-version"`)
	h.RetryDeliveryJob(w, r)
}

func (h *SocialPostHandler) CancelDeliveryJobHandler(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing job id")
		return
	}
	job, err := h.CancelDeliveryJob(r.Context(), workspaceID, jobID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	writeSuccess(w, postDeliveryJobResponseFromRow(job))
}

func (h *SocialPostHandler) DismissDeliveryJobHandler(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing job id")
		return
	}
	job, err := h.DismissDeliveryJob(r.Context(), workspaceID, jobID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	writeSuccess(w, postDeliveryJobResponseFromRow(job))
}
