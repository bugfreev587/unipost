package platform

import (
	"context"
	"path"
	"strings"
	"time"
)

// MediaKind classifies a single piece of media that an adapter receives.
// Adapters use this to decide which API path to take (e.g. Instagram chooses
// between IMAGE and REELS containers based on Kind).
type MediaKind string

const (
	MediaKindImage   MediaKind = "image"
	MediaKindVideo   MediaKind = "video"
	MediaKindGIF     MediaKind = "gif"
	MediaKindUnknown MediaKind = ""
)

// MediaItem is one piece of media to publish. The handler builds these from
// the request payload (which still accepts a flat media_urls []string for
// backward compatibility) and passes them to the adapter, which is then free
// to interpret them per platform conventions.
type MediaItem struct {
	URL  string    `json:"url"`
	Kind MediaKind `json:"kind"`
	// Alt text / caption for accessibility (Bluesky alt, IG alt, etc.).
	// Optional — adapters may ignore.
	Alt string `json:"alt,omitempty"`
}

// SniffMediaKind guesses the kind from a URL's file extension. Used as a
// fallback when the caller didn't explicitly classify the media. Errs on the
// side of "image" because that's the most common case and the safest default
// for adapters that don't support video.
func SniffMediaKind(rawURL string) MediaKind {
	// Strip query string and fragment so foo.mp4?token=abc still detects.
	clean := rawURL
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	ext := strings.ToLower(path.Ext(clean))
	switch ext {
	case ".mp4", ".mov", ".m4v", ".webm", ".mkv", ".avi":
		return MediaKindVideo
	case ".gif":
		return MediaKindGIF
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".heif":
		return MediaKindImage
	}
	return MediaKindUnknown
}

// MediaFromURLs builds a MediaItem slice from a flat URL list, sniffing each
// kind from the extension. Convenience helper used by the request handler so
// existing API consumers (which send media_urls as []string) keep working
// without having to specify a kind.
func MediaFromURLs(urls []string) []MediaItem {
	if len(urls) == 0 {
		return nil
	}
	out := make([]MediaItem, 0, len(urls))
	for _, u := range urls {
		out = append(out, MediaItem{URL: u, Kind: SniffMediaKind(u)})
	}
	return out
}

// MediaFromContentType creates a MediaItem from a content-type string.
// Used by the validator to count media_ids (R2 uploads) alongside media_urls.
func MediaFromContentType(contentType string) MediaItem {
	ct := strings.ToLower(contentType)
	switch {
	case strings.HasPrefix(ct, "video/"):
		return MediaItem{Kind: MediaKindVideo}
	case ct == "image/gif":
		return MediaItem{Kind: MediaKindGIF}
	case strings.HasPrefix(ct, "image/"):
		return MediaItem{Kind: MediaKindImage}
	default:
		return MediaItem{Kind: MediaKindUnknown}
	}
}

// FilterByKind returns only the items whose Kind matches one of the provided
// kinds. Adapters use it to split a heterogeneous media list into image-only
// and video-only halves when the platform requires separate API paths.
func FilterByKind(items []MediaItem, kinds ...MediaKind) []MediaItem {
	if len(items) == 0 {
		return nil
	}
	allowed := make(map[MediaKind]bool, len(kinds))
	for _, k := range kinds {
		allowed[k] = true
	}
	out := make([]MediaItem, 0, len(items))
	for _, it := range items {
		if allowed[it.Kind] {
			out = append(out, it)
		}
	}
	return out
}

// URLs returns just the URL strings from a MediaItem slice. Convenience for
// adapters that don't yet care about per-item kind.
func URLs(items []MediaItem) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.URL
	}
	return out
}

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
//
// CID is Bluesky-specific: AT-proto threads require both the URI and
// the content-addressed CID of the parent + root posts in the
// record.reply field. Other platforms leave it empty.
type PostResult struct {
	ExternalID string // Platform-specific post ID
	URL        string // Public URL to view the post
	CID        string // Content hash; populated only by Bluesky for threading
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

// FirstCommentAdapter is the optional interface adapters implement
// when they support attaching a first_comment to a published post.
// The handler dispatcher checks for this interface after a successful
// main post and calls PostComment with the parent's externalID.
//
// Sprint 4 PR3 implementations: Twitter (reply-to-self), LinkedIn
// (own-post comment via UGC API), Instagram (first comment via media
// comments API). Bluesky/Threads/TikTok/YouTube don't implement it.
type FirstCommentAdapter interface {
	// PostComment attaches text as a comment / reply on the existing
	// post identified by parentExternalID. Returns the comment's own
	// external id (useful for delete / analytics) plus the public URL.
	// Failure of this call is logged as a warning on the parent
	// result; the parent post is NEVER rolled back.
	PostComment(ctx context.Context, accessToken string, parentExternalID string, text string) (*PostResult, error)
}

// InboxEntry represents a single comment, DM, or reply fetched from a platform.
type InboxEntry struct {
	ExternalID       string
	ParentExternalID string // media ID (comments), conversation ID (DMs), parent post ID (Threads)
	AuthorName       string
	AuthorID         string
	AuthorAvatarURL  string
	Body             string
	Timestamp        time.Time
	Source           string // "ig_comment", "ig_dm", "threads_reply"
}

// InboxAdapter is optionally implemented by platforms that support
// reading and replying to comments, messages, or replies.
type InboxAdapter interface {
	// FetchComments returns comments/replies on a given media/post.
	FetchComments(ctx context.Context, accessToken string, mediaExternalID string) ([]InboxEntry, error)
	// ReplyToComment posts a reply to a comment/message.
	ReplyToComment(ctx context.Context, accessToken string, commentExternalID string, text string) (*PostResult, error)
}

// DMAdapter is optionally implemented by platforms that support DMs.
type DMAdapter interface {
	// FetchConversations returns recent DM conversations.
	FetchConversations(ctx context.Context, accessToken string) ([]InboxEntry, error)
	// SendDM sends a direct message.
	SendDM(ctx context.Context, accessToken string, recipientID string, text string) (*PostResult, error)
}

// PlatformAdapter defines the interface all platform integrations must implement.
type PlatformAdapter interface {
	// Platform returns the platform identifier (e.g., "bluesky").
	Platform() string

	// Connect authenticates with the platform and returns account info + tokens.
	Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error)

	// Post publishes content to the platform. media is a structured slice of
	// MediaItem so each adapter can decide per-item how to handle image vs.
	// video vs. gif. opts carries per-platform options (e.g.
	// {"privacy_status": "public"} for YouTube). Both may be nil/empty.
	Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error)

	// DeletePost removes a post from the platform.
	DeletePost(ctx context.Context, accessToken string, externalID string) error

	// RefreshToken exchanges a refresh token for new access/refresh tokens.
	RefreshToken(ctx context.Context, refreshToken string) (newAccess, newRefresh string, expiresAt time.Time, err error)
}
