// facebook.go implements Facebook Pages publishing and read-back.
//
// Unlike Instagram/Threads where one OAuth = one account, Facebook
// returns a list of Pages the user manages; each Page gets its own
// social_accounts row with its own permanent Page Access Token
// (derived from the user's long-lived token). The OAuth callback
// handler takes a detour through pending_connections to let the user
// pick which Pages to connect before any social_accounts row is
// written — see handler/oauth.go's facebook branch.
//
// Phase 1 of the Facebook PRD only covers OAuth + Page listing +
// account creation. Post(), analytics, inbox, and messenger DMs are
// stubbed and land in Phase 2-5.

package platform

import (
	"context"
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

// FacebookAdapter implements PlatformAdapter and OAuthAdapter for
// Facebook Pages. Posting uses a Page Access Token (per-Page,
// permanent); OAuth returns a User Token which we use once to
// enumerate the Pages the user manages.
type FacebookAdapter struct {
	client *http.Client
}

func NewFacebookAdapter() *FacebookAdapter {
	return &FacebookAdapter{client: debugrt.NewClient(60 * time.Second)}
}

func (a *FacebookAdapter) Platform() string { return "facebook" }

// facebookGraphBase is the Meta Graph API base — pinned to a version
// compatible with Pages publishing + messenger. Bump in lockstep
// with Instagram/Threads when they're upgraded.
const facebookGraphBase = "https://graph.facebook.com/v22.0"

// FacebookPagesScopes is the full permission set the PRD lists for
// Facebook Pages. Exported so tests + the dashboard docs page can
// reference the canonical list.
var FacebookPagesScopes = []string{
	"pages_show_list",
	"pages_manage_posts",
	"pages_read_engagement",
	"pages_read_user_content",
	"pages_manage_engagement",
	"pages_messaging",
	"pages_manage_metadata",
}

func (a *FacebookAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	// Meta reuses the same App for IG/Threads/Facebook Pages; the
	// env vars match the existing Instagram adapter so White-Label
	// customers don't have to configure a second app. Facebook-only
	// customers can set FACEBOOK_APP_ID / FACEBOOK_APP_SECRET to
	// override.
	clientID := os.Getenv("FACEBOOK_APP_ID")
	if clientID == "" {
		clientID = os.Getenv("INSTAGRAM_APP_ID")
	}
	clientSecret := os.Getenv("FACEBOOK_APP_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("INSTAGRAM_APP_SECRET")
	}
	return OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://www.facebook.com/v22.0/dialog/oauth",
		TokenURL:     facebookGraphBase + "/oauth/access_token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/facebook",
		Scopes:       FacebookPagesScopes,
	}
}

func (a *FacebookAdapter) GetAuthURL(config OAuthConfig, state string) string {
	return BuildAuthURL(config.AuthURL, config.ClientID, config.RedirectURL, state, config.Scopes)
}

// ExchangeCode turns the OAuth code into a short-lived User Access
// Token + Meta user identity. The callback handler follows up with
// ExchangeForLongLivedUserToken and FetchPages — this method on its
// own does NOT enumerate Pages or create a social_accounts row,
// because the user still needs to pick which Pages to connect.
//
// The returned ConnectResult uses the Meta user ID as
// ExternalAccountID so the callback can correlate the pending row
// with the eventually-finalized selections. AccessToken is the
// short-lived User Token; callers must exchange it for a long-lived
// one before storing.
func (a *FacebookAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	params := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"redirect_uri":  {config.RedirectURL},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", config.TokenURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook token exchange (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("facebook token decode: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("facebook token exchange: empty access_token in response: %s", string(body))
	}

	// Fetch the Meta user's own identity. This doubles as the tenant
	// key on the meta_user_tokens table — a single user may own
	// multiple Pages, so we index the stored LL token by user, not
	// by Page.
	userID, name, err := a.fetchMetaUserIdentity(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("facebook /me lookup: %w", err)
	}

	// expires_in is short (~1h) for the initial code exchange — the
	// callback will immediately exchange for a 60-day LL token so
	// this value never ends up in the DB.
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		expiresAt = time.Now().Add(1 * time.Hour)
	}
	return &ConnectResult{
		AccessToken:       tokenResp.AccessToken,
		RefreshToken:      "",
		TokenExpiresAt:    expiresAt,
		ExternalAccountID: userID,
		AccountName:       name,
		Metadata: map[string]any{
			"meta_user_id": userID,
		},
	}, nil
}

// ExchangeForLongLivedUserToken turns the short-lived User Token
// from ExchangeCode into a ~60-day token. This is what we store on
// meta_user_tokens so "Add another Page" later can re-call
// /me/accounts without a full re-OAuth.
//
// Despite the name, Meta returns a token valid for exactly 60 days;
// any refresh after that forces the user to re-authorize (Meta does
// not rotate Facebook user tokens server-side).
func (a *FacebookAdapter) ExchangeForLongLivedUserToken(ctx context.Context, clientID, clientSecret, shortLivedToken string) (string, time.Time, error) {
	params := url.Values{
		"grant_type":        {"fb_exchange_token"},
		"client_id":         {clientID},
		"client_secret":     {clientSecret},
		"fb_exchange_token": {shortLivedToken},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", facebookGraphBase+"/oauth/access_token?"+params.Encode(), nil)
	if err != nil {
		return "", time.Time{}, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("facebook LL exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("facebook LL exchange (%d): %s", resp.StatusCode, string(body))
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("facebook LL exchange decode: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("facebook LL exchange: empty access_token (body=%s)", string(body))
	}
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		// Meta doesn't always include expires_in on LL tokens; the
		// documented lifetime is 60 days.
		expiresAt = time.Now().Add(60 * 24 * time.Hour)
	}
	return tokenResp.AccessToken, expiresAt, nil
}

// FacebookPage describes one row from /me/accounts. AccessToken here
// is the Page Access Token (permanent when derived from a LL User
// Token), which the finalize handler encrypts before writing to
// social_accounts.access_token.
type FacebookPage struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Category    string   `json:"category"`
	PictureURL  string   `json:"picture_url"`
	Tasks       []string `json:"tasks"` // admin tasks granted to this user for this Page
}

// FetchPages calls /me/accounts and returns the Pages the user
// manages, each with its own Page Access Token. The callback handler
// uses this right after ExchangeForLongLivedUserToken so the picker
// has everything it needs to render.
//
// Missing permissions surface as an empty list rather than an error:
// the dashboard renders different copy for "0 Pages" vs "has Pages
// but lacks CREATE_CONTENT task" (see PRD §15), so we return the
// raw list and let the caller decide.
func (a *FacebookAdapter) FetchPages(ctx context.Context, userAccessToken string) ([]FacebookPage, error) {
	params := url.Values{
		"access_token": {userAccessToken},
		"fields":       {"id,name,access_token,category,picture{url},tasks"},
		"limit":        {"100"},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", facebookGraphBase+"/me/accounts?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook /me/accounts: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook /me/accounts (%d): %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
			Category    string `json:"category"`
			Picture     struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			} `json:"picture"`
			Tasks []string `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("facebook /me/accounts decode: %w", err)
	}
	out := make([]FacebookPage, 0, len(parsed.Data))
	for _, p := range parsed.Data {
		out = append(out, FacebookPage{
			ID:          p.ID,
			Name:        p.Name,
			AccessToken: p.AccessToken,
			Category:    p.Category,
			PictureURL:  p.Picture.Data.URL,
			Tasks:       p.Tasks,
		})
	}
	return out, nil
}

// PageHasPublishTask returns true when the Page's tasks list includes
// the permission required to publish content. Meta names this task
// "CREATE_CONTENT" in current API versions; we keep the check lenient
// so older tokens that still return the legacy "MANAGE" don't trip
// the picker into showing "insufficient permissions".
func PageHasPublishTask(tasks []string) bool {
	for _, t := range tasks {
		switch strings.ToUpper(t) {
		case "CREATE_CONTENT", "MANAGE", "MODERATE":
			return true
		}
	}
	return false
}

// fetchMetaUserIdentity returns the authenticated Meta user's id and
// name. Used during the initial code exchange so we can key the
// pending_connections + meta_user_tokens rows by a stable identity
// before the user has picked any Pages.
func (a *FacebookAdapter) fetchMetaUserIdentity(ctx context.Context, userAccessToken string) (id, name string, err error) {
	params := url.Values{
		"access_token": {userAccessToken},
		"fields":       {"id,name"},
	}
	req, hErr := http.NewRequestWithContext(ctx, "GET", facebookGraphBase+"/me?"+params.Encode(), nil)
	if hErr != nil {
		return "", "", hErr
	}
	resp, dErr := a.client.Do(req)
	if dErr != nil {
		return "", "", dErr
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("facebook /me (%d): %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", err
	}
	return parsed.ID, parsed.Name, nil
}

// Connect — OAuth-only, same pattern as Instagram/Threads.
func (a *FacebookAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("facebook requires OAuth flow, use /v1/oauth/connect/facebook")
}

// Post is stubbed for Phase 1. Full publishing lands in Phase 2 of
// the Facebook PRD (text + photo + video + link + scheduled).
func (a *FacebookAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	return nil, fmt.Errorf("facebook publishing is not yet implemented (Phase 2)")
}

// DeletePost is stubbed for Phase 1.
func (a *FacebookAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	return fmt.Errorf("facebook post deletion is not yet implemented")
}

// RefreshToken is a no-op for Facebook. Page Tokens derived from
// long-lived User Tokens do not expire; invalidation is detected
// passively via error code 190 on publish. The standard adapter
// interface requires this method, so we satisfy it with a clear
// error rather than silent success.
func (a *FacebookAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	return "", "", time.Time{}, fmt.Errorf("facebook page tokens do not support refresh; reconnect the account on invalidation")
}
