// social_posts_validate.go houses the request parser shared between
// POST /v1/social-posts and POST /v1/social-posts/validate, plus the
// /validate handler itself. The parser handles both the legacy shape
// (caption + account_ids) and the new AgentPost shape (platform_posts[]),
// expanding the legacy form into the same internal []PlatformPostInput
// the validator expects. PR5 will switch the Create handler over to use
// the same parser so the two endpoints stay in lockstep.

package handler

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// publishRequestBody is the union of the legacy and new request shapes.
// Validation rules:
//   - Exactly one of platform_posts / account_ids must be present.
//   - When the legacy form is used, the top-level caption / media_urls /
//     platform_options apply to every account in account_ids.
//   - When the new form is used, top-level caption / media_urls are
//     ignored (they could be added later as fallbacks but explicit is
//     better than implicit for an LLM-facing API).
type publishRequestBody struct {
	// Legacy fields.
	Caption         string                    `json:"caption"`
	MediaURLs       []string                  `json:"media_urls"`
	AccountIDs      []string                  `json:"account_ids"`
	PlatformOptions map[string]map[string]any `json:"platform_options"`

	// New shape.
	PlatformPosts []platformPostBody `json:"platform_posts"`

	// Common.
	ScheduledAt    *string `json:"scheduled_at"`
	IdempotencyKey string  `json:"idempotency_key"`

	// Sprint 2: status="draft" persists the post without dispatching.
	// Anything else (or omitted) preserves the existing immediate /
	// scheduled flow.
	Status string `json:"status"`
}

// platformPostBody is one entry in the new platform_posts[] shape.
// Per §3.1 of the sprint 1 PRD, per-post scheduled_at is rejected —
// only the top-level scheduled_at is honored. The field exists here
// so we can fail loud if a caller sends it.
//
// Sprint 2 added MediaIDs (R2-hosted uploads via the media library)
// and ThreadPosition (Twitter-only multi-tweet threads).
type platformPostBody struct {
	AccountID       string         `json:"account_id"`
	Caption         string         `json:"caption"`
	MediaURLs       []string       `json:"media_urls"`
	MediaIDs        []string       `json:"media_ids"`
	PlatformOptions map[string]any `json:"platform_options"`
	InReplyTo       string         `json:"in_reply_to"`
	ThreadPosition  int            `json:"thread_position"`
	ScheduledAt     *string        `json:"scheduled_at"`  // forbidden
	FirstComment    string         `json:"first_comment"` // Sprint 4 PR3
}

// parsedRequest is what the parser hands back. The validator and the
// publish path both consume it. Caller is expected to NOT mutate.
type parsedRequest struct {
	Posts          []platform.PlatformPostInput
	ScheduledAt    *time.Time
	IdempotencyKey string
	// Status is set when the request body explicitly asked for
	// "draft". Empty otherwise — the create handler maps that to
	// either immediate publish or scheduled publish based on
	// ScheduledAt.
	Status string
}

// parsePublishRequest is the single entry point for converting the
// raw HTTP body into the internal []PlatformPostInput shape. It
// handles the legacy / new branch, validates the cross-field
// invariants the validator can't, and returns a ready-to-validate
// parsedRequest.
//
// On a structural error (mutually-exclusive fields, unparseable
// scheduled_at, etc.) it returns an http status + a message the
// handler can write straight back to the client.
func parsePublishRequest(body publishRequestBody) (parsedRequest, int, string) {
	hasLegacy := len(body.AccountIDs) > 0
	hasNew := len(body.PlatformPosts) > 0

	switch {
	case hasLegacy && hasNew:
		return parsedRequest{}, http.StatusUnprocessableEntity,
			"platform_posts and account_ids are mutually exclusive — pass one, not both"
	case !hasLegacy && !hasNew:
		return parsedRequest{}, http.StatusUnprocessableEntity,
			"either platform_posts or account_ids is required"
	}

	pr := parsedRequest{IdempotencyKey: body.IdempotencyKey}

	// Status (Sprint 2): only "draft" is meaningful at create time;
	// reject other explicit values to keep the surface narrow.
	switch body.Status {
	case "", "draft":
		pr.Status = body.Status
	default:
		return parsedRequest{}, http.StatusUnprocessableEntity,
			"status must be empty or \"draft\" when creating a post"
	}

	// scheduled_at parsing is shared.
	if body.ScheduledAt != nil && *body.ScheduledAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ScheduledAt)
		if err != nil {
			return parsedRequest{}, http.StatusUnprocessableEntity,
				"invalid scheduled_at: " + err.Error()
		}
		pr.ScheduledAt = &t
	}

	if hasNew {
		pr.Posts = make([]platform.PlatformPostInput, 0, len(body.PlatformPosts))
		for i, pp := range body.PlatformPosts {
			if pp.ScheduledAt != nil && *pp.ScheduledAt != "" {
				return parsedRequest{}, http.StatusUnprocessableEntity,
					fmt.Sprintf("platform_posts[%d].scheduled_at is not supported in v1; use the top-level scheduled_at", i)
			}
			pr.Posts = append(pr.Posts, platform.PlatformPostInput{
				AccountID:       pp.AccountID,
				Caption:         pp.Caption,
				MediaURLs:       pp.MediaURLs,
				MediaIDs:        pp.MediaIDs,
				PlatformOptions: pp.PlatformOptions,
				InReplyTo:       pp.InReplyTo,
				ThreadPosition:  pp.ThreadPosition,
				FirstComment:    pp.FirstComment,
			})
		}
		return pr, 0, ""
	}

	// Legacy expansion: one input per account_id, sharing the
	// top-level caption / media_urls / platform_options. Per-platform
	// option lookup happens later in the publish loop because we don't
	// know each account's platform here without a DB lookup — the
	// validator does the join via its Accounts map.
	pr.Posts = make([]platform.PlatformPostInput, 0, len(body.AccountIDs))
	for _, id := range body.AccountIDs {
		pr.Posts = append(pr.Posts, platform.PlatformPostInput{
			AccountID: id,
			Caption:   body.Caption,
			MediaURLs: body.MediaURLs,
			// platform_options is keyed by platform name, not account
			// ID, so we can't pre-resolve here. The publish loop and
			// the validator each handle this themselves.
		})
	}
	return pr, 0, ""
}

// loadValidateAccounts builds the ValidateAccount map the pure
// validator needs. We load every account for the workspace once (cheap
// — workspaces rarely have more than a few dozen accounts) so the
// validator can resolve any account_id without a per-id round trip.
func (h *SocialPostHandler) loadValidateAccounts(r *http.Request, workspaceID string) (map[string]platform.ValidateAccount, error) {
	accounts, err := h.queries.ListSocialAccountsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]platform.ValidateAccount, len(accounts))
	for _, a := range accounts {
		out[a.ID] = platform.ValidateAccount{
			Platform:       a.Platform,
			Disconnected:   a.DisconnectedAt.Valid,
			ConnectionType: a.ConnectionType,
		}
	}
	return out, nil
}

// loadValidateMedia loads each referenced media_id from the workspace's
// media table so the validator can check ownership + status. Only
// the IDs explicitly mentioned in posts are loaded — we don't list
// the whole workspace's media library, since most validate calls won't
// touch any media at all.
//
// Returns an empty map (NOT nil) when posts reference NO media_ids,
// so the validator's "Media != nil → check" gate runs and reports
// any unknown IDs as media_id_not_in_workspace. nil would skip the
// check entirely; that's only used by callers that don't want media
// validation at all.
func (h *SocialPostHandler) loadValidateMedia(r *http.Request, workspaceID string, posts []platform.PlatformPostInput) map[string]platform.ValidateMedia {
	// Collect every media_id referenced anywhere in the request.
	wanted := make(map[string]bool)
	for _, p := range posts {
		for _, mid := range p.MediaIDs {
			wanted[mid] = true
		}
	}
	out := make(map[string]platform.ValidateMedia, len(wanted))
	for mid := range wanted {
		row, err := h.queries.GetMediaByIDAndWorkspace(r.Context(), db.GetMediaByIDAndWorkspaceParams{
			ID:          mid,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			// Not in this workspace (or not found at all). The
			// validator reports it via media_id_not_in_workspace — we
			// just leave it absent from the map.
			continue
		}
		out[mid] = platform.ValidateMedia{
			Status:      row.Status,
			ContentType: row.ContentType,
			SizeBytes:   row.SizeBytes,
		}
	}
	return out
}

func (h *SocialPostHandler) runPublishValidation(r *http.Request, workspaceID string, posts []platform.PlatformPostInput, scheduledAt *time.Time, accounts map[string]platform.ValidateAccount) platform.ValidationResult {
	media := h.loadValidateMedia(r, workspaceID, posts)
	result := platform.ValidatePlatformPosts(platform.ValidateOptions{
		Capabilities: platform.Capabilities,
		Accounts:     accounts,
		Media:        media,
		Posts:        posts,
		ScheduledAt:  scheduledAt,
	})
	result.Errors = append(result.Errors, h.loadImageMetadataValidationIssues(r, workspaceID, posts, accounts)...)
	result.Valid = len(result.Errors) == 0
	return result
}

func (h *SocialPostHandler) loadImageMetadataValidationIssues(r *http.Request, workspaceID string, posts []platform.PlatformPostInput, accounts map[string]platform.ValidateAccount) []platform.Issue {
	if h.storage == nil {
		return nil
	}
	var issues []platform.Issue
	for i, post := range posts {
		acc, ok := accounts[post.AccountID]
		if !ok || acc.Platform != "tiktok" {
			continue
		}
		for _, mediaID := range post.MediaIDs {
			row, err := h.queries.GetMediaByIDAndWorkspace(r.Context(), db.GetMediaByIDAndWorkspaceParams{
				ID:          mediaID,
				WorkspaceID: workspaceID,
			})
			if err != nil || (row.Status != "uploaded" && row.Status != "attached") {
				continue
			}
			if row.ContentType != "image/jpeg" && row.ContentType != "image/jpg" && row.ContentType != "image/png" {
				continue
			}
			cfg, err := h.loadImageConfig(r, row.StorageKey)
			if err != nil {
				continue
			}
			if tiktokImageWithin1080p(cfg.Width, cfg.Height) {
				continue
			}
			issues = append(issues, platform.Issue{
				PlatformPostIndex: i,
				AccountID:         post.AccountID,
				Platform:          "tiktok",
				Field:             "media_ids",
				Code:              platform.CodeDimensionsOutOfRange,
				Message:           fmt.Sprintf("TikTok photos must be no larger than 1080p. This image is %dx%d. Resize it so the long edge is at most 1920 px and the short edge is at most 1080 px.", cfg.Width, cfg.Height),
				Actual:            map[string]int{"width": cfg.Width, "height": cfg.Height},
				Limit:             map[string]int{"long_edge": 1920, "short_edge": 1080},
				Severity:          platform.SeverityError,
			})
		}
	}
	return issues
}

func (h *SocialPostHandler) loadImageConfig(r *http.Request, storageKey string) (image.Config, error) {
	url, err := h.storage.PresignGet(r.Context(), storageKey, 5*time.Minute)
	if err != nil {
		return image.Config{}, err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return image.Config{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return image.Config{}, err
	}
	defer resp.Body.Close()
	cfg, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return image.Config{}, err
	}
	return cfg, nil
}

func tiktokImageWithin1080p(width, height int) bool {
	if width <= 0 || height <= 0 {
		return true
	}
	longEdge := width
	shortEdge := height
	if height > width {
		longEdge = height
		shortEdge = width
	}
	return longEdge <= 1920 && shortEdge <= 1080
}

// Validate handles POST /v1/social-posts/validate.
//
// Pure preflight — no DB writes, no platform API calls. Returns the
// same ValidationResult shape that the publish handler will use as a
// defense-in-depth preflight inside Create() (PR5).
//
// Performance budget: p95 < 50ms. The single DB hit is the
// workspace-wide social_accounts list, which is bounded and cached at
// the pgx layer.
func (h *SocialPostHandler) Validate(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body publishRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid request body: "+err.Error())
		return
	}

	parsed, status, msg := parsePublishRequest(body)
	if status != 0 {
		writeError(w, status, "VALIDATION_ERROR", msg)
		return
	}

	accounts, err := h.loadValidateAccounts(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load accounts")
		return
	}

	result := h.runPublishValidation(r, workspaceID, parsed.Posts, parsed.ScheduledAt, accounts)

	// Always 200 even on validation failures — the body carries the
	// outcome. Treating /validate as a 200-only endpoint is what
	// Stripe / GitHub / etc. do for similar preflight calls and it
	// keeps client error handling cleaner.
	writeSuccess(w, result)
}
