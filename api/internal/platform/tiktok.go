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
	"time"
)

type TikTokAdapter struct {
	client *http.Client
}

func NewTikTokAdapter() *TikTokAdapter {
	return &TikTokAdapter{client: &http.Client{Timeout: 120 * time.Second}}
}

func (a *TikTokAdapter) Platform() string { return "tiktok" }

func (a *TikTokAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("TIKTOK_CLIENT_KEY"),
		ClientSecret: os.Getenv("TIKTOK_CLIENT_SECRET"),
		AuthURL:      "https://www.tiktok.com/v2/auth/authorize/",
		TokenURL:     "https://open.tiktokapis.com/v2/oauth/token/",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/tiktok",
		// video.list is required by /v2/video/query/, which powers analytics.
		// Existing connected accounts predate this scope and must reconnect
		// for analytics to work — old access tokens don't carry it.
		Scopes:       []string{"video.publish", "video.upload", "video.list", "user.info.basic"},
	}
}

func (a *TikTokAdapter) GetAuthURL(config OAuthConfig, state string) string {
	// TikTok uses client_key instead of client_id
	params := url.Values{
		"client_key":    {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"video.publish,video.upload,video.list,user.info.basic"},
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
	userInfo, err := a.getUserInfo(ctx, accessToken)
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
		Metadata: map[string]any{
			"open_id":      openID,
			"display_name": userInfo.displayName,
		},
	}, nil
}

func (a *TikTokAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("tiktok requires OAuth flow, use /v1/oauth/connect/tiktok")
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

// Post publishes a video to TikTok using direct file upload.
//
// Supported opts:
//   - privacy_level: one of TikTokPrivacyValues. Defaults to "SELF_ONLY".
func (a *TikTokAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string, opts map[string]any) (*PostResult, error) {
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("tiktok requires a video URL")
	}

	privacyLevel := optString(opts, "privacy_level")
	if err := validateEnum("tiktok", "privacy_level", privacyLevel, TikTokPrivacyValues); err != nil {
		return nil, err
	}
	if privacyLevel == "" {
		privacyLevel = "SELF_ONLY"
	}

	// Step 1: Download the video
	videoResp, err := a.client.Get(imageURLs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	defer videoResp.Body.Close()

	videoData, err := io.ReadAll(videoResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read video: %w", err)
	}

	videoSize := len(videoData)
	slog.Info("tiktok post: downloaded video", "size", videoSize)

	// Step 2: Initialize upload with FILE_UPLOAD source
	initBody, _ := json.Marshal(map[string]any{
		"post_info": map[string]any{
			"title":         text,
			"privacy_level": privacyLevel,
		},
		"source_info": map[string]any{
			"source":     "FILE_UPLOAD",
			"video_size": videoSize,
			"chunk_size": videoSize,
			"total_chunk_count": 1,
		},
	})

	initReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/video/init/", bytes.NewReader(initBody))
	if err != nil {
		return nil, err
	}
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	initReq.Header.Set("Authorization", "Bearer "+accessToken)

	initResp, err := a.client.Do(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to init upload: %w", err)
	}
	defer initResp.Body.Close()

	initRespBody, _ := io.ReadAll(initResp.Body)

	if initResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok upload init failed (%d): %s", initResp.StatusCode, string(initRespBody))
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
		return nil, fmt.Errorf("tiktok error: %s", initResult.Error.Message)
	}

	if initResult.Data.UploadURL == "" {
		return nil, fmt.Errorf("tiktok returned no upload URL")
	}

	slog.Info("tiktok post: uploading video", "publish_id", initResult.Data.PublishID)

	// Step 3: Upload the video to the upload URL
	uploadReq, err := http.NewRequestWithContext(ctx, "PUT", initResult.Data.UploadURL, bytes.NewReader(videoData))
	if err != nil {
		return nil, err
	}
	uploadReq.Header.Set("Content-Type", "application/octet-stream")
	uploadReq.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", videoSize-1, videoSize))
	uploadReq.Header.Set("Content-Length", fmt.Sprintf("%d", videoSize))

	uploadResp, err := a.client.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("failed to upload video: %w", err)
	}
	defer uploadResp.Body.Close()

	uploadRespBody, _ := io.ReadAll(uploadResp.Body)
	slog.Info("tiktok post: upload response", "status", uploadResp.StatusCode, "body", string(uploadRespBody))

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("tiktok upload failed (%d): %s", uploadResp.StatusCode, string(uploadRespBody))
	}

	slog.Info("tiktok post: video uploaded, polling status", "publish_id", initResult.Data.PublishID)

	// Step 4: Poll publish status until complete or failed (max 60 seconds)
	for i := 0; i < 12; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		status, err := a.CheckPublishStatus(ctx, accessToken, initResult.Data.PublishID)
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
			slog.Info("tiktok post: publish complete", "publish_id", initResult.Data.PublishID)
			return &PostResult{
				ExternalID: initResult.Data.PublishID,
				URL:        "https://www.tiktok.com",
			}, nil
		case "FAILED":
			reason, _ := data["fail_reason"].(string)
			return nil, fmt.Errorf("tiktok publish failed: %s", reason)
		}
		// PROCESSING_UPLOAD or PROCESSING_DOWNLOAD — keep polling
	}

	// Timeout — return as published with publish_id, user can check status later
	return &PostResult{
		ExternalID: initResult.Data.PublishID,
		URL:        "https://www.tiktok.com",
	}, nil
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

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
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

	var tokenResp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	return tokenResp.Data.AccessToken, tokenResp.Data.RefreshToken,
		time.Now().Add(time.Duration(tokenResp.Data.ExpiresIn) * time.Second), nil
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

	videoID := tiktokExtractPublicPostID(data)
	if videoID == "" {
		return nil, fmt.Errorf("tiktok analytics: no public post ID in publish status response")
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
func tiktokExtractPublicPostID(data map[string]any) string {
	for _, key := range []string{
		"publicaly_available_post_id",  // TikTok's current (typo'd) spelling
		"publically_available_post_id", // possible future fix
		"publicly_available_post_id",   // another possible future fix
	} {
		arr, ok := data[key].([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		switch v := arr[0].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return fmt.Sprintf("%.0f", v)
		}
	}
	return ""
}

type tiktokUserInfo struct {
	openID      string
	displayName string
	avatarURL   string
}

func (a *TikTokAdapter) getUserInfo(ctx context.Context, accessToken string) (*tiktokUserInfo, error) {
	body, _ := json.Marshal(map[string]any{
		"fields": []string{"open_id", "display_name", "avatar_url"},
	})

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://open.tiktokapis.com/v2/user/info/?fields=open_id,display_name,avatar_url", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			User struct {
				OpenID      string `json:"open_id"`
				DisplayName string `json:"display_name"`
				AvatarURL   string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return &tiktokUserInfo{
		openID:      result.Data.User.OpenID,
		displayName: result.Data.User.DisplayName,
		avatarURL:   result.Data.User.AvatarURL,
	}, nil
}
