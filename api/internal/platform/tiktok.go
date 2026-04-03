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
		Scopes:       []string{"video.publish", "video.upload", "user.info.basic"},
	}
}

func (a *TikTokAdapter) GetAuthURL(config OAuthConfig, state string) string {
	// TikTok uses client_key instead of client_id
	params := url.Values{
		"client_key":    {config.ClientID},
		"redirect_uri":  {config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"video.publish,video.upload,user.info.basic"},
		"state":         {state},
	}
	return config.AuthURL + "?" + params.Encode()
}

func (a *TikTokAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	body, _ := json.Marshal(map[string]string{
		"client_key":    config.ClientID,
		"client_secret": config.ClientSecret,
		"code":          code,
		"grant_type":    "authorization_code",
		"redirect_uri":  config.RedirectURL,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			OpenID       string `json:"open_id"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	if tokenResp.Error.Code != "" && tokenResp.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok error: %s", tokenResp.Error.Message)
	}

	slog.Info("tiktok token exchange",
		"has_access_token", tokenResp.Data.AccessToken != "",
		"token_length", len(tokenResp.Data.AccessToken),
		"open_id", tokenResp.Data.OpenID,
		"expires_in", tokenResp.Data.ExpiresIn,
	)

	// Get user info
	userInfo, err := a.getUserInfo(ctx, tokenResp.Data.AccessToken)
	if err != nil {
		userInfo = &tiktokUserInfo{openID: tokenResp.Data.OpenID, displayName: tokenResp.Data.OpenID}
	}

	return &ConnectResult{
		AccessToken:       tokenResp.Data.AccessToken,
		RefreshToken:      tokenResp.Data.RefreshToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(tokenResp.Data.ExpiresIn) * time.Second),
		ExternalAccountID: tokenResp.Data.OpenID,
		AccountName:       userInfo.displayName,
		AvatarURL:         userInfo.avatarURL,
		Metadata: map[string]any{
			"open_id":      tokenResp.Data.OpenID,
			"display_name": userInfo.displayName,
		},
	}, nil
}

func (a *TikTokAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("tiktok requires OAuth flow, use /v1/oauth/connect/tiktok")
}

// Post publishes a video to TikTok. Requires video URL in imageURLs[0].
func (a *TikTokAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string) (*PostResult, error) {
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("tiktok requires a video URL")
	}

	// Initialize video upload
	body, _ := json.Marshal(map[string]any{
		"post_info": map[string]any{
			"title":         text,
			"privacy_level": "SELF_ONLY", // Default to private, user can change
		},
		"source_info": map[string]any{
			"source":    "PULL_FROM_URL",
			"video_url": imageURLs[0],
		},
	})

	slog.Info("tiktok post: initiating upload", "token_length", len(accessToken), "token_prefix", accessToken[:min(10, len(accessToken))])

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/post/publish/video/init/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to init upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tiktok upload init failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var initResp struct {
		Data struct {
			PublishID string `json:"publish_id"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&initResp)

	if initResp.Error.Code != "" && initResp.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok error: %s", initResp.Error.Message)
	}

	return &PostResult{
		ExternalID: initResp.Data.PublishID,
		URL:        "https://www.tiktok.com",
	}, nil
}

func (a *TikTokAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	return fmt.Errorf("tiktok does not support post deletion via API")
}

func (a *TikTokAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	config := a.DefaultOAuthConfig("")
	body, _ := json.Marshal(map[string]string{
		"client_key":    config.ClientID,
		"client_secret": config.ClientSecret,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewReader(body))
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")

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
