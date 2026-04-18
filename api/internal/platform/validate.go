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
	"errors"
	"regexp"
	"strings"
	"time"
)

// PlatformPostInput is the per-account half of a publish request.
// One PlatformPostInput becomes one platform post — they're not grouped
// by account_id, so the same account_id can appear twice (e.g. a
// 2-tweet thread on Twitter).
type PlatformPostInput struct {
	AccountID string
	Caption   string
	MediaURLs []string
	// MediaIDs (Sprint 2) references rows in the media library —
	// uploaded via POST /v1/media before publish. The handler resolves
	// each ID to a presigned download URL at dispatch time and adds
	// it to the adapter's media list. mixed MediaIDs + MediaURLs is
	// allowed; both feed into the same MediaItem slice eventually.
	MediaIDs        []string
	PlatformOptions map[string]any
	InReplyTo       string // optional, for thread support (Sprint 2 / 3)

	// ThreadPosition (Sprint 2, Twitter only) declares this post's
	// 1-indexed position in a multi-post thread. All entries with the
	// same AccountID and any non-zero ThreadPosition form one thread.
	// 0 means "not part of a thread" — single post. Validated for
	// contiguity (positions 1..N with no gaps) before dispatch.
	ThreadPosition int

	// FirstComment (Sprint 4 PR3) is an optional reply / comment that
	// gets posted immediately after the main post lands. The handler
	// orchestrates: publish main post → capture external_id → call
	// adapter.PostComment(externalID, FirstComment). Failure of the
	// first comment is recorded as a warning on the main result; the
	// main post is NOT rolled back.
	//
	// Supported on Twitter (self-reply), LinkedIn (own post comment),
	// Instagram (first comment via media comments API). Bluesky and
	// Threads reject this field with first_comment_unsupported because
	// they have native thread support — use thread_position instead.
	FirstComment string
}

// ValidateAccount is what the validator needs to know about each
// account_id referenced by the input. The handler builds this map by
// joining the request's account IDs against the social_accounts table.
//
// ConnectionType (Sprint 3) is "byo" or "managed" — used by the
// managed-Twitter media guard. Managed Twitter accounts are text-only
// in Sprint 3 because the OAuth flow doesn't request media.write.
type ValidateAccount struct {
	Platform       string
	Disconnected   bool
	ConnectionType string
}

// ValidateMedia (Sprint 2) is what the validator needs to know about
// each media_id referenced via PlatformPostInput.MediaIDs. The handler
// builds this map by joining the referenced IDs against the media
// table BEFORE calling ValidatePlatformPosts. Missing entries surface
// as media_id_not_found / media_id_not_in_project errors.
type ValidateMedia struct {
	Status      string // "pending" | "uploaded" | "attached" | "deleted"
	ContentType string
	SizeBytes   int64
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

	// Media maps each media_id the caller is allowed to reference. Nil
	// is fine — the validator skips media-id checks when the map is
	// nil (used by /validate when the caller hasn't pre-loaded any).
	// Empty (not nil) means "the caller pre-loaded media but found
	// nothing", which makes any reference an error.
	Media map[string]ValidateMedia

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
	CodeAccountNotInWorkspace  = "account_not_in_workspace"
	CodeMaxImagesExceeded      = "max_images_exceeded"
	CodeMaxVideosExceeded      = "max_videos_exceeded"
	CodeMixedMediaUnsupported  = "mixed_media_unsupported"
	CodeEmptyPosts             = "empty_posts"
	CodeTooManyPosts           = "too_many_posts"
	CodeUnknown                = "unknown"

	// Sprint 2 thread codes.
	CodeThreadsUnsupported           = "threads_unsupported"
	CodeThreadPositionsNotContiguous = "thread_positions_not_contiguous"
	CodeThreadMixedWithSingle        = "thread_mixed_with_single"

	// Sprint 2 media library codes.
	CodeMediaIDNotFound       = "media_id_not_found"
	CodeMediaIDNotInWorkspace = "media_id_not_in_workspace"
	CodeMediaNotUploaded      = "media_not_uploaded"

	// Sprint 4 PR3: first_comment field codes.
	CodeFirstCommentUnsupported         = "first_comment_unsupported"
	CodeFirstCommentTooLong             = "first_comment_too_long"
	CodeYouTubeTitleRequired            = "youtube_title_required"
	CodeYouTubeMadeForKidsRequired      = "youtube_made_for_kids_required"
	CodeInvalidPrivacyStatus            = "invalid_privacy_status"
	CodeInvalidLicense                  = "invalid_license"
	CodeInvalidPublishAt                = "invalid_publish_at"
	CodeInvalidRecordingDate            = "invalid_recording_date"
	CodeInvalidDefaultLanguage          = "invalid_default_language"
	CodeYouTubePublishAtRequiresPrivate = "youtube_publish_at_requires_private"
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

var youtubeLanguagePattern = regexp.MustCompile(`^[A-Za-z]{2,3}([_-][A-Za-z0-9]{2,8})*$`)

func hasOpt(opts map[string]any, key string) bool {
	if opts == nil {
		return false
	}
	_, ok := opts[key]
	return ok
}

func parseYouTubeTimestamp(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, format := range formats {
		if _, err := time.Parse(format, value); err == nil {
			return nil
		}
	}
	return errors.New("invalid timestamp")
}

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

	// Cross-post checks: thread contiguity / single-vs-thread mixing.
	// Run AFTER the per-post pass so an LLM sees both kinds of error
	// in one shot rather than having to fix per-post issues first.
	validateThreads(opts, &res)

	res.Valid = len(res.Errors) == 0
	return res
}

// validateThreads enforces the Sprint 2 thread rules:
//
//   - thread_position is only allowed on platforms whose capability
//     reports text.supports_threads = true.
//   - All entries with the same account_id and any non-zero
//     thread_position form one logical thread; positions must be
//     contiguous starting at 1 (1, 2, 3 — not 1, 3 or 2, 3).
//   - On the same account, you cannot mix thread entries with single
//     non-thread entries in the same publish call. Either all of an
//     account's posts are part of a thread, or none are.
//
// A "thread of 1" (one post with thread_position=1, no siblings) is
// silently treated as a single post and not flagged. The chaining
// logic in the handler is a no-op when there's only one entry.
func validateThreads(opts ValidateOptions, res *ValidationResult) {
	// Group entries by account_id, separating threaded from single
	// posts in the same pass.
	type group struct {
		threaded []int // indices into opts.Posts
		singles  []int
		platform string
	}
	groups := make(map[string]*group)
	for i, p := range opts.Posts {
		if p.AccountID == "" {
			continue // already reported elsewhere
		}
		g, ok := groups[p.AccountID]
		if !ok {
			g = &group{}
			if acc, hit := opts.Accounts[p.AccountID]; hit {
				g.platform = strings.ToLower(acc.Platform)
			}
			groups[p.AccountID] = g
		}
		if p.ThreadPosition > 0 {
			g.threaded = append(g.threaded, i)
		} else {
			g.singles = append(g.singles, i)
		}
	}

	for accountID, g := range groups {
		if len(g.threaded) == 0 {
			continue
		}

		// Capability gate — does the platform support threading at
		// all? If not, every threaded entry gets a thread_unsupported
		// error.
		if cap, ok := opts.Capabilities[g.platform]; ok && !cap.Text.SupportsThreads {
			for _, idx := range g.threaded {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: idx,
					AccountID:         accountID,
					Platform:          g.platform,
					Field:             "thread_position",
					Code:              CodeThreadsUnsupported,
					Message:           g.platform + " does not support multi-post threads via UniPost",
					Severity:          SeverityError,
				})
			}
			continue
		}

		// Mixing rule — threaded + non-threaded on the same account
		// in one call is ambiguous and rejected.
		if len(g.singles) > 0 {
			for _, idx := range g.threaded {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: idx,
					AccountID:         accountID,
					Platform:          g.platform,
					Field:             "thread_position",
					Code:              CodeThreadMixedWithSingle,
					Message:           "cannot mix thread entries with non-thread entries on the same account in one call",
					Severity:          SeverityError,
				})
			}
			// Don't bother with the contiguity check below if we
			// already failed mixing.
			continue
		}

		// Contiguity: positions must be a permutation of 1..N.
		positions := make([]int, 0, len(g.threaded))
		for _, idx := range g.threaded {
			positions = append(positions, opts.Posts[idx].ThreadPosition)
		}
		if !contiguousFromOne(positions) {
			for _, idx := range g.threaded {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: idx,
					AccountID:         accountID,
					Platform:          g.platform,
					Field:             "thread_position",
					Code:              CodeThreadPositionsNotContiguous,
					Message:           "thread_position values must be contiguous starting at 1",
					Actual:            positions,
					Severity:          SeverityError,
				})
			}
		}
	}
}

// contiguousFromOne reports whether the input is a permutation of
// {1, 2, ..., len(s)}. Cheap O(n) check via a "seen" map sized to
// the longest expected thread (Twitter caps at 25).
func contiguousFromOne(s []int) bool {
	if len(s) == 0 {
		return true
	}
	seen := make(map[int]bool, len(s))
	for _, v := range s {
		if v < 1 || v > len(s) {
			return false
		}
		if seen[v] {
			return false // duplicate
		}
		seen[v] = true
	}
	return true
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
			Code:              CodeAccountNotInWorkspace,
			Message:           "account does not belong to this workspace",
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

	if plat == "youtube" {
		title := strings.TrimSpace(optString(post.PlatformOptions, "title"))
		if title == "" {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "platform_options.title",
				Code:              CodeYouTubeTitleRequired,
				Message:           "youtube requires a non-empty video title before publishing",
				Severity:          SeverityError,
			})
		}
		if titleLen := len([]rune(strings.TrimSpace(optString(post.PlatformOptions, "title")))); titleLen > 100 {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "platform_options.title",
				Code:              CodeExceedsMaxLength,
				Message:           "youtube video title exceeds the 100 character limit",
				Actual:            titleLen,
				Limit:             100,
				Severity:          SeverityError,
			})
		}
		if !hasOpt(post.PlatformOptions, "made_for_kids") {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "platform_options.made_for_kids",
				Code:              CodeYouTubeMadeForKidsRequired,
				Message:           "youtube requires an explicit made_for_kids selection before publishing",
				Severity:          SeverityError,
			})
		}
		privacyStatus := strings.TrimSpace(optString(post.PlatformOptions, "privacy_status"))
		if err := validateEnum("youtube", "privacy_status", privacyStatus, YouTubePrivacyValues); err != nil {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "platform_options.privacy_status",
				Code:              CodeInvalidPrivacyStatus,
				Message:           "youtube privacy_status must be private, public, or unlisted",
				Actual:            privacyStatus,
				Limit:             YouTubePrivacyValues,
				Severity:          SeverityError,
			})
		}
		license := strings.TrimSpace(optString(post.PlatformOptions, "license"))
		if err := validateEnum("youtube", "license", license, YouTubeLicenseValues); err != nil {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "platform_options.license",
				Code:              CodeInvalidLicense,
				Message:           "youtube license must be youtube or creativeCommon",
				Actual:            license,
				Limit:             YouTubeLicenseValues,
				Severity:          SeverityError,
			})
		}
		defaultLanguage := strings.TrimSpace(optString(post.PlatformOptions, "default_language"))
		if defaultLanguage != "" && !youtubeLanguagePattern.MatchString(defaultLanguage) {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "platform_options.default_language",
				Code:              CodeInvalidDefaultLanguage,
				Message:           "youtube default_language must look like en, en-US, or zh-CN",
				Actual:            defaultLanguage,
				Severity:          SeverityError,
			})
		}
		publishAt := strings.TrimSpace(optString(post.PlatformOptions, "publish_at"))
		if publishAt != "" {
			if err := parseYouTubeTimestamp(publishAt); err != nil {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: i,
					AccountID:         post.AccountID,
					Platform:          plat,
					Field:             "platform_options.publish_at",
					Code:              CodeInvalidPublishAt,
					Message:           "youtube publish_at must be an RFC3339 datetime",
					Actual:            publishAt,
					Severity:          SeverityError,
				})
			}
			if privacyStatus == "" {
				privacyStatus = "private"
			}
			if privacyStatus != "private" {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: i,
					AccountID:         post.AccountID,
					Platform:          plat,
					Field:             "platform_options.publish_at",
					Code:              CodeYouTubePublishAtRequiresPrivate,
					Message:           "youtube publish_at requires privacy_status to be private",
					Actual:            privacyStatus,
					Limit:             "private",
					Severity:          SeverityError,
				})
			}
		}
		recordingDate := strings.TrimSpace(optString(post.PlatformOptions, "recording_date"))
		if recordingDate != "" {
			if err := parseYouTubeTimestamp(recordingDate); err != nil {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: i,
					AccountID:         post.AccountID,
					Platform:          plat,
					Field:             "platform_options.recording_date",
					Code:              CodeInvalidRecordingDate,
					Message:           "youtube recording_date must be YYYY-MM-DD or RFC3339 datetime",
					Actual:            recordingDate,
					Severity:          SeverityError,
				})
			}
		}
	}

	// Sprint 4 PR3: first_comment field validation. Reject on platforms
	// that don't support it (Bluesky/Threads have native threads instead;
	// they reject with first_comment_unsupported per PRD §W4 D10).
	// Enforce per-platform max length when set.
	if post.FirstComment != "" {
		if !cap.FirstComment.Supported {
			res.Errors = append(res.Errors, Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          plat,
				Field:             "first_comment",
				Code:              CodeFirstCommentUnsupported,
				Message:           plat + " does not support first_comment — use thread_position for native thread support",
				Severity:          SeverityError,
			})
		} else if cap.FirstComment.MaxLength > 0 {
			fcLen := len([]rune(post.FirstComment))
			if fcLen > cap.FirstComment.MaxLength {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: i,
					AccountID:         post.AccountID,
					Platform:          plat,
					Field:             "first_comment",
					Code:              CodeFirstCommentTooLong,
					Message:           "first_comment exceeds the platform maximum",
					Actual:            fcLen,
					Limit:             cap.FirstComment.MaxLength,
					Severity:          SeverityError,
				})
			}
		}
	}

	// Step 3: media count + mixing rules.
	// Count media from both MediaURLs and MediaIDs.
	mediaItems := MediaFromURLs(post.MediaURLs)
	// Also count media from media_ids (R2 uploads)
	if opts.Media != nil {
		for _, mid := range post.MediaIDs {
			if m, ok := opts.Media[mid]; ok && (m.Status == "uploaded" || m.Status == "attached") {
				mediaItems = append(mediaItems, MediaFromContentType(m.ContentType))
			}
		}
	}
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

	// Step 3.5 (Sprint 2): media_ids ownership + state.
	// We only run this check when the caller pre-loaded a media map.
	// /validate without DB access passes Media=nil and skips this.
	if opts.Media != nil {
		for _, mid := range post.MediaIDs {
			m, ok := opts.Media[mid]
			if !ok {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: i,
					AccountID:         post.AccountID,
					Platform:          plat,
					Field:             "media_ids",
					Code:              CodeMediaIDNotInWorkspace,
					Message:           "media_id " + mid + " not found or not in this workspace",
					Severity:          SeverityError,
				})
				continue
			}
			if m.Status != "uploaded" && m.Status != "attached" {
				res.Errors = append(res.Errors, Issue{
					PlatformPostIndex: i,
					AccountID:         post.AccountID,
					Platform:          plat,
					Field:             "media_ids",
					Code:              CodeMediaNotUploaded,
					Message:           "media_id " + mid + " is in status " + m.Status + "; PUT the bytes to the presigned URL first",
					Severity:          SeverityError,
				})
			}
		}
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
