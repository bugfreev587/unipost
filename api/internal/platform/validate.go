// validate.go is a PURE validation function for the AgentPost-style
// publish request shape. It runs the same checks the live publish
// path will run, but without I/O — no DB queries, no HTTP, no
// adapter calls. The handler at POST /v1/social-posts/validate uses
// it directly, and POST /v1/social-posts uses it as a defense-in-depth
// preflight before its publish loop, so the two endpoints can never
// disagree.
//
// What this function CAN check:
//   - caption length / required-ness against the static capability map
//   - missing-required-media against requires_media
//   - per-platform image and video count caps
//   - mixed image+video posts on platforms that forbid them
//   - account ownership (caller passes a map of valid accounts)
//   - account disconnection (caller passes a flag per account)
//   - scheduled_at sanity (too soon, too far ahead)
//   - in_reply_to on a platform that doesn't support threads
//
// What this function CANNOT check (deferred to runtime adapter calls):
//   - actual file size, dimensions, duration, codec
//   - fresh OAuth token validity (vs. just disconnected_at)
//   - quota — that's a soft block in the live path anyway
//
// Add new checks here whenever a class of failure appears in the wild;
// the goal is for /validate to predict every error /social-posts can
// throw, so an LLM can self-correct before publishing.

package platform

import (
	"strings"
	"time"
)

// PlatformPostInput is the per-account half of a publish request.
// One PlatformPostInput becomes one platform post — they're not grouped
// by account_id, so the same account_id can appear twice (e.g. a
// 2-tweet thread on Twitter).
type PlatformPostInput struct {
	AccountID       string
	Caption         string
	MediaURLs       []string
	PlatformOptions map[string]any
	InReplyTo       string // optional, for thread support (Sprint 2)
}

// ValidateAccount is what the validator needs to know about each
// account_id referenced by the input. The handler builds this map by
// joining the request's account IDs against the social_accounts table.
type ValidateAccount struct {
	Platform     string
	Disconnected bool
}

// ValidateOptions packages everything ValidatePlatformPosts needs.
// Time is injectable for deterministic tests.
type ValidateOptions struct {
	// Capabilities is the platform-level rule map. Tests can pass a
	// stub map; production callers pass platform.Capabilities.
	Capabilities map[string]Capability

	// Accounts maps each account_id the caller is allowed to use to its
	// platform name + disconnect status. account_ids in Posts that are
	// NOT in this map are reported as account_not_in_project errors.
	Accounts map[string]ValidateAccount

	// Posts is the slice to validate.
	Posts []PlatformPostInput

	// ScheduledAt is the optional top-level scheduled_at on the
	// request. nil = publish immediately.
	ScheduledAt *time.Time

	// Now is injectable for tests. Defaults to time.Now() when zero.
	Now time.Time

	// MaxScheduleAhead is the upper bound on how far in the future a
	// post can be scheduled. Defaults to 90 days when zero.
	MaxScheduleAhead time.Duration
}

// Issue is one validation finding. PlatformPostIndex is the 0-based
// position in the input slice; AccountID and Platform are populated
// when known so clients can pinpoint exactly which entry failed.
type Issue struct {
	PlatformPostIndex int    `json:"platform_post_index"`
	AccountID         string `json:"account_id,omitempty"`
	Platform          string `json:"platform,omitempty"`
	Field             string `json:"field"`
	Code              string `json:"code"`
	Message           string `json:"message"`
	Actual            any    `json:"actual,omitempty"`
	Limit             any    `json:"limit,omitempty"`
	Severity          string `json:"severity"`
}

// ValidationResult is what /validate (and the publish preflight)
// returns. Valid is shorthand for `len(Errors) == 0`.
type ValidationResult struct {
	Valid    bool    `json:"valid"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
}

// Severities.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Validation error codes. The full list lives here so handler /
// dashboard / SDK can switch on them without hard-coding strings.
const (
	CodeExceedsMaxLength       = "exceeds_max_length"
	CodeBelowMinLength         = "below_min_length"
	CodeMissingRequired        = "missing_required"
	CodeUnsupportedFormat      = "unsupported_format"
	CodeFileTooLarge           = "file_too_large"
	CodeDimensionsOutOfRange   = "dimensions_out_of_range"
	CodeAspectRatioUnsupported = "aspect_ratio_unsupported"
	CodeDurationOutOfRange     = "duration_out_of_range"
	CodeAccountDisconnected    = "account_disconnected"
	CodeAccountTokenExpired    = "account_token_expired"
	CodeQuotaExceeded          = "quota_exceeded"
	CodeScheduledTooSoon       = "scheduled_too_soon"
	CodeScheduledTooFar        = "scheduled_too_far"
	CodeUnsupportedInReplyTo   = "unsupported_in_reply_to"
	CodeUnknownPlatform        = "unknown_platform"
	CodeAccountNotFound        = "account_not_found"
	CodeAccountNotInProject    = "account_not_in_project"
	CodeMaxImagesExceeded      = "max_images_exceeded"
	CodeMaxVideosExceeded      = "max_videos_exceeded"
	CodeMixedMediaUnsupported  = "mixed_media_unsupported"
	CodeEmptyPosts             = "empty_posts"
	CodeTooManyPosts           = "too_many_posts"
	CodeUnknown                = "unknown"
)

// MaxPlatformPosts is the upper bound on how many entries one
// /social-posts request can carry. Matches the §3.1 contract.
const MaxPlatformPosts = 20

// defaultMaxScheduleAhead is how far in the future scheduled_at can
// be when the caller doesn't override it. 90 days mirrors what the
// big SaaS schedulers do.
const defaultMaxScheduleAhead = 90 * 24 * time.Hour

// minScheduleAhead is the minimum lead time for a scheduled post.
// Anything closer than this is treated as "publish now" and will
// trip the scheduled_too_soon error to surface intent more clearly.
const minScheduleAhead = 30 * time.Second

// ValidatePlatformPosts runs every pure check we know about and
// returns the full list of issues. Issues are returned in input order
// (by platform_post_index), then within an entry by the order the
// checks run, so output is deterministic across calls with the same
// input — important for snapshot tests and stable LLM retry behavior.
//
// The function never returns an error itself; the only thing that can
// fail is the validation, which is reported via the Issue list.
func ValidatePlatformPosts(opts ValidateOptions) ValidationResult {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	maxAhead := opts.MaxScheduleAhead
	if maxAhead == 0 {
		maxAhead = defaultMaxScheduleAhead
	}

	res := ValidationResult{Errors: []Issue{}, Warnings: []Issue{}}

	// Top-level checks first — bail out early on shape errors so we
	// don't bury the real issue under a wave of per-post failures.
	if len(opts.Posts) == 0 {
		res.Errors = append(res.Errors, Issue{
			Field:    "platform_posts",
			Code:     CodeEmptyPosts,
			Message:  "platform_posts must contain at least one entry",
			Severity: SeverityError,
		})
		res.Valid = false
		return res
	}
	if len(opts.Posts) > MaxPlatformPosts {
		res.Errors = append(res.Errors, Issue{
			Field:    "platform_posts",
			Code:     CodeTooManyPosts,
			Message:  "platform_posts has too many entries",
			Actual:   len(opts.Posts),
			Limit:    MaxPlatformPosts,
			Severity: SeverityError,
		})
		// Continue — the per-post checks below are still useful.
	}

	// Schedule sanity (top-level scheduled_at applies to every post).
	if opts.ScheduledAt != nil {
		delta := opts.ScheduledAt.Sub(now)
		switch {
		case delta < minScheduleAhead:
			res.Errors = append(res.Errors, Issue{
				Field:    "scheduled_at",
				Code:     CodeScheduledTooSoon,
				Message:  "scheduled_at must be at least 30 seconds in the future",
				Actual:   opts.ScheduledAt.Format(time.RFC3339),
				Severity: SeverityError,
			})
		case delta > maxAhead:
			res.Errors = append(res.Errors, Issue{
				Field:    "scheduled_at",
				Code:     CodeScheduledTooFar,
				Message:  "scheduled_at is further in the future than allowed",
				Actual:   opts.ScheduledAt.Format(time.RFC3339),
				Limit:    int(maxAhead.Hours() / 24),
				Severity: SeverityError,
			})
		}
	}

	// Per-post checks. Each post is validated independently — one
	// post's errors never short-circuit later posts, so the LLM gets
	// the full picture and can fix everything in a single retry.
	for i, post := range opts.Posts {
		validateOnePost(i, post, opts, &res)
	}

	res.Valid = len(res.Errors) == 0
	return res
}

// validateOnePost runs every check that applies to a single
// PlatformPostInput. Mutates res.Errors / res.Warnings in place.
func validateOnePost(i int, post PlatformPostInput, opts ValidateOptions, res *ValidationResult) {
	// Step 1: resolve the account → platform.
	if post.AccountID == "" {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			Field:             "account_id",
			Code:              CodeAccountNotFound,
			Message:           "account_id is required",
			Severity:          SeverityError,
		})
		return
	}

	acc, ok := opts.Accounts[post.AccountID]
	if !ok {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Field:             "account_id",
			Code:              CodeAccountNotInProject,
			Message:           "account does not belong to this project",
			Severity:          SeverityError,
		})
		return
	}

	plat := strings.ToLower(acc.Platform)
	cap, capOK := opts.Capabilities[plat]
	if !capOK {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "account_id",
			Code:              CodeUnknownPlatform,
			Message:           "no capability data for platform " + plat,
			Severity:          SeverityError,
		})
		return
	}

	if acc.Disconnected {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "account_id",
			Code:              CodeAccountDisconnected,
			Message:           "account is disconnected — reconnect on the Accounts page",
			Severity:          SeverityError,
		})
		// Don't return — still useful to report caption/media issues
		// so the LLM can fix everything in one go.
	}

	// Step 2: caption length / required-ness.
	captionLen := len([]rune(post.Caption))
	if cap.Text.Required && captionLen == 0 {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "caption",
			Code:              CodeMissingRequired,
			Message:           plat + " requires a non-empty caption",
			Severity:          SeverityError,
		})
	}
	if cap.Text.MinLength > 0 && captionLen < cap.Text.MinLength {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "caption",
			Code:              CodeBelowMinLength,
			Message:           "caption is shorter than the platform minimum",
			Actual:            captionLen,
			Limit:             cap.Text.MinLength,
			Severity:          SeverityError,
		})
	}
	if cap.Text.MaxLength > 0 && captionLen > cap.Text.MaxLength {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "caption",
			Code:              CodeExceedsMaxLength,
			Message:           "caption exceeds the platform maximum",
			Actual:            captionLen,
			Limit:             cap.Text.MaxLength,
			Severity:          SeverityError,
		})
	}

	// Step 3: media count + mixing rules.
	mediaItems := MediaFromURLs(post.MediaURLs)
	imageCount := len(FilterByKind(mediaItems, MediaKindImage, MediaKindGIF, MediaKindUnknown))
	videoCount := len(FilterByKind(mediaItems, MediaKindVideo))

	if cap.Media.RequiresMedia && len(mediaItems) == 0 {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "media_urls",
			Code:              CodeMissingRequired,
			Message:           plat + " requires at least one image or video",
			Severity:          SeverityError,
		})
	}
	if imageCount > 0 && cap.Media.Images.MaxCount == 0 {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "media_urls",
			Code:              CodeMaxImagesExceeded,
			Message:           plat + " does not support image posts",
			Actual:            imageCount,
			Limit:             0,
			Severity:          SeverityError,
		})
	} else if imageCount > cap.Media.Images.MaxCount {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "media_urls",
			Code:              CodeMaxImagesExceeded,
			Message:           "too many images for this platform",
			Actual:            imageCount,
			Limit:             cap.Media.Images.MaxCount,
			Severity:          SeverityError,
		})
	}
	if videoCount > cap.Media.Videos.MaxCount {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "media_urls",
			Code:              CodeMaxVideosExceeded,
			Message:           "too many videos for this platform",
			Actual:            videoCount,
			Limit:             cap.Media.Videos.MaxCount,
			Severity:          SeverityError,
		})
	}
	if imageCount > 0 && videoCount > 0 && !cap.Media.AllowMixed {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "media_urls",
			Code:              CodeMixedMediaUnsupported,
			Message:           plat + " does not allow mixing images and video in one post",
			Severity:          SeverityError,
		})
	}

	// Step 4: in_reply_to threading sanity.
	if post.InReplyTo != "" && !cap.Thread.Supported {
		res.Errors = append(res.Errors, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "in_reply_to",
			Code:              CodeUnsupportedInReplyTo,
			Message:           plat + " does not support thread replies via UniPost",
			Severity:          SeverityError,
		})
	}

	// Step 5: warnings — soft suggestions, never block publish.
	// Hardcode one to prove the channel works (per PRD §5.4).
	if plat == "linkedin" && len(mediaItems) == 0 {
		res.Warnings = append(res.Warnings, Issue{
			PlatformPostIndex: i,
			AccountID:         post.AccountID,
			Platform:          plat,
			Field:             "media_urls",
			Code:              "low_engagement_likely",
			Message:           "LinkedIn posts without media typically see lower reach — consider attaching an image or video",
			Severity:          SeverityWarning,
		})
	}
}
