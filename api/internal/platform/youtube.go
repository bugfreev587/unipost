package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type YouTubeAdapter struct {
	client *http.Client
}

func NewYouTubeAdapter() *YouTubeAdapter {
	return &YouTubeAdapter{client: &http.Client{Timeout: 120 * time.Second}}
}

func (a *YouTubeAdapter) Platform() string { return "youtube" }

func (a *YouTubeAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("YOUTUBE_CLIENT_ID"),
		ClientSecret: os.Getenv("YOUTUBE_CLIENT_SECRET"),
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/youtube",
		Scopes:       []string{"https://www.googleapis.com/auth/youtube.upload", "https://www.googleapis.com/auth/youtube.readonly"},
	}
}

func (a *YouTubeAdapter) GetAuthURL(config OAuthConfig, state string) string {
	params := url.Values{
		"client_id":     {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly"},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	return config.AuthURL + "?" + params.Encode()
}

func (a *YouTubeAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"redirect_uri":  {config.RedirectURL},
		"grant_type":    {"authorization_code"},
	}

	resp, err := a.client.PostForm(config.TokenURL, data)
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

	// Get channel info
	channel, err := a.getChannel(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}

	return &ConnectResult{
		AccessToken:       tokenResp.AccessToken,
		RefreshToken:      tokenResp.RefreshToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		ExternalAccountID: channel.id,
		AccountName:       channel.title,
		AvatarURL:         channel.thumbnailURL,
		Metadata: map[string]any{
			"channel_id": channel.id,
			"title":      channel.title,
		},
	}, nil
}

func (a *YouTubeAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("youtube requires OAuth flow, use /v1/oauth/connect/youtube")
}

// YouTubePrivacyValues is the set of allowed values for opts["privacy_status"].
// Mirrors the YouTube Data API videos.insert status.privacyStatus enum.
var YouTubePrivacyValues = []string{"private", "public", "unlisted"}

// Post uploads a video to YouTube. Requires video URL in imageURLs[0].
//
// Supported opts:
//   - privacy_status: "private" (default), "public", or "unlisted"
func (a *YouTubeAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string, opts map[string]any) (*PostResult, error) {
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("youtube requires a video URL")
	}

	privacyStatus := optString(opts, "privacy_status")
	if err := validateEnum("youtube", "privacy_status", privacyStatus, YouTubePrivacyValues); err != nil {
		return nil, err
	}
	if privacyStatus == "" {
		privacyStatus = "private"
	}

	// Download video
	videoResp, err := a.client.Get(imageURLs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	defer videoResp.Body.Close()

	videoData, err := io.ReadAll(videoResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read video: %w", err)
	}

	// Initialize resumable upload
	metadata, _ := json.Marshal(map[string]any{
		"snippet": map[string]any{
			"title":       text,
			"description": text,
		},
		"status": map[string]any{
			"privacyStatus": privacyStatus,
		},
	})

	initReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status",
		bytes.NewReader(metadata))
	if err != nil {
		return nil, err
	}
	initReq.Header.Set("Authorization", "Bearer "+accessToken)
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	initReq.Header.Set("X-Upload-Content-Length", fmt.Sprintf("%d", len(videoData)))
	initReq.Header.Set("X-Upload-Content-Type", "video/*")

	initResp, err := a.client.Do(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to init upload: %w", err)
	}
	defer initResp.Body.Close()

	if initResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(initResp.Body)
		return nil, fmt.Errorf("youtube upload init failed (%d): %s", initResp.StatusCode, string(body))
	}

	uploadURL := initResp.Header.Get("Location")
	if uploadURL == "" {
		return nil, fmt.Errorf("no upload URL returned")
	}

	// Upload video
	uploadReq, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, bytes.NewReader(videoData))
	if err != nil {
		return nil, err
	}
	uploadReq.Header.Set("Content-Type", "video/*")

	uploadResp, err := a.client.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("failed to upload video: %w", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(uploadResp.Body)
		return nil, fmt.Errorf("youtube upload failed (%d): %s", uploadResp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(uploadResp.Body).Decode(&result)

	return &PostResult{
		ExternalID: result.ID,
		URL:        fmt.Sprintf("https://www.youtube.com/watch?v=%s", result.ID),
	}, nil
}

func (a *YouTubeAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		"https://www.googleapis.com/youtube/v3/videos?id="+externalID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("youtube delete failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *YouTubeAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	config := a.DefaultOAuthConfig("")
	data := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	resp, err := a.client.PostForm(config.TokenURL, data)
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	// Google doesn't always return a new refresh token
	return tokenResp.AccessToken, refreshToken, time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second), nil
}

// GetAnalytics fetches video statistics from YouTube Data API.
func (a *YouTubeAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://www.googleapis.com/youtube/v3/videos?part=statistics&id="+externalID, nil)
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
		Items []struct {
			Statistics struct {
				ViewCount     string `json:"viewCount"`
				LikeCount     string `json:"likeCount"`
				CommentCount  string `json:"commentCount"`
				FavoriteCount string `json:"favoriteCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Items) == 0 {
		return &PostMetrics{}, nil
	}

	s := result.Items[0].Statistics
	views := parseInt64(s.ViewCount)
	// YouTube Data API doesn't expose impressions; EngagementRate is computed
	// by the analytics handler.
	return &PostMetrics{
		VideoViews: views,
		Views:      views, // legacy alias
		Likes:      parseInt64(s.LikeCount),
		Comments:   parseInt64(s.CommentCount),
	}, nil
}

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

type ytChannel struct {
	id           string
	title        string
	thumbnailURL string
}

func (a *YouTubeAdapter) getChannel(ctx context.Context, accessToken string) (*ytChannel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", nil)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get channel (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no YouTube channel found")
	}

	item := result.Items[0]
	return &ytChannel{
		id:           item.ID,
		title:        item.Snippet.Title,
		thumbnailURL: item.Snippet.Thumbnails.Default.URL,
	}, nil
}
