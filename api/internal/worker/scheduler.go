package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// SchedulerWorker publishes scheduled posts when their scheduled_at time arrives.
type SchedulerWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewSchedulerWorker(queries *db.Queries, encryptor *crypto.AESEncryptor) *SchedulerWorker {
	return &SchedulerWorker{queries: queries, encryptor: encryptor}
}

func (w *SchedulerWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	slog.Info("scheduler worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler worker stopped")
			return
		case <-ticker.C:
			w.publishDue(ctx)
		}
	}
}

func (w *SchedulerWorker) publishDue(ctx context.Context) {
	posts, err := w.queries.GetDueScheduledPosts(ctx)
	if err != nil {
		slog.Error("scheduler: failed to get due posts", "error", err)
		return
	}

	if len(posts) == 0 {
		return
	}

	slog.Info("scheduler: found due posts", "count", len(posts))

	for _, post := range posts {
		// Claim the post (optimistic lock)
		claimed, err := w.queries.ClaimScheduledPost(ctx, post.ID)
		if err != nil {
			continue // Another instance already claimed it
		}

		go w.publishPost(ctx, claimed)
	}
}

func (w *SchedulerWorker) publishPost(ctx context.Context, post db.SocialPost) {
	slog.Info("scheduler: publishing post", "post_id", post.ID)

	// Get accounts and per-platform options for this post from metadata.
	// Both are stored when the post was created.
	var accountIDs []string
	var platformOptions map[string]map[string]any
	if post.Metadata != nil {
		var meta struct {
			AccountIDs      []string                    `json:"account_ids"`
			PlatformOptions map[string]map[string]any   `json:"platform_options"`
		}
		if err := json.Unmarshal(post.Metadata, &meta); err == nil {
			accountIDs = meta.AccountIDs
			platformOptions = meta.PlatformOptions
		}
	}

	if len(accountIDs) == 0 {
		slog.Error("scheduler: no account IDs in post metadata", "post_id", post.ID)
		w.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
			ID: post.ID, Status: "failed",
		})
		return
	}

	// Publish to each account
	type result struct {
		accountID string
		platform  string
		postResult *platform.PostResult
		err       error
	}

	var results []result
	var wg sync.WaitGroup

	for _, accID := range accountIDs {
		wg.Add(1)
		go func(accountID string) {
			defer wg.Done()

			acc, err := w.queries.GetSocialAccount(ctx, accountID)
			if err != nil {
				results = append(results, result{accountID: accountID, err: err})
				return
			}

			adapter, err := platform.Get(acc.Platform)
			if err != nil {
				results = append(results, result{accountID: accountID, platform: acc.Platform, err: err})
				return
			}

			accessToken, err := w.encryptor.Decrypt(acc.AccessToken)
			if err != nil {
				results = append(results, result{accountID: accountID, platform: acc.Platform, err: err})
				return
			}

			// Refresh token if expired
			if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
				refreshToken, decErr := w.encryptor.Decrypt(acc.RefreshToken.String)
				if decErr == nil {
					newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(ctx, refreshToken)
					if refErr == nil {
						accessToken = newAccess
						encAccess, _ := w.encryptor.Encrypt(newAccess)
						encRefresh, _ := w.encryptor.Encrypt(newRefresh)
						w.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
							ID: acc.ID, AccessToken: encAccess,
							RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
							TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
						})
					}
				}
			}

			caption := ""
			if post.Caption.Valid {
				caption = post.Caption.String
			}

			pr, err := adapter.Post(ctx, accessToken, caption, post.MediaUrls, platformOptions[acc.Platform])
			results = append(results, result{accountID: accountID, platform: acc.Platform, postResult: pr, err: err})
		}(accID)
	}
	wg.Wait()

	// Store results
	allPublished := true
	anyPublished := false

	for _, res := range results {
		status := "published"
		var extID, errMsg pgtype.Text
		var pubAt pgtype.Timestamptz

		if res.err != nil {
			status = "failed"
			errMsg = pgtype.Text{String: res.err.Error(), Valid: true}
			allPublished = false
		} else {
			extID = pgtype.Text{String: res.postResult.ExternalID, Valid: true}
			pubAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			anyPublished = true
		}

		w.queries.CreateSocialPostResult(ctx, db.CreateSocialPostResultParams{
			PostID: post.ID, SocialAccountID: res.accountID,
			Status: status, ExternalID: extID, ErrorMessage: errMsg, PublishedAt: pubAt,
		})
	}

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
	w.queries.UpdateSocialPostStatus(ctx, db.UpdateSocialPostStatusParams{
		ID: post.ID, Status: postStatus, PublishedAt: publishedAt,
	})

	slog.Info("scheduler: post published", "post_id", post.ID, "status", postStatus)
}

