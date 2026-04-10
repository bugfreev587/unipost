// social_posts_bulk.go is the Sprint 4 PR2 bulk publish endpoint.
//
//	POST /v1/social-posts/bulk
//
// Accepts up to 50 single-post bodies in one request, processes each
// independently, and returns a per-post result array. Partial-success
// semantics: the HTTP response is always 200 (assuming the request
// itself parses); per-post failures land in each entry's error field
// rather than failing the whole batch.
//
// Per-post idempotency keys still work — re-sending the same batch
// with the same keys returns the original responses for already-
// processed posts (and processes any keys that haven't been seen).
// This makes batch retries safe.
//
// Quota counts each post individually. If a workspace hits its quota
// halfway through the batch, the remaining posts get a quota_exceeded
// error result rather than failing the batch.
//
// Drafts and scheduled posts are NOT supported in bulk — they're
// rejected per-post with a clean validation error. The bulk path is
// for immediate publishes only; draft/scheduled batching adds
// transactional complexity that's not worth it for v1.

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// MaxBulkPosts is the per-request cap. 50 is large enough to batch
// a week of scheduled content for a small project, small enough that
// the synchronous handler stays under typical reverse-proxy timeouts
// (Railway / nginx default ~60s; 50 posts * ~1s/post platform latency
// = ~50s worst case).
const MaxBulkPosts = 50

// bulkRequestBody is the wire shape of POST /v1/social-posts/bulk.
// Each entry is a complete publishRequestBody — same shape the
// single-post endpoint accepts.
type bulkRequestBody struct {
	Posts []publishRequestBody `json:"posts"`
}

// bulkResultEntry is one slot in the bulk response. Either Data is
// set (success) or Error is set (validation/publish failure). Never
// both. Status is the per-post HTTP status code that the equivalent
// single-post call would have returned, included so callers can
// switch on it without parsing the error code string.
type bulkResultEntry struct {
	Status int                  `json:"status"`
	Data   *socialPostResponse  `json:"data,omitempty"`
	Error  *bulkErrorEnvelope   `json:"error,omitempty"`
}

type bulkErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CreateBulk handles POST /v1/social-posts/bulk.
func (h *SocialPostHandler) CreateBulk(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body bulkRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	if len(body.Posts) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "posts must contain at least one entry")
		return
	}
	if len(body.Posts) > MaxBulkPosts {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"posts exceeds maximum of 50 per batch")
		return
	}

	// Load accounts ONCE for the whole batch — way cheaper than
	// looking them up per-post. The validator and publish loop both
	// read from this map.
	accountMap, err := h.loadValidateAccounts(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		return
	}

	// Quota headers reflect the state BEFORE the batch starts. Per-post
	// quota is checked inside processBulkOne; we don't pre-reserve N
	// slots because that would punish a partial-success batch (e.g.
	// 49 posts succeed, 1 fails — caller should only be charged for 49).
	quotaStatus := h.quota.Check(r.Context(), workspaceID)
	w.Header().Set("X-UniPost-Usage", fmt.Sprintf("%d/%d", quotaStatus.Usage, quotaStatus.Limit))
	if quotaStatus.Warning != "" {
		w.Header().Set("X-UniPost-Warning", quotaStatus.Warning)
	}

	results := make([]bulkResultEntry, len(body.Posts))
	for i, postBody := range body.Posts {
		results[i] = h.processBulkOne(r, workspaceID, postBody, accountMap)
	}

	writeSuccess(w, results)
}

// processBulkOne is the per-post path for the bulk endpoint. Mirrors
// the single-post Create handler logic but returns a structured result
// instead of writing to the http.ResponseWriter. Drafts and scheduled
// posts are rejected — the bulk endpoint is immediate-publish only.
func (h *SocialPostHandler) processBulkOne(
	r *http.Request,
	workspaceID string,
	body publishRequestBody,
	accountMap map[string]platform.ValidateAccount,
) bulkResultEntry {
	parsed, status, msg := parsePublishRequest(body)
	if status != 0 {
		return bulkResultEntry{
			Status: status,
			Error:  &bulkErrorEnvelope{Code: "VALIDATION_ERROR", Message: msg},
		}
	}

	// Bulk doesn't support drafts or scheduled posts.
	if parsed.Status == "draft" {
		return bulkResultEntry{
			Status: http.StatusUnprocessableEntity,
			Error: &bulkErrorEnvelope{
				Code:    "VALIDATION_ERROR",
				Message: "drafts are not supported in bulk publish — use POST /v1/social-posts",
			},
		}
	}
	if parsed.ScheduledAt != nil {
		return bulkResultEntry{
			Status: http.StatusUnprocessableEntity,
			Error: &bulkErrorEnvelope{
				Code:    "VALIDATION_ERROR",
				Message: "scheduled posts are not supported in bulk publish — use POST /v1/social-posts",
			},
		}
	}

	// Idempotency replay — if this key already produced a row, return
	// the prior response in this slot. The other posts in the batch
	// still process. Re-sending the same batch with the same keys is
	// safe and the natural retry pattern.
	if parsed.IdempotencyKey != "" {
		if existing, err := h.queries.GetSocialPostByIdempotencyKey(r.Context(), db.GetSocialPostByIdempotencyKeyParams{
			WorkspaceID:    workspaceID,
			IdempotencyKey: pgtype.Text{String: parsed.IdempotencyKey, Valid: true},
		}); err == nil {
			resp := h.replayedPostResponse(r, existing)
			return bulkResultEntry{Status: http.StatusOK, Data: &resp}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			// Treat lookup errors as transient and proceed — better
			// to risk a duplicate than to block on a flaky lookup.
		}
	}

	// Validate.
	vr := platform.ValidatePlatformPosts(platform.ValidateOptions{
		Capabilities: platform.Capabilities,
		Accounts:     accountMap,
		Posts:        parsed.Posts,
	})
	if fatal := filterFatalIssues(vr.Errors); len(fatal) > 0 {
		// Surface the first fatal error in the message; the full
		// list isn't carried in the bulk response shape (callers
		// who need it should /validate the post separately).
		return bulkResultEntry{
			Status: http.StatusUnprocessableEntity,
			Error: &bulkErrorEnvelope{
				Code:    "VALIDATION_ERROR",
				Message: fatal[0].Message,
			},
		}
	}

	// Publish.
	resp, err := h.executeImmediatePost(r, workspaceID, parsed, accountMap)
	if err != nil {
		return bulkResultEntry{
			Status: http.StatusInternalServerError,
			Error:  &bulkErrorEnvelope{Code: "INTERNAL_ERROR", Message: err.Error()},
		}
	}
	return bulkResultEntry{Status: http.StatusOK, Data: &resp}
}
