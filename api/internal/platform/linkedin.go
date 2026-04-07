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

type LinkedInAdapter struct {
	client *http.Client
}

func NewLinkedInAdapter() *LinkedInAdapter {
	return &LinkedInAdapter{client: &http.Client{Timeout: 30 * time.Second}}
}

func (a *LinkedInAdapter) Platform() string { return "linkedin" }

func (a *LinkedInAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("LINKEDIN_CLIENT_ID"),
		ClientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET"),
		AuthURL:      "https://www.linkedin.com/oauth/v2/authorization",
		TokenURL:     "https://www.linkedin.com/oauth/v2/accessToken",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/linkedin",
		Scopes:       []string{"openid", "profile", "email", "w_member_social"},
	}
}

func (a *LinkedInAdapter) GetAuthURL(config OAuthConfig, state string) string {
	return BuildAuthURL(config.AuthURL, config.ClientID, config.RedirectURL, state, config.Scopes)
}

func (a *LinkedInAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	// Exchange code for token
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// Get user info
	userInfo, err := a.getUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return &ConnectResult{
		AccessToken:       tokenResp.AccessToken,
		RefreshToken:      tokenResp.RefreshToken,
		TokenExpiresAt:    expiresAt,
		ExternalAccountID: userInfo.sub,
		AccountName:       userInfo.name,
		AvatarURL:         userInfo.picture,
		Metadata: map[string]any{
			"sub":   userInfo.sub,
			"email": userInfo.email,
		},
	}, nil
}

// Connect is not used for OAuth platforms — use the OAuth flow instead.
func (a *LinkedInAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("linkedin requires OAuth flow, use /v1/oauth/connect/linkedin")
}

// Post publishes a text post to LinkedIn.
func (a *LinkedInAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string, opts map[string]any) (*PostResult, error) {
	_ = opts
	// Get person URN from userinfo
	userInfo, err := a.getUserInfo(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	authorURN := "urn:li:person:" + userInfo.sub

	shareContent := map[string]any{
		"shareCommentary": map[string]any{
			"text": text,
		},
		"shareMediaCategory": "NONE",
	}

	// Handle images if provided
	if len(imageURLs) > 0 {
		var mediaList []map[string]any
		for _, imgURL := range imageURLs {
			mediaList = append(mediaList, map[string]any{
				"status": "READY",
				"originalUrl": imgURL,
				"media": map[string]any{
					"title": "",
				},
			})
		}
		shareContent["shareMediaCategory"] = "ARTICLE"
		shareContent["media"] = mediaList
	}

	body, _ := json.Marshal(map[string]any{
		"author":         authorURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": shareContent,
		},
		"visibility": map[string]any{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.linkedin.com/v2/ugcPosts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("linkedin post failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	postID := result.ID
	if postID == "" {
		postID = resp.Header.Get("X-RestLi-Id")
	}

	return &PostResult{
		ExternalID: postID,
		URL:        fmt.Sprintf("https://www.linkedin.com/feed/update/%s", postID),
	}, nil
}

func (a *LinkedInAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", "https://api.linkedin.com/v2/ugcPosts/"+externalID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("linkedin delete failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *LinkedInAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	config := a.DefaultOAuthConfig("")
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", time.Time{}, fmt.Errorf("linkedin refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", time.Time{}, err
	}

	newRefresh := tokenResp.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken
	}

	return tokenResp.AccessToken, newRefresh, time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second), nil
}

// GetAnalytics fetches post metrics from LinkedIn.
func (a *LinkedInAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	// Use the socialMetadata endpoint for UGC posts
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.linkedin.com/v2/socialMetadata/"+url.PathEscape(externalID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &PostMetrics{}, nil
	}

	var result struct {
		TotalShareStatistics struct {
			ShareCount       int64 `json:"shareCount"`
			LikeCount        int64 `json:"likeCount"`
			CommentCount     int64 `json:"commentCount"`
			ImpressionCount  int64 `json:"impressionCount"`
			UniqueImpression int64 `json:"uniqueImpressionsCount"`
			ClickCount       int64 `json:"clickCount"`
		} `json:"totalShareStatistics"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	s := result.TotalShareStatistics
	total := s.LikeCount + s.CommentCount + s.ShareCount
	var engRate float64
	if s.ImpressionCount > 0 {
		engRate = float64(total) / float64(s.ImpressionCount)
	}

	return &PostMetrics{
		Views:          s.ImpressionCount,
		Likes:          s.LikeCount,
		Comments:       s.CommentCount,
		Shares:         s.ShareCount,
		Reach:          s.UniqueImpression,
		Impressions:    s.ImpressionCount,
		EngagementRate: engRate,
	}, nil
}

type linkedInUserInfo struct {
	sub     string
	name    string
	email   string
	picture string
}

func (a *LinkedInAdapter) getUserInfo(ctx context.Context, accessToken string) (*linkedInUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.linkedin.com/v2/userinfo", nil)
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
		return nil, fmt.Errorf("userinfo failed (%d): %s", resp.StatusCode, string(body))
	}

	var info struct {
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &linkedInUserInfo{
		sub:     info.Sub,
		name:    info.Name,
		email:   info.Email,
		picture: info.Picture,
	}, nil
}
