// facebook.go implements Connector for Facebook Page hosted Connect.
//
// Facebook's OAuth response represents the Meta user, not a single
// Page. The hosted Connect flow needs to finish with one managed
// social_accounts row, so this connector exchanges the user token for
// Pages and selects the first Page that grants publish permissions.

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
	facebookAuthorizeEndpoint = "https://www.facebook.com/v22.0/dialog/oauth"
	facebookGraphBase         = "https://graph.facebook.com/v22.0"
	facebookTokenEndpoint     = facebookGraphBase + "/oauth/access_token"
	facebookProfileEndpoint   = facebookGraphBase + "/me"
	facebookPagesEndpoint     = facebookGraphBase + "/me/accounts"
)

var facebookConnectScopes = []string{
	"pages_show_list",
	"pages_manage_posts",
	"pages_read_engagement",
	"pages_read_user_content",
	"pages_manage_engagement",
	"pages_messaging",
	"pages_manage_metadata",
}

// FacebookConnector is the Connect Connector for Facebook Pages.
type FacebookConnector struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	ProfileEndpoint   string
	PagesEndpoint     string
}

// NewFacebookConnector returns a ready Connector or nil if either
// credential is missing.
func NewFacebookConnector(clientID, clientSecret, callbackBaseURL string) *FacebookConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &FacebookConnector{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/facebook",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: facebookAuthorizeEndpoint,
		TokenEndpoint:     facebookTokenEndpoint,
		ProfileEndpoint:   facebookProfileEndpoint,
		PagesEndpoint:     facebookPagesEndpoint,
	}
}

func (c *FacebookConnector) Platform() string { return "facebook" }

func (c *FacebookConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(facebookConnectScopes, " "))
	q.Set("state", session.OAuthState)
	return c.AuthorizeEndpoint + "?" + q.Encode(), nil
}

func (c *FacebookConnector) ExchangeCode(ctx context.Context, _ SessionView, code string) (*TokenSet, error) {
	shortToken, err := c.exchangeShortLivedUserToken(ctx, code)
	if err != nil {
		return nil, err
	}
	longToken, err := c.exchangeLongLivedUserToken(ctx, shortToken)
	if err != nil {
		return nil, fmt.Errorf("facebook long-lived exchange: %w", err)
	}
	pages, err := c.fetchPages(ctx, longToken)
	if err != nil {
		return nil, err
	}
	page, ok := firstPublishableFacebookPage(pages)
	if !ok {
		return nil, fmt.Errorf("facebook connect found no Pages with publish permissions")
	}
	if page.AccessToken == "" {
		return nil, fmt.Errorf("facebook connect selected Page has no access_token")
	}
	return &TokenSet{
		AccessToken: page.AccessToken,
		// Page access tokens derived from a long-lived user token do
		// not support refresh. Store no expiry so the managed refresh
		// worker leaves the row alone until the platform rejects it.
		ExpiresAt: time.Time{},
		Scopes:    append([]string(nil), facebookConnectScopes...),
	}, nil
}

func (c *FacebookConnector) exchangeShortLivedUserToken(ctx context.Context, code string) (string, error) {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("client_secret", c.clientSecret)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, "GET", c.TokenEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("facebook token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facebook token exchange %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("facebook token exchange decode: %w", err)
	}
	if raw.AccessToken == "" {
		return "", fmt.Errorf("facebook token exchange returned empty access_token")
	}
	return raw.AccessToken, nil
}

func (c *FacebookConnector) exchangeLongLivedUserToken(ctx context.Context, shortToken string) (string, error) {
	q := url.Values{}
	q.Set("grant_type", "fb_exchange_token")
	q.Set("client_id", c.clientID)
	q.Set("client_secret", c.clientSecret)
	q.Set("fb_exchange_token", shortToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.TokenEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("facebook long-lived exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facebook long-lived exchange %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("facebook long-lived exchange decode: %w", err)
	}
	if raw.AccessToken == "" {
		return "", fmt.Errorf("facebook long-lived exchange returned empty access_token")
	}
	return raw.AccessToken, nil
}

type facebookConnectPage struct {
	ID          string
	Name        string
	AccessToken string
	PictureURL  string
	Tasks       []string
}

func (c *FacebookConnector) fetchPages(ctx context.Context, userAccessToken string) ([]facebookConnectPage, error) {
	q := url.Values{}
	q.Set("access_token", userAccessToken)
	q.Set("fields", "id,name,access_token,picture{url},tasks")
	q.Set("limit", "100")

	req, err := http.NewRequestWithContext(ctx, "GET", c.PagesEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook /me/accounts: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook /me/accounts %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
			Picture     struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			} `json:"picture"`
			Tasks []string `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("facebook /me/accounts decode: %w", err)
	}
	out := make([]facebookConnectPage, 0, len(raw.Data))
	for _, page := range raw.Data {
		out = append(out, facebookConnectPage{
			ID:          page.ID,
			Name:        page.Name,
			AccessToken: page.AccessToken,
			PictureURL:  page.Picture.Data.URL,
			Tasks:       page.Tasks,
		})
	}
	return out, nil
}

func firstPublishableFacebookPage(pages []facebookConnectPage) (facebookConnectPage, bool) {
	for _, page := range pages {
		if facebookPageHasPublishTask(page.Tasks) {
			return page, true
		}
	}
	return facebookConnectPage{}, false
}

func facebookPageHasPublishTask(tasks []string) bool {
	for _, task := range tasks {
		switch strings.ToUpper(task) {
		case "CREATE_CONTENT", "MANAGE", "MODERATE":
			return true
		}
	}
	return false
}

func (c *FacebookConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	q := url.Values{}
	q.Set("fields", "id,name,picture{url}")
	q.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.ProfileEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook /me page: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook /me page %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("facebook /me page decode: %w", err)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("facebook /me page returned empty id")
	}
	return &Profile{
		ExternalAccountID: raw.ID,
		Username:          firstNonEmpty(raw.Name, raw.ID),
		DisplayName:       raw.Name,
		AvatarURL:         raw.Picture.Data.URL,
	}, nil
}

func (c *FacebookConnector) Refresh(context.Context, string) (*TokenSet, error) {
	return nil, fmt.Errorf("facebook page tokens do not support refresh; reconnect the account on invalidation")
}
