package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestTikTokAnalyticsErrorResponses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantReason string
	}{
		{
			name:       "invalid token",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokAccountTokenInvalid, "video.query", http.StatusUnauthorized, "access_token_invalid", errors.New("expired")),
			wantStatus: http.StatusConflict,
			wantCode:   "NEEDS_RECONNECT",
			wantReason: "account_token_invalid",
		},
		{
			name:       "missing scope",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokAnalyticsScopeRequired, "video.query", http.StatusForbidden, "scope_not_authorized", errors.New("missing")),
			wantStatus: http.StatusConflict,
			wantCode:   "NEEDS_RECONNECT",
			wantReason: "analytics_scope_required",
		},
		{
			name:       "rate limited",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokProviderRateLimited, "video.query", http.StatusTooManyRequests, "", errors.New("limited")),
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "UPSTREAM_RATE_LIMITED",
			wantReason: "provider_rate_limited",
		},
		{
			name:       "temporary",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokProviderTemporaryError, "video.query", http.StatusBadGateway, "", errors.New("down")),
			wantStatus: http.StatusBadGateway,
			wantCode:   "TIKTOK_TEMPORARY_ERROR",
			wantReason: "provider_temporary_error",
		},
		{
			name:       "not ready",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokVideoNotReady, "publish.status", http.StatusOK, "", errors.New("processing")),
			wantStatus: http.StatusBadGateway,
			wantCode:   "TIKTOK_ANALYTICS_UNAVAILABLE",
			wantReason: "video_not_ready",
		},
		{
			name:       "not found",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokVideoNotFound, "video.query", http.StatusOK, "", errors.New("missing")),
			wantStatus: http.StatusBadGateway,
			wantCode:   "TIKTOK_ANALYTICS_UNAVAILABLE",
			wantReason: "video_not_found",
		},
		{
			name:       "legacy fallback",
			err:        errors.New("legacy TikTok failure"),
			wantStatus: http.StatusBadGateway,
			wantCode:   "TIKTOK_ERROR",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			writeTikTokAnalyticsError(recorder, tc.err)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			var got ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Error.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", got.Error.Code, tc.wantCode)
			}
			if tc.wantReason == "" {
				if got.Error.Details != nil {
					t.Fatalf("details = %#v, want nil", got.Error.Details)
				}
				return
			}
			if got.Error.Details["reason"] != tc.wantReason {
				t.Fatalf("details = %#v, want reason %q", got.Error.Details, tc.wantReason)
			}
		})
	}
}

func TestTikTokAnalyticsAccountStateResponses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		account    db.SocialAccount
		wantOK     bool
		wantStatus int
		wantCode   string
		wantReason string
	}{
		{name: "active", account: db.SocialAccount{Status: "active"}, wantOK: false},
		{name: "disconnected status", account: db.SocialAccount{Status: " disconnected "}, wantOK: true, wantStatus: http.StatusConflict, wantCode: "ACCOUNT_DISCONNECTED", wantReason: "account_disconnected"},
		{name: "disconnected timestamp", account: db.SocialAccount{Status: "active", DisconnectedAt: pgtype.Timestamptz{Valid: true}}, wantOK: true, wantStatus: http.StatusConflict, wantCode: "ACCOUNT_DISCONNECTED", wantReason: "account_disconnected"},
		{name: "reconnect required", account: db.SocialAccount{Status: "RECONNECT_REQUIRED"}, wantOK: true, wantStatus: http.StatusConflict, wantCode: "NEEDS_RECONNECT", wantReason: "account_token_invalid"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, code, _, reason, ok := tiktokAnalyticsAccountStateError(&tc.account)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if status != tc.wantStatus || code != tc.wantCode || string(reason) != tc.wantReason {
				t.Fatalf("response = (%d, %q, %q), want (%d, %q, %q)", status, code, reason, tc.wantStatus, tc.wantCode, tc.wantReason)
			}
		})
	}
}
