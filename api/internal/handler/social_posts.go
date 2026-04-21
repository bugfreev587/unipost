package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/debugrt"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

type SocialPostHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	quota     *quota.Checker
	// bus is the publish-side EventBus used to fan out
	// post.published / post.partial / post.failed events to webhook
	// subscribers. Always non-nil — main.go injects either the real
	// worker or a NoopBus, never nil — so handler code can call
	// h.bus.Publish unconditionally.
	bus events.EventBus
	// storage (Sprint 2) is the R2-backed media library client used
	// to resolve PlatformPostInput.MediaIDs into presigned download
	// URLs at adapter dispatch time. Optional — nil disables the
	// media_ids feature; the handler returns a clear error if a
	// caller tries to use it on a server without R2 configured.
	storage *storage.Client
}

func NewSocialPostHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, quotaChecker *quota.Checker, bus events.EventBus, store *storage.Client) *SocialPostHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &SocialPostHandler{queries: queries, encryptor: encryptor, quota: quotaChecker, bus: bus, storage: store}
}

type postResultResponse struct {
	// ID is the social_post_results row ID. Required by the dashboard
	// for the per-platform retry path — without it the frontend can't
	// tell the API which failed row to re-run.
	ID              string  `json:"id"`
	SocialAccountID string  `json:"social_account_id"`
	Platform        string  `json:"platform,omitempty"`
	AccountName     string  `json:"account_name,omitempty"`
	Caption         string  `json:"caption,omitempty"`
	Status          string  `json:"status"`
	ExternalID      *string `json:"external_id,omitempty"`
	// URL is the platform's canonical post URL, returned by the
	// adapter at publish time (e.g. Threads permalink from the Graph
	// API). When present, the dashboard uses this directly instead of
	// constructing one from external_id — important for platforms like
	// Threads where the public URL uses shortcodes, not numeric IDs.
	URL           *string        `json:"url,omitempty"`
	ErrorMessage  *string        `json:"error_message,omitempty"`
	PublishedAt   *string        `json:"published_at,omitempty"`
	PublishStatus map[string]any `json:"publish_status,omitempty"`
	// Warnings (Sprint 4 PR3) carries non-fatal issues that didn't
	// prevent the main post from being published. The first user is
	// first_comment failure: the parent post lands and reports
	// status='published', but the customer is told the comment didn't.
	Warnings []string `json:"warnings,omitempty"`
	// DebugCurl is the serialized curl dump of every failing HTTP
	// request the adapter made during this dispatch — populated only
	// on status='failed'. Auth headers and token query params are
	// redacted; see internal/debugrt for the redaction rules.
	DebugCurl *string `json:"debug_curl,omitempty"`
	// Submitted captures what the user actually sent for this account
	// at publish time — the per-account caption override, media, and
	// platform-specific options (TikTok privacy/toggles/disclosure,
	// YouTube category/visibility, Instagram media_type, etc.). The
	// dashboard surfaces this as a "Submitted settings" panel so users
	// can review their own choices after the fact. Nil when we can't
	// reconstruct it (legacy posts without metadata).
	Submitted *submittedSettings `json:"submitted,omitempty"`
}

// submittedSettings is the per-account snapshot of what was sent to
// the adapter. Fields mirror platform.PlatformPostInput minus the
// identifiers (AccountID lives on the parent result already).
type submittedSettings struct {
	Caption         string         `json:"caption,omitempty"`
	MediaURLs       []string       `json:"media_urls,omitempty"`
	MediaIDs        []string       `json:"media_ids,omitempty"`
	PlatformOptions map[string]any `json:"platform_options,omitempty"`
	FirstComment    string         `json:"first_comment,omitempty"`
	InReplyTo       string         `json:"in_reply_to,omitempty"`
	ThreadPosition  int            `json:"thread_position,omitempty"`
}

// buildSubmittedMap decodes the stored metadata into an accountID →
// submittedSettings lookup so per-result rows can attach their own
// snapshot. Returns nil + nil for legacy posts with empty metadata; the
// caller treats that as "no data available" rather than an error so the
// rest of the response still renders.
func buildSubmittedMap(metadata []byte, fallbackCaption string) map[string]*submittedSettings {
	parsed, err := platform.DecodePostMetadata(metadata, fallbackCaption)
	if err != nil || len(parsed) == 0 {
		return nil
	}
	out := make(map[string]*submittedSettings, len(parsed))
	for _, pp := range parsed {
		out[pp.AccountID] = &submittedSettings{
			Caption:         pp.Caption,
			MediaURLs:       pp.MediaURLs,
			MediaIDs:        pp.MediaIDs,
			PlatformOptions: pp.PlatformOptions,
			FirstComment:    pp.FirstComment,
			InReplyTo:       pp.InReplyTo,
			ThreadPosition:  pp.ThreadPosition,
		}
	}
	return out
}

func applyTikTokLiveStatus(ctx context.Context, rr *postResultResponse, status map[string]any, adapter *platform.TikTokAdapter, accessToken string) {
	rr.PublishStatus = status
	data, _ := status["data"].(map[string]any)
	if data == nil {
		return
	}

	if url := platform.TikTokPublicPostURLFromStatusData(data); url != "" {
		rr.URL = &url
	} else if adapter != nil {
		if fallback := adapter.ResolvePostOrProfileURL(ctx, accessToken, status); fallback != "" {
			rr.URL = &fallback
		} else if rr.URL != nil && *rr.URL == "https://www.tiktok.com" {
			rr.URL = nil
		}
	} else if rr.URL != nil && *rr.URL == "https://www.tiktok.com" {
		rr.URL = nil
	}

	publishStatus, _ := data["status"].(string)
	switch publishStatus {
	case "PUBLISH_COMPLETE":
		if rr.Status == "processing" || rr.Status == "published" {
			rr.Status = "published"
		}
	case "FAILED":
		rr.Status = "failed"
		if rr.ErrorMessage == nil {
			reason, _ := data["fail_reason"].(string)
			if strings.TrimSpace(reason) == "" {
				reason = "unknown_error"
			}
			msg := "tiktok publish failed: " + reason
			rr.ErrorMessage = &msg
		}
	default:
		if rr.Status == "published" || rr.Status == "processing" {
			rr.Status = "processing"
		}
	}
}

// accountSummary is what the List handler stores per social_account_id so the
// per-result rows can resolve both the platform name and the human-readable
// display name in a single map lookup.
type accountSummary struct {
	Platform string
	Name     string
}

type socialPostResponse struct {
	ID                 string     `json:"id"`
	Caption            *string    `json:"caption"`
	MediaURLs          []string   `json:"media_urls,omitempty"`
	Status             string     `json:"status"`
	ExecutionMode      string     `json:"execution_mode,omitempty"`
	QueuedResultsCount int        `json:"queued_results_count,omitempty"`
	ActiveJobCount     int        `json:"active_job_count,omitempty"`
	RetryingCount      int        `json:"retrying_count,omitempty"`
	DeadCount          int        `json:"dead_count,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	ScheduledAt        *time.Time `json:"scheduled_at,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty"`
	// Source identifies who triggered this post: "ui" (dashboard) vs
	// "api" (external API key). Stamped at row creation from the auth
	// context and never changes thereafter.
	Source string `json:"source"`
	// ProfileIDs is the set of distinct profile_ids derived from the
	// target social_accounts — a single post can target accounts across
	// multiple profiles. Drives per-profile filtering in the dashboard.
	ProfileIDs []string `json:"profile_ids"`
	// TargetPlatforms are derived from the post's stored metadata, so the
	// dashboard can still show intended platforms before or without any
	// social_post_results rows.
	TargetPlatforms []string             `json:"target_platforms,omitempty"`
	Results         []postResultResponse `json:"results,omitempty"`
}

// Create handles POST /v1/social-posts.
//
// Sprint 1 rewrite — accepts both the legacy shape (caption +
// account_ids) and the new AgentPost shape (platform_posts[] with
// per-account captions). The two shapes are normalized to a single
// internal []PlatformPostInput by parsePublishRequest, then validated
// once via the same pure function /validate uses, then dispatched to
// either the scheduled or immediate path.
//
// Idempotency: when the request carries an idempotency_key, we look
// for an existing post with that (project_id, key) within the 24h
// window. On hit we hydrate the prior response and return it
// unchanged — no new platform posts are created.
//
// Validation behavior: structural errors (caption too long, mixing
// image+video, missing required media, schedule out of range, etc.)
// return 400 with the issue list. Account-state errors (disconnected,
// not in workspace) are recorded as failed per-account results so the
// overall publish doesn't get blocked by one bad account — preserves
// legacy soft-failure semantics.
func (h *SocialPostHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body publishRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	parsed, status, msg := parsePublishRequest(body)
	if status != 0 {
		writeError(w, status, "VALIDATION_ERROR", msg)
		return
	}

	// Idempotency replay — if this key already produced a row, return
	// the prior response unchanged. Cheap: one indexed lookup.
	if parsed.IdempotencyKey != "" {
		if existing, err := h.queries.GetSocialPostByIdempotencyKey(r.Context(), db.GetSocialPostByIdempotencyKeyParams{
			WorkspaceID:    workspaceID,
			IdempotencyKey: pgtype.Text{String: parsed.IdempotencyKey, Valid: true},
		}); err == nil {
			h.writeReplayedPost(w, r, existing)
			return
		}
		// pgx.ErrNoRows is the expected miss; anything else we treat
		// as transient and proceed (better to risk a duplicate than
		// to block a publish on a flaky lookup).
	}

	// Quota headers (soft — never blocks).
	quotaStatus := h.quota.Check(r.Context(), workspaceID)
	w.Header().Set("X-UniPost-Usage", fmt.Sprintf("%d/%d", quotaStatus.Usage, quotaStatus.Limit))
	if quotaStatus.Warning != "" {
		w.Header().Set("X-UniPost-Warning", quotaStatus.Warning)
	}

	// Load accounts once so the validator and the publish loop both
	// see the same view of which accounts the workspace owns.
	accountMap, err := h.loadValidateAccounts(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		return
	}

	// Load media rows referenced by the request so the validator can
	// check ownership, status, and content type.
	// Run the same validator /social-posts/validate uses, then filter
	// out non-fatal issues (account_disconnected, account_not_in_workspace)
	// — those are still recorded as failed results below to preserve
	// legacy soft-failure semantics.
	vr := h.runPublishValidation(r, workspaceID, parsed.Posts, parsed.ScheduledAt, accountMap)
	// Drafts (Sprint 2): persist with status='draft', skip the
	// publish loop entirely, but still SURFACE validation results in
	// the response so the user can see what's wrong before publishing.
	// Validation errors do NOT block draft creation — drafts are an
	// editing surface, not a transactional surface.
	if parsed.Status == "draft" {
		h.createDraft(w, r, workspaceID, parsed, vr)
		return
	}

	if fatal := filterFatalIssues(vr.Errors); len(fatal) > 0 {
		writeValidationErrors(w, fatal)
		return
	}

	// Branch on scheduled vs immediate.
	if parsed.ScheduledAt != nil {
		h.createScheduledPost(w, r, workspaceID, parsed)
		return
	}
	h.createImmediatePost(w, r, workspaceID, parsed, accountMap)
}

// createScheduledPost persists the post with status="scheduled" and
// the v2 metadata blob so the scheduler can later fan out per-account
// when the time arrives. Returns 200 with a minimal response (no
// results yet — they'll exist after the scheduler fires).
func (h *SocialPostHandler) createScheduledPost(w http.ResponseWriter, r *http.Request, workspaceID string, parsed parsedRequest) {
	// Persist the parsed request shape into metadata so the scheduler
	// can reconstruct the per-account captions.
	metaJSON, err := platform.EncodePostMetadata(parsed.Posts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode metadata")
		return
	}

	// social_posts.caption is the canonical / "first" caption — used
	// by legacy reads and the dashboard hash UI. We populate it from
	// the first platform post so existing consumers keep working.
	canonicalCaption := pgtype.Text{}
	if len(parsed.Posts) > 0 && parsed.Posts[0].Caption != "" {
		canonicalCaption = pgtype.Text{String: parsed.Posts[0].Caption, Valid: true}
	}

	// media_urls on the parent row is also legacy — fall back to the
	// first post's media so dashboard previews keep showing something.
	canonicalMedia := []string{}
	if len(parsed.Posts) > 0 {
		canonicalMedia = parsed.Posts[0].MediaURLs
	}
	if canonicalMedia == nil {
		canonicalMedia = []string{}
	}

	post, err := h.queries.CreateSocialPost(r.Context(), db.CreateSocialPostParams{
		WorkspaceID:    workspaceID,
		Caption:        canonicalCaption,
		MediaUrls:      canonicalMedia,
		Status:         "scheduled",
		Metadata:       metaJSON,
		ScheduledAt:    pgtype.Timestamptz{Time: *parsed.ScheduledAt, Valid: true},
		IdempotencyKey: idempotencyKeyParam(parsed.IdempotencyKey),
		Source:         resolveSource(r.Context()),
		ProfileIds:     h.resolveProfileIDs(r.Context(), workspaceID, uniqueAccountIDs(parsed.Posts)),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create scheduled post")
		return
	}

	var caption *string
	if post.Caption.Valid {
		caption = &post.Caption.String
	}
	scheduledAt := post.ScheduledAt.Time

	writeCreated(w, socialPostResponse{
		ID:          post.ID,
		Caption:     caption,
		MediaURLs:   post.MediaUrls,
		Status:      "scheduled",
		CreatedAt:   post.CreatedAt.Time,
		ScheduledAt: &scheduledAt,
		Source:      post.Source,
		ProfileIDs:  post.ProfileIds,
	})
}

// createImmediatePost persists the post and queues one delivery job per
// target input. The actual adapter dispatch happens asynchronously in
// the background workers.
func (h *SocialPostHandler) createImmediatePost(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID string,
	parsed parsedRequest,
	accountMap map[string]platform.ValidateAccount,
) {
	resp, err := h.executeImmediatePost(r, workspaceID, parsed, accountMap)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeSuccess(w, resp)
}

// executeImmediatePost is createImmediatePost without the response
// writer — it returns either the success response or an error. Used
// by both single (Create) and bulk (CreateBulk) entry points.
func (h *SocialPostHandler) executeImmediatePost(
	r *http.Request,
	workspaceID string,
	parsed parsedRequest,
	accountMap map[string]platform.ValidateAccount,
) (socialPostResponse, error) {
	return h.queueImmediatePost(r.Context(), workspaceID, parsed, accountMap)
}

// publishExistingPost is the publish-from-draft entry point. The parent
// row already exists after ClaimDraftForPublish locked it; we now create
// pending result rows plus dispatch jobs instead of publishing inline.
func (h *SocialPostHandler) publishExistingPost(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID string,
	post db.SocialPost,
	parsed parsedRequest,
	accountMap map[string]platform.ValidateAccount,
) {
	uniqueIDs := uniqueAccountIDs(parsed.Posts)
	post.ProfileIds = h.ensureProfileIDsForPost(r.Context(), post, uniqueIDs)
	resp, err := h.enqueueExistingPostDeliveries(r.Context(), post, parsed.Posts, accountMap)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeSuccess(w, resp)
}

// runPublishLoop is the shared body of createImmediatePost and
// publishExistingPost. Takes a parent post that already exists in
// status='publishing', dispatches each PlatformPostInput to its
// adapter, persists results, updates the parent status, fires the
// webhook event, and writes the response.
func (h *SocialPostHandler) runPublishLoop(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID string,
	post db.SocialPost,
	parsed parsedRequest,
	dbAccounts map[string]db.SocialAccount,
	accountMap map[string]platform.ValidateAccount,
) {
	resp := h.executePublishLoop(r, workspaceID, post, parsed, dbAccounts, accountMap)
	writeSuccess(w, resp)
}

// executePublishLoop is the shared body without the writeSuccess. Used
// by both runPublishLoop (single-post path) and CreateBulk (bulk path).
// Persists results, updates parent status, fires the webhook event,
// increments quota — everything except writing the HTTP response.
func (h *SocialPostHandler) executePublishLoop(
	r *http.Request,
	workspaceID string,
	post db.SocialPost,
	parsed parsedRequest,
	dbAccounts map[string]db.SocialAccount,
	accountMap map[string]platform.ValidateAccount,
) socialPostResponse {
	// Sprint 5 PR2: per-account monthly quota. Load the workspace once
	// to read its per_account_monthly_limit, then snapshot the
	// current-month publish counts for every account this request
	// will dispatch to. The tracker decrements as publishOne fires;
	// any account that runs out gets a deterministic
	// "per_account_monthly_quota_exceeded" error on its result row
	// instead of a dispatch. Workspace lookup failures degrade to
	// "no cap" rather than blocking — the legacy behavior is the
	// safer fallback if metadata is briefly unavailable.
	var perAccountLimit pgtype.Int4
	if ws, wsErr := h.queries.GetWorkspace(r.Context(), workspaceID); wsErr == nil {
		perAccountLimit = ws.PerAccountMonthlyLimit
	}
	tracker := quota.NewPerAccountTracker(
		r.Context(),
		h.queries,
		perAccountLimit,
		uniqueAccountIDs(parsed.Posts),
	)

	// Publish each platform post. Standalone posts run in parallel
	// (one goroutine each); thread groups run serially within their
	// group but groups are still parallel with each other. The
	// outcomes slice is indexed by input position so the per-post
	// caption row in the response stays in lockstep with the input.
	outcomes := make([]publishOneOutcome, len(parsed.Posts))

	// Group indices by (account_id, has-thread-position). Posts with
	// thread_position > 0 share a serial group keyed by account_id;
	// every standalone post gets its own group.
	groups := groupForDispatch(parsed.Posts)

	var wg sync.WaitGroup
	for _, group := range groups {
		wg.Add(1)
		go func(g []int) {
			defer wg.Done()
			h.runDispatchGroup(r, g, parsed.Posts, dbAccounts, accountMap, outcomes, tracker)
		}(group)
	}
	wg.Wait()

	// Persist results + build response in input order.
	var responseResults []postResultResponse
	allPublished := true
	anyPublished := false
	publishedCount := 0
	persistErrors := make([]string, 0) // track insert failures so we can surface them on the parent post

	for i, oc := range outcomes {
		var extID, errMsg, postURL pgtype.Text
		var pubAt pgtype.Timestamptz
		status := "published"

		if oc.err != nil {
			status = "failed"
			errMsg = pgtype.Text{String: oc.err.Error(), Valid: true}
			allPublished = false
		} else if oc.result != nil {
			extID = pgtype.Text{String: oc.result.ExternalID, Valid: true}
			pubAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			if oc.result.URL != "" {
				postURL = pgtype.Text{String: oc.result.URL, Valid: true}
			}
			// Adapters may override the default "published" with
			// "processing" when the upstream is still finishing the
			// job (currently Facebook videos when our poll times
			// out). A processing row doesn't count toward the
			// parent post's "all published" summary — the Get
			// handler's re-poll is what flips it to "published"
			// once the platform confirms completion.
			if oc.result.Status != "" {
				status = oc.result.Status
			}
			if status == "published" {
				anyPublished = true
				publishedCount++
			} else {
				allPublished = false
			}
		}

		// Only store the debug curl dump on failed rows — successful
		// publishes don't need diagnostic data and it'd bloat the row
		// unnecessarily. Nil-valued when the recorder was empty
		// (every request 2xx'd, or nothing made an HTTP call).
		var debugCurl pgtype.Text
		if status == "failed" && oc.debugCurl != "" {
			debugCurl = pgtype.Text{String: oc.debugCurl, Valid: true}
		}

		dbResult, dbErr := h.queries.CreateSocialPostResult(r.Context(), db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: parsed.Posts[i].AccountID,
			Caption:         parsed.Posts[i].Caption,
			Status:          status,
			ExternalID:      extID,
			ErrorMessage:    errMsg,
			PublishedAt:     pubAt,
			Url:             postURL,
			DebugCurl:       debugCurl,
		})
		if dbErr != nil {
			// Most common cause: FK violation from a deleted social account.
			// Capture the reason so the user sees why the post failed even
			// though no result row exists.
			reason := oc.err
			if reason == nil {
				reason = dbErr
			}
			persistErrors = append(persistErrors,
				fmt.Sprintf("account %s: %s (result not saved: %v)",
					parsed.Posts[i].AccountID, reason.Error(), dbErr))
			slog.Error("failed to save post result",
				"error", dbErr, "account_id", parsed.Posts[i].AccountID, "dispatch_error", reason.Error())
			allPublished = false
			h.recordPostFailure(r.Context(), postfailures.BuildParams(
				post.ID,
				"",
				workspaceID,
				parsed.Posts[i].AccountID,
				postfailures.FirstNonEmpty(oc.platform, accountMap[parsed.Posts[i].AccountID].Platform),
				"result_persist",
				reason.Error(),
				dbErr.Error(),
			))
			continue
		}

		if status == "failed" && dbResult.ErrorMessage.Valid {
			h.recordPostFailure(r.Context(), postfailures.BuildParams(
				post.ID,
				dbResult.ID,
				workspaceID,
				dbResult.SocialAccountID,
				postfailures.FirstNonEmpty(oc.platform, accountMap[parsed.Posts[i].AccountID].Platform),
				"dispatch",
				dbResult.ErrorMessage.String,
				dbResult.ErrorMessage.String,
			))
		}

		rr := postResultResponse{
			ID:              dbResult.ID,
			SocialAccountID: dbResult.SocialAccountID,
			Platform:        oc.platform,
			AccountName:     oc.accountName,
			Caption:         dbResult.Caption,
			Status:          dbResult.Status,
		}
		if dbResult.ExternalID.Valid {
			rr.ExternalID = &dbResult.ExternalID.String
		}
		if dbResult.Url.Valid {
			rr.URL = &dbResult.Url.String
		}
		if dbResult.ErrorMessage.Valid {
			rr.ErrorMessage = &dbResult.ErrorMessage.String
		}
		if dbResult.PublishedAt.Valid {
			t := dbResult.PublishedAt.Time.Format(time.RFC3339)
			rr.PublishedAt = &t
		}
		if dbResult.DebugCurl.Valid {
			rr.DebugCurl = &dbResult.DebugCurl.String
		}
		// Submitted settings come straight from parsed.Posts[i] — the
		// request we just dispatched — instead of re-decoding metadata.
		// Parity with the read handlers that decode from the DB blob.
		rr.Submitted = &submittedSettings{
			Caption:         parsed.Posts[i].Caption,
			MediaURLs:       parsed.Posts[i].MediaURLs,
			MediaIDs:        parsed.Posts[i].MediaIDs,
			PlatformOptions: parsed.Posts[i].PlatformOptions,
			FirstComment:    parsed.Posts[i].FirstComment,
			InReplyTo:       parsed.Posts[i].InReplyTo,
			ThreadPosition:  parsed.Posts[i].ThreadPosition,
		}
		// Sprint 4 PR3: surface first_comment failure as a warning
		// without affecting the main result status.
		if oc.firstCommentWarning != "" {
			rr.Warnings = append(rr.Warnings, "first_comment_failed: "+oc.firstCommentWarning)
		}
		responseResults = append(responseResults, rr)
	}

	// Update parent status.
	postStatus := "failed"
	if allPublished {
		postStatus = "published"
	} else if anyPublished {
		postStatus = "partial"
	}
	var publishedAt pgtype.Timestamptz
	if anyPublished {
		publishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	h.queries.UpdateSocialPostStatus(r.Context(), db.UpdateSocialPostStatusParams{
		ID:          post.ID,
		Status:      postStatus,
		PublishedAt: publishedAt,
	})

	// When the post fails, persist a human-readable error summary on
	// the parent post's metadata so users can understand why.
	if postStatus == "failed" {
		errSummary := make([]string, 0, len(outcomes))
		for i, oc := range outcomes {
			if oc.err != nil {
				errSummary = append(errSummary, fmt.Sprintf("[%s] %s", parsed.Posts[i].AccountID, oc.err.Error()))
			}
		}
		errSummary = append(errSummary, persistErrors...)
		summaryText := strings.Join(errSummary, "; ")
		if summaryText == "" {
			summaryText = "Post failed with no platform results. This usually means the target account was deleted, disconnected, or never existed."
		}
		_ = h.queries.UpdateSocialPostErrorMetadata(r.Context(), db.UpdateSocialPostErrorMetadataParams{
			ID:      post.ID,
			Column2: summaryText,
		})
		slog.Warn("social post failed",
			"post_id", post.ID,
			"workspace_id", workspaceID,
			"errors", summaryText,
			"result_count", len(responseResults))
	}

	if publishedCount > 0 {
		h.quota.Increment(r.Context(), workspaceID, publishedCount)
	}

	var caption *string
	if post.Caption.Valid {
		caption = &post.Caption.String
	}
	resp := socialPostResponse{
		ID:         post.ID,
		Caption:    caption,
		MediaURLs:  post.MediaUrls,
		Status:     postStatus,
		CreatedAt:  post.CreatedAt.Time,
		Source:     post.Source,
		ProfileIDs: post.ProfileIds,
		Results:    responseResults,
	}

	// Fan out webhook events. Best-effort — Publish recovers panics
	// internally and never blocks the response. The event payload is
	// the same socialPostResponse the caller just got back, so
	// subscribers can correlate by post ID.
	h.bus.Publish(r.Context(), workspaceID, eventForStatus(postStatus), resp)

	return resp
}

func (h *SocialPostHandler) recordPostFailure(ctx context.Context, arg db.CreatePostFailureParams) {
	if _, err := h.queries.CreatePostFailure(ctx, arg); err != nil {
		slog.Warn("failed to persist structured post failure",
			"post_id", arg.PostID,
			"platform", arg.Platform,
			"stage", arg.FailureStage,
			"error", err)
	}
}

// eventForStatus maps a post status to its outbound webhook event
// name. Centralized so the immediate path, the scheduler, and any
// future replay path all agree.
func eventForStatus(postStatus string) string {
	switch postStatus {
	case "published":
		return events.EventPostPublished
	case "partial":
		return events.EventPostPartial
	case "failed":
		return events.EventPostFailed
	default:
		return events.EventPostFailed
	}
}

// groupForDispatch buckets parsed.Posts indices into dispatch groups.
// Standalone posts get their own single-element group. Threaded posts
// (thread_position > 0) on the same account share one group and are
// returned in thread_position order so the runDispatchGroup loop can
// chain them in the right sequence.
//
// Output is a slice of int slices: each inner slice is one group.
// Order between groups is irrelevant (they run concurrently); order
// within a group matters (threads run sequentially).
func groupForDispatch(posts []platform.PlatformPostInput) [][]int {
	// First pass: collect threaded indices keyed by account_id.
	threaded := make(map[string][]int)
	var standalone []int
	for i, p := range posts {
		if p.ThreadPosition > 0 && p.AccountID != "" {
			threaded[p.AccountID] = append(threaded[p.AccountID], i)
		} else {
			standalone = append(standalone, i)
		}
	}

	groups := make([][]int, 0, len(standalone)+len(threaded))
	for _, idx := range standalone {
		groups = append(groups, []int{idx})
	}
	for _, idxs := range threaded {
		// Sort by ThreadPosition so the serial chain runs in the
		// declared order even when the input arrived out of order.
		sortIndicesByThreadPosition(idxs, posts)
		groups = append(groups, idxs)
	}
	return groups
}

// sortIndicesByThreadPosition sorts an []int of indices into posts by
// posts[i].ThreadPosition ascending. Tiny insertion sort — thread
// groups cap at ~25 entries (Twitter), so an O(n²) sort is faster
// than calling sort.Slice + reflect overhead.
func sortIndicesByThreadPosition(idxs []int, posts []platform.PlatformPostInput) {
	for i := 1; i < len(idxs); i++ {
		j := i
		for j > 0 && posts[idxs[j]].ThreadPosition < posts[idxs[j-1]].ThreadPosition {
			idxs[j], idxs[j-1] = idxs[j-1], idxs[j]
			j--
		}
	}
}

// runDispatchGroup runs one dispatch group. Standalone groups (single
// index) just call publishOne once. Thread groups iterate in order,
// passing the previous tweet's external_id into the next post's opts
// as in_reply_to_tweet_id so the adapter chains the reply chain
// correctly.
//
// Mid-thread failure: any error STOPS the chain. Remaining tweets in
// the group are marked as failed with a clear "upstream thread post
// failed" error so the response shows exactly which tweet broke and
// which weren't even attempted.
func (h *SocialPostHandler) runDispatchGroup(
	r *http.Request,
	groupIndices []int,
	posts []platform.PlatformPostInput,
	dbAccounts map[string]db.SocialAccount,
	accountMap map[string]platform.ValidateAccount,
	outcomes []publishOneOutcome,
	tracker *quota.PerAccountTracker,
) {
	// threadState carries per-platform thread plumbing across iterations.
	// Twitter only needs the previous tweet id; Bluesky needs root URI+CID
	// (frozen after post 1) and parent URI+CID (updated each iteration).
	// Other platforms ignore everything in here.
	var (
		prevExternalID                         string // twitter
		rootURI, rootCID, parentURI, parentCID string // bluesky
	)
	for chainIdx, postIdx := range groupIndices {
		pp := posts[postIdx]

		if chainIdx > 0 {
			// Copy opts so we don't mutate the caller's map.
			clone := make(map[string]any, len(pp.PlatformOptions)+4)
			for k, v := range pp.PlatformOptions {
				clone[k] = v
			}
			if prevExternalID != "" {
				clone["in_reply_to_tweet_id"] = prevExternalID
			}
			if rootURI != "" {
				clone["thread_root_uri"] = rootURI
				clone["thread_root_cid"] = rootCID
				clone["thread_parent_uri"] = parentURI
				clone["thread_parent_cid"] = parentCID
			}
			pp.PlatformOptions = clone
		}

		oc := h.publishOne(r, pp, dbAccounts, accountMap, tracker)
		outcomes[postIdx] = oc

		if oc.err != nil {
			// Stop the chain: every later tweet in this group is
			// marked as not-attempted with a deterministic error so
			// the response is honest about what happened.
			for skipIdx := chainIdx + 1; skipIdx < len(groupIndices); skipIdx++ {
				skipPost := posts[groupIndices[skipIdx]]
				outcomes[groupIndices[skipIdx]] = publishOneOutcome{
					platform:    oc.platform,
					accountName: oc.accountName,
					err:         fmt.Errorf("upstream thread post failed at thread_position %d: %w", pp.ThreadPosition, oc.err),
				}
				_ = skipPost // unused but kept for clarity in case we add per-skip logging later
			}
			return
		}
		if oc.result != nil {
			prevExternalID = oc.result.ExternalID
			parentURI = oc.result.ExternalID
			parentCID = oc.result.CID
			if chainIdx == 0 {
				rootURI = oc.result.ExternalID
				rootCID = oc.result.CID
			}
		}
	}
}

// publishOne dispatches a single PlatformPostInput to its adapter.
// Handles token decryption, expired-token refresh, and the actual
// adapter.Post call. The caller is expected to merge per-platform
// options from the parent payload before calling — but for v1 we
// take options off the per-post struct directly so they're already
// scoped to the right account.
func (h *SocialPostHandler) publishOne(
	r *http.Request,
	pp platform.PlatformPostInput,
	dbAccounts map[string]db.SocialAccount,
	accountMap map[string]platform.ValidateAccount,
	tracker *quota.PerAccountTracker,
) (oc publishOneOutcome) {
	return h.publishOneContext(r.Context(), pp, dbAccounts, accountMap, tracker)
}

func (h *SocialPostHandler) publishOneContext(
	ctx context.Context,
	pp platform.PlatformPostInput,
	dbAccounts map[string]db.SocialAccount,
	accountMap map[string]platform.ValidateAccount,
	tracker *quota.PerAccountTracker,
) (oc publishOneOutcome) {
	// Resolve account.
	acc, ok := dbAccounts[pp.AccountID]
	if !ok {
		// Either not in project or disconnected — accountMap may
		// still know the platform.
		summary := accountMap[pp.AccountID]
		oc.platform = summary.Platform
		if summary.Platform == "" {
			oc.err = fmt.Errorf("account not found")
		} else if summary.Disconnected {
			oc.err = fmt.Errorf("account is disconnected")
		} else {
			oc.err = fmt.Errorf("account not found")
		}
		return
	}
	oc.platform = acc.Platform
	if acc.AccountName.Valid {
		oc.accountName = acc.AccountName.String
	}

	if acc.DisconnectedAt.Valid {
		oc.err = fmt.Errorf("account is disconnected")
		return
	}

	// Sprint 5 PR2: per-account monthly quota gate. Atomically
	// check-and-decrement the per-request budget for this account.
	// Refusal here is final for this dispatch — no adapter call,
	// no token decrypt, no media resolve. The result row carries
	// the deterministic error string so the caller (and the
	// dashboard) can distinguish "you hit your cap" from
	// "the platform rejected the post".
	if tracker != nil && !tracker.Allow(acc.ID) {
		oc.err = quota.ErrPerAccountQuotaExceeded
		return
	}

	adapter, err := platform.Get(acc.Platform)
	if err != nil {
		oc.err = err
		return
	}
	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		oc.err = err
		return
	}

	// Inline token refresh if expired. We only persist the rotated
	// tokens when EVERY step succeeds — a silent failure here used
	// to overwrite good tokens with empty strings (see TikTok
	// "Failed to fetch" after publish). Defensive: an error anywhere
	// in the chain leaves the stored tokens untouched so the worker
	// can retry the refresh later.
	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
		if refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String); decErr == nil {
			if newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(ctx, refreshTok); refErr == nil && newAccess != "" {
				encAccess, encErr := h.encryptor.Encrypt(newAccess)
				encRefresh, encErr2 := h.encryptor.Encrypt(newRefresh)
				if encErr == nil && encErr2 == nil {
					accessToken = newAccess
					if updateErr := h.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
						ID:             acc.ID,
						AccessToken:    encAccess,
						RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
						TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
					}); updateErr != nil {
						slog.Error("token refresh: update failed", "account_id", acc.ID, "platform", acc.Platform, "error", updateErr)
					}
				} else {
					slog.Error("token refresh: encrypt failed", "account_id", acc.ID, "platform", acc.Platform, "access_err", encErr, "refresh_err", encErr2)
				}
			} else if refErr != nil {
				slog.Error("token refresh: upstream failed", "account_id", acc.ID, "platform", acc.Platform, "error", refErr)
			}
		}
	}

	// Resolve any media_ids to presigned download URLs and append
	// them to the URL list. The adapter doesn't care about the
	// distinction — both halves end up in the same MediaItem slice.
	// Errors here are fatal for the post (we can't dispatch without
	// the media), so we surface them as the post's err.
	mediaURLs := append([]string(nil), pp.MediaURLs...)
	if len(pp.MediaIDs) > 0 {
		extra, mediaErr := h.resolveMediaIDsToURLs(ctx, pp.MediaIDs)
		if mediaErr != nil {
			oc.err = mediaErr
			return
		}
		mediaURLs = append(mediaURLs, extra...)
	}

	// Per-platform routing log — emitted at INFO so smoke-tests can
	// verify each PlatformPostInput is reaching the right adapter
	// with the right caption. Mirrors the same line in scheduler.go
	// so the immediate and scheduled paths produce comparable output.
	slog.Info("publish: dispatching to adapter",
		"account_id", acc.ID,
		"platform", acc.Platform,
		"caption_preview", truncateForLog(pp.Caption, 40),
		"media_urls", len(mediaURLs))

	// Attach a debugrt recorder so the shared RoundTripper captures
	// failing HTTP requests into a curl dump we can persist on the
	// result row. Per-dispatch so concurrent publishes don't trample
	// each other.
	debugRec := debugrt.NewRecorder()
	dispatchCtx := debugrt.WithRecorder(ctx, debugRec)

	postResult, err := adapter.Post(
		dispatchCtx,
		accessToken,
		pp.Caption,
		platform.MediaFromURLs(mediaURLs),
		pp.PlatformOptions,
	)
	oc.result = postResult
	oc.err = err
	oc.debugCurl = debugRec.Serialize()

	// Sprint 4 PR3: first_comment dispatch. Only fires when the main
	// post succeeded AND the adapter implements FirstCommentAdapter
	// (validator already rejected first_comment on platforms that
	// don't support it). Failure of the first comment NEVER rolls
	// back the main post — it's recorded as a warning instead.
	if err == nil && postResult != nil && pp.FirstComment != "" {
		if commenter, ok := adapter.(platform.FirstCommentAdapter); ok {
			if _, ferr := commenter.PostComment(ctx, accessToken, postResult.ExternalID, pp.FirstComment); ferr != nil {
				oc.firstCommentWarning = ferr.Error()
				slog.Warn("first_comment failed",
					"account_id", acc.ID,
					"platform", acc.Platform,
					"parent_id", postResult.ExternalID,
					"err", ferr,
				)
			}
		}
	}
	return
}

// resolveMediaIDsToURLs is the publish-time half of the media library
// flow. For each media_id the caller referenced in platform_posts:
//
//  1. Fetch the row from the media table.
//  2. If status is "pending", HEAD the R2 object and hydrate the row
//     (the same poll-on-attach pattern the GET /v1/media endpoint
//     uses). If R2 says the object isn't there, fail.
//  3. Mint a fresh 15-minute presigned download URL for the adapter.
//
// Returns the URLs in the same order as the input media_ids so the
// caller can interleave them with the per-post media_urls list.
func (h *SocialPostHandler) resolveMediaIDsToURLs(ctx context.Context, mediaIDs []string) ([]string, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	if h.storage == nil {
		return nil, fmt.Errorf("media_ids supplied but R2 storage is not configured on this server")
	}
	out := make([]string, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		row, err := h.queries.GetMedia(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("media_id %s: %w", id, err)
		}
		// Hydrate pending rows lazily.
		if row.Status == "pending" {
			head, hErr := h.storage.Head(ctx, row.StorageKey)
			if hErr != nil || !head.Exists {
				return nil, fmt.Errorf("media_id %s: not yet uploaded", id)
			}
			updated, uErr := h.queries.MarkMediaUploaded(ctx, db.MarkMediaUploadedParams{
				ID:          row.ID,
				SizeBytes:   head.SizeBytes,
				ContentType: pickContentType(head.ContentType, row.ContentType),
			})
			if uErr == nil {
				row = updated
			}
		}
		dlURL, err := h.storage.PresignGet(ctx, row.StorageKey, 15*time.Minute)
		if err != nil {
			return nil, fmt.Errorf("media_id %s: presign get: %w", id, err)
		}
		out = append(out, dlURL)
	}
	return out, nil
}

// truncateForLog returns a copy of s shortened to at most n runes,
// appending an ellipsis if it was actually truncated. Used to keep
// dispatch log lines bounded when captions get long.
func truncateForLog(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// publishOneOutcome is what publishOne returns. Pulled out into a named
// type so the goroutine in createImmediatePost can declare a fixed-size
// slice without re-stating the field names.
//
// firstCommentWarning (Sprint 4 PR3) is set when the main post landed
// successfully but the first_comment failed. The main result still
// reports status='published' — the comment failure is informational.
type publishOneOutcome struct {
	platform            string
	accountName         string
	result              *platform.PostResult
	err                 error
	firstCommentWarning string
	// debugCurl is the serialized curl+response dump of every non-2xx
	// HTTP request the adapter made during this dispatch (see
	// internal/debugrt). Populated whenever entries exist, but only
	// persisted on failure — successful publishes ignore it.
	debugCurl string
}

// resolveSource classifies the publish trigger as either "ui" (Clerk
// session) or "api" (API key). We branch on GetUserID: the Clerk
// middleware stamps user_id; the API key middleware doesn't (it only
// stamps workspace_id). Stamped once at create time and never changes.
func resolveSource(ctx context.Context) string {
	if auth.GetUserID(ctx) != "" {
		return "ui"
	}
	return "api"
}

// resolveProfileIDs looks up the distinct profile_ids across the
// request's target accounts. Used at create time to populate
// social_posts.profile_ids so per-profile dashboards can filter posts
// via `profile_id = ANY(profile_ids)`. Empty slice is valid — the row
// still inserts; filters just won't match it.
func (h *SocialPostHandler) resolveProfileIDs(ctx context.Context, workspaceID string, accountIDs []string) []string {
	if len(accountIDs) == 0 {
		return []string{}
	}
	ids, err := h.queries.GetDistinctProfileIDsForAccounts(ctx, db.GetDistinctProfileIDsForAccountsParams{
		Column1:     accountIDs,
		WorkspaceID: workspaceID,
	})
	if err != nil || ids == nil {
		return []string{}
	}
	return ids
}

// ensureProfileIDsForPost lazy-populates profile_ids on rows created
// before migration 043 landed. Called from the publish/claim paths
// (draft publish, scheduler worker) so pre-migration drafts and
// scheduled posts still get correct per-profile attribution when they
// eventually publish. No-op when the row already has profile_ids.
func (h *SocialPostHandler) ensureProfileIDsForPost(ctx context.Context, post db.SocialPost, accountIDs []string) []string {
	if len(post.ProfileIds) > 0 {
		return post.ProfileIds
	}
	ids := h.resolveProfileIDs(ctx, post.WorkspaceID, accountIDs)
	if len(ids) == 0 {
		return ids
	}
	_ = h.queries.SetSocialPostProfileIDs(ctx, db.SetSocialPostProfileIDsParams{
		ID:         post.ID,
		ProfileIds: ids,
	})
	return ids
}

// uniqueAccountIDs returns the distinct account IDs across a slice of
// PlatformPostInput. Used by createImmediatePost so we only fetch each
// account once even when the same account appears multiple times
// (e.g. a thread with two tweets from the same handle).
func uniqueAccountIDs(posts []platform.PlatformPostInput) []string {
	seen := make(map[string]bool, len(posts))
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		if p.AccountID == "" || seen[p.AccountID] {
			continue
		}
		seen[p.AccountID] = true
		out = append(out, p.AccountID)
	}
	return out
}

// idempotencyKeyParam wraps a string into the pgtype.Text shape sqlc
// expects, returning an invalid (NULL) value when the key is empty.
func idempotencyKeyParam(key string) pgtype.Text {
	if key == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: key, Valid: true}
}

// fatalErrorCodes is the set of validator error codes that block
// publish. Account-state codes (disconnected, not_in_project,
// not_found) are intentionally excluded so the publish loop can
// record them as per-account failures and let the rest succeed —
// preserves legacy partial-success semantics.
var fatalErrorCodes = map[string]bool{
	platform.CodeExceedsMaxLength:       true,
	platform.CodeBelowMinLength:         true,
	platform.CodeMissingRequired:        true,
	platform.CodeMaxImagesExceeded:      true,
	platform.CodeMaxVideosExceeded:      true,
	platform.CodeMixedMediaUnsupported:  true,
	platform.CodeUnsupportedInReplyTo:   true,
	platform.CodeScheduledTooSoon:       true,
	platform.CodeScheduledTooFar:        true,
	platform.CodeUnknownPlatform:        true,
	platform.CodeEmptyPosts:             true,
	platform.CodeTooManyPosts:           true,
	platform.CodeUnsupportedFormat:      true,
	platform.CodeFileTooLarge:           true,
	platform.CodeDimensionsOutOfRange:   true,
	platform.CodeAspectRatioUnsupported: true,
	platform.CodeDurationOutOfRange:     true,
	// Sprint 2 thread codes — fatal because the post can't be
	// dispatched in a meaningful way without the structure being
	// intelligible.
	platform.CodeThreadsUnsupported:           true,
	platform.CodeThreadPositionsNotContiguous: true,
	platform.CodeThreadMixedWithSingle:        true,
	// Sprint 2 media-library codes — fatal because the publish path
	// would 4xx if it tried to dispatch with a missing media id.
	platform.CodeMediaIDNotFound:       true,
	platform.CodeMediaIDNotInWorkspace: true,
	platform.CodeMediaNotUploaded:      true,
	// Sprint 4 PR3 first_comment codes.
	platform.CodeFirstCommentUnsupported: true,
	platform.CodeFirstCommentTooLong:     true,
}

// filterFatalIssues splits the validator's full Errors slice into
// just the ones that should block the publish path. See fatalErrorCodes.
func filterFatalIssues(errs []platform.Issue) []platform.Issue {
	out := make([]platform.Issue, 0, len(errs))
	for _, e := range errs {
		if fatalErrorCodes[e.Code] {
			out = append(out, e)
		}
	}
	return out
}

// writeValidationErrors writes a 400 response carrying the structured
// issue list. Mirrors the shape of the /validate endpoint's response so
// clients can use the same error-handling code for both.
func writeValidationErrors(w http.ResponseWriter, errs []platform.Issue) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    "VALIDATION_ERROR",
			"message": "request failed pre-publish validation",
			"issues":  errs,
		},
	})
}

// writeReplayedPost is the writer shim for the single-post path.
// Calls replayedPostResponse and stamps the Idempotent-Replay header.
func (h *SocialPostHandler) writeReplayedPost(w http.ResponseWriter, r *http.Request, post db.SocialPost) {
	resp := h.replayedPostResponse(r, post)
	w.Header().Set("Idempotent-Replay", "true")
	writeSuccess(w, resp)
}

// replayedPostResponse rebuilds a socialPostResponse from a previously-
// stored post (looked up by idempotency_key) and returns it as if it
// were the original publish response. No new platform posts are made.
// Used by both writeReplayedPost (single) and processBulkOne (bulk).
func (h *SocialPostHandler) replayedPostResponse(r *http.Request, post db.SocialPost) socialPostResponse {
	results, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)
	jobs, _ := h.queries.ListPostDeliveryJobsByPost(r.Context(), post.ID)

	// Resolve platform + account name once per result via the workspace
	// account map (cheaper than per-result GetSocialAccount calls).
	allAccounts, _ := h.queries.ListSocialAccountsByWorkspace(r.Context(), post.WorkspaceID)
	accountInfo := make(map[string]struct {
		Platform string
		Name     string
	}, len(allAccounts))
	for _, a := range allAccounts {
		name := ""
		if a.AccountName.Valid {
			name = a.AccountName.String
		}
		accountInfo[a.ID] = struct {
			Platform string
			Name     string
		}{Platform: a.Platform, Name: name}
	}

	resp := socialPostResponse{
		ID:         post.ID,
		MediaURLs:  post.MediaUrls,
		Status:     deriveSocialPostStatus(post, results, jobs),
		CreatedAt:  post.CreatedAt.Time,
		Source:     post.Source,
		ProfileIDs: post.ProfileIds,
	}
	if post.Caption.Valid {
		c := post.Caption.String
		resp.Caption = &c
	}
	if post.PublishedAt.Valid {
		t := post.PublishedAt.Time
		resp.PublishedAt = &t
	}
	if post.ScheduledAt.Valid {
		t := post.ScheduledAt.Time
		resp.ScheduledAt = &t
	}
	if post.ArchivedAt.Valid {
		t := post.ArchivedAt.Time
		resp.ArchivedAt = &t
	}

	submittedByAccount := buildSubmittedMap(post.Metadata, derefText(post.Caption))

	for _, res := range results {
		info := accountInfo[res.SocialAccountID]
		rr := postResultResponse{
			ID:              res.ID,
			SocialAccountID: res.SocialAccountID,
			Platform:        info.Platform,
			AccountName:     info.Name,
			Caption:         res.Caption,
			Status:          res.Status,
		}
		if res.ExternalID.Valid {
			rr.ExternalID = &res.ExternalID.String
		}
		if res.Url.Valid {
			rr.URL = &res.Url.String
		}
		if res.ErrorMessage.Valid {
			rr.ErrorMessage = &res.ErrorMessage.String
		}
		if res.PublishedAt.Valid {
			t := res.PublishedAt.Time.Format(time.RFC3339)
			rr.PublishedAt = &t
		}
		if res.DebugCurl.Valid {
			rr.DebugCurl = &res.DebugCurl.String
		}
		if sub := submittedByAccount[res.SocialAccountID]; sub != nil {
			rr.Submitted = sub
		}
		resp.Results = append(resp.Results, rr)
	}
	applyQueueSummary(&resp, jobs)

	return resp
}

// Get handles GET /v1/social-posts/{id}
func (h *SocialPostHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	post, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get post")
		return
	}

	results, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)
	jobs, _ := h.queries.ListPostDeliveryJobsByPost(r.Context(), post.ID)
	submittedByAccount := buildSubmittedMap(post.Metadata, derefText(post.Caption))
	var responseResults []postResultResponse
	for _, res := range results {
		rr := postResultResponse{
			ID:              res.ID,
			SocialAccountID: res.SocialAccountID,
			Status:          res.Status,
		}
		if res.ExternalID.Valid {
			rr.ExternalID = &res.ExternalID.String
		}
		if res.Url.Valid {
			rr.URL = &res.Url.String
		}
		if res.ErrorMessage.Valid {
			rr.ErrorMessage = &res.ErrorMessage.String
		}
		if res.PublishedAt.Valid {
			t := res.PublishedAt.Time.Format(time.RFC3339)
			rr.PublishedAt = &t
		}
		if res.DebugCurl.Valid {
			rr.DebugCurl = &res.DebugCurl.String
		}
		if sub := submittedByAccount[res.SocialAccountID]; sub != nil {
			rr.Submitted = sub
		}

		// Resolve platform + account display name from social account
		acc, accErr := h.queries.GetSocialAccount(r.Context(), res.SocialAccountID)
		if accErr == nil {
			rr.Platform = acc.Platform
			if acc.AccountName.Valid {
				rr.AccountName = acc.AccountName.String
			}
		}

		// For TikTok, check real-time publish status
		if res.ExternalID.Valid && accErr == nil && acc.Platform == "tiktok" {
			if adapter, adErr := platform.Get("tiktok"); adErr == nil {
				if tiktokAdapter, ok := adapter.(*platform.TikTokAdapter); ok {
					accessToken, decErr := h.encryptor.Decrypt(acc.AccessToken)
					if decErr == nil {
						if status, stErr := tiktokAdapter.CheckPublishStatus(r.Context(), accessToken, res.ExternalID.String); stErr == nil {
							applyTikTokLiveStatus(r.Context(), &rr, status, tiktokAdapter, accessToken)
						}
					}
				}
			}
		}

		// For Facebook videos stuck in "processing", refresh the
		// status from Graph — if FB is now done we flip the row to
		// "published" with the final /posts/{story} URL so the user
		// doesn't have to retry or reload. Only runs for "processing"
		// rows to avoid an unnecessary round-trip on every Get.
		if res.ExternalID.Valid && accErr == nil && acc.Platform == "facebook" && res.Status == "processing" {
			if adapter, adErr := platform.Get("facebook"); adErr == nil {
				if fbAdapter, ok := adapter.(*platform.FacebookAdapter); ok {
					accessToken, decErr := h.encryptor.Decrypt(acc.AccessToken)
					if decErr == nil {
						st, stErr := fbAdapter.CheckVideoStatus(r.Context(), accessToken, res.ExternalID.String)
						if stErr == nil && st != nil {
							// Surface the phase breakdown so the UI can
							// render "Uploading → Processing → Ready".
							rr.PublishStatus = map[string]any{
								"video_status":            st.VideoStatus,
								"uploading_phase_status":  st.UploadingStatus,
								"processing_phase_status": st.ProcessingStatus,
								"publishing_phase_status": st.PublishingStatus,
								"post_id":                 st.PostID,
							}
							// Promote to published when FB finishes.
							if st.VideoStatus == "ready" {
								pageID := acc.ExternalAccountID
								newURL := res.Url.String
								if st.PostID != "" {
									newURL = facebookFeedStoryURL(pageID, st.PostID)
								}
								_, _ = h.queries.UpdateSocialPostResultAfterRetry(r.Context(), db.UpdateSocialPostResultAfterRetryParams{
									ID:           res.ID,
									Status:       "published",
									ExternalID:   res.ExternalID,
									ErrorMessage: pgtype.Text{Valid: false},
									PublishedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
									Url:          pgtype.Text{String: newURL, Valid: newURL != ""},
									DebugCurl:    pgtype.Text{Valid: false},
								})
								rr.Status = "published"
								if newURL != "" {
									rr.URL = &newURL
								}
							} else if st.VideoStatus == "error" {
								errMsg := "Facebook rejected the video"
								if st.ErrorMessage != "" {
									errMsg = "Facebook rejected the video: " + st.ErrorMessage
								}
								_, _ = h.queries.UpdateSocialPostResultAfterRetry(r.Context(), db.UpdateSocialPostResultAfterRetryParams{
									ID:           res.ID,
									Status:       "failed",
									ExternalID:   res.ExternalID,
									ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
									PublishedAt:  pgtype.Timestamptz{Valid: false},
									Url:          res.Url,
									DebugCurl:    pgtype.Text{Valid: false},
								})
								rr.Status = "failed"
								rr.ErrorMessage = &errMsg
							}
						}
					}
				}
			}
		}

		responseResults = append(responseResults, rr)
	}

	var caption *string
	if post.Caption.Valid {
		caption = &post.Caption.String
	}
	var publishedAt *time.Time
	if post.PublishedAt.Valid {
		publishedAt = &post.PublishedAt.Time
	}

	resp := socialPostResponse{
		ID:          post.ID,
		Caption:     caption,
		MediaURLs:   post.MediaUrls,
		Status:      deriveSocialPostStatus(post, results, jobs),
		CreatedAt:   post.CreatedAt.Time,
		PublishedAt: publishedAt,
		Source:      post.Source,
		ProfileIDs:  post.ProfileIds,
		Results:     responseResults,
	}
	applyQueueSummary(&resp, jobs)
	writeSuccess(w, resp)
}

// List handles GET /v1/social-posts. Sprint 2 added query-string
// filters and cursor pagination:
//
//	?status=draft,published    multi-status (comma-separated)
//	?from=2026-04-01T00:00:00Z RFC3339, inclusive lower bound on created_at
//	?to=2026-04-08T00:00:00Z   RFC3339, exclusive upper bound on created_at
//	?limit=25                  default 25, max 100
//	?cursor=...                opaque, returned as next_cursor in the prior page
//
// Cursor format: base64url(created_at|id) — keyset on the
// (created_at DESC, id DESC) index added in migration 019. Stable
// across inserts because it doesn't depend on row offsets.
//
// account_id and platform filters are deferred to Sprint 3 — they
// require EXISTS subqueries against social_post_results and a
// separate index. Sprint 2 ships the filters that share one index.
func (h *SocialPostHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	// Decode the cursor if present. Empty cursor → start from a
	// far-future timestamp + max-sorting id so the keyset query
	// returns the first page naturally.
	cursorAt, cursorID, err := decodeListCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid cursor: "+err.Error())
		return
	}

	limit := parseLimitParam(r.URL.Query().Get("limit"), 25, 100)

	statusCSV := r.URL.Query().Get("status")
	from := parseRFC3339Param(r.URL.Query().Get("from"))
	to := parseRFC3339Param(r.URL.Query().Get("to"))

	posts, err := h.queries.ListSocialPostsFiltered(r.Context(), db.ListSocialPostsFilteredParams{
		WorkspaceID: workspaceID,
		Column2:     statusCSV,
		Column3:     from,
		Column4:     to,
		Column5:     pgtype.Timestamptz{Time: cursorAt, Valid: true},
		Column6:     cursorID,
		Limit:       int32(limit),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list posts")
		return
	}

	// Pre-load ALL social accounts (including disconnected) for this workspace
	// to resolve platform names AND display names. Historical post results
	// may reference accounts that have since been disconnected — we still
	// want their platform / handle to show up in the analytics list.
	allAccounts, _ := h.queries.ListSocialAccountsByWorkspace(r.Context(), workspaceID)
	accountMap := make(map[string]accountSummary, len(allAccounts))
	for _, acc := range allAccounts {
		name := ""
		if acc.AccountName.Valid {
			name = acc.AccountName.String
		}
		accountMap[acc.ID] = accountSummary{Platform: acc.Platform, Name: name}
	}
	postIDs := make([]string, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	jobRows, _ := h.queries.ListPostDeliveryJobsByPostIDs(r.Context(), postIDs)
	jobsByPost := make(map[string][]db.PostDeliveryJob, len(postIDs))
	for _, job := range jobRows {
		jobsByPost[job.PostID] = append(jobsByPost[job.PostID], job)
	}

	var result []socialPostResponse
	for _, p := range posts {
		var caption *string
		if p.Caption.Valid {
			caption = &p.Caption.String
		}
		var scheduledAt *time.Time
		if p.ScheduledAt.Valid {
			scheduledAt = &p.ScheduledAt.Time
		}
		var publishedAt *time.Time
		if p.PublishedAt.Valid {
			publishedAt = &p.PublishedAt.Time
		}
		var archivedAt *time.Time
		if p.ArchivedAt.Valid {
			archivedAt = &p.ArchivedAt.Time
		}

		// Fetch results for this post
		postResults, _ := h.queries.ListSocialPostResultsByPost(r.Context(), p.ID)
		submittedByAccount := buildSubmittedMap(p.Metadata, derefText(p.Caption))
		var responseResults []postResultResponse
		for _, res := range postResults {
			summary := accountMap[res.SocialAccountID]
			rr := postResultResponse{
				ID:              res.ID,
				SocialAccountID: res.SocialAccountID,
				Platform:        summary.Platform,
				AccountName:     summary.Name,
				Status:          res.Status,
			}
			if res.ExternalID.Valid {
				rr.ExternalID = &res.ExternalID.String
			}
			if res.Url.Valid {
				rr.URL = &res.Url.String
			}
			if res.ErrorMessage.Valid {
				rr.ErrorMessage = &res.ErrorMessage.String
			}
			if res.PublishedAt.Valid {
				t := res.PublishedAt.Time.Format(time.RFC3339)
				rr.PublishedAt = &t
			}
			if res.DebugCurl.Valid {
				rr.DebugCurl = &res.DebugCurl.String
			}
			if sub := submittedByAccount[res.SocialAccountID]; sub != nil {
				rr.Submitted = sub
			}
			if res.ExternalID.Valid && summary.Platform == "tiktok" {
				if acc, accErr := h.queries.GetSocialAccount(r.Context(), res.SocialAccountID); accErr == nil {
					accessToken, decErr := h.encryptor.Decrypt(acc.AccessToken)
					if decErr == nil {
						if adapter, adErr := platform.Get("tiktok"); adErr == nil {
							if tiktokAdapter, ok := adapter.(*platform.TikTokAdapter); ok {
								if status, stErr := tiktokAdapter.CheckPublishStatus(r.Context(), accessToken, res.ExternalID.String); stErr == nil {
									applyTikTokLiveStatus(r.Context(), &rr, status, tiktokAdapter, accessToken)
								}
							}
						}
					}
				}
			}
			responseResults = append(responseResults, rr)
		}

		var targetPlatforms []string
		if platformPosts, decErr := platform.DecodePostMetadata(p.Metadata, derefText(p.Caption)); decErr == nil {
			seenPlatforms := make(map[string]struct{}, len(platformPosts))
			for _, pp := range platformPosts {
				if summary, ok := accountMap[pp.AccountID]; ok && summary.Platform != "" {
					if _, seen := seenPlatforms[summary.Platform]; seen {
						continue
					}
					seenPlatforms[summary.Platform] = struct{}{}
					targetPlatforms = append(targetPlatforms, summary.Platform)
				}
			}
		}

		resp := socialPostResponse{
			ID:              p.ID,
			Caption:         caption,
			MediaURLs:       p.MediaUrls,
			Status:          deriveSocialPostStatus(p, postResults, jobsByPost[p.ID]),
			CreatedAt:       p.CreatedAt.Time,
			ScheduledAt:     scheduledAt,
			PublishedAt:     publishedAt,
			ArchivedAt:      archivedAt,
			Source:          p.Source,
			ProfileIDs:      p.ProfileIds,
			TargetPlatforms: targetPlatforms,
			Results:         responseResults,
		}
		applyQueueSummary(&resp, jobsByPost[p.ID])
		result = append(result, resp)
	}
	if result == nil {
		result = []socialPostResponse{}
	}

	// Build next_cursor from the last row when we returned a full
	// page. If we got fewer rows than the limit, this is the last
	// page → next_cursor is empty.
	var nextCursor string
	if len(posts) == limit && len(posts) > 0 {
		last := posts[len(posts)-1]
		nextCursor = encodeListCursor(last.CreatedAt.Time, last.ID)
	}

	// Cursor pagination uses a flat envelope ({data, next_cursor}) rather
	// than the nested writeSuccess path. Calling writeSuccess here would
	// double-wrap as {data: {data: [...], next_cursor: ""}}, which the
	// dashboard and the smoke test both reject because they read .data
	// and .next_cursor at the top level.
	writeJSON(w, http.StatusOK, map[string]any{
		"data":        result,
		"next_cursor": nextCursor,
	})
}

func derefText(v pgtype.Text) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// parseLimitParam parses ?limit=N with a default + ceiling.
func parseLimitParam(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// parseRFC3339Param returns a pgtype.Timestamptz from an RFC3339
// query param. Empty / invalid input → invalid timestamp (which the
// SQL query treats as "no filter").
func parseRFC3339Param(raw string) pgtype.Timestamptz {
	if raw == "" {
		return pgtype.Timestamptz{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// encodeListCursor packs a (created_at, id) tuple into an opaque
// base64url string for the next_cursor response field. Format is
// "<unix_nanos>|<id>" so it round-trips losslessly.
func encodeListCursor(t time.Time, id string) string {
	raw := strconv.FormatInt(t.UnixNano(), 10) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeListCursor is the inverse. Empty cursor → far-future
// timestamp + max-sorting id sentinel so the keyset query naturally
// returns the first page (every real row sorts before "max id").
func decodeListCursor(raw string) (time.Time, string, error) {
	if raw == "" {
		// Sentinel: end of "max" string is a tilde so it sorts after
		// any printable ASCII (the IDs we generate are uuid hex which
		// always sorts before tilde).
		return time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), "~", nil
	}
	bytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("not base64url")
	}
	parts := strings.SplitN(string(bytes), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("malformed")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("bad timestamp")
	}
	return time.Unix(0, nanos), parts[1], nil
}

// Archive handles POST /v1/social-posts/{id}/archive.
func (h *SocialPostHandler) Archive(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	post, err := h.queries.ArchiveSocialPost(r.Context(), db.ArchiveSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to archive post")
		return
	}

	writeSuccess(w, socialPostResponseFromRow(post))
}

// Restore handles POST /v1/social-posts/{id}/restore.
func (h *SocialPostHandler) Restore(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	post, err := h.queries.RestoreSocialPost(r.Context(), db.RestoreSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to restore post")
		return
	}

	writeSuccess(w, socialPostResponseFromRow(post))
}

// Delete handles DELETE /v1/social-posts/{id}
func (h *SocialPostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	_, err := h.queries.SoftDeleteSocialPost(r.Context(), db.SoftDeleteSocialPostParams{
		ID:          postID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete post")
		return
	}

	writeSuccess(w, map[string]bool{"deleted": true})
}

// getWorkspaceID extracts workspace ID from API key context or URL param (dashboard routes).
func (h *SocialPostHandler) getWorkspaceID(r *http.Request) string {
	if pid := auth.GetWorkspaceID(r.Context()); pid != "" {
		return pid
	}
	workspaceID := chi.URLParam(r, "workspaceID")
	if workspaceID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID:     workspaceID,
		UserID: userID,
	})
	if err != nil {
		return ""
	}
	return workspaceID
}
