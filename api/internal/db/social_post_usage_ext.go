package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const updateSocialPostResultAfterRetryAndIncrementUsageSQL = `
WITH updated_result AS (
  UPDATE social_post_results
  SET
    status = $2,
    external_id = $3,
    error_message = $4,
    published_at = $5,
    url = $6,
    debug_curl = $7,
    error_code = NULL,
    failure_stage = NULL,
    platform_error_code = NULL,
    is_retriable = NULL,
    next_action = NULL,
    error_source = NULL,
    error_temporality = NULL,
    provider_error = NULL
  WHERE id = $1
    AND status <> 'published'
  RETURNING id, post_id, social_account_id, status, external_id, error_message,
    published_at, caption, url, debug_curl, fb_media_type, remotely_deleted_at,
    error_code, failure_stage, platform_error_code, is_retriable, next_action,
    error_source, error_temporality, provider_error, publish_token
),
usage_increment AS (
  INSERT INTO usage (workspace_id, period, post_count)
  SELECT $8, $9, $10
  FROM updated_result
  ON CONFLICT (workspace_id, period)
  DO UPDATE SET
    post_count = usage.post_count + EXCLUDED.post_count,
    updated_at = NOW()
  RETURNING workspace_id
)
SELECT
  r.id, r.post_id, r.social_account_id, r.status, r.external_id,
  r.error_message, r.published_at, r.caption, r.url, r.debug_curl,
  r.fb_media_type, r.remotely_deleted_at, r.error_code, r.failure_stage,
  r.platform_error_code, r.is_retriable, r.next_action, r.error_source,
  r.error_temporality, r.provider_error, r.publish_token
FROM updated_result r
CROSS JOIN usage_increment
`

type UpdateSocialPostResultAfterRetryAndIncrementUsageParams struct {
	ID           string
	Status       string
	ExternalID   pgtype.Text
	ErrorMessage pgtype.Text
	PublishedAt  pgtype.Timestamptz
	Url          pgtype.Text
	DebugCurl    pgtype.Text
	WorkspaceID  string
	Period       string
	PostCount    int32
}

// UpdateSocialPostResultAfterRetryAndIncrementUsage exchanges one outstanding
// scheduled reservation for completed usage atomically. This prevents quota
// admission from observing a result as published before its usage is counted.
func (q *Queries) UpdateSocialPostResultAfterRetryAndIncrementUsage(
	ctx context.Context,
	arg UpdateSocialPostResultAfterRetryAndIncrementUsageParams,
) (SocialPostResult, error) {
	row := q.db.QueryRow(
		ctx,
		updateSocialPostResultAfterRetryAndIncrementUsageSQL,
		arg.ID,
		arg.Status,
		arg.ExternalID,
		arg.ErrorMessage,
		arg.PublishedAt,
		arg.Url,
		arg.DebugCurl,
		arg.WorkspaceID,
		arg.Period,
		arg.PostCount,
	)
	var result SocialPostResult
	err := row.Scan(
		&result.ID,
		&result.PostID,
		&result.SocialAccountID,
		&result.Status,
		&result.ExternalID,
		&result.ErrorMessage,
		&result.PublishedAt,
		&result.Caption,
		&result.Url,
		&result.DebugCurl,
		&result.FbMediaType,
		&result.RemotelyDeletedAt,
		&result.ErrorCode,
		&result.FailureStage,
		&result.PlatformErrorCode,
		&result.IsRetriable,
		&result.NextAction,
		&result.ErrorSource,
		&result.ErrorTemporality,
		&result.ProviderError,
		&result.PublishToken,
	)
	return result, err
}

const updateSocialPostStatusAndIncrementUsageSQL = `
WITH updated_post AS (
  UPDATE social_posts
  SET status = $2,
      published_at = $3
  WHERE id = $1
    AND status = 'publishing'
  RETURNING id
),
usage_increment AS (
  INSERT INTO usage (workspace_id, period, post_count)
  SELECT $4, $5, $6
  FROM updated_post
  ON CONFLICT (workspace_id, period)
  DO UPDATE SET
    post_count = usage.post_count + EXCLUDED.post_count,
    updated_at = NOW()
  RETURNING workspace_id
)
SELECT 1
FROM updated_post
CROSS JOIN usage_increment
`

type UpdateSocialPostStatusAndIncrementUsageParams struct {
	ID          string
	Status      string
	PublishedAt pgtype.Timestamptz
	WorkspaceID string
	Period      string
	PostCount   int32
}

// UpdateSocialPostStatusAndIncrementUsage atomically replaces the parent
// scheduled reservation with completed usage for synchronous publishing.
func (q *Queries) UpdateSocialPostStatusAndIncrementUsage(
	ctx context.Context,
	arg UpdateSocialPostStatusAndIncrementUsageParams,
) error {
	var marker int
	return q.db.QueryRow(
		ctx,
		updateSocialPostStatusAndIncrementUsageSQL,
		arg.ID,
		arg.Status,
		arg.PublishedAt,
		arg.WorkspaceID,
		arg.Period,
		arg.PostCount,
	).Scan(&marker)
}
