package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
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
}

func NewSocialPostHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, quotaChecker *quota.Checker, bus events.EventBus) *SocialPostHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &SocialPostHandler{queries: queries, encryptor: encryptor, quota: quotaChecker, bus: bus}
}

type postResultResponse struct {
	SocialAccountID string         `json:"social_account_id"`
	Platform        string         `json:"platform,omitempty"`
	AccountName     string         `json:"account_name,omitempty"`
	Caption         string         `json:"caption,omitempty"`
	Status          string         `json:"status"`
	ExternalID      *string        `json:"external_id,omitempty"`
	ErrorMessage    *string        `json:"error_message,omitempty"`
	PublishedAt     *string        `json:"published_at,omitempty"`
	PublishStatus   map[string]any `json:"publish_status,omitempty"`
}

// accountSummary is what the List handler stores per social_account_id so the
// per-result rows can resolve both the platform name and the human-readable
// display name in a single map lookup.
type accountSummary struct {
	Platform string
	Name     string
}

type socialPostResponse struct {
	ID          string               `json:"id"`
	Caption     *string              `json:"caption"`
	Status      string               `json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
	ScheduledAt *time.Time           `json:"scheduled_at,omitempty"`
	PublishedAt *time.Time           `json:"published_at,omitempty"`
	Results     []postResultResponse `json:"results,omitempty"`
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
// not in project) are recorded as failed per-account results so the
// overall publish doesn't get blocked by one bad account — preserves
// legacy soft-failure semantics.
func (h *SocialPostHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
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
			ProjectID:      projectID,
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
	quotaStatus := h.quota.Check(r.Context(), projectID)
	w.Header().Set("X-UniPost-Usage", fmt.Sprintf("%d/%d", quotaStatus.Usage, quotaStatus.Limit))
	if quotaStatus.Warning != "" {
		w.Header().Set("X-UniPost-Warning", quotaStatus.Warning)
	}

	// Load accounts once so the validator and the publish loop both
	// see the same view of which accounts the project owns.
	accountMap, err := h.loadValidateAccounts(r, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load accounts")
		return
	}

	// Run the same validator /social-posts/validate uses, then filter
	// out non-fatal issues (account_disconnected, account_not_in_project)
	// — those are still recorded as failed results below to preserve
	// legacy soft-failure semantics.
	vr := platform.ValidatePlatformPosts(platform.ValidateOptions{
		Capabilities: platform.Capabilities,
		Accounts:     accountMap,
		Posts:        parsed.Posts,
		ScheduledAt:  parsed.ScheduledAt,
	})
	if fatal := filterFatalIssues(vr.Errors); len(fatal) > 0 {
		writeValidationErrors(w, fatal)
		return
	}

	// Branch on scheduled vs immediate.
	if parsed.ScheduledAt != nil {
		h.createScheduledPost(w, r, projectID, parsed)
		return
	}
	h.createImmediatePost(w, r, projectID, parsed, accountMap)
}

// createScheduledPost persists the post with status="scheduled" and
// the v2 metadata blob so the scheduler can later fan out per-account
// when the time arrives. Returns 200 with a minimal response (no
// results yet — they'll exist after the scheduler fires).
func (h *SocialPostHandler) createScheduledPost(w http.ResponseWriter, r *http.Request, projectID string, parsed parsedRequest) {
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
		ProjectID:      projectID,
		Caption:        canonicalCaption,
		MediaUrls:      canonicalMedia,
		Status:         "scheduled",
		Metadata:       metaJSON,
		ScheduledAt:    pgtype.Timestamptz{Time: *parsed.ScheduledAt, Valid: true},
		IdempotencyKey: idempotencyKeyParam(parsed.IdempotencyKey),
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
		Status:      "scheduled",
		CreatedAt:   post.CreatedAt.Time,
		ScheduledAt: &scheduledAt,
	})
}

// createImmediatePost is the synchronous publish path. Walks each
// PlatformPostInput, dispatches to its account's adapter with the
// per-post caption / media / options, persists results with the
// per-result caption populated, and returns the full response.
func (h *SocialPostHandler) createImmediatePost(
	w http.ResponseWriter,
	r *http.Request,
	projectID string,
	parsed parsedRequest,
	accountMap map[string]platform.ValidateAccount,
) {
	// We still need the FULL db.SocialAccount for each unique account
	// (token, refresh, etc.) — accountMap only has platform name +
	// disconnected flag. Look up each unique account_id once.
	uniqueIDs := uniqueAccountIDs(parsed.Posts)
	dbAccounts := make(map[string]db.SocialAccount, len(uniqueIDs))
	for _, id := range uniqueIDs {
		acc, err := h.queries.GetSocialAccountByIDAndProject(r.Context(), db.GetSocialAccountByIDAndProjectParams{
			ID:        id,
			ProjectID: projectID,
		})
		if err != nil {
			// Missing accounts are reported as failed results below;
			// continue so we don't fail the whole request.
			continue
		}
		dbAccounts[id] = acc
	}

	// Persist the parent post FIRST so per-result rows can FK to it.
	metaJSON, _ := platform.EncodePostMetadata(parsed.Posts)
	canonicalCaption := pgtype.Text{}
	if len(parsed.Posts) > 0 && parsed.Posts[0].Caption != "" {
		canonicalCaption = pgtype.Text{String: parsed.Posts[0].Caption, Valid: true}
	}
	canonicalMedia := []string{}
	if len(parsed.Posts) > 0 && parsed.Posts[0].MediaURLs != nil {
		canonicalMedia = parsed.Posts[0].MediaURLs
	}

	post, err := h.queries.CreateSocialPost(r.Context(), db.CreateSocialPostParams{
		ProjectID:      projectID,
		Caption:        canonicalCaption,
		MediaUrls:      canonicalMedia,
		Status:         "publishing",
		Metadata:       metaJSON,
		ScheduledAt:    pgtype.Timestamptz{},
		IdempotencyKey: idempotencyKeyParam(parsed.IdempotencyKey),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create post")
		return
	}

	// Publish each platform post concurrently. Order is preserved by
	// using a fixed-size slice indexed by input position.
	outcomes := make([]publishOneOutcome, len(parsed.Posts))
	var wg sync.WaitGroup

	for i, pp := range parsed.Posts {
		wg.Add(1)
		go func(idx int, pp platform.PlatformPostInput) {
			defer wg.Done()
			outcomes[idx] = h.publishOne(r, pp, dbAccounts, accountMap)
		}(i, pp)
	}
	wg.Wait()

	// Persist results + build response in input order.
	var responseResults []postResultResponse
	allPublished := true
	anyPublished := false
	publishedCount := 0

	for i, oc := range outcomes {
		var extID, errMsg pgtype.Text
		var pubAt pgtype.Timestamptz
		status := "published"

		if oc.err != nil {
			status = "failed"
			errMsg = pgtype.Text{String: oc.err.Error(), Valid: true}
			allPublished = false
		} else if oc.result != nil {
			extID = pgtype.Text{String: oc.result.ExternalID, Valid: true}
			pubAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			anyPublished = true
			publishedCount++
		}

		dbResult, dbErr := h.queries.CreateSocialPostResult(r.Context(), db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: parsed.Posts[i].AccountID,
			Caption:         parsed.Posts[i].Caption,
			Status:          status,
			ExternalID:      extID,
			ErrorMessage:    errMsg,
			PublishedAt:     pubAt,
		})
		if dbErr != nil {
			slog.Error("failed to save post result", "error", dbErr)
			continue
		}

		rr := postResultResponse{
			SocialAccountID: dbResult.SocialAccountID,
			Platform:        oc.platform,
			AccountName:     oc.accountName,
			Caption:         dbResult.Caption,
			Status:          dbResult.Status,
		}
		if dbResult.ExternalID.Valid {
			rr.ExternalID = &dbResult.ExternalID.String
		}
		if dbResult.ErrorMessage.Valid {
			rr.ErrorMessage = &dbResult.ErrorMessage.String
		}
		if dbResult.PublishedAt.Valid {
			t := dbResult.PublishedAt.Time.Format(time.RFC3339)
			rr.PublishedAt = &t
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

	if publishedCount > 0 {
		h.quota.Increment(r.Context(), projectID, publishedCount)
	}

	var caption *string
	if post.Caption.Valid {
		caption = &post.Caption.String
	}
	resp := socialPostResponse{
		ID:        post.ID,
		Caption:   caption,
		Status:    postStatus,
		CreatedAt: post.CreatedAt.Time,
		Results:   responseResults,
	}

	// Fan out webhook events. Best-effort — Publish recovers panics
	// internally and never blocks the response. The event payload is
	// the same socialPostResponse the caller just got back, so
	// subscribers can correlate by post ID.
	h.bus.Publish(r.Context(), projectID, eventForStatus(postStatus), resp)

	writeSuccess(w, resp)
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

	// Inline token refresh if expired.
	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
		if refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String); decErr == nil {
			if newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(r.Context(), refreshTok); refErr == nil {
				accessToken = newAccess
				encAccess, _ := h.encryptor.Encrypt(newAccess)
				encRefresh, _ := h.encryptor.Encrypt(newRefresh)
				h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
					ID:             acc.ID,
					AccessToken:    encAccess,
					RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
					TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
			}
		}
	}

	// Per-platform routing log — emitted at INFO so smoke-tests can
	// verify each PlatformPostInput is reaching the right adapter
	// with the right caption. Mirrors the same line in scheduler.go
	// so the immediate and scheduled paths produce comparable output.
	slog.Info("publish: dispatching to adapter",
		"account_id", acc.ID,
		"platform", acc.Platform,
		"caption_preview", truncateForLog(pp.Caption, 40))

	postResult, err := adapter.Post(
		r.Context(),
		accessToken,
		pp.Caption,
		platform.MediaFromURLs(pp.MediaURLs),
		pp.PlatformOptions,
	)
	oc.result = postResult
	oc.err = err
	return
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
type publishOneOutcome struct {
	platform    string
	accountName string
	result      *platform.PostResult
	err         error
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

// writeReplayedPost rebuilds a socialPostResponse from a previously-
// stored post (looked up by idempotency_key) and returns it as if it
// were the original publish response. No new platform posts are made.
func (h *SocialPostHandler) writeReplayedPost(w http.ResponseWriter, r *http.Request, post db.SocialPost) {
	results, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)

	// Resolve platform + account name once per result via the project
	// account map (cheaper than per-result GetSocialAccount calls).
	allAccounts, _ := h.queries.ListAllSocialAccountsByProject(r.Context(), post.ProjectID)
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
		ID:        post.ID,
		Status:    post.Status,
		CreatedAt: post.CreatedAt.Time,
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

	for _, res := range results {
		info := accountInfo[res.SocialAccountID]
		rr := postResultResponse{
			SocialAccountID: res.SocialAccountID,
			Platform:        info.Platform,
			AccountName:     info.Name,
			Caption:         res.Caption,
			Status:          res.Status,
		}
		if res.ExternalID.Valid {
			rr.ExternalID = &res.ExternalID.String
		}
		if res.ErrorMessage.Valid {
			rr.ErrorMessage = &res.ErrorMessage.String
		}
		if res.PublishedAt.Valid {
			t := res.PublishedAt.Time.Format(time.RFC3339)
			rr.PublishedAt = &t
		}
		resp.Results = append(resp.Results, rr)
	}

	// Stamp the replay so callers can tell from the headers.
	w.Header().Set("Idempotent-Replay", "true")
	writeSuccess(w, resp)
}

// Get handles GET /v1/social-posts/{id}
func (h *SocialPostHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	post, err := h.queries.GetSocialPostByIDAndProject(r.Context(), db.GetSocialPostByIDAndProjectParams{
		ID:        postID,
		ProjectID: projectID,
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
	var responseResults []postResultResponse
	for _, res := range results {
		rr := postResultResponse{
			SocialAccountID: res.SocialAccountID,
			Status:          res.Status,
		}
		if res.ExternalID.Valid {
			rr.ExternalID = &res.ExternalID.String
		}
		if res.ErrorMessage.Valid {
			rr.ErrorMessage = &res.ErrorMessage.String
		}
		if res.PublishedAt.Valid {
			t := res.PublishedAt.Time.Format(time.RFC3339)
			rr.PublishedAt = &t
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
							rr.PublishStatus = status
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

	writeSuccess(w, socialPostResponse{
		ID:          post.ID,
		Caption:     caption,
		Status:      post.Status,
		CreatedAt:   post.CreatedAt.Time,
		PublishedAt: publishedAt,
		Results:     responseResults,
	})
}

// List handles GET /v1/social-posts
func (h *SocialPostHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	posts, err := h.queries.ListSocialPostsByProject(r.Context(), db.ListSocialPostsByProjectParams{
		ProjectID: projectID,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list posts")
		return
	}

	// Pre-load ALL social accounts (including disconnected) for this project
	// to resolve platform names AND display names. Historical post results
	// may reference accounts that have since been disconnected — we still
	// want their platform / handle to show up in the analytics list.
	allAccounts, _ := h.queries.ListAllSocialAccountsByProject(r.Context(), projectID)
	accountMap := make(map[string]accountSummary, len(allAccounts))
	for _, acc := range allAccounts {
		name := ""
		if acc.AccountName.Valid {
			name = acc.AccountName.String
		}
		accountMap[acc.ID] = accountSummary{Platform: acc.Platform, Name: name}
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

		// Fetch results for this post
		postResults, _ := h.queries.ListSocialPostResultsByPost(r.Context(), p.ID)
		var responseResults []postResultResponse
		for _, res := range postResults {
			summary := accountMap[res.SocialAccountID]
			rr := postResultResponse{
				SocialAccountID: res.SocialAccountID,
				Platform:        summary.Platform,
				AccountName:     summary.Name,
				Status:          res.Status,
			}
			if res.ExternalID.Valid {
				rr.ExternalID = &res.ExternalID.String
			}
			if res.ErrorMessage.Valid {
				rr.ErrorMessage = &res.ErrorMessage.String
			}
			if res.PublishedAt.Valid {
				t := res.PublishedAt.Time.Format(time.RFC3339)
				rr.PublishedAt = &t
			}
			responseResults = append(responseResults, rr)
		}

		result = append(result, socialPostResponse{
			ID:          p.ID,
			Caption:     caption,
			Status:      p.Status,
			CreatedAt:   p.CreatedAt.Time,
			ScheduledAt: scheduledAt,
			PublishedAt: publishedAt,
			Results:     responseResults,
		})
	}
	if result == nil {
		result = []socialPostResponse{}
	}

	writeSuccessWithMeta(w, result, len(result))
}

// Delete handles DELETE /v1/social-posts/{id}
func (h *SocialPostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	post, err := h.queries.GetSocialPostByIDAndProject(r.Context(), db.GetSocialPostByIDAndProjectParams{
		ID:        postID,
		ProjectID: projectID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get post")
		return
	}

	// Delete from platforms
	results, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)
	for _, res := range results {
		if !res.ExternalID.Valid {
			continue
		}
		acc, err := h.queries.GetSocialAccount(r.Context(), res.SocialAccountID)
		if err != nil {
			continue
		}
		adapter, err := platform.Get(acc.Platform)
		if err != nil {
			continue
		}
		accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
		if err != nil {
			continue
		}
		if err := adapter.DeletePost(r.Context(), accessToken, res.ExternalID.String); err != nil {
			slog.Error("failed to delete post from platform", "platform", acc.Platform, "error", err)
		}
	}

	h.queries.DeleteSocialPostResultsByPost(r.Context(), post.ID)
	h.queries.DeleteSocialPost(r.Context(), post.ID)

	writeSuccess(w, map[string]bool{"deleted": true})
}

// getProjectID extracts project ID from API key context or URL param (dashboard routes).
func (h *SocialPostHandler) getProjectID(r *http.Request) string {
	if pid := auth.GetProjectID(r.Context()); pid != "" {
		return pid
	}
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID:      projectID,
		OwnerID: userID,
	})
	if err != nil {
		return ""
	}
	return projectID
}
