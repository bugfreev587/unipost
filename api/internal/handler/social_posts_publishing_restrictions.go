package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type publishingRestrictionEvaluator interface {
	Evaluate(context.Context, string, string) (publishingrestrictions.Decision, error)
}

func (h *SocialPostHandler) SetPublishingRestrictions(evaluator publishingRestrictionEvaluator) *SocialPostHandler {
	if h != nil {
		h.publishingRestrictions = evaluator
	}
	return h
}

func (h *SocialPostHandler) evaluatePublishingRestrictions(
	ctx context.Context,
	workspaceID string,
	posts []platform.PlatformPostInput,
	accountMap map[string]platform.ValidateAccount,
) (map[string]publishingrestrictions.Decision, error) {
	blocked := make(map[string]publishingrestrictions.Decision)
	if h == nil || h.publishingRestrictions == nil {
		return blocked, nil
	}
	for _, post := range posts {
		platformName := strings.ToLower(strings.TrimSpace(accountMap[post.AccountID].Platform))
		decision, err := h.publishingRestrictions.Evaluate(ctx, workspaceID, platformName)
		if err != nil {
			return nil, err
		}
		if decision.Restricted {
			blocked[post.AccountID] = decision
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

func writePublishingRestrictionError(w http.ResponseWriter, decision publishingrestrictions.Decision) {
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

func publishingRestrictionFailure(postID, resultID, workspaceID, accountID, platformName string) db.CreatePostFailureParams {
	failure := postfailures.BuildParams(
		postID, resultID, workspaceID, accountID, platformName,
		publishingrestrictions.FailureStage, publishingrestrictions.UserMessage, publishingrestrictions.UserMessage,
	)
	failure.ErrorCode = publishingrestrictions.NormalizedCode
	failure.FailureStage = publishingrestrictions.FailureStage
	failure.IsRetriable = false
	failure.PlatformErrorCode = pgtype.Text{}
	failure.ErrorSource = postfailures.ToText("unipost")
	failure.ErrorTemporality = postfailures.ToText("temporary")
	failure.ProviderError = []byte(`{}`)
	return failure
}
