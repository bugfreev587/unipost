package platform

import (
	"bytes"
	"context"
	"encoding/base64"
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

const (
	pinterestOAuthEndpoint = "https://www.pinterest.com/oauth/"
	pinterestTokenEndpoint = "https://api.pinterest.com/v5/oauth/token"
	pinterestAPIBase       = "https://api.pinterest.com/v5"
	pinterestSandboxAPIBase = "https://api-sandbox.pinterest.com/v5"
)

var pinterestScopes = []string{
	"boards:read",
	"boards:write",
	"pins:read",
	"pins:write",
	"user_accounts:read",
}

type PinterestAdapter struct {
	client *http.Client
}

type PinterestBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewPinterestAdapter() *PinterestAdapter {
	return &PinterestAdapter{client: debugrt.NewClient(60 * time.Second)}
}

func (a *PinterestAdapter) Platform() string { return "pinterest" }

func (a *PinterestAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	clientID := os.Getenv("PINTEREST_APP_ID")
	if clientID == "" {
		clientID = os.Getenv("PINTEREST_CLIENT_ID")
	}
	clientSecret := os.Getenv("PINTEREST_APP_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("PINTEREST_CLIENT_SECRET")
	}
	return OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      pinterestAuthURL(),
		TokenURL:     pinterestTokenURL(),
		RedirectURL:  strings.TrimRight(baseRedirectURL, "/") + "/v1/oauth/callback/pinterest",
		Scopes:       pinterestScopes,
	}
}

func (a *PinterestAdapter) GetAuthURL(config OAuthConfig, state string) string {
	q := url.Values{}
	q.Set("consumer_id", config.ClientID)
	q.Set("redirect_uri", config.RedirectURL)
	q.Set("response_type", "code")
	q.Set("refreshable", "true")
	q.Set("scope", strings.Join(config.Scopes, ","))
	q.Set("state", state)
	return config.AuthURL + "?" + q.Encode()
}

func (a *PinterestAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", config.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(config.ClientID+":"+config.ClientSecret)))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest token exchange (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("pinterest token exchange decode: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("pinterest token exchange returned empty access_token")
	}

	profile, err := a.fetchUserAccount(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}

	return &ConnectResult{
		AccessToken:       tokenResp.AccessToken,
		RefreshToken:      tokenResp.RefreshToken,
		TokenExpiresAt:    expiresAt,
		ExternalAccountID: profile.ID,
		AccountName:       firstNonEmptyString(profile.Username, profile.AccountType, profile.ID),
		AvatarURL:         profile.ProfileImage,
		Metadata: map[string]any{
			"username":     profile.Username,
			"account_type": profile.AccountType,
		},
	}, nil
}

func (a *PinterestAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("pinterest requires OAuth flow, use /v1/oauth/connect/pinterest")
}

func (a *PinterestAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	if len(media) != 1 {
		return nil, fmt.Errorf("pinterest requires exactly one image or video")
	}

	boardID := strings.TrimSpace(optString(opts, "board_id"))
	if boardID == "" {
		boardID = strings.TrimSpace(optString(opts, "boardId"))
	}
	if boardID == "" {
		return nil, fmt.Errorf("pinterest requires platform_options.board_id")
	}

	title := strings.TrimSpace(optString(opts, "title"))
	link := strings.TrimSpace(optString(opts, "link"))
	item := media[0]
	kind := item.Kind
	if kind == MediaKindUnknown {
		kind = SniffMediaKind(item.URL)
	}

	reqBody := map[string]any{
		"board_id":    boardID,
		"title":       title,
		"description": text,
	}
	if link != "" {
		reqBody["link"] = link
	}
	switch kind {
	case MediaKindVideo:
		reqBody["media_source"] = map[string]any{
			"source_type": "video_url",
			"url":         item.URL,
		}
	default:
		reqBody["media_source"] = map[string]any{
			"source_type": "image_url",
			"url":         item.URL,
		}
	}

	payload, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", pinterestAPIBaseURL()+"/pins", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest create pin: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("pinterest create pin (%d): %s", resp.StatusCode, string(body))
	}

	var pin struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &pin); err != nil {
		return nil, fmt.Errorf("pinterest create pin decode: %w", err)
	}
	if pin.URL == "" && pin.ID != "" {
		pin.URL = "https://www.pinterest.com/pin/" + pin.ID + "/"
	}
	return &PostResult{
		ExternalID: pin.ID,
		URL:        pin.URL,
	}, nil
}

func (a *PinterestAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", pinterestAPIBaseURL()+"/pins/"+url.PathEscape(externalID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("pinterest delete pin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pinterest delete pin (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *PinterestAdapter) RefreshToken(ctx context.Context, refreshToken string) (newAccess, newRefresh string, expiresAt time.Time, err error) {
	clientID := os.Getenv("PINTEREST_APP_ID")
	if clientID == "" {
		clientID = os.Getenv("PINTEREST_CLIENT_ID")
	}
	clientSecret := os.Getenv("PINTEREST_APP_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("PINTEREST_CLIENT_SECRET")
	}
	if clientID == "" || clientSecret == "" {
		return "", "", time.Time{}, fmt.Errorf("pinterest client credentials not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", pinterestTokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token decode: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token returned empty access_token")
	}
	if tokenResp.RefreshToken == "" {
		tokenResp.RefreshToken = refreshToken
	}

	expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	return tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt, nil
}

func (a *PinterestAdapter) FetchBoards(ctx context.Context, accessToken string) ([]PinterestBoard, error) {
	boards := []PinterestBoard{}
	bookmark := ""

	for {
		q := url.Values{}
		q.Set("page_size", "250")
		if bookmark != "" {
			q.Set("bookmark", bookmark)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", pinterestAPIBaseURL()+"/boards?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("pinterest list boards: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("pinterest list boards (%d): %s", resp.StatusCode, string(body))
		}

		var raw struct {
			Items    []PinterestBoard `json:"items"`
			Bookmark string           `json:"bookmark"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("pinterest list boards decode: %w", err)
		}
		boards = append(boards, raw.Items...)
		if raw.Bookmark == "" || len(raw.Items) == 0 {
			break
		}
		bookmark = raw.Bookmark
	}

	return boards, nil
}

type pinterestUserAccount struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	AccountType  string `json:"account_type"`
	ProfileImage string `json:"profile_image"`
}

func (a *PinterestAdapter) fetchUserAccount(ctx context.Context, accessToken string) (*pinterestUserAccount, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pinterestAPIBaseURL()+"/user_account", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest user_account: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest user_account (%d): %s", resp.StatusCode, string(body))
	}

	var raw pinterestUserAccount
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pinterest user_account decode: %w", err)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("pinterest user_account returned empty id")
	}
	return &raw, nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func pinterestAuthURL() string {
	if v := strings.TrimSpace(os.Getenv("PINTEREST_AUTH_URL")); v != "" {
		return v
	}
	return pinterestOAuthEndpoint
}

func pinterestTokenURL() string {
	if v := strings.TrimSpace(os.Getenv("PINTEREST_TOKEN_URL")); v != "" {
		return v
	}
	if pinterestUseSandbox() {
		return pinterestSandboxAPIBase + "/oauth/token"
	}
	return pinterestTokenEndpoint
}

func pinterestAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("PINTEREST_API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if pinterestUseSandbox() {
		return pinterestSandboxAPIBase
	}
	return pinterestAPIBase
}

func pinterestUseSandbox() bool {
	v := strings.TrimSpace(os.Getenv("PINTEREST_USE_SANDBOX"))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
