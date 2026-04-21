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
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/debugrt"
)

type YouTubeAdapter struct {
	client *http.Client
}

func NewYouTubeAdapter() *YouTubeAdapter {
	return &YouTubeAdapter{client: debugrt.NewClient(120 * time.Second)}
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

var YouTubeLicenseValues = []string{"youtube", "creativeCommon"}

var youTubeCategoryAliases = map[string]string{
	"people & blogs":       "22",
	"science & technology": "28",
	"education":            "27",
	"entertainment":        "24",
	"gaming":               "20",
	"music":                "10",
	"news & politics":      "25",
	"sports":               "17",
}

func youtubeOptString(opts map[string]any, primary string, aliases ...string) string {
	if v := strings.TrimSpace(optString(opts, primary)); v != "" {
		return v
	}
	for _, alias := range aliases {
		if v := strings.TrimSpace(optString(opts, alias)); v != "" {
			return v
		}
	}
	return ""
}

func normalizeYouTubeCategory(value string) string {
	if value == "" {
		return ""
	}
	if alias, ok := youTubeCategoryAliases[strings.ToLower(strings.TrimSpace(value))]; ok {
		return alias
	}
	return strings.TrimSpace(value)
}

func youtubeOptBool(opts map[string]any, key string, fallback bool) bool {
	if opts == nil {
		return fallback
	}
	if _, ok := opts[key]; !ok {
		return fallback
	}
	return optBool(opts, key)
}

func youtubeOptTime(opts map[string]any, key string) (time.Time, error) {
	value := strings.TrimSpace(optString(opts, key))
	if value == "" {
		return time.Time{}, nil
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("youtube: invalid %s %q, expected RFC3339 datetime or YYYY-MM-DD date", key, value)
}

func youtubeOptStringList(opts map[string]any, key string) []string {
	if opts == nil {
		return nil
	}
	rawTags, ok := opts[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rawTags))
	for _, t := range rawTags {
		if s, ok := t.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func (a *YouTubeAdapter) addVideoToPlaylist(ctx context.Context, accessToken, playlistID, videoID string) error {
	body, _ := json.Marshal(map[string]any{
		"snippet": map[string]any{
			"playlistId": playlistID,
			"resourceId": map[string]any{
				"kind":    "youtube#video",
				"videoId": videoID,
			},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.googleapis.com/youtube/v3/playlistItems?part=snippet", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add video to playlist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("youtube playlist insert failed (%d): %s", resp.StatusCode, string(payload))
	}
	return nil
}

// Post uploads a video to YouTube. Requires video URL in imageURLs[0].
//
// Supported opts:
//   - privacy_status: "private" (default), "public", or "unlisted"
func (a *YouTubeAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	if len(media) == 0 {
		return nil, fmt.Errorf("youtube requires a video URL")
	}
	if len(media) > 1 {
		return nil, fmt.Errorf("youtube accepts exactly one video per upload")
	}
	videoURL := media[0].URL

	privacyStatus := youtubeOptString(opts, "privacy_status", "visibility")
	if err := validateEnum("youtube", "privacy_status", privacyStatus, YouTubePrivacyValues); err != nil {
		return nil, err
	}
	if privacyStatus == "" {
		privacyStatus = "private"
	}
	license := youtubeOptString(opts, "license")
	if err := validateEnum("youtube", "license", license, YouTubeLicenseValues); err != nil {
		return nil, err
	}
	if license == "" {
		license = "youtube"
	}

	// Shorts hint: when true, append #Shorts to the title (and description if
	// missing). YouTube uses the hashtag as the primary signal that a vertical
	// short-form video should be surfaced in the Shorts shelf — combined with
	// 9:16 aspect ratio + < 60 s duration, both of which are caller-controlled
	// at the source video level.
	shorts := optBool(opts, "shorts")
	title := youtubeOptString(opts, "title")
	description := text
	defaultLanguage := youtubeOptString(opts, "default_language")
	playlistID := youtubeOptString(opts, "playlist_id")
	publishAt, err := youtubeOptTime(opts, "publish_at")
	if err != nil {
		return nil, err
	}
	recordingDate, err := youtubeOptTime(opts, "recording_date")
	if err != nil {
		return nil, err
	}
	if shorts {
		if !strings.Contains(strings.ToLower(title), "#shorts") {
			title = strings.TrimSpace(title + " #Shorts")
		}
		if !strings.Contains(strings.ToLower(description), "#shorts") {
			description = strings.TrimSpace(description + "\n#Shorts")
		}
	}

	// Optional category id (e.g. "22" for People & Blogs) and tag list.
	categoryID := normalizeYouTubeCategory(youtubeOptString(opts, "category_id", "category"))
	tags := youtubeOptStringList(opts, "tags")
	notifySubscribers := youtubeOptBool(opts, "notify_subscribers", true)
	embeddable := youtubeOptBool(opts, "embeddable", true)
	publicStatsViewable := youtubeOptBool(opts, "public_stats_viewable", true)
	madeForKids := youtubeOptBool(opts, "made_for_kids", false)
	containsSyntheticMedia := youtubeOptBool(opts, "contains_synthetic_media", false)

	// Download video
	videoResp, err := a.client.Get(videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	defer videoResp.Body.Close()

	videoData, err := io.ReadAll(videoResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read video: %w", err)
	}

	// Initialize resumable upload
	snippet := map[string]any{
		"title":       title,
		"description": description,
	}
	if categoryID != "" {
		snippet["categoryId"] = categoryID
	}
	if len(tags) > 0 {
		snippet["tags"] = tags
	}
	if defaultLanguage != "" {
		snippet["defaultLanguage"] = defaultLanguage
	}

	status := map[string]any{
		"privacyStatus":           privacyStatus,
		"license":                 license,
		"embeddable":              embeddable,
		"publicStatsViewable":     publicStatsViewable,
		"selfDeclaredMadeForKids": madeForKids,
		"containsSyntheticMedia":  containsSyntheticMedia,
	}
	if !publishAt.IsZero() {
		status["publishAt"] = publishAt.UTC().Format(time.RFC3339)
	}

	payload := map[string]any{
		"snippet": snippet,
		"status":  status,
	}
	if !recordingDate.IsZero() {
		payload["recordingDetails"] = map[string]any{
			"recordingDate": recordingDate.UTC().Format(time.RFC3339),
		}
	}

	metadata, _ := json.Marshal(payload)

	initReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status,recordingDetails&notifySubscribers=%t", notifySubscribers),
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
	if playlistID != "" {
		if err := a.addVideoToPlaylist(ctx, accessToken, playlistID, result.ID); err != nil {
			return nil, err
		}
	}

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
