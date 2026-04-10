-- name: ListManagedUsersByProfile :many
SELECT
  external_user_id::TEXT AS external_user_id,
  COALESCE(
    MAX(external_user_email) FILTER (WHERE external_user_email IS NOT NULL AND external_user_email <> ''),
    ''
  )::TEXT AS external_user_email,
  COUNT(*)::INTEGER AS account_count,
  COUNT(*) FILTER (WHERE platform = 'twitter')::INTEGER  AS twitter_count,
  COUNT(*) FILTER (WHERE platform = 'linkedin')::INTEGER AS linkedin_count,
  COUNT(*) FILTER (WHERE platform = 'bluesky')::INTEGER  AS bluesky_count,
  COUNT(*) FILTER (WHERE status = 'reconnect_required')::INTEGER AS reconnect_count,
  MIN(connected_at)::TIMESTAMPTZ   AS first_connected_at,
  MAX(last_refreshed_at)::TIMESTAMPTZ AS last_refreshed_at
FROM social_accounts
WHERE profile_id = $1
  AND external_user_id IS NOT NULL
  AND disconnected_at IS NULL
  AND connection_type = 'managed'
GROUP BY external_user_id
ORDER BY MIN(connected_at) DESC, external_user_id DESC
LIMIT $2;

-- name: ListManagedAccountsByExternalUser :many
SELECT *
FROM social_accounts
WHERE profile_id = $1
  AND external_user_id = $2
  AND disconnected_at IS NULL
  AND connection_type = 'managed'
ORDER BY connected_at DESC;

-- name: CountManagedUsersByProfile :one
SELECT COUNT(DISTINCT external_user_id)::INTEGER AS total
FROM social_accounts
WHERE profile_id = $1
  AND external_user_id IS NOT NULL
  AND disconnected_at IS NULL
  AND connection_type = 'managed';
