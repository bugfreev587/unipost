package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// DefaultOAuthConfig — Twitter has no Quickstart mode, returns empty config.
// Native credentials must be provided via platform_credentials.
func (a *TwitterAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		AuthURL:     "https://twitter.com/i/oauth2/authorize",
		TokenURL:    "https://api.x.com/2/oauth2/token",
		RedirectURL: baseRedirectURL + "/v1/oauth/callback/twitter",
		Scopes:      []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
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
	data := url.Values{
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"client_id":     {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"code_verifier": {""}, // Simplified PKCE
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

// Post creates a tweet.
func (a *TwitterAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string) (*PostResult, error) {
	body, _ := json.Marshal(map[string]any{
		"text": text,
	})

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
	total := pm.LikeCount + pm.ReplyCount + pm.RetweetCount
	var engRate float64
	if pm.ImpressionCount > 0 {
		engRate = float64(total) / float64(pm.ImpressionCount)
	}

	return &PostMetrics{
		Views:          pm.ImpressionCount,
		Likes:          pm.LikeCount,
		Comments:       pm.ReplyCount,
		Shares:         pm.RetweetCount + pm.QuoteCount,
		Impressions:    pm.ImpressionCount,
		EngagementRate: engRate,
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
