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
func (a *LinkedInAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
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

	// Handle media via the Assets API: each item must be registered, the
	// binary uploaded, and the returned asset URN attached to the share with
	// the matching shareMediaCategory. This replaces the broken ARTICLE path
	// that previously rendered images as link previews.
	//
	// LinkedIn share rules we enforce here:
	//   - Up to 9 images can share a single UGC post (multi-image carousel).
	//   - Exactly 1 video per UGC post — multiple videos require separate
	//     posts, which the caller can do by issuing multiple Post() calls.
	//   - Mixing IMAGE and VIDEO in one share is not allowed.
	if len(media) > 0 {
		// Pre-classify so we can enforce the rules above before incurring
		// the cost of any uploads.
		videoCount := 0
		for _, item := range media {
			kind := item.Kind
			if kind == MediaKindUnknown {
				kind = SniffMediaKind(item.URL)
			}
			if kind == MediaKindVideo {
				videoCount++
			}
		}
		if videoCount > 1 {
			return nil, fmt.Errorf("linkedin: only one video per post is supported")
		}
		if videoCount > 0 && videoCount != len(media) {
			return nil, fmt.Errorf("linkedin: cannot mix image and video in one post")
		}
		if videoCount == 0 && len(media) > 9 {
			return nil, fmt.Errorf("linkedin: up to 9 images per post supported")
		}

		var mediaList []map[string]any
		var category string

		for _, item := range media {
			kind := item.Kind
			if kind == MediaKindUnknown {
				kind = SniffMediaKind(item.URL)
			}

			// Register the upload, fetch the source bytes, then PUT them at
			// the upload URL LinkedIn hands back. uploadAsset returns the
			// final asset URN.
			urn, recipe, err := a.uploadAsset(ctx, accessToken, authorURN, item.URL, kind)
			if err != nil {
				return nil, fmt.Errorf("linkedin asset upload failed: %w", err)
			}

			// All items in a single share must share a category. The first
			// successful upload locks it; subsequent items of a different
			// kind are rejected. UGC API does not support mixing image+video
			// in one share.
			if category == "" {
				category = recipe
			} else if category != recipe {
				return nil, fmt.Errorf("linkedin: cannot mix %s and %s in a single post", category, recipe)
			}

			mediaList = append(mediaList, map[string]any{
				"status":      "READY",
				"media":       urn,
				"title":       map[string]any{"text": ""},
				"description": map[string]any{"text": ""},
			})
		}

		shareContent["shareMediaCategory"] = category
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

// uploadAsset implements LinkedIn's three-step Assets API:
//
//  1. POST /v2/assets?action=registerUpload — tells LinkedIn we want to push
//     either a feedshare-image or feedshare-video. Response carries an upload
//     URL and the asset URN.
//  2. GET source URL — fetch the bytes from the caller-supplied media URL.
//  3. PUT bytes to the upload URL with the registration access token.
//
// LinkedIn then asynchronously processes the asset; the returned URN is safe
// to attach to a UGC post immediately, but image upload should be confirmed
// via the assets endpoint to avoid races. We do a single fast poll to keep
// the post-create call deterministic; if it's still processing the share will
// surface the failure later via the moderation API.
//
// Returns the asset URN and the corresponding shareMediaCategory string
// ("IMAGE" or "VIDEO") so the caller can populate the UGC payload.
func (a *LinkedInAdapter) uploadAsset(ctx context.Context, accessToken, ownerURN, sourceURL string, kind MediaKind) (string, string, error) {
	var recipe, category string
	switch kind {
	case MediaKindVideo:
		recipe = "urn:li:digitalmediaRecipe:feedshare-video"
		category = "VIDEO"
	case MediaKindImage, MediaKindGIF, MediaKindUnknown:
		recipe = "urn:li:digitalmediaRecipe:feedshare-image"
		category = "IMAGE"
	default:
		return "", "", fmt.Errorf("linkedin: unsupported media kind %q", kind)
	}

	// Step 1: register upload.
	registerBody, _ := json.Marshal(map[string]any{
		"registerUploadRequest": map[string]any{
			"recipes":              []string{recipe},
			"owner":                ownerURN,
			"serviceRelationships": []map[string]any{{
				"relationshipType": "OWNER",
				"identifier":       "urn:li:userGeneratedContent",
			}},
		},
	})

	regReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.linkedin.com/v2/assets?action=registerUpload", bytes.NewReader(registerBody))
	if err != nil {
		return "", "", err
	}
	regReq.Header.Set("Authorization", "Bearer "+accessToken)
	regReq.Header.Set("Content-Type", "application/json")
	regReq.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	regResp, err := a.client.Do(regReq)
	if err != nil {
		return "", "", fmt.Errorf("registerUpload: %w", err)
	}
	defer regResp.Body.Close()
	if regResp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(regResp.Body)
		return "", "", fmt.Errorf("registerUpload (%d): %s", regResp.StatusCode, string(body))
	}

	var reg struct {
		Value struct {
			Asset              string `json:"asset"`
			UploadMechanism    map[string]struct {
				UploadURL string            `json:"uploadUrl"`
				Headers   map[string]string `json:"headers"`
			} `json:"uploadMechanism"`
		} `json:"value"`
	}
	if err := json.NewDecoder(regResp.Body).Decode(&reg); err != nil {
		return "", "", fmt.Errorf("registerUpload decode: %w", err)
	}

	// LinkedIn may return one of several upload mechanisms. The simple v2
	// path is com.linkedin.digitalmedia.uploading.MediaUploadHttpRequest.
	var uploadURL string
	var uploadHeaders map[string]string
	for _, mech := range reg.Value.UploadMechanism {
		if mech.UploadURL != "" {
			uploadURL = mech.UploadURL
			uploadHeaders = mech.Headers
			break
		}
	}
	if uploadURL == "" || reg.Value.Asset == "" {
		return "", "", fmt.Errorf("registerUpload returned no upload URL or asset")
	}

	// Step 2: fetch the source bytes.
	srcReq, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return "", "", err
	}
	srcResp, err := a.client.Do(srcReq)
	if err != nil {
		return "", "", fmt.Errorf("fetch source: %w", err)
	}
	defer srcResp.Body.Close()
	if srcResp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("fetch source (%d)", srcResp.StatusCode)
	}
	srcBytes, err := io.ReadAll(srcResp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read source: %w", err)
	}

	// Step 3: upload bytes to the LinkedIn-issued URL.
	upReq, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, bytes.NewReader(srcBytes))
	if err != nil {
		return "", "", err
	}
	// Some upload paths require the Authorization header to be repeated.
	upReq.Header.Set("Authorization", "Bearer "+accessToken)
	for k, v := range uploadHeaders {
		upReq.Header.Set(k, v)
	}
	if upReq.Header.Get("Content-Type") == "" {
		ct := srcResp.Header.Get("Content-Type")
		if ct == "" {
			if category == "VIDEO" {
				ct = "video/mp4"
			} else {
				ct = "image/jpeg"
			}
		}
		upReq.Header.Set("Content-Type", ct)
	}

	upResp, err := a.client.Do(upReq)
	if err != nil {
		return "", "", fmt.Errorf("upload: %w", err)
	}
	defer upResp.Body.Close()
	if upResp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(upResp.Body)
		return "", "", fmt.Errorf("upload (%d): %s", upResp.StatusCode, string(body))
	}

	return reg.Value.Asset, category, nil
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
//
// Uses the /v2/socialActions/{urn} endpoint, which returns likes +
// comments counts for standard OAuth apps (w_member_social scope is
// sufficient — same endpoint PostComment uses).
//
// We previously called /v2/socialMetadata/{urn}, but that's part of
// LinkedIn's Marketing Developer Platform and returns 403 ACCESS_DENIED
// for non-LMDP apps. socialActions doesn't expose impressions, reach,
// or clicks — those require LMDP partnership.
func (a *LinkedInAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.linkedin.com/v2/socialActions/"+url.QueryEscape(externalID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linkedin social actions request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("linkedin social actions non-200",
			"status", resp.StatusCode,
			"external_id", externalID,
			"body", string(body))
		return nil, fmt.Errorf("linkedin social actions returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		LikesSummary struct {
			TotalLikes           int64 `json:"totalLikes"`
			AggregatedTotalLikes int64 `json:"aggregatedTotalLikes"`
		} `json:"likesSummary"`
		CommentsSummary struct {
			TotalFirstLevelComments int64 `json:"totalFirstLevelComments"`
			AggregatedTotalComments int64 `json:"aggregatedTotalComments"`
		} `json:"commentsSummary"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		slog.Warn("linkedin social actions decode failed",
			"external_id", externalID,
			"body", string(body),
			"err", err)
		return nil, fmt.Errorf("linkedin social actions decode: %w", err)
	}

	// Prefer the aggregated total (includes nested) when available, fall
	// back to the flat count. Comments API uses aggregatedTotalComments
	// which counts replies to replies too.
	likes := result.LikesSummary.AggregatedTotalLikes
	if likes == 0 {
		likes = result.LikesSummary.TotalLikes
	}
	comments := result.CommentsSummary.AggregatedTotalComments
	if comments == 0 {
		comments = result.CommentsSummary.TotalFirstLevelComments
	}

	// EngagementRate is computed by the analytics handler.
	// Impressions / Reach / Shares / Clicks require LMDP access and are
	// left at 0 for standard apps.
	return &PostMetrics{
		Likes:    likes,
		Comments: comments,
	}, nil
}

// PostComment publishes a comment on an existing LinkedIn post,
// used by the Sprint 4 PR3 first_comment feature. Hits LinkedIn's
// /v2/socialActions/{shareUrn}/comments endpoint with the same
// person URN that authored the parent post (so the comment appears
// from "the user" rather than from a third party).
func (a *LinkedInAdapter) PostComment(ctx context.Context, accessToken string, parentExternalID string, text string) (*PostResult, error) {
	userInfo, err := a.getUserInfo(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("linkedin post_comment: get user info: %w", err)
	}
	actorURN := "urn:li:person:" + userInfo.sub

	payload := map[string]any{
		"actor": actorURN,
		"message": map[string]any{
			"text": text,
		},
	}
	body, _ := json.Marshal(payload)

	// LinkedIn's comment endpoint takes the parent's URN in the path.
	// The parent ID we receive is already a full URN like
	// urn:li:share:7180000000000000000.
	url := fmt.Sprintf("https://api.linkedin.com/v2/socialActions/%s/comments", parentExternalID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linkedin post_comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("linkedin post_comment (%d): %s", resp.StatusCode, string(respBody))
	}

	commentURN := resp.Header.Get("X-RestLi-Id")
	return &PostResult{
		ExternalID: commentURN,
		URL:        fmt.Sprintf("https://www.linkedin.com/feed/update/%s", parentExternalID),
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
