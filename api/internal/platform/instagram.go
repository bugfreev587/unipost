package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/debugrt"
)

type InstagramAdapter struct {
	client *http.Client
}

var (
	instagramContainerPollAttempts = 30
	instagramContainerPollInterval = 2 * time.Second
)

func NewInstagramAdapter() *InstagramAdapter {
	return &InstagramAdapter{client: debugrt.NewClient(60 * time.Second)}
}

func (a *InstagramAdapter) Platform() string { return "instagram" }

func (a *InstagramAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("INSTAGRAM_APP_ID"),
		ClientSecret: os.Getenv("INSTAGRAM_APP_SECRET"),
		AuthURL:      "https://www.instagram.com/oauth/authorize",
		TokenURL:     "https://api.instagram.com/oauth/access_token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/instagram",
		Scopes: []string{
			"instagram_business_basic",
			"instagram_business_content_publish",
			"instagram_business_manage_insights",
			"instagram_business_manage_comments",
			"instagram_business_manage_messages",
		},
	}
}

func (a *InstagramAdapter) GetAuthURL(config OAuthConfig, state string) string {
	params := url.Values{
		"client_id":       {config.ClientID},
		"redirect_uri":    {config.RedirectURL},
		"response_type":   {"code"},
		"state":           {state},
		"scope":           {strings.Join(config.Scopes, ",")},
		"enable_fb_login": {"0"},
	}
	return config.AuthURL + "?" + params.Encode()
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

	// Exchange for long-lived token (60 days) via Instagram API
	longToken, expiresIn, err := a.exchangeForLongLivedToken(ctx, config, tokenResp.AccessToken)
	if err != nil {
		longToken = tokenResp.AccessToken
		expiresIn = 3600
	}

	// Get Instagram profile directly via Instagram Graph API
	profile, err := a.getProfile(ctx, longToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get Instagram profile: %w", err)
	}

	return &ConnectResult{
		AccessToken:       longToken,
		RefreshToken:      longToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		ExternalAccountID: profile.id,
		AccountName:       profile.username,
		AvatarURL:         profile.profilePicURL,
		Scopes:            config.Scopes,
		Metadata: map[string]any{
			"ig_user_id":                profile.id,
			"instagram_webhook_user_id": profile.webhookUserID,
			"username":                  profile.username,
			"granted_scopes":            config.Scopes,
		},
	}, nil
}

func (a *InstagramAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("instagram requires OAuth flow, use POST /v1/oauth/connect")
}

// Post publishes to Instagram using the two-step container flow.
func (a *InstagramAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	igUserID, err := a.getIGUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if len(media) == 0 {
		return nil, fmt.Errorf("instagram requires at least one media item")
	}
	if len(media) > 10 {
		return nil, fmt.Errorf("instagram carousels accept at most 10 items")
	}

	mediaType := instagramPublishType(opts)

	// Build a creation container per the IG Graph API rules:
	//   - 1 image    → media_type=IMAGE (default), image_url
	//   - 1 video    → media_type=REELS or STORIES depending on selection
	//   - 2+ items   → media_type=CAROUSEL, children=[item1,item2,...] where
	//                  each child container is created beforehand with
	//                  is_carousel_item=true.
	var creationID string
	if resume := resumePublishToken(opts); resume != "" {
		// Idempotent resume: a prior attempt already created (and uploaded)
		// this container but may have crashed before media_publish. Re-use it
		// instead of building a new one, so we publish the same container once
		// rather than duplicating the post.
		creationID = resume
		slog.Info("instagram post: resuming from persisted container", "creation_id", creationID)
	} else {
		switch mediaType {
		case "story":
			if len(media) != 1 {
				return nil, fmt.Errorf("instagram stories require exactly one image or video")
			}
			creationID, err = a.createSingleContainer(ctx, accessToken, igUserID, text, media[0], false, mediaType)
		case "reels":
			if len(media) != 1 {
				return nil, fmt.Errorf("instagram reels require exactly one video")
			}
			creationID, err = a.createSingleContainer(ctx, accessToken, igUserID, text, media[0], false, mediaType)
		default:
			switch {
			case len(media) == 1:
				creationID, err = a.createSingleContainer(ctx, accessToken, igUserID, text, media[0], false, mediaType)
			default:
				creationID, err = a.createCarouselContainer(ctx, accessToken, igUserID, text, media)
			}
		}
		if err != nil {
			return nil, err
		}
		// Persist the container id before publishing so a crash between here
		// and media_publish resumes with this same container.
		persistPublishToken(opts, creationID)
	}

	if err := a.waitForContainer(ctx, accessToken, creationID); err != nil {
		return nil, err
	}

	// Publish the (parent) container.
	publishURL := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/media_publish?creation_id=%s&access_token=%s",
		igUserID, creationID, accessToken)
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

	// Fetch the permalink — IG URLs use shortcodes, not numeric IDs.
	permalink := ""
	if published.ID != "" {
		permReq, _ := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("https://graph.instagram.com/v21.0/%s?fields=permalink&access_token=%s", published.ID, accessToken), nil)
		if permResp, permErr := a.client.Do(permReq); permErr == nil {
			defer permResp.Body.Close()
			var permData struct {
				Permalink string `json:"permalink"`
			}
			json.NewDecoder(permResp.Body).Decode(&permData)
			permalink = permData.Permalink
		}
	}
	if permalink == "" {
		permalink = fmt.Sprintf("https://www.instagram.com/p/%s/", published.ID)
	}

	return &PostResult{
		ExternalID: published.ID,
		URL:        permalink,
	}, nil
}

func instagramPublishType(opts map[string]any) string {
	value := strings.TrimSpace(strings.ToLower(optString(opts, "mediaType")))
	if value == "" {
		value = strings.TrimSpace(strings.ToLower(optString(opts, "media_type")))
	}
	switch value {
	case "story", "stories":
		return "story"
	case "reel", "reels":
		return "reels"
	default:
		return "feed"
	}
}

// PostComment publishes a comment on an existing Instagram post,
// used by the Sprint 4 PR3 first_comment feature. Hits the Graph API
// /{ig-media-id}/comments endpoint with the page access token. The
// comment appears as a top-level reply on the parent post and is
// authored by the same Instagram Business account.
func (a *InstagramAdapter) PostComment(ctx context.Context, accessToken string, parentExternalID string, text string) (*PostResult, error) {
	// Graph API: POST /v21.0/{media-id}/comments?message=...&access_token=...
	commentURL := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/comments?message=%s&access_token=%s",
		parentExternalID,
		url.QueryEscape(text),
		accessToken,
	)
	req, err := http.NewRequestWithContext(ctx, "POST", commentURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram post_comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("instagram post_comment (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return &PostResult{
		ExternalID: result.ID,
		URL:        fmt.Sprintf("https://www.instagram.com/p/%s/", parentExternalID),
	}, nil
}

// FetchComments returns comments on an Instagram media object.
// Uses the "Instagram API with Instagram Login" fields — the `from`
// expansion is NOT available on this API surface, so we request only
// id, text, username, timestamp. The `username` field identifies
// the comment author.
func (a *InstagramAdapter) FetchComments(ctx context.Context, accessToken string, mediaExternalID string) ([]InboxEntry, error) {
	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/comments?fields=id,text,username,timestamp&access_token=%s",
		mediaExternalID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram fetch comments: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("instagram fetch comments failed",
			"status", resp.StatusCode,
			"media_id", mediaExternalID,
			"body", string(body))
		return nil, fmt.Errorf("instagram fetch comments %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Username  string `json:"username"`
			Timestamp string `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("instagram fetch comments decode: %w", err)
	}

	entries := make([]InboxEntry, 0, len(result.Data))
	for _, c := range result.Data {
		ts, _ := time.Parse(time.RFC3339Nano, c.Timestamp)
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05-0700", c.Timestamp)
		}
		if ts.IsZero() {
			ts = time.Now()
		}
		entries = append(entries, InboxEntry{
			ExternalID:       c.ID,
			ParentExternalID: mediaExternalID,
			AuthorName:       c.Username,
			Body:             c.Text,
			Timestamp:        ts,
			Source:           "ig_comment",
		})
	}
	return entries, nil
}

// ReplyToComment replies to an Instagram comment.
// POST /v21.0/{comment-id}/replies?message=...
func (a *InstagramAdapter) ReplyToComment(ctx context.Context, accessToken string, commentExternalID string, text string) (*PostResult, error) {
	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/replies?message=%s&access_token=%s",
		commentExternalID, url.QueryEscape(text), accessToken)
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram reply to comment: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram reply to comment %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &result)
	return &PostResult{ExternalID: result.ID}, nil
}

// FetchConversations returns recent Instagram DM messages.
// GET /v21.0/{ig-user-id}/conversations?fields=participants,messages{id,message,from,created_time}
func (a *InstagramAdapter) FetchConversations(ctx context.Context, accessToken string) ([]InboxEntry, error) {
	igUserID, err := a.getIGUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/conversations?fields=id,messages{id,message,from,created_time}&platform=instagram&access_token=%s",
		igUserID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram fetch conversations: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("instagram fetch conversations failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("instagram fetch conversations %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Messages struct {
				Data []struct {
					ID          string `json:"id"`
					Message     string `json:"message"`
					CreatedTime string `json:"created_time"`
					From        struct {
						Username string `json:"username"`
						ID       string `json:"id"`
					} `json:"from"`
				} `json:"data"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("instagram fetch conversations decode: %w", err)
	}

	var entries []InboxEntry
	for _, conv := range result.Data {
		for _, msg := range conv.Messages.Data {
			ts, _ := time.Parse("2006-01-02T15:04:05-0700", msg.CreatedTime)
			isOwn := msg.From.ID == igUserID
			entries = append(entries, InboxEntry{
				ExternalID:       msg.ID,
				ParentExternalID: conv.ID,
				AuthorName:       msg.From.Username,
				AuthorID:         msg.From.ID,
				Body:             msg.Message,
				Timestamp:        ts,
				Source:           "ig_dm",
			})
			_ = isOwn // caller determines is_own by comparing AuthorID to account's external_account_id
		}
	}
	return entries, nil
}

// ResolveDMRecipient looks up the participant ids for a specific
// Instagram DM conversation and returns the recipient id that is not
// the connected business account itself.
func (a *InstagramAdapter) ResolveDMRecipient(ctx context.Context, accessToken string, conversationID string) (string, error) {
	igUserID, err := a.getIGUserID(ctx, accessToken)
	if err != nil {
		return "", err
	}

	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/conversations?fields=id,participants{id,username}&platform=instagram&access_token=%s",
		igUserID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("instagram resolve dm recipient: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("instagram resolve dm recipient %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID           string `json:"id"`
			Participants struct {
				Data []struct {
					ID       string `json:"id"`
					Username string `json:"username"`
				} `json:"data"`
			} `json:"participants"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("instagram resolve dm recipient decode: %w", err)
	}

	for _, conv := range result.Data {
		if conv.ID != conversationID {
			continue
		}
		for _, participant := range conv.Participants.Data {
			if participant.ID != "" && participant.ID != igUserID {
				return participant.ID, nil
			}
		}
	}

	return "", fmt.Errorf("recipient not found for conversation %s", conversationID)
}

// SendDM sends a direct message on Instagram.
// POST /v21.0/{ig-user-id}/messages
func (a *InstagramAdapter) SendDM(ctx context.Context, accessToken string, recipientID string, text string) (*PostResult, error) {
	igUserID, err := a.getIGUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	slog.Info("instagram send dm",
		"ig_user_id", igUserID, "recipient_id", recipientID)

	params := url.Values{
		"recipient":    {fmt.Sprintf(`{"id":"%s"}`, recipientID)},
		"message":      {fmt.Sprintf(`{"text":"%s"}`, text)},
		"access_token": {accessToken},
	}
	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/messages", igUserID)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram send dm: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram send dm %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		MessageID string `json:"message_id"`
	}
	json.Unmarshal(body, &result)
	return &PostResult{ExternalID: result.MessageID}, nil
}

// MediaDetails holds basic info about an Instagram media object.
type MediaDetails struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	MediaURL  string `json:"media_url"`
	Timestamp string `json:"timestamp"`
	MediaType string `json:"media_type"`
	Permalink string `json:"permalink"`
}

// FetchMediaDetails returns details about a specific media object.
func (a *InstagramAdapter) FetchMediaDetails(ctx context.Context, accessToken string, mediaID string) (*MediaDetails, error) {
	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s?fields=id,caption,media_url,timestamp,media_type,permalink&access_token=%s",
		mediaID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram fetch media details: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram fetch media details %d: %s", resp.StatusCode, string(body))
	}
	var details MediaDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, err
	}
	return &details, nil
}

// FetchRaw makes a raw GET request to the IG Graph API and returns the response body.
func (a *InstagramAdapter) FetchRaw(ctx context.Context, accessToken string, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url+"&access_token="+accessToken, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ig api %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// FetchRecentMedia returns the IDs of the account's recent posts
// directly from the IG API. This covers posts published natively
// on Instagram, not just those published through UniPost.
func (a *InstagramAdapter) FetchRecentMedia(ctx context.Context, accessToken string) ([]string, error) {
	igUserID, err := a.getIGUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/media?fields=id&limit=10&access_token=%s",
		igUserID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram fetch recent media: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram fetch recent media %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// createSingleContainer creates a non-carousel media container. caption is
// only attached when isCarouselChild is false (children inherit it from the
// parent CAROUSEL container).
func (a *InstagramAdapter) createSingleContainer(ctx context.Context, accessToken, igUserID, caption string, item MediaItem, isCarouselChild bool, mediaType string) (string, error) {
	params := url.Values{
		"access_token": {accessToken},
	}
	if isCarouselChild {
		params.Set("is_carousel_item", "true")
	} else if mediaType != "story" {
		params.Set("caption", caption)
	}

	kind := item.Kind
	if kind == MediaKindUnknown {
		kind = SniffMediaKind(item.URL)
	}

	switch kind {
	case MediaKindVideo:
		switch mediaType {
		case "story":
			params.Set("media_type", "STORIES")
		default:
			// Meta no longer supports plain VIDEO container for standalone video publishing.
			params.Set("media_type", "REELS")
			if mediaType == "feed" {
				params.Set("share_to_feed", "true")
			} else if mediaType == "reels" {
				params.Set("share_to_feed", "false")
			}
		}
		params.Set("video_url", item.URL)
	default:
		if mediaType == "story" {
			params.Set("media_type", "STORIES")
		}
		params.Set("image_url", item.URL)
	}

	containerURL := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/media?%s", igUserID, params.Encode())
	resp, err := a.client.Post(containerURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("container creation failed (%d): %s", resp.StatusCode, string(body))
	}

	var container struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", err
	}
	return container.ID, nil
}

// createCarouselContainer builds the per-child containers (waiting for each
// to finish), then assembles them into a CAROUSEL parent container.
func (a *InstagramAdapter) createCarouselContainer(ctx context.Context, accessToken, igUserID, caption string, items []MediaItem) (string, error) {
	childIDs := make([]string, 0, len(items))
	for _, item := range items {
		id, err := a.createSingleContainer(ctx, accessToken, igUserID, caption, item, true, "feed")
		if err != nil {
			return "", err
		}
		// IG requires each child container to be ready before the parent can
		// reference it.
		if err := a.waitForContainer(ctx, accessToken, id); err != nil {
			return "", err
		}
		childIDs = append(childIDs, id)
	}

	params := url.Values{
		"access_token": {accessToken},
		"caption":      {caption},
		"media_type":   {"CAROUSEL"},
		"children":     {strings.Join(childIDs, ",")},
	}
	carouselURL := fmt.Sprintf("https://graph.instagram.com/v21.0/%s/media?%s", igUserID, params.Encode())
	resp, err := a.client.Post(carouselURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create carousel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("carousel creation failed (%d): %s", resp.StatusCode, string(body))
	}

	var container struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", err
	}
	return container.ID, nil
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
		"grant_type":    {"ig_exchange_token"},
		"client_secret": {config.ClientSecret},
		"access_token":  {shortToken},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.instagram.com/access_token?"+params.Encode(), nil)
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
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("empty access token in long-lived exchange")
	}

	return result.AccessToken, result.ExpiresIn, nil
}

type igProfile struct {
	id            string
	webhookUserID string
	username      string
	profilePicURL string
}

type InstagramProfile struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	ProfilePictureURL string `json:"profile_picture_url"`
	FollowersCount    int64  `json:"followers_count"`
	FollowsCount      int64  `json:"follows_count"`
	MediaCount        int64  `json:"media_count"`
}

func (a *InstagramAdapter) getProfile(ctx context.Context, accessToken string) (*igProfile, error) {
	params := url.Values{}
	params.Set("fields", "id,user_id,username,profile_picture_url")
	params.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.instagram.com/v21.0/me?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get profile (%d): %s", resp.StatusCode, string(body))
	}

	var profile struct {
		ID                string `json:"id"`
		UserID            string `json:"user_id"`
		Username          string `json:"username"`
		ProfilePictureURL string `json:"profile_picture_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	if profile.ID == "" {
		return nil, fmt.Errorf("empty profile ID")
	}
	if profile.UserID == "" {
		return nil, fmt.Errorf("empty profile user_id")
	}

	return &igProfile{
		id:            profile.ID,
		webhookUserID: profile.UserID,
		username:      profile.Username,
		profilePicURL: profile.ProfilePictureURL,
	}, nil
}

func (a *InstagramAdapter) FetchProfile(ctx context.Context, accessToken string) (*InstagramProfile, error) {
	params := url.Values{}
	params.Set("fields", "id,username,profile_picture_url,followers_count,follows_count,media_count")
	params.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.instagram.com/v22.0/me?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram profile: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram profile (%d): %s", resp.StatusCode, string(body))
	}

	var profile InstagramProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("instagram profile decode: %w", err)
	}
	if profile.ID == "" {
		return nil, fmt.Errorf("instagram profile: empty id")
	}
	return &profile, nil
}

func (a *InstagramAdapter) GetAccountMetrics(ctx context.Context, accessToken, externalAccountID string) (*AccountMetrics, error) {
	profile, err := a.FetchProfile(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &AccountMetrics{
		FollowerCount:  profile.FollowersCount,
		FollowingCount: profile.FollowsCount,
		PostCount:      profile.MediaCount,
		PlatformSpecific: map[string]any{
			"media_count": profile.MediaCount,
		},
	}, nil
}

type InstagramMedia struct {
	ID            string `json:"id"`
	Caption       string `json:"caption"`
	MediaType     string `json:"media_type"`
	MediaURL      string `json:"media_url"`
	ThumbnailURL  string `json:"thumbnail_url"`
	Permalink     string `json:"permalink"`
	Timestamp     string `json:"timestamp"`
	LikeCount     int64  `json:"like_count"`
	CommentsCount int64  `json:"comments_count"`
}

type InstagramMediaList struct {
	Media []InstagramMedia `json:"media"`
}

func (a *InstagramAdapter) ListMedia(ctx context.Context, accessToken string, limit int) (*InstagramMediaList, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	params := url.Values{}
	params.Set("fields", "id,caption,media_type,media_url,thumbnail_url,permalink,timestamp,like_count,comments_count")
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.instagram.com/v22.0/me/media?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram media list: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram media list (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []InstagramMedia `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("instagram media list decode: %w", err)
	}
	if result.Data == nil {
		result.Data = []InstagramMedia{}
	}
	return &InstagramMediaList{Media: result.Data}, nil
}

func (a *InstagramAdapter) getIGUserID(ctx context.Context, accessToken string) (string, error) {
	params := url.Values{}
	params.Set("fields", "id")
	params.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.instagram.com/v21.0/me?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyText := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("instagram get user id failed (%d): %s", resp.StatusCode, bodyText)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("instagram get user id decode failed: %w", err)
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", fmt.Errorf("instagram get user id missing id: %s", bodyText)
	}
	return result.ID, nil
}

// GetAnalytics fetches post metrics from Instagram Insights API.
//
// Important: as of Instagram Graph API v22 (April 2024), the `impressions`
// metric was removed for IMAGE / CAROUSEL_ALBUM media. Requesting it returns
// HTTP 400 for the WHOLE call (not just the bad metric), which previously
// caused every image-post fetch to fail silently and write all-zero rows.
//
// We now request only the metric set that works across all media types in
// v22+, and return a real error on non-200 so the handler/worker skips the
// upsert instead of poisoning the cache. Impressions stay at 0 for IG (Meta
// no longer exposes them at the media level for organic content), which the
// dashboard renders as "--".
func (a *InstagramAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	metricsURL := fmt.Sprintf(
		"https://graph.instagram.com/v22.0/%s/insights?metric=reach,likes,comments,shares,saved&access_token=%s",
		externalID, accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", metricsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram insights request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("instagram insights non-200",
			"status", resp.StatusCode,
			"external_id", externalID,
			"body", string(body))
		return nil, fmt.Errorf("instagram insights returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value int64 `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("instagram insights decode failed: %w", err)
	}

	m := &PostMetrics{}
	for _, metric := range result.Data {
		val := int64(0)
		if len(metric.Values) > 0 {
			val = metric.Values[0].Value
		}
		switch metric.Name {
		case "reach":
			m.Reach = val
		case "likes":
			m.Likes = val
		case "comments":
			m.Comments = val
		case "shares":
			m.Shares = val
		case "saved":
			m.Saves = val
		}
	}

	// EngagementRate is computed by the analytics handler from the unified
	// formula in PRD §9.1; do not set it here.
	return m, nil
}

type instagramContainerPollDiagnostics struct {
	containerID    string
	pollCount      int
	elapsed        time.Duration
	lastHTTPStatus int
	statusCode     string
	responseBody   string
}

func (d instagramContainerPollDiagnostics) format() string {
	parts := []string{
		"container_id=" + d.containerID,
		fmt.Sprintf("poll_count=%d", d.pollCount),
		fmt.Sprintf("elapsed_ms=%d", d.elapsed.Milliseconds()),
	}
	if d.lastHTTPStatus != 0 {
		parts = append(parts, fmt.Sprintf("last_http_status=%d", d.lastHTTPStatus))
	}
	if d.statusCode != "" {
		parts = append(parts, "status_code="+d.statusCode)
	}
	if d.responseBody != "" {
		parts = append(parts, "response_body="+truncateForLog(d.responseBody, 1000))
	}
	return strings.Join(parts, " ")
}

func (a *InstagramAdapter) waitForContainer(ctx context.Context, accessToken string, containerID string) error {
	start := time.Now()
	diag := instagramContainerPollDiagnostics{containerID: containerID}

	for i := 0; i < instagramContainerPollAttempts; i++ {
		if instagramContainerPollInterval > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(instagramContainerPollInterval):
			}
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		diag.pollCount = i + 1
		diag.elapsed = time.Since(start)

		req, _ := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("https://graph.instagram.com/v21.0/%s?fields=status_code&access_token=%s", containerID, accessToken), nil)
		resp, err := a.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		diag.lastHTTPStatus = resp.StatusCode
		diag.responseBody = strings.TrimSpace(string(body))

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var status struct {
			StatusCode string `json:"status_code"`
		}
		if err := json.Unmarshal(body, &status); err != nil {
			return fmt.Errorf("instagram container poll decode failed: %s decode_error=%v", diag.format(), err)
		}
		diag.statusCode = strings.TrimSpace(status.StatusCode)

		if status.StatusCode == "FINISHED" {
			return nil
		}
		if status.StatusCode == "ERROR" {
			return fmt.Errorf("instagram container processing failed: %s", diag.format())
		}
	}
	diag.elapsed = time.Since(start)
	return fmt.Errorf("instagram container processing timed out: %s", diag.format())
}
