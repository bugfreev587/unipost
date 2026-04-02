package platform

import (
	"context"
	"time"
)

// ConnectResult holds the result of connecting a social account.
type ConnectResult struct {
	AccessToken       string
	RefreshToken      string
	TokenExpiresAt    time.Time
	ExternalAccountID string         // Platform-specific account ID (e.g., Bluesky DID)
	AccountName       string         // Display name or handle
	AvatarURL         string
	Metadata          map[string]any // Platform-specific metadata
}

// PostResult holds the result of publishing a post.
type PostResult struct {
	ExternalID string // Platform-specific post ID
	URL        string // Public URL to view the post
}

// PlatformAdapter defines the interface all platform integrations must implement.
type PlatformAdapter interface {
	// Platform returns the platform identifier (e.g., "bluesky").
	Platform() string

	// Connect authenticates with the platform and returns account info + tokens.
	Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error)

	// Post publishes content to the platform.
	Post(ctx context.Context, accessToken string, text string, imageURLs []string) (*PostResult, error)

	// DeletePost removes a post from the platform.
	DeletePost(ctx context.Context, accessToken string, externalID string) error

	// RefreshToken exchanges a refresh token for new access/refresh tokens.
	RefreshToken(ctx context.Context, refreshToken string) (newAccess, newRefresh string, expiresAt time.Time, err error)
}
