package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

const (
	defaultDeliveryJobMaxAttempts = 5
	staleDeliveryAttemptTimeout   = 5 * time.Minute
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
		if dbAcc.DisconnectedAt.Valid {
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
		return fmt.Errorf("decode post metadata: %w", err)
	}

	accountMap := make(map[string]platform.ValidateAccount, len(parsed))
	dbAccounts := h.loadDBAccountsByIDs(ctx, post.WorkspaceID, uniqueAccountIDs(parsed))
	for _, pp := range parsed {
		acc, ok := dbAccounts[pp.AccountID]
		accountMap[pp.AccountID] = platform.ValidateAccount{
			Platform:     acc.Platform,
			Disconnected: ok && acc.DisconnectedAt.Valid,
		}
	}
	_, _, err = h.enqueueParsedPostDeliveries(ctx, post, parsed, accountMap)
	return err
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

func (h *SocialPostHandler) ProcessPostDeliveryJob(ctx context.Context, job db.PostDeliveryJob) error {
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

	pp, err := platformPostInputAtIndex(post, int(job.PostInputIndex))
	if err != nil {
		return h.finalizeJobLoadFailure(ctx, job, res, post, err)
	}

	dbAccounts := h.loadDBAccountsByIDs(ctx, post.WorkspaceID, []string{pp.AccountID})
	accountMap := map[string]platform.ValidateAccount{}
	if acc, ok := dbAccounts[pp.AccountID]; ok {
		accountMap[pp.AccountID] = platform.ValidateAccount{
			Platform:     acc.Platform,
			Disconnected: acc.DisconnectedAt.Valid,
		}
	}

	oc := h.publishOneContext(ctx, pp, dbAccounts, accountMap, h.buildPerAccountTracker(ctx, post.WorkspaceID, pp.AccountID))
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
	if _, err := h.queries.MarkPostDeliveryJobSucceeded(ctx, job.ID); err != nil {
		return err
	}
	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
	if updated.Status == "published" {
		h.quota.Increment(ctx, post.WorkspaceID, 1)
	}
	return nil
}

func (h *SocialPostHandler) finalizeJobLoadFailure(ctx context.Context, job db.PostDeliveryJob, res db.SocialPostResult, post db.SocialPost, dispatchErr error) error {
	msg := dispatchErr.Error()
	_, _ = h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           res.ID,
		Status:       "failed",
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: msg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		DebugCurl:    pgtype.Text{},
	})
	_, _ = h.queries.MarkPostDeliveryJobFailed(ctx, db.MarkPostDeliveryJobFailedParams{
		ID:                job.ID,
		State:             "dead",
		FailureStage:      pgtype.Text{String: "dispatch_prepare", Valid: true},
		ErrorCode:         pgtype.Text{String: "validation_error", Valid: true},
		PlatformErrorCode: pgtype.Text{},
		LastError:         pgtype.Text{String: msg, Valid: true},
		NextRunAt:         pgtype.Timestamptz{},
	})
	h.recordPostFailure(ctx, postfailures.BuildParams(
		post.ID, res.ID, post.WorkspaceID, res.SocialAccountID, job.Platform, "dispatch_prepare", msg, msg,
	))
	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
	return nil
}

func (h *SocialPostHandler) handleJobDispatchFailure(ctx context.Context, post db.SocialPost, res db.SocialPostResult, job db.PostDeliveryJob, oc publishOneOutcome) error {
	errMsg := oc.err.Error()
	debugCurl := pgtype.Text{}
	if oc.debugCurl != "" {
		debugCurl = pgtype.Text{String: oc.debugCurl, Valid: true}
	}
	updated, err := h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           res.ID,
		Status:       "failed",
		ExternalID:   pgtype.Text{},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		PublishedAt:  pgtype.Timestamptz{},
		Url:          pgtype.Text{},
		DebugCurl:    debugCurl,
	})
	if err != nil {
		return err
	}

	failure := postfailures.BuildParams(
		post.ID,
		updated.ID,
		post.WorkspaceID,
		updated.SocialAccountID,
		postfailures.FirstNonEmpty(oc.platform, job.Platform),
		"dispatch",
		errMsg,
		errMsg,
	)
	h.recordPostFailure(ctx, failure)

	if failure.IsRetriable {
		if job.Kind == "dispatch" {
			if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, db.MarkPostDeliveryJobFailedParams{
				ID:                job.ID,
				State:             "failed",
				FailureStage:      pgtype.Text{String: failure.FailureStage, Valid: true},
				ErrorCode:         pgtype.Text{String: failure.ErrorCode, Valid: true},
				PlatformErrorCode: failure.PlatformErrorCode,
				LastError:         pgtype.Text{String: errMsg, Valid: true},
				NextRunAt:         pgtype.Timestamptz{},
			}); err != nil {
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
			if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, db.MarkPostDeliveryJobFailedParams{
				ID:                job.ID,
				State:             state,
				FailureStage:      pgtype.Text{String: failure.FailureStage, Valid: true},
				ErrorCode:         pgtype.Text{String: failure.ErrorCode, Valid: true},
				PlatformErrorCode: failure.PlatformErrorCode,
				LastError:         pgtype.Text{String: errMsg, Valid: true},
				NextRunAt:         nextRunAt,
			}); err != nil {
				return err
			}
		}
	} else {
		if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, db.MarkPostDeliveryJobFailedParams{
			ID:                job.ID,
			State:             "dead",
			FailureStage:      pgtype.Text{String: failure.FailureStage, Valid: true},
			ErrorCode:         pgtype.Text{String: failure.ErrorCode, Valid: true},
			PlatformErrorCode: failure.PlatformErrorCode,
			LastError:         pgtype.Text{String: errMsg, Valid: true},
			NextRunAt:         pgtype.Timestamptz{},
		}); err != nil {
			return err
		}
	}

	allResults, _ := h.queries.ListSocialPostResultsByPost(ctx, post.ID)
	h.refreshParentPostStatusContext(ctx, post, allResults)
	return nil
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
		return err
	}
	result, err := h.queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
		ID:     job.SocialPostResultID,
		PostID: job.PostID,
	})
	if err != nil {
		return err
	}

	errMsg := fmt.Sprintf("delivery attempt stalled after %s and was re-queued automatically", staleDeliveryAttemptTimeout.Round(time.Second))
	if job.Kind == "retry" && job.Attempts >= job.MaxAttempts {
		errMsg = fmt.Sprintf("delivery attempt stalled after %s and exhausted retry attempts", staleDeliveryAttemptTimeout.Round(time.Second))
	}

	if _, err := h.queries.UpdateSocialPostResultAfterRetry(ctx, db.UpdateSocialPostResultAfterRetryParams{
		ID:           result.ID,
		Status:       "failed",
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
		if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, db.MarkPostDeliveryJobFailedParams{
			ID:                job.ID,
			State:             "failed",
			FailureStage:      failureStage,
			ErrorCode:         errorCode,
			PlatformErrorCode: pgtype.Text{},
			LastError:         lastError,
			NextRunAt:         pgtype.Timestamptz{},
		}); err != nil {
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
		if _, err := h.queries.MarkPostDeliveryJobFailed(ctx, db.MarkPostDeliveryJobFailedParams{
			ID:                job.ID,
			State:             state,
			FailureStage:      failureStage,
			ErrorCode:         errorCode,
			PlatformErrorCode: pgtype.Text{},
			LastError:         lastError,
			NextRunAt:         nextRunAt,
		}); err != nil {
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
	return h.queries.CreatePostDeliveryJob(ctx, db.CreatePostDeliveryJobParams{
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
	Attempts           int32      `json:"attempts"`
	MaxAttempts        int32      `json:"max_attempts"`
	FailureStage       *string    `json:"failure_stage,omitempty"`
	ErrorCode          *string    `json:"error_code,omitempty"`
	PlatformErrorCode  *string    `json:"platform_error_code,omitempty"`
	LastError          *string    `json:"last_error,omitempty"`
	NextRunAt          *time.Time `json:"next_run_at,omitempty"`
	LastAttemptAt      *time.Time `json:"last_attempt_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func postDeliveryJobResponseFromRow(row db.PostDeliveryJob) postDeliveryJobResponse {
	resp := postDeliveryJobResponse{
		ID:                 row.ID,
		PostID:             row.PostID,
		SocialPostResultID: row.SocialPostResultID,
		SocialAccountID:    row.SocialAccountID,
		Platform:           row.Platform,
		Kind:               row.Kind,
		State:              row.State,
		Attempts:           row.Attempts,
		MaxAttempts:        row.MaxAttempts,
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
	return resp
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
