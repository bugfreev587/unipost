package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TwitterAdapter implements PlatformAdapter and OAuthAdapter for X/Twitter.
// Only works in Native mode — requires user's own API credentials.
type TwitterAdapter struct {
	client *http.Client
}

func NewTwitterAdapter() *TwitterAdapter {
	return &TwitterAdapter{client: &http.Client{Timeout: 30 * time.Second}}
}

func (a *TwitterAdapter) Platform() string { return "twitter" }

// DefaultOAuthConfig falls back to TWITTER_CLIENT_ID / TWITTER_CLIENT_SECRET
// env vars (the same pair Sprint 3 Connect uses) so the dashboard quickstart
// works for projects that haven't set up white-label platform_credentials.
// White-label credentials in platform_credentials still take precedence —
// the BYO oauth handler reads them first and only falls through to this
// function when none are configured.
func (a *TwitterAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("TWITTER_CLIENT_ID"),
		ClientSecret: os.Getenv("TWITTER_CLIENT_SECRET"),
		AuthURL:      "https://twitter.com/i/oauth2/authorize",
		TokenURL:     "https://api.x.com/2/oauth2/token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/twitter",
		Scopes:       []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
	}
}

func (a *TwitterAdapter) GetAuthURL(config OAuthConfig, state string) string {
	// X/Twitter uses OAuth 2.0 with PKCE
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"scope":         {"tweet.read tweet.write users.read offline.access"},
		"state":         {state},
		"code_challenge":        {state[:43]}, // Use part of state as challenge (simplified PKCE)
		"code_challenge_method": {"plain"},
	}
	return config.AuthURL + "?" + params.Encode()
}

func (a *TwitterAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	// PKCE: GetAuthURL sets code_challenge=state[:43] with method=plain, so
	// the verifier we send here MUST equal that same string. The handler
	// passes the original state through in OAuthConfig.State so we can
	// reconstruct it here without storing it on the row.
	data := url.Values{
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"client_id":     {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"code_verifier": {config.State[:43]},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(config.ClientID, config.ClientSecret)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	// Get user info
	userInfo, err := a.getUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return &ConnectResult{
		AccessToken:       tokenResp.AccessToken,
		RefreshToken:      tokenResp.RefreshToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		ExternalAccountID: userInfo.id,
		AccountName:       userInfo.username,
		AvatarURL:         userInfo.profileImageURL,
		Metadata: map[string]any{
			"id":       userInfo.id,
			"username": userInfo.username,
		},
	}, nil
}

func (a *TwitterAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("twitter requires OAuth flow with your own API credentials (Native mode only)")
}

// Post creates a tweet, optionally attaching uploaded media. Twitter's
// /2/tweets endpoint accepts up to 4 media_ids; we upload each item via
// /2/media/upload (chunked for video, simple for images) and pass the IDs
// through. Mixed image+video is not allowed by Twitter — the call will fail
// at attach time if a caller tries it.
//
// Threading: when opts["in_reply_to_tweet_id"] is set, the tweet is
// posted as a reply to the given ID. The handler chains tweets in a
// thread by passing the previous tweet's external_id through this
// option (Sprint 2 PR6 — keeps the adapter interface stable).
func (a *TwitterAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	var mediaIDs []string
	if len(media) > 0 {
		if len(media) > 4 {
			media = media[:4]
		}
		for _, item := range media {
			id, err := a.uploadMedia(ctx, accessToken, item)
			if err != nil {
				return nil, fmt.Errorf("twitter media upload: %w", err)
			}
			mediaIDs = append(mediaIDs, id)
		}
	}

	payload := map[string]any{"text": text}
	if len(mediaIDs) > 0 {
		payload["media"] = map[string]any{"media_ids": mediaIDs}
	}
	// Threading: chain to a previous tweet via the v2 reply object.
	// The handler passes us the previous tweet's external_id; we
	// just pass it through to Twitter unchanged.
	if replyTo, ok := opts["in_reply_to_tweet_id"].(string); ok && replyTo != "" {
		payload["reply"] = map[string]any{"in_reply_to_tweet_id": replyTo}
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.x.com/2/tweets", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create tweet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tweet failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return &PostResult{
		ExternalID: result.Data.ID,
		URL:        fmt.Sprintf("https://x.com/i/status/%s", result.Data.ID),
	}, nil
}

// uploadMedia uploads a single media item to X/Twitter and returns the
// media_id_string. Routes to the chunked path for video and to the simple
// multipart upload for images. Both use the v2 endpoint at
// https://api.x.com/2/media/upload.
func (a *TwitterAdapter) uploadMedia(ctx context.Context, accessToken string, item MediaItem) (string, error) {
	// Fetch the source bytes once.
	srcReq, err := http.NewRequestWithContext(ctx, "GET", item.URL, nil)
	if err != nil {
		return "", err
	}
	srcResp, err := a.client.Do(srcReq)
	if err != nil {
		return "", fmt.Errorf("fetch source: %w", err)
	}
	defer srcResp.Body.Close()
	if srcResp.StatusCode/100 != 2 {
		return "", fmt.Errorf("fetch source (%d)", srcResp.StatusCode)
	}
	data, err := io.ReadAll(srcResp.Body)
	if err != nil {
		return "", fmt.Errorf("read source: %w", err)
	}

	contentType := srcResp.Header.Get("Content-Type")
	kind := item.Kind
	if kind == MediaKindUnknown {
		kind = SniffMediaKind(item.URL)
	}

	if kind == MediaKindVideo {
		return a.uploadMediaChunked(ctx, accessToken, data, contentType, "tweet_video")
	}
	if kind == MediaKindGIF {
		return a.uploadMediaChunked(ctx, accessToken, data, "image/gif", "tweet_gif")
	}
	return a.uploadMediaSimple(ctx, accessToken, data, contentType)
}

// uploadMediaSimple performs a single multipart POST to /2/media/upload for
// small images. Twitter accepts up to 5 MB per image on this path.
func (a *TwitterAdapter) uploadMediaSimple(ctx context.Context, accessToken string, data []byte, contentType string) (string, error) {
	if contentType == "" {
		contentType = "image/jpeg"
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("media", "upload")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.x.com/2/media/upload", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("media upload (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		MediaIDString string `json:"media_id_string"`
	}
	json.Unmarshal(body, &result)
	if result.Data.ID != "" {
		return result.Data.ID, nil
	}
	if result.MediaIDString != "" {
		return result.MediaIDString, nil
	}
	return "", fmt.Errorf("media upload: empty media_id in response: %s", string(body))
}

// uploadMediaChunked performs the INIT/APPEND/FINALIZE/STATUS dance required
// for video and GIF uploads. mediaCategory is the X-side category string
// (tweet_video or tweet_gif).
func (a *TwitterAdapter) uploadMediaChunked(ctx context.Context, accessToken string, data []byte, contentType, mediaCategory string) (string, error) {
	if contentType == "" {
		contentType = "video/mp4"
	}

	// INIT.
	initParams := url.Values{
		"command":        {"INIT"},
		"total_bytes":    {fmt.Sprintf("%d", len(data))},
		"media_type":     {contentType},
		"media_category": {mediaCategory},
	}
	initReq, _ := http.NewRequestWithContext(ctx, "POST",
		"https://api.x.com/2/media/upload?"+initParams.Encode(), nil)
	initReq.Header.Set("Authorization", "Bearer "+accessToken)
	initResp, err := a.client.Do(initReq)
	if err != nil {
		return "", fmt.Errorf("INIT: %w", err)
	}
	defer initResp.Body.Close()
	initBody, _ := io.ReadAll(initResp.Body)
	if initResp.StatusCode/100 != 2 {
		return "", fmt.Errorf("INIT (%d): %s", initResp.StatusCode, string(initBody))
	}
	var initResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		MediaIDString string `json:"media_id_string"`
	}
	json.Unmarshal(initBody, &initResult)
	mediaID := initResult.Data.ID
	if mediaID == "" {
		mediaID = initResult.MediaIDString
	}
	if mediaID == "" {
		return "", fmt.Errorf("INIT: no media_id: %s", string(initBody))
	}

	// APPEND in 4 MB chunks.
	const chunkSize = 4 * 1024 * 1024
	for i, segment := 0, 0; i < len(data); i, segment = i+chunkSize, segment+1 {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("command", "APPEND")
		_ = mw.WriteField("media_id", mediaID)
		_ = mw.WriteField("segment_index", fmt.Sprintf("%d", segment))
		part, _ := mw.CreateFormFile("media", "chunk")
		part.Write(data[i:end])
		mw.Close()

		req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.x.com/2/media/upload", &buf)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := a.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("APPEND segment %d: %w", segment, err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return "", fmt.Errorf("APPEND segment %d (%d): %s", segment, resp.StatusCode, string(respBody))
		}
	}

	// FINALIZE.
	finParams := url.Values{
		"command":  {"FINALIZE"},
		"media_id": {mediaID},
	}
	finReq, _ := http.NewRequestWithContext(ctx, "POST",
		"https://api.x.com/2/media/upload?"+finParams.Encode(), nil)
	finReq.Header.Set("Authorization", "Bearer "+accessToken)
	finResp, err := a.client.Do(finReq)
	if err != nil {
		return "", fmt.Errorf("FINALIZE: %w", err)
	}
	defer finResp.Body.Close()
	finBody, _ := io.ReadAll(finResp.Body)
	if finResp.StatusCode/100 != 2 {
		return "", fmt.Errorf("FINALIZE (%d): %s", finResp.StatusCode, string(finBody))
	}

	type processingInfo struct {
		State          string `json:"state"`
		CheckAfterSecs int    `json:"check_after_secs"`
		Error          *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	var finResult struct {
		Data struct {
			ProcessingInfo *processingInfo `json:"processing_info"`
		} `json:"data"`
		ProcessingInfo *processingInfo `json:"processing_info"`
	}
	json.Unmarshal(finBody, &finResult)

	pi := finResult.Data.ProcessingInfo
	if pi == nil {
		pi = finResult.ProcessingInfo
	}
	// Poll STATUS until processing finishes (cap at 60 s — caller's HTTP
	// timeout will eventually clip us anyway).
	deadline := time.Now().Add(60 * time.Second)
	for pi != nil && (pi.State == "pending" || pi.State == "in_progress") && time.Now().Before(deadline) {
		wait := time.Duration(pi.CheckAfterSecs) * time.Second
		if wait < time.Second {
			wait = time.Second
		}
		time.Sleep(wait)

		statusParams := url.Values{
			"command":  {"STATUS"},
			"media_id": {mediaID},
		}
		statusReq, _ := http.NewRequestWithContext(ctx, "GET",
			"https://api.x.com/2/media/upload?"+statusParams.Encode(), nil)
		statusReq.Header.Set("Authorization", "Bearer "+accessToken)
		statusResp, err := a.client.Do(statusReq)
		if err != nil {
			return "", fmt.Errorf("STATUS: %w", err)
		}
		statusBody, _ := io.ReadAll(statusResp.Body)
		statusResp.Body.Close()

		var statusResult struct {
			Data struct {
				ProcessingInfo *processingInfo `json:"processing_info"`
			} `json:"data"`
		}
		json.Unmarshal(statusBody, &statusResult)
		pi = statusResult.Data.ProcessingInfo
		if pi != nil && pi.Error != nil {
			return "", fmt.Errorf("media processing failed: %s", pi.Error.Message)
		}
	}
	if pi != nil && pi.State != "" && pi.State != "succeeded" {
		return "", fmt.Errorf("media still %s after polling timeout", pi.State)
	}

	// Suppress the "strings" import warning by using it in a no-op format
	// path; the field tags above already pull it in for json, but we keep an
	// explicit reference to make the import obvious to readers.
	_ = strings.TrimSpace
	return mediaID, nil
}

func (a *TwitterAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", "https://api.x.com/2/tweets/"+externalID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete tweet failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *TwitterAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.x.com/2/oauth2/token", bytes.NewBufferString(data.Encode()))
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
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	newRefresh := tokenResp.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken
	}

	return tokenResp.AccessToken, newRefresh, time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second), nil
}

// GetAnalytics fetches tweet metrics from X/Twitter API v2.
func (a *TwitterAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.x.com/2/tweets/"+externalID+"?tweet.fields=public_metrics", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &PostMetrics{}, nil
	}

	var result struct {
		Data struct {
			PublicMetrics struct {
				ImpressionCount int64 `json:"impression_count"`
				LikeCount       int64 `json:"like_count"`
				ReplyCount      int64 `json:"reply_count"`
				RetweetCount    int64 `json:"retweet_count"`
				QuoteCount      int64 `json:"quote_count"`
				BookmarkCount   int64 `json:"bookmark_count"`
			} `json:"public_metrics"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	pm := result.Data.PublicMetrics
	// EngagementRate is computed by the analytics handler.
	return &PostMetrics{
		Impressions: pm.ImpressionCount,
		Views:       pm.ImpressionCount, // legacy alias
		Likes:       pm.LikeCount,
		Comments:    pm.ReplyCount,
		Shares:      pm.RetweetCount + pm.QuoteCount,
		Saves:       pm.BookmarkCount,
	}, nil
}

type twitterUserInfo struct {
	id              string
	username        string
	profileImageURL string
}

func (a *TwitterAdapter) getUserInfo(ctx context.Context, accessToken string) (*twitterUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.x.com/2/users/me?user.fields=profile_image_url", nil)
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
			ID              string `json:"id"`
			Username        string `json:"username"`
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return &twitterUserInfo{
		id:              result.Data.ID,
		username:        result.Data.Username,
		profileImageURL: result.Data.ProfileImageURL,
	}, nil
}
