package xaccountreads

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type ReconcileStats struct {
	Scanned   int
	Succeeded int
	Released  int
	Deferred  int
	Cleaned   int
}

func (s *Service) Reconcile(ctx context.Context, limit int, now time.Time) (ReconcileStats, error) {
	store, ok := s.store.(RecoveryStore)
	if !ok {
		return ReconcileStats{}, errors.New("X account-read recovery store is not configured")
	}
	if s.tokens == nil {
		return ReconcileStats{}, errors.New("X account-read token resolver is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cleaned, err := store.DeleteExpiredReceipts(ctx, limit, now)
	if err != nil {
		return ReconcileStats{}, err
	}
	receipts, err := store.ClaimRecoverable(ctx, limit, "x-read-recovery-"+uuid.NewString(), now)
	if err != nil {
		return ReconcileStats{}, err
	}
	stats := ReconcileStats{Scanned: len(receipts), Cleaned: cleaned}
	for _, receipt := range receipts {
		if !receipt.ExpiresAt.After(now) {
			if err := s.credits.ReleaseExposure(ctx, receipt.ExposureID); err != nil {
				_ = s.credits.MarkExposureReleasePending(ctx, receipt.ExposureID, "expired X account-read release failed")
				stats.Deferred++
				continue
			}
			_ = s.store.MarkFailed(ctx, receipt.OperationID, "reconciliation_expired", now)
			stats.Released++
			continue
		}
		if receipt.Status == ReceiptSettlementPending {
			if err := s.credits.FinalizeExposure(ctx, receipt.ExposureID, receipt.ActualUnits); err != nil {
				_ = s.store.MarkSettlementPending(ctx, receipt.OperationID, receipt.ResponseJSON, nextRecoveryAttempt(receipt.AttemptCount, now))
				stats.Deferred++
				continue
			}
			if err := s.store.MarkSucceeded(ctx, receipt.OperationID, receipt.ResponseJSON, now); err != nil {
				stats.Deferred++
				continue
			}
			stats.Succeeded++
			continue
		}
		if err := s.retryUnknown(ctx, receipt, now); err != nil {
			var operationErr *OperationError
			if errors.As(err, &operationErr) && operationErr.Code != CodeReadSettlementPending {
				stats.Released++
			} else {
				stats.Deferred++
			}
			continue
		}
		stats.Succeeded++
	}
	return stats, nil
}

func (s *Service) retryUnknown(ctx context.Context, receipt Receipt, now time.Time) error {
	plaintext, err := s.cipher.Decrypt(receipt.EncryptedRequest)
	if err != nil {
		_ = s.store.MarkOutcomeUnknown(ctx, receipt.OperationID, nextRecoveryAttempt(receipt.AttemptCount, now))
		return &OperationError{Code: CodeReadSettlementPending, Cause: err}
	}
	accessToken, err := s.tokens.ResolveAccountReadToken(ctx, receipt.AccountID)
	if err != nil {
		_ = s.store.MarkOutcomeUnknown(ctx, receipt.OperationID, nextRecoveryAttempt(receipt.AttemptCount, now))
		return &OperationError{Code: CodeReadSettlementPending, Cause: err}
	}
	reservation := xcredits.ExposureReservation{
		ID: receipt.ExposureID, ReservedUnits: receipt.ReservedUnits,
		AccountingEnabled: receipt.AccountingEnabled, BypassReason: receipt.BypassReason,
		CatalogVersion: receipt.CatalogVersion,
	}
	admitted := admission{receipt: receipt, reservation: reservation}
	switch receipt.Endpoint {
	case "profile":
		var input profileRetryInput
		if err := json.Unmarshal([]byte(plaintext), &input); err != nil {
			return err
		}
		profile, err := s.provider.ReadAccountProfile(ctx, accessToken, input.ExternalAccountID)
		if err != nil {
			return s.retryFailure(ctx, admitted, err, now)
		}
		result := ProfileResult{
			Data: ProfileData{AccountID: receipt.AccountID, Platform: "twitter", TwitterAccountProfile: profile, RetrievedAt: now},
			Meta: ProfileMeta{Credits: recoveryCredits(receipt, "user.read", xcredits.OperationWeight("user.read"))},
		}
		return s.complete(ctx, admitted, xcredits.OperationWeight("user.read"), &result.Meta.Credits, result, now)
	case "posts":
		var input postsRetryInput
		if err := json.Unmarshal([]byte(plaintext), &input); err != nil {
			return err
		}
		start, err := parseOptionalTime(input.StartTime)
		if err != nil {
			return err
		}
		end, err := parseOptionalTime(input.EndTime)
		if err != nil {
			return err
		}
		page, err := s.provider.ReadAuthoredPosts(ctx, accessToken, input.ExternalAccountID, platform.TwitterAuthoredPostsRequest{
			Limit: input.Limit, PaginationToken: input.PaginationToken, StartTime: start, EndTime: end,
			ExcludeReposts: input.ExcludeReposts, ExcludeRepliesToOthers: input.ExcludeRepliesToOthers,
		})
		if err != nil {
			return s.retryFailure(ctx, admitted, err, now)
		}
		data := make([]PostData, 0, len(page.Posts))
		for _, post := range page.Posts {
			data = append(data, PostData{TwitterAuthoredPost: post, AccountID: receipt.AccountID, Thread: PostThread{ThreadID: post.ConversationID}})
		}
		scope := CursorScope{
			WorkspaceID: receipt.WorkspaceID, AccountID: receipt.AccountID, ExternalUserID: receipt.ExternalUserID,
			StartTime: input.StartTime, EndTime: input.EndTime, ExcludeReposts: input.ExcludeReposts,
			ExcludeRepliesToOthers: input.ExcludeRepliesToOthers,
		}
		result := PostsResult{Data: data, Meta: PostsMeta{
			Limit: input.Limit, ScannedCount: page.ScannedCount, ReturnedCount: len(data), HasMore: page.NextToken != "",
			Credits: recoveryCredits(receipt, "post.read", int64(page.ScannedCount)*xcredits.OperationWeight("post.read")),
		}}
		if page.NextToken != "" {
			next, expiresAt, err := s.cursors.Encode(scope, page.NextToken, now)
			if err != nil {
				return err
			}
			result.Meta.NextCursor = next
			result.Meta.CursorExpiresAt = &expiresAt
		}
		actual := int64(page.ScannedCount) * xcredits.OperationWeight("post.read")
		return s.complete(ctx, admitted, actual, &result.Meta.Credits, result, now)
	default:
		return errors.New("unsupported X account-read recovery endpoint")
	}
}

func (s *Service) retryFailure(ctx context.Context, admitted admission, err error, now time.Time) error {
	var providerErr *platform.TwitterAccountReadError
	if errors.As(err, &providerErr) && providerErr.OutcomeUnknown {
		_ = s.store.MarkOutcomeUnknown(ctx, admitted.receipt.OperationID, nextRecoveryAttempt(admitted.receipt.AttemptCount, now))
		return &OperationError{Code: CodeReadSettlementPending, Cause: err}
	}
	return s.failRead(ctx, admitted, err, now)
}

func recoveryCredits(receipt Receipt, operation string, actual int64) CreditsReceipt {
	if !receipt.AccountingEnabled {
		return CreditsReceipt{
			OperationID: receipt.OperationID, Status: "bypassed", AccountingEnabled: false,
			BillingMode: receipt.AppMode, BypassReason: receipt.BypassReason,
			Operation: operation, CatalogVersion: receipt.CatalogVersion,
		}
	}
	return CreditsReceipt{
		OperationID: receipt.OperationID, Status: "finalized", AccountingEnabled: true,
		BillingMode: receipt.AppMode, Operation: operation,
		Estimated: int64(receipt.RequestedResources) * xcredits.OperationWeight(operation),
		Reserved:  receipt.ReservedUnits, Charged: actual, Released: receipt.ReservedUnits - actual,
		CatalogVersion: receipt.CatalogVersion,
	}
}

func nextRecoveryAttempt(attempt int, now time.Time) time.Time {
	delay := 5 * time.Second
	for i := 0; i < attempt && delay < 15*time.Minute; i++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	return now.Add(delay)
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
