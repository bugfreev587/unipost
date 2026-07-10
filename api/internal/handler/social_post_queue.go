package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
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
	if ok {
		platformName = dbAcc.Platform
		if socialAccountDisconnectedForPublish(dbAcc, true) {
			return platformName, fmt.Errorf("account is disconnected")
		}
		return platformName, nil
	}
	platformName = fallback.Platform
	if fallback.Platform == "" {
		return platformName, fmt.Errorf("account not found")
	}
	if fallback.Disconnected {
		return platformName, fmt.Errorf("account is disconnected")
	}
	return platformName, fmt.Errorf("account not found")
}

func (h *SocialPostHandler) enqueueParsedPostDeliveries(
	ctx context.Context,
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
) ([]db.SocialPostResult, []db.PostDeliveryJob, error) {
	dbAccounts := h.loadDBAccountsByIDs(ctx, post.WorkspaceID, uniqueAccountIDs(parsed))
	results := make([]db.SocialPostResult, 0, len(parsed))
	jobs := make([]db.PostDeliveryJob, 0, len(parsed))
	failureSummaries := make([]string, 0)

	for idx, pp := range parsed {
		acc, ok := dbAccounts[pp.AccountID]
		platformName, validationErr := summarizeAccountValidation(acc, ok, accountMap[pp.AccountID])

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
			h.recordPostFailure(ctx, postfailures.BuildParams(
				post.ID,
				res.ID,
				post.WorkspaceID,
				pp.AccountID,
				postfailures.FirstNonEmpty(platformName, accountMap[pp.AccountID].Platform),
				"dispatch_prepare",
				validationErr.Error(),
				validationErr.Error(),
			))
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
	h.syncPostMediaRetention(ctx, post, newStatus)
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

	results, jobs, err := h.enqueueParsedPostDeliveries(ctx, post, parsed.Posts, accountMap)
	if err != nil {
		return socialPostResponse{}, err
	}
	h.logPublishingEvent(ctx, integrationlogs.Event{
		WorkspaceID: workspaceID,
		Level:       integrationlogs.LevelInfo,
		Status:      integrationlogs.StatusSuccess,
		Action:      integrationlogs.ActionPostPublishQueued,
		Message:     "Queued post deliveries for publishing.",
		PostID:      post.ID,
		Metadata: map[string]any{
			"mode":            "immediate",
			"target_count":    len(parsed.Posts),
			"queued_jobs":     len(jobs),
			"result_count":    len(results),
			"target_accounts": uniqueAccountIDs(parsed.Posts),
		},
	})
	return h.socialPostResponseFromData(post, results, jobs, "async"), nil
}

func (h *SocialPostHandler) enqueueExistingPostDeliveries(
	ctx context.Context,
	post db.SocialPost,
	parsed []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
) (socialPostResponse, error) {
	results, jobs, err := h.enqueueParsedPostDeliveries(ctx, post, parsed, accountMap)
	if err != nil {
		return socialPostResponse{}, err
	}
	h.logPublishingEvent(ctx, integrationlogs.Event{
		WorkspaceID: post.WorkspaceID,
		Level:       integrationlogs.LevelInfo,
		Status:      integrationlogs.StatusSuccess,
		Action:      integrationlogs.ActionPostPublishQueued,
		Message:     "Queued draft deliveries for publishing.",
		PostID:      post.ID,
		Metadata: map[string]any{
			"mode":            "draft_publish",
			"target_count":    len(parsed),
			"queued_jobs":     len(jobs),
			"result_count":    len(results),
			"target_accounts": uniqueAccountIDs(parsed),
		},
	})
	return h.socialPostResponseFromData(post, results, jobs, "async"), nil
}

func (h *SocialPostHandler) EnqueueScheduledPost(ctx context.Context, post db.SocialPost) error {
	parentCaption := ""
	if post.Caption.Valid {
		parentCaption = post.Caption.String
	}
	parsed, err := platform.DecodePostMetadata(post.Metadata, parentCaption)
	if err != nil || len(parsed) == 0 {
		_ = h.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
			ID:          post.ID,
			Status:      "failed",
			PublishedAt: pgtype.Timestamptz{},
		})
		post.Status = "failed"
		post.PublishedAt = pgtype.Timestamptz{}
		h.syncPostMediaRetention(ctx, post, post.Status)
		return fmt.Errorf("decode post metadata: %w", err)
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
	quotaUnits := countPublishQuotaUnits(parsed, accountMap)
	if status, blocked := h.checkFreePlanPostQuota(ctx, post.WorkspaceID, quotaUnits); blocked {
		h.maybeSendFreePlanQuotaEmail(ctx, post.WorkspaceID, quotaemail.Evaluation{
			Blocked:        true,
			RequestedUnits: quotaUnits,
		})
		return h.failScheduledPostForQuota(ctx, post, parsed, accountMap, status, quotaUnits)
	}
	_, _, err = h.enqueueParsedPostDeliveries(ctx, post, parsed, accountMap)
	return err
}

func (h *SocialPostHandler) failScheduledPostForQuota(ctx context.Context, post db.SocialPost, parsed []platform.PlatformPostInput, accountMap map[string]platform.ValidateAccount, status quota.QuotaStatus, requestedUnits int) error {
	msg := freePlanQuotaExceededMessage(status, requestedUnits)
	summaries := make([]string, 0, len(parsed))

	for _, pp := range parsed {
		platformName := accountMap[pp.AccountID].Platform
		res, err := h.queries.CreateSocialPostResult(ctx, db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: pp.AccountID,
			Caption:         pp.Caption,
			Status:          "failed",
			ExternalID:      pgtype.Text{},
			ErrorMessage:    pgtype.Text{String: msg, Valid: true},
			PublishedAt:     pgtype.Timestamptz{},
			Url:             pgtype.Text{},
			DebugCurl:       pgtype.Text{},
			FbMediaType:     pgtype.Text{},
		})
		if err != nil {
			return err
		}
		summaries = append(summaries, fmt.Sprintf("[%s] %s", pp.AccountID, msg))
		h.recordPostFailure(ctx, postfailures.BuildParams(
			post.ID,
			res.ID,
			post.WorkspaceID,
			pp.AccountID,
			platformName,
			"quota",
			msg,
			msg,
		))
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
	h.syncPostMediaRetention(ctx, post, post.Status)
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

func markDeliveryJobSucceededParams(job db.PostDeliveryJob) db.MarkPostDeliveryJobSucceededParams {
	return db.MarkPostDeliveryJobSucceededParams{
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
func (h *SocialPostHandler) attachPublishTokenResume(ctx context.Context, pp *platform.PlatformPostInput, res db.SocialPostResult) {
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
	pp.PlatformOptions[platform.OptOnPublishToken] = func(token string) {
		if token == "" {
			return
		}
		if err := h.queries.SetSocialPostResultPublishToken(ctx, db.SetSocialPostResultPublishTokenParams{
			ID:           resultID,
			PublishToken: pgtype.Text{String: token, Valid: true},
		}); err != nil {
			slog.Warn("publish token persist failed", "result_id", resultID, "error", err)
		}
	}
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
	if res.Status == "published" {
		slog.Info("delivery job: result already published, skipping duplicate publish",
			"job_id", job.ID, "post_id", job.PostID, "result_id", res.ID)
		if _, err := h.queries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job)); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	}

	pp, err := platformPostInputAtIndex(post, int(job.PostInputIndex))
	if err != nil {
		return h.finalizeJobLoadFailure(ctx, job, res, post, err)
	}

	dbAccounts := h.loadDBAccountsByIDs(ctx, post.WorkspaceID, []string{pp.AccountID})
	accountMap := map[string]platform.ValidateAccount{}
	if acc, ok := dbAccounts[pp.AccountID]; ok {
		accountMap[pp.AccountID] = platform.ValidateAccount{
			Platform:     acc.Platform,
			Disconnected: socialAccountDisconnectedForPublish(acc, true),
		}
	}

	// Idempotent-publish wiring (IG/TikTok). Let the adapter resume from a
	// token a prior attempt persisted, and persist the token this attempt
	// obtains, so a crash between "media uploaded" and "external_id recorded"
	// re-uses the same container/publish_id instead of duplicating the post.
	h.attachPublishTokenResume(ctx, &pp, res)

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
		pp,
		dbAccounts,
		accountMap,
		h.buildPerAccountTracker(ctx, post.WorkspaceID, pp.AccountID),
		quota.NewPerPlatformDailyTracker(ctx, h.queries, dailyTargetsFor([]platform.PlatformPostInput{pp}, accountMap)),
		h.disallowedPlatformsForDispatch(ctx, post.WorkspaceID, []platform.PlatformPostInput{pp}, accountMap),
	)
	if oc.err != nil {
		return h.handleJobDispatchFailure(ctx, post, res, job, oc)
	}

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
		if status == "published" {
			publishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}
	}
	updated, err := h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           res.ID,
		Status:       status,
		ExternalID:   externalID,
		ErrorMessage: pgtype.Text{},
		PublishedAt:  publishedAt,
		Url:          postURL,
		DebugCurl:    debugCurl,
	})
	if err != nil {
		return err
	}
	if _, err := h.queries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job)); err != nil {
		return err
	}
	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
	if updated.Status == "published" {
		h.quota.Increment(ctx, post.WorkspaceID, 1)
		h.maybeSendFreePlanQuotaEmail(ctx, post.WorkspaceID, quotaemail.Evaluation{})
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

func (h *SocialPostHandler) finalizeJobLoadFailure(ctx context.Context, job db.PostDeliveryJob, res db.SocialPostResult, post db.SocialPost, dispatchErr error) error {
	msg := sanitizeDeliveryErrorText(dispatchErr.Error())
	_, _ = h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           res.ID,
		Status:       "failed",
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: msg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		DebugCurl:    pgtype.Text{},
	})
	_, _ = h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
		job,
		"dead",
		pgtype.Text{String: "dispatch_prepare", Valid: true},
		pgtype.Text{String: "validation_error", Valid: true},
		pgtype.Text{},
		pgtype.Text{String: msg, Valid: true},
		pgtype.Timestamptz{},
	))
	failure := postfailures.BuildParams(
		post.ID, res.ID, post.WorkspaceID, res.SocialAccountID, job.Platform, "dispatch_prepare", msg, msg,
	)
	h.recordPostFailure(ctx, failure)
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
	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
	h.syncLoopsPostFailed(ctx, post, res, job, failure, false)
	return nil
}

func (h *SocialPostHandler) handleJobDispatchFailure(ctx context.Context, post db.SocialPost, res db.SocialPostResult, job db.PostDeliveryJob, oc publishOneOutcome) error {
	errMsg := sanitizeDeliveryErrorText(oc.err.Error())
	debugCurl := pgtype.Text{}
	if sanitizedDebugCurl := sanitizeDeliveryErrorText(oc.debugCurl); sanitizedDebugCurl != "" {
		debugCurl = pgtype.Text{String: sanitizedDebugCurl, Valid: true}
	}
	failureStage := inferDispatchFailureStage(errMsg)

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

	if _, err := h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           res.ID,
		Status:       resultStatus,
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		DebugCurl:    debugCurl,
	}); err != nil {
		return err
	}
	h.recordPostFailure(ctx, failure)

	if failure.IsRetriable {
		if job.Kind == "dispatch" {
			if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
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
			if _, err := h.queries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
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
		} else {
			nextRunAt := pgtype.Timestamptz{Time: time.Now().Add(retryBackoff(job.Attempts)), Valid: true}
			state := "pending"
			if job.Attempts >= job.MaxAttempts {
				state = "dead"
				nextRunAt = pgtype.Timestamptz{}
			}
			if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
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
		if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
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

	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
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
			"error":      errMsg,
			"debug_curl": oc.debugCurl,
		},
	}))
	h.syncLoopsPostFailed(ctx, post, res, job, failure, anotherAttempt)
	return nil
}

func inferDispatchFailureStage(errMsg string) string {
	msg := strings.ToLower(strings.TrimSpace(errMsg))
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
	if result.Status == "published" {
		slog.Info("stale recovery: result already published, closing job without retry",
			"job_id", job.ID, "post_id", job.PostID, "result_id", result.ID)
		if _, err := h.queries.MarkPostDeliveryJobSucceeded(ctx, markDeliveryJobSucceededParams(job)); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
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

	if _, err := h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           result.ID,
		Status:       resultStatus,
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		DebugCurl:    pgtype.Text{},
	}); err != nil {
		return err
	}

	failureStage := pgtype.Text{String: "worker_timeout", Valid: true}
	errorCode := pgtype.Text{String: "worker_stalled", Valid: true}
	lastError := pgtype.Text{String: errMsg, Valid: true}

	switch job.Kind {
	case "dispatch":
		if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
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
		if _, err := h.queries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
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
	default:
		nextRunAt := pgtype.Timestamptz{Time: time.Now().Add(retryBackoff(job.Attempts)), Valid: true}
		state := "pending"
		if job.Attempts >= job.MaxAttempts {
			state = "dead"
			nextRunAt = pgtype.Timestamptz{}
		}
		if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, markDeliveryJobFailedParams(
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

	h.recordPostFailure(ctx, postfailures.BuildParams(
		post.ID,
		result.ID,
		post.WorkspaceID,
		result.SocialAccountID,
		postfailures.FirstNonEmpty(job.Platform),
		"worker_timeout",
		errMsg,
		errMsg,
	))
	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
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
	activeJobs, err := h.queries.ListPostDeliveryJobsByResult(ctx, job.SocialPostResultID)
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	for _, candidate := range activeJobs {
		if candidate.ID != job.ID && (candidate.State == "pending" || candidate.State == "running" || candidate.State == "retrying") {
			return db.PostDeliveryJob{}, h.queueConflictError()
		}
	}
	post, err := h.queries.GetSocialPostByID(ctx, job.PostID)
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	result, err := h.queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
		ID:     job.SocialPostResultID,
		PostID: job.PostID,
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	if result.Status != "failed" {
		return db.PostDeliveryJob{}, fmt.Errorf("only failed deliveries can be retried")
	}
	job, err = h.queries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
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
		NextRunAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	h.syncPostMediaRetention(ctx, post, "publishing")
	return job, nil
}

func (h *SocialPostHandler) EnqueueRetryForResult(ctx context.Context, workspaceID, postID, resultID string) (db.PostDeliveryJob, error) {
	post, err := h.queries.GetSocialPostByIDAndWorkspace(ctx, db.GetSocialPostByIDAndWorkspaceParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	result, err := h.queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
		ID:     resultID,
		PostID: post.ID,
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	if result.Status != "failed" {
		return db.PostDeliveryJob{}, fmt.Errorf("only failed deliveries can be retried")
	}

	jobs, err := h.queries.ListPostDeliveryJobsByResult(ctx, result.ID)
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	postInputIndex := int32(findPostInputIndexForResult(post, result))
	platformName := ""
	for _, job := range jobs {
		if job.State == "pending" || job.State == "running" || job.State == "retrying" {
			return db.PostDeliveryJob{}, h.queueConflictError()
		}
		postInputIndex = job.PostInputIndex
		if job.Platform != "" {
			platformName = job.Platform
		}
	}
	if postInputIndex < 0 {
		return db.PostDeliveryJob{}, fmt.Errorf("unable to resolve original post input for retry")
	}
	if platformName == "" {
		if acc, accErr := h.queries.GetSocialAccount(ctx, result.SocialAccountID); accErr == nil {
			platformName = acc.Platform
		}
	}
	return h.queries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
		PostID:             post.ID,
		SocialPostResultID: result.ID,
		WorkspaceID:        post.WorkspaceID,
		SocialAccountID:    result.SocialAccountID,
		Platform:           platformName,
		PostInputIndex:     postInputIndex,
		Kind:               "retry",
		State:              "pending",
		Attempts:           0,
		MaxAttempts:        int32(defaultDeliveryJobMaxAttempts),
		NextRunAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
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
	job, err := h.queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID:          jobID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	if job.State != "pending" && job.State != "retrying" {
		return db.PostDeliveryJob{}, fmt.Errorf("only pending or retrying jobs can be cancelled")
	}
	cancelled, err := h.queries.CancelPostDeliveryJob(ctx, job.ID)
	if err != nil {
		return db.PostDeliveryJob{}, err
	}
	post, err := h.queries.GetSocialPostByID(ctx, cancelled.PostID)
	if err == nil {
		if results, listErr := h.queries.ListSocialPostResultsByPost(ctx, cancelled.PostID); listErr == nil {
			h.refreshParentPostStatusContext(ctx, post, results)
		}
	}
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
	return err != nil && strings.Contains(err.Error(), "active queue job")
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
		if isQueueConflict(err) {
			writeError(w, http.StatusConflict, "QUEUE_JOB_ACTIVE", err.Error())
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
