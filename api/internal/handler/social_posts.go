package handler

import (
	"encoding/json"
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
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type SocialPostHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewSocialPostHandler(queries *db.Queries, encryptor *crypto.AESEncryptor) *SocialPostHandler {
	return &SocialPostHandler{queries: queries, encryptor: encryptor}
}

type postResultResponse struct {
	SocialAccountID string  `json:"social_account_id"`
	Platform        string  `json:"platform,omitempty"`
	Status          string  `json:"status"`
	ExternalID      *string `json:"external_id,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	PublishedAt     *string `json:"published_at,omitempty"`
}

type socialPostResponse struct {
	ID          string               `json:"id"`
	Caption     *string              `json:"caption"`
	Status      string               `json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
	PublishedAt *time.Time           `json:"published_at,omitempty"`
	Results     []postResultResponse `json:"results,omitempty"`
}

// Create handles POST /v1/social-posts
func (h *SocialPostHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	var body struct {
		Caption    string   `json:"caption"`
		MediaURLs  []string `json:"media_urls"`
		AccountIDs []string `json:"account_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Caption == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Caption is required")
		return
	}
	if len(body.AccountIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "At least one account_id is required")
		return
	}

	// Validate accounts belong to project — disconnected accounts are included
	// but will be marked as failed in results (one failure doesn't block others)
	type accountEntry struct {
		account      db.SocialAccount
		disconnected bool
		notFound     bool
	}
	var entries []accountEntry
	for _, id := range body.AccountIDs {
		acc, err := h.queries.GetSocialAccountByIDAndProject(r.Context(), db.GetSocialAccountByIDAndProjectParams{
			ID:        id,
			ProjectID: projectID,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				entries = append(entries, accountEntry{account: db.SocialAccount{ID: id}, notFound: true})
				continue
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to validate account")
			return
		}
		entries = append(entries, accountEntry{
			account:      acc,
			disconnected: acc.DisconnectedAt.Valid,
		})
	}

	var accounts []db.SocialAccount
	for _, e := range entries {
		if !e.notFound && !e.disconnected {
			accounts = append(accounts, e.account)
		}
	}

	// Create post record
	mediaURLs := body.MediaURLs
	if mediaURLs == nil {
		mediaURLs = []string{}
	}

	post, err := h.queries.CreateSocialPost(r.Context(), db.CreateSocialPostParams{
		ProjectID: projectID,
		Caption:   pgtype.Text{String: body.Caption, Valid: true},
		MediaUrls: mediaURLs,
		Status:    "publishing",
		Metadata:  nil,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create post")
		return
	}

	// Publish to each account concurrently
	type accountResult struct {
		accountID string
		platform  string
		result    *platform.PostResult
		err       error
	}

	results := make([]accountResult, len(accounts))
	var wg sync.WaitGroup

	for i, acc := range accounts {
		wg.Add(1)
		go func(idx int, account db.SocialAccount) {
			defer wg.Done()

			adapter, err := platform.Get(account.Platform)
			if err != nil {
				results[idx] = accountResult{accountID: account.ID, platform: account.Platform, err: err}
				return
			}

			accessToken, err := h.encryptor.Decrypt(account.AccessToken)
			if err != nil {
				results[idx] = accountResult{accountID: account.ID, platform: account.Platform, err: err}
				return
			}

			postResult, err := adapter.Post(r.Context(), accessToken, body.Caption, body.MediaURLs)
			results[idx] = accountResult{
				accountID: account.ID,
				platform:  account.Platform,
				result:    postResult,
				err:       err,
			}
		}(i, acc)
	}
	wg.Wait()

	// Store results
	var responseResults []postResultResponse
	allPublished := true
	anyPublished := false

	for _, res := range results {
		var extID, errMsg pgtype.Text
		var pubAt pgtype.Timestamptz
		status := "published"

		if res.err != nil {
			status = "failed"
			errMsg = pgtype.Text{String: res.err.Error(), Valid: true}
			allPublished = false
		} else {
			extID = pgtype.Text{String: res.result.ExternalID, Valid: true}
			pubAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			anyPublished = true
		}

		dbResult, err := h.queries.CreateSocialPostResult(r.Context(), db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: res.accountID,
			Status:          status,
			ExternalID:      extID,
			ErrorMessage:    errMsg,
			PublishedAt:     pubAt,
		})
		if err != nil {
			slog.Error("failed to save post result", "error", err)
			continue
		}

		rr := postResultResponse{
			SocialAccountID: dbResult.SocialAccountID,
			Platform:        res.platform,
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

	// Record failed results for disconnected/not-found accounts
	for _, e := range entries {
		if !e.notFound && !e.disconnected {
			continue
		}
		allPublished = false
		errMessage := "account is disconnected"
		if e.notFound {
			errMessage = "account not found"
		}
		dbResult, err := h.queries.CreateSocialPostResult(r.Context(), db.CreateSocialPostResultParams{
			PostID:          post.ID,
			SocialAccountID: e.account.ID,
			Status:          "failed",
			ExternalID:      pgtype.Text{},
			ErrorMessage:    pgtype.Text{String: errMessage, Valid: true},
			PublishedAt:     pgtype.Timestamptz{},
		})
		if err != nil {
			slog.Error("failed to save post result", "error", err)
			continue
		}
		msg := dbResult.ErrorMessage.String
		responseResults = append(responseResults, postResultResponse{
			SocialAccountID: dbResult.SocialAccountID,
			Platform:        e.account.Platform,
			Status:          "failed",
			ErrorMessage:    &msg,
		})
	}

	// Update post status
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

	var caption *string
	if post.Caption.Valid {
		caption = &post.Caption.String
	}

	writeSuccess(w, socialPostResponse{
		ID:        post.ID,
		Caption:   caption,
		Status:    postStatus,
		CreatedAt: post.CreatedAt.Time,
		Results:   responseResults,
	})
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
		Limit:     20,
		Offset:    0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list posts")
		return
	}

	var result []socialPostResponse
	for _, p := range posts {
		var caption *string
		if p.Caption.Valid {
			caption = &p.Caption.String
		}
		result = append(result, socialPostResponse{
			ID:        p.ID,
			Caption:   caption,
			Status:    p.Status,
			CreatedAt: p.CreatedAt.Time,
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
