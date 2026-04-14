package platform

import (
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
)

type ThreadsAdapter struct {
	client *http.Client
}

func NewThreadsAdapter() *ThreadsAdapter {
	return &ThreadsAdapter{client: &http.Client{Timeout: 60 * time.Second}}
}

func (a *ThreadsAdapter) Platform() string { return "threads" }

func (a *ThreadsAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("THREADS_APP_ID"),
		ClientSecret: os.Getenv("THREADS_APP_SECRET"),
		AuthURL:      "https://threads.net/oauth/authorize",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/threads",
		Scopes:       []string{"threads_basic", "threads_content_publish", "threads_manage_replies", "threads_manage_insights", "threads_read_replies"},
	}
}

func (a *ThreadsAdapter) GetAuthURL(config OAuthConfig, state string) string {
	return BuildAuthURL(config.AuthURL, config.ClientID, config.RedirectURL, state, config.Scopes)
}

func (a *ThreadsAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	data := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {config.RedirectURL},
		"code":          {code},
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
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		UserID      string `json:"user_id"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	// Exchange for long-lived token
	longToken, expiresIn, err := a.exchangeForLongLivedToken(ctx, config, tokenResp.AccessToken)
	if err != nil {
		longToken = tokenResp.AccessToken
		expiresIn = 3600
	}

	// Get profile
	profile, err := a.getProfile(ctx, longToken, tokenResp.UserID)
	if err != nil {
		profile = &threadsProfile{id: tokenResp.UserID, username: tokenResp.UserID}
	}

	return &ConnectResult{
		AccessToken:       longToken,
		RefreshToken:      longToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		ExternalAccountID: profile.id,
		AccountName:       profile.username,
		AvatarURL:         profile.profilePicURL,
		Metadata: map[string]any{
			"threads_user_id": profile.id,
			"username":        profile.username,
		},
	}, nil
}

func (a *ThreadsAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("threads requires OAuth flow, use /v1/oauth/connect/threads")
}

// Post publishes a text post (with optional image) to Threads.
func (a *ThreadsAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	_ = opts
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if len(media) > 20 {
		return nil, fmt.Errorf("threads carousels accept at most 20 items")
	}

	// Threads container shape rules:
	//   - 0 items   → media_type=TEXT
	//   - 1 image   → media_type=IMAGE, image_url
	//   - 1 video   → media_type=VIDEO, video_url
	//   - 2+ items  → media_type=CAROUSEL, children=[...] where each child is
	//                 created up-front with is_carousel_item=true.
	var creationID string
	switch {
	case len(media) == 0:
		creationID, err = a.createTextContainer(ctx, accessToken, userID, text)
	case len(media) == 1:
		creationID, err = a.createMediaContainer(ctx, accessToken, userID, text, media[0], false)
	default:
		creationID, err = a.createCarouselContainer(ctx, accessToken, userID, text, media)
	}
	if err != nil {
		return nil, err
	}

	// Wait at least 30s for video, less for image/text. Threads recommends
	// 30s for video before calling threads_publish.
	hasVideo := false
	for _, m := range media {
		if m.Kind == MediaKindVideo {
			hasVideo = true
			break
		}
	}
	if hasVideo {
		time.Sleep(30 * time.Second)
	} else {
		time.Sleep(3 * time.Second)
	}

	publishURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads_publish?creation_id=%s&access_token=%s",
		userID, creationID, accessToken)
	pubResp, err := a.client.Post(publishURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to publish: %w", err)
	}
	defer pubResp.Body.Close()

	if pubResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pubResp.Body)
		return nil, fmt.Errorf("publish failed (%d): %s", pubResp.StatusCode, string(body))
	}

	var published struct {
		ID string `json:"id"`
	}
	json.NewDecoder(pubResp.Body).Decode(&published)

	return &PostResult{
		ExternalID: published.ID,
		URL:        fmt.Sprintf("https://www.threads.net/@%s/post/%s", userID, published.ID),
	}, nil
}

func (a *ThreadsAdapter) createTextContainer(ctx context.Context, accessToken, userID, text string) (string, error) {
	params := url.Values{
		"media_type":   {"TEXT"},
		"text":         {text},
		"access_token": {accessToken},
	}
	return a.postContainer(ctx, userID, params)
}

func (a *ThreadsAdapter) createMediaContainer(ctx context.Context, accessToken, userID, text string, item MediaItem, isCarouselChild bool) (string, error) {
	params := url.Values{
		"access_token": {accessToken},
	}
	if !isCarouselChild {
		params.Set("text", text)
	} else {
		params.Set("is_carousel_item", "true")
	}

	kind := item.Kind
	if kind == MediaKindUnknown {
		kind = SniffMediaKind(item.URL)
	}

	switch kind {
	case MediaKindVideo:
		params.Set("media_type", "VIDEO")
		params.Set("video_url", item.URL)
	default:
		params.Set("media_type", "IMAGE")
		params.Set("image_url", item.URL)
	}
	return a.postContainer(ctx, userID, params)
}

func (a *ThreadsAdapter) createCarouselContainer(ctx context.Context, accessToken, userID, text string, items []MediaItem) (string, error) {
	childIDs := make([]string, 0, len(items))
	for _, item := range items {
		id, err := a.createMediaContainer(ctx, accessToken, userID, text, item, true)
		if err != nil {
			return "", err
		}
		childIDs = append(childIDs, id)
	}
	// Children need a moment to finish processing, especially videos.
	time.Sleep(5 * time.Second)

	params := url.Values{
		"access_token": {accessToken},
		"text":         {text},
		"media_type":   {"CAROUSEL"},
		"children":     {strings.Join(childIDs, ",")},
	}
	return a.postContainer(ctx, userID, params)
}

func (a *ThreadsAdapter) postContainer(ctx context.Context, userID string, params url.Values) (string, error) {
	containerURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads?%s", userID, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "POST", containerURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("container creation failed (%d): %s", resp.StatusCode, string(body))
	}

	var container struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", err
	}
	return container.ID, nil
}

// FetchRecentMedia returns the IDs of the account's recent Threads posts
// directly from the API. Covers posts published natively, not just
// those published through UniPost.
func (a *ThreadsAdapter) FetchRecentMedia(ctx context.Context, accessToken string) ([]string, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads?fields=id&limit=10&access_token=%s",
		userID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads fetch recent media: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads fetch recent media %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// FetchComments returns replies on a Threads post.
// GET /v1.0/{post-id}/replies?fields=id,text,username,timestamp
func (a *ThreadsAdapter) FetchComments(ctx context.Context, accessToken string, postExternalID string) ([]InboxEntry, error) {
	u := fmt.Sprintf("https://graph.threads.net/v1.0/%s/replies?fields=id,text,username,timestamp&access_token=%s",
		postExternalID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads fetch replies: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("threads fetch replies failed",
			"status", resp.StatusCode,
			"post_id", postExternalID,
			"body", string(body))
		return nil, fmt.Errorf("threads fetch replies %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Username  string `json:"username"`
			Timestamp string `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("threads fetch replies decode: %w", err)
	}

	entries := make([]InboxEntry, 0, len(result.Data))
	for _, r := range result.Data {
		ts, _ := time.Parse(time.RFC3339, r.Timestamp)
		entries = append(entries, InboxEntry{
			ExternalID:       r.ID,
			ParentExternalID: postExternalID,
			AuthorName:       r.Username,
			Body:             r.Text,
			Timestamp:        ts,
			Source:            "threads_reply",
		})
	}
	return entries, nil
}

// ReplyToComment publishes a reply to a Threads post using the
// two-step container flow with reply_to_id.
func (a *ThreadsAdapter) ReplyToComment(ctx context.Context, accessToken string, replyToExternalID string, text string) (*PostResult, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// Step 1: create a TEXT container with reply_to_id
	params := url.Values{
		"media_type":   {"TEXT"},
		"text":         {text},
		"reply_to_id":  {replyToExternalID},
		"access_token": {accessToken},
	}
	containerID, err := a.postContainer(ctx, userID, params)
	if err != nil {
		return nil, fmt.Errorf("threads reply container: %w", err)
	}

	// Step 2: publish
	time.Sleep(3 * time.Second)
	publishURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads_publish?creation_id=%s&access_token=%s",
		userID, containerID, accessToken)
	pubResp, err := a.client.Post(publishURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return nil, fmt.Errorf("threads reply publish: %w", err)
	}
	defer pubResp.Body.Close()
	body, _ := io.ReadAll(pubResp.Body)
	if pubResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads reply publish %d: %s", pubResp.StatusCode, string(body))
	}

	var published struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &published)
	return &PostResult{ExternalID: published.ID}, nil
}

func (a *ThreadsAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	return fmt.Errorf("threads does not support post deletion via API")
}

func (a *ThreadsAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	params := url.Values{
		"grant_type":   {"th_refresh_token"},
		"access_token": {refreshToken},
	}

	resp, err := a.client.Get("https://graph.threads.net/refresh_access_token?" + params.Encode())
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", time.Time{}, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return result.AccessToken, result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn) * time.Second), nil
}

// GetAnalytics fetches post metrics from Threads Insights API.
func (a *ThreadsAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	insightsURL := fmt.Sprintf(
		"https://graph.threads.net/v1.0/%s/insights?metric=views,likes,replies,reposts,quotes&access_token=%s",
		externalID, accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", insightsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &PostMetrics{}, nil
	}

	var result struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value int64 `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	m := &PostMetrics{}
	for _, metric := range result.Data {
		val := int64(0)
		if len(metric.Values) > 0 {
			val = metric.Values[0].Value
		}
		switch metric.Name {
		case "views":
			// Threads "views" represents post impressions (text + media), not
			// just video plays. Map to Impressions and keep legacy Views alias.
			m.Impressions = val
			m.Views = val
		case "likes":
			m.Likes = val
		case "replies":
			m.Comments = val
		case "reposts":
			m.Shares = val
		case "quotes":
			m.Shares += val
		}
	}

	// EngagementRate is computed by the analytics handler.
	return m, nil
}

func (a *ThreadsAdapter) exchangeForLongLivedToken(_ context.Context, config OAuthConfig, shortToken string) (string, int, error) {
	params := url.Values{
		"grant_type":   {"th_exchange_token"},
		"client_secret": {config.ClientSecret},
		"access_token": {shortToken},
	}

	resp, err := a.client.Get("https://graph.threads.net/access_token?" + params.Encode())
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("empty access token")
	}

	return result.AccessToken, result.ExpiresIn, nil
}

type threadsProfile struct {
	id            string
	username      string
	profilePicURL string
}

func (a *ThreadsAdapter) getProfile(ctx context.Context, accessToken string, userID string) (*threadsProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://graph.threads.net/v1.0/%s?fields=id,username,threads_profile_picture_url&access_token=%s", userID, accessToken), nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var profile struct {
		ID                string `json:"id"`
		Username          string `json:"username"`
		ProfilePictureURL string `json:"threads_profile_picture_url"`
	}
	json.NewDecoder(resp.Body).Decode(&profile)

	return &threadsProfile{
		id:            profile.ID,
		username:      profile.Username,
		profilePicURL: profile.ProfilePictureURL,
	}, nil
}

func (a *ThreadsAdapter) getUserID(ctx context.Context, accessToken string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://graph.threads.net/v1.0/me?fields=id&access_token="+accessToken, nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ID == "" {
		return "", fmt.Errorf("failed to get Threads user ID")
	}
	return result.ID, nil
}
