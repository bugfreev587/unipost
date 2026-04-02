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

type InstagramAdapter struct {
	client *http.Client
}

func NewInstagramAdapter() *InstagramAdapter {
	return &InstagramAdapter{client: &http.Client{Timeout: 60 * time.Second}}
}

func (a *InstagramAdapter) Platform() string { return "instagram" }

func (a *InstagramAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("INSTAGRAM_APP_ID"),
		ClientSecret: os.Getenv("INSTAGRAM_APP_SECRET"),
		AuthURL:      "https://api.instagram.com/oauth/authorize",
		TokenURL:     "https://api.instagram.com/oauth/access_token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/instagram",
		Scopes:       []string{"instagram_business_basic", "instagram_business_content_publish", "instagram_business_manage_insights"},
	}
}

func (a *InstagramAdapter) GetAuthURL(config OAuthConfig, state string) string {
	return BuildAuthURL(config.AuthURL, config.ClientID, config.RedirectURL, state, config.Scopes)
}

func (a *InstagramAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	// Exchange code for short-lived token
	params := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"redirect_uri":  {config.RedirectURL},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}

	resp, err := a.client.PostForm(config.TokenURL, params)
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
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	// Exchange for long-lived token (60 days)
	longToken, expiresIn, err := a.exchangeForLongLivedToken(ctx, config, tokenResp.AccessToken)
	if err != nil {
		// Fall back to short-lived token
		longToken = tokenResp.AccessToken
		expiresIn = tokenResp.ExpiresIn
	}

	// Get Instagram account info via pages
	igAccount, err := a.getInstagramAccount(ctx, longToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get Instagram account: %w", err)
	}

	return &ConnectResult{
		AccessToken:       longToken,
		RefreshToken:      longToken, // Meta uses token exchange instead of refresh tokens
		TokenExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		ExternalAccountID: igAccount.id,
		AccountName:       igAccount.username,
		AvatarURL:         igAccount.profilePicURL,
		Metadata: map[string]any{
			"ig_user_id": igAccount.id,
			"username":   igAccount.username,
			"page_id":    igAccount.pageID,
		},
	}, nil
}

func (a *InstagramAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("instagram requires OAuth flow, use /v1/oauth/connect/instagram")
}

// Post publishes to Instagram using the two-step container flow.
func (a *InstagramAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string) (*PostResult, error) {
	// Get IG user ID from token
	igUserID, err := a.getIGUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("instagram requires at least one image")
	}

	// Step 1: Create media container
	params := url.Values{
		"image_url":    {imageURLs[0]},
		"caption":      {text},
		"access_token": {accessToken},
	}

	containerURL := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/media?%s", igUserID, params.Encode())
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

	// Wait for container to be ready (poll)
	if err := a.waitForContainer(ctx, accessToken, container.ID); err != nil {
		return nil, err
	}

	// Step 2: Publish container
	publishURL := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/media_publish?creation_id=%s&access_token=%s",
		igUserID, container.ID, accessToken)
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
		URL:        fmt.Sprintf("https://www.instagram.com/p/%s/", published.ID),
	}, nil
}

func (a *InstagramAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	// Instagram API doesn't support deleting posts via API for most apps
	return fmt.Errorf("instagram does not support post deletion via API")
}

func (a *InstagramAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	// Meta uses token exchange for long-lived tokens
	params := url.Values{
		"grant_type":   {"ig_exchange_token"},
		"access_token": {refreshToken},
	}

	resp, err := a.client.Get("https://graph.instagram.com/refresh_access_token?" + params.Encode())
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", time.Time{}, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return result.AccessToken, result.AccessToken, expiresAt, nil
}

func (a *InstagramAdapter) exchangeForLongLivedToken(ctx context.Context, config OAuthConfig, shortToken string) (string, int, error) {
	params := url.Values{
		"grant_type":    {"fb_exchange_token"},
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"fb_exchange_token": {shortToken},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.facebook.com/v21.0/oauth/access_token?"+params.Encode(), nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, err
	}

	return result.AccessToken, result.ExpiresIn, nil
}

type igAccountInfo struct {
	id            string
	username      string
	profilePicURL string
	pageID        string
}

func (a *InstagramAdapter) getInstagramAccount(ctx context.Context, accessToken string) (*igAccountInfo, error) {
	// Get pages, then find connected Instagram account
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.facebook.com/v21.0/me/accounts?fields=id,name,instagram_business_account&access_token="+accessToken, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pages struct {
		Data []struct {
			ID                       string `json:"id"`
			InstagramBusinessAccount struct {
				ID string `json:"id"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&pages)

	for _, page := range pages.Data {
		if page.InstagramBusinessAccount.ID != "" {
			// Get Instagram account details
			igReq, _ := http.NewRequestWithContext(ctx, "GET",
				fmt.Sprintf("https://graph.instagram.com/v21.0/%s?fields=id,username,profile_picture_url&access_token=%s",
					page.InstagramBusinessAccount.ID, accessToken), nil)
			igResp, err := a.client.Do(igReq)
			if err != nil {
				continue
			}
			defer igResp.Body.Close()

			var ig struct {
				ID                string `json:"id"`
				Username          string `json:"username"`
				ProfilePictureURL string `json:"profile_picture_url"`
			}
			json.NewDecoder(igResp.Body).Decode(&ig)

			return &igAccountInfo{
				id:            ig.ID,
				username:      ig.Username,
				profilePicURL: ig.ProfilePictureURL,
				pageID:        page.ID,
			}, nil
		}
	}

	return nil, fmt.Errorf("no Instagram business account found. Make sure your Facebook page has a connected Instagram account")
}

func (a *InstagramAdapter) getIGUserID(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.instagram.com/v21.0/me?fields=id&access_token="+accessToken, nil)
	if err != nil {
		return "", err
	}

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
		return "", fmt.Errorf("failed to get Instagram user ID")
	}
	return result.ID, nil
}

func (a *InstagramAdapter) waitForContainer(ctx context.Context, accessToken string, containerID string) error {
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}

		req, _ := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("https://graph.instagram.com/v21.0/%s?fields=status_code&access_token=%s", containerID, accessToken), nil)
		resp, err := a.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var status struct {
			StatusCode string `json:"status_code"`
		}
		json.NewDecoder(resp.Body).Decode(&status)

		if status.StatusCode == "FINISHED" {
			return nil
		}
		if status.StatusCode == "ERROR" {
			return fmt.Errorf("container processing failed")
		}
	}
	return fmt.Errorf("container processing timed out")
}
