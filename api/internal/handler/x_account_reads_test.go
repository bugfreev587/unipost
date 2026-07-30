package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xaccountreads"
)

type fakeXAccountReadService struct {
	profile        xaccountreads.ProfileResult
	posts          xaccountreads.PostsResult
	err            error
	calls          int
	profileRequest xaccountreads.ProfileRequest
	postsRequest   xaccountreads.PostsRequest
}

func (f *fakeXAccountReadService) ReadProfile(_ context.Context, req xaccountreads.ProfileRequest) (xaccountreads.ProfileResult, error) {
	f.calls++
	f.profileRequest = req
	return f.profile, f.err
}
func (f *fakeXAccountReadService) ReadPosts(_ context.Context, req xaccountreads.PostsRequest) (xaccountreads.PostsResult, error) {
	f.calls++
	f.postsRequest = req
	return f.posts, f.err
}

type fakeXAccountReadCipher struct{}

func (fakeXAccountReadCipher) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (fakeXAccountReadCipher) Decrypt(value string) (string, error) {
	if value == "encrypted-access" {
		return "access-token", nil
	}
	return "", errors.New("decrypt failed")
}

func invokeXAccountRead(t *testing.T, store *xCapabilityTestDB, service *fakeXAccountReadService, method, target string, handler func(*XAccountReadsHandler, http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Idempotency-Key", "request-key")
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sa_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h := NewXAccountReadsHandler(db.New(store), fakeXAccountReadCipher{}, service, nil)
	handler(h, rec, req)
	return rec
}

func TestXAccountProfileReadReturnsDocumentedEnvelope(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	service := &fakeXAccountReadService{profile: xaccountreads.ProfileResult{
		Data: xaccountreads.ProfileData{AccountID: "sa_1", Platform: "twitter", TwitterAccountProfile: platform.TwitterAccountProfile{ExternalAccountID: "x-user-1", Username: "v", DisplayName: "V", AccountCreatedAt: now}, RetrievedAt: now},
		Meta: xaccountreads.ProfileMeta{Credits: xaccountreads.CreditsReceipt{OperationID: "xro_1", Status: "finalized", Charged: 10}},
	}}
	store := &xCapabilityTestDB{planID: "basic", appMode: "unipost_managed_app", externalUser: "managed_1", refreshToken: "refresh", scopes: []string{"users.read", "tweet.read", "offline.access"}}
	rec := invokeXAccountRead(t, store, service, http.MethodGet, "/v1/accounts/sa_1/profile?external_user_id=managed_1", (*XAccountReadsHandler).Profile)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data xaccountreads.ProfileData `json:"data"`
		Meta xaccountreads.ProfileMeta `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Username != "v" || body.Meta.Credits.Charged != 10 || service.calls != 1 {
		t.Fatalf("body=%+v calls=%d", body, service.calls)
	}
	if service.profileRequest.AccessToken != "" {
		t.Fatalf("handler resolved access token before service admission")
	}
}

func TestXAccountPostsRejectsManagedUserMismatchBeforeService(t *testing.T) {
	service := &fakeXAccountReadService{}
	store := &xCapabilityTestDB{planID: "basic", appMode: "unipost_managed_app", externalUser: "managed_1", refreshToken: "refresh", scopes: []string{"users.read", "tweet.read", "offline.access"}}
	rec := invokeXAccountRead(t, store, service, http.MethodGet, "/v1/accounts/sa_1/posts?external_user_id=managed_2&limit=5", (*XAccountReadsHandler).Posts)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "ACCOUNT_ACCESS_DENIED") || service.calls != 0 {
		t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), service.calls)
	}
}

func TestXAccountPostsRequiresSelectorLimitAndIdempotency(t *testing.T) {
	service := &fakeXAccountReadService{}
	store := &xCapabilityTestDB{planID: "basic", appMode: "unipost_managed_app", externalUser: "managed_1", refreshToken: "refresh", scopes: []string{"users.read", "tweet.read", "offline.access"}}
	rec := invokeXAccountRead(t, store, service, http.MethodGet, "/v1/accounts/sa_1/posts?external_user_id=managed_1&limit=101", (*XAccountReadsHandler).Posts)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") || service.calls != 0 {
		t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), service.calls)
	}
}

func TestXAccountReadMapsInsufficientCreditsTo402(t *testing.T) {
	service := &fakeXAccountReadService{err: &xaccountreads.OperationError{Code: xaccountreads.CodeInsufficientCredits, EstimatedCredits: 500, AvailableCredits: 240, MaxAffordableLimit: 48}}
	store := &xCapabilityTestDB{planID: "basic", appMode: "unipost_managed_app", externalUser: "managed_1", refreshToken: "refresh", scopes: []string{"users.read", "tweet.read", "offline.access"}}
	rec := invokeXAccountRead(t, store, service, http.MethodGet, "/v1/accounts/sa_1/posts?external_user_id=managed_1&limit=100", (*XAccountReadsHandler).Posts)
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), `"max_affordable_limit":48`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestXAccountReadReturnsRefreshedRetryCursorForLongRateLimit(t *testing.T) {
	expiresAt := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	service := &fakeXAccountReadService{err: &xaccountreads.OperationError{
		Code: xaccountreads.CodeRateLimited, RetryAfter: 2 * time.Hour,
		RetryCursor: "refreshed-cursor", RetryCursorExpiresAt: expiresAt,
	}}
	store := &xCapabilityTestDB{planID: "basic", appMode: "unipost_managed_app", externalUser: "managed_1", refreshToken: "refresh", scopes: []string{"users.read", "tweet.read", "offline.access"}}
	rec := invokeXAccountRead(t, store, service, http.MethodGet, "/v1/accounts/sa_1/posts?external_user_id=managed_1&limit=5&cursor=old", (*XAccountReadsHandler).Posts)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "7200" ||
		!strings.Contains(rec.Body.String(), `"retry_cursor":"refreshed-cursor"`) ||
		!strings.Contains(rec.Body.String(), `"retry_cursor_expires_at":"2026-08-05T12:00:00Z"`) {
		t.Fatalf("status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
}
