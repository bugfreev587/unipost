package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BlueskyAdapter implements PlatformAdapter for the AT Protocol (Bluesky).
type BlueskyAdapter struct {
	baseURL string
	client  *http.Client
}

// NewBlueskyAdapter creates a new Bluesky adapter.
func NewBlueskyAdapter() *BlueskyAdapter {
	return &BlueskyAdapter{
		baseURL: "https://bsky.social",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *BlueskyAdapter) Platform() string { return "bluesky" }

// Connect authenticates with Bluesky using handle + app password.
func (b *BlueskyAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	handle := credentials["handle"]
	appPassword := credentials["app_password"]
	if handle == "" || appPassword == "" {
		return nil, fmt.Errorf("handle and app_password are required")
	}

	body, _ := json.Marshal(map[string]string{
		"identifier": handle,
		"password":   appPassword,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/xrpc/com.atproto.server.createSession", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Bluesky: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bluesky auth failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var session struct {
		DID        string `json:"did"`
		Handle     string `json:"handle"`
		AccessJwt  string `json:"accessJwt"`
		RefreshJwt string `json:"refreshJwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	expiresAt := time.Now().Add(2 * time.Hour) // Bluesky accessJwt ~2 hours
	if exp, err := parseJWTExp(session.AccessJwt); err == nil {
		expiresAt = exp
	}

	return &ConnectResult{
		AccessToken:       session.AccessJwt,
		RefreshToken:      session.RefreshJwt,
		TokenExpiresAt:    expiresAt,
		ExternalAccountID: session.DID,
		AccountName:       session.Handle,
		Metadata: map[string]any{
			"did":    session.DID,
			"handle": session.Handle,
		},
	}, nil
}

// Post publishes a text post (with optional images) to Bluesky.
func (b *BlueskyAdapter) Post(ctx context.Context, accessToken string, text string, imageURLs []string) (*PostResult, error) {
	did, err := parseJWTSub(accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DID from token: %w", err)
	}

	// Build the post record
	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Upload images if provided
	if len(imageURLs) > 0 {
		var images []map[string]any
		for _, url := range imageURLs {
			blob, err := b.uploadImage(ctx, accessToken, url)
			if err != nil {
				return nil, fmt.Errorf("failed to upload image %s: %w", url, err)
			}
			images = append(images, map[string]any{
				"alt":   "",
				"image": blob,
			})
		}
		record["embed"] = map[string]any{
			"$type":  "app.bsky.embed.images",
			"images": images,
		}
	}

	body, _ := json.Marshal(map[string]any{
		"repo":       did,
		"collection": "app.bsky.feed.post",
		"record":     record,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/xrpc/com.atproto.repo.createRecord", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bluesky post failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode post result: %w", err)
	}

	// Build public URL: at://did:plc:xxx/app.bsky.feed.post/rkey → https://bsky.app/profile/did/post/rkey
	rkey := ""
	parts := strings.Split(result.URI, "/")
	if len(parts) > 0 {
		rkey = parts[len(parts)-1]
	}
	publicURL := fmt.Sprintf("https://bsky.app/profile/%s/post/%s", did, rkey)

	return &PostResult{
		ExternalID: result.URI,
		URL:        publicURL,
	}, nil
}

// DeletePost removes a post from Bluesky.
func (b *BlueskyAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	did, err := parseJWTSub(accessToken)
	if err != nil {
		return fmt.Errorf("failed to parse DID from token: %w", err)
	}

	// Parse rkey from at:// URI
	parts := strings.Split(externalID, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid external ID: %s", externalID)
	}
	rkey := parts[len(parts)-1]
	collection := parts[len(parts)-2]

	body, _ := json.Marshal(map[string]string{
		"repo":       did,
		"collection": collection,
		"rkey":       rkey,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/xrpc/com.atproto.repo.deleteRecord", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bluesky delete failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// RefreshToken refreshes the Bluesky access token.
func (b *BlueskyAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/xrpc/com.atproto.server.refreshSession", nil)
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+refreshToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", time.Time{}, fmt.Errorf("bluesky refresh failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var session struct {
		AccessJwt  string `json:"accessJwt"`
		RefreshJwt string `json:"refreshJwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to decode refresh result: %w", err)
	}

	expiresAt := time.Now().Add(2 * time.Hour)
	if exp, err := parseJWTExp(session.AccessJwt); err == nil {
		expiresAt = exp
	}

	return session.AccessJwt, session.RefreshJwt, expiresAt, nil
}

// uploadImage downloads an image from a URL and uploads it to Bluesky.
func (b *BlueskyAdapter) uploadImage(ctx context.Context, accessToken string, imageURL string) (map[string]any, error) {
	// Download the image
	imgReq, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}
	imgResp, err := b.client.Do(imgReq)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer imgResp.Body.Close()

	imgData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	contentType := imgResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Upload to Bluesky
	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewReader(imgData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("blob upload failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Blob map[string]any `json:"blob"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode blob result: %w", err)
	}

	return result.Blob, nil
}

// parseJWTSub extracts the "sub" claim from a JWT without verification.
func parseJWTSub(token string) (string, error) {
	claims, err := parseJWTClaims(token)
	if err != nil {
		return "", err
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("missing sub claim")
	}
	return sub, nil
}

// parseJWTExp extracts the "exp" claim from a JWT.
func parseJWTExp(token string) (time.Time, error) {
	claims, err := parseJWTClaims(token)
	if err != nil {
		return time.Time{}, err
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, fmt.Errorf("missing exp claim")
	}
	return time.Unix(int64(exp), 0), nil
}

// parseJWTClaims decodes the payload segment of a JWT (no signature verification).
func parseJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Add padding if needed
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}
	return claims, nil
}
