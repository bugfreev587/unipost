package worker

import (
	"errors"
	"net/http"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestAnalyticsRefreshFailurePolicy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		platform   string
		err        error
		wantReason string
		wantMark   bool
	}{
		{
			name:       "Pinterest auth still marks reconnect",
			platform:   "pinterest",
			err:        errors.New("pinterest analytics (401): unauthorized"),
			wantReason: "Pinterest rejected the analytics token. Reconnect Pinterest to refresh analytics access; contact support if this continues after reconnecting.",
			wantMark:   true,
		},
		{
			name:       "TikTok scope does not mark account",
			platform:   "tiktok",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokAnalyticsScopeRequired, "video.query", http.StatusForbidden, "scope_not_authorized", errors.New("denied")),
			wantReason: "TikTok analytics unavailable: analytics_scope_required (video.query)",
		},
		{
			name:       "TikTok rate limit does not mark account",
			platform:   "tiktok",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokProviderRateLimited, "video.query", http.StatusTooManyRequests, "", errors.New("limited")),
			wantReason: "TikTok analytics unavailable: provider_rate_limited (video.query)",
		},
		{
			name:       "TikTok temporary error does not mark account",
			platform:   "tiktok",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokProviderTemporaryError, "user.info", http.StatusBadGateway, "", errors.New("down")),
			wantReason: "TikTok analytics unavailable: provider_temporary_error (user.info)",
		},
		{
			name:       "TikTok missing video does not mark account",
			platform:   "tiktok",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokVideoNotFound, "video.query", http.StatusOK, "", errors.New("missing")),
			wantReason: "TikTok analytics unavailable: video_not_found (video.query)",
		},
		{
			name:       "TikTok pending video does not mark account",
			platform:   "tiktok",
			err:        platform.NewTikTokAnalyticsError(platform.TikTokVideoNotReady, "publish.status", http.StatusOK, "", errors.New("processing")),
			wantReason: "TikTok analytics unavailable: video_not_ready (publish.status)",
		},
		{
			name:       "unclassified provider keeps error",
			platform:   "youtube",
			err:        errors.New("provider unavailable"),
			wantReason: "provider unavailable",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reason, mark := analyticsRefreshFailurePolicy(tc.platform, tc.err)
			if reason != tc.wantReason || mark != tc.wantMark {
				t.Fatalf("reason=%q mark=%v, want reason=%q mark=%v", reason, mark, tc.wantReason, tc.wantMark)
			}
		})
	}
}
