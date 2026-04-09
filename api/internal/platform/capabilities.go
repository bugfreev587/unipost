// Capabilities describes what each platform's adapter accepts on the
// PUBLISH side: caption length, media counts/sizes, supported features
// like threads / scheduling / first-comment, and so on. It is the
// authoritative reference an LLM (or any other client) consults BEFORE
// generating content, so the same rules can be enforced server-side
// during validation.
//
// This is intentionally distinct from analytics capabilities (which
// describes which metrics each platform exposes after publish — see
// dashboard/src/lib/platform-capabilities.ts).
//
// Source of truth ordering when limits drift over time:
//
//  1. The corresponding adapter's enforcement code in
//     internal/platform/<platform>.go (if it has one).
//  2. The platform's official developer docs.
//  3. This file.
//
// When (1) or (2) changes, update this file and bump CapabilitiesSchemaVersion.

package platform

// CapabilitiesSchemaVersion identifies the shape of the Capability /
// MediaCapability / etc. structs. Bump when adding or renaming fields so
// clients can detect schema drift. Bump the minor version for backwards-
// compatible additions; bump the major version for breaking changes.
//
// 1.0 → 1.1 (Sprint 2): added TextCapability.SupportsThreads. Old
// 1.0 consumers ignore the unknown field, so this is purely additive.
// 1.1 → 1.2 (Sprint 3): flipped bluesky.text.supports_threads to true
// after the orchestrator gained AT-proto root+parent reply plumbing.
// Field set unchanged — same purely-additive contract.
// 1.2 → 1.3 (Sprint 4 PR1): removed the managed-Twitter media guard
// after media.write was added to the OAuth scope. Behavior loosening,
// not a schema change — bumped to give clients a way to detect when
// managed-Twitter media became supported.
// 1.3 → 1.4 (Sprint 4 PR3): added FirstCommentCapability.MaxLength
// and flipped twitter.first_comment.supported to true. Old consumers
// keep working — the new MaxLength field is omitempty.
const CapabilitiesSchemaVersion = "1.4"

// Capability is the full set of post-creation rules for one platform.
// Clients hit GET /v1/platforms/capabilities to fetch the whole map.
type Capability struct {
	DisplayName  string                 `json:"display_name"`
	Text         TextCapability         `json:"text"`
	Media        MediaCapability        `json:"media"`
	Thread       ThreadCapability       `json:"thread"`
	Scheduling   SchedulingCapability   `json:"scheduling"`
	FirstComment FirstCommentCapability `json:"first_comment"`
}

// TextCapability bounds the post body / caption.
type TextCapability struct {
	// MaxLength is the inclusive upper bound on the caption character
	// count. Counted in Unicode code points (not bytes).
	MaxLength int `json:"max_length"`
	// MinLength is the inclusive lower bound. 0 means "empty captions
	// are allowed" — true for image-only Instagram posts, etc.
	MinLength int `json:"min_length"`
	// Required reports whether a caption is mandatory. False for IG /
	// TikTok / Bluesky which accept media-only posts.
	Required bool `json:"required"`
	// SupportsThreads (Sprint 2 schema 1.1) reports whether the
	// platform's adapter can chain multiple posts to the same account
	// as a single thread via thread_position. Twitter is the only true
	// in Sprint 2; Bluesky / Threads land in Sprint 3.
	SupportsThreads bool `json:"supports_threads"`
}

// MediaCapability describes what media (images, videos, GIFs) the
// platform accepts. RequiresMedia means the platform doesn't support
// text-only posts (Instagram, TikTok video, YouTube).
type MediaCapability struct {
	RequiresMedia bool             `json:"requires_media"`
	Images        ImageCapability  `json:"images"`
	Videos        VideoCapability  `json:"videos"`
	AllowMixed    bool             `json:"allow_mixed"` // image + video in one post
}

// ImageCapability lists per-image and per-post image limits. Most
// fields are nullable (zero = unspecified) so we don't lie about
// platform-specific quirks we don't know.
type ImageCapability struct {
	// MaxCount is the most images allowed in a single post (carousel
	// upper bound). 0 means images aren't supported at all.
	MaxCount int `json:"max_count"`
	// MaxFileSizeBytes is the per-image upper bound. 0 = unspecified.
	MaxFileSizeBytes int64 `json:"max_file_size_bytes,omitempty"`
	// AllowedFormats are the lowercase MIME-style extensions accepted
	// (e.g. "jpg", "png", "webp"). Empty = unspecified.
	AllowedFormats []string `json:"allowed_formats,omitempty"`
}

// VideoCapability lists video-specific limits.
type VideoCapability struct {
	// MaxCount is the most videos allowed in a single post. Most
	// platforms cap at 1; TikTok/IG carousel can mix multiple under a
	// single video container.
	MaxCount int `json:"max_count"`
	// MaxDurationSeconds is the longest video clip the platform will
	// accept via API. 0 = unspecified.
	MaxDurationSeconds int `json:"max_duration_seconds,omitempty"`
	// MaxFileSizeBytes is the upper bound on video file size. 0 = unspecified.
	MaxFileSizeBytes int64 `json:"max_file_size_bytes,omitempty"`
	// AllowedFormats lists accepted container/codec hints (e.g. "mp4", "mov").
	AllowedFormats []string `json:"allowed_formats,omitempty"`
}

// ThreadCapability indicates whether the platform supports threading
// (reply chains from the same author).
type ThreadCapability struct {
	Supported bool `json:"supported"`
	// MaxItems is 0 when unbounded by the platform.
	MaxItems int `json:"max_items,omitempty"`
}

// SchedulingCapability indicates whether UniPost can schedule posts on
// this platform (currently true for everything since we use our own
// scheduler, not the platform's native one).
type SchedulingCapability struct {
	Supported bool `json:"supported"`
}

// FirstCommentCapability flags platforms where the first comment can
// be posted alongside the main post (Sprint 4 PR3: Twitter, LinkedIn,
// Instagram). This unlocks the "hashtags as first comment" pattern
// and other community-conventions where the actual content lives in
// the first reply.
//
// MaxLength is the per-platform character cap on the first comment
// itself. 0 means "no separate limit" (the platform's main caption
// rules apply); a non-zero value enforces a tighter cap.
type FirstCommentCapability struct {
	Supported bool `json:"supported"`
	MaxLength int  `json:"max_length,omitempty"`
}

// Capabilities is the full per-platform map. Order matters only for
// readability — the JSON consumer is free to iterate keys.
//
// IMPORTANT: this is a SNAPSHOT of platform docs as of 2026-04. When
// any of the underlying limits change, update both this file AND the
// adapter that enforces it. The validate API and the LLM-facing
// capability endpoint both pull from here.
var Capabilities = map[string]Capability{
	"twitter": {
		DisplayName: "Twitter / X",
		Text: TextCapability{
			MaxLength:       280,
			MinLength:       0,
			Required:        false,
			SupportsThreads: true,
		},
		Media: MediaCapability{
			RequiresMedia: false,
			AllowMixed:    false, // Twitter rejects mixing image+video
			Images: ImageCapability{
				MaxCount:         4,
				MaxFileSizeBytes: 5 * 1024 * 1024, // 5 MB per image
				AllowedFormats:   []string{"jpg", "png", "webp", "gif"},
			},
			Videos: VideoCapability{
				MaxCount:           1,
				MaxDurationSeconds: 140,                  // 2:20 for non-verified
				MaxFileSizeBytes:   512 * 1024 * 1024,    // 512 MB
				AllowedFormats:     []string{"mp4", "mov"},
			},
		},
		Thread:       ThreadCapability{Supported: true},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: true, MaxLength: 280},
	},
	"instagram": {
		DisplayName: "Instagram",
		Text: TextCapability{
			MaxLength: 2200,
			MinLength: 0,
			Required:  false,
		},
		Media: MediaCapability{
			RequiresMedia: true, // text-only IG posts not supported
			AllowMixed:    true, // carousels can mix image + video
			Images: ImageCapability{
				MaxCount:         10, // carousel upper bound
				MaxFileSizeBytes: 8 * 1024 * 1024,
				AllowedFormats:   []string{"jpg", "png"},
			},
			Videos: VideoCapability{
				MaxCount:           1, // single Reels per post
				MaxDurationSeconds: 90,
				MaxFileSizeBytes:   1024 * 1024 * 1024, // 1 GB
				AllowedFormats:     []string{"mp4", "mov"},
			},
		},
		Thread:       ThreadCapability{Supported: false},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: true, MaxLength: 2200},
	},
	"tiktok": {
		DisplayName: "TikTok",
		Text: TextCapability{
			MaxLength: 2200,
			MinLength: 0,
			Required:  false,
		},
		Media: MediaCapability{
			RequiresMedia: true, // text-only TikTok not supported
			AllowMixed:    false,
			Images: ImageCapability{
				MaxCount:         35, // photo carousel upper bound
				MaxFileSizeBytes: 20 * 1024 * 1024,
				AllowedFormats:   []string{"jpg", "png", "webp"},
			},
			Videos: VideoCapability{
				MaxCount:           1,
				MaxDurationSeconds: 600, // 10 min for some accounts
				MaxFileSizeBytes:   4 * 1024 * 1024 * 1024,
				AllowedFormats:     []string{"mp4", "mov", "webm"},
			},
		},
		Thread:       ThreadCapability{Supported: false},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: false},
	},
	"youtube": {
		DisplayName: "YouTube",
		Text: TextCapability{
			MaxLength: 5000, // description; title is shorter
			MinLength: 0,
			Required:  false,
		},
		Media: MediaCapability{
			RequiresMedia: true, // YouTube needs a video
			AllowMixed:    false,
			Images: ImageCapability{
				MaxCount: 0, // no still-image posts via API
			},
			Videos: VideoCapability{
				MaxCount:           1,
				MaxDurationSeconds: 12 * 60 * 60, // 12 hours upper bound
				MaxFileSizeBytes:   256 * 1024 * 1024 * 1024,
				AllowedFormats:     []string{"mp4", "mov", "webm"},
			},
		},
		Thread:       ThreadCapability{Supported: false},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: false},
	},
	"threads": {
		DisplayName: "Threads",
		Text: TextCapability{
			MaxLength: 500,
			MinLength: 0,
			Required:  false,
		},
		Media: MediaCapability{
			RequiresMedia: false,
			AllowMixed:    true, // carousel allows image+video mix
			Images: ImageCapability{
				MaxCount:         20, // carousel upper bound
				MaxFileSizeBytes: 8 * 1024 * 1024,
				AllowedFormats:   []string{"jpg", "png"},
			},
			Videos: VideoCapability{
				MaxCount:           1,
				MaxDurationSeconds: 5 * 60, // 5 min
				MaxFileSizeBytes:   1024 * 1024 * 1024,
				AllowedFormats:     []string{"mp4", "mov"},
			},
		},
		Thread:       ThreadCapability{Supported: true},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: false},
	},
	"linkedin": {
		DisplayName: "LinkedIn",
		Text: TextCapability{
			MaxLength: 3000,
			MinLength: 0,
			Required:  false,
		},
		Media: MediaCapability{
			RequiresMedia: false,
			AllowMixed:    false, // LinkedIn rejects mixing
			Images: ImageCapability{
				MaxCount:         9, // per Assets API + UGC share rules
				MaxFileSizeBytes: 100 * 1024 * 1024,
				AllowedFormats:   []string{"jpg", "png"},
			},
			Videos: VideoCapability{
				MaxCount:           1,
				MaxDurationSeconds: 10 * 60,
				MaxFileSizeBytes:   5 * 1024 * 1024 * 1024,
				AllowedFormats:     []string{"mp4", "mov"},
			},
		},
		Thread:       ThreadCapability{Supported: false},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: true, MaxLength: 1250},
	},
	"bluesky": {
		DisplayName: "Bluesky",
		Text: TextCapability{
			MaxLength:       300,
			MinLength:       0,
			Required:        false,
			SupportsThreads: true, // Sprint 3 PR8: AT-proto root+parent reply plumbing
		},
		Media: MediaCapability{
			RequiresMedia: false,
			AllowMixed:    false, // image and video can't be mixed in one record
			Images: ImageCapability{
				MaxCount:         4,
				MaxFileSizeBytes: 1024 * 1024, // 1 MB per image (PDS-side cap)
				AllowedFormats:   []string{"jpg", "png", "webp"},
			},
			Videos: VideoCapability{
				MaxCount:           1,
				MaxDurationSeconds: 60,
				MaxFileSizeBytes:   100 * 1024 * 1024,
				AllowedFormats:     []string{"mp4", "mov"},
			},
		},
		Thread:       ThreadCapability{Supported: true},
		Scheduling:   SchedulingCapability{Supported: true},
		FirstComment: FirstCommentCapability{Supported: false},
	},
}

// CapabilityFor returns the capability for a platform, or false if the
// platform isn't recognized. Lookup is case-insensitive on the caller side
// (the map keys are already lowercase).
func CapabilityFor(platform string) (Capability, bool) {
	c, ok := Capabilities[platform]
	return c, ok
}
