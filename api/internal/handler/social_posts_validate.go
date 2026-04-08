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
	"net/http"
	"time"

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
	ScheduledAt     *string        `json:"scheduled_at"` // forbidden
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
// validator needs. We load every account for the project once (cheap
// — projects rarely have more than a few dozen accounts) so the
// validator can resolve any account_id without a per-id round trip.
func (h *SocialPostHandler) loadValidateAccounts(r *http.Request, projectID string) (map[string]platform.ValidateAccount, error) {
	accounts, err := h.queries.ListAllSocialAccountsByProject(r.Context(), projectID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]platform.ValidateAccount, len(accounts))
	for _, a := range accounts {
		out[a.ID] = platform.ValidateAccount{
			Platform:     a.Platform,
			Disconnected: a.DisconnectedAt.Valid,
		}
	}
	return out, nil
}

// Validate handles POST /v1/social-posts/validate.
//
// Pure preflight — no DB writes, no platform API calls. Returns the
// same ValidationResult shape that the publish handler will use as a
// defense-in-depth preflight inside Create() (PR5).
//
// Performance budget: p95 < 50ms. The single DB hit is the
// project-wide social_accounts list, which is bounded and cached at
// the pgx layer.
func (h *SocialPostHandler) Validate(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
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

	accounts, err := h.loadValidateAccounts(r, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load accounts")
		return
	}

	result := platform.ValidatePlatformPosts(platform.ValidateOptions{
		Capabilities: platform.Capabilities,
		Accounts:     accounts,
		Posts:        parsed.Posts,
		ScheduledAt:  parsed.ScheduledAt,
	})

	// Always 200 even on validation failures — the body carries the
	// outcome. Treating /validate as a 200-only endpoint is what
	// Stripe / GitHub / etc. do for similar preflight calls and it
	// keeps client error handling cleaner.
	writeSuccess(w, result)
}
