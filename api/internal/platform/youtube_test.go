package platform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestYouTubeOAuthScopesIncludeAnalyticsReadonly(t *testing.T) {
	adapter := NewYouTubeAdapter()
	config := adapter.DefaultOAuthConfig("https://api.example.com")
	for _, want := range []string{
		"https://www.googleapis.com/auth/youtube.upload",
		"https://www.googleapis.com/auth/youtube.readonly",
		"https://www.googleapis.com/auth/yt-analytics.readonly",
	} {
		if !containsString(config.Scopes, want) {
			t.Fatalf("DefaultOAuthConfig scopes missing %q: %#v", want, config.Scopes)
		}
	}

	authURL := adapter.GetAuthURL(config, "state-123")
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	scope := parsed.Query().Get("scope")
	if !strings.Contains(scope, "https://www.googleapis.com/auth/yt-analytics.readonly") {
		t.Fatalf("auth URL scope = %q, want yt-analytics.readonly", scope)
	}
}

func TestYouTubeExchangeCodeStoresGrantedAnalyticsScope(t *testing.T) {
	transport := &youtubeExchangeTransport{}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}
	config := OAuthConfig{
		ClientID:     "yt-client",
		ClientSecret: "yt-secret",
		TokenURL:     "https://oauth2.googleapis.com/token",
		RedirectURL:  "https://api.example.com/v1/oauth/callback/youtube",
	}

	result, err := adapter.ExchangeCode(context.Background(), config, "auth-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}

	if !containsString(result.Scopes, "https://www.googleapis.com/auth/yt-analytics.readonly") {
		t.Fatalf("scopes = %#v, want yt-analytics.readonly", result.Scopes)
	}
	if result.ExternalAccountID != "UC123" {
		t.Fatalf("ExternalAccountID = %q, want UC123", result.ExternalAccountID)
	}
}

func TestYouTubeUploadQuotaBreakerSkipsDownloadAfterProjectQuota(t *testing.T) {
	now := time.Date(2026, 5, 30, 15, 30, 0, 0, time.UTC)
	breaker := newYouTubeUploadQuotaBreaker(func() time.Time { return now })
	transport := &youtubeQuotaTransport{
		initStatus: http.StatusTooManyRequests,
		initBody:   `{"error":{"status":"RESOURCE_EXHAUSTED","errors":[{"reason":"rateLimitExceeded"}],"message":"The request cannot be completed because you have exceeded your quota. Video Uploads per day"}}`,
	}
	adapter := &YouTubeAdapter{
		client:       &http.Client{Transport: transport},
		quotaBreaker: breaker,
	}

	_, firstErr := adapter.Post(context.Background(), "yt-token", "caption", []MediaItem{{URL: "https://video.example/clip.mp4", Kind: MediaKindVideo}}, nil)
	if firstErr == nil {
		t.Fatal("expected first quota error")
	}
	if !strings.Contains(firstErr.Error(), "quota_scope=platform_project") {
		t.Fatalf("first error = %q, want quota_scope metadata", firstErr.Error())
	}
	carrier, ok := firstErr.(interface{ ProviderErrorFields() map[string]any })
	if !ok {
		t.Fatalf("first error %T does not carry provider fields", firstErr)
	}
	fields := carrier.ProviderErrorFields()
	if fields["provider"] != "youtube" || fields["http_status"] != http.StatusTooManyRequests || fields["reason"] != "rateLimitExceeded" || fields["quota_limit"] != "defaultVideoInsertPerDayPerProject" || fields["quota_location"] != "platform_project" {
		t.Fatalf("provider fields = %#v", fields)
	}
	if transport.videoDownloads != 1 || transport.uploadInits != 1 {
		t.Fatalf("first attempt downloads/inits = %d/%d, want 1/1", transport.videoDownloads, transport.uploadInits)
	}
	if _, ok := breaker.OpenUntil(); !ok {
		t.Fatal("breaker should be open after project upload quota")
	}

	_, secondErr := adapter.Post(context.Background(), "yt-token", "caption", []MediaItem{{URL: "https://video.example/clip.mp4", Kind: MediaKindVideo}}, nil)
	if secondErr == nil {
		t.Fatal("expected fast-fail quota error")
	}
	if !strings.Contains(secondErr.Error(), "youtube upload quota temporarily exhausted") {
		t.Fatalf("second error = %q, want fast-fail quota message", secondErr.Error())
	}
	if transport.videoDownloads != 1 || transport.uploadInits != 1 {
		t.Fatalf("second attempt should skip HTTP work; downloads/inits = %d/%d, want 1/1", transport.videoDownloads, transport.uploadInits)
	}
}

func TestYouTubeUploadQuotaBreakerIgnoresNonUploadQuota429(t *testing.T) {
	now := time.Date(2026, 5, 30, 15, 30, 0, 0, time.UTC)
	breaker := newYouTubeUploadQuotaBreaker(func() time.Time { return now })
	transport := &youtubeQuotaTransport{
		initStatus: http.StatusTooManyRequests,
		initBody:   `{"error":{"status":"RESOURCE_EXHAUSTED","errors":[{"reason":"userRateLimitExceeded"}],"message":"Please slow down"}}`,
	}
	adapter := &YouTubeAdapter{
		client:       &http.Client{Transport: transport},
		quotaBreaker: breaker,
	}

	_, err := adapter.Post(context.Background(), "yt-token", "caption", []MediaItem{{URL: "https://video.example/clip.mp4", Kind: MediaKindVideo}}, nil)
	if err == nil {
		t.Fatal("expected upload init error")
	}
	if _, ok := breaker.OpenUntil(); ok {
		t.Fatal("breaker should stay closed for non-upload quota 429")
	}
}

func TestNextYouTubeUploadQuotaResetUsesPacificMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 30, 23, 30, 0, 0, loc)

	got := nextYouTubeUploadQuotaReset(now).In(loc)
	want := time.Date(2026, 5, 31, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("reset = %s, want %s", got, want)
	}
}

func TestYouTubeGetAccountMetricsMapsChannelStatistics(t *testing.T) {
	transport := &youtubeMetricsTransport{
		status: http.StatusOK,
		body: `{
			"items": [{
				"statistics": {
					"viewCount": "9876543",
					"subscriberCount": "123000",
					"hiddenSubscriberCount": false,
					"videoCount": "42"
				}
			}]
		}`,
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	metrics, err := adapter.GetAccountMetrics(context.Background(), "yt-token", "UC123")
	if err != nil {
		t.Fatalf("GetAccountMetrics: %v", err)
	}

	if transport.path != "/youtube/v3/channels" {
		t.Fatalf("path = %q, want /youtube/v3/channels", transport.path)
	}
	if got := transport.query.Get("part"); got != "statistics" {
		t.Fatalf("part = %q, want statistics", got)
	}
	if got := transport.query.Get("id"); got != "UC123" {
		t.Fatalf("id = %q, want UC123", got)
	}
	if transport.auth != "Bearer yt-token" {
		t.Fatalf("Authorization = %q, want bearer token", transport.auth)
	}
	if metrics.FollowerCount != 123000 || metrics.FollowingCount != 0 || metrics.PostCount != 42 {
		t.Fatalf("metrics counts = %#v", metrics)
	}
	assertPlatformSpecific(t, metrics.PlatformSpecific, "view_count", int64(9876543))
	assertPlatformSpecific(t, metrics.PlatformSpecific, "hidden_subscriber_count", false)
	assertPlatformSpecific(t, metrics.PlatformSpecific, "following_count_supported", false)
	assertPlatformSpecific(t, metrics.PlatformSpecific, "subscriber_count_rounded", true)
	assertPlatformSpecific(t, metrics.PlatformSpecific, "post_count_public_only", true)
}

func TestYouTubeGetAccountMetricsHandlesHiddenSubscriberCount(t *testing.T) {
	transport := &youtubeMetricsTransport{
		status: http.StatusOK,
		body: `{
			"items": [{
				"statistics": {
					"viewCount": "44",
					"hiddenSubscriberCount": true,
					"videoCount": "3"
				}
			}]
		}`,
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	metrics, err := adapter.GetAccountMetrics(context.Background(), "yt-token", "UC123")
	if err != nil {
		t.Fatalf("GetAccountMetrics: %v", err)
	}

	if metrics.FollowerCount != 0 || metrics.PostCount != 3 {
		t.Fatalf("metrics counts = %#v", metrics)
	}
	assertPlatformSpecific(t, metrics.PlatformSpecific, "hidden_subscriber_count", true)
}

func TestYouTubeGetAccountMetricsEmptyChannelNeedsReconnect(t *testing.T) {
	transport := &youtubeMetricsTransport{
		status: http.StatusOK,
		body:   `{"items":[]}`,
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	_, err := adapter.GetAccountMetrics(context.Background(), "yt-token", "UC123")
	if !errors.Is(err, ErrYouTubeNoChannel) {
		t.Fatalf("GetAccountMetrics error = %v, want ErrYouTubeNoChannel", err)
	}
}

func TestYouTubeGetAccountMetricsAuthFailureNeedsReconnect(t *testing.T) {
	transport := &youtubeMetricsTransport{
		status: http.StatusUnauthorized,
		body:   `{"error":{"code":401,"message":"Invalid Credentials"}}`,
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	_, err := adapter.GetAccountMetrics(context.Background(), "yt-token", "UC123")
	if !errors.Is(err, ErrNeedsReconnect) {
		t.Fatalf("GetAccountMetrics error = %v, want ErrNeedsReconnect", err)
	}
}

func TestYouTubeGetAccountMetricsUpstreamFailure(t *testing.T) {
	transport := &youtubeMetricsTransport{
		status: http.StatusTooManyRequests,
		body:   `{"error":{"code":429,"message":"quota exceeded"}}`,
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	_, err := adapter.GetAccountMetrics(context.Background(), "yt-token", "UC123")
	if err == nil {
		t.Fatal("expected upstream error")
	}
	if errors.Is(err, ErrNeedsReconnect) {
		t.Fatalf("GetAccountMetrics error = %v, did not want ErrNeedsReconnect", err)
	}
	if !strings.Contains(err.Error(), "youtube channel metrics failed (429)") {
		t.Fatalf("GetAccountMetrics error = %q, want status detail", err.Error())
	}
}

func TestYouTubeAnalyticsSummaryQueriesChannelReport(t *testing.T) {
	transport := &youtubeAnalyticsTransport{
		responses: []youtubeAnalyticsResponse{{
			status: http.StatusOK,
			body: `{
				"columnHeaders": [
					{"name":"views"},
					{"name":"likes"},
					{"name":"comments"},
					{"name":"shares"},
					{"name":"estimatedMinutesWatched"},
					{"name":"averageViewDuration"},
					{"name":"averageViewPercentage"},
					{"name":"subscribersGained"},
					{"name":"subscribersLost"}
				],
				"rows": [[1200, 88, 17, 9, 5400, 84, 62.5, 31, 4]]
			}`,
		}},
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	summary, err := adapter.GetYouTubeAnalyticsSummary(context.Background(), "yt-token", "UC123", start, end)
	if err != nil {
		t.Fatalf("GetYouTubeAnalyticsSummary: %v", err)
	}

	req := transport.requests[0]
	if req.path != "/v2/reports" {
		t.Fatalf("path = %q, want /v2/reports", req.path)
	}
	if req.auth != "Bearer yt-token" {
		t.Fatalf("Authorization = %q, want bearer token", req.auth)
	}
	if got := req.query.Get("ids"); got != "channel==UC123" {
		t.Fatalf("ids = %q, want channel==UC123", got)
	}
	if got := req.query.Get("startDate"); got != "2026-07-01" {
		t.Fatalf("startDate = %q, want 2026-07-01", got)
	}
	if got := req.query.Get("endDate"); got != "2026-07-28" {
		t.Fatalf("endDate = %q, want 2026-07-28", got)
	}
	if got := req.query.Get("metrics"); !strings.Contains(got, "subscribersGained") || !strings.Contains(got, "averageViewPercentage") {
		t.Fatalf("metrics = %q, want v2 non-monetary metrics", got)
	}
	if summary.Metrics.Views != 1200 || summary.Metrics.Likes != 88 || summary.Metrics.Comments != 17 || summary.Metrics.Shares != 9 {
		t.Fatalf("engagement metrics = %#v", summary.Metrics)
	}
	if summary.Metrics.EstimatedMinutesWatched != 5400 || summary.Metrics.AverageViewDuration != 84 || summary.Metrics.AverageViewPercentage != 62.5 {
		t.Fatalf("watch metrics = %#v", summary.Metrics)
	}
	if summary.Metrics.SubscribersGained != 31 || summary.Metrics.SubscribersLost != 4 {
		t.Fatalf("subscriber metrics = %#v", summary.Metrics)
	}
}

func TestYouTubeAnalyticsTrendQueriesDailyReport(t *testing.T) {
	transport := &youtubeAnalyticsTransport{
		responses: []youtubeAnalyticsResponse{{
			status: http.StatusOK,
			body: `{
				"columnHeaders": [{"name":"day"},{"name":"views"},{"name":"likes"}],
				"rows": [["2026-07-01", 10, 1], ["2026-07-02", 20, 2]]
			}`,
		}},
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	rows, err := adapter.GetYouTubeAnalyticsTrend(
		context.Background(),
		"yt-token",
		"UC123",
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("GetYouTubeAnalyticsTrend: %v", err)
	}

	req := transport.requests[0]
	if got := req.query.Get("dimensions"); got != "day" {
		t.Fatalf("dimensions = %q, want day", got)
	}
	if got := req.query.Get("sort"); got != "day" {
		t.Fatalf("sort = %q, want day", got)
	}
	if len(rows) != 2 || rows[0].Date != "2026-07-01" || rows[1].Metrics.Views != 20 || rows[1].Metrics.Likes != 2 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestYouTubeAnalyticsVideosQueriesTopVideosReport(t *testing.T) {
	transport := &youtubeAnalyticsTransport{
		responses: []youtubeAnalyticsResponse{{
			status: http.StatusOK,
			body: `{
				"columnHeaders": [{"name":"video"},{"name":"views"},{"name":"estimatedMinutesWatched"}],
				"rows": [["abc123", 300, 1200]]
			}`,
		}},
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	rows, err := adapter.GetYouTubeAnalyticsVideos(
		context.Background(),
		"yt-token",
		"UC123",
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		25,
	)
	if err != nil {
		t.Fatalf("GetYouTubeAnalyticsVideos: %v", err)
	}

	req := transport.requests[0]
	if got := req.query.Get("dimensions"); got != "video" {
		t.Fatalf("dimensions = %q, want video", got)
	}
	if got := req.query.Get("sort"); got != "-views" {
		t.Fatalf("sort = %q, want -views", got)
	}
	if got := req.query.Get("maxResults"); got != "25" {
		t.Fatalf("maxResults = %q, want 25", got)
	}
	if len(rows) != 1 || rows[0].VideoID != "abc123" || rows[0].Metrics.Views != 300 || rows[0].Metrics.EstimatedMinutesWatched != 1200 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestYouTubeAnalyticsAuthFailureNeedsReconnect(t *testing.T) {
	transport := &youtubeAnalyticsTransport{
		responses: []youtubeAnalyticsResponse{{
			status: http.StatusForbidden,
			body:   `{"error":{"code":403,"message":"Request had insufficient authentication scopes."}}`,
		}},
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	_, err := adapter.GetYouTubeAnalyticsSummary(
		context.Background(),
		"yt-token",
		"UC123",
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
	)
	if !errors.Is(err, ErrNeedsReconnect) {
		t.Fatalf("GetYouTubeAnalyticsSummary error = %v, want ErrNeedsReconnect", err)
	}
}

func TestYouTubeAnalyticsQuotaFailureIsUpstream(t *testing.T) {
	transport := &youtubeAnalyticsTransport{
		responses: []youtubeAnalyticsResponse{{
			status: http.StatusTooManyRequests,
			body:   `{"error":{"code":429,"message":"quota exceeded"}}`,
		}},
	}
	adapter := &YouTubeAdapter{client: &http.Client{Transport: transport}}

	_, err := adapter.GetYouTubeAnalyticsSummary(
		context.Background(),
		"yt-token",
		"UC123",
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
	)
	if err == nil {
		t.Fatal("expected upstream error")
	}
	if errors.Is(err, ErrNeedsReconnect) {
		t.Fatalf("GetYouTubeAnalyticsSummary error = %v, did not want ErrNeedsReconnect", err)
	}
	if !strings.Contains(err.Error(), "youtube analytics report failed (429)") {
		t.Fatalf("GetYouTubeAnalyticsSummary error = %q, want status detail", err.Error())
	}
}

func assertPlatformSpecific(t *testing.T, got map[string]any, key string, want any) {
	t.Helper()
	if got[key] != want {
		t.Fatalf("platform_specific[%q] = %#v, want %#v", key, got[key], want)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type youtubeQuotaTransport struct {
	videoDownloads int
	uploadInits    int
	initStatus     int
	initBody       string
}

func (t *youtubeQuotaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodGet && req.URL.Host == "video.example":
		t.videoDownloads++
		return youtubeHTTPResponse(http.StatusOK, "video-bytes", req), nil
	case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/upload/youtube/"):
		t.uploadInits++
		return youtubeHTTPResponse(t.initStatus, t.initBody, req), nil
	default:
		return youtubeHTTPResponse(http.StatusNotFound, `{}`, req), nil
	}
}

type youtubeMetricsTransport struct {
	status int
	body   string
	auth   string
	path   string
	query  url.Values
}

func (t *youtubeMetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.auth = req.Header.Get("Authorization")
	t.path = req.URL.Path
	t.query = req.URL.Query()
	return youtubeHTTPResponse(t.status, t.body, req), nil
}

type youtubeExchangeTransport struct {
	tokenRequests   int
	channelRequests int
}

func (t *youtubeExchangeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodPost && req.URL.Host == "oauth2.googleapis.com":
		t.tokenRequests++
		return youtubeHTTPResponse(http.StatusOK, `{
			"access_token": "yt-access",
			"refresh_token": "yt-refresh",
			"expires_in": 3600,
			"scope": "https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly https://www.googleapis.com/auth/yt-analytics.readonly"
		}`, req), nil
	case req.Method == http.MethodGet && req.URL.Host == "www.googleapis.com" && req.URL.Path == "/youtube/v3/channels":
		t.channelRequests++
		return youtubeHTTPResponse(http.StatusOK, `{
			"items": [{
				"id": "UC123",
				"snippet": {
					"title": "Studio Channel",
					"thumbnails": {"default": {"url": "https://example.com/avatar.jpg"}}
				}
			}]
		}`, req), nil
	default:
		return youtubeHTTPResponse(http.StatusNotFound, `{}`, req), nil
	}
}

type youtubeAnalyticsResponse struct {
	status int
	body   string
}

type youtubeAnalyticsRequest struct {
	auth  string
	path  string
	query url.Values
}

type youtubeAnalyticsTransport struct {
	responses []youtubeAnalyticsResponse
	requests  []youtubeAnalyticsRequest
}

func (t *youtubeAnalyticsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, youtubeAnalyticsRequest{
		auth:  req.Header.Get("Authorization"),
		path:  req.URL.Path,
		query: req.URL.Query(),
	})
	idx := len(t.requests) - 1
	if idx >= len(t.responses) {
		return youtubeHTTPResponse(http.StatusNotFound, `{}`, req), nil
	}
	resp := t.responses[idx]
	return youtubeHTTPResponse(resp.status, resp.body, req), nil
}

func youtubeHTTPResponse(status int, body string, req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}
