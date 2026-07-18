package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/debugrt"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

type TikTokAdapter struct {
	client     *http.Client
	mediaProxy tiktokMediaProxy // optional; required only for photo posts
}

const tiktokHomepageURL = "https://www.tiktok.com"

var (
	tiktokPublishPollAttempts = 12
	tiktokPublishPollInterval = 5 * time.Second
)

const (
	tiktokMaxRegularChunkSizeBytes = 64_000_000
	tiktokDefaultUploadChunkSize   = 30_000_000
)

type tiktokMediaProxy interface {
	UploadFromURL(context.Context, string) (string, error)
}

var tiktokLegacyScopes = []string{
	"video.publish",
	"video.upload",
	"user.info.basic",
}

var tiktokAnalyticsScopes = []string{
	"user.info.profile",
	"user.info.stats",
	"video.list",
}

var tiktokBasicUserInfoFields = []string{
	"open_id",
	"display_name",
	"avatar_url",
}

var tiktokProfileUserInfoFields = []string{
	"open_id",
	"display_name",
	"avatar_url",
	"username",
	"profile_web_link",
	"profile_deep_link",
	"bio_description",
	"is_verified",
}

var tiktokMetricsUserInfoFields = []string{
	"open_id",
	"display_name",
	"avatar_url",
	"username",
	"profile_web_link",
	"profile_deep_link",
	"bio_description",
	"is_verified",
	"follower_count",
	"following_count",
	"likes_count",
	"video_count",
}

func NewTikTokAdapter() *TikTokAdapter {
	return &TikTokAdapter{client: debugrt.NewClient(120 * time.Second)}
}

// SetMediaProxy attaches an R2-backed media proxy. Photo posts require it
// because TikTok's photo Direct Post only accepts PULL_FROM_URL from
// developer-verified domains — see internal/mediaproxy for the rationale.
// Safe to call with nil to "unset", though that means photo posts will fail.
func (a *TikTokAdapter) SetMediaProxy(c *storage.Client) {
	a.mediaProxy = c
}

func (a *TikTokAdapter) Platform() string { return "tiktok" }

func (a *TikTokAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("TIKTOK_CLIENT_KEY"),
		ClientSecret: os.Getenv("TIKTOK_CLIENT_SECRET"),
		AuthURL:      "https://www.tiktok.com/v2/auth/authorize/",
		TokenURL:     "https://open.tiktokapis.com/v2/oauth/token/",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/tiktok",
		Scopes:       tiktokOAuthScopes(),
	}
}

func tiktokOAuthScopes() []string {
	scopes := append([]string(nil), tiktokLegacyScopes...)
	return append(scopes, tiktokAnalyticsScopes...)
}

func (a *TikTokAdapter) GetAuthURL(config OAuthConfig, state string) string {
	// TikTok uses client_key instead of client_id
	params := url.Values{
		"client_key":    {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"response_type": {"code"},
		"scope":         {strings.Join(config.Scopes, ",")},
		"state":         {state},
	}
	return config.AuthURL + "?" + params.Encode()
}

func (a *TikTokAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	data := url.Values{
		"client_key":    {config.ClientID},
		"client_secret": {config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {config.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	slog.Info("tiktok token response", "status", resp.StatusCode, "body_length", len(respBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(respBody))
	}

	// TikTok may return tokens nested under "data" or at root level
	// Parse as generic map first to handle both formats
	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w, body: %s", err, string(respBody))
	}

	slog.Info("tiktok token response body", "body", string(respBody))

	// Check for error
	if errMsg, ok := raw["error"].(string); ok && errMsg != "" {
		desc, _ := raw["error_description"].(string)
		return nil, fmt.Errorf("tiktok error: %s - %s", errMsg, desc)
	}
	if errObj, ok := raw["error"].(map[string]any); ok {
		if code, _ := errObj["code"].(string); code != "" && code != "ok" {
			msg, _ := errObj["message"].(string)
			return nil, fmt.Errorf("tiktok error: %s", msg)
		}
	}

	// Extract token data — try nested "data" first, then root level
	tokenData := raw
	if data, ok := raw["data"].(map[string]any); ok {
		tokenData = data
	}

	accessToken := ""
	if v, ok := tokenData["access_token"].(string); ok {
		accessToken = v
	}
	refreshToken := ""
	if v, ok := tokenData["refresh_token"].(string); ok {
		refreshToken = v
	}
	openID := ""
	if v, ok := tokenData["open_id"].(string); ok {
		openID = v
	}
	expiresIn := 86400
	if v, ok := tokenData["expires_in"].(float64); ok {
		expiresIn = int(v)
	}

	slog.Info("tiktok token exchange",
		"has_access_token", accessToken != "",
		"token_length", len(accessToken),
		"open_id", openID,
		"expires_in", expiresIn,
	)

	if accessToken == "" {
		return nil, fmt.Errorf("tiktok returned empty access token, response: %s", string(respBody))
	}

	// Get user info
	userInfo, err := a.getBasicUserInfo(ctx, accessToken)
	if err != nil {
		userInfo = &tiktokUserInfo{openID: openID, displayName: openID}
	}

	return &ConnectResult{
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		ExternalAccountID: openID,
		AccountName:       userInfo.displayName,
		AvatarURL:         userInfo.avatarURL,
		Scopes:            config.Scopes,
		Metadata: map[string]any{
			"open_id":          openID,
			"display_name":     userInfo.displayName,
			"username":         userInfo.username,
			"profile_web_link": userInfo.profileWebLink,
			"is_verified":      userInfo.isVerified,
			"granted_scopes":   config.Scopes,
		},
	}, nil
}

func (a *TikTokAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("tiktok requires OAuth flow, use POST /v1/oauth/connect")
}

// TikTokPrivacyValues is the set of allowed values for opts["privacy_level"].
// Mirrors TikTok Content Posting API post_info.privacy_level. Note: which of
// these are actually accepted depends on whether the connected app is in
// sandbox/unaudited mode (TikTok forces SELF_ONLY in that case).
var TikTokPrivacyValues = []string{
	"PUBLIC_TO_EVERYONE",
	"MUTUAL_FOLLOW_FRIENDS",
	"FOLLOWER_OF_CREATOR",
	"SELF_ONLY",
}

// buildTikTokPostInfo normalizes the common post_info object used across
// TikTok's video and photo publish-init endpoints. TikTok returns a
// misleading "Invalid authorization header" 400 for some malformed bodies,
// so we always send the required toggle fields with permissive defaults.
//
// mediaKind is "video" or "photo"; photo posts put the full caption in
// description and omit title because TikTok caps photo titles at 90 UTF-16
// code units. Video posts additionally emit disable_duet / disable_stitch
// (TikTok rejects those fields on photo posts). disable_* fields default to
// false which keeps existing API callers unchanged, but the dashboard sends
// them all true by default to satisfy the Content Posting API audit (toggles
// off by default).
func buildTikTokPostInfo(text, privacyLevel string, opts map[string]any, mediaKind string) map[string]any {
	info := map[string]any{
		"privacy_level":        privacyLevel,
		"disable_comment":      optBool(opts, "disable_comment"),
		"auto_add_music":       true,
		"brand_content_toggle": optBool(opts, "brand_content_toggle"),
		"brand_organic_toggle": optBool(opts, "brand_organic_toggle"),
	}
	if mediaKind == "video" {
		info["title"] = text
		info["description"] = text
		info["disable_duet"] = optBool(opts, "disable_duet")
		info["disable_stitch"] = optBool(opts, "disable_stitch")
	} else {
		info["description"] = text
	}
	return info
}

func wrapTikTokInitError(prefix string, status int, body []byte, privacyLevel string) error {
	code, providerMessage := parseTikTokErrorBody(body)
	fields := map[string]any{
		"provider":    "tiktok",
		"http_status": status,
		"code":        code,
	}
	if providerMessage != "" {
		fields["reason"] = providerMessage
	}
	if code == "invalid_params" {
		return newProviderError(tiktokInvalidParamsMessage(prefix, status, providerMessage, privacyLevel), fields)
	}
	msg := fmt.Sprintf("%s (%d): %s", prefix, status, string(body))
	return newProviderError(msg, fields)
}

func parseTikTokErrorBody(body []byte) (code string, message string) {
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	return strings.TrimSpace(payload.Error.Code), strings.TrimSpace(payload.Error.Message)
}

func tiktokInvalidParamsMessage(prefix string, status int, providerMessage string, privacyLevel string) string {
	mediaKind := "publish"
	if strings.Contains(prefix, "photo init") {
		mediaKind = "photo publish"
	} else if strings.Contains(prefix, "pull init") {
		mediaKind = "video publish"
	}
	parts := []string{
		fmt.Sprintf("TikTok rejected the %s request: TikTok reported invalid_params.", mediaKind),
		"Common fixes: confirm the TikTok privacy/content options are supported.",
		"UniPost sends TikTok's required interaction toggles automatically.",
	}
	if strings.Contains(prefix, "photo init") {
		parts = append(parts, "For photo posts, keep photo captions/titles to 90 characters or fewer.")
	}
	if privacyLevel != "SELF_ONLY" {
		parts = append(parts, "If this TikTok app is still in sandbox/unaudited mode, use SELF_ONLY privacy until app review is complete.")
	}
	if providerMessage != "" && !strings.Contains(strings.ToLower(providerMessage), "invalid authorization header") {
		parts = append(parts, "TikTok message: "+providerMessage+".")
	}
	parts = append(parts, fmt.Sprintf("(provider_error=invalid_params, status=%d)", status))
	return strings.Join(parts, " ")
}

func shouldRetryTikTokWithSelfOnly(status int, body []byte, privacyLevel string) bool {
	if privacyLevel == "SELF_ONLY" || status != http.StatusBadRequest {
		return false
	}
	return strings.Contains(string(body), `"code":"invalid_params"`)
}

// Post publishes a video to TikTok using direct file upload.
//
// Supported opts (all coming from the compose UI; see tiktok-fields.tsx):
//   - privacy_level: one of TikTokPrivacyValues. Defaults to
//     "PUBLIC_TO_EVERYONE"; sandbox apps that can't actually use that
//     fall back to SELF_ONLY via shouldRetryTikTokWithSelfOnly.
//   - disable_comment / disable_duet / disable_stitch: interaction
//     toggles. UI sends all three OFF (true) by default; we forward
//     them to TikTok. disable_duet / disable_stitch are only emitted
//     on video posts (TikTok rejects them for photos).
//   - brand_content_toggle / brand_organic_toggle: commercial content
//     disclosure. If brand_content_toggle is true, privacy_level MUST
//     NOT be SELF_ONLY — TikTok's Content Posting API audit rejects
//     that combo.
func (a *TikTokAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	if len(media) == 0 {
		return nil, fmt.Errorf("tiktok requires at least one media item")
	}

	// Photo carousels go down a separate API path. If none of the items
	// are videos, dispatch to the PHOTO uploader. We treat unknown-kind URLs
	// (no file extension — common with image CDNs like Unsplash) as photos
	// because TikTok doesn't accept text-only posts and "video served at an
	// extensionless URL" is much rarer than "image served at an extensionless
	// URL". Callers that need to force the video path can use a URL with a
	// .mp4 / .mov / .webm extension.
	videos := FilterByKind(media, MediaKindVideo)
	if len(videos) == 0 {
		return a.postPhoto(ctx, accessToken, text, media, opts)
	}

	// Video path: TikTok doesn't accept mixed media or multi-video posts.
	if len(media) > 1 {
		return nil, fmt.Errorf("tiktok video posts accept exactly one video")
	}
	videoURL := media[0].URL

	privacyLevel := optString(opts, "privacy_level")
	if err := validateEnum("tiktok", "privacy_level", privacyLevel, TikTokPrivacyValues); err != nil {
		return nil, err
	}
	if privacyLevel == "" {
		privacyLevel = "PUBLIC_TO_EVERYONE"
	}

	// Branded Content (= "paid partnership") cannot be posted privately.
	// The compose UI already blocks this combo; we re-check here to catch
	// direct API callers who bypass the dashboard.
	if optBool(opts, "brand_content_toggle") && privacyLevel == "SELF_ONLY" {
		return nil, fmt.Errorf("tiktok: branded content cannot be posted with privacy_level=SELF_ONLY")
	}

	// Default to FILE_UPLOAD: TikTok hands back an upload URL and we PUT the
	// video bytes to it directly. Unlike PULL_FROM_URL this works for any
	// source domain — TikTok doesn't require URL prefix verification when
	// the developer is the one uploading. The cost is bandwidth: every
	// video transits this server. Callers who have registered their CDN
	// with their TikTok dev portal can opt in to the faster path with
	// platform_options.tiktok.upload_mode = "pull_from_url".
	uploadMode := strings.ToLower(optString(opts, "upload_mode"))
	if uploadMode == "" {
		uploadMode = "file_upload"
	}

	var (
		publishID string
		initErr   error
	)
	if resume := resumePublishToken(opts); resume != "" {
		// Idempotent resume: a prior attempt already submitted this video.
		// Poll the existing publish_id instead of re-uploading — waitForPublish
		// returns the finished post if it already completed, so we never
		// submit the same video twice.
		publishID = resume
		slog.Info("tiktok post: resuming from persisted publish_id", "publish_id", publishID)
	} else {
		if uploadMode == "pull_from_url" {
			publishID, initErr = a.initVideoPull(ctx, accessToken, text, privacyLevel, videoURL, opts)
		} else {
			publishID, initErr = a.initAndPushVideoFile(ctx, accessToken, text, privacyLevel, videoURL, opts)
		}
		if initErr != nil {
			return nil, initErr
		}
		// Persist before polling so a crash during the poll resumes here.
		persistPublishToken(opts, publishID)
	}

	slog.Info("tiktok post: video submitted, polling status", "publish_id", publishID)
	return a.waitForPublish(ctx, accessToken, publishID)
}

// postPhoto handles TikTok photo carousels via the content/init endpoint.
// Unlike the video path this uses PULL_FROM_URL — TikTok pulls each image
// from the caller's CDN, so we don't have to download/re-upload bytes here.
// All image URLs must already be hosted on a verified domain or URL prefix
// in the developer portal; otherwise TikTok rejects the init call.
func (a *TikTokAdapter) postPhoto(ctx context.Context, accessToken, text string, images []MediaItem, opts map[string]any) (*PostResult, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("tiktok photo post requires at least one image")
	}

	// Idempotent resume: a prior attempt already submitted this carousel.
	// Poll the existing publish_id instead of re-staging images to R2 and
	// re-initiating, so we never submit the same carousel twice.
	if resume := resumePublishToken(opts); resume != "" {
		slog.Info("tiktok photo post: resuming from persisted publish_id", "publish_id", resume)
		return a.waitForPublish(ctx, accessToken, resume)
	}

	privacyLevel := optString(opts, "privacy_level")
	if err := validateEnum("tiktok", "privacy_level", privacyLevel, TikTokPrivacyValues); err != nil {
		return nil, err
	}
	if privacyLevel == "" {
		privacyLevel = "PUBLIC_TO_EVERYONE"
	}

	// Same branded-private interlock as the video path; see the comment
	// above Post() for the audit reason.
	if optBool(opts, "brand_content_toggle") && privacyLevel == "SELF_ONLY" {
		return nil, fmt.Errorf("tiktok: branded content cannot be posted with privacy_level=SELF_ONLY")
	}

	// TikTok photo Direct Post only accepts PULL_FROM_URL, and the source
	// domain must be verified in the developer portal. Since we can't ask
	// every UniPost customer to register their CDN with us, we stage each
	// image on our own R2 bucket (whose URL prefix is registered once)
	// and hand TikTok the proxied URLs. See internal/mediaproxy.
	if a.mediaProxy == nil {
		return nil, fmt.Errorf("tiktok photo posts require the mediaproxy R2 client to be configured (set R2_* env vars)")
	}

	urls := make([]string, 0, len(images))
	for _, item := range images {
		proxied, err := a.mediaProxy.UploadFromURL(ctx, item.URL)
		if err != nil {
			return nil, fmt.Errorf("tiktok photo: stage to R2: %w", err)
		}
		urls = append(urls, proxied)
	}

	cover := 0
	if v, ok := opts["photo_cover_index"].(float64); ok {
		cover = int(v)
	}
	if cover < 0 || cover >= len(urls) {
		cover = 0
	}

	return a.initPhotoContentWithPrivacy(ctx, accessToken, text, urls, cover, privacyLevel, opts, true)
}

func (a *TikTokAdapter) initPhotoContentWithPrivacy(ctx context.Context, accessToken, text string, urls []string, cover int, privacyLevel string, opts map[string]any, allowRetry bool) (*PostResult, error) {
	// TikTok's photo Direct Post requires every "toggle" field to be present
	// in the request body — omitting any of disable_comment, auto_add_music,
	// brand_content_toggle, or brand_organic_toggle returns a misleading
	// 400 "Invalid authorization header" error rather than a sensible
	// validation message. Defaults match the most permissive choice:
	// comments enabled, music enabled, no brand disclosures.
	body, _ := json.Marshal(map[string]any{
		"post_info": buildTikTokPostInfo(text, privacyLevel, opts, "photo"),
		"source_info": map[string]any{
			"source":            "PULL_FROM_URL",
			"photo_cover_index": cover,
			"photo_images":      urls,
		},
		"post_mode":  "DIRECT_POST",
		"media_type": "PHOTO",
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/content/init/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok photo init: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if allowRetry && !optBool(opts, "brand_content_toggle") && shouldRetryTikTokWithSelfOnly(resp.StatusCode, respBody, privacyLevel) {
			slog.Warn("tiktok photo init rejected requested privacy; retrying with SELF_ONLY", "requested_privacy", privacyLevel)
			return a.initPhotoContentWithPrivacy(ctx, accessToken, text, urls, cover, "SELF_ONLY", opts, false)
		}
		return nil, wrapTikTokInitError("tiktok photo init failed", resp.StatusCode, respBody, privacyLevel)
	}

	var result struct {
		Data struct {
			PublishID string `json:"publish_id"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(respBody, &result)
	if result.Error.Code != "" && result.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok photo error: %s", result.Error.Message)
	}
	if result.Data.PublishID == "" {
		return nil, fmt.Errorf("tiktok photo init: empty publish_id")
	}

	// Persist before polling so a crash during the poll resumes here.
	persistPublishToken(opts, result.Data.PublishID)

	slog.Info("tiktok photo post: submitted, polling status", "publish_id", result.Data.PublishID)
	return a.waitForPublish(ctx, accessToken, result.Data.PublishID)
}

func (a *TikTokAdapter) waitForPublish(ctx context.Context, accessToken string, publishID string) (*PostResult, error) {
	for i := 0; i < tiktokPublishPollAttempts; i++ {
		if tiktokPublishPollInterval > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(tiktokPublishPollInterval):
			}
		} else {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}

		status, err := a.CheckPublishStatus(ctx, accessToken, publishID)
		if err != nil {
			continue
		}

		data, _ := status["data"].(map[string]any)
		if data == nil {
			continue
		}

		publishStatus, _ := data["status"].(string)
		switch publishStatus {
		case "PUBLISH_COMPLETE":
			slog.Info("tiktok post: publish complete", "publish_id", publishID)
			return a.tiktokPublishedResult(ctx, accessToken, publishID, data), nil
		case "FAILED":
			reason, _ := data["fail_reason"].(string)
			if strings.TrimSpace(reason) == "" {
				reason = "unknown_error"
			}
			return nil, newProviderError(fmt.Sprintf("tiktok publish failed: %s", reason), map[string]any{
				"provider": "tiktok",
				"reason":   reason,
			})
		}
	}

	return &PostResult{
		ExternalID: publishID,
		Status:     "processing",
	}, nil
}

func (a *TikTokAdapter) tiktokPublishedResult(ctx context.Context, accessToken string, publishID string, data map[string]any) *PostResult {
	result := &PostResult{ExternalID: publishID}
	if url := TikTokPublicPostURLFromStatusData(data); url != "" {
		result.URL = url
		return result
	}
	if url := a.TikTokProfileURL(ctx, accessToken); url != "" {
		result.URL = url
	}
	return result
}

// initVideoPull asks TikTok to pull the video itself from the source URL.
// This is the preferred path: it skips a download/re-upload round-trip on our
// side and means the video bytes never touch the API server. The source URL
// must be on a domain registered with TikTok's developer portal.
func (a *TikTokAdapter) initVideoPull(ctx context.Context, accessToken, text, privacyLevel, videoURL string, opts map[string]any) (string, error) {
	return a.initVideoPullWithPrivacy(ctx, accessToken, text, privacyLevel, videoURL, opts, true)
}

func (a *TikTokAdapter) initVideoPullWithPrivacy(ctx context.Context, accessToken, text, privacyLevel, videoURL string, opts map[string]any, allowRetry bool) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"post_info": buildTikTokPostInfo(text, privacyLevel, opts, "video"),
		"source_info": map[string]any{
			"source":    "PULL_FROM_URL",
			"video_url": videoURL,
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/video/init/", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tiktok pull init: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if allowRetry && shouldRetryTikTokWithSelfOnly(resp.StatusCode, respBody, privacyLevel) {
			slog.Warn("tiktok pull init rejected requested privacy; retrying with SELF_ONLY", "requested_privacy", privacyLevel)
			return a.initVideoPullWithPrivacy(ctx, accessToken, text, "SELF_ONLY", videoURL, opts, false)
		}
		return "", wrapTikTokInitError("tiktok pull init failed", resp.StatusCode, respBody, privacyLevel)
	}

	var result struct {
		Data struct {
			PublishID string `json:"publish_id"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(respBody, &result)
	if result.Error.Code != "" && result.Error.Code != "ok" {
		return "", fmt.Errorf("tiktok error: %s", result.Error.Message)
	}
	if result.Data.PublishID == "" {
		return "", fmt.Errorf("tiktok pull init: empty publish_id")
	}
	return result.Data.PublishID, nil
}

// initAndPushVideoFile is the legacy FILE_UPLOAD flow: download the video
// bytes locally, INIT with TikTok asking for an upload URL, then PUT the
// bytes to that URL. Used as a fallback when PULL_FROM_URL isn't viable
// (source not on a verified domain, intranet URLs, etc.).
func (a *TikTokAdapter) initAndPushVideoFile(ctx context.Context, accessToken, text, privacyLevel, videoURL string, opts map[string]any) (string, error) {
	return a.initAndPushVideoFileWithPrivacy(ctx, accessToken, text, privacyLevel, videoURL, opts, true)
}

func (a *TikTokAdapter) initAndPushVideoFileWithPrivacy(ctx context.Context, accessToken, text, privacyLevel, videoURL string, opts map[string]any, allowRetry bool) (string, error) {
	videoResp, err := a.client.Get(videoURL)
	if err != nil {
		return "", fmt.Errorf("failed to download video: %w", err)
	}
	defer videoResp.Body.Close()

	videoData, err := io.ReadAll(videoResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read video: %w", err)
	}

	videoSize := len(videoData)
	slog.Info("tiktok post: downloaded video", "size", videoSize)

	chunkSize, totalChunkCount := tiktokVideoUploadPlan(videoSize)
	initBody, _ := json.Marshal(map[string]any{
		"post_info": buildTikTokPostInfo(text, privacyLevel, opts, "video"),
		"source_info": map[string]any{
			"source":            "FILE_UPLOAD",
			"video_size":        videoSize,
			"chunk_size":        chunkSize,
			"total_chunk_count": totalChunkCount,
		},
	})

	initReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/video/init/", bytes.NewReader(initBody))
	if err != nil {
		return "", err
	}
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	initReq.Header.Set("Authorization", "Bearer "+accessToken)

	initResp, err := a.client.Do(initReq)
	if err != nil {
		return "", fmt.Errorf("failed to init upload: %w", err)
	}
	defer initResp.Body.Close()

	initRespBody, _ := io.ReadAll(initResp.Body)
	if initResp.StatusCode != http.StatusOK {
		if allowRetry && shouldRetryTikTokWithSelfOnly(initResp.StatusCode, initRespBody, privacyLevel) {
			slog.Warn("tiktok upload init rejected requested privacy; retrying with SELF_ONLY", "requested_privacy", privacyLevel)
			return a.initAndPushVideoFileWithPrivacy(ctx, accessToken, text, "SELF_ONLY", videoURL, opts, false)
		}
		return "", wrapTikTokInitError("tiktok upload init failed", initResp.StatusCode, initRespBody, privacyLevel)
	}

	var initResult struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(initRespBody, &initResult)
	if initResult.Error.Code != "" && initResult.Error.Code != "ok" {
		return "", fmt.Errorf("tiktok error: %s", initResult.Error.Message)
	}
	if initResult.Data.UploadURL == "" {
		return "", fmt.Errorf("tiktok returned no upload URL")
	}

	slog.Info("tiktok post: uploading video",
		"publish_id", initResult.Data.PublishID,
		"chunk_size", chunkSize,
		"total_chunk_count", totalChunkCount,
	)

	for chunkIndex := 0; chunkIndex < totalChunkCount; chunkIndex++ {
		start := chunkIndex * chunkSize
		end := start + chunkSize
		if chunkIndex == totalChunkCount-1 {
			end = videoSize
		}
		chunk := videoData[start:end]

		uploadReq, err := http.NewRequestWithContext(ctx, "PUT", initResult.Data.UploadURL, bytes.NewReader(chunk))
		if err != nil {
			return "", err
		}
		uploadReq.Header.Set("Content-Type", "application/octet-stream")
		uploadReq.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end-1, videoSize))

		uploadResp, err := a.client.Do(uploadReq)
		if err != nil {
			return "", fmt.Errorf("failed to upload video chunk %d/%d: %w", chunkIndex+1, totalChunkCount, err)
		}
		uploadRespBody, _ := io.ReadAll(uploadResp.Body)
		_ = uploadResp.Body.Close()
		slog.Info("tiktok post: upload response",
			"status", uploadResp.StatusCode,
			"chunk", chunkIndex+1,
			"total_chunks", totalChunkCount,
			"body", string(uploadRespBody),
		)
		if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated && uploadResp.StatusCode != http.StatusPartialContent {
			return "", fmt.Errorf("tiktok upload chunk %d/%d failed (%d): %s", chunkIndex+1, totalChunkCount, uploadResp.StatusCode, string(uploadRespBody))
		}
	}

	return initResult.Data.PublishID, nil
}

func tiktokVideoUploadPlan(videoSize int) (int, int) {
	if videoSize <= tiktokMaxRegularChunkSizeBytes {
		return videoSize, 1
	}
	totalChunkCount := videoSize / tiktokDefaultUploadChunkSize
	return tiktokDefaultUploadChunkSize, totalChunkCount
}

// TikTokCreatorInfo mirrors the subset of /v2/post/publish/creator_info/query/
// we need to drive the compose UI. All fields come straight from TikTok's
// response (we don't synthesize defaults here — callers decide how to render
// missing data).
//
// Required for TikTok's Content Posting API audit: the UI must display the
// creator's nickname, constrain the privacy dropdown to privacy_level_option,
// disable interaction toggles the creator has turned off on TikTok, and
// reject videos longer than max_video_post_duration_sec. See
// https://developers.tiktok.com/doc/content-posting-api-reference-query-creator-info/
type TikTokCreatorInfo struct {
	CreatorAvatarURL        string   `json:"creator_avatar_url"`
	CreatorUsername         string   `json:"creator_username"`
	CreatorNickname         string   `json:"creator_nickname"`
	PrivacyLevelOptions     []string `json:"privacy_level_options"`
	CommentDisabled         bool     `json:"comment_disabled"`
	DuetDisabled            bool     `json:"duet_disabled"`
	StitchDisabled          bool     `json:"stitch_disabled"`
	MaxVideoPostDurationSec int      `json:"max_video_post_duration_sec"`
}

// FetchCreatorInfo calls /v2/post/publish/creator_info/query/ and returns the
// creator metadata needed by the compose UI. The endpoint requires the
// video.publish scope, which every TikTok-connected account in UniPost already
// has (see DefaultOAuthConfig above).
//
// TikTok returns HTTP 200 with an "error" envelope even for auth failures,
// so we treat any non-ok error.code as a fatal error and surface the message.
func (a *TikTokAdapter) FetchCreatorInfo(ctx context.Context, accessToken string) (*TikTokCreatorInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/creator_info/query/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok creator_info: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok creator_info (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data  TikTokCreatorInfo `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("tiktok creator_info decode: %w", err)
	}
	if result.Error.Code != "" && result.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok creator_info: %s", result.Error.Message)
	}
	return &result.Data, nil
}

// CheckPublishStatus queries TikTok for the publish status of a video.
func (a *TikTokAdapter) CheckPublishStatus(ctx context.Context, accessToken string, publishID string) (map[string]any, error) {
	body, _ := json.Marshal(map[string]string{
		"publish_id": publishID,
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/status/fetch/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tiktok publish status: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok publish status returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(respBody))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("tiktok publish status: decode: %w", err)
	}
	if errObj, ok := result["error"].(map[string]any); ok {
		code, _ := errObj["code"].(string)
		if code != "" && code != "ok" {
			message, _ := errObj["message"].(string)
			return nil, fmt.Errorf("tiktok publish status: %s: %s", code, message)
		}
	}
	return result, nil
}

func (a *TikTokAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	return fmt.Errorf("tiktok does not support post deletion via API")
}

func (a *TikTokAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	config := a.DefaultOAuthConfig("")
	data := url.Values{
		"client_key":    {config.ClientID},
		"client_secret": {config.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer resp.Body.Close()

	// Previously we parsed whatever TikTok returned and ignored the
	// status code. A rate-limited / temporarily-invalid 4xx response
	// would decode to a zero struct, and callers would silently write
	// empty tokens + zero expires_at into the DB — the next request
	// then sent `Authorization: Bearer ` and TikTok closed the
	// connection, surfacing to the browser as "Failed to fetch". Now:
	// any non-2xx is a fatal error, and a 200 that somehow lacks an
	// access_token is treated the same — we refuse to return empty
	// credentials even when TikTok claims success.
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", time.Time{}, fmt.Errorf("tiktok refresh (%d): %s", resp.StatusCode, string(respBody))
	}

	// TikTok may also nest the response under a top-level "error"
	// envelope with code != "ok" while still returning 200. Walk both
	// shapes so we don't treat those as success.
	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return "", "", time.Time{}, fmt.Errorf("tiktok refresh: decode: %w", err)
	}
	if errObj, ok := raw["error"].(map[string]any); ok {
		if code, _ := errObj["code"].(string); code != "" && code != "ok" {
			msg, _ := errObj["message"].(string)
			return "", "", time.Time{}, fmt.Errorf("tiktok refresh: %s: %s", code, msg)
		}
	}

	var tokenResp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
		// Fields also appear at root level in some TikTok sandbox
		// responses; capture both so we don't silently miss either.
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", "", time.Time{}, fmt.Errorf("tiktok refresh: decode: %w", err)
	}

	accessToken := firstNonEmpty(tokenResp.Data.AccessToken, tokenResp.AccessToken)
	rotatedRefresh := firstNonEmpty(tokenResp.Data.RefreshToken, tokenResp.RefreshToken)
	expiresIn := tokenResp.Data.ExpiresIn
	if expiresIn == 0 {
		expiresIn = tokenResp.ExpiresIn
	}
	if accessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("tiktok refresh: response missing access_token (body=%s)", string(respBody))
	}
	if rotatedRefresh == "" {
		// TikTok always rotates refresh tokens on refresh; if the
		// server didn't include one, reuse the caller's so we don't
		// silently clobber the stored refresh token with "".
		rotatedRefresh = refreshToken
	}
	if expiresIn <= 0 {
		// Fall back to TikTok's documented 24h access-token lifetime
		// so a bogus expires_in doesn't immediately re-trigger this
		// refresh path on the next call.
		expiresIn = 86400
	}

	return accessToken, rotatedRefresh, time.Now().Add(time.Duration(expiresIn) * time.Second), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetAnalytics fetches video metrics from TikTok.
//
// Two-step flow because the externalID we store is a publish_id (the token
// returned by /v2/post/publish/video/init/), NOT a real TikTok video ID.
// The /v2/video/query/ endpoint expects the 19-digit public video ID.
//
//  1. Call /v2/post/publish/status/fetch/ with the publish_id to resolve it
//     to a public post ID. The field is misspelled in TikTok's API as
//     "publicaly_available_post_id" (their typo, not ours).
//  2. Call /v2/video/query/ with the resolved video_id to get stats.
//
// Requires the video.list OAuth scope on the connected account. Accounts
// connected before video.list was added to the scope list will get 401 here
// and must reconnect — there's no way to upgrade an existing token in place.
func (a *TikTokAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	// Step 1: resolve publish_id → video_id via the publish status endpoint.
	statusResp, err := a.CheckPublishStatus(ctx, accessToken, externalID)
	if err != nil {
		return nil, fmt.Errorf("tiktok analytics: status fetch failed: %w", err)
	}
	data, _ := statusResp["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("tiktok analytics: publish status returned no data")
	}
	publishStatus, _ := data["status"].(string)
	if publishStatus != "PUBLISH_COMPLETE" {
		// Post isn't fully published yet (still uploading, processing, or
		// failed). Nothing to fetch. Return zero metrics with no error so the
		// handler caches this and tries again on the next refresh.
		slog.Info("tiktok analytics: post not yet published",
			"publish_id", externalID, "status", publishStatus)
		return &PostMetrics{}, nil
	}

	videoID, err := tiktokExtractPublicPostID(data)
	if err != nil {
		return nil, fmt.Errorf("tiktok analytics: public post ID: %w", err)
	}

	// Step 2: query the video for stats.
	body, _ := json.Marshal(map[string]any{
		"filters": map[string]any{
			"video_ids": []string{videoID},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/video/query/?fields=id,like_count,comment_count,share_count,view_count",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok analytics: video query request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("tiktok analytics: video query non-200",
			"status", resp.StatusCode,
			"video_id", videoID,
			"body", string(respBody))
		// 401 here almost always means the account was connected before the
		// video.list scope was added — surface a clearer error so the user
		// knows to reconnect.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("tiktok analytics: missing video.list scope (reconnect the account)")
		}
		return nil, fmt.Errorf("tiktok analytics: video query returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			Videos []struct {
				ViewCount    int64 `json:"view_count"`
				LikeCount    int64 `json:"like_count"`
				CommentCount int64 `json:"comment_count"`
				ShareCount   int64 `json:"share_count"`
			} `json:"videos"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("tiktok analytics: decode failed: %w", err)
	}

	if len(result.Data.Videos) == 0 {
		// Video query succeeded but returned no rows — usually means the video
		// is private or was deleted. Cache zeros to avoid retry storms.
		slog.Warn("tiktok analytics: video not found in query response",
			"video_id", videoID)
		return &PostMetrics{}, nil
	}

	v := result.Data.Videos[0]
	// TikTok exposes view_count (= video plays) but not display impressions in
	// the basic video query. EngagementRate is computed by the analytics handler.
	return &PostMetrics{
		VideoViews: v.ViewCount,
		Views:      v.ViewCount, // legacy alias
		Likes:      v.LikeCount,
		Comments:   v.CommentCount,
		Shares:     v.ShareCount,
		PlatformSpecific: map[string]any{
			"tiktok_video_id": videoID,
		},
	}, nil
}

// tiktokExtractPublicPostID pulls the public post ID out of a publish status
// response. TikTok's field name is "publicaly_available_post_id" (their typo);
// we also check spellings they might fix it to. Values can come back as
// numbers or strings depending on API version.
func tiktokExtractPublicPostID(data map[string]any) (string, error) {
	for _, key := range []string{
		"publicaly_available_post_id",  // TikTok's current (typo'd) spelling
		"publically_available_post_id", // possible future fix
		"publicly_available_post_id",   // another possible future fix
	} {
		arr, ok := data[key].([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		var id string
		switch v := arr[0].(type) {
		case string:
			id = v
		case json.Number:
			id = v.String()
		default:
			return "", fmt.Errorf("%s has unsupported value type %T", key, arr[0])
		}
		if id == "" {
			return "", fmt.Errorf("%s is empty", key)
		}
		for _, digit := range id {
			if digit < '0' || digit > '9' {
				return "", fmt.Errorf("%s %q is not an unsigned base-10 integer", key, id)
			}
		}
		return id, nil
	}
	return "", fmt.Errorf("no public post ID in publish status response")
}

func TikTokPublicPostURL(postID string) string {
	if strings.TrimSpace(postID) == "" {
		return ""
	}
	return fmt.Sprintf("https://www.tiktok.com/player/v1/%s", postID)
}

func TikTokProfileURL(username string) string {
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if username == "" {
		return ""
	}
	return fmt.Sprintf("%s/@%s", tiktokHomepageURL, username)
}

func TikTokPublicPostURLFromStatus(status map[string]any) string {
	data, _ := status["data"].(map[string]any)
	return TikTokPublicPostURLFromStatusData(data)
}

func TikTokPublicPostURLFromStatusData(data map[string]any) string {
	if data == nil {
		return ""
	}
	postID, err := tiktokExtractPublicPostID(data)
	if err != nil {
		return ""
	}
	return TikTokPublicPostURL(postID)
}

func (a *TikTokAdapter) ResolvePostOrProfileURL(ctx context.Context, accessToken string, status map[string]any) string {
	if url := TikTokPublicPostURLFromStatus(status); url != "" {
		return url
	}
	return a.TikTokProfileURL(ctx, accessToken)
}

func (a *TikTokAdapter) TikTokProfileURL(ctx context.Context, accessToken string) string {
	info, err := a.FetchCreatorInfo(ctx, accessToken)
	if err != nil || info == nil {
		return ""
	}
	return TikTokProfileURL(info.CreatorUsername)
}

type tiktokUserInfo struct {
	openID          string
	displayName     string
	avatarURL       string
	username        string
	profileWebLink  string
	profileDeepLink string
	bioDescription  string
	isVerified      bool
	followerCount   int64
	followingCount  int64
	likesCount      int64
	videoCount      int64
}

func (a *TikTokAdapter) getBasicUserInfo(ctx context.Context, accessToken string) (*tiktokUserInfo, error) {
	return a.getUserInfo(ctx, accessToken, tiktokBasicUserInfoFields)
}

func (a *TikTokAdapter) getProfileUserInfo(ctx context.Context, accessToken string) (*tiktokUserInfo, error) {
	return a.getUserInfo(ctx, accessToken, tiktokProfileUserInfoFields)
}

func (a *TikTokAdapter) getMetricsUserInfo(ctx context.Context, accessToken string) (*tiktokUserInfo, error) {
	return a.getUserInfo(ctx, accessToken, tiktokMetricsUserInfoFields)
}

func (a *TikTokAdapter) getUserInfo(ctx context.Context, accessToken string, fieldList []string) (*tiktokUserInfo, error) {
	fields := strings.Join(fieldList, ",")

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://open.tiktokapis.com/v2/user/info/?fields="+fields, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("tiktok user info: missing profile/stats scope or invalid token")
		}
		return nil, fmt.Errorf("tiktok user info (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			User struct {
				OpenID          string `json:"open_id"`
				DisplayName     string `json:"display_name"`
				AvatarURL       string `json:"avatar_url"`
				Username        string `json:"username"`
				ProfileWebLink  string `json:"profile_web_link"`
				ProfileDeepLink string `json:"profile_deep_link"`
				BioDescription  string `json:"bio_description"`
				IsVerified      bool   `json:"is_verified"`
				FollowerCount   int64  `json:"follower_count"`
				FollowingCount  int64  `json:"following_count"`
				LikesCount      int64  `json:"likes_count"`
				VideoCount      int64  `json:"video_count"`
			} `json:"user"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("tiktok user info decode: %w", err)
	}
	if result.Error.Code != "" && result.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok user info: %s", result.Error.Message)
	}

	return &tiktokUserInfo{
		openID:          result.Data.User.OpenID,
		displayName:     result.Data.User.DisplayName,
		avatarURL:       result.Data.User.AvatarURL,
		username:        result.Data.User.Username,
		profileWebLink:  result.Data.User.ProfileWebLink,
		profileDeepLink: result.Data.User.ProfileDeepLink,
		bioDescription:  result.Data.User.BioDescription,
		isVerified:      result.Data.User.IsVerified,
		followerCount:   result.Data.User.FollowerCount,
		followingCount:  result.Data.User.FollowingCount,
		likesCount:      result.Data.User.LikesCount,
		videoCount:      result.Data.User.VideoCount,
	}, nil
}

type TikTokProfile struct {
	OpenID          string `json:"open_id"`
	DisplayName     string `json:"display_name"`
	AvatarURL       string `json:"avatar_url"`
	Username        string `json:"username"`
	ProfileWebLink  string `json:"profile_web_link"`
	ProfileDeepLink string `json:"profile_deep_link"`
	BioDescription  string `json:"bio_description"`
	IsVerified      bool   `json:"is_verified"`
}

func (a *TikTokAdapter) FetchProfile(ctx context.Context, accessToken string) (*TikTokProfile, error) {
	info, err := a.getProfileUserInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &TikTokProfile{
		OpenID:          info.openID,
		DisplayName:     info.displayName,
		AvatarURL:       info.avatarURL,
		Username:        info.username,
		ProfileWebLink:  info.profileWebLink,
		ProfileDeepLink: info.profileDeepLink,
		BioDescription:  info.bioDescription,
		IsVerified:      info.isVerified,
	}, nil
}

func (a *TikTokAdapter) GetAccountMetrics(ctx context.Context, accessToken, externalAccountID string) (*AccountMetrics, error) {
	info, err := a.getMetricsUserInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &AccountMetrics{
		FollowerCount:  info.followerCount,
		FollowingCount: info.followingCount,
		PostCount:      info.videoCount,
		PlatformSpecific: map[string]any{
			"likes_count": info.likesCount,
			"video_count": info.videoCount,
		},
	}, nil
}

type TikTokVideo struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	CoverImageURL    string `json:"cover_image_url"`
	ShareURL         string `json:"share_url"`
	CreateTime       int64  `json:"create_time"`
	ViewCount        int64  `json:"view_count"`
	LikeCount        int64  `json:"like_count"`
	CommentCount     int64  `json:"comment_count"`
	ShareCount       int64  `json:"share_count"`
	Duration         int64  `json:"duration"`
	EmbedLink        string `json:"embed_link"`
	VideoDescription string `json:"video_description"`
}

type TikTokVideoList struct {
	Videos  []TikTokVideo `json:"videos"`
	Cursor  int64         `json:"cursor"`
	HasMore bool          `json:"has_more"`
}

func (a *TikTokAdapter) ListVideos(ctx context.Context, accessToken string, cursor int64, limit int) (*TikTokVideoList, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	body, _ := json.Marshal(map[string]any{
		"max_count": limit,
		"cursor":    cursor,
	})
	fields := "id,title,cover_image_url,share_url,create_time,view_count,like_count,comment_count,share_count,duration,embed_link,video_description"
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/video/list/?fields="+fields,
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok video list: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("tiktok video list: missing video.list scope or invalid token")
		}
		return nil, fmt.Errorf("tiktok video list (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data  TikTokVideoList `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("tiktok video list decode: %w", err)
	}
	if result.Error.Code != "" && result.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok video list: %s", result.Error.Message)
	}
	if result.Data.Videos == nil {
		result.Data.Videos = []TikTokVideo{}
	}
	return &result.Data, nil
}
