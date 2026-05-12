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
	youtubeAuthorizeEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	youtubeTokenEndpoint     = "https://oauth2.googleapis.com/token"
	youtubeChannelsEndpoint  = "https://www.googleapis.com/youtube/v3/channels"

	youtubeScopes = "https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly"
)

type YouTubeConnector struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	ChannelsEndpoint  string
}

func NewYouTubeConnector(clientID, clientSecret, callbackBaseURL string) *YouTubeConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &YouTubeConnector{
		clientID:           clientID,
		clientSecret:       clientSecret,
		redirectURI:        strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/youtube",
		httpClient:         &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint:  youtubeAuthorizeEndpoint,
		TokenEndpoint:      youtubeTokenEndpoint,
		ChannelsEndpoint:   youtubeChannelsEndpoint,
	}
}

func (c *YouTubeConnector) Platform() string { return "youtube" }

func (c *YouTubeConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("scope", youtubeScopes)
	q.Set("state", session.OAuthState)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	return c.AuthorizeEndpoint + "?" + q.Encode(), nil
}

func (c *YouTubeConnector) ExchangeCode(ctx context.Context, _ SessionView, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.redirectURI)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube token exchange %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("youtube token exchange decode: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("youtube token exchange returned empty access_token: %s", string(body))
	}

	scopes := strings.Fields(raw.Scope)
	if len(scopes) == 0 {
		scopes = strings.Fields(youtubeScopes)
	}

	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       scopes,
	}, nil
}

func (c *YouTubeConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	q := url.Values{}
	q.Set("part", "snippet")
	q.Set("mine", "true")

	req, err := http.NewRequestWithContext(ctx, "GET", c.ChannelsEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube channels.mine: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube channels.mine %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				CustomURL  string `json:"customUrl"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("youtube channels.mine decode: %w", err)
	}
	if len(raw.Items) == 0 || raw.Items[0].ID == "" {
		return nil, fmt.Errorf("youtube account has no channel")
	}

	item := raw.Items[0]
	username := strings.TrimSpace(item.Snippet.CustomURL)
	if username == "" {
		username = item.Snippet.Title
	}

	return &Profile{
		ExternalAccountID: item.ID,
		Username:          username,
		DisplayName:       item.Snippet.Title,
		AvatarURL:         item.Snippet.Thumbnails.Default.URL,
	}, nil
}

func (c *YouTubeConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube refresh %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("youtube refresh empty access_token")
	}
	nextRefresh := raw.RefreshToken
	if nextRefresh == "" {
		nextRefresh = refreshToken
	}

	scopes := strings.Fields(raw.Scope)
	if len(scopes) == 0 {
		scopes = strings.Fields(youtubeScopes)
	}

	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: nextRefresh,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       scopes,
	}, nil
}
