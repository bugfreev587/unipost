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
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// FacebookAdapter implements PlatformAdapter and OAuthAdapter for
// Facebook Pages. Posting uses a Page Access Token (per-Page,
// permanent); OAuth returns a User Token which we use once to
// enumerate the Pages the user manages.
type FacebookAdapter struct {
	client     *http.Client
	mediaProxy *storage.Client // optional; recommended for video posts
}

func NewFacebookAdapter() *FacebookAdapter {
	return &FacebookAdapter{client: debugrt.NewClient(60 * time.Second)}
}

// SetMediaProxy attaches an R2-backed media proxy. Facebook video
// uploads rely on the file_url pull model — FB fetches the source
// asynchronously, often long after the 15-minute presigned URL we
// mint from R2 has expired, which leaves the video stuck in
// video_status=uploading forever. Staging video bytes through the
// public R2 bucket (no TTL on those URLs) avoids that race entirely.
// Safe to call with nil; videos then fall back to the original URL.
func (a *FacebookAdapter) SetMediaProxy(c *storage.Client) {
	a.mediaProxy = c
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

// Post publishes text, a link, a single photo, or a single video to
// a Facebook Page. The Page Access Token is scoped to one Page, so
// we don't need a page_id in opts — we resolve it by asking Graph
// "GET /me" with the Page Token, which returns the Page itself.
//
// Combination matrix (PRD §4, answer 4). Everything outside this
// set is rejected by the adapter (and additionally by the validator
// at compose time, so direct API callers get the same error):
//
//	text only                   → /{page_id}/feed     message
//	link only                   → /{page_id}/feed     link
//	text + link                 → /{page_id}/feed     message + link
//	text + single image         → /{page_id}/photos   url + caption
//	text + single video         → /{page_id}/videos   file_url + description
//
// Scheduling: UniPost's scheduler owns timing for every platform,
// including Facebook. By the time this method runs, scheduled_at is
// already "now" — no FB-native scheduled_publish_time params are
// emitted from here. Photo/video scheduling is rejected at the
// validator level per the Phase-2 decisions, not here.
func (a *FacebookAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	link := strings.TrimSpace(optString(opts, "link"))
	hasText := strings.TrimSpace(text) != ""
	hasLink := link != ""
	hasMedia := len(media) > 0

	if !hasText && !hasLink && !hasMedia {
		return nil, fmt.Errorf("facebook: need text, link, or media to post")
	}
	// Link + media is explicitly disallowed per the PRD matrix: FB's
	// link preview gets absorbed into the photo/video post, which
	// silently drops one of the user's inputs. Refuse cleanly.
	if hasLink && hasMedia {
		return nil, fmt.Errorf("facebook: link and media cannot be combined in the same post")
	}
	if hasMedia && len(media) > 1 {
		return nil, fmt.Errorf("facebook: v1 supports one photo or one video per post (got %d media items)", len(media))
	}

	pageID, err := a.fetchPageSelfID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// Dispatch by media kind.
	if hasMedia {
		m := media[0]
		kind := m.Kind
		if kind == MediaKindUnknown {
			kind = SniffMediaKind(m.URL)
		}
		switch kind {
		case MediaKindImage, MediaKindGIF:
			return a.postPhoto(ctx, accessToken, pageID, text, m.URL)
		case MediaKindVideo:
			return a.postVideo(ctx, accessToken, pageID, text, m.URL)
		default:
			return nil, fmt.Errorf("facebook: unsupported media kind %q for %s", kind, m.URL)
		}
	}

	// No media → /feed. Scheduling params (published=false,
	// scheduled_publish_time) are intentionally omitted; UniPost's
	// scheduler owns timing uniformly across platforms.
	return a.postFeed(ctx, accessToken, pageID, text, link)
}

// postFeed handles /feed posts — text, link, or text+link.
func (a *FacebookAdapter) postFeed(ctx context.Context, accessToken, pageID, text, link string) (*PostResult, error) {
	form := url.Values{
		"access_token": {accessToken},
	}
	if text != "" {
		form.Set("message", text)
	}
	if link != "" {
		form.Set("link", link)
	}
	raw, err := a.publishRaw(ctx, facebookGraphBase+"/"+pageID+"/feed", form)
	if err != nil {
		return nil, err
	}
	// /feed returns id as "{page_id}_{story_id}" — canonical URL
	// splits on the underscore.
	return &PostResult{
		ExternalID: raw.ID,
		URL:        feedStoryURL(pageID, raw.ID),
	}, nil
}

// postPhoto uploads a single photo by remote URL. FB's /photos
// endpoint treats `url=<remote>` as pull-from-URL; message doubles
// as the caption. No multipart streaming path in v1.
func (a *FacebookAdapter) postPhoto(ctx context.Context, accessToken, pageID, caption, imageURL string) (*PostResult, error) {
	form := url.Values{
		"access_token": {accessToken},
		"url":          {imageURL},
	}
	if caption != "" {
		form.Set("message", caption)
	}
	raw, err := a.publishRaw(ctx, facebookGraphBase+"/"+pageID+"/photos", form)
	if err != nil {
		return nil, err
	}
	// /photos returns { id: <photo_id>, post_id: "{page}_{story}" }.
	// Prefer post_id because it's the feed-story id — that's what
	// "View on Facebook" should resolve to, and also what the
	// analytics endpoint will later recognize.
	storyID := raw.PostID
	if storyID == "" {
		storyID = raw.ID
	}
	return &PostResult{
		ExternalID: storyID,
		URL:        feedStoryURL(pageID, storyID),
	}, nil
}

// postVideo uploads a single video by remote URL. Same pull model
// as photos; `file_url` is FB's param for the remote source. v1 does
// not support resumable upload — videos over ~1GB should be
// rejected at the validator.
//
// When a media proxy is configured (the usual case), we stage the
// video through our public R2 bucket first so FB's async fetch
// sees a URL that won't expire mid-download. Without this, any FB
// video upload that takes longer than the source URL's TTL gets
// stuck in video_status="uploading" indefinitely.
func (a *FacebookAdapter) postVideo(ctx context.Context, accessToken, pageID, description, videoURL string) (*PostResult, error) {
	stagedURL := videoURL
	if a.mediaProxy != nil {
		proxied, err := a.mediaProxy.UploadFromURL(ctx, videoURL)
		if err != nil {
			return nil, fmt.Errorf("facebook video: stage to R2: %w", err)
		}
		stagedURL = proxied
	}
	form := url.Values{
		"access_token": {accessToken},
		"file_url":     {stagedURL},
	}
	if description != "" {
		form.Set("description", description)
	}
	raw, err := a.publishRaw(ctx, facebookGraphBase+"/"+pageID+"/videos", form)
	if err != nil {
		return nil, err
	}
	// /videos returns only { id: <video_id> } — there's no story id
	// yet because Facebook processes the video asynchronously. The
	// public URL uses the /videos/ path; /posts/<video_id> 404s
	// because video ids aren't story ids.
	if raw.ID == "" {
		return nil, fmt.Errorf("facebook video publish: response missing id")
	}
	return &PostResult{
		ExternalID: raw.ID,
		URL:        fmt.Sprintf("https://www.facebook.com/%s/videos/%s", pageID, raw.ID),
	}, nil
}

// facebookPublishResult is the shape every publish endpoint returns,
// parsed once here so callers don't each replicate the error envelope
// handling. ID + PostID are both captured because each endpoint
// populates them differently (see postPhoto / postVideo comments).
type facebookPublishResult struct {
	ID     string `json:"id"`
	PostID string `json:"post_id"`
}

// publishRaw POSTs the form and returns the parsed response. Auth
// codes (190 / 102 / 200) are promoted to the "needs reconnect"
// error; other Graph API errors pass through with code + trace id
// so the admin debug view can surface them verbatim.
func (a *FacebookAdapter) publishRaw(ctx context.Context, endpoint string, form url.Values) (*facebookPublishResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook publish: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, wrapFacebookPublishError(resp.StatusCode, body)
	}

	var parsed facebookPublishResult
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("facebook publish: decode response: %w (body=%s)", err, string(body))
	}
	if parsed.ID == "" && parsed.PostID == "" {
		return nil, fmt.Errorf("facebook publish: response missing id (body=%s)", string(body))
	}
	return &parsed, nil
}

// feedStoryURL builds the canonical "/{page}/posts/{story}" URL from
// either a combined "{page}_{story}" id or a bare story id.
// Empty pageID falls back to the short form, which Facebook redirects
// to the right Page if the id is globally unique.
func feedStoryURL(pageID, storyID string) string {
	if storyID == "" {
		return "https://www.facebook.com/"
	}
	if strings.Contains(storyID, "_") {
		parts := strings.SplitN(storyID, "_", 2)
		return fmt.Sprintf("https://www.facebook.com/%s/posts/%s", parts[0], parts[1])
	}
	if pageID != "" {
		return fmt.Sprintf("https://www.facebook.com/%s/posts/%s", pageID, storyID)
	}
	return "https://www.facebook.com/" + storyID
}

// wrapFacebookPublishError turns Graph API error responses into
// something the UI can act on. Auth-shaped errors (190 / 102 / 200)
// route to a "needs reconnect" message that the Posts Overview's
// error hint logic recognizes.
func wrapFacebookPublishError(status int, body []byte) error {
	var parsed struct {
		Error struct {
			Code      int    `json:"code"`
			Message   string `json:"message"`
			Type      string `json:"type"`
			FBTraceID string `json:"fbtrace_id"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &parsed)

	if parsed.Error.Message != "" {
		switch parsed.Error.Code {
		case 190, 102, 200:
			return fmt.Errorf("facebook: access token rejected (code %d: %s). Please reconnect the Page.",
				parsed.Error.Code, parsed.Error.Message)
		}
		return fmt.Errorf("facebook publish (%d): %s [code=%d trace=%s]",
			status, parsed.Error.Message, parsed.Error.Code, parsed.Error.FBTraceID)
	}
	return fmt.Errorf("facebook publish (%d): %s", status, string(body))
}

// fetchPageSelfID calls /me with a Page Access Token, which returns
// the Page (not the authorizing user). Used by Post() to avoid
// threading page_id through the adapter signature. One cached call
// per publish is fine — they already cost one HTTP round trip on
// Meta's side.
func (a *FacebookAdapter) fetchPageSelfID(ctx context.Context, pageAccessToken string) (string, error) {
	params := url.Values{
		"access_token": {pageAccessToken},
		"fields":       {"id"},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", facebookGraphBase+"/me?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("facebook /me (page): %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", wrapFacebookPublishError(resp.StatusCode, body)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.ID == "" {
		return "", fmt.Errorf("facebook /me (page): response missing id (body=%s)", string(body))
	}
	return parsed.ID, nil
}

// DeletePost removes a post by its external id. Not exercised in
// Phase 2 (the posts list doesn't wire a delete action for FB yet)
// but wiring it now means the Retry flow on the UI also fails
// cleanly when a post was deleted out-of-band.
func (a *FacebookAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	form := url.Values{"access_token": {accessToken}}
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		facebookGraphBase+"/"+externalID+"?"+form.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("facebook delete: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return wrapFacebookPublishError(resp.StatusCode, body)
	}
	return nil
}

// RefreshToken is a no-op for Facebook. Page Tokens derived from
// long-lived User Tokens do not expire; invalidation is detected
// passively via error code 190 on publish. The standard adapter
// interface requires this method, so we satisfy it with a clear
// error rather than silent success.
func (a *FacebookAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	return "", "", time.Time{}, fmt.Errorf("facebook page tokens do not support refresh; reconnect the account on invalidation")
}
