package xaccountreads

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type fakeReceiptStore struct {
	receipt Receipt
}

func (s *fakeReceiptStore) ClaimRecoverable(context.Context, int, string, time.Time) ([]Receipt, error) {
	if s.receipt.Status == ReceiptOutcomeUnknown || s.receipt.Status == ReceiptSettlementPending {
		return []Receipt{s.receipt}, nil
	}
	return nil, nil
}

func (s *fakeReceiptStore) Lookup(_ context.Context, workspaceID, keyHash string, now time.Time) (Receipt, error) {
	if s.receipt.OperationID == "" || s.receipt.WorkspaceID != workspaceID || s.receipt.IdempotencyKeyHash != keyHash ||
		!s.receipt.ExpiresAt.After(now) {
		return Receipt{}, ErrReceiptNotFound
	}
	return s.receipt, nil
}

func (s *fakeReceiptStore) AdmissionMutation(input NewReceipt) xcredits.ExposureMutation {
	return func(_ context.Context, _ pgx.Tx, exposure xcredits.ExposureReservation) error {
		s.receipt = input.WithExposure(exposure.ID)
		return nil
	}
}

func (s *fakeReceiptStore) MarkSucceeded(_ context.Context, operationID string, response []byte, now time.Time) error {
	s.receipt.Status = ReceiptSucceeded
	s.receipt.ResponseJSON = append([]byte(nil), response...)
	s.receipt.CompletedAt = &now
	return nil
}

func (s *fakeReceiptStore) MarkFailed(_ context.Context, operationID, class string, now time.Time) error {
	s.receipt.Status = ReceiptFailed
	s.receipt.FailureClass = class
	s.receipt.CompletedAt = &now
	return nil
}

func (s *fakeReceiptStore) MarkOutcomeUnknown(_ context.Context, operationID string, next time.Time) error {
	s.receipt.Status = ReceiptOutcomeUnknown
	s.receipt.NextAttemptAt = next
	return nil
}

func (s *fakeReceiptStore) MarkSettlementPending(_ context.Context, operationID string, response []byte, next time.Time) error {
	s.receipt.Status = ReceiptSettlementPending
	s.receipt.ResponseJSON = append([]byte(nil), response...)
	s.receipt.NextAttemptAt = next
	return nil
}

type fakeCredits struct {
	reservation xcredits.ExposureReservation
	reserve     xcredits.ExposureReservationRequest
	reserveErr  error
	started     int
	finalized   int64
	released    int
}

func (f *fakeCredits) ReserveExposureWithMutation(ctx context.Context, req xcredits.ExposureReservationRequest, mutation xcredits.ExposureMutation) (xcredits.ExposureReservation, error) {
	f.reserve = req
	if f.reserveErr != nil {
		return xcredits.ExposureReservation{}, f.reserveErr
	}
	reservation := f.reservation
	if reservation.ID == "" {
		reservation = xcredits.ExposureReservation{ID: "exposure-1", ReservedUnits: int64(req.RequestedResources) * req.UnitsPerResource, AccountingEnabled: true, CatalogVersion: xcredits.CatalogVersion}
	}
	if err := mutation(ctx, nil, reservation); err != nil {
		return xcredits.ExposureReservation{}, err
	}
	return reservation, nil
}

func (f *fakeCredits) MarkExposureReadStarted(context.Context, string) error { f.started++; return nil }
func (f *fakeCredits) FinalizeExposure(_ context.Context, _ string, units int64) error {
	f.finalized = units
	return nil
}
func (f *fakeCredits) ReleaseExposure(context.Context, string) error { f.released++; return nil }
func (f *fakeCredits) MarkExposureFinalizePending(context.Context, string, int64, string) error {
	return nil
}
func (f *fakeCredits) MarkExposureReleasePending(context.Context, string, string) error { return nil }

type fakeProvider struct {
	profile platform.TwitterAccountProfile
	posts   platform.TwitterAuthoredPostsPage
	err     error
	calls   int
}

func (f *fakeProvider) ReadAccountProfile(context.Context, string, string) (platform.TwitterAccountProfile, error) {
	f.calls++
	return f.profile, f.err
}
func (f *fakeProvider) ReadAuthoredPosts(context.Context, string, string, platform.TwitterAuthoredPostsRequest) (platform.TwitterAuthoredPostsPage, error) {
	f.calls++
	return f.posts, f.err
}

type passthroughCipher struct{}

func (passthroughCipher) Encrypt(value string) (string, error) { return "encrypted:" + value, nil }
func (passthroughCipher) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "encrypted:"), nil
}

type fakeTokenResolver struct{ token string }

func (f fakeTokenResolver) ResolveAccountReadToken(context.Context, string) (string, error) {
	return f.token, nil
}

func newTestService(store *fakeReceiptStore, credits *fakeCredits, provider *fakeProvider) *Service {
	return NewService(store, credits, provider, passthroughCipher{}, []byte("0123456789abcdef0123456789abcdef"))
}

func TestReadPostsSettlesScannedResourcesAndReplaysWithoutSecondXCall(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store := &fakeReceiptStore{}
	credits := &fakeCredits{}
	provider := &fakeProvider{posts: platform.TwitterAuthoredPostsPage{
		ScannedCount: 5, RawCount: 5,
		Posts: []platform.TwitterAuthoredPost{{ExternalPostID: "post-1", Text: "hello", ConversationID: "post-1", ContentType: "original_post", CreatedAt: now}},
	}}
	service := newTestService(store, credits, provider)
	req := PostsRequest{
		WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1", ExternalAccountID: "x-user",
		AppMode: "unipost_managed_app", AccessToken: "token", IdempotencyKey: "page-1", Limit: 5, Now: now,
	}

	first, err := service.ReadPosts(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ReadPosts(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || credits.finalized != 25 || first.Meta.ScannedCount != 5 ||
		first.Meta.Credits.Charged != 25 || !second.Meta.Replayed {
		t.Fatalf("calls=%d finalized=%d first=%+v second=%+v", provider.calls, credits.finalized, first, second)
	}
}

func TestReadProfileIdempotencyConflictDoesNotCallXAgain(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store := &fakeReceiptStore{}
	credits := &fakeCredits{}
	provider := &fakeProvider{profile: platform.TwitterAccountProfile{
		ExternalAccountID: "x-user", Username: "v", DisplayName: "V", AccountCreatedAt: now,
	}}
	service := newTestService(store, credits, provider)
	req := ProfileRequest{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1", ExternalAccountID: "x-user", AppMode: "unipost_managed_app", AccessToken: "token", IdempotencyKey: "profile", Now: now}
	if _, err := service.ReadProfile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.ExternalUserID = "managed_2"
	_, err := service.ReadProfile(context.Background(), req)
	var operationErr *OperationError
	if !errors.As(err, &operationErr) || operationErr.Code != CodeIdempotencyConflict || provider.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, provider.calls)
	}
}

func TestAmbiguousReadRemainsReservedAndReturnsSettlementPending(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store := &fakeReceiptStore{}
	credits := &fakeCredits{}
	provider := &fakeProvider{err: &platform.TwitterAccountReadError{Retryable: true, OutcomeUnknown: true}}
	service := newTestService(store, credits, provider)
	req := ProfileRequest{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1", ExternalAccountID: "x-user", AppMode: "unipost_managed_app", AccessToken: "token", IdempotencyKey: "profile", Now: now}

	_, err := service.ReadProfile(context.Background(), req)
	var operationErr *OperationError
	if !errors.As(err, &operationErr) || operationErr.Code != CodeReadSettlementPending ||
		store.receipt.Status != ReceiptOutcomeUnknown || credits.released != 0 {
		t.Fatalf("err=%v receipt=%+v credits=%+v", err, store.receipt, credits)
	}
}

func TestInsufficientCreditsStopsBeforeXCall(t *testing.T) {
	store := &fakeReceiptStore{}
	credits := &fakeCredits{reserveErr: xcredits.ErrMonthlyLimitExceeded}
	provider := &fakeProvider{}
	service := newTestService(store, credits, provider)
	_, err := service.ReadPosts(context.Background(), PostsRequest{
		WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1", ExternalAccountID: "x-user",
		AppMode: "unipost_managed_app", AccessToken: "token", IdempotencyKey: "page", Limit: 100,
	})
	var operationErr *OperationError
	if !errors.As(err, &operationErr) || operationErr.Code != CodeInsufficientCredits || provider.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, provider.calls)
	}
}

func TestRecoveryRetriesAmbiguousReadAndMakesResponseReplayable(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store := &fakeReceiptStore{}
	credits := &fakeCredits{}
	provider := &fakeProvider{err: &platform.TwitterAccountReadError{Retryable: true, OutcomeUnknown: true}}
	service := newTestService(store, credits, provider).SetTokenResolver(fakeTokenResolver{token: "fresh-token"})
	req := ProfileRequest{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1", ExternalAccountID: "x-user", AppMode: "unipost_managed_app", AccessToken: "token", IdempotencyKey: "profile", Now: now}
	if _, err := service.ReadProfile(context.Background(), req); err == nil {
		t.Fatal("expected pending read")
	}
	provider.err = nil
	provider.profile = platform.TwitterAccountProfile{ExternalAccountID: "x-user", Username: "v", DisplayName: "V", AccountCreatedAt: now}

	stats, err := service.Reconcile(context.Background(), 10, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Succeeded != 1 || store.receipt.Status != ReceiptSucceeded || provider.calls != 2 {
		t.Fatalf("stats=%+v receipt=%+v calls=%d", stats, store.receipt, provider.calls)
	}
}

func TestRecoveryReleasesExpiredUnknownOutcomeWithoutBackCharge(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := &fakeReceiptStore{receipt: Receipt{
		OperationID: "xro_1", WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1",
		ExposureID: "exposure-1", Endpoint: "profile", Status: ReceiptOutcomeUnknown,
		ExpiresAt: now.Add(-time.Second),
	}}
	credits := &fakeCredits{}
	service := newTestService(store, credits, &fakeProvider{}).SetTokenResolver(fakeTokenResolver{token: "token"})

	stats, err := service.Reconcile(context.Background(), 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Released != 1 || credits.released != 1 || credits.finalized != 0 || store.receipt.Status != ReceiptFailed {
		t.Fatalf("stats=%+v credits=%+v receipt=%+v", stats, credits, store.receipt)
	}
}
