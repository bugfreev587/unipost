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

// PostMetrics holds unified analytics metrics across all platforms.
//
// Adapters return raw counts; the handler computes EngagementRate per the
// PRD §9.1 formula: (likes + comments + shares + saves + clicks) / impressions.
// Adapters MUST leave EngagementRate at 0 — it will be overwritten downstream.
//
// Views is a legacy alias for VideoViews and is being phased out; new code
// should populate VideoViews. Both are emitted in JSON during the transition
// so existing dashboard code that reads `views` keeps working.
type PostMetrics struct {
	Impressions      int64          `json:"impressions"`
	Reach            int64          `json:"reach"`
	Likes            int64          `json:"likes"`
	Comments         int64          `json:"comments"`
	Shares           int64          `json:"shares"`
	Saves            int64          `json:"saves"`
	Clicks           int64          `json:"clicks"`
	VideoViews       int64          `json:"video_views"`
	Views            int64          `json:"views"` // legacy alias for VideoViews; remove once dashboard migrated
	EngagementRate   float64        `json:"engagement_rate"`
	PlatformSpecific map[string]any `json:"platform_specific,omitempty"`
}

// AnalyticsAdapter is optionally implemented by platforms that support analytics.
type AnalyticsAdapter interface {
	GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error)
}

// PlatformAdapter defines the interface all platform integrations must implement.
type PlatformAdapter interface {
	// Platform returns the platform identifier (e.g., "bluesky").
	Platform() string

	// Connect authenticates with the platform and returns account info + tokens.
	Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error)

	// Post publishes content to the platform. opts carries per-platform options
	// (e.g. {"privacy_status": "public"} for YouTube). May be nil.
	Post(ctx context.Context, accessToken string, text string, imageURLs []string, opts map[string]any) (*PostResult, error)

	// DeletePost removes a post from the platform.
	DeletePost(ctx context.Context, accessToken string, externalID string) error

	// RefreshToken exchanges a refresh token for new access/refresh tokens.
	RefreshToken(ctx context.Context, refreshToken string) (newAccess, newRefresh string, expiresAt time.Time, err error)
}
