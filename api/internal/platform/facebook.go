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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

type FacebookCommentAuthor struct {
	ID        string
	Name      string
	AvatarURL string
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

	// mediaType switches which Facebook publish surface we target for
	// video media. `feed` (or empty — the historical default) goes to
	// /{page_id}/videos with file_url; `reel` goes to the 3-phase
	// /{page_id}/video_reels flow. Anything else is rejected at the
	// validator, so by the time we reach here we only have to handle
	// the two shipped values.
	mediaType := strings.ToLower(strings.TrimSpace(optString(opts, "mediaType")))
	if mediaType == "" {
		mediaType = strings.ToLower(strings.TrimSpace(optString(opts, "media_type")))
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
			if mediaType == "reel" {
				return a.postVideoReel(ctx, accessToken, pageID, text, m.URL, opts)
			}
			return a.postVideo(ctx, accessToken, pageID, text, m.URL, opts)
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
//
// After the initial POST we poll FB's status endpoint for up to
// 60s. If the video finishes processing within that window we
// return the canonical /posts/{story} URL and a normal published
// result. If it doesn't, we return Status="processing" so the row
// shows as in-flight in the dashboard — the Get handler's re-poll
// flips it to "published" once FB is done.
func (a *FacebookAdapter) postVideo(ctx context.Context, accessToken, pageID, description, videoURL string, opts map[string]any) (*PostResult, error) {
	stagedURL := videoURL
	if a.mediaProxy != nil {
		proxied, err := a.mediaProxy.UploadFromURL(ctx, videoURL)
		if err != nil {
			return nil, fmt.Errorf("facebook video: stage to R2: %w", err)
		}
		stagedURL = proxied
		slog.Info("facebook video: staged to R2", "staged_url", proxied, "source_url", videoURL)
	} else {
		slog.Warn("facebook video: no media proxy configured; passing source URL directly to FB (may expire mid-fetch)", "source_url", videoURL)
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
	if raw.ID == "" {
		return nil, fmt.Errorf("facebook video publish: response missing id")
	}
	videoID := raw.ID

	// Poll FB for up to ~60s (12 × 5s) until the video is either
	// ready or errored. This turns the common "small video finishes
	// in <60s" case into a clean "published" result, and leaves the
	// larger-video case as "processing" for the Get handler to
	// refresh.
	for i := 0; i < 12; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
		st, err := a.CheckVideoStatus(ctx, accessToken, videoID)
		if err != nil {
			// Transient errors get another poll round rather than
			// bailing — the caller already got a video_id, so
			// giving up mid-poll would mean losing state.
			slog.Warn("facebook: video status poll failed", "video_id", videoID, "error", err)
			continue
		}
		// Reel reclassification: if FB's permalink_url starts with
		// /reel/ the upload was routed to the Reels pipeline, which
		// /videos + file_url doesn't drive — the upload would stay
		// stuck in uploading/in_progress indefinitely.
		//
		// When the Reels feature flag is on, transparently clean up
		// the stuck feed-video resource and retry via the Reels
		// path. The user's intent was "publish this video"; the
		// feed-vs-reel distinction is FB's aspect-ratio routing
		// decision, not something they should have to care about.
		// The FinalMediaType="reel" hint tells the handler to
		// persist fb_media_type='reel' so the status worker treats
		// the row as an intentional Reel.
		//
		// When the flag is off, fall back to the Phase-1 fast-fail
		// behavior but with a more actionable message pointing at
		// the flag / composer toggle.
		if isReelPermalink(st.PermalinkURL) {
			_ = a.tryDeleteVideo(ctx, accessToken, videoID)
			if facebookReelsEnabled() {
				slog.Info("facebook: feed video reclassified as Reel, retrying via /video_reels", "video_id", videoID)
				res, err := a.postVideoReel(ctx, accessToken, pageID, description, videoURL, opts)
				if err != nil {
					return nil, err
				}
				if res != nil {
					res.FinalMediaType = "reel"
				}
				return res, nil
			}
			return nil, fmt.Errorf("facebook: Facebook reclassified this vertical video as a Reel. Enable FEATURE_FACEBOOK_REELS on the API or switch to the Reel option in the composer")
		}
		switch st.VideoStatus {
		case "ready":
			// Prefer the feed-story id so the public URL lands on
			// the Page timeline rather than the raw video watch
			// page, and so downstream Graph calls (/comments,
			// /insights, DELETE) target the canonical Page post
			// rather than the raw video object — Graph rejects
			// /{video_id}/comments with "Object does not exist"
			// when the token scope is the Page rather than the
			// video itself. post_id only becomes available after
			// processing completes.
			if st.PostID != "" {
				return &PostResult{
					ExternalID: st.PostID,
					URL:        feedStoryURL(pageID, st.PostID),
				}, nil
			}
			return &PostResult{
				ExternalID: videoID,
				URL:        fmt.Sprintf("https://www.facebook.com/%s/videos/%s", pageID, videoID),
			}, nil
		case "error", "upload_failed", "expired":
			msg := "facebook: video processing failed"
			if st.ErrorMessage != "" {
				msg += ": " + st.ErrorMessage
			}
			return nil, fmt.Errorf("%s", msg)
		}
		// Phase-level failure with top-level still transient — FB
		// sometimes reports the phase as errored while
		// `video_status` still reads "uploading"/"processing".
		// Fail fast on the phase signal rather than waiting for the
		// top-level status to catch up.
		if phaseHasError(st) {
			msg := "facebook: video phase failed"
			if st.ErrorMessage != "" {
				msg += ": " + st.ErrorMessage
			}
			return nil, fmt.Errorf("%s", msg)
		}
		// "uploading" / "processing" / "publishing" → keep polling
	}

	// Timeout — FB is still working on it. Return a "processing"
	// row; the Get handler's re-poll will flip to "published" once
	// FB finishes. The /videos/ URL still works for the user to
	// check progress directly on Facebook.
	slog.Info("facebook: video still processing after 60s; returning processing state", "video_id", videoID)
	return &PostResult{
		ExternalID: videoID,
		URL:        fmt.Sprintf("https://www.facebook.com/%s/videos/%s", pageID, videoID),
		Status:     "processing",
	}, nil
}

// postVideoReel publishes to the /{page_id}/video_reels endpoint in
// three phases. Reels can't be driven through /videos + file_url —
// Facebook explicitly routes Reel uploads through a different
// initialize/transfer/finish flow. UniPost picks the file_url
// transfer variant so the upload path matches the Feed video flow
// as closely as possible (stage to R2 once, hand Meta a public URL,
// Meta pulls asynchronously).
//
// 1. start:   POST /{page_id}/video_reels?upload_phase=start
//             → returns video_id + upload_url
// 2. transfer POST {upload_url}?upload_phase=transfer&file_url=<R2>
//             Meta pulls the bytes asynchronously
// 3. finish:  POST /{page_id}/video_reels?upload_phase=finish&
//             video_id=&video_state=PUBLISHED[&description&title&...]
//
// After finish we poll /{video_id}?fields=status for up to ~60s just
// like the Feed path, and return Status="processing" if Meta is still
// transcoding so the status worker can flip it to "published" later.
func (a *FacebookAdapter) postVideoReel(ctx context.Context, accessToken, pageID, description, videoURL string, opts map[string]any) (*PostResult, error) {
	// Stage to R2 the same way postVideo does so Meta's async pull
	// sees a stable public URL rather than a short-lived presigned
	// download. Without this the transfer can expire mid-pull on
	// large videos.
	stagedURL := videoURL
	if a.mediaProxy != nil {
		proxied, err := a.mediaProxy.UploadFromURL(ctx, videoURL)
		if err != nil {
			return nil, fmt.Errorf("facebook reel: stage to R2: %w", err)
		}
		stagedURL = proxied
		slog.Info("facebook reel: staged to R2", "staged_url", proxied, "source_url", videoURL)
	} else {
		slog.Warn("facebook reel: no media proxy configured; passing source URL directly to FB (may expire mid-fetch)", "source_url", videoURL)
	}

	// Phase 1: start.
	videoID, uploadURL, err := a.startVideoReelUpload(ctx, accessToken, pageID)
	if err != nil {
		return nil, fmt.Errorf("facebook reel: start: %w", err)
	}
	if videoID == "" || uploadURL == "" {
		return nil, fmt.Errorf("facebook reel: start response missing video_id or upload_url")
	}

	// Phase 2: transfer. Meta expects Authorization: OAuth <token> on
	// the transfer request against upload.facebook.com, and a
	// file_url query param telling it where to pull from.
	transferURL := uploadURL
	if strings.Contains(transferURL, "?") {
		transferURL += "&"
	} else {
		transferURL += "?"
	}
	transferURL += "upload_phase=transfer&file_url=" + url.QueryEscape(stagedURL)
	req, err := http.NewRequestWithContext(ctx, "POST", transferURL, nil)
	if err != nil {
		return nil, fmt.Errorf("facebook reel: build transfer request: %w", err)
	}
	req.Header.Set("Authorization", "OAuth "+accessToken)
	transferResp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook reel: transfer: %w", err)
	}
	defer transferResp.Body.Close()
	transferBody, _ := io.ReadAll(transferResp.Body)
	if transferResp.StatusCode >= 400 {
		return nil, wrapFacebookPublishError(transferResp.StatusCode, transferBody)
	}

	// Phase 3: finish.
	finishForm := url.Values{
		"access_token": {accessToken},
		"upload_phase": {"finish"},
		"video_id":     {videoID},
		"video_state":  {"PUBLISHED"},
	}
	if description != "" {
		finishForm.Set("description", description)
	}
	if t := strings.TrimSpace(optString(opts, "title")); t != "" {
		finishForm.Set("title", t)
	}
	if n, ok := opts["thumb_offset_ms"]; ok {
		if v, ok := coerceOptionIntFB(n); ok {
			finishForm.Set("thumb_offset", strconv.Itoa(v))
		}
	}
	if err := a.finishVideoReelUpload(ctx, pageID, finishForm); err != nil {
		return nil, fmt.Errorf("facebook reel: finish: %w", err)
	}

	// Poll status the same way the Feed path does — a Reel that
	// finishes inside 60s returns a clean "published" result, and
	// longer uploads return "processing" for the status worker to
	// flip once Meta is done.
	for i := 0; i < 12; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
		st, err := a.CheckVideoStatus(ctx, accessToken, videoID)
		if err != nil {
			slog.Warn("facebook reel: video status poll failed", "video_id", videoID, "error", err)
			continue
		}
		switch st.VideoStatus {
		case "ready":
			if st.PostID != "" {
				return &PostResult{
					ExternalID:     st.PostID,
					URL:            feedStoryURL(pageID, st.PostID),
					FinalMediaType: "reel",
				}, nil
			}
			// Fallback — build the /reel/ permalink from the raw
			// video id until the Page post id propagates.
			return &PostResult{
				ExternalID:     videoID,
				URL:            fmt.Sprintf("https://www.facebook.com/reel/%s", videoID),
				FinalMediaType: "reel",
			}, nil
		case "error", "upload_failed", "expired":
			msg := "facebook reel: processing failed"
			if st.ErrorMessage != "" {
				msg += ": " + st.ErrorMessage
			}
			return nil, fmt.Errorf("%s", msg)
		}
		if phaseHasError(st) {
			msg := "facebook reel: phase failed"
			if st.ErrorMessage != "" {
				msg += ": " + st.ErrorMessage
			}
			return nil, fmt.Errorf("%s", msg)
		}
		// Still transient — keep polling.
	}

	slog.Info("facebook reel: still processing after 60s; returning processing state", "video_id", videoID)
	return &PostResult{
		ExternalID:     videoID,
		URL:            fmt.Sprintf("https://www.facebook.com/reel/%s", videoID),
		Status:         "processing",
		FinalMediaType: "reel",
	}, nil
}

// startVideoReelUpload performs Phase 1 of the /video_reels flow and
// returns the video_id + upload_url Meta hands back. Kept separate
// from publishRaw because the Reel start response uses different
// field names ({"video_id", "upload_url"}) than the generic publish
// endpoints ({"id", "post_id"}).
func (a *FacebookAdapter) startVideoReelUpload(ctx context.Context, accessToken, pageID string) (string, string, error) {
	form := url.Values{
		"access_token": {accessToken},
		"upload_phase": {"start"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		facebookGraphBase+"/"+pageID+"/video_reels",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("reel start: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", wrapFacebookPublishError(resp.StatusCode, body)
	}
	var parsed struct {
		VideoID   string `json:"video_id"`
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("reel start: decode: %w (body=%s)", err, string(body))
	}
	return parsed.VideoID, parsed.UploadURL, nil
}

// finishVideoReelUpload performs Phase 3 of the /video_reels flow.
// The response shape is `{"success": true}` — no id or post_id —
// which publishRaw would flag as missing-id, so this helper only
// checks the HTTP status and the "success" flag directly.
func (a *FacebookAdapter) finishVideoReelUpload(ctx context.Context, pageID string, form url.Values) error {
	req, err := http.NewRequestWithContext(ctx, "POST",
		facebookGraphBase+"/"+pageID+"/video_reels",
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("reel finish: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return wrapFacebookPublishError(resp.StatusCode, body)
	}
	var parsed struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("reel finish: decode: %w (body=%s)", err, string(body))
	}
	if !parsed.Success {
		return fmt.Errorf("reel finish: server reported success=false (body=%s)", string(body))
	}
	return nil
}

// coerceOptionIntFB pulls an int out of a platform_options value that
// may be a float64 (JSON number), int, or numeric string. Local copy
// to keep this adapter file self-contained; the validator has its
// own flavor for its own error shape.
func coerceOptionIntFB(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		if x == float64(int(x)) {
			return int(x), true
		}
	case int:
		return x, true
	case int64:
		return int(x), true
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			return n, true
		}
	}
	return 0, false
}

// isReelPermalink returns true when FB assigned a permalink under
// /reel/, which means it reclassified the video into the Reels
// pipeline. The /videos + file_url path cannot drive that pipeline, so
// the upload will never finish and we should fail fast.
func isReelPermalink(permalinkURL string) bool {
	trimmed := strings.TrimSpace(permalinkURL)
	if trimmed == "" {
		return false
	}
	// Graph returns either a fully-qualified URL or a root-relative
	// path depending on the field selection.
	return strings.Contains(trimmed, "/reel/")
}

// phaseHasError returns true when any of the three upload phases
// reports status="error" — this can fire before the top-level
// `video_status` flips, so it's worth a separate check.
func phaseHasError(st *FacebookVideoStatus) bool {
	if st == nil {
		return false
	}
	return strings.EqualFold(st.UploadingStatus, "error") ||
		strings.EqualFold(st.ProcessingStatus, "error") ||
		strings.EqualFold(st.PublishingStatus, "error")
}

// tryDeleteVideo best-efforts the cleanup of a stuck video resource.
// We issue DELETE /{video_id} so FB doesn't leave an orphan row
// attached to the Page. Failures are logged and swallowed — we've
// already decided to return an error for this publish regardless.
func (a *FacebookAdapter) tryDeleteVideo(ctx context.Context, accessToken, videoID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", facebookGraphBase+"/"+videoID+"?access_token="+url.QueryEscape(accessToken), nil)
	if err != nil {
		slog.Warn("facebook: build delete request failed", "video_id", videoID, "error", err)
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		slog.Warn("facebook: delete video failed", "video_id", videoID, "error", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("facebook: delete video non-2xx", "video_id", videoID, "status", resp.StatusCode, "body", truncateForLog(string(body), 240))
		return fmt.Errorf("facebook: delete video returned %d", resp.StatusCode)
	}
	return nil
}

// truncateForLog clips a string to n bytes for log readability.
func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// FacebookVideoStatus mirrors the subset of /{video_id}?fields=status,post_id
// we care about. "uploading" / "processing" / "publishing" are
// all transient; "ready" is terminal success; "error" is terminal
// failure. The individual phase sub-objects are kept so the
// dashboard can eventually render a three-step progress indicator.
type FacebookVideoStatus struct {
	VideoStatus      string `json:"video_status"`
	UploadingStatus  string `json:"uploading_phase_status"`
	ProcessingStatus string `json:"processing_phase_status"`
	PublishingStatus string `json:"publishing_phase_status"`
	PostID           string `json:"post_id"`
	PermalinkURL     string `json:"permalink_url"`
	ErrorMessage     string `json:"error_message"`
}

// CheckVideoStatus queries Graph for the lifecycle state of a video.
// Used both by the publish-time poll in postVideo and by the Get
// handler to refresh the stored row when the user views a video
// whose status hasn't been confirmed yet.
func (a *FacebookAdapter) CheckVideoStatus(ctx context.Context, accessToken, videoID string) (*FacebookVideoStatus, error) {
	params := url.Values{
		"access_token": {accessToken},
		"fields":       {"status,post_id,permalink_url"},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", facebookGraphBase+"/"+videoID+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook status: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, wrapFacebookPublishError(resp.StatusCode, body)
	}

	var raw struct {
		Status struct {
			VideoStatus     string                         `json:"video_status"`
			UploadingPhase  struct{ Status, Error string } `json:"uploading_phase"`
			ProcessingPhase struct{ Status, Error string } `json:"processing_phase"`
			PublishingPhase struct{ Status, Error string } `json:"publishing_phase"`
		} `json:"status"`
		PostID       string `json:"post_id"`
		PermalinkURL string `json:"permalink_url"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("facebook status: decode: %w", err)
	}
	out := &FacebookVideoStatus{
		VideoStatus:      raw.Status.VideoStatus,
		UploadingStatus:  raw.Status.UploadingPhase.Status,
		ProcessingStatus: raw.Status.ProcessingPhase.Status,
		PublishingStatus: raw.Status.PublishingPhase.Status,
		PostID:           raw.PostID,
		PermalinkURL:     raw.PermalinkURL,
	}
	// Surface the first non-empty phase error so the caller can
	// include it in a user-facing message when the whole upload
	// fails.
	for _, phase := range []struct{ Status, Error string }{raw.Status.UploadingPhase, raw.Status.ProcessingPhase, raw.Status.PublishingPhase} {
		if phase.Error != "" {
			out.ErrorMessage = phase.Error
			break
		}
	}
	return out, nil
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

// FeedStoryURL is the exported, empty-safe variant used by callers
// outside the platform package (handler Get-time refresh, video status
// worker). Returns "" on empty storyID so callers can decide whether
// to keep the prior URL instead of overwriting it with a Facebook
// homepage link.
func FeedStoryURL(pageID, storyID string) string {
	if storyID == "" {
		return ""
	}
	return feedStoryURL(pageID, storyID)
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

// GetAnalytics fetches post-level metrics for a Facebook Page
// post. One call to Graph's field-expansion API pulls reactions,
// comments, shares, and insights in a single round trip — cheaper
// than separate /insights + /comments.summary calls.
//
// Meta returns 400 rather than 0 when a metric isn't available
// (fresh posts, posts under the 100-like threshold, unsupported
// by post type). We log and fall back to zero for missing fields
// rather than failing the whole refresh — partial metrics are
// better than none.
func (a *FacebookAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	params := url.Values{
		"access_token": {accessToken},
		"fields": {
			"id," +
				"reactions.summary(true).limit(0)," +
				"comments.summary(true).limit(0)," +
				"shares," +
				"insights.metric(post_impressions,post_impressions_unique,post_clicks,post_engaged_users,post_video_views)",
		},
	}
	endpoint := facebookGraphBase + "/" + externalID + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook analytics: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, wrapFacebookPublishError(resp.StatusCode, body)
	}

	var parsed struct {
		ID        string `json:"id"`
		Reactions struct {
			Summary struct {
				TotalCount int64 `json:"total_count"`
			} `json:"summary"`
		} `json:"reactions"`
		Comments struct {
			Summary struct {
				TotalCount int64 `json:"total_count"`
			} `json:"summary"`
		} `json:"comments"`
		Shares struct {
			Count int64 `json:"count"`
		} `json:"shares"`
		Insights struct {
			Data []struct {
				Name   string `json:"name"`
				Values []struct {
					Value any `json:"value"`
				} `json:"values"`
			} `json:"data"`
		} `json:"insights"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("facebook analytics decode: %w", err)
	}

	out := &PostMetrics{
		Likes:    parsed.Reactions.Summary.TotalCount,
		Comments: parsed.Comments.Summary.TotalCount,
		Shares:   parsed.Shares.Count,
	}
	for _, m := range parsed.Insights.Data {
		if len(m.Values) == 0 {
			continue
		}
		val := coerceInsightInt(m.Values[0].Value)
		switch m.Name {
		case "post_impressions":
			out.Impressions = val
		case "post_impressions_unique":
			out.Reach = val
		case "post_clicks":
			out.Clicks = val
		case "post_video_views":
			out.VideoViews = val
			out.Views = val
		}
	}
	return out, nil
}

// FacebookPageInsights is the subset of Page-level metrics we
// surface in the analytics dashboard. Populated by GetPageInsights
// when the Page has ≥100 likes (FB's hard-coded threshold — any
// Page under that returns zeros across the board).
type FacebookPageInsights struct {
	Follows         int64 `json:"follows"`
	Impressions     int64 `json:"impressions"`
	PostEngagements int64 `json:"post_engagements"`
	// Below100LikesNotice flips true when FB rejected the insights
	// query because the Page hasn't crossed the 100-like floor;
	// the dashboard renders a friendly "Keep growing!" state in
	// that case instead of the raw error.
	Below100LikesNotice bool `json:"below_100_likes_notice"`
}

// GetPageInsights aggregates daily Page insights over the given
// window. Window sizes are clamped to Meta's documented limits:
// min 1 day, max 92 days.
func (a *FacebookAdapter) GetPageInsights(ctx context.Context, accessToken, pageID string, since, until time.Time) (*FacebookPageInsights, error) {
	if pageID == "" {
		pid, err := a.fetchPageSelfID(ctx, accessToken)
		if err != nil {
			return nil, err
		}
		pageID = pid
	}
	params := url.Values{
		"access_token": {accessToken},
		"metric":       {"page_follows,page_impressions,page_post_engagements"},
		"period":       {"day"},
		"since":        {fmt.Sprintf("%d", since.Unix())},
		"until":        {fmt.Sprintf("%d", until.Unix())},
	}
	endpoint := facebookGraphBase + "/" + pageID + "/insights?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook page insights: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// FB surfaces the <100-likes guard via error code 100.
		// Detect that specifically so the caller can render the
		// "Keep growing!" state rather than an opaque error.
		if strings.Contains(string(body), "100 likes") || strings.Contains(string(body), "requires at least 100") {
			return &FacebookPageInsights{Below100LikesNotice: true}, nil
		}
		return nil, wrapFacebookPublishError(resp.StatusCode, body)
	}
	var parsed struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value any `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("facebook page insights decode: %w", err)
	}
	out := &FacebookPageInsights{}
	for _, m := range parsed.Data {
		total := int64(0)
		for _, v := range m.Values {
			total += coerceInsightInt(v.Value)
		}
		switch m.Name {
		case "page_follows":
			out.Follows = total
		case "page_impressions":
			out.Impressions = total
		case "page_post_engagements":
			out.PostEngagements = total
		}
	}
	return out, nil
}

// coerceInsightInt handles Meta's mix of number / object values in
// insights.data[].values[]. For simple metrics we get a bare
// number; for breakdowns we get a map we sum. Missing or invalid
// values become 0.
func coerceInsightInt(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case map[string]any:
		// Sum breakdown maps (post_reactions_by_type_total etc).
		var total int64
		for _, inner := range x {
			total += coerceInsightInt(inner)
		}
		return total
	}
	return 0
}

// FetchComments reads replies on a single Page post by ID. Returns
// at most 25 entries (Meta's default limit) sorted newest first;
// the sync worker relies on the UNIQUE(social_account_id,
// external_id) constraint on inbox_items for dedup across polls
// rather than tracking a "since" cursor here.
//
// Meta's comment order field (`reverse_chronological`) is the
// default for Pages — omitted intentionally so we match whatever
// default the account's locale/version applies.
func (a *FacebookAdapter) FetchComments(ctx context.Context, accessToken string, postExternalID string) ([]InboxEntry, error) {
	params := url.Values{
		"access_token": {accessToken},
		"fields":       {"id,message,from{id,name,picture{url}},created_time,parent{id},comments.limit(50){id,message,from{id,name,picture{url}},created_time,parent{id}}"},
		"limit":        {"25"},
	}
	endpoint := facebookGraphBase + "/" + postExternalID + "/comments?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook fetch comments: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("facebook fetch comments failed",
			"status", resp.StatusCode,
			"post_id", postExternalID,
			"body", string(body))
		return nil, fmt.Errorf("facebook fetch comments %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			From    struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Picture struct {
					Data struct {
						URL string `json:"url"`
					} `json:"data"`
				} `json:"picture"`
			} `json:"from"`
			CreatedTime string `json:"created_time"`
			Parent      struct {
				ID string `json:"id"`
			} `json:"parent"`
			Comments struct {
				Data []struct {
					ID      string `json:"id"`
					Message string `json:"message"`
					From    struct {
						ID      string `json:"id"`
						Name    string `json:"name"`
						Picture struct {
							Data struct {
								URL string `json:"url"`
							} `json:"data"`
						} `json:"picture"`
					} `json:"from"`
					CreatedTime string `json:"created_time"`
					Parent      struct {
						ID string `json:"id"`
					} `json:"parent"`
				} `json:"data"`
			} `json:"comments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("facebook fetch comments decode: %w", err)
	}

	parseFacebookCommentTime := func(raw string) time.Time {
		ts, _ := time.Parse(time.RFC3339Nano, raw)
		if ts.IsZero() {
			// Meta's typical created_time format (no fractional seconds):
			// "2026-04-20T18:47:30+0000"
			ts, _ = time.Parse("2006-01-02T15:04:05-0700", raw)
		}
		if ts.IsZero() {
			ts = time.Now()
		}
		return ts
	}

	entries := make([]InboxEntry, 0, len(result.Data))
	for _, c := range result.Data {
		parentID := postExternalID
		if c.Parent.ID != "" {
			parentID = c.Parent.ID
		}
		entries = append(entries, InboxEntry{
			ExternalID:       c.ID,
			ParentExternalID: parentID,
			AuthorID:         c.From.ID,
			AuthorName:       c.From.Name,
			AuthorAvatarURL:  c.From.Picture.Data.URL,
			Body:             c.Message,
			Timestamp:        parseFacebookCommentTime(c.CreatedTime),
			Source:           "fb_comment",
		})
		for _, reply := range c.Comments.Data {
			replyParentID := c.ID
			if reply.Parent.ID != "" {
				replyParentID = reply.Parent.ID
			}
			entries = append(entries, InboxEntry{
				ExternalID:       reply.ID,
				ParentExternalID: replyParentID,
				AuthorID:         reply.From.ID,
				AuthorName:       reply.From.Name,
				AuthorAvatarURL:  reply.From.Picture.Data.URL,
				Body:             reply.Message,
				Timestamp:        parseFacebookCommentTime(reply.CreatedTime),
				Source:           "fb_comment",
			})
		}
	}
	return entries, nil
}

// FetchCommentAuthor looks up a single comment by ID and returns the
// author identity fields when Meta exposes them. Used as a best-effort
// enrichment path for webhook rows that arrived without sender metadata.
func (a *FacebookAdapter) FetchCommentAuthor(ctx context.Context, accessToken, commentExternalID string) (*FacebookCommentAuthor, error) {
	params := url.Values{
		"access_token": {accessToken},
		"fields":       {"from{id,name,picture{url}}"},
	}
	endpoint := facebookGraphBase + "/" + commentExternalID + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook fetch comment author: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook fetch comment author %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		From struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Picture struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			} `json:"picture"`
		} `json:"from"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("facebook fetch comment author decode: %w", err)
	}
	if parsed.From.ID == "" && parsed.From.Name == "" && parsed.From.Picture.Data.URL == "" {
		return nil, nil
	}
	return &FacebookCommentAuthor{
		ID:        parsed.From.ID,
		Name:      parsed.From.Name,
		AvatarURL: parsed.From.Picture.Data.URL,
	}, nil
}

// FetchConversations returns recent Messenger messages across all
// conversations the Page participates in. One nested Graph call
// pulls the latest 25 messages per conversation along with the
// participant metadata so the sync loop doesn't need an extra
// round trip to resolve names and avatars. `platform=messenger`
// scopes the list to Messenger (Instagram DMs on linked accounts
// live under the same endpoint family).
//
// Returns one InboxEntry per message, matching the pattern IG's
// adapter uses. The sync worker picks own/not-own by comparing
// AuthorID against the stored Page id.
func (a *FacebookAdapter) FetchConversations(ctx context.Context, accessToken string) ([]InboxEntry, error) {
	pageID, err := a.fetchPageSelfID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	params := url.Values{
		"access_token": {accessToken},
		"platform":     {"messenger"},
		"fields":       {"id,participants{id,name,picture{url}},messages.limit(25){id,message,from,created_time}"},
		"limit":        {"25"},
	}
	endpoint := facebookGraphBase + "/" + pageID + "/conversations?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook fetch conversations: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("facebook fetch conversations failed",
			"status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("facebook fetch conversations %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID           string `json:"id"`
			Participants struct {
				Data []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Picture struct {
						Data struct {
							URL string `json:"url"`
						} `json:"data"`
					} `json:"picture"`
				} `json:"data"`
			} `json:"participants"`
			Messages struct {
				Data []struct {
					ID          string `json:"id"`
					Message     string `json:"message"`
					CreatedTime string `json:"created_time"`
					From        struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"from"`
				} `json:"data"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("facebook fetch conversations decode: %w", err)
	}

	entries := make([]InboxEntry, 0, len(result.Data)*5)
	for _, conv := range result.Data {
		// Index participants so each message can look up its sender's
		// avatar without a second request. Page itself appears here
		// too but we just skip it when we see our own id.
		participantIndex := make(map[string]struct {
			Name   string
			Avatar string
		}, len(conv.Participants.Data))
		for _, p := range conv.Participants.Data {
			participantIndex[p.ID] = struct {
				Name   string
				Avatar string
			}{Name: p.Name, Avatar: p.Picture.Data.URL}
		}

		for _, msg := range conv.Messages.Data {
			ts, _ := time.Parse(time.RFC3339Nano, msg.CreatedTime)
			if ts.IsZero() {
				ts, _ = time.Parse("2006-01-02T15:04:05-0700", msg.CreatedTime)
			}
			if ts.IsZero() {
				ts = time.Now()
			}
			name := msg.From.Name
			avatar := ""
			if p, ok := participantIndex[msg.From.ID]; ok {
				if name == "" {
					name = p.Name
				}
				avatar = p.Avatar
			}
			entries = append(entries, InboxEntry{
				ExternalID:       msg.ID,
				ParentExternalID: conv.ID,
				AuthorID:         msg.From.ID,
				AuthorName:       name,
				AuthorAvatarURL:  avatar,
				Body:             msg.Message,
				Timestamp:        ts,
				Source:           "fb_dm",
			})
		}
	}
	return entries, nil
}

// ResolveDMRecipient returns the PSID of the non-Page participant
// in a conversation. Used by the reply path as a fallback when the
// clicked inbox item doesn't carry an author id we can send to
// (e.g. the user is replying to their own last message).
func (a *FacebookAdapter) ResolveDMRecipient(ctx context.Context, accessToken string, conversationID string) (string, error) {
	pageID, err := a.fetchPageSelfID(ctx, accessToken)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"access_token": {accessToken},
		"fields":       {"participants{id,name}"},
	}
	endpoint := facebookGraphBase + "/" + conversationID + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("facebook resolve dm recipient: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facebook resolve dm recipient %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Participants struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"participants"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("facebook resolve dm recipient decode: %w", err)
	}
	for _, p := range parsed.Participants.Data {
		if p.ID != "" && p.ID != pageID {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("recipient not found for conversation %s", conversationID)
}

// SendDM sends a Messenger message from the Page to a user. Meta's
// 24-hour window applies: if the user hasn't messaged the Page in
// the last day, this call will return an error. The dashboard
// disables the Send button client-side in that case so users don't
// see a cryptic rejection — see the fb_dm reply gate in inbox UI.
func (a *FacebookAdapter) SendDM(ctx context.Context, accessToken string, recipientPSID string, text string) (*PostResult, error) {
	pageID, err := a.fetchPageSelfID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"recipient":      map[string]string{"id": recipientPSID},
		"messaging_type": "RESPONSE",
		"message":        map[string]string{"text": text},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := facebookGraphBase + "/" + pageID + "/messages?access_token=" + url.QueryEscape(accessToken)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook send dm: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, wrapFacebookPublishError(resp.StatusCode, respBody)
	}
	var parsed struct {
		MessageID   string `json:"message_id"`
		RecipientID string `json:"recipient_id"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return &PostResult{ExternalID: parsed.MessageID}, nil
}

// ReplyToComment posts a reply to a Page comment. The reply is
// authored by the Page itself (since the Page token is the
// actor), and the returned id is the new comment's own id.
func (a *FacebookAdapter) ReplyToComment(ctx context.Context, accessToken string, commentExternalID string, text string) (*PostResult, error) {
	form := url.Values{
		"access_token": {accessToken},
		"message":      {text},
	}
	endpoint := facebookGraphBase + "/" + commentExternalID + "/comments"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook reply comment: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, wrapFacebookPublishError(resp.StatusCode, body)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("facebook reply comment decode: %w", err)
	}
	if result.ID == "" {
		return nil, fmt.Errorf("facebook reply comment: response missing id (body=%s)", string(body))
	}
	return &PostResult{
		ExternalID: result.ID,
		// FB's reply URLs resolve via the parent comment path; we
		// don't try to construct a canonical URL here because comment
		// URLs on Pages require the enclosing post_id + comment_id.
		// Callers that need the link can derive it from the inbox
		// item's parent post.
	}, nil
}

// FetchMediaDetails returns post details for the inbox's media-context
// panel. The inbox handler dispatches to this for `fb_comment` items
// so the conversation view can render the parent post's caption +
// permalink alongside the comment thread. Full picture is exposed
// for photo/video posts; link posts surface the preview via
// attachments[0].media.image.src which Facebook returns for both.
func (a *FacebookAdapter) FetchMediaDetails(ctx context.Context, accessToken, postID string) (*MediaDetails, error) {
	params := url.Values{
		"access_token": {accessToken},
		"fields":       {"id,message,permalink_url,full_picture,created_time,attachments{media{image{src}},media_type}"},
	}
	endpoint := facebookGraphBase + "/" + postID + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook fetch media details: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, wrapFacebookPublishError(resp.StatusCode, body)
	}

	var parsed struct {
		ID           string `json:"id"`
		Message      string `json:"message"`
		PermalinkURL string `json:"permalink_url"`
		FullPicture  string `json:"full_picture"`
		CreatedTime  string `json:"created_time"`
		Attachments  struct {
			Data []struct {
				MediaType string `json:"media_type"`
				Media     struct {
					Image struct {
						Src string `json:"src"`
					} `json:"image"`
				} `json:"media"`
			} `json:"data"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("facebook fetch media details decode: %w", err)
	}

	mediaURL := parsed.FullPicture
	mediaType := "POST"
	if len(parsed.Attachments.Data) > 0 {
		att := parsed.Attachments.Data[0]
		if att.Media.Image.Src != "" && mediaURL == "" {
			mediaURL = att.Media.Image.Src
		}
		switch strings.ToLower(att.MediaType) {
		case "photo":
			mediaType = "IMAGE"
		case "video", "video_inline", "video_autoplay":
			mediaType = "VIDEO"
		case "share":
			mediaType = "LINK"
		}
	}
	return &MediaDetails{
		ID:        parsed.ID,
		Caption:   parsed.Message,
		MediaURL:  mediaURL,
		Timestamp: parsed.CreatedTime,
		MediaType: mediaType,
		Permalink: parsed.PermalinkURL,
	}, nil
}

// SubscribePageToWebhooks subscribes our Meta App to the Page's feed +
// Messenger events. Without this call, Meta only delivers webhooks to
// the Page owner's own App — our App receives nothing. Called once per
// Page right after connect finalize; idempotent server-side.
//
// Fields:
//   - feed: new posts, comments, reactions on the Page
//   - messages / messaging_postbacks: Messenger inbound + quick replies
//
// Errors are surfaced so the caller can log them, but they should not
// block connect finalize — the Page is still usable for publishing,
// and the user can re-trigger subscription by reconnecting.
func (a *FacebookAdapter) SubscribePageToWebhooks(ctx context.Context, pageAccessToken, pageID string) error {
	form := url.Values{
		"access_token":      {pageAccessToken},
		"subscribed_fields": {"feed,messages,messaging_postbacks"},
	}
	endpoint := facebookGraphBase + "/" + pageID + "/subscribed_apps"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("facebook subscribe webhooks: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return wrapFacebookPublishError(resp.StatusCode, body)
	}

	var parsed struct {
		Success bool `json:"success"`
	}
	_ = json.Unmarshal(body, &parsed)
	if !parsed.Success {
		// Meta returns {"success": true} on acceptance; anything else
		// (even a 200) is suspect enough to log at the caller.
		return fmt.Errorf("facebook subscribe webhooks: unexpected response body=%s", string(body))
	}
	return nil
}
