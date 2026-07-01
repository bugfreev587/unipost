package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	tiktokAuthorizeEndpoint = "https://www.tiktok.com/v2/auth/authorize/"
	tiktokTokenEndpoint     = "https://open.tiktokapis.com/v2/oauth/token/"
	tiktokUserInfoEndpoint  = "https://open.tiktokapis.com/v2/user/info/"
)

var tiktokConnectBaseScopes = []string{
	"video.publish",
	"video.upload",
	"user.info.basic",
}

var tiktokConnectAnalyticsScopes = []string{
	"user.info.profile",
	"user.info.stats",
	"video.list",
}

// TikTokConnector is the managed Connect connector for TikTok's OAuth 2 flow.
type TikTokConnector struct {
	clientKey    string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	UserInfoEndpoint  string
}

// NewTikTokConnector returns a ready Connector or nil if either credential is
// missing. TikTok calls the OAuth client id "client_key", but we keep the
// constructor shape aligned with the other connectors.
func NewTikTokConnector(clientKey, clientSecret, callbackBaseURL string) *TikTokConnector {
	if clientKey == "" || clientSecret == "" {
		return nil
	}
	return &TikTokConnector{
		clientKey:         clientKey,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/tiktok",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: tiktokAuthorizeEndpoint,
		TokenEndpoint:     tiktokTokenEndpoint,
		UserInfoEndpoint:  tiktokUserInfoEndpoint,
	}
}

func (c *TikTokConnector) Platform() string { return "tiktok" }

func (c *TikTokConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("client_key", c.clientKey)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(tiktokConnectScopesForSession(session), ","))
	q.Set("state", session.OAuthState)
	return c.AuthorizeEndpoint + "?" + q.Encode(), nil
}

func (c *TikTokConnector) ExchangeCode(ctx context.Context, session SessionView, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("client_key", c.clientKey)
	form.Set("client_secret", c.clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok token exchange %d: %s", resp.StatusCode, string(body))
	}

	tokenData, err := parseTikTokTokenData(body)
	if err != nil {
		return nil, fmt.Errorf("tiktok token exchange decode: %w", err)
	}
	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("tiktok token exchange returned empty access_token: %s", string(body))
	}

	return &TokenSet{
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(defaultInt(tokenData.ExpiresIn, 86400)) * time.Second),
		Scopes:       tiktokConnectScopesForSession(session),
	}, nil
}

func (c *TikTokConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	q := url.Values{}
	q.Set("fields", "open_id,display_name,avatar_url")

	req, err := http.NewRequestWithContext(ctx, "GET", c.UserInfoEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok profile: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok profile %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Data struct {
			User struct {
				OpenID      string `json:"open_id"`
				DisplayName string `json:"display_name"`
				AvatarURL   string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("tiktok profile decode: %w", err)
	}
	if raw.Error.Code != "" && raw.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok profile: %s", raw.Error.Message)
	}
	if raw.Data.User.OpenID == "" {
		return nil, fmt.Errorf("tiktok profile empty open_id: %s", string(body))
	}
	return &Profile{
		ExternalAccountID: raw.Data.User.OpenID,
		Username:          raw.Data.User.DisplayName,
		DisplayName:       raw.Data.User.DisplayName,
		AvatarURL:         raw.Data.User.AvatarURL,
	}, nil
}

func (c *TikTokConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("client_key", c.clientKey)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok refresh: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok refresh %d: %s", resp.StatusCode, string(body))
	}

	tokenData, err := parseTikTokTokenData(body)
	if err != nil {
		return nil, fmt.Errorf("tiktok refresh decode: %w", err)
	}
	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("tiktok refresh returned empty access_token: %s", string(body))
	}
	rotatedRefresh := tokenData.RefreshToken
	if rotatedRefresh == "" {
		rotatedRefresh = refreshToken
	}

	return &TokenSet{
		AccessToken:  tokenData.AccessToken,
		RefreshToken: rotatedRefresh,
		ExpiresAt:    time.Now().Add(time.Duration(defaultInt(tokenData.ExpiresIn, 86400)) * time.Second),
		Scopes:       tiktokConnectScopes(),
	}, nil
}

type tiktokTokenData struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func parseTikTokTokenData(body []byte) (tiktokTokenData, error) {
	var raw struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        any    `json:"error"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return tiktokTokenData{}, err
	}
	if msg := tiktokErrorMessage(raw.Error); msg != "" {
		return tiktokTokenData{}, fmt.Errorf("%s", msg)
	}
	return tiktokTokenData{
		AccessToken:  firstNonEmpty(raw.Data.AccessToken, raw.AccessToken),
		RefreshToken: firstNonEmpty(raw.Data.RefreshToken, raw.RefreshToken),
		ExpiresIn:    defaultInt(raw.Data.ExpiresIn, raw.ExpiresIn),
	}, nil
}

func tiktokErrorMessage(v any) string {
	switch errValue := v.(type) {
	case string:
		return strings.TrimSpace(errValue)
	case map[string]any:
		code, _ := errValue["code"].(string)
		if code == "" || code == "ok" {
			return ""
		}
		msg, _ := errValue["message"].(string)
		if msg != "" {
			return code + ": " + msg
		}
		return code
	default:
		return ""
	}
}

func tiktokConnectScopes() []string {
	return tiktokConnectScopesForSession(SessionView{})
}

func tiktokConnectScopesForSession(session SessionView) []string {
	scopes := append([]string(nil), tiktokConnectBaseScopes...)
	return append(scopes, tiktokConnectAnalyticsScopes...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultInt(value, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}
