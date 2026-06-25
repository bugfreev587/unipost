package platform

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

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

func youtubeHTTPResponse(status int, body string, req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}
