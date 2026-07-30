package xaccountreads

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type Store interface {
	Lookup(context.Context, string, string, time.Time) (Receipt, error)
	ResetFailedForRetry(context.Context, string, time.Time) (bool, error)
	AdmissionMutation(NewReceipt) xcredits.ExposureMutation
	MarkFailed(context.Context, string, string, time.Time, time.Time) error
	MarkOutcomeUnknown(context.Context, string, time.Time) error
	MarkSettlementPending(context.Context, string, []byte, time.Time) error
	SuccessMutation(string, []byte, time.Time) xcredits.ExposureSettlementMutation
	SettlementPendingMutation(string, []byte, time.Time) xcredits.ExposureSettlementMutation
}

type CreditsService interface {
	ReserveExposureWithMutation(context.Context, xcredits.ExposureReservationRequest, xcredits.ExposureMutation) (xcredits.ExposureReservation, error)
	MarkExposureReadStarted(context.Context, string) error
	FinalizeExposure(context.Context, string, int64) error
	FinalizeExposureWithMutation(context.Context, string, int64, xcredits.ExposureSettlementMutation) error
	MarkExposureFinalizePendingWithMutation(context.Context, string, int64, string, xcredits.ExposureSettlementMutation) error
	ReleaseExposure(context.Context, string) error
	MarkExposureFinalizePending(context.Context, string, int64, string) error
	MarkExposureReleasePending(context.Context, string, string) error
}

type Provider interface {
	ReadAccountProfile(context.Context, string, string) (platform.TwitterAccountProfile, error)
	ReadAuthoredPosts(context.Context, string, string, platform.TwitterAuthoredPostsRequest) (platform.TwitterAuthoredPostsPage, error)
}

type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type TokenResolver interface {
	ResolveAccountReadToken(context.Context, string) (string, error)
}

type RecoveryStore interface {
	ClaimRecoverable(context.Context, int, string, time.Time) ([]Receipt, error)
	DeleteExpiredReceipts(context.Context, int, time.Time) (int, error)
}

type Service struct {
	store    Store
	credits  CreditsService
	provider Provider
	cipher   Cipher
	hashKey  []byte
	cursors  *CursorCodec
	tokens   TokenResolver
}

func NewService(store Store, credits CreditsService, provider Provider, cipher Cipher, secret []byte, previousSecrets ...[]byte) *Service {
	return NewServiceWithSecrets(store, credits, provider, cipher, secret, secret, previousSecrets...)
}

func NewServiceWithSecrets(
	store Store,
	credits CreditsService,
	provider Provider,
	cipher Cipher,
	idempotencySecret []byte,
	cursorSecret []byte,
	previousCursorSecrets ...[]byte,
) *Service {
	cursors, _ := NewCursorCodec(cursorSecret, previousCursorSecrets...)
	key := sha256.Sum256(append([]byte("unipost:x-account-read:idempotency:"), idempotencySecret...))
	return &Service{store: store, credits: credits, provider: provider, cipher: cipher, hashKey: key[:], cursors: cursors}
}

func (s *Service) SetTokenResolver(resolver TokenResolver) *Service {
	s.tokens = resolver
	return s
}

type profileRetryInput struct {
	ExternalAccountID string `json:"external_account_id"`
}

type postsRetryInput struct {
	ExternalAccountID      string `json:"external_account_id"`
	PaginationToken        string `json:"pagination_token,omitempty"`
	StartTime              string `json:"start_time,omitempty"`
	EndTime                string `json:"end_time,omitempty"`
	Limit                  int    `json:"limit"`
	ExcludeReposts         bool   `json:"exclude_reposts"`
	ExcludeRepliesToOthers bool   `json:"exclude_replies_to_others"`
}

type admissionInput struct {
	WorkspaceID        string
	AccountID          string
	ExternalUserID     string
	ExternalAccountID  string
	AppMode            string
	IdempotencyKey     string
	Endpoint           string
	OperationKey       string
	Purpose            string
	RequestedResources int
	UnitsPerResource   int64
	FingerprintInput   any
	RetryInput         any
	Now                time.Time
}

type admission struct {
	receipt     Receipt
	reservation xcredits.ExposureReservation
	replay      bool
}

func (s *Service) begin(ctx context.Context, input admissionInput) (admission, error) {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	keyHash := s.keyHash(input.IdempotencyKey)
	fingerprint, err := requestFingerprint(input.FingerprintInput)
	if err != nil {
		return admission{}, err
	}
	if existing, err := s.store.Lookup(ctx, input.WorkspaceID, keyHash, input.Now); err == nil {
		if existing.RequestFingerprint != fingerprint {
			return existingAdmission(existing, fingerprint, input.Now)
		}
		if existing.Status == ReceiptFailed && retryableFailure(existing.FailureClass) && !existing.NextAttemptAt.After(input.Now) {
			reset, resetErr := s.store.ResetFailedForRetry(ctx, existing.OperationID, input.Now)
			if resetErr != nil {
				return admission{}, resetErr
			}
			if !reset {
				return existingAdmission(existing, fingerprint, input.Now)
			}
		} else {
			return existingAdmission(existing, fingerprint, input.Now)
		}
	} else if !errors.Is(err, ErrReceiptNotFound) {
		return admission{}, err
	}
	retryJSON, err := json.Marshal(input.RetryInput)
	if err != nil {
		return admission{}, err
	}
	encryptedRequest, err := s.cipher.Encrypt(string(retryJSON))
	if err != nil {
		return admission{}, err
	}
	operationID := "xro_" + uuid.NewString()
	receipt := NewReceipt{Receipt: Receipt{
		OperationID: operationID, WorkspaceID: input.WorkspaceID, AccountID: input.AccountID,
		ExternalUserID: input.ExternalUserID, Endpoint: input.Endpoint,
		IdempotencyKeyHash: keyHash, RequestFingerprint: fingerprint,
		EncryptedRequest: encryptedRequest, RequestedResources: input.RequestedResources,
		OperationKey: input.OperationKey, Status: ReceiptExecuting, NextAttemptAt: input.Now,
		ExpiresAt: input.Now.Add(24 * time.Hour),
	}}
	reservation, err := s.credits.ReserveExposureWithMutation(ctx, xcredits.ExposureReservationRequest{
		WorkspaceID: input.WorkspaceID, SocialAccountID: input.AccountID,
		ExternalUserID: input.ExternalUserID, AppMode: input.AppMode,
		OperationKey: input.OperationKey, Purpose: input.Purpose,
		IdempotencyKey: operationID, RequestedResources: input.RequestedResources,
		MinimumResources: input.RequestedResources, UnitsPerResource: input.UnitsPerResource,
		Now: input.Now,
	}, s.store.AdmissionMutation(receipt))
	if err != nil {
		if existing, lookupErr := s.store.Lookup(ctx, input.WorkspaceID, keyHash, input.Now); lookupErr == nil {
			return existingAdmission(existing, fingerprint, input.Now)
		}
		if errors.Is(err, xcredits.ErrMonthlyLimitExceeded) || errors.Is(err, xcredits.ErrAllowanceNotConfigured) {
			estimated := int64(input.RequestedResources) * input.UnitsPerResource
			available := int64(0)
			if snapshots, ok := s.credits.(interface {
				Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error)
			}); ok {
				if snapshot, snapshotErr := snapshots.Snapshot(ctx, input.WorkspaceID, input.Now); snapshotErr == nil && snapshot.MonthlyRemaining != nil {
					available = *snapshot.MonthlyRemaining
				}
			}
			maxAffordable := int(available / input.UnitsPerResource)
			if maxAffordable > 100 {
				maxAffordable = 100
			}
			if maxAffordable < 5 && input.Endpoint == "posts" {
				maxAffordable = 0
			}
			return admission{}, &OperationError{
				Code: CodeInsufficientCredits, Cause: err, EstimatedCredits: estimated,
				AvailableCredits: available, MaxAffordableLimit: maxAffordable,
			}
		}
		if errors.Is(err, ErrAccountReadRateLimited) {
			slog.Warn("X account read rate limited",
				"workspace_id", input.WorkspaceID,
				"account_id", input.AccountID,
				"endpoint", input.Endpoint,
				"source", "unipost_admission",
			)
			return admission{}, &OperationError{Code: CodeRateLimited, RetryAfter: time.Minute, Cause: err}
		}
		return admission{}, err
	}
	receipt.Receipt.ExposureID = reservation.ID
	return admission{receipt: receipt.Receipt, reservation: reservation}, nil
}

func retryableFailure(code string) bool {
	return code == CodeRateLimited || code == CodeXUpstream
}

func existingAdmission(receipt Receipt, fingerprint string, now time.Time) (admission, error) {
	if receipt.RequestFingerprint != fingerprint {
		slog.Warn("X account read idempotency conflict",
			"operation_id", receipt.OperationID,
			"workspace_id", receipt.WorkspaceID,
			"account_id", receipt.AccountID,
			"endpoint", receipt.Endpoint,
		)
		return admission{}, &OperationError{Code: CodeIdempotencyConflict}
	}
	switch receipt.Status {
	case ReceiptSucceeded:
		return admission{receipt: receipt, replay: true}, nil
	case ReceiptExecuting:
		return admission{}, &OperationError{Code: CodeReadInProgress, RetryAfter: 2 * time.Second}
	case ReceiptOutcomeUnknown, ReceiptSettlementPending:
		return admission{}, &OperationError{Code: CodeReadSettlementPending, RetryAfter: 5 * time.Second}
	case ReceiptFailed:
		code := receipt.FailureClass
		if code == "" {
			code = CodeXUpstream
		}
		retryAfter := time.Duration(0)
		if receipt.NextAttemptAt.After(now) {
			retryAfter = receipt.NextAttemptAt.Sub(now)
		}
		return admission{}, &OperationError{Code: code, RetryAfter: retryAfter}
	default:
		return admission{}, errors.New("invalid X account-read receipt state")
	}
}

func (s *Service) ReadProfile(ctx context.Context, req ProfileRequest) (ProfileResult, error) {
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	fingerprintInput := struct {
		WorkspaceID, AccountID, ExternalUserID, Route string
	}{req.WorkspaceID, req.AccountID, req.ExternalUserID, "/profile"}
	admitted, err := s.begin(ctx, admissionInput{
		WorkspaceID: req.WorkspaceID, AccountID: req.AccountID, ExternalUserID: req.ExternalUserID,
		ExternalAccountID: req.ExternalAccountID, AppMode: req.AppMode, IdempotencyKey: req.IdempotencyKey,
		Endpoint: "profile", OperationKey: "user.read", Purpose: xcredits.ExposurePurposeAccountProfile,
		RequestedResources: 1, UnitsPerResource: xcredits.OperationWeight("user.read"),
		FingerprintInput: fingerprintInput, RetryInput: profileRetryInput{ExternalAccountID: req.ExternalAccountID}, Now: req.Now,
	})
	if err != nil {
		return ProfileResult{}, err
	}
	if admitted.replay {
		var result ProfileResult
		if err := json.Unmarshal(admitted.receipt.ResponseJSON, &result); err != nil {
			return ProfileResult{}, err
		}
		result.Meta.Replayed = true
		logReadCompleted(admitted, 1, 1, result.Meta.Credits, true)
		return result, nil
	}
	if err := s.credits.MarkExposureReadStarted(ctx, admitted.reservation.ID); err != nil {
		return ProfileResult{}, err
	}
	accessToken, err := s.resolveAccessToken(ctx, admitted, req.AccessToken, req.Now)
	if err != nil {
		return ProfileResult{}, err
	}
	profile, err := s.provider.ReadAccountProfile(ctx, accessToken, req.ExternalAccountID)
	if err != nil {
		return ProfileResult{}, s.failRead(ctx, admitted, err, req.Now)
	}
	result := ProfileResult{
		Data: ProfileData{AccountID: req.AccountID, Platform: "twitter", TwitterAccountProfile: profile, RetrievedAt: req.Now},
		Meta: ProfileMeta{Credits: creditsReceipt(admitted, "user.read", req.AppMode, xcredits.OperationWeight("user.read"))},
	}
	if err := s.complete(ctx, admitted, xcredits.OperationWeight("user.read"), &result.Meta.Credits, result, req.Now); err != nil {
		return ProfileResult{}, err
	}
	logReadCompleted(admitted, 1, 1, result.Meta.Credits, false)
	return result, nil
}

func (s *Service) ReadPosts(ctx context.Context, req PostsRequest) (PostsResult, error) {
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	scope := CursorScope{
		WorkspaceID: req.WorkspaceID, AccountID: req.AccountID, ExternalUserID: req.ExternalUserID,
		StartTime: formatOptionalTime(req.StartTime), EndTime: formatOptionalTime(req.EndTime),
		ExcludeReposts: req.ExcludeReposts, ExcludeRepliesToOthers: req.ExcludeRepliesToOthers,
	}
	upstreamCursor := ""
	cursorExpiresAt := time.Time{}
	if req.Cursor != "" {
		var err error
		upstreamCursor, cursorExpiresAt, err = s.cursors.Decode(req.Cursor, scope, req.Now)
		if err != nil {
			return PostsResult{}, err
		}
	}
	fingerprintInput := struct {
		WorkspaceID, AccountID, ExternalUserID, Route, Cursor, StartTime, EndTime string
		Limit                                                                     int
		ExcludeReposts, ExcludeRepliesToOthers                                    bool
	}{
		req.WorkspaceID, req.AccountID, req.ExternalUserID, "/posts", upstreamCursor,
		scope.StartTime, scope.EndTime, req.Limit, req.ExcludeReposts, req.ExcludeRepliesToOthers,
	}
	admitted, err := s.begin(ctx, admissionInput{
		WorkspaceID: req.WorkspaceID, AccountID: req.AccountID, ExternalUserID: req.ExternalUserID,
		ExternalAccountID: req.ExternalAccountID, AppMode: req.AppMode, IdempotencyKey: req.IdempotencyKey,
		Endpoint: "posts", OperationKey: "post.read", Purpose: xcredits.ExposurePurposeAccountPostHistory,
		RequestedResources: req.Limit, UnitsPerResource: xcredits.OperationWeight("post.read"),
		FingerprintInput: fingerprintInput,
		RetryInput: postsRetryInput{
			ExternalAccountID: req.ExternalAccountID, PaginationToken: upstreamCursor,
			StartTime: scope.StartTime, EndTime: scope.EndTime, Limit: req.Limit,
			ExcludeReposts: req.ExcludeReposts, ExcludeRepliesToOthers: req.ExcludeRepliesToOthers,
		},
		Now: req.Now,
	})
	if err != nil {
		return PostsResult{}, s.withRetryCursor(err, req.Cursor, scope, cursorExpiresAt, req.Now)
	}
	if admitted.replay {
		var result PostsResult
		if err := json.Unmarshal(admitted.receipt.ResponseJSON, &result); err != nil {
			return PostsResult{}, err
		}
		result.Meta.Replayed = true
		logReadCompleted(admitted, result.Meta.ScannedCount, result.Meta.ReturnedCount, result.Meta.Credits, true)
		return result, nil
	}
	if err := s.credits.MarkExposureReadStarted(ctx, admitted.reservation.ID); err != nil {
		return PostsResult{}, err
	}
	accessToken, err := s.resolveAccessToken(ctx, admitted, req.AccessToken, req.Now)
	if err != nil {
		return PostsResult{}, err
	}
	page, err := s.provider.ReadAuthoredPosts(ctx, accessToken, req.ExternalAccountID, platform.TwitterAuthoredPostsRequest{
		Limit: req.Limit, PaginationToken: upstreamCursor, StartTime: req.StartTime, EndTime: req.EndTime,
		ExcludeReposts: req.ExcludeReposts, ExcludeRepliesToOthers: req.ExcludeRepliesToOthers,
	})
	if err != nil {
		return PostsResult{}, s.withRetryCursor(s.failRead(ctx, admitted, err, req.Now), req.Cursor, scope, cursorExpiresAt, req.Now)
	}
	data := make([]PostData, 0, len(page.Posts))
	for _, post := range page.Posts {
		data = append(data, PostData{TwitterAuthoredPost: post, AccountID: req.AccountID, Thread: PostThread{ThreadID: post.ConversationID}})
	}
	result := PostsResult{Data: data, Meta: PostsMeta{
		Limit: req.Limit, ScannedCount: page.ScannedCount, ReturnedCount: len(data), HasMore: page.NextToken != "",
		Credits: creditsReceipt(admitted, "post.read", req.AppMode, int64(page.ScannedCount)*xcredits.OperationWeight("post.read")),
	}}
	if page.NextToken != "" {
		next, expiresAt, err := s.cursors.Encode(scope, page.NextToken, req.Now)
		if err != nil {
			return PostsResult{}, err
		}
		result.Meta.NextCursor = next
		result.Meta.CursorExpiresAt = &expiresAt
	}
	actualUnits := int64(page.ScannedCount) * xcredits.OperationWeight("post.read")
	if err := s.complete(ctx, admitted, actualUnits, &result.Meta.Credits, result, req.Now); err != nil {
		return PostsResult{}, err
	}
	logReadCompleted(admitted, result.Meta.ScannedCount, result.Meta.ReturnedCount, result.Meta.Credits, false)
	return result, nil
}

func (s *Service) resolveAccessToken(ctx context.Context, admitted admission, provided string, now time.Time) (string, error) {
	if provided != "" {
		return provided, nil
	}
	if s.tokens == nil {
		return "", s.failBeforeProvider(ctx, admitted, errors.New("X account-read token resolver is not configured"), now)
	}
	token, err := s.tokens.ResolveAccountReadToken(ctx, admitted.receipt.AccountID)
	if err != nil || token == "" {
		if err == nil {
			err = errors.New("X account-read token resolver returned an empty token")
		}
		return "", s.failBeforeProvider(ctx, admitted, err, now)
	}
	return token, nil
}

func (s *Service) failBeforeProvider(ctx context.Context, admitted admission, cause error, now time.Time) error {
	if err := s.credits.ReleaseExposure(ctx, admitted.reservation.ID); err != nil {
		_ = s.credits.MarkExposureReleasePending(ctx, admitted.reservation.ID, "X account-read pre-provider release failed")
		_ = s.store.MarkOutcomeUnknown(ctx, admitted.receipt.OperationID, now.Add(time.Minute))
		return &OperationError{Code: CodeReadSettlementPending, RetryAfter: time.Minute, Cause: err}
	}
	_ = s.store.MarkFailed(ctx, admitted.receipt.OperationID, CodeReauthorization, now, now)
	return &OperationError{Code: CodeReauthorization, Cause: cause}
}

func (s *Service) withRetryCursor(err error, cursor string, scope CursorScope, expiresAt time.Time, now time.Time) error {
	var operationErr *OperationError
	if cursor == "" || !errors.As(err, &operationErr) || operationErr.RetryAfter <= 0 ||
		expiresAt.After(now.Add(operationErr.RetryAfter)) {
		return err
	}
	retryCursor, retryExpiresAt, refreshErr := s.cursors.Refresh(cursor, scope, now)
	if refreshErr == nil {
		operationErr.RetryCursor = retryCursor
		operationErr.RetryCursorExpiresAt = retryExpiresAt
	}
	return err
}

func (s *Service) complete(ctx context.Context, admitted admission, actualUnits int64, credits *CreditsReceipt, response any, now time.Time) error {
	if admitted.reservation.AccountingEnabled {
		credits.Charged = actualUnits
		credits.Released = credits.Reserved - actualUnits
	}
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if err := s.credits.FinalizeExposureWithMutation(
		ctx,
		admitted.reservation.ID,
		actualUnits,
		s.store.SuccessMutation(admitted.receipt.OperationID, body, now),
	); err != nil {
		pendingErr := s.credits.MarkExposureFinalizePendingWithMutation(
			ctx,
			admitted.reservation.ID,
			actualUnits,
			"X account-read finalization failed",
			s.store.SettlementPendingMutation(admitted.receipt.OperationID, body, now.Add(time.Minute)),
		)
		if pendingErr != nil {
			return &OperationError{Code: CodeReadSettlementPending, RetryAfter: time.Minute, Cause: errors.Join(err, pendingErr)}
		}
		return &OperationError{Code: CodeReadSettlementPending, RetryAfter: time.Minute, Cause: err}
	}
	return nil
}

func (s *Service) failRead(ctx context.Context, admitted admission, readErr error, now time.Time) error {
	var providerErr *platform.TwitterAccountReadError
	if errors.As(readErr, &providerErr) && providerErr.OutcomeUnknown {
		_ = s.store.MarkOutcomeUnknown(ctx, admitted.receipt.OperationID, now.Add(5*time.Second))
		return &OperationError{Code: CodeReadSettlementPending, RetryAfter: 5 * time.Second, Cause: readErr}
	}
	if err := s.credits.ReleaseExposure(ctx, admitted.reservation.ID); err != nil {
		_ = s.credits.MarkExposureReleasePending(ctx, admitted.reservation.ID, "X account-read release failed")
		_ = s.store.MarkOutcomeUnknown(ctx, admitted.receipt.OperationID, now.Add(time.Minute))
		return &OperationError{Code: CodeReadSettlementPending, RetryAfter: time.Minute, Cause: err}
	}
	code := CodeXUpstream
	if errors.As(readErr, &providerErr) {
		switch providerErr.StatusCode {
		case http.StatusTooManyRequests:
			code = CodeRateLimited
		case http.StatusUnauthorized, http.StatusForbidden:
			code = CodeReauthorization
		}
	}
	retryAfter := time.Duration(0)
	if providerErr != nil {
		retryAfter = providerErr.RetryAfter
	}
	_ = s.store.MarkFailed(ctx, admitted.receipt.OperationID, code, now, now.Add(retryAfter))
	if code == CodeRateLimited {
		slog.Warn("X account read rate limited",
			"operation_id", admitted.receipt.OperationID,
			"workspace_id", admitted.receipt.WorkspaceID,
			"account_id", admitted.receipt.AccountID,
			"endpoint", admitted.receipt.Endpoint,
			"source", "x_provider",
			"retry_after_seconds", int64(retryAfter.Seconds()),
		)
	}
	return &OperationError{Code: code, RetryAfter: retryAfter, Cause: readErr}
}

func logReadCompleted(admitted admission, scanned, returned int, credits CreditsReceipt, replayed bool) {
	slog.Info("X account read completed",
		"operation_id", admitted.receipt.OperationID,
		"workspace_id", admitted.receipt.WorkspaceID,
		"account_id", admitted.receipt.AccountID,
		"endpoint", admitted.receipt.Endpoint,
		"scanned_count", scanned,
		"returned_count", returned,
		"estimated_credits", credits.Estimated,
		"charged_credits", credits.Charged,
		"released_credits", credits.Released,
		"accounting_enabled", credits.AccountingEnabled,
		"replayed", replayed,
	)
}

func creditsReceipt(admitted admission, operation, appMode string, charged int64) CreditsReceipt {
	accounting := admitted.reservation.AccountingEnabled
	reserved := admitted.reservation.ReservedUnits
	estimated := int64(admitted.receipt.RequestedResources) * xcredits.OperationWeight(operation)
	status := "finalized"
	if !accounting {
		estimated, reserved, charged = 0, 0, 0
		status = "bypassed"
	}
	return CreditsReceipt{
		OperationID: admitted.receipt.OperationID, Status: status,
		AccountingEnabled: accounting, BillingMode: appMode, BypassReason: admitted.reservation.BypassReason,
		Operation: operation, Estimated: estimated, Reserved: reserved, Charged: charged,
		CatalogVersion: admitted.reservation.CatalogVersion,
	}
}

func (s *Service) keyHash(value string) string {
	hash := hmac.New(sha256.New, s.hashKey)
	_, _ = hash.Write([]byte(value))
	return hex.EncodeToString(hash.Sum(nil))
}

func requestFingerprint(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
