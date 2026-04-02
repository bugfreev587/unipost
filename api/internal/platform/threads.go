package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
		ClientID:     os.Getenv("META_APP_ID"),
		ClientSecret: os.Getenv("META_APP_SECRET"),
		AuthURL:      "https://threads.net/oauth/authorize",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/threads",
		Scopes:       []string{"threads_basic", "threads_content_publish"},
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
func (a *ThreadsAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string) (*PostResult, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// Step 1: Create container
	params := url.Values{
		"text":         {text},
		"access_token": {accessToken},
	}

	mediaType := "TEXT"
	if len(imageURLs) > 0 {
		mediaType = "IMAGE"
		params.Set("image_url", imageURLs[0])
	}
	params.Set("media_type", mediaType)

	containerURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads?%s", userID, params.Encode())
	resp, err := a.client.Post(containerURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("container creation failed (%d): %s", resp.StatusCode, string(body))
	}

	var container struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&container)

	// Wait briefly for processing
	time.Sleep(3 * time.Second)

	// Step 2: Publish
	publishURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads_publish?creation_id=%s&access_token=%s",
		userID, container.ID, accessToken)
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

func (a *ThreadsAdapter) exchangeForLongLivedToken(ctx context.Context, config OAuthConfig, shortToken string) (string, int, error) {
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
