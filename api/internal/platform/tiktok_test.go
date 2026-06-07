package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuildTikTokPostInfoIncludesRequiredToggles(t *testing.T) {
	info := buildTikTokPostInfo("hello", "PUBLIC_TO_EVERYONE", nil, "video")

	if got := info["privacy_level"]; got != "PUBLIC_TO_EVERYONE" {
		t.Fatalf("privacy_level = %v, want PUBLIC_TO_EVERYONE", got)
	}
	if got := info["disable_comment"]; got != false {
		t.Fatalf("disable_comment = %v, want false", got)
	}
	if got := info["auto_add_music"]; got != true {
		t.Fatalf("auto_add_music = %v, want true", got)
	}
	if got := info["brand_content_toggle"]; got != false {
		t.Fatalf("brand_content_toggle = %v, want false", got)
	}
	if got := info["brand_organic_toggle"]; got != false {
		t.Fatalf("brand_organic_toggle = %v, want false", got)
	}
	if got := info["disable_duet"]; got != false {
		t.Fatalf("disable_duet = %v, want false", got)
	}
	if got := info["disable_stitch"]; got != false {
		t.Fatalf("disable_stitch = %v, want false", got)
	}
}

func TestTikTokOAuthScopesDefaultToApprovedProductionSet(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "production")
	unsetenv(t, "TIKTOK_ANALYTICS_SCOPES_ENABLED")

	adapter := NewTikTokAdapter()
	config := adapter.DefaultOAuthConfig("https://api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "video.publish,video.upload,user.info.basic"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func TestTikTokOAuthScopesIncludeAnalyticsWhenEnabled(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "production")
	t.Setenv("TIKTOK_ANALYTICS_SCOPES_ENABLED", "true")

	adapter := NewTikTokAdapter()
	config := adapter.DefaultOAuthConfig("https://api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "video.publish,video.upload,user.info.basic,user.info.profile,user.info.stats,video.list"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func TestTikTokOAuthScopesIncludeAnalyticsByDefaultOutsideProduction(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")
	unsetenv(t, "TIKTOK_ANALYTICS_SCOPES_ENABLED")

	adapter := NewTikTokAdapter()
	config := adapter.DefaultOAuthConfig("https://dev-api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "video.publish,video.upload,user.info.basic,user.info.profile,user.info.stats,video.list"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func unsetenv(t *testing.T, name string) {
	t.Helper()
	old, ok := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(name, old)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}

func TestTikTokBasicUserInfoFieldsStayWithinBasicScope(t *testing.T) {
	disallowed := map[string]bool{
		"username":         true,
		"profile_web_link": true,
		"is_verified":      true,
		"follower_count":   true,
		"following_count":  true,
		"likes_count":      true,
		"video_count":      true,
	}
	for _, field := range tiktokBasicUserInfoFields {
		if disallowed[field] {
			t.Fatalf("basic user-info fields include %q, which requires non-basic TikTok scopes", field)
		}
	}
}

func TestBuildTikTokPostInfoPhotoOmitsDuetStitch(t *testing.T) {
	info := buildTikTokPostInfo("hello", "PUBLIC_TO_EVERYONE", nil, "photo")

	if _, ok := info["disable_duet"]; ok {
		t.Fatal("photo post_info must not include disable_duet (TikTok rejects it)")
	}
	if _, ok := info["disable_stitch"]; ok {
		t.Fatal("photo post_info must not include disable_stitch (TikTok rejects it)")
	}
}

func TestShouldRetryTikTokWithSelfOnly(t *testing.T) {
	body := []byte(`{"error":{"code":"invalid_params","message":"Invalid authorization header. Please check the format."}}`)

	if !shouldRetryTikTokWithSelfOnly(400, body, "PUBLIC_TO_EVERYONE") {
		t.Fatal("expected retry for invalid_params with non-SELF_ONLY privacy")
	}
	if shouldRetryTikTokWithSelfOnly(400, body, "SELF_ONLY") {
		t.Fatal("did not expect retry when already using SELF_ONLY")
	}
	if shouldRetryTikTokWithSelfOnly(500, body, "PUBLIC_TO_EVERYONE") {
		t.Fatal("did not expect retry for non-400 responses")
	}
}

func TestWrapTikTokInitErrorIncludesSandboxGuidance(t *testing.T) {
	body := []byte(`{"error":{"code":"invalid_params","message":"Invalid authorization header. Please check the format."}}`)

	err := wrapTikTokInitError("tiktok photo init failed", http.StatusBadRequest, body, "PUBLIC_TO_EVERYONE")
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	got := err.Error()
	for _, want := range []string{
		"tiktok photo init failed (400)",
		"malformed request bodies",
		"sandbox/unaudited mode",
		"SELF_ONLY",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want to contain %q", got, want)
		}
	}
}

func TestTikTokPhotoInitRetriesInvalidParamsWithSelfOnly(t *testing.T) {
	withTikTokPublishPollConfig(t, 1, 0)
	transport := &tiktokPhotoInitRetryTransport{}
	adapter := NewTikTokAdapter()
	adapter.client = &http.Client{Transport: transport}
	adapter.mediaProxy = fakeTikTokMediaProxy{}

	result, err := adapter.postPhoto(
		context.Background(),
		"tiktok-token",
		"caption",
		[]MediaItem{{URL: "https://source.example/photo.jpg", Kind: MediaKindImage}},
		map[string]any{"privacy_level": "PUBLIC_TO_EVERYONE"},
	)
	if err != nil {
		t.Fatalf("postPhoto: %v", err)
	}
	if result == nil || result.ExternalID != "publish_123" {
		t.Fatalf("result = %#v, want publish_123", result)
	}
	want := []string{"PUBLIC_TO_EVERYONE", "SELF_ONLY"}
	if len(transport.privacyLevels) != len(want) {
		t.Fatalf("privacy retries = %v, want %v", transport.privacyLevels, want)
	}
	for i := range want {
		if transport.privacyLevels[i] != want[i] {
			t.Fatalf("privacy retry %d = %q, want %q", i, transport.privacyLevels[i], want[i])
		}
	}
}

func TestTikTokFileUploadChunksVideosLargerThan64MB(t *testing.T) {
	withTikTokPublishPollConfig(t, 1, 0)
	const videoSize = 67_800_000

	transport := &tiktokChunkedUploadTransport{videoSize: videoSize}
	adapter := NewTikTokAdapter()
	adapter.client = &http.Client{Transport: transport}

	result, err := adapter.Post(
		context.Background(),
		"tiktok-token",
		"caption",
		[]MediaItem{{URL: "https://video.example/large.mp4", Kind: MediaKindVideo}},
		map[string]any{"privacy_level": "SELF_ONLY", "upload_mode": "file_upload"},
	)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if result == nil || result.ExternalID != "publish_123" {
		t.Fatalf("result = %#v, want publish_123", result)
	}

	if transport.initSource.VideoSize != videoSize {
		t.Fatalf("video_size = %d, want %d", transport.initSource.VideoSize, videoSize)
	}
	if transport.initSource.ChunkSize != 30_000_000 {
		t.Fatalf("chunk_size = %d, want 30000000", transport.initSource.ChunkSize)
	}
	if transport.initSource.TotalChunkCount != 2 {
		t.Fatalf("total_chunk_count = %d, want 2", transport.initSource.TotalChunkCount)
	}

	wantRanges := []string{
		fmt.Sprintf("bytes 0-29999999/%d", videoSize),
		fmt.Sprintf("bytes 30000000-%d/%d", videoSize-1, videoSize),
	}
	if len(transport.uploadRanges) != len(wantRanges) {
		t.Fatalf("upload ranges = %v, want %v", transport.uploadRanges, wantRanges)
	}
	for i := range wantRanges {
		if transport.uploadRanges[i] != wantRanges[i] {
			t.Fatalf("upload range %d = %q, want %q", i, transport.uploadRanges[i], wantRanges[i])
		}
	}

	wantLengths := []int64{30_000_000, 37_800_000}
	if len(transport.uploadLengths) != len(wantLengths) {
		t.Fatalf("upload lengths = %v, want %v", transport.uploadLengths, wantLengths)
	}
	for i := range wantLengths {
		if transport.uploadLengths[i] != wantLengths[i] {
			t.Fatalf("upload length %d = %d, want %d", i, transport.uploadLengths[i], wantLengths[i])
		}
	}
}

func withTikTokPublishPollConfig(t *testing.T, attempts int, intervalDuration time.Duration) {
	t.Helper()
	oldAttempts := tiktokPublishPollAttempts
	oldInterval := tiktokPublishPollInterval
	tiktokPublishPollAttempts = attempts
	tiktokPublishPollInterval = intervalDuration
	t.Cleanup(func() {
		tiktokPublishPollAttempts = oldAttempts
		tiktokPublishPollInterval = oldInterval
	})
}

type fakeTikTokMediaProxy struct{}

func (fakeTikTokMediaProxy) UploadFromURL(_ context.Context, rawURL string) (string, error) {
	return "https://media.unipost.test/proxy/" + strings.TrimPrefix(rawURL, "https://source.example/"), nil
}

type tiktokPhotoInitRetryTransport struct {
	privacyLevels []string
}

type tiktokChunkedUploadTransport struct {
	videoSize     int64
	initSource    tiktokChunkedUploadSource
	uploadRanges  []string
	uploadLengths []int64
}

type tiktokChunkedUploadSource struct {
	Source          string `json:"source"`
	VideoSize       int64  `json:"video_size"`
	ChunkSize       int64  `json:"chunk_size"`
	TotalChunkCount int64  `json:"total_chunk_count"`
}

func (t *tiktokPhotoInitRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case strings.Contains(req.URL.Path, "/content/init/"):
		body, _ := io.ReadAll(req.Body)
		var payload struct {
			PostInfo struct {
				PrivacyLevel string `json:"privacy_level"`
			} `json:"post_info"`
		}
		_ = json.Unmarshal(body, &payload)
		t.privacyLevels = append(t.privacyLevels, payload.PostInfo.PrivacyLevel)

		if len(t.privacyLevels) == 1 {
			return tiktokHTTPResponse(http.StatusBadRequest, `{"error":{"code":"invalid_params","message":"Invalid authorization header. Please check the format."}}`, req), nil
		}
		return tiktokHTTPResponse(http.StatusOK, `{"data":{"publish_id":"publish_123"},"error":{"code":"ok"}}`, req), nil
	case strings.Contains(req.URL.Path, "/status/fetch/"):
		return tiktokHTTPResponse(http.StatusOK, `{"data":{"status":"PUBLISH_COMPLETE","publicaly_available_post_id":["7350123456789012345"]},"error":{"code":"ok"}}`, req), nil
	default:
		return tiktokHTTPResponse(http.StatusNotFound, `{}`, req), nil
	}
}

func (t *tiktokChunkedUploadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodGet && req.URL.Host == "video.example":
		return tiktokHTTPResponseWithBody(http.StatusOK, io.LimitReader(zeroReader{}, t.videoSize), req), nil
	case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/video/init/"):
		body, _ := io.ReadAll(req.Body)
		var payload struct {
			SourceInfo tiktokChunkedUploadSource `json:"source_info"`
		}
		_ = json.Unmarshal(body, &payload)
		t.initSource = payload.SourceInfo
		return tiktokHTTPResponse(http.StatusOK, `{"data":{"publish_id":"publish_123","upload_url":"https://upload.example/video"},"error":{"code":"ok"}}`, req), nil
	case req.Method == http.MethodPut && req.URL.Host == "upload.example":
		body, _ := io.ReadAll(req.Body)
		t.uploadRanges = append(t.uploadRanges, req.Header.Get("Content-Range"))
		t.uploadLengths = append(t.uploadLengths, int64(len(body)))
		if strings.Contains(req.Header.Get("Content-Range"), fmt.Sprintf("-%d/", t.videoSize-1)) {
			return tiktokHTTPResponse(http.StatusCreated, `{}`, req), nil
		}
		return tiktokHTTPResponse(http.StatusPartialContent, `{}`, req), nil
	case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/status/fetch/"):
		return tiktokHTTPResponse(http.StatusOK, `{"data":{"status":"PUBLISH_COMPLETE","publicaly_available_post_id":["7350123456789012345"]},"error":{"code":"ok"}}`, req), nil
	default:
		return tiktokHTTPResponse(http.StatusNotFound, `{}`, req), nil
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func tiktokHTTPResponse(status int, body string, req *http.Request) *http.Response {
	return tiktokHTTPResponseWithBody(status, strings.NewReader(body), req)
}

func tiktokHTTPResponseWithBody(status int, body io.Reader, req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(body),
		Header:     make(http.Header),
		Request:    req,
	}
}

func TestTikTokPublicPostURLFromStatusData(t *testing.T) {
	data := map[string]any{
		"status":                      "PUBLISH_COMPLETE",
		"publicaly_available_post_id": []any{"7350123456789012345"},
	}
	got := TikTokPublicPostURLFromStatusData(data)
	want := "https://www.tiktok.com/player/v1/7350123456789012345"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestTikTokPublicPostURLFromStatusDataMissingID(t *testing.T) {
	data := map[string]any{
		"status":                      "PUBLISH_COMPLETE",
		"publicaly_available_post_id": []any{},
	}
	if got := TikTokPublicPostURLFromStatusData(data); got != "" {
		t.Fatalf("url = %q, want empty", got)
	}
}

func TestTikTokProfileURL(t *testing.T) {
	got := TikTokProfileURL("@magicxiaobo")
	want := "https://www.tiktok.com/@magicxiaobo"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}
