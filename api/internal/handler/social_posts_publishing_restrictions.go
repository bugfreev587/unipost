package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type publishingRestrictionEvaluator = publishingrestrictions.Evaluator

const (
	platformOperationDisabledNormalizedCode = "platform_operation_disabled"
	platformOperationDisabledFailureStage   = "platform_operational_control"
	platformOperationDisabledNextAction     = "retry_after_platform_recovery"
)

type transactionBoundPublishingRestrictionEvaluator interface {
	WithDBTX(db.DBTX) publishingrestrictions.Evaluator
}

func (h *SocialPostHandler) withTransactionPublishingRestrictions(dbtx db.DBTX) *SocialPostHandler {
	if h == nil || dbtx == nil {
		return h
	}
	clone := *h
	if binder, ok := h.publishingRestrictions.(transactionBoundPublishingRestrictionEvaluator); ok {
		clone.publishingRestrictions = binder.WithDBTX(dbtx)
	}
	return &clone
}

type publishingRestrictionPolicyEvaluation struct {
	decision publishingrestrictions.Decision
	err      error
}

type publishingRestrictionPolicySnapshot map[string]publishingRestrictionPolicyEvaluation

func (h *SocialPostHandler) SetPublishingRestrictions(evaluator publishingRestrictionEvaluator) *SocialPostHandler {
	if h != nil {
		h.publishingRestrictions = evaluator
	}
	return h
}

func (h *SocialPostHandler) SetPlatformOperationalControls(evaluator platformcontrols.Evaluator) *SocialPostHandler {
	if h != nil {
		h.platformControls = evaluator
	}
	return h
}

func platformControlPublishingDecision(decision platformcontrols.Decision) publishingrestrictions.Decision {
	if !decision.Blocked {
		return publishingrestrictions.Decision{}
	}
	return publishingrestrictions.Decision{
		Restricted: true,
		Platform:   decision.Platform,
		ReasonCode: decision.ReasonCode,
		Code:       platformOperationDisabledNormalizedCode,
		Message:    decisionMessage(decision),
		NextAction: platformOperationDisabledNextAction,
	}
}

func publishingBlockMessage(decision publishingrestrictions.Decision) string {
	if message := strings.TrimSpace(decision.Message); message != "" {
		return message
	}
	return publishingrestrictions.UserMessage
}

func publishingBlockFailureStage(decision publishingrestrictions.Decision) string {
	if decision.Code == platformOperationDisabledNormalizedCode {
		return platformOperationDisabledFailureStage
	}
	return publishingrestrictions.FailureStage
}

func publishingBlockCode(decision publishingrestrictions.Decision) string {
	if code := strings.TrimSpace(decision.Code); code != "" {
		return code
	}
	return publishingrestrictions.NormalizedCode
}

func publishingBlockNextAction(decision publishingrestrictions.Decision) string {
	if action := strings.TrimSpace(decision.NextAction); action != "" {
		return action
	}
	return publishingrestrictions.NextAction
}

func (h *SocialPostHandler) evaluatePublishingRestrictions(
	ctx context.Context,
	workspaceID string,
	posts []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
) (map[string]publishingrestrictions.Decision, error) {
	return h.evaluatePublishingRestrictionsWithSnapshot(ctx, workspaceID, posts, accountMap, nil)
}

func (h *SocialPostHandler) evaluatePublishingRestrictionsWithSnapshot(
	ctx context.Context,
	workspaceID string,
	posts []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
	snapshot publishingRestrictionPolicySnapshot,
) (map[string]publishingrestrictions.Decision, error) {
	blocked, err := h.evaluateOperationalPublishingControls(ctx, posts, accountMap)
	if err != nil {
		return nil, err
	}
	if h == nil || h.publishingRestrictions == nil {
		return blocked, nil
	}
	if snapshot == nil {
		snapshot = make(publishingRestrictionPolicySnapshot)
	}
	for _, post := range posts {
		if blocked[post.AccountID].Restricted {
			continue
		}
		account, trusted := accountMap[post.AccountID]
		if !trusted || account.Disconnected {
			continue
		}
		platformName := strings.ToLower(strings.TrimSpace(account.Platform))
		if platformName == "" {
			continue
		}
		evaluation, evaluated := snapshot[platformName]
		if !evaluated {
			evaluation.decision, evaluation.err = h.publishingRestrictions.Evaluate(ctx, workspaceID, platformName)
			snapshot[platformName] = evaluation
		}
		if evaluation.err != nil {
			return nil, evaluation.err
		}
		if evaluation.decision.Restricted {
			blocked[post.AccountID] = evaluation.decision
		}
	}
	return blocked, nil
}

func (h *SocialPostHandler) evaluateOperationalPublishingControls(
	ctx context.Context,
	posts []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
) (map[string]publishingrestrictions.Decision, error) {
	blocked := make(map[string]publishingrestrictions.Decision)
	if h == nil || h.platformControls == nil {
		return blocked, nil
	}
	type evaluation struct {
		decision platformcontrols.Decision
		err      error
	}
	snapshot := make(map[string]evaluation)
	for _, post := range posts {
		account, trusted := accountMap[post.AccountID]
		if !trusted || account.Disconnected {
			continue
		}
		platformName := strings.ToLower(strings.TrimSpace(account.Platform))
		if platformName == "" {
			continue
		}
		result, ok := snapshot[platformName]
		if !ok {
			result.decision, result.err = h.platformControls.Evaluate(ctx, platformName, platformcontrols.OperationPublish)
			snapshot[platformName] = result
		}
		if result.err != nil {
			return nil, result.err
		}
		if result.decision.Blocked {
			blocked[post.AccountID] = platformControlPublishingDecision(result.decision)
		}
	}
	return blocked, nil
}

func fullyRestrictedDecision(
	posts []platform.PlatformPostInput,
	blocked map[string]publishingrestrictions.Decision,
) (publishingrestrictions.Decision, bool) {
	if len(posts) == 0 {
		return publishingrestrictions.Decision{}, false
	}
	var first publishingrestrictions.Decision
	for i, post := range posts {
		decision, ok := blocked[post.AccountID]
		if !ok || !decision.Restricted {
			return publishingrestrictions.Decision{}, false
		}
		if i == 0 {
			first = decision
		}
	}
	return first, true
}

func allowedPublishingTargets(
	posts []platform.PlatformPostInput,
	blocked map[string]publishingrestrictions.Decision,
) []platform.PlatformPostInput {
	allowed := make([]platform.PlatformPostInput, 0, len(posts))
	for _, post := range posts {
		if decision, restricted := blocked[post.AccountID]; restricted && decision.Restricted {
			continue
		}
		allowed = append(allowed, post)
	}
	return allowed
}

func writePublishingRestrictionError(w http.ResponseWriter, decision publishingrestrictions.Decision) {
	if decision.Code == platformOperationDisabledNormalizedCode {
		isRetriable := false
		writeErrorWithDetails(w, http.StatusServiceUnavailable, platformcontrols.APICode, publishingBlockMessage(decision), ErrorDetails{
			NextAction:       platformOperationDisabledNextAction,
			IsRetriable:      &isRetriable,
			ErrorSource:      "unipost",
			ErrorTemporality: "temporary",
			Details: map[string]any{
				"platform": decision.Platform,
				"state":    "disabled",
			},
		})
		return
	}
	isRetriable := false
	platformName := strings.TrimSpace(decision.Platform)
	if platformName == "" {
		platformName = "tiktok"
	}
	planID := strings.TrimSpace(decision.PlanID)
	if planID == "" {
		planID = "free"
	}
	writeErrorWithDetails(w, http.StatusPaymentRequired, publishingrestrictions.APICode, publishingrestrictions.UserMessage, ErrorDetails{
		NextAction:       publishingrestrictions.NextAction,
		IsRetriable:      &isRetriable,
		ErrorSource:      "unipost",
		ErrorTemporality: "temporary",
		Details:          map[string]any{"platform": platformName, "plan_id": planID},
	})
}

func publishingBlockFailure(postID, resultID, workspaceID, accountID, platformName string, decision publishingrestrictions.Decision) db.CreatePostFailureParams {
	message := publishingBlockMessage(decision)
	failure := postfailures.BuildParams(
		postID, resultID, workspaceID, accountID, platformName,
		publishingBlockFailureStage(decision), message, message,
	)
	failure.ErrorCode = publishingBlockCode(decision)
	failure.FailureStage = publishingBlockFailureStage(decision)
	failure.IsRetriable = false
	failure.PlatformErrorCode = pgtype.Text{}
	failure.ErrorSource = postfailures.ToText("unipost")
	failure.ErrorTemporality = postfailures.ToText("temporary")
	failure.ProviderError = nil
	if strings.TrimSpace(decision.CycleID) != "" {
		failure.RestrictionCycleID = pgtype.Text{String: strings.TrimSpace(decision.CycleID), Valid: true}
	}
	return failure
}

func publishingRestrictionFailure(postID, resultID, workspaceID, accountID, platformName string, cycleID ...string) db.CreatePostFailureParams {
	decision := publishingrestrictions.Decision{Restricted: true, Platform: platformName}
	if len(cycleID) > 0 && strings.TrimSpace(cycleID[0]) != "" {
		decision.CycleID = strings.TrimSpace(cycleID[0])
	}
	return publishingBlockFailure(postID, resultID, workspaceID, accountID, platformName, decision)
}

func (h *SocialPostHandler) applyPublishingRestrictionRetryProjection(
	ctx context.Context,
	workspaceID string,
	post db.SocialPost,
	response *postResultResponse,
	result db.SocialPostResult,
	jobs []db.PostDeliveryJob,
) {
	if response == nil || !result.ErrorCode.Valid || result.ErrorCode.String != publishingrestrictions.NormalizedCode {
		return
	}
	if response.RetryPolicy == nil {
		response.RetryPolicy = deriveRetryPolicy(result, jobs)
	}
	account, accountErr := h.queries.GetSocialAccount(ctx, result.SocialAccountID)
	if accountErr != nil || socialAccountUnavailableForDelivery(account, true) {
		response.RetryPolicy.IsRetriable = false
		response.RetryPolicy.WillRetry = false
		response.RetryPolicy.NextRunAt = nil
		response.RetryPolicy.ManualRetryAllowed = false
		response.RetryPolicy.RetryState = "blocked"
		response.RetryPolicy.Reason = "social_account_not_available"
		return
	}
	restrictionActive := true
	if h != nil && h.publishingRestrictions != nil {
		decision, err := h.publishingRestrictions.Evaluate(ctx, workspaceID, response.Platform)
		if err == nil {
			restrictionActive = decision.Restricted
		}
	}
	mediaAvailable := h.postMediaAvailableForRetry(ctx, post)
	applyPublishingRestrictionRetryEligibility(response.RetryPolicy, restrictionActive, mediaAvailable)
	if retainedUntil, err := h.queries.GetPostPublishingRestrictionMediaRetention(ctx, post.ID); err == nil && retainedUntil.Valid {
		value := retainedUntil.Time.UTC().Format(time.RFC3339)
		response.MediaRetainedUntil = &value
	}
}

func (h *SocialPostHandler) postMediaAvailableForRetry(ctx context.Context, post db.SocialPost) bool {
	ids, ok := decodeMediaIDsForRetention(post)
	if !ok {
		return false
	}
	for _, mediaID := range ids {
		media, err := h.queries.GetMediaByIDAndWorkspace(ctx, db.GetMediaByIDAndWorkspaceParams{
			ID:          mediaID,
			WorkspaceID: post.WorkspaceID,
		})
		if err != nil || media.Status != "uploaded" {
			return false
		}
	}
	return true
}
